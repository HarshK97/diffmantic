// Package inline renders AST-aware inline diffs.
package inline

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

type hunkMoveMetadata struct {
	srcLine1Badges map[int]string // Line index -> " ➔ L..."
	dstLine1Badges map[int]string // Line index -> " ⤹ L..."
	hunkHeaders    map[int]string // Hunk index -> " <sig> (moved {from/to} L...[, modified])"
}

// Render formats the diff envelope as an inline diff with AST highlights and move markers.
func Render(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope, opts RenderOptions) string {
	if env == nil {
		return ""
	}

	if env.IsBinary {
		if opts.Color {
			return fmt.Sprintf("%sBinary files %s and %s differ%s\n", color.UpdateFg, srcFile, dstFile, color.Reset)
		}
		return fmt.Sprintf("Binary files %s and %s differ\n", srcFile, dstFile)
	}

	if len(env.LineAlignment) == 0 {
		return ""
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

	rightToLeft := make(map[int]int, len(filteredPairs))
	leftToRight := make(map[int]int, len(filteredPairs))
	for _, pair := range filteredPairs {
		if pair.LeftLine >= 0 && pair.RightLine >= 0 {
			rightToLeft[pair.RightLine] = pair.LeftLine
			leftToRight[pair.LeftLine] = pair.RightLine
		}
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
	if langName, err := treesitter.DetectLanguageName(srcFile); err == nil {
		r = rules.Get(langName)
	} else if langName, err := treesitter.DetectLanguageName(dstFile); err == nil {
		r = rules.Get(langName)
	}

	meta := buildHunkMoveMetadata(env.Actions, hunks, filteredPairs, srcOffsets, dstOffsets, srcLines, dstLines, r)

	scratch := newInlineScratch(256)

	maxLine := max(len(srcLines), len(dstLines))
	numWidth := max(3, len(strconv.Itoa(maxLine)))

	wrapActive := opts.Wrap && opts.TerminalWidth > 0
	targetWidth := 0
	if wrapActive {
		gutterWidth := 1
		if opts.LineNumbers {
			gutterWidth = 2*numWidth + 3
			if !opts.Color {
				gutterWidth++
			}
		}
		targetWidth = opts.TerminalWidth - gutterWidth
		if targetWidth < 20 {
			targetWidth = 20
		}
	}
	tabWidth := opts.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}

	var out strings.Builder
	out.Grow(len(srcBytes) + len(dstBytes))

	srcHeaderPath := formatFilePath(srcFile, "a/")
	dstHeaderPath := formatFilePath(dstFile, "b/")

	if opts.Color {
		out.WriteString(color.Bold + color.TextFg + "--- " + srcHeaderPath + color.Reset + "\n")
		out.WriteString(color.Bold + color.TextFg + "+++ " + dstHeaderPath + color.Reset + "\n")
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
				bEnd := k + 1
				// Sub-block replacement grouping:
				// If k is an aligned pair (both Left >= 0 and Right >= 0), absorb any trailing gap lines.
				// If k is a gap line (Left == -1 or Right == -1), absorb any consecutive gap lines.
				for bEnd <= h.end && isPairChanged[bEnd] {
					if filteredPairs[bEnd].LeftLine >= 0 && filteredPairs[bEnd].RightLine >= 0 {
						break
					}
					bEnd++
				}

				for p := k; p < bEnd; p++ {
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

				for p := k; p < bEnd; p++ {
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

				k = bEnd
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
			out.WriteString(color.HeaderFg + hunkHeader + color.Reset + "\n")
		} else {
			out.WriteString(hunkHeader + "\n")
		}

		for _, l := range lines {
			var gutter string
			var contGutter string
			if opts.LineNumbers {
				sNum := 0
				if l.srcLineIdx >= 0 {
					sNum = l.srcLineIdx + 1
				}
				dNum := 0
				if l.dstLineIdx >= 0 {
					dNum = l.dstLineIdx + 1
				}
				gutter = formatLineGutter(sNum, dNum, numWidth, opts.Color, l.kind)
				contGutter = formatContinuationGutter(numWidth)
				if !opts.Color {
					contGutter += " "
				}
			} else {
				contGutter = " "
			}

			switch l.kind {
			case kindContext:
				if wrapActive {
					chunks := sliceInlineLine(l.text, "", nil, targetWidth, tabWidth, "left", kindContext, opts.Color, scratch, false)
					for i, chunk := range chunks {
						if i == 0 {
							if opts.LineNumbers {
								if opts.Color {
									out.WriteString(gutter + string(chunk) + "\n")
								} else {
									out.WriteString(gutter + " " + string(chunk) + "\n")
								}
							} else {
								out.WriteString(" " + string(chunk) + "\n")
							}
						} else {
							out.WriteString(contGutter + string(chunk) + "\n")
						}
					}
				} else {
					if opts.LineNumbers {
						if opts.Color {
							out.WriteString(gutter + l.text + "\n")
						} else {
							out.WriteString(gutter + " " + l.text + "\n")
						}
					} else {
						out.WriteString(" " + l.text + "\n")
					}
				}
				if !srcEndsWithNL && !dstEndsWithNL && l.srcLineIdx == lastSrcLineIdx && l.dstLineIdx == lastDstLineIdx {
					out.WriteString("\\ No newline at end of file\n")
				}

			case kindDelete:
				var badge string
				if !opts.DisableAnnotations {
					badge = meta.srcLine1Badges[l.srcLineIdx]
				}
				if wrapActive {
					chunks := sliceInlineLine(l.text, badge, leftSpansByLine[l.srcLineIdx], targetWidth, tabWidth, "left", kindDelete, opts.Color, scratch, peerDepictsEdit(leftToRight, rightSpansByLine, l.srcLineIdx))
					for i, chunk := range chunks {
						if i == 0 {
							if opts.LineNumbers {
								if opts.Color {
									out.WriteString(gutter + string(chunk) + "\n")
								} else {
									out.WriteString(gutter + "-" + string(chunk) + "\n")
								}
							} else {
								if opts.Color {
									prefix := color.DeleteFg + "-" + color.Reset
									out.WriteString(prefix + string(chunk) + "\n")
								} else {
									out.WriteString("-" + string(chunk) + "\n")
								}
							}
						} else {
							out.WriteString(contGutter + string(chunk) + "\n")
						}
					}
				} else {
					lineRendered := renderLineWithSpans(l.text, leftSpansByLine[l.srcLineIdx], true, "left", opts.Color, scratch, peerDepictsEdit(leftToRight, rightSpansByLine, l.srcLineIdx))
					if opts.Color {
						if badge != "" {
							badge = color.Italic + color.OverlayFg + badge + color.Reset
						}
						if opts.LineNumbers {
							out.WriteString(gutter + lineRendered + badge + "\n")
						} else {
							prefix := color.DeleteFg + "-" + color.Reset
							out.WriteString(prefix + lineRendered + badge + "\n")
						}
					} else {
						if opts.LineNumbers {
							out.WriteString(gutter + "-" + lineRendered + badge + "\n")
						} else {
							out.WriteString("-" + lineRendered + badge + "\n")
						}
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
				if wrapActive {
					chunks := sliceInlineLine(l.text, badge, rightSpansByLine[l.dstLineIdx], targetWidth, tabWidth, "right", kindInsert, opts.Color, scratch, peerDepictsEdit(rightToLeft, leftSpansByLine, l.dstLineIdx))
					for i, chunk := range chunks {
						if i == 0 {
							if opts.LineNumbers {
								if opts.Color {
									out.WriteString(gutter + string(chunk) + "\n")
								} else {
									out.WriteString(gutter + "+" + string(chunk) + "\n")
								}
							} else {
								if opts.Color {
									prefix := color.InsertFg + "+" + color.Reset
									out.WriteString(prefix + string(chunk) + "\n")
								} else {
									out.WriteString("+" + string(chunk) + "\n")
								}
							}
						} else {
							out.WriteString(contGutter + string(chunk) + "\n")
						}
					}
				} else {
					lineRendered := renderLineWithSpans(l.text, rightSpansByLine[l.dstLineIdx], false, "right", opts.Color, scratch, peerDepictsEdit(rightToLeft, leftSpansByLine, l.dstLineIdx))
					if opts.Color {
						if badge != "" {
							badge = color.Italic + color.OverlayFg + badge + color.Reset
						}
						if opts.LineNumbers {
							out.WriteString(gutter + lineRendered + badge + "\n")
						} else {
							prefix := color.InsertFg + "+" + color.Reset
							out.WriteString(prefix + lineRendered + badge + "\n")
						}
					} else {
						if opts.LineNumbers {
							out.WriteString(gutter + "+" + lineRendered + badge + "\n")
						} else {
							out.WriteString("+" + lineRendered + badge + "\n")
						}
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

func formatLineGutter(srcLine, dstLine int, numWidth int, colorMode bool, kind lineKind) string {
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

	if colorMode {
		switch kind {
		case kindDelete:
			return color.DeleteFg + srcPad + color.Reset + " " + color.OverlayFg + dstPad + color.Reset + "  "
		case kindInsert:
			return color.OverlayFg + srcPad + color.Reset + " " + color.InsertFg + dstPad + color.Reset + "  "
		default:
			return color.OverlayFg + srcPad + color.Reset + " " + color.OverlayFg + dstPad + color.Reset + "  "
		}
	}
	return srcPad + " " + dstPad + "  "
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

	clear(colHasMove)
	for i := range n {
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
				if k == color.ActionMove || k == color.ActionMoveUpdate {
					colHasMove[col] = true
				}

				curLen := colSpanLen[col]
				if colHighlight[col] == -1 || astLen < curLen ||
					(astLen == curLen && candidateLen < colCandidateLen[col]) ||
					(astLen == curLen && candidateLen == colCandidateLen[col] && actionPriority(k) > actionPriority(color.ActionKind(colHighlight[col]))) {
					colHighlight[col] = int(k)
					colSpanID[col] = sIdx
					colSpanLen[col] = astLen
					colCandidateLen[col] = candidateLen
				}
			}
		}
	}

	for col := range n {
		if colHighlight[col] == int(color.ActionUpdate) && colHasMove[col] {
			colHighlight[col] = int(color.ActionMoveUpdate)
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
	if len(hunks) == 0 {
		return meta
	}

	// Collect destination mutating byte offsets to check whether moved nodes were edited
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

	// Index lines into hunks so we can tell if moves cross hunk boundaries
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

	// Identify cross-hunk moves and attach headers or badges
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

		// Omit annotations for moves staying within the same hunk
		if inSrcHunk && inDstHunk && hSrc == hDst {
			continue
		}

		// Check if any mutations fall inside the destination range
		idx := sort.Search(len(dstMutOffsets), func(i int) bool {
			return dstMutOffsets[i] >= dStartByte
		})
		isModified := idx < len(dstMutOffsets) && dstMutOffsets[idx] < dEndByte

		isDecl := r != nil && r.IsDeclaration(a.Node.Type)
		isBlock := r != nil && r.IsBlock(a.Node.Type)
		isMultiLine := sEnd > sStart || dEnd > dStart
		isStatement := a.Node.Type == "statement" || strings.HasSuffix(a.Node.Type, "_statement") || (r != nil && r.IsCall(a.Node.Type))
		if !isDecl && !isBlock && !isMultiLine && !isStatement {
			continue
		}
		if isDecl {
			// Top-level declaration moves are summarized directly in the hunk header
			sig := extractDeclarationSignature(a.Node, srcLines, sStart, sEnd)
			if sig == "declaration" {
				sig = extractDeclarationSignature(a.Node, dstLines, dStart, dEnd)
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
			// Sub-block or nested moves show a directional badge on the opening line
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

func extractDeclarationSignature(node *serialize.NodeRef, lines []string, sStartLine, sEndLine int) string {
	if sStartLine >= 0 && sStartLine < len(lines) {
		maxLine := sEndLine
		if maxLine < sStartLine || maxLine >= len(lines) {
			maxLine = sStartLine
		}
		for lineIdx := sStartLine; lineIdx <= maxLine; lineIdx++ {
			line := strings.TrimSpace(lines[lineIdx])
			if line == "" {
				continue
			}
			// Skip decorators, annotations, and comment lines
			if strings.HasPrefix(line, "@") || strings.HasPrefix(line, "#[") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "///") {
				continue
			}
			line = strings.TrimSuffix(line, " {")
			line = strings.TrimSuffix(line, "{")
			line = strings.TrimSuffix(line, ";")
			line = strings.TrimSpace(line)
			if len(line) > 0 {
				runes := []rune(line)
				if len(runes) > 80 {
					return string(runes[:77]) + "..."
				}
				return line
			}
		}
	}
	if node != nil && node.Label != "" {
		return node.Label
	}
	if node != nil && node.Type != "" {
		return node.Type
	}
	return "declaration"
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

func actionPriority(k color.ActionKind) int {
	switch k {
	case color.ActionMoveUpdate:
		return 5
	case color.ActionUpdate:
		return 4
	case color.ActionMove:
		return 3
	case color.ActionInsert:
		return 2
	case color.ActionDelete:
		return 1
	default:
		return 0
	}
}

func parseActionKind(act string) color.ActionKind {
	switch act {
	case "delete":
		return color.ActionDelete
	case "insert":
		return color.ActionInsert
	case "update":
		return color.ActionUpdate
	case "move":
		return color.ActionMove
	case "move_update":
		return color.ActionMoveUpdate
	default:
		return color.ActionUpdate
	}
}

// peerDepictsEdit says whether the aligned counterpart carries the
// highlights for this change. counterpart maps a line index on this side
// to the other side's; peerSpans holds that side's spans by line. No
// counterpart, or nothing highlighted there: the line is new on its own
// and keeps the whole-line color.
func peerDepictsEdit(counterpart map[int]int, peerSpans map[int][]serialize.HighlightSpan, lineIdx int) bool {
	peer, ok := counterpart[lineIdx]
	return ok && len(peerSpans[peer]) > 0
}

func renderLineWithSpans(lineText string, spans []serialize.HighlightSpan, isDeleteLine bool, pane string, colorMode bool, scratch *inlineScratch, peerDepicts bool) string {
	if len(lineText) == 0 {
		return ""
	}
	lineText = strings.TrimSuffix(lineText, "\r")

	if len(spans) == 0 {
		if !colorMode {
			return lineText
		}
		if peerDepicts {
			// The other side shows the edit, so there's nothing to paint here.
			leadingLen := len(lineText) - len(strings.TrimLeft(lineText, " \t"))
			if leadingLen > 0 {
				return lineText[:leadingLen] + color.TextFg + lineText[leadingLen:] + color.Reset
			}
			return color.TextFg + lineText + color.Reset
		}
		baseFg := color.InsertFg
		if isDeleteLine {
			baseFg = color.DeleteFg
		}
		leadingLen := len(lineText) - len(strings.TrimLeft(lineText, " \t"))
		if leadingLen > 0 {
			return lineText[:leadingLen] + baseFg + lineText[leadingLen:] + color.Reset
		}
		return baseFg + lineText + color.Reset
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
			writeStyledSegment(&b, segText, segKind, isDeleteLine, colorMode)
			segStart = byteOffset
			segKind = hKind
			segSpan = sID
		}

		byteOffset += runeLen
	}

	if !first && segStart < len(lineText) {
		segText := lineText[segStart:]
		writeStyledSegment(&b, segText, segKind, isDeleteLine, colorMode)
	}

	return b.String()
}

func writeStyledSegment(b *strings.Builder, segText string, segKind int, isDeleteLine, colorMode bool) {
	if len(segText) == 0 {
		return
	}
	if !colorMode {
		b.WriteString(segText)
		return
	}
	leadingWS := segText[:len(segText)-len(strings.TrimLeft(segText, " \t"))]
	content := segText[len(leadingWS):]
	b.WriteString(leadingWS)
	if content == "" {
		return
	}

	if segKind == -1 {
		b.WriteString(color.TextFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
		return
	}

	switch color.ActionKind(segKind) {
	case color.ActionUpdate:
		b.WriteString(color.Bold)
		b.WriteString(color.UpdateFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	case color.ActionMove:
		b.WriteString(color.Bold)
		b.WriteString(color.MoveFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	case color.ActionMoveUpdate:
		b.WriteString(color.Bold)
		b.WriteString(color.Underline)
		b.WriteString(color.UpdateFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	case color.ActionInsert:
		b.WriteString(color.InsertFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	case color.ActionDelete:
		b.WriteString(color.DeleteFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	default:
		b.WriteString(color.TextFg)
		b.WriteString(content)
		b.WriteString(color.Reset)
	}
}
