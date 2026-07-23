package serialize

import (
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
				movedSrcNodes[a.Node] = true
				if ms != nil {
					if destNode := ms.Src()[a.Node]; destNode != nil {
						movedDstNodes[destNode] = true
					}
				}
			}
		}
	}

	movedSrcLines := make(map[int]bool)
	movedDstLines := make(map[int]bool)

	// Collect leaf nodes to map their line numbers.
	var srcLeaves, dstLeaves []*treesitter.ASTNode
	collectLeaves(srcRoot, &srcLeaves)
	collectLeaves(dstRoot, &dstLeaves)

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

	// Count overlapping mapped leaf nodes between before and after lines.
	overlap := make(map[int]map[int]int)
	if ms != nil {
		for _, srcLeaf := range srcLeaves {
			if dstLeaf, mapped := ms.Src()[srcLeaf]; mapped {
				sRow := int(srcLeaf.StartRow)
				dRow := int(dstLeaf.StartRow)
				if overlap[sRow] == nil {
					overlap[sRow] = make(map[int]int)
				}
				overlap[sRow][dRow]++
			}
		}
	}

	// Run LCS (longest common subsequence) using a DP matrix to align lines.
	n := len(srcLines)
	m := len(dstLines)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			srcLineIdx := i - 1
			dstLineIdx := j - 1

			// Do not pair lines that are part of a moved block.
			if movedSrcLines[srcLineIdx] || movedDstLines[dstLineIdx] {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
				continue
			}

			// Determine match weight.
			weight := 0
			if srcLines[srcLineIdx] == dstLines[dstLineIdx] {
				weight = 1000 // Exact line matches take high priority.
				if count, ok := overlap[srcLineIdx][dstLineIdx]; ok && count > 0 {
					weight += 10 * count // Add a small bonus to break ties correctly.
				}
			} else if count, ok := overlap[srcLineIdx][dstLineIdx]; ok && count > 0 {
				weight = 100 * count // Modified lines align based on mapped tokens.
			}

			if weight > 0 {
				dp[i][j] = max(dp[i-1][j-1]+weight, max(dp[i-1][j], dp[i][j-1]))
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	// Reconstruct the alignment grid by backtracking.
	var reversedGrid []LineAlignmentPair
	i, j := n, m

	for i > 0 || j > 0 {
		if i > 0 && j > 0 {
			srcLineIdx := i - 1
			dstLineIdx := j - 1

			matched := false
			weight := 0
			if !movedSrcLines[srcLineIdx] && !movedDstLines[dstLineIdx] {
				if srcLines[srcLineIdx] == dstLines[dstLineIdx] {
					weight = 1000
					if count, ok := overlap[srcLineIdx][dstLineIdx]; ok && count > 0 {
						weight += 10 * count
					}
				} else if count, ok := overlap[srcLineIdx][dstLineIdx]; ok && count > 0 {
					weight = 100 * count
				}
				if weight > 0 && dp[i][j] == dp[i-1][j-1]+weight {
					matched = true
				}
			}

			if matched {
				reversedGrid = append(reversedGrid, LineAlignmentPair{LeftLine: srcLineIdx, RightLine: dstLineIdx})
				i--
				j--
				continue
			}
		}

		// Step towards the higher DP score.
		if i > 0 && (j == 0 || dp[i-1][j] > dp[i][j-1]) {
			reversedGrid = append(reversedGrid, LineAlignmentPair{LeftLine: i - 1, RightLine: -1})
			i--
		} else {
			reversedGrid = append(reversedGrid, LineAlignmentPair{LeftLine: -1, RightLine: j - 1})
			j--
		}
	}

	// Reverse the grid so it reads top-to-bottom.
	grid := make([]LineAlignmentPair, len(reversedGrid))
	for k := range reversedGrid {
		grid[k] = reversedGrid[len(reversedGrid)-1-k]
	}

	return grid
}

func collectLeaves(n *treesitter.ASTNode, out *[]*treesitter.ASTNode) {
	if n == nil {
		return
	}
	if len(n.Children) == 0 || n.Type == "string" || n.Type == "string_literal" || n.Type == "interpreted_string_literal" || n.Type == "raw_string_literal" {
		*out = append(*out, n)
		return
	}
	for _, child := range n.Children {
		collectLeaves(child, out)
	}
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
