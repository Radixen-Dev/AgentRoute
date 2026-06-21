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

type dashboardScreen struct {
	services *Services
	width    int
	height   int

	cfg         config.Config
	cfgErr      error
	hasProfile  bool
	profileName string
	platStatus  platform.LinkStatus
	platErr     error

	spark        sparkline.Model
	lastReqCount int
	pending      string // "starting" | "stopping" | ""
}

func newDashboardScreen(services *Services) Screen {
	return &dashboardScreen{services: services, spark: sparkline.New(20, 3)}
}

func (s *dashboardScreen) Title() string { return titleFor(ScreenDashboard) }

func (s *dashboardScreen) Bindings() []key.Binding {
	bindings := []key.Binding{
		key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "start gateway")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "stop gateway")),
	}
	return bindings
}

func (s *dashboardScreen) Init() tea.Cmd {
	s.cfg, s.cfgErr = config.Load()
	if s.cfg.ActiveProfile != "" {
		if _, err := profile.Load(s.cfg.ActiveProfile); err == nil {
			s.hasProfile = true
			s.profileName = s.cfg.ActiveProfile
		}
	}
	s.platStatus, s.platErr = s.services.NewPlatform().Status(context.Background())
	return dashTickCmd()
}

type dashTickMsg struct{}

func dashTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return dashTickMsg{} })
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
			return toast(toastErr, "no active profile; open Profiles first")
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
		return s, dashTickCmd()

	case gatewayStartedMsg:
		s.pending = ""
		if msg.err != nil {
			return s, toast(toastErr, "start failed: "+msg.err.Error())
		}
		s.services.Running = msg.run
		return s, toast(toastOK, "gateway up")

	case gatewayStoppedMsg:
		s.pending = ""
		s.services.Running = nil
		return s, toast(toastInfo, "gateway stopped")

	case tea.KeyMsg:
		switch msg.String() {
		case "u", "d":
			return s, s.toggleGateway()
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
	}
	platLine := "not linked"
	if s.platErr != nil {
		platLine = "error: " + s.platErr.Error()
	} else if s.platStatus.Linked {
		platLine = fmt.Sprintf("claude-code linked -> %s", s.platStatus.GatewayURL)
	}
	infoCard := styles.Card.Width(width - 2).Render(
		styles.CardTitle.Render("Profile & Platforms") + "\n" +
			"active profile: " + profileLine + "\n" +
			"claude-code: " + platLine,
	)

	s.spark.Draw()
	sparkCard := styles.Card.Width(width - 2).Render(
		styles.CardTitle.Render("Request activity") + "\n" + s.spark.View(),
	)

	return lipgloss.JoinVertical(lipgloss.Left, gatewayCard, infoCard, sparkCard)
}
