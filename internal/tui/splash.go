// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Radixen-Dev/AgentRoute/internal/tui/anim"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
	"github.com/Radixen-Dev/AgentRoute/internal/version"
)

const splashFPS = 60

// splashState animates the wordmark's fade-in via a Harmonica spring
// (plan §7.4: ~600ms reveal, skipped entirely under reduced motion — see
// New's skipSplash/reduceMotion handling, which never constructs one).
type splashState struct {
	fade *anim.Spring
}

func newSplashState() *splashState {
	return &splashState{fade: anim.NewSpring(splashFPS, 1.0)}
}

// step advances the animation one tick and reports whether it has settled
// (the splash should hand off to the Dashboard).
func (s *splashState) step() bool {
	s.fade.Step()
	return s.fade.Settled()
}

func (s *splashState) tickCmd() tea.Cmd {
	return tea.Tick(time.Second/splashFPS, func(time.Time) tea.Msg { return splashTickMsg{} })
}

func renderSplash(s theme.Styles, width, height int, state *splashState) string {
	opacity := state.fade.Pos()

	wordmark := s.Wordmark.Render("Agent") + s.WordmarkAccent.Render("Route")
	if opacity < 1 {
		// A simple, deterministic two-stage reveal: dim until ~halfway,
		// full brightness after. lipgloss has no true alpha blending for
		// terminal cells, so we approximate "fade in" with a faded color
		// to avoid implying a feature the terminal can't render.
		faded := lipgloss.NewStyle().Foreground(theme.Muted).Render("AgentRoute")
		if opacity < 0.5 {
			wordmark = faded
		}
	}

	sub := s.Muted.Render(version.String())
	block := lipgloss.JoinVertical(lipgloss.Center, wordmark, sub)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, block, lipgloss.WithWhitespaceBackground(theme.Ink))
}
