// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/NimbleMarkets/ntcharts/sparkline"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/diagnostics"
	"github.com/Radixen-Dev/AgentRoute/internal/orchestrator"
	"github.com/Radixen-Dev/AgentRoute/internal/platform"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/anim"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

const dashGatewayZone = "dash-gateway-toggle"

// dashPlatEntry is a lightweight status+detection snapshot for one
// platform, held by the dashboard for its summary card. Full per-platform
// detail and link/unlink actions live on the Platforms/Wiring screen.
type dashPlatEntry struct {
	id          string
	displayName string
	installed   bool
	version     string
	status      platform.LinkStatus
	err         error
}

// dashboardScreen is AgentRoute's home screen: a single-glance command
// center for the gateway, the active profile's tier mapping, every linked
// platform, recent request activity, and environment health — everything
// the other monitoring screens (Live Log, Platforms, Doctor) show in detail,
// condensed here so a user only has to leave it for screens that actually
// require interaction (Profiles, Role Mapper, Model Picker).
type dashboardScreen struct {
	services *Services
	width    int
	height   int

	cfg           config.Config
	cfgErr        error
	hasProfile    bool
	profileReady  bool // true when the active profile has at least one tier model
	profileName   string
	profileModels map[string]string
	platEntries   []dashPlatEntry

	checks        []diagnostics.Check
	checksLoading bool

	spark        sparkline.Model
	lastReqCount int
	pending      string // "starting" | "stopping" | ""

	reduceMotion bool
	pulseOn      bool // alternates each tick to drive the "gateway up" heartbeat dot
	spinFrame    int  // advances while pending != "" to animate the status glyph
}

func newDashboardScreen(services *Services) Screen {
	return &dashboardScreen{
		services: services,
		spark:    sparkline.New(20, 3, sparkline.WithStyle(lipgloss.NewStyle().Foreground(theme.AccentCyan))),
	}
}

func (s *dashboardScreen) Title() string { return titleFor(ScreenDashboard) }

func (s *dashboardScreen) Bindings() []key.Binding {
	refresh := key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh"))
	if s.services.Running == nil {
		return []key.Binding{
			key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "start gateway")),
			refresh,
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "stop gateway")),
		refresh,
	}
}

func (s *dashboardScreen) Init() tea.Cmd {
	s.reduceMotion = anim.Reduced()
	s.cfg, s.cfgErr = config.Load()
	if s.cfg.ActiveProfile != "" {
		if prof, err := profile.Load(s.cfg.ActiveProfile); err == nil {
			s.hasProfile = true
			s.profileReady = len(prof.Models) > 0
			s.profileName = s.cfg.ActiveProfile
			s.profileModels = prof.Models
		}
	}
	s.checksLoading = true
	return tea.Batch(dashTickCmd(), fetchPlatStatusCmd(s.services.Platforms), runDoctorChecksCmd(s.services))
}

type dashTickMsg struct{}
type dashPlatStatusMsg []dashPlatEntry
type dashSpinTickMsg struct{}

func dashTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return dashTickMsg{} })
}

// dashSpinTickCmd drives the short-lived spinner glyph shown only while a
// start/stop action is pending — a dedicated faster ticker rather than
// piggybacking dashTickCmd, since unlike the heartbeat dot it must stop
// itself the moment the action resolves.
func dashSpinTickCmd() tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg { return dashSpinTickMsg{} })
}

func fetchPlatStatusCmd(platforms []platform.Platform) tea.Cmd {
	return func() tea.Msg {
		entries := make([]dashPlatEntry, len(platforms))
		for i, p := range platforms {
			ctx := context.Background()
			status, err := p.Status(ctx)
			entry := dashPlatEntry{id: p.ID(), displayName: p.DisplayName(), status: status, err: err}
			if err == nil {
				if d, derr := p.Detect(ctx); derr == nil {
					entry.installed = d.Installed
					entry.version = d.Version
				}
			}
			entries[i] = entry
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

// beginPending kicks off cmd alongside the spinner ticker (unless motion is
// reduced, in which case the static glyph in statusDot is enough).
func (s *dashboardScreen) beginPending(cmd tea.Cmd) tea.Cmd {
	if s.reduceMotion {
		return cmd
	}
	s.spinFrame = 0
	return tea.Batch(cmd, dashSpinTickCmd())
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
		return s.beginPending(startGatewayCmd(s.services))
	}
	s.pending = "stopping"
	return s.beginPending(stopGatewayCmd(s.services.Running))
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
			if !s.reduceMotion {
				s.pulseOn = !s.pulseOn
			}
		} else {
			s.lastReqCount = 0
			s.pulseOn = false
			s.spark.Push(0)
		}
		return s, tea.Batch(dashTickCmd(), fetchPlatStatusCmd(s.services.Platforms))

	case dashSpinTickMsg:
		if s.pending == "" {
			return s, nil
		}
		s.spinFrame++
		return s, dashSpinTickCmd()

	case dashPlatStatusMsg:
		s.platEntries = []dashPlatEntry(msg)
		return s, nil

	case doctorChecksMsg:
		s.checksLoading = false
		s.checks = msg.checks
		return s, nil

	case gatewayStartedMsg:
		s.pending = ""
		if msg.err != nil {
			return s, toast(toastErr, "start failed: "+msg.err.Error())
		}
		s.services.Running = msg.run
		return s, tea.Batch(
			toast(toastOK, "gateway up"),
			fetchPlatStatusCmd(s.services.Platforms),
			runDoctorChecksCmd(s.services),
		)

	case gatewayStoppedMsg:
		s.pending = ""
		s.services.Running = nil
		return s, tea.Batch(
			toast(toastInfo, "gateway stopped"),
			fetchPlatStatusCmd(s.services.Platforms),
			runDoctorChecksCmd(s.services),
		)

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
		case "r":
			s.checksLoading = true
			return s, tea.Batch(fetchPlatStatusCmd(s.services.Platforms), runDoctorChecksCmd(s.services))
		}

	case tea.MouseMsg:
		if zone.Get(dashGatewayZone).InBounds(msg) && msg.Action == tea.MouseActionPress {
			return s, s.toggleGateway()
		}
	}
	return s, nil
}

// requestStats summarizes the running gateway's request log: total
// requests seen, how many errored or returned >= 400, and the mean
// duration across all retained entries.
func (s *dashboardScreen) requestStats() (total, errs int, avg time.Duration) {
	if s.services.Running == nil {
		return 0, 0, 0
	}
	entries := s.services.Running.Server.RequestLog().Recent(0)
	total = len(entries)
	if total == 0 {
		return
	}
	var sum time.Duration
	for _, e := range entries {
		if e.Err != "" || e.StatusCode >= 400 {
			errs++
		}
		sum += e.Duration
	}
	avg = sum / time.Duration(total)
	return
}

// statusDot renders the gateway's single-glyph state indicator: a braille
// spinner while a start/stop is pending, a slow-breathing dot while up
// (alternating weight each second — lipgloss can't blend true alpha in a
// terminal cell, so brightness is approximated the same way splash.go fades
// in the wordmark), and a flat dot while down. All animation is skipped
// under reduced motion.
func (s *dashboardScreen) statusDot(styles theme.Styles) string {
	switch {
	case s.pending != "":
		glyph := "◐"
		if !s.reduceMotion {
			frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
			glyph = frames[s.spinFrame%len(frames)]
		}
		return styles.Warn.Render(glyph)
	case s.services.Running != nil:
		return styles.OK.Bold(s.reduceMotion || s.pulseOn).Render("●")
	default:
		return styles.Muted.Render("●")
	}
}

func formatUptime(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	sec := d / time.Second
	switch {
	case h > 0:
		return fmt.Sprintf("%dh%02dm", h, m)
	case m > 0:
		return fmt.Sprintf("%dm%02ds", m, sec)
	default:
		return fmt.Sprintf("%ds", sec)
	}
}

func formatLatency(d time.Duration) string {
	if d == 0 {
		return "—"
	}
	return d.Round(time.Millisecond).String()
}

func (s *dashboardScreen) renderGatewayCard(styles theme.Styles, width int) string {
	running := s.services.Running

	stateWord, stateStyle := "DOWN", styles.Muted
	switch {
	case s.pending == "starting":
		stateWord, stateStyle = "STARTING…", styles.Warn
	case s.pending == "stopping":
		stateWord, stateStyle = "STOPPING…", styles.Warn
	case running != nil:
		stateWord, stateStyle = "UP", styles.OK
	}

	line1 := s.statusDot(styles) + " " + stateStyle.Bold(true).Render(stateWord)
	if running != nil {
		line1 += styles.Muted.Render(fmt.Sprintf("   127.0.0.1:%d  ·  sidecar :%d  ·  profile %q  ·  up %s",
			running.Server.Port(), running.SidecarPort, running.ProfileName, formatUptime(time.Since(running.StartedAt))))
	} else {
		line1 += styles.Muted.Render("   no gateway running")
	}

	total, errs, avg := s.requestStats()
	errStyle := styles.Muted
	if errs > 0 {
		errStyle = styles.Err
	}
	line2 := styles.Accent.Render(fmt.Sprintf("%d", total)) + styles.Muted.Render(" requests") +
		"   " + errStyle.Render(fmt.Sprintf("%d", errs)) + styles.Muted.Render(" errors") +
		"   " + styles.Accent.Render(formatLatency(avg)) + styles.Muted.Render(" avg latency")

	buttonLabel := "[u] start gateway"
	switch {
	case s.pending != "":
		buttonLabel = "please wait…"
	case running != nil:
		buttonLabel = "[d] stop gateway"
	}
	button := zone.Mark(dashGatewayZone, styles.Selected.Render(" "+buttonLabel+" "))

	body := line1 + "\n" + line2 + "\n\n" + button
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(styles.CardTitle.Render("Gateway")+"\n"+body))
}

func (s *dashboardScreen) renderProfileCard(styles theme.Styles, width int) string {
	var body string
	if !s.hasProfile {
		body = styles.Muted.Render("no active profile") + "\n" + styles.Muted.Render("open Profiles (2) to create one")
	} else {
		header := s.profileName
		if !s.profileReady {
			header += "  " + styles.Warn.Render("⚠ no models configured")
		}
		body = header
		for _, r := range rolesOrder {
			model := s.profileModels[r.tier]
			if model == "" {
				model = styles.Muted.Render("(not set)")
			}
			body += "\n" + lipgloss.NewStyle().Width(20).Render(r.label) + model
		}
	}
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(styles.CardTitle.Render("Profile & Tiers")+"\n"+body))
}

func versionSuffix(v string) string {
	if v == "" {
		return ""
	}
	return " " + v
}

func (s *dashboardScreen) renderPlatformsCard(styles theme.Styles, width int) string {
	var body string
	if len(s.platEntries) == 0 {
		body = styles.Muted.Render("no platforms registered")
	}
	for i, pe := range s.platEntries {
		if i > 0 {
			body += "\n"
		}
		name := lipgloss.NewStyle().Width(14).Bold(true).Render(pe.displayName)
		var line string
		switch {
		case pe.err != nil:
			line = styles.Err.Render("error: " + pe.err.Error())
		case pe.status.Linked:
			line = styles.OK.Render("● linked") + styles.Muted.Render(" → "+pe.status.GatewayURL)
		case pe.installed:
			line = styles.Warn.Render("○ not linked") + styles.Muted.Render("  (installed"+versionSuffix(pe.version)+")")
		default:
			line = styles.Muted.Render("○ not detected")
		}
		body += name + line
		if pe.status.ConfigPath != "" {
			body += "\n" + lipgloss.NewStyle().Width(14).Render("") + styles.Muted.Render("config: "+pe.status.ConfigPath)
		}
	}
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(styles.CardTitle.Render("Platforms")+"\n"+body))
}

func (s *dashboardScreen) renderActivityCard(styles theme.Styles, width int) string {
	s.spark.Draw()

	var feed string
	switch s.services.Running {
	case nil:
		feed = styles.Muted.Render("no gateway running")
	default:
		entries := s.services.Running.Server.RequestLog().Recent(3)
		if len(entries) == 0 {
			feed = styles.Muted.Render("no requests yet")
		} else {
			lines := make([]string, len(entries))
			for i, e := range entries {
				lines[len(entries)-1-i] = formatRequestLine(styles, e) // newest first
			}
			feed = strings.Join(lines, "\n")
		}
	}

	body := s.spark.View() + "\n" + feed
	hint := styles.Muted.Render("  (full history: 5)")
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(styles.CardTitle.Render("Request activity")+hint+"\n"+body))
}

func (s *dashboardScreen) renderHealthCard(styles theme.Styles, width int) string {
	header := styles.CardTitle.Render("Health")
	if s.checksLoading {
		return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(header+"\n"+styles.Muted.Render("running checks...")))
	}

	pass := 0
	var failing []diagnostics.Check
	for _, c := range s.checks {
		if c.OK {
			pass++
		} else {
			failing = append(failing, c)
		}
	}

	summaryStyle := styles.OK
	if len(failing) > 0 {
		summaryStyle = styles.Warn
	}
	body := summaryStyle.Render(fmt.Sprintf("%d/%d checks passing", pass, len(s.checks)))
	for _, c := range failing {
		body += "\n" + styles.Err.Render("✗ "+c.Name) + "  " + styles.Muted.Render(c.Detail)
	}
	if len(failing) > 0 {
		body += "\n" + styles.Muted.Render("press 7 for full diagnostics")
	}
	return theme.Opaque(theme.SurfaceAlt, styles.Card.Width(width-2).Render(header+"\n"+body))
}

// splitWidth divides total (an outer card width, border included — see the
// renderXCard width convention) into two card widths separated by a
// single-column gap, used to lay the Profile and Platforms cards side by
// side on wide enough terminals.
func splitWidth(total int) (left, right int) {
	avail := total - 1
	left = avail / 2
	right = avail - left
	if left < 10 {
		left = 10
	}
	if right < 10 {
		right = 10
	}
	return
}

// twoColumnMinWidth is the narrowest screen width at which the Profile and
// Platforms cards sit side by side; below it they stack to stay readable.
const twoColumnMinWidth = 90

func (s *dashboardScreen) View() string {
	styles := s.services.Styles
	width := s.width
	if width <= 0 {
		width = 80
	}

	gateway := s.renderGatewayCard(styles, width)

	var midRow string
	if width >= twoColumnMinWidth {
		leftW, rightW := splitWidth(width)
		midRow = lipgloss.JoinHorizontal(lipgloss.Top,
			s.renderProfileCard(styles, leftW), " ", s.renderPlatformsCard(styles, rightW))
	} else {
		midRow = lipgloss.JoinVertical(lipgloss.Left,
			s.renderProfileCard(styles, width), s.renderPlatformsCard(styles, width))
	}

	activity := s.renderActivityCard(styles, width)
	health := s.renderHealthCard(styles, width)

	return lipgloss.JoinVertical(lipgloss.Left, gateway, midRow, activity, health)
}
