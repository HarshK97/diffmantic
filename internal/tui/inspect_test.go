package tui

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestActionsAtCursor(t *testing.T) {
	srcBytes := []byte("func foo() {\n\t// old comment\n}\n")
	dstBytes := []byte("func bar() {\n\t// new comment\n}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24

	m.cursorY = 0
	m.cursorX = 5
	m.activePane = "left"
	m.updateInspectActions()
	if len(m.inspectActions) != 0 {
		t.Errorf("expected 0 inspect actions, got %d", len(m.inspectActions))
	}

	updateAction := serialize.Action{
		Action: "update",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "identifier",
			Label:     "foo",
			StartByte: 5,
			EndByte:   8,
		},
		OldValue: "foo",
		NewValue: "bar",
	}

	dstStart := uint32(14)
	dstEnd := uint32(28)
	moveAction := serialize.Action{
		Action: "move",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "comment",
			Label:     "// old comment",
			StartByte: 14,
			EndByte:   28,
		},
		DestStartByte: &dstStart,
		DestEndByte:   &dstEnd,
	}

	m.srcHighlights = &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 5, endCol: 8, kind: kindUpdate, action: &updateAction},
			},
			1: {
				{startCol: 1, endCol: 15, kind: kindMove, action: &moveAction},
			},
		},
		tinted: map[int]actionKind{
			0: kindUpdate,
			1: kindMove,
		},
	}

	m.dstHighlights = &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 5, endCol: 8, kind: kindUpdate, action: &updateAction},
			},
			1: {
				{startCol: 1, endCol: 15, kind: kindMove, action: &moveAction},
			},
		},
		tinted: map[int]actionKind{
			0: kindUpdate,
			1: kindMove,
		},
	}

	m.rebuildVirtualLines()
	m.openAllFolds()

	m.cursorY = 0
	m.cursorX = 5
	m.activePane = "left"
	m.updateInspectActions()

	if len(m.inspectActions) != 1 {
		t.Fatalf("expected 1 action at cursor, got %d", len(m.inspectActions))
	}
	if m.inspectActions[0].Action != "update" {
		t.Errorf("expected update action, got %s", m.inspectActions[0].Action)
	}

	preview := formatActionPreview(m.inspectActions, 100)
	if !strings.Contains(preview, "UPDATE") {
		t.Errorf("expected preview to contain UPDATE, got %q", preview)
	}
	if !strings.Contains(preview, "'foo' → 'bar'") {
		t.Errorf("expected preview to show value transition, got %q", preview)
	}

	panelText := m.renderInspectPanel()
	if !strings.Contains(panelText, "foo") || !strings.Contains(panelText, "bar") {
		t.Errorf("expected detailed panel to describe old/new values, got %q", panelText)
	}

	// Tab \t maps to visual columns 0, 1, 2, 3. The move action starts at byte 14 (which is after \t, which was at byte 13).
	// Line 1 is: "\t// old comment"
	// "\t" is visual col 0-3 (byte col 0).
	// "// old comment" starts at visual col 4 (byte col 1).
	// We'll place the cursor at visual col 5.
	m.cursorY = 1
	m.cursorX = 5
	m.activePane = "left"
	m.updateInspectActions()

	if len(m.inspectActions) != 1 {
		t.Fatalf("expected 1 action at cursor for line 1, got %d", len(m.inspectActions))
	}
	if m.inspectActions[0].Action != "move" {
		t.Errorf("expected move action, got %s", m.inspectActions[0].Action)
	}

	panelTextMove := m.renderInspectPanel()
	if !strings.Contains(panelTextMove, "L2:2 - L2:16") {
		t.Errorf("expected panel to contain human-readable range 'L2:2 - L2:16', got %q", panelTextMove)
	}
	if strings.Contains(panelTextMove, "bytes") {
		t.Errorf("expected panel to hide raw bytes, got %q", panelTextMove)
	}

	insertAction := serialize.Action{
		Action: "insert",
		Node: &serialize.NodeRef{
			Tree:      "after",
			Type:      "comment",
			Label:     "// new comment",
			StartByte: 14,
			EndByte:   28,
		},
	}
	m.srcHighlights.spans[1] = append(m.srcHighlights.spans[1], span{
		startCol: 1,
		endCol:   15,
		kind:     kindInsert,
		action:   &insertAction,
	})
	m.updateInspectActions()

	if len(m.inspectActions) != 2 {
		t.Fatalf("expected 2 actions at cursor, got %d", len(m.inspectActions))
	}

	panelTextMultiple := m.renderInspectPanel()

	if !strings.Contains(panelTextMultiple, "MOVE") || !strings.Contains(panelTextMultiple, "INSERT") {
		t.Errorf("expected side-by-side panel to contain both MOVE and INSERT actions, got %q", panelTextMultiple)
	}

	m.jumpToMoveCounterpart()

	if m.activePane != "right" {
		t.Errorf("expected jump to switch active pane to 'right', got %s", m.activePane)
	}
	if m.cursorY != 1 {
		t.Errorf("expected jump to preserve line index 1, got row %d", m.cursorY)
	}
	if m.scrollY < 0 {
		t.Errorf("expected scrollY to be non-negative after jump, got %d", m.scrollY)
	}
}

func TestByteToLineColFromLines(t *testing.T) {
	tests := []struct {
		name   string
		lines  []string
		offset uint32
		line   int
		col    int
	}{
		// Standard LF line breaks: "abc\nde\nf\n"
		{"LF first byte", []string{"abc", "de", "f"}, 0, 0, 0},
		{"LF mid first line", []string{"abc", "de", "f"}, 1, 0, 1},
		{"LF end first line", []string{"abc", "de", "f"}, 2, 0, 2},
		{"LF newline byte", []string{"abc", "de", "f"}, 3, 0, 3},
		{"LF start second line", []string{"abc", "de", "f"}, 4, 1, 0},
		{"LF mid second line", []string{"abc", "de", "f"}, 5, 1, 1},
		{"LF end second line", []string{"abc", "de", "f"}, 6, 1, 2},
		{"LF start third line", []string{"abc", "de", "f"}, 7, 2, 0},
		{"LF past end", []string{"abc", "de", "f"}, 99, 2, 0},

		// CRLF: split("\n") leaves \r at end of each line string.
		{"CRLF first byte", []string{"abc\r", "de\r", "f"}, 0, 0, 0},
		{"CRLF CR byte", []string{"abc\r", "de\r", "f"}, 3, 0, 3},
		{"CRLF LF byte", []string{"abc\r", "de\r", "f"}, 4, 0, 4},
		{"CRLF start second", []string{"abc\r", "de\r", "f"}, 5, 1, 0},
		{"CRLF second CR", []string{"abc\r", "de\r", "f"}, 7, 1, 2},
		{"CRLF second LF", []string{"abc\r", "de\r", "f"}, 8, 1, 3},
		{"CRLF start third", []string{"abc\r", "de\r", "f"}, 9, 2, 0},

		// Multibyte: ⌘ is 3 bytes in UTF-8.
		{"MB first byte", []string{"⌘", "⌘"}, 0, 0, 0},
		{"MB third byte", []string{"⌘", "⌘"}, 2, 0, 2},
		{"MB newline", []string{"⌘", "⌘"}, 3, 0, 3},
		{"MB second line", []string{"⌘", "⌘"}, 4, 1, 0},
		{"MB second last byte", []string{"⌘", "⌘"}, 6, 1, 2},
		{"MB past end", []string{"⌘", "⌘"}, 99, 1, 0},

		// Empty lines.
		{"empty input", []string{}, 0, 0, 0},
		{"single empty line", []string{""}, 0, 0, 0},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLine, gotCol := byteToLineColFromLines(tc.lines, tc.offset)
			if gotLine != tc.line || gotCol != tc.col {
				t.Errorf("byteToLineColFromLines(%q, %d) = (%d, %d), want (%d, %d)",
					tc.lines, tc.offset, gotLine, gotCol, tc.line, tc.col)
			}
		})
	}
}
