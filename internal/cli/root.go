// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/tui"
)

// New builds AgentRoute's root cobra command with every plain-mode
// subcommand registered, plus the TUI: invoked with no subcommand on an
// interactive TTY, the root command itself launches the TUI (plan §7.6);
// `agentroute tui` forces it regardless of TTY detection.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:           "agentroute",
		Short:         "Route Claude Code (and, later, other coding agents) through OpenRouter via a local gateway",
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !shouldAutoLaunchTUI() {
				return cmd.Help()
			}
			return tui.Run(tui.DefaultServices())
		},
	}
	root.PersistentFlags().Bool("json", false, "emit newline-delimited JSON on stdout instead of human-readable text")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newKeyCmd())
	root.AddCommand(newProfilesCmd())
	root.AddCommand(newModelsCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newUpCmd())
	root.AddCommand(newDownCmd())
	root.AddCommand(newStatusCmd())
	root.AddCommand(newLinkCmd())
	root.AddCommand(newUnlinkCmd())
	root.AddCommand(newTUICmd())

	return root
}

// shouldAutoLaunchTUI gates the bare "agentroute" (no subcommand) case.
// AGENTROUTE_PLAIN=1 is the same escape hatch the plain-mode contract
// (plan §7.5) uses elsewhere, so a script that sets it once and forgets
// to pass a subcommand gets help text instead of an unexpected TUI hang.
func shouldAutoLaunchTUI() bool {
	if os.Getenv("AGENTROUTE_PLAIN") == "1" {
		return false
	}
	return isatty.IsTerminal(os.Stdout.Fd())
}

// newPrinter builds the printer a command's RunE should write all output
// through, honoring the (possibly inherited) --json flag.
func newPrinter(cmd *cobra.Command) *printer {
	asJSON, _ := cmd.Flags().GetBool("json")
	return &printer{out: cmd.OutOrStdout(), errw: cmd.ErrOrStderr(), json: asJSON}
}
