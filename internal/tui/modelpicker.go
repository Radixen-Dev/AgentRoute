// SPDX-License-Identifier: GPL-3.0-only

package tui

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Radixen-Dev/AgentRoute/internal/openrouter"
	"github.com/Radixen-Dev/AgentRoute/internal/secret"
	"github.com/Radixen-Dev/AgentRoute/internal/tui/theme"
)

// ── list item ────────────────────────────────────────────────────────────────

type modelItem struct{ m openrouter.Model }

func (i modelItem) Title() string       { return i.m.Name }
func (i modelItem) Description() string { return i.m.ID }
func (i modelItem) FilterValue() string { return i.m.ID + " " + i.m.Name }

// ── custom delegate ───────────────────────────────────────────────────────────

// richModelDelegate renders each OpenRouter model as a 2-line row:
//
//	[provider]  Display Name
//	  ctx 128K  in $0.30/M  out $1.50/M
type richModelDelegate struct {
	styles theme.Styles
}

func (d richModelDelegate) Height() int                               { return 2 }
func (d richModelDelegate) Spacing() int                              { return 1 }
func (d richModelDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd  { return nil }

func (d richModelDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	mi, ok := item.(modelItem)
	if !ok {
		return
	}
	sel := index == m.Index()
	s := d.styles

	provider := modelProvider(mi.m.ID)
	name := mi.m.Name
	if name == "" {
		name = mi.m.ID
	}
	meta := fmt.Sprintf("ctx %-6s  in %-9s  out %s",
		formatCtx(mi.m.ContextLength),
		formatPricePerM(mi.m.Pricing.Prompt),
		formatPricePerM(mi.m.Pricing.Completion),
	)

	if sel {
		badge := s.OK.Render("[" + provider + "]")
		line1 := badge + "  " + lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(name)
		line2 := "  " + s.Muted.Render(meta)
		fmt.Fprintf(w, "%s\n%s", line1, line2)
	} else {
		badge := s.Muted.Render("[" + provider + "]")
		line1 := badge + "  " + name
		line2 := "  " + s.Muted.Render(meta)
		fmt.Fprintf(w, "%s\n%s", line1, line2)
	}
}

// ── screen ────────────────────────────────────────────────────────────────────

type modelPickerScreen struct {
	services    *Services
	list        list.Model
	delegate    richModelDelegate
	loading     bool
	loadErr     error
	width       int
	height      int
	listWidth   int
	detailWidth int
}

// splitThreshold is the minimum terminal width at which the two-panel layout
// (list + detail card) is shown instead of the single-column list.
const splitThreshold = 100

func newModelPickerScreen(services *Services) Screen {
	d := richModelDelegate{styles: services.Styles}
	l := list.New(nil, d, 0, 0)
	l.Title = "Pick a model"
	l.SetShowHelp(false)
	return &modelPickerScreen{services: services, list: l, delegate: d}
}

func (s *modelPickerScreen) Title() string { return titleFor(ScreenModelPicker) }

// CapturingInput implements InputCapturer: while the list's filter input is
// active, all keys must reach the list component, not the global keymap.
func (s *modelPickerScreen) CapturingInput() bool {
	return s.list.FilterState() == list.Filtering
}

func (s *modelPickerScreen) Bindings() []key.Binding {
	return []key.Binding{
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "choose model")),
		key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
		key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "refresh catalog")),
	}
}

// recalcLayout sets listWidth and detailWidth from the current terminal width.
func (s *modelPickerScreen) recalcLayout() {
	if s.width < splitThreshold {
		s.listWidth = s.width
		s.detailWidth = 0
		return
	}
	s.detailWidth = min(46, s.width/3)
	s.listWidth = s.width - s.detailWidth - 2 // 2 = gap
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
			return modelsLoadedMsg{err: fmt.Errorf("no OpenRouter API key configured; run: agentroute key set --value <key>")}
		}
		client := services.NewOpenRouterClient(apiKey)
		models, err := client.FetchModels(context.Background())
		return modelsLoadedMsg{models: models, err: err}
	}
}

func (s *modelPickerScreen) Init() tea.Cmd {
	if len(s.services.CachedModels) > 0 {
		return s.setItems(s.services.CachedModels)
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
		s.width, s.height = msg.Width, msg.Height
		s.recalcLayout()
		s.list.SetSize(s.listWidth, s.height)
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

	listView := s.list.View()

	if s.detailWidth <= 0 {
		return listView
	}

	// Detail panel for the selected model
	detailContent := styles.Muted.Render("navigate to a model to see details")
	if item, ok := s.list.SelectedItem().(modelItem); ok {
		detailContent = renderModelDetail(item.m, styles)
	}
	detailPanel := styles.Card.Width(s.detailWidth - 2).Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listView, "  ", detailPanel)
}

// renderModelDetail builds the content for the right-hand detail panel.
func renderModelDetail(m openrouter.Model, s theme.Styles) string {
	var b strings.Builder

	b.WriteString(s.CardTitle.Render("Model") + "\n\n")

	name := m.Name
	if name == "" {
		name = m.ID
	}
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(theme.Text).Render(name) + "\n\n")

	b.WriteString(s.Muted.Render("ID") + "\n")
	b.WriteString(m.ID + "\n\n")

	b.WriteString(s.Muted.Render("Context") + "\n")
	b.WriteString(formatCtx(m.ContextLength) + " tokens\n\n")

	b.WriteString(s.Muted.Render("Pricing  (per 1M tokens)") + "\n")
	b.WriteString(fmt.Sprintf("  prompt:      %s\n", formatPricePerM(m.Pricing.Prompt)))
	b.WriteString(fmt.Sprintf("  completion:  %s\n", formatPricePerM(m.Pricing.Completion)))

	return b.String()
}

// ── helpers ───────────────────────────────────────────────────────────────────

// modelProvider extracts the provider prefix from an OpenRouter model ID
// (e.g. "anthropic/claude-opus-4.5" → "anthropic").
func modelProvider(id string) string {
	if idx := strings.Index(id, "/"); idx >= 0 {
		return id[:idx]
	}
	return id
}

// formatCtx converts a context-length token count to a short human string.
func formatCtx(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%dM", n/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%dK", n/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

// formatPricePerM converts an OpenRouter per-token price string to a
// dollars-per-million-tokens display string, e.g. "0.000003" → "$3.00/M".
func formatPricePerM(s string) string {
	if s == "" {
		return "–"
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f == 0 {
		return "free"
	}
	perM := f * 1_000_000
	if perM < 0.01 {
		return fmt.Sprintf("$%.4f/M", perM)
	}
	if perM < 1 {
		return fmt.Sprintf("$%.3f/M", perM)
	}
	return fmt.Sprintf("$%.2f/M", perM)
}
