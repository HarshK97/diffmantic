package engine

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

const maxDataNestingDepth = 128

// MatchDataContainers aligns key-value pairs in dictionaries and markup trees.
// Matches identical keys first, then pairs up unmatched entries with unique
// identical values to detect key renames.
func MatchDataContainers(t1Root, t2Root *treesitter.ASTNode, m *Mapping, r *rules.Rules) {
	if t1Root == nil || t2Root == nil || m == nil || r == nil {
		return
	}

	if !m.Has(t1Root) && !m.HasDst(t2Root) && TypesMatch(t1Root.Type, t2Root.Type, r) {
		m.Add(t1Root, t2Root)
	}

	var matchContainersTopDown func(c1, c2 *treesitter.ASTNode, depth int)
	matchContainersTopDown = func(c1, c2 *treesitter.ASTNode, depth int) {
		if c1 == nil || c2 == nil || depth > maxDataNestingDepth {
			return
		}
		if hasPairChildren(c1, r) && hasPairChildren(c2, r) {
			matchContainerPairs(c1, c2, m, r, depth+1)
		}

		// Step through single-child wrappers (like document -> object or wrapper -> mapping).
		if len(c1.Children) == 1 && len(c2.Children) == 1 {
			ch1 := c1.Children[0]
			ch2 := c2.Children[0]
			if !r.IsPair(ch1.Type) && !r.IsPair(ch2.Type) && TypesMatch(ch1.Type, ch2.Type, r) {
				if !m.Has(ch1) && !m.HasDst(ch2) {
					m.Add(ch1, ch2)
				}
				matchContainersTopDown(ch1, ch2, depth+1)
			}
		} else if isContainerNode(c1, r) && isContainerNode(c2, r) {
			matchNestedContainers(c1, c2, m, r, depth+1)
		}
	}

	matchContainersTopDown(t1Root, t2Root, 0)
}

func matchContainerPairs(c1, c2 *treesitter.ASTNode, m *Mapping, r *rules.Rules, depth int) {
	if c1 == nil || c2 == nil || m == nil || r == nil || depth > maxDataNestingDepth {
		return
	}

	pairs1 := getPairChildren(c1, r)
	pairs2 := getPairChildren(c2, r)
	if len(pairs1) == 0 && len(pairs2) == 0 {
		return
	}

	// Match pairs with identical keys first.
	keyIndex2 := make(map[string][]*treesitter.ASTNode, len(pairs2))
	for _, p2 := range pairs2 {
		if m.HasDst(p2) {
			continue
		}
		k2 := Unquote(getKeyLabel(p2))
		if k2 != "" {
			keyIndex2[k2] = append(keyIndex2[k2], p2)
		}
	}

	for _, p1 := range pairs1 {
		if m.Has(p1) {
			continue
		}
		k1 := Unquote(getKeyLabel(p1))
		if k1 == "" {
			continue
		}
		candidates := keyIndex2[k1]
		var p2 *treesitter.ASTNode
		for _, cand := range candidates {
			if !m.HasDst(cand) {
				p2 = cand
				break
			}
		}
		if p2 == nil {
			continue
		}

		m.Add(p1, p2)
		if len(p1.Children) > 0 && len(p2.Children) > 0 {
			mapKeyNodes(p1.Children[0], p2.Children[0], m, r)
		}

		val1 := getPairValue(p1)
		val2 := getPairValue(p2)
		if val1 != nil && val2 != nil {
			if Isomorphic(val1, val2) {
				addIsomorphicPairs(val1, val2, m)
			} else if TypesMatch(val1.Type, val2.Type, r) {
				if !m.Has(val1) && !m.HasDst(val2) {
					m.Add(val1, val2)
				}
				if hasPairChildren(val1, r) && hasPairChildren(val2, r) {
					matchContainerPairs(val1, val2, m, r, depth+1)
				} else if isContainerNode(val1, r) && isContainerNode(val2, r) {
					matchNestedContainers(val1, val2, m, r, depth+1)
				}
			}
		}
	}

	// For remaining unmatched pairs, look for identical values to detect key renames.
	uSrc := make([]*treesitter.ASTNode, 0, len(pairs1))
	for _, p1 := range pairs1 {
		if !m.Has(p1) {
			uSrc = append(uSrc, p1)
		}
	}

	uDst := make([]*treesitter.ASTNode, 0, len(pairs2))
	for _, p2 := range pairs2 {
		if !m.HasDst(p2) {
			uDst = append(uDst, p2)
		}
	}

	if len(uSrc) == 0 || len(uDst) == 0 {
		return
	}

	type dstValEntry struct {
		pair *treesitter.ASTNode
		val  *treesitter.ASTNode
	}
	valHashMap := make(map[uint64][]dstValEntry, len(uDst))
	for _, p2 := range uDst {
		v2 := getPairValue(p2)
		if v2 != nil && !isTrivialValue(v2) {
			if v2.Hash == 0 {
				v2.ComputeHashes()
			}
			valHashMap[v2.Hash] = append(valHashMap[v2.Hash], dstValEntry{pair: p2, val: v2})
		}
	}

	for _, p1 := range uSrc {
		if m.Has(p1) {
			continue
		}
		v1 := getPairValue(p1)
		if v1 == nil || isTrivialValue(v1) {
			continue
		}
		if v1.Hash == 0 {
			v1.ComputeHashes()
		}
		matches := valHashMap[v1.Hash]
		// Only rename if the value is unique in this container.
		if len(matches) == 1 {
			entry := matches[0]
			p2 := entry.pair
			v2 := entry.val
			if !m.HasDst(p2) && Isomorphic(v1, v2) {
				m.Add(p1, p2)
				if len(p1.Children) > 0 && len(p2.Children) > 0 {
					mapKeyNodes(p1.Children[0], p2.Children[0], m, r)
				}
				addIsomorphicPairs(v1, v2, m)
			}
		}
	}

	// If exactly one pair is left on each side and their structures match, pair them up.
	remainingSrc := make([]*treesitter.ASTNode, 0, len(uSrc))
	for _, p1 := range uSrc {
		if !m.Has(p1) {
			remainingSrc = append(remainingSrc, p1)
		}
	}
	remainingDst := make([]*treesitter.ASTNode, 0, len(uDst))
	for _, p2 := range uDst {
		if !m.HasDst(p2) {
			remainingDst = append(remainingDst, p2)
		}
	}

	if len(remainingSrc) == 1 && len(remainingDst) == 1 {
		p1 := remainingSrc[0]
		p2 := remainingDst[0]
		if TypesMatch(p1.Type, p2.Type, r) && !m.Has(p1) && !m.HasDst(p2) {
			v1 := getPairValue(p1)
			v2 := getPairValue(p2)
			canMatch := false
			if v1 == nil && v2 == nil {
				canMatch = true
			} else if v1 != nil && v2 != nil && TypesMatch(v1.Type, v2.Type, r) {
				if Isomorphic(v1, v2) || StructureIsomorphic(v1, v2) {
					canMatch = true
				}
			}

			if canMatch {
				m.Add(p1, p2)
				if len(p1.Children) > 0 && len(p2.Children) > 0 {
					mapKeyNodes(p1.Children[0], p2.Children[0], m, r)
				}
				if v1 != nil && v2 != nil && !m.Has(v1) && !m.HasDst(v2) {
					if Isomorphic(v1, v2) {
						addIsomorphicPairs(v1, v2, m)
					} else {
						m.Add(v1, v2)
						if hasPairChildren(v1, r) && hasPairChildren(v2, r) {
							matchContainerPairs(v1, v2, m, r, depth+1)
						} else if isContainerNode(v1, r) && isContainerNode(v2, r) {
							matchNestedContainers(v1, v2, m, r, depth+1)
						}
					}
				}
			}
		}
	}
}

func matchNestedContainers(v1, v2 *treesitter.ASTNode, m *Mapping, r *rules.Rules, depth int) {
	if v1 == nil || v2 == nil || m == nil || r == nil || depth > maxDataNestingDepth {
		return
	}

	// Unwrap single-child wrappers before matching children.
	w1 := unwrapSingleNode(v1, r)
	w2 := unwrapSingleNode(v2, r)
	if w1 != v1 || w2 != v2 {
		if TypesMatch(w1.Type, w2.Type, r) && !m.Has(w1) && !m.HasDst(w2) {
			m.Add(w1, w2)
			if hasPairChildren(w1, r) && hasPairChildren(w2, r) {
				matchContainerPairs(w1, w2, m, r, depth+1)
				return
			}
		}
		v1, v2 = w1, w2
	}

	// Pass 1: Isomorphic matches anywhere in the sequence (preserves permuted/moved elements).
	matchedDst := make([]bool, len(v2.Children))
	matchedSrc := make([]bool, len(v1.Children))
	for i, c1 := range v1.Children {
		if m.Has(c1) {
			matchedSrc[i] = true
			continue
		}
		for j, c2 := range v2.Children {
			if matchedDst[j] || m.HasDst(c2) {
				continue
			}
			if TypesMatch(c1.Type, c2.Type, r) && Isomorphic(c1, c2) {
				addIsomorphicPairs(c1, c2, m)
				matchedSrc[i] = true
				matchedDst[j] = true
				break
			}
		}
	}
	// Pass 2: Positional pairing for remaining unmatched elements if lengths match.
	if len(v1.Children) == len(v2.Children) {
		for i := range v1.Children {
			if matchedSrc[i] || matchedDst[i] {
				continue
			}
			c1 := v1.Children[i]
			c2 := v2.Children[i]
			if TypesMatch(c1.Type, c2.Type, r) && !m.Has(c1) && !m.HasDst(c2) {
				if hasPairChildren(c1, r) && hasPairChildren(c2, r) {
					m.Add(c1, c2)
					matchContainerPairs(c1, c2, m, r, depth+1)
				} else if isContainerNode(c1, r) && isContainerNode(c2, r) {
					matchNestedContainers(c1, c2, m, r, depth+1)
				}
			}
		}
	}
}

func unwrapSingleNode(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	curr := n
	steps := 0
	for curr != nil && steps < 64 && len(curr.Children) == 1 && (r.IsWrapper(curr.Type) || r.IsScaffolding(curr.Type)) {
		curr = curr.Children[0]
		steps++
	}
	return curr
}

func mapKeyNodes(k1, k2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) {
	if k1 == nil || k2 == nil || m == nil {
		return
	}
	if !m.Has(k1) && !m.HasDst(k2) && (r == nil || TypesMatch(k1.Type, k2.Type, r)) {
		m.Add(k1, k2)
	}
	if len(k1.Children) > 0 && len(k2.Children) > 0 && len(k1.Children) == len(k2.Children) {
		for i := range k1.Children {
			mapKeyNodes(k1.Children[i], k2.Children[i], m, r)
		}
	}
}

func getPairValue(p *treesitter.ASTNode) *treesitter.ASTNode {
	if p == nil || len(p.Children) < 2 {
		return nil
	}
	return p.Children[len(p.Children)-1]
}

func hasPairChildren(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || r == nil || len(r.Pairs) == 0 {
		return false
	}
	for _, c := range n.Children {
		if r.IsPair(c.Type) {
			return true
		}
	}
	return false
}

func getPairChildren(n *treesitter.ASTNode, r *rules.Rules) []*treesitter.ASTNode {
	if n == nil || r == nil {
		return nil
	}
	pairs := make([]*treesitter.ASTNode, 0, len(n.Children))
	for _, c := range n.Children {
		if r.IsPair(c.Type) {
			pairs = append(pairs, c)
		}
	}
	return pairs
}

func isContainerNode(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || r == nil {
		return false
	}
	if r.IsUnordered(n.Type) || r.IsWrapper(n.Type) || r.IsScaffolding(n.Type) || r.IsBlock(n.Type) {
		return true
	}
	return hasPairChildren(n, r)
}

func isTrivialValue(val *treesitter.ASTNode) bool {
	if val == nil {
		return true
	}
	curr := val
	for len(curr.Children) == 1 {
		curr = curr.Children[0]
	}
	if len(curr.Children) == 0 {
		lbl := strings.TrimSpace(Unquote(curr.Label))
		switch strings.ToLower(lbl) {
		case "", "true", "false", "null", "nil", "none", "undefined", "0", "{}", "[]", "\"\"":
			return true
		}
		if len(lbl) <= 1 {
			return true
		}
	}
	return false
}
