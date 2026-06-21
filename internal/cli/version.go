// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/version"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the AgentRoute version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			if p.json {
				return p.JSON(map[string]string{
					"version": version.Version,
					"commit":  version.Commit,
					"date":    version.Date,
				})
			}
			p.Line("agentroute %s", version.String())
			return nil
		},
	}
}
