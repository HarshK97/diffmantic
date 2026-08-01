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
	// Index T2 nodes by type so candidate lookups aren't O(N).
	t2NodesByType := make(map[string][]*treesitter.ASTNode)
	for _, node := range PostOrder(t2Root) {
		t2NodesByType[node.Type] = append(t2NodesByType[node.Type], node)
	}

	for _, t1 := range PostOrder(t1Root) {
		if t1 == t1Root {
			if !m.Has(t1) && !m.HasDst(t2Root) {
				m.Add(t1, t2Root)
			}
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChildrenSrc(t1, m) && hasUnmappedChildrenDst(t2, m) {
				SimpleRecovery(t1, t2, m)
			}
			break
		}

		if m.Has(t1) {
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChildrenSrc(t1, m) && hasUnmappedChildrenDst(t2, m) {
				SimpleRecovery(t1, t2, m)
			}
			continue
		}

		if len(t1.Children) == 0 {
			continue
		}

		if !hasMatchedChild(t1, m) {
			continue
		}

		t2 := candidate(t1, t2NodesByType[t1.Type], m)
		if t2 == nil {
			continue
		}

		sim := ChawatheSimilarity(t1, t2, m.Src())
		threshold := 1.0 / (1.0 + math.Log(float64(len(Descendants(t1))+len(Descendants(t2)))))
		if m.DiceSrc(t1, t2) >= minDice || sim >= threshold {
			m.Add(t1, t2)
			SimpleRecovery(t1, t2, m)
		}
	}
}

func hasUnmappedChildrenSrc(t *treesitter.ASTNode, m *Mapping) bool {
	for _, c := range t.Children {
		if !m.Has(c) {
			return true
		}
	}
	return false
}

func hasUnmappedChildrenDst(t *treesitter.ASTNode, m *Mapping) bool {
	for _, c := range t.Children {
		if !m.HasDst(c) {
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

// Find the best unmatched node in T2 to pair with t1.
// Breaks ties using same child position, Chawathe similarity, and then Dice.
func candidate(
	t1 *treesitter.ASTNode,
	candidates []*treesitter.ASTNode,
	m *Mapping,
) *treesitter.ASTNode {
	s1 := t1.DescendantSet()

	var best *treesitter.ASTNode
	bestSim := -1.0
	bestDice := -1.0
	bestLabelScore := -1
	var bestSamePositional bool

	t1Labels := t1.LeafLabels()

	for _, c := range candidates {
		if m.HasDst(c) {
			continue
		}

		if !hasCommonDescendant(s1, c, m) {
			continue
		}

		sim := ChawatheSimilarity(t1, c, m.Src())
		d := Dice(t1, c, m.Src())

		samePositional := false
		if t1.Parent != nil && c.Parent != nil {
			t1Idx := childIndexWithin(t1, t1.Parent)
			cIdx := childIndexWithin(c, c.Parent)
			samePositional = t1Idx == cIdx
		}

		anc1 := NearestMatchedAncestor(t1, m, false)
		anc2 := NearestMatchedAncestor(c, m, true)
		cMatches := (anc1 == nil && anc2 == nil) || (anc1 != nil && anc2 != nil && m.Src()[anc1] == anc2)

		ancBest1 := NearestMatchedAncestor(t1, m, false)
		var ancBest2 *treesitter.ASTNode
		if best != nil {
			ancBest2 = NearestMatchedAncestor(best, m, true)
		}
		bestCMatches := best == nil || (ancBest1 == nil && ancBest2 == nil) || (ancBest1 != nil && ancBest2 != nil && m.Src()[ancBest1] == ancBest2)

		diff := sim - bestSim
		isBetter := false
		ls := -1

		if math.Abs(diff) > 0.05 {
			if sim > bestSim {
				isBetter = true
				ls = labelOverlap(t1Labels, c)
			}
		} else {
			if samePositional && !bestSamePositional {
				isBetter = true
				ls = labelOverlap(t1Labels, c)
			} else if sim > bestSim {
				isBetter = true
				ls = labelOverlap(t1Labels, c)
			} else if sim == bestSim {
				if d > bestDice {
					isBetter = true
					ls = labelOverlap(t1Labels, c)
				} else if d == bestDice {
					if cMatches && !bestCMatches {
						isBetter = true
						ls = labelOverlap(t1Labels, c)
					} else if cMatches == bestCMatches {
						ls = labelOverlap(t1Labels, c)
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
		}
	}
	return best
}

// Count how many leaf labels t1 and t2 share.
func labelOverlap(t1Labels map[string]int, t2 *treesitter.ASTNode) int {
	count := 0
	for label, freq2 := range t2.LeafLabels() {
		if t1Labels[label] > 0 {
			count += freq2
		}
	}
	return count
}

func childIndexWithin(child, parent *treesitter.ASTNode) int {
	if parent == nil {
		return -1
	}
	return slices.Index(parent.Children, child)
}

func hasCommonDescendant(
	s1 map[*treesitter.ASTNode]struct{},
	c *treesitter.ASTNode,
	m *Mapping,
) bool {
	for _, d := range Descendants(c) {
		if t1Partner, ok := m.Dst()[d]; ok {
			if _, in := s1[t1Partner]; in {
				return true
			}
		}
	}
	return false
}
