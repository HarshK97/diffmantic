package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// SimpleRecovery maps unmatched children inside container pair (t1, t2) using
// label LCS, structural LCS, and unique-type matching.
func SimpleRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	uc1 := unmatchedChildrenSrc(t1, m)
	uc2 := unmatchedChildrenDst(t2, m)

	for _, pair := range LCSLabel(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildrenSrc(t1, m)
	uc2 = unmatchedChildrenDst(t2, m)

	for _, pair := range LCSStructure(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildrenSrc(t1, m)
	uc2 = unmatchedChildrenDst(t2, m)

	for _, pair := range uniqueTypePairs(uc1, uc2) {
		m.Add(pair[0], pair[1])
		SimpleRecovery(pair[0], pair[1], m)
	}
}

func unmatchedChildrenSrc(t *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !m.Has(c) {
			if IsTrivialLeaf(c) {
				continue
			}
			out = append(out, c)
		}
	}
	return out
}

func unmatchedChildrenDst(t *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !m.HasDst(c) {
			if IsTrivialLeaf(c) {
				continue
			}
			out = append(out, c)
		}
	}
	return out
}

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

	seen := make(map[string]bool)
	var pairs [][2]*treesitter.ASTNode
	for _, c := range uc1 {
		typ := c.Type
		if seen[typ] {
			continue
		}
		seen[typ] = true
		nodes1 := count1[typ]
		nodes2 := count2[typ]
		if len(nodes1) == 1 && len(nodes2) == 1 {
			pairs = append(pairs, [2]*treesitter.ASTNode{nodes1[0], nodes2[0]})
		}
	}
	return pairs
}
