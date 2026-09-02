// Package inline renders AST-aware inline diffs.
package inline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

type lineKind int

const (
	kindContext lineKind = iota
	kindDelete
	kindInsert
)

type interval struct {
	start int
	end   int
}

type hunkLine struct {
	kind       lineKind
	srcLineIdx int
	dstLineIdx int
	text       string
}

// Render formats the diff envelope as an inline diff with AST highlights and move markers.
func Render(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope, opts RenderOptions, th *theme.Theme) string {
	if env == nil || len(env.LineAlignment) == 0 {
		return ""
	}

	if th == nil {
		th = theme.CatppuccinMochaTheme()
	}

	contextLines := opts.ContextLines
	if contextLines < 0 {
		contextLines = 3
	}

	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	// Accurately detect trailing newlines in both files
	srcEndsWithNL := len(srcBytes) > 0 && srcBytes[len(srcBytes)-1] == '\n'
	dstEndsWithNL := len(dstBytes) > 0 && dstBytes[len(dstBytes)-1] == '\n'

	// Determine the last meaningful source and destination line index
	lastSrcLineIdx := len(srcLines) - 1
	if srcEndsWithNL && lastSrcLineIdx >= 0 && srcLines[lastSrcLineIdx] == "" {
		lastSrcLineIdx--
	}
	lastDstLineIdx := len(dstLines) - 1
	if dstEndsWithNL && lastDstLineIdx >= 0 && dstLines[lastDstLineIdx] == "" {
		lastDstLineIdx--
	}

	srcEOFLine := -1
	if srcEndsWithNL && len(srcLines) > 0 && srcLines[len(srcLines)-1] == "" {
		srcEOFLine = len(srcLines) - 1
	}
	dstEOFLine := -1
	if dstEndsWithNL && len(dstLines) > 0 && dstLines[len(dstLines)-1] == "" {
		dstEOFLine = len(dstLines) - 1
	}

	// Drop the synthetic empty line at EOF so it doesn't show up as an edit.
	filteredPairs := make([]serialize.LineAlignmentPair, 0, len(env.LineAlignment))
	for _, pair := range env.LineAlignment {
		if srcEOFLine != -1 && pair.LeftLine == srcEOFLine && (pair.RightLine == dstEOFLine || pair.RightLine == -1) {
			continue
		}
		if dstEOFLine != -1 && pair.RightLine == dstEOFLine && (pair.LeftLine == srcEOFLine || pair.LeftLine == -1) {
			continue
		}
		filteredPairs = append(filteredPairs, pair)
	}

	leftSpansByLine := make(map[int][]serialize.HighlightSpan)
	for _, sp := range env.LeftHighlights {
		leftSpansByLine[sp.Line] = append(leftSpansByLine[sp.Line], sp)
	}

	rightSpansByLine := make(map[int][]serialize.HighlightSpan)
	for _, sp := range env.RightHighlights {
		rightSpansByLine[sp.Line] = append(rightSpansByLine[sp.Line], sp)
	}

	// If there are no actions or highlights anywhere in the envelope, and EOF newline status is identical, there are no diffs
	if len(env.Actions) == 0 && len(leftSpansByLine) == 0 && len(rightSpansByLine) == 0 && srcEndsWithNL == dstEndsWithNL && bytes.Equal(srcBytes, dstBytes) {
		return ""
	}

	isPairChanged := make([]bool, len(filteredPairs))
	hasAnyChange := false

	for i, pair := range filteredPairs {
		if pair.LeftLine == -1 || pair.RightLine == -1 {
			isPairChanged[i] = true
			hasAnyChange = true
		} else {
			sText := ""
			if pair.LeftLine < len(srcLines) {
				sText = srcLines[pair.LeftLine]
			}
			dText := ""
			if pair.RightLine < len(dstLines) {
				dText = dstLines[pair.RightLine]
			}
			isEOFLine := (pair.LeftLine == lastSrcLineIdx && pair.RightLine == lastDstLineIdx)
			if sText != dText || (isEOFLine && srcEndsWithNL != dstEndsWithNL) {
				isPairChanged[i] = true
				hasAnyChange = true
			}
		}
	}

	if !hasAnyChange {
		return ""
	}

	changeIntervals := buildChangeIntervals(isPairChanged)
	if len(changeIntervals) == 0 {
		return ""
	}

	hunks := mergeHunks(changeIntervals, len(filteredPairs), contextLines)

	srcOffsets := serialize.BuildLineIndex(srcBytes)
	dstOffsets := serialize.BuildLineIndex(dstBytes)

	srcBlockMoves, dstBlockMoves, srcInBlockMove, dstInBlockMove := buildBlockMoveAnnotations(env.Actions, srcOffsets, dstOffsets, srcLines, dstLines, leftSpansByLine, rightSpansByLine)

	renderer := lipgloss.NewRenderer(io.Discard)
	if opts.Color {
		renderer.SetColorProfile(termenv.TrueColor)
	} else {
		renderer.SetColorProfile(termenv.Ascii)
	}

	var (
		headerStyle    lipgloss.Style
		hunkHdrStyle   lipgloss.Style
		deleteStyle    lipgloss.Style
		insertStyle    lipgloss.Style
		moveAnnotStyle lipgloss.Style
	)

	if opts.Color {
		headerStyle = renderer.NewStyle().Bold(true).Foreground(th.UI.Text)
		hunkHdrStyle = renderer.NewStyle().Foreground(th.UI.Lavender)
		deleteStyle = renderer.NewStyle().Foreground(th.Actions.DeleteFg)
		insertStyle = renderer.NewStyle().Foreground(th.Actions.InsertFg)
		moveAnnotStyle = renderer.NewStyle().Foreground(th.UI.Overlay0).Italic(true)
	}

	maxLine := max(len(srcLines), len(dstLines))
	numWidth := max(3, len(strconv.Itoa(maxLine)))

	var out strings.Builder

	srcHeaderPath := formatFilePath(srcFile, "a/")
	dstHeaderPath := formatFilePath(dstFile, "b/")

	if opts.Color {
		out.WriteString(headerStyle.Render("--- " + srcHeaderPath))
		out.WriteByte('\n')
		out.WriteString(headerStyle.Render("+++ " + dstHeaderPath))
		out.WriteByte('\n')
	} else {
		out.WriteString("--- " + srcHeaderPath + "\n")
		out.WriteString("+++ " + dstHeaderPath + "\n")
	}

	for _, h := range hunks {
		var lines []hunkLine
		k := h.start
		for k <= h.end {
			if !isPairChanged[k] {
				pair := filteredPairs[k]
				text := ""
				if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
					text = srcLines[pair.LeftLine]
				} else if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
					text = dstLines[pair.RightLine]
				}
				lines = append(lines, hunkLine{
					kind:       kindContext,
					srcLineIdx: pair.LeftLine,
					dstLineIdx: pair.RightLine,
					text:       text,
				})
				k++
			} else {
				cEnd := k
				for cEnd <= h.end && isPairChanged[cEnd] {
					cEnd++
				}

				for p := k; p < cEnd; p++ {
					pair := filteredPairs[p]
					if pair.LeftLine != -1 && pair.LeftLine < len(srcLines) {
						lines = append(lines, hunkLine{
							kind:       kindDelete,
							srcLineIdx: pair.LeftLine,
							dstLineIdx: -1,
							text:       srcLines[pair.LeftLine],
						})
					}
				}

				for p := k; p < cEnd; p++ {
					pair := filteredPairs[p]
					if pair.RightLine != -1 && pair.RightLine < len(dstLines) {
						lines = append(lines, hunkLine{
							kind:       kindInsert,
							srcLineIdx: -1,
							dstLineIdx: pair.RightLine,
							text:       dstLines[pair.RightLine],
						})
					}
				}

				k = cEnd
			}
		}

		srcCount := 0
		dstCount := 0
		srcStart := -1
		dstStart := -1

		for _, l := range lines {
			if l.kind == kindContext || l.kind == kindDelete {
				if srcStart == -1 && l.srcLineIdx != -1 {
					srcStart = l.srcLineIdx + 1
				}
				srcCount++
			}
			if l.kind == kindContext || l.kind == kindInsert {
				if dstStart == -1 && l.dstLineIdx != -1 {
					dstStart = l.dstLineIdx + 1
				}
				dstCount++
			}
		}

		if srcStart == -1 {
			for p := h.start - 1; p >= 0; p-- {
				if filteredPairs[p].LeftLine != -1 {
					srcStart = filteredPairs[p].LeftLine + 1
					break
				}
			}
			if srcStart == -1 {
				srcStart = 0
			}
		}
		if dstStart == -1 {
			for p := h.start - 1; p >= 0; p-- {
				if filteredPairs[p].RightLine != -1 {
					dstStart = filteredPairs[p].RightLine + 1
					break
				}
			}
			if dstStart == -1 {
				dstStart = 0
			}
		}

		hunkHeader := fmt.Sprintf("@@ -%s +%s @@", formatRange(srcStart, srcCount), formatRange(dstStart, dstCount))
		if opts.Color {
			out.WriteString(hunkHdrStyle.Render(hunkHeader))
		} else {
			out.WriteString(hunkHeader)
		}
		out.WriteByte('\n')

		for _, l := range lines {
			var gutter string
			if opts.LineNumbers {
				sNum := 0
				if l.srcLineIdx >= 0 {
					sNum = l.srcLineIdx + 1
				}
				dNum := 0
				if l.dstLineIdx >= 0 {
					dNum = l.dstLineIdx + 1
				}
				gutter = formatLineGutter(sNum, dNum, numWidth, opts.Color, l.kind, th, renderer)
			}

			switch l.kind {
			case kindContext:
				if opts.LineNumbers {
					out.WriteString(gutter + " " + l.text + "\n")
				} else {
					out.WriteString(" " + l.text + "\n")
				}
				if !srcEndsWithNL && !dstEndsWithNL && l.srcLineIdx == lastSrcLineIdx && l.dstLineIdx == lastDstLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}

			case kindDelete:
				var annot string
				if !opts.DisableAnnotations {
					annot = srcBlockMoves[l.srcLineIdx]
					if annot == "" && !srcInBlockMove[l.srcLineIdx] {
						annot = buildTokenMoveAnnotation(l.text, leftSpansByLine[l.srcLineIdx], "left", srcOffsets, dstOffsets)
					}
				}
				lineRendered := renderLineWithSpans(l.text, leftSpansByLine[l.srcLineIdx], true, "left", opts.Color, th, renderer)
				if opts.Color {
					if annot != "" {
						annot = moveAnnotStyle.Render(annot)
					}
					prefix := deleteStyle.Render("-")
					if opts.LineNumbers {
						out.WriteString(gutter + prefix + lineRendered + annot + "\n")
					} else {
						out.WriteString(prefix + lineRendered + annot + "\n")
					}
				} else {
					if opts.LineNumbers {
						out.WriteString(gutter + "-" + lineRendered + annot + "\n")
					} else {
						out.WriteString("-" + lineRendered + annot + "\n")
					}
				}
				if !srcEndsWithNL && l.srcLineIdx == lastSrcLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}

			case kindInsert:
				var annot string
				if !opts.DisableAnnotations {
					annot = dstBlockMoves[l.dstLineIdx]
					if annot == "" && !dstInBlockMove[l.dstLineIdx] {
						annot = buildTokenMoveAnnotation(l.text, rightSpansByLine[l.dstLineIdx], "right", srcOffsets, dstOffsets)
					}
				}
				lineRendered := renderLineWithSpans(l.text, rightSpansByLine[l.dstLineIdx], false, "right", opts.Color, th, renderer)
				if opts.Color {
					if annot != "" {
						annot = moveAnnotStyle.Render(annot)
					}
					prefix := insertStyle.Render("+")
					if opts.LineNumbers {
						out.WriteString(gutter + prefix + lineRendered + annot + "\n")
					} else {
						out.WriteString(prefix + lineRendered + annot + "\n")
					}
				} else {
					if opts.LineNumbers {
						out.WriteString(gutter + "+" + lineRendered + annot + "\n")
					} else {
						out.WriteString("+" + lineRendered + annot + "\n")
					}
				}
				if !dstEndsWithNL && l.dstLineIdx == lastDstLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}
			}
		}
	}

	return out.String()
}

func formatLineGutter(srcLine, dstLine int, numWidth int, color bool, kind lineKind, th *theme.Theme, renderer *lipgloss.Renderer) string {
	srcStr := ""
	if srcLine > 0 {
		srcStr = strconv.Itoa(srcLine)
	}
	dstStr := ""
	if dstLine > 0 {
		dstStr = strconv.Itoa(dstLine)
	}

	srcPad := fmt.Sprintf("%*s", numWidth, srcStr)
	dstPad := fmt.Sprintf("%*s", numWidth, dstStr)

	if color {
		numStyle := renderer.NewStyle().Foreground(th.UI.Overlay0)
		sepStyle := renderer.NewStyle().Foreground(th.UI.Surface1)
		switch kind {
		case kindDelete:
			return renderer.NewStyle().Foreground(th.Actions.DeleteFg).Render(srcPad) + " " + numStyle.Render(dstPad) + " " + sepStyle.Render("│") + " "
		case kindInsert:
			return numStyle.Render(srcPad) + " " + renderer.NewStyle().Foreground(th.Actions.InsertFg).Render(dstPad) + " " + sepStyle.Render("│") + " "
		default:
			return numStyle.Render(srcPad) + " " + numStyle.Render(dstPad) + " " + sepStyle.Render("│") + " "
		}
	}
	return srcPad + " " + dstPad + " │ "
}

func buildChangeIntervals(isPairChanged []bool) []interval {
	var changeIntervals []interval
	inChange := false
	startIdx := 0

	for i, changed := range isPairChanged {
		if changed {
			if !inChange {
				inChange = true
				startIdx = i
			}
		} else {
			if inChange {
				changeIntervals = append(changeIntervals, interval{start: startIdx, end: i - 1})
				inChange = false
			}
		}
	}
	if inChange {
		changeIntervals = append(changeIntervals, interval{start: startIdx, end: len(isPairChanged) - 1})
	}
	return changeIntervals
}

func mergeHunks(changeIntervals []interval, totalPairs, contextLines int) []interval {
	var hunks []interval
	for _, ci := range changeIntervals {
		hStart := max(0, ci.start-contextLines)
		hEnd := min(totalPairs-1, ci.end+contextLines)

		if len(hunks) > 0 && hStart <= hunks[len(hunks)-1].end+1 {
			hunks[len(hunks)-1].end = hEnd
		} else {
			hunks = append(hunks, interval{start: hStart, end: hEnd})
		}
	}
	return hunks
}

func resolveColHighlights(lineText string, spans []serialize.HighlightSpan, pane string) ([]int, []int) {
	lineBytes := []byte(lineText)
	n := len(lineBytes)
	if n == 0 {
		return nil, nil
	}

	colHighlight := make([]int, n)
	colSpanID := make([]int, n)
	colSpanLen := make([]int, n)
	colCandidateLen := make([]int, n)
	colHasMove := make([]bool, n)

	for i := range colHighlight {
		colHighlight[i] = -1
		colSpanID[i] = -1
		colSpanLen[i] = 1<<31 - 1
		colCandidateLen[i] = 1<<31 - 1
	}

	for sIdx, s := range spans {
		sc := max(0, min(s.StartCol, n))
		ec := max(0, min(s.EndCol, n))
		if sc >= 0 && ec > sc {
			candidateLen := ec - sc
			astLen := getSpanASTLength(s, pane)
			k := parseActionKind(s.Action)

			for col := sc; col < ec; col++ {
				if k == theme.ActionMove || k == theme.ActionMoveUpdate {
					colHasMove[col] = true
				}

				curLen := colSpanLen[col]
				if colHighlight[col] == -1 || astLen < curLen ||
					(astLen == curLen && candidateLen < colCandidateLen[col]) ||
					(astLen == curLen && candidateLen == colCandidateLen[col] && actionPriority(k) > actionPriority(theme.ActionKind(colHighlight[col]))) {
					colHighlight[col] = int(k)
					colSpanID[col] = sIdx
					colSpanLen[col] = astLen
					colCandidateLen[col] = candidateLen
				}
			}
		}
	}

	for col := range colHighlight {
		if colHighlight[col] == int(theme.ActionUpdate) && colHasMove[col] {
			colHighlight[col] = int(theme.ActionMoveUpdate)
		}
	}

	return colHighlight, colSpanID
}

func lineMoveCoverage(lineText string, colHighlight []int) float64 {
	trimmed := strings.TrimSpace(lineText)
	if len(trimmed) == 0 || len(colHighlight) == 0 {
		return 0
	}
	n := len(lineText)
	movedCount := 0
	nonSpaceCount := 0
	for i := 0; i < n; i++ {
		if lineText[i] != ' ' && lineText[i] != '\t' {
			nonSpaceCount++
			if colHighlight[i] == int(theme.ActionMove) ||
				colHighlight[i] == int(theme.ActionUpdate) ||
				colHighlight[i] == int(theme.ActionMoveUpdate) {
				movedCount++
			}
		}
	}
	if nonSpaceCount == 0 {
		return 0
	}
	return float64(movedCount) / float64(nonSpaceCount)
}

func buildBlockMoveAnnotations(actions []serialize.Action, srcOffsets, dstOffsets []int, srcLines, dstLines []string, leftSpans, rightSpans map[int][]serialize.HighlightSpan) (map[int]string, map[int]string, map[int]bool, map[int]bool) {
	srcMoveAnnotations := make(map[int]string)
	dstMoveAnnotations := make(map[int]string)
	srcInBlockMove := make(map[int]bool)
	dstInBlockMove := make(map[int]bool)

	if len(actions) == 0 {
		return srcMoveAnnotations, dstMoveAnnotations, srcInBlockMove, dstInBlockMove
	}

	for _, a := range actions {
		if a.Action == "move" && a.Node != nil {
			var dStart, dEnd int
			if a.DestStartByte != nil && a.DestEndByte != nil {
				dStart, _ = serialize.ByteToLineCol(dstOffsets, *a.DestStartByte)
				dEnd, _ = serialize.ByteToLineCol(dstOffsets, *a.DestEndByte)
			} else if a.DestNode != nil {
				dStart, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.StartByte)
				dEnd, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.EndByte)
			} else {
				continue
			}

			sStart, _ := serialize.ByteToLineCol(srcOffsets, a.Node.StartByte)
			sEnd, _ := serialize.ByteToLineCol(srcOffsets, a.Node.EndByte)

			// Make sure the start of the block has enough moved tokens before tagging it.
			var sCov float64
			if sStart < len(srcLines) {
				if len(leftSpans[sStart]) > 0 {
					cols, _ := resolveColHighlights(srcLines[sStart], leftSpans[sStart], "left")
					sCov = lineMoveCoverage(srcLines[sStart], cols)
				} else {
					trimmedLen := len(strings.TrimSpace(srcLines[sStart]))
					if trimmedLen > 0 {
						sCov = 1.0
					}
				}
			}

			var dCov float64
			if dStart < len(dstLines) {
				if len(rightSpans[dStart]) > 0 {
					cols, _ := resolveColHighlights(dstLines[dStart], rightSpans[dStart], "right")
					dCov = lineMoveCoverage(dstLines[dStart], cols)
				} else {
					trimmedLen := len(strings.TrimSpace(dstLines[dStart]))
					if trimmedLen > 0 {
						dCov = 1.0
					}
				}
			}

			// Only tag the first line of the moved block so we don't spam every line.
			if sCov >= 0.6 {
				if _, exists := srcMoveAnnotations[sStart]; !exists {
					srcMoveAnnotations[sStart] = fmt.Sprintf("  ← moved to line %d", dStart+1)
				}
				for sLine := sStart; sLine <= sEnd && sLine < len(srcLines); sLine++ {
					srcInBlockMove[sLine] = true
				}
			}

			if dCov >= 0.6 {
				if _, exists := dstMoveAnnotations[dStart]; !exists {
					dstMoveAnnotations[dStart] = fmt.Sprintf("  ← moved from line %d", sStart+1)
				}
				for dLine := dStart; dLine <= dEnd && dLine < len(dstLines); dLine++ {
					dstInBlockMove[dLine] = true
				}
			}
		}
	}
	return srcMoveAnnotations, dstMoveAnnotations, srcInBlockMove, dstInBlockMove
}

func formatRange(start, count int) string {
	if count == 1 {
		return strconv.Itoa(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// formatFilePath builds standard diff headers like "a/foo.go" or "/dev/null".
func formatFilePath(path, prefix string) string {
	if path == os.DevNull || path == "/dev/null" || path == "" {
		return "/dev/null"
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if strings.HasPrefix(clean, "a/") || strings.HasPrefix(clean, "b/") {
		return clean
	}
	return prefix + clean
}

func getSpanASTLength(s serialize.HighlightSpan, pane string) int {
	if s.ActionRef != nil {
		if pane == "left" {
			if s.ActionRef.Node != nil {
				return int(s.ActionRef.Node.EndByte - s.ActionRef.Node.StartByte)
			}
		} else {
			if s.ActionRef.DestStartByte != nil && s.ActionRef.DestEndByte != nil {
				return int(*s.ActionRef.DestEndByte - *s.ActionRef.DestStartByte)
			}
			if s.ActionRef.DestNode != nil {
				return int(s.ActionRef.DestNode.EndByte - s.ActionRef.DestNode.StartByte)
			}
			if s.ActionRef.Node != nil {
				return int(s.ActionRef.Node.EndByte - s.ActionRef.Node.StartByte)
			}
		}
	}
	return s.EndCol - s.StartCol
}

func actionPriority(k theme.ActionKind) int {
	switch k {
	case theme.ActionMoveUpdate:
		return 5
	case theme.ActionUpdate:
		return 4
	case theme.ActionInsert:
		return 3
	case theme.ActionMove:
		return 2
	case theme.ActionDelete:
		return 1
	default:
		return 0
	}
}

func parseActionKind(act string) theme.ActionKind {
	switch act {
	case "delete":
		return theme.ActionDelete
	case "insert":
		return theme.ActionInsert
	case "update":
		return theme.ActionUpdate
	case "move":
		return theme.ActionMove
	case "move_update":
		return theme.ActionMoveUpdate
	default:
		return theme.ActionUpdate
	}
}

func getActionStyle(k theme.ActionKind, isDeleteLine bool, th *theme.Theme, renderer *lipgloss.Renderer) lipgloss.Style {
	switch k {
	case theme.ActionUpdate:
		return renderer.NewStyle().Foreground(th.Actions.UpdateFg).Bold(true)
	case theme.ActionMove:
		return renderer.NewStyle().Foreground(th.Actions.MoveFg).Bold(true)
	case theme.ActionMoveUpdate:
		return renderer.NewStyle().Foreground(th.Actions.MoveUpdateFg).Bold(true).Underline(true)
	case theme.ActionInsert:
		return renderer.NewStyle().Foreground(th.Actions.InsertFg)
	case theme.ActionDelete:
		return renderer.NewStyle().Foreground(th.Actions.DeleteFg)
	default:
		if isDeleteLine {
			return renderer.NewStyle().Foreground(th.Actions.DeleteFg)
		}
		return renderer.NewStyle().Foreground(th.Actions.InsertFg)
	}
}

func buildTokenMoveAnnotation(lineText string, spans []serialize.HighlightSpan, pane string, srcOffsets, dstOffsets []int) string {
	if len(lineText) == 0 || len(spans) == 0 {
		return ""
	}

	colHighlight, colSpanID := resolveColHighlights(lineText, spans, pane)
	if len(colHighlight) == 0 {
		return ""
	}

	var targetOrder []int
	tokensByTarget := make(map[int][]string)
	seenTokenTarget := make(map[string]bool)

	byteOffset := 0
	segStart := 0
	segKind := -1
	segSpan := -1
	first := true

	for byteOffset < len(lineText) {
		_, runeLen := utf8.DecodeRuneInString(lineText[byteOffset:])
		hKind := -1
		sID := -1
		if byteOffset < len(colHighlight) {
			hKind = colHighlight[byteOffset]
			sID = colSpanID[byteOffset]
		}

		if first {
			segKind = hKind
			segSpan = sID
			segStart = byteOffset
			first = false
		} else if hKind != segKind || sID != segSpan {
			processTokenSeg(lineText[segStart:byteOffset], segKind, segStart, byteOffset, spans, pane, srcOffsets, dstOffsets, &targetOrder, tokensByTarget, seenTokenTarget)
			segStart = byteOffset
			segKind = hKind
			segSpan = sID
		}

		byteOffset += runeLen
	}

	if !first && segStart < len(lineText) {
		processTokenSeg(lineText[segStart:], segKind, segStart, len(lineText), spans, pane, srcOffsets, dstOffsets, &targetOrder, tokensByTarget, seenTokenTarget)
	}

	if len(targetOrder) == 0 {
		return ""
	}

	var parts []string
	for _, tLine := range targetOrder {
		tokens := tokensByTarget[tLine]
		var quotedTokens []string
		for _, tok := range tokens {
			quotedTokens = append(quotedTokens, fmt.Sprintf("'%s'", tok))
		}
		toksStr := strings.Join(quotedTokens, ", ")

		if pane == "left" {
			parts = append(parts, fmt.Sprintf("%s ➔ L%d", toksStr, tLine))
		} else {
			parts = append(parts, fmt.Sprintf("%s ⤹ L%d", toksStr, tLine))
		}
	}

	return "  ← " + strings.Join(parts, ", ")
}

func processTokenSeg(segText string, segKind, segStart, segEnd int, spans []serialize.HighlightSpan, pane string, srcOffsets, dstOffsets []int, targetOrder *[]int, tokensByTarget map[int][]string, seenTokenTarget map[string]bool) {
	if segKind != int(theme.ActionMove) && segKind != int(theme.ActionMoveUpdate) {
		return
	}

	var bestSpan *serialize.HighlightSpan
	bestLen := 1<<31 - 1
	for sIdx := range spans {
		s := &spans[sIdx]
		k := parseActionKind(s.Action)
		if (k == theme.ActionMove || k == theme.ActionMoveUpdate) && s.ActionRef != nil {
			if s.StartCol <= segStart && s.EndCol >= segEnd {
				astLen := getSpanASTLength(*s, pane)
				if astLen < bestLen {
					bestLen = astLen
					bestSpan = s
				}
			}
		}
	}

	if bestSpan != nil && bestSpan.ActionRef != nil {
		targetLine := 0
		if pane == "left" {
			if bestSpan.ActionRef.DestStartByte != nil {
				dStart, _ := serialize.ByteToLineCol(dstOffsets, *bestSpan.ActionRef.DestStartByte)
				targetLine = dStart + 1
			} else if bestSpan.ActionRef.DestNode != nil {
				dStart, _ := serialize.ByteToLineCol(dstOffsets, bestSpan.ActionRef.DestNode.StartByte)
				targetLine = dStart + 1
			}
		} else {
			if bestSpan.ActionRef.Node != nil {
				sStart, _ := serialize.ByteToLineCol(srcOffsets, bestSpan.ActionRef.Node.StartByte)
				targetLine = sStart + 1
			}
		}

		tokClean := cleanTokenExpr(segText)
		if tokClean != "" && targetLine > 0 {
			key := fmt.Sprintf("%s:%d", tokClean, targetLine)
			if !seenTokenTarget[key] {
				seenTokenTarget[key] = true
				if len(tokensByTarget[targetLine]) == 0 {
					*targetOrder = append(*targetOrder, targetLine)
				}
				tokensByTarget[targetLine] = append(tokensByTarget[targetLine], tokClean)
			}
		}
	}
}

func cleanTokenExpr(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimRight(s, " \t;:,{}+-*/=&|")
	s = strings.TrimLeft(s, " \t;:,{}+-*/=&|")

	for len(s) > 0 && (strings.HasSuffix(s, "(") || strings.HasSuffix(s, "[")) {
		s = s[:len(s)-1]
		s = strings.TrimRight(s, " \t")
	}

	for len(s) > 0 && (strings.HasPrefix(s, ")") || strings.HasPrefix(s, "]")) {
		s = s[1:]
		s = strings.TrimLeft(s, " \t")
	}

	if len(strings.Trim(s, " \t.,;()[]{}!?:+-/*=&|<>\"'`")) == 0 {
		return ""
	}

	runes := []rune(s)
	if len(runes) > 35 {
		s = string(runes[:32]) + "..."
	}
	return s
}

func renderLineWithSpans(lineText string, spans []serialize.HighlightSpan, isDeleteLine bool, pane string, color bool, th *theme.Theme, renderer *lipgloss.Renderer) string {
	if len(spans) == 0 {
		if !color {
			return lineText
		}
		baseStyle := renderer.NewStyle().Foreground(th.Actions.InsertFg)
		if isDeleteLine {
			baseStyle = renderer.NewStyle().Foreground(th.Actions.DeleteFg)
		}
		leadingLen := len(lineText) - len(strings.TrimLeft(lineText, " \t"))
		if leadingLen > 0 {
			return lineText[:leadingLen] + baseStyle.Render(lineText[leadingLen:])
		}
		return baseStyle.Render(lineText)
	}

	if len(lineText) == 0 {
		return ""
	}

	colHighlight, colSpanID := resolveColHighlights(lineText, spans, pane)

	var b strings.Builder
	byteOffset := 0
	segStart := 0
	segKind := -1
	segSpan := -1
	first := true

	for byteOffset < len(lineText) {
		_, runeLen := utf8.DecodeRuneInString(lineText[byteOffset:])
		hKind := -1
		sID := -1
		if byteOffset < len(colHighlight) {
			hKind = colHighlight[byteOffset]
			sID = colSpanID[byteOffset]
		}

		if first {
			segKind = hKind
			segSpan = sID
			segStart = byteOffset
			first = false
		} else if hKind != segKind || sID != segSpan {
			segText := lineText[segStart:byteOffset]
			writeStyledSegment(&b, segText, segKind, isDeleteLine, color, th, renderer)
			segStart = byteOffset
			segKind = hKind
			segSpan = sID
		}

		byteOffset += runeLen
	}

	if !first && segStart < len(lineText) {
		segText := lineText[segStart:]
		writeStyledSegment(&b, segText, segKind, isDeleteLine, color, th, renderer)
	}

	return b.String()
}

func writeStyledSegment(b *strings.Builder, segText string, segKind int, isDeleteLine, color bool, th *theme.Theme, renderer *lipgloss.Renderer) {
	if !color {
		b.WriteString(segText)
		return
	}
	leadingWS := segText[:len(segText)-len(strings.TrimLeft(segText, " \t"))]
	content := segText[len(leadingWS):]
	b.WriteString(leadingWS)
	if content != "" {
		style := getActionStyle(theme.ActionKind(segKind), isDeleteLine, th, renderer)
		b.WriteString(style.Render(content))
	}
}
