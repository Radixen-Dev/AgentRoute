// SPDX-License-Identifier: GPL-3.0-only

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

// AgentRoute is foreground-only by design: `up` runs until interrupted, at
// which point it unlinks the platform, stops the sidecar, and shuts down
// the gateway, in that order. There is no self-daemonization (see the
// architecture decision recorded for Phase 7) — users who want this
// running across logout/login use their OS's own service manager.
//
// The actual gateway+sidecar+link lifecycle lives in internal/orchestrator
// so the TUI can drive the identical lifecycle in-process instead of
// shelling out to its own binary.

func newUpCmd() *cobra.Command {
	var profileFlag string
	var portFlag int
	var noLink bool

	cmd := &cobra.Command{
		Use:   "up",
		Short: "Start the gateway (and the LiteLLM sidecar) in the foreground",
		Long: "Runs until interrupted (Ctrl+C / SIGTERM), at which point it cleanly " +
			"unlinks Claude Code, stops the sidecar, and shuts down the gateway. " +
			"AgentRoute does not daemonize: keep this terminal open, or run it under " +
			"your own process supervisor (systemd, launchd, Windows Task Scheduler) " +
			"if you want it to survive logout.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runUp(ctx, newPrinter(cmd), orchestrator.Options{
				ProfileName: profileFlag,
				Port:        portFlag,
				NoLink:      noLink,
			}, orchestrator.DefaultDeps())
		},
	}
	cmd.Flags().StringVar(&profileFlag, "profile", "", "profile to use (defaults to the active profile)")
	cmd.Flags().IntVar(&portFlag, "port", 0, "gateway port (defaults to the configured port; 0 means use config default)")
	cmd.Flags().BoolVar(&noLink, "no-link", false, "start the gateway and sidecar but do not link Claude Code")
	return cmd
}

func runUp(ctx context.Context, p *printer, opts orchestrator.Options, deps orchestrator.Deps) error {
	run, err := orchestrator.Start(ctx, opts, deps, func(format string, args ...any) { p.Errf(format, args...) })
	if err != nil {
		switch {
		case errors.Is(err, orchestrator.ErrNoActiveProfile):
			return withExitCode(ExitUsage, fmt.Errorf("%w; pass --profile or run: agentroute profiles activate <name>", err))
		case errors.Is(err, orchestrator.ErrEmptyProfile),
			errors.Is(err, profile.ErrNotFound),
			errors.Is(err, profile.ErrInvalidName):
			return withExitCode(ExitUsage, err)
		case errors.Is(err, orchestrator.ErrMissingAPIKey):
			return withExitCode(ExitMissingKey, fmt.Errorf("%w; run: agentroute key set --value <key>", err))
		case errors.Is(err, orchestrator.ErrLinkFailed):
			return withExitCode(ExitLinkFailed, err)
		default:
			return withExitCode(ExitGatewayFailed, err)
		}
	}
	defer run.Stop(context.Background())

	if err := writeGatewayState(gatewayState{
		Port:        run.Server.Port(),
		Token:       run.GatewayToken,
		Profile:     run.ProfileName,
		SidecarPort: run.SidecarPort,
		StartedAt:   time.Now().UTC(),
	}); err != nil {
		return withExitCode(ExitGatewayFailed, err)
	}
	defer func() { _ = removeGatewayState() }()

	if p.json {
		_ = p.JSON(map[string]any{
			"status":      "running",
			"port":        run.Server.Port(),
			"sidecarPort": run.SidecarPort,
			"profile":     run.ProfileName,
		})
	} else {
		p.Line("agentroute is up. Press Ctrl+C to stop.")
	}

	select {
	case <-ctx.Done():
		p.Errf("shutting down...")
		return nil
	case err := <-run.Done():
		return withExitCode(ExitGatewayFailed, fmt.Errorf("gateway exited unexpectedly: %w", err))
	}
}
