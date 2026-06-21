// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/tui"
)

// newTUICmd forces the TUI regardless of TTY detection (plan §7.6) — an
// escape hatch for terminals isatty can't recognize, or for users who
// just prefer typing the subcommand explicitly.
func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive TUI",
		RunE: func(*cobra.Command, []string) error {
			return tui.Run(tui.DefaultServices())
		},
	}
}
