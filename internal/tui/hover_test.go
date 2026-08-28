package tui

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestActionsAtCursorAndHover(t *testing.T) {
	srcBytes := []byte("func foo() {\n\t// old comment\n}\n")
	dstBytes := []byte("func bar() {\n\t// new comment\n}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24

	m.cursorY = 0
	m.cursorX = 5
	m.activePane = "left"

	actions := actionsAtCursor(&m)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions at cursor, got %d", len(actions))
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

	moveAction := serialize.Action{
		Action: "move",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "comment",
			Label:     "// old comment",
			StartByte: 14,
			EndByte:   28,
		},
		DestStartByte: new(uint32(14)),
		DestEndByte:   new(uint32(28)),
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

	actions = actionsAtCursor(&m)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action at cursor, got %d", len(actions))
	}
	if actions[0].Action != "update" {
		t.Errorf("expected update action, got %s", actions[0].Action)
	}

	preview := formatActionPreview(actions, 100)
	if !strings.Contains(preview, "UPDATE") {
		t.Errorf("expected preview to contain UPDATE, got %q", preview)
	}
	if !strings.Contains(preview, "'foo' → 'bar'") {
		t.Errorf("expected preview to show value transition, got %q", preview)
	}

	boxText := renderHoverBox(m.srcLines, m.dstLines, actions, 80, "left")
	if !strings.Contains(boxText, "foo") || !strings.Contains(boxText, "bar") {
		t.Errorf("expected detailed hover box to describe old/new values, got %q", boxText)
	}

	m.cursorY = 1
	m.cursorX = 5
	m.activePane = "left"

	actions = actionsAtCursor(&m)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action at cursor for line 1, got %d", len(actions))
	}
	if actions[0].Action != "move" {
		t.Errorf("expected move action, got %s", actions[0].Action)
	}

	boxTextMove := renderHoverBox(m.srcLines, m.dstLines, actions, 80, "left")
	if !strings.Contains(boxTextMove, "L2:2 - L2:16") {
		t.Errorf("expected hover box to contain human-readable range 'L2:2 - L2:16', got %q", boxTextMove)
	}
	if strings.Contains(boxTextMove, "bytes") {
		t.Errorf("expected hover box to hide raw bytes, got %q", boxTextMove)
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

	actions = actionsAtCursor(&m)
	if len(actions) != 1 {
		t.Fatalf("expected 1 action at cursor (innermost/prioritized), got %d", len(actions))
	}
	if actions[0].Action != "move" {
		t.Errorf("expected move action (higher priority than insert), got %s", actions[0].Action)
	}

	boxTextMovePrioritized := renderHoverBox(m.srcLines, m.dstLines, actions, 80, "left")
	if !strings.Contains(boxTextMovePrioritized, "MOVE") {
		t.Errorf("expected hover box to contain MOVE action, got %q", boxTextMovePrioritized)
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

func TestKeyboardHoverToggleAndDismissal(t *testing.T) {
	srcBytes := []byte("func foo() {\n}\n")
	dstBytes := []byte("func bar() {\n}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24

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

	m.srcHighlights = &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 5, endCol: 8, kind: kindUpdate, action: &updateAction},
			},
		},
		tinted: map[int]actionKind{
			0: kindUpdate,
		},
	}
	m.dstHighlights = m.srcHighlights
	m.rebuildVirtualLines()
	m.openAllFolds()

	m.cursorY = 0
	m.cursorX = 5
	m.activePane = "left"

	updatedModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = updatedModel.(model)
	if !m.hoverOpen {
		t.Fatalf("expected hoverOpen = true on K press")
	}
	if len(m.hoverActions) != 1 {
		t.Fatalf("expected 1 hoverAction, got %d", len(m.hoverActions))
	}

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Fatalf("expected hoverOpen = false on second K press")
	}

	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = updatedModel.(model)
	if !m.hoverOpen {
		t.Fatalf("expected hoverOpen = true on shift+k")
	}

	// Cursor navigation should dismiss the popover
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Fatalf("expected hoverOpen = false after j navigation")
	}

	// Pressing K on an unhighlighted line should not open the popover
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'K'}})
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Fatalf("expected hoverOpen = false on empty line")
	}
}

func TestMouseHoverMotion(t *testing.T) {
	srcBytes := []byte("func foo() {\n}\n")
	dstBytes := []byte("func bar() {\n}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24

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

	m.srcHighlights = &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 5, endCol: 8, kind: kindUpdate, action: &updateAction},
			},
		},
		tinted: map[int]actionKind{
			0: kindUpdate,
		},
	}
	m.dstHighlights = m.srcHighlights
	m.rebuildVirtualLines()
	m.openAllFolds()

	gw := m.gutterWidth()

	mouseMsg := tea.MouseMsg{
		X:      gw + 5,
		Y:      1,
		Action: tea.MouseActionMotion,
	}

	updatedModel, _ := m.Update(mouseMsg)
	m = updatedModel.(model)
	if !m.hoverOpen {
		t.Fatalf("expected hoverOpen = true on mouse hover over action")
	}
	if m.hoverSource != "mouse" {
		t.Errorf("expected hoverSource = 'mouse', got %q", m.hoverSource)
	}

	mouseAwayMsg := tea.MouseMsg{
		X:      1,
		Y:      1,
		Action: tea.MouseActionMotion,
	}
	updatedModel, _ = m.Update(mouseAwayMsg)
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Fatalf("expected hoverOpen = false when mouse moves away")
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

func TestRenderHoverBoxNarrowWidth(t *testing.T) {
	updateAction := serialize.Action{
		Action: "update",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "identifier",
			Label:     "veryLongIdentifierNameThatMightExceed",
			StartByte: 0,
			EndByte:   37,
		},
		OldValue: "veryLongIdentifierNameThatMightExceed",
		NewValue: "short",
	}
	actions := []*serialize.Action{&updateAction}
	srcLines := []string{"veryLongIdentifierNameThatMightExceed"}
	dstLines := []string{"short"}

	box := renderHoverBox(srcLines, dstLines, actions, 20, "left")
	if box == "" {
		t.Fatalf("expected non-empty hover box for width 20")
	}
}

func TestMouseHoverMotionHelpOpen(t *testing.T) {
	srcBytes := []byte("func foo() {}\n")
	dstBytes := []byte("func bar() {}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.helpOpen = true
	m.hoverOpen = true
	m.hoverSource = "mouse"

	gw := m.gutterWidth()
	mouseMsg := tea.MouseMsg{
		X:      gw + 5,
		Y:      1,
		Action: tea.MouseActionMotion,
	}

	updatedModel, _ := m.Update(mouseMsg)
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Errorf("expected hoverOpen = false when help modal is open")
	}
}

func TestNestedInnermostActionAtCursor(t *testing.T) {
	// Simulate:
	// Outer action: delete for-loop (cols 0..30, AST byte len 100)
	// Inner action: move variable declaration (cols 5..20, AST byte len 30)
	srcBytes := []byte("for i := 0; i < 10; i++ {\n\tx := 1\n}\n")
	dstBytes := []byte("")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.activePane = "left"

	deleteLoopAction := serialize.Action{
		Action: "delete",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "for_statement",
			StartByte: 0,
			EndByte:   100,
		},
	}

	moveVarAction := serialize.Action{
		Action: "move",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "short_var_declaration",
			Label:     "x := 1",
			StartByte: 27,
			EndByte:   33,
		},
		DestStartByte: new(uint32(50)),
		DestEndByte:   new(uint32(80)),
	}

	m.srcHighlights = &highlights{
		spans: map[int][]span{
			1: {
				// Outer span covering the whole line for the for_statement delete
				{startCol: 0, endCol: 10, kind: kindDelete, action: &deleteLoopAction},
				// Inner span covering only "x := 1" for the move
				{startCol: 1, endCol: 7, kind: kindMove, action: &moveVarAction},
			},
		},
		tinted: map[int]actionKind{
			1: kindMove,
		},
	}

	m.rebuildVirtualLines()
	m.openAllFolds()

	// Line 1 is "\tx := 1" where \t expands to 4 spaces:
	// Visual columns 0..3: tab indentation (covered only by outer delete)
	// Visual columns 4..9: "x := 1" (covered by inner move, byte range 1..7)
	// Visual columns 10+: end of line (covered only by outer delete)

	// 1. Cursor inside the inner move action (col 5: "x := 1")
	m.cursorY = 1
	m.cursorX = 5
	actions := actionsAtCursor(&m)
	if len(actions) != 1 {
		t.Fatalf("expected exactly 1 action at inner cursor position, got %d", len(actions))
	}
	if actions[0].Action != "move" {
		t.Errorf("expected inner action 'move', got %q", actions[0].Action)
	}

	// 2. Cursor outside the inner action, but inside the outer delete action (col 1: tab indentation)
	m.cursorX = 1
	actions = actionsAtCursor(&m)
	if len(actions) != 1 {
		t.Fatalf("expected exactly 1 action at outer cursor position, got %d", len(actions))
	}
	if actions[0].Action != "delete" {
		t.Errorf("expected outer action 'delete', got %q", actions[0].Action)
	}

	// 3. Cursor on line 0 where no spans exist
	m.cursorY = 0
	m.cursorX = 0
	actions = actionsAtCursor(&m)
	if len(actions) != 0 {
		t.Errorf("expected 0 actions on unhighlighted line, got %d", len(actions))
	}
}

func TestMouseHoverMotionModalsOpen(t *testing.T) {
	srcBytes := []byte("func foo() {}\n")
	dstBytes := []byte("func bar() {}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24
	m.hoverOpen = true
	m.hoverSource = "mouse"

	gw := m.gutterWidth()
	mouseMsg := tea.MouseMsg{
		X:      gw + 5,
		Y:      1,
		Action: tea.MouseActionMotion,
	}

	m.gitTreeOpen = true
	m.gitCommitOpen = false
	updatedModel, _ := m.Update(mouseMsg)
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Errorf("expected hoverOpen = false when git tree is open")
	}

	m.hoverOpen = true
	m.hoverSource = "mouse"
	m.gitTreeOpen = false
	m.gitCommitOpen = true
	updatedModel, _ = m.Update(mouseMsg)
	m = updatedModel.(model)
	if m.hoverOpen {
		t.Errorf("expected hoverOpen = false when git commit panel is open")
	}
}

func TestJumpToMoveCounterpartWithDestNodeFallback(t *testing.T) {
	srcBytes := []byte("func moved() {}\n")
	dstBytes := []byte("func moved() {}\n")

	m := newModel("before.go", "after.go", srcBytes, dstBytes, nil)
	m.width = 80
	m.height = 24

	moveAction := serialize.Action{
		Action: "move",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "function_declaration",
			Label:     "moved",
			StartByte: 0,
			EndByte:   15,
		},
		DestNode: &serialize.NodeRef{
			Tree:      "after",
			Type:      "function_declaration",
			Label:     "moved",
			StartByte: 0,
			EndByte:   15,
		},
		DestStartByte: nil,
	}

	m.srcHighlights = &highlights{
		spans: map[int][]span{
			0: {
				{startCol: 0, endCol: 15, kind: kindMove, action: &moveAction},
			},
		},
		tinted: map[int]actionKind{
			0: kindMove,
		},
	}
	m.dstHighlights = m.srcHighlights
	m.rebuildVirtualLines()
	m.openAllFolds()

	m.cursorY = 0
	m.cursorX = 5
	m.activePane = "left"

	m.jumpToMoveCounterpart()

	if m.activePane != "right" {
		t.Errorf("expected jump to switch active pane to 'right', got %s", m.activePane)
	}
}

func TestRenderHoverBoxRightPaneMove(t *testing.T) {
	moveAction := serialize.Action{
		Action: "move",
		Node: &serialize.NodeRef{
			Tree:      "before",
			Type:      "function_declaration",
			Label:     "alpha",
			StartByte: 0,
			EndByte:   15,
		},
		DestNode: &serialize.NodeRef{
			Tree:      "after",
			Type:      "function_declaration",
			Label:     "alpha",
			StartByte: 0,
			EndByte:   15,
		},
	}
	actions := []*serialize.Action{&moveAction}
	srcLines := []string{"func alpha() {}"}
	dstLines := []string{"func alpha() {}"}

	boxLeft := renderHoverBox(srcLines, dstLines, actions, 80, "left")
	if !strings.Contains(boxLeft, "→ dest:") {
		t.Errorf("expected left pane hover box to contain '→ dest:', got %q", boxLeft)
	}

	boxRight := renderHoverBox(srcLines, dstLines, actions, 80, "right")
	if !strings.Contains(boxRight, "← src:") {
		t.Errorf("expected right pane hover box to contain '← src:', got %q", boxRight)
	}
	if strings.Contains(boxRight, "→ dest:") {
		t.Errorf("expected right pane hover box not to contain '→ dest:', got %q", boxRight)
	}
}

func TestFormatNodeSummary(t *testing.T) {
	tests := []struct {
		name string
		node *serialize.NodeRef
		want string
	}{
		{
			name: "nil node",
			node: nil,
			want: "",
		},
		{
			name: "node without label",
			node: &serialize.NodeRef{Type: "block"},
			want: "block",
		},
		{
			name: "node with label",
			node: &serialize.NodeRef{Type: "identifier", Label: "myVar"},
			want: "identifier 'myVar'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatNodeSummary(tt.node)
			if got != tt.want {
				t.Errorf("formatNodeSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHoverUnlabelledNodeNoEmptyQuotes(t *testing.T) {
	act := serialize.Action{
		Action: "delete",
		Node: &serialize.NodeRef{
			Type: "block",
		},
		Parent: &serialize.NodeRef{
			Type: "for_statement",
		},
	}

	box := renderHoverBox([]string{"for {}"}, []string{}, []*serialize.Action{&act}, 80, "left")
	if strings.Contains(box, "''") {
		t.Errorf("expected hover box not to contain empty quotes '', got %q", box)
	}
	if !strings.Contains(box, "parent: for_statement") {
		t.Errorf("expected hover box to contain 'parent: for_statement', got %q", box)
	}
}

func TestRenderHoverBoxLatteBorderBackground(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)

	latte := theme.CatppuccinLatteTheme()
	act := serialize.Action{
		Action: "delete",
		Node: &serialize.NodeRef{
			Type: "block",
		},
	}
	box := renderHoverBox([]string{"for {}"}, []string{}, []*serialize.Action{&act}, 80, "left", latte)
	lines := strings.Split(box, "\n")
	if len(lines) == 0 {
		t.Fatalf("expected non-empty hover box")
	}
	cells := parseAnsi(lines[0])
	if len(cells) == 0 {
		t.Fatalf("expected cells in top border line")
	}
	// The border cell must carry a background color matching Latte Surface0 (#ccd0da)
	if !strings.Contains(cells[0].style, "48;2;") && !strings.Contains(cells[0].style, "48;5;") {
		t.Errorf("expected border cell to have background color set, got style %q", cells[0].style)
	}
}
