// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
)

type modelItem struct{ m openrouter.Model }

func (i modelItem) Title() string { return i.m.ID }
func (i modelItem) Description() string {
	return fmt.Sprintf("%s · ctx %d · in $%s/out $%s", i.m.Name, i.m.ContextLength, i.m.Pricing.Prompt, i.m.Pricing.Completion)
}
func (i modelItem) FilterValue() string { return i.m.ID + " " + i.m.Name }

type modelPickerScreen struct {
	services *Services
	list     list.Model
	loading  bool
	loadErr  error
}

func newModelPickerScreen(services *Services) Screen {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Pick a model"
	l.SetShowHelp(false)
	return &modelPickerScreen{services: services, list: l}
}

func (s *modelPickerScreen) Title() string { return titleFor(ScreenModelPicker) }

func (s *modelPickerScreen) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose model")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh catalog")),
	}
}

type modelsLoadedMsg struct {
	models []openrouter.Model
	err    error
}

func fetchModelsCmd(services *Services) tea.Cmd {
	return func() tea.Msg {
		apiKey, _, err := secret.OpenRouterAPIKey()
		if err != nil {
			return modelsLoadedMsg{err: err}
		}
		if apiKey == "" {
			return modelsLoadedMsg{err: fmt.Errorf("no OpenRouter API key configured; set one from a terminal: agentroute key set --value <key>")}
		}
		client := services.NewOpenRouterClient(apiKey)
		models, err := client.FetchModels(context.Background())
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (s *modelPickerScreen) Init() tea.Cmd {
	if len(s.services.CachedModels) > 0 {
		s.setItems(s.services.CachedModels)
		return nil
	}
	s.loading = true
	return fetchModelsCmd(s.services)
}

func (s *modelPickerScreen) setItems(models []openrouter.Model) tea.Cmd {
	items := make([]list.Item, len(models))
	for i, m := range models {
		items[i] = modelItem{m: m}
	}
	return s.list.SetItems(items)
}

func (s *modelPickerScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.list.SetSize(msg.Width, msg.Height)
		return s, nil

	case modelsLoadedMsg:
		s.loading = false
		s.loadErr = msg.err
		if msg.err != nil {
			return s, nil
		}
		s.services.CachedModels = msg.models
		return s, s.setItems(msg.models)

	case tea.KeyMsg:
		if s.list.FilterState() == list.Filtering {
			break
		}
		switch msg.String() {
		case "r":
			s.loading = true
			s.loadErr = nil
			return s, fetchModelsCmd(s.services)
		case "enter":
			item, ok := s.list.SelectedItem().(modelItem)
			if !ok {
				return s, nil
			}
			if s.services.EditingProfile.Models == nil {
				s.services.EditingProfile.Models = map[string]string{}
			}
			s.services.EditingProfile.Models[s.services.PickerTier] = item.m.ID
			return s, func() tea.Msg { return backMsg{} }
		}
	}

	var cmd tea.Cmd
	s.list, cmd = s.list.Update(msg)
	return s, cmd
}

func (s *modelPickerScreen) View() string {
	styles := s.services.Styles
	if s.loading {
		return styles.Muted.Render("fetching OpenRouter catalog...")
	}
	if s.loadErr != nil {
		return styles.Err.Render(s.loadErr.Error())
	}
	return s.list.View()
}
