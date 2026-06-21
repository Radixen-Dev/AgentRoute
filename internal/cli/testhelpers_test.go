// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/sidecar"
)

// withIsolatedState redirects AgentRoute's own state dir into a temp dir,
// matching the pattern used by every other package's tests in this repo.
func withIsolatedState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	return dir
}

// withFakeOpenRouterKey sets OPENROUTER_API_KEY for the test, which always
// wins over the keyring/file fallback (see internal/secret) — this is the
// one path that lets cli tests configure a key without ever touching the
// real OS keyring.
func withFakeOpenRouterKey(t *testing.T, key string) {
	t.Helper()
	t.Setenv("OPENROUTER_API_KEY", key)
}

// seedProfile saves a profile with all three tiers populated, isolated
// under whatever state dir withIsolatedState set up.
func seedProfile(t *testing.T, name string) profile.Profile {
	t.Helper()
	p := profile.Profile{
		Name: name,
		Models: map[string]string{
			profile.TierHeavy:    "openrouter/anthropic/claude-opus-4.5",
			profile.TierBalanced: "openrouter/anthropic/claude-sonnet-4.5",
			profile.TierFast:     "openrouter/deepseek/deepseek-v4-flash",
		},
	}
	if err := profile.Save(p); err != nil {
		t.Fatalf("seedProfile: %v", err)
	}
	return p
}

// fakeUpDeps returns orchestrator.Deps wired to the TestMain fake-litellm
// helper process and a claudecode adapter pointed at a temp settings.json,
// never touching a real litellm install or a real ~/.claude/settings.json.
// It also returns accessors for the actual Supervisor/Adapter instances
// the orchestrator constructs internally, so tests can assert on their
// final state after runUp returns.
func fakeUpDeps(t *testing.T) (deps orchestrator.Deps, settingsPath string, getSupervisor func() *sidecar.Supervisor, getAdapter func() platform.Platform) {
	t.Helper()
	settingsPath = filepath.Join(t.TempDir(), "settings.json")

	var sup *sidecar.Supervisor
	var ad platform.Platform
	deps = orchestrator.Deps{
		NewSupervisor: func() *sidecar.Supervisor {
			sup = &sidecar.Supervisor{
				Binary:        os.Args[0],
				ExtraEnv:      []string{"AGENTROUTE_FAKE_LITELLM=1"},
				HealthTimeout: 5 * time.Second,
			}
			return sup
		},
		NewAdapter: func() platform.Platform {
			ad = &claudecode.Adapter{SettingsPath: settingsPath}
			return ad
		},
	}
	return deps, settingsPath, func() *sidecar.Supervisor { return sup }, func() platform.Platform { return ad }
}

func testPrinter() *printer {
	return &printer{out: &bytes.Buffer{}, errw: &bytes.Buffer{}, json: false}
}
