package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMouseInteraction(t *testing.T) {
	srcBytes := []byte("line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n")
	dstBytes := []byte("line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 10 // contentHeight = 10 - 1 (title) - 1 (status) = 8
	m.ready = true
	m.openAllFolds()

	t.Run("MouseWheelDown scrolls down", func(t *testing.T) {
		m.scrollY = 0
		m2, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
		m = m2.(model)
		if m.scrollY != 3 {
			t.Errorf("expected scrollY to be 3 after MouseWheelDown, got %d", m.scrollY)
		}
	})

	t.Run("MouseWheelUp scrolls up", func(t *testing.T) {
		m2, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
		m = m2.(model)
		if m.scrollY != 0 {
			t.Errorf("expected scrollY to be 0 after MouseWheelUp, got %d", m.scrollY)
		}
	})

	t.Run("Left click switches pane and moves cursor", func(t *testing.T) {
		m.activePane = "left"
		m.cursorY = 0

		// The width is 80, so the divider sits at 39.
		// The left pane takes X from 0 to 38, and the right pane takes 40 to 79.
		// We click Y=3 on X=50, which falls in the right pane.
		m2, _ := m.handleMouse(tea.MouseMsg{
			X:      50,
			Y:      3,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		m = m2.(model)

		if m.activePane != "right" {
			t.Errorf("expected activePane to switch to 'right', got %s", m.activePane)
		}
		if m.cursorY != 2 {
			t.Errorf("expected cursorY to be 2, got %d", m.cursorY)
		}
	})

	t.Run("Left click on left pane", func(t *testing.T) {
		m2, _ := m.handleMouse(tea.MouseMsg{
			X:      10,
			Y:      2,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		m = m2.(model)

		if m.activePane != "left" {
			t.Errorf("expected activePane to switch to 'left', got %s", m.activePane)
		}
		if m.cursorY != 1 {
			t.Errorf("expected cursorY to be 1, got %d", m.cursorY)
		}
	})

	t.Run("Horizontal scroll via MouseWheelRight", func(t *testing.T) {
		m.scrollXLeft = 0
		m.scrollXRight = 0
		m2, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelRight})
		m = m2.(model)

		m.srcLines = []string{"very long line indeed that exceeds terminal width so it can be horizontally scrolled"}
		m.dstLines = []string{"very long line indeed that exceeds terminal width so it can be horizontally scrolled"}
		m.scrollXLeft = 0
		m.scrollXRight = 0
		m3, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelRight})
		m = m3.(model)
		if m.scrollXLeft != 4 {
			t.Errorf("expected scrollXLeft to be 4 after horizontal scroll, got %d", m.scrollXLeft)
		}
	})
}

func TestMouseFoldInteraction(t *testing.T) {
	srcBytes := []byte("line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n")
	dstBytes := []byte("line 1\nline 2\nline 3\nline 4\nline 5\nline 6\nline 7\nline 8\nline 9\nline 10\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 10
	m.ready = true

	t.Run("initial closed fold", func(t *testing.T) {
		if len(m.folds) != 1 {
			t.Fatalf("expected 1 fold, got %d", len(m.folds))
		}
		if m.folds[0].open {
			t.Fatal("expected fold to be collapsed initially")
		}
		if len(m.virtualLines) != 1 {
			t.Fatalf("expected 1 virtual line (the fold marker), got %d", len(m.virtualLines))
		}
		if m.virtualLines[0].foldIdx != 0 {
			t.Errorf("expected virtual line 0 to be a fold marker, got %+v", m.virtualLines[0])
		}
	})

	t.Run("click fold marker to expand", func(t *testing.T) {
		m2, _ := m.handleMouse(tea.MouseMsg{
			X:      10,
			Y:      1,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		m = m2.(model)

		if !m.folds[0].open {
			t.Error("expected fold to be expanded after click")
		}
		if len(m.virtualLines) <= 1 {
			t.Errorf("expected multiple virtual lines after fold expansion, got %d", len(m.virtualLines))
		}
	})

	t.Run("click inside expanded fold remains open", func(t *testing.T) {
		m2, _ := m.handleMouse(tea.MouseMsg{
			X:      10,
			Y:      2,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		m = m2.(model)
		if !m.folds[0].open {
			t.Error("expected fold to remain open when clicking code area")
		}
	})

	t.Run("click gutter to collapse fold", func(t *testing.T) {
		m2, _ := m.handleMouse(tea.MouseMsg{
			X:      1,
			Y:      2,
			Action: tea.MouseActionPress,
			Button: tea.MouseButtonLeft,
		})
		m = m2.(model)
		if m.folds[0].open {
			t.Error("expected fold to be collapsed after clicking gutter fold indicator")
		}
	})
}
