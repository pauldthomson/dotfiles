package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Exercises the actual launch path without selecting an app session or sending
// Pi a prompt. Only this test's disposable session is closed.
func TestLiveInteractiveAgentLaunch(t *testing.T) {
	if os.Getenv("PREX_LIVE") != "1" {
		t.Skip("opt-in live Rex test")
	}
	endpoint := os.Getenv("PREX_LIVE_SERVER")
	if endpoint == "" {
		t.Fatal("PREX_LIVE_SERVER required")
	}
	r, err := connectRex(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	path := t.TempDir()
	terminal, err := r.launch(path, "prex-interactive-smoke", "")
	if err != nil {
		t.Fatal(err)
	}
	sid := terminal.Session
	t.Cleanup(func() {
		if err := r.run("kill", sid); err != nil {
			t.Errorf("close disposable session %s: %v", sid, err)
		}
	})
	waitFor := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(30 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Fatal("timed out waiting for disposable Pi session")
	}
	for _, phase := range []string{"new", "reopen"} {
		if phase == "reopen" {
			reopened, err := r.launch(path, "prex-interactive-smoke", sid)
			if err != nil || reopened.Session != sid {
				t.Fatalf("reopen: session=%s error=%v", reopened.Session, err)
			}
			terminal = reopened
		}
		if err := r.checkStartup(terminal); err != nil {
			t.Fatal(err)
		}
		var bid string
		waitFor(func() bool {
			blocks, err := r.blocks(sid)
			if err != nil || len(blocks) != 1 {
				return false
			}
			bid = blocks[0].ID
			p, err := r.process(sid, bid)
			physical, _ := filepath.EvalSymlinks(path)
			if err != nil || p.LastExit != nil || p.Foreground == nil || p.Foreground.CWD != physical {
				return false
			}
			// Raw input confirms the terminal UI has initialized without
			// depending on Pi's logo or printing potentially private output.
			var mode struct {
				Canonical *bool `json:"canonical"`
				Echo      *bool `json:"echo"`
				Signals   *bool `json:"signals"`
			}
			err = r.json(&mode, "block", "call", "-s", sid, "-b", bid, "input_mode")
			return err == nil && mode.Canonical != nil && !*mode.Canonical && mode.Echo != nil && !*mode.Echo && mode.Signals != nil && !*mode.Signals
		})
		t.Logf("%s: Pi UI started", phase)
		if err := r.run("send-key", "-s", sid, "-b", bid, "ctrl+d"); err != nil {
			t.Fatal(err)
		}
		waitFor(func() bool {
			view, err := r.view(sid)
			return err == nil && len(view.Windows) == 0
		})
	}
}
