// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

// newKeyCmd is not in the architecture plan's §7.6 command list verbatim,
// but plain mode requires a non-interactive way to get an OpenRouter key
// into storage (the plan's TUI screens assume one exists already) — this
// fills that gap.
func newKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "key",
		Short: "Manage the stored OpenRouter API key",
	}
	cmd.AddCommand(newKeySetCmd())
	cmd.AddCommand(newKeyClearCmd())
	cmd.AddCommand(newKeyStatusCmd())
	return cmd
}

func newKeySetCmd() *cobra.Command {
	var value string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:   "set",
		Short: "Store an OpenRouter API key in the OS keyring (or a 0600 file fallback)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)

			switch {
			case value != "" && fromStdin:
				return withExitCode(ExitUsage, fmt.Errorf("--value and --stdin are mutually exclusive"))
			case fromStdin:
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
				value = strings.TrimSpace(string(data))
			case value == "":
				return withExitCode(ExitUsage, fmt.Errorf("no key provided: pass --value <key> or --stdin"))
			}
			if value == "" {
				return withExitCode(ExitUsage, fmt.Errorf("the provided key is empty"))
			}

			source, err := secret.SetOpenRouterAPIKey(value)
			if err != nil {
				return err
			}
			if p.json {
				return p.JSON(map[string]string{"source": string(source)})
			}
			p.Line("API key stored (%s).", source)
			return nil
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "the OpenRouter API key")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "read the key from stdin instead of --value")
	return cmd
}

func newKeyClearCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clear",
		Short: "Remove the stored OpenRouter API key from the keyring and file fallback",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			if err := secret.ClearOpenRouterAPIKey(); err != nil {
				return err
			}
			if p.json {
				return p.JSON(map[string]bool{"cleared": true})
			}
			p.Line("API key cleared.")
			return nil
		},
	}
}

func newKeyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether an OpenRouter API key is configured, and where it came from",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			key, source, err := secret.OpenRouterAPIKey()
			if err != nil {
				return err
			}
			configured := key != ""
			if p.json {
				return p.JSON(map[string]any{"configured": configured, "source": string(source)})
			}
			if !configured {
				p.Line("no OpenRouter API key configured.")
				return nil
			}
			p.Line("API key configured (source: %s).", source)
			return nil
		},
	}
}
