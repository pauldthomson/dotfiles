package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ktr0731/go-fuzzyfinder"
)

// Claim only the disposable test pane and keep one connection while resizing.
// Otherwise Rex can retain the creating CLI client's terminal size.
const pickerSmokeScript = `
local _, err = rex.call("session.attach", { session_id = rex.args.session_id })
if err then error(err, 0) end
local _, err = rex.call("session.own_block", {
  session_id = rex.args.session_id, block_ids = {rex.args.block_id}, mode = "take",
})
if err then error(err, 0) end
local function block(method, args)
  local result, err = rex.call("block.method", {
    session_id = rex.args.session_id, block_id = rex.args.block_id,
    method = method, args = args,
  })
  if err then error(err, 0) end
  return result
end
local function wait_screen(check)
  local content = ""
  for i = 1, 80 do
    content = block("format", { trim = true }).content
    if check(content) then return content end
    rex.sleep(0.1)
  end
  error("picker did not render expected content:\n" .. content, 0)
end
local function contains(s, text) return string.find(s, text, 1, true) ~= nil end
local frames = {}
for _, width in ipairs({140, 80}) do
  block("resize", { columns = width, rows = 36 })
  for attempt = 1, 40 do
    if block("size").columns == width then break end
    rex.sleep(0.05)
  end
  local size = block("size")
  if size.columns ~= width then error("test pane size " .. size.columns .. ", owner " .. tostring(size.owner), 0) end
  local screen = wait_screen(function(s)
    return contains(s, "Choose switch/reopen") and contains(s, "existing-agent")
  end)
  if width == 140 then frames.Wide = screen else frames.Narrow = screen end
end
block("write", {data = "\t"})
wait_screen(function(s) return not contains(s, "Choose switch/reopen") end)
block("write", {data = "\t"})
wait_screen(function(s) return contains(s, "Choose switch/reopen") end)
block("write", {data = "beta"})
wait_screen(function(s) return contains(s, "Start Pi in this repository") end)
block("write", {data = rex.args.key})
return frames
`

type pickerSmokeResult struct {
	Selection projectSelection
	Cancelled bool
	Error     string
}

// Runs only inside a disposable terminal created by the opt-in test below.
// This exercises the real fzf UI, but never calls the launch/clone workflow.
func TestProjectPickerSubprocess(t *testing.T) {
	resultFile := os.Getenv("PREX_PICKER_SMOKE_RESULT")
	if resultFile == "" {
		t.Skip("picker smoke-test subprocess")
	}
	choices := []projectChoice{
		{Repo: repo{Host: "example.com", Org: "org", Name: "alpha", Path: "/example/alpha"}, Sessions: []session{{ID: "session:fixture", Label: "existing-agent"}}},
		{Repo: repo{Host: "example.com", Org: "org", Name: "beta", Path: "/example/beta"}},
	}
	selected, err := styledProjectPicker(choices, 0, "", "")
	result := pickerSmokeResult{Selection: selected, Cancelled: errors.Is(err, fuzzyfinder.ErrAbort)}
	if err != nil && !result.Cancelled {
		result.Error = err.Error()
	}
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resultFile, data, 0600); err != nil {
		t.Fatal(err)
	}
}

// Creates only disposable, undisplayed sessions. No app switching or agents.
func TestLiveProjectPicker(t *testing.T) {
	if os.Getenv("PREX_LIVE_PICKER") != "1" {
		t.Skip("opt-in live picker test")
	}
	endpoint := os.Getenv("PREX_LIVE_SERVER")
	if endpoint == "" {
		t.Fatal("PREX_LIVE_SERVER required")
	}
	r, err := connectRex(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"enter", "ctrl+n", "escape"} {
		t.Run(key, func(t *testing.T) {
			dir := t.TempDir()
			resultFile := filepath.Join(dir, "result.json")
			var created struct {
				ID      string `json:"session_id"`
				Windows []struct {
					Blocks []string `json:"block_ids"`
				} `json:"initial_windows"`
			}
			if err := r.json(&created, "new", "prex-picker-smoke", "--shell=none", "--keep-open", "-c", dir, "--json", "--", "/usr/bin/env", "PREX_PICKER_SMOKE_RESULT="+resultFile, "TMPDIR="+dir, binary, "-test.run=^TestProjectPickerSubprocess$"); err != nil {
				t.Fatal(err)
			}
			if created.ID == "" {
				t.Fatal("missing test session ID")
			}
			t.Cleanup(func() { _ = r.run("kill", created.ID) })
			if len(created.Windows) != 1 || len(created.Windows[0].Blocks) != 1 {
				t.Fatal("missing test pane ID")
			}
			bid := created.Windows[0].Blocks[0]
			input := map[string]string{"enter": "\r", "ctrl+n": "\x0e", "escape": "\x1b"}[key]
			args := jsonTest(map[string]string{"session_id": created.ID, "block_id": bid, "key": input})
			var frames struct{ Wide, Narrow string }
			if err := r.json(&frames, "do", "-e", pickerSmokeScript, "--args", args, "--json"); err != nil {
				t.Fatal(err)
			}
			if key == "enter" {
				t.Logf("Wide picker:\n%s\nNarrow picker:\n%s", frames.Wide, frames.Narrow)
			}
			deadline := time.Now().Add(8 * time.Second)
			for time.Now().Before(deadline) {
				data, err := os.ReadFile(resultFile)
				if err != nil {
					time.Sleep(100 * time.Millisecond)
					continue
				}
				var result pickerSmokeResult
				if err := json.Unmarshal(data, &result); err != nil {
					t.Fatal(err)
				}
				if result.Error != "" || result.Cancelled != (key == "escape") || (key != "escape" && (result.Selection.Index != 1 || result.Selection.Clone != (key == "ctrl+n"))) {
					t.Fatalf("key=%s result=%+v", key, result)
				}
				return
			}
			t.Fatal("picker did not return a result")
		})
	}
}
