package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// TODO: Implement optimal recovery (RTED) for small subtrees, and use it

// SimpleRecovery recovers additional mappings inside a pair of matched
// container nodes (t1, t2). It is called by BottomUp after a new
// container mapping is established.
//
// Three steps:
//  1. Label-based LCS on unmatched children → map isomorphic subtrees
//  2. Structure-based LCS on remaining unmatched children → map
//     structurally isomorphic subtrees (labels may differ)
//  3. Unique-type pairing on remaining unmatched children → map nodes
//     whose type appears exactly once on each side, then recurse
func SimpleRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	uc1 := unmatchedChildrenSrc(t1, m)
	uc2 := unmatchedChildrenDst(t2, m)

	for _, pair := range LCSLabel(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildrenSrc(t1, m)
	uc2 = unmatchedChildrenDst(t2, m)

	for _, pair := range LCSStructure(uc1, uc2) {
		addDescendantPairsStructure(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildrenSrc(t1, m)
	uc2 = unmatchedChildrenDst(t2, m)

	for _, pair := range uniqueTypePairs(uc1, uc2) {
		m.Add(pair[0], pair[1])
		SimpleRecovery(pair[0], pair[1], m)
	}
}

// unmatchedChildrenSrc returns children of t that are NOT in the src
func unmatchedChildrenSrc(t *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !m.Has(c) {
			out = append(out, c)
		}
	}
	return out
}

// unmatchedChildrenDst returns children of t that are NOT in the dst
func unmatchedChildrenDst(t *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !m.HasDst(c) {
			out = append(out, c)
		}
	}
	return out
}

// uniqueTypePairs finds pairs (ta, tb) where ta.Type == tb.Type and
// that type appears exactly once in each unmatched list.
func uniqueTypePairs(
	uc1, uc2 []*treesitter.ASTNode,
) [][2]*treesitter.ASTNode {
	count1 := make(map[string][]*treesitter.ASTNode)
	count2 := make(map[string][]*treesitter.ASTNode)
	for _, c := range uc1 {
		count1[c.Type] = append(count1[c.Type], c)
	}
	for _, c := range uc2 {
		count2[c.Type] = append(count2[c.Type], c)
	}

	var pairs [][2]*treesitter.ASTNode
	for typ, nodes1 := range count1 {
		nodes2 := count2[typ]
		if len(nodes1) == 1 && len(nodes2) == 1 {
			pairs = append(pairs, [2]*treesitter.ASTNode{nodes1[0], nodes2[0]})
		}
	}
	return pairs
}

// addDescendantPairsStructure maps descendants of two structurally
// isomorphic subtrees by walking both in lockstep.
func addDescendantPairsStructure(t1, t2 *treesitter.ASTNode, m *Mapping) {
	m.Add(t1, t2)
	for i := range t1.Children {
		addDescendantPairsStructure(t1.Children[i], t2.Children[i], m)
	}
}
