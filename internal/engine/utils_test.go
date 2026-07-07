package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestHeight(t *testing.T) {
	tests := []struct {
		name string
		node *treesitter.ASTNode
		want int
	}{
		{"nil node", nil, 0},
		{"single leaf", mkLeaf("id", "x"), 1},
		{"parent with leaf", mkNode("call", "", mkLeaf("id", "f")), 2},
		{"deep tree", mkNode("a", "", mkNode("b", "", mkLeaf("c", ""))), 3},
		{"wide tree", mkNode("a", "", mkLeaf("b", ""), mkLeaf("c", "")), 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Height(tt.node); got != tt.want {
				t.Errorf("Height() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDescendants(t *testing.T) {
	c1 := mkLeaf("id", "a")
	c2 := mkLeaf("id", "b")
	root := mkNode("call", "", c1, c2)

	desc := Descendants(root)
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
	if desc[0] != c1 || desc[1] != c2 {
		t.Error("descendants not in expected order")
	}
}

func TestDescendantsNested(t *testing.T) {
	leaf := mkLeaf("id", "x")
	mid := mkNode("call", "", leaf)
	root := mkNode("func", "", mid)

	desc := Descendants(root)
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
}

func TestDice(t *testing.T) {
	// Two identical trees with full mapping → dice = 1.0.
	a1 := mkLeaf("id", "x")
	a2 := mkLeaf("id", "y")
	rootA := mkNode("call", "", a1, a2)

	b1 := mkLeaf("id", "x")
	b2 := mkLeaf("id", "y")
	rootB := mkNode("call", "", b1, b2)

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{a1: b1, a2: b2}
	d := Dice(rootA, rootB, m)
	if d != 1.0 {
		t.Errorf("fully mapped dice = %f, want 1.0", d)
	}
}

func TestDiceNoMapping(t *testing.T) {
	rootA := mkNode("call", "", mkLeaf("id", "x"))
	rootB := mkNode("call", "", mkLeaf("id", "y"))

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}
	d := Dice(rootA, rootB, m)
	if d != 0.0 {
		t.Errorf("empty mapping dice = %f, want 0.0", d)
	}
}

func TestDiceEmptyTrees(t *testing.T) {
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")
	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}

	// Leaves have no descendants, so denom = 0.
	d := Dice(a, b, m)
	if d != 0.0 {
		t.Errorf("leaf dice = %f, want 0.0", d)
	}
}

func TestChawatheSimilarity(t *testing.T) {
	a1 := mkLeaf("id", "x")
	rootA := mkNode("call", "", a1)
	b1 := mkLeaf("id", "x")
	rootB := mkNode("call", "", b1)

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{a1: b1}
	sim := ChawatheSimilarity(rootA, rootB, m)
	if sim != 1.0 {
		t.Errorf("fully mapped chawathe = %f, want 1.0", sim)
	}
}

func TestChawatheSimilarityEmpty(t *testing.T) {
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")
	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}
	sim := ChawatheSimilarity(a, b, m)
	if sim != 0.0 {
		t.Errorf("empty chawathe = %f, want 0.0", sim)
	}
}

func TestIsomorphic(t *testing.T) {
	tests := []struct {
		name string
		a, b *treesitter.ASTNode
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, mkLeaf("id", "x"), false},
		{"b nil", mkLeaf("id", "x"), nil, false},
		{"same leaf", mkLeaf("id", "x"), mkLeaf("id", "x"), true},
		{"diff label", mkLeaf("id", "x"), mkLeaf("id", "y"), false},
		{"diff type", mkLeaf("id", "x"), mkLeaf("str", "x"), false},
		{
			"same tree",
			mkNode("call", "", mkLeaf("id", "f")),
			mkNode("call", "", mkLeaf("id", "f")),
			true,
		},
		{
			"diff children count",
			mkNode("call", "", mkLeaf("id", "f")),
			mkNode("call", "", mkLeaf("id", "f"), mkLeaf("id", "g")),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Isomorphic(tt.a, tt.b); got != tt.want {
				t.Errorf("Isomorphic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStructureIsomorphic(t *testing.T) {
	// Same structure, different labels → true.
	a := mkNode("call", "", mkLeaf("id", "x"))
	b := mkNode("call", "", mkLeaf("id", "y"))
	if !StructureIsomorphic(a, b) {
		t.Error("same structure should be StructureIsomorphic")
	}

	// Different structure → false.
	c := mkNode("call", "", mkLeaf("id", "x"), mkLeaf("id", "y"))
	if StructureIsomorphic(a, c) {
		t.Error("different structure should not be StructureIsomorphic")
	}
}

func TestPostOrder(t *testing.T) {
	leaf := mkLeaf("id", "x")
	root := mkNode("call", "", leaf)
	order := PostOrder(root)

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != leaf || order[1] != root {
		t.Error("post-order should visit child before parent")
	}
}

func TestPreOrder(t *testing.T) {
	leaf := mkLeaf("id", "x")
	root := mkNode("call", "", leaf)
	order := PreOrder(root)

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != root || order[1] != leaf {
		t.Error("pre-order should visit parent before child")
	}
}

func TestNearestMatchedAncestor(t *testing.T) {
	grandchild := mkLeaf("id", "x")
	child := mkNode("call", "", grandchild)
	root := mkNode("func", "", child)

	m := NewMapping()
	m.Add(root, mkLeaf("func", ""))

	// Grandchild's nearest matched ancestor should be root.
	got := NearestMatchedAncestor(grandchild, m, false)
	if got != root {
		t.Errorf("expected root, got %v", got)
	}

	// Root itself has no matched ancestor.
	got = NearestMatchedAncestor(root, m, false)
	if got != nil {
		t.Errorf("expected nil for root, got %v", got)
	}
}

func TestNearestMatchedAncestorDst(t *testing.T) {
	grandchild := mkLeaf("id", "x")
	child := mkNode("call", "", grandchild)
	root := mkNode("func", "", child)

	m := NewMapping()
	m.Add(mkLeaf("func", ""), root)

	got := NearestMatchedAncestor(grandchild, m, true)
	if got != root {
		t.Errorf("expected root on dst side, got %v", got)
	}
}

func TestAncestorNameSimilarity(t *testing.T) {
	// Build two trees with overlapping identifier children.
	a := mkNode("func", "", mkLeaf("identifier", "foo"), mkLeaf("id", "x"))
	b := mkNode("func", "", mkLeaf("identifier", "foo"), mkLeaf("id", "y"))
	leaf1 := a.Children[1]
	leaf2 := b.Children[1]

	overlap := AncestorNameSimilarity(leaf1, leaf2)
	if overlap != 1 {
		t.Errorf("expected overlap=1, got %d", overlap)
	}
}

func TestAncestorNameSimilarityNil(t *testing.T) {
	if AncestorNameSimilarity(nil, mkLeaf("id", "x")) != 0 {
		t.Error("nil input should return 0")
	}
}
