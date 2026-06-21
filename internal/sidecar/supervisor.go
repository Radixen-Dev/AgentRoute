// SPDX-License-Identifier: GPL-3.0-only

package sidecar

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/Radixen-Dev/AgentRoute/internal/paths"
)

// State is the lifecycle state of a supervised sidecar process.
type State string

const (
	StateStopped  State = "stopped"
	StateStarting State = "starting"
	StateRunning  State = "running"
	StateCrashed  State = "crashed"
)

// ErrBinaryNotFound is returned by Start when the configured binary (default
// "litellm") is not on PATH. This is the v1 hybrid-engine trade-off: LiteLLM
// is a Python tool we shell out to until v2's native Anthropic translator
// removes the dependency.
var ErrBinaryNotFound = errors.New(`sidecar: litellm not found on PATH; install it with "pipx install litellm" or see https://docs.litellm.ai/docs/proxy/quick_start`)

// Supervisor starts, health-checks, and stops a single LiteLLM proxy
// subprocess. It is safe for concurrent use.
type Supervisor struct {
	// Binary is the executable to run. Defaults to "litellm" if empty.
	Binary string
	// ExtraEnv is appended to the subprocess's environment in addition to
	// the current process's environment. Tests use this to re-exec the
	// test binary as a fake LiteLLM server instead of requiring a real
	// litellm install.
	ExtraEnv []string
	// HealthTimeout bounds how long Start waits for the sidecar to report
	// healthy before giving up. Defaults to 20s if zero.
	HealthTimeout time.Duration

	mu    sync.Mutex
	cmd   *exec.Cmd
	state State
	done  chan struct{}
	// stopping is set by Stop before killing the process, so the reaper
	// goroutine can tell an intentional stop apart from a crash.
	stopping bool
	exitErr  error
	logPath  string
}

// State reports the supervisor's current view of the sidecar's lifecycle.
func (s *Supervisor) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.state == "" {
		return StateStopped
	}
	return s.state
}

// LogPath returns the path of the sidecar's combined stdout/stderr log from
// the most recent Start call, or "" if Start has never been called.
func (s *Supervisor) LogPath() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.logPath
}

// Start launches the sidecar with configPath, listening on port, and blocks
// until it reports healthy or ctx/HealthTimeout expires. If health never
// arrives, the process is killed and the last lines of its log are included
// in the returned error.
func (s *Supervisor) Start(ctx context.Context, configPath string, port int) error {
	s.mu.Lock()
	if s.cmd != nil {
		s.mu.Unlock()
		return fmt.Errorf("sidecar: already started (state=%s)", s.state)
	}

	binary := s.Binary
	if binary == "" {
		binary = "litellm"
	}
	resolved, err := exec.LookPath(binary)
	if err != nil {
		s.mu.Unlock()
		return ErrBinaryNotFound
	}

	logDir, err := paths.SidecarDir()
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("sidecar: resolve log dir: %w", err)
	}
	logPath := filepath.Join(logDir, "litellm.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("sidecar: create log file: %w", err)
	}

	cmd := exec.Command(resolved, "--config", configPath, "--port", strconv.Itoa(port))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.Env = append(os.Environ(), s.ExtraEnv...)
	setProcAttrs(cmd)

	if err := cmd.Start(); err != nil {
		logFile.Close()
		s.mu.Unlock()
		return fmt.Errorf("sidecar: start %s: %w", binary, err)
	}

	s.cmd = cmd
	s.state = StateStarting
	s.stopping = false
	s.exitErr = nil
	s.logPath = logPath
	done := make(chan struct{})
	s.done = done
	s.mu.Unlock()

	go s.reap(cmd, logFile, done)

	timeout := s.HealthTimeout
	if timeout == 0 {
		timeout = 20 * time.Second
	}
	if err := s.waitHealthy(ctx, port, timeout); err != nil {
		_ = s.Stop(5 * time.Second)
		tail, _ := readTail(logPath, 10)
		if tail != "" {
			return fmt.Errorf("%w\n--- last log lines (%s) ---\n%s", err, logPath, tail)
		}
		return err
	}

	s.mu.Lock()
	s.state = StateRunning
	s.mu.Unlock()
	return nil
}

// reap waits for the subprocess to exit and records whether that was an
// intentional Stop or a crash. It runs exactly once per Start call.
func (s *Supervisor) reap(cmd *exec.Cmd, logFile *os.File, done chan struct{}) {
	err := cmd.Wait()
	logFile.Close()

	s.mu.Lock()
	if s.stopping {
		s.state = StateStopped
	} else {
		s.state = StateCrashed
	}
	s.exitErr = err
	s.mu.Unlock()
	close(done)
}

func (s *Supervisor) waitHealthy(ctx context.Context, port int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	url := fmt.Sprintf("http://127.0.0.1:%d/health/liveliness", port)
	client := &http.Client{Timeout: 2 * time.Second}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if resp, err := client.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("sidecar: did not become healthy within %s", timeout)
		case <-ticker.C:
		}
	}
}

// Stop terminates the sidecar's whole process tree (a Python LiteLLM
// process commonly spawns worker subprocesses; killing only the immediate
// pid leaves orphans and the port held). It waits up to timeout for a
// graceful exit before force-killing. Stop on a never-started or
// already-stopped Supervisor is a no-op.
func (s *Supervisor) Stop(timeout time.Duration) error {
	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	if cmd == nil {
		s.mu.Unlock()
		return nil
	}
	s.stopping = true
	s.mu.Unlock()

	// killProcessGroup is best-effort: a failure here (e.g. the process
	// already exited, or a transient taskkill error on Windows) must not
	// skip the fallback Kill below, which is the actual guarantee against
	// leaving an orphaned sidecar holding its port.
	_ = killProcessGroup(cmd)

	select {
	case <-done:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
		<-done
	}

	s.mu.Lock()
	s.cmd = nil
	s.mu.Unlock()
	return nil
}

func readTail(path string, n int) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := splitLines(string(data))
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	out := ""
	for _, l := range lines {
		out += l + "\n"
	}
	return out, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
