package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

//go:embed startup.lua
var startupScript string

type launchedTerminal struct {
	Session string
	Block   string
}

func (r *rexClient) checkStartup(terminal launchedTerminal) error {
	unverified := func(reason string) error {
		return fmt.Errorf("session %s is recorded, but Pi startup could not be verified (%s); inspect it before launching again; repository retained", terminal.Session, reason)
	}
	if !strings.HasPrefix(terminal.Block, "block:") {
		return unverified("Rex omitted the new terminal ID")
	}
	args, err := json.Marshal(map[string]string{"session_id": terminal.Session, "block_id": terminal.Block})
	if err != nil {
		return err
	}
	var result struct {
		Host     string `json:"host_id"`
		Instance string `json:"instance_id"`
		State    string `json:"state"`
		ExitCode *int   `json:"exit_code"`
	}
	if err := r.json(&result, "do", "-e", startupScript, "--args", string(args), "--json"); err != nil {
		// Do not dump the script, command arguments or terminal output.
		return unverified("Rex inspection failed")
	}
	if result.Host != r.Host || result.Instance != r.Instance {
		return unverified("Rex server identity changed")
	}
	switch result.State {
	case "observed":
		return nil
	case "exited":
		if result.ExitCode != nil {
			return fmt.Errorf("Pi exited during startup (exit code %d) in session %s, pane %s; inspect the pane for output; no session or repository was removed by prex", *result.ExitCode, terminal.Session, terminal.Block)
		}
	}
	return unverified("terminal disappeared or process state is unavailable")
}
