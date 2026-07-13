package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func repoReadyForRemoval(repoPath string) (bool, string) {
	if isJJRepo(repoPath) {
		return jjRepoReadyForRemoval(repoPath)
	}
	return gitRepoReadyForRemoval(repoPath)
}

func jjRepoReadyForRemoval(repoPath string) (bool, string) {
	if _, err := jjOutput(repoPath, "git", "fetch", "--all-remotes"); err != nil {
		return false, fmt.Sprintf("Unable to fetch remotes with jj to verify pushed changes: %v", err)
	}

	status, err := jjOutput(repoPath, "status")
	if err != nil {
		return false, fmt.Sprintf("Unable to read jj status: %v", err)
	}
	if strings.Contains(status, "Working copy changes:") {
		return false, "The cloned repo has working copy changes."
	}

	unpushed, err := jjOutput(repoPath, "log", "-r", "~ancestors(remote_bookmarks() | remote_tags()) & (~empty() | ~description(\"\") | bookmarks() | tags())", "--no-graph", "-T", "change_id.short() ++ \" \" ++ description.first_line() ++ \"\\n\"")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect jj commits for unpushed changes: %v", err)
	}
	unpushed = strings.TrimSpace(unpushed)
	if unpushed != "" {
		return false, fmt.Sprintf("The cloned repo has unpushed jj changes:\n%s", unpushed)
	}

	return true, ""
}

func gitRepoReadyForRemoval(repoPath string) (bool, string) {
	if err := runSilent("git", "-C", repoPath, "fetch", "--all", "--prune"); err != nil {
		return false, fmt.Sprintf("Unable to fetch remotes to verify pushed changes: %v", err)
	}

	status, err := gitOutput(repoPath, "status", "--porcelain=v1")
	if err != nil {
		return false, fmt.Sprintf("Unable to read git status: %v", err)
	}
	if strings.TrimSpace(status) != "" {
		return false, "The cloned repo has uncommitted or untracked changes."
	}

	if _, err := gitOutput(repoPath, "symbolic-ref", "--quiet", "HEAD"); err != nil {
		if _, verifyErr := gitOutput(repoPath, "rev-parse", "--verify", "HEAD"); verifyErr != nil {
			return false, fmt.Sprintf("Unable to inspect detached Git HEAD: %v", verifyErr)
		}
		unpushed, countErr := gitOutput(repoPath, "rev-list", "--count", "HEAD", "--not", "--remotes")
		if countErr != nil {
			return false, fmt.Sprintf("Unable to inspect detached HEAD for unpushed commits: %v", countErr)
		}
		count, parseErr := parseGitCommitCount(unpushed)
		if parseErr != nil {
			return false, fmt.Sprintf("Unable to parse detached HEAD commit count %q: %v", strings.TrimSpace(unpushed), parseErr)
		}
		if count > 0 {
			return false, "Detached HEAD has commits that are not present on any remote."
		}
	}

	localOnly, err := gitOutput(repoPath, "rev-list", "--count", "--all", "--not", "--remotes")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect local Git refs for unpushed commits: %v", err)
	}
	localOnlyCount, err := parseGitCommitCount(localOnly)
	if err != nil {
		return false, fmt.Sprintf("Unable to parse local Git ref commit count %q: %v", strings.TrimSpace(localOnly), err)
	}
	if localOnlyCount > 0 {
		return false, "Local Git refs (such as branches, tags, or stashes) contain commits that are not present on any remote."
	}

	branches, err := gitOutput(repoPath, "for-each-ref", "--format=%(refname:short)%09%(upstream:short)", "refs/heads")
	if err != nil {
		return false, fmt.Sprintf("Unable to inspect local branches: %v", err)
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
			return false, fmt.Sprintf("Unable to compare branch %q with upstream %q: %v", branch, upstream, err)
		}
		count, err := parseGitCommitCount(unpushed)
		if err != nil {
			return false, fmt.Sprintf("Unable to parse commit count for branch %q: %v", branch, err)
		}
		if count > 0 {
			return false, fmt.Sprintf("Branch %q has unpushed commits.", branch)
		}
	}

	return true, ""
}

func parseGitCommitCount(output string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(output))
}

func removeClonedRepo(repoPath string, isCopy bool) error {
	rootPath := reposRoot()
	if !isCopy {
		return fmt.Errorf("refusing to remove %s because it is not marked or inferred as a cloned copy", repoPath)
	}
	if !pathWithin(repoPath, rootPath) || samePath(repoPath, rootPath) {
		return fmt.Errorf("refusing to remove %s because it is not under %s", repoPath, rootPath)
	}

	rel, err := filepath.Rel(rootPath, repoPath)
	if err != nil {
		return fmt.Errorf("resolve cloned repo path %s relative to %s: %w", repoPath, rootPath, err)
	}
	root, err := os.OpenRoot(rootPath)
	if err != nil {
		return fmt.Errorf("open repository root %s: %w", rootPath, err)
	}
	defer root.Close()
	if err := root.RemoveAll(rel); err != nil {
		return fmt.Errorf("remove cloned repo %s safely: %w", repoPath, err)
	}
	return nil
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
