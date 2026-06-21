// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/platform"
)

// platformsScreen is intentionally thin: v1 ships exactly one platform
// (Claude Code), so this screen is "Dashboard's platform card, with an
// unlink action" rather than a full management UI. See plan §6.3 —
// manifest-driven adapters (Codex, Gemini) arrive in Phase 9 without
// changing this screen's shape, just its list of one.
type platformsScreen struct {
	services *Services
	width    int
	status   platform.LinkStatus
	detect   platform.Detection
	loadErr  error
}

func newPlatformsScreen(services *Services) Screen {
	return &platformsScreen{services: services}
}

func (s *platformsScreen) Title() string { return titleFor(ScreenPlatforms) }

func (s *platformsScreen) Bindings() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unlink claude-code"))}
}

type platformStatusMsg struct {
	status platform.LinkStatus
	detect platform.Detection
	err    error
}

func loadPlatformStatusCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		adapter := services.NewPlatform()
		status, err := adapter.Status(context.Background())
		if err != nil {
			return platformStatusMsg{err: err}
		}
		detect, err := adapter.Detect(context.Background())
		return platformStatusMsg{status: status, detect: detect, err: err}
	}
}

func (s *platformsScreen) Init() tea.Cmd { return loadPlatformStatusCmd(s.services) }

func (s *platformsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	case platformStatusMsg:
		s.status, s.detect, s.loadErr = msg.status, msg.detect, msg.err
		return s, nil
	case tea.KeyMsg:
		if msg.String() == "u" {
			if !s.status.Linked {
				return s, toast(toastInfo, "claude-code is not linked")
			}
			if err := s.services.NewPlatform().Unlink(context.Background()); err != nil {
				return s, toast(toastErr, "unlink failed: "+err.Error())
			}
			return s, tea.Batch(loadPlatformStatusCmd(s.services), toast(toastOK, "claude-code unlinked"))
		}
	}
	return s, nil
}

func (s *platformsScreen) View() string {
	styles := s.services.Styles
	if s.loadErr != nil {
		return styles.Err.Render(s.loadErr.Error())
	}

	installed := "not detected on PATH"
	if s.detect.Installed {
		installed = "detected"
		if s.detect.Version != "" {
			installed += " (" + s.detect.Version + ")"
		}
	}
	linked := "not linked"
	if s.status.Linked {
		linked = fmt.Sprintf("linked -> %s", s.status.GatewayURL)
	}

	body := styles.CardTitle.Render("Claude Code") + "\n" +
		"install: " + installed + "\n" +
		"link:    " + linked
	if s.status.ConfigPath != "" {
		body += "\nconfig:  " + s.status.ConfigPath
	}
	return styles.Card.Width(maxInt(s.width-2, 20)).Render(body)
}
