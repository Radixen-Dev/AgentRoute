// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

// Run starts the TUI program and blocks until the user quits. If a gateway
// was started from the Dashboard during the session, it is stopped on the
// way out regardless of which screen was active when the user pressed q —
// AgentRoute is foreground-only (see internal/orchestrator's doc comment),
// and the TUI is just another foreground driver of that same lifecycle.
func Run(services Services) error {
	// Initialized synchronously, before the program starts, rather than as a
	// tea.Cmd from Init(): a Cmd runs on a goroutine with no ordering
	// guarantee against the first View() call, and dashboardScreen.View
	// calls zone.Mark unconditionally — a race that only didn't surface in
	// tests because testServices() also initializes it synchronously.
	zone.NewGlobal()
	defer zone.Close()

	m := New(services, false)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	final, err := p.Run()

	if fm, ok := final.(Model); ok && fm.services.Running != nil {
		fm.services.Running.Stop(context.Background())
	}
	return err
}
