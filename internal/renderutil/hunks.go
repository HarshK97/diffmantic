package renderutil

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

// Interval represents an inclusive index range [Start, End].
type Interval struct {
	Start int
	End   int
}

// BuildChangeIntervals collapses consecutive changed line pairs into intervals.
func BuildChangeIntervals(isPairChanged []bool) []Interval {
	var changeIntervals []Interval
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
				changeIntervals = append(changeIntervals, Interval{Start: startIdx, End: i - 1})
				inChange = false
			}
		}
	}
	if inChange {
		changeIntervals = append(changeIntervals, Interval{Start: startIdx, End: len(isPairChanged) - 1})
	}
	return changeIntervals
}

// MergeHunks expands change intervals by contextLines and merges overlapping hunks.
func MergeHunks(changeIntervals []Interval, totalPairs, contextLines int) []Interval {
	var hunks []Interval
	for _, ci := range changeIntervals {
		hStart := max(0, ci.Start-contextLines)
		hEnd := min(totalPairs-1, ci.End+contextLines)

		if len(hunks) > 0 && hStart <= hunks[len(hunks)-1].End+1 {
			hunks[len(hunks)-1].End = hEnd
		} else {
			hunks = append(hunks, Interval{Start: hStart, End: hEnd})
		}
	}
	return hunks
}

// ComputeHunkRange calculates 1-based start line numbers and line counts for both sides of a hunk.
func ComputeHunkRange(
	h Interval,
	pairs []serialize.LineAlignmentPair,
	hasSrc, hasDst bool,
) (srcStart, srcCount, dstStart, dstCount int) {
	srcStart = -1
	dstStart = -1

	for p := h.Start; p <= h.End && p < len(pairs); p++ {
		pair := pairs[p]
		if pair.LeftLine != -1 {
			if srcStart == -1 {
				srcStart = pair.LeftLine + 1
			}
			srcCount++
		}
		if pair.RightLine != -1 {
			if dstStart == -1 {
				dstStart = pair.RightLine + 1
			}
			dstCount++
		}
	}

	if srcStart == -1 {
		for p := h.Start - 1; p >= 0 && p < len(pairs); p-- {
			if pairs[p].LeftLine != -1 {
				srcStart = pairs[p].LeftLine + 1
				break
			}
		}
		if srcStart == -1 {
			if hasSrc {
				srcStart = 1
			} else {
				srcStart = 0
			}
		}
	}

	if dstStart == -1 {
		for p := h.Start - 1; p >= 0 && p < len(pairs); p-- {
			if pairs[p].RightLine != -1 {
				dstStart = pairs[p].RightLine + 1
				break
			}
		}
		if dstStart == -1 {
			if hasDst {
				dstStart = 1
			} else {
				dstStart = 0
			}
		}
	}

	return srcStart, srcCount, dstStart, dstCount
}

// FormatRange formats a line start and count for a unified diff hunk header.
func FormatRange(start, count int) string {
	if count == 1 {
		return strconv.Itoa(start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// FormatHunkHeader formats a standard unified diff hunk header line.
func FormatHunkHeader(srcStart, srcCount, dstStart, dstCount int, extra string) string {
	return fmt.Sprintf("@@ -%s +%s @@%s", FormatRange(srcStart, srcCount), FormatRange(dstStart, dstCount), extra)
}

// MaxVisibleLine returns the highest 1-indexed line number visible across the hunks, or 0 if empty.
func MaxVisibleLine(hunks []Interval, pairs []serialize.LineAlignmentPair) int {
	maxLine := 0
	for _, h := range hunks {
		for p := h.Start; p <= h.End && p < len(pairs); p++ {
			maxLine = max(maxLine, pairs[p].LeftLine+1, pairs[p].RightLine+1)
		}
	}
	return maxLine
}

// LineDisplayWidth returns the column width of text in the terminal,
// expanding tabs and accounting for wide characters.
func LineDisplayWidth(text string, tabWidth int) int {
	if tabWidth <= 0 {
		tabWidth = 4
	}
	text = strings.TrimRight(text, "\r\n")
	col := 0
	for _, r := range text {
		if r == '\t' {
			col += tabWidth - (col % tabWidth)
		} else {
			col += max(1, runewidth.RuneWidth(r))
		}
	}
	return col
}

// MaxVisibleLineDisplayWidth returns the widest display width among source
// and destination lines across all visible hunks, including move badges.
func MaxVisibleLineDisplayWidth(
	hunks []Interval,
	pairs []serialize.LineAlignmentPair,
	srcLines, dstLines []string,
	srcBadges, dstBadges map[int]string,
	tabWidth int,
) int {
	maxCol := 0
	for _, h := range hunks {
		for p := max(0, h.Start); p <= h.End && p < len(pairs); p++ {
			pair := pairs[p]
			if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
				w := LineDisplayWidth(srcLines[pair.LeftLine], tabWidth)
				if srcBadges != nil && srcBadges[pair.LeftLine] != "" {
					w += runewidth.StringWidth(srcBadges[pair.LeftLine])
				}
				maxCol = max(maxCol, w)
			}
			if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
				w := LineDisplayWidth(dstLines[pair.RightLine], tabWidth)
				if dstBadges != nil && dstBadges[pair.RightLine] != "" {
					w += runewidth.StringWidth(dstBadges[pair.RightLine])
				}
				maxCol = max(maxCol, w)
			}
		}
	}
	return maxCol
}
