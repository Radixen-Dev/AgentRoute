// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/diagnostics"
)

type doctorScreen struct {
	services *Services
	width    int
	checks   []diagnostics.Check
	running  bool
}

func newDoctorScreen(services *Services) Screen {
	return &doctorScreen{services: services}
}

func (s *doctorScreen) Title() string { return titleFor(ScreenDoctor) }

func (s *doctorScreen) Bindings() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run checks"))}
}

type doctorChecksMsg struct{ checks []diagnostics.Check }

func runDoctorChecksCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		checks := diagnostics.Run(context.Background(), services.NewPlatform())
		return doctorChecksMsg{checks: checks}
	}
}

func (s *doctorScreen) Init() tea.Cmd {
	s.running = true
	return runDoctorChecksCmd(s.services)
}

func (s *doctorScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil
	case doctorChecksMsg:
		s.running = false
		s.checks = msg.checks
		return s, nil
	case tea.KeyMsg:
		if msg.String() == "r" {
			s.running = true
			return s, runDoctorChecksCmd(s.services)
		}
	}
	return s, nil
}

func (s *doctorScreen) View() string {
	styles := s.services.Styles
	if s.running {
		return styles.Muted.Render("running checks...")
	}
	out := ""
	for _, c := range s.checks {
		mark := styles.OK.Render("OK  ")
		if !c.OK {
			mark = styles.Err.Render("FAIL")
		}
		out += fmt.Sprintf("%s %-16s %s\n", mark, c.Name, c.Detail)
	}
	return styles.Card.Width(maxInt(s.width-2, 20)).Render(out)
}
