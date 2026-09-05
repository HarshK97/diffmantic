package serialize

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/engine"
)

// AlignLines builds a side-by-side line alignment grid using line-level LCS diffing.
func AlignLines(srcBytes, dstBytes []byte) []LineAlignmentPair {
	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	// If one file is empty, align the other against filler lines.
	if len(srcLines) == 1 && srcLines[0] == "" {
		res := make([]LineAlignmentPair, len(dstLines))
		for j := range dstLines {
			res[j] = LineAlignmentPair{LeftLine: -1, RightLine: j}
		}
		return res
	}
	if len(dstLines) == 1 && dstLines[0] == "" {
		res := make([]LineAlignmentPair, len(srcLines))
		for i := range srcLines {
			res[i] = LineAlignmentPair{LeftLine: i, RightLine: -1}
		}
		return res
	}

	matched := engine.LineDiff(srcLines, dstLines)

	type anchor struct {
		src int
		dst int
	}

	var anchors []anchor
	lastDst := -1
	for i := range len(srcLines) {
		if j, ok := matched[i]; ok {
			if j > lastDst {
				anchors = append(anchors, anchor{src: i, dst: j})
				lastDst = j
			}
		}
	}

	grid := make([]LineAlignmentPair, 0, max(len(srcLines), len(dstLines)))
	currSrc := 0
	currDst := 0

	alignSegment := func(targetSrc, targetDst int) {
		srcCount := targetSrc - currSrc
		dstCount := targetDst - currDst
		minLen := min(srcCount, dstCount)

		for k := range minLen {
			grid = append(grid, LineAlignmentPair{LeftLine: currSrc + k, RightLine: currDst + k})
		}
		for k := minLen; k < srcCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: currSrc + k, RightLine: -1})
		}
		for k := minLen; k < dstCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: -1, RightLine: currDst + k})
		}
	}

	for _, a := range anchors {
		alignSegment(a.src, a.dst)
		grid = append(grid, LineAlignmentPair{LeftLine: a.src, RightLine: a.dst})
		currSrc = a.src + 1
		currDst = a.dst + 1
	}

	alignSegment(len(srcLines), len(dstLines))

	return grid
}
