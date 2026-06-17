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

// MatchUnmatchedLeaves performs a final greedy pass to map any unmatched leaf nodes
// that have the exact same type and label, using the Dice coefficient of their
// parents to break ties and select the most structurally similar context.
func MatchUnmatchedLeaves(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	for _, t1 := range PostOrder(t1Root) {
		if m.Has(t1) || len(t1.Children) > 0 || t1.Label == "" {
			continue
		}

		var bestT2 *treesitter.ASTNode
		bestDice := 0.0

		for _, t2 := range PostOrder(t2Root) {
			if m.HasDst(t2) || t2.Type != t1.Type || t2.Label != t1.Label || len(t2.Children) > 0 {
				continue
			}

			d := 0.0
			if t1.Parent != nil && t2.Parent != nil {
				d = Dice(t1.Parent, t2.Parent, m.Src())
			}

			if d > bestDice {
				bestDice = d
				bestT2 = t2
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
	FprintMappings(os.Stdout, r)
}

func FprintMappings(w io.Writer, r *MatchResult) {
	if r == nil || r.Mappings == nil {
		fmt.Fprintln(w, "(no mappings)")
		return
	}

	pairs := r.Mappings.Pairs
	if len(pairs) == 0 {
		fmt.Fprintln(w, "(no mappings found)")
		return
	}
	fmt.Fprintf(w, "%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label")
	fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────────────────────────")
	for i, p := range pairs {
		t1Label := p.Src.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := p.Dst.Label
		if t2Label == "" {
			t2Label = "-"
		}
		fmt.Fprintf(w, "%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i+1, p.Src.Type, t1Label, p.Dst.Type, t2Label)
	}
	fmt.Fprintf(w, "\nTotal mappings: %d\n", len(pairs))
}
