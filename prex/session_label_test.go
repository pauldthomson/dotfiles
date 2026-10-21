package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRepoSessionLabels(t *testing.T) {
	for _, tc := range []struct {
		r    repo
		want string
	}{
		{repo{Host: "github.com", Org: "nc-helix", Name: "helix"}, "nc-helix/helix"},
		{repo{Host: "github.com", Org: "nc-helix", Name: "helix-2"}, "nc-helix/helix-2"},
		{repo{Host: "git.example.com", Org: "another", Name: "helix"}, "another/helix"},
	} {
		if got := repoSessionLabel(tc.r); got != tc.want {
			t.Errorf("got %q want %q", got, tc.want)
		}
	}
	long := repo{Host: "github.com", Org: strings.Repeat("org", 60), Name: strings.Repeat("界", 45) + "-project"}
	first := repoSessionLabel(long)
	if len(first) > 110 || !utf8.ValidString(first) || !strings.Contains(first, "-project-") {
		t.Fatalf("invalid shortened label %q", first)
	}
	if repoSessionLabel(long) != first {
		t.Fatal("label is not stable")
	}
	long.Org += "different"
	if repoSessionLabel(long) == first {
		t.Fatal("different long paths have same label")
	}
}

func TestOpenNewSessionUsesRepoPathLabel(t *testing.T) {
	s := isolatedState(t)
	projectDir(t)
	r, f := fakeRex(t)
	base := f.output
	created := false
	f.output = func(a []string) (string, error) {
		args := rexTail(a)
		if strings.Join(args, " ") == "ls --json" {
			return `{"sessions":[]}`, nil
		}
		if args[0] == "new" {
			created = true
			if args[1] != "org/project" {
				t.Fatalf("wrong session label: %v", args)
			}
			return `{"session_id":"session:new"}`, nil
		}
		return base(a)
	}
	if err := openCommand(r, s, "project"); err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("session wasn't created")
	}
}

func TestExistingSessionRenamedToRepoPath(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "switch", true: "reopen"}[empty], func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			base := f.output
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				switch strings.Join(args, " ") {
				case "session inspect session:old --json":
					if empty {
						return `{"session_id":"session:old","label":"misty-prairie"}`, nil
					}
					return `{"session_id":"session:old","label":"misty-prairie","windows":[{"window_id":"window:existing"}]}`, nil
				case "block ls -s session:old --json":
					return `{"blocks":[]}`, nil
				}
				if args[0] == "window" {
					return `{"window_id":"window:agent"}`, nil
				}
				return base(a)
			}
			if err := openExisting(r, s, session{ID: "session:old", Label: "misty-prairie"}, path); err != nil {
				t.Fatal(err)
			}
			renamed := false
			for _, call := range f.calls {
				if strings.Join(rexTail(call), " ") == "session rename session:old org/project" {
					renamed = true
				}
			}
			if !renamed {
				t.Fatal("existing session was not renamed")
			}
		})
	}
}
