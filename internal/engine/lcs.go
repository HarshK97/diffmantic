package engine

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type eqFunc func(a, b *treesitter.ASTNode) bool

// LCS computes the longest common subsequence of two node sequences using a custom equality predicate.
func LCS(seq1, seq2 []*treesitter.ASTNode, eq eqFunc) [][2]*treesitter.ASTNode {
	n := len(seq1)
	m := len(seq2)
	if n == 0 || m == 0 {
		return nil
	}

	stride := m + 1
	opt := make([]int, (n+1)*stride)
	for i := 1; i <= n; i++ {
		rowCurr := i * stride
		rowPrev := (i - 1) * stride
		for j := 1; j <= m; j++ {
			if eq(seq1[i-1], seq2[j-1]) {
				weight := seq1[i-1].Size()
				opt[rowCurr+j] = opt[rowPrev+j-1] + weight
			} else {
				opt[rowCurr+j] = max(opt[rowPrev+j], opt[rowCurr+j-1])
			}
		}
	}

	var pairs [][2]*treesitter.ASTNode
	i, j := n, m
	for i > 0 && j > 0 {
		rowCurr := i * stride
		rowPrev := (i - 1) * stride
		if eq(seq1[i-1], seq2[j-1]) {
			pairs = append(pairs, [2]*treesitter.ASTNode{seq1[i-1], seq2[j-1]})
			i--
			j--
		} else if opt[rowPrev+j] >= opt[rowCurr+j-1] {
			i--
		} else {
			j--
		}
	}

	slices.Reverse(pairs)
	return pairs
}

// LCSLabel matches node sequences by type, label, and structure.
func LCSLabel(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	return LCS(seq1, seq2, Isomorphic)
}

// LCSStructure matches node sequences by type and shape (ignoring leaf labels),
// using child position and label similarity to resolve ambiguous matches.
func LCSStructure(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	pairs := LCS(seq1, seq2, StructureIsomorphic)
	if len(pairs) == 0 {
		return pairs
	}

	dstUsed := make(map[*treesitter.ASTNode]bool, len(pairs))
	for _, p := range pairs {
		dstUsed[p[1]] = true
	}

	var filtered [][2]*treesitter.ASTNode
	for _, p := range pairs {
		src := p[0]
		current := p[1]
		best := current
		if hasMultipleStructuralMatches(src, seq2, current) {
			cand := bestStructuralPartner(src, seq2, current, dstUsed)
			if cand != nil {
				scoreCurr := scorePartner(src, current, src.ChildIndex())
				scoreCand := scorePartner(src, cand, src.ChildIndex())
				if scoreCand > scoreCurr {
					dstUsed[current] = false
					dstUsed[cand] = true
					best = cand
				}
			}
		}
		if scorePartner(src, best, src.ChildIndex()) >= 0 {
			filtered = append(filtered, [2]*treesitter.ASTNode{src, best})
		} else {
			dstUsed[best] = false
		}
	}

	return filtered
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
	srcIdx := src.ChildIndex()
	var best *treesitter.ASTNode
	bestScore := scorePartner(src, current, srcIdx)
	for _, d := range seq2 {
		if d == current || dstUsed[d] {
			continue
		}
		if !StructureIsomorphic(src, d) {
			continue
		}
		s := scorePartner(src, d, srcIdx)
		if s > bestScore {
			bestScore = s
			best = d
		}
	}
	return best
}

func scorePartner(src, dst *treesitter.ASTNode, srcChildIdx int) int {
	if (HasLongLeafToken(src) || HasLongLeafToken(dst)) && LeafSimilarity(src, dst) == 0 {
		return -100
	}
	score := 0
	dstChildIdx := dst.ChildIndex()
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
