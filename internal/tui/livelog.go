// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/gateway"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

type liveLogScreen struct {
	services *Services
	viewport viewport.Model
	width    int
	height   int
	lastN    int // how many entries we've already rendered, to detect new arrivals
}

func newLiveLogScreen(services *Services) Screen {
	return &liveLogScreen{services: services, viewport: viewport.New(0, 0)}
}

func (s *liveLogScreen) Title() string { return titleFor(ScreenLiveLog) }

func (s *liveLogScreen) Bindings() []key.Binding {
	return []key.Binding{key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "jump to latest"))}
}

type liveLogTickMsg struct{}

func liveLogTickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg { return liveLogTickMsg{} })
}

func (s *liveLogScreen) Init() tea.Cmd {
	s.refresh()
	return liveLogTickCmd()
}

func (s *liveLogScreen) refresh() {
	if s.services.Running == nil {
		s.viewport.SetContent(dimText(s.services, "no gateway running — start one from the Dashboard (1)"))
		return
	}
	entries := s.services.Running.Server.RequestLog().Recent(0)
	atBottom := s.viewport.AtBottom()
	s.viewport.SetContent(renderRequestLog(s.services, entries))
	if atBottom || len(entries) != s.lastN {
		s.viewport.GotoBottom()
	}
	s.lastN = len(entries)
}

func dimText(services *Services, text string) string {
	return services.Styles.Muted.Render(text)
}

// formatRequestLine renders one request-log entry as a single line, shared
// by the full Live Log view and the Dashboard's condensed recent-activity
// feed so the two never drift out of sync on format.
func formatRequestLine(styles theme.Styles, e gateway.RequestEntry) string {
	status := fmt.Sprintf("%d", e.StatusCode)
	style := styles.OK
	if e.Err != "" {
		status = "ERR"
		style = styles.Err
	} else if e.StatusCode >= 400 {
		style = styles.Err
	}
	line := fmt.Sprintf("%s  %-5s  %-22s -> %-32s  %s  %s",
		e.Time.Format("15:04:05"), e.Wire, e.Alias, e.Model, style.Render(status), e.Duration.Round(time.Millisecond))
	if e.Err != "" {
		line += "  " + styles.Err.Render(e.Err)
	}
	return line
}

func renderRequestLog(services *Services, entries []gateway.RequestEntry) string {
	if len(entries) == 0 {
		return dimText(services, "no requests yet")
	}
	var b strings.Builder
	for _, e := range entries {
		b.WriteString(formatRequestLine(services.Styles, e))
		b.WriteString("\n")
	}
	return b.String()
}

func (s *liveLogScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		s.viewport.Width = msg.Width
		s.viewport.Height = msg.Height
		s.refresh()
		return s, nil
	case liveLogTickMsg:
		s.refresh()
		return s, liveLogTickCmd()
	case tea.KeyMsg:
		if msg.String() == "G" {
			s.viewport.GotoBottom()
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

func (s *liveLogScreen) View() string {
	return s.viewport.View()
}
