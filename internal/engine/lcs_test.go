package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestLCSLabelIdentical(t *testing.T) {
	srcA := []byte("package p\nvar ( x = 1\n y = 2 )\n")
	srcB := []byte("package p\nvar ( x = 1\n y = 2 )\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	x1, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	y1, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 3})
	if err != nil {
		t.Fatal(err)
	}
	x2, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	y2, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 3})
	if err != nil {
		t.Fatal(err)
	}

	pairs := LCSLabel([]*treesitter.ASTNode{x1, y1}, []*treesitter.ASTNode{x2, y2})
	if len(pairs) != 2 {
		t.Errorf("identical seqs: want 2 pairs, got %d", len(pairs))
	}
}

func TestLCSLabelPartial(t *testing.T) {
	srcA := []byte("package p\nvar ( x = 1\n y = 2\n z = 3 )\n")
	srcB := []byte("package p\nvar ( x = 1\n z = 3 )\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	x1, _ := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 2})
	y1, _ := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 3})
	z1, _ := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 4})

	x2, _ := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 2})
	z2, _ := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 3})

	pairs := LCSLabel([]*treesitter.ASTNode{x1, y1, z1}, []*treesitter.ASTNode{x2, z2})
	if len(pairs) != 2 {
		t.Errorf("partial match: want 2 pairs, got %d", len(pairs))
	}
	if pairs[0][0].Label != "x" || pairs[1][0].Label != "z" {
		t.Error("LCS should match x and z")
	}
}

func TestLCSLabelEmpty(t *testing.T) {
	src := []byte("package p\nvar x = 1\n")
	root, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	x, _ := testutil.FindNode(root, testutil.NodeQuery{Type: "identifier", Line: 2})
	a := []*treesitter.ASTNode{x}

	if pairs := LCSLabel(nil, a); pairs != nil {
		t.Error("nil seq1 should return nil")
	}
	if pairs := LCSLabel(a, nil); pairs != nil {
		t.Error("nil seq2 should return nil")
	}
}

func TestLCSLabelNoMatch(t *testing.T) {
	srcA := []byte("package p\nvar x = 1\n")
	srcB := []byte("package p\nvar y = 1\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	x, _ := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 2})
	y, _ := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 2})

	pairs := LCSLabel([]*treesitter.ASTNode{x}, []*treesitter.ASTNode{y})
	if len(pairs) != 0 {
		t.Errorf("no match: want 0 pairs, got %d", len(pairs))
	}
}

func TestLCSStructureBasic(t *testing.T) {
	// Matches same structure even if labels differ.
	srcA := []byte("package p\nfunc f() { foo(1) }\n")
	srcB := []byte("package p\nfunc f() { bar(2) }\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	callA, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	callB, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}

	pairs := LCSStructure([]*treesitter.ASTNode{callA}, []*treesitter.ASTNode{callB}, nil)
	if len(pairs) != 1 {
		t.Errorf("structural match: want 1 pair, got %d", len(pairs))
	}
}

func TestLCSStructureDiffShape(t *testing.T) {
	srcA := []byte("package p\nfunc f() { foo(1) }\n")
	srcB := []byte("package p\nfunc f() { bar(1, 2) }\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	callA, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	callB, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}

	pairs := LCSStructure([]*treesitter.ASTNode{callA}, []*treesitter.ASTNode{callB}, nil)
	if len(pairs) != 0 {
		t.Errorf("different shape: want 0 pairs, got %d", len(pairs))
	}
}

func TestChildIndex(t *testing.T) {
	src := []byte("package p\nfunc f() { x(); y() }\n")
	root, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	c0 := root.Children[0]
	c1 := root.Children[1]

	if got := c0.ChildIndex(); got != 0 {
		t.Errorf("c0.ChildIndex() = %d, want 0", got)
	}
	if got := c1.ChildIndex(); got != 1 {
		t.Errorf("c1.ChildIndex() = %d, want 1", got)
	}
}

func TestChildIndexNoParent(t *testing.T) {
	src := []byte("package p\n")
	root, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	if got := root.ChildIndex(); got != -1 {
		t.Errorf("orphan ChildIndex() = %d, want -1", got)
	}
	var nilNode *treesitter.ASTNode
	if got := nilNode.ChildIndex(); got != -1 {
		t.Errorf("nil ChildIndex() = %d, want -1", got)
	}
}

func TestScorePartner(t *testing.T) {
	srcA := []byte("package p\nfunc f() { x() }\n")
	srcB := []byte("package p\nfunc f() { x() }\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	idA, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	if err != nil {
		t.Fatal(err)
	}
	idB, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	if err != nil {
		t.Fatal(err)
	}

	score := scorePartner(idA, idB, idA.ChildIndex(), nil)
	if score < 100 {
		t.Errorf("same position + label should score >= 100, got %d", score)
	}
}
