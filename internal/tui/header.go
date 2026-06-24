// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

func titleFor(id ScreenID) string {
	switch id {
	case ScreenDashboard:
		return "Dashboard"
	case ScreenModelPicker:
		return "Model Picker"
	case ScreenRoleMapper:
		return "Role Mapper"
	case ScreenProfiles:
		return "Profiles"
	case ScreenLiveLog:
		return "Gateway / Live Log"
	case ScreenPlatforms:
		return "Platforms / Wiring"
	case ScreenDoctor:
		return "Doctor / Diagnostics"
	default:
		return ""
	}
}

// renderHeader draws the wordmark, current screen title, and a gateway
// status pill, all within the given width.
func renderHeader(s theme.Styles, width int, screenTitle string, gatewayUp bool) string {
	wordmark := s.Wordmark.Render("Agent") + s.WordmarkAccent.Render("Route")

	pillText := "gateway: down"
	pillStyle := s.Muted
	if gatewayUp {
		pillText = "gateway: up"
		pillStyle = s.OK
	}
	pill := pillStyle.Render(pillText)

	left := wordmark + "  " + s.Muted.Render(screenTitle)
	gap := width - lipgloss.Width(left) - lipgloss.Width(pill) - 2
	if gap < 1 {
		gap = 1
	}
	line := left + lipgloss.NewStyle().Width(gap).Render("") + pill
	return theme.Opaque(theme.Surface, s.Header.Width(width).Render(line))
}

// renderStatusBar draws the k9s-style "key: action" hint line plus any
// active toast, right-aligned.
func renderStatusBar(s theme.Styles, width int, hints string, t *activeToast) string {
	left := s.StatusBar.Render(hints)
	if t == nil {
		return theme.Opaque(theme.Surface, s.StatusBar.Width(width).Render(hints))
	}

	style := s.Muted
	switch t.level {
	case toastOK:
		style = s.OK
	case toastWarn:
		style = s.Warn
	case toastErr:
		style = s.Err
	}
	right := style.Render(t.text)
	gap := width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap < 1 {
		gap = 1
	}
	return theme.Opaque(theme.Surface, s.StatusBar.Width(width).Render(left+lipgloss.NewStyle().Width(gap).Render("")+right))
}

type activeToast struct {
	text  string
	level toastLevel
}

func formatHints(bindings []keyHint) string {
	out := ""
	for i, b := range bindings {
		if i > 0 {
			out += "  "
		}
		out += fmt.Sprintf("<%s> %s", b.key, b.label)
	}
	return out
}

type keyHint struct {
	key   string
	label string
}
