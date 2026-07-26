package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSearchFunctionality(t *testing.T) {
	srcBytes := []byte("Copyright 2026 Harsh Kapse\nLicensed under MIT license\nCopyright Google Inc\n")
	dstBytes := []byte("Copyright 2026 Harsh Kapse\nLicensed under Apache license\nCopyright Google Inc\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 20
	m.ready = true
	m.openAllFolds()

	t.Run("initial state", func(t *testing.T) {
		if len(m.searchMatches) != 0 {
			t.Errorf("expected 0 matches initially, got %d", len(m.searchMatches))
		}
	})

	t.Run("compute matches", func(t *testing.T) {
		m.searchQuery = "copyright"
		m.computeSearchMatches()

		if len(m.searchMatches) != 4 {
			t.Fatalf("expected 4 search matches, got %d", len(m.searchMatches))
		}

		first := m.searchMatches[0]
		if first.virtualRow != 0 || first.startCol != 0 || first.endCol != 9 || first.pane != "left" {
			t.Errorf("unexpected first match details: %+v", first)
		}

		second := m.searchMatches[1]
		if second.virtualRow != 0 || second.startCol != 0 || second.endCol != 9 || second.pane != "right" {
			t.Errorf("unexpected second match details: %+v", second)
		}
	})

	t.Run("find nearest match", func(t *testing.T) {
		m.cursorY = 0
		m.cursorX = 0
		m.activePane = "left"
		m.findNearestSearchMatch()
		if m.searchMatchIdx != 0 {
			t.Errorf("expected nearest match idx to be 0, got %d", m.searchMatchIdx)
		}

		m.cursorY = 1
		m.cursorX = 0
		m.activePane = "left"
		m.findNearestSearchMatch()
		if m.searchMatchIdx != 2 {
			t.Errorf("expected nearest match idx to be 2, got %d", m.searchMatchIdx)
		}
	})

	t.Run("jump to match", func(t *testing.T) {
		m.searchMatchIdx = 2
		m.jumpToSearchMatch()
		if m.cursorY != 2 || m.activePane != "left" || m.cursorX != 0 {
			t.Errorf("jumpToSearchMatch failed to set cursor, got Y=%d, pane=%s, X=%d", m.cursorY, m.activePane, m.cursorX)
		}
	})

	t.Run("navigation wrapping", func(t *testing.T) {
		m.searchMatchIdx = 3
		m.searchMatchIdx = (m.searchMatchIdx + 1) % len(m.searchMatches)
		m.jumpToSearchMatch()
		if m.searchMatchIdx != 0 || m.cursorY != 0 || m.activePane != "left" {
			t.Errorf("next match wrapping failed, got matchIdx=%d, Y=%d, pane=%s", m.searchMatchIdx, m.cursorY, m.activePane)
		}

		m.searchMatchIdx = 0
		m.searchMatchIdx = (m.searchMatchIdx - 1) % len(m.searchMatches)
		if m.searchMatchIdx < 0 {
			m.searchMatchIdx += len(m.searchMatches)
		}
		m.jumpToSearchMatch()
		if m.searchMatchIdx != 3 || m.cursorY != 2 || m.activePane != "right" {
			t.Errorf("prev match wrapping failed, got matchIdx=%d, Y=%d, pane=%s", m.searchMatchIdx, m.cursorY, m.activePane)
		}
	})
}

func TestSearchMatchesFor(t *testing.T) {
	srcBytes := []byte("foo bar\nbaz foo\n")
	dstBytes := []byte("foo bar\nbaz qux\n")
	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 20
	m.ready = true
	m.openAllFolds()

	t.Run("empty matches returns nil", func(t *testing.T) {
		m.searchMatches = nil
		matches := m.searchMatchesFor(0, "left")
		if matches != nil {
			t.Errorf("expected nil matches, got %v", matches)
		}
	})

	t.Run("returns only matches for specified row and pane", func(t *testing.T) {
		m.searchQuery = "foo"
		m.computeSearchMatches()

		matchesLeft0 := m.searchMatchesFor(0, "left")
		if len(matchesLeft0) != 1 {
			t.Fatalf("expected 1 match for row 0 left, got %d", len(matchesLeft0))
		}
		if matchesLeft0[0].startCol != 0 {
			t.Errorf("expected startCol 0, got %d", matchesLeft0[0].startCol)
		}
	})

	t.Run("returns nil for row/pane with no matches", func(t *testing.T) {
		matchesRight1 := m.searchMatchesFor(1, "right")
		if len(matchesRight1) != 0 {
			t.Errorf("expected 0 matches for row 1 right, got %d", len(matchesRight1))
		}
	})
}

func TestSearchKeyboardIntegration(t *testing.T) {
	srcBytes := []byte("search text one\n")
	dstBytes := []byte("search text two\n")
	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.ready = true
	m.openAllFolds()

	t.Run("activate search mode", func(t *testing.T) {
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
		m = m2.(model)
		if !m.searchActive {
			t.Error("expected searchActive to be true after '/'")
		}
	})

	t.Run("deactivate search mode with esc", func(t *testing.T) {
		m.searchActive = true
		m2, _ := m.Update(tea.KeyMsg{Type: tea.KeyEscape})
		m = m2.(model)
		if m.searchActive {
			t.Error("expected searchActive to be false after 'esc'")
		}
	})

	t.Run("cycle matches", func(t *testing.T) {
		m.searchQuery = "search"
		m.computeSearchMatches()
		m.searchActive = true
		m.searchMatchIdx = 0

		// Cycle to the next match
		m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
		m = m2.(model)
		if m.searchMatchIdx != 1 {
			t.Errorf("expected searchMatchIdx 1 after 'n', got %d", m.searchMatchIdx)
		}

		// Cycle to the previous match
		m3, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'N'}})
		m = m3.(model)
		if m.searchMatchIdx != 0 {
			t.Errorf("expected searchMatchIdx 0 after 'N', got %d", m.searchMatchIdx)
		}
	})
}
