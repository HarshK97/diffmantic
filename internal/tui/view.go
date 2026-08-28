package tui

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	if m.helpOpen {
		return m.renderHelpModal()
	}

	var b strings.Builder

	b.WriteString(m.renderTitleBar())
	b.WriteByte('\n')

	b.WriteString(m.renderContent())

	if m.gitCommitOpen {
		b.WriteByte('\n')
		b.WriteString(m.renderCommitPanel())
	}

	b.WriteByte('\n')
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m model) renderTitleBar() string {
	t := m.getTheme()
	pw := m.paneWidth()
	bg := t.UI.Surface0

	leftTitle := m.srcFile
	if m.gitTreeOpen {
		leftTitle = "Changed Files"
	}
	left := truncateStr(" "+leftTitle, pw)
	right := truncateStr(" "+m.dstFile, pw)

	titleStyle := t.Styles.Title.Background(bg)
	divStyle := lipgloss.NewStyle().Foreground(t.UI.Surface1).Background(bg)

	leftRendered := titleStyle.Render(padRight(left, pw))
	rightRendered := titleStyle.Render(padRight(right, pw))

	div := divStyle.Render("│")

	// Pad any extra column if the screen width is odd.
	totalUsed := pw + dividerWidth + pw
	remainder := ""
	if m.width > totalUsed {
		remainder = titleStyle.Render(strings.Repeat(" ", m.width-totalUsed))
	}

	return leftRendered + div + rightRendered + remainder
}

func (m model) renderContent() string {
	height := m.contentHeight()
	if height <= 0 {
		return ""
	}

	t := m.getTheme()
	pw := m.paneWidth()
	gw := m.gutterWidth()
	tw := max(pw-gw, 1)

	leftLines := m.renderPane(m.srcLines, m.srcHighlights, m.srcSyntax, m.scrollXLeft, height, pw, gw, tw, true)
	rightLines := m.renderPane(m.dstLines, m.dstHighlights, m.dstSyntax, m.scrollXRight, height, pw, gw, tw, false)

	if m.gitTreeOpen {
		treeWidth := max(min(38, pw-4), 20)

		treeHeight := len(m.gitItems) + 2
		maxHeight := max(height-4, 5)
		if treeHeight > maxHeight {
			treeHeight = maxHeight
		}

		innerWidth := treeWidth - 2
		innerHeight := treeHeight - 2
		if innerWidth < 1 {
			innerWidth = 1
		}
		if innerHeight < 1 {
			innerHeight = 1
		}
		treeHeight = innerHeight + 2

		boxStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.UI.Lavender).
			BorderBackground(t.UI.Base).
			Background(t.UI.Base)

		treeContentLines := m.renderGitTreeOverlay(innerHeight, innerWidth)
		treeContent := strings.Join(treeContentLines, "\n")
		boxedTree := boxStyle.Width(innerWidth).Height(innerHeight).Render(treeContent)
		boxedTreeLines := strings.Split(boxedTree, "\n")

		offsetX := 0
		offsetY := 0

		for i := range treeHeight {
			row := offsetY + i
			if row < height && i < len(boxedTreeLines) {
				leftLines[row] = overlayAnsi(leftLines[row], boxedTreeLines[i], offsetX)
			}
		}
	}

	div := t.Styles.Divider.Render("│")

	contentRows := make([]string, height)
	for i := range height {
		// Check if this row is a fold marker: render a unified fold line across the divider.
		vIdx := m.scrollY + i
		if !m.gitTreeOpen && vIdx < len(m.virtualLines) && m.virtualLines[vIdx].foldIdx >= 0 {
			contentRows[i] = leftLines[i] + t.Styles.Divider.Background(t.UI.Surface0).Render("│") + rightLines[i]
		} else {
			contentRows[i] = leftLines[i] + div + rightLines[i]
		}
	}

	if m.hoverOpen && !m.gitTreeOpen && !m.gitCommitOpen && len(m.hoverActions) > 0 {
		hoverPane := m.activePane
		if m.hoverSource == "mouse" {
			if m.hoverX >= pw+dividerWidth {
				hoverPane = "right"
			} else {
				hoverPane = "left"
			}
		}
		boxStr := renderHoverBox(m.srcLines, m.dstLines, m.hoverActions, max(m.width-4, 20), hoverPane, t)
		if boxStr != "" {
			boxLines := strings.Split(boxStr, "\n")
			boxHeight := len(boxLines)
			boxWidth := 0
			for _, l := range boxLines {
				boxWidth = max(boxWidth, lipgloss.Width(l))
			}

			targetX := m.hoverX
			if targetX+boxWidth > m.width {
				targetX = m.width - boxWidth
			}
			if targetX < 0 {
				targetX = 0
			}

			var targetY int
			if m.hoverY-boxHeight >= 0 {
				targetY = m.hoverY - boxHeight
			} else if m.hoverY+1+boxHeight <= height {
				targetY = m.hoverY + 1
			} else {
				targetY = max(0, height-boxHeight)
			}

			for i := range boxHeight {
				row := targetY + i
				if row >= 0 && row < height && i < len(boxLines) {
					contentRows[row] = overlayAnsi(contentRows[row], boxLines[i], targetX)
				}
			}
		}
	}

	return strings.Join(contentRows, "\n")
}

func (m model) renderPane(lines []string, hl *highlights, syntax map[int][]syntaxSpan, scrollX, height, paneWidth, gutterW, textW int, isLeftPane bool) []string {
	t := m.getTheme()
	result := make([]string, height)

	lineNumWidth := max(gutterW-4, 1)
	for i := range height {
		vIdx := m.scrollY + i

		// Past the end of virtual lines.
		if vIdx >= len(m.virtualLines) {
			gutter := t.Styles.LineNum.Render(padRight("~", gutterW))
			content := t.Styles.Content.Render(strings.Repeat(" ", textW))
			result[i] = gutter + content
			continue
		}

		vl := m.virtualLines[vIdx]
		isCursorRow := vIdx == m.cursorY
		isActivePane := (isLeftPane && m.activePane == "left") || (!isLeftPane && m.activePane == "right")
		isCursor := isCursorRow && isActivePane

		// Fold marker row.
		if vl.foldIdx >= 0 {
			result[i] = m.renderFoldLine(vl.foldIdx, paneWidth, isCursor)
			continue
		}

		// Real line.
		var lineIdx int
		if isLeftPane {
			lineIdx = vl.leftLine
		} else {
			lineIdx = vl.rightLine
		}

		if lineIdx == -1 {
			emptyLineNum := strings.Repeat(" ", lineNumWidth)
			badgeStr := renderGutterBadgeWithTheme(nil, isCursorRow && isActivePane, isLeftPane, t)

			var gutter string
			if isCursorRow && isActivePane {
				gutter = t.Styles.CursorGutter.Render(" "+emptyLineNum+" ") + badgeStr + t.Styles.CursorGutter.Render(" ")
			} else {
				gutter = t.Styles.LineNum.Render(" "+emptyLineNum+" ") + badgeStr + t.Styles.LineNum.Render(" ")
			}

			var content string
			fillerText := strings.Repeat("╱", textW)

			if isCursor {
				content = t.Styles.CursorContent.Render(fillerText)
			} else {
				content = t.Styles.LineNum.Render(fillerText)
			}

			result[i] = gutter + content
			continue
		}

		if lineIdx < len(lines) {
			lineSpans := hl.spans[lineIdx]

			symbol := " "
			symStyle := t.Styles.LineNum
			if m.foldStartingAt(vl.alignedRow) >= 0 {
				symbol = "▼"
				symStyle = t.Styles.GutterFold
			} else if m.foldContaining(vl.alignedRow) >= 0 {
				symbol = "·"
				symStyle = t.Styles.LineNum
			}

			// Leave 1 slot for cursor indicator, 1 space, 1 badge slot, and 1 for fold symbol.
			lineNumStr := fmt.Sprintf("%*d", lineNumWidth, lineIdx+1)
			badgeStr := renderGutterBadgeWithTheme(lineSpans, isCursorRow && isActivePane, isLeftPane, t)

			var gutter string
			if isCursorRow && isActivePane {
				gutter = t.Styles.CursorGutter.Render(" "+lineNumStr+" ") + badgeStr + t.Styles.CursorGutter.Render(symbol)
			} else {
				gutter = t.Styles.LineNum.Render(" "+lineNumStr+" ") + badgeStr + symStyle.Render(symbol)
			}

			rawLine := lines[lineIdx]
			var content string
			cursorCol := -1
			if isCursorRow && isActivePane {
				cursorCol = m.cursorX
			}

			paneStr := "left"
			if !isLeftPane {
				paneStr = "right"
			}
			matches := m.searchMatchesFor(vIdx, paneStr)
			syntaxSpans := syntax[lineIdx]

			if len(lineSpans) > 0 || len(syntaxSpans) > 0 || len(matches) > 0 {
				content = m.renderStyledLine(rawLine, lineSpans, syntaxSpans, matches, scrollX, textW, cursorCol, paneStr, vIdx)
			} else {
				// Fast path: no highlights, no syntax, and no search matches
				line := strings.ReplaceAll(rawLine, "\t", "    ")
				runes := []rune(line)
				runeLen := len(runes)

				style := t.Styles.Content
				if isCursor {
					style = t.Styles.CursorContent
				} else if kind, ok := hl.tinted[lineIdx]; ok {
					style = t.HlStyle(kind)
				}

				padStyle := lipgloss.NewStyle().Background(t.UI.Base)
				if isCursor {
					padStyle = padStyle.Background(t.UI.Surface0)
				}

				var b strings.Builder
				for idx := range textW {
					col := scrollX + idx
					var r rune
					var s lipgloss.Style
					if col < runeLen {
						r = runes[col]
						s = style
					} else {
						r = ' '
						s = padStyle
					}

					if col == cursorCol {
						b.WriteString(s.Reverse(true).Blink(true).Render(string(r)))
					} else {
						b.WriteString(s.Render(string(r)))
					}
				}
				content = b.String()
			}
			result[i] = gutter + content
		} else {
			// EOF for this side (the other side might still have lines).
			gutter := t.Styles.LineNum.Render(padRight("~", gutterW))
			content := t.Styles.Content.Render(strings.Repeat(" ", textW))
			result[i] = gutter + content
		}
	}

	return result
}

func (m model) renderFoldLine(foldIdx, paneWidth int, isCursor bool) string {
	t := m.getTheme()
	f := m.folds[foldIdx]
	count := f.endLine - f.startLine + 1
	label := fmt.Sprintf("⋯ %d lines hidden ⋯", count)
	style := t.Styles.Fold
	if isCursor {
		style = t.Styles.CursorFold
	}
	return style.Render(centerPad(label, paneWidth))
}

func centerPad(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	totalPad := width - len(runes)
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

func renderGutterBadgeWithTheme(spans []span, isCursor bool, isLeftPane bool, t *Theme) string {
	t = cmp.Or(t, defaultTheme)
	var hasUpdate, hasInsert, hasDelete, hasMove bool
	for _, s := range spans {
		switch s.kind {
		case kindUpdate:
			hasUpdate = true
		case kindInsert:
			hasInsert = true
		case kindDelete:
			hasDelete = true
		case kindMove:
			hasMove = true
		case kindMoveUpdate:
			hasUpdate = true
			hasMove = true
		}
	}

	inactiveBg := t.UI.Base
	if isCursor {
		inactiveBg = t.UI.Surface1
	}

	// Left Pipe (Foreground of ▌)
	leftColor := inactiveBg
	if isLeftPane && hasDelete {
		leftColor = t.Actions.DeleteFg
	} else if !isLeftPane && hasInsert {
		leftColor = t.Actions.InsertFg
	}

	// Right Pipe (Background of ▌)
	rightColor := inactiveBg
	if hasUpdate && hasMove {
		rightColor = t.Actions.MoveUpdateFg
	} else if hasUpdate {
		rightColor = t.Actions.UpdateFg
	} else if hasMove {
		rightColor = t.Actions.MoveFg
	}

	if leftColor == inactiveBg && rightColor == inactiveBg {
		return lipgloss.NewStyle().Background(inactiveBg).Render(" ")
	}

	return lipgloss.NewStyle().Foreground(leftColor).Background(rightColor).Render("▌")
}

func (m model) renderStyledLine(rawLine string, lineSpans []span, synSpans []syntaxSpan, matches []searchMatch, scrollX, textW int, cursorCol int, pane string, virtualRow int) string {
	t := m.getTheme()
	// Expand tabs and map original byte offsets to visual column positions.
	expanded, byteToVisual := expandLine(rawLine)
	runeLen := len([]rune(expanded))

	// Calculate off-screen chevrons for left/right viewport boundaries
	var hasLeftChevron, hasRightChevron bool
	var leftChevronKind, rightChevronKind actionKind

	closestLeftEnd := -1
	closestRightStart := 1<<31 - 1

	for _, s := range lineSpans {
		sc := 0
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		ec := runeLen
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		}

		if sc >= 0 && ec > sc {
			// Span lies entirely to the left of scrollX
			if ec <= scrollX {
				if ec > closestLeftEnd {
					closestLeftEnd = ec
					leftChevronKind = s.kind
					hasLeftChevron = true
				}
			}
			// Span lies entirely past scrollX + textW
			if sc >= scrollX+textW {
				if sc < closestRightStart {
					closestRightStart = sc
					rightChevronKind = s.kind
					hasRightChevron = true
				}
			}
		}
	}

	// Action kind and total byte length for each column.
	// Inner (smaller) AST nodes override outer container nodes.
	colHighlight := make([]int, runeLen)
	colSpanLen := make([]int, runeLen)
	colCandidateLen := make([]int, runeLen)
	colHasMove := make([]bool, runeLen)
	colHasUpdate := make([]bool, runeLen)
	for i := range colHighlight {
		colHighlight[i] = -1
		colSpanLen[i] = 1<<31 - 1 // max int sentinel
		colCandidateLen[i] = 1<<31 - 1
	}
	for _, s := range lineSpans {
		sc := -1
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		var ec int
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = runeLen
		}
		if sc >= 0 && ec > sc {
			candidateLen := ec - sc
			astLen := getSpanASTLen(s, pane)
			for col := sc; col < ec && col < runeLen; col++ {
				if s.kind == kindMove || s.kind == kindMoveUpdate {
					colHasMove[col] = true
				}
				if s.kind == kindUpdate || s.kind == kindMoveUpdate {
					colHasUpdate[col] = true
				}

				curLen := colSpanLen[col]
				if colHighlight[col] == -1 || astLen < curLen ||
					(astLen == curLen && candidateLen < colCandidateLen[col]) ||
					(astLen == curLen && candidateLen == colCandidateLen[col] && actionPriority(s.kind) > actionPriority(actionKind(colHighlight[col]))) {
					colHighlight[col] = int(s.kind)
					colSpanLen[col] = astLen
					colCandidateLen[col] = candidateLen
				}
			}
		}
	}

	for col := range colHighlight {
		if colHighlight[col] == int(kindUpdate) && colHasMove[col] {
			colHighlight[col] = int(kindMoveUpdate)
		}
	}

	colSyntax := make([]lipgloss.Color, runeLen)
	for _, s := range synSpans {
		sc := -1
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		var ec int
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = runeLen
		}
		if sc >= 0 && ec > sc {
			for col := sc; col < ec && col < runeLen; col++ {
				if colSyntax[col] == "" {
					colSyntax[col] = s.color
				}
			}
		}
	}

	basePadStyle := lipgloss.NewStyle().Background(t.UI.Base)
	if cursorCol >= 0 {
		basePadStyle = basePadStyle.Background(t.UI.Surface0)
	}

	expRunes := []rune(expanded)
	var b strings.Builder
	for idx := range textW {
		col := scrollX + idx
		var style lipgloss.Style
		var r rune

		if idx == 0 && hasLeftChevron {
			r = '<'
			style = lipgloss.NewStyle().Foreground(t.ActionFg(leftChevronKind)).Background(t.UI.Base)
			if cursorCol >= 0 {
				style = style.Background(t.UI.Surface0)
			}
		} else if idx == textW-1 && hasRightChevron {
			r = '>'
			style = lipgloss.NewStyle().Foreground(t.ActionFg(rightChevronKind)).Background(t.UI.Base)
			if cursorCol >= 0 {
				style = style.Background(t.UI.Surface0)
			}
		} else if col < runeLen {
			r = expRunes[col]

			searchStyleState := -1
			for _, match := range matches {
				if col >= match.startCol && col < match.endCol {
					searchStyleState = 0
				}
			}
			if searchStyleState == 0 && m.searchMatchIdx >= 0 && m.searchMatchIdx < len(m.searchMatches) {
				active := m.searchMatches[m.searchMatchIdx]
				if active.virtualRow == virtualRow && active.pane == pane && col >= active.startCol && col < active.endCol {
					searchStyleState = 1
				}
			}

			switch searchStyleState {
			case 1:
				style = t.Styles.SearchActiveHl
			case 0:
				style = t.Styles.SearchHl
			default:
				actionIdx := colHighlight[col]
				synColor := colSyntax[col]

				if actionIdx >= 0 {
					style = t.HlStyle(actionKind(actionIdx))
					if actionKind(actionIdx) == kindMoveUpdate {
						style = style.Foreground(t.Actions.UpdateFg).Underline(true)
					} else if synColor != "" {
						style = style.Foreground(synColor)
					} else {
						style = style.Foreground(t.UI.Text)
					}
				} else if synColor != "" {
					if cursorCol >= 0 {
						style = t.Styles.CursorContent.Foreground(synColor)
					} else {
						style = t.Styles.Content.Foreground(synColor)
					}
				} else {
					if cursorCol >= 0 {
						style = t.Styles.CursorContent
					} else {
						style = t.Styles.Content
					}
				}
			}
		} else {
			r = ' '
			style = basePadStyle
		}

		if col == cursorCol {
			b.WriteString(style.Reverse(true).Blink(true).Render(string(r)))
		} else {
			b.WriteString(style.Render(string(r)))
		}
	}

	return b.String()
}

func (m model) renderStatusBar() string {
	t := m.getTheme()
	bg := t.UI.Surface0
	cursorActions := actionsAtCursor(&m)
	hasActions := len(cursorActions) > 0

	var keys string
	if m.gitMode {
		if m.gitTreeOpen {
			if m.gitStagedOnly {
				keys = " j/k: scroll • t: toggle tree • enter: open diff • q: quit"
			} else {
				keys = " j/k: scroll • s/u: stage/unstage • c: commit • t: toggle tree • enter: open diff • q: quit"
			}
		} else if hasActions {
			keys = " j/k: scroll • t: tree • za: fold • K: hover • ?: help • q: quit"
		} else {
			keys = " j/k: scroll • t: toggle tree • za: fold • K: hover • ?: help • q: quit"
		}
	} else if hasActions {
		keys = " j/k • za • K • ?: help • q"
	} else {
		keys = " j/k: scroll • za: fold • K: hover • ?: help • q: quit"
	}

	prefix := m.digitBuffer
	if m.pendingZ {
		prefix += "z"
	}
	if m.pendingBracket != "" {
		prefix += m.pendingBracket
	}
	prefixLen := len([]rune(prefix))

	var keysPart string
	if prefixLen > 0 {
		keysPart = keys + " (" + prefix + ")"
	} else {
		keysPart = keys
	}

	keysWidth := lipgloss.Width(keysPart)

	// Action preview or conflict warning.
	var preview string
	var warningStyle bool
	if m.conflictWarning != "" {
		preview = m.conflictWarning
		warningStyle = true
	} else if hasActions && !m.gitTreeOpen && !m.gitCommitOpen {
		availForPreview := m.width - keysWidth - 2
		if availForPreview > 10 {
			preview = formatActionPreview(cursorActions, availForPreview, t)
		}
	}

	baseStyle := t.Styles.Status.Background(bg)
	if preview != "" {
		previewWidth := lipgloss.Width(preview)
		padding := m.width - keysWidth - previewWidth
		if padding >= 0 {
			var formattedPreview string
			if warningStyle {
				formattedPreview = lipgloss.NewStyle().Foreground(t.UI.Red).Background(bg).Render(preview)
			} else {
				formattedPreview = preview
			}
			keysRendered := baseStyle.Render(keysPart)
			padRendered := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", padding))
			return keysRendered + padRendered + formattedPreview
		}
	}

	truncatedKeys := truncateStr(keysPart, m.width)
	return baseStyle.Render(padRight(truncatedKeys, m.width))
}

func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

func padRight(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func (m model) renderGitTreeOverlay(height, paneWidth int) []string {
	t := m.getTheme()
	result := make([]string, height)

	scrollY := 0
	if m.gitCursorY >= height {
		scrollY = m.gitCursorY - height + 1
	}
	if scrollY < 0 {
		scrollY = 0
	}

	for i := range height {
		idx := scrollY + i
		if idx >= len(m.gitItems) {
			result[i] = lipgloss.NewStyle().Background(t.UI.Base).Render(strings.Repeat(" ", paneWidth))
			continue
		}

		item := m.gitItems[idx]
		isCursorRow := idx == m.gitCursorY

		var line string
		if item.isHeader {
			line = t.Styles.GitHeader.Render(" " + item.headerText)
		} else {
			statusColor := t.UI.Subtext0
			statusChar := item.status

			// Conflicts show up in pink so they pop immediately.
			isConflict := strings.Contains(item.rawStatus, "U") || item.rawStatus == "AA" || item.rawStatus == "DD"
			if isConflict {
				statusChar = "CF"
				statusColor = t.UI.Pink
			} else {
				cleanStatus := strings.TrimSpace(statusChar)
				switch cleanStatus {
				case "M":
					statusColor = t.UI.Yellow
				case "A", "??":
					statusColor = t.UI.Green
				case "D":
					statusColor = t.UI.Red
				default:
					if strings.HasPrefix(cleanStatus, "R") {
						statusColor = t.UI.Blue
					}
				}
			}

			statusStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(statusColor).
				Background(t.UI.Base)

			if isCursorRow {
				statusStyle = statusStyle.Background(t.UI.Surface1)
			}

			cursorStr := "  "
			itemStyle := t.Styles.Content.Background(t.UI.Base)
			if isCursorRow {
				cursorStr = "█ "
				itemStyle = t.Styles.GitCursor
			}

			renderedStatus := statusStyle.Render(statusChar)

			pathStr := item.path
			if item.oldPath != "" {
				pathStr = item.oldPath + " -> " + item.path
			}

			maxPathWidth := max(paneWidth-7, 5)
			truncatedPath := truncateStr(pathStr, maxPathWidth)

			lineContent := renderedStatus + " " + itemStyle.Render(truncatedPath)

			if isCursorRow {
				cursorStyle := t.Styles.GitCursor
				line = cursorStyle.Render(cursorStr) + lineContent
			} else {
				normalStyle := t.Styles.GitNormal
				line = normalStyle.Render(cursorStr) + lineContent
			}
		}

		// Pad short lines to the full pane width.
		lineLen := lipgloss.Width(line)
		if lineLen < paneWidth {
			bgStyle := lipgloss.NewStyle()
			if isCursorRow {
				bgStyle = bgStyle.Background(t.UI.Surface1)
			} else {
				bgStyle = bgStyle.Background(t.UI.Base)
			}
			line += bgStyle.Render(strings.Repeat(" ", paneWidth-lineLen))
		}

		result[i] = line
	}

	return result
}

func (m model) renderCommitPanel() string {
	t := m.getTheme()
	pw := m.width
	inputView := m.gitCommitInput.View()
	return t.Styles.CommitPanel.Render(padRight(inputView, pw))
}

type ansiCell struct {
	char  rune
	style string
}

func parseAnsi(s string) []ansiCell {
	var cells []ansiCell
	var currentStyle string
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			start := i
			i += 2
			for i < len(runes) && runes[i] != 'm' {
				i++
			}
			if i < len(runes) {
				style := string(runes[start : i+1])
				if style == "\x1b[0m" {
					currentStyle = ""
				} else {
					currentStyle += style
				}
			}
		} else {
			cells = append(cells, ansiCell{
				char:  runes[i],
				style: currentStyle,
			})
		}
	}
	return cells
}

func overlayCells(bgCells, fgCells []ansiCell, startX int) []ansiCell {
	totalLen := max(len(bgCells), startX+len(fgCells))
	result := make([]ansiCell, totalLen)
	for i := len(bgCells); i < totalLen; i++ {
		result[i] = ansiCell{char: ' '}
	}
	copy(result, bgCells)
	copy(result[startX:], fgCells)
	return result
}

func cellsToAnsi(cells []ansiCell) string {
	var b strings.Builder
	var activeStyle string
	for _, cell := range cells {
		if cell.style != activeStyle {
			if activeStyle != "" {
				b.WriteString("\x1b[0m")
			}
			b.WriteString(cell.style)
			activeStyle = cell.style
		}
		b.WriteRune(cell.char)
	}
	if activeStyle != "" {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func overlayAnsi(bg, fg string, x int) string {
	bgCells := parseAnsi(bg)
	fgCells := parseAnsi(fg)
	overlaid := overlayCells(bgCells, fgCells, x)
	return cellsToAnsi(overlaid)
}

func byteToVisualMapping(line string) []int {
	byteToVisual := make([]int, len(line)+1)
	visualCol := 0
	for byteIdx, r := range line {
		byteToVisual[byteIdx] = visualCol
		if r == '\t' {
			visualCol += 4
		} else {
			visualCol++
		}
	}
	byteToVisual[len(line)] = visualCol
	return byteToVisual
}

func expandLine(line string) (string, []int) {
	return strings.ReplaceAll(line, "\t", "    "), byteToVisualMapping(line)
}
