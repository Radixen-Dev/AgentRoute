// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

const dashGatewayZone = "dash-gateway-toggle"

// dashPlatEntry is a lightweight status snapshot for one platform, held by
// the dashboard for its summary card. Full per-platform details live in the
// Platforms/Wiring screen.
type dashPlatEntry struct {
	id     string
	status platform.LinkStatus
	err    error
}

type dashboardScreen struct {
	services *Services
	width    int
	height   int

	cfg          config.Config
	cfgErr       error
	hasProfile   bool
	profileReady bool // true when the active profile has at least one tier model
	profileName  string
	platEntries  []dashPlatEntry

	spark        sparkline.Model
	lastReqCount int
	pending      string // "starting" | "stopping" | ""
}

func newDashboardScreen(services *Services) Screen {
	return &dashboardScreen{services: services, spark: sparkline.New(20, 3)}
}

func (s *dashboardScreen) Title() string { return titleFor(ScreenDashboard) }

func (s *dashboardScreen) Bindings() []key.Binding {
	if s.services.Running == nil {
		return []key.Binding{
			key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "start gateway")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "stop gateway")),
	}
}

func (s *dashboardScreen) Init() tea.Cmd {
	s.cfg, s.cfgErr = config.Load()
	if s.cfg.ActiveProfile != "" {
		if prof, err := profile.Load(s.cfg.ActiveProfile); err == nil {
			s.hasProfile = true
			s.profileReady = len(prof.Models) > 0
			s.profileName = s.cfg.ActiveProfile
		}
	}
	s.platEntries = make([]dashPlatEntry, len(s.services.Platforms))
	for i, p := range s.services.Platforms {
		status, err := p.Status(context.Background())
		s.platEntries[i] = dashPlatEntry{id: p.ID(), status: status, err: err}
	}
	return dashTickCmd()
}

type dashTickMsg struct{}
type dashPlatStatusMsg []dashPlatEntry

func dashTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return dashTickMsg{} })
}

func fetchPlatStatusCmd(platforms []platform.Platform) tea.Cmd {
	return func() tea.Msg {
		entries := make([]dashPlatEntry, len(platforms))
		for i, p := range platforms {
			status, err := p.Status(context.Background())
			entries[i] = dashPlatEntry{id: p.ID(), status: status, err: err}
		}
		return dashPlatStatusMsg(entries)
	}
}

type gatewayStartedMsg struct {
	run *orchestrator.Running
	err error
}

type gatewayStoppedMsg struct{}

func startGatewayCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		run, err := orchestrator.Start(context.Background(), orchestrator.Options{}, services.OrchestratorDeps, nil)
		return gatewayStartedMsg{run: run, err: err}
	}
}

func stopGatewayCmd(run *orchestrator.Running) tea.Cmd {
	return func() tea.Msg {
		run.Stop(context.Background())
		return gatewayStoppedMsg{}
	}
}

func (s *dashboardScreen) toggleGateway() tea.Cmd {
	if s.pending != "" {
		return nil
	}
	if s.services.Running == nil {
		if !s.hasProfile {
			return toast(toastErr, "no active profile — open Profiles (2) to create one")
		}
		if !s.profileReady {
			return toast(toastWarn, "profile has no models — open Role Mapper (3) to configure it")
		}
		s.pending = "starting"
		return startGatewayCmd(s.services)
	}
	s.pending = "stopping"
	return stopGatewayCmd(s.services.Running)
}

func (s *dashboardScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width, s.height = msg.Width, msg.Height
		w := msg.Width - 4
		if w < 1 {
			w = 1
		}
		s.spark.Resize(w, 3)
		return s, nil

	case dashTickMsg:
		if s.services.Running != nil {
			total := len(s.services.Running.Server.RequestLog().Recent(0))
			delta := total - s.lastReqCount
			if delta < 0 {
				delta = 0
			}
			s.lastReqCount = total
			s.spark.Push(float64(delta))
		} else {
			s.lastReqCount = 0
			s.spark.Push(0)
		}
		return s, tea.Batch(dashTickCmd(), fetchPlatStatusCmd(s.services.Platforms))

	case dashPlatStatusMsg:
		s.platEntries = []dashPlatEntry(msg)
		return s, nil

	case gatewayStartedMsg:
		s.pending = ""
		if msg.err != nil {
			return s, toast(toastErr, "start failed: "+msg.err.Error())
		}
		s.services.Running = msg.run
		return s, tea.Batch(toast(toastOK, "gateway up"), fetchPlatStatusCmd(s.services.Platforms))

	case gatewayStoppedMsg:
		s.pending = ""
		s.services.Running = nil
		return s, tea.Batch(toast(toastInfo, "gateway stopped"), fetchPlatStatusCmd(s.services.Platforms))

	case tea.KeyMsg:
		switch msg.String() {
		case "u":
			if s.services.Running == nil {
				return s, s.toggleGateway()
			}
		case "d":
			if s.services.Running != nil {
				return s, s.toggleGateway()
			}
		}

	case tea.MouseMsg:
		if zone.Get(dashGatewayZone).InBounds(msg) && msg.Action == tea.MouseActionPress {
			return s, s.toggleGateway()
		}
	}
	return s, nil
}

func (s *dashboardScreen) View() string {
	styles := s.services.Styles
	width := s.width
	if width <= 0 {
		width = 80
	}

	gatewayLines := "down"
	if s.services.Running != nil {
		r := s.services.Running
		gatewayLines = fmt.Sprintf("up on 127.0.0.1:%d  (sidecar :%d, profile %q)", r.Server.Port(), r.SidecarPort, r.ProfileName)
	}
	if s.pending != "" {
		gatewayLines += fmt.Sprintf("  (%s...)", s.pending)
	}
	buttonLabel := "[u] start"
	if s.services.Running != nil {
		buttonLabel = "[d] stop"
	}
	gatewayCard := styles.Card.Width(width - 2).Render(
		styles.CardTitle.Render("Gateway") + "\n" +
			gatewayLines + "\n" +
			zone.Mark(dashGatewayZone, styles.Selected.Render(" "+buttonLabel+" ")),
	)

	profileLine := "none active — open Profiles (2) to create one"
	if s.hasProfile {
		profileLine = s.profileName
		if !s.profileReady {
			profileLine += " " + styles.Warn.Render("(no models — press 3 to configure)")
		}
	}
	platLines := ""
	for _, pe := range s.platEntries {
		var statusStr string
		switch {
		case pe.err != nil:
			statusStr = styles.Err.Render("error: " + pe.err.Error())
		case pe.status.Linked:
			statusStr = styles.OK.Render(fmt.Sprintf("linked → %s", pe.status.GatewayURL))
		default:
			statusStr = styles.Muted.Render("not linked")
		}
		platLines += "\n" + lipgloss.NewStyle().Width(14).Render(pe.id+":") + statusStr
	}
	infoCard := styles.Card.Width(width - 2).Render(
		styles.CardTitle.Render("Profile & Platforms") + "\n" +
			"active profile: " + profileLine +
			platLines,
	)

	s.spark.Draw()
	sparkCard := styles.Card.Width(width - 2).Render(
		styles.CardTitle.Render("Request activity") + "\n" + s.spark.View(),
	)

	return lipgloss.JoinVertical(lipgloss.Left, gatewayCard, infoCard, sparkCard)
}
