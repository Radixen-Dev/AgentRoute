// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/platform/claudecode"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

// Services bundles everything screens need to reach the rest of AgentRoute,
// with the same test-seam pattern used by internal/cli: production code
// gets the real OpenRouter client, the real orchestrator lifecycle, and the
// real Claude Code adapter; tests substitute fakes.
type Services struct {
	Styles theme.Styles

	// NewOpenRouterClient builds the client used by the Model Picker.
	// Defaults to openrouter.NewClient; tests point it at an httptest.Server.
	NewOpenRouterClient func(apiKey string) *openrouter.Client

	// OrchestratorDeps wires the gateway/sidecar/link lifecycle the
	// Dashboard and Gateway/Live Log screens drive. Defaults to
	// orchestrator.DefaultDeps(); tests substitute a fake litellm process
	// and a claudecode adapter pointed at a temp settings.json.
	OrchestratorDeps orchestrator.Deps

	// NewPlatform builds the platform adapter the Platforms/Wiring and
	// Doctor screens query for Detect/Status. Defaults to claudecode.New.
	NewPlatform func() platform.Platform

	// Running is the live gateway started from the TUI (Dashboard's "u"
	// action), if any. nil means no gateway is running. Owned by the root
	// model; screens read it but only the Dashboard screen starts/stops it.
	Running *orchestrator.Running

	// EditingProfile is the profile shown/edited by Profiles, Role Mapper,
	// and Model Picker. Screens are reconstructed fresh on every
	// navigation (see screen.go), so this — not screen-local state — is
	// what carries an in-progress edit across that navigation.
	EditingProfile profile.Profile

	// PickerTier is the tier the Model Picker is currently choosing a
	// model for, set by Role Mapper before navigating to ScreenModelPicker.
	PickerTier string

	// CachedModels avoids re-fetching the OpenRouter catalog every time
	// the user opens the Model Picker in one TUI session. Empty until the
	// first successful fetch; "r" forces a refresh.
	CachedModels []openrouter.Model
}

// DefaultServices wires every seam to its real production implementation.
func DefaultServices() Services {
	return Services{
		Styles:              theme.New(),
		NewOpenRouterClient: openrouter.NewClient,
		OrchestratorDeps:    orchestrator.DefaultDeps(),
		NewPlatform:         func() platform.Platform { return claudecode.New() },
	}
}
