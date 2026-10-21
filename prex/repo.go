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
	Path        string
	Rel         string
	Host        string
	Org         string
	Name        string
	cloneDevice uint64
	cloneInode  uint64
}

// findRepos follows the local <host>/<org>/<repo> layout, not symlinked
// directories. Project names are literal names, not find/glob expressions.
func findRepos(projectName string) ([]repo, error) {
	var repos []repo
	var visit func(string, int) error
	visit = func(path string, depth int) error {
		entries, err := os.ReadDir(path)
		if err != nil {
			return fmt.Errorf("find repositories in %s: %w", path, err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".prex-removing-") {
				continue
			}
			child := filepath.Join(path, entry.Name())
			if depth < 2 {
				if err := visit(child, depth+1); err != nil {
					return err
				}
				continue
			}
			if projectName != "" && entry.Name() != projectName {
				continue
			}
			r, err := repoFromPath(child)
			if err != nil {
				return err
			}
			repos = append(repos, r)
		}
		return nil
	}
	if err := visit(reposRoot(), 0); err != nil {
		return nil, err
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Rel < repos[j].Rel })
	return repos, nil
}

// cloneRepo reserves the destination before starting JJ. A failed command may
// have written valuable data, so failures deliberately leave that directory in
// place. The caller must capture ownership only after this function succeeds.
func cloneRepo(source repo) (repo, error) {
	source, err := repoFromPath(source.Path)
	if err != nil {
		return repo{}, err
	}
	cloneParent := reposRoot()
	if err := os.MkdirAll(cloneParent, 0o755); err != nil {
		return repo{}, fmt.Errorf("create repository root: %w", err)
	}
	for _, component := range []string{source.Host, source.Org} {
		cloneParent = filepath.Join(cloneParent, component)
		if err := os.Mkdir(cloneParent, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return repo{}, fmt.Errorf("create clone parent %s: %w", cloneParent, err)
		}
		info, err := os.Lstat(cloneParent)
		if err != nil {
			return repo{}, fmt.Errorf("inspect clone parent %s: %w", cloneParent, err)
		}
		if !info.IsDir() {
			return repo{}, fmt.Errorf("clone parent %s is not a real directory", cloneParent)
		}
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
		if strings.ContainsAny(suffix, "/\\:\x00\r\n") {
			fmt.Println("Suffix must not contain path separators, ':', or control characters.")
			continue
		}
		destPath = filepath.Join(cloneParent, source.Name+"-"+suffix)
		if err := os.Mkdir(destPath, 0o755); err == nil {
			break
		} else if !errors.Is(err, os.ErrExist) {
			return repo{}, fmt.Errorf("reserve clone destination %s: %w", destPath, err)
		}
		fmt.Printf("%s already exists; choose another suffix.\n", destPath)
		// An EOF/blank response must not loop forever on a raced reservation.
		defaultSuffix, err = nextCloneSuffix(source)
		if err != nil {
			return repo{}, err
		}
	}

	device, inode, err := directoryIdentity(destPath)
	if err != nil {
		return repo{}, fmt.Errorf("reserved clone retained at %s: %w", destPath, err)
	}
	remote, err := gitOutput(source.Path, "config", "--get", "remote.origin.url")
	if err != nil || strings.TrimSpace(remote) == "" {
		remote = source.Path
	}
	remote = strings.TrimSpace(remote)
	// JJ performs both the Git clone and workspace initialization. The explicit
	// option also overrides a user's git.colocate=false configuration.
	if err := runCmd("jj", "git", "clone", "--colocate", "--", remote, destPath); err != nil {
		return repo{}, fmt.Errorf("clone failed; incomplete clone preserved at %s: %w", destPath, err)
	}
	cloned, err := repoFromPath(destPath)
	cloned.cloneDevice, cloned.cloneInode = device, inode
	return cloned, err
}

func nextCloneSuffix(source repo) (string, error) {
	for _, component := range []string{source.Host, source.Org, source.Name} {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, "/\\\x00") {
			return "", fmt.Errorf("invalid repository path component %q", component)
		}
	}
	for i := 2; ; i++ {
		candidate := filepath.Join(reposRoot(), source.Host, source.Org, fmt.Sprintf("%s-%d", source.Name, i))
		// Stat would mistake a dangling symlink for an available destination.
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return strconv.Itoa(i), nil
		} else if err != nil {
			return "", fmt.Errorf("check clone suffix %s: %w", candidate, err)
		}
	}
}

func repoDisplayName(r repo) string { return filepath.Join(r.Host, r.Org, r.Name) }

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
	return repo{Path: cleanPath, Rel: rel, Host: parts[0], Org: parts[1], Name: parts[2]}, nil
}

func repoRootForPath(path string) (string, bool) {
	cleanPath, err := filepath.Abs(path)
	if err != nil || !pathWithin(cleanPath, reposRoot()) {
		return "", false
	}
	rel, err := filepath.Rel(reposRoot(), cleanPath)
	if err != nil {
		return "", false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) < 3 {
		return "", false
	}
	return filepath.Join(reposRoot(), parts[0], parts[1], parts[2]), true
}

func reposRoot() string { return filepath.Join(os.Getenv("HOME"), "repos") }

// These are lexical layout helpers, not authorization to delete a directory.
// Physical-path/identity checks belong to the caller's deletion boundary.
func samePath(a, b string) bool {
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	return errA == nil && errB == nil && absA == absB
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
	return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))))
}
