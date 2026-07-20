package tui

import "sort"

// A single visible row on the screen.
type virtualLine struct {
	realLine int // Real line number, 0-indexed. Set to -1 if this is a fold marker.
	foldIdx  int // Index into model.folds. Set to -1 if this is a real line.
}

// A group of unchanged lines that can be collapsed.
type fold struct {
	startLine int
	endLine   int
	open      bool
}

// Find unchanged code blocks that are far enough away from any changes.
func computeFolds(changeLines []int, totalLines, context int) []fold {
	if totalLines == 0 {
		return nil
	}
	if len(changeLines) == 0 {
		if totalLines > 1 {
			return []fold{{startLine: 0, endLine: totalLines - 1}}
		}
		return nil
	}

	// Create visible padding windows around the changed lines.
	type window struct{ lo, hi int }
	windows := make([]window, 0, len(changeLines))
	for _, cl := range changeLines {
		lo := cl - context
		hi := cl + context
		if lo < 0 {
			lo = 0
		}
		if hi >= totalLines {
			hi = totalLines - 1
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
	if lastHi < totalLines-1 {
		folds = append(folds, fold{startLine: lastHi + 1, endLine: totalLines - 1})
	}

	// Ignore tiny folds of only 1 line.
	var result []fold
	for _, f := range folds {
		if f.endLine-f.startLine+1 >= 2 {
			result = append(result, f)
		}
	}

	return result
}

// Build the list of visible lines and fold markers we want to show.
func buildVirtualLines(folds []fold, totalLines int) []virtualLine {
	if totalLines == 0 {
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
	line := 0
	for line < totalLines {
		if fi, ok := foldByStart[line]; ok {
			f := folds[fi]
			if f.open {
				for rl := f.startLine; rl <= f.endLine; rl++ {
					vlines = append(vlines, virtualLine{realLine: rl, foldIdx: -1})
				}
			} else {
				vlines = append(vlines, virtualLine{realLine: -1, foldIdx: fi})
			}
			line = f.endLine + 1
		} else {
			vlines = append(vlines, virtualLine{realLine: line, foldIdx: -1})
			line++
		}
	}

	return vlines
}

func realToVirtual(vlines []virtualLine, realLine int) int {
	for i, vl := range vlines {
		if vl.realLine == realLine {
			return i
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
	if vl.realLine >= 0 {
		for i, f := range folds {
			if vl.realLine >= f.startLine && vl.realLine <= f.endLine {
				return i
			}
		}
	}
	return -1
}
