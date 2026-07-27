package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGitTUIModel(t *testing.T) {
	m := newGitModel(".")
	if !m.gitMode {
		t.Error("expected gitMode to be true")
	}
	if !m.gitTreeOpen {
		t.Error("expected gitTreeOpen to be true initially")
	}

	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = m2.(model)
	if m.gitTreeOpen {
		t.Error("expected gitTreeOpen to be false after pressing t")
	}

	m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	m = m3.(model)
	if !m.gitTreeOpen {
		t.Error("expected gitTreeOpen to be true after pressing t again")
	}
}

func TestOverlayAnsi(t *testing.T) {
	bg := "\x1b[31mhello world\x1b[0m"
	fg := "\x1b[32mabc\x1b[0m"

	result := overlayAnsi(bg, fg, 2)
	cells := parseAnsi(result)

	if len(cells) != 11 {
		t.Fatalf("expected 11 cells, got %d", len(cells))
	}

	expectedRunes := []rune("heabc world")
	for i, cell := range cells {
		if cell.char != expectedRunes[i] {
			t.Errorf("at index %d, expected rune %c, got %c", i, expectedRunes[i], cell.char)
		}
	}

	if cells[0].style != "\x1b[31m" {
		t.Errorf("expected cell 0 to have red style, got %q", cells[0].style)
	}
	if cells[2].style != "\x1b[32m" {
		t.Errorf("expected cell 2 to have green style, got %q", cells[2].style)
	}
	if cells[5].style != "\x1b[31m" {
		t.Errorf("expected cell 5 to have red style, got %q", cells[5].style)
	}
}
