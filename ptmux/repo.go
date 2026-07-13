package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type repo struct {
	Path string
	Rel  string
	Host string
	Org  string
	Name string
}

func findRepos(projectName string) ([]repo, error) {
	args := []string{reposRoot(), "-maxdepth", "3", "-mindepth", "3", "-type", "d"}
	if projectName != "" {
		args = append(args, "-name", projectName)
	}

	out, err := outputCmd("find", args...)
	if err != nil {
		return nil, fmt.Errorf("find repositories: %w", err)
	}

	trimmed := strings.TrimSpace(out)
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

func cloneRepo(source repo) (repo, error) {
	cloneParent := filepath.Join(reposRoot(), source.Host, source.Org)
	if err := os.MkdirAll(cloneParent, 0o755); err != nil {
		return repo{}, fmt.Errorf("create clone parent %s: %w", cloneParent, err)
	}

	defaultSuffix, err := nextCloneSuffix(source)
	if err != nil {
		return repo{}, err
	}
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

		destPath = filepath.Join(cloneParent, source.Name+"-"+suffix)
		if err := os.Mkdir(destPath, 0o755); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return repo{}, fmt.Errorf("reserve clone destination %s: %w", destPath, err)
		}
		fmt.Printf("%s already exists; choose another suffix.\n", destPath)
	}

	remote, err := gitOutput(source.Path, "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(remote) == "" {
		remote = source.Path
	}
	remote = strings.TrimSpace(remote)

	if err := runCmd("git", "clone", remote, destPath); err != nil {
		return repo{}, removeIncompleteClone(destPath, err)
	}

	if err := runCmd("jj", "git", "init", "--colocate", destPath); err != nil {
		return repo{}, removeIncompleteClone(destPath, err)
	}

	return repoFromPath(destPath)
}

func removeIncompleteClone(path string, cause error) error {
	if cleanupErr := os.RemoveAll(path); cleanupErr != nil {
		return errors.Join(cause, fmt.Errorf("remove incomplete clone %s: %w", path, cleanupErr))
	}
	return cause
}

func nextCloneSuffix(source repo) (string, error) {
	for i := 2; ; i++ {
		candidate := filepath.Join(reposRoot(), source.Host, source.Org, fmt.Sprintf("%s-%d", source.Name, i))
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return strconv.Itoa(i), nil
		} else if err != nil {
			return "", fmt.Errorf("check clone suffix %s: %w", candidate, err)
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
	if !pathWithin(cleanPath, reposRoot()) || samePath(cleanPath, reposRoot()) {
		return repo{}, fmt.Errorf("%s is outside repository root %s", path, reposRoot())
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
	if err != nil || !pathWithin(cleanPath, reposRoot()) {
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
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}
