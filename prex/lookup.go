package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed lookup.lua
var lookupScript string

type lookupBlock struct {
	Host    string       `json:"host_id"`
	Creator string       `json:"creator_name"`
	Process *processInfo `json:"process"`
	Error   string       `json:"error"`
}

type lookupSession struct {
	Label    string                 `json:"label"`
	Position int                    `json:"position"`
	Blocks   map[string]lookupBlock `json:"blocks"`
	Error    string                 `json:"error"`
}

// List sessions and collect directories for unmanaged ones in one CLI call.
// This is for project discovery, never cleanup or recovery, which retain their
// independent fresh inspections. Only Go decides which associations are valid.
func (r *rexClient) lookupSessions(s *stateStore) ([]session, map[string]lookupSession, error) {
	known := make(map[string]bool)
	for id := range s.Data.Sessions {
		if s.pathFor(r, id) != "" {
			known[id] = true
		}
	}
	args, err := json.Marshal(struct {
		Known map[string]bool `json:"known_sessions"`
	}{known})
	if err != nil {
		return nil, nil, err
	}
	var result struct {
		Host     string                   `json:"host_id"`
		Instance string                   `json:"instance_id"`
		Count    *int                     `json:"session_count"`
		Sessions map[string]lookupSession `json:"sessions"`
	}
	if err := r.json(&result, "do", "-e", lookupScript, "--args", string(args), "--json"); err != nil {
		return nil, nil, fmt.Errorf("collect Rex session directories: %w", err)
	}
	if result.Host != r.Host || result.Instance != r.Instance {
		return nil, nil, fmt.Errorf("Rex server identity changed during session lookup")
	}
	if result.Count == nil || result.Sessions == nil || *result.Count != len(result.Sessions) {
		return nil, nil, fmt.Errorf("Rex lookup returned an incomplete session set")
	}
	// Explicit positions preserve server ordering even though Lua serializes
	// empty tables as objects, so the wire format uses a map rather than a list.
	sessions := make([]session, len(result.Sessions))
	for id, entry := range result.Sessions {
		if !strings.HasPrefix(id, "session:") || entry.Position < 1 || entry.Position > len(sessions) {
			return nil, nil, fmt.Errorf("Rex lookup returned an invalid session ID or position")
		}
		if sessions[entry.Position-1].ID != "" {
			return nil, nil, fmt.Errorf("Rex lookup returned duplicate session positions")
		}
		sessions[entry.Position-1] = session{ID: id, Label: entry.Label}
	}
	return sessions, result.Sessions, nil
}

func (r *rexClient) lookupRepoPaths(sid string, entry lookupSession) ([]string, error) {
	if entry.Error != "" {
		return nil, fmt.Errorf("%s", entry.Error)
	}
	if entry.Blocks == nil {
		return nil, fmt.Errorf("Rex lookup omitted the blocks for session %s", sid)
	}
	blocks := make([]block, 0, len(entry.Blocks))
	for id, b := range entry.Blocks {
		if !strings.HasPrefix(id, "block:") {
			return nil, fmt.Errorf("Rex block list has an invalid ID")
		}
		blocks = append(blocks, block{ID: id, Host: b.Host, Creator: b.Creator})
	}
	// Match session.list_blocks order, regardless of JSON object ordering.
	sort.Slice(blocks, func(i, j int) bool { return blocks[i].ID < blocks[j].ID })
	return r.repoPathsWithProcess(sid, blocks, func(_, bid string) (processInfo, error) {
		b := entry.Blocks[bid]
		if b.Error != "" {
			return processInfo{}, fmt.Errorf("%s", b.Error)
		}
		if b.Process == nil {
			return processInfo{}, fmt.Errorf("Rex lookup omitted the process for block %s", bid)
		}
		return *b.Process, nil
	})
}
