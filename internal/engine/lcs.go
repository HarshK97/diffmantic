package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

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
				dp[i][j] = dp[i-1][j-1] + 1
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
		} else if dp[i-1][j] > dp[i][j-1] {
			i--
		} else {
			j--
		}
	}

	for l, r := 0, len(pairs)-1; l < r; l, r = l+1, r-1 {
		pairs[l], pairs[r] = pairs[r], pairs[l]
	}
	return pairs
}

// isomorphic equality (type + label + structure).
func LCSLabel(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	return lcs(seq1, seq2, Isomorphic)
}

// structure-only isomorphism (type + shape, ignoring leaf labels).
func LCSStructure(seq1, seq2 []*treesitter.ASTNode) [][2]*treesitter.ASTNode {
	return lcs(seq1, seq2, StructureIsomorphic)
}
