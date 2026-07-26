package tui

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	tea "github.com/charmbracelet/bubbletea"
)

func TestComputeFolds(t *testing.T) {
	t.Run("Empty file", func(t *testing.T) {
		folds := computeFolds(nil, 0, 3)
		if len(folds) != 0 {
			t.Errorf("expected 0 folds for empty file, got %d", len(folds))
		}
	})

	t.Run("File with no changes", func(t *testing.T) {
		folds := computeFolds(nil, 10, 3)
		if len(folds) != 1 {
			t.Fatalf("expected 1 fold for unchanged file, got %d", len(folds))
		}
		if folds[0].startLine != 0 || folds[0].endLine != 9 {
			t.Errorf("expected fold to cover entire file 0..9, got %+v", folds[0])
		}
	})

	t.Run("Single change in middle", func(t *testing.T) {
		folds := computeFolds([]int{5}, 10, 2)
		if len(folds) != 2 {
			t.Fatalf("expected 2 folds, got %d", len(folds))
		}
		if folds[0].startLine != 0 || folds[0].endLine != 2 {
			t.Errorf("expected first fold 0..2, got %+v", folds[0])
		}
		if folds[1].startLine != 8 || folds[1].endLine != 9 {
			t.Errorf("expected second fold 8..9, got %+v", folds[1])
		}
	})
}

func TestBuildVirtualLines(t *testing.T) {
	alignment := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: 3, RightLine: 3},
		{LeftLine: 4, RightLine: 4},
	}

	t.Run("All folds closed", func(t *testing.T) {
		folds := []fold{
			{startLine: 1, endLine: 3, open: false},
		}
		vlines := buildVirtualLines(folds, len(alignment), alignment)
		if len(vlines) != 3 {
			t.Fatalf("expected 3 virtual lines, got %d", len(vlines))
		}
		if vlines[0].alignedRow != 0 || vlines[0].foldIdx != -1 {
			t.Errorf("row 0 incorrect: %+v", vlines[0])
		}
		if vlines[1].foldIdx != 0 || vlines[1].alignedRow != -1 {
			t.Errorf("row 1 incorrect (should be fold marker): %+v", vlines[1])
		}
		if vlines[2].alignedRow != 4 || vlines[2].foldIdx != -1 {
			t.Errorf("row 2 incorrect: %+v", vlines[2])
		}
	})

	t.Run("Fold is open", func(t *testing.T) {
		folds := []fold{
			{startLine: 1, endLine: 3, open: true},
		}
		vlines := buildVirtualLines(folds, len(alignment), alignment)
		if len(vlines) != 5 {
			t.Errorf("expected 5 virtual lines for open fold, got %d", len(vlines))
		}

		vIdx := realToVirtual(vlines, folds, 2)
		if vIdx != 2 {
			t.Errorf("expected realToVirtual(2) to be 2, got %d", vIdx)
		}
	})

	t.Run("Closed fold realToVirtual", func(t *testing.T) {
		folds := []fold{
			{startLine: 1, endLine: 3, open: false},
		}
		vlines := buildVirtualLines(folds, len(alignment), alignment)
		vIdx := realToVirtual(vlines, folds, 2)
		if vIdx != 1 {
			t.Errorf("expected realToVirtual(2) inside closed fold to map to fold marker row 1, got %d", vIdx)
		}

		fi := foldAtVirtual(vlines, folds, 1)
		if fi != 0 {
			t.Errorf("expected foldAtVirtual(1) to be 0, got %d", fi)
		}
	})
}

func TestFoldGutterHelpers(t *testing.T) {
	t.Run("foldStartingAt", func(t *testing.T) {
		m := model{
			folds: []fold{
				{startLine: 2, endLine: 5, open: true},
				{startLine: 8, endLine: 10, open: false},
			},
		}

		if idx := m.foldStartingAt(2); idx != 0 {
			t.Errorf("expected 0 for open fold starting at 2, got %d", idx)
		}
		if idx := m.foldStartingAt(8); idx != -1 {
			t.Errorf("expected -1 for closed fold, got %d", idx)
		}
		if idx := m.foldStartingAt(3); idx != -1 {
			t.Errorf("expected -1 for row not at fold start, got %d", idx)
		}
		if idx := m.foldStartingAt(12); idx != -1 {
			t.Errorf("expected -1 for row outside folds, got %d", idx)
		}

		m2 := model{folds: nil}
		if idx := m2.foldStartingAt(2); idx != -1 {
			t.Errorf("expected -1 when no folds, got %d", idx)
		}
	})

	t.Run("foldContaining", func(t *testing.T) {
		m := model{
			folds: []fold{
				{startLine: 2, endLine: 5, open: true},
				{startLine: 8, endLine: 10, open: false},
			},
		}

		if idx := m.foldContaining(3); idx != 0 {
			t.Errorf("expected 0 for row inside open fold, got %d", idx)
		}
		if idx := m.foldContaining(1); idx != -1 {
			t.Errorf("expected -1 for row outside folds, got %d", idx)
		}
		if idx := m.foldContaining(9); idx != -1 {
			t.Errorf("expected -1 for row inside closed fold, got %d", idx)
		}

		m2 := model{folds: nil}
		if idx := m2.foldContaining(3); idx != -1 {
			t.Errorf("expected -1 when no folds, got %d", idx)
		}
	})
}

func TestFoldKeyboardShortcuts(t *testing.T) {
	srcBytes := []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")
	dstBytes := []byte("line1\nCHANGED\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.ready = true

	// We expect some folds here because only line 2 changed.
	if len(m.folds) == 0 {
		t.Fatalf("expected some folds to be created")
	}

	t.Run("za toggle", func(t *testing.T) {
		initialState := m.folds[0].open
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
		m = m2.(model)
		m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
		m = m2.(model)
		if m.folds[0].open == initialState {
			t.Errorf("expected fold[0] to toggle open state")
		}
	})

	t.Run("zR open all", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
		m = m2.(model)
		m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'R'}})
		m = m2.(model)
		for i, f := range m.folds {
			if !f.open {
				t.Errorf("expected fold %d to be open", i)
			}
		}
	})

	t.Run("zM close all", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'z'}})
		m = m2.(model)
		m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'M'}})
		m = m2.(model)
		for i, f := range m.folds {
			if f.open {
				t.Errorf("expected fold %d to be closed", i)
			}
		}
	})
}
