package engine

import (
	"math"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// BottomUp pairs unmatched container nodes in post-order when they share matched
// children, running recovery on newly matched blocks.
func BottomUp(
	t1Root, t2Root *treesitter.ASTNode,
	m *Mapping,
	minDice float64,
) {
	for _, t1 := range t1Root.PostOrder() {
		if t1 == t1Root {
			if !m.Has(t1) && !m.HasDst(t2Root) {
				m.Add(t1, t2Root)
			}
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChild(t1, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(t1, t2, m)
			}
			break
		}

		if m.Has(t1) {
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChild(t1, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(t1, t2, m)
			}
			continue
		}

		if len(t1.Children) == 0 {
			continue
		}

		if !hasMatchedChild(t1, m) {
			continue
		}

		candidates := findCandidatesWithCommonDescendants(t1, m)
		t2 := candidate(t1, candidates, m)
		if t2 == nil {
			continue
		}

		sim := ChawatheSimilarity(t1, t2, m.Src())
		threshold := 1.0 / (1.0 + math.Log(float64((t1.Size()-1)+(t2.Size()-1))))
		if m.DiceSrc(t1, t2) >= minDice || sim >= threshold {
			m.Add(t1, t2)
			Recover(t1, t2, m)
		}
	}
}

func findCandidatesWithCommonDescendants(t1 *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	candMap := make(map[*treesitter.ASTNode]bool)
	var cands []*treesitter.ASTNode

	for _, d1 := range t1.Descendants() {
		if d2, ok := m.Src()[d1]; ok {
			for anc := d2.Parent; anc != nil; anc = anc.Parent {
				if anc.Type == t1.Type && !m.HasDst(anc) {
					if !candMap[anc] {
						candMap[anc] = true
						cands = append(cands, anc)
					}
				}
			}
		}
	}
	return cands
}

func hasUnmappedChild(t *treesitter.ASTNode, hasFn func(*treesitter.ASTNode) bool) bool {
	for _, c := range t.Children {
		if !hasFn(c) {
			return true
		}
	}
	return false
}

func hasMatchedChild(t1 *treesitter.ASTNode, m *Mapping) bool {
	return slices.ContainsFunc(t1.Children, func(c *treesitter.ASTNode) bool {
		return m.Has(c)
	})
}

// candidate finds the best unmatched node in T2 to pair with t1.
// Prefers same child position, higher Chawathe similarity, then Dice score.
func candidate(
	t1 *treesitter.ASTNode,
	candidates []*treesitter.ASTNode,
	m *Mapping,
) *treesitter.ASTNode {
	var best *treesitter.ASTNode
	bestSim := -1.0
	bestDice := -1.0
	bestLabelScore := -1
	var bestSamePositional bool
	var bestKeyMatched bool

	t1Labels := t1.LeafLabels()
	t1Key := getKeyLabel(t1)

	for _, c := range candidates {
		if m.HasDst(c) {
			continue
		}

		if !hasCommonDescendant(t1, c, m) {
			continue
		}

		sim := ChawatheSimilarity(t1, c, m.Src())
		d := Dice(t1, c, m.Src())

		samePositional := false
		if t1.Parent != nil && c.Parent != nil {
			t1Idx := t1.ChildIndex()
			cIdx := c.ChildIndex()
			samePositional = t1Idx == cIdx
		}

		anc1 := NearestMatchedAncestor(t1, m, false)
		anc2 := NearestMatchedAncestor(c, m, true)
		cMatches := areAncestorsMatched(anc1, anc2, m)

		var ancBest2 *treesitter.ASTNode
		if best != nil {
			ancBest2 = NearestMatchedAncestor(best, m, true)
		}
		bestCMatches := best == nil || areAncestorsMatched(anc1, ancBest2, m)

		diff := sim - bestSim
		isBetter := false
		ls := labelOverlap(t1Labels, c)
		cKey := getKeyLabel(c)
		keyMatched := t1Key != "" && cKey != "" && t1Key == cKey

		if t1Key != "" && keyMatched != bestKeyMatched && t1.IsUnordered {
			if keyMatched {
				isBetter = true
			}
		} else if math.Abs(diff) > 0.05 {
			if sim > bestSim {
				isBetter = true
			}
		} else {
			if samePositional && !bestSamePositional {
				isBetter = true
			} else if sim > bestSim {
				isBetter = true
			} else if sim == bestSim {
				if d > bestDice {
					isBetter = true
				} else if d == bestDice {
					if cMatches && !bestCMatches {
						isBetter = true
					} else if cMatches == bestCMatches {
						if ls > bestLabelScore {
							isBetter = true
						}
					}
				}
			}
		}

		if isBetter {
			bestSim = sim
			bestDice = d
			best = c
			bestLabelScore = ls
			bestSamePositional = samePositional
			bestKeyMatched = keyMatched
		}
	}
	return best
}

func getKeyLabel(n *treesitter.ASTNode) string {
	if n == nil || len(n.Children) == 0 {
		return ""
	}
	k := n.Children[0]
	if k.Label != "" {
		return k.Label
	}
	for _, desc := range k.Descendants() {
		if desc.Label != "" {
			return desc.Label
		}
	}
	return ""
}

// labelOverlap returns the number of shared leaf labels in t2's subtree.
func labelOverlap(t1Labels map[string]int, t2 *treesitter.ASTNode) int {
	count := 0
	for label := range t1Labels {
		count += t2.FrequencyInSubtree(label)
	}
	return count
}

func hasCommonDescendant(
	t1 *treesitter.ASTNode,
	c *treesitter.ASTNode,
	m *Mapping,
) bool {
	for _, d := range c.Descendants() {
		if t1Partner, ok := m.Dst()[d]; ok && t1.Contains(t1Partner) {
			return true
		}
	}
	return false
}

// RollupMatchedContainers pairs unmatched container nodes in post-order when their
// mapped children predominantly belong to the same unmatched parent container in T2.
func RollupMatchedContainers(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	for _, t1 := range t1Root.PostOrder() {
		if m.Has(t1) || len(t1.Children) == 0 {
			continue
		}

		parentCounts := make(map[*treesitter.ASTNode]int)
		var bestParent *treesitter.ASTNode
		bestCount := 0
		for _, c := range t1.Children {
			if c2, ok := m.Src()[c]; ok && c2.Parent != nil && !m.HasDst(c2.Parent) && c2.Parent.Type == t1.Type {
				cnt := parentCounts[c2.Parent] + 1
				parentCounts[c2.Parent] = cnt
				if cnt > bestCount {
					bestCount = cnt
					bestParent = c2.Parent
				}
			}
		}

		if bestParent != nil {
			m.Add(t1, bestParent)
		}
	}
}

// ContestContainers reassigns T2 containers that got greedily claimed by an inner T1 node
// during bottom-up matching back to their proper outer T1 parent.
//
// Example: deleting an if-block can trick BottomUp into matching the if's inner block
// to the outer function body (if they share something like a throw). That leaves the
// real function body unmapped and generates a mess of spurious Move actions.
func ContestContainers(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	dstMap := m.Dst()

	for _, t2 := range t2Root.PostOrder() {
		if !m.HasDst(t2) || len(t2.Children) == 0 || t2.Parent == nil {
			continue
		}

		currentT1 := dstMap[t2]

		t1MappedParent := dstMap[t2.Parent]
		if t1MappedParent == nil {
			continue
		}

		// Already at the expected depth under the mapped parent — nothing to fix.
		if currentT1.Parent == t1MappedParent {
			continue
		}

		// T1 sits deeper than expected. Look for an unmapped sibling at the expected depth.
		for _, candidate := range t1MappedParent.Children {
			if candidate.Type != t2.Type || m.Has(candidate) {
				continue
			}
			if !candidate.Contains(currentT1) {
				continue
			}

			m.Remove(currentT1)
			m.Add(candidate, t2)

			if hasUnmappedChild(candidate, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(candidate, t2, m)
			}
			break
		}
	}
}
