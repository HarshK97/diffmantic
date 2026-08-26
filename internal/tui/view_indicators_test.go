package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestGutterBadgeRendering(t *testing.T) {
	tests := []struct {
		name     string
		spans    []span
		isCursor bool
	}{
		{
			name:     "No spans on line",
			spans:    nil,
			isCursor: false,
		},
		{
			name: "Update span only",
			spans: []span{
				{startCol: 10, endCol: 20, kind: kindUpdate},
			},
			isCursor: false,
		},
		{
			name: "Insert and delete spans",
			spans: []span{
				{startCol: 10, endCol: 20, kind: kindInsert},
				{startCol: 30, endCol: 40, kind: kindDelete},
			},
			isCursor: false,
		},
		{
			name: "All four change kinds present",
			spans: []span{
				{startCol: 10, endCol: 20, kind: kindUpdate},
				{startCol: 30, endCol: 40, kind: kindInsert},
				{startCol: 50, endCol: 60, kind: kindDelete},
				{startCol: 70, endCol: 80, kind: kindMove},
			},
			isCursor: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderedLeft := renderGutterBadgeWithTheme(tt.spans, tt.isCursor, true, defaultTheme)
			plainLeft := parseAnsiString(renderedLeft)
			if len([]rune(plainLeft)) != 1 {
				t.Errorf("renderGutterBadgeWithTheme(left) len = %d, want 1", len([]rune(plainLeft)))
			}

			renderedRight := renderGutterBadgeWithTheme(tt.spans, tt.isCursor, false, defaultTheme)
			plainRight := parseAnsiString(renderedRight)
			if len([]rune(plainRight)) != 1 {
				t.Errorf("renderGutterBadgeWithTheme(right) len = %d, want 1", len([]rune(plainRight)))
			}
		})
	}
}

func TestEdgeChevronsAndJumpKeybindings(t *testing.T) {
	// Build a synthetic long line (>200 cols) with all four change types at different offsets:
	// Update at cols 10-20
	// Insert at cols 74-94
	// Delete at cols 138-158
	// Move at cols 202-222
	rawLine := "0123456789" + // 0..9
		"UPDATE_SPAN_TEXT________" + // 10..33
		"MID_PADDING_1___________________________" + // 34..73
		"INSERT_SPAN_TEXT________" + // 74..97
		"MID_PADDING_2___________________________" + // 98..137
		"DELETE_SPAN_TEXT________" + // 138..161
		"MID_PADDING_3___________________________" + // 162..201
		"MOVE_SPAN_TEXT__________" + // 202..225
		"END"

	spans := []span{
		{startCol: 10, endCol: 30, kind: kindUpdate},
		{startCol: 74, endCol: 94, kind: kindInsert},
		{startCol: 138, endCol: 158, kind: kindDelete},
		{startCol: 202, endCol: 222, kind: kindMove},
	}

	m := model{
		width:         145, // Pane width ~64 cols
		height:        20,
		ready:         true,
		srcLines:      []string{rawLine},
		dstLines:      []string{rawLine},
		srcHighlights: &highlights{spans: map[int][]span{0: spans}},
		dstHighlights: &highlights{spans: map[int][]span{0: spans}},
		virtualLines:  []virtualLine{{leftLine: 0, rightLine: 0, alignedRow: 0, foldIdx: -1}},
		activePane:    "left",
		cursorY:       0,
		scrollXLeft:   0,
	}

	textW := m.textWidth() // Expected ~64 cols
	if textW < 50 || textW > 70 {
		t.Logf("Computed textWidth: %d", textW)
	}

	// 1. Scrolled to 0:
	// Span 0 (10..30) is inside [0..64)
	// Spans 1, 2, 3 (74, 138, 202) are entirely to the right (>= 64)
	// Expect right chevron '>', no left chevron.
	renderedLine0 := m.renderStyledLine(rawLine, spans, nil, nil, 0, textW, -1, "left", 0)
	if !strings.Contains(renderedLine0, ">") {
		t.Errorf("Expected right chevron '>' when scrolled to 0, got rendered line: %s", renderedLine0)
	}
	if strings.HasPrefix(parseAnsiString(renderedLine0), "<") {
		t.Errorf("Unexpected left chevron '<' when scrolled to 0")
	}

	// 2. Scrolled to 50:
	// Span 0 (10..30) is entirely to the left (<= 50)
	// Span 1 (74..94) is inside [50..114)
	// Spans 2, 3 (138, 202) are entirely to the right (>= 114)
	// Expect BOTH left chevron '<' and right chevron '>'.
	renderedLine50 := m.renderStyledLine(rawLine, spans, nil, nil, 50, textW, -1, "left", 0)
	if !strings.Contains(renderedLine50, "<") || !strings.Contains(renderedLine50, ">") {
		t.Errorf("Expected both '<' and '>' chevrons when scrolled to 50, got: %s", renderedLine50)
	}

	// 3. Partially visible span check:
	// Scroll to 20: Span 0 (10..30) overlaps [20..84) -> partially visible!
	// It should NOT trigger a left chevron.
	renderedLine20 := m.renderStyledLine(rawLine, spans, nil, nil, 20, textW, -1, "left", 0)
	// Check first visible cell is NOT '<'
	plain20 := parseAnsiString(renderedLine20)
	if len(plain20) > 0 && plain20[0] == '<' {
		t.Errorf("Partially visible span triggered left chevron '<' at scroll 20")
	}

	// 4. Test Jump-to-change keybinding ']h'
	m.scrollXLeft = 0
	m.cursorX = 0
	// Jump next
	m2, _ := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("]")})
	m3, _ := m2.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	updatedM := m3.(model)

	if updatedM.scrollXLeft == 0 && updatedM.cursorX == 0 {
		t.Errorf("]h keybinding did not pan scrollX or update cursorX")
	}

	// 5. Test Jump-to-change keybinding '[h'
	m4, _ := updatedM.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("[")})
	m5, _ := m4.(model).handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	prevM := m5.(model)

	if prevM.cursorX != 10 { // Jumped back to first span startCol 10
		t.Logf("After [h jump, cursorX=%d, scrollXLeft=%d", prevM.cursorX, prevM.scrollXLeft)
	}
}

func parseAnsiString(s string) string {
	var b strings.Builder
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			i += 2
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
		} else {
			b.WriteRune(runes[i])
		}
	}
	return b.String()
}
