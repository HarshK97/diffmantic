package engine

import (
	"fmt"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode, minHeight int) *MatchResult {
	mappings, _ := TopDown(t1, t2, minHeight)
	return &MatchResult{Mappings: mappings}
}

func PrintMappings(r *MatchResult) {
	if r == nil || r.Mappings == nil {
		fmt.Println("(no mappings)")
		return
	}

	src := r.Mappings.Src()
	if len(src) == 0 {
		fmt.Println("(no mappings found)")
		return
	}

	fmt.Printf("%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────────")

	i := 1
	for t1, t2 := range src {
		t1Label := t1.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := t2.Label
		if t2Label == "" {
			t2Label = "-"
		}
		fmt.Printf("%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i, t1.Type, t1Label, t2.Type, t2Label)
		i++
	}
	fmt.Printf("\nTotal mappings: %d\n", len(src))
}
