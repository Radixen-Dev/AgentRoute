// SPDX-License-Identifier: GPL-3.0-only

// Package diagnostics implements the environment checks run by
// "agentroute doctor" (both the CLI command and the TUI screen).
package diagnostics

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

// Check is the result of a single doctor check.
type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// Run executes all environment checks and returns the results. p is the
// platform adapter used for the claude-code detection step.
func Run(ctx context.Context, p platform.Platform) []Check {
	var checks []Check

	key, source, err := secret.OpenRouterAPIKey()
	switch {
	case err != nil:
		checks = append(checks, Check{Name: "openrouter-key", OK: false, Detail: err.Error()})
	case key == "":
		checks = append(checks, Check{Name: "openrouter-key", OK: false, Detail: "not configured; run: agentroute key set --value <key>"})
	default:
		checks = append(checks, Check{Name: "openrouter-key", OK: true, Detail: fmt.Sprintf("configured (source: %s)", source)})
	}

	checks = append(checks, checkLiteLLM())

	detection, err := p.Detect(ctx)
	switch {
	case err != nil:
		checks = append(checks, Check{Name: "claude-code", OK: false, Detail: err.Error()})
	case !detection.Installed:
		checks = append(checks, Check{Name: "claude-code", OK: false, Detail: "`claude` not found on PATH"})
	default:
		checks = append(checks, Check{Name: "claude-code", OK: true, Detail: "found on PATH"})
	}

	port := config.DefaultPort
	if cfg, err := config.Load(); err == nil {
		port = cfg.Port
	}
	if PortFree(port) {
		checks = append(checks, Check{Name: "gateway-port", OK: true, Detail: fmt.Sprintf("port %d is free", port)})
	} else {
		checks = append(checks, Check{Name: "gateway-port", OK: false, Detail: fmt.Sprintf("port %d is already in use", port)})
	}

	return checks
}

// PortFree reports whether the given TCP port is available on 127.0.0.1.
func PortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// checkLiteLLM verifies that (a) the litellm binary is on PATH and (b) the
// proxy extras are installed so the sidecar can actually start. The extras
// check uses the Python interpreter in the same venv as the binary (the
// standard layout for pipx and most virtualenv installs).
func checkLiteLLM() Check {
	resolved, err := exec.LookPath("litellm")
	if err != nil {
		return Check{
			Name:   "litellm",
			OK:     false,
			Detail: `not found on PATH; install with: pipx install 'litellm[proxy]'`,
		}
	}

	py := litellmPython(resolved)
	if py == "" {
		return Check{Name: "litellm", OK: true, Detail: "found on PATH (proxy extras unverified: Python interpreter not found)"}
	}

	out, err := exec.Command(py, "-c", "import litellm.proxy.proxy_server").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		hint := "proxy extras not installed; run: pipx inject litellm 'litellm[proxy]'"
		if msg != "" {
			hint += "\n  " + msg
		}
		return Check{Name: "litellm", OK: false, Detail: hint}
	}

	return Check{Name: "litellm", OK: true, Detail: "found on PATH (proxy extras ok)"}
}

// litellmPython returns the Python interpreter that owns the litellm binary,
// or "" if it cannot be determined. For pipx and virtualenv installs the
// PATH-visible binary is a symlink into the venv's bin/ directory, so we
// resolve symlinks before looking for Python alongside it.
func litellmPython(litellmPath string) string {
	if resolved, err := filepath.EvalSymlinks(litellmPath); err == nil {
		litellmPath = resolved
	}
	dir := filepath.Dir(litellmPath)
	for _, name := range []string{"python3", "python", "python.exe"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
