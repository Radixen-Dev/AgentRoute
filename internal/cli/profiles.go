// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

func newProfilesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage saved profiles (named sets of per-tier OpenRouter model choices)",
	}
	cmd.AddCommand(newProfilesListCmd())
	cmd.AddCommand(newProfilesCreateCmd())
	cmd.AddCommand(newProfilesDeleteCmd())
	cmd.AddCommand(newProfilesActivateCmd())
	return cmd
}

func newProfilesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List saved profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := newPrinter(cmd)
			profiles, err := profile.List()
			if err != nil {
				return err
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}

			if p.json {
				type entry struct {
					Name    string            `json:"name"`
					Active  bool              `json:"active"`
					Models  map[string]string `json:"models"`
					Created string            `json:"created"`
				}
				out := make([]entry, 0, len(profiles))
				for _, pr := range profiles {
					out = append(out, entry{
						Name:    pr.Name,
						Active:  pr.Name == cfg.ActiveProfile,
						Models:  pr.Models,
						Created: pr.Created.Format("2006-01-02T15:04:05Z07:00"),
					})
				}
				return p.JSON(out)
			}

			if len(profiles) == 0 {
				p.Line("no profiles saved. Create one with: agentroute profiles create <name>")
				return nil
			}
			for _, pr := range profiles {
				marker := "  "
				if pr.Name == cfg.ActiveProfile {
					marker = "* "
				}
				p.Line("%s%s", marker, pr.Name)
			}
			return nil
		},
	}
}

func newProfilesCreateCmd() *cobra.Command {
	var heavy, balanced, fast string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create (or overwrite) a profile with explicit per-tier OpenRouter model ids",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			name := args[0]

			if heavy == "" && balanced == "" && fast == "" {
				return withExitCode(ExitUsage, fmt.Errorf("at least one of --heavy, --balanced, --fast is required"))
			}

			models := map[string]string{}
			if heavy != "" {
				models[profile.TierHeavy] = heavy
			}
			if balanced != "" {
				models[profile.TierBalanced] = balanced
			}
			if fast != "" {
				models[profile.TierFast] = fast
			}

			if err := profile.Save(profile.Profile{Name: name, Models: models}); err != nil {
				if err == profile.ErrInvalidName {
					return withExitCode(ExitUsage, err)
				}
				return err
			}

			if p.json {
				return p.JSON(map[string]string{"name": name})
			}
			p.Line("profile %q saved.", name)
			return nil
		},
	}
	cmd.Flags().StringVar(&heavy, "heavy", "", "OpenRouter model id for the heavy tier, e.g. openrouter/anthropic/claude-opus-4.5")
	cmd.Flags().StringVar(&balanced, "balanced", "", "OpenRouter model id for the balanced tier")
	cmd.Flags().StringVar(&fast, "fast", "", "OpenRouter model id for the fast tier")
	return cmd
}

func newProfilesDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a saved profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			name := args[0]
			if err := profile.Delete(name); err != nil {
				if err == profile.ErrInvalidName {
					return withExitCode(ExitUsage, err)
				}
				return err
			}
			if p.json {
				return p.JSON(map[string]string{"name": name})
			}
			p.Line("profile %q deleted.", name)
			return nil
		},
	}
}

func newProfilesActivateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "activate <name>",
		Short: "Set a profile as the active one used by `agentroute up` when --profile is omitted",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p := newPrinter(cmd)
			name := args[0]

			exists, err := profile.Exists(name)
			if err != nil {
				if err == profile.ErrInvalidName {
					return withExitCode(ExitUsage, err)
				}
				return err
			}
			if !exists {
				return withExitCode(ExitUsage, fmt.Errorf("no profile named %q (see: agentroute profiles list)", name))
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			cfg.ActiveProfile = name
			if err := config.Save(cfg); err != nil {
				return err
			}

			if p.json {
				return p.JSON(map[string]string{"activeProfile": name})
			}
			p.Line("active profile set to %q.", name)
			return nil
		},
	}
}
