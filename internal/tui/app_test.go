// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderSplashShowsWordmarkAndVersion(t *testing.T) {
	services := testServices(t)
	state := newSplashState()
	out := renderSplash(services.Styles, 80, 24, state)
	if !strings.Contains(out, "AgentRoute") && !strings.Contains(out, "Agent") {
		t.Fatalf("splash output missing wordmark:\n%s", out)
	}
}

func TestRenderHelpOverlayListsGlobalKeysAndAllScreens(t *testing.T) {
	services := testServices(t)
	dashboard := newDashboardScreen(&services)
	out := renderHelpOverlay(services.Styles, 100, 30, DefaultKeyMap(), dashboard)

	for _, want := range []string{"quit", "back", "filter"} {
		if !strings.Contains(out, want) {
			t.Errorf("help overlay missing global key %q:\n%s", want, out)
		}
	}
	for _, id := range screenOrder {
		if !strings.Contains(out, titleFor(id)) {
			t.Errorf("help overlay missing screen %q:\n%s", titleFor(id), out)
		}
	}
}

func TestDashboardViewWhenGatewayDownAndNoProfile(t *testing.T) {
	services := testServices(t)
	d := newDashboardScreen(&services)
	d.Init()
	d.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	out := d.View()
	if !strings.Contains(out, "down") {
		t.Errorf("expected dashboard to report gateway down:\n%s", out)
	}
	if !strings.Contains(out, "Profiles") {
		t.Errorf("expected dashboard to point at Profiles when none is active:\n%s", out)
	}
}

func TestRootModelBootsDirectlyToDashboardWhenSplashSkipped(t *testing.T) {
	services := testServices(t)
	m := New(services, true)

	var model tea.Model = m
	model = drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})

	view := model.View()
	if !strings.Contains(view, "AgentRoute") {
		t.Fatalf("expected header wordmark in view, got:\n%s", view)
	}
	if !strings.Contains(view, "Dashboard") {
		t.Fatalf("expected to land on Dashboard, got:\n%s", view)
	}
}

func TestRootModelHelpOverlayTogglesOnQuestionMark(t *testing.T) {
	services := testServices(t)
	m := New(services, true)
	var model tea.Model = m
	model = drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})

	model = drive(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	view := model.View()
	if !strings.Contains(view, "Keymap") {
		t.Fatalf("expected help overlay after '?', got:\n%s", view)
	}

	model = drive(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	view = model.View()
	if strings.Contains(view, "Keymap") {
		t.Fatalf("expected help overlay to close on second '?', got:\n%s", view)
	}
}

func TestRootModelNumberKeyNavigatesAndEscGoesBack(t *testing.T) {
	services := testServices(t)
	m := New(services, true)
	var model tea.Model = m
	model = drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})

	model = drive(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")}) // Profiles
	if !strings.Contains(model.View(), "Profiles") {
		t.Fatalf("expected Profiles screen after '2', got:\n%s", model.View())
	}

	model = drive(t, model, tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(model.View(), "Dashboard") {
		t.Fatalf("expected Esc to return to Dashboard, got:\n%s", model.View())
	}
}

func TestRootModelQuitReturnsQuitCmd(t *testing.T) {
	services := testServices(t)
	m := New(services, true)
	var model tea.Model = m
	model = drive(t, model, tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatalf("expected a non-nil cmd from 'q'")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Fatalf("expected tea.QuitMsg, got %T", msg)
	}
}
