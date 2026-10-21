package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestKillRefusesTargetsAndUnknownCurrentSession(t *testing.T) {
	for _, tc := range []struct {
		name, current string
		args          []string
	}{
		{"outside Rex", "", nil},
		{"missing current session", "session:gone", nil},
		{"explicit other target", "session:caller", []string{"session:other"}},
		{"explicit current target", "session:caller", []string{"session:caller"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := isolatedState(t)
			r, f := fakeRex(t)
			r.Session = tc.current
			f.output = func(a []string) (string, error) {
				if strings.Join(rexTail(a), " ") != "ls --json" {
					t.Fatalf("unexpected inspection: %v", a)
				}
				return `{"sessions":[{"session_id":"session:other","label":"session:gone"}]}`, nil
			}
			old := startDetachedProcess
			t.Cleanup(func() { startDetachedProcess = old })
			startDetachedProcess = func(string, ...string) error { t.Fatal("started cleanup for rejected request"); return nil }
			if err := killCommand(r, s, tc.args); err == nil {
				t.Fatal("accepted unsafe or explicit target")
			}
			jobs, err := filepath.Glob(filepath.Join(s.Dir, "*.job.json"))
			if err != nil {
				t.Fatal(err)
			}
			if len(jobs) != 0 {
				t.Fatalf("queued cleanup: %v", jobs)
			}
			for _, call := range f.calls {
				if rexTail(call)[0] == "kill" {
					t.Fatal("closed a session")
				}
			}
		})
	}
}

func TestCloseHelperDoesNotRaceSessionClosureWithAppSelection(t *testing.T) {
	r, f := fakeRex(t)
	f.output = func(a []string) (string, error) { return `{"window_id":"window:close-helper"}`, nil }
	if err := pickerTab(r, []string{"kill"}); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 {
		t.Fatalf("parent must only create the helper tab: %v", f.calls)
	}
	args := strings.Join(rexTail(f.calls[0]), " ")
	if !strings.Contains(args, "window new -s session:caller Close current session") || !strings.HasSuffix(args, "--_return-window window:caller kill") {
		t.Fatalf("incorrect close helper command: %s", args)
	}
}
