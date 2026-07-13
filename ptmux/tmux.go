package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

func createProjectSession(sessionName, startDir string, isCopy bool) (err error) {
	if err = runCmd("tmux", "new", "-d", "-s", sessionName, "-c", startDir); err != nil {
		return err
	}
	defer func() {
		if err == nil {
			return
		}
		if cleanupErr := runCmd("tmux", "kill-session", "-t="+sessionName); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("clean up incomplete tmux session %s: %w", sessionName, cleanupErr))
		}
	}()

	if err = setTmuxOption(sessionName, "@ptmux_repo_path", startDir); err != nil {
		return err
	}
	if isCopy {
		if err = setTmuxOption(sessionName, "@ptmux_is_copy", "1"); err != nil {
			return err
		}
	}

	if err = runCmd("tmux", "new-window", "-n", "agent", "-t="+sessionName, "-c", startDir); err != nil {
		return err
	}

	if err = runCmd("tmux", "send-keys", "-t="+fmt.Sprintf("%s:2", sessionName), "pi", "C-m"); err != nil {
		return err
	}

	if err = runCmd("tmux", "new-window", "-n", "scratch", "-t="+sessionName, "-c", startDir); err != nil {
		return err
	}

	err = runCmd("tmux", "send-keys", "-t="+fmt.Sprintf("%s:1", sessionName), "nvim", "C-m")
	return err
}

func switchToSession(sessionName string) error {
	if os.Getenv("TMUX") != "" {
		return runCmd("tmux", "switch-client", "-t="+fmt.Sprintf("%s:1", sessionName))
	}
	return runCmd("tmux", "attach-session", "-t="+fmt.Sprintf("%s:1", sessionName))
}

func sessionForRepo(repoPath string) (string, bool) {
	out, err := outputCmd("tmux", "list-sessions", "-F", "#{session_name}\t#{@ptmux_repo_path}")
	if err == nil {
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) == 2 && samePath(parts[1], repoPath) {
				return parts[0], true
			}
		}
	}

	out, err = outputCmd("tmux", "list-panes", "-a", "-F", "#{session_name}\t#{pane_current_path}")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) == 2 && pathWithin(parts[1], repoPath) {
			return parts[0], true
		}
	}

	return "", false
}

func sessionRepoPath(session string) (string, bool) {
	repoPath, err := tmuxOption(session, "@ptmux_repo_path")
	if err == nil && repoPath != "" {
		return repoPath, true
	}

	out, err := outputCmd("tmux", "list-panes", "-t="+session, "-F", "#{pane_current_path}")
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if repoPath, ok := repoRootForPath(line); ok {
			return repoPath, true
		}
	}

	return "", false
}

func listSessions() ([]string, error) {
	out, err := outputCmd("tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		return nil, err
	}
	trimmed := strings.TrimSpace(out)
	if trimmed == "" {
		return nil, nil
	}
	return strings.Split(trimmed, "\n"), nil
}

func uniqueSessionName(base string) string {
	candidate := base
	for i := 2; sessionExists(candidate); i++ {
		candidate = fmt.Sprintf("%s-%d", base, i)
	}
	return candidate
}

func sessionExists(sessionName string) bool {
	return runSilent("tmux", "has-session", fmt.Sprintf("-t=%s", sessionName)) == nil
}

func setTmuxOption(sessionName, option, value string) error {
	// Unlike other tmux commands, set-option does not accept the leading '='
	// exact-match marker for session targets. This function is only called for a
	// session that ptmux has just created, so the plain name resolves exactly.
	return runCmd("tmux", "set-option", "-t", sessionName, option, value)
}

func tmuxOption(sessionName, option string) (string, error) {
	// show-options silently ignores the '=' exact-match marker. Validate the
	// session exactly first, then use the plain name that is now known to exist.
	if err := runSilent("tmux", "has-session", "-t="+sessionName); err != nil {
		return "", err
	}
	out, err := outputCmd("tmux", "show-options", "-t", sessionName, "-qv", option)
	return strings.TrimSpace(out), err
}
