// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Recover from an unclean shutdown: unlink Claude Code and clear stale state",
		Long: "AgentRoute is foreground-only: stop a running `agentroute up` with " +
			"Ctrl+C in its own terminal, which already unlinks cleanly on the way " +
			"out. Use `down` when that didn't happen (crash, closed terminal) and " +
			"Claude Code may still be pointed at a gateway that is no longer running.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)

			adapter := newClaudeCodeAdapter()
			unlinkErr := adapter.Unlink(cmd.Context())
			stateErr := removeGatewayState()

			if unlinkErr != nil {
				return unlinkErr
			}
			if stateErr != nil {
				return stateErr
			}

			if p.json {
				return p.JSON(map[string]bool{"unlinked": true})
			}
			p.Line("claude-code unlinked; stale gateway state cleared (if any).")
			return nil
		},
	}
}
