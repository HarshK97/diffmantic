package serialize

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// AlignLines computes a visual side-by-side alignment grid for the source and
// destination lines, using line-level similarity and AST mappings.
func AlignLines(
	srcBytes, dstBytes []byte,
	es *actions.EditScript,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
	commentLineMappings map[int]int,
) []LineAlignmentPair {
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
	movedSrcLines := make(map[int]bool)
	movedDstLines := make(map[int]bool)

	var r *rules.Rules
	if srcRoot != nil {
		r = rules.Get(srcRoot.GetLanguage())
	}
	if r == nil && dstRoot != nil {
		r = rules.Get(dstRoot.GetLanguage())
	}

	if es != nil {
		for _, a := range es.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				destNode := a.DestNode
				if destNode == nil && ms != nil {
					destNode = ms.Src()[a.Node]
				}
				if destNode != nil {
					if a.Node.Parent == nil || destNode.Parent == nil || ms == nil || ms.Src()[a.Node.Parent] != destNode.Parent {
						isComment := r != nil && (r.IsComment(a.Node.Type) || r.IsComment(destNode.Type))
						if ms == nil || isDisplacedMove(a.Node, destNode, ms) || isComment {
							movedSrcNodes[a.Node] = true
							movedDstNodes[destNode] = true
							for row := int(a.Node.StartRow); row <= int(a.Node.EndRow); row++ {
								movedSrcLines[row] = true
							}
							for row := int(destNode.StartRow); row <= int(destNode.EndRow); row++ {
								movedDstLines[row] = true
							}
						}
					}
				}
			}
		}
	}

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
	for srcRow, dstRow := range commentLineMappings {
		if srcLineMapsTo[srcRow] == nil {
			srcLineMapsTo[srcRow] = make(map[int]bool)
		}
		srcLineMapsTo[srcRow][dstRow] = true

		if dstLineMapsTo[dstRow] == nil {
			dstLineMapsTo[dstRow] = make(map[int]bool)
		}
		dstLineMapsTo[dstRow][srcRow] = true
	}
	if es != nil {
		for _, a := range es.Actions() {
			if a.Type == actions.Update && a.Node != nil && a.DestNode != nil {
				srcRow := int(a.Node.StartRow)
				dstRow := int(a.DestNode.StartRow)
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

		// Back up past trailing blanks or comment openers so they don't greedily lock down the prefix.
		if prefixLimit < n && prefixLimit < m {
			for prefixLimit > 0 {
				lastSrc := srcStart + prefixLimit - 1
				trimmed := strings.TrimSpace(srcLines[lastSrc])
				if trimmed == "" || isTrivialAnchorLine(srcLines[lastSrc]) {
					prefixLimit--
				} else {
					break
				}
			}
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

		// Back up past leading blanks or comment delimiters so the suffix doesn't greedily grab them.
		if suffixLimit < n-prefixLimit && suffixLimit < m-prefixLimit {
			for suffixLimit > 0 {
				firstSrc := srcEnd - suffixLimit
				trimmed := strings.TrimSpace(srcLines[firstSrc])
				if trimmed == "" || isTrivialAnchorLine(srcLines[firstSrc]) {
					suffixLimit--
				} else {
					break
				}
			}
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

					weight := computeLineWeight(srcLineIdx, dstLineIdx, srcLines, dstLines, subOverlap, ms, srcRoot, dstRoot, isLineMappedToOther, srcLineMapsTo)
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
						if weight := computeLineWeight(srcLineIdx, dstLineIdx, srcLines, dstLines, subOverlap, ms, srcRoot, dstRoot, isLineMappedToOther, srcLineMapsTo); weight > 0 && dp[i][j] == dp[i-1][j-1]+weight {
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

	return coalesceAlignmentGrid(grid, srcLines, dstLines, movedSrcLines, movedDstLines, srcLineMapsTo)
}

// coalesceAlignmentGrid pairs unaligned left/right lines side-by-side within changed blocks
// so modified lines render next to each other instead of producing interleaved gaps.
func coalesceAlignmentGrid(
	grid []LineAlignmentPair,
	srcLines, dstLines []string,
	movedSrcLines, movedDstLines map[int]bool,
	srcLineMapsTo map[int]map[int]bool,
) []LineAlignmentPair {
	var result []LineAlignmentPair
	var unalignedLeft []int
	var unalignedRight []int

	flushUnaligned := func(nextPair *LineAlignmentPair) {
		if len(unalignedLeft) == 0 && len(unalignedRight) == 0 {
			return
		}

		var stdLeft []int
		var stdRight []int

		flushStd := func(isEnd bool) {
			nLeft := len(stdLeft)
			nRight := len(stdRight)
			if nLeft == 0 && nRight == 0 {
				return
			}
			if isEnd && nLeft > 0 && nRight > 0 && nLeft != nRight {
				alignBottom := false
				if nextPair != nil && nextPair.LeftLine != -1 && nextPair.RightLine != -1 {
					if stdLeft[nLeft-1]+1 == nextPair.LeftLine && stdRight[nRight-1]+1 == nextPair.RightLine {
						alignBottom = true
					}
				}
				if alignBottom {
					if nLeft < nRight {
						for k := 0; k < nRight-nLeft; k++ {
							result = append(result, LineAlignmentPair{LeftLine: -1, RightLine: stdRight[k]})
						}
						for k := 0; k < nLeft; k++ {
							result = append(result, LineAlignmentPair{LeftLine: stdLeft[k], RightLine: stdRight[nRight-nLeft+k]})
						}
						stdLeft = nil
						stdRight = nil
						return
					} else if nLeft > nRight {
						for k := 0; k < nLeft-nRight; k++ {
							result = append(result, LineAlignmentPair{LeftLine: stdLeft[k], RightLine: -1})
						}
						for k := 0; k < nRight; k++ {
							result = append(result, LineAlignmentPair{LeftLine: stdLeft[nLeft-nRight+k], RightLine: stdRight[k]})
						}
						stdLeft = nil
						stdRight = nil
						return
					}
				}
			}

			minLen := min(nLeft, nRight)
			for k := 0; k < minLen; k++ {
				result = append(result, LineAlignmentPair{LeftLine: stdLeft[k], RightLine: stdRight[k]})
			}
			for k := minLen; k < nLeft; k++ {
				result = append(result, LineAlignmentPair{LeftLine: stdLeft[k], RightLine: -1})
			}
			for k := minLen; k < nRight; k++ {
				result = append(result, LineAlignmentPair{LeftLine: -1, RightLine: stdRight[k]})
			}
			stdLeft = nil
			stdRight = nil
		}

		for _, l := range unalignedLeft {
			if movedSrcLines[l] {
				flushStd(false)
				result = append(result, LineAlignmentPair{LeftLine: l, RightLine: -1})
			} else {
				stdLeft = append(stdLeft, l)
			}
		}

		for _, r := range unalignedRight {
			if movedDstLines[r] {
				flushStd(false)
				result = append(result, LineAlignmentPair{LeftLine: -1, RightLine: r})
			} else {
				stdRight = append(stdRight, r)
			}
		}

		flushStd(true)
		unalignedLeft = nil
		unalignedRight = nil
	}

	for _, pair := range grid {
		if pair.LeftLine != -1 && pair.RightLine != -1 {
			// Bare delimiters like "/**" or "*/" shouldn't break up an unaligned block unless explicitly mapped.
			isMapped := srcLineMapsTo[pair.LeftLine] != nil && srcLineMapsTo[pair.LeftLine][pair.RightLine]
			if !isMapped && (len(unalignedLeft) > 0 || len(unalignedRight) > 0) && isTrivialAnchorLine(srcLines[pair.LeftLine]) {
				unalignedLeft = append(unalignedLeft, pair.LeftLine)
				unalignedRight = append(unalignedRight, pair.RightLine)
				continue
			}
			flushUnaligned(&pair)
			result = append(result, pair)
		} else {
			if pair.LeftLine != -1 {
				unalignedLeft = append(unalignedLeft, pair.LeftLine)
			}
			if pair.RightLine != -1 {
				unalignedRight = append(unalignedRight, pair.RightLine)
			}
		}
	}
	flushUnaligned(nil)
	return result
}

// isTrivialAnchorLine returns true for comment markers like "/*" or "<p>" that shouldn't anchor alignment.
func isTrivialAnchorLine(s string) bool {
	trimmed := strings.TrimSpace(s)
	switch trimmed {
	case "*", "/**", "/*", "*/", "//", "<p>", "</p>":
		return true
	}
	return false
}

// computeLineWeight rates how strongly two lines align. Standalone brackets only match if their parent AST containers map.
func computeLineWeight(
	srcLineIdx, dstLineIdx int,
	srcLines, dstLines []string,
	overlap map[int]map[int]int,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
	isMappedOther func(int, int) bool,
	srcLineMapsTo map[int]map[int]bool,
) int {
	if isMappedOther != nil && isMappedOther(srcLineIdx, dstLineIdx) {
		return 0
	}

	isMapped := false
	if targets, ok := srcLineMapsTo[srcLineIdx]; ok && targets[dstLineIdx] {
		isMapped = true
	}

	count := overlap[srcLineIdx][dstLineIdx]

	if srcLines[srcLineIdx] == dstLines[dstLineIdx] {
		if count > 0 || isMapped {
			return 1000 + 10*count
		}
		if isTrivialLine(srcLines[srcLineIdx]) && !areContainersMatched(srcRoot, dstRoot, srcLineIdx, dstLineIdx, ms) {
			return 0
		}
		if isTrivialAnchorLine(srcLines[srcLineIdx]) {
			return 10
		}
		return 1000
	} else if isMapped {
		return 500
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

func isDisplacedMove(srcNode, dstNode *treesitter.ASTNode, ms *engine.Mapping) bool {
	if srcNode == nil || dstNode == nil || ms == nil {
		return true
	}

	var srcContainer, dstContainer *treesitter.ASTNode
	for curr := srcNode.Parent; curr != nil; curr = curr.Parent {
		if mappedDst, ok := ms.Src()[curr]; ok {
			srcContainer = curr
			dstContainer = mappedDst
			break
		}
	}

	if srcContainer != nil && dstContainer != nil {
		if dstNode.StartRow >= dstContainer.StartRow && dstNode.EndRow <= dstContainer.EndRow {
			relSrc := int(srcNode.StartRow - srcContainer.StartRow)
			relDst := int(dstNode.StartRow - dstContainer.StartRow)
			diff := relSrc - relDst
			if diff < 0 {
				diff = -diff
			}
			if diff <= 2 {
				return false
			}
		}
	}

	return int(srcNode.StartRow) != int(dstNode.StartRow)
}
