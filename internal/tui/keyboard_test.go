package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestKeyboardNavigation(t *testing.T) {
	srcBytes := []byte("hello world this is a test line\nanother line here\nthird line is longer\n")
	dstBytes := []byte("hello world this is a test line\nanother line here\nthird line is longer\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 20
	m.ready = true
	m.openAllFolds()

	t.Run("Initial position", func(t *testing.T) {
		if m.cursorY != 0 || m.cursorX != 0 {
			t.Errorf("expected cursor at (0,0), got (%d,%d)", m.cursorY, m.cursorX)
		}
	})

	t.Run("Move down one line using 'j'", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = m2.(model)
		if m.cursorY != 1 {
			t.Errorf("expected cursorY to be 1 after 'j', got %d", m.cursorY)
		}
	})

	t.Run("Count prefix navigation", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		m = m2.(model)
		m2, _ = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		m = m2.(model)
		if m.cursorY != 0 {
			t.Errorf("expected cursorY to be 0 after '2k', got %d", m.cursorY)
		}
		if m.digitBuffer != "" {
			t.Errorf("expected digitBuffer to be cleared, got %q", m.digitBuffer)
		}
	})

	t.Run("Word navigation 'w'", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
		m = m2.(model)
		if m.cursorX != 6 {
			t.Errorf("expected cursorX to be 6 after 'w', got %d", m.cursorX)
		}
	})

	t.Run("Word navigation 'b'", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
		m = m2.(model)
		if m.cursorX != 0 {
			t.Errorf("expected cursorX to be 0 after 'b', got %d", m.cursorX)
		}
	})

	t.Run("Line end '$'", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'$'}})
		m = m2.(model)
		if m.cursorX != 30 {
			t.Errorf("expected cursorX to be 30 after '$', got %d", m.cursorX)
		}
	})

	t.Run("Line start '0'", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
		m = m2.(model)
		if m.cursorX != 0 {
			t.Errorf("expected cursorX to be 0 after '0', got %d", m.cursorX)
		}
	})

	t.Run("Switch active pane 'tab'", func(t *testing.T) {
		if m.activePane != "left" {
			t.Errorf("expected activePane to be 'left' initially, got %s", m.activePane)
		}
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyTab})
		m = m2.(model)
		if m.activePane != "right" {
			t.Errorf("expected activePane to switch to 'right' after Tab, got %s", m.activePane)
		}
	})
}
