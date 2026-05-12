package engine

import (
	"fmt"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode) *MatchResult {
	minHeight := 1
	mappings := TopDown(t1, t2, minHeight)
	return &MatchResult{Mappings: mappings}
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
