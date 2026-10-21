package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func safetyEnvironment(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"jj", "git"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s required: %v", tool, err)
		}
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_COUNT", "0")
	t.Setenv("GIT_AUTHOR_NAME", "Safety Test")
	t.Setenv("GIT_AUTHOR_EMAIL", "safety@example.invalid")
	t.Setenv("GIT_COMMITTER_NAME", "Safety Test")
	t.Setenv("GIT_COMMITTER_EMAIL", "safety@example.invalid")
	for _, key := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_COMMON_DIR", "GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES"} {
		old, exists := os.LookupEnv(key)
		os.Unsetenv(key)
		t.Cleanup(func() {
			if exists {
				os.Setenv(key, old)
			} else {
				os.Unsetenv(key)
			}
		})
	}
	config := filepath.Join(home, "jj-config.toml")
	safetyWrite(t, config, "[user]\nname = 'Safety Test'\nemail = 'safety@example.invalid'\n")
	t.Setenv("JJ_CONFIG", config)
	t.Setenv("NO_COLOR", "1")
	previous := commands
	commands = systemCommandRunner{}
	t.Cleanup(func() { commands = previous })
}

func safetyCommand(t *testing.T, name string, args ...string) string {
	t.Helper()
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s %q: %v\n%s", name, args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func safetyGit(t *testing.T, path string, args ...string) string {
	t.Helper()
	return safetyCommand(t, "git", append([]string{"-C", path}, args...)...)
}

func safetyJJ(t *testing.T, path string, args ...string) string {
	t.Helper()
	return safetyCommand(t, "jj", append([]string{"--no-pager", "--color=never", "-R", path}, args...)...)
}

func safetyWrite(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func safetySource(t *testing.T) string {
	t.Helper()
	path := filepath.Join(reposRoot(), "example.com", "org", "project")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	safetyCommand(t, "jj", "git", "init", "--colocate", path)
	safetyWrite(t, filepath.Join(path, "tracked.txt"), "original\n")
	safetyWrite(t, filepath.Join(path, ".gitignore"), ".cache/\nsecret*\n")
	safetyJJ(t, path, "commit", "-m", "initial")
	safetyJJ(t, path, "bookmark", "create", "main", "-r", "@-")
	// Fixture-only plumbing makes the source's default branch unambiguous.
	safetyGit(t, path, "symbolic-ref", "HEAD", "refs/heads/main")
	return path
}

func safetyCloneFixture(t *testing.T, jj bool) string {
	t.Helper()
	source := safetySource(t)
	dest := filepath.Join(filepath.Dir(source), "project-2")
	if jj {
		safetyCommand(t, "jj", "git", "clone", "--colocate", source, dest)
	} else {
		safetyCommand(t, "git", "clone", source, dest)
	}
	return dest
}

func safetyLocalCommit(t *testing.T, path string) string {
	t.Helper()
	tree := safetyGit(t, path, "rev-parse", "HEAD^{tree}")
	return safetyGit(t, path, "commit-tree", tree, "-p", "HEAD", "-m", "local-only safety fixture")
}

func TestSafetyRemovalRealRepositories(t *testing.T) {
	states := []struct {
		name   string
		mutate func(*testing.T, string)
		ready  bool
	}{
		{"clean", func(*testing.T, string) {}, true},
		{"tracked edit", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, "tracked.txt"), "changed\n") }, false},
		{"untracked", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, "new.txt"), "local\n") }, false},
		{"ignored file", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, "secret.txt"), "local\n") }, false},
		{"ignored directory", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, ".cache", "data"), "local\n") }, false},
		{"nested administration name", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, "nested", ".jj", "data"), "local\n") }, false},
		{"newline filename", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, "secret\n!! .jj"), "local\n") }, false},
		{"staged edit", func(t *testing.T, p string) {
			safetyWrite(t, filepath.Join(p, "tracked.txt"), "staged\n")
			safetyGit(t, p, "add", "tracked.txt")
		}, false},
		{"stash", func(t *testing.T, p string) {
			safetyWrite(t, filepath.Join(p, "tracked.txt"), "stashed\n")
			safetyGit(t, p, "stash", "push", "-m", "keep me")
		}, false},
		{"local branch", func(t *testing.T, p string) {
			safetyGit(t, p, "update-ref", "refs/heads/local", safetyLocalCommit(t, p))
		}, false},
		{"local tag", func(t *testing.T, p string) {
			safetyGit(t, p, "update-ref", "refs/tags/local", safetyLocalCommit(t, p))
		}, false},
		{"custom ref", func(t *testing.T, p string) {
			safetyGit(t, p, "update-ref", "refs/safety/local", safetyLocalCommit(t, p))
		}, false},
		{"custom jj ref", func(t *testing.T, p string) {
			safetyGit(t, p, "update-ref", "refs/jj/user", safetyLocalCommit(t, p))
		}, false},
		{"noncommit ref", func(t *testing.T, p string) {
			tree := safetyGit(t, p, "rev-parse", "HEAD^{tree}")
			safetyGit(t, p, "update-ref", "refs/safety/tree", tree)
		}, false},
		{"detached local head", func(t *testing.T, p string) {
			safetyGit(t, p, "checkout", "--detach", safetyLocalCommit(t, p))
		}, false},
		{"submodule configuration", func(t *testing.T, p string) { safetyWrite(t, filepath.Join(p, ".gitmodules"), "") }, false},
		{"gitlink without configuration", func(t *testing.T, p string) {
			head := safetyGit(t, p, "rev-parse", "HEAD")
			safetyGit(t, p, "update-index", "--add", "--cacheinfo", "160000,"+head+",submodule")
		}, false},
		{"assume unchanged", func(t *testing.T, p string) {
			safetyGit(t, p, "update-index", "--assume-unchanged", "tracked.txt")
			safetyWrite(t, filepath.Join(p, "tracked.txt"), "hidden\n")
		}, false},
		{"fetch failure", func(t *testing.T, p string) {
			safetyGit(t, p, "config", "remote.origin.url", filepath.Join(t.TempDir(), "missing"))
		}, false},
	}
	for _, jj := range []bool{false, true} {
		mode := "git"
		if jj {
			mode = "jj"
		}
		for _, state := range states {
			t.Run(mode+"/"+state.name, func(t *testing.T) {
				safetyEnvironment(t)
				path := safetyCloneFixture(t, jj)
				state.mutate(t, path)
				ready, reason := repoReadyForRemoval(path)
				if ready != state.ready || (ready && reason != "") || (!ready && reason == "") {
					t.Fatalf("ready=%v reason=%q; want ready=%v", ready, reason, state.ready)
				}
				if _, err := os.Stat(path); err != nil {
					t.Fatalf("readiness check removed repo: %v", err)
				}
			})
		}
	}
}

func TestSafetyJJEmptyCommits(t *testing.T) {
	for _, described := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal working copy", true: "described empty"}[described], func(t *testing.T) {
			safetyEnvironment(t)
			path := safetyCloneFixture(t, true)
			if described {
				safetyJJ(t, path, "describe", "-m", "intent worth preserving")
				safetyJJ(t, path, "new")
			}
			ready, reason := repoReadyForRemoval(path)
			if ready == described {
				t.Fatalf("ready=%v reason=%q", ready, reason)
			}
		})
	}
}

func TestSafetyJJCommittedUnpushedChange(t *testing.T) {
	safetyEnvironment(t)
	path := safetyCloneFixture(t, true)
	safetyWrite(t, filepath.Join(path, "tracked.txt"), "committed but unpushed\n")
	safetyJJ(t, path, "commit", "-m", "preserve this commit")
	if ready, reason := repoReadyForRemoval(path); ready || !strings.Contains(reason, "unpushed JJ") {
		t.Fatalf("ready=%v reason=%q", ready, reason)
	}
}

func TestSafetyRemovalRejectsUnsupportedPaths(t *testing.T) {
	safetyEnvironment(t)
	path := safetyCloneFixture(t, true)
	for _, candidate := range []string{filepath.Join(path, "missing"), filepath.Dir(path)} {
		if ready, _ := repoReadyForRemoval(candidate); ready {
			t.Fatalf("accepted non-repository %s", candidate)
		}
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	if ready, _ := repoReadyForRemoval(link); ready {
		t.Fatal("accepted symlink")
	}
	nonColocated := filepath.Join(t.TempDir(), "non-colocated")
	safetyCommand(t, "jj", "git", "clone", "--no-colocate", path, nonColocated)
	if ready, _ := repoReadyForRemoval(nonColocated); ready {
		t.Fatal("accepted non-colocated JJ")
	}
}

func TestSafetyGitBranchWithoutUpstream(t *testing.T) {
	safetyEnvironment(t)
	path := safetyCloneFixture(t, false)
	safetyGit(t, path, "update-ref", "refs/heads/no-upstream", "HEAD")
	if ready, reason := repoReadyForRemoval(path); ready || !strings.Contains(reason, "no upstream") {
		t.Fatalf("ready=%v reason=%q", ready, reason)
	}
}

// Inject a failure at each command on the successful path, using real repos
// for every other command. In particular, a failed JJ probe must not fall back
// to accepting an apparently clean Git checkout.
type safetyFailRunner struct {
	systemCommandRunner
	calls  int
	failAt int
}

func (r *safetyFailRunner) Output(name string, args ...string) (string, error) {
	r.calls++
	if r.calls == r.failAt {
		return "", errors.New("injected safety inspection failure")
	}
	return r.systemCommandRunner.Output(name, args...)
}

func (r *safetyFailRunner) Silent(name string, args ...string) error {
	r.calls++
	if r.calls == r.failAt {
		return errors.New("injected safety fetch failure")
	}
	return r.systemCommandRunner.Silent(name, args...)
}

func TestSafetyRemovalCommandErrorsFailClosed(t *testing.T) {
	for _, jj := range []bool{false, true} {
		t.Run(fmt.Sprintf("jj=%v", jj), func(t *testing.T) {
			safetyEnvironment(t)
			path := safetyCloneFixture(t, jj)
			baseline := &safetyFailRunner{}
			commands = baseline
			if ready, reason := repoReadyForRemoval(path); !ready {
				t.Fatalf("baseline not ready: %s", reason)
			}
			for failAt := 1; failAt <= baseline.calls; failAt++ {
				commands = &safetyFailRunner{failAt: failAt}
				if ready, reason := repoReadyForRemoval(path); ready || reason == "" {
					t.Fatalf("command %d failure accepted: ready=%v reason=%q", failAt, ready, reason)
				}
			}
		})
	}
}

func TestSafetyGitCommitCounts(t *testing.T) {
	for _, text := range []string{"", "nonsense", "-1", "1\nwarning", "999999999999999999999999999"} {
		if _, err := parseGitCommitCount(text); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if count, err := parseGitCommitCount(" 0\n"); err != nil || count != 0 {
		t.Fatalf("count=%d err=%v", count, err)
	}
}
