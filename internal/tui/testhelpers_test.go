// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"sync"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

// zoneOnce guards bubblezone's global manager: production code initializes
// it via the root model's Init() (see app.go's zoneInit), but tests that
// construct a screen directly (bypassing the root model) need it too,
// since dashboardScreen.View marks a clickable zone.
var zoneOnce sync.Once

func initZone() { zoneOnce.Do(zone.NewGlobal) }

func withIsolatedState(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
}

// fakePlatform is a minimal platform.Platform that never touches a real
// ~/.claude/settings.json, used by every TUI test that constructs Services.
type fakePlatform struct {
	detect platform.Detection
	status platform.LinkStatus
}

func (f *fakePlatform) ID() string          { return "claude-code" }
func (f *fakePlatform) DisplayName() string { return "Claude Code" }
func (f *fakePlatform) Wire() gateway.Wire  { return gateway.WireAnthropic }
func (f *fakePlatform) Roles() []platform.Role {
	return []platform.Role{{ID: "balanced", DisplayName: "Sonnet"}}
}
func (f *fakePlatform) Detect(context.Context) (platform.Detection, error) { return f.detect, nil }
func (f *fakePlatform) Link(context.Context, platform.LinkInput) (platform.LinkResult, error) {
	return platform.LinkResult{}, nil
}
func (f *fakePlatform) Unlink(context.Context) error { return nil }
func (f *fakePlatform) Status(context.Context) (platform.LinkStatus, error) {
	return f.status, nil
}

func testServices(t *testing.T) Services {
	t.Helper()
	initZone()
	withIsolatedState(t)
	return Services{
		Styles:              theme.New(),
		NewOpenRouterClient: openrouter.NewClient,
		OrchestratorDeps:    orchestrator.DefaultDeps(),
		NewPlatform:         func() platform.Platform { return &fakePlatform{} },
	}
}

// drive sends msg through Update and returns the resulting Model — a small
// helper so tests read as a sequence of steps without repeating the type
// assertion every time.
func drive(t *testing.T, m tea.Model, msg tea.Msg) tea.Model {
	t.Helper()
	next, _ := m.Update(msg)
	return next
}
