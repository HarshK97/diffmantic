package sidebyside

import (
	"io"
	"strings"

	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

// RenderScratch wraps a reusable line slicer and terminal tab width configuration.
type RenderScratch struct {
	TabWidth int
	slicer   renderutil.Slicer
}

// SliceLineToChunks formats a single line of text with syntax/action spans into multiple targetWidth display rows (wrapping).
func (s *RenderScratch) SliceLineToChunks(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	targetWidth int,
	lineCtx renderutil.LineContext,
	padToTargetWidth bool,
	colorMode bool,
) [][]byte {
	cfg := renderutil.SliceConfig{
		TargetWidth:      targetWidth,
		TabWidth:         s.TabWidth,
		ColorMode:        colorMode,
		PadToTargetWidth: padToTargetWidth,
		BadgeAnchor:      renderutil.BadgeAnchorRow0,
		Context:          lineCtx,
	}
	return s.slicer.SliceLineToChunks(lineText, badgeText, spans, cfg)
}

// SliceLineToColumn formats a single line of text into a single targetWidth column.
func (s *RenderScratch) SliceLineToColumn(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	targetWidth int,
	lineCtx renderutil.LineContext,
	padToTargetWidth bool,
	colorMode bool,
	w io.Writer,
) error {
	chunks := s.SliceLineToChunks(lineText, badgeText, spans, targetWidth, lineCtx, padToTargetWidth, colorMode)
	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
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
		return false
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
	h renderutil.Interval,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	codeWidth int,
	threshold int,
	forceSBS bool,
) []hunkSegment {
	if forceSBS || threshold <= 0 {
		return []hunkSegment{{start: h.Start, end: h.End, layout: HunkLayoutSideBySide}}
	}

	var rawSegments []hunkSegment
	curStart := h.Start

	for p := h.Start; p <= h.End; {
		pair := filteredPairs[p]

		if pair.LeftLine == -1 && pair.RightLine >= 0 {
			// Pure insert run
			runStart := p
			for p <= h.End && filteredPairs[p].LeftLine == -1 && filteredPairs[p].RightLine >= 0 {
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
			for p <= h.End && filteredPairs[p].LeftLine >= 0 && filteredPairs[p].RightLine == -1 {
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

	if curStart <= h.End {
		rawSegments = append(rawSegments, hunkSegment{
			start:  curStart,
			end:    h.End,
			layout: determineSegmentLayout(curStart, h.End, filteredPairs, isPairChanged, srcLines, dstLines, leftSpansByLine, rightSpansByLine, codeWidth),
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
		return []hunkSegment{{start: h.Start, end: h.End, layout: HunkLayoutSideBySide}}
	}

	return merged
}
