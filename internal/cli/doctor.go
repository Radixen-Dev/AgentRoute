// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

type doctorCheck struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the local environment for everything `agentroute up` needs",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			checks := runDoctorChecks(cmd.Context())

			allOK := true
			for _, c := range checks {
				if !c.OK {
					allOK = false
				}
			}

			if p.json {
				if err := p.JSON(checks); err != nil {
					return err
				}
			} else {
				for _, c := range checks {
					status := "OK"
					if !c.OK {
						status = "FAIL"
					}
					p.Line("[%s] %-16s %s", status, c.Name, c.Detail)
				}
			}

			if !allOK {
				return withExitCode(ExitGeneric, fmt.Errorf("one or more doctor checks failed"))
			}
			return nil
		},
	}
}

func runDoctorChecks(ctx context.Context) []doctorCheck {
	var checks []doctorCheck

	key, source, err := secret.OpenRouterAPIKey()
	switch {
	case err != nil:
		checks = append(checks, doctorCheck{Name: "openrouter-key", OK: false, Detail: err.Error()})
	case key == "":
		checks = append(checks, doctorCheck{Name: "openrouter-key", OK: false, Detail: "not configured; run: agentroute key set --value <key>"})
	default:
		checks = append(checks, doctorCheck{Name: "openrouter-key", OK: true, Detail: fmt.Sprintf("configured (source: %s)", source)})
	}

	if _, err := exec.LookPath("litellm"); err != nil {
		checks = append(checks, doctorCheck{Name: "litellm", OK: false, Detail: `not found on PATH; install with: pipx install litellm`})
	} else {
		checks = append(checks, doctorCheck{Name: "litellm", OK: true, Detail: "found on PATH"})
	}

	detection, err := newClaudeCodeAdapter().Detect(ctx)
	switch {
	case err != nil:
		checks = append(checks, doctorCheck{Name: "claude-code", OK: false, Detail: err.Error()})
	case !detection.Installed:
		checks = append(checks, doctorCheck{Name: "claude-code", OK: false, Detail: "`claude` not found on PATH"})
	default:
		checks = append(checks, doctorCheck{Name: "claude-code", OK: true, Detail: "found on PATH"})
	}

	port := config.DefaultPort
	if cfg, err := config.Load(); err == nil {
		port = cfg.Port
	}
	if portFree(port) {
		checks = append(checks, doctorCheck{Name: "gateway-port", OK: true, Detail: fmt.Sprintf("port %d is free", port)})
	} else {
		checks = append(checks, doctorCheck{Name: "gateway-port", OK: false, Detail: fmt.Sprintf("port %d is already in use", port)})
	}

	return checks
}

func portFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}
