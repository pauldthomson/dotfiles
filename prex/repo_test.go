package main

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSafetyRepoLayout(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, rel := range []string{"z.example/org/z", "a.example/org/project", "a.example/other/project", "a.example/org/project*"} {
		if err := os.MkdirAll(filepath.Join(reposRoot(), rel), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	link := filepath.Join(reposRoot(), "a.example", "org", "linked")
	if err := os.Symlink(filepath.Join(reposRoot(), "a.example", "org", "project"), link); err != nil {
		t.Fatal(err)
	}
	repos, err := findRepos("")
	if err != nil {
		t.Fatal(err)
	}
	var rels []string
	for _, r := range repos {
		rels = append(rels, r.Rel)
	}
	want := []string{"a.example/org/project", "a.example/org/project*", "a.example/other/project", "z.example/org/z"}
	if !reflect.DeepEqual(rels, want) {
		t.Fatalf("got %v, want %v", rels, want)
	}
	repos, err = findRepos("project*")
	if err != nil || len(repos) != 1 || repos[0].Name != "project*" {
		t.Fatalf("literal project lookup: %v, %v", repos, err)
	}
	path := filepath.Join(reposRoot(), "a.example", "org", "project")
	r, err := repoFromPath(path)
	if err != nil || r.Path != path || r.Host != "a.example" || r.Org != "org" || r.Name != "project" || repoDisplayName(r) != "a.example/org/project" {
		t.Fatalf("repo=%+v err=%v", r, err)
	}
	root, ok := repoRootForPath(filepath.Join(path, "nested", "file"))
	if !ok || root != path {
		t.Fatalf("root=%q ok=%v", root, ok)
	}
	for _, bad := range []string{reposRoot(), filepath.Dir(path), filepath.Join(path, "nested"), reposRoot() + "-outside/host/org/repo"} {
		if _, err := repoFromPath(bad); err == nil {
			t.Fatalf("accepted bad repo path %q", bad)
		}
	}
	if _, ok := repoRootForPath(reposRoot() + "-outside/host/org/repo/nested"); ok {
		t.Fatal("accepted sibling prefix")
	}
	if !samePath(path, path+"/.") || samePath(path, path+"-2") || !pathWithin(path, reposRoot()) || pathWithin(reposRoot()+"-other", reposRoot()) {
		t.Fatal("incorrect lexical path comparison")
	}
}

func TestSafetyNextCloneSuffixRespectsAllEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	r := repo{Host: "example.com", Org: "org", Name: "project"}
	parent := filepath.Join(reposRoot(), r.Host, r.Org)
	if err := os.MkdirAll(filepath.Join(parent, "project-2"), 0o755); err != nil {
		t.Fatal(err)
	}
	safetyWrite(t, filepath.Join(parent, "project-3"), "do not overwrite")
	if err := os.Symlink(filepath.Join(parent, "missing"), filepath.Join(parent, "project-4")); err != nil {
		t.Fatal(err)
	}
	if suffix, err := nextCloneSuffix(r); err != nil || suffix != "5" {
		t.Fatalf("suffix=%q err=%v", suffix, err)
	}
	r.Org = "../outside"
	if _, err := nextCloneSuffix(r); err == nil {
		t.Fatal("accepted traversal")
	}
}

func safetyInput(t *testing.T, text string) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(text); err != nil {
		t.Fatal(err)
	}
	if _, err := file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = old; file.Close() })
}

func TestSafetyCloneRealJJ(t *testing.T) {
	safetyEnvironment(t)
	sourcePath := safetySource(t)
	source, err := repoFromPath(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	safetyInput(t, "\n")
	cloned, err := cloneRepo(source)
	if err != nil {
		t.Fatal(err)
	}
	if cloned.Name != "project-2" {
		t.Fatalf("clone=%+v", cloned)
	}
	for _, admin := range []string{".git", ".jj"} {
		if info, err := os.Stat(filepath.Join(cloned.Path, admin)); err != nil || !info.IsDir() {
			t.Fatalf("missing %s after clone success: %v", admin, err)
		}
	}
	if ready, reason := repoReadyForRemoval(cloned.Path); !ready {
		t.Fatalf("fresh clone refused: %s", reason)
	}
}

func TestSafetyCloneRealFailureKeepsReservation(t *testing.T) {
	safetyEnvironment(t)
	path := safetySource(t)
	safetyGit(t, path, "config", "remote.origin.url", filepath.Join(t.TempDir(), "missing-remote"))
	source, err := repoFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	safetyInput(t, "\n")
	cloned, err := cloneRepo(source)
	if err == nil || cloned != (repo{}) {
		t.Fatalf("clone=%+v err=%v", cloned, err)
	}
	if info, statErr := os.Stat(path + "-2"); statErr != nil || !info.IsDir() {
		t.Fatalf("failed clone reservation removed: %v", statErr)
	}
	if !strings.Contains(err.Error(), path+"-2") {
		t.Fatalf("error does not locate incomplete clone: %v", err)
	}
}

type safetyCloneRunner struct {
	t       *testing.T
	remote  string
	dest    string
	failure error
}

func (r *safetyCloneRunner) Output(name string, args ...string) (string, error) {
	if name != "git" || !reflect.DeepEqual(args[2:], []string{"config", "--get", "remote.origin.url"}) {
		r.t.Fatalf("unexpected output command: %s %q", name, args)
	}
	return r.remote + "\n", nil
}

func (r *safetyCloneRunner) Run(name string, args ...string) error {
	want := []string{"git", "clone", "--colocate", "--", r.remote, r.dest}
	if name != "jj" || !reflect.DeepEqual(args, want) {
		r.t.Fatalf("clone command: %s %q; want jj %q", name, args, want)
	}
	entries, err := os.ReadDir(r.dest)
	if err != nil || len(entries) != 0 {
		r.t.Fatalf("destination not exclusively reserved empty: %v %v", entries, err)
	}
	safetyWrite(r.t, filepath.Join(r.dest, "partial-data"), "preserve me")
	return r.failure
}

func (r *safetyCloneRunner) Silent(name string, args ...string) error {
	r.t.Fatalf("unexpected silent command: %s %q", name, args)
	return errors.New("unexpected command")
}

func TestSafetyClonePreservesFailureAndExistingEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(reposRoot(), "example.com", "org", "project")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	source, err := repoFromPath(path)
	if err != nil {
		t.Fatal(err)
	}
	safetyWrite(t, path+"-2", "existing data")
	if err := os.Symlink("missing", path+"-3"); err != nil {
		t.Fatal(err)
	}
	safetyInput(t, "\n")
	failure := errors.New("initialization failed after fetch")
	runner := &safetyCloneRunner{t: t, remote: "https://example.invalid/repo", dest: path + "-4", failure: failure}
	old := commands
	commands = runner
	t.Cleanup(func() { commands = old })
	cloned, err := cloneRepo(source)
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), runner.dest) || cloned != (repo{}) {
		t.Fatalf("clone=%+v err=%v", cloned, err)
	}
	for p, want := range map[string]string{path + "-2": "existing data", filepath.Join(runner.dest, "partial-data"): "preserve me"} {
		content, err := os.ReadFile(p)
		if err != nil || string(content) != want {
			t.Fatalf("lost data in %s: %q %v", p, content, err)
		}
	}
	if _, err := os.Readlink(path + "-3"); err != nil {
		t.Fatalf("lost existing symlink: %v", err)
	}
}
