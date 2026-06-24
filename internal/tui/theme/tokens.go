// SPDX-License-Identifier: GPL-3.0-only

// Package theme is the single source of truth for AgentRoute's locked
// brand palette (see BRANDING.md). No other package in internal/tui may
// hard-code a hex color — every style is built from the tokens here.
package theme

import "github.com/charmbracelet/lipgloss"

// Canonical palette. Locked; do not add or rename without updating
// BRANDING.md and tokens_test.go's expected set in the same change.
const (
	Ink        = lipgloss.Color("#0F1419")
	Surface    = lipgloss.Color("#171E25")
	SurfaceAlt = lipgloss.Color("#202A33")
	Border     = lipgloss.Color("#27343F")
	AccentCyan = lipgloss.Color("#41D6C3")
	AccentBlue = lipgloss.Color("#7AA7FF")
	OK         = lipgloss.Color("#80DF96")
	Warn       = lipgloss.Color("#FFC86B")
	Err        = lipgloss.Color("#FF7676")
	Text       = lipgloss.Color("#E6EDF3")
	Muted      = lipgloss.Color("#7D8A99")
)

// Styles holds every lipgloss.Style the TUI renders with, built once from
// the tokens above so screens never construct a style with a raw color.
type Styles struct {
	App            lipgloss.Style
	Header         lipgloss.Style
	Wordmark       lipgloss.Style
	WordmarkAccent lipgloss.Style
	StatusBar      lipgloss.Style
	Border         lipgloss.Style
	Card           lipgloss.Style
	CardTitle      lipgloss.Style
	Accent         lipgloss.Style
	Muted          lipgloss.Style
	Selected       lipgloss.Style
	OK             lipgloss.Style
	Warn           lipgloss.Style
	Err            lipgloss.Style
	Help           lipgloss.Style
	HelpKey        lipgloss.Style
	Toast          lipgloss.Style
}

// New builds the default Styles from the locked tokens.
func New() Styles {
	return Styles{
		App: lipgloss.NewStyle().Background(Ink).Foreground(Text),
		Header: lipgloss.NewStyle().
			Background(Surface).
			Foreground(Text).
			Padding(0, 1).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(Border),
		Wordmark:       lipgloss.NewStyle().Foreground(Text).Bold(true),
		WordmarkAccent: lipgloss.NewStyle().Foreground(AccentCyan).Bold(true),
		StatusBar: lipgloss.NewStyle().
			Background(Surface).
			Foreground(Muted).
			Padding(0, 1),
		Border: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).BorderForeground(Border),
		Card: lipgloss.NewStyle().
			Background(SurfaceAlt).
			Foreground(Text).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Border).
			Padding(0, 1),
		CardTitle: lipgloss.NewStyle().Foreground(AccentBlue).Bold(true),
		Accent:    lipgloss.NewStyle().Foreground(AccentCyan).Bold(true),
		Muted:     lipgloss.NewStyle().Foreground(Muted),
		Selected:  lipgloss.NewStyle().Foreground(Ink).Background(AccentCyan).Bold(true),
		OK:        lipgloss.NewStyle().Foreground(OK),
		Warn:      lipgloss.NewStyle().Foreground(Warn),
		Err:       lipgloss.NewStyle().Foreground(Err),
		Help:      lipgloss.NewStyle().Foreground(Muted),
		HelpKey:   lipgloss.NewStyle().Foreground(AccentCyan).Bold(true),
		Toast: lipgloss.NewStyle().
			Background(SurfaceAlt).
			Foreground(Text).
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(AccentBlue).
			Padding(0, 1),
	}
}
