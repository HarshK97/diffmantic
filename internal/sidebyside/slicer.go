package sidebyside

import (
	"io"

	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
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
	badgeColor ...int,
) [][]byte {
	bColor := 0
	if len(badgeColor) > 0 {
		bColor = badgeColor[0]
	}
	cfg := renderutil.SliceConfig{
		TargetWidth:      targetWidth,
		TabWidth:         s.TabWidth,
		ColorMode:        colorMode,
		PadToTargetWidth: padToTargetWidth,
		BadgeColor:       bColor,
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
	badgeColor ...int,
) error {
	chunks := s.SliceLineToChunks(lineText, badgeText, spans, targetWidth, lineCtx, padToTargetWidth, colorMode, badgeColor...)
	for _, chunk := range chunks {
		if _, err := w.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

// HunkLayoutKind determines how a hunk is visually structured on screen.
type HunkLayoutKind uint8

const (
	// HunkLayoutDualColumn renders a standard dual-pane 50/50 split with a central divider.
	HunkLayoutDualColumn HunkLayoutKind = iota
	// HunkLayoutSingleColumnRight renders pure insertions across the full terminal width.
	HunkLayoutSingleColumnRight
	// HunkLayoutSingleColumnLeft renders pure deletions across the full terminal width.
	HunkLayoutSingleColumnLeft
)

// ClassifyHunk picks dual-column if both sides changed, or single-column for pure additions or deletions.
func ClassifyHunk(
	h renderutil.Interval,
	filteredPairs []serialize.LineAlignmentPair,
	isPairChanged []bool,
	srcLines, dstLines []string,
	leftSpansByLine, rightSpansByLine map[int][]serialize.HighlightSpan,
	forceSBS bool,
) HunkLayoutKind {
	if forceSBS {
		return HunkLayoutDualColumn
	}

	hasLeftChanges := false
	hasRightChanges := false

	for p := h.Start; p <= h.End; p++ {
		if p < 0 || p >= len(filteredPairs) {
			continue
		}
		if p < len(isPairChanged) && !isPairChanged[p] {
			continue
		}

		pair := filteredPairs[p]
		if pair.RightLine == -1 && pair.LeftLine >= 0 {
			hasLeftChanges = true
		} else if pair.LeftLine == -1 && pair.RightLine >= 0 {
			hasRightChanges = true
		} else if pair.LeftLine >= 0 && pair.RightLine >= 0 {
			var srcText, dstText string
			if pair.LeftLine < len(srcLines) {
				srcText = srcLines[pair.LeftLine]
			}
			if pair.RightLine < len(dstLines) {
				dstText = dstLines[pair.RightLine]
			}

			leftChanged := len(leftSpansByLine[pair.LeftLine]) > 0 || srcText != dstText
			rightChanged := len(rightSpansByLine[pair.RightLine]) > 0 || srcText != dstText

			if leftChanged {
				hasLeftChanges = true
			}
			if rightChanged {
				hasRightChanges = true
			}
		}

		if hasLeftChanges && hasRightChanges {
			return HunkLayoutDualColumn
		}
	}

	if !hasLeftChanges && hasRightChanges {
		return HunkLayoutSingleColumnRight
	}
	if hasLeftChanges && !hasRightChanges {
		return HunkLayoutSingleColumnLeft
	}

	return HunkLayoutDualColumn
}
