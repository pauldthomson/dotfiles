package main

import (
	"os"
	"strings"
	"testing"
)

// Creates only a disposable failing terminal, without changing app selection.
func TestLiveImmediateStartupExit(t *testing.T) {
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
	var created struct {
		ID      string `json:"session_id"`
		Initial []struct {
			Blocks []string `json:"block_ids"`
		} `json:"initial_windows"`
	}
	// Do not pass --keep-open: exercise Rex's default quick-failure retention.
	if err := r.json(&created, "new", "prex-startup-failure-smoke", "-c", path, "--json", "--", "sh", "-c", "exit 23"); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" {
		t.Fatal("Rex omitted session ID")
	}
	t.Cleanup(func() {
		if err := r.run("kill", created.ID); err != nil {
			t.Errorf("close disposable session: %v", err)
		}
	})
	if len(created.Initial) != 1 || len(created.Initial[0].Blocks) != 1 {
		t.Fatal("Rex omitted terminal ID")
	}
	terminal := launchedTerminal{Session: created.ID, Block: created.Initial[0].Blocks[0]}
	if err := r.checkStartup(terminal); err == nil || !strings.Contains(err.Error(), "exit code 23") {
		t.Fatalf("startup result: %v", err)
	}
	blocks, err := r.blocks(created.ID)
	if err != nil || len(blocks) != 1 || blocks[0].ID != terminal.Block {
		t.Fatal("diagnostic pane was not retained")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("working directory was removed")
	}
	t.Log("Immediate exit reported; diagnostic pane and working directory retained")
}
