package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

type commandCall struct {
	kind string
	name string
	args []string
}

type fakeCommandRunner struct {
	calls    []commandCall
	outputFn func(string, ...string) (string, error)
	runFn    func(string, ...string) error
	silentFn func(string, ...string) error
}

func (f *fakeCommandRunner) Output(name string, args ...string) (string, error) {
	f.record("output", name, args)
	if f.outputFn != nil {
		return f.outputFn(name, args...)
	}
	return "", errors.New("unexpected output command")
}

func (f *fakeCommandRunner) Run(name string, args ...string) error {
	f.record("run", name, args)
	if f.runFn != nil {
		return f.runFn(name, args...)
	}
	return nil
}

func (f *fakeCommandRunner) Silent(name string, args ...string) error {
	f.record("silent", name, args)
	if f.silentFn != nil {
		return f.silentFn(name, args...)
	}
	return nil
}

func (f *fakeCommandRunner) record(kind, name string, args []string) {
	f.calls = append(f.calls, commandCall{kind: kind, name: name, args: slices.Clone(args)})
}

func useRunner(t *testing.T, runner commandRunner) {
	t.Helper()
	previous := commands
	commands = runner
	t.Cleanup(func() { commands = previous })
}

func useStdin(t *testing.T, input string) {
	t.Helper()
	previous := os.Stdin
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := writer.WriteString(input); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = reader
	t.Cleanup(func() {
		os.Stdin = previous
		_ = reader.Close()
	})
}

func TestKillSessionKillsTmuxBeforeRemovingClone(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	repoPath := filepath.Join(home, "repos", "github.com", "example", "project-2")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		t.Fatal(err)
	}

	killed := false
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
			return "", fmt.Errorf("unexpected output command: %s %v", name, args)
		}
	}
	fake.runFn = func(name string, args ...string) error {
		if name == "tmux" && slices.Contains(args, "kill-session") {
			if _, err := os.Stat(repoPath); err != nil {
				t.Fatalf("repo was removed before tmux session was killed: %v", err)
			}
			killed = true
		}
		return nil
	}
	useRunner(t, fake)
	useStdin(t, "y\n")

	if err := killSession("project-2"); err != nil {
		t.Fatal(err)
	}
	if !killed {
		t.Fatal("expected tmux session to be killed")
	}
	if _, err := os.Stat(repoPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected cloned repo to be removed, got %v", err)
	}
}

func TestCreateProjectSessionRollsBackOnSetupFailure(t *testing.T) {
	setupFailure := errors.New("new window failed")
	fake := &fakeCommandRunner{}
	fake.runFn = func(name string, args ...string) error {
		if name == "tmux" && slices.Contains(args, "new-window") {
			return setupFailure
		}
		return nil
	}
	useRunner(t, fake)

	err := createProjectSession("project", t.TempDir(), false)
	if !errors.Is(err, setupFailure) {
		t.Fatalf("expected setup failure, got %v", err)
	}

	for _, call := range fake.calls {
		if call.kind == "run" && call.name == "tmux" && slices.Contains(call.args, "kill-session") {
			return
		}
	}
	t.Fatal("expected incomplete tmux session to be killed")
}

func TestCloneRepoRemovesCloneWhenJJInitializationFails(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	sourcePath := filepath.Join(home, "repos", "github.com", "example", "project")
	if err := os.MkdirAll(sourcePath, 0o755); err != nil {
		t.Fatal(err)
	}
	destPath := sourcePath + "-2"
	jjFailure := errors.New("jj init failed")

	fake := &fakeCommandRunner{}
	fake.outputFn = func(name string, args ...string) (string, error) {
		if name == "git" && slices.Contains(args, "config") {
			return "git@example.com:example/project.git", nil
		}
		return "", fmt.Errorf("unexpected output command: %s %v", name, args)
	}
	fake.runFn = func(name string, args ...string) error {
		switch name {
		case "git":
			return os.MkdirAll(destPath, 0o755)
		case "jj":
			return jjFailure
		default:
			return fmt.Errorf("unexpected run command: %s %v", name, args)
		}
	}
	useRunner(t, fake)
	useStdin(t, "\n")

	_, err := cloneRepo(repo{Path: sourcePath, Host: "github.com", Org: "example", Name: "project"})
	if !errors.Is(err, jjFailure) {
		t.Fatalf("expected jj failure, got %v", err)
	}
	if _, err := os.Stat(destPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected incomplete clone to be removed, got %v", err)
	}
}

func TestRunDispatchesNamedSessionToKillCommand(t *testing.T) {
	fake := &fakeCommandRunner{}
	fake.runFn = func(name string, args ...string) error { return nil }
	useRunner(t, fake)

	if err := run([]string{"kill", "plain-session"}); err != nil {
		t.Fatal(err)
	}
	for _, call := range fake.calls {
		if call.kind == "run" && call.name == "tmux" && slices.Equal(call.args, []string{"kill-session", "-t=plain-session"}) {
			return
		}
	}
	t.Fatal("expected named tmux session to be killed")
}

func TestRepoFromPathRejectsPathOutsideReposRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	outside := filepath.Join(home, "outside", "example", "project")

	if _, err := repoFromPath(outside); err == nil {
		t.Fatal("expected path outside ~/repos to be rejected")
	}
}
