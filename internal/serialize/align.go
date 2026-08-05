package serialize

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// AlignLines computes a visual side-by-side alignment grid for the source and
// destination lines, using line-level similarity and AST mappings.
func AlignLines(srcBytes, dstBytes []byte, es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode) []LineAlignmentPair {
	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	// If one file is empty, align the other with empty space (fillers).
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

	// Identify moved nodes and track which lines they are on.
	movedSrcNodes := make(map[*treesitter.ASTNode]bool)
	movedDstNodes := make(map[*treesitter.ASTNode]bool)

	if es != nil {
		for _, a := range es.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				if ms != nil {
					if destNode := ms.Src()[a.Node]; destNode != nil {
						if a.Node.Parent == nil || destNode.Parent == nil || ms.Src()[a.Node.Parent] != destNode.Parent {
							movedSrcNodes[a.Node] = true
							movedDstNodes[destNode] = true
						}
					}
				}
			}
		}
	}

	movedSrcLines := make(map[int]bool)
	movedDstLines := make(map[int]bool)

	// Collect leaf nodes to map their line numbers.
	srcLeaves := srcRoot.Leaves()
	dstLeaves := dstRoot.Leaves()

	for _, n := range srcLeaves {
		if hasMovedAncestor(n, movedSrcNodes) {
			for r := int(n.StartRow); r <= int(n.EndRow); r++ {
				movedSrcLines[r] = true
			}
		}
	}
	for _, n := range dstLeaves {
		if hasMovedAncestor(n, movedDstNodes) {
			for r := int(n.StartRow); r <= int(n.EndRow); r++ {
				movedDstLines[r] = true
			}
		}
	}

	closeGaps(movedSrcLines, len(srcLines))
	closeGaps(movedDstLines, len(dstLines))

	srcLineMapsTo := make(map[int]map[int]bool)
	dstLineMapsTo := make(map[int]map[int]bool)
	if ms != nil {
		for srcNode, dstNode := range ms.Src() {
			srcRow := int(srcNode.StartRow)
			dstRow := int(dstNode.StartRow)
			if srcLineMapsTo[srcRow] == nil {
				srcLineMapsTo[srcRow] = make(map[int]bool)
			}
			srcLineMapsTo[srcRow][dstRow] = true

			if dstLineMapsTo[dstRow] == nil {
				dstLineMapsTo[dstRow] = make(map[int]bool)
			}
			dstLineMapsTo[dstRow][srcRow] = true
		}
	}

	isLineMappedToOther := func(srcIdx, dstIdx int) bool {
		srcTargets := srcLineMapsTo[srcIdx]
		if len(srcTargets) > 1 || (len(srcTargets) == 1 && !srcTargets[dstIdx]) {
			return true
		}
		dstTargets := dstLineMapsTo[dstIdx]
		return len(dstTargets) > 1 || (len(dstTargets) == 1 && !dstTargets[srcIdx])
	}

	type anchorPair struct {
		left  int
		right int
	}

	var anchors []anchorPair
	lastJ := -1
	for i := 0; i < len(srcLines); i++ {
		if movedSrcLines[i] {
			continue
		}
		targets := srcLineMapsTo[i]
		if len(targets) != 1 {
			continue
		}
		var j int
		for j = range targets {
		}
		if movedDstLines[j] || j <= lastJ {
			continue
		}
		reverseTargets := dstLineMapsTo[j]
		if len(reverseTargets) != 1 || !reverseTargets[i] {
			continue
		}
		if srcLines[i] != dstLines[j] {
			continue
		}
		anchors = append(anchors, anchorPair{left: i, right: j})
		lastJ = j
	}

	alignRegion := func(srcStart, srcEnd, dstStart, dstEnd int) []LineAlignmentPair {
		n := srcEnd - srcStart
		m := dstEnd - dstStart

		if n <= 0 {
			res := make([]LineAlignmentPair, m)
			for j := 0; j < m; j++ {
				res[j] = LineAlignmentPair{LeftLine: -1, RightLine: dstStart + j}
			}
			return res
		}
		if m <= 0 {
			res := make([]LineAlignmentPair, n)
			for i := 0; i < n; i++ {
				res[i] = LineAlignmentPair{LeftLine: srcStart + i, RightLine: -1}
			}
			return res
		}

		prefixLimit := 0
		for prefixLimit < n && prefixLimit < m {
			srcIdx := srcStart + prefixLimit
			dstIdx := dstStart + prefixLimit
			if srcLines[srcIdx] != dstLines[dstIdx] {
				break
			}
			if movedSrcLines[srcIdx] || movedDstLines[dstIdx] {
				break
			}
			if isLineMappedToOther(srcIdx, dstIdx) {
				break
			}
			prefixLimit++
		}

		suffixLimit := 0
		for suffixLimit < n-prefixLimit && suffixLimit < m-prefixLimit {
			srcIdx := srcEnd - 1 - suffixLimit
			dstIdx := dstEnd - 1 - suffixLimit
			if srcLines[srcIdx] != dstLines[dstIdx] {
				break
			}
			if movedSrcLines[srcIdx] || movedDstLines[dstIdx] {
				break
			}
			if isLineMappedToOther(srcIdx, dstIdx) {
				break
			}
			suffixLimit++
		}

		grid := make([]LineAlignmentPair, 0, n+m)

		for k := 0; k < prefixLimit; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: srcStart + k, RightLine: dstStart + k})
		}

		nMiddle := n - prefixLimit - suffixLimit
		mMiddle := m - prefixLimit - suffixLimit

		if nMiddle > 0 || mMiddle > 0 {
			middleSrcStart := srcStart + prefixLimit
			middleDstStart := dstStart + prefixLimit

			dp := make([][]int, nMiddle+1)
			for i := range dp {
				dp[i] = make([]int, mMiddle+1)
			}

			subOverlap := make(map[int]map[int]int)
			for _, srcLeaf := range srcLeaves {
				if dstLeaf, mapped := ms.Src()[srcLeaf]; mapped {
					sRow := int(srcLeaf.StartRow)
					dRow := int(dstLeaf.StartRow)
					if sRow >= middleSrcStart && sRow < middleSrcStart+nMiddle &&
						dRow >= middleDstStart && dRow < middleDstStart+mMiddle {
						if subOverlap[sRow] == nil {
							subOverlap[sRow] = make(map[int]int)
						}
						subOverlap[sRow][dRow]++
					}
				}
			}

			for i := 1; i <= nMiddle; i++ {
				for j := 1; j <= mMiddle; j++ {
					srcLineIdx := middleSrcStart + i - 1
					dstLineIdx := middleDstStart + j - 1

					if movedSrcLines[srcLineIdx] || movedDstLines[dstLineIdx] {
						dp[i][j] = max(dp[i-1][j], dp[i][j-1])
						continue
					}

					weight := computeLineWeight(srcLineIdx, dstLineIdx, srcLines, dstLines, subOverlap, ms, srcRoot, dstRoot)
					if weight > 0 {
						dp[i][j] = max(dp[i-1][j-1]+weight, max(dp[i-1][j], dp[i][j-1]))
					} else {
						dp[i][j] = max(dp[i-1][j], dp[i][j-1])
					}
				}
			}

			var reversedMiddle []LineAlignmentPair
			i, j := nMiddle, mMiddle

			for i > 0 || j > 0 {
				if i > 0 && j > 0 {
					srcLineIdx := middleSrcStart + i - 1
					dstLineIdx := middleDstStart + j - 1

					if !movedSrcLines[srcLineIdx] && !movedDstLines[dstLineIdx] {
						if weight := computeLineWeight(srcLineIdx, dstLineIdx, srcLines, dstLines, subOverlap, ms, srcRoot, dstRoot); weight > 0 && dp[i][j] == dp[i-1][j-1]+weight {
							reversedMiddle = append(reversedMiddle, LineAlignmentPair{LeftLine: srcLineIdx, RightLine: dstLineIdx})
							i--
							j--
							continue
						}
					}
				}

				if i > 0 && (j == 0 || dp[i-1][j] > dp[i][j-1]) {
					reversedMiddle = append(reversedMiddle, LineAlignmentPair{LeftLine: middleSrcStart + i - 1, RightLine: -1})
					i--
				} else {
					reversedMiddle = append(reversedMiddle, LineAlignmentPair{LeftLine: -1, RightLine: middleDstStart + j - 1})
					j--
				}
			}

			slices.Reverse(reversedMiddle)
			grid = append(grid, reversedMiddle...)
		}

		for k := 0; k < suffixLimit; k++ {
			srcIdx := srcEnd - suffixLimit + k
			dstIdx := dstEnd - suffixLimit + k
			grid = append(grid, LineAlignmentPair{LeftLine: srcIdx, RightLine: dstIdx})
		}

		return grid
	}

	var grid []LineAlignmentPair
	currentSrc := 0
	currentDst := 0

	for _, a := range anchors {
		grid = append(grid, alignRegion(currentSrc, a.left, currentDst, a.right)...)
		grid = append(grid, LineAlignmentPair{LeftLine: a.left, RightLine: a.right})
		currentSrc = a.left + 1
		currentDst = a.right + 1
	}

	grid = append(grid, alignRegion(currentSrc, len(srcLines), currentDst, len(dstLines))...)

	return grid
}

// computeLineWeight rates how strongly two lines align. Standalone brackets only match if their parent AST containers map.
func computeLineWeight(srcLineIdx, dstLineIdx int, srcLines, dstLines []string, overlap map[int]map[int]int, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode) int {
	count := overlap[srcLineIdx][dstLineIdx]

	if srcLines[srcLineIdx] == dstLines[dstLineIdx] {
		if count > 0 {
			return 1000 + 10*count
		}
		if isTrivialLine(srcLines[srcLineIdx]) && !areContainersMatched(srcRoot, dstRoot, srcLineIdx, dstLineIdx, ms) {
			return 0
		}
		return 1000
	} else if count > 0 {
		return 100 * count
	}
	return 0
}

// areContainersMatched checks whether the AST blocks surrounding srcRow and dstRow map to each other.
func areContainersMatched(srcRoot, dstRoot *treesitter.ASTNode, srcRow, dstRow int, ms *engine.Mapping) bool {
	if srcRoot == nil || dstRoot == nil || ms == nil {
		return true
	}

	srcBlock := findEnclosingContainer(srcRoot, srcRow)
	dstBlock := findEnclosingContainer(dstRoot, dstRow)

	if srcBlock == nil || dstBlock == nil {
		return srcBlock == dstBlock
	}

	for curr := srcBlock; curr != nil && curr != srcRoot; curr = curr.Parent {
		if mappedDst, ok := ms.Src()[curr]; ok {
			for dCurr := dstBlock; dCurr != nil && dCurr != dstRoot; dCurr = dCurr.Parent {
				if dCurr == mappedDst {
					return true
				}
			}
		}
	}
	return false
}

// findEnclosingContainer returns the smallest AST container wrapping the given row.
func findEnclosingContainer(root *treesitter.ASTNode, row int) *treesitter.ASTNode {
	var best *treesitter.ASTNode
	for curr := root; curr != nil; {
		if int(curr.StartRow) > row || row > int(curr.EndRow) {
			break
		}
		if curr != root && len(curr.Children) > 0 {
			best = curr
		}
		var next *treesitter.ASTNode
		for _, child := range curr.Children {
			if int(child.StartRow) <= row && row <= int(child.EndRow) {
				next = child
				break
			}
		}
		curr = next
	}
	return best
}

// isTrivialLine reports whether a line is just brackets or structural punctuation.
func isTrivialLine(s string) bool {
	s = strings.TrimSpace(s)
	switch s {
	case "{", "}", "(", ")", "[", "]", "};", ");", "})", "});", "()", "{}", "() {":
		return true
	}
	return false
}

func hasMovedAncestor(n *treesitter.ASTNode, movedNodes map[*treesitter.ASTNode]bool) bool {
	curr := n
	for curr != nil {
		if movedNodes[curr] {
			return true
		}
		curr = curr.Parent
	}
	return false
}

func closeGaps(moved map[int]bool, maxLine int) {
	last := -1
	for i := range maxLine {
		if moved[i] {
			if last != -1 && i-last <= 4 {
				for j := last + 1; j < i; j++ {
					moved[j] = true
				}
			}
			last = i
		}
	}
}
