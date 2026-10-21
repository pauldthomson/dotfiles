package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

type association struct {
	Host     string `json:"host"`
	Instance string `json:"instance"`
	Path     string `json:"path"`
}
type cloneRecord struct {
	Host       string `json:"host"`
	Generation string `json:"generation"`
	Device     uint64 `json:"device"`
	Inode      uint64 `json:"inode"`
}
type launchIntent struct {
	Host     string `json:"host"`
	Instance string `json:"instance"`
	Path     string `json:"path"`
	Label    string `json:"label"`
	Session  string `json:"session,omitempty"`
}
type registry struct {
	Version  int                    `json:"version"`
	Sessions map[string]association `json:"sessions"`
	Clones   map[string]cloneRecord `json:"clones"`
	Pending  *launchIntent          `json:"pending,omitempty"`
}
type stateStore struct {
	Dir  string
	Lock *os.File
	Data registry
}

func stateDir() string {
	root := os.Getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(root) {
		root = filepath.Join(os.Getenv("HOME"), ".local", "state")
	}
	return filepath.Join(root, "prex")
}
func lockState(wait bool) (*stateStore, error) {
	dir := stateDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() || stat.Uid != uint32(os.Getuid()) || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("state directory must be a private, owned directory: %s", dir)
	}
	// O_NOFOLLOW prevents substituting a lock-file symlink.
	fd, err := syscall.Open(filepath.Join(dir, "lock"), syscall.O_CREAT|syscall.O_RDWR|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	lock := os.NewFile(uintptr(fd), "prex lock")
	mode := syscall.LOCK_EX
	if !wait {
		mode |= syscall.LOCK_NB
	}
	if err := syscall.Flock(fd, mode); err != nil {
		lock.Close()
		return nil, fmt.Errorf("another prex operation is in progress: %w", err)
	}
	s := &stateStore{Dir: dir, Lock: lock, Data: registry{Version: 1, Sessions: map[string]association{}, Clones: map[string]cloneRecord{}}}
	data, err := os.ReadFile(filepath.Join(dir, "registry.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &s.Data)
	}
	if err == nil && (s.Data.Version != 1 || s.Data.Sessions == nil || s.Data.Clones == nil) {
		err = fmt.Errorf("unsupported or incomplete registry")
	}
	if err != nil {
		s.Close()
		return nil, fmt.Errorf("registry unreadable; no operations performed: %w", err)
	}
	return s, nil
}
func (s *stateStore) Close() {
	if s.Lock != nil {
		_ = syscall.Flock(int(s.Lock.Fd()), syscall.LOCK_UN)
		_ = s.Lock.Close()
		s.Lock = nil
	}
}
func (s *stateStore) save() error { return atomicJSON(filepath.Join(s.Dir, "registry.json"), s.Data) }
func atomicJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".prex-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func token() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func directoryIdentity(path string) (uint64, uint64, error) {
	// Reject all symlink components, including a repository-root redirection.
	absolute, err := filepath.Abs(path)
	if err != nil {
		return 0, 0, err
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return 0, 0, err
	}
	root, err := filepath.EvalSymlinks(reposRoot())
	if err != nil {
		return 0, 0, err
	}
	lexicalRoot, err := filepath.Abs(reposRoot())
	if err != nil {
		return 0, 0, err
	}
	// Permit an OS-level alias in HOME (e.g. /var -> /private/var), but not
	// symlinks in <host>/<org>/<repo> below the repository root.
	rel, err := filepath.Rel(lexicalRoot, absolute)
	if err != nil {
		return 0, 0, err
	}
	if _, err := repoFromPath(path); err != nil {
		return 0, 0, err
	}
	if resolved != filepath.Join(root, rel) {
		return 0, 0, fmt.Errorf("refusing symlinked clone path %s", path)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return 0, 0, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || !info.IsDir() {
		return 0, 0, fmt.Errorf("not a clone directory: %s", path)
	}
	return uint64(st.Dev), uint64(st.Ino), nil
}
func (s *stateStore) ownClone(path, host string, expected ...repo) error {
	dev, ino, err := directoryIdentity(path)
	if err != nil {
		return err
	}
	if len(expected) > 0 && (expected[0].cloneDevice != dev || expected[0].cloneInode != ino) {
		return fmt.Errorf("clone directory replaced during creation; ownership not recorded")
	}
	gen, err := token()
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	admin, err := root.Lstat(".git")
	if err != nil || !admin.IsDir() {
		return fmt.Errorf("clone has no local Git administration directory")
	}
	marker, err := root.OpenFile(".git/prex-owner", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("cannot create exclusive clone ownership marker: %w", err)
	}
	_, writeErr := marker.WriteString(gen + "\n")
	if writeErr == nil {
		writeErr = marker.Sync()
	}
	closeErr := marker.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	s.Data.Clones[path] = cloneRecord{Host: host, Generation: gen, Device: dev, Inode: ino}
	return s.save()
}
func validateClone(path string, c cloneRecord, host string) error {
	if c.Host != host || c.Generation == "" || c.Inode == 0 {
		return fmt.Errorf("clone ownership is not verified for this host")
	}
	dev, ino, err := directoryIdentity(path)
	if err != nil {
		return err
	}
	if dev != c.Device || ino != c.Inode {
		return fmt.Errorf("clone directory identity changed; retaining %s", path)
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	return validateGeneration(root, c.Generation)
}
func validateGeneration(root *os.Root, generation string) error {
	admin, err := root.Lstat(".git")
	if err != nil || !admin.IsDir() {
		return fmt.Errorf("clone Git administration changed")
	}
	info, err := root.Lstat(".git/prex-owner")
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("clone ownership marker missing or invalid")
	}
	marker, err := root.Open(".git/prex-owner")
	if err != nil {
		return err
	}
	defer marker.Close()
	data, err := io.ReadAll(io.LimitReader(marker, 128))
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != generation {
		return fmt.Errorf("clone generation changed")
	}
	return nil
}
func (s *stateStore) associate(r *rexClient, id, path string) {
	s.Data.Sessions[id] = association{Host: r.Host, Instance: r.Instance, Path: path}
}
func (s *stateStore) pathFor(r *rexClient, id string) string {
	a, ok := s.Data.Sessions[id]
	if ok && a.Host == r.Host && a.Instance == r.Instance {
		return a.Path
	}
	return ""
}
