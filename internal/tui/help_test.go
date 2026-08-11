package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpMenu(t *testing.T) {
	srcBytes := []byte("func main() {}\n")
	dstBytes := []byte("func main() {}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 20
	m.ready = true

	if m.helpOpen {
		t.Error("expected helpOpen to be false initially")
	}

	viewTextNormal := m.View()
	if strings.Contains(viewTextNormal, "DIFFMANTIC HELP & KEYBINDINGS") {
		t.Error("expected normal view to not render help card title")
	}

	m2, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	m = m2.(model)
	if !m.helpOpen {
		t.Error("expected helpOpen to be true after pressing '?'")
	}
	if cmd != nil {
		t.Errorf("expected nil command for toggle help key, got %v", cmd)
	}

	viewTextHelp := m.View()
	if !strings.Contains(viewTextHelp, "HELP & KEYBINDINGS") {
		t.Error("expected help view to render help card title")
	}
	if !strings.Contains(viewTextHelp, "COLOR LEGEND") {
		t.Error("expected help view to render color legend section")
	}
	if !strings.Contains(viewTextHelp, "MOVE+UPDATE") {
		t.Error("expected help view to render MOVE+UPDATE legend entry")
	}
	if !strings.Contains(viewTextHelp, "]h / [h") {
		t.Error("expected help view to render ]h / [h keybinding")
	}

	m2, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = m2.(model)
	if m.helpOpen {
		t.Error("expected helpOpen to be false after pressing any key to dismiss")
	}
	if cmd != nil {
		t.Errorf("expected nil command for dismissing help, got %v", cmd)
	}

	m.helpOpen = true

	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected non-nil command for quitting help")
	}
	_ = cmd()
}
