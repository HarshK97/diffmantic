package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

// actionsAtCursor returns actions under the cursor on the current line.
func actionsAtCursor(m *model) []*serialize.Action {
	if m.cursorY < 0 || m.cursorY >= len(m.virtualLines) {
		return nil
	}

	vl := m.virtualLines[m.cursorY]
	if vl.foldIdx >= 0 {
		return nil
	}

	var hl *highlights
	var lineIdx int
	if m.activePane == "left" {
		hl = m.srcHighlights
		lineIdx = vl.leftLine
	} else {
		hl = m.dstHighlights
		lineIdx = vl.rightLine
	}

	if lineIdx < 0 || hl == nil {
		return nil
	}

	lineSpans := hl.spans[lineIdx]
	if len(lineSpans) == 0 {
		return nil
	}

	var lines []string
	if m.activePane == "left" {
		lines = m.srcLines
	} else {
		lines = m.dstLines
	}
	if lineIdx >= len(lines) {
		return nil
	}

	_, byteToVisual := expandLine(lines[lineIdx])

	seen := map[*serialize.Action]bool{}
	var result []*serialize.Action

	for _, s := range lineSpans {
		if s.action == nil {
			continue
		}

		var sc, ec int
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = byteToVisual[len(byteToVisual)-1]
		}

		if m.cursorX >= sc && m.cursorX < ec {
			if !seen[s.action] {
				seen[s.action] = true
				result = append(result, s.action)
			}
		}
	}

	return result
}

func (m *model) updateInspectActions() {
	m.inspectActions = actionsAtCursor(m)
}

func actionKindFromString(s string) actionKind {
	switch s {
	case "delete":
		return kindDelete
	case "insert":
		return kindInsert
	case "update":
		return kindUpdate
	case "move":
		return kindMove
	default:
		return kindDelete
	}
}

// formatActionPreview formats a one-line action preview for the status bar.
func formatActionPreview(actions []*serialize.Action, maxWidth int) string {
	if len(actions) == 0 {
		return ""
	}

	a := actions[0]
	kind := actionKindFromString(a.Action)
	icon := actionIcon(kind)
	fg := actionFg(kind)

	label := strings.ToUpper(a.Action)
	nodeType := ""
	nodeName := ""
	if a.Node != nil {
		nodeType = a.Node.Type
		nodeName = a.Node.Label
	}

	var detail string
	switch a.Action {
	case "update":
		if a.OldValue != "" && a.NewValue != "" {
			old := truncateStr(a.OldValue, 20)
			new := truncateStr(a.NewValue, 20)
			detail = fmt.Sprintf("'%s' → '%s'", old, new)
		} else if nodeType != "" {
			detail = nodeType
			if nodeName != "" {
				detail += " '" + nodeName + "'"
			}
		}
	default:
		detail = nodeType
		if nodeName != "" {
			detail += " '" + nodeName + "'"
		}
	}

	iconStyled := lipgloss.NewStyle().Foreground(fg).Render(icon)
	labelStyled := lipgloss.NewStyle().Foreground(fg).Bold(true).Render(label)

	preview := fmt.Sprintf(" ▸ %s %s  %s", iconStyled, labelStyled, detail)

	if len(actions) > 1 {
		badge := inspectDimStyle.Render(fmt.Sprintf("  (+%d more)", len(actions)-1))
		preview += badge
	}

	return truncateStr(preview, maxWidth)
}

// lineIndexFromLines builds a byte offset index for lines.
func lineIndexFromLines(lines []string) []int {
	return buildLineIndex([]byte(strings.Join(lines, "\n")))
}

// formatNodeRange formats a node's byte range to a human-readable 1-indexed line and column range.
func formatNodeRange(lines []string, node *serialize.NodeRef) string {
	if node == nil {
		return ""
	}
	idx := lineIndexFromLines(lines)
	startL, startC := byteToLineCol(idx, node.StartByte)
	endL, endC := byteToLineCol(idx, node.EndByte)
	return fmt.Sprintf("L%d:%d - L%d:%d", startL+1, startC+1, endL+1, endC+1)
}

// formatByteRange formats byte offsets to human-readable line and column offsets.
func formatByteRange(lines []string, startByte, endByte uint32) string {
	idx := lineIndexFromLines(lines)
	startL, startC := byteToLineCol(idx, startByte)
	endL, endC := byteToLineCol(idx, endByte)
	return fmt.Sprintf("L%d:%d - L%d:%d", startL+1, startC+1, endL+1, endC+1)
}

// formatActionColumn formats an action into three detail lines padded to colWidth.
func formatActionColumn(lines []string, a *serialize.Action, colWidth int) []string {
	colLines := make([]string, 3)

	kind := actionKindFromString(a.Action)
	fg := actionFg(kind)
	icon := actionIcon(kind)

	iconStyled := lipgloss.NewStyle().Foreground(fg).Render(icon)
	labelStyled := lipgloss.NewStyle().Foreground(fg).Bold(true).Render(strings.ToUpper(a.Action))

	nodeDesc := ""
	if a.Node != nil {
		nodeDesc = a.Node.Type
		if a.Node.Label != "" {
			nodeDesc += " '" + a.Node.Label + "'"
		}
	}

	// Line 0: Icon + Action Type + Node Type/Label
	line0 := fmt.Sprintf("%s %s %s", iconStyled, labelStyled, nodeDesc)
	colLines[0] = truncateStr(line0, colWidth)

	// Line 1: Parent or Destination
	var line1 string
	switch a.Action {
	case "update":
		if a.OldValue != "" && a.NewValue != "" {
			old := truncateStr(a.OldValue, colWidth/2-2)
			new := truncateStr(a.NewValue, colWidth/2-2)
			line1 = inspectDetailStyle.Render(fmt.Sprintf("'%s' → '%s'", old, new))
		} else if a.Parent != nil {
			line1 = inspectDetailStyle.Render(fmt.Sprintf("parent: %s '%s'", a.Parent.Type, a.Parent.Label))
		}
	case "move":
		if a.DestNode != nil {
			line1 = inspectDetailStyle.Render(fmt.Sprintf("→ dest: %s '%s' (Enter to jump)", a.DestNode.Type, a.DestNode.Label))
		} else if a.DestStartByte != nil && a.DestEndByte != nil {
			line1 = inspectDetailStyle.Render(fmt.Sprintf("→ dest: %s (Enter to jump)", formatByteRange(lines, *a.DestStartByte, *a.DestEndByte)))
		}
	default:
		if a.Parent != nil {
			line1 = inspectDetailStyle.Render(fmt.Sprintf("parent: %s '%s'", a.Parent.Type, a.Parent.Label))
		}
	}
	colLines[1] = truncateStr(line1, colWidth)

	// Line 2: Line/Col range and Group ID
	var line2 string
	if a.Node != nil {
		line2 = inspectDimStyle.Render(formatNodeRange(lines, a.Node))
	}
	if a.GroupID != "" && a.Action != "move" {
		if line2 != "" {
			line2 += inspectDimStyle.Render(" │ grp: " + a.GroupID)
		} else {
			line2 = inspectDimStyle.Render("grp: " + a.GroupID)
		}
	}
	colLines[2] = truncateStr(line2, colWidth)

	// Pad lines to colWidth.
	for idx := 0; idx < 3; idx++ {
		colLines[idx] = padRight(colLines[idx], colWidth)
	}

	return colLines
}

// renderInspectPanel renders the inspect panel.
func (m model) renderInspectPanel() string {
	width := m.width
	if width <= 0 {
		return ""
	}

	panelLines := make([]string, inspectPanelHeight)

	if len(m.inspectActions) == 0 {
		// No action at cursor.
		noAction := inspectDimStyle.Render("  No action at cursor")
		panelLines[0] = inspectPanelStyle.Render(padRight(noAction, width))
		for i := 1; i < inspectPanelHeight; i++ {
			panelLines[i] = inspectPanelStyle.Render(strings.Repeat(" ", width))
		}
		return strings.Join(panelLines, "\n")
	}

	var lines []string
	if m.activePane == "left" {
		lines = m.srcLines
	} else {
		lines = m.dstLines
	}

	if len(m.inspectActions) == 1 {
		colWidth := width - 4 // border/padding
		accent := actionFg(actionKindFromString(m.inspectActions[0].Action))
		border := lipgloss.NewStyle().Foreground(accent).Render("│")

		colLines := formatActionColumn(lines, m.inspectActions[0], colWidth)

		titleLine := border + " " + lipgloss.NewStyle().Foreground(accent).Bold(true).Render("SEMANTIC INSPECTOR")
		panelLines[0] = inspectPanelStyle.Render(padRight(titleLine, width))

		for i := 0; i < 3; i++ {
			panelLines[i+1] = inspectPanelStyle.Render(border + " " + colLines[i])
		}
	} else {
		// Side-by-side columns!
		numCols := len(m.inspectActions)
		if numCols > 3 {
			numCols = 3 // cap at 3 columns
		}

		divSpacing := 3 * (numCols - 1)
		availWidth := width - 4 - divSpacing
		colWidth := availWidth / numCols
		if colWidth < 15 {
			colWidth = 15
		}

		colData := make([][]string, numCols)
		for c := 0; c < numCols; c++ {
			colData[c] = formatActionColumn(lines, m.inspectActions[c], colWidth)
		}

		accent := actionFg(actionKindFromString(m.inspectActions[0].Action))
		border := lipgloss.NewStyle().Foreground(accent).Render("│")
		titleLine := border + " " + lipgloss.NewStyle().Foreground(accent).Bold(true).Render("SEMANTIC INSPECTOR")
		panelLines[0] = inspectPanelStyle.Render(padRight(titleLine, width))

		for i := 0; i < 3; i++ {
			var rowParts []string
			rowParts = append(rowParts, border+" ")
			for c := 0; c < numCols; c++ {
				if c > 0 {
					rowParts = append(rowParts, inspectDimStyle.Render(" │ "))
				}
				rowParts = append(rowParts, colData[c][i])
			}
			panelLines[i+1] = inspectPanelStyle.Render(strings.Join(rowParts, ""))
		}
	}

	return strings.Join(panelLines, "\n")
}

// byteToLine converts a byte offset to a 0-indexed line number.
func byteToLine(lines []string, byteOffset uint32) int {
	offset := int(byteOffset)
	current := 0
	for lineIdx, line := range lines {
		current += len(line) + 1 // +1 for the newline character
		if current > offset {
			return lineIdx
		}
	}
	return len(lines) - 1
}

// jumpToMoveCounterpart jumps the cursor to the other side of a move action.
func (m *model) jumpToMoveCounterpart() {
	if len(m.inspectActions) == 0 {
		return
	}

	// Find the first MOVE action at cursor.
	var moveAct *serialize.Action
	for _, a := range m.inspectActions {
		if a.Action == "move" {
			moveAct = a
			break
		}
	}
	if moveAct == nil {
		return
	}

	targetRow := -1
	var targetPane string

	if m.activePane == "left" {
		if moveAct.DestStartByte == nil {
			return
		}
		dstLine := byteToLine(m.dstLines, *moveAct.DestStartByte)
		// Find the aligned grid row.
		for r, pair := range m.lineAlignment {
			if pair.RightLine == dstLine {
				targetRow = r
				break
			}
		}
		targetPane = "right"
	} else {
		if moveAct.Node == nil {
			return
		}
		srcLine := byteToLine(m.srcLines, moveAct.Node.StartByte)
		// Find the aligned grid row.
		for r, pair := range m.lineAlignment {
			if pair.LeftLine == srcLine {
				targetRow = r
				break
			}
		}
		targetPane = "left"
	}

	if targetRow == -1 {
		return
	}

	// Check if this row is inside a collapsed fold.
	foldOpened := false
	for fi := range m.folds {
		f := &m.folds[fi]
		if !f.open && targetRow >= f.startLine && targetRow <= f.endLine {
			f.open = true
			foldOpened = true
		}
	}
	if foldOpened {
		m.rebuildVirtualLines()
	}

	// Find the display line for the target row.
	for i, vl := range m.virtualLines {
		if vl.alignedRow == targetRow {
			m.cursorY = i
			m.activePane = targetPane
			m.clampCursor()
			m.keepCursorInViewport()
			m.updateInspectActions()
			break
		}
	}
}
