// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/diagnostics"
)

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

func runDoctorChecks(ctx context.Context) []diagnostics.Check {
	return diagnostics.Run(ctx, newClaudeCodeAdapter())
}
