package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"syscall"
)

// Registry writes are atomic renames, so a read-only report can use a snapshot
// without creating a directory, lock file, or registry. It is not deletion proof.
func readStateSnapshot() (*stateStore, error) {
	dir := stateDir()
	s := &stateStore{Dir: dir, Data: registry{Version: 1, Sessions: map[string]association{}, Clones: map[string]cloneRecord{}}}
	info, err := os.Lstat(dir)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != uint32(os.Getuid()) || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("state directory must be a private, owned directory: %s", dir)
	}
	data, err := os.ReadFile(filepath.Join(dir, "registry.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &s.Data)
	}
	if err == nil && (s.Data.Version != 1 || s.Data.Sessions == nil || s.Data.Clones == nil) {
		err = fmt.Errorf("unsupported or incomplete registry")
	}
	if err != nil {
		return nil, fmt.Errorf("registry unreadable; no report produced: %w", err)
	}
	return s, nil
}

type orphanRow struct {
	Path, Status, Reason string
}

func orphanReport(r *rexClient, s *stateStore) ([]orphanRow, error) {
	if len(s.Data.Clones) == 0 {
		return nil, nil
	}
	// Inspect all sessions, even currently recorded ones: another pane may use
	// a different clone. Do not refresh associations in this read-only command.
	sessions, lookup, err := r.lookupSessions(&stateStore{})
	if err != nil {
		return nil, err
	}
	var paths []string
	uncertain := false
	live := make(map[string]bool)
	for _, session := range sessions {
		live[session.ID] = true
		observed, err := r.lookupRepoPaths(session.ID, lookup[session.ID])
		if err != nil {
			uncertain = true
			continue
		}
		paths = append(paths, observed...)
	}
	var rows []orphanRow
	for path, clone := range s.Data.Clones {
		row := orphanRow{Path: path, Status: "unknown"}
		switch {
		case clone.Host != r.Host:
			row.Status, row.Reason = "other-host", "ownership belongs to another host; not inspected"
		case validateClone(path, clone, r.Host) != nil:
			row.Reason = "directory/ownership identity could not be verified (missing, replaced, or inaccessible)"
		default:
			inUse, staleReference := false, false
			for _, cwd := range paths {
				inUse = inUse || physicalWithin(cwd, path)
			}
			for sid, association := range s.Data.Sessions {
				if !live[sid] || association.Host != r.Host || !physicalWithin(association.Path, path) {
					continue
				}
				if association.Instance == r.Instance {
					inUse = true
				} else {
					staleReference = true
				}
			}
			pending := s.Data.Pending != nil && s.Data.Pending.Host == r.Host && physicalWithin(s.Data.Pending.Path, path)
			switch {
			case inUse:
				row.Status, row.Reason = "in-use", "a live session uses or is currently associated with this clone"
			case pending:
				row.Reason = "an unresolved launch references this clone; run prex recover"
			case staleReference:
				row.Reason = "a surviving session has an unverified pre-restart association"
			case uncertain:
				row.Reason = "one or more sessions could not be fully inspected"
			default:
				row.Status, row.Reason = "orphan", "no session use found on the selected Rex server"
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Path < rows[j].Path })
	return rows, nil
}

func orphansCommand(r *rexClient, s *stateStore, args []string, out io.Writer) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: prex orphans (read-only; no deletion options)")
	}
	rows, err := orphanReport(r, s)
	if err != nil {
		return err
	}
	fmt.Fprintln(out, "Read-only snapshot of recorded clones; selected Rex server only.")
	fmt.Fprintln(out, "Not a deletion-safety check: other servers, external processes, and unpushed work are not checked.")
	if len(rows) == 0 {
		fmt.Fprintln(out, "No recorded clones.")
	}
	for _, row := range rows {
		fmt.Fprintf(out, "%s\t%q\t%s\n", row.Status, row.Path, row.Reason)
	}
	return nil
}
