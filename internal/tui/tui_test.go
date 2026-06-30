package tui

import (
	"testing"

	"env-man/internal/config"
	"env-man/internal/state"

	"github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mkState(layers []string) *state.File {
	f := state.New("")
	f.SetLayers(layers)
	return f
}

// mkStateOrder builds a state file with both an applied layer stack and a full
// saved ordering (including disabled layers).
func mkStateOrder(layers, order []string) *state.File {
	f := state.New("")
	f.SetLayers(layers)
	f.SetOrder(order)
	return f
}

func TestNewModel_OrderAppliedFirstThenRestSorted(t *testing.T) {
	p := config.Paths{}
	st := mkState([]string{"dev", "local"})
	all := []string{"base", "dev", "local", "staging", "prod"}

	m := NewModel(p, st, all)

	names := make([]string, len(m.items))
	for i, it := range m.items {
		names[i] = it.name
	}
	assert.Equal(t, []string{"dev", "local", "prod", "staging"}, names)
	// enabled flags
	enabled := map[string]bool{}
	for _, it := range m.items {
		enabled[it.name] = it.enabled
	}
	assert.True(t, enabled["dev"])
	assert.True(t, enabled["local"])
	assert.False(t, enabled["prod"])
	assert.False(t, enabled["staging"])
}

func TestNewModel_HiddenAppliedButMissingFromDisk(t *testing.T) {
	// A layer in state.Layers but with no folder (not in all) is hidden.
	p := config.Paths{}
	st := mkState([]string{"dev", "ghost"})
	all := []string{"base", "dev", "staging"}

	m := NewModel(p, st, all)
	names := make([]string, len(m.items))
	for i, it := range m.items {
		names[i] = it.name
	}
	assert.Equal(t, []string{"dev", "staging"}, names)
}

func TestNewModel_EmptyWhenOnlyBase(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base"})
	assert.Empty(t, m.items)
	assert.Empty(t, m.Selected())
}

func TestNewModel_UsesSavedOrderKeepingDisabledPositions(t *testing.T) {
	// Saved order interleaves disabled layers (staging, prod) with enabled
	// ones (dev, local). The TUI must preserve those positions rather than
	// dropping disabled layers into the alphabetical tail.
	p := config.Paths{}
	st := mkStateOrder([]string{"dev", "local"}, []string{"dev", "staging", "local", "prod"})
	all := []string{"base", "dev", "local", "staging", "prod"}

	m := NewModel(p, st, all)

	assert.Equal(t, []string{"dev", "staging", "local", "prod"}, itemNames(m))
	enabled := map[string]bool{}
	for _, it := range m.items {
		enabled[it.name] = it.enabled
	}
	assert.True(t, enabled["dev"])
	assert.True(t, enabled["local"])
	assert.False(t, enabled["staging"])
	assert.False(t, enabled["prod"])
}

func TestNewModel_SavedOrderAppendsNewFoldersAlphabetically(t *testing.T) {
	// "newlayer" exists on disk but not in the saved order: it is appended
	// alphabetically after the saved entries.
	p := config.Paths{}
	st := mkStateOrder([]string{"dev"}, []string{"dev", "local"})
	all := []string{"base", "dev", "local", "newlayer"}

	m := NewModel(p, st, all)
	assert.Equal(t, []string{"dev", "local", "newlayer"}, itemNames(m))
}

func TestNewModel_SavedOrderDropsGhostEntries(t *testing.T) {
	// "ghost" is in the saved order but has no folder: hidden, like applied
	// ghosts.
	p := config.Paths{}
	st := mkStateOrder([]string{"dev"}, []string{"dev", "ghost", "local"})
	all := []string{"base", "dev", "local"}

	m := NewModel(p, st, all)
	assert.Equal(t, []string{"dev", "local"}, itemNames(m))
}

func TestSelected_ReturnsEnabledInOrder(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState([]string{"dev"}), []string{"base", "dev", "local", "staging"})
	assert.Equal(t, []string{"dev"}, m.Selected())
}

func TestOrder_ReturnsAllItemsIncludingDisabled(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkStateOrder([]string{"dev"}, []string{"dev", "staging", "local"}),
		[]string{"base", "dev", "local", "staging"})
	// Includes disabled layers (staging, local) in display order.
	assert.Equal(t, []string{"dev", "staging", "local"}, m.Order())
	// Toggling a layer off must not remove it from Order().
	m = update(m, tea.KeyMsg{Type: tea.KeySpace}) // toggle dev off at cursor 0
	assert.Equal(t, []string{"dev", "staging", "local"}, m.Order())
	assert.Empty(t, m.Selected())
}

// update sends a key to the model and returns the resulting model.
func update(m Model, key tea.KeyMsg) Model {
	mm, _ := m.Update(key)
	return mm.(Model)
}

func TestUpdate_Toggle(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState([]string{"dev"}), []string{"base", "dev", "local", "staging"})
	// cursor at 0 (dev, enabled). Toggle off.
	m = update(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.False(t, m.Items()[0].enabled)
	assert.Empty(t, m.Selected())

	// move down to local (index 1) and toggle on
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(m, tea.KeyMsg{Type: tea.KeySpace})
	assert.Equal(t, []string{"local"}, m.Selected())
}

func TestUpdate_CursorMovement(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b", "c"})
	// cursor 0 -> down 2 -> cursor 2
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.Cursor())
	// down at end is a no-op
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, m.Cursor())
	// up -> cursor 1
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 1, m.Cursor())
	// vim-style j/k
	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 2, m.Cursor())
	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 1, m.Cursor())
}

func TestUpdate_UpAtTopIsNoOp(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b"})
	m = update(m, tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, m.Cursor())
}

func TestUpdate_EnterAppliesAndQuits(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState([]string{"dev"}), []string{"base", "dev", "local"})
	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	require.NotNil(t, cmd, "enter should return a quit command")
	m2 := mm.(Model)
	assert.True(t, m2.Applied())
}

func TestUpdate_QuitKeysQuitWithoutApplying(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState([]string{"dev"}), []string{"base", "dev", "local"})
	for _, k := range []tea.KeyMsg{
		{Type: tea.KeyEsc},
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
	} {
		mm, cmd := m.Update(k)
		assert.NotNil(t, cmd, "%v should quit", k)
		assert.False(t, mm.(Model).Applied(), "%v should not apply", k)
	}
}

func TestUpdate_ToggleWhenEmptyIsNoOp(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base"})
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m2 := mm.(Model)
	assert.Empty(t, m2.items)
}

func TestView_IncludesBaseAndLayers(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState([]string{"dev"}), []string{"base", "dev", "local"})
	v := m.View()
	assert.Contains(t, v, "base")
	assert.Contains(t, v, "dev")
	assert.Contains(t, v, "local")
	assert.Contains(t, v, "(locked)")
}

// itemNames returns the model's item names in display order.
func itemNames(m Model) []string {
	out := make([]string, len(m.items))
	for i, it := range m.items {
		out[i] = it.name
	}
	return out
}

func TestUpdate_MoveItemDown_ShiftDown(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b", "c"})
	// cursor at 0 ("a"); move down -> "a" swaps with "b", cursor follows.
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	assert.Equal(t, []string{"b", "a", "c"}, itemNames(m))
	assert.Equal(t, 1, m.Cursor())
}

func TestUpdate_MoveItemUp_ShiftUp(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b", "c"})
	// move cursor to 2 ("c"), then move up -> swaps with "b".
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftUp})
	assert.Equal(t, []string{"a", "c", "b"}, itemNames(m))
	assert.Equal(t, 1, m.Cursor())
}

func TestUpdate_MoveItemWithUppercaseJK(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b", "c"})

	// uppercase J = move item down (reorder), like shift+down
	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
	assert.Equal(t, []string{"b", "a", "c"}, itemNames(m))

	// uppercase K = move item up (reorder); cursor is now at 1 ("a")
	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	assert.Equal(t, []string{"a", "b", "c"}, itemNames(m))
	assert.Equal(t, 0, m.Cursor())
}

func TestUpdate_LowercaseJK_OnlyMovesCursor(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b", "c"})
	before := itemNames(m)

	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	assert.Equal(t, 1, m.Cursor())
	assert.Equal(t, before, itemNames(m), "lowercase j must NOT reorder")

	m = update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	assert.Equal(t, 0, m.Cursor())
	assert.Equal(t, before, itemNames(m), "lowercase k must NOT reorder")
}

func TestUpdate_MoveAtBoundariesIsNoOp(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base", "a", "b"})
	original := itemNames(m)

	// at top: shift+up does nothing
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftUp})
	assert.Equal(t, original, itemNames(m))
	assert.Equal(t, 0, m.Cursor())

	// move to bottom, shift+down does nothing
	m = update(m, tea.KeyMsg{Type: tea.KeyDown})
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	assert.Equal(t, original, itemNames(m))
	assert.Equal(t, 1, m.Cursor())
}

func TestReorder_ChangesSelectedPriority(t *testing.T) {
	p := config.Paths{}
	// both dev and local enabled; dev is lower priority (index 0).
	m := NewModel(p, mkState([]string{"dev", "local"}), []string{"base", "dev", "local"})
	assert.Equal(t, []string{"dev", "local"}, m.Selected())

	// cursor on dev (0), move it below local -> local now lower priority.
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	assert.Equal(t, []string{"local", "dev"}, m.Selected())
}

func TestReorder_EmptyListIsNoOp(t *testing.T) {
	p := config.Paths{}
	m := NewModel(p, mkState(nil), []string{"base"})
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftDown})
	m = update(m, tea.KeyMsg{Type: tea.KeyShiftUp})
	assert.Empty(t, m.items)
}
