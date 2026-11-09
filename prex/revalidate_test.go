package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevalidateOnlyVerifiedSurvivingAssociations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		refreshed bool
	}{
		{"same project", true}, {"owned clone", true}, {"empty", false}, {"exited with stale cwd", false},
		{"mixed directories", false}, {"different project", false}, {"relative cwd", false},
		{"uninspectable", false}, {"remote block", false}, {"different host", false},
		{"replacement session ID", false}, {"session gone", false}, {"missing old instance", false},
		{"already current", false}, {"pending launch", false}, {"changed clone identity", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			old := association{Host: r.Host, Instance: "old-instance", Path: path}
			s.Data.Sessions["session:survivor"] = old
			child, foreground := path, path
			sessions := []session{{ID: "session:survivor", Label: "renamed"}}
			entry := lookupSession{Blocks: map[string]lookupBlock{}}
			switch tc.name {
			case "owned clone", "changed clone identity":
				if err := s.ownClone(path, r.Host); err != nil {
					t.Fatal(err)
				}
				if tc.name == "changed clone identity" {
					if err := os.WriteFile(filepath.Join(path, ".git", "prex-owner"), []byte("replacement"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			case "mixed directories":
				foreground = t.TempDir()
			case "different project":
				child, foreground = t.TempDir(), t.TempDir()
			case "relative cwd":
				child, foreground = "relative", "relative"
			case "different host":
				old.Host = "other"
				s.Data.Sessions["session:survivor"] = old
			case "replacement session ID":
				sessions = []session{{ID: "session:replacement", Label: "renamed"}}
			case "session gone":
				sessions = nil
			case "missing old instance":
				old.Instance = ""
				s.Data.Sessions["session:survivor"] = old
			case "already current":
				old.Instance = r.Instance
				s.Data.Sessions["session:survivor"] = old
			case "pending launch":
				s.Data.Pending = &launchIntent{Host: r.Host, Instance: "old-instance", Path: path}
			}
			if tc.name != "empty" {
				entry.Blocks["block:agent"] = lookupBlock{Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"child": map[string]string{"cwd": child}, "foreground": map[string]string{"cwd": foreground}}))}
			}
			if tc.name == "exited with stale cwd" {
				b := entry.Blocks["block:agent"]
				b.Process = lookupProcess(t, jsonTest(map[string]any{"child": map[string]string{"cwd": path}, "last_exit": map[string]int{"exit_code": 1}}))
				entry.Blocks["block:agent"] = b
			}
			if tc.name == "remote block" {
				b := entry.Blocks["block:agent"]
				b.Host = "remote"
				entry.Blocks["block:agent"] = b
			}
			if tc.name == "uninspectable" {
				entry.Error = "inspection failed"
			}
			lookup := map[string]lookupSession{"session:survivor": entry, "session:replacement": entry}
			if err := s.save(); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
			clones, pending := jsonTest(s.Data.Clones), jsonTest(s.Data.Pending)
			n, err := s.revalidateAssociations(r, sessions, lookup)
			if err != nil || (n == 1) != tc.refreshed {
				t.Fatalf("count=%d err=%v", n, err)
			}
			if jsonTest(s.Data.Clones) != clones || jsonTest(s.Data.Pending) != pending {
				t.Fatal("revalidation changed clone ownership or pending launch")
			}
			after, _ := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
			if tc.refreshed {
				if s.pathFor(r, "session:survivor") != path {
					t.Fatal("association not rebound")
				}
				var saved registry
				if json.Unmarshal(after, &saved) != nil || saved.Sessions["session:survivor"].Instance != r.Instance {
					t.Fatal("association not durable")
				}
				n, err = s.revalidateAssociations(r, sessions, lookup)
				if err != nil || n != 0 || len(f.calls) != 1 {
					t.Fatal("current associations were revalidated again")
				}
			} else if !bytes.Equal(before, after) || s.Data.Sessions["session:survivor"] != old || len(f.calls) != 0 {
				t.Fatal("unverified association was changed")
			}
			if _, ok := s.Data.Sessions["session:replacement"]; ok {
				t.Fatal("adopted session by label")
			}
		})
	}
}

func TestRevalidationFailurePreservesOldAssociation(t *testing.T) {
	for _, failure := range []string{"server restarted", "save failed"} {
		t.Run(failure, func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			old := association{Host: r.Host, Instance: "old-instance", Path: path}
			s.Data.Sessions["session:survivor"] = old
			if err := s.save(); err != nil {
				t.Fatal(err)
			}
			originalDir := s.Dir
			before, _ := os.ReadFile(filepath.Join(originalDir, "registry.json"))
			if failure == "save failed" {
				s.Dir = filepath.Join(s.Dir, "missing")
			} else {
				f.output = func(a []string) (string, error) {
					if !strings.Contains(strings.Join(a, " "), "whoami") {
						t.Fatal("unexpected command")
					}
					changed := r.identity
					changed.Instance = "different"
					return jsonTest(changed), nil
				}
			}
			lookup := map[string]lookupSession{"session:survivor": {Blocks: map[string]lookupBlock{"block:agent": {Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"child": map[string]string{"cwd": path}}))}}}}
			if n, err := s.revalidateAssociations(r, []session{{ID: "session:survivor"}}, lookup); err == nil || n != 0 {
				t.Fatal("accepted failed revalidation")
			}
			after, _ := os.ReadFile(filepath.Join(originalDir, "registry.json"))
			if !bytes.Equal(before, after) || s.Data.Sessions["session:survivor"] != old {
				t.Fatal("changed association on failure")
			}
		})
	}
}
