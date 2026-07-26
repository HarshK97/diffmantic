package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateMessageHandling(t *testing.T) {
	srcBytes := []byte("first line\nsecond line\n")
	dstBytes := []byte("first line\nsecond line\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)

	t.Run("initial ready state", func(t *testing.T) {
		if m.ready {
			t.Error("expected ready to be false initially")
		}
	})

	t.Run("WindowSizeMsg", func(t *testing.T) {
		m2, cmd := m.Update(tea.WindowSizeMsg{
			Width:  100,
			Height: 30,
		})
		m = m2.(model)

		if !m.ready {
			t.Error("expected ready to be true after WindowSizeMsg")
		}
		if m.width != 100 || m.height != 30 {
			t.Errorf("expected dimensions 100x30, got %dx%d", m.width, m.height)
		}
		if cmd != nil {
			t.Errorf("expected nil cmd, got %v", cmd)
		}
	})

	t.Run("resize clamping", func(t *testing.T) {
		m.scrollY = 200
		m3, _ := m.Update(tea.WindowSizeMsg{
			Width:  100,
			Height: 30,
		})
		m = m3.(model)
		if m.scrollY > m.maxScrollY() {
			t.Errorf("expected scrollY to clamp on resize, got %d (max %d)", m.scrollY, m.maxScrollY())
		}
	})
}
