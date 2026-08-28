package tui

import (
	"cmp"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/charmbracelet/lipgloss"
)

func actionsAtCursor(m *model) []*serialize.Action {
	if m.cursorY < 0 || m.cursorY >= len(m.virtualLines) {
		return nil
	}

	vl := m.virtualLines[m.cursorY]
	if vl.foldIdx >= 0 {
		return nil
	}

	var lineIdx int
	if m.activePane == "left" {
		lineIdx = vl.leftLine
	} else {
		lineIdx = vl.rightLine
	}

	return actionsAtCoord(m, m.activePane, lineIdx, m.cursorX)
}

// actionsAtCoord finds the innermost diff action at the given coordinates.
func actionsAtCoord(m *model, pane string, lineIdx int, visualCol int) []*serialize.Action {
	if lineIdx < 0 {
		return nil
	}

	var hl *highlights
	var lines []string
	if pane == "left" {
		hl = m.srcHighlights
		lines = m.srcLines
	} else {
		hl = m.dstHighlights
		lines = m.dstLines
	}

	if hl == nil || lineIdx >= len(lines) {
		return nil
	}

	lineSpans := hl.spans[lineIdx]
	if len(lineSpans) == 0 {
		return nil
	}

	byteToVisual := byteToVisualMapping(lines[lineIdx])

	var bestAction *serialize.Action
	var bestSpan span
	bestAstLen := math.MaxInt
	bestCandidateLen := math.MaxInt

	for _, s := range lineSpans {
		if s.action == nil {
			continue
		}

		sc := -1
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		var ec int
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else if len(byteToVisual) > 0 {
			ec = byteToVisual[len(byteToVisual)-1]
		}

		if sc >= 0 && visualCol >= sc && visualCol < ec {
			candidateLen := ec - sc
			astLen := getSpanASTLen(s, pane)
			if bestAction == nil || cmp.Or(
				cmp.Compare(astLen, bestAstLen),
				cmp.Compare(candidateLen, bestCandidateLen),
				cmp.Compare(actionPriority(bestSpan.kind), actionPriority(s.kind)),
			) < 0 {
				bestAction = s.action
				bestSpan = s
				bestAstLen = astLen
				bestCandidateLen = candidateLen
			}
		}
	}

	if bestAction != nil {
		return []*serialize.Action{bestAction}
	}
	return nil
}

func formatActionPreview(actions []*serialize.Action, maxWidth int, themeOpt ...*Theme) string {
	if len(actions) == 0 || maxWidth <= 0 {
		return ""
	}

	t := defaultTheme
	if len(themeOpt) > 0 {
		t = cmp.Or(themeOpt[0], defaultTheme)
	}
	bg := t.UI.Surface0

	a := actions[0]
	kind := parseActionKind(a.Action)
	icon := t.ActionIcon(kind)
	fg := t.ActionFg(kind)

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
			newVal := truncateStr(a.NewValue, 20)
			detail = fmt.Sprintf("'%s' → '%s'", old, newVal)
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

	parts := []styledPart{
		{text: " ▸ ", style: t.Styles.Status},
		{text: icon, style: lipgloss.NewStyle().Foreground(fg)},
		{text: " ", style: lipgloss.NewStyle()},
		{text: label, style: lipgloss.NewStyle().Foreground(fg).Bold(true)},
		{text: "  ", style: lipgloss.NewStyle()},
		{text: detail, style: t.Styles.Status},
	}

	// actionsAtCoord returns at most one action right now, but support multiple
	// in case we handle compound actions later.
	if len(actions) > 1 {
		parts = append(parts, styledPart{
			text:  fmt.Sprintf("  (+%d more)", len(actions)-1),
			style: t.Styles.HoverDim,
		})
	}

	return renderHoverRow(parts, maxWidth, bg)
}

func formatByteRange(lines []string, startByte, endByte uint32) string {
	startL, startC := byteToLineColFromLines(lines, startByte)
	endL, endC := byteToLineColFromLines(lines, endByte)
	return fmt.Sprintf("L%d:%d - L%d:%d", startL+1, startC+1, endL+1, endC+1)
}

func byteToLineColFromLines(lines []string, byteOffset uint32) (int, int) {
	off := int(byteOffset)
	curr := 0
	for i, l := range lines {
		lineLen := len(l) + 1
		if off < curr+lineLen {
			col := min(off-curr, len(l))
			return i, col
		}
		curr += lineLen
	}
	if len(lines) == 0 {
		return 0, 0
	}
	return len(lines) - 1, 0
}

type styledPart struct {
	text  string
	style lipgloss.Style
}

func renderHoverRow(parts []styledPart, totalWidth int, bg lipgloss.Color) string {
	var b strings.Builder
	used := 0
	baseStyle := lipgloss.NewStyle().Background(bg)

	for _, p := range parts {
		if used >= totalWidth || p.text == "" {
			continue
		}
		runes := []rune(p.text)
		avail := totalWidth - used
		if len(runes) > avail {
			if avail > 1 {
				runes = append(runes[:avail-1], '…')
			} else {
				runes = []rune{'…'}
			}
		}
		st := p.style.Background(bg)
		b.WriteString(st.Render(string(runes)))
		used += len(runes)
	}

	if used < totalWidth {
		b.WriteString(baseStyle.Render(strings.Repeat(" ", totalWidth-used)))
	}
	return b.String()
}

func formatNodeSummary(node *serialize.NodeRef) string {
	if node == nil {
		return ""
	}
	if node.Label != "" {
		return fmt.Sprintf("%s '%s'", node.Type, node.Label)
	}
	return node.Type
}

func renderHoverBox(srcLines, dstLines []string, actions []*serialize.Action, maxBoxWidth int, pane string, themeOpt ...*Theme) string {
	if len(actions) == 0 || maxBoxWidth <= 0 {
		return ""
	}

	t := defaultTheme
	if len(themeOpt) > 0 {
		t = cmp.Or(themeOpt[0], defaultTheme)
	}

	primary := actions[0]
	kind := parseActionKind(primary.Action)
	accent := t.ActionFg(kind)
	bg := t.UI.Surface0

	cardInnerWidth := min(56, max(8, maxBoxWidth-4))

	var contentLines []string

	for idx, a := range actions {
		if idx > 0 {
			sep := t.Styles.HoverDim.Render(strings.Repeat("─", cardInnerWidth))
			contentLines = append(contentLines, sep)
		}

		aKind := parseActionKind(a.Action)
		aFg := t.ActionFg(aKind)
		aIcon := t.ActionIcon(aKind)

		titleParts := []styledPart{
			{text: aIcon + " ", style: lipgloss.NewStyle().Foreground(aFg)},
			{text: strings.ToUpper(a.Action), style: lipgloss.NewStyle().Foreground(aFg).Bold(true)},
		}
		if a.Node != nil {
			titleParts = append(titleParts, styledPart{
				text:  " " + formatNodeSummary(a.Node),
				style: lipgloss.NewStyle().Foreground(t.UI.Text),
			})
		}
		contentLines = append(contentLines, renderHoverRow(titleParts, cardInnerWidth, bg))

		switch a.Action {
		case "update":
			if a.OldValue != "" && a.NewValue != "" {
				old := truncateStr(a.OldValue, cardInnerWidth/2-2)
				newVal := truncateStr(a.NewValue, cardInnerWidth/2-2)
				valRow := []styledPart{
					{text: fmt.Sprintf("'%s' → '%s'", old, newVal), style: t.Styles.HoverDetail},
				}
				contentLines = append(contentLines, renderHoverRow(valRow, cardInnerWidth, bg))
			}
			if a.Parent != nil {
				parentRow := []styledPart{
					{text: fmt.Sprintf("parent: %s", formatNodeSummary(a.Parent)), style: t.Styles.HoverDetail},
				}
				contentLines = append(contentLines, renderHoverRow(parentRow, cardInnerWidth, bg))
			}
		case "move":
			if pane == "right" {
				if a.Node != nil {
					srcRow := []styledPart{
						{text: fmt.Sprintf("← src: %s (Enter to jump)", formatNodeSummary(a.Node)), style: t.Styles.HoverDetail},
					}
					contentLines = append(contentLines, renderHoverRow(srcRow, cardInnerWidth, bg))
				} else {
					srcRow := []styledPart{
						{text: "← src (Enter to jump)", style: t.Styles.HoverDetail},
					}
					contentLines = append(contentLines, renderHoverRow(srcRow, cardInnerWidth, bg))
				}
			} else {
				if a.DestNode != nil {
					destRow := []styledPart{
						{text: fmt.Sprintf("→ dest: %s (Enter to jump)", formatNodeSummary(a.DestNode)), style: t.Styles.HoverDetail},
					}
					contentLines = append(contentLines, renderHoverRow(destRow, cardInnerWidth, bg))
				} else if a.DestStartByte != nil && a.DestEndByte != nil {
					destRow := []styledPart{
						{text: fmt.Sprintf("→ dest: %s (Enter to jump)", formatByteRange(dstLines, *a.DestStartByte, *a.DestEndByte)), style: t.Styles.HoverDetail},
					}
					contentLines = append(contentLines, renderHoverRow(destRow, cardInnerWidth, bg))
				}
			}
		default:
			if a.Parent != nil {
				parentRow := []styledPart{
					{text: fmt.Sprintf("parent: %s", formatNodeSummary(a.Parent)), style: t.Styles.HoverDetail},
				}
				contentLines = append(contentLines, renderHoverRow(parentRow, cardInnerWidth, bg))
			}
		}

		var metaParts []styledPart
		if a.Action == "move" && pane == "right" {
			if a.DestStartByte != nil && a.DestEndByte != nil {
				metaParts = append(metaParts, styledPart{
					text:  formatByteRange(dstLines, *a.DestStartByte, *a.DestEndByte),
					style: t.Styles.HoverDim,
				})
			} else if a.DestNode != nil {
				metaParts = append(metaParts, styledPart{
					text:  formatByteRange(dstLines, a.DestNode.StartByte, a.DestNode.EndByte),
					style: t.Styles.HoverDim,
				})
			}
		} else if a.Node != nil {
			nodeLines := srcLines
			if a.Node.Tree == "after" || a.Action == "insert" {
				nodeLines = dstLines
			}
			metaParts = append(metaParts, styledPart{
				text:  formatByteRange(nodeLines, a.Node.StartByte, a.Node.EndByte),
				style: t.Styles.HoverDim,
			})
		}
		if a.GroupID != "" && a.Action != "move" {
			sep := ""
			if len(metaParts) > 0 {
				sep = " │ "
			}
			metaParts = append(metaParts, styledPart{
				text:  sep + "grp: " + a.GroupID,
				style: t.Styles.HoverDim,
			})
		}
		if len(metaParts) > 0 {
			contentLines = append(contentLines, renderHoverRow(metaParts, cardInnerWidth, bg))
		}
	}

	boxStyle := t.Styles.HoverCard.BorderForeground(accent).BorderBackground(bg)
	return boxStyle.Render(strings.Join(contentLines, "\n"))
}

func visualColFromByte(lines []string, lineIdx, byteCol int) int {
	if lineIdx < 0 || lineIdx >= len(lines) {
		return 0
	}
	byteToVisual := byteToVisualMapping(lines[lineIdx])
	if byteCol < len(byteToVisual) {
		return byteToVisual[byteCol]
	}
	if len(byteToVisual) > 0 {
		return byteToVisual[len(byteToVisual)-1]
	}
	return 0
}

func (m *model) jumpToMoveCounterpart() {
	actions := actionsAtCursor(m)
	if len(actions) == 0 {
		return
	}

	idx := slices.IndexFunc(actions, func(a *serialize.Action) bool {
		return a.Action == "move"
	})
	if idx == -1 {
		return
	}
	moveAct := actions[idx]

	targetRow := -1
	var targetPane string
	var targetCol int

	if m.activePane == "left" {
		var dstByte uint32
		if moveAct.DestStartByte != nil {
			dstByte = *moveAct.DestStartByte
		} else if moveAct.DestNode != nil {
			dstByte = moveAct.DestNode.StartByte
		} else {
			return
		}
		dstLine, dstCol := byteToLineColFromLines(m.dstLines, dstByte)
		targetCol = visualColFromByte(m.dstLines, dstLine, dstCol)

		targetRow = slices.IndexFunc(m.lineAlignment, func(pair serialize.LineAlignmentPair) bool {
			return pair.RightLine == dstLine
		})
		targetPane = "right"
	} else {
		if moveAct.Node == nil {
			return
		}
		srcLine, srcCol := byteToLineColFromLines(m.srcLines, moveAct.Node.StartByte)
		targetCol = visualColFromByte(m.srcLines, srcLine, srcCol)

		targetRow = slices.IndexFunc(m.lineAlignment, func(pair serialize.LineAlignmentPair) bool {
			return pair.LeftLine == srcLine
		})
		targetPane = "left"
	}

	if targetRow == -1 {
		return
	}

	// Unfold target line if it's inside a closed fold.
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

	if i := slices.IndexFunc(m.virtualLines, func(vl virtualLine) bool {
		return vl.alignedRow == targetRow
	}); i != -1 {
		m.cursorY = i
		m.cursorX = targetCol
		m.activePane = targetPane
		m.scrollY = clamp(m.cursorY-m.contentHeight()/2, 0, m.maxScrollY())

		m.clampCursor()
		m.keepCursorInViewport()
		m.hoverOpen = false
	}
}
