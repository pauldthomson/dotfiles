package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPickerSurfacesStartupFailureInOrigin(t *testing.T) {
	s := isolatedState(t)
	projectDir(t)
	s.Close() // run acquires its own state lock.
	r, f := fakeRex(t)
	base := f.output
	var selections []string
	f.output = func(a []string) (string, error) {
		args := rexTail(a)
		if args[0] == "client" {
			return `{"client":{"client_id":"client:app","info":{"kind":"app"},"connection_state":"connected"},"actions":[{"name":"session.select"}]}`, nil
		}
		if args[0] == "new" {
			return `{"session_id":"session:new","initial_windows":[{"block_ids":["block:new"]}]}`, nil
		}
		if args[0] == "-C" {
			selections = append(selections, strings.Join(args, " "))
		}
		if len(args) > 2 && args[0] == "do" {
			if args[2] == lookupScript {
				return lookupResult(r, nil, nil), nil
			}
			if args[2] == startupScript {
				return jsonTest(map[string]any{"host_id": r.Host, "instance_id": r.Instance, "state": "exited", "exit_code": 1}), nil
			}
		}
		return base(a)
	}
	safetyInput(t, "\n")
	if err := run([]string{"--_picker", "--client", "client:app", "project"}); err != nil {
		t.Fatal(err)
	}
	if len(selections) != 3 || !strings.Contains(selections[1], "session_id=session:new") || !strings.Contains(selections[2], "session_id=session:caller window_id=window:caller") {
		t.Fatalf("error was not surfaced in the origin picker: %v", selections)
	}
}

func TestStartupOutcomeKeepsKnownCreationAndClone(t *testing.T) {
	for _, existing := range []bool{false, true} {
		for _, outcome := range []string{"observed", "exited", "unknown", "restarted", "malformed", "disconnected", "missing-block"} {
			t.Run(outcome+map[bool]string{false: "/new", true: "/reopen"}[existing], func(t *testing.T) {
				s := isolatedState(t)
				path := projectDir(t)
				r, f := fakeRex(t)
				if err := s.ownClone(path, r.Host); err != nil {
					t.Fatal(err)
				}
				clone := s.Data.Clones[path]
				id, sid := "", "session:new"
				if existing {
					id, sid = "session:old", "session:old"
				}
				base := f.output
				created, displayed, inspected := 0, false, false
				f.output = func(a []string) (string, error) {
					args := rexTail(a)
					if args[0] == "new" || args[0] == "window" {
						created++
						if outcome == "missing-block" {
							return `{"session_id":"session:new"}`, nil
						}
						if existing {
							return `{"window_id":"window:new","block_ids":["block:new"]}`, nil
						}
						return `{"session_id":"session:new","initial_windows":[{"block_ids":["block:new"]}]}`, nil
					}
					if args[0] == "-C" {
						displayed = true
					}
					if len(args) > 2 && args[0] == "do" && args[2] == startupScript {
						inspected = true
						var target map[string]string
						if err := json.Unmarshal([]byte(args[4]), &target); err != nil {
							t.Fatal(err)
						}
						if target["session_id"] != sid || target["block_id"] != "block:new" || !displayed {
							t.Fatal("inspected the wrong pane or delayed displaying it")
						}
						data, err := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
						if err != nil {
							t.Fatal(err)
						}
						var saved registry
						if err := json.Unmarshal(data, &saved); err != nil || saved.Pending != nil || saved.Sessions[sid].Path != path {
							t.Fatal("creation was not durably recorded before inspection")
						}
						if outcome == "malformed" {
							return `{}`, nil
						}
						if outcome == "disconnected" {
							return "", errors.New("disconnected")
						}
						instance := r.Instance
						if outcome == "restarted" {
							instance = "new-instance"
						}
						return jsonTest(map[string]any{"host_id": r.Host, "instance_id": instance, "state": outcome, "exit_code": 127}), nil
					}
					return base(a)
				}
				err := launchAndShow(r, s, path, "project", id)
				if (err == nil) != (outcome == "observed") {
					t.Fatalf("outcome=%s error=%v", outcome, err)
				}
				if outcome == "exited" && !strings.Contains(err.Error(), "exit code 127") {
					t.Fatal("exit status was not reported")
				}
				if created != 1 || !displayed || inspected == (outcome == "missing-block") {
					t.Fatal("wrong launch/inspection behavior")
				}
				if s.Data.Pending != nil || s.pathFor(r, sid) != path || s.Data.Clones[path] != clone {
					t.Fatal("startup result changed ownership or lost the known session")
				}
				if err := validateClone(path, clone, r.Host); err != nil {
					t.Fatal("clone was changed or removed")
				}
				for _, a := range f.calls {
					if rexTail(a)[0] == "kill" || rexTail(a)[0] == "send-key" {
						t.Fatal("startup observation closed or interrupted the program")
					}
				}
			})
		}
	}
}
