package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGitRepoReadyForRemovalRejectsUnpushedDetachedHead(t *testing.T) {
	root := t.TempDir()
	remotePath := filepath.Join(root, "remote.git")
	repoPath := filepath.Join(root, "clone")
	runGitCommand(t, "", "init", "--bare", remotePath)
	runGitCommand(t, "", "clone", remotePath, repoPath)
	runGitCommand(t, repoPath, "config", "user.name", "Test User")
	runGitCommand(t, repoPath, "config", "user.email", "test@example.com")

	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repoPath, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("tracked.txt", "pushed\n")
	runGitCommand(t, repoPath, "add", "tracked.txt")
	runGitCommand(t, repoPath, "commit", "-m", "pushed commit")
	runGitCommand(t, repoPath, "branch", "-M", "main")
	runGitCommand(t, repoPath, "push", "-u", "origin", "main")
	runGitCommand(t, repoPath, "checkout", "--detach")
	writeFile("detached.txt", "not pushed\n")
	runGitCommand(t, repoPath, "add", "detached.txt")
	runGitCommand(t, repoPath, "commit", "-m", "detached commit")

	ready, reason := gitRepoReadyForRemoval(repoPath)
	if ready {
		t.Fatal("expected unpushed detached HEAD to block removal")
	}
	if !strings.Contains(reason, "Detached HEAD") {
		t.Fatalf("expected detached HEAD reason, got %q", reason)
	}
}

func TestGitRepoReadyForRemovalRejectsUnpushedStash(t *testing.T) {
	repoPath := createPushedGitClone(t)
	if err := os.WriteFile(filepath.Join(repoPath, "tracked.txt"), []byte("stashed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, repoPath, "stash", "push", "-m", "local stash")

	ready, reason := gitRepoReadyForRemoval(repoPath)
	if ready {
		t.Fatal("expected an unpushed stash to block removal")
	}
	if !strings.Contains(reason, "Local Git refs") {
		t.Fatalf("expected local Git refs reason, got %q", reason)
	}
}

func TestJJRepoReadyForRemovalRejectsDescribedEmptyCommit(t *testing.T) {
	repoPath := createPushedGitClone(t)
	runJJCommand(t, "", "git", "init", "--colocate", repoPath)
	runJJCommand(t, repoPath, "describe", "-m", "important empty change")

	ready, reason := jjRepoReadyForRemoval(repoPath)
	if ready {
		t.Fatal("expected a described empty jj commit to block removal")
	}
	if !strings.Contains(reason, "unpushed jj changes") {
		t.Fatalf("expected unpushed jj reason, got %q", reason)
	}
}

func TestGitRepoReadyForRemovalRejectsDirtyRepository(t *testing.T) {
	fake := &fakeCommandRunner{}
	fake.outputFn = func(name string, args ...string) (string, error) {
		if name == "git" && slices.Contains(args, "status") {
			return " M tracked.txt\n", nil
		}
		return "", errors.New("unexpected output command")
	}
	useRunner(t, fake)

	ready, reason := gitRepoReadyForRemoval(t.TempDir())
	if ready || !strings.Contains(reason, "uncommitted or untracked") {
		t.Fatalf("expected dirty repository refusal, got ready=%v reason=%q", ready, reason)
	}
}

func TestGitRepoReadyForRemovalRejectsFetchFailure(t *testing.T) {
	fetchFailure := errors.New("network unavailable")
	fake := &fakeCommandRunner{
		silentFn: func(string, ...string) error { return fetchFailure },
	}
	useRunner(t, fake)

	ready, reason := gitRepoReadyForRemoval(t.TempDir())
	if ready || !strings.Contains(reason, fetchFailure.Error()) {
		t.Fatalf("expected fetch failure refusal, got ready=%v reason=%q", ready, reason)
	}
}

func TestJJRepoReadyForRemovalRejectsUnpushedChange(t *testing.T) {
	fake := &fakeCommandRunner{}
	fake.outputFn = func(name string, args ...string) (string, error) {
		switch {
		case name == "jj" && slices.Contains(args, "root"):
			return "/repo", nil
		case name == "jj" && slices.Contains(args, "fetch"):
			return "", nil
		case name == "jj" && slices.Contains(args, "status"):
			return "The working copy is clean\n", nil
		case name == "jj" && slices.Contains(args, "log"):
			return "abc123 unpushed change\n", nil
		default:
			return "", errors.New("unexpected output command")
		}
	}
	useRunner(t, fake)

	ready, reason := repoReadyForRemoval("/repo")
	if ready || !strings.Contains(reason, "unpushed jj changes") {
		t.Fatalf("expected unpushed jj refusal, got ready=%v reason=%q", ready, reason)
	}
}

func TestKillSessionDecliningRemovalKeepsCloneAndUsesExactTarget(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoPath := filepath.Join(home, "repos", "github.com", "example", "project-2")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	fake := cleanCloneRunner(repoPath)
	useRunner(t, fake)
	useStdin(t, "n\n")

	if err := killSession("project"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(repoPath); err != nil {
		t.Fatalf("expected clone to remain after declining removal: %v", err)
	}
	for _, call := range fake.calls {
		if call.kind == "run" && call.name == "tmux" && slices.Equal(call.args, []string{"kill-session", "-t=project"}) {
			return
		}
	}
	t.Fatal("expected exact tmux session target")
}

func TestKillSessionRevalidatesCloneAfterConfirmation(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoPath := filepath.Join(home, "repos", "github.com", "example", "project-2")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	fake := cleanCloneRunner(repoPath)
	cleanOutput := fake.outputFn
	statusCalls := 0
	fake.outputFn = func(name string, args ...string) (string, error) {
		if name == "git" && slices.Contains(args, "status") {
			statusCalls++
			if statusCalls > 1 {
				return "?? created-while-confirming.txt\n", nil
			}
		}
		return cleanOutput(name, args...)
	}
	useRunner(t, fake)
	useStdin(t, "y\n")

	err := killSession("project-2")
	if err == nil || !strings.Contains(err.Error(), "retained") {
		t.Fatalf("expected changed clone to be retained, got %v", err)
	}
	if _, statErr := os.Stat(repoPath); statErr != nil {
		t.Fatalf("expected revalidated clone to remain: %v", statErr)
	}
	for _, call := range fake.calls {
		if call.kind == "run" && call.name == "tmux" && slices.Equal(call.args, []string{"kill-session", "-t=project-2"}) {
			return
		}
	}
	t.Fatal("expected session to be killed before final revalidation")
}

func TestSetTmuxOptionOnNewSession(t *testing.T) {
	// tmux's Unix socket has a small path-length limit, so use a short /tmp path.
	tmuxDir, err := os.MkdirTemp("/tmp", "ptmux-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmuxDir) })
	t.Setenv("TMUX_TMPDIR", tmuxDir)
	t.Setenv("TMUX", "")

	runTmux := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("tmux", args...).CombinedOutput()
		if err != nil {
			t.Fatalf("tmux %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runTmux("new-session", "-d", "-s", "teamcity-cli")
	t.Cleanup(func() { _ = exec.Command("tmux", "kill-server").Run() })

	if err := setTmuxOption("teamcity-cli", "@ptmux_repo_path", "/tmp/teamcity-cli"); err != nil {
		t.Fatal(err)
	}
	got, err := tmuxOption("teamcity-cli", "@ptmux_repo_path")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/teamcity-cli" {
		t.Fatalf("unexpected tmux option value %q", got)
	}
}

func TestRemoveClonedRepoRejectsSymlinkEscape(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(home, "repos")
	outsideHost := filepath.Join(home, "outside", "github.com")
	outsideRepo := filepath.Join(outsideHost, "example", "project-2")
	if err := os.MkdirAll(outsideRepo, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(outsideRepo, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideHost, filepath.Join(root, "github.com")); err != nil {
		t.Fatal(err)
	}

	repoPath := filepath.Join(root, "github.com", "example", "project-2")
	if err := removeClonedRepo(repoPath, true); err == nil {
		t.Fatal("expected removal through an escaping symlink to be rejected")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("outside repository was modified: %v", err)
	}
}

func cleanCloneRunner(repoPath string) *fakeCommandRunner {
	fake := &fakeCommandRunner{}
	fake.outputFn = func(name string, args ...string) (string, error) {
		switch {
		case name == "tmux" && slices.Contains(args, "@ptmux_repo_path"):
			return repoPath, nil
		case name == "tmux" && slices.Contains(args, "@ptmux_is_copy"):
			return "1", nil
		case name == "jj":
			return "", errors.New("not a jj repository")
		case name == "git" && slices.Contains(args, "status"):
			return "", nil
		case name == "git" && slices.Contains(args, "symbolic-ref"):
			return "refs/heads/main", nil
		case name == "git" && slices.Contains(args, "rev-list"):
			return "0", nil
		case name == "git" && slices.Contains(args, "for-each-ref"):
			return "", nil
		default:
			return "", errors.New("unexpected output command")
		}
	}
	return fake
}

func createPushedGitClone(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	remotePath := filepath.Join(root, "remote.git")
	repoPath := filepath.Join(root, "clone")
	runGitCommand(t, "", "init", "--bare", remotePath)
	runGitCommand(t, "", "clone", remotePath, repoPath)
	runGitCommand(t, repoPath, "config", "user.name", "Test User")
	runGitCommand(t, repoPath, "config", "user.email", "test@example.com")
	if err := os.WriteFile(filepath.Join(repoPath, "tracked.txt"), []byte("pushed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCommand(t, repoPath, "add", "tracked.txt")
	runGitCommand(t, repoPath, "commit", "-m", "pushed commit")
	runGitCommand(t, repoPath, "branch", "-M", "main")
	runGitCommand(t, repoPath, "push", "-u", "origin", "main")
	return repoPath
}

func runJJCommand(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	jjArgs := args
	if repoPath != "" {
		jjArgs = append([]string{"-R", repoPath}, args...)
	}
	if out, err := exec.Command("jj", jjArgs...).CombinedOutput(); err != nil {
		t.Fatalf("jj %v: %v\n%s", jjArgs, err, out)
	}
}

func runGitCommand(t *testing.T, repoPath string, args ...string) {
	t.Helper()
	gitArgs := args
	if repoPath != "" {
		gitArgs = append([]string{"-C", repoPath}, args...)
	}
	if out, err := exec.Command("git", gitArgs...).CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", gitArgs, err, out)
	}
}
