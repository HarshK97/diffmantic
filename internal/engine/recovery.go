package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// Recover aligns unmatched nodes inside a matched container pair.
// Uses Zhang-Shasha (1989) tree edit distance as a last-chance fallback on tiny
// subtrees (< 20 nodes), and uses SimpleRecovery (Sibling LCS) on larger subtrees.
func Recover(t1, t2 *treesitter.ASTNode, m *Mapping) {
	if t1.Size() < 20 || t2.Size() < 20 {
		RunZSRecovery(t1, t2, m)
	} else {
		SimpleRecovery(t1, t2, m)
	}
}

// SimpleRecovery maps unmatched children inside container pair (t1, t2) using
// label LCS, structural LCS, and unique-type matching.
func SimpleRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	uc1 := unmatchedChildren(t1, m.Has)
	uc2 := unmatchedChildren(t2, m.HasDst)

	for _, pair := range LCSLabel(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	for _, pair := range LCSStructure(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	for _, pair := range uniqueTypePairs(uc1, uc2) {
		m.Add(pair[0], pair[1])
		Recover(pair[0], pair[1], m)
	}
}

func unmatchedChildren(t *treesitter.ASTNode, hasFn func(*treesitter.ASTNode) bool) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !hasFn(c) {
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
