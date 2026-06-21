// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

// newOpenRouterClient is a test seam: tests reassign it to point at an
// httptest.Server instead of the real OpenRouter API.
var newOpenRouterClient = openrouter.NewClient

func newModelsCmd() *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "models",
		Short: "List the OpenRouter model catalog",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)

			key, _, err := secret.OpenRouterAPIKey()
			if err != nil {
				return err
			}
			if key == "" {
				return withExitCode(ExitMissingKey, fmt.Errorf("no OpenRouter API key configured; run: agentroute key set --value <key>"))
			}

			client := newOpenRouterClient(key)
			models, err := client.FetchModels(cmd.Context())
			if err != nil {
				if errors.Is(err, openrouter.ErrNoAPIKey) || strings.Contains(err.Error(), "invalid API key") {
					return withExitCode(ExitMissingKey, err)
				}
				return err
			}

			if filter != "" {
				needle := strings.ToLower(filter)
				var filtered []openrouter.Model
				for _, m := range models {
					if strings.Contains(strings.ToLower(m.ID), needle) || strings.Contains(strings.ToLower(m.Name), needle) {
						filtered = append(filtered, m)
					}
				}
				models = filtered
			}

			if p.json {
				return p.JSON(models)
			}
			if len(models) == 0 {
				p.Line("no models matched.")
				return nil
			}
			for _, m := range models {
				p.Line("%-50s %-40s ctx=%d", m.ID, m.Name, m.ContextLength)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "only show models whose id or name contains this substring (case-insensitive)")
	return cmd
}
