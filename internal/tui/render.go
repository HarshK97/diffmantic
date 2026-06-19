package tui

import (
	"fmt"
	"sort"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)

const lineNumberWidth = 4

type moveConnection struct {
	srcMid int
	dstMid int
}

func renderDiffContent(file DiffFile, width int, s styles) string {
	if width < 12 {
		width = 12
	}

	leftAnn, rightAnn := buildLineAnnotations(file)

	// 1. Build maps of changed lines
	oldChanged := make([]bool, len(file.OldLines)+1)
	newChanged := make([]bool, len(file.NewLines)+1)

	for _, h := range file.displayHunks() {
		if h.Kind == ChangeDelete || h.Kind == ChangeUpdate || h.Kind == ChangeMove {
			for l := h.SrcStartLine; l <= h.SrcEndLine; l++ {
				if l > 0 && l < len(oldChanged) {
					oldChanged[l] = true
				}
			}
		}
		if h.Kind == ChangeInsert || h.Kind == ChangeUpdate || h.Kind == ChangeMove {
			for r := h.DstStartLine; r <= h.DstEndLine; r++ {
				if r > 0 && r < len(newChanged) {
					newChanged[r] = true
				}
			}
		}
	}

	// 2. Extract context lines
	var oldContext []int
	for l := 1; l <= len(file.OldLines); l++ {
		if !oldChanged[l] {
			oldContext = append(oldContext, l)
		}
	}
	var newContext []int
	for r := 1; r <= len(file.NewLines); r++ {
		if !newChanged[r] {
			newContext = append(newContext, r)
		}
	}

	// 3. Align context lines using LCS (distance-penalized to match nearest occurrences)
	baseScore := len(file.OldLines) + len(file.NewLines) + 1000
	dp := make([][]int, len(oldContext)+1)
	for i := range dp {
		dp[i] = make([]int, len(newContext)+1)
	}
	for i := 1; i <= len(oldContext); i++ {
		for j := 1; j <= len(newContext); j++ {
			if file.OldLines[oldContext[i-1]-1] == file.NewLines[newContext[j-1]-1] {
				score := baseScore - absVal(oldContext[i-1]-newContext[j-1])
				dp[i][j] = dp[i-1][j-1] + score
			} else {
				dp[i][j] = maxInt(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	matchLeftToRight := make(map[int]int)
	matchRightToLeft := make(map[int]int)
	i, j := len(oldContext), len(newContext)
	for i > 0 && j > 0 {
		if file.OldLines[oldContext[i-1]-1] == file.NewLines[newContext[j-1]-1] {
			score := baseScore - absVal(oldContext[i-1]-newContext[j-1])
			if dp[i][j] == dp[i-1][j-1]+score {
				matchLeftToRight[oldContext[i-1]] = newContext[j-1]
				matchRightToLeft[newContext[j-1]] = oldContext[i-1]
				i--
				j--
				continue
			}
		}
		if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	// 4. Identify unmatched destination context lines and add them as virtual Insert hunks
	var extraHunks []Hunk
	for _, r := range newContext {
		if matchRightToLeft[r] == 0 {
			extraHunks = append(extraHunks, Hunk{
				Kind:         ChangeInsert,
				DstStartLine: r,
				DstEndLine:   r,
			})
		}
	}

	// 5. Gather and sort all hunks by boundary position.
	// For ChangeMove hunks, we split them into a left-only source hunk and a right-only destination hunk.
	type hunkWithVirtual struct {
		hunk         Hunk
		srcBoundary  int // The old-file line number after which this hunk belongs
		subOrder     int // 1 for left-only, 2 for update, 3 for right-only
		dstStartLine int // To preserve stable sort order
		isMove       bool
		origSrcS     int
		origSrcE     int
		origDstS     int
		origDstE     int
		isDst        bool
	}
	var hunksWithV []hunkWithVirtual

	for _, h := range file.displayHunks() {
		if h.Kind == ChangeMove {
			// Source block (left-only)
			srcH := h
			srcH.DstStartLine = 0
			srcH.DstEndLine = 0

			// Destination block (right-only)
			dstH := h
			dstH.SrcStartLine = 0
			dstH.SrcEndLine = 0

			// Calculate prevCtxSrc for dstH
			prevCtxSrc := 0
			for r := h.DstStartLine - 1; r >= 1; r-- {
				if src := matchRightToLeft[r]; src > 0 {
					prevCtxSrc = src
					break
				}
			}

			// Place source block at its source position, and destination block at its destination position
			hunksWithV = append(hunksWithV, hunkWithVirtual{
				hunk:         srcH,
				srcBoundary:  h.SrcStartLine - 1,
				subOrder:     1,
				dstStartLine: h.DstStartLine,
				isMove:       true,
				origSrcS:     h.SrcStartLine,
				origSrcE:     h.SrcEndLine,
				origDstS:     h.DstStartLine,
				origDstE:     h.DstEndLine,
				isDst:        false,
			})
			hunksWithV = append(hunksWithV, hunkWithVirtual{
				hunk:         dstH,
				srcBoundary:  prevCtxSrc,
				subOrder:     3,
				dstStartLine: h.DstStartLine,
				isMove:       true,
				origSrcS:     h.SrcStartLine,
				origSrcE:     h.SrcEndLine,
				origDstS:     h.DstStartLine,
				origDstE:     h.DstEndLine,
				isDst:        true,
			})
		} else {
			var srcBoundary int
			var subOrder int
			if h.DstStartLine == 0 { // Left-only (Delete)
				srcBoundary = h.SrcStartLine - 1
				subOrder = 1
			} else if h.SrcStartLine == 0 { // Right-only (Insert)
				prevCtxSrc := 0
				for r := h.DstStartLine - 1; r >= 1; r-- {
					if src := matchRightToLeft[r]; src > 0 {
						prevCtxSrc = src
						break
					}
				}
				srcBoundary = prevCtxSrc
				subOrder = 3
			} else { // Side-by-side (Update)
				srcBoundary = h.SrcStartLine - 1
				subOrder = 2
			}
			hunksWithV = append(hunksWithV, hunkWithVirtual{
				hunk:         h,
				srcBoundary:  srcBoundary,
				subOrder:     subOrder,
				dstStartLine: h.DstStartLine,
			})
		}
	}

	for _, h := range extraHunks {
		prevCtxSrc := 0
		for r := h.DstStartLine - 1; r >= 1; r-- {
			if src := matchRightToLeft[r]; src > 0 {
				prevCtxSrc = src
				break
			}
		}
		hunksWithV = append(hunksWithV, hunkWithVirtual{
			hunk:         h,
			srcBoundary:  prevCtxSrc,
			subOrder:     3,
			dstStartLine: h.DstStartLine,
		})
	}

	sort.SliceStable(hunksWithV, func(i, j int) bool {
		if hunksWithV[i].srcBoundary != hunksWithV[j].srcBoundary {
			return hunksWithV[i].srcBoundary < hunksWithV[j].srcBoundary
		}
		if hunksWithV[i].subOrder != hunksWithV[j].subOrder {
			return hunksWithV[i].subOrder < hunksWithV[j].subOrder
		}
		return hunksWithV[i].dstStartLine < hunksWithV[j].dstStartLine
	})

	// 6. Build the aligned rows
	type alignedRow struct {
		leftLineNum  int
		rightLineNum int
		ghostText    string
		isGhostLeft  bool
	}
	var alignedRows []alignedRow
	srcRendered := make([]bool, len(file.OldLines)+1)

	appendHunk := func(info hunkWithVirtual) {
		h := info.hunk
		if info.isMove {
			if !info.isDst {
				// Source block: insert ghost text above
				var ghostText string
				if info.origDstS == info.origDstE {
					ghostText = fmt.Sprintf("Moved To Line Number %d", info.origDstS)
				} else {
					ghostText = fmt.Sprintf("Moved To Line Number %d-%d", info.origDstS, info.origDstE)
				}
				alignedRows = append(alignedRows, alignedRow{
					leftLineNum:  0,
					rightLineNum: 0,
					ghostText:    ghostText,
					isGhostLeft:  true,
				})
				// Render lines
				for src := h.SrcStartLine; src <= h.SrcEndLine; src++ {
					alignedRows = append(alignedRows, alignedRow{leftLineNum: src, rightLineNum: 0})
					srcRendered[src] = true
				}
			} else {
				// Destination block: insert ghost text above
				var ghostText string
				if info.origSrcS == info.origSrcE {
					ghostText = fmt.Sprintf("Moved From Line Number %d", info.origSrcS)
				} else {
					ghostText = fmt.Sprintf("Moved From Line Number %d-%d", info.origSrcS, info.origSrcE)
				}
				alignedRows = append(alignedRows, alignedRow{
					leftLineNum:  0,
					rightLineNum: 0,
					ghostText:    ghostText,
					isGhostLeft:  false,
				})
				// Render lines
				for r := h.DstStartLine; r <= h.DstEndLine; r++ {
					alignedRows = append(alignedRows, alignedRow{leftLineNum: 0, rightLineNum: r})
				}
			}
		} else {
			// Normal hunk
			if h.DstStartLine == 0 { // Left-only (Delete)
				for src := h.SrcStartLine; src <= h.SrcEndLine; src++ {
					alignedRows = append(alignedRows, alignedRow{leftLineNum: src, rightLineNum: 0})
					srcRendered[src] = true
				}
			} else if h.SrcStartLine == 0 { // Right-only (Insert)
				for r := h.DstStartLine; r <= h.DstEndLine; r++ {
					alignedRows = append(alignedRows, alignedRow{leftLineNum: 0, rightLineNum: r})
				}
			} else {
				// Side-by-side (Update)
				numSrc := h.SrcEndLine - h.SrcStartLine + 1
				numDst := h.DstEndLine - h.DstStartLine + 1
				maxLines := maxInt(numSrc, numDst)
				for k := 0; k < maxLines; k++ {
					srcLine := 0
					if k < numSrc {
						srcLine = h.SrcStartLine + k
						srcRendered[srcLine] = true
					}
					dstLine := 0
					if k < numDst {
						dstLine = h.DstStartLine + k
					}
					alignedRows = append(alignedRows, alignedRow{leftLineNum: srcLine, rightLineNum: dstLine})
				}
			}
		}
	}

	hunkIndex := 0
	for l := 1; l <= len(file.OldLines); l++ {
		if srcRendered[l] {
			continue
		}

		// Process hunks before line l
		for hunkIndex < len(hunksWithV) && hunksWithV[hunkIndex].srcBoundary < l {
			info := hunksWithV[hunkIndex]
			hunkIndex++
			appendHunk(info)
		}

		// Since srcRendered[l] could have been set by a hunk in the loop above, check again
		if srcRendered[l] {
			continue
		}

		// Process context line l
		r := matchLeftToRight[l]
		alignedRows = append(alignedRows, alignedRow{leftLineNum: l, rightLineNum: r})
		srcRendered[l] = true
	}

	// Process remaining trailing hunks
	for hunkIndex < len(hunksWithV) {
		info := hunksWithV[hunkIndex]
		hunkIndex++
		appendHunk(info)
	}

	// 7. Collect move connections in terms of aligned row indices
	var moves []moveConnection
	for _, h := range file.displayHunks() {
		if h.Kind == ChangeMove {
			srcRawMid := (h.SrcStartLine + h.SrcEndLine) / 2
			dstRawMid := (h.DstStartLine + h.DstEndLine) / 2

			srcRowMid := -1
			dstRowMid := -1
			for idx, row := range alignedRows {
				if row.ghostText == "" { // Only match code rows
					if row.leftLineNum == srcRawMid {
						srcRowMid = idx + 1 // 1-indexed for renderMiddlePart loop
					}
					if row.rightLineNum == dstRawMid {
						dstRowMid = idx + 1 // 1-indexed for renderMiddlePart loop
					}
				}
			}

			if srcRowMid > 0 && dstRowMid > 0 {
				moves = append(moves, moveConnection{srcMid: srcRowMid, dstMid: dstRowMid})
			}
		}
	}

	middleW := 3 // Width of the connection curve area
	// Border separators: left border is 1, right border is 1. Total = 2 columns.
	panelW := maxInt(1, (width-middleW-2)/2)
	rightW := maxInt(1, width-panelW-middleW-2)
	if len(alignedRows) == 0 {
		return s.Help.Render("(empty files)")
	}

	var b strings.Builder
	ghostStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Bold(true).Italic(true)
	prefixSpaces := strings.Repeat(" ", lineNumberWidth+1)

	for i := 1; i <= len(alignedRows); i++ {
		row := alignedRows[i-1]

		if row.ghostText != "" {
			var leftOut, rightOut string
			if row.isGhostLeft {
				contentW := maxInt(0, panelW-lipgloss.Width(prefixSpaces))
				truncatedGhost := truncateToWidth(row.ghostText, contentW)
				renderedGhost := ghostStyle.Render(truncatedGhost)
				padding := strings.Repeat(" ", maxInt(0, contentW-lipgloss.Width(truncatedGhost)))
				leftOut = prefixSpaces + renderedGhost + padding

				rightContentW := maxInt(0, rightW-lipgloss.Width(prefixSpaces))
				rightOut = prefixSpaces + s.Separator.Render(strings.Repeat("╱", rightContentW))
			} else {
				leftContentW := maxInt(0, panelW-lipgloss.Width(prefixSpaces))
				leftOut = prefixSpaces + s.Separator.Render(strings.Repeat("╱", leftContentW))

				contentW := maxInt(0, rightW-lipgloss.Width(prefixSpaces))
				truncatedGhost := truncateToWidth(row.ghostText, contentW)
				renderedGhost := ghostStyle.Render(truncatedGhost)
				padding := strings.Repeat(" ", maxInt(0, contentW-lipgloss.Width(truncatedGhost)))
				rightOut = prefixSpaces + renderedGhost + padding
			}

			middleContent := renderMiddlePart(i, moves, middleW, s)

			b.WriteString(leftOut)
			b.WriteString(s.Separator.Render("│"))
			b.WriteString(middleContent)
			b.WriteString(s.Separator.Render("│"))
			b.WriteString(rightOut)
		} else {
			leftLineNum := row.leftLineNum
			rightLineNum := row.rightLineNum

			leftLine, leftOK := lineAt(file.OldLines, leftLineNum)
			rightLine, rightOK := lineAt(file.NewLines, rightLineNum)
			leftToks := tokensAt(file.OldTokens, leftLineNum)
			rightToks := tokensAt(file.NewTokens, rightLineNum)
			leftAnnotation, leftAnnotated := leftAnn[leftLineNum]
			rightAnnotation, rightAnnotated := rightAnn[rightLineNum]

			leftOut := renderSideLine(leftLineNum, leftLine, leftToks, leftOK, leftAnnotation, leftAnnotated, panelW, s)
			middleContent := renderMiddlePart(i, moves, middleW, s)
			rightOut := renderSideLine(rightLineNum, rightLine, rightToks, rightOK, rightAnnotation, rightAnnotated, rightW, s)

			b.WriteString(leftOut)
			b.WriteString(s.Separator.Render("│"))
			b.WriteString(middleContent)
			b.WriteString(s.Separator.Render("│"))
			b.WriteString(rightOut)
		}

		if i < len(alignedRows) {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func renderMiddlePart(line int, moves []moveConnection, width int, s styles) string {
	if width <= 0 {
		return ""
	}

	// First pass: check if any move has a bend/straight arrow on this line
	for _, m := range moves {
		if line == m.srcMid {
			if m.srcMid < m.dstMid {
				// Bends down: e.g. "─╮"
				return s.MoveArrow.Render("─╮" + strings.Repeat(" ", maxInt(0, width-2)))
			} else if m.srcMid > m.dstMid {
				// Bends up: e.g. "─╯"
				return s.MoveArrow.Render("─╯" + strings.Repeat(" ", maxInt(0, width-2)))
			} else {
				// Straight: e.g. "──❯"
				if width > 1 {
					return s.MoveArrow.Render(strings.Repeat("─", width-1) + "❯")
				}
				return s.MoveArrow.Render("❯")
			}
		}

		if line == m.dstMid {
			if m.srcMid < m.dstMid {
				// Coming from above: e.g. " ╰❯"
				if width >= 3 {
					return s.MoveArrow.Render(" " + "╰" + strings.Repeat("─", width-3) + "❯")
				}
				return s.MoveArrow.Render("╰" + strings.Repeat("─", maxInt(0, width-2)) + "❯")
			} else if m.srcMid > m.dstMid {
				// Coming from below: e.g. " ╭❯"
				if width >= 3 {
					return s.MoveArrow.Render(" " + "╭" + strings.Repeat("─", width-3) + "❯")
				}
				return s.MoveArrow.Render("╭" + strings.Repeat("─", maxInt(0, width-2)) + "❯")
			}
		}
	}

	// Second pass: check if any move has a vertical connecting line on this line
	for _, m := range moves {
		minL := minInt(m.srcMid, m.dstMid)
		maxL := maxInt(m.srcMid, m.dstMid)
		if line > minL && line < maxL {
			leftSpace := maxInt(0, (width-1)/2)
			rightSpace := maxInt(0, width-leftSpace-1)

			distSource := absVal(line - m.srcMid)
			distDest := absVal(line - m.dstMid)

			var char string
			if distSource == 1 || distDest == 1 {
				char = s.MoveArrow.Render("│")
			} else if distSource == 2 || distDest == 2 {
				char = s.MoveArrow.Render("|")
			} else {
				char = " "
			}
			return strings.Repeat(" ", leftSpace) + char + strings.Repeat(" ", rightSpace)
		}
	}

	return strings.Repeat(" ", width)
}

func absVal(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func lineAt(lines []string, line int) (string, bool) {
	if line <= 0 || line > len(lines) {
		return "", false
	}
	return lines[line-1], true
}

func tokensAt(tokens [][]chroma.Token, line int) []chroma.Token {
	if line <= 0 || line > len(tokens) {
		return nil
	}
	return tokens[line-1]
}

func renderSideLine(line int, content string, tokens []chroma.Token, ok bool, ann lineAnnotation, annotated bool, width int, s styles) string {
	if width <= 0 {
		return ""
	}

	lineNo := fmt.Sprintf("%*d", lineNumberWidth, line)
	if !ok {
		lineNo = strings.Repeat(" ", lineNumberWidth)
	}

	fillStyle := lipgloss.NewStyle()
	baseStyle := s.Context

	if annotated {
		switch ann.Kind {
		case ChangeInsert:
			fillStyle = s.InsertFill
		case ChangeDelete:
			fillStyle = s.DeleteFill
		case ChangeUpdate:
			fillStyle = s.UpdateFill
		case ChangeMove:
			fillStyle = s.MoveFill
		}
		baseStyle = fillStyle
	}

	if !ok {
		numStyle := s.LineNumber
		prefix := numStyle.Render(lineNo) + " "
		contentW := maxInt(0, width-lipgloss.Width(prefix))
		if contentW <= 0 {
			return prefix
		}
		filler := strings.Repeat("╱", contentW)
		return prefix + s.Separator.Render(filler)
	}

	numStyle := s.LineNumber
	if annotated {
		numStyle = s.LineNumber.Foreground(fillStyle.GetForeground())
	}
	prefix := numStyle.Render(lineNo) + " "
	contentW := maxInt(0, width-lipgloss.Width(prefix))
	spans := ann.Spans
	if annotated && (ann.Kind == ChangeInsert || ann.Kind == ChangeDelete) {
		spans = nil
	}
	rendered := renderTokenizedContent(content, tokens, spans, contentW, baseStyle, s)
	padding := ""
	if pad := contentW - lipgloss.Width(rendered); pad > 0 {
		padding = fillStyle.Render(strings.Repeat(" ", pad))
	}
	return prefix + rendered + padding
}

func renderTokenizedContent(originalContent string, tokens []chroma.Token, spans []visualSpan, width int, baseStyle lipgloss.Style, s styles) string {
	if width <= 0 {
		return ""
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].StartCol != spans[j].StartCol {
			return spans[i].StartCol < spans[j].StartCol
		}
		return spans[i].Priority > spans[j].Priority
	})

	// If Chroma tokens are available, use the token-based loop to combine syntax & diff styles
	if len(tokens) > 0 {
		var b strings.Builder
		used := 0
		byteIndex := 0

		monokaiStyle := chromastyles.Get("monokai")
		if monokaiStyle == nil {
			monokaiStyle = chromastyles.Fallback
		}

		for _, tok := range tokens {
			chromaEntry := monokaiStyle.Get(tok.Type)

			for _, r := range tok.Value {
				style := baseStyle

				// Map syntax highlighting colors (foreground only to prevent overriding diff panel backgrounds)
				if chromaEntry.Colour.IsSet() {
					style = style.Foreground(lipgloss.Color(chromaEntry.Colour.String()))
				}
				if chromaEntry.Bold == chroma.Yes {
					style = style.Bold(true)
				}
				if chromaEntry.Italic == chroma.Yes {
					style = style.Italic(true)
				}
				if chromaEntry.Underline == chroma.Yes {
					style = style.Underline(true)
				}

				// If this rune is within a diff/span annotation, inherit/overlay it
				if kind, ok := spanKindAt(spans, byteIndex); ok {
					spanStyle := tokenStyleForKind(kind, s)
					style = spanStyle.Inherit(style)
				}

				runeLen := len(string(r))
				if r == '\t' {
					// Tab is expanded to 4 spaces
					for k := 0; k < 4; k++ {
						rw := 1
						if used+rw > width {
							if used < width {
								b.WriteString(baseStyle.Render("."))
								used += rw
							}
							break
						}
						b.WriteString(style.Render(" "))
						used += rw
					}
					if used >= width {
						break
					}
				} else {
					rw := lipgloss.Width(string(r))
					if used+rw > width {
						if used < width {
							b.WriteString(baseStyle.Render("."))
						}
						break
					}
					b.WriteString(style.Render(string(r)))
					used += rw
				}

				byteIndex += runeLen
			}
			if used >= width {
				break
			}
		}
		return b.String()
	}

	// Fallback to raw plain text rendering
	var b strings.Builder
	used := 0
	for byteIndex, r := range originalContent {
		style := baseStyle
		if kind, ok := spanKindAt(spans, byteIndex); ok {
			style = tokenStyleForKind(kind, s).Inherit(baseStyle)
		}

		if r == '\t' {
			for k := 0; k < 4; k++ {
				rw := 1
				if used+rw > width {
					if used < width {
						b.WriteString(baseStyle.Render("."))
						used += rw
					}
					break
				}
				b.WriteString(style.Render(" "))
				used += rw
			}
			if used >= width {
				break
			}
		} else {
			rw := lipgloss.Width(string(r))
			if used+rw > width {
				if used < width {
					b.WriteString(baseStyle.Render("."))
				}
				break
			}
			b.WriteString(style.Render(string(r)))
			used += rw
		}
	}
	return b.String()
}

func spanKindAt(spans []visualSpan, col int) (ChangeKind, bool) {
	bestPriority := 0
	var kind ChangeKind
	for _, span := range spans {
		end := span.EndCol
		if end <= 0 {
			end = span.StartCol + 1
		}
		if col >= span.StartCol && col < end && span.Priority >= bestPriority {
			bestPriority = span.Priority
			kind = span.Kind
		}
	}
	return kind, bestPriority > 0
}

func tokenStyleForKind(kind ChangeKind, s styles) lipgloss.Style {
	switch kind {
	case ChangeInsert:
		return s.Insert
	case ChangeDelete:
		return s.Delete
	case ChangeUpdate:
		return s.Update
	case ChangeMove:
		return s.Move
	default:
		return s.Context
	}
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)) > width-1 {
			break
		}
		b.WriteRune(r)
	}
	if width == 1 {
		return "."
	}
	return b.String() + "."
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
