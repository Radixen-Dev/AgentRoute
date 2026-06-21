// SPDX-License-Identifier: GPL-3.0-only

package sidecar

import (
	"context"
	"flag"
	"net"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestMain re-execs this test binary as a fake LiteLLM server when
// AGENTROUTE_FAKE_LITELLM=1 is set in its environment. This is the standard
// Go "helper process" pattern (see os/exec_test.go upstream) and lets
// Supervisor tests exercise a real subprocess, real process-group
// start/stop, and a real HTTP health check without requiring a real litellm
// install in CI or on a dev machine.
func TestMain(m *testing.M) {
	if os.Getenv("AGENTROUTE_FAKE_LITELLM") == "1" {
		runFakeLiteLLM()
		return
	}
	os.Exit(m.Run())
}

// runFakeLiteLLM mimics just enough of `litellm --config ... --port ...` to
// drive Supervisor: it accepts the same flags and serves
// /health/liveliness. It never returns (os.Exit is called by the test
// harness via Stop's kill, or by a fatal flag-parse error).
func runFakeLiteLLM() {
	fs := flag.NewFlagSet("litellm", flag.ContinueOnError)
	port := fs.String("port", "", "")
	_ = fs.String("config", "", "")
	if err := fs.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health/liveliness", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{Addr: "127.0.0.1:" + *port, Handler: mux}
	if err := srv.ListenAndServe(); err != nil {
		os.Exit(1)
	}
}

func fakeSupervisor(t *testing.T) (*Supervisor, int) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	port := freePort(t)
	s := &Supervisor{
		Binary:        os.Args[0],
		ExtraEnv:      []string{"AGENTROUTE_FAKE_LITELLM=1"},
		HealthTimeout: 5 * time.Second,
	}
	return s, port
}

func TestSupervisorStartReachesRunningAndHealthCheckPasses(t *testing.T) {
	s, port := fakeSupervisor(t)
	configPath := writeEmptyConfig(t)

	if err := s.Start(context.Background(), configPath, port); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = s.Stop(5 * time.Second) })

	if got := s.State(); got != StateRunning {
		t.Fatalf("State() = %q, want %q", got, StateRunning)
	}
	if s.LogPath() == "" {
		t.Fatalf("expected a non-empty LogPath after Start")
	}
}

func TestSupervisorStopTerminatesTheProcess(t *testing.T) {
	s, port := fakeSupervisor(t)
	configPath := writeEmptyConfig(t)

	if err := s.Start(context.Background(), configPath, port); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := s.Stop(5 * time.Second); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if got := s.State(); got != StateStopped {
		t.Fatalf("State() after Stop = %q, want %q", got, StateStopped)
	}

	// The health endpoint must actually be gone, not just "supervisor
	// thinks it's stopped" — this is what catches an orphaned process
	// tree left behind by an incomplete Windows taskkill /T.
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, err := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health/liveliness")
		if err != nil {
			return // connection refused/reset: process is gone, as expected.
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("sidecar still answering health checks 3s after Stop")
}

func TestSupervisorStartUnknownBinaryReturnsErrBinaryNotFound(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	s := &Supervisor{Binary: "agentroute-litellm-does-not-exist"}
	err := s.Start(context.Background(), writeEmptyConfig(t), freePort(t))
	if err != ErrBinaryNotFound {
		t.Fatalf("Start: got %v, want ErrBinaryNotFound", err)
	}
}

func TestSupervisorStartTimesOutIfNeverHealthy(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)

	// Re-exec this test binary without AGENTROUTE_FAKE_LITELLM set, so it
	// just runs the real test suite and exits almost immediately instead
	// of serving health/liveliness — simulating a binary that starts but
	// never becomes healthy, distinct from "binary not found".
	s := &Supervisor{Binary: os.Args[0], HealthTimeout: 1 * time.Second}
	err := s.Start(context.Background(), writeEmptyConfig(t), freePort(t))
	if err == nil {
		t.Fatalf("Start: got nil error, want a health-timeout error")
	}
	if got := s.State(); got != StateCrashed && got != StateStopped {
		t.Fatalf("State() after failed Start = %q, want StateCrashed or StateStopped", got)
	}
}

func writeEmptyConfig(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "litellm-*.yaml")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer f.Close()
	return f.Name()
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}
