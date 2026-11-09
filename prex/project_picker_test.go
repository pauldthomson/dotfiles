package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ktr0731/go-fuzzyfinder"
)

func pickerTestUI(t *testing.T, project func([]projectChoice, int, string, string) (projectSelection, error), session func(*rexClient, []session) (int, error)) {
	t.Helper()
	oldProject, oldSession, oldAction := chooseProject, chooseProjectSession, chooseProjectAction
	chooseProject, chooseProjectSession = project, session
	t.Cleanup(func() { chooseProject, chooseProjectSession, chooseProjectAction = oldProject, oldSession, oldAction })
}

func TestProjectPickerEnterAutomaticallyPromptsForExistingSessions(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			r.Picker = true
			var sessions []session
			for i := 0; i < count; i++ {
				id := fmt.Sprintf("session:%d", i)
				sessions = append(sessions, session{ID: id, Label: "org/project"})
				s.associate(r, id, path)
			}
			projectPrompts, sessionPrompts, actionPrompts, launches := 0, 0, 0, 0
			pickerTestUI(t, func(choices []projectChoice, warnings int, _, _ string) (projectSelection, error) {
				projectPrompts++
				if len(choices) != 1 || len(choices[0].Sessions) != count || warnings != 0 {
					t.Fatal("incorrect project preview model")
				}
				return projectSelection{Index: 0}, nil
			}, func(_ *rexClient, choices []session) (int, error) {
				sessionPrompts++
				return len(choices) - 1, nil
			})
			chooseProjectAction = func(_ *rexClient, target session) (int, error) {
				actionPrompts++
				if count == 0 || target.ID != sessions[count-1].ID {
					t.Fatal("wrong action-menu target")
				}
				return 0, nil
			}
			base := f.output
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				if len(args) > 2 && args[0] == "do" && args[2] == lookupScript {
					return lookupResult(r, sessions, nil), nil
				}
				if args[0] == "session" {
					return jsonTest(map[string]any{"session_id": args[2], "label": "org/project", "windows": []map[string]string{{"window_id": "window:existing"}}}), nil
				}
				if args[0] == "block" {
					return `{"blocks":[{"block_id":"block:existing"}]}`, nil
				}
				if args[0] == "new" {
					launches++
					return `{"session_id":"session:new","initial_windows":[{"block_ids":["block:new"]}]}`, nil
				}
				return base(a)
			}
			if err := openCommand(r, s, "project"); err != nil {
				t.Fatal(err)
			}
			wantLaunches, wantSessionPrompts := 0, 0
			if count == 0 {
				wantLaunches = 1
			}
			if count > 1 {
				wantSessionPrompts = 1
			}
			wantActionPrompts := 0
			if count > 0 {
				wantActionPrompts = 1
			}
			if projectPrompts != 1 || sessionPrompts != wantSessionPrompts || actionPrompts != wantActionPrompts || launches != wantLaunches || len(s.Data.Clones) != 0 {
				t.Fatal("Enter did not automatically offer actions for existing sessions")
			}
			target := "session:new"
			if count > 0 {
				target = sessions[count-1].ID
			}
			found := false
			for _, call := range f.calls {
				if strings.Join(rexTail(call), " ") == "-C client:app do session.select session_id="+target {
					found = true
				}
			}
			if !found {
				t.Fatal("wrong session displayed")
			}
		})
	}
}

func TestProjectPickerExistingSessionActionCancellation(t *testing.T) {
	for _, escape := range []bool{false, true} {
		t.Run(fmt.Sprint(escape), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			r.Picker = true
			target := session{ID: "session:existing"}
			s.associate(r, target.ID, path)
			pickerTestUI(t, func([]projectChoice, int, string, string) (projectSelection, error) {
				return projectSelection{Index: 0}, nil
			}, nil)
			prompted := false
			chooseProjectAction = func(*rexClient, session) (int, error) {
				prompted = true
				if escape {
					return 0, fuzzyfinder.ErrAbort
				}
				return 2, nil
			}
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				if len(args) < 3 || args[0] != "do" || args[2] != lookupScript {
					t.Fatal("cancellation switched or launched a session")
				}
				return lookupResult(r, []session{target}, nil), nil
			}
			err := openCommand(r, s, "project")
			if !prompted || (!escape && err != nil) || (escape && !errors.Is(err, fuzzyfinder.ErrAbort)) {
				t.Fatalf("prompted=%v error=%v", prompted, err)
			}
			if len(f.calls) != 2 || s.Data.Pending != nil || len(s.Data.Clones) != 0 {
				t.Fatal("cancellation changed launch state")
			}
		})
	}
}

func TestProjectPickerCancelHasNoLaunchOrClone(t *testing.T) {
	s := isolatedState(t)
	projectDir(t)
	r, f := fakeRex(t)
	r.Picker = true
	pickerTestUI(t, func([]projectChoice, int, string, string) (projectSelection, error) {
		return projectSelection{}, fuzzyfinder.ErrAbort
	}, nil)
	f.output = func(a []string) (string, error) {
		args := rexTail(a)
		if len(args) < 3 || args[0] != "do" || args[2] != lookupScript {
			t.Fatal("cancellation performed an operation")
		}
		return lookupResult(r, nil, nil), nil
	}
	if err := openCommand(r, s, "project"); !errors.Is(err, fuzzyfinder.ErrAbort) {
		t.Fatalf("error=%v", err)
	}
	if len(f.calls) != 1 || len(s.Data.Clones) != 0 || s.Data.Pending != nil {
		t.Fatal("cancellation changed state")
	}
}

func TestProjectPickerRefreshRequiresConfirmationWhenActionChanges(t *testing.T) {
	for _, appeared := range []bool{false, true} {
		t.Run(fmt.Sprint(appeared), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			r.Picker = true
			reads, prompts := 0, 0
			pickerTestUI(t, func(choices []projectChoice, _ int, query, notice string) (projectSelection, error) {
				prompts++
				if prompts == 2 {
					if query == "" || notice == "" || (len(choices[0].Sessions) == 1) != appeared {
						t.Fatal("changed state was not presented for confirmation")
					}
					return projectSelection{}, fuzzyfinder.ErrAbort
				}
				return projectSelection{Index: 0}, nil
			}, nil)
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				if len(args) < 3 || args[0] != "do" || args[2] != lookupScript {
					t.Fatal("acted on a stale preview")
				}
				reads++
				present := (reads == 1) != appeared
				if !present {
					return lookupResult(r, nil, nil), nil
				}
				return lookupResult(r, []session{{ID: "session:changed"}}, map[string]lookupSession{"session:changed": {Blocks: map[string]lookupBlock{"block:agent": {Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"child": map[string]string{"cwd": path}}))}}}}), nil
			}
			if err := openCommand(r, s, "project"); !errors.Is(err, fuzzyfinder.ErrAbort) {
				t.Fatalf("error=%v", err)
			}
			if prompts != 2 || reads != 2 || len(s.Data.Clones) != 0 || s.Data.Pending != nil {
				t.Fatal("unexpected refresh behavior")
			}
		})
	}
}

func TestProjectPickerCloneIsExplicitAndKeepsOwnershipChecks(t *testing.T) {
	for _, shortcut := range []bool{false, true} {
		t.Run(fmt.Sprintf("shortcut=%v", shortcut), func(t *testing.T) { testProjectPickerClone(t, shortcut) })
	}
}

func testProjectPickerClone(t *testing.T, shortcut bool) {
	safetyEnvironment(t)
	path := safetySource(t)
	t.Setenv("XDG_STATE_HOME", filepath.Join(os.Getenv("HOME"), "state"))
	s, err := lockState(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	r, f := fakeRex(t)
	r.Picker = true
	pickerTestUI(t, func([]projectChoice, int, string, string) (projectSelection, error) {
		return projectSelection{Index: 0, Clone: shortcut}, nil
	}, nil)
	var sessions []session
	if !shortcut {
		sessions = []session{{ID: "session:original", Label: "org/project"}}
		s.associate(r, sessions[0].ID, path)
	}
	actionPrompts := 0
	chooseProjectAction = func(_ *rexClient, target session) (int, error) {
		actionPrompts++
		if target.ID != "session:original" {
			t.Fatal("wrong clone-menu target")
		}
		return 1, nil
	}
	base := f.output
	clones := 0
	f.output = func(a []string) (string, error) {
		if a[0] != "rex" {
			return (systemCommandRunner{}).Output(a[0], a[1:]...)
		}
		args := rexTail(a)
		if len(args) > 2 && args[0] == "do" && args[2] == lookupScript {
			return lookupResult(r, sessions, nil), nil
		}
		if args[0] == "new" {
			return `{"session_id":"session:clone","initial_windows":[{"block_ids":["block:new"]}]}`, nil
		}
		return base(a)
	}
	f.run = func(a []string) error {
		if a[0] != "jj" {
			t.Fatalf("unexpected command: %s", a[0])
		}
		clones++
		return (systemCommandRunner{}).Run(a[0], a[1:]...)
	}
	safetyInput(t, "\n")
	if err := openCommand(r, s, "project"); err != nil {
		t.Fatal(err)
	}
	wantPrompts := 1
	if shortcut {
		wantPrompts = 0
	}
	if actionPrompts != wantPrompts {
		t.Fatal("incorrect clone action prompts")
	}
	destination := path + "-2"
	if clones != 1 || s.pathFor(r, "session:clone") != destination || len(s.Data.Clones) != 1 {
		t.Fatal("explicit clone was not recorded")
	}
	if err := validateClone(destination, s.Data.Clones[destination], r.Host); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "tracked.txt")); err != nil {
		t.Fatal("original repository changed")
	}
}

func TestProjectPickerRefusesFailedRefreshAndSessionCancellation(t *testing.T) {
	for _, failRefresh := range []bool{false, true} {
		t.Run(fmt.Sprint(failRefresh), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			r.Picker = true
			sessions := []session{{ID: "session:one"}, {ID: "session:two"}}
			for _, item := range sessions {
				s.associate(r, item.ID, path)
			}
			pickerTestUI(t, func([]projectChoice, int, string, string) (projectSelection, error) {
				return projectSelection{Index: 0}, nil
			}, func(*rexClient, []session) (int, error) { return 0, fuzzyfinder.ErrAbort })
			reads := 0
			f.output = func(a []string) (string, error) {
				args := rexTail(a)
				if len(args) < 3 || args[0] != "do" || args[2] != lookupScript {
					t.Fatal("unexpected operation after failure/cancellation")
				}
				reads++
				if reads == 2 && failRefresh {
					return "", errors.New("disconnected")
				}
				return lookupResult(r, sessions, nil), nil
			}
			err := openCommand(r, s, "project")
			if err == nil || (!failRefresh && !errors.Is(err, fuzzyfinder.ErrAbort)) {
				t.Fatalf("error=%v", err)
			}
			if reads != 2 || s.Data.Pending != nil || len(s.Data.Clones) != 0 {
				t.Fatal("unexpected launch or clone")
			}
		})
	}
}

func TestProjectPickerCleansPreviewsAndIgnoresPersonalFZFOptions(t *testing.T) {
	for _, status := range []int{0, 2, 130} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TMPDIR", dir)
			t.Setenv("PATH", dir)
			for _, key := range []string{"FZF_DEFAULT_OPTS", "FZF_DEFAULT_OPTS_FILE", "FZF_DEFAULT_COMMAND"} {
				t.Setenv(key, "must not reach embedded fzf")
			}
			script := "#!/bin/sh\n" +
				"if [ -n \"${FZF_DEFAULT_OPTS+x}${FZF_DEFAULT_OPTS_FILE+x}${FZF_DEFAULT_COMMAND+x}\" ]; then exit 9; fi\n" +
				"printf 'enter\\0000\\tfixture\\000'\n" + fmt.Sprintf("exit %d\n", status)
			if err := os.WriteFile(filepath.Join(dir, "fzf"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			selected, err := styledProjectPicker([]projectChoice{{Repo: repo{Name: "fixture"}}}, 0, "", "")
			if (err == nil) != (status == 0) || (status == 0 && selected != (projectSelection{Index: 0})) {
				t.Fatalf("selection=%v error=%v", selected, err)
			}
			if status == 130 && !errors.Is(err, fuzzyfinder.ErrAbort) {
				t.Fatalf("cancel error=%v", err)
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, "prex-project-preview-*"))
			if err != nil || len(leftovers) != 0 {
				t.Fatalf("preview files retained: %v (%v)", leftovers, err)
			}
		})
	}
}

func TestProjectPickerActionParsing(t *testing.T) {
	for _, key := range []string{"", "enter", "ctrl-n"} {
		got, err := projectPickerSelection([]byte(key+"\x001\tduplicate label\x00"), 2)
		if err != nil || got.Index != 1 || got.Clone != (key == "ctrl-n") {
			t.Fatalf("key=%q got=%v error=%v", key, got, err)
		}
	}
	for _, bad := range []string{"enter", "delete\x000\tx\x00", "ctrl-n\x00-1\tx\x00", "ctrl-n\x002\tx\x00", "enter\x000\tx\x001\ty\x00"} {
		if _, err := projectPickerSelection([]byte(bad), 2); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, empty := range [][]byte{nil, []byte("ctrl-n\x00")} {
		if _, err := projectPickerSelection(empty, 2); !errors.Is(err, fuzzyfinder.ErrAbort) {
			t.Fatalf("missing choice: %v", err)
		}
	}
}

func plainPickerText(text string) string {
	for _, code := range []string{pickerMuted, pickerName, pickerAccent, pickerGreen, pickerWarning, pickerReset} {
		text = strings.ReplaceAll(text, code, "")
	}
	return text
}

func TestProjectRowsOnlyAnnotateExistingSessions(t *testing.T) {
	choices := []projectChoice{
		{Repo: repo{Host: "host", Org: "org", Name: "plain"}},
		{Repo: repo{Host: "host", Org: "org", Name: "existing"}, Sessions: []session{{ID: "session:one"}}},
		{Repo: repo{Host: "host", Org: "org", Name: "evil\x1b[31m\n\tname"}, Sessions: []session{{}, {}}},
	}
	rows := projectPickerRows(choices)
	plain := plainPickerText(rows)
	if strings.ContainsRune(plain, '\x1b') || strings.Contains(plain, "start Pi") {
		t.Fatal("unsafe escapes or redundant launch badge")
	}
	if !strings.Contains(plain, "0\thost/org/plain\x00") || !strings.Contains(plain, "existing  [1 session]") || !strings.Contains(plain, "[2 sessions]") {
		t.Fatal("incorrect session annotations")
	}
	if _, err := exec.LookPath("fzf"); err != nil {
		t.Skip("fzf not installed")
	}
	cmd := exec.Command("fzf", append(pickerArgs(false, "project> ", "Projects"), "--ansi", "--filter=existing")...)
	cmd.Env = []string{"PATH=/usr/bin:/bin", "FZF_DEFAULT_OPTS=", "FZF_DEFAULT_OPTS_FILE="}
	cmd.Stdin = strings.NewReader(rows)
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	selected, err := pickerSelection(out, len(choices), false)
	if err != nil || !reflect.DeepEqual(selected, []int{1}) {
		t.Fatalf("colored selection=%v error=%v", selected, err)
	}
}

func TestProjectPreviewIsDataNotShellCode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "preview ' $(touch injected)")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	choice := projectChoice{Repo: repo{Host: "host", Org: "org", Name: "evil\x1b[31m\nname", Path: "/repo/$(touch injected)"}, Sessions: []session{{ID: "session:one", Label: "label\twith\x1bcontrols"}}}
	preview := projectPreview(choice, 1)
	if strings.ContainsRune(plainPickerText(preview), '\x1b') || strings.Contains(preview, "label\t") || !strings.Contains(preview, "Matches may be incomplete") || !strings.Contains(preview, "Ctrl+N") {
		t.Fatal("preview omitted actions or allowed terminal controls")
	}
	if err := os.WriteFile(filepath.Join(dir, "0"), []byte(preview), 0600); err != nil {
		t.Fatal(err)
	}
	var command string
	for _, arg := range projectPickerArgs(dir, "", "") {
		if strings.HasPrefix(arg, "--preview=") {
			command = strings.TrimPrefix(arg, "--preview=")
		}
	}
	command = strings.ReplaceAll(command, "{1}", "'0'")
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Dir = filepath.Dir(dir)
	out, err := cmd.Output()
	if err != nil || string(out) != preview {
		t.Fatalf("preview command failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "injected")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("preview interpreted data as code")
	}
}

func TestProjectPreviewUsesSameMatchingAsTerminalFlow(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, _ := fakeRex(t)
	project, _ := repoFromPath(path)
	other := project
	other.Path = path + "-other"
	other.Name = "other"
	if err := os.Mkdir(other.Path, 0755); err != nil {
		t.Fatal(err)
	}
	s.associate(r, "session:recorded", path)
	sessions := []session{{ID: "session:observed"}, {ID: "session:recorded"}, {ID: "session:bad"}}
	lookup := map[string]lookupSession{"session:observed": {Blocks: map[string]lookupBlock{"block:x": {Host: r.Host, Creator: "com.superlogical.terminal", Process: lookupProcess(t, jsonTest(map[string]any{"foreground": map[string]string{"cwd": path}}))}}}, "session:bad": {Error: "unavailable"}}
	choices, warnings := projectChoices(r, s, []repo{project, other}, sessions, lookup)
	if warnings != 1 {
		t.Fatal("missing incomplete-inspection warning")
	}
	for _, choice := range choices {
		want := sessionsForRepo(r, s, sessions, lookup, choice.Repo.Path)
		if !reflect.DeepEqual(want, choice.Sessions) {
			t.Fatal("preview matches differ from normal lookup")
		}
	}
}
