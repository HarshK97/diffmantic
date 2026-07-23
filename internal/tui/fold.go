package tui

import (
	"sort"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

// A single visible row on the screen.
type virtualLine struct {
	alignedRow int // index in LineAlignment grid (-1 if fold marker)
	leftLine   int // 0-indexed line number in source file (-1 if filler or fold marker)
	rightLine  int // 0-indexed line number in dest file (-1 if filler or fold marker)
	foldIdx    int // index into model.folds (-1 if real line)
}

// fold is a collapsible group of unchanged aligned grid rows.
type fold struct {
	startLine int  // first grid row index in the fold (inclusive)
	endLine   int  // last grid row index in the fold (inclusive)
	open      bool // true = expanded, false = collapsed
}

// computeFolds identifies unchanged code blocks outside of context windows around changes.
func computeFolds(changeRows []int, totalRows, context int) []fold {
	if totalRows == 0 {
		return nil
	}
	if len(changeRows) == 0 {
		// Fold the entire file if nothing changed.
		if totalRows > 1 {
			return []fold{{startLine: 0, endLine: totalRows - 1}}
		}
		return nil
	}

	// Keep context lines visible around each change.
	type window struct{ lo, hi int }
	windows := make([]window, 0, len(changeRows))
	for _, cl := range changeRows {
		lo := cl - context
		hi := cl + context
		if lo < 0 {
			lo = 0
		}
		if hi >= totalRows {
			hi = totalRows - 1
		}
		windows = append(windows, window{lo, hi})
	}

	// Merge any windows that overlap.
	merged := []window{windows[0]}
	for i := 1; i < len(windows); i++ {
		last := &merged[len(merged)-1]
		if windows[i].lo <= last.hi+1 {
			if windows[i].hi > last.hi {
				last.hi = windows[i].hi
			}
		} else {
			merged = append(merged, windows[i])
		}
	}

	// Turn the gaps between visible windows into folds.
	var folds []fold

	if merged[0].lo > 0 {
		folds = append(folds, fold{startLine: 0, endLine: merged[0].lo - 1})
	}

	for i := 1; i < len(merged); i++ {
		gapStart := merged[i-1].hi + 1
		gapEnd := merged[i].lo - 1
		if gapEnd >= gapStart {
			folds = append(folds, fold{startLine: gapStart, endLine: gapEnd})
		}
	}

	lastHi := merged[len(merged)-1].hi
	if lastHi < totalRows-1 {
		folds = append(folds, fold{startLine: lastHi + 1, endLine: totalRows - 1})
	}

	// Skip single-line folds: not worth collapsing.
	var result []fold
	for _, f := range folds {
		if f.endLine-f.startLine+1 >= 2 {
			result = append(result, f)
		}
	}

	return result
}

// buildVirtualLines creates a list of visible lines and fold markers to display.
func buildVirtualLines(folds []fold, totalRows int, lineAlignment []serialize.LineAlignmentPair) []virtualLine {
	if totalRows == 0 {
		return nil
	}

	sortedFolds := make([]fold, len(folds))
	copy(sortedFolds, folds)
	sort.Slice(sortedFolds, func(i, j int) bool {
		return sortedFolds[i].startLine < sortedFolds[j].startLine
	})

	foldByStart := make(map[int]int, len(folds))
	for i, f := range folds {
		foldByStart[f.startLine] = i
	}

	var vlines []virtualLine
	row := 0
	for row < totalRows {
		if fi, ok := foldByStart[row]; ok {
			f := folds[fi]
			if f.open {
				// Emit lines normally for open folds.
				for r := f.startLine; r <= f.endLine; r++ {
					pair := lineAlignment[r]
					vlines = append(vlines, virtualLine{
						alignedRow: r,
						leftLine:   pair.LeftLine,
						rightLine:  pair.RightLine,
						foldIdx:    -1,
					})
				}
			} else {
				// Emit a single fold marker for collapsed folds.
				vlines = append(vlines, virtualLine{
					alignedRow: -1,
					leftLine:   -1,
					rightLine:  -1,
					foldIdx:    fi,
				})
			}
			row = f.endLine + 1
		} else {
			pair := lineAlignment[row]
			vlines = append(vlines, virtualLine{
				alignedRow: row,
				leftLine:   pair.LeftLine,
				rightLine:  pair.RightLine,
				foldIdx:    -1,
			})
			row++
		}
	}

	return vlines
}

// realToVirtual maps an aligned grid row index to its display index.
func realToVirtual(vlines []virtualLine, folds []fold, alignedRow int) int {
	for i, vl := range vlines {
		if vl.foldIdx >= 0 {
			f := folds[vl.foldIdx]
			if alignedRow >= f.startLine && alignedRow <= f.endLine {
				return i
			}
		} else {
			if vl.alignedRow == alignedRow {
				return i
			}
		}
	}
	return -1
}

func foldAtVirtual(vlines []virtualLine, folds []fold, idx int) int {
	if idx < 0 || idx >= len(vlines) {
		return -1
	}
	vl := vlines[idx]
	if vl.foldIdx >= 0 {
		return vl.foldIdx
	}
	if vl.alignedRow >= 0 {
		for i, f := range folds {
			if vl.alignedRow >= f.startLine && vl.alignedRow <= f.endLine {
				return i
			}
		}
	}
	return -1
}
