// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether `agentroute up` is currently running",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)

			st, ok, err := readGatewayState()
			if err != nil {
				return err
			}
			if !ok {
				if p.json {
					return p.JSON(map[string]any{"running": false})
				}
				p.Line("not running.")
				return nil
			}

			alive := pingHealthz(st.Port)
			if p.json {
				return p.JSON(map[string]any{
					"running":     alive,
					"stale":       !alive,
					"port":        st.Port,
					"sidecarPort": st.SidecarPort,
					"profile":     st.Profile,
					"startedAt":   st.StartedAt,
				})
			}
			if !alive {
				p.Line("stale state found (recorded port %d is not responding) — a previous `up` may have crashed. Run `agentroute down` to clean up.", st.Port)
				return nil
			}
			p.Line("running: port=%d profile=%q sidecarPort=%d startedAt=%s", st.Port, st.Profile, st.SidecarPort, st.StartedAt.Format(time.RFC3339))
			return nil
		},
	}
}

func pingHealthz(port int) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/healthz", port))
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}
