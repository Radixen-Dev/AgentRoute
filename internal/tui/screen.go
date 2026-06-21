// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// ScreenID identifies one of the TUI's screens (plan §7.2). Splash and
// Help are not in this enum: Splash only ever runs once at boot and Help
// is rendered as an overlay on top of whatever screen is active, not a
// destination you navigate to.
type ScreenID int

const (
	ScreenDashboard ScreenID = iota
	ScreenModelPicker
	ScreenRoleMapper
	ScreenProfiles
	ScreenLiveLog
	ScreenPlatforms
	ScreenDoctor
)

// screenTitles drives both the header and the numbered jump list in the
// help overlay; order here is the order number keys 1..7 jump to.
var screenOrder = []ScreenID{
	ScreenDashboard, ScreenProfiles, ScreenRoleMapper, ScreenModelPicker,
	ScreenLiveLog, ScreenPlatforms, ScreenDoctor,
}

// Screen is one full-window destination in the TUI. Screens are
// reconstructed fresh from Services every time the root model navigates
// to them (see navigateMsg in app.go) rather than kept alive in the
// background — simpler than threading invalidation through every screen,
// and cheap because every Init only does local file reads or an already
// in-flight HTTP call.
type Screen interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Screen, tea.Cmd)
	View() string
	Title() string
	// Bindings returns this screen's own key bindings, appended to the
	// global keymap in the Help overlay and the status bar hint line.
	Bindings() []key.Binding
}

// navigateMsg asks the root model to switch the active screen, optionally
// pushing the current one onto the back-stack first.
type navigateMsg struct {
	to       ScreenID
	pushBack bool
}

// backMsg pops the back-stack (Esc).
type backMsg struct{}

// toastMsg shows a transient status-bar message.
type toastMsg struct {
	text  string
	level toastLevel
}

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastOK
	toastWarn
	toastErr
)

func navigate(to ScreenID) tea.Cmd {
	return func() tea.Msg { return navigateMsg{to: to, pushBack: true} }
}

func toast(level toastLevel, text string) tea.Cmd {
	return func() tea.Msg { return toastMsg{text: text, level: level} }
}
