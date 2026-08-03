package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

func openCommand(args []string) error {
	var projectName string
	if len(args) > 0 {
		projectName = args[0]
	}

	repos, err := findRepos(projectName)
	if err != nil {
		return err
	}

	if len(repos) == 0 {
		if projectName == "" {
			return errors.New("no projects found")
		}

		fmt.Println("Project doesn't exist")
		if err := runCmd("tmux", "new", "-d", "-s", sanitizeSessionName(projectName)); err != nil {
			return err
		}
		return switchToSession(sanitizeSessionName(projectName))
	}

	selected, err := selectRepo(repos)
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return nil
	}
	if err != nil {
		return err
	}

	projectName = sanitizeSessionName(selected.Name)
	isCopy := false

	if sessionName, ok := sessionForRepo(selected.Path); ok {
		action, err := chooseExistingSessionAction(sessionName)
		if err != nil {
			return err
		}

		switch action {
		case "switch":
			return switchToSession(sessionName)
		case "clone":
			cloned, err := cloneRepo(selected)
			if err != nil {
				return err
			}
			selected = cloned
			projectName = sanitizeSessionName(cloned.Name)
			isCopy = true
		case "abort":
			return nil
		}
	}

	sessionName := uniqueSessionName(projectName)
	if err := createProjectSession(sessionName, selected.Path, isCopy); err != nil {
		if isCopy {
			return fmt.Errorf("create session for cloned repo %s (clone retained): %w", selected.Path, err)
		}
		return err
	}

	return switchToSession(sessionName)
}

func killCommand(args []string) error {
	sessions := args
	if len(sessions) == 0 {
		allSessions, err := listSessions()
		if err != nil {
			return err
		}
		if len(allSessions) == 0 {
			fmt.Println("No tmux sessions found")
			return nil
		}

		idxs, err := fuzzyfinder.FindMulti(allSessions, func(i int) string {
			return allSessions[i]
		}, fuzzyfinder.WithPromptString("kill> "), fuzzyfinder.WithHeader("TAB to select, ENTER to kill"))
		if errors.Is(err, fuzzyfinder.ErrAbort) {
			return nil
		}
		if err != nil {
			return err
		}
		for _, idx := range idxs {
			sessions = append(sessions, allSessions[idx])
		}
	}

	for _, session := range sessions {
		if err := killSession(session); err != nil {
			return err
		}
	}
	return nil
}

func killSession(session string) error {
	repoPath, _ := sessionRepoPath(session)
	isCopy, err := tmuxOption(session, "@ptmux_is_copy")
	knownCopy := err == nil && isCopy == "1"
	inferredCopy := repoPath != "" && looksLikeClonedRepo(repoPath)
	if !knownCopy && !inferredCopy {
		return runCmd("tmux", "kill-session", "-t="+session)
	}

	if repoPath == "" {
		showWarning(fmt.Sprintf("Not killing %s.\n\nThis session is marked as a cloned copy, but ptmux could not determine its repo path.", session))
		return nil
	}

	ready, reason := repoReadyForRemoval(repoPath)
	if !ready {
		showWarning(fmt.Sprintf("Not killing %s.\n\n%s\n\nPush or clean the cloned repo before closing this session.", session, reason))
		return nil
	}

	removeRepo, err := confirm(fmt.Sprintf("Remove cloned repo %s after killing session %s? [y/N]: ", repoPath, session))
	if err != nil {
		return err
	}

	if removeRepo && sessionIsCurrent(session) {
		return scheduleClonedSessionCleanup(session, repoPath)
	}

	if err := runCmd("tmux", "kill-session", "-t="+session); err != nil {
		return err
	}

	if removeRepo {
		ready, reason := repoReadyForRemoval(repoPath)
		if !ready {
			return fmt.Errorf("session %s was killed, but cloned repo %s was retained because its state changed or could not be revalidated: %s", session, repoPath, reason)
		}
		if err := removeClonedRepo(repoPath, knownCopy || inferredCopy); err != nil {
			return fmt.Errorf("session %s was killed, but its cloned repo could not be removed: %w", session, err)
		}
		fmt.Printf("Removed %s\n", repoPath)
	}

	return nil
}

func sessionIsCurrent(session string) bool {
	if os.Getenv("TMUX") == "" {
		return false
	}
	current, err := outputCmd("tmux", "display-message", "-p", "#{client_session}")
	return err == nil && strings.TrimSpace(current) == session
}

func scheduleClonedSessionCleanup(session, repoPath string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate ptmux executable for clone cleanup: %w", err)
	}
	// A popup belongs to the session from which it was opened. Killing that
	// session terminates the popup and ptmux before inline clone removal runs,
	// so start a process in a separate OS session that can finish independently.
	return startDetachedProcess(executable, "_cleanup-cloned-session", session, repoPath)
}

func cleanupClonedSessionCommand(session, expectedRepoPath string) error {
	err := cleanupClonedSession(session, expectedRepoPath)
	if err != nil {
		_ = runSilent("tmux", "display-message", "ptmux cleanup failed: "+err.Error())
		return err
	}
	message := fmt.Sprintf("Removed %s", expectedRepoPath)
	fmt.Println(message)
	_ = runSilent("tmux", "display-message", message)
	return nil
}

func cleanupClonedSession(session, expectedRepoPath string) error {
	repoPath, ok := sessionRepoPath(session)
	if !ok || !samePath(repoPath, expectedRepoPath) {
		return fmt.Errorf("refusing cleanup because session %s no longer points to %s", session, expectedRepoPath)
	}

	isCopy, err := tmuxOption(session, "@ptmux_is_copy")
	knownCopy := err == nil && isCopy == "1"
	inferredCopy := looksLikeClonedRepo(repoPath)
	if !knownCopy && !inferredCopy {
		return fmt.Errorf("refusing cleanup because session %s is no longer recognized as a cloned copy", session)
	}

	ready, reason := repoReadyForRemoval(repoPath)
	if !ready {
		return fmt.Errorf("refusing cleanup because cloned repo state changed: %s", reason)
	}
	if err := runCmd("tmux", "kill-session", "-t="+session); err != nil {
		return err
	}
	ready, reason = repoReadyForRemoval(repoPath)
	if !ready {
		return fmt.Errorf("session %s was killed, but cloned repo %s was retained because its state changed or could not be revalidated: %s", session, repoPath, reason)
	}
	if err := removeClonedRepo(repoPath, knownCopy || inferredCopy); err != nil {
		return fmt.Errorf("session %s was killed, but its cloned repo could not be removed: %w", session, err)
	}
	return nil
}

func cleanupClonedSessionUsageError() error {
	return errors.New("internal clone cleanup requires a session and repository path")
}

func selectRepo(repos []repo) (repo, error) {
	if len(repos) == 1 {
		return repos[0], nil
	}

	idx, err := fuzzyfinder.Find(repos, func(i int) string {
		return repoDisplayName(repos[i])
	})
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return repo{}, err
	}
	if err != nil {
		return repo{}, err
	}
	return repos[idx], nil
}

func chooseExistingSessionAction(sessionName string) (string, error) {
	choices := []struct {
		label  string
		action string
	}{
		{fmt.Sprintf("Switch to existing session (%s)", sessionName), "switch"},
		{"Clone a new copy", "clone"},
		{"Abort", "abort"},
	}

	idx, err := fuzzyfinder.Find(choices, func(i int) string {
		return choices[i].label
	}, fuzzyfinder.WithPromptString("session exists> "))
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		return "abort", nil
	}
	if err != nil {
		return "", err
	}
	return choices[idx].action, nil
}
