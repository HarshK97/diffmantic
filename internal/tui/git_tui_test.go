package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/git"
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

func initGitRepo(t *testing.T, dir string) {
	_, err := git.RunGit(dir, "init")
	if err != nil {
		t.Fatalf("failed to init git: %v", err)
	}
	_, _ = git.RunGit(dir, "config", "user.name", "Test User")
	_, _ = git.RunGit(dir, "config", "user.email", "test@example.com")
	_, _ = git.RunGit(dir, "config", "commit.gpgsign", "false")
}

func TestGitDiffCache(t *testing.T) {
	tempDir := t.TempDir()
	initGitRepo(t, tempDir)

	// Initial commit with a supported text file, unsupported plain text file, and binary image
	textFile := "main.go"
	plainFile := "notes.txt"
	binFile := "image.png"
	err := os.WriteFile(filepath.Join(tempDir, textFile), []byte("package main\n\nfunc main() {\n\tprintln(\"v1\")\n}\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write textFile: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, plainFile), []byte("line 1\nline 2\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write plainFile: %v", err)
	}

	binData := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x0d, 0x0a, 0x1a, 0x0a}
	err = os.WriteFile(filepath.Join(tempDir, binFile), binData, 0o644)
	if err != nil {
		t.Fatalf("failed to write binFile: %v", err)
	}

	_, err = git.RunGit(tempDir, "add", textFile, plainFile, binFile)
	if err != nil {
		t.Fatalf("failed to add files: %v", err)
	}
	_, err = git.RunGit(tempDir, "commit", "-m", "initial commit")
	if err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Make unstaged edits and stage a brand-new file
	err = os.WriteFile(filepath.Join(tempDir, textFile), []byte("package main\n\nfunc main() {\n\tprintln(\"v2\")\n}\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to modify textFile: %v", err)
	}

	err = os.WriteFile(filepath.Join(tempDir, plainFile), []byte("line 1\nline 2 modified\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to modify plainFile: %v", err)
	}

	binDataModified := []byte{0x89, 0x50, 0x4e, 0x47, 0x00, 0x0d, 0x0a, 0x1a, 0x0b}
	err = os.WriteFile(filepath.Join(tempDir, binFile), binDataModified, 0o644)
	if err != nil {
		t.Fatalf("failed to modify binFile: %v", err)
	}

	stagedFile := "staged.go"
	err = os.WriteFile(filepath.Join(tempDir, stagedFile), []byte("package main\n\nvar Staged = true\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to write stagedFile: %v", err)
	}
	_, err = git.RunGit(tempDir, "add", stagedFile)
	if err != nil {
		t.Fatalf("failed to add stagedFile: %v", err)
	}

	// Creating the git model kicks off background precomputation
	m := newGitModel(tempDir, "", "", "", false)

	// Verify all modified/staged files exist in gitItems
	expectedFiles := map[string]bool{textFile: false, plainFile: false, binFile: false, stagedFile: false}
	for _, item := range m.gitItems {
		if !item.isHeader {
			if _, exists := expectedFiles[item.path]; exists {
				expectedFiles[item.path] = true
			}
		}
	}
	for f, found := range expectedFiles {
		if !found {
			t.Errorf("expected %s to be present in gitItems", f)
		}
	}

	// Check text file cache entry
	textEntry, ok := m.gitDiffCache[textFile]
	if !ok {
		t.Fatalf("expected %s in gitDiffCache", textFile)
	}
	if textEntry.isBinary {
		t.Errorf("expected %s not to be marked binary", textFile)
	}
	if len(textEntry.srcBytes) == 0 || len(textEntry.dstBytes) == 0 {
		t.Errorf("expected non-empty srcBytes and dstBytes for %s", textFile)
	}
	if textEntry.env == nil {
		t.Errorf("expected non-nil AST envelope for %s", textFile)
	}

	// Check unsupported plain text file cache entry (handled via line diff)
	plainEntry, ok := m.gitDiffCache[plainFile]
	if !ok {
		t.Fatalf("expected %s in gitDiffCache", plainFile)
	}
	if plainEntry.isBinary {
		t.Errorf("expected %s not to be marked binary", plainFile)
	}
	if plainEntry.env == nil {
		t.Errorf("expected non-nil line-diff envelope for %s", plainFile)
	}

	// Check binary file cache entry
	binEntry, ok := m.gitDiffCache[binFile]
	if !ok {
		t.Fatalf("expected %s in gitDiffCache", binFile)
	}
	if !binEntry.isBinary {
		t.Errorf("expected %s to be marked as binary", binFile)
	}

	// Check staged file cache entry
	stagedEntry, ok := m.gitDiffCache[stagedFile]
	if !ok {
		t.Fatalf("expected %s in gitDiffCache", stagedFile)
	}
	if len(stagedEntry.dstBytes) == 0 {
		t.Errorf("expected non-empty dstBytes for staged file %s", stagedFile)
	}

	// Verify binary files set the placeholder message on load
	for i, item := range m.gitItems {
		if item.isHeader {
			continue
		}
		err = m.loadGitFileDiff(i)
		if err != nil {
			t.Errorf("failed loadGitFileDiff for %s: %v", item.path, err)
		}
		if item.path == binFile {
			if len(m.srcLines) == 0 || m.srcLines[0] != "[Binary File Diff Not Supported]" {
				t.Errorf("expected binary placeholder in srcLines for %s, got: %v", binFile, m.srcLines)
			}
		}
	}

	// Verify cache updates when refresh is called after a disk change
	err = os.WriteFile(filepath.Join(tempDir, textFile), []byte("package main\n\nfunc main() {\n\tprintln(\"v3 updated\")\n}\n"), 0o644)
	if err != nil {
		t.Fatalf("failed to update textFile: %v", err)
	}

	m.refreshGitStatus()

	updatedTextEntry, ok := m.gitDiffCache[textFile]
	if !ok {
		t.Fatalf("expected %s in gitDiffCache after refresh", textFile)
	}
	if !strings.Contains(string(updatedTextEntry.dstBytes), "v3 updated") {
		t.Errorf("expected dstBytes to contain 'v3 updated', got: %s", string(updatedTextEntry.dstBytes))
	}
}
