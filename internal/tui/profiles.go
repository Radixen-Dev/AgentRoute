// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/config"
	"github.com/Radixen-Dev/AgentRoute/internal/profile"
)

type profileItem struct {
	p      profile.Profile
	active bool
}

func (i profileItem) Title() string {
	if i.active {
		return "* " + i.p.Name
	}
	return "  " + i.p.Name
}
func (i profileItem) Description() string {
	return fmt.Sprintf("%d tier(s) configured", len(i.p.Models))
}
func (i profileItem) FilterValue() string { return i.p.Name }

type profilesScreen struct {
	services *Services
	list     list.Model
	cfg      config.Config
	loadErr  error

	creating   bool
	input      textinput.Model
	confirmDel string // profile name pending a second "d" to confirm delete
}

func newProfilesScreen(services *Services) Screen {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Profiles"
	l.SetShowHelp(false)
	ti := textinput.New()
	ti.Placeholder = "new profile name"
	return &profilesScreen{services: services, list: l, input: ti}
}

func (s *profilesScreen) Title() string { return titleFor(ScreenProfiles) }

// CapturingInput implements InputCapturer: while the new-profile name field
// is open, all keys must reach the textinput, not the global keymap.
func (s *profilesScreen) CapturingInput() bool { return s.creating }

func (s *profilesScreen) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new profile")),
		key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete (press twice)")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "activate & edit roles")),
	}
}

func (s *profilesScreen) reload() tea.Cmd {
	s.cfg, _ = config.Load()
	profiles, err := profile.List()
	s.loadErr = err
	items := make([]list.Item, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, profileItem{p: p, active: p.Name == s.cfg.ActiveProfile})
	}
	return s.list.SetItems(items)
}

func (s *profilesScreen) Init() tea.Cmd {
	return s.reload()
}

func (s *profilesScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.list.SetSize(msg.Width, msg.Height)
		s.input.Width = msg.Width - 4
		return s, nil

	case tea.KeyMsg:
		if s.creating {
			switch msg.String() {
			case "enter":
				name := s.input.Value()
				s.creating = false
				s.input.Reset()
				if name == "" {
					return s, toast(toastWarn, "profile name cannot be empty")
				}
				if err := profile.Save(profile.Profile{Name: name}); err != nil {
					return s, toast(toastErr, "create failed: "+err.Error())
				}
				s.cfg.ActiveProfile = name
				if err := config.Save(s.cfg); err != nil {
					return s, toast(toastErr, "activate failed: "+err.Error())
				}
				s.services.EditingProfile = profile.Profile{Name: name}
				return s, tea.Batch(s.reload(), navigate(ScreenRoleMapper))
			case "esc":
				s.creating = false
				s.input.Reset()
				return s, nil
			}
			var cmd tea.Cmd
			s.input, cmd = s.input.Update(msg)
			return s, cmd
		}

		switch msg.String() {
		case "n":
			s.creating = true
			s.input.Focus()
			return s, nil
		case "enter":
			item, ok := s.list.SelectedItem().(profileItem)
			if !ok {
				return s, nil
			}
			s.cfg.ActiveProfile = item.p.Name
			if err := config.Save(s.cfg); err != nil {
				return s, toast(toastErr, "activate failed: "+err.Error())
			}
			s.services.EditingProfile = item.p
			return s, tea.Batch(s.reload(), navigate(ScreenRoleMapper))
		case "d":
			item, ok := s.list.SelectedItem().(profileItem)
			if !ok {
				return s, nil
			}
			if s.confirmDel == item.p.Name {
				s.confirmDel = ""
				if err := profile.Delete(item.p.Name); err != nil {
					return s, toast(toastErr, "delete failed: "+err.Error())
				}
				return s, tea.Batch(s.reload(), toast(toastOK, "deleted "+item.p.Name))
			}
			s.confirmDel = item.p.Name
			return s, tea.Batch(clearConfirmDelAfter(item.p.Name, 3*time.Second), toast(toastWarn, "press d again to delete "+item.p.Name))
		}

	case confirmDelExpiredMsg:
		if s.confirmDel == msg.name {
			s.confirmDel = ""
		}
		return s, nil
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

type confirmDelExpiredMsg struct{ name string }

func clearConfirmDelAfter(name string, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return confirmDelExpiredMsg{name: name} })
}

func (s *profilesScreen) View() string {
	if s.creating {
		return s.services.Styles.Card.Render("New profile name:\n\n" + s.input.View())
	}
	if s.loadErr != nil {
		return s.services.Styles.Err.Render("failed to list profiles: " + s.loadErr.Error())
	}
	return s.list.View()
}
