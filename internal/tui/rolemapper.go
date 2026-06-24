// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

var rolesOrder = []struct{ tier, label string }{
	{profile.TierHeavy, "Heavy (Opus)"},
	{profile.TierBalanced, "Balanced (Sonnet)"},
	{profile.TierFast, "Fast (Haiku)"},
}

type roleMapperScreen struct {
	services *Services
	cursor   int
	width    int
}

func newRoleMapperScreen(services *Services) Screen {
	return &roleMapperScreen{services: services}
}

func (s *roleMapperScreen) Title() string { return titleFor(ScreenRoleMapper) }

func (s *roleMapperScreen) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "pick model for tier")),
		key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "save profile")),
	}
}

func (s *roleMapperScreen) Init() tea.Cmd { return nil }

func (s *roleMapperScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
		case "down", "j":
			if s.cursor < len(rolesOrder)-1 {
				s.cursor++
			}
		case "enter":
			s.services.PickerTier = rolesOrder[s.cursor].tier
			return s, navigate(ScreenModelPicker)
		case "s":
			if s.services.EditingProfile.Name == "" {
				return s, toast(toastErr, "no profile selected; create one in Profiles first")
			}
			if err := profile.Save(s.services.EditingProfile); err != nil {
				return s, toast(toastErr, "save failed: "+err.Error())
			}
			return s, toast(toastOK, "saved "+s.services.EditingProfile.Name)
		}
	}
	return s, nil
}

func (s *roleMapperScreen) View() string {
	styles := s.services.Styles
	if s.services.EditingProfile.Name == "" {
		return styles.Muted.Render("no profile selected — open Profiles (2) and create or activate one")
	}

	out := styles.CardTitle.Render("Profile: "+s.services.EditingProfile.Name) + "\n\n"
	for i, r := range rolesOrder {
		model := s.services.EditingProfile.Models[r.tier]
		if model == "" {
			model = "(not set)"
		}
		line := fmt.Sprintf("%-20s %s", r.label, model)
		if i == s.cursor {
			line = styles.Selected.Render(line)
		}
		out += line + "\n"
	}
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(maxInt(s.width-2, 20)).Render(out))
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
