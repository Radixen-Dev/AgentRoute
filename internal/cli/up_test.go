// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
	"github.com/Radixen-Dev/AgentRoute/internal/sidecar"
)

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

func TestUpFullLifecycleLinksAndCleansUpOnCancel(t *testing.T) {
	withIsolatedState(t)
	withFakeOpenRouterKey(t, "sk-test-key")
	seedProfile(t, "work")

	deps, settingsPath, getSupervisor, _ := fakeUpDeps(t)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- runUp(ctx, testPrinter(), orchestrator.Options{ProfileName: "work"}, deps)
	}()

	// Wait until up has reached "running": gateway.json present and
	// /healthz answering.
	waitForCondition(t, 10*time.Second, func() bool {
		st, ok, err := readGatewayState()
		return err == nil && ok && pingHealthz(st.Port)
	})

	// Claude Code's settings.json must now point at the gateway.
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("read settings.json: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("unmarshal settings.json: %v", err)
	}
	env := doc["env"].(map[string]any)
	if env["ANTHROPIC_BASE_URL"] == nil || env["ANTHROPIC_AUTH_TOKEN"] == nil {
		t.Fatalf("expected ANTHROPIC_BASE_URL/ANTHROPIC_AUTH_TOKEN to be set, got env=%v", env)
	}

	st, _, _ := readGatewayState()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("runUp returned error after cancel: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatalf("runUp did not return within 15s of cancel")
	}

	// gateway.json must be gone.
	if _, ok, _ := readGatewayState(); ok {
		t.Fatalf("expected gateway state to be removed after shutdown")
	}

	// settings.json must be deleted (it didn't exist before Link).
	if _, err := os.Stat(settingsPath); !os.IsNotExist(err) {
		t.Fatalf("expected settings.json to be removed after unlink, stat err = %v", err)
	}

	// The sidecar process must actually be dead, not just "we think it's
	// stopped" — poll its health endpoint until the connection is refused.
	sup := getSupervisor()
	if sup.State() != sidecar.StateStopped {
		t.Errorf("supervisor state = %q, want %q", sup.State(), sidecar.StateStopped)
	}
	waitForCondition(t, 3*time.Second, func() bool {
		client := &http.Client{Timeout: 300 * time.Millisecond}
		_, err := client.Get("http://127.0.0.1:" + strconv.Itoa(st.SidecarPort) + "/health/liveliness")
		return err != nil
	})
}

func TestUpUnwindsSidecarAndGatewayWhenLinkFails(t *testing.T) {
	withIsolatedState(t)
	withFakeOpenRouterKey(t, "sk-test-key")
	seedProfile(t, "work")

	deps, _, getSupervisor, _ := fakeUpDeps(t)

	// Force Link to fail deterministically and cross-platform: point the
	// adapter's settings path at a location whose parent is a regular
	// file, not a directory, so os.MkdirAll (inside fsutil.AtomicWrite)
	// fails.
	notADir := filepath.Join(t.TempDir(), "this-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o600); err != nil {
		t.Fatalf("seed not-a-dir file: %v", err)
	}
	deps.NewAdapter = func() platform.Platform {
		return &claudecode.Adapter{SettingsPath: filepath.Join(notADir, "settings.json")}
	}

	err := runUp(context.Background(), testPrinter(), orchestrator.Options{ProfileName: "work"}, deps)
	if err == nil {
		t.Fatalf("expected runUp to fail when Link fails")
	}
	var ec interface{ ExitCode() int }
	if !errors.As(err, &ec) || ec.ExitCode() != ExitLinkFailed {
		t.Fatalf("got error %v, want ExitLinkFailed (5)", err)
	}

	// gateway.json must never have been written: writeGatewayState only
	// runs after a successful Link.
	if _, ok, _ := readGatewayState(); ok {
		t.Fatalf("expected no gateway state to be written when Link fails")
	}

	// The sidecar (started before Link, in the orchestration order) must
	// have been stopped by the unwind, not left running.
	sup := getSupervisor()
	waitForCondition(t, 3*time.Second, func() bool {
		return sup.State() == sidecar.StateStopped
	})
}
