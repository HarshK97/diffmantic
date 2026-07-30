package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGitTUIModel(t *testing.T) {
	// Initialize git model in current workspace directory
	m := newGitModel(".", "", "", "", false)
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

func TestGitRevisionMode(t *testing.T) {
	// Initialize git model in revision compare mode: HEAD~1 vs Working Copy
	m := newGitModel(".", "HEAD~1", "", "", false)
	if m.refA != "HEAD~1" {
		t.Errorf("expected refA to be HEAD~1, got %q", m.refA)
	}
	if m.refB != "" {
		t.Errorf("expected refB to be empty, got %q", m.refB)
	}

	// Pressing 's' in revision mode should do nothing (return model with same state)
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m3 := m2.(model)
	if m3.gitCursorY != m.gitCursorY {
		t.Error("expected state to remain unchanged on staging in revision mode")
	}

	// Verify status bar help string contains only navigation shortcuts and no stage/commit shortcuts
	helpBar := m.renderStatusBar()
	if strings.Contains(helpBar, "stage") || strings.Contains(helpBar, "commit") {
		t.Errorf("expected read-only status bar help in revision mode, got %q", helpBar)
	}
}

func TestConflictStagingGuard(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test case: file has conflict status ("UU")
	m1 := newGitModel(tempDir, "", "", "", false)
	m1.gitItems = []gitTreeItem{
		{isHeader: false, path: "nonexistent.txt", rawStatus: "UU", isStaged: false},
	}
	m1.gitCursorY = 0

	m1Res, _ := m1.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m1Model := m1Res.(model)
	if m1Model.conflictWarning != "resolve conflicts before staging" {
		t.Errorf("expected conflictWarning 'resolve conflicts before staging', got %q", m1Model.conflictWarning)
	}

	// 2. Test case: status is " M" (unstaged mod) but file contains conflict markers on disk
	conflictFilePath := "conflict.txt"
	conflictContent := "<<<<<<< HEAD\nour changes\n=======\ntheir changes\n>>>>>>> feat-branch\n"
	err := os.WriteFile(filepath.Join(tempDir, conflictFilePath), []byte(conflictContent), 0o644)
	if err != nil {
		t.Fatalf("failed to write mock conflict file: %v", err)
	}

	m2 := newGitModel(tempDir, "", "", "", false)
	m2.gitItems = []gitTreeItem{
		{isHeader: false, path: conflictFilePath, rawStatus: " M", isStaged: false},
	}
	m2.gitCursorY = 0

	m2Res, _ := m2.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m2Model := m2Res.(model)
	if m2Model.conflictWarning != "resolve conflicts before staging" {
		t.Errorf("expected conflictWarning 'resolve conflicts before staging' for file with markers on disk, got %q", m2Model.conflictWarning)
	}
}

func TestCommitStagingGuard(t *testing.T) {
	m := newGitModel(t.TempDir(), "", "", "", false)
	// No staged items, only unstaged changes
	m.gitItems = []gitTreeItem{
		{isHeader: false, path: "file.go", rawStatus: " M", isStaged: false},
	}
	m.gitCursorY = 0

	mRes, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	mModel := mRes.(model)
	if mModel.conflictWarning != "no changes staged for commit" {
		t.Errorf("expected conflictWarning 'no changes staged for commit', got %q", mModel.conflictWarning)
	}
	if mModel.gitCommitOpen {
		t.Error("expected git commit input to remain closed")
	}
}
