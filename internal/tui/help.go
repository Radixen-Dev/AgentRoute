// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

// renderHelpOverlay draws the full keymap (global + the active screen's
// own bindings + the numbered screen jump list) centered over a dimmed
// background, toggled by "?" anywhere (plan §7.2 screen 9 / §7.3).
func renderHelpOverlay(s theme.Styles, width, height int, _ KeyMap, active Screen) string {
	var b strings.Builder
	b.WriteString(s.CardTitle.Render("Keymap"))
	b.WriteString("\n\n")

	rows := []struct{ key, label string }{
		{"?", "toggle this help"},
		{"q / ctrl+c", "quit"},
		{"esc", "back"},
		{"tab / shift+tab", "cycle focus"},
		{"↑↓ / jk", "move"},
		{"/", "filter"},
		{"enter", "select"},
		{"g / G", "top / bottom"},
		{"r", "refresh"},
		{"1..7", "jump to screen"},
	}
	for _, r := range rows {
		_, _ = fmt.Fprintf(&b, "  %s  %s\n", s.HelpKey.Render(pad(r.key, 16)), s.Help.Render(r.label))
	}

	if bindings := active.Bindings(); len(bindings) > 0 {
		b.WriteString("\n")
		b.WriteString(s.CardTitle.Render(active.Title() + " keys"))
		b.WriteString("\n\n")
		for _, bd := range bindings {
			keys := strings.Join(bd.Keys(), "/")
			_, _ = fmt.Fprintf(&b, "  %s  %s\n", s.HelpKey.Render(pad(keys, 16)), s.Help.Render(bd.Help().Desc))
		}
	}

	b.WriteString("\n")
	b.WriteString(s.CardTitle.Render("Screens"))
	b.WriteString("\n\n")
	for i, id := range screenOrder {
		_, _ = fmt.Fprintf(&b, "  %s  %s\n", s.HelpKey.Render(pad(fmt.Sprintf("%d", i+1), 16)), s.Help.Render(titleFor(id)))
	}

	card := s.Card.Render(b.String())
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, card, lipgloss.WithWhitespaceBackground(theme.Ink))
}

func pad(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}
