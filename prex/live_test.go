package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveDetachedCallerCleanup(t *testing.T) {
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
	dir := t.TempDir()
	binary := filepath.Join(dir, "prex")
	if out, err := exec.Command("go", "build", "-o", binary, ".").CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, out)
	}
	state := filepath.Join(dir, "state")
	var created struct {
		ID string `json:"session_id"`
	}
	err = r.json(&created, "new", "prex-live-cleanup", "-c", dir, "--json", "--", "sh", "-c", `sleep 1; exec env XDG_STATE_HOME="$1" "$2" kill`, "sh", state, binary)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("no test session")
	}
	t.Cleanup(func() {
		sessions, _ := r.sessions()
		for _, item := range sessions {
			if item.ID == created.ID {
				_ = r.run("kill", created.ID)
			}
		}
	})
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		files, _ := filepath.Glob(filepath.Join(state, "prex", "*.result.json"))
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var result cleanupResult
			if err := json.Unmarshal(data, &result); err != nil {
				t.Fatal(err)
			}
			if result.State == "failed" {
				t.Fatalf("detached cleanup failed: %s", result.Error)
			}
			if result.State == "complete" {
				if len(result.Completed) != 1 || result.Completed[0] != created.ID {
					t.Fatalf("wrong cleanup targets: %v", result.Completed)
				}
				sessions, err := r.sessions()
				if err != nil {
					t.Fatal(err)
				}
				for _, item := range sessions {
					if item.ID == created.ID {
						t.Fatal("caller session still exists")
					}
				}
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("detached cleanup did not produce a durable completion result")
}

// Opt in explicitly: this creates disposable sessions, changes the selected
// app session, and launches Pi (without sending it a prompt). Never run in CI.
func TestLiveRexAgentLifecycle(t *testing.T) {
	if os.Getenv("PREX_LIVE") != "1" {
		t.Skip("set PREX_LIVE=1 and PREX_LIVE_SERVER to smoke-test a local Rex app")
	}
	endpoint := os.Getenv("PREX_LIVE_SERVER")
	if endpoint == "" {
		t.Fatal("PREX_LIVE_SERVER must explicitly identify the local Unix socket")
	}
	r, err := connectRex(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.selectClient(os.Getenv("PREX_LIVE_CLIENT")); err != nil {
		t.Fatal(err)
	}
	original := r.Session
	t.Cleanup(func() {
		if original != "" {
			_ = r.show(original)
		}
	})
	s := isolatedState(t)
	path := projectDir(t)
	t.Cleanup(func() {
		live, err := r.sessions()
		if err != nil {
			t.Logf("cannot inspect test sessions for cleanup: %v", err)
			return
		}
		for _, item := range live {
			if _, owned := s.Data.Sessions[item.ID]; owned {
				_ = r.run("kill", item.ID)
			}
		}
	})
	checkDisplay := func(err error) {
		t.Helper()
		if err != nil {
			if strings.Contains(err.Error(), "Remote Control") {
				t.Log("App switching not verified: Remote Control disabled")
			} else {
				t.Fatal(err)
			}
		}
	}
	checkDisplay(launchAndShow(r, s, path, "prex-live-agent", ""))
	var sid string
	for id := range s.Data.Sessions {
		sid = id
	}
	if sid == "" {
		t.Fatal("no created session")
	}
	waitFor := func(check func() bool) {
		t.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if check() {
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
		t.Fatal("timed out waiting for Rex state")
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
		return err == nil && p.LastExit == nil && p.Foreground != nil && (p.Foreground.CWD == path || p.Foreground.CWD == physical)
	})
	// Give Pi's terminal UI a chance to install its key handlers. This delay is
	// test interaction pacing, not a readiness handshake in the launcher.
	time.Sleep(2 * time.Second)
	output, err := outputCmd("rex", r.args("capture", "-s", sid, "-b", bid, "--trim")...)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Pi screen before exit:\n%s", output)
	if err := r.run("send-key", "-s", sid, "-b", bid, "ctrl+d"); err != nil {
		t.Fatal(err)
	}
	// A server-only session can remain empty, but the displayed Rex app may
	// remove it after the last tab exits. Do not mistake failed inspection for
	// absence: confirm absence using the authoritative session listing.
	removed := false
	var emptySince time.Time
	waitFor(func() bool {
		live, err := r.sessions()
		if err != nil {
			return false
		}
		exists := false
		for _, item := range live {
			if item.ID == sid {
				exists = true
			}
		}
		if !exists {
			removed = true
			return true
		}
		view, err := r.view(sid)
		if err != nil || len(view.Windows) != 0 {
			emptySince = time.Time{}
			return false
		}
		if emptySince.IsZero() {
			emptySince = time.Now()
		}
		return time.Since(emptySince) > time.Second
	})
	if removed {
		t.Log("Displayed session was removed after Pi exited; opening the project must create a new session")
		oldID := sid
		checkDisplay(openCommand(r, s, "project"))
		for id := range s.Data.Sessions {
			if id != oldID {
				sid = id
			}
		}
		if sid == oldID {
			t.Fatal("no replacement session created")
		}
	} else {
		t.Log("Session remained empty; reopening should reuse it")
		blocks, err := r.blocks(sid)
		if err != nil || len(blocks) != 0 {
			t.Fatalf("empty session blocks=%v err=%v", blocks, err)
		}
		checkDisplay(openExisting(r, s, session{ID: sid, Label: "prex-live-agent"}, path))
	}
	waitFor(func() bool { blocks, err := r.blocks(sid); return err == nil && len(blocks) == 1 })
	sessions, err := r.sessions()
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, item := range sessions {
		if item.ID == sid {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("reopen did not reuse %s", sid)
	}
	if _, err := os.Stat(filepath.Join(path)); err != nil {
		t.Fatal("repository disappeared on Pi exit")
	}
}
