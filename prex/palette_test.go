package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ktr0731/go-fuzzyfinder"
)

func TestPaletteChoicesFailClosed(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rex"), []byte("#!/bin/sh\nprintf '%s' \"$REPLY_JSON\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	r := &rexClient{Palette: true}
	rows := []paletteItem{{Title: "first"}, {Title: "second"}}
	for _, tc := range []struct {
		reply  string
		index  int
		cancel bool
	}{
		{`{"value":"1"}`, 1, false}, {`{"reason":"cancelled"}`, -1, true}, {`{"reason":"unsupported"}`, -1, false}, {`{"value":"2"}`, -1, false}, {`{}`, -1, false},
	} {
		t.Run(tc.reply, func(t *testing.T) {
			t.Setenv("REPLY_JSON", tc.reply)
			got, err := r.palettePick("test", "", rows)
			if (err == nil) != (tc.index >= 0) || errors.Is(err, fuzzyfinder.ErrAbort) != tc.cancel || (err == nil && got != tc.index) {
				t.Fatalf("got %d, %v", got, err)
			}
		})
	}
}
