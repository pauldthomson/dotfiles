package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/ktr0731/go-fuzzyfinder"
)

type repo struct {
	Path string
	Rel  string
	Host string
	Org  string
	Name string
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "kill" {
		if err := killCommand(args[1:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	if err := openCommand(args); err != nil {
		log.Fatal(err)
	}
}

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
		return runCmd("tmux", "kill-session", "-t", session)
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

	if removeRepo {
		if err := removeClonedRepo(repoPath, knownCopy || inferredCopy); err != nil {
			return err
		}
		fmt.Printf("Removed %s\n", repoPath)
	}

	return runCmd("tmux", "kill-session", "-t", session)
}

func findRepos(projectName string) ([]repo, error) {
	args := []string{reposRoot(), "-maxdepth", "3", "-mindepth", "3", "-type", "d"}
	if projectName != "" {
		args = append(args, "-name", projectName)
	}

	out, err := exec.Command("find", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}

	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return nil, nil
	}

	paths := strings.Split(trimmed, "\n")
	repos := make([]repo, 0, len(paths))
	for _, p := range paths {
		r, err := repoFromPath(p)
		if err != nil {
			continue
		}
		repos = append(repos, r)
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].Rel < repos[j].Rel
	})
	return repos, nil
}

func selectRepo(repos []repo) (repo, error) {
	if len(repos) == 1 {
		return repos[0], nil
	}

	idx, err := fuzzyfinder.Find(repos, func(i int) string {
		return repoDisplayName(repos[i])
	})
	if errors.Is(err, fuzzyfinder.ErrAbort) {
		os.Exit(0)
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

func cloneRepo(source repo) (repo, error) {
	defaultSuffix := nextCloneSuffix(source)
	var destPath string
	for {
		suffix, err := promptLine(fmt.Sprintf("Suffix for cloned repo [%s]: ", defaultSuffix))
		if err != nil {
			return repo{}, err
		}
		suffix = strings.TrimSpace(suffix)
		if suffix == "" {
			suffix = defaultSuffix
		}
		if strings.ContainsAny(suffix, string(os.PathSeparator)+"/:") {
			fmt.Println("Suffix must not contain path separators or ':'")
			continue
		}

		destPath = filepath.Join(reposRoot(), source.Host, source.Org, source.Name+"-"+suffix)
		if _, err := os.Stat(destPath); errors.Is(err, os.ErrNotExist) {
			break
		}
		fmt.Printf("%s already exists; choose another suffix.\n", destPath)
	}

	remote, err := gitOutput(source.Path, "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(remote) == "" {
		remote = source.Path
	}
	remote = strings.TrimSpace(remote)

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return repo{}, err
	}

	if err := runCmd("git", "clone", remote, destPath); err != nil {
		return repo{}, err
	}

	if err := runCmd("jj", "git", "init", "--colocate", destPath); err != nil {
		return repo{}, err
	}

	return repoFromPath(destPath)
}

func createProjectSession(sessionName, startDir string, isCopy bool) error {
	if err := runCmd("tmux", "new", "-d", "-s", sessionName, "-c", startDir); err != nil {
		return err
	}
	if err := setTmuxOption(sessionName, "@ptmux_repo_path", startDir); err != nil {
		return err
	}
	if isCopy {
		if err := setTmuxOption(sessionName, "@ptmux_is_copy", "1"); err != nil {
			return err
		}
	}

	if err := runCmd("tmux", "new-window", "-n", "agent", "-t", sessionName, "-c", startDir); err != nil {
		return err
	}

	if err := runCmd("tmux", "send-keys", "-t", fmt.Sprintf("%s:2", sessionName), "pi", "C-m"); err != nil {
		return err
	}

	if err := runCmd("tmux", "new-window", "-n", "scratch", "-t", sessionName, "-c", startDir); err != nil {
		return err
	}

	return runCmd("tmux", "send-keys", "-t", fmt.Sprintf("%s:1", sessionName), "nvim", "C-m")
}

func switchToSession(sessionName string) error {
	if os.Getenv("TMUX") != "" {
		return runCmd("tmux", "switch-client", "-t", fmt.Sprintf("%s:1", sessionName))
	}
	return runCmd("tmux", "attach-session", "-t", fmt.Sprintf("%s:1", sessionName))
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

	out, err := outputCmd("tmux", "list-panes", "-t", session, "-F", "#{pane_current_path}")
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

func repoReadyForRemoval(repoPath string) (bool, string) {
	if isJJRepo(repoPath) {
		return jjRepoReadyForRemoval(repoPath)
	}
	return gitRepoReadyForRemoval(repoPath)
}

func jjRepoReadyForRemoval(repoPath string) (bool, string) {
	if _, err := jjOutput(repoPath, "git", "fetch", "--all-remotes"); err != nil {
		return false, "Unable to fetch remotes with jj to verify pushed changes."
	}

	status, err := jjOutput(repoPath, "status")
	if err != nil {
		return false, "Unable to read jj status."
	}
	if strings.Contains(status, "Working copy changes:") {
		return false, "The cloned repo has working copy changes."
	}

	unpushed, err := jjOutput(repoPath, "log", "-r", "mutable() & ~empty() & ~ancestors(remote_bookmarks())", "--no-graph", "-T", "change_id.short() ++ \" \" ++ description.first_line() ++ \"\\n\"")
	if err != nil {
		return false, "Unable to inspect jj commits for unpushed changes."
	}
	unpushed = strings.TrimSpace(unpushed)
	if unpushed != "" {
		return false, fmt.Sprintf("The cloned repo has unpushed jj changes:\n%s", unpushed)
	}

	return true, ""
}

func gitRepoReadyForRemoval(repoPath string) (bool, string) {
	if err := runSilent("git", "-C", repoPath, "fetch", "--all", "--prune"); err != nil {
		return false, "Unable to fetch remotes to verify pushed changes."
	}

	status, err := gitOutput(repoPath, "status", "--porcelain=v1")
	if err != nil {
		return false, "Unable to read git status."
	}
	if strings.TrimSpace(status) != "" {
		return false, "The cloned repo has uncommitted or untracked changes."
	}

	branches, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname:short)%09%(upstream:short)", "refs/heads")
	if err != nil {
		return false, "Unable to inspect local branches."
	}

	for _, line := range strings.Split(strings.TrimSpace(branches), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		branch := parts[0]
		upstream := ""
		if len(parts) == 2 {
			upstream = parts[1]
		}
		if upstream == "" {
			return false, fmt.Sprintf("Branch %q has no upstream, so ptmux cannot verify it has been pushed.", branch)
		}

		unpushed, err := gitOutput(repoPath, "rev-list", "--count", fmt.Sprintf("%s..%s", upstream, branch))
		if err != nil {
			return false, fmt.Sprintf("Unable to compare branch %q with upstream %q.", branch, upstream)
		}
		count, err := strconv.Atoi(strings.TrimSpace(unpushed))
		if err != nil || count > 0 {
			return false, fmt.Sprintf("Branch %q has unpushed commits.", branch)
		}
	}

	return true, ""
}

func removeClonedRepo(repoPath string, isCopy bool) error {
	root := reposRoot()
	if !isCopy {
		return fmt.Errorf("refusing to remove %s because it is not marked or inferred as a cloned copy", repoPath)
	}
	if !pathWithin(repoPath, root) || samePath(repoPath, root) {
		return fmt.Errorf("refusing to remove %s because it is not under %s", repoPath, root)
	}
	return os.RemoveAll(repoPath)
}

func nextCloneSuffix(source repo) string {
	for i := 2; ; i++ {
		candidate := filepath.Join(reposRoot(), source.Host, source.Org, fmt.Sprintf("%s-%d", source.Name, i))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return strconv.Itoa(i)
		}
	}
}

func repoDisplayName(r repo) string {
	return filepath.Join(r.Org, r.Name)
}

func repoFromPath(path string) (repo, error) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return repo{}, err
	}
	rel, err := filepath.Rel(reposRoot(), cleanPath)
	if err != nil {
		return repo{}, err
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) != 3 {
		return repo{}, fmt.Errorf("%s is not a repo path under %s/<host>/<org>/<repo>", path, reposRoot())
	}
	return repo{
		Path: cleanPath,
		Rel:  filepath.Join(parts[0], parts[1], parts[2]),
		Host: parts[0],
		Org:  parts[1],
		Name: parts[2],
	}, nil
}

func repoRootForPath(path string) (string, bool) {
	cleanPath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(reposRoot(), cleanPath)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) < 3 {
		return "", false
	}
	return filepath.Join(reposRoot(), parts[0], parts[1], parts[2]), true
}

func looksLikeClonedRepo(repoPath string) bool {
	r, err := repoFromPath(repoPath)
	if err != nil {
		return false
	}

	for i := len(r.Name) - 1; i >= 0; i-- {
		if r.Name[i] != '-' {
			continue
		}

		candidate := filepath.Join(reposRoot(), r.Host, r.Org, r.Name[:i])
		if samePath(candidate, repoPath) {
			continue
		}
		if info, err := os.Stat(candidate); err != nil || !info.IsDir() {
			continue
		}
		if sameGitRemote(candidate, repoPath) {
			return true
		}
	}

	return false
}

func sameGitRemote(a, b string) bool {
	remoteA, errA := gitOutput(a, "config", "--get", "remote.origin.url")
	remoteB, errB := gitOutput(b, "config", "--get", "remote.origin.url")
	if errA != nil || errB != nil {
		return false
	}
	return strings.TrimSpace(remoteA) != "" && strings.TrimSpace(remoteA) == strings.TrimSpace(remoteB)
}

func sanitizeSessionName(name string) string {
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, ":", "-")
	return name
}

func setTmuxOption(sessionName, option, value string) error {
	return runCmd("tmux", "set-option", "-t", sessionName, option, value)
}

func tmuxOption(sessionName, option string) (string, error) {
	out, err := outputCmd("tmux", "show-options", "-t", sessionName, "-qv", option)
	return strings.TrimSpace(out), err
}

func gitOutput(repoPath string, args ...string) (string, error) {
	gitArgs := append([]string{"-C", repoPath}, args...)
	return outputCmd("git", gitArgs...)
}

func jjOutput(repoPath string, args ...string) (string, error) {
	jjArgs := append([]string{"-R", repoPath}, args...)
	return outputCmd("jj", jjArgs...)
}

func isJJRepo(repoPath string) bool {
	_, err := jjOutput(repoPath, "root")
	return err == nil
}

func outputCmd(cmd string, args ...string) (string, error) {
	c := exec.Command(cmd, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return string(out), err
	}
	return string(out), nil
}

func runCmd(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func runSilent(cmd string, args ...string) error {
	c := exec.Command(cmd, args...)
	return c.Run()
}

func promptLine(prompt string) (string, error) {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func confirm(prompt string) (bool, error) {
	line, err := promptLine(prompt)
	if err != nil {
		return false, err
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func showWarning(message string) {
	fmt.Fprintln(os.Stderr, message)
	if os.Getenv("TMUX") != "" {
		_, _ = promptLine("\nPress enter to close this warning...")
	}
}

func reposRoot() string {
	return filepath.Join(os.Getenv("HOME"), "repos")
}

func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return filepath.Clean(a) == filepath.Clean(b)
	}
	return filepath.Clean(absA) == filepath.Clean(absB)
}

func pathWithin(path, root string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absPath)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "..")
}
