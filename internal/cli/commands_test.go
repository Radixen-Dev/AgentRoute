// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
)

// withFakeClaudeAdapter redirects every command's claude-code adapter
// resolution (doctor, down, link, unlink — see platforms.go) to a temp
// settings.json for the duration of the test, never touching a real
// ~/.claude/settings.json.
func withFakeClaudeAdapter(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "settings.json")
	orig := newClaudeCodeAdapter
	newClaudeCodeAdapter = func() platform.Platform { return &claudecode.Adapter{SettingsPath: path} }
	t.Cleanup(func() { newClaudeCodeAdapter = orig })
	return path
}

// execCmd runs root with args, capturing stdout/stderr, and returns the
// error Execute produced (nil on success).
func execCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := New()
	var out, errBuf bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return out.String(), errBuf.String(), err
}

func exitCodeOf(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return ExitOK
	}
	var ec interface{ ExitCode() int }
	if errors.As(err, &ec) {
		return ec.ExitCode()
	}
	return ExitGeneric
}

func TestVersionCmdTextAndJSON(t *testing.T) {
	out, _, err := execCmd(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "agentroute") {
		t.Fatalf("got %q, want it to mention agentroute", out)
	}

	out, _, err = execCmd(t, "version", "--json")
	if err != nil {
		t.Fatalf("version --json: %v", err)
	}
	var doc map[string]string
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v (%q)", err, out)
	}
	if doc["version"] == "" {
		t.Fatalf("got %v, want a version field", doc)
	}
}

func TestKeyStatusReflectsEnvVar(t *testing.T) {
	withIsolatedState(t)
	withFakeOpenRouterKey(t, "sk-test")

	out, _, err := execCmd(t, "key", "status", "--json")
	if err != nil {
		t.Fatalf("key status: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["configured"] != true || doc["source"] != "env" {
		t.Fatalf("got %v, want configured=true source=env", doc)
	}
}

func TestKeySetRequiresValueOrStdin(t *testing.T) {
	// No keyring is touched here: this must fail validation before ever
	// calling secret.SetOpenRouterAPIKey.
	_, _, err := execCmd(t, "key", "set")
	if exitCodeOf(t, err) != ExitUsage {
		t.Fatalf("got exit code %d, want %d (ExitUsage); err=%v", exitCodeOf(t, err), ExitUsage, err)
	}

	_, _, err = execCmd(t, "key", "set", "--value", "x", "--stdin")
	if exitCodeOf(t, err) != ExitUsage {
		t.Fatalf("--value + --stdin: got exit code %d, want %d; err=%v", exitCodeOf(t, err), ExitUsage, err)
	}
}

func TestProfilesCreateListActivateDelete(t *testing.T) {
	withIsolatedState(t)

	_, _, err := execCmd(t, "profiles", "create", "work",
		"--heavy", "openrouter/a", "--balanced", "openrouter/b", "--fast", "openrouter/c")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	out, _, err := execCmd(t, "profiles", "list", "--json")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, `"name":"work"`) {
		t.Fatalf("got %q, want it to list the work profile", out)
	}

	_, _, err = execCmd(t, "profiles", "activate", "work")
	if err != nil {
		t.Fatalf("activate: %v", err)
	}
	out, _, err = execCmd(t, "profiles", "list", "--json")
	if err != nil {
		t.Fatalf("list after activate: %v", err)
	}
	if !strings.Contains(out, `"active":true`) {
		t.Fatalf("got %q, want active:true after activation", out)
	}

	_, _, err = execCmd(t, "profiles", "activate", "does-not-exist")
	if exitCodeOf(t, err) != ExitUsage {
		t.Fatalf("activate unknown profile: got exit %d, want %d", exitCodeOf(t, err), ExitUsage)
	}

	_, _, err = execCmd(t, "profiles", "delete", "work")
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	out, _, err = execCmd(t, "profiles", "list")
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if !strings.Contains(out, "no profiles saved") {
		t.Fatalf("got %q, want no profiles after delete", out)
	}
}

func TestProfilesCreateRequiresAtLeastOneTier(t *testing.T) {
	withIsolatedState(t)
	_, _, err := execCmd(t, "profiles", "create", "empty")
	if exitCodeOf(t, err) != ExitUsage {
		t.Fatalf("got exit %d, want %d; err=%v", exitCodeOf(t, err), ExitUsage, err)
	}
}

func TestDoctorJSONReportsAllChecks(t *testing.T) {
	withIsolatedState(t)
	withFakeClaudeAdapter(t)
	withFakeOpenRouterKey(t, "sk-test")

	out, _, err := execCmd(t, "doctor", "--json")
	// litellm/claude binaries are very unlikely to be on the test
	// runner's PATH, so doctor is expected to report ExitGeneric (1) —
	// what we're verifying is the *shape* of the report, not that every
	// check passes on a bare CI box.
	if err != nil && exitCodeOf(t, err) != ExitGeneric {
		t.Fatalf("doctor: got exit %d, want 0 or %d; err=%v", exitCodeOf(t, err), ExitGeneric, err)
	}

	var checks []doctorCheck
	if jerr := json.Unmarshal([]byte(out), &checks); jerr != nil {
		t.Fatalf("unmarshal: %v (%q)", jerr, out)
	}
	names := map[string]bool{}
	for _, c := range checks {
		names[c.Name] = true
	}
	for _, want := range []string{"openrouter-key", "litellm", "claude-code", "gateway-port"} {
		if !names[want] {
			t.Errorf("missing doctor check %q in %v", want, checks)
		}
	}

	var keyCheck doctorCheck
	for _, c := range checks {
		if c.Name == "openrouter-key" {
			keyCheck = c
		}
	}
	if !keyCheck.OK {
		t.Fatalf("openrouter-key check should pass when OPENROUTER_API_KEY is set: %+v", keyCheck)
	}
}

func TestModelsAgainstFakeServer(t *testing.T) {
	withIsolatedState(t)
	withFakeOpenRouterKey(t, "sk-test")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing/wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"openrouter/b","name":"B","context_length":8000},{"id":"openrouter/a","name":"A","context_length":4000}]}`))
	}))
	defer srv.Close()

	orig := newOpenRouterClient
	newOpenRouterClient = func(apiKey string) *openrouter.Client {
		c := openrouter.NewClient(apiKey)
		c.BaseURL = srv.URL
		return c
	}
	t.Cleanup(func() { newOpenRouterClient = orig })

	out, _, err := execCmd(t, "models", "--json")
	if err != nil {
		t.Fatalf("models: %v", err)
	}
	var models []openrouter.Model
	if err := json.Unmarshal([]byte(out), &models); err != nil {
		t.Fatalf("unmarshal: %v (%q)", err, out)
	}
	if len(models) != 2 || models[0].ID != "openrouter/a" {
		t.Fatalf("got %+v, want 2 models sorted by id", models)
	}
}

func TestModelsNoKeyConfiguredReturnsExitMissingKey(t *testing.T) {
	withIsolatedState(t)
	_, _, err := execCmd(t, "models")
	if exitCodeOf(t, err) != ExitMissingKey {
		t.Fatalf("got exit %d, want %d (ExitMissingKey); err=%v", exitCodeOf(t, err), ExitMissingKey, err)
	}
}

func TestStatusWhenNotRunning(t *testing.T) {
	withIsolatedState(t)
	out, _, err := execCmd(t, "status", "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["running"] != false {
		t.Fatalf("got %v, want running:false", doc)
	}
}

func TestStatusDetectsStaleState(t *testing.T) {
	withIsolatedState(t)
	// Record a gateway state pointing at a port nothing is listening on.
	if err := writeGatewayState(gatewayState{Port: 1, Profile: "work"}); err != nil {
		t.Fatalf("writeGatewayState: %v", err)
	}
	out, _, err := execCmd(t, "status", "--json")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc["running"] != false || doc["stale"] != true {
		t.Fatalf("got %v, want running:false stale:true", doc)
	}
}

func TestDownWhenNothingLinkedIsNoop(t *testing.T) {
	withIsolatedState(t)
	withFakeClaudeAdapter(t)

	_, _, err := execCmd(t, "down")
	if err != nil {
		t.Fatalf("down: %v", err)
	}
}

func TestLinkRequiresRunningGateway(t *testing.T) {
	withIsolatedState(t)
	withFakeClaudeAdapter(t)

	_, _, err := execCmd(t, "link", "claude-code")
	if exitCodeOf(t, err) != ExitGatewayFailed {
		t.Fatalf("got exit %d, want %d (ExitGatewayFailed); err=%v", exitCodeOf(t, err), ExitGatewayFailed, err)
	}
}

func TestLinkAndUnlinkAgainstFakeRunningGateway(t *testing.T) {
	withIsolatedState(t)
	settingsPath := withFakeClaudeAdapter(t)
	seedProfile(t, "work")

	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()
	port := healthy.Listener.Addr().(*net.TCPAddr).Port

	if err := writeGatewayState(gatewayState{Port: port, Token: "tok", Profile: "work"}); err != nil {
		t.Fatalf("writeGatewayState: %v", err)
	}

	_, _, err := execCmd(t, "link", "claude-code")
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	data, rerr := os.ReadFile(settingsPath)
	if rerr != nil {
		t.Fatalf("read settings: %v", rerr)
	}
	if !strings.Contains(string(data), "ANTHROPIC_BASE_URL") {
		t.Fatalf("got %s, want ANTHROPIC_BASE_URL to be set after link", data)
	}

	_, _, err = execCmd(t, "unlink", "claude-code")
	if err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if _, statErr := os.Stat(settingsPath); statErr == nil {
		t.Fatalf("expected settings.json to be removed after unlink (it was created fresh by link)")
	}
}

func TestUnknownPlatformReturnsExitUsage(t *testing.T) {
	_, _, err := execCmd(t, "link", "nonexistent-tool")
	if exitCodeOf(t, err) != ExitUsage {
		t.Fatalf("got exit %d, want %d; err=%v", exitCodeOf(t, err), ExitUsage, err)
	}
}

func TestExitCodeTable(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		want  int
		setup func(t *testing.T)
	}{
		{
			name:  "missing profile",
			args:  []string{"profiles", "activate", "ghost"},
			want:  ExitUsage,
			setup: func(t *testing.T) { withIsolatedState(t) },
		},
		{
			name:  "missing key for models",
			args:  []string{"models"},
			want:  ExitMissingKey,
			setup: func(t *testing.T) { withIsolatedState(t) },
		},
		{
			name:  "unknown platform",
			args:  []string{"unlink", "ghost-tool"},
			want:  ExitUsage,
			setup: func(*testing.T) {},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.setup != nil {
				c.setup(t)
			}
			_, _, err := execCmd(t, c.args...)
			if got := exitCodeOf(t, err); got != c.want {
				t.Fatalf("got exit %d, want %d; err=%v", got, c.want, err)
			}
		})
	}
}
