// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"github.com/spf13/cobra"
)

func newUnlinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unlink <platform>",
		Short: "Restore a platform's config to its pre-Link state",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			name := args[0]

			adapter, err := resolvePlatform(name)
			if err != nil {
				return withExitCode(ExitUsage, err)
			}

			if err := adapter.Unlink(cmd.Context()); err != nil {
				return withExitCode(ExitLinkFailed, err)
			}

			if p.json {
				return p.JSON(map[string]string{"platform": name})
			}
			p.Line("%s unlinked.", name)
			return nil
		},
	}
}
