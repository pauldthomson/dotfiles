package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

const usage = `prex — project sessions with one Pi agent in Rex

Usage: prex [--client CLIENT] [--server LOCAL-ENDPOINT] [PROJECT]
       prex [options] kill
       prex [options] picker [kill]   (popup is a compatibility alias)
       prex [options] recover
       prex completion

No project: choose a repository under ~/repos/<host>/<org>/<repo>.
Existing session: switch/reopen, clone a new copy, or cancel.
Empty sessions get a new Pi tab; existing programs are not restarted.
kill: closes only the current Rex session. Only recorded prex clones
can be removed, with explicit confirmation and conservative safety checks.
recover: inspect/acknowledge a pending launch before allowing another one.
Requires a local Rex server and connected app. Flags precede the command.
`

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "prex:", err)
		os.Exit(1)
	}
}
func run(args []string) error {
	flags := flag.NewFlagSet("prex", flag.ContinueOnError)
	client := flags.String("client", "", "Rex app client ID or label")
	endpoint := flags.String("server", "", "local Rex Unix socket endpoint")
	picker := flags.Bool("_picker", false, "internal picker tab")
	returnWindow := flags.String("_return-window", "", "internal originating tab ID")
	flags.Usage = func() { fmt.Print(usage) }
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	args = flags.Args()
	if *picker {
		paintPicker()
	}
	if len(args) > 0 && args[0] == "completion" {
		return completion(*endpoint, args[1:])
	}
	if len(args) > 0 && args[0] == "_cleanup" {
		if len(args) != 2 {
			return fmt.Errorf("internal cleanup requires a job token")
		}
		return runCleanupJob(args[1])
	}
	r, err := connectRex(*endpoint)
	if err != nil {
		return err
	}
	r.Picker = *picker
	if *picker {
		defer restorePickerWindow(r, *returnWindow)
	}
	command := "open"
	if len(args) > 0 {
		switch args[0] {
		case "kill", "picker", "popup", "recover":
			command = args[0]
			args = args[1:]
		}
	}
	if command == "popup" {
		command = "picker"
	}
	if r.Picker {
		// Both open and kill pickers must select their own app tab before
		// reading input; Rex server focus/Lua queues alone are insufficient.
		if err := activatePicker(r, *client); err != nil {
			return err
		}
	} else if command == "open" || command == "picker" {
		if err := r.selectClient(*client); err != nil {
			return err
		}
	}
	if command == "picker" {
		return pickerTab(r, args)
	}
	s, err := lockState(false)
	if err != nil {
		return err
	}
	defer s.Close()
	switch command {
	case "kill":
		err = killCommand(r, s, args)
	case "recover":
		err = recoverCommand(r, s)
	default:
		if len(args) > 1 {
			return fmt.Errorf("expected at most one project name")
		}
		name := ""
		if len(args) == 1 {
			name = args[0]
		}
		err = openCommand(r, s, name)
	}
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return nil
	}
	if err != nil && *picker {
		fmt.Fprintln(os.Stderr, err)
		_, _ = promptLine("Press Enter to close...")
		return nil
	}
	return err
}
func completion(endpoint string, args []string) error {
	if len(args) == 1 && args[0] == "sessions" {
		r, err := connectRex(endpoint)
		if err != nil {
			return err
		}
		sessions, err := r.sessions()
		if err != nil {
			return err
		}
		for _, s := range sessions {
			fmt.Println(s.ID)
		}
		return nil
	}
	if len(args) != 0 {
		return fmt.Errorf("usage: prex completion [sessions]")
	}
	repos, err := findRepos("")
	if err != nil {
		return err
	}
	for _, r := range repos {
		fmt.Println(r.Name)
	}
	return nil
}
func openCommand(r *rexClient, s *stateStore, name string) error {
	if s.Data.Pending != nil {
		return fmt.Errorf("a previous launch has an uncertain outcome; run prex recover before opening another session")
	}
	repos, err := findRepos(name)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		return fmt.Errorf("no repositories match %q under %s (prex does not create placeholder projects)", name, reposRoot())
	}
	selected := repos[0]
	if len(repos) > 1 {
		i, err := chooseOne(r, repos, func(i int) string { return repoDisplayName(repos[i]) }, "project> ", "Projects")
		if err != nil {
			return err
		}
		selected = repos[i]
	}
	sessions, err := r.sessions()
	if err != nil {
		return err
	}
	matches, err := sessionsForRepo(r, s, sessions, selected.Path)
	if err != nil {
		return err
	}
	if len(matches) > 0 {
		target := matches[0]
		if len(matches) > 1 {
			i, err := chooseOne(r, matches, func(i int) string { return sessionDisplay(matches[i]) }, "session> ", "Project sessions")
			if err != nil {
				return err
			}
			target = matches[i]
		}
		choices := []string{"Switch to / reopen " + sessionDisplay(target), "Clone a new copy", "Cancel"}
		i, err := chooseOne(r, choices, func(i int) string { return choices[i] }, "action> ", "Session already exists")
		if err != nil {
			return err
		}
		switch i {
		case 0:
			return openExisting(r, s, target, selected.Path)
		case 2:
			return nil
		}
		cloned, err := cloneRepo(selected)
		if err != nil {
			return err
		}
		selected = cloned
		if err := s.ownClone(selected.Path, r.Host, selected); err != nil {
			return fmt.Errorf("clone retained at %s; cannot record ownership: %w", selected.Path, err)
		}
	}
	label := uniqueLabel(repoSessionLabel(selected), sessions)
	return launchAndShow(r, s, selected.Path, label, "")
}
func sessionsForRepo(r *rexClient, s *stateStore, sessions []session, path string) ([]session, error) {
	var matches []session
	for _, item := range sessions {
		recorded := s.pathFor(r, item.ID)
		if recorded != "" {
			if samePath(recorded, path) {
				matches = append(matches, item)
			}
			continue
		}
		paths, err := r.repoPaths(item.ID)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Skipping unmanaged session", item.ID, ":", err)
			continue
		}
		for _, cwd := range paths {
			if physicalWithin(cwd, path) {
				matches = append(matches, item)
				break
			}
		}
	}
	return matches, nil
}
func sessionDisplay(s session) string { return s.Label + " (" + s.ID + ")" }
func openExisting(r *rexClient, s *stateStore, target session, path string) error {
	view, err := r.view(target.ID)
	if err != nil {
		return err
	}
	blocks, err := r.blocks(target.ID)
	if err != nil {
		return err
	}
	project, err := repoFromPath(path)
	if err != nil {
		return err
	}
	label := repoSessionLabel(project)
	if view.Label != label {
		if err := r.verify(); err != nil {
			return err
		}
		if err := r.run("session", "rename", target.ID, label); err != nil {
			return err
		}
	}
	if (len(view.Windows) == 0 && len(blocks) == 0) || (r.Picker && len(blocks) == 1 && blocks[0].ID == r.Block) {
		return launchAndShow(r, s, path, label, target.ID)
	}
	s.associate(r, target.ID, path)
	if err := s.save(); err != nil {
		return err
	}
	return r.show(target.ID)
}
func launchAndShow(r *rexClient, s *stateStore, path, label, id string) error {
	s.Data.Pending = &launchIntent{Host: r.Host, Instance: r.Instance, Path: path, Label: label, Session: id}
	if err := s.save(); err != nil {
		return err
	}
	sid, err := r.launch(path, label, id)
	if err != nil {
		return fmt.Errorf("repository retained at %s: %w", path, err)
	}
	s.associate(r, sid, path)
	s.Data.Pending = nil
	if err := s.save(); err != nil {
		return fmt.Errorf("session %s exists but registry could not be saved: %w", sid, err)
	}
	fmt.Printf("Session %s — %s\n", sid, path)
	return r.show(sid)
}
func recoverCommand(r *rexClient, s *stateStore) error {
	p := s.Data.Pending
	if p == nil {
		fmt.Println("No pending launch.")
		return nil
	}
	fmt.Printf("Pending launch: %s (%s)\n", p.Path, p.Label)
	if p.Host != r.Host || p.Instance != r.Instance {
		fmt.Println("The launch belongs to another server instance. No current sessions will be adopted, and no directories will be deleted.")
		yes, err := confirm("After inspecting the old server, abandon this pending launch intent? [y/N]: ")
		if err != nil || !yes {
			return err
		}
		s.Data.Pending = nil
		return s.save()
	}
	sessions, err := r.sessions()
	if err != nil {
		return err
	}
	var matches []session
	for _, item := range sessions {
		if p.Session != "" && item.ID != p.Session {
			continue
		}
		if p.Session == "" && item.Label != p.Label {
			continue
		}
		paths, err := r.repoPaths(item.ID)
		if err != nil {
			return err
		}
		for _, cwd := range paths {
			if physicalWithin(cwd, p.Path) {
				matches = append(matches, item)
				break
			}
		}
	}
	if len(matches) == 1 {
		s.associate(r, matches[0].ID, p.Path)
		s.Data.Pending = nil
		if err := s.save(); err != nil {
			return err
		}
		fmt.Println("Recovered", sessionDisplay(matches[0]))
		return nil
	}
	fmt.Println("Could not uniquely associate the pending launch. Current sessions:")
	for _, item := range sessions {
		fmt.Println(" ", sessionDisplay(item))
	}
	yes, err := confirm("After inspecting Rex, clear the launch intent without deleting anything? [y/N]: ")
	if err != nil || !yes {
		return err
	}
	s.Data.Pending = nil
	return s.save()
}
func pickerTab(r *rexClient, args []string) error {
	if len(args) > 1 || (len(args) == 1 && args[0] != "kill") {
		return fmt.Errorf("usage: prex picker [kill]")
	}
	if r.Session == "" || r.Window == "" {
		return fmt.Errorf("picker must be launched from a Rex terminal; use prex directly otherwise")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	command := append([]string{executable, "--server", r.Endpoint, "--client", r.Client, "--_picker", "--_return-window", r.Window}, args...)
	label := "Projects"
	if len(args) > 0 {
		label = "Close current session"
	}
	var created struct {
		WindowID string `json:"window_id"`
	}
	if err := r.json(&created, append([]string{"window", "new", "-s", r.Session, label, "-c", os.Getenv("HOME"), "--json", "--"}, command...)...); err != nil {
		return fmt.Errorf("picker creation outcome may be unknown; inspect Rex before retrying: %w", err)
	}
	if !strings.HasPrefix(created.WindowID, "window:") {
		return fmt.Errorf("Rex omitted the picker tab ID; inspect Rex before retrying")
	}
	// The close helper selects its own tab if needed and may already be
	// closing this session; do not race it with another app selection.
	if len(args) > 0 {
		return nil
	}
	return r.showWindow(r.Session, created.WindowID)
}

func restorePickerWindow(r *rexClient, origin string) {
	if origin == "" || origin == r.Window {
		return
	}
	view, err := r.view(r.Session)
	if err != nil || view.ActiveWindow != r.Window {
		return
	}
	for _, window := range view.Windows {
		if window.ID == origin {
			// Focus within the source session, not session.select: selecting a
			// project in another session must not switch the app back here.
			if err := r.run("focus", "-s", r.Session, origin); err != nil {
				fmt.Fprintln(os.Stderr, "Could not restore source tab:", err)
			}
			return
		}
	}
}
