package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// repoReadyForRemoval is a conservative content check, not deletion authority.
// The caller must separately validate ownership/identity and prevent races with
// writers. Non-colocated JJ, shared worktrees, and submodules are unsupported.
func repoReadyForRemoval(repoPath string) (bool, string) {
	info, err := os.Lstat(repoPath)
	if err != nil || !info.IsDir() {
		return false, "Repository path is missing, symlinked, or not a directory."
	}
	jj := false
	if info, err := os.Lstat(filepath.Join(repoPath, ".jj")); err == nil {
		if !info.IsDir() {
			return false, "JJ administration is not a local directory."
		}
		jj = true
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Sprintf("Unable to inspect JJ administration: %v", err)
	}
	info, err = os.Lstat(filepath.Join(repoPath, ".git"))
	if err != nil || !info.IsDir() {
		return false, "Removal requires a local Git directory (non-colocated JJ and shared Git worktrees are unsupported)."
	}
	top, err := gitOutput(repoPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return false, fmt.Sprintf("Unable to locate Git repository: %v", err)
	}
	physical, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return false, fmt.Sprintf("Unable to resolve repository path: %v", err)
	}
	if !samePath(strings.TrimSpace(top), physical) {
		return false, "Git repository root does not match the removal path."
	}
	if jj {
		info, err := os.Lstat(filepath.Join(repoPath, ".jj", "repo"))
		if err != nil || !info.IsDir() {
			return false, "Shared or unreadable JJ workspaces are unsupported."
		}
		root, err := jjOutput(repoPath, "root")
		if err != nil || !samePath(strings.TrimSpace(root), physical) {
			return false, "Unable to verify the local JJ repository root."
		}
	}
	if ready, reason := gitWorktreeReadyForRemoval(repoPath, jj); !ready {
		return false, reason
	}

	if jj {
		if ready, reason := jjRepoReadyForRemoval(repoPath); !ready {
			return false, reason
		}
	} else if err := runSilent("git", "-C", repoPath, "fetch", "--all", "--prune"); err != nil {
		return false, fmt.Sprintf("Unable to fetch remotes to verify pushed changes: %v", err)
	}
	// Fetch/snapshot can change the index and working tree. Inspect them again.
	if ready, reason := gitWorktreeReadyForRemoval(repoPath, jj); !ready {
		return false, reason
	}
	return gitRefsReadyForRemoval(repoPath, jj)
}

func gitWorktreeReadyForRemoval(repoPath string, jj bool) (bool, string) {
	if _, err := os.Lstat(filepath.Join(repoPath, ".gitmodules")); err == nil {
		return false, "Repositories with submodules (.gitmodules) are unsupported."
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, fmt.Sprintf("Unable to inspect submodule configuration: %v", err)
	}
	// Check both index and HEAD: a gitlink need not have a .gitmodules file,
	// and an uninitialized/deleted submodule can otherwise look clean.
	for _, args := range [][]string{{"ls-files", "--stage", "-z"}, {"ls-tree", "-r", "-z", "HEAD"}} {
		entries, err := gitOutput(repoPath, args...)
		if err != nil {
			return false, fmt.Sprintf("Unable to inspect Git submodules: %v", err)
		}
		for _, entry := range strings.Split(entries, "\x00") {
			if strings.HasPrefix(entry, "160000 ") {
				return false, "Repositories with submodules are unsupported."
			}
		}
	}
	flags, err := gitOutput(repoPath, "ls-files", "-v", "-z")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect Git index flags: %v", err)
	}
	for _, entry := range strings.Split(flags, "\x00") {
		if entry != "" && (entry[0] == 'S' || (entry[0] >= 'a' && entry[0] <= 'z')) {
			return false, "Git index has skip-worktree or assume-unchanged entries that could hide local changes."
		}
	}
	status, err := gitOutput(repoPath, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--ignored=matching", "--ignore-submodules=none")
	if err != nil {
		return false, fmt.Sprintf("Unable to read Git status: %v", err)
	}
	for _, entry := range strings.Split(status, "\x00") {
		if entry == "" {
			continue
		}
		// Only root JJ administration is exempt, never nested .jj directories
		// or a tracked path. Git itself excludes its .git administration.
		if jj && (strings.HasPrefix(entry, "!! .jj/") || strings.HasPrefix(entry, "?? .jj/")) {
			continue
		}
		return false, "The cloned repo has uncommitted, untracked, or ignored files."
	}
	return true, ""
}

func jjRepoReadyForRemoval(repoPath string) (bool, string) {
	if _, err := jjOutput(repoPath, "git", "fetch", "--all-remotes"); err != nil {
		return false, fmt.Sprintf("Unable to fetch remotes with JJ to verify pushed changes: %v", err)
	}
	// A diff is independent of the user's status template/localized prose.
	diff, err := jjOutput(repoPath, "diff", "-r", "@", "--summary")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect JJ working copy: %v", err)
	}
	if strings.TrimSpace(diff) != "" {
		return false, "The cloned repo has JJ working copy changes (or JJ could not inspect it without warnings)."
	}
	// Ordinary empty, undescribed working-copy commits are disposable. Empty
	// commits with descriptions, bookmarks, or tags are not silently discarded.
	unpushed, err := jjOutput(repoPath, "log", "-r", "~ancestors(remote_bookmarks() | remote_tags()) & (~empty() | ~description(\"\") | bookmarks() | tags())", "--no-graph", "-T", "change_id.short() ++ \" \" ++ description.first_line() ++ \"\\n\"")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect JJ commits for unpushed changes: %v", err)
	}
	if strings.TrimSpace(unpushed) != "" {
		return false, fmt.Sprintf("The cloned repo has unpushed JJ changes:\n%s", strings.TrimSpace(unpushed))
	}
	return true, ""
}

func gitRefsReadyForRemoval(repoPath string, jj bool) (bool, string) {
	refs, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname)")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect local Git refs: %v", err)
	}
	for _, ref := range strings.Fields(refs) {
		if ref == "refs/stash" || strings.HasPrefix(ref, "refs/stash/") {
			return false, "Local Git stashes must be preserved."
		}
		if strings.HasPrefix(ref, "refs/remotes/") || (jj && strings.HasPrefix(ref, "refs/jj/keep/")) {
			continue
		}
		if _, err := gitOutput(repoPath, "rev-parse", "--verify", ref+"^{commit}"); err != nil {
			return false, fmt.Sprintf("Unable to verify local Git ref %q as a commit: %v", ref, err)
		}
	}
	args := []string{"rev-list", "--count"}
	if jj {
		// JJ keeps internal snapshot/operation commits here, including its
		// normal empty @. They are covered by the JJ check, not user Git refs.
		args = append(args, "--exclude=refs/jj/keep/*")
	}
	// HEAD must be explicit: --all does not reliably cover a detached HEAD.
	args = append(args, "--all", "HEAD", "--not", "--remotes")
	localOnly, err := gitOutput(repoPath, args...)
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect local Git refs and HEAD: %v", err)
	}
	count, err := parseGitCommitCount(localOnly)
	if err != nil {
		return false, fmt.Sprintf("Unable to parse local Git commit count: %v", err)
	}
	if count != 0 {
		return false, "Local Git refs or HEAD contain commits that are not present on any remote."
	}
	if jj {
		// JJ clones do not configure Git branch upstreams. Reachability above
		// and JJ remote-bookmark checks replace that requirement for JJ only.
		return true, ""
	}
	branches, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname)%09%(upstream)", "refs/heads")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect local branches: %v", err)
	}
	for _, line := range strings.Split(strings.TrimSuffix(branches, "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 || parts[1] == "" {
			return false, fmt.Sprintf("Branch %q has no upstream, so prex cannot verify it has been pushed.", parts[0])
		}
		ahead, err := gitOutput(repoPath, "rev-list", "--count", parts[1]+".."+parts[0])
		if err != nil {
			return false, fmt.Sprintf("Unable to compare branch %q with its upstream: %v", parts[0], err)
		}
		count, err := parseGitCommitCount(ahead)
		if err != nil {
			return false, fmt.Sprintf("Unable to parse commit count for branch %q: %v", parts[0], err)
		}
		if count != 0 {
			return false, fmt.Sprintf("Branch %q has unpushed commits.", parts[0])
		}
	}
	return true, ""
}

func parseGitCommitCount(output string) (int, error) {
	count, err := strconv.Atoi(strings.TrimSpace(output))
	if err == nil && count < 0 {
		err = fmt.Errorf("negative commit count %d", count)
	}
	return count, err
}

func gitOutput(repoPath string, args ...string) (string, error) {
	return outputCmd("git", append([]string{"-C", repoPath}, args...)...)
}

func jjOutput(repoPath string, args ...string) (string, error) {
	return outputCmd("jj", append([]string{"--no-pager", "--color=never", "-R", repoPath}, args...)...)
}
