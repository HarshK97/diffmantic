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

	denom := float64(len(s1) + len(s2))
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

// ChawatheSimilarity computes the Chawathe similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// chawathe(t1, t2, m) = |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / max(|s(t1)|, |s(t2)|)
func ChawatheSimilarity(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	s1 := Descendants(t1)
	s2 := descendantSet(t2)

	maxDesc := len(s1)
	if len(s2) > maxDesc {
		maxDesc = len(s2)
	}
	if maxDesc == 0 {
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
	return float64(common) / float64(maxDesc)
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

// PostOrder returns all nodes in the subtree rooted at n
// in post-order (children before parent).
func PostOrder(n *treesitter.ASTNode) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range n.Children {
		out = append(out, PostOrder(c)...)
	}
	out = append(out, n)
	return out
}

// PreOrder returns all nodes in the subtree rooted at n
// in pre-order (parent before children). Used for deterministic
// mapping output.
func PreOrder(n *treesitter.ASTNode) []*treesitter.ASTNode {
	out := []*treesitter.ASTNode{n}
	for _, c := range n.Children {
		out = append(out, PreOrder(c)...)
	}
	return out
}

// StructureIsomorphic returns true when two subtrees are structurally
// identical (same Type, same shape) but ignores leaf labels.
//
// TODO: replace with O(1) hash comparison alongside Isomorphic.
func StructureIsomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type || len(a.Children) != len(b.Children) {
		return false
	}

	for i := range a.Children {
		if !StructureIsomorphic(a.Children[i], b.Children[i]) {
			return false
		}
	}

	return true
}

// NearestMatchedAncestor finds the closest ancestor of n that is present in the mapping.
// If isDst is true, it checks m.HasDst; otherwise it checks m.Has.
func NearestMatchedAncestor(n *treesitter.ASTNode, m *Mapping, isDst bool) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	curr := n.Parent
	for curr != nil {
		if isDst {
			if m.HasDst(curr) {
				return curr
			}
		} else {
			if m.Has(curr) {
				return curr
			}
		}
		curr = curr.Parent
	}
	return nil
}

// AncestorNameSimilarity calculates the number of matching identifier labels
// among the ancestors of t1 and t2. This helps break ties in top-down matching
// by preferring pairs located in similarly named functions or classes.
func AncestorNameSimilarity(t1, t2 *treesitter.ASTNode) int {
	if t1 == nil || t2 == nil {
		return 0
	}
	labels1 := make(map[string]bool)
	curr := t1.Parent
	for curr != nil {
		for _, child := range curr.Children {
			if child.Type == "identifier" && child.Label != "" {
				labels1[child.Label] = true
			}
		}
		curr = curr.Parent
	}

	overlap := 0
	curr = t2.Parent
	for curr != nil {
		for _, child := range curr.Children {
			if child.Type == "identifier" && child.Label != "" {
				if labels1[child.Label] {
					overlap++
				}
			}
		}
		curr = curr.Parent
	}
	return overlap
}
