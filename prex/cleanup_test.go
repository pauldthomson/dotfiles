package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCleanupRealCloneOutcomes(t *testing.T) {
	for _, outcome := range []string{"remove", "decline", "kill failure", "dirty after close", "open files"} {
		t.Run(outcome, func(t *testing.T) {
			safetyEnvironment(t)
			path := safetyCloneFixture(t, true)
			t.Setenv("XDG_STATE_HOME", filepath.Join(os.Getenv("HOME"), "state"))
			s, err := lockState(false)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			r, f := fakeRex(t)
			base := f.output
			if err := s.ownClone(path, r.Host); err != nil {
				t.Fatal(err)
			}
			s.associate(r, "session:clone", path)
			c := s.Data.Clones[path]
			closed := false
			f.output = func(a []string) (string, error) {
				if a[0] != "rex" {
					return (systemCommandRunner{}).Output(a[0], a[1:]...)
				}
				tail := strings.Join(rexTail(a), " ")
				switch tail {
				case "ls --json":
					if closed {
						return `{"sessions":[]}`, nil
					}
					return `{"sessions":[{"session_id":"session:clone","label":"clone"}]}`, nil
				case "block ls -s session:clone --json":
					return `{"blocks":[]}`, nil
				}
				return base(a)
			}
			f.run = func(a []string) error {
				if a[0] != "rex" {
					return (systemCommandRunner{}).Run(a[0], a[1:]...)
				}
				if strings.Join(rexTail(a), " ") != "kill session:clone" {
					return fmt.Errorf("unexpected %v", a)
				}
				if outcome == "kill failure" {
					return errors.New("RPC timeout")
				}
				closed = true
				if outcome == "dirty after close" {
					return os.WriteFile(filepath.Join(path, "new-work"), []byte("keep"), 0600)
				}
				return nil
			}
			old := noOpenFiles
			t.Cleanup(func() { noOpenFiles = old })
			noOpenFiles = func(string) error {
				if outcome == "open files" {
					return errors.New("writer still active")
				}
				return nil
			}
			task := cleanupTask{Session: "session:clone", Path: path, Clone: &c, Remove: outcome != "decline"}
			err = executeCleanup(r, s, task)
			if outcome == "remove" || outcome == "decline" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("expected failure")
			}
			_, statErr := os.Stat(path)
			if outcome == "remove" {
				if !errors.Is(statErr, os.ErrNotExist) {
					t.Fatal("clone retained unexpectedly")
				}
				if _, ok := s.Data.Clones[path]; ok {
					t.Fatal("ownership not removed")
				}
			} else if statErr != nil {
				t.Fatalf("clone lost: %v", statErr)
			}
		})
	}
}
func TestOwnershipMarkerRequired(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	if err := s.ownClone(path, "host"); err != nil {
		t.Fatal(err)
	}
	c := s.Data.Clones[path]
	if err := os.WriteFile(filepath.Join(path, ".git", "prex-owner"), []byte("different generation"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedClone(path, c, "host"); err == nil {
		t.Fatal("removed clone with changed generation")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("clone lost")
	}
}
func TestConfirmationDoesNotDiscardFollowingInput(t *testing.T) {
	safetyInput(t, "yes\nno\n")
	first, err := confirm("")
	if err != nil || !first {
		t.Fatalf("first=%v %v", first, err)
	}
	second, err := confirm("")
	if err != nil || second {
		t.Fatalf("second=%v %v", second, err)
	}
}
