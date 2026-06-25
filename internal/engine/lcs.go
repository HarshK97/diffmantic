package engine

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type eqFunc func(a, b *treesitter.ASTNode) bool

// lcs computes the longest common subsequence of two node slices
// using the given equality function. Returns matched pairs.
func lcs(seq1, seq2 []*treesitter.ASTNode, eq eqFunc) [][2]*treesitter.ASTNode {
	n := len(seq1)
	m := len(seq2)
	if n == 0 || m == 0 {
		return nil
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if eq(seq1[i-1], seq2[j-1]) {
				weight := seq1[i-1].Size()
				dp[i][j] = dp[i-1][j-1] + weight
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	var pairs [][2]*treesitter.ASTNode
	i, j := n, m
	for i > 0 && j > 0 {
		if eq(seq1[i-1], seq2[j-1]) {
			pairs = append(pairs, [2]*treesitter.ASTNode{seq1[i-1], seq2[j-1]})
			i--
			j--
		} else if dp[i-1][j] >= dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	slices.Reverse(pairs)
	return pairs
}

// isomorphic equality (type + label + structure).
func LCSLabel(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	return lcs(seq1, seq2, Isomorphic)
}

// structure-only isomorphism (type + shape, ignoring leaf labels).
//
// This variant resolves ambiguous structural matches (one src node
// structurally equal to several dst nodes) with a positional +
// label-similarity tie-break. It first computes the plain structural LCS,
// then for each matched src node that is structurally equal to more than one
// matched/unmatched dst node, re-selects the best partner: same child index
// within the parent first, then the most label-similar candidate. This keeps
// a loop variable like "import_from_child" paired with "child" instead of the
// rightmost "children".
func LCSStructure(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	pairs := lcs(seq1, seq2, StructureIsomorphic)
	if len(pairs) == 0 {
		return pairs
	}

	dstUsed := make(map[*treesitter.ASTNode]bool, len(pairs))
	for _, p := range pairs {
		dstUsed[p[1]] = true
	}

	for k, p := range pairs {
		src := p[0]
		if !hasMultipleStructuralMatches(src, seq2, p[1]) {
			continue
		}
		best := bestStructuralPartner(src, seq2, p[1], dstUsed)
		if best != nil && best != p[1] {
			dstUsed[p[1]] = false
			dstUsed[best] = true
			pairs[k] = [2]*treesitter.ASTNode{src, best}
		}
	}

	slicesReverse(pairs)
	return pairs
}

func hasMultipleStructuralMatches(src *treesitter.ASTNode, seq2 []*treesitter.ASTNode, exclude *treesitter.ASTNode) bool {
	count := 0
	for _, d := range seq2 {
		if d == exclude {
			continue
		}
		if StructureIsomorphic(src, d) {
			count++
			if count >= 1 {
				return true
			}
		}
	}
	return false
}

func bestStructuralPartner(
	src *treesitter.ASTNode,
	seq2 []*treesitter.ASTNode,
	current *treesitter.ASTNode,
	dstUsed map[*treesitter.ASTNode]bool,
) *treesitter.ASTNode {
	srcIdx := childIndex(src)
	var best *treesitter.ASTNode
	bestScore := scorePartner(src, current, srcIdx, false)
	for _, d := range seq2 {
		if d == current || dstUsed[d] {
			continue
		}
		if !StructureIsomorphic(src, d) {
			continue
		}
		s := scorePartner(src, d, srcIdx, true)
		if s > bestScore {
			bestScore = s
			best = d
		}
	}
	return best
}

func scorePartner(src, dst *treesitter.ASTNode, srcChildIdx int, isCandidate bool) int {
	score := 0
	dstChildIdx := childIndex(dst)
	if srcChildIdx == dstChildIdx {
		score += 100
	}
	if src.Label == dst.Label && src.Label != "" {
		score += 50
	} else if src.Label != "" && dst.Label != "" {
		if strings.Contains(src.Label, dst.Label) || strings.Contains(dst.Label, src.Label) {
			score += 30
		}
	}
	return score
}

func childIndex(n *treesitter.ASTNode) int {
	if n == nil || n.Parent == nil {
		return -1
	}
	for i, c := range n.Parent.Children {
		if c == n {
			return i
		}
	}
	return -1
}

func slicesReverse[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}
