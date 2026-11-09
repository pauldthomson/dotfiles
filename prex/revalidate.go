package main

import "fmt"

// Rebind only surviving, previously recorded session IDs on the same host.
// Every observed process directory must still belong to the recorded project.
// This never creates clone ownership, adopts replacement session IDs, or
// clears a pending launch. Empty and ambiguous sessions remain unverified.
func (s *stateStore) revalidateAssociations(r *rexClient, sessions []session, lookup map[string]lookupSession) (int, error) {
	if s.Data.Pending != nil {
		return 0, nil
	}
	updates := make(map[string]association)
	for _, session := range sessions {
		old, ok := s.Data.Sessions[session.ID]
		if !ok || old.Host != r.Host || old.Instance == "" || old.Instance == r.Instance {
			continue
		}
		if _, _, err := directoryIdentity(old.Path); err != nil {
			continue
		}
		if clone, owned := s.Data.Clones[old.Path]; owned {
			if err := validateClone(old.Path, clone, r.Host); err != nil {
				continue
			}
		}
		entry := lookup[session.ID]
		paths, err := r.lookupRepoPaths(session.ID, entry)
		if err != nil || len(paths) == 0 {
			continue
		}
		liveProcess := false
		for _, b := range entry.Blocks {
			if b.Process != nil && b.Process.LastExit == nil && (b.Process.Child != nil || b.Process.Foreground != nil) {
				liveProcess = true
			}
		}
		if !liveProcess {
			continue
		}
		verified := true
		for _, cwd := range paths {
			if !physicalWithin(cwd, old.Path) {
				verified = false
				break
			}
		}
		if verified {
			old.Instance = r.Instance
			updates[session.ID] = old
		}
	}
	if len(updates) == 0 {
		return 0, nil
	}
	if err := r.verify(); err != nil {
		return 0, err
	}
	original := s.Data.Sessions
	s.Data.Sessions = make(map[string]association, len(original))
	for id, a := range original {
		s.Data.Sessions[id] = a
	}
	for id, a := range updates {
		s.Data.Sessions[id] = a
	}
	if err := s.save(); err != nil {
		s.Data.Sessions = original
		return 0, fmt.Errorf("could not save revalidated associations: %w", err)
	}
	return len(updates), nil
}
