package engine

import (
	"cmp"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// enclosingDelimitedContainer returns the nearest ancestor of n that is a delimited container
// (such as a struct literal, array, argument list, or dictionary), halting at declarations.
func enclosingDelimitedContainer(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r = cmp.Or(r, rulesFor(n))
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		if (r != nil && r.IsDelimitedContainer(curr.Type)) || (r == nil && rules.IsDelimitedContainer(curr.Type)) {
			return curr
		}
		if (r != nil && r.IsDeclaration(curr.Type)) || (r == nil && rules.IsDeclaration(curr.Type)) {
			return nil
		}
	}
	return nil
}

// hasDelimitedContainerConflict reports whether matching t1 to t2 would introduce a conflict
// across delimited container boundaries or steal a target element from a superior peer.
func hasDelimitedContainerConflict(t1, t2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) bool {
	if t1 == nil || t2 == nil || m == nil {
		return false
	}
	r = cmp.Or(r, rulesFor(t1), rulesFor(t2))

	var elem1, elem2 *treesitter.ASTNode
	if r != nil && r.IsDelimitedContainer(t1.Type) && r.IsDelimitedContainer(t2.Type) {
		elem1, elem2 = t1, t2
	} else {
		elem1 = enclosingDelimitedContainer(t1, r)
		elem2 = enclosingDelimitedContainer(t2, r)
	}
	if elem1 == nil || elem2 == nil || m.Src()[elem1] == elem2 {
		return false
	}
	outer1 := enclosingDelimitedContainer(elem1, r)
	outer2 := enclosingDelimitedContainer(elem2, r)

	// Guard only applies to key-value pairs (struct fields, dict entries) or sibling
	// elements inside the same outer collection.
	isPairMatch := false
	if r != nil {
		isPairMatch = r.IsPair(t1.Type) || r.IsPair(t2.Type) ||
			(t1.Parent != nil && r.IsPair(t1.Parent.Type)) ||
			(t2.Parent != nil && r.IsPair(t2.Parent.Type))
	}

	isSiblingCollectionElements := !isPairMatch && outer1 != nil && outer2 != nil && m.Src()[outer1] == outer2

	if !isPairMatch && !isSiblingCollectionElements {
		return false
	}

	if hasAnyMappedDescendantOutside(elem2, elem1, m.Dst()) || hasAnyMappedDescendantOutside(elem1, elem2, m.Src()) {
		return true
	}

	return hasUnextinguishedPeerConflict(elem1, elem2, outer1, outer2, m, r)
}

// hasAnyMappedDescendantOutside reports whether any descendant of root is mapped
// to a node that is neither targetContainer nor contained within targetContainer.
func hasAnyMappedDescendantOutside(root, targetContainer *treesitter.ASTNode, mapping map[*treesitter.ASTNode]*treesitter.ASTNode) bool {
	if root == nil || targetContainer == nil || len(mapping) == 0 {
		return false
	}
	checkNode := func(d *treesitter.ASTNode) bool {
		partner := mapping[d]
		return partner != nil && partner != targetContainer && !targetContainer.Contains(partner)
	}
	if root.Index != nil && root.PreSize > 1 {
		for _, d := range root.Index.Nodes[root.ID+1 : root.ID+root.PreSize] {
			if checkNode(d) {
				return true
			}
		}
		return false
	}
	for _, d := range root.Descendants() {
		if checkNode(d) {
			return true
		}
	}
	return false
}

// collectContainerPairs appends key-value pair nodes directly within container c
// or nested within wrapper/scaffolding children into dst, without descending into blocks or closures.
func collectContainerPairs(c *treesitter.ASTNode, r *rules.Rules, dst []*treesitter.ASTNode) []*treesitter.ASTNode {
	if c == nil || r == nil {
		return dst
	}
	for _, ch := range c.Children {
		if r.IsPair(ch.Type) {
			dst = append(dst, ch)
		} else if r.IsWrapper(ch.Type) || r.IsScaffolding(ch.Type) {
			for _, sub := range ch.Children {
				if r.IsPair(sub.Type) {
					dst = append(dst, sub)
				}
			}
		}
	}
	return dst
}

// getElementContainer unwraps 1-hop wrappers to find the enclosed delimited container, or returns node itself.
func getElementContainer(node *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if node == nil {
		return nil
	}
	r = cmp.Or(r, rulesFor(node))
	if r != nil && r.IsDelimitedContainer(node.Type) {
		return node
	}
	for _, ch := range node.Children {
		if r != nil && r.IsDelimitedContainer(ch.Type) {
			return ch
		}
	}
	return node
}

// isPairValueMatch reports whether the values of two pairs match by text or tree isomorphism.
func isPairValueMatch(p1, p2 *treesitter.ASTNode) bool {
	if p1 == nil || p2 == nil || len(p1.Children) < 2 || len(p2.Children) < 2 {
		return false
	}
	v1 := p1.Children[len(p1.Children)-1]
	v2 := p2.Children[len(p2.Children)-1]
	t1 := identifierText(v1)
	if t1 != "" && t1 == identifierText(v2) {
		return true
	}
	return Isomorphic(v1, v2)
}

// scorePairMatch tallies key matches, conflicts, and match weight between candidate and target pairs.
func scorePairMatch(fromPairs, toPairs []*treesitter.ASTNode) (matches, conflicts, matchedWeight int) {
	for _, p1 := range fromPairs {
		k1 := getKeyLabel(p1)
		if k1 == "" {
			continue
		}
		for _, p2 := range toPairs {
			if getKeyLabel(p2) == k1 {
				if isPairValueMatch(p1, p2) {
					matches++
					v1 := p1.Children[len(p1.Children)-1]
					matchedWeight += max(1, len(identifierText(v1)))
				} else {
					conflicts++
				}
				break
			}
		}
	}
	return matches, conflicts, matchedWeight
}

// hasUnextinguishedPeerConflict checks if an unmapped sibling in the same collection is a better match
// for the target container than the candidate element.
func hasUnextinguishedPeerConflict(elem1, elem2, outer1, outer2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) bool {
	if elem1 == nil || elem2 == nil || outer1 == nil || r == nil {
		return false
	}
	if outer2 != nil {
		if m.Src()[outer1] != nil && m.Src()[outer1] != outer2 {
			return false
		}
		if m.Dst()[outer2] != nil && m.Dst()[outer2] != outer1 {
			return false
		}
	}
	if checkPeerSuperiority(elem1, elem2, outer1, m.Src(), r) {
		return true
	}
	return outer2 != nil && checkPeerSuperiority(elem2, elem1, outer2, m.Dst(), r)
}

// checkPeerSuperiority checks if any sibling of fromElem within fromOuter is a better match
// for toElem based on key-value pair agreement.
func checkPeerSuperiority(fromElem, toElem, fromOuter *treesitter.ASTNode, mapping map[*treesitter.ASTNode]*treesitter.ASTNode, r *rules.Rules) bool {
	var fromBuf, toBuf [8]*treesitter.ASTNode
	fromPairs := collectContainerPairs(fromElem, r, fromBuf[:0])
	toPairs := collectContainerPairs(toElem, r, toBuf[:0])
	if len(fromPairs) == 0 || len(toPairs) == 0 {
		return false
	}

	fromMatches, fromConflicts, fromWeight := scorePairMatch(fromPairs, toPairs)
	if fromConflicts == 0 {
		return false
	}

	var peerBuf [8]*treesitter.ASTNode
	for _, ch := range fromOuter.Children {
		if r != nil && (r.IsPunctuation(ch.Type) || r.IsIgnored(ch.Type, ch.Label)) {
			continue
		}
		peer := getElementContainer(ch, r)
		if peer == nil || peer == fromElem {
			continue
		}
		if mapped := mapping[peer]; mapped != nil && mapped != toElem {
			continue
		}
		if hasAnyMappedDescendantOutside(peer, toElem, mapping) {
			continue
		}

		peerPairs := collectContainerPairs(peer, r, peerBuf[:0])
		peerMatches, _, peerWeight := scorePairMatch(peerPairs, toPairs)
		if peerMatches > fromMatches || (peerMatches == fromMatches && peerWeight > fromWeight) {
			return true
		}
	}
	return false
}
