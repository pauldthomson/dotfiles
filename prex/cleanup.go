package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
)

type cleanupTask struct {
	Session string       `json:"session"`
	Path    string       `json:"path,omitempty"`
	Clone   *cloneRecord `json:"clone,omitempty"`
	Remove  bool         `json:"remove"`
}
type cleanupJob struct {
	Endpoint string        `json:"endpoint"`
	Host     string        `json:"host"`
	Instance string        `json:"instance"`
	Tasks    []cleanupTask `json:"tasks"`
}
type cleanupResult struct {
	State     string   `json:"state"`
	Completed []string `json:"completed"`
	Error     string   `json:"error,omitempty"`
}

func killCommand(r *rexClient, s *stateStore, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: prex kill (closes only the current Rex session; no targets accepted)")
	}
	if !strings.HasPrefix(r.Session, "session:") {
		return fmt.Errorf("prex kill must run inside a Rex session; no current session was identified")
	}
	if s.Data.Pending != nil {
		return fmt.Errorf("resolve the pending launch with prex recover before closing the current session")
	}
	sessions, err := r.sessions()
	if err != nil {
		return err
	}
	var current session
	for _, candidate := range sessions {
		if candidate.ID == r.Session {
			current = candidate
			break
		}
	}
	if current.ID == "" {
		return fmt.Errorf("current Rex session no longer exists; nothing closed")
	}
	task, err := prepareCleanup(r, s, current)
	if err != nil {
		return err
	}
	job := cleanupJob{Endpoint: r.Endpoint, Host: r.Host, Instance: r.Instance, Tasks: []cleanupTask{task}}
	id, err := token()
	if err != nil {
		return err
	}
	if err := atomicJSON(filepath.Join(s.Dir, id+".job.json"), job); err != nil {
		return err
	}
	if err := atomicJSON(filepath.Join(s.Dir, id+".result.json"), cleanupResult{State: "queued"}); err != nil {
		return err
	}
	// The target is always our own session. An independent worker must finish
	// after Rex closes this terminal, whether or not a clone is being removed.
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	fmt.Printf("Closing %s; results: %s\n", sessionDisplay(current), filepath.Join(s.Dir, id+".result.json"))
	return startDetachedProcess(exe, "_cleanup", id)
}
func prepareCleanup(r *rexClient, s *stateStore, target session) (cleanupTask, error) {
	task := cleanupTask{Session: target.ID}
	paths, err := r.repoPaths(target.ID)
	if err != nil {
		return task, err
	}
	if path := s.pathFor(r, target.ID); path != "" {
		paths = append(paths, path)
	}
	// Discovery may identify an already-owned clone, but never creates ownership.
	owned := map[string]cloneRecord{}
	for path, c := range s.Data.Clones {
		if c.Host != r.Host {
			continue
		}
		for _, cwd := range paths {
			if physicalWithin(cwd, path) {
				owned[path] = c
			}
		}
	}
	if len(owned) > 1 {
		return task, fmt.Errorf("%s uses multiple owned clones; close its extra tabs before cleanup", target.Label)
	}
	for path, c := range owned {
		if err := validateClone(path, c, r.Host); err != nil {
			return task, err
		}
		if ready, reason := repoReadyForRemoval(path); !ready {
			return task, fmt.Errorf("not closing %s: %s", target.Label, reason)
		}
		task.Path = path
		task.Clone = &c
		remove, err := confirm(fmt.Sprintf("Remove owned clone %s after closing %s? All other writers (including other Rex servers) must be stopped. Ignored files block removal. [y/N]: ", path, target.Label))
		if err != nil {
			return task, err
		}
		task.Remove = remove
	}
	return task, nil
}

var jobIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func runCleanupJob(id string) error {
	if !jobIDPattern.MatchString(id) {
		return fmt.Errorf("invalid cleanup job token")
	}
	s, err := lockState(true)
	if err != nil {
		return err
	}
	defer s.Close()
	data, err := os.ReadFile(filepath.Join(s.Dir, id+".job.json"))
	if err != nil {
		return err
	}
	var job cleanupJob
	if err := json.Unmarshal(data, &job); err != nil {
		return err
	}
	r, err := connectRex(job.Endpoint)
	if err != nil {
		_ = atomicJSON(filepath.Join(s.Dir, id+".result.json"), cleanupResult{State: "failed", Error: err.Error()})
		return err
	}
	return executeJob(r, s, id, job)
}
func executeJob(r *rexClient, s *stateStore, id string, job cleanupJob) error {
	if r.Host != job.Host || r.Instance != job.Instance {
		err := fmt.Errorf("cleanup job's server identity changed; nothing closed or removed")
		return errors.Join(err, atomicJSON(filepath.Join(s.Dir, id+".result.json"), cleanupResult{State: "failed", Error: err.Error()}))
	}
	// Claim once. A crash leaves a running job for manual inspection, never replay.
	if err := os.Rename(filepath.Join(s.Dir, id+".job.json"), filepath.Join(s.Dir, id+".running.json")); err != nil {
		return err
	}
	result := cleanupResult{State: "running", Completed: []string{}}
	resultPath := filepath.Join(s.Dir, id+".result.json")
	if err := atomicJSON(resultPath, result); err != nil {
		return err
	}
	for _, task := range job.Tasks {
		if err := executeCleanup(r, s, task); err != nil {
			result.State = "failed"
			result.Error = err.Error()
			return errors.Join(err, atomicJSON(resultPath, result))
		}
		result.Completed = append(result.Completed, task.Session)
		if err := atomicJSON(resultPath, result); err != nil {
			return err
		}
	}
	result.State = "complete"
	if err := atomicJSON(resultPath, result); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(s.Dir, id+".running.json")); err != nil {
		return err
	}
	fmt.Println("Cleanup complete; results:", resultPath)
	return nil
}
func executeCleanup(r *rexClient, s *stateStore, task cleanupTask) error {
	if err := r.verify(); err != nil {
		return err
	}
	sessions, err := r.sessions()
	if err != nil {
		return err
	}
	if _, err := resolveSession(task.Session, sessions); err != nil {
		return fmt.Errorf("target disappeared; no clone will be removed: %w", err)
	}
	if task.Clone != nil {
		current, ok := s.Data.Clones[task.Path]
		if !ok || current != *task.Clone {
			return fmt.Errorf("clone ownership changed; retaining %s", task.Path)
		}
		if err := validateClone(task.Path, current, r.Host); err != nil {
			return err
		}
		if s.Data.Pending != nil {
			return fmt.Errorf("launch pending; refusing cleanup")
		}
		if ready, reason := repoReadyForRemoval(task.Path); !ready {
			return fmt.Errorf("not closing %s; clone changed: %s", task.Session, reason)
		}
		// Revalidate association as well as identity after confirmation/detachment.
		paths, err := r.repoPaths(task.Session)
		if err != nil {
			return err
		}
		paths = append(paths, s.pathFor(r, task.Session))
		associated := false
		for _, path := range paths {
			if path != "" && physicalWithin(path, task.Path) {
				associated = true
			}
		}
		if !associated {
			return fmt.Errorf("session no longer points to the confirmed clone")
		}
		if task.Remove {
			if err := checkShared(r, s, task.Path, task.Session); err != nil {
				return err
			}
		}
	}
	if err := r.run("kill", task.Session); err != nil {
		return fmt.Errorf("session-close outcome uncertain; clone retained: %w", err)
	}
	remaining, err := r.sessions()
	if err != nil {
		return fmt.Errorf("cannot confirm closure; clone retained: %w", err)
	}
	for _, item := range remaining {
		if item.ID == task.Session {
			return fmt.Errorf("session still exists; clone retained")
		}
	}
	delete(s.Data.Sessions, task.Session)
	if err := s.save(); err != nil {
		return err
	}
	if !task.Remove {
		return nil
	}
	if task.Clone == nil {
		return fmt.Errorf("missing clone authorization")
	}
	if err := checkShared(r, s, task.Path, ""); err != nil {
		return fmt.Errorf("session closed; clone retained: %w", err)
	}
	if err := noOpenFiles(task.Path); err != nil {
		return fmt.Errorf("session closed; clone retained: %w", err)
	}
	if ready, reason := repoReadyForRemoval(task.Path); !ready {
		return fmt.Errorf("session closed; clone retained: %s", reason)
	}
	if err := removeOwnedClone(task.Path, *task.Clone, r.Host); err != nil {
		return fmt.Errorf("session closed; clone retained: %w", err)
	}
	delete(s.Data.Clones, task.Path)
	return s.save()
}
func checkShared(r *rexClient, s *stateStore, path, except string) error {
	sessions, err := r.sessions()
	if err != nil {
		return err
	}
	for _, item := range sessions {
		if item.ID == except {
			continue
		}
		if recorded := s.pathFor(r, item.ID); recorded != "" && physicalWithin(recorded, path) {
			return fmt.Errorf("clone is shared with session %s", item.ID)
		}
		paths, err := r.repoPaths(item.ID)
		if err != nil {
			return fmt.Errorf("cannot rule out shared clone use: %w", err)
		}
		for _, cwd := range paths {
			if physicalWithin(cwd, path) {
				return fmt.Errorf("clone is used by session %s", item.ID)
			}
		}
	}
	return nil
}

var noOpenFiles = checkOpenFiles

func checkOpenFiles(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "lsof", "-nP", "-F", "pcn", "+D", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if strings.TrimSpace(stderr.String()) != "" {
		return fmt.Errorf("cannot inspect open files: %s", strings.TrimSpace(stderr.String()))
	}
	if len(bytes.TrimSpace(out)) > 0 {
		return fmt.Errorf("processes still have files or working directories open in %s; stop them and remove manually", path)
	}
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return err
	}
	return nil
}
func removeOwnedClone(path string, c cloneRecord, host string, beforeMove ...func()) error {
	if err := validateClone(path, c, host); err != nil {
		return err
	}
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	id, err := token()
	if err != nil {
		return err
	}
	name := ".prex-removing-" + id
	if err := parent.Mkdir(name, 0700); err != nil {
		return err
	}
	quarantine, err := parent.OpenRoot(name)
	if err != nil {
		return err
	}
	defer quarantine.Close()
	// Test seam for an ordinary concurrent replacement after validation.
	for _, hook := range beforeMove {
		hook()
	}
	if err := parent.Rename(filepath.Base(path), filepath.Join(name, "repo")); err != nil {
		_ = parent.Remove(name)
		return err
	}
	retained := filepath.Join(filepath.Dir(path), name, "repo")
	info, err := quarantine.Lstat("repo")
	if err != nil {
		return fmt.Errorf("quarantined clone retained at %s: %w", retained, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || uint64(st.Dev) != c.Device || uint64(st.Ino) != c.Inode {
		return fmt.Errorf("directory changed during removal; moved entry retained at %s (nothing deleted)", retained)
	}
	moved, err := quarantine.OpenRoot("repo")
	if err != nil {
		return fmt.Errorf("quarantined clone retained at %s: %w", retained, err)
	}
	err = validateGeneration(moved, c.Generation)
	_ = moved.Close()
	if err != nil {
		return fmt.Errorf("quarantined clone retained at %s: %w", retained, err)
	}
	// Delete the verified entry in a private same-filesystem quarantine, never
	// a replacement subsequently installed at the original repository path.
	if err := quarantine.RemoveAll("repo"); err != nil {
		return fmt.Errorf("cleanup incomplete at %s: %w", retained, err)
	}
	return parent.Remove(name)
}
