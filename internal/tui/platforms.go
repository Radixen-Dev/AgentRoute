// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Radixen-Dev/AgentRoute/internal/platform"
)

// platformEntry holds the live detect + link status for one registered
// platform adapter, loaded asynchronously on Init and refresh.
type platformEntry struct {
	adapter platform.Platform
	detect  platform.Detection
	status  platform.LinkStatus
	err     error
}

// platformsScreen lists every registered platform, shows its install and
// link state, and lets the user link or unlink the selected platform.
type platformsScreen struct {
	services *Services
	width    int
	cursor   int
	entries  []platformEntry
	loading  bool
}

func newPlatformsScreen(services *Services) Screen {
	return &platformsScreen{services: services, loading: true}
}

func (s *platformsScreen) Title() string { return titleFor(ScreenPlatforms) }

// Bindings returns context-sensitive hints for the selected platform: link
// if it is not linked, unlink if it is. Navigation and refresh are global.
func (s *platformsScreen) Bindings() []key.Binding {
	if s.loading || len(s.entries) == 0 {
		return nil
	}
	e := s.entries[s.cursor]
	if e.err != nil {
		return nil
	}
	if e.status.Linked {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter", "u"), key.WithHelp("enter/u", "unlink")),
		}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter", "l"), key.WithHelp("enter/l", "link")),
	}
}

// allPlatformStatusMsg carries the result of loading status+detect for every
// platform in Services.Platforms.
type allPlatformStatusMsg struct {
	entries []platformEntry
}

func loadAllPlatformStatusCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		entries := make([]platformEntry, len(services.Platforms))
		for i, adapter := range services.Platforms {
			ctx := context.Background()
			status, err := adapter.Status(ctx)
			if err != nil {
				entries[i] = platformEntry{adapter: adapter, err: err}
				continue
			}
			detect, _ := adapter.Detect(ctx)
			entries[i] = platformEntry{adapter: adapter, status: status, detect: detect}
		}
		return allPlatformStatusMsg{entries: entries}
	}
}

// platformActionDoneMsg signals completion of a link or unlink action.
type platformActionDoneMsg struct {
	displayName string
	verb        string // "linked" | "unlinked"
	err         error
}

func (s *platformsScreen) Init() tea.Cmd { return loadAllPlatformStatusCmd(s.services) }

func (s *platformsScreen) selected() *platformEntry {
	if len(s.entries) == 0 {
		return nil
	}
	return &s.entries[s.cursor]
}

func (s *platformsScreen) doLink() tea.Cmd {
	e := s.selected()
	if e == nil {
		return nil
	}
	r := s.services.Running
	if r == nil {
		return toast(toastWarn, "start the gateway first  (press 1 then u)")
	}
	adapter := e.adapter
	in := platform.LinkInput{
		GatewayURL:  fmt.Sprintf("http://127.0.0.1:%d", r.Server.Port()),
		AuthToken:   r.GatewayToken,
		RoleAliases: r.Profile.RoleAliases(),
	}
	return func() tea.Msg {
		_, err := adapter.Link(context.Background(), in)
		return platformActionDoneMsg{displayName: adapter.DisplayName(), verb: "linked", err: err}
	}
}

func (s *platformsScreen) doUnlink() tea.Cmd {
	e := s.selected()
	if e == nil {
		return nil
	}
	adapter := e.adapter
	return func() tea.Msg {
		err := adapter.Unlink(context.Background())
		return platformActionDoneMsg{displayName: adapter.DisplayName(), verb: "unlinked", err: err}
	}
}

func (s *platformsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.width = msg.Width
		return s, nil

	case allPlatformStatusMsg:
		s.loading = false
		s.entries = msg.entries
		if s.cursor >= len(s.entries) {
			s.cursor = max(0, len(s.entries)-1)
		}
		return s, nil

	case platformActionDoneMsg:
		if msg.err != nil {
			return s, tea.Batch(
				loadAllPlatformStatusCmd(s.services),
				toast(toastErr, msg.verb+" failed: "+msg.err.Error()),
			)
		}
		return s, tea.Batch(
			loadAllPlatformStatusCmd(s.services),
			toast(toastOK, msg.displayName+" "+msg.verb),
		)

	case tea.KeyMsg:
		switch msg.String() {
		case "r":
			s.loading = true
			return s, loadAllPlatformStatusCmd(s.services)
		case "up", "k":
			if s.cursor > 0 {
				s.cursor--
			}
			return s, nil
		case "down", "j":
			if s.cursor < len(s.entries)-1 {
				s.cursor++
			}
			return s, nil
		case "enter":
			e := s.selected()
			if e == nil || e.err != nil {
				return s, nil
			}
			if e.status.Linked {
				return s, s.doUnlink()
			}
			return s, s.doLink()
		case "l":
			e := s.selected()
			if e != nil && e.err == nil && !e.status.Linked {
				return s, s.doLink()
			}
		case "u":
			e := s.selected()
			if e != nil && e.err == nil && e.status.Linked {
				return s, s.doUnlink()
			}
		}
	}
	return s, nil
}

const (
	platNameWidth    = 18
	platInstallWidth = 24
)

func (s *platformsScreen) View() string {
	styles := s.services.Styles
	w := maxInt(s.width-2, 40)

	if s.loading {
		return styles.Card.Width(w).Render(styles.Muted.Render("loading platforms..."))
	}
	if len(s.entries) == 0 {
		return styles.Card.Width(w).Render(styles.Muted.Render("no platforms registered"))
	}

	var b strings.Builder

	for i, e := range s.entries {
		sel := i == s.cursor

		// Cursor glyph
		cursor := "  "
		if sel {
			cursor = styles.OK.Render("> ")
		}

		// Name column
		nameStr := lipgloss.NewStyle().Width(platNameWidth).Bold(sel).Render(e.adapter.DisplayName())

		// Install column
		var installStr string
		if e.err != nil {
			installStr = styles.Err.Render("error")
		} else if e.detect.Installed {
			v := "installed"
			if e.detect.Version != "" {
				v += " (" + e.detect.Version + ")"
			}
			installStr = styles.OK.Render(v)
		} else {
			installStr = styles.Muted.Render("not detected")
		}
		installStr = lipgloss.NewStyle().Width(platInstallWidth).Render(installStr)

		// Link-status column
		var linkStr string
		if e.err != nil {
			linkStr = styles.Muted.Render("–")
		} else if e.status.Linked {
			linkStr = styles.OK.Render("linked → " + e.status.GatewayURL)
		} else {
			linkStr = styles.Muted.Render("not linked")
		}

		b.WriteString(cursor + nameStr + installStr + linkStr + "\n")
	}

	// Detail panel for the selected platform
	if sel := s.selected(); sel != nil {
		b.WriteString("\n")
		if sel.err != nil {
			b.WriteString(styles.Err.Render("error: " + sel.err.Error()) + "\n")
		} else {
			if sel.status.ConfigPath != "" {
				b.WriteString(styles.Muted.Render("config:  ") + sel.status.ConfigPath + "\n")
			}
			if sel.status.Linked {
				b.WriteString(styles.Muted.Render("gateway: ") + sel.status.GatewayURL + "\n")
			}
			b.WriteString("\n")
			if sel.status.Linked {
				b.WriteString(styles.Muted.Render("press enter or u to unlink"))
			} else {
				b.WriteString(styles.Muted.Render("press enter or l to link") +
					"  " + styles.Muted.Render("(gateway must be running — press 1 → u)"))
			}
		}
	}

	return styles.Card.Width(w).Render(b.String())
}
