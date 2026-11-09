package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

type projectChoice struct {
	Repo     repo
	Sessions []session
}

type projectSelection struct {
	Index int
	Clone bool
}

var chooseProject = styledProjectPicker
var chooseProjectSession = func(r *rexClient, sessions []session) (int, error) {
	return chooseOne(r, sessions, func(i int) string { return sessionDisplay(sessions[i]) }, "session> ", "Project sessions")
}

var chooseProjectAction = func(r *rexClient, target session) (int, error) {
	choices := []string{"Switch to / reopen " + sessionDisplay(target), "Clone a new copy", "Cancel"}
	return chooseOne(r, choices, func(i int) string { return choices[i] }, "action> ", "Session already exists")
}

// Validate each unmanaged session once, not once per project or preview redraw.
func projectChoices(r *rexClient, s *stateStore, repos []repo, sessions []session, lookup map[string]lookupSession) ([]projectChoice, int) {
	choices := make([]projectChoice, len(repos))
	physical := make([]string, len(repos))
	for i, project := range repos {
		choices[i].Repo = project
		physical[i], _ = filepath.EvalSymlinks(project.Path)
	}
	warnings := 0
	for _, item := range sessions {
		recorded := s.pathFor(r, item.ID)
		var paths []string
		if recorded == "" {
			var err error
			paths, err = r.lookupRepoPaths(item.ID, lookup[item.ID])
			if err != nil {
				warnings++
				continue
			}
		}
		for i, project := range repos {
			matched := recorded != "" && samePath(recorded, project.Path)
			if recorded == "" && physical[i] != "" {
				for _, cwd := range paths {
					if pathWithin(cwd, physical[i]) {
						matched = true
						break
					}
				}
			}
			if matched {
				choices[i].Sessions = append(choices[i].Sessions, item)
			}
		}
	}
	return choices, warnings
}

func sameSessionIDs(a, b []session) bool {
	if len(a) != len(b) {
		return false
	}
	ids := make(map[string]int, len(a))
	for _, item := range a {
		ids[item.ID]++
	}
	for _, item := range b {
		ids[item.ID]--
		if ids[item.ID] < 0 {
			return false
		}
	}
	return true
}

func openProjectPicker(r *rexClient, s *stateStore, repos []repo) error {
	sessions, lookup, err := r.lookupSessions(s)
	if err != nil {
		return err
	}
	query, notice := "", ""
	for {
		choices, warnings := projectChoices(r, s, repos, sessions, lookup)
		selected, err := chooseProject(choices, warnings, query, notice)
		if err != nil {
			return err
		}
		if selected.Index < 0 || selected.Index >= len(choices) {
			return fmt.Errorf("picker returned an invalid project")
		}
		choice := choices[selected.Index]
		// The user may browse for a while. Refresh before acting, and require
		// another Enter if its existing-session/start meaning has changed.
		sessions, lookup, err = r.lookupSessions(s)
		if err != nil {
			return err
		}
		fresh, freshWarnings := projectChoices(r, s, []repo{choice.Repo}, sessions, lookup)
		if !selected.Clone && (!sameSessionIDs(choice.Sessions, fresh[0].Sessions) || warnings != freshWarnings) {
			query = repoDisplayName(choice.Repo)
			notice = "Sessions changed; review the preview and press Enter again."
			continue
		}
		if _, err := s.revalidateAssociations(r, sessions, lookup); err != nil {
			return err
		}
		if selected.Clone {
			return cloneAndLaunch(r, s, choice.Repo, sessions)
		}
		matches := fresh[0].Sessions
		if len(matches) == 0 {
			return launchAndShow(r, s, choice.Repo.Path, uniqueLabel(repoSessionLabel(choice.Repo), sessions), "")
		}
		target := matches[0]
		if len(matches) > 1 {
			i, err := chooseProjectSession(r, matches)
			if err != nil {
				return err
			}
			if i < 0 || i >= len(matches) {
				return fmt.Errorf("picker returned an invalid session")
			}
			target = matches[i]
		}
		action, err := chooseProjectAction(r, target)
		if err != nil {
			return err
		}
		switch action {
		case 0:
			return openExisting(r, s, target, choice.Repo.Path)
		case 1:
			return cloneAndLaunch(r, s, choice.Repo, sessions)
		case 2:
			return nil
		default:
			return fmt.Errorf("picker returned an invalid action")
		}
	}
}

func cloneAndLaunch(r *rexClient, s *stateStore, source repo, sessions []session) error {
	cloned, err := cloneRepo(source)
	if err != nil {
		return err
	}
	if err := s.ownClone(cloned.Path, r.Host, cloned); err != nil {
		return fmt.Errorf("clone retained at %s; cannot record ownership: %w", cloned.Path, err)
	}
	return launchAndShow(r, s, cloned.Path, uniqueLabel(repoSessionLabel(cloned), sessions), "")
}

// Foreground-only Catppuccin colors preserve fzf's selected-row background.
const (
	pickerMuted   = "\x1b[38;2;166;173;200m"
	pickerName    = "\x1b[38;2;180;190;254m"
	pickerAccent  = "\x1b[38;2;137;180;250m"
	pickerGreen   = "\x1b[38;2;166;227;161m"
	pickerWarning = "\x1b[38;2;249;226;175m"
	pickerReset   = "\x1b[39m"
)

func pickerInk(color, text string) string {
	// Only our own escape sequences are allowed; names and paths remain data.
	return color + pickerText(text) + pickerReset
}

func projectPickerRows(choices []projectChoice) string {
	var rows strings.Builder
	for i, choice := range choices {
		project := choice.Repo
		label := pickerInk(pickerMuted, project.Host+"/") + pickerText(project.Org+"/") + pickerInk(pickerName, project.Name)
		if count := len(choice.Sessions); count > 0 {
			noun := "sessions"
			if count == 1 {
				noun = "session"
			}
			label += "  " + pickerInk(pickerGreen, fmt.Sprintf("[%d %s]", count, noun))
		}
		fmt.Fprintf(&rows, "%d\t%s\x00", i, label)
	}
	return rows.String()
}

func projectPreview(choice projectChoice, warnings int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", pickerInk(pickerName, repoDisplayName(choice.Repo)))
	switch len(choice.Sessions) {
	case 0:
		b.WriteString(pickerInk(pickerAccent, "Enter") + "   Start Pi in this repository\n")
	default:
		b.WriteString(pickerInk(pickerAccent, "Enter") + "   Choose switch/reopen, clone, or cancel\n")
	}
	b.WriteString(pickerInk(pickerAccent, "Ctrl+N") + "  Clone another copy (asks for a suffix)\n")
	fmt.Fprintf(&b, "\n%s  %s\n\n%s\n", pickerInk(pickerAccent, "Path"), pickerText(choice.Repo.Path), pickerInk(pickerAccent, "Sessions (snapshot)"))
	if len(choice.Sessions) == 0 {
		b.WriteString("  " + pickerInk(pickerMuted, "No verified matching session") + "\n")
	}
	for _, session := range choice.Sessions {
		fmt.Fprintf(&b, "  %s\n    %s\n", pickerInk(pickerGreen, session.Label), pickerInk(pickerMuted, session.ID))
	}
	b.WriteString("\n" + pickerInk(pickerMuted, "Session matches are refreshed before acting.") + "\n")
	if warnings > 0 {
		fmt.Fprintf(&b, "\n%s\nMatches may be incomplete.\n", pickerInk(pickerWarning, fmt.Sprintf("Warning: %d session(s) could not be inspected.", warnings)))
	}
	return b.String()
}

func projectPickerArgs(dir, query, notice string) []string {
	args := pickerArgs(false, "project> ", "Projects")
	header := "Enter select · Ctrl+N clone · Tab preview · Esc cancel"
	if notice != "" {
		header = notice + "\n" + header
	}
	// Only the generated numeric ID reaches the preview command, never a repo
	// name or session label. A private directory holds sanitized plain text.
	quotedDir := "'" + strings.ReplaceAll(dir, "'", "'\"'\"'") + "'"
	return append(args, "--ansi", "--header="+header, "--expect=enter,ctrl-n", "--query="+pickerText(query),
		"--with-shell=/bin/sh -c", "--preview=/bin/cat -- "+quotedDir+"/{1}",
		"--preview-window=right,50%,wrap,border-left,<45(down,45%,border-top)",
		"--preview-label= Project details ", "--bind=tab:toggle-preview")
}

func projectPickerSelection(output []byte, count int) (projectSelection, error) {
	key, rows, ok := bytes.Cut(output, []byte{0})
	if !ok {
		if len(output) == 0 {
			return projectSelection{}, fuzzyfinder.ErrAbort
		}
		return projectSelection{}, fmt.Errorf("picker omitted the action key")
	}
	if string(key) != "" && string(key) != "enter" && string(key) != "ctrl-n" {
		return projectSelection{}, fmt.Errorf("picker returned an unknown action")
	}
	selected, err := pickerSelection(rows, count, false)
	if err != nil {
		return projectSelection{}, err
	}
	return projectSelection{Index: selected[0], Clone: string(key) == "ctrl-n"}, nil
}

func styledProjectPicker(choices []projectChoice, warnings int, query, notice string) (projectSelection, error) {
	dir, err := os.MkdirTemp("", "prex-project-preview-")
	if err != nil {
		return projectSelection{}, err
	}
	defer os.RemoveAll(dir)
	for i, choice := range choices {
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(i)), []byte(projectPreview(choice, warnings)), 0600); err != nil {
			return projectSelection{}, err
		}
	}
	rows := projectPickerRows(choices)
	if warnings > 0 {
		notice = strings.TrimSpace(notice + fmt.Sprintf("\nWarning: %d sessions uninspectable; matches may be incomplete.", warnings))
	}
	out, err := runStyledFZF(projectPickerArgs(dir, query, notice), rows)
	if err != nil {
		return projectSelection{}, err
	}
	return projectPickerSelection(out, len(choices))
}
