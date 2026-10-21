package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf8"
)

type identity struct {
	Endpoint string `json:"endpoint"`
	Host     string `json:"host_id"`
	Instance string `json:"instance_id"`
	Account  string `json:"account"`
	Session  string `json:"session_id"`
	Window   string `json:"window_id"`
	Block    string `json:"block_id"`
}
type rexClient struct {
	identity
	Client string
	Picker bool
}
type session struct {
	ID    string `json:"session_id"`
	Label string `json:"label"`
}
type block struct {
	ID      string `json:"block_id"`
	Host    string `json:"host_id"`
	Creator string `json:"creator_name"`
}
type sessionView struct {
	session
	ActiveWindow string `json:"active_window_id"`
	Windows      []struct {
		ID string `json:"window_id"`
	} `json:"windows"`
}
type processInfo struct {
	LastExit *struct {
		Code  int    `json:"exit_code"`
		Error string `json:"error"`
	} `json:"last_exit"`
	Child *struct {
		CWD string `json:"cwd"`
	} `json:"child"`
	Foreground *struct {
		CWD string `json:"cwd"`
	} `json:"foreground"`
}

func decodeCommand(target any, name string, args ...string) error {
	out, err := outputCmd(name, args...)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(out), target); err != nil {
		return fmt.Errorf("invalid %s JSON: %w", name, err)
	}
	return nil
}
func connectRex(endpoint string) (*rexClient, error) {
	args := []string{"--autostart=false"}
	if endpoint != "" {
		args = append(args, "-S", endpoint)
	}
	var id identity
	if err := decodeCommand(&id, "rex", append(args, "whoami", "--json")...); err != nil {
		return nil, err
	}
	if err := validateLocal(id); err != nil {
		return nil, err
	}
	return &rexClient{identity: id}, nil
}
func validateLocal(id identity) error {
	u, err := url.Parse(id.Endpoint)
	if err != nil || u.Scheme != "unix" || u.Host != "" || !filepath.IsAbs(u.Path) {
		return fmt.Errorf("prex v1 requires a local Unix-socket Rex server, got %q", id.Endpoint)
	}
	info, err := os.Stat(u.Path)
	if err != nil {
		return err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	current, err := user.Current()
	if err != nil {
		return err
	}
	if !ok || info.Mode()&os.ModeSocket == 0 || stat.Uid != uint32(os.Getuid()) || id.Account != current.Username || id.Host == "" || id.Instance == "" {
		return fmt.Errorf("cannot verify Rex server belongs to this local account")
	}
	return nil
}
func (r *rexClient) args(args ...string) []string {
	return append([]string{"--autostart=false", "-S", r.Endpoint}, args...)
}
func (r *rexClient) json(out any, args ...string) error {
	return decodeCommand(out, "rex", r.args(args...)...)
}
func (r *rexClient) run(args ...string) error { return runCmd("rex", r.args(args...)...) }
func (r *rexClient) verify() error {
	now, err := connectRex(r.Endpoint)
	if err != nil {
		return err
	}
	if now.Host != r.Host || now.Instance != r.Instance {
		return fmt.Errorf("Rex server identity changed; operation stopped")
	}
	return nil
}
func (r *rexClient) selectClient(requested string) error {
	if requested == "" {
		requested = os.Getenv("REX_CLIENT")
	}
	var result struct {
		Client struct {
			ID    string `json:"client_id"`
			State string `json:"connection_state"`
			Info  struct {
				Kind string `json:"kind"`
			} `json:"info"`
		} `json:"client"`
		Actions []struct {
			Name string `json:"name"`
		} `json:"actions"`
	}
	args := []string{"client", "inspect"}
	if requested != "" {
		args = append(args, requested)
	}
	if err := r.json(&result, append(args, "--json")...); err != nil {
		return fmt.Errorf("select a connected Rex app with --client (or REX_CLIENT): %w", err)
	}
	if result.Client.ID == "" || result.Client.Info.Kind != "app" || result.Client.State == "lost" {
		return fmt.Errorf("target is not a connected Rex app")
	}
	for _, action := range result.Actions {
		if action.Name == "session.select" {
			r.Client = result.Client.ID
			return nil
		}
	}
	return fmt.Errorf("Rex app does not advertise session.select")
}
func (r *rexClient) sessions() ([]session, error) {
	var result []session
	if err := r.collection(&result, "sessions", "ls", "--json"); err != nil {
		return nil, err
	}
	for _, item := range result {
		if !strings.HasPrefix(item.ID, "session:") {
			return nil, fmt.Errorf("Rex session list has an invalid ID")
		}
	}
	return result, nil
}
func (r *rexClient) collection(out any, key string, args ...string) error {
	var envelope map[string]json.RawMessage
	if err := r.json(&envelope, args...); err != nil {
		return err
	}
	raw, ok := envelope[key]
	if !ok {
		return fmt.Errorf("Rex response omitted %s; state is unknown", key)
	}
	return json.Unmarshal(raw, out)
}
func (r *rexClient) view(id string) (sessionView, error) {
	var result sessionView
	err := r.json(&result, "session", "inspect", id, "--json")
	if err == nil && result.ID != id {
		err = fmt.Errorf("Rex returned a different or missing session ID")
	}
	return result, err
}
func (r *rexClient) blocks(id string) ([]block, error) {
	var result []block
	if err := r.collection(&result, "blocks", "block", "ls", "-s", id, "--json"); err != nil {
		return nil, err
	}
	for _, item := range result {
		if !strings.HasPrefix(item.ID, "block:") {
			return nil, fmt.Errorf("Rex block list has an invalid ID")
		}
	}
	return result, nil
}
func (r *rexClient) process(sid, bid string) (processInfo, error) {
	var p processInfo
	err := r.json(&p, "block", "call", "-s", sid, "-b", bid, "process")
	return p, err
}
func (r *rexClient) repoPaths(sid string) ([]string, error) {
	blocks, err := r.blocks(sid)
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, b := range blocks {
		if b.Host != r.Host {
			return nil, fmt.Errorf("session %s has a remote or unknown-host block", sid)
		}
		if b.Creator != "com.superlogical.terminal" {
			return nil, fmt.Errorf("cannot inspect non-terminal block %s", b.ID)
		}
		p, err := r.process(sid, b.ID)
		if err != nil {
			return nil, err
		}
		found := false
		for _, cwd := range processCWDs(p) {
			if !filepath.IsAbs(cwd) {
				continue
			}
			physical, err := filepath.EvalSymlinks(cwd)
			if err != nil {
				return nil, fmt.Errorf("cannot resolve block %s working directory: %w", b.ID, err)
			}
			found = true
			paths = append(paths, physical)
		}
		if !found && p.LastExit == nil {
			return nil, fmt.Errorf("cannot determine working directory of block %s", b.ID)
		}
	}
	return paths, nil
}

// Process CWDs are physical paths; HOME/repository roots can have OS aliases.
func physicalWithin(path, root string) bool {
	p, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	r, err := filepath.EvalSymlinks(root)
	if err != nil {
		return false
	}
	return pathWithin(p, r)
}
func processCWDs(p processInfo) []string {
	var paths []string
	if p.Child != nil {
		paths = append(paths, p.Child.CWD)
	}
	if p.Foreground != nil {
		paths = append(paths, p.Foreground.CWD)
	}
	return paths
}
func (r *rexClient) show(sid string) error { return r.showWindow(sid, "") }
func (r *rexClient) showWindow(sid, wid string) error {
	if err := r.verify(); err != nil {
		return err
	}
	args := []string{"-C", r.Client, "do", "session.select", "session_id=" + sid}
	if wid != "" {
		args = append(args, "window_id="+wid)
	}
	if _, err := outputCmd("rex", r.args(args...)...); err != nil {
		return fmt.Errorf("session %s retained; could not display it (do not recreate blindly): %w", sid, err)
	}
	return nil
}
func (r *rexClient) launch(path, label, sid string) (string, error) {
	if err := r.verify(); err != nil {
		return "", err
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		return "", fmt.Errorf("repository directory unavailable: %s", path)
	}
	args := []string{"new", label, "--window", "agent"}
	if sid != "" {
		args = []string{"window", "new", "-s", sid, "agent"}
	}
	args = append(args, "-c", path, "--json", "--", "pi")
	var result struct {
		ID      string   `json:"session_id"`
		Blocks  []string `json:"block_ids"`
		Initial []struct {
			Blocks []string `json:"block_ids"`
		} `json:"initial_windows"`
	}
	if err := r.json(&result, args...); err != nil {
		return "", fmt.Errorf("launch outcome may be unknown; intent retained for recovery: %w", err)
	}
	if sid == "" {
		sid = result.ID
	}
	if !strings.HasPrefix(sid, "session:") {
		return "", fmt.Errorf("Rex returned no full session ID; launch intent retained")
	}
	return sid, nil
}

// Rex labels are limited to 128 characters. Keep long paths recognizable by
// their repo suffix and add a stable digest, leaving room for collision suffixes.
func repoSessionLabel(r repo) string {
	label := filepath.ToSlash(filepath.Join(r.Org, r.Name))
	if len(label) <= 110 {
		return label
	}
	tail := label[len(label)-96:]
	for !utf8.ValidString(tail) {
		tail = tail[1:]
	}
	digest := sha256.Sum256([]byte(label))
	return fmt.Sprintf("…/%s-%x", tail, digest[:4])
}

func uniqueLabel(base string, sessions []session) string {
	exists := func(s string) bool {
		for _, x := range sessions {
			if x.Label == s {
				return true
			}
		}
		return false
	}
	name := base
	for i := 2; exists(name); i++ {
		name = fmt.Sprintf("%s-%d", base, i)
	}
	return name
}
func resolveSession(arg string, sessions []session) (session, error) {
	for _, s := range sessions {
		if s.ID == arg {
			return s, nil
		}
	}
	var matches []session
	for _, s := range sessions {
		if s.Label == arg {
			matches = append(matches, s)
		}
	}
	if len(matches) != 1 {
		return session{}, fmt.Errorf("session %q matches %d sessions; use a full session ID", arg, len(matches))
	}
	return matches[0], nil
}
