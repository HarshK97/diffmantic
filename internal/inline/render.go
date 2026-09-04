// Package inline renders AST-aware inline diffs.
package inline

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

type inlineScratch struct {
	colHighlight    []int
	colSpanID       []int
	colSpanLen      []int
	colCandidateLen []int
	colHasMove      []bool
}

func newInlineScratch(capacity int) *inlineScratch {
	if capacity < 256 {
		capacity = 256
	}
	return &inlineScratch{
		colHighlight:    make([]int, capacity),
		colSpanID:       make([]int, capacity),
		colSpanLen:      make([]int, capacity),
		colCandidateLen: make([]int, capacity),
		colHasMove:      make([]bool, capacity),
	}
}

func (s *inlineScratch) ensureCapacity(n int) {
	if len(s.colHighlight) >= n {
		return
	}
	newCap := max(n, len(s.colHighlight)*2)
	s.colHighlight = make([]int, newCap)
	s.colSpanID = make([]int, newCap)
	s.colSpanLen = make([]int, newCap)
	s.colCandidateLen = make([]int, newCap)
	s.colHasMove = make([]bool, newCap)
}

type renderStyles struct {
	headerStyle    lipgloss.Style
	hunkHdrStyle   lipgloss.Style
	deleteStyle    lipgloss.Style
	insertStyle    lipgloss.Style
	updateStyle    lipgloss.Style
	moveStyle      lipgloss.Style
	moveUpdStyle   lipgloss.Style
	moveAnnotStyle lipgloss.Style
	numStyle       lipgloss.Style
	sepStyle       lipgloss.Style
}

func initRenderStyles(th *theme.Theme, renderer *lipgloss.Renderer) renderStyles {
	return renderStyles{
		headerStyle:    renderer.NewStyle().Bold(true).Foreground(th.UI.Text),
		hunkHdrStyle:   renderer.NewStyle().Foreground(th.UI.Lavender),
		deleteStyle:    renderer.NewStyle().Foreground(th.Actions.DeleteFg),
		insertStyle:    renderer.NewStyle().Foreground(th.Actions.InsertFg),
		updateStyle:    renderer.NewStyle().Foreground(th.Actions.UpdateFg).Bold(true),
		moveStyle:      renderer.NewStyle().Foreground(th.Actions.MoveFg).Bold(true),
		moveUpdStyle:   renderer.NewStyle().Foreground(th.Actions.MoveUpdateFg).Bold(true).Underline(true),
		moveAnnotStyle: renderer.NewStyle().Foreground(th.UI.Overlay0).Italic(true),
		numStyle:       renderer.NewStyle().Foreground(th.UI.Overlay0),
		sepStyle:       renderer.NewStyle().Foreground(th.UI.Surface1),
	}
}

type hunkMoveMetadata struct {
	srcLine1Badges map[int]string // Line index -> " ➔ L..."
	dstLine1Badges map[int]string // Line index -> " ⤹ L..."
	hunkHeaders    map[int]string // Hunk index -> " <sig> (moved {from/to} L...[, modified])"
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

	// Bidirectional synthetic EOF line remapping
	filteredPairs := make([]serialize.LineAlignmentPair, 0, len(env.LineAlignment))
	for _, pair := range env.LineAlignment {
		if srcEOFLine != -1 && pair.LeftLine == srcEOFLine && (pair.RightLine == dstEOFLine || pair.RightLine == -1) {
			continue
		}
		if dstEOFLine != -1 && pair.RightLine == dstEOFLine && (pair.LeftLine == srcEOFLine || pair.LeftLine == -1) {
			continue
		}
		if srcEOFLine != -1 && pair.LeftLine == srcEOFLine && pair.RightLine != -1 {
			pair.LeftLine = -1
		}
		if dstEOFLine != -1 && pair.RightLine == dstEOFLine && pair.LeftLine != -1 {
			pair.RightLine = -1
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

	// Resolve language rules for declaration classification
	var r *rules.Rules
	if entry := treesitter.DetectGrammarEntry(srcFile); entry != nil {
		r = rules.Get(entry.Name)
	} else if entry := treesitter.DetectGrammarEntry(dstFile); entry != nil {
		r = rules.Get(entry.Name)
	}

	meta := buildHunkMoveMetadata(env.Actions, hunks, filteredPairs, srcOffsets, dstOffsets, srcLines, dstLines, r)

	renderer := lipgloss.NewRenderer(io.Discard)
	if opts.Color {
		renderer.SetColorProfile(termenv.TrueColor)
	} else {
		renderer.SetColorProfile(termenv.Ascii)
	}

	styles := initRenderStyles(th, renderer)
	scratch := newInlineScratch(256)

	maxLine := max(len(srcLines), len(dstLines))
	numWidth := max(3, len(strconv.Itoa(maxLine)))

	var out strings.Builder
	out.Grow(len(srcBytes) + len(dstBytes))

	srcHeaderPath := formatFilePath(srcFile, "a/")
	dstHeaderPath := formatFilePath(dstFile, "b/")

	if opts.Color {
		out.WriteString(styles.headerStyle.Render("--- " + srcHeaderPath))
		out.WriteByte('\n')
		out.WriteString(styles.headerStyle.Render("+++ " + dstHeaderPath))
		out.WriteByte('\n')
	} else {
		out.WriteString("--- " + srcHeaderPath + "\n")
		out.WriteString("+++ " + dstHeaderPath + "\n")
	}

	for hunkIdx, h := range hunks {
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
				if len(srcBytes) > 0 && srcFile != os.DevNull && srcFile != "/dev/null" {
					srcStart = 1
				} else {
					srcStart = 0
				}
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
				if len(dstBytes) > 0 && dstFile != os.DevNull && dstFile != "/dev/null" {
					dstStart = 1
				} else {
					dstStart = 0
				}
			}
		}

		hunkHdrExtra := meta.hunkHeaders[hunkIdx]
		hunkHeader := fmt.Sprintf("@@ -%s +%s @@%s", formatRange(srcStart, srcCount), formatRange(dstStart, dstCount), hunkHdrExtra)
		if opts.Color {
			out.WriteString(styles.hunkHdrStyle.Render(hunkHeader))
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
				gutter = formatLineGutter(sNum, dNum, numWidth, opts.Color, l.kind, &styles)
			}

			switch l.kind {
			case kindContext:
				if opts.LineNumbers {
					if opts.Color {
						out.WriteString(gutter + l.text + "\n")
					} else {
						out.WriteString(gutter + " " + l.text + "\n")
					}
				} else {
					out.WriteString(" " + l.text + "\n")
				}
				if !srcEndsWithNL && !dstEndsWithNL && l.srcLineIdx == lastSrcLineIdx && l.dstLineIdx == lastDstLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}

			case kindDelete:
				var badge string
				if !opts.DisableAnnotations {
					badge = meta.srcLine1Badges[l.srcLineIdx]
				}
				lineRendered := renderLineWithSpans(l.text, leftSpansByLine[l.srcLineIdx], true, "left", opts.Color, &styles, scratch)
				if opts.Color {
					if badge != "" {
						badge = styles.moveAnnotStyle.Render(badge)
					}
					if opts.LineNumbers {
						out.WriteString(gutter + lineRendered + badge + "\n")
					} else {
						prefix := styles.deleteStyle.Render("-")
						out.WriteString(prefix + lineRendered + badge + "\n")
					}
				} else {
					if opts.LineNumbers {
						out.WriteString(gutter + "-" + lineRendered + badge + "\n")
					} else {
						out.WriteString("-" + lineRendered + badge + "\n")
					}
				}
				if !srcEndsWithNL && l.srcLineIdx == lastSrcLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}

			case kindInsert:
				var badge string
				if !opts.DisableAnnotations {
					badge = meta.dstLine1Badges[l.dstLineIdx]
				}
				lineRendered := renderLineWithSpans(l.text, rightSpansByLine[l.dstLineIdx], false, "right", opts.Color, &styles, scratch)
				if opts.Color {
					if badge != "" {
						badge = styles.moveAnnotStyle.Render(badge)
					}
					if opts.LineNumbers {
						out.WriteString(gutter + lineRendered + badge + "\n")
					} else {
						prefix := styles.insertStyle.Render("+")
						out.WriteString(prefix + lineRendered + badge + "\n")
					}
				} else {
					if opts.LineNumbers {
						out.WriteString(gutter + "+" + lineRendered + badge + "\n")
					} else {
						out.WriteString("+" + lineRendered + badge + "\n")
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

func formatLineGutter(srcLine, dstLine int, numWidth int, color bool, kind lineKind, styles *renderStyles) string {
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

	if color && styles != nil {
		switch kind {
		case kindDelete:
			return styles.deleteStyle.Render(srcPad) + " " + styles.numStyle.Render(dstPad) + " " + styles.sepStyle.Render("│") + " "
		case kindInsert:
			return styles.numStyle.Render(srcPad) + " " + styles.insertStyle.Render(dstPad) + " " + styles.sepStyle.Render("│") + " "
		default:
			return styles.numStyle.Render(srcPad) + " " + styles.numStyle.Render(dstPad) + " " + styles.sepStyle.Render("│") + " "
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

func resolveColHighlights(lineText string, spans []serialize.HighlightSpan, pane string, scratch *inlineScratch) ([]int, []int) {
	lineBytes := []byte(lineText)
	n := len(lineBytes)
	if n == 0 {
		return nil, nil
	}

	scratch.ensureCapacity(n)
	colHighlight := scratch.colHighlight[:n]
	colSpanID := scratch.colSpanID[:n]
	colSpanLen := scratch.colSpanLen[:n]
	colCandidateLen := scratch.colCandidateLen[:n]
	colHasMove := scratch.colHasMove[:n]

	for i := 0; i < n; i++ {
		colHighlight[i] = -1
		colSpanID[i] = -1
		colSpanLen[i] = 1<<31 - 1
		colCandidateLen[i] = 1<<31 - 1
		colHasMove[i] = false
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

	for col := 0; col < n; col++ {
		if colHighlight[col] == int(theme.ActionUpdate) && colHasMove[col] {
			colHighlight[col] = int(theme.ActionMoveUpdate)
		}
	}

	return colHighlight, colSpanID
}

func buildHunkMoveMetadata(actions []serialize.Action, hunks []interval, pairs []serialize.LineAlignmentPair, srcOffsets, dstOffsets []int, srcLines, dstLines []string, r *rules.Rules) hunkMoveMetadata {
	meta := hunkMoveMetadata{
		srcLine1Badges: make(map[int]string),
		dstLine1Badges: make(map[int]string),
		hunkHeaders:    make(map[int]string),
	}

	if len(actions) == 0 {
		return meta
	}

	// 1. Pre-index destination mutating action byte offsets in O(A log A)
	var dstMutOffsets []uint32
	for _, a := range actions {
		if a.Action == "insert" || a.Action == "update" || a.Action == "move_update" {
			if a.DestStartByte != nil {
				dstMutOffsets = append(dstMutOffsets, *a.DestStartByte)
			} else if a.DestNode != nil {
				dstMutOffsets = append(dstMutOffsets, a.DestNode.StartByte)
			}
		}
	}
	slices.Sort(dstMutOffsets)

	// 2. Map lines to hunks in O(N)
	srcLineToHunk := make(map[int]int)
	dstLineToHunk := make(map[int]int)
	for hIdx, h := range hunks {
		for p := h.start; p <= h.end && p < len(pairs); p++ {
			pair := pairs[p]
			if pair.LeftLine >= 0 {
				srcLineToHunk[pair.LeftLine] = hIdx
			}
			if pair.RightLine >= 0 {
				dstLineToHunk[pair.RightLine] = hIdx
			}
		}
	}

	// 3. Evaluate moves in O(M log A)
	for _, a := range actions {
		if a.Action != "move" || a.Node == nil {
			continue
		}
		sStart, _ := serialize.ByteToLineCol(srcOffsets, a.Node.StartByte)
		sEnd, _ := serialize.ByteToLineCol(srcOffsets, a.Node.EndByte)

		var dStart, dEnd int
		var dStartByte, dEndByte uint32
		if a.DestStartByte != nil && a.DestEndByte != nil {
			dStart, _ = serialize.ByteToLineCol(dstOffsets, *a.DestStartByte)
			dEnd, _ = serialize.ByteToLineCol(dstOffsets, *a.DestEndByte)
			dStartByte, dEndByte = *a.DestStartByte, *a.DestEndByte
		} else if a.DestNode != nil {
			dStart, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.StartByte)
			dEnd, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.EndByte)
			dStartByte, dEndByte = a.DestNode.StartByte, a.DestNode.EndByte
		} else {
			continue
		}

		hSrc, inSrcHunk := srcLineToHunk[sStart]
		hDst, inDstHunk := dstLineToHunk[dStart]

		// Tier 1: Intra-hunk moves (Hs == Hd) are 100% suppressed from line-level badges
		if inSrcHunk && inDstHunk && hSrc == hDst {
			continue
		}

		// Evaluate descendant mutations in O(log A) via binary search
		idx := sort.Search(len(dstMutOffsets), func(i int) bool {
			return dstMutOffsets[i] >= dStartByte
		})
		isModified := idx < len(dstMutOffsets) && dstMutOffsets[idx] < dEndByte

		isDecl := r != nil && r.IsDeclaration(a.Node.Type)
		if isDecl {
			// Tier 2: Single declaration cross-hunk move promoted to hunk header
			sig := extractDeclarationSignature(srcLines, sStart, sEnd, r)
			if sig == "" {
				sig = extractDeclarationSignature(dstLines, dStart, dEnd, r)
			}
			modStr := ""
			if isModified {
				modStr = ", modified"
			}
			if inSrcHunk && sig != "" {
				if _, exists := meta.hunkHeaders[hSrc]; !exists {
					meta.hunkHeaders[hSrc] = fmt.Sprintf(" %s (moved to L%d%s)", sig, dStart+1, modStr)
				}
			}
			if inDstHunk && sig != "" {
				if _, exists := meta.hunkHeaders[hDst]; !exists {
					meta.hunkHeaders[hDst] = fmt.Sprintf(" %s (moved from L%d%s)", sig, sStart+1, modStr)
				}
			}
		} else {
			// Tier 3: Sub-block moves and multi-move hunks attach micro-badges strictly on Line 1 (zero repetition)
			if inSrcHunk {
				if _, exists := meta.srcLine1Badges[sStart]; !exists {
					meta.srcLine1Badges[sStart] = fmt.Sprintf(" ➔ L%d", dStart+1)
				}
			}
			if inDstHunk {
				if _, exists := meta.dstLine1Badges[dStart]; !exists {
					meta.dstLine1Badges[dStart] = fmt.Sprintf(" ⤹ L%d", sStart+1)
				}
			}
		}
	}

	return meta
}

func extractDeclarationSignature(lines []string, startLine, endLine int, r *rules.Rules) string {
	for i := startLine; i <= endLine && i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "@") || strings.HasPrefix(trimmed, "#[") || strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "/*") {
			continue
		}
		sig := trimmed
		sig = strings.TrimSuffix(sig, " {")
		sig = strings.TrimSuffix(sig, "{")
		sig = strings.TrimSuffix(sig, ";")
		sig = strings.TrimSuffix(sig, ":")
		sig = strings.TrimSpace(sig)
		if len(sig) > 80 {
			sig = sig[:77] + "..."
		}
		return sig
	}
	if startLine < len(lines) {
		sig := strings.TrimSpace(lines[startLine])
		if len(sig) > 80 {
			sig = sig[:77] + "..."
		}
		return sig
	}
	return ""
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
	case theme.ActionMove:
		return 3
	case theme.ActionInsert:
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

func renderLineWithSpans(lineText string, spans []serialize.HighlightSpan, isDeleteLine bool, pane string, color bool, styles *renderStyles, scratch *inlineScratch) string {
	if len(spans) == 0 {
		if !color || styles == nil {
			return lineText
		}
		baseStyle := styles.insertStyle
		if isDeleteLine {
			baseStyle = styles.deleteStyle
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

	colHighlight, colSpanID := resolveColHighlights(lineText, spans, pane, scratch)

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
			writeStyledSegment(&b, segText, segKind, isDeleteLine, color, styles)
			segStart = byteOffset
			segKind = hKind
			segSpan = sID
		}

		byteOffset += runeLen
	}

	if !first && segStart < len(lineText) {
		segText := lineText[segStart:]
		writeStyledSegment(&b, segText, segKind, isDeleteLine, color, styles)
	}

	return b.String()
}

func writeStyledSegment(b *strings.Builder, segText string, segKind int, isDeleteLine, color bool, styles *renderStyles) {
	if len(segText) == 0 {
		return
	}
	if !color || styles == nil {
		b.WriteString(segText)
		return
	}
	leadingWS := segText[:len(segText)-len(strings.TrimLeft(segText, " \t"))]
	content := segText[len(leadingWS):]
	b.WriteString(leadingWS)
	if content != "" {
		switch theme.ActionKind(segKind) {
		case theme.ActionUpdate:
			b.WriteString(styles.updateStyle.Render(content))
		case theme.ActionMove:
			b.WriteString(styles.moveStyle.Render(content))
		case theme.ActionMoveUpdate:
			b.WriteString(styles.moveUpdStyle.Render(content))
		case theme.ActionInsert:
			b.WriteString(styles.insertStyle.Render(content))
		case theme.ActionDelete:
			b.WriteString(styles.deleteStyle.Render(content))
		default:
			if isDeleteLine {
				b.WriteString(styles.deleteStyle.Render(content))
			} else {
				b.WriteString(styles.insertStyle.Render(content))
			}
		}
	}
}
