package engine

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode) *MatchResult {
	minHeight := 2
	minDice := 0.5
	mappings := TopDown(t1, t2, minHeight)
	BottomUp(t1, t2, mappings, minDice)

	MatchUnmatchedLeaves(t1, t2, mappings)

	if !mappings.Has(t1) && !mappings.HasDst(t2) {
		mappings.Add(t1, t2)
	}

	sortMappingsByPreOrder(t1, mappings)

	return &MatchResult{Mappings: mappings}
}

// MatchUnmatchedLeaves greedily maps remaining leaf nodes with matching type
// and label. Dice of parents picks the best candidate; ties are broken by a
// positional score (parent mapped > parent same slot in matched ancestor >
// same child index > matched ancestor pair).
// Leaves with an unmatched parent are skipped — they belong to deleted or
// inserted subtrees and have no real counterpart.
func MatchUnmatchedLeaves(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	t2Nodes := PostOrder(t2Root)
	for _, t1 := range PostOrder(t1Root) {
		if m.Has(t1) || len(t1.Children) > 0 || t1.Label == "" {
			continue
		}

		// No matched parent → leaf is in a deleted/inserted subtree.
		if t1.Parent != nil && !m.Has(t1.Parent) {
			continue
		}

		var bestT2 *treesitter.ASTNode
		bestDice := 0.0
		bestPosScore := -1

		t1Idx := -1
		if t1.Parent != nil {
			t1Idx = childIndexWithin(t1, t1.Parent)
		}

		for _, t2 := range t2Nodes {
			if m.HasDst(t2) || t2.Type != t1.Type || t2.Label != t1.Label || len(t2.Children) > 0 {
				continue
			}

			// No matched parent on destination leaf -> it belongs to an inserted subtree.
			if t2.Parent != nil && !m.HasDst(t2.Parent) {
				continue
			}

			d := 0.0
			if t1.Parent != nil && t2.Parent != nil {
				d = Dice(t1.Parent, t2.Parent, m.Src())
			}

			anc1 := NearestMatchedAncestor(t1, m, false)
			anc2 := NearestMatchedAncestor(t2, m, true)
			cMatches := (anc1 == nil && anc2 == nil) || (anc1 != nil && anc2 != nil && m.Src()[anc1] == anc2)

			parentMatched := t1.Parent != nil && t2.Parent != nil && m.Src()[t1.Parent] == t2.Parent

			samePositional := false
			if t1.Parent != nil && t2.Parent != nil {
				samePositional = t1Idx == childIndexWithin(t2, t2.Parent)
			}

			// Check if both parents sit at the same index within their
			// nearest matched ancestors (for fixed-position keywords like
			// "if" where samePositional can't tell them apart).
			parentPositional := false
			if cMatches && anc1 != nil && anc2 != nil &&
				t1.Parent != nil && t2.Parent != nil &&
				anc1 != t1.Parent && anc2 != t2.Parent {
				p1Idx := childIndexWithin(t1.Parent, anc1)
				p2Idx := childIndexWithin(t2.Parent, anc2)
				if p1Idx >= 0 && p2Idx >= 0 {
					parentPositional = p1Idx == p2Idx
				}
			}

			posScore := 0
			if parentMatched {
				posScore += 1000
			}
			if parentPositional {
				posScore += 100
			}
			if samePositional {
				posScore += 10
			}
			if cMatches {
				posScore += 1
			}

			isBetter := false
			if d > bestDice {
				isBetter = true
			} else if d == bestDice && d > 0.0 {
				if posScore > bestPosScore {
					isBetter = true
				}
			}

			if isBetter {
				bestDice = d
				bestT2 = t2
				bestPosScore = posScore
			}
		}

		if bestT2 != nil {
			m.Add(t1, bestT2)
		}
	}
}

// sortMappingsByPreOrder sorts m.Pairs by the pre-order index of each
// pair's Src node within the T1 tree.
func sortMappingsByPreOrder(t1Root *treesitter.ASTNode, m *Mapping) {
	nodes := PreOrder(t1Root)
	index := make(map[*treesitter.ASTNode]int, len(nodes))
	for i, n := range nodes {
		index[n] = i
	}
	sort.SliceStable(m.Pairs, func(i, j int) bool {
		return index[m.Pairs[i].Src] < index[m.Pairs[j].Src]
	})
}

func PrintMappings(r *MatchResult) {
	_ = FprintMappings(os.Stdout, r)
}

func FprintMappings(w io.Writer, r *MatchResult) error {
	if r == nil || r.Mappings == nil {
		_, err := fmt.Fprintln(w, "(no mappings)")
		return err
	}

	pairs := r.Mappings.Pairs
	if len(pairs) == 0 {
		_, err := fmt.Fprintln(w, "(no mappings found)")
		return err
	}
	if _, err := fmt.Fprintf(w, "%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────────────────────────"); err != nil {
		return err
	}
	for i, p := range pairs {
		t1Label := p.Src.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := p.Dst.Label
		if t2Label == "" {
			t2Label = "-"
		}
		if _, err := fmt.Fprintf(w, "%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i+1, p.Src.Type, t1Label, p.Dst.Type, t2Label); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nTotal mappings: %d\n", len(pairs))
	return err
}
