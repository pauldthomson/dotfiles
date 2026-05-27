package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestRepoFromPathParsesHostOrgAndName(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, "repos", "github.com", "example", "project")
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}

	repo, err := repoFromPath(path)
	if err != nil {
		t.Fatal(err)
	}

	if repo.Rel != filepath.Join("github.com", "example", "project") {
		t.Fatalf("unexpected rel: %q", repo.Rel)
	}
	if repo.Host != "github.com" || repo.Org != "example" || repo.Name != "project" {
		t.Fatalf("unexpected repo parts: %#v", repo)
	}
}

func TestNextCloneSuffixSkipsExistingIndexedCopies(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	existingCopy := filepath.Join(home, "repos", "github.com", "example", "project-2")
	if err := os.MkdirAll(existingCopy, 0755); err != nil {
		t.Fatal(err)
	}

	suffix := nextCloneSuffix(repo{Host: "github.com", Org: "example", Name: "project"})
	if suffix != "3" {
		t.Fatalf("expected suffix 3, got %q", suffix)
	}
}

func TestRepoRootForPathFindsRepoFromSubdirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	repoPath := filepath.Join(home, "repos", "github.com", "example", "project")
	subdir := filepath.Join(repoPath, "cmd", "ptmux")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatal(err)
	}

	got, ok := repoRootForPath(subdir)
	if !ok {
		t.Fatal("expected repo root for subdirectory")
	}
	if got != repoPath {
		t.Fatalf("expected %s, got %s", repoPath, got)
	}
}

func TestLooksLikeClonedRepoUsesLongestSiblingWithSameRemote(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	orgDir := filepath.Join(home, "repos", "github.com", "example")
	baseRepo := filepath.Join(orgDir, "project-with-dash")
	cloneRepo := filepath.Join(orgDir, "project-with-dash-2")
	otherPrefixRepo := filepath.Join(orgDir, "project")
	for _, path := range []string{baseRepo, cloneRepo} {
		initGitRepo(t, path, "git@github.com:example/project-with-dash.git")
	}
	initGitRepo(t, otherPrefixRepo, "git@github.com:example/other.git")

	if !looksLikeClonedRepo(cloneRepo) {
		t.Fatal("expected suffixed repo with matching sibling remote to look like a cloned repo")
	}
	if looksLikeClonedRepo(baseRepo) {
		t.Fatal("did not expect base repo to look like a cloned repo")
	}
}

func TestPathWithin(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "child")
	outside := filepath.Join(filepath.Dir(root), "outside")

	if !pathWithin(child, root) {
		t.Fatalf("expected %s to be within %s", child, root)
	}
	if pathWithin(outside, root) {
		t.Fatalf("expected %s not to be within %s", outside, root)
	}
}

func initGitRepo(t *testing.T, path, remote string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", path, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s", out)
	}
	if out, err := exec.Command("git", "-C", path, "remote", "add", "origin", remote).CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %s", out)
	}
}
