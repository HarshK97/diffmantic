package engine

import (
	"fmt"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode) *MatchResult {
	minHeight := 1
	minDice := 0.5
	mappings := TopDown(t1, t2, minHeight)
	BottomUp(t1, t2, mappings, minDice)

	sortMappingsByPreOrder(t1, mappings)

	return &MatchResult{Mappings: mappings}
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
	if r == nil || r.Mappings == nil {
		fmt.Println("(no mappings)")
		return
	}

	pairs := r.Mappings.Pairs
	if len(pairs) == 0 {
		fmt.Println("(no mappings found)")
		return
	}

	fmt.Printf("%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────")

	for i, p := range pairs {
		t1Label := p.Src.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := p.Dst.Label
		if t2Label == "" {
			t2Label = "-"
		}
		fmt.Printf("%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i+1, p.Src.Type, t1Label, p.Dst.Type, t2Label)
	}
	fmt.Printf("\nTotal mappings: %d\n", len(pairs))
}
