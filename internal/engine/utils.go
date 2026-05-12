package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// Height returns the height of a subtree rooted at n.
// A leaf has height 1.
func Height(n *treesitter.ASTNode) int {
	if n == nil {
		return 0
	}
	if len(n.Children) == 0 {
		return 1
	}
	max := 0
	for _, c := range n.Children {
		if h := Height(c); h > max {
			max = h
		}
	}
	return max + 1
}

// Descendants returns all nodes in the subtree rooted at n,
// excluding n itself.
func Descendants(n *treesitter.ASTNode) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range n.Children {
		out = append(out, c)
		out = append(out, Descendants(c)...)
	}
	return out
}

// s(t) - the set of descendants of t used by the dice formula.
func descendantSet(n *treesitter.ASTNode) map[*treesitter.ASTNode]struct{} {
	s := make(map[*treesitter.ASTNode]struct{})
	for _, d := range Descendants(n) {
		s[d] = struct{}{}
	}
	return s
}

// Dice computes the Dice similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// dice(t1, t2, m) = 2 × |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / (|s(t1)| + |s(t2)|)
func Dice(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	s1 := Descendants(t1)
	s2 := descendantSet(t2)

	denom := float64(len(s1) + len(Descendants(t2)))
	if denom == 0 {
		return 0
	}

	common := 0
	for _, d := range s1 {
		if mapped, ok := m[d]; ok {
			if _, inS2 := s2[mapped]; inS2 {
				common++
			}
		}
	}
	return 2.0 * float64(common) / denom
}

// TODO: replace the O(n) algorithm with O(1) hash comparison from
// M. Chilowicz, E. Duris, and G. Roussel. Syntax tree
// fingerprinting for source code similarity detection.
//
// Isomorphic returns true when two subtrees are structurally
// and label-wise identical.
func Isomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type || a.Label != b.Label || len(a.Children) != len(b.Children) {
		return false
	}

	for i := range a.Children {
		if !Isomorphic(a.Children[i], b.Children[i]) {
			return false
		}
	}

	return true
}
