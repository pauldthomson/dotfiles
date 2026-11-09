package main

import (
	"os"
	"reflect"
	"testing"
)

// Read-only: no session creation, app selection, registry writes or cleanup.
// Run while terminal working directories are stable for a meaningful comparison.
func TestLiveLookupCollection(t *testing.T) {
	if os.Getenv("PREX_LIVE_LOOKUP") != "1" {
		t.Skip("opt-in read-only live lookup test")
	}
	endpoint := os.Getenv("PREX_LIVE_SERVER")
	if endpoint == "" {
		t.Fatal("PREX_LIVE_SERVER required")
	}
	r, err := connectRex(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	sessions, collected, err := r.lookupSessions(&stateStore{})
	if err != nil {
		t.Fatal(err)
	}
	listed, err := r.sessions()
	if err != nil || !reflect.DeepEqual(listed, sessions) {
		t.Fatal("collected session listing differs from Rex listing")
	}
	known := &stateStore{Data: registry{Sessions: map[string]association{}}}
	for _, s := range sessions {
		before, beforeErr := r.repoPaths(s.ID)
		after, afterErr := r.lookupRepoPaths(s.ID, collected[s.ID])
		if (beforeErr != nil) != (afterErr != nil) || !reflect.DeepEqual(before, after) {
			// Do not dump directories or process data into test logs.
			t.Errorf("lookup differs for %s; a process may have changed during inspection", s.ID)
		}
		if beforeErr == nil && len(before) > 0 {
			known.associate(r, s.ID, before[0])
		}
	}
	cachedList, cached, err := r.lookupSessions(known)
	if err != nil || !reflect.DeepEqual(sessions, cachedList) {
		t.Fatal("recorded associations changed the session listing")
	}
	for id := range known.Data.Sessions {
		if cached[id].Blocks != nil || cached[id].Error != "" {
			t.Errorf("collector inspected recorded session %s", id)
		}
	}
	t.Logf("Compared sequential and collected paths for %d live sessions", len(sessions))
}
