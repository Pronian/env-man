// Package tui implements the interactive layer-toggle interface built on
// Bubbletea. The Model's state transitions are pure functions of key events,
// so they can be unit-tested without a real terminal.
package tui

import (
	"fmt"
	"sort"
	"strings"

	"env-man/internal/config"
	"env-man/internal/state"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// item is one togglable overlay layer in the list.
type item struct {
	name    string
	enabled bool
}

// Model is the Bubbletea model for the layer-toggle screen.
type Model struct {
	paths   config.Paths
	items   []item
	cursor  int
	applied bool // true if the user pressed enter to apply
	width   int
	height  int
}

// NewModel builds the initial model. The display order is fixed for the
// session: currently applied layers in their saved priority order first, then
// any remaining available layers alphabetically. The base layer is implicit and
// not shown as a togglable item.
func NewModel(p config.Paths, st *state.File, all []string) Model {
	existing := make(map[string]bool, len(all))
	for _, l := range all {
		existing[l] = true
	}
	applied := make(map[string]bool, len(st.Layers))
	for _, l := range st.Layers {
		applied[l] = true
	}

	var order []string
	added := map[string]bool{}
	for _, l := range st.Layers {
		if l == config.BaseName || !existing[l] {
			continue
		}
		order = append(order, l)
		added[l] = true
	}
	var rest []string
	for _, l := range all {
		if l == config.BaseName || added[l] {
			continue
		}
		rest = append(rest, l)
	}
	sort.Strings(rest)
	order = append(order, rest...)

	items := make([]item, len(order))
	for i, l := range order {
		items[i] = item{name: l, enabled: applied[l]}
	}
	return Model{paths: p, items: items}
}

// Init satisfies tea.Model; the TUI performs no async I/O.
func (m Model) Init() tea.Cmd { return nil }

// Update handles terminal messages (resize and key presses).
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "shift+up", "K":
			m = m.moveItemUp()
		case "shift+down", "J":
			m = m.moveItemDown()
		case " ":
			if len(m.items) > 0 {
				m.items[m.cursor].enabled = !m.items[m.cursor].enabled
			}
		case "enter":
			m.applied = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// moveItemUp swaps the cursor item with the one above it and moves the cursor
// along, so the selection follows the item being reordered.
func (m Model) moveItemUp() Model {
	if m.cursor <= 0 || len(m.items) < 2 {
		return m
	}
	m.items[m.cursor-1], m.items[m.cursor] = m.items[m.cursor], m.items[m.cursor-1]
	m.cursor--
	return m
}

// moveItemDown swaps the cursor item with the one below it and moves the cursor
// along. Moving an item down raises its priority (applied later).
func (m Model) moveItemDown() Model {
	if m.cursor >= len(m.items)-1 || len(m.items) < 2 {
		return m
	}
	m.items[m.cursor+1], m.items[m.cursor] = m.items[m.cursor], m.items[m.cursor+1]
	m.cursor++
	return m
}

// View renders the screen.
func (m Model) View() string {
	var b strings.Builder

	title := lipgloss.NewStyle().Bold(true).Render("env-man") +
		"  " + lipgloss.NewStyle().Faint(true).Render("toggle layers with space, press enter to apply")
	b.WriteString(title)
	b.WriteString("\n")
	b.WriteString(strings.Repeat("─", max(40, 1)))
	b.WriteString("\n")

	hint := lipgloss.NewStyle().Faint(true).Render("base is always applied; layers below run low → high priority (reorder to change)")
	b.WriteString(hint)
	b.WriteString("\n\n")

	// base layer: always on, locked
	b.WriteString(renderRow(false, true, true, config.BaseName) + "\n")
	for i, it := range m.items {
		b.WriteString(renderRow(i == m.cursor, it.enabled, false, it.name) + "\n")
	}

	b.WriteString("\n")
	help := lipgloss.NewStyle().Faint(true).Render("j/k or ↑/↓ move  ·  J/K or shift+↑/↓ reorder  ·  space toggle  ·  enter apply  ·  q quit")
	b.WriteString(help)
	b.WriteString("\n")
	return b.String()
}

// renderRow formats a single layer row.
func renderRow(cursor, enabled, locked bool, name string) string {
	ind := " "
	if cursor {
		ind = ">"
	}
	box := "[ ]"
	if enabled {
		box = "[x]"
	}
	suffix := ""
	if locked {
		suffix = "  (locked)"
	}
	return fmt.Sprintf("%s %s %s%s", ind, box, name, suffix)
}

// Applied reports whether the user chose to apply the current selection.
func (m Model) Applied() bool { return m.applied }

// Selected returns the enabled layer names in display (priority) order.
func (m Model) Selected() []string {
	var sel []string
	for _, it := range m.items {
		if it.enabled {
			sel = append(sel, it.name)
		}
	}
	return sel
}

// Cursor returns the current cursor index (useful for tests).
func (m Model) Cursor() int { return m.cursor }

// Items returns a copy of the current items (useful for tests).
func (m Model) Items() []item {
	out := make([]item, len(m.items))
	copy(out, m.items)
	return out
}

// Run launches the interactive program and returns the final model.
func Run(p config.Paths, st *state.File, all []string) (Model, error) {
	m := NewModel(p, st, all)
	prog := tea.NewProgram(m, tea.WithAltScreen())
	final, err := prog.Run()
	if err != nil {
		return Model{}, err
	}
	if fm, ok := final.(Model); ok {
		return fm, nil
	}
	return m, nil
}
