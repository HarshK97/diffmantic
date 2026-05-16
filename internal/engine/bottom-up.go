package engine

import (
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// BottomUp implements the GumTree bottom-up matching phase.
//
// It walks T1 in post-order and for each unmatched node that has at
// least one matched child, it finds the best candidate in T2 (same
// type, highest dice) and adds the pair to the mapping. Then it runs
// SimpleRecovery to map descendants inside the newly matched container.
//
// Parameters:
//
//	t1Root  - root of source tree T1
//	t2Root  - root of destination tree T2
//	m       - existing mapping from the top-down phase (modified in place)
//	minDice - minimum dice coefficient to accept a candidate
func BottomUp(
	t1Root, t2Root *treesitter.ASTNode,
	m *Mapping,
	minDice float64,
) {
	for _, t1 := range PostOrder(t1Root) {
		if m.Has(t1) {
			continue
		}

		if !hasMatchedChild(t1, m) {
			continue
		}

		t2 := candidate(t1, t2Root, m)
		if t2 == nil {
			continue
		}

		if m.DiceSrc(t1, t2) >= minDice {
			m.Add(t1, t2)
			// TODO: Use Hybrid Approach for recovery
			// - subtree size < maxSize → run optimal (RTED) -> To be implemented
			// - subtree size ≥ maxSize → run simple recovery
			SimpleRecovery(t1, t2, m)
		}
	}
}

// hasMatchedChild returns true if any direct child of t1 is in the
// src side of the mapping.
func hasMatchedChild(t1 *treesitter.ASTNode, m *Mapping) bool {
	return slices.ContainsFunc(t1.Children, func(c *treesitter.ASTNode) bool {
		return m.Has(c)
	})
}

// candidate finds the best unmatched node in T2 to pair with t1.
// Among all valid candidates, we return the one with the highest dice.
func candidate(
	t1 *treesitter.ASTNode,
	t2Root *treesitter.ASTNode,
	m *Mapping,
) *treesitter.ASTNode {
	s1 := descendantSet(t1)

	var best *treesitter.ASTNode
	bestDice := -1.0
	bestLabelScore := -1

	t1Labels := leafLabels(t1)

	for _, c := range PostOrder(t2Root) {
		if c.Type != t1.Type {
			continue
		}
		if m.HasDst(c) {
			continue
		}

		if !hasCommonDescendant(s1, c, m) {
			continue
		}

		d := Dice(t1, c, m.Src())
		if d > bestDice {
			bestDice = d
			best = c
			bestLabelScore = labelOverlap(t1Labels, c)
		} else if d == bestDice {
			// Tie-break: prefer candidate with more matching leaf labels.
			ls := labelOverlap(t1Labels, c)
			if ls > bestLabelScore {
				best = c
				bestLabelScore = ls
			}
		}
	}
	return best
}

// leafLabels collects all non-empty labels from leaf nodes in the subtree.
func leafLabels(n *treesitter.ASTNode) map[string]int {
	labels := make(map[string]int)
	for _, d := range Descendants(n) {
		if len(d.Children) == 0 && d.Label != "" {
			labels[d.Label]++
		}
	}
	if len(n.Children) == 0 && n.Label != "" {
		labels[n.Label]++
	}
	return labels
}

// labelOverlap counts how many leaf labels in t2's subtree also appear in t1Labels.
func labelOverlap(t1Labels map[string]int, t2 *treesitter.ASTNode) int {
	count := 0
	for _, d := range Descendants(t2) {
		if len(d.Children) == 0 && d.Label != "" {
			if t1Labels[d.Label] > 0 {
				count++
			}
		}
	}
	if len(t2.Children) == 0 && t2.Label != "" {
		if t1Labels[t2.Label] > 0 {
			count++
		}
	}
	return count
}

// hasCommonDescendant returns true if some descendant of c (in T2) is
// mapped to a descendant of t1 (in T1).
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
