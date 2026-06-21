// SPDX-License-Identifier: GPL-3.0-only

package tui

import "github.com/charmbracelet/bubbles/key"

// KeyMap is the global keymap (plan §7.3). Screens may extend it with their
// own bindings but must not redefine these.
type KeyMap struct {
	Help     key.Binding
	Quit     key.Binding
	Back     key.Binding
	Tab      key.Binding
	ShiftTab key.Binding
	Up       key.Binding
	Down     key.Binding
	Filter   key.Binding
	Select   key.Binding
	Top      key.Binding
	Bottom   key.Binding
	GoTo     [9]key.Binding // "1".."9" jump to screen
	Refresh  key.Binding
}

// DefaultKeyMap returns the locked global keymap.
func DefaultKeyMap() KeyMap {
	km := KeyMap{
		Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
		Tab:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next")),
		ShiftTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "prev")),
		Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		Select:   key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
		Top:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
		Bottom:   key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
		Refresh:  key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh")),
	}
	for i := 0; i < 9; i++ {
		digit := string(rune('1' + i))
		km.GoTo[i] = key.NewBinding(key.WithKeys(digit), key.WithHelp(digit, "screen "+digit))
	}
	return km
}
