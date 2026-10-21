package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/ktr0731/go-fuzzyfinder"
)

// Called before dispatching either picker command, including kill. Selecting
// the app window here works for both keyboard actions and CLI-created tabs.
func activatePicker(r *rexClient, client string) error {
	if r.Session == "" || r.Window == "" {
		return fmt.Errorf("picker cannot determine its own Rex tab")
	}
	if err := r.selectClient(client); err != nil {
		return err
	}
	return r.showWindow(r.Session, r.Window)
}

// Keep plain confirmation/error prompts consistent with the styled picker.
func paintPicker() {
	fmt.Fprint(os.Stdout, "\x1b[48;2;30;30;46m\x1b[38;2;205;214;244m\x1b[2J\x1b[H")
}

// Temporary picker tabs use the styled fzf UI. Ordinary terminal invocation
// retains the existing Go picker.
func chooseOne[T any](r *rexClient, items []T, label func(int) string, prompt, title string) (int, error) {
	if !r.Picker {
		return fuzzyfinder.Find(items, label, fuzzyfinder.WithPromptString(prompt), fuzzyfinder.WithHeader(title))
	}
	indices, err := styledPicker(len(items), label, false, prompt, title)
	if err != nil {
		return 0, err
	}
	return indices[0], nil
}
func pickerArgs(multi bool, prompt, title string) []string {
	hint := "Type to filter · ↑↓ move · Enter select · Esc cancel"
	args := []string{"--read0", "--print0", "--delimiter=\t", "--with-nth=2..", "--layout=reverse", "--border=rounded", "--border-label= " + title + " ", "--padding=1,2", "--margin=0", "--info=inline-right", "--prompt=" + prompt,
		"--color=bg:#1e1e2e,bg+:#313244,fg:#cdd6f4,fg+:#cdd6f4,border:#89b4fa,label:#89b4fa,header:#a6adc8,hl:#a6e3a1,hl+:#a6e3a1,prompt:#89b4fa,pointer:#f5c2e7,marker:#a6e3a1,info:#a6adc8"}
	if multi {
		args = append(args, "--multi")
		hint = "Type to filter · Tab select · Enter confirm · Esc cancel"
	}
	return append(args, "--header="+hint)
}
func pickerRows(count int, label func(int) string) string {
	var rows strings.Builder
	for i := 0; i < count; i++ {
		// IDs are separate from presentation. Names containing tabs/newlines or
		// escape characters cannot inject rows, terminal sequences, or selections.
		text := strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return ' '
			}
			return r
		}, label(i))
		fmt.Fprintf(&rows, "%d\t%s\x00", i, text)
	}
	return rows.String()
}
func pickerSelection(output []byte, count int, multi bool) ([]int, error) {
	var selected []int
	seen := map[int]bool{}
	for _, row := range bytes.Split(output, []byte{0}) {
		if len(row) == 0 {
			continue
		}
		index, _, ok := strings.Cut(string(row), "\t")
		n, err := strconv.Atoi(index)
		if !ok || err != nil || n < 0 || n >= count {
			return nil, fmt.Errorf("picker returned an invalid selection")
		}
		if !seen[n] {
			selected = append(selected, n)
			seen[n] = true
		}
	}
	if len(selected) == 0 {
		return nil, fuzzyfinder.ErrAbort
	}
	if !multi && len(selected) != 1 {
		return nil, fmt.Errorf("picker returned multiple selections")
	}
	return selected, nil
}
func styledPicker(count int, label func(int) string, multi bool, prompt, title string) ([]int, error) {
	cmd := exec.Command("fzf", pickerArgs(multi, prompt, title)...)
	cmd.Stdin = strings.NewReader(pickerRows(count, label))
	cmd.Stderr = os.Stderr
	// Personal fzf bindings/options can run commands or change output format.
	// Keep this embedded picker deterministic, including its opaque background.
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "FZF_DEFAULT_OPTS" && key != "FZF_DEFAULT_OPTS_FILE" && key != "FZF_DEFAULT_COMMAND" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	out, err := cmd.Output()
	paintPicker()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && (exit.ExitCode() == 1 || exit.ExitCode() == 130) {
			return nil, fuzzyfinder.ErrAbort
		}
		return nil, fmt.Errorf("picker tab requires fzf (brew install fzf): %w", err)
	}
	return pickerSelection(out, count, multi)
}
