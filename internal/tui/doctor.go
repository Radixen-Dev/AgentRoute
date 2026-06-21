// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

type doctorEntry struct {
	name   string
	ok     bool
	detail string
}

type doctorScreen struct {
	services *Services
	width    int
	checks   []doctorEntry
	running  bool
}

func newDoctorScreen(services *Services) Screen {
	return &doctorScreen{services: services}
}

func (s *doctorScreen) Title() string { return titleFor(ScreenDoctor) }

func (s *doctorScreen) Bindings() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "re-run checks"))}
}

type doctorChecksMsg struct{ checks []doctorEntry }

func runDoctorChecksCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		var checks []doctorEntry

		k, src, err := secret.OpenRouterAPIKey()
		switch {
		case err != nil:
			checks = append(checks, doctorEntry{"openrouter-key", false, err.Error()})
		case k == "":
			checks = append(checks, doctorEntry{"openrouter-key", false, "not configured"})
		default:
			checks = append(checks, doctorEntry{"openrouter-key", true, "configured (source: " + string(src) + ")"})
		}

		if _, err := exec.LookPath("litellm"); err != nil {
			checks = append(checks, doctorEntry{"litellm", false, "not found on PATH"})
		} else {
			checks = append(checks, doctorEntry{"litellm", true, "found on PATH"})
		}

		det, err := services.NewPlatform().Detect(context.Background())
		switch {
		case err != nil:
			checks = append(checks, doctorEntry{"claude-code", false, err.Error()})
		case !det.Installed:
			checks = append(checks, doctorEntry{"claude-code", false, "`claude` not found on PATH"})
		default:
			checks = append(checks, doctorEntry{"claude-code", true, "found on PATH"})
		}

		port := config.DefaultPort
		if cfg, err := config.Load(); err == nil {
			port = cfg.Port
		}
		if ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port)); err == nil {
			_ = ln.Close()
			checks = append(checks, doctorEntry{"gateway-port", true, fmt.Sprintf("port %d is free", port)})
		} else {
			checks = append(checks, doctorEntry{"gateway-port", false, fmt.Sprintf("port %d is already in use", port)})
		}

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
		if !c.ok {
			mark = styles.Err.Render("FAIL")
		}
		out += fmt.Sprintf("%s %-16s %s\n", mark, c.name, c.detail)
	}
	return styles.Card.Width(maxInt(s.width-2, 20)).Render(out)
}
