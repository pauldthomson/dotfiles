package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestProjectPickerTabUsesNativeWindowAndOrigin(t *testing.T) {
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		if rexTail(a)[0] == "window" {
			return `{"window_id":"window:new-picker"}`, nil
		}
		return base(a)
	}
	if err := pickerTab(r, []string{"tab"}); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 3 {
		t.Fatalf("unexpected calls: %v", f.calls)
	}
	args := rexTail(f.calls[0])
	joined := strings.Join(args, " ")
	for _, part := range []string{"window new -s session:caller Projects", "--client client:app", "--_picker --_return-window window:caller"} {
		if !strings.Contains(joined, part) {
			t.Errorf("missing %q in %s", part, joined)
		}
	}
	if strings.Contains(joined, "new_layer") {
		t.Fatal("picker still uses floating layer")
	}
	if strings.Contains(joined, "--interactive") {
		t.Fatal("picker should retain the login-shell environment")
	}
	selected := strings.Join(rexTail(f.calls[2]), " ")
	if selected != "-C client:app do session.select session_id=session:caller window_id=window:new-picker" {
		t.Fatalf("app did not select returned picker tab: %s", selected)
	}
}
func TestProjectPickerDefaultsToFloatingLayer(t *testing.T) {
	r, f := fakeRex(t)
	base := f.output
	f.output = func(a []string) (string, error) {
		args := rexTail(a)
		if args[0] == "api" {
			if len(args) != 6 || args[4] != "session.new_layer" {
				t.Fatalf("unexpected creation: %v", args)
			}
			var request struct {
				Session string `json:"session_id"`
				Window  string `json:"window_id"`
				Focus   bool   `json:"focus"`
				Layout  struct {
					Block struct {
						Options struct {
							Command []string `json:"command"`
						} `json:"options"`
					} `json:"block"`
				} `json:"layout"`
			}
			if err := json.Unmarshal([]byte(args[5]), &request); err != nil {
				t.Fatal(err)
			}
			if request.Session != r.Session || request.Window != r.Window || !request.Focus {
				t.Fatal("picker does not overlay the caller's tab")
			}
			if !strings.Contains(strings.Join(request.Layout.Block.Options.Command, " "), "--client client:app --_picker") {
				t.Fatal("missing picker/app routing")
			}
			return `{"layer_id":"layer:picker","block_ids":["block:picker"]}`, nil
		}
		return base(a)
	}
	if err := pickerTab(r, nil); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 3 || !strings.Contains(strings.Join(rexTail(f.calls[2]), " "), "window_id=window:caller") {
		t.Fatal("floating picker did not select its source tab")
	}
}

func TestFloatingPickerUncertainCreationNeverFallsBack(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprint(fail), func(t *testing.T) {
			r, f := fakeRex(t)
			f.output = func([]string) (string, error) {
				if fail {
					return "", fmt.Errorf("connection lost")
				}
				return "{}", nil
			}
			if err := pickerTab(r, nil); err == nil {
				t.Fatal("accepted unknown creation outcome")
			}
			if len(f.calls) != 1 {
				t.Fatal("retried or selected an unverified picker")
			}
		})
	}
}

func TestPickerMissingTabIDDoesNotSelectApp(t *testing.T) {
	r, f := fakeRex(t)
	f.output = func([]string) (string, error) { return `{}`, nil }
	if err := pickerTab(r, []string{"tab"}); err == nil {
		t.Fatal("accepted missing tab ID")
	}
	if len(f.calls) != 1 {
		t.Fatal("attempted to select an unknown tab")
	}
}
func TestPickerStartupSelectsItsOwnAppTab(t *testing.T) {
	r, f := fakeRex(t)
	r.Window = "window:close-picker"
	base := f.output
	f.output = func(a []string) (string, error) {
		if strings.Join(rexTail(a), " ") == "client inspect client:app --json" {
			return `{"client":{"client_id":"client:app","info":{"kind":"app"},"connection_state":"connected"},"actions":[{"name":"session.select"}]}`, nil
		}
		return base(a)
	}
	if err := activatePicker(r, "client:app"); err != nil {
		t.Fatal(err)
	}
	last := strings.Join(rexTail(f.calls[len(f.calls)-1]), " ")
	if last != "-C client:app do session.select session_id=session:caller window_id=window:close-picker" {
		t.Fatalf("picker startup didn't select its own tab: %s", last)
	}
}
func TestPickerStartupRefusesMissingTab(t *testing.T) {
	r, f := fakeRex(t)
	r.Window = ""
	if err := activatePicker(r, "client:app"); err == nil {
		t.Fatal("accepted unknown picker tab")
	}
	if len(f.calls) != 0 {
		t.Fatal("performed app action without a known picker tab")
	}
}

func TestPickerRestoresOriginWithoutChangingAppSession(t *testing.T) {
	for _, tc := range []struct {
		name, active            string
		originExists, wantFocus bool
	}{
		{"cancel", "window:picker", true, true},
		{"new agent focused", "window:agent", true, false},
		{"source tab closed", "window:picker", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, f := fakeRex(t)
			r.Window = "window:picker"
			f.output = func(a []string) (string, error) {
				if strings.Join(rexTail(a), " ") != "session inspect session:caller --json" {
					return "", fmt.Errorf("unexpected %v", a)
				}
				windows := []map[string]string{{"window_id": "window:picker"}}
				if tc.originExists {
					windows = append(windows, map[string]string{"window_id": "window:origin"})
				}
				return jsonTest(map[string]any{"session_id": r.Session, "active_window_id": tc.active, "windows": windows}), nil
			}
			restorePickerWindow(r, "window:origin")
			focused := false
			for _, call := range f.calls {
				args := strings.Join(rexTail(call), " ")
				if args == "focus -s session:caller window:origin" {
					focused = true
				}
				if strings.Contains(args, "session.select") {
					t.Fatal("restoration switched app session")
				}
			}
			if focused != tc.wantFocus {
				t.Fatalf("focused=%v want %v", focused, tc.wantFocus)
			}
		})
	}
}
