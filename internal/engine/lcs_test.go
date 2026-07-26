package engine

import (
	"github.com/HarshK97/diffmantic/internal/testutil"

	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestLCSLabelIdentical(t *testing.T) {
	a := []*treesitter.ASTNode{testutil.Leaf("id", "x"), testutil.Leaf("id", "y")}
	b := []*treesitter.ASTNode{testutil.Leaf("id", "x"), testutil.Leaf("id", "y")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 2 {
		t.Errorf("identical seqs: want 2 pairs, got %d", len(pairs))
	}
}

func TestLCSLabelPartial(t *testing.T) {
	a := []*treesitter.ASTNode{testutil.Leaf("id", "x"), testutil.Leaf("id", "y"), testutil.Leaf("id", "z")}
	b := []*treesitter.ASTNode{testutil.Leaf("id", "x"), testutil.Leaf("id", "z")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 2 {
		t.Errorf("partial match: want 2 pairs, got %d", len(pairs))
	}
	if pairs[0][0].Label != "x" || pairs[1][0].Label != "z" {
		t.Error("LCS should match x and z")
	}
}

func TestLCSLabelEmpty(t *testing.T) {
	a := []*treesitter.ASTNode{testutil.Leaf("id", "x")}

	if pairs := LCSLabel(nil, a); pairs != nil {
		t.Error("nil seq1 should return nil")
	}
	if pairs := LCSLabel(a, nil); pairs != nil {
		t.Error("nil seq2 should return nil")
	}
}

func TestLCSLabelNoMatch(t *testing.T) {
	a := []*treesitter.ASTNode{testutil.Leaf("id", "x")}
	b := []*treesitter.ASTNode{testutil.Leaf("id", "y")}

	pairs := LCSLabel(a, b)
	if len(pairs) != 0 {
		t.Errorf("no match: want 0 pairs, got %d", len(pairs))
	}
}

func TestLCSStructureBasic(t *testing.T) {
	// Matches same structure even if labels differ.
	a := []*treesitter.ASTNode{testutil.Node("call", "", testutil.Leaf("id", "x"))}
	b := []*treesitter.ASTNode{testutil.Node("call", "", testutil.Leaf("id", "y"))}

	pairs := LCSStructure(a, b)
	if len(pairs) != 1 {
		t.Errorf("structural match: want 1 pair, got %d", len(pairs))
	}
}

func TestLCSStructureDiffShape(t *testing.T) {
	a := []*treesitter.ASTNode{testutil.Node("call", "", testutil.Leaf("id", "x"))}
	b := []*treesitter.ASTNode{testutil.Node("call", "", testutil.Leaf("id", "x"), testutil.Leaf("id", "y"))}

	pairs := LCSStructure(a, b)
	if len(pairs) != 0 {
		t.Errorf("different shape: want 0 pairs, got %d", len(pairs))
	}
}

func TestChildIndex(t *testing.T) {
	c1 := testutil.Leaf("id", "x")
	c2 := testutil.Leaf("id", "y")
	testutil.Node("call", "", c1, c2)

	if got := childIndex(c1); got != 0 {
		t.Errorf("childIndex(c1) = %d, want 0", got)
	}
	if got := childIndex(c2); got != 1 {
		t.Errorf("childIndex(c2) = %d, want 1", got)
	}
}

func TestChildIndexNoParent(t *testing.T) {
	n := testutil.Leaf("id", "x")
	if got := childIndex(n); got != -1 {
		t.Errorf("orphan childIndex = %d, want -1", got)
	}
	if got := childIndex(nil); got != -1 {
		t.Errorf("nil childIndex = %d, want -1", got)
	}
}

func TestScorePartner(t *testing.T) {
	src := testutil.Leaf("id", "x")
	dst := testutil.Leaf("id", "x")
	testutil.Node("call", "", src)
	testutil.Node("call", "", dst)

	score := scorePartner(src, dst, 0, false)
	if score < 100 {
		t.Errorf("same position + label should score >= 100, got %d", score)
	}
}
