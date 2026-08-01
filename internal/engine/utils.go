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
	maxH := 0
	for _, c := range n.Children {
		maxH = max(maxH, Height(c))
	}
	return maxH + 1
}

// IsTrivialLeaf reports whether a leaf node contains only punctuation (no alphanumeric chars or underscores).
func IsTrivialLeaf(n *treesitter.ASTNode) bool {
	if n == nil || len(n.Children) > 0 || n.Label == "" {
		return false
	}
	for _, c := range n.Label {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			return false
		}
	}
	return true
}

// Descendants returns all nodes in the subtree rooted at n,
// excluding n itself.
func Descendants(n *treesitter.ASTNode) []*treesitter.ASTNode {
	return n.Descendants()
}

func commonMappedDescendants(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) (common, lenS1, lenS2 int) {
	s1 := Descendants(t1)
	s2 := t2.DescendantSet()
	for _, d := range s1 {
		if mapped, ok := m[d]; ok {
			if _, inS2 := s2[mapped]; inS2 {
				common++
			}
		}
	}
	return common, len(s1), len(s2)
}

// Dice computes the Dice similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// dice(t1, t2, m) = 2 × |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / (|s(t1)| + |s(t2)|)
func Dice(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	common, lenS1, lenS2 := commonMappedDescendants(t1, t2, m)
	denom := float64(lenS1 + lenS2)
	if denom == 0 {
		return 0
	}
	return 2.0 * float64(common) / denom
}

// ChawatheSimilarity computes the Chawathe similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// chawathe(t1, t2, m) = |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / max(|s(t1)|, |s(t2)|)
func ChawatheSimilarity(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	common, lenS1, lenS2 := commonMappedDescendants(t1, t2, m)
	maxDesc := max(lenS1, lenS2)
	if maxDesc == 0 {
		return 0
	}
	return float64(common) / float64(maxDesc)
}

// Isomorphic returns true if both subtrees match in type, label, and structure.
func Isomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Hash == 0 {
		a.ComputeHashes()
	}
	if b.Hash == 0 {
		b.ComputeHashes()
	}
	return a.Hash == b.Hash
}

// StructureIsomorphic returns true if both subtrees match in shape and node types, ignoring leaf labels.
func StructureIsomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.StructureHash == 0 {
		a.ComputeHashes()
	}
	if b.StructureHash == 0 {
		b.ComputeHashes()
	}
	return a.StructureHash == b.StructureHash
}

// PostOrder returns all nodes in the subtree rooted at n
// in post-order (children before parent).
func PostOrder(n *treesitter.ASTNode) []*treesitter.ASTNode {
	if n == nil {
		return nil
	}
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
	if n == nil {
		return nil
	}
	out := []*treesitter.ASTNode{n}
	for _, c := range n.Children {
		out = append(out, PreOrder(c)...)
	}
	return out
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
