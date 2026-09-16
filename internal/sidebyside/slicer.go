package sidebyside

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

// RenderScratch provides reusable highlight index arrays and tab width settings.
type RenderScratch struct {
	TabWidth        int
	colHighlight    []int
	colSpanID       []int
	colSpanLen      []int
	colCandidateLen []int
	colHasMove      []bool
}

// SliceLineToChunks formats a single line of text with syntax/action spans into multiple targetWidth display rows (wrapping).
func (s *RenderScratch) SliceLineToChunks(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	targetWidth int,
	pane string,
	isDeleteLine bool,
	colorMode bool,
) [][]byte {
	if targetWidth <= 0 {
		targetWidth = 10
	}

	colHighlight, _ := s.ResolveColHighlights(lineText, spans, pane)

	var chunks [][]byte
	var curChunk []byte
	curDisplayCol := 0

	badgeWidth := 0
	if badgeText != "" {
		badgeWidth = runewidth.StringWidth(badgeText)
	}
	maxTextWidth := targetWidth - badgeWidth
	if maxTextWidth <= 0 {
		maxTextWidth = targetWidth
		badgeText = ""
		badgeWidth = 0
	}

	activeKind := -2 // -2 = none initialized, -1 = default text, >=0 = action kind
	byteOffset := 0

	finishChunk := func(isFirstChunk bool) {
		// Reset styling before wrapping.
		if colorMode && activeKind != -2 {
			curChunk = append(curChunk, color.Reset...)
		}

		// Put the move badge on the first chunk only.
		if isFirstChunk && badgeText != "" {
			if colorMode {
				curChunk = append(curChunk, color.Italic+color.OverlayFg+badgeText+color.Reset...)
			} else {
				curChunk = append(curChunk, badgeText...)
			}
			curDisplayCol += badgeWidth
		}

		for curDisplayCol < targetWidth {
			curChunk = append(curChunk, ' ')
			curDisplayCol++
		}

		chunks = append(chunks, curChunk)
		curChunk = nil
		curDisplayCol = 0
		maxTextWidth = targetWidth
		activeKind = -2
	}

	for byteOffset < len(lineText) {
		tok := nextToken(lineText, byteOffset)
		if tok.byteLen == 0 {
			break
		}

		if tok.isSpace && lineText[byteOffset] == ' ' {
			if curDisplayCol+1 > maxTextWidth {
				// Wrap and drop trailing space at the boundary.
				finishChunk(len(chunks) == 0)
				byteOffset += tok.byteLen
				continue
			}
			if colorMode {
				hKind := -1
				if byteOffset < len(colHighlight) {
					hKind = colHighlight[byteOffset]
				}
				if hKind != activeKind {
					curChunk = appendSGRTransition(curChunk, hKind)
					activeKind = hKind
				}
			}
			curChunk = append(curChunk, ' ')
			curDisplayCol++
			byteOffset += tok.byteLen
			continue
		}

		if tok.isSpace && lineText[byteOffset] == '\t' {
			tabStop := s.TabWidth
			if tabStop <= 0 {
				tabStop = 4
			}
			tabSpaces := tabStop - (curDisplayCol % tabStop)
			if tabSpaces == 0 {
				tabSpaces = tabStop
			}

			if curDisplayCol+tabSpaces > maxTextWidth {
				// Wrap to the next line if the tab overflows.
				finishChunk(len(chunks) == 0)
				tabSpaces = tabStop
			}

			if colorMode {
				hKind := -1
				if byteOffset < len(colHighlight) {
					hKind = colHighlight[byteOffset]
				}
				if hKind != activeKind {
					curChunk = appendSGRTransition(curChunk, hKind)
					activeKind = hKind
				}
			}

			for range tabSpaces {
				curChunk = append(curChunk, ' ')
			}
			curDisplayCol += tabSpaces
			byteOffset += tok.byteLen
			continue
		}

		if curDisplayCol+tok.displayWidth > maxTextWidth {
			if tok.displayWidth > targetWidth {
				// If a single token is wider than the column, break it rune-by-rune.
				currRuneOffset := byteOffset
				for currRuneOffset < byteOffset+tok.byteLen {
					r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
					rWidth := max(1, runewidth.RuneWidth(r))

					if curDisplayCol+rWidth > maxTextWidth {
						finishChunk(len(chunks) == 0)
					}

					if colorMode {
						hKind := -1
						if currRuneOffset < len(colHighlight) {
							hKind = colHighlight[currRuneOffset]
						}
						if hKind != activeKind {
							curChunk = appendSGRTransition(curChunk, hKind)
							activeKind = hKind
						}
					}

					curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
					curDisplayCol += rWidth
					currRuneOffset += rLen
				}
				byteOffset += tok.byteLen
				continue
			}

			// Wrap at the word boundary.
			finishChunk(len(chunks) == 0)
		}

		currRuneOffset := byteOffset
		for currRuneOffset < byteOffset+tok.byteLen {
			r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
			rWidth := max(1, runewidth.RuneWidth(r))

			if colorMode {
				hKind := -1
				if currRuneOffset < len(colHighlight) {
					hKind = colHighlight[currRuneOffset]
				}
				if hKind != activeKind {
					curChunk = appendSGRTransition(curChunk, hKind)
					activeKind = hKind
				}
			}

			curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
			curDisplayCol += rWidth
			currRuneOffset += rLen
		}
		byteOffset += tok.byteLen
	}

	if len(chunks) == 0 || curDisplayCol > 0 || len(curChunk) > 0 {
		finishChunk(len(chunks) == 0)
	}

	return chunks
}

// SliceLineToColumn formats a single line of text into a single targetWidth column.
func (s *RenderScratch) SliceLineToColumn(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	targetWidth int,
	pane string,
	isDeleteLine bool,
	colorMode bool,
	w io.Writer,
) error {
	chunks := s.SliceLineToChunks(lineText, badgeText, spans, targetWidth, pane, isDeleteLine, colorMode)
	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

type tokenInfo struct {
	byteLen      int
	displayWidth int
	isSpace      bool
}

func nextToken(s string, offset int) tokenInfo {
	if offset >= len(s) {
		return tokenInfo{}
	}
	r, runeLen := utf8.DecodeRuneInString(s[offset:])
	if r == ' ' {
		return tokenInfo{byteLen: runeLen, displayWidth: 1, isSpace: true}
	}
	if r == '\t' {
		return tokenInfo{byteLen: runeLen, displayWidth: 1, isSpace: true}
	}

	byteLen := 0
	displayWidth := 0
	curr := offset
	for curr < len(s) {
		ch, sz := utf8.DecodeRuneInString(s[curr:])
		if ch == ' ' || ch == '\t' {
			break
		}
		rw := runewidth.RuneWidth(ch)
		byteLen += sz
		displayWidth += rw
		curr += sz
		if (ch == ',' || ch == ';') && curr < len(s) {
			break
		}
	}
	return tokenInfo{byteLen: byteLen, displayWidth: displayWidth, isSpace: false}
}

func appendSGRTransition(b []byte, hKind int) []byte {
	b = append(b, color.Reset...)

	switch color.ActionKind(hKind) {
	case color.ActionDelete:
		return append(b, color.DeleteFg...)
	case color.ActionInsert:
		return append(b, color.InsertFg...)
	case color.ActionUpdate:
		return append(b, color.Bold+color.UpdateFg...)
	case color.ActionMove:
		return append(b, color.Bold+color.MoveFg...)
	case color.ActionMoveUpdate:
		return append(b, color.Bold+color.Underline+color.UpdateFg...)
	default:
		return append(b, color.TextFg...)
	}
}

// ResolveColHighlights maps each byte offset in lineText to an action kind and span ID.
func (s *RenderScratch) ResolveColHighlights(lineText string, spans []serialize.HighlightSpan, pane string) ([]int, []int) {
	n := len(lineText)
	if n == 0 {
		return nil, nil
	}

	if cap(s.colHighlight) < n {
		s.colHighlight = make([]int, n)
		s.colSpanID = make([]int, n)
		s.colSpanLen = make([]int, n)
		s.colCandidateLen = make([]int, n)
		s.colHasMove = make([]bool, n)
	} else {
		s.colHighlight = s.colHighlight[:n]
		s.colSpanID = s.colSpanID[:n]
		s.colSpanLen = s.colSpanLen[:n]
		s.colCandidateLen = s.colCandidateLen[:n]
		s.colHasMove = s.colHasMove[:n]
	}

	for i := range s.colHighlight {
		s.colHighlight[i] = -1
		s.colSpanID[i] = -1
		s.colSpanLen[i] = 1<<31 - 1
		s.colCandidateLen[i] = 1<<31 - 1
		s.colHasMove[i] = false
	}

	for sIdx, sp := range spans {
		sc := max(0, min(sp.StartCol, n))
		ec := max(0, min(sp.EndCol, n))
		if sc >= 0 && ec > sc {
			candidateLen := ec - sc
			astLen := getSpanASTLength(sp, pane)
			k := parseActionKind(sp.Action)

			for col := sc; col < ec; col++ {
				if k == color.ActionMove || k == color.ActionMoveUpdate {
					s.colHasMove[col] = true
				}

				curLen := s.colSpanLen[col]
				if s.colHighlight[col] == -1 || astLen < curLen ||
					(astLen == curLen && candidateLen < s.colCandidateLen[col]) ||
					(astLen == curLen && candidateLen == s.colCandidateLen[col] && actionPriority(k) > actionPriority(color.ActionKind(s.colHighlight[col]))) {
					s.colHighlight[col] = int(k)
					s.colSpanID[col] = sIdx
					s.colSpanLen[col] = astLen
					s.colCandidateLen[col] = candidateLen
				}
			}
		}
	}

	for col := range s.colHighlight {
		if s.colHighlight[col] == int(color.ActionUpdate) && s.colHasMove[col] {
			s.colHighlight[col] = int(color.ActionMoveUpdate)
		}
	}

	return s.colHighlight, s.colSpanID
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
	switch strings.ToLower(act) {
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

// HunkLayoutKind determines how a hunk or hunk segment is visually structured on screen.
type HunkLayoutKind uint8

const (
	// HunkLayoutSideBySide renders a standard dual-pane 50/50 split with a central divider.
	HunkLayoutSideBySide HunkLayoutKind = iota
	// HunkLayoutFullWidthInline renders pure single-sided inserts/deletes using the full terminal width.
	HunkLayoutFullWidthInline
)

type hunkSegment struct {
	start  int
	end    int
	layout HunkLayoutKind
}

func isLineSubstantiallySimilar(s1, s2 string) bool {
	t1 := strings.TrimSpace(s1)
	t2 := strings.TrimSpace(s2)
	if t1 == t2 {
		return true
	}
	if len(t1) == 0 || len(t2) == 0 {
		return false
	}
	pref := 0
	minLen := min(len(t1), len(t2))
	for pref < minLen && t1[pref] == t2[pref] {
		pref++
	}
	suff := 0
	for suff < (minLen-pref) && t1[len(t1)-1-suff] == t2[len(t2)-1-suff] {
		suff++
	}
	common := pref + suff
	maxLen := max(len(t1), len(t2))
	return float64(common) >= float64(maxLen)*0.4
}

func isWrappingTokenEdit(
	pair serialize.LineAlignmentPair,
	srcLines, dstLines []string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	codeWidth int,
) bool {
	if pair.LeftLine < 0 || pair.RightLine < 0 {
		return false
	}
	var srcLine, dstLine string
	if pair.LeftLine < len(srcLines) {
		srcLine = srcLines[pair.LeftLine]
	}
	if pair.RightLine < len(dstLines) {
		dstLine = dstLines[pair.RightLine]
	}

	srcWidth := runewidth.StringWidth(srcLine)
	dstWidth := runewidth.StringWidth(dstLine)
	if srcWidth <= codeWidth && dstWidth <= codeWidth {
		return false // Fits comfortably in side-by-side without wrapping
	}

	leftSpans := leftSpansByLine[pair.LeftLine]
	rightSpans := rightSpansByLine[pair.RightLine]

	if len(leftSpans) > 0 || len(rightSpans) > 0 {
		leftEditLen := 0
		for _, sp := range leftSpans {
			leftEditLen += (sp.EndCol - sp.StartCol)
		}
		rightEditLen := 0
		for _, sp := range rightSpans {
			rightEditLen += (sp.EndCol - sp.StartCol)
		}
		maxLen := max(len(srcLine), len(dstLine))
		if maxLen > 0 && float64(max(leftEditLen, rightEditLen)) <= float64(maxLen)*0.6 {
			return true
		}
	}

	return isLineSubstantiallySimilar(srcLine, dstLine)
}

func determineSegmentLayout(
	segStart, segEnd int,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	codeWidth int,
) HunkLayoutKind {
	hasWrappingTokenEdit := false

	for p := segStart; p <= segEnd; p++ {
		if p < 0 || p >= len(isPairChanged) || !isPairChanged[p] {
			continue
		}
		pair := filteredPairs[p]
		if isWrappingTokenEdit(pair, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth) {
			hasWrappingTokenEdit = true
			break
		}
	}

	if hasWrappingTokenEdit {
		return HunkLayoutFullWidthInline
	}
	return HunkLayoutSideBySide
}

// partitionHunkIntoSegments splits a hunk into segments, expanding large additions,
// deletions, or lines with wrapping edits to a full-width inline layout.
func partitionHunkIntoSegments(
	h interval,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	codeWidth int,
	threshold int,
	forceSBS bool,
) []hunkSegment {
	if forceSBS || threshold <= 0 {
		return []hunkSegment{{start: h.start, end: h.end, layout: HunkLayoutSideBySide}}
	}

	var rawSegments []hunkSegment
	curStart := h.start

	for p := h.start; p <= h.end; {
		pair := filteredPairs[p]

		if pair.LeftLine == -1 && pair.RightLine >= 0 {
			// Pure insert run
			runStart := p
			for p <= h.end && filteredPairs[p].LeftLine == -1 && filteredPairs[p].RightLine >= 0 {
				p++
			}
			runLen := p - runStart
			if runLen >= threshold {
				if runStart > curStart {
					rawSegments = append(rawSegments, hunkSegment{
						start:  curStart,
						end:    runStart - 1,
						layout: determineSegmentLayout(curStart, runStart-1, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth),
					})
				}
				rawSegments = append(rawSegments, hunkSegment{
					start:  runStart,
					end:    p - 1,
					layout: HunkLayoutFullWidthInline,
				})
				curStart = p
				continue
			}
		} else if pair.LeftLine >= 0 && pair.RightLine == -1 {
			// Pure delete run
			runStart := p
			for p <= h.end && filteredPairs[p].LeftLine >= 0 && filteredPairs[p].RightLine == -1 {
				p++
			}
			runLen := p - runStart
			if runLen >= threshold {
				if runStart > curStart {
					rawSegments = append(rawSegments, hunkSegment{
						start:  curStart,
						end:    runStart - 1,
						layout: determineSegmentLayout(curStart, runStart-1, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth),
					})
				}
				rawSegments = append(rawSegments, hunkSegment{
					start:  runStart,
					end:    p - 1,
					layout: HunkLayoutFullWidthInline,
				})
				curStart = p
				continue
			}
		} else {
			p++
		}
	}

	if curStart <= h.end {
		rawSegments = append(rawSegments, hunkSegment{
			start:  curStart,
			end:    h.end,
			layout: determineSegmentLayout(curStart, h.end, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth),
		})
	}

	// Merge adjacent segments that share the same layout kind
	var merged []hunkSegment
	for _, seg := range rawSegments {
		if seg.start > seg.end {
			continue
		}
		if len(merged) > 0 && merged[len(merged)-1].layout == seg.layout && merged[len(merged)-1].end+1 == seg.start {
			merged[len(merged)-1].end = seg.end
		} else {
			merged = append(merged, seg)
		}
	}

	if len(merged) == 0 {
		return []hunkSegment{{start: h.start, end: h.end, layout: HunkLayoutSideBySide}}
	}

	return merged
}
