// SPDX-License-Identifier: GPL-3.0-only

package claudecode

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/Radixen-Dev/AgentRoute/internal/fsutil"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

// withIsolatedState redirects AgentRoute's own state dir (where the
// link-state bookkeeping file lives) into a temp dir, exactly like the
// pattern used in internal/profile and internal/secret tests.
func withIsolatedState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

// newTestAdapter returns an Adapter pointed at a settings.json path inside
// a fresh temp dir, never touching a real ~/.claude/settings.json. Every
// test in this file must go through this helper.
func newTestAdapter(t *testing.T) (*Adapter, string) {
	t.Helper()
	withIsolatedState(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	return &Adapter{SettingsPath: path}, path
}

func testLinkInput(gatewayURL string) platform.LinkInput {
	return platform.LinkInput{
		GatewayURL: gatewayURL,
		AuthToken:  "session-tok",
		RoleAliases: map[string]string{
			profile.TierHeavy:    profile.Alias(profile.TierHeavy),
			profile.TierBalanced: profile.Alias(profile.TierBalanced),
			profile.TierFast:     profile.Alias(profile.TierFast),
		},
	}
}

func TestLinkOnFirstTimeUserCreatesSettingsWithAllKeys(t *testing.T) {
	a, path := newTestAdapter(t)

	res, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505"))
	if err != nil {
		t.Fatalf("Link: %v", err)
	}
	if res.ConfigPath != path {
		t.Fatalf("ConfigPath = %q, want %q", res.ConfigPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal settings: %v", err)
	}
	env := doc["env"].(map[string]any)

	want := map[string]string{
		"ANTHROPIC_BASE_URL":             "http://127.0.0.1:4505",
		"ANTHROPIC_AUTH_TOKEN":           "session-tok",
		"ANTHROPIC_DEFAULT_OPUS_MODEL":   "agentroute-heavy",
		"ANTHROPIC_DEFAULT_SONNET_MODEL": "agentroute-balanced",
		"ANTHROPIC_DEFAULT_HAIKU_MODEL":  "agentroute-fast",
	}
	for k, v := range want {
		if env[k] != v {
			t.Errorf("env[%q] = %v, want %q", k, env[k], v)
		}
	}
}

func TestUnlinkAfterLinkOnFirstTimeUserDeletesFile(t *testing.T) {
	a, path := newTestAdapter(t)

	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505")); err != nil {
		t.Fatalf("Link: %v", err)
	}
	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected settings.json to be deleted, stat err = %v", err)
	}
}

func TestUnlinkAfterLinkOnExistingFileRestoresByteIdentical(t *testing.T) {
	a, path := newTestAdapter(t)

	original := []byte(`{
  "someUnrelatedSetting": true,
  "env": {
    "USER_OWN_VAR": "keep-me",
    "ANTHROPIC_BASE_URL": "https://my-own-proxy.example"
  }
}
`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505")); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Sanity: Link actually changed the live file.
	linkedData, _ := os.ReadFile(path)
	if string(linkedData) == string(original) {
		t.Fatalf("Link did not modify settings.json")
	}

	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored settings.json: %v", err)
	}
	if string(restored) != string(original) {
		t.Fatalf("Unlink did not restore byte-identical original.\ngot:  %s\nwant: %s", restored, original)
	}
}

func TestRelinkDoesNotClobberOriginalBackup(t *testing.T) {
	a, path := newTestAdapter(t)

	original := []byte(`{"env":{"USER_OWN_VAR":"keep-me"}}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505")); err != nil {
		t.Fatalf("Link #1: %v", err)
	}
	// Re-link as if switching profiles (different gateway URL/alias map).
	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:9999")); err != nil {
		t.Fatalf("Link #2: %v", err)
	}

	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	restored, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read restored settings.json: %v", err)
	}
	if string(restored) != string(original) {
		t.Fatalf("got %s, want byte-identical original %s", restored, original)
	}
}

func TestUnlinkFallsBackToKeyRemovalWhenBackupMissing(t *testing.T) {
	a, path := newTestAdapter(t)

	original := []byte(`{"env":{"USER_OWN_VAR":"keep-me"}}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatalf("seed settings.json: %v", err)
	}

	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505")); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Simulate the backup having been deleted out-of-band (disk cleanup,
	// user error, whatever) between Link and Unlink.
	if err := os.Remove(fsutil.BackupPath(path)); err != nil {
		t.Fatalf("remove backup to simulate loss: %v", err)
	}

	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read settings.json after fallback unlink: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	env := doc["env"].(map[string]any)

	if _, present := env["ANTHROPIC_BASE_URL"]; present {
		t.Errorf("expected ANTHROPIC_BASE_URL to be removed by fallback path")
	}
	if env["USER_OWN_VAR"] != "keep-me" {
		t.Errorf("expected unrelated user key to survive fallback removal, got %v", env["USER_OWN_VAR"])
	}
}

func TestUnlinkWithoutPriorLinkIsNoop(t *testing.T) {
	a, path := newTestAdapter(t)
	if err := a.Unlink(context.Background()); err != nil {
		t.Fatalf("Unlink without prior Link: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected no settings.json to have been created, stat err = %v", err)
	}
}

func TestStatusReflectsLiveFileNotJustBookkeeping(t *testing.T) {
	a, path := newTestAdapter(t)

	st, err := a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status (never linked): %v", err)
	}
	if st.Linked {
		t.Fatalf("expected Linked=false before any Link call")
	}

	if _, err := a.Link(context.Background(), testLinkInput("http://127.0.0.1:4505")); err != nil {
		t.Fatalf("Link: %v", err)
	}
	st, err = a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status (after link): %v", err)
	}
	if !st.Linked || st.GatewayURL != "http://127.0.0.1:4505" {
		t.Fatalf("got %+v, want Linked=true with GatewayURL set", st)
	}

	// Simulate the user (or Claude Code) independently clearing the key —
	// Status must trust the live file, not the stale link-state bookkeeping.
	if err := os.WriteFile(path, []byte(`{"env":{}}`), 0o600); err != nil {
		t.Fatalf("simulate manual edit: %v", err)
	}
	st, err = a.Status(context.Background())
	if err != nil {
		t.Fatalf("Status (after manual edit): %v", err)
	}
	if st.Linked {
		t.Fatalf("expected Linked=false after ANTHROPIC_BASE_URL was removed independently")
	}
}

func TestRolesCoverAllThreeTiers(t *testing.T) {
	a := New()
	roles := a.Roles()
	if len(roles) != 3 {
		t.Fatalf("got %d roles, want 3", len(roles))
	}
	seen := map[string]bool{}
	for _, r := range roles {
		seen[r.ID] = true
	}
	for _, tier := range []string{profile.TierHeavy, profile.TierBalanced, profile.TierFast} {
		if !seen[tier] {
			t.Errorf("missing role for tier %q", tier)
		}
	}
}
