package main

import (
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/ktr0731/go-fuzzyfinder"
)

func TestPickerRowsKeepIdentitySeparateFromLabels(t *testing.T) {
	labels := []string{"same", "same", "line\nwith\tcontrol\x1b"}
	rows := pickerRows(len(labels), func(i int) string { return labels[i] })
	if rows != "0\tsame\x001\tsame\x002\tline with control \x00" {
		t.Fatalf("unexpected encoded rows %q", rows)
	}
	got, err := pickerSelection([]byte("1\tsame\x000\tsame\x001\tsame\x00"), 3, true)
	if err != nil || !reflect.DeepEqual(got, []int{1, 0}) {
		t.Fatalf("selection=%v err=%v", got, err)
	}
}
func TestPickerRejectsInvalidSelection(t *testing.T) {
	for _, out := range []string{"-1\tx\x00", "2\tx\x00", "oops\tx\x00", "1", "0\tx\x001\tx\x00"} {
		if _, err := pickerSelection([]byte(out), 2, false); err == nil {
			t.Fatalf("accepted %q", out)
		}
	}
	if _, err := pickerSelection(nil, 2, false); !errors.Is(err, fuzzyfinder.ErrAbort) {
		t.Fatalf("empty output error %v", err)
	}
}
func TestPickerHasOpaqueColorsBorderAndHints(t *testing.T) {
	args := strings.Join(pickerArgs(true, "close> ", "Close sessions"), " ")
	for _, want := range []string{"--color=bg:#1e1e2e,bg+:#313244", "--border=rounded", "--border-label= Close sessions ", "--margin=0", "--multi", "Tab select", "Esc cancel", "--read0", "--print0"} {
		if !strings.Contains(args, want) {
			t.Errorf("missing %q in %s", want, args)
		}
	}
}
func TestFZFReturnsOriginalIndexWithHiddenID(t *testing.T) {
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed")
	}
	args := append(pickerArgs(false, "project> ", "Projects"), "--filter=beta")
	cmd := exec.Command("fzf", args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE="}
	cmd.Stdin = strings.NewReader(pickerRows(2, func(i int) string { return []string{"alpha", "beta"}[i] }))
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	got, err := pickerSelection(out, 2, false)
	if err != nil || !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("selection=%v err=%v raw=%q", got, err, out)
	}
}
