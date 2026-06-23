// SPDX-License-Identifier: GPL-3.0-only

// Package tui is AgentRoute's Bubble Tea application: a root model that
// routes between the screens described in the architecture plan §7, a
// persistent header/status bar, a toast overlay, and a help overlay.
package tui

import (
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"

	"github.com/Radixen-Dev/AgentRoute/internal/tui/anim"
)

// Model is the TUI's root Bubble Tea model.
type Model struct {
	// services is a pointer so that all screens — and initScreen when it
	// creates new screens — share one Services object. Without the pointer,
	// Model.Update's value receiver creates a copy of m on each call, and
	// &m.services in initScreen would point to a different allocation than
	// the one earlier screens hold, silently dropping mutations like
	// EditingProfile that screens write between navigation frames.
	services *Services
	keymap   KeyMap

	width, height int

	booting      bool
	splash       *splashState
	reduceMotion bool

	active    ScreenID
	screen    Screen
	backStack []ScreenID

	showHelp bool
	toast    *activeToast
	toastGen int
}

// New builds the root model. skipSplash is true under --plain, non-TTY,
// NO_COLOR, or when the caller otherwise wants to land directly on the
// Dashboard (e.g. snapshot tests).
func New(services Services, skipSplash bool) Model {
	// Allocate services on the heap so every screen and every initScreen call
	// gets the same pointer. Callers still pass Services by value (no API change).
	sp := new(Services)
	*sp = services
	reduceMotion := anim.Reduced()
	m := Model{
		services:     sp,
		keymap:       DefaultKeyMap(),
		active:       ScreenDashboard,
		reduceMotion: reduceMotion,
	}
	if !skipSplash && !reduceMotion {
		m.booting = true
		m.splash = newSplashState()
	}
	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	if m.booting {
		return m.splash.tickCmd()
	}
	// Init has a value receiver (Elm architecture rule: Init can't mutate
	// model state — only Update's return value is kept by the runtime), so
	// m.screen is constructed lazily on the first message Update sees
	// instead (below), not here. bubblezone's global manager is initialized
	// synchronously by Run before the program starts, not here.
	return nil
}

func (m *Model) initScreen(id ScreenID) tea.Cmd {
	m.screen = newScreen(id, m.services)
	m.active = id
	return m.screen.Init()
}

// forwardSize replays the current terminal dimensions into m.screen so that
// screens navigated to after the initial WindowSizeMsg start at the correct
// size rather than 0×0. Must be called after initScreen, never before.
func (m *Model) forwardSize() {
	if m.screen == nil || m.width == 0 {
		return
	}
	m.screen, _ = m.screen.Update(tea.WindowSizeMsg{
		Width:  m.width,
		Height: bodyHeight(m.height),
	})
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = typed.Width, typed.Height
		adjusted := typed
		adjusted.Height = bodyHeight(typed.Height)
		msg = adjusted

	case splashTickMsg:
		if m.booting {
			done := m.splash.step()
			if done {
				m.booting = false
				return m, m.initScreen(m.active)
			}
			return m, m.splash.tickCmd()
		}
		return m, nil

	case toastMsg:
		gen := m.toastGen + 1
		m.toastGen = gen
		m.toast = &activeToast{text: typed.text, level: typed.level}
		return m, clearToastAfter(gen, 4*time.Second)

	case clearToastMsg:
		if typed.gen == m.toastGen {
			m.toast = nil
		}
		return m, nil

	case navigateMsg:
		if typed.pushBack && !m.booting {
			m.backStack = append(m.backStack, m.active)
		}
		initCmd := m.initScreen(typed.to)
		m.forwardSize()
		return m, initCmd

	case backMsg:
		if n := len(m.backStack); n > 0 {
			prev := m.backStack[n-1]
			m.backStack = m.backStack[:n-1]
			initCmd := m.initScreen(prev)
			m.forwardSize()
			return m, initCmd
		}
		return m, nil

	case tea.KeyMsg:
		if m.booting {
			// Any key skips the splash.
			m.booting = false
			return m, m.initScreen(m.active)
		}
		if cmd, handled := m.handleGlobalKey(typed); handled {
			return m, cmd
		}
	}

	if m.booting {
		return m, nil
	}

	if m.screen == nil {
		// Lazily construct the initial screen on whichever message happens
		// to arrive first (commonly the startup tea.WindowSizeMsg, but the
		// exact ordering against Init's own returned Cmd isn't guaranteed —
		// see Init's comment). Replay msg into the freshly built screen so
		// it isn't silently dropped (a list/viewport that never sees its
		// first WindowSizeMsg would stay sized at 0x0 forever).
		initCmd := m.initScreen(m.active)
		scr, screenCmd := m.screen.Update(msg)
		m.screen = scr
		return m, tea.Batch(initCmd, screenCmd)
	}

	var cmd tea.Cmd
	m.screen, cmd = m.screen.Update(msg)
	return m, cmd
}

func (m *Model) handleGlobalKey(msg tea.KeyMsg) (tea.Cmd, bool) {
	// If the active screen has a focused text input, let it handle the key
	// directly — except Ctrl+C, which always quits.
	if ic, ok := m.screen.(InputCapturer); ok && ic.CapturingInput() {
		if msg.Type == tea.KeyCtrlC {
			return tea.Quit, true
		}
		return nil, false
	}
	switch {
	case key.Matches(msg, m.keymap.Quit):
		return tea.Quit, true
	case key.Matches(msg, m.keymap.Help):
		m.showHelp = !m.showHelp
		return nil, true
	case m.showHelp:
		// Any other key closes the help overlay rather than reaching the
		// screen underneath it.
		m.showHelp = false
		return nil, true
	case key.Matches(msg, m.keymap.Back):
		return func() tea.Msg { return backMsg{} }, true
	}
	for i, b := range m.keymap.GoTo {
		if key.Matches(msg, b) && i < len(screenOrder) {
			id := screenOrder[i]
			if id == m.active {
				return nil, true
			}
			return navigate(id), true
		}
	}
	return nil, false
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return ""
	}
	if m.booting {
		return zone.Scan(renderSplash(m.services.Styles, m.width, m.height, m.splash))
	}

	header := renderHeader(m.services.Styles, m.width, titleFor(m.active), m.services.Running != nil)
	hints := formatHints(globalHints(m.keymap, m.screen.Bindings()))
	status := renderStatusBar(m.services.Styles, m.width, hints, m.toast)
	body := m.screen.View()

	view := header + "\n" + body + "\n" + status
	if m.showHelp {
		view = renderHelpOverlay(m.services.Styles, m.width, m.height, m.keymap, m.screen)
	}
	return zone.Scan(view)
}

func globalHints(_ KeyMap, screenBindings []key.Binding) []keyHint {
	hints := []keyHint{
		{key: "?", label: "help"},
		{key: "q", label: "quit"},
		{key: "esc", label: "back"},
	}
	for _, b := range screenBindings {
		h := b.Help()
		if h.Key == "" {
			continue
		}
		hints = append(hints, keyHint{key: h.Key, label: h.Desc})
	}
	return hints
}

// headerLines and statusBarLines are the fixed heights of the persistent
// chrome (header has a bottom border, so it's 2 lines; the status bar is
// borderless, 1 line) that every screen's available body height must
// subtract.
const (
	headerLines    = 2
	statusBarLines = 1
)

func bodyHeight(totalHeight int) int {
	h := totalHeight - headerLines - statusBarLines
	if h < 0 {
		return 0
	}
	return h
}

type splashTickMsg struct{}

type clearToastMsg struct{ gen int }

func clearToastAfter(gen int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return clearToastMsg{gen: gen} })
}
