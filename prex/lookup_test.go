package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func lookupProcess(t *testing.T, value string) *processInfo {
	t.Helper()
	var p processInfo
	if err := json.Unmarshal([]byte(value), &p); err != nil {
		t.Fatal(err)
	}
	return &p
}

func lookupResult(r *rexClient, sessions []session, entries map[string]lookupSession) string {
	result := make(map[string]lookupSession)
	for i, s := range sessions {
		entry := entries[s.ID]
		entry.Label, entry.Position = s.Label, i+1
		result[s.ID] = entry
	}
	return jsonTest(map[string]any{"host_id": r.Host, "instance_id": r.Instance, "session_count": len(sessions), "sessions": result})
}

func TestLookupKeepsOrderAndSkipsRecordedSessions(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	s.associate(r, "session:recorded", path)
	s.associate(r, "session:elsewhere", t.TempDir())
	s.associate(r, "session:gone", path)
	s.Data.Sessions["session:foreign"] = association{Host: "other", Instance: r.Instance, Path: path}
	// An association from a previous server instance must be rediscovered.
	s.Data.Sessions["session:stale"] = association{Host: r.Host, Instance: "old", Path: path}
	valid := lookupBlock{Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"child": map[string]string{"cwd": path}}))}
	sessions := []session{{ID: "session:z", Label: "first"}, {ID: "session:recorded"}, {ID: "session:bad"}, {ID: "session:a", Label: "last match"}, {ID: "session:elsewhere"}, {ID: "session:stale"}}
	f.output = func(a []string) (string, error) {
		args := rexTail(a)
		if len(args) != 6 || args[0] != "do" || args[1] != "-e" || args[2] != lookupScript || args[3] != "--args" || args[5] != "--json" {
			t.Fatalf("unexpected lookup command")
		}
		var request struct {
			Known map[string]bool `json:"known_sessions"`
		}
		if err := json.Unmarshal([]byte(args[4]), &request); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(request.Known, map[string]bool{"session:recorded": true, "session:elsewhere": true, "session:gone": true}) {
			t.Fatalf("wrong inspection exclusions: %v", request.Known)
		}
		return lookupResult(r, sessions, map[string]lookupSession{
			"session:a":     {Blocks: map[string]lookupBlock{"block:a": valid}},
			"session:z":     {Blocks: map[string]lookupBlock{"block:z": valid}},
			"session:bad":   {Error: "session disappeared"},
			"session:stale": {Blocks: map[string]lookupBlock{}},
		}), nil
	}
	listed, collected, err := r.lookupSessions(s)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(listed, sessions) {
		t.Fatalf("session order/labels changed: %v", listed)
	}
	got := sessionsForRepo(r, s, listed, collected, path)
	want := []session{sessions[0], sessions[1], sessions[3]}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("matches %v, want %v", got, want)
	}
	if len(f.calls) != 1 {
		t.Fatalf("lookup made %d CLI calls", len(f.calls))
	}
	if s.pathFor(r, "session:z") != "" || len(s.Data.Clones) != 0 {
		t.Fatal("discovery changed associations or clone ownership")
	}
}

func TestEmptyAndRecordedLookupUseOnlyOneCall(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	s.associate(r, "session:known", path)
	for _, sessions := range [][]session{nil, {{ID: "session:known"}}} {
		f.output = func(a []string) (string, error) {
			if rexTail(a)[0] != "do" {
				t.Fatal("issued a separate list or process inspection")
			}
			return lookupResult(r, sessions, nil), nil
		}
		listed, collected, err := r.lookupSessions(s)
		if err != nil {
			t.Fatal(err)
		}
		got := sessionsForRepo(r, s, listed, collected, path)
		if len(got) != len(sessions) {
			t.Fatalf("matches=%v", got)
		}
	}
	if len(f.calls) != 2 {
		t.Fatal("expected one CLI invocation per lookup")
	}
}

func TestLookupFailureDoesNotLaunchDuplicateSession(t *testing.T) {
	for _, response := range []string{
		"transport-error", "not json", `{}`,
		`{"host_id":"other","instance_id":"instance","session_count":0,"sessions":{}}`,
		`{"host_id":"host","instance_id":"restarted","session_count":0,"sessions":{}}`,
		`{"host_id":"host","instance_id":"instance","sessions":{}}`,
		`{"host_id":"host","instance_id":"instance","session_count":0}`,
		`{"host_id":"host","instance_id":"instance","session_count":0,"sessions":null}`,
		`{"host_id":"host","instance_id":"instance","session_count":1,"sessions":{}}`,
		`{"host_id":"host","instance_id":"instance","session_count":1,"sessions":{"bad-id":{"position":1}}}`,
		`{"host_id":"host","instance_id":"instance","session_count":1,"sessions":{"session:a":{"position":0}}}`,
		`{"host_id":"host","instance_id":"instance","session_count":1,"sessions":{"session:a":{"position":2}}}`,
		`{"host_id":"host","instance_id":"instance","session_count":2,"sessions":{"session:a":{"position":1},"session:b":{"position":1}}}`,
	} {
		t.Run(response, func(t *testing.T) {
			s := isolatedState(t)
			projectDir(t)
			r, f := fakeRex(t)
			f.output = func(a []string) (string, error) {
				tail := rexTail(a)
				if tail[0] != "do" {
					t.Fatalf("unexpected command after failed lookup: %s", tail[0])
				}
				if response == "transport-error" {
					return "", errors.New("disconnected")
				}
				return response, nil
			}
			if err := openCommand(r, s, "project"); err == nil {
				t.Fatal("lookup failure permitted opening a session")
			}
			if len(f.calls) != 1 || s.Data.Pending != nil || len(s.Data.Clones) != 0 {
				t.Fatal("lookup failure had launch or clone side effects")
			}
		})
	}
}

func TestLookupDirectoryValidation(t *testing.T) {
	r, _ := fakeRex(t)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	live := func(cwd string) *processInfo {
		return lookupProcess(t, jsonTest(map[string]any{"foreground": map[string]string{"cwd": cwd}}))
	}
	valid := lookupBlock{Host: r.Host, Creator: "com.superlogical.terminal", Process: live(root)}
	type pathCase struct {
		name    string
		entry   lookupSession
		want    []string
		wantErr bool
	}
	cases := []pathCase{
		{"empty session", lookupSession{Blocks: map[string]lookupBlock{}}, nil, false},
		{"missing blocks", lookupSession{}, nil, true},
		{"session closed", lookupSession{Error: "session_not_found"}, nil, true},
		{"valid cwd", lookupSession{Blocks: map[string]lookupBlock{"block:a": valid}}, []string{root}, false},
		{"invalid ID", lookupSession{Blocks: map[string]lookupBlock{"invalid": valid}}, nil, true},
	}
	for _, tc := range []struct {
		name string
		b    lookupBlock
		want []string
		bad  bool
	}{
		{"symlink", lookupBlock{Host: r.Host, Creator: valid.Creator, Process: live(link)}, []string{root}, false},
		{"relative cwd", lookupBlock{Host: r.Host, Creator: valid.Creator, Process: live("relative")}, nil, true},
		{"unresolved cwd", lookupBlock{Host: r.Host, Creator: valid.Creator, Process: live(filepath.Join(root, "missing"))}, nil, true},
		{"unknown host", lookupBlock{Creator: valid.Creator, Process: live(root)}, nil, true},
		{"remote host", lookupBlock{Host: "remote", Creator: valid.Creator, Process: live(root)}, nil, true},
		{"nonterminal", lookupBlock{Host: r.Host, Creator: "other", Process: live(root)}, nil, true},
		{"missing process", lookupBlock{Host: r.Host, Creator: valid.Creator}, nil, true},
		{"process failed", lookupBlock{Host: r.Host, Creator: valid.Creator, Error: "not_found"}, nil, true},
		{"running without cwd", lookupBlock{Host: r.Host, Creator: valid.Creator, Process: lookupProcess(t, `{}`)}, nil, true},
		{"exited without cwd", lookupBlock{Host: r.Host, Creator: valid.Creator, Process: lookupProcess(t, `{"last_exit":{"exit_code":0}}`)}, nil, false},
	} {
		cases = append(cases, pathCase{tc.name, lookupSession{Blocks: map[string]lookupBlock{"block:a": tc.b}}, tc.want, tc.bad})
	}
	cases = append(cases, pathCase{"partial inspection", lookupSession{Blocks: map[string]lookupBlock{"block:a": valid, "block:b": {Host: r.Host, Creator: valid.Creator, Error: "not_found"}}}, nil, true})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.lookupRepoPaths("session:test", tc.entry)
			if (err != nil) != tc.wantErr || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("paths=%v error=%v, want=%v error=%v", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
