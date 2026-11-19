package main

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

type paletteItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Icon     string `json:"icon,omitempty"`
}

func (r *rexClient) palettePrompt(spec map[string]any) (string, error) {
	args, err := json.Marshal(map[string]any{"app": r.Client, "spec": spec})
	if err != nil {
		return "", err
	}
	cmd := exec.Command("rex", r.args("do", "prex_prompt", "--args", "-", "--json")...)
	cmd.Stdin = strings.NewReader(string(args))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Rex palette: %w: %s", err, strings.TrimSpace(string(out)))
	}
	var reply struct {
		Value  *string
		Reason string
	}
	if err = json.Unmarshal(out, &reply); err != nil {
		return "", err
	}
	if reply.Reason == "cancelled" {
		return "", fuzzyfinder.ErrAbort
	}
	if reply.Value == nil || reply.Reason != "" {
		return "", fmt.Errorf("Rex palette returned no answer (%s); use prex picker", reply.Reason)
	}
	return *reply.Value, nil
}
func (r *rexClient) palettePick(title, query string, items []paletteItem) (int, error) {
	if len(items) > 2000 {
		return 0, fmt.Errorf("Rex palette supports up to 2000 choices; use prex picker")
	}
	for i := range items {
		items[i].ID = strconv.Itoa(i)
	}
	result, err := r.palettePrompt(map[string]any{"title": title, "value": query, "items": items})
	if err != nil {
		return 0, err
	}
	i, err := strconv.Atoi(result)
	if err != nil || i < 0 || i >= len(items) {
		return 0, fmt.Errorf("invalid palette selection")
	}
	return i, nil
}
func (r *rexClient) paletteProject(choices []projectChoice, warnings int, query, notice string) (projectSelection, error) {
	title := "Open Project"
	if notice != "" {
		title += " — " + notice
	}
	if warnings > 0 {
		title += fmt.Sprintf(" (%d inspection warnings)", warnings)
	}
	items := make([]paletteItem, len(choices))
	for i, c := range choices {
		items[i] = paletteItem{Title: pickerText(c.Repo.Name), Subtitle: pickerText(c.Repo.Host + "/" + c.Repo.Org), Icon: "folder"}
		if n := len(c.Sessions); n > 0 {
			items[i].Detail = fmt.Sprintf("%d session(s)", n)
		}
	}
	i, err := r.palettePick(title, query, items)
	return projectSelection{Index: i}, err
}
func (r *rexClient) paletteSuffix(value string) (string, error) {
	return r.palettePrompt(map[string]any{"title": "Suffix for cloned repository", "value": value, "confirm": "Clone"})
}
