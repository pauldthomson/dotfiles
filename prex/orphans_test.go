package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOrphanReportIsConservativeAndReadOnly(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"no sessions", "orphan"}, {"observed use", "in-use"}, {"recorded empty session", "in-use"},
		{"closed recorded session", "orphan"}, {"unverified restored session", "unknown"},
		{"uninspectable session", "unknown"}, {"pending launch", "unknown"},
		{"replaced identity", "unknown"}, {"missing directory", "unknown"},
		{"foreign ownership", "other-host"}, {"same label unrelated cwd", "orphan"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			if err := s.ownClone(path, r.Host); err != nil {
				t.Fatal(err)
			}
			var sessions []session
			entries := map[string]lookupSession{}
			if tc.name == "observed use" || tc.name == "same label unrelated cwd" {
				cwd := path
				if tc.name == "same label unrelated cwd" {
					cwd = t.TempDir()
				}
				sessions = []session{{ID: "session:live", Label: "project"}}
				entries["session:live"] = lookupSession{Blocks: map[string]lookupBlock{"block:detached": {
					Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"foreground": map[string]string{"cwd": cwd}})),
				}}}
			}
			switch tc.name {
			case "recorded empty session", "closed recorded session", "unverified restored session":
				s.associate(r, "session:recorded", path)
				if tc.name != "closed recorded session" {
					sessions = []session{{ID: "session:recorded"}}
					entries["session:recorded"] = lookupSession{Blocks: map[string]lookupBlock{}}
				}
				if tc.name == "unverified restored session" {
					a := s.Data.Sessions["session:recorded"]
					a.Instance = "old-instance"
					s.Data.Sessions["session:recorded"] = a
				}
			case "uninspectable session":
				sessions = []session{{ID: "session:unknown"}}
				entries["session:unknown"] = lookupSession{Error: "inspection failed"}
			case "pending launch":
				s.Data.Pending = &launchIntent{Host: r.Host, Instance: "old", Path: path}
			case "replaced identity":
				if err := os.WriteFile(filepath.Join(path, ".git", "prex-owner"), []byte("replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing directory":
				if err := os.Rename(path, path+"-moved"); err != nil {
					t.Fatal(err)
				}
			case "foreign ownership":
				c := s.Data.Clones[path]
				c.Host = "other-host"
				s.Data.Clones[path] = c
			}
			if err := s.save(); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				if len(args) != 6 || args[0] != "do" || args[2] != lookupScript {
					t.Fatal("report issued a non-collection command")
				}
				var request struct {
					Known map[string]bool `json:"known_sessions"`
				}
				if err := json.Unmarshal([]byte(args[4]), &request); err != nil || len(request.Known) != 0 {
					t.Fatal("report skipped inspecting recorded sessions")
				}
				return lookupResult(r, sessions, entries), nil
			}
			rows, err := orphanReport(r, s)
			if err != nil || len(rows) != 1 || rows[0].Status != tc.want || rows[0].Path != path {
				t.Fatalf("rows=%v error=%v", rows, err)
			}
			after, _ := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
			var saved registry
			if !bytes.Equal(before, after) || json.Unmarshal(before, &saved) != nil || jsonTest(saved) != jsonTest(s.Data) {
				t.Fatal("report changed the registry")
			}
			if len(f.calls) != 1 {
				t.Fatal("report performed extra commands")
			}
		})
	}
}

func TestOrphanReportFailureDoesNotClaimNoUse(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	if err := s.ownClone(path, r.Host); err != nil {
		t.Fatal(err)
	}
	f.output = func([]string) (string, error) { return "", errors.New("disconnected") }
	var out bytes.Buffer
	if err := orphansCommand(r, s, nil, &out); err == nil || out.Len() != 0 {
		t.Fatal("failed inspection produced a report")
	}
	if err := orphansCommand(r, s, []string{"--delete"}, &out); err == nil {
		t.Fatal("accepted deletion option")
	}
}

func TestOrphansCommandDoesNotCreateStateOrRequireApp(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", filepath.Join(os.Getenv("HOME"), "absent-state"))
	_, f := fakeRex(t)
	if err := run([]string{"orphans"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(stateDir()); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("read-only command created state")
	}
	if len(f.calls) != 1 || !strings.Contains(strings.Join(f.calls[0], " "), "whoami") {
		t.Fatal("empty report required an app or changed Rex")
	}
}

func TestReadOnlySnapshotRejectsCorruptState(t *testing.T) {
	s := isolatedState(t)
	file := filepath.Join(s.Dir, "registry.json")
	for _, data := range []string{"broken", `{"version":999}`, `{"version":1,"sessions":{},"clones":null}`} {
		if err := os.WriteFile(file, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := readStateSnapshot(); err == nil {
			t.Fatal("accepted corrupt state")
		}
		after, _ := os.ReadFile(file)
		if string(after) != data {
			t.Fatal("changed corrupt evidence")
		}
	}
}
