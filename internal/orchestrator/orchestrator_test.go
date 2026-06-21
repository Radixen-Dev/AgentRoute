// SPDX-License-Identifier: GPL-3.0-only

package orchestrator

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/sidecar"
)

func withIsolatedState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

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

// fakeDeps wires Deps to the TestMain fake-litellm helper process and a
// claudecode adapter pointed at a temp settings.json, exactly like
// internal/cli's own fakeUpDeps test helper.
func fakeDeps(t *testing.T) (deps Deps, settingsPath string, getSupervisor func() *sidecar.Supervisor) {
	t.Helper()
	settingsPath = filepath.Join(t.TempDir(), "settings.json")

	var sup *sidecar.Supervisor
	deps = Deps{
		NewSupervisor: func() *sidecar.Supervisor {
			sup = &sidecar.Supervisor{
				Binary:        os.Args[0],
				ExtraEnv:      []string{"AGENTROUTE_FAKE_LITELLM=1"},
				HealthTimeout: 5 * time.Second,
			}
			return sup
		},
		NewAdapter: func() platform.Platform { return &claudecode.Adapter{SettingsPath: settingsPath} },
	}
	return deps, settingsPath, func() *sidecar.Supervisor { return sup }
}

func waitForCondition(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestStartReturnsRunningGatewayLinkedAndHealthy(t *testing.T) {
	withIsolatedState(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test-key")
	seedProfile(t, "work")

	deps, settingsPath, _ := fakeDeps(t)

	run, err := Start(context.Background(), Options{ProfileName: "work"}, deps, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer run.Stop(context.Background())

	if !run.Linked {
		t.Fatalf("expected Linked=true")
	}
	if _, err := os.Stat(settingsPath); err != nil {
		t.Fatalf("expected settings.json to be written by Link: %v", err)
	}

	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(run.Server.Port()) + "/healthz")
	if err != nil {
		t.Fatalf("healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
}

func TestStopUnlinksStopsSidecarAndShutsDownGateway(t *testing.T) {
	withIsolatedState(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test-key")
	seedProfile(t, "work")

	deps, settingsPath, getSupervisor := fakeDeps(t)

	run, err := Start(context.Background(), Options{ProfileName: "work"}, deps, nil)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	sidecarPort := run.SidecarPort

	run.Stop(context.Background())

	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected settings.json to be removed (it didn't exist before Link), stat err = %v", err)
	}

	sup := getSupervisor()
	waitForCondition(t, 3*time.Second, func() bool { return sup.State() == sidecar.StateStopped })
	waitForCondition(t, 3*time.Second, func() bool {
		client := &http.Client{Timeout: 300 * time.Millisecond}
		_, err := client.Get("http://127.0.0.1:" + strconv.Itoa(sidecarPort) + "/health/liveliness")
		return err != nil
	})

	// A second Stop must be a no-op, not a double-unwind panic/error.
	run.Stop(context.Background())
}

func TestStartFailsWithErrNoActiveProfile(t *testing.T) {
	withIsolatedState(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test-key")
	deps, _, _ := fakeDeps(t)

	_, err := Start(context.Background(), Options{}, deps, nil)
	if !errors.Is(err, ErrNoActiveProfile) {
		t.Fatalf("got %v, want ErrNoActiveProfile", err)
	}
}

func TestStartFailsWithErrMissingAPIKey(t *testing.T) {
	withIsolatedState(t)
	seedProfile(t, "work")
	deps, _, _ := fakeDeps(t)

	_, err := Start(context.Background(), Options{ProfileName: "work"}, deps, nil)
	if !errors.Is(err, ErrMissingAPIKey) {
		t.Fatalf("got %v, want ErrMissingAPIKey", err)
	}
}

func TestStartUnwindsSidecarWhenLinkFails(t *testing.T) {
	withIsolatedState(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test-key")
	seedProfile(t, "work")

	deps, _, getSupervisor := fakeDeps(t)

	notADir := filepath.Join(t.TempDir(), "this-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed not-a-dir file: %v", err)
	}
	deps.NewAdapter = func() platform.Platform {
		return &claudecode.Adapter{SettingsPath: filepath.Join(notADir, "settings.json")}
	}

	_, err := Start(context.Background(), Options{ProfileName: "work"}, deps, nil)
	if !errors.Is(err, ErrLinkFailed) {
		t.Fatalf("got %v, want ErrLinkFailed", err)
	}

	sup := getSupervisor()
	waitForCondition(t, 3*time.Second, func() bool { return sup.State() == sidecar.StateStopped })
}

func TestStartFailsWithErrEmptyProfile(t *testing.T) {
	withIsolatedState(t)
	t.Setenv("OPENROUTER_API_KEY", "sk-test-key")
	if err := profile.Save(profile.Profile{Name: "empty"}); err != nil {
		t.Fatalf("save empty profile: %v", err)
	}
	deps, _, _ := fakeDeps(t)

	_, err := Start(context.Background(), Options{ProfileName: "empty"}, deps, nil)
	if !errors.Is(err, ErrEmptyProfile) {
		t.Fatalf("got %v, want ErrEmptyProfile", err)
	}
}
