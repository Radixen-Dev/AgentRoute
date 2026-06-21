// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

func newLinkCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "link <platform>",
		Short: "Point a coding tool at the currently running gateway",
		Long: "Requires a gateway already running via `agentroute up` in another " +
			"terminal — this command reads its recorded port/token and the active " +
			"run's profile, it does not start anything itself.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			name := args[0]

			adapter, err := resolvePlatform(name)
			if err != nil {
				return withExitCode(ExitUsage, err)
			}

			st, ok, err := readGatewayState()
			if err != nil {
				return err
			}
			if !ok || !pingHealthz(st.Port) {
				return withExitCode(ExitGatewayFailed, fmt.Errorf("no running gateway found; start one with: agentroute up"))
			}

			prof, err := profile.Load(st.Profile)
			if err != nil {
				return err
			}

			res, err := adapter.Link(cmd.Context(), platform.LinkInput{
				GatewayURL:  fmt.Sprintf("http://127.0.0.1:%d", st.Port),
				AuthToken:   st.Token,
				RoleAliases: prof.RoleAliases(),
			})
			if err != nil {
				return withExitCode(ExitLinkFailed, err)
			}

			if p.json {
				return p.JSON(res)
			}
			p.Line("%s linked (config: %s)", name, res.ConfigPath)
			return nil
		},
	}
}
