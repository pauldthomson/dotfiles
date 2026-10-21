package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	calls  [][]string
	output func([]string) (string, error)
	run    func([]string) error
}

func (f *fakeRunner) Output(name string, args ...string) (string, error) {
	a := append([]string{name}, args...)
	f.calls = append(f.calls, a)
	if f.output != nil {
		return f.output(a)
	}
	return "", fmt.Errorf("unexpected output: %v", a)
}
func (f *fakeRunner) Run(name string, args ...string) error {
	a := append([]string{name}, args...)
	f.calls = append(f.calls, a)
	if f.run != nil {
		return f.run(a)
	}
	return nil
}
func (f *fakeRunner) Silent(name string, args ...string) error { return f.Run(name, args...) }
func jsonTest(v any) string                                    { b, _ := json.Marshal(v); return string(b) }
func fakeRex(t *testing.T) (*rexClient, *fakeRunner) {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "prex-socket-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "rex.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	u := url.URL{Scheme: "unix", Path: socket}
	account, err := user.Current()
	if err != nil {
		t.Fatal(err)
	}
	r := &rexClient{identity: identity{Endpoint: u.String(), Host: "host", Instance: "instance", Account: account.Username, Session: "session:caller", Window: "window:caller"}, Client: "client:app"}
	f := &fakeRunner{}
	f.output = func(a []string) (string, error) {
		if rexTail(a)[0] == "-C" {
			return "", nil
		}
		if strings.Contains(strings.Join(a, " "), "whoami --json") {
			return jsonTest(r.identity), nil
		}
		return "", fmt.Errorf("unexpected command %v", a)
	}
	old := commands
	commands = f
	t.Cleanup(func() { commands = old })
	return r, f
}
func isolatedState(t *testing.T) *stateStore {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", filepath.Join(os.Getenv("HOME"), "state"))
	s, err := lockState(false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func projectDir(t *testing.T) string {
	t.Helper()
	path := filepath.Join(reposRoot(), "example.com", "org", "project")
	if err := os.MkdirAll(filepath.Join(path, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	return path
}
func rexTail(a []string) []string {
	if len(a) > 4 && a[0] == "rex" && a[1] == "--autostart=false" && a[2] == "-S" {
		return a[4:]
	}
	return a
}
func TestLaunchRecordsIntentAndNativeCommand(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		tail := rexTail(a)
		if len(tail) > 0 && tail[0] == "new" {
			data, err := os.ReadFile(filepath.Join(s.Dir, "registry.json"))
			if err != nil {
				t.Fatal(err)
			}
			var saved registry
			if err := json.Unmarshal(data, &saved); err != nil {
				t.Fatal(err)
			}
			if saved.Pending == nil || saved.Pending.Path != path {
				t.Fatal("intent not durable before launch")
			}
			want := []string{"new", "project", "--window", "agent", "-c", path, "--json", "--", "pi"}
			if !reflect.DeepEqual(tail, want) {
				t.Fatalf("launch %v, want %v", tail, want)
			}
			return `{"session_id":"session:new"}`, nil
		}
		return base(a)
	}
	if err := launchAndShow(r, s, path, "project", ""); err != nil {
		t.Fatal(err)
	}
	if s.Data.Pending != nil || s.pathFor(r, "session:new") != path {
		t.Fatal("session not recorded")
	}
	last := rexTail(f.calls[len(f.calls)-1])
	if !reflect.DeepEqual(last, []string{"-C", "client:app", "do", "session.select", "session_id=session:new"}) {
		t.Fatalf("wrong app switch: %v", last)
	}
}
func TestLaunchTimeoutRetainsIntentAndClone(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		tail := rexTail(a)
		if tail[0] == "new" {
			return "", errors.New("timeout")
		}
		return base(a)
	}
	if err := launchAndShow(r, s, path, "project", ""); err == nil {
		t.Fatal("expected failure")
	}
	if s.Data.Pending == nil {
		t.Fatal("lost launch intent")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("repository removed")
	}
	before := len(f.calls)
	if err := openCommand(r, s, "project"); err == nil {
		t.Fatal("retried uncertain launch")
	}
	if len(f.calls) != before {
		t.Fatal("issued RPC after uncertain launch")
	}
}
func TestSwitchFailurePreservesCreatedSession(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		if rexTail(a)[0] == "new" {
			return `{"session_id":"session:new"}`, nil
		}
		if rexTail(a)[0] == "-C" {
			return "", errors.New("app disconnected")
		}
		return base(a)
	}
	if err := launchAndShow(r, s, path, "project", ""); err == nil {
		t.Fatal("expected switch failure")
	}
	if s.Data.Pending != nil || s.pathFor(r, "session:new") != path {
		t.Fatal("lost successful creation")
	}
	for _, a := range f.calls {
		if rexTail(a)[0] == "kill" {
			t.Fatal("rolled back live session")
		}
	}
}
func TestExistingSessionLifecycle(t *testing.T) {
	for _, nonempty := range []bool{false, true} {
		t.Run(fmt.Sprint(nonempty), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			base := f.output
			launched := false
			f.output = func(a []string) (string, error) {
				tail := rexTail(a)
				switch strings.Join(tail, " ") {
				case "session inspect session:old --json":
					if nonempty {
						return `{"session_id":"session:old","windows":[{"window_id":"window:existing"}]}`, nil
					}
					return `{"session_id":"session:old"}`, nil
				case "block ls -s session:old --json":
					return `{"blocks":[]}`, nil
				}
				if len(tail) > 1 && tail[0] == "window" && tail[1] == "new" {
					launched = true
					return `{"window_id":"window:new","block_ids":["block:new"]}`, nil
				}
				return base(a)
			}
			if err := openExisting(r, s, session{ID: "session:old", Label: "renamed"}, path); err != nil {
				t.Fatal(err)
			}
			if launched == nonempty {
				t.Fatalf("launched=%v nonempty=%v", launched, nonempty)
			}
			if s.pathFor(r, "session:old") != path {
				t.Fatal("association missing")
			}
		})
	}
}
func TestExactSessionResolution(t *testing.T) {
	sessions := []session{{ID: "session:a", Label: "same"}, {ID: "session:b", Label: "same"}, {ID: "session:c", Label: "same-more"}}
	for _, name := range []string{"same", "sam", "missing"} {
		if _, err := resolveSession(name, sessions); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	target, err := resolveSession("session:a", sessions)
	if err != nil || target.ID != "session:a" {
		t.Fatalf("%v %v", target, err)
	}
}
func TestClientSelectionCapabilityAndAmbiguity(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		fail           bool
	}{
		{"app", `{"client":{"client_id":"client:full","info":{"kind":"app"},"connection_state":"connected"},"actions":[{"name":"session.select"}]}`, false},
		{"lost", `{"client":{"client_id":"client:full","info":{"kind":"app"},"connection_state":"lost"},"actions":[{"name":"session.select"}]}`, true},
		{"no capability", `{"client":{"client_id":"client:full","info":{"kind":"app"}}}`, true},
		{"CLI", `{"client":{"client_id":"client:full","info":{"kind":"cli"}},"actions":[{"name":"session.select"}]}`, true},
		{"ambiguous", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, f := fakeRex(t)
			f.output = func(a []string) (string, error) {
				if tc.response == "" {
					return "", errors.New("ambiguous")
				}
				return tc.response, nil
			}
			err := r.selectClient("explicit")
			if (err != nil) != tc.fail {
				t.Fatalf("err=%v", err)
			}
			if !tc.fail && r.Client != "client:full" {
				t.Fatal("not pinned to full ID")
			}
		})
	}
}
func TestRemoteServerRejected(t *testing.T) {
	if err := validateLocal(identity{Endpoint: "https://example.com"}); err == nil {
		t.Fatal("accepted remote server")
	}
	if err := validateLocal(identity{Endpoint: "unix://remote/tmp/rex.sock"}); err == nil {
		t.Fatal("accepted remote authority")
	}
}
func TestUnknownBlockHostNeverAssociatesLocalPath(t *testing.T) {
	r, f := fakeRex(t)
	f.output = func(a []string) (string, error) {
		return `{"blocks":[{"block_id":"block:x","host_id":"remote","creator_name":"com.superlogical.terminal"}]}`, nil
	}
	if _, err := r.repoPaths("session:x"); err == nil {
		t.Fatal("accepted remote block")
	}
}
func TestSharedManagedAndUnmanagedSessions(t *testing.T) {
	for _, managed := range []bool{true, false} {
		t.Run(fmt.Sprint(managed), func(t *testing.T) {
			s := isolatedState(t)
			path := projectDir(t)
			r, f := fakeRex(t)
			if managed {
				s.associate(r, "session:other", path)
			}
			f.output = func(a []string) (string, error) {
				tail := strings.Join(rexTail(a), " ")
				switch tail {
				case "ls --json":
					return `{"sessions":[{"session_id":"session:other"}]}`, nil
				case "block ls -s session:other --json":
					return `{"blocks":[{"block_id":"block:other","host_id":"host","creator_name":"com.superlogical.terminal"}]}`, nil
				case "block call -s session:other -b block:other process":
					return jsonTest(map[string]any{"foreground": map[string]string{"cwd": filepath.Join(path, "subdir")}}), nil
				}
				return "", fmt.Errorf("unexpected %s", tail)
			}
			if err := checkShared(r, s, path, ""); err == nil {
				t.Fatal("accepted shared clone")
			}
		})
	}
}
func TestPendingRecoveryAssociatesWithoutRelaunch(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	s.Data.Pending = &launchIntent{Host: r.Host, Instance: r.Instance, Path: path, Label: "project"}
	f.output = func(a []string) (string, error) {
		switch strings.Join(rexTail(a), " ") {
		case "ls --json":
			return `{"sessions":[{"session_id":"session:recovered","label":"project"}]}`, nil
		case "block ls -s session:recovered --json":
			return `{"blocks":[{"block_id":"block:recovered","host_id":"host","creator_name":"com.superlogical.terminal"}]}`, nil
		case "block call -s session:recovered -b block:recovered process":
			return jsonTest(map[string]any{"child": map[string]string{"cwd": path}}), nil
		}
		return "", fmt.Errorf("unexpected %v", a)
	}
	if err := recoverCommand(r, s); err != nil {
		t.Fatal(err)
	}
	if s.Data.Pending != nil || s.pathFor(r, "session:recovered") != path {
		t.Fatal("not recovered")
	}
}
func TestRegistryLockCorruptionAndStaleInstance(t *testing.T) {
	s := isolatedState(t)
	if other, err := lockState(false); err == nil {
		other.Close()
		t.Fatal("concurrent lock allowed")
	}
	r := &rexClient{identity: identity{Host: "host", Instance: "old"}}
	s.associate(r, "session:x", "/project")
	r.Instance = "new"
	if s.pathFor(r, "session:x") != "" {
		t.Fatal("stale association reused")
	}
	if err := os.WriteFile(filepath.Join(s.Dir, "registry.json"), []byte("corrupt"), 0600); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if reopened, err := lockState(false); err == nil {
		reopened.Close()
		t.Fatal("corruption silently replaced")
	}
}
func TestCloneIdentityReplacementAndSymlinks(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	if err := s.ownClone(path, "host"); err != nil {
		t.Fatal(err)
	}
	c := s.Data.Clones[path]
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedClone(path, c, "host"); err == nil {
		t.Fatal("removed replacement")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path+"-old", path); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedClone(path, c, "host"); err == nil {
		t.Fatal("removed symlink")
	}
	if _, err := os.Stat(path + "-old"); err != nil {
		t.Fatal("original lost")
	}
}
func TestOwnedCloneRemovalConfined(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	if err := s.ownClone(path, "host"); err != nil {
		t.Fatal(err)
	}
	if err := removeOwnedClone(path, s.Data.Clones[path], "host"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("clone remains")
	}
	if err := removeOwnedClone(reposRoot(), cloneRecord{}, "host"); err == nil {
		t.Fatal("accepted root")
	}
}
func TestCleanupCurrentSessionOnlyAndSingleUseJob(t *testing.T) {
	s := isolatedState(t)
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		tail := rexTail(a)
		if strings.Join(tail, " ") == "ls --json" {
			return `{"sessions":[{"session_id":"session:caller","label":"caller"},{"session_id":"session:other","label":"other"}]}`, nil
		}
		if len(tail) > 1 && tail[0] == "block" && tail[1] == "ls" {
			return `{"blocks":[]}`, nil
		}
		return base(a)
	}
	old := startDetachedProcess
	t.Cleanup(func() { startDetachedProcess = old })
	var jobID string
	startDetachedProcess = func(name string, args ...string) error {
		if len(args) != 2 || args[0] != "_cleanup" {
			t.Fatalf("bad detach %v", args)
		}
		jobID = args[1]
		return nil
	}
	if err := killCommand(r, s, nil); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(s.Dir, jobID+".job.json"))
	if err != nil {
		t.Fatal(err)
	}
	var job cleanupJob
	if err := json.Unmarshal(data, &job); err != nil {
		t.Fatal(err)
	}
	if len(job.Tasks) != 1 || job.Tasks[0].Session != "session:caller" {
		t.Fatalf("must close only the current session: %v", job.Tasks)
	}
	// Once a worker claims a job it cannot be replayed, even after failure.
	if err := os.Rename(filepath.Join(s.Dir, jobID+".job.json"), filepath.Join(s.Dir, jobID+".running.json")); err != nil {
		t.Fatal(err)
	}
	if err := executeJob(r, s, jobID, job); err == nil {
		t.Fatal("replayed claimed job")
	}
	for _, call := range f.calls {
		if rexTail(call)[0] == "kill" {
			t.Fatal("parent killed a session before worker")
		}
	}
}
func TestPendingIntentCanBeAbandonedAfterRestart(t *testing.T) {
	s := isolatedState(t)
	r, f := fakeRex(t)
	s.Data.Pending = &launchIntent{Host: r.Host, Instance: "old", Path: "/retained", Label: "old"}
	safetyInput(t, "y\n")
	if err := recoverCommand(r, s); err != nil {
		t.Fatal(err)
	}
	if s.Data.Pending != nil || len(f.calls) != 0 {
		t.Fatal("abandon should clear intent without Rex operations")
	}
}
func TestQuarantinePreservesRacedReplacement(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	if err := s.ownClone(path, "host"); err != nil {
		t.Fatal(err)
	}
	c := s.Data.Clones[path]
	err := removeOwnedClone(path, c, "host", func() {
		if err := os.Rename(path, path+"-original"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(path, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "valuable"), []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
	})
	if err == nil || !strings.Contains(err.Error(), "nothing deleted") {
		t.Fatalf("err=%v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".prex-removing-*", "repo", "valuable"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("replacement not retained: %v %v", matches, err)
	}
	if _, err := os.Stat(path + "-original"); err != nil {
		t.Fatal("original clone lost")
	}
}
func TestMalformedListsAreNotEmptyState(t *testing.T) {
	r, f := fakeRex(t)
	f.output = func([]string) (string, error) { return `{}`, nil }
	if _, err := r.sessions(); err == nil {
		t.Fatal("missing sessions accepted as empty")
	}
	if _, err := r.blocks("session:x"); err == nil {
		t.Fatal("missing blocks accepted as empty")
	}
}

// Real subprocess regression: the launching parent exits without unlocking.
// The detached child must not inherit a descriptor that keeps the lock alive.
func TestLockSurvivesAbruptParentExit(t *testing.T) {
	if mode := os.Getenv("PREX_TEST_LOCK_CHILD"); mode != "" {
		s, err := lockState(true)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		if mode == "worker" {
			err := os.WriteFile(filepath.Join(s.Dir, "worker-acquired"), []byte("ok"), 0600)
			s.Close()
			if err != nil {
				os.Exit(3)
			}
			os.Exit(0)
		}
		cmd := exec.Command(os.Args[0], "-test.run=^TestLockSurvivesAbruptParentExit$")
		cmd.Env = append(os.Environ(), "PREX_TEST_LOCK_CHILD=worker")
		if err := cmd.Start(); err != nil {
			os.Exit(4)
		}
		// Do not call Close: model SIGKILL/terminal death after spawning.
		os.Exit(0)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", filepath.Join(os.Getenv("HOME"), "state"))
	cmd := exec.Command(os.Args[0], "-test.run=^TestLockSurvivesAbruptParentExit$")
	cmd.Env = append(os.Environ(), "PREX_TEST_LOCK_CHILD=parent")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("parent: %v %s", err, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(stateDir(), "worker-acquired")); err == nil {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("worker blocked on inherited lock after parent exit")
}

func TestMissingCleanupTargetNeverRemovesClone(t *testing.T) {
	s := isolatedState(t)
	path := projectDir(t)
	r, f := fakeRex(t)
	base := f.output
	if err := s.ownClone(path, r.Host); err != nil {
		t.Fatal(err)
	}
	f.output = func(a []string) (string, error) {
		if strings.Join(rexTail(a), " ") == "ls --json" {
			return `{"sessions":[]}`, nil
		}
		return base(a)
	}
	c := s.Data.Clones[path]
	if err := executeCleanup(r, s, cleanupTask{Session: "session:gone", Path: path, Clone: &c, Remove: true}); err == nil {
		t.Fatal("accepted absent target")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("deleted clone")
	}
}
