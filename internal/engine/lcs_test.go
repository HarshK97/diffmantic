package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestLCSLabelIdentical(t *testing.T) {
	a := []*treesitter.ASTNode{mkLeaf("id", "x"), mkLeaf("id", "y")}
	b := []*treesitter.ASTNode{mkLeaf("id", "x"), mkLeaf("id", "y")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 2 {
		t.Errorf("identical seqs: want 2 pairs, got %d", len(pairs))
	}
}

func TestLCSLabelPartial(t *testing.T) {
	a := []*treesitter.ASTNode{mkLeaf("id", "x"), mkLeaf("id", "y"), mkLeaf("id", "z")}
	b := []*treesitter.ASTNode{mkLeaf("id", "x"), mkLeaf("id", "z")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 2 {
		t.Errorf("partial match: want 2 pairs, got %d", len(pairs))
	}
	if pairs[0][0].Label != "x" || pairs[1][0].Label != "z" {
		t.Error("LCS should match x and z")
	}
}

func TestLCSLabelEmpty(t *testing.T) {
	a := []*treesitter.ASTNode{mkLeaf("id", "x")}

	if pairs := LCSLabel(nil, a); pairs != nil {
		t.Error("nil seq1 should return nil")
	}
	if pairs := LCSLabel(a, nil); pairs != nil {
		t.Error("nil seq2 should return nil")
	}
}

func TestLCSLabelNoMatch(t *testing.T) {
	a := []*treesitter.ASTNode{mkLeaf("id", "x")}
	b := []*treesitter.ASTNode{mkLeaf("id", "y")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 0 {
		t.Errorf("no match: want 0 pairs, got %d", len(pairs))
	}
}

func TestLCSStructureBasic(t *testing.T) {
	// Same structure, different labels → should match.
	a := []*treesitter.ASTNode{mkNode("call", "", mkLeaf("id", "x"))}
	b := []*treesitter.ASTNode{mkNode("call", "", mkLeaf("id", "y"))}

	pairs := LCSStructure(a, b)
	if len(pairs) != 1 {
		t.Errorf("structural match: want 1 pair, got %d", len(pairs))
	}
}

func TestLCSStructureDiffShape(t *testing.T) {
	a := []*treesitter.ASTNode{mkNode("call", "", mkLeaf("id", "x"))}
	b := []*treesitter.ASTNode{mkNode("call", "", mkLeaf("id", "x"), mkLeaf("id", "y"))}

	pairs := LCSStructure(a, b)
	if len(pairs) != 0 {
		t.Errorf("different shape: want 0 pairs, got %d", len(pairs))
	}
}

func TestChildIndex(t *testing.T) {
	c1 := mkLeaf("id", "x")
	c2 := mkLeaf("id", "y")
	mkNode("call", "", c1, c2)

	if got := childIndex(c1); got != 0 {
		t.Errorf("childIndex(c1) = %d, want 0", got)
	}
	if got := childIndex(c2); got != 1 {
		t.Errorf("childIndex(c2) = %d, want 1", got)
	}
}

func TestChildIndexNoParent(t *testing.T) {
	n := mkLeaf("id", "x")
	if got := childIndex(n); got != -1 {
		t.Errorf("orphan childIndex = %d, want -1", got)
	}
	if got := childIndex(nil); got != -1 {
		t.Errorf("nil childIndex = %d, want -1", got)
	}
}

func TestScorePartner(t *testing.T) {
	src := mkLeaf("id", "x")
	dst := mkLeaf("id", "x")
	mkNode("call", "", src)
	mkNode("call", "", dst)

	score := scorePartner(src, dst, 0, false)
	if score < 100 {
		t.Errorf("same position + label should score >= 100, got %d", score)
	}
}
