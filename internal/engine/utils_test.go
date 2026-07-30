package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestHeight(t *testing.T) {
	tests := []struct {
		name string
		node *treesitter.ASTNode
		want int
	}{
		{"nil node", nil, 0},
		{"single leaf", testutil.Leaf("id", "x"), 1},
		{"parent with leaf", testutil.Node("call", "", testutil.Leaf("id", "f")), 2},
		{"deep tree", testutil.Node("a", "", testutil.Node("b", "", testutil.Leaf("c", ""))), 3},
		{"wide tree", testutil.Node("a", "", testutil.Leaf("b", ""), testutil.Leaf("c", "")), 2},
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
	c1 := testutil.Leaf("id", "a")
	c2 := testutil.Leaf("id", "b")
	root := testutil.Node("call", "", c1, c2)

	desc := Descendants(root)
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
	if desc[0] != c1 || desc[1] != c2 {
		t.Error("descendants not in expected order")
	}
}

func TestDescendantsNested(t *testing.T) {
	leaf := testutil.Leaf("id", "x")
	mid := testutil.Node("call", "", leaf)
	root := testutil.Node("func", "", mid)

	desc := Descendants(root)
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
}

func TestDice(t *testing.T) {
	// Identical mapped trees have a Dice coefficient of 1.0.
	a1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("id", "y")
	rootA := testutil.Node("call", "", a1, a2)

	b1 := testutil.Leaf("id", "x")
	b2 := testutil.Leaf("id", "y")
	rootB := testutil.Node("call", "", b1, b2)

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{a1: b1, a2: b2}
	d := Dice(rootA, rootB, m)
	if d != 1.0 {
		t.Errorf("fully mapped dice = %f, want 1.0", d)
	}
}

func TestDiceNoMapping(t *testing.T) {
	rootA := testutil.Node("call", "", testutil.Leaf("id", "x"))
	rootB := testutil.Node("call", "", testutil.Leaf("id", "y"))

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}
	d := Dice(rootA, rootB, m)
	if d != 0.0 {
		t.Errorf("empty mapping dice = %f, want 0.0", d)
	}
}

func TestDiceEmptyTrees(t *testing.T) {
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")
	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}

	// Dice is 0.0 for leaves because they have no descendants.
	d := Dice(a, b, m)
	if d != 0.0 {
		t.Errorf("leaf dice = %f, want 0.0", d)
	}
}

func TestChawatheSimilarity(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	rootA := testutil.Node("call", "", a1)
	b1 := testutil.Leaf("id", "x")
	rootB := testutil.Node("call", "", b1)

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{a1: b1}
	sim := ChawatheSimilarity(rootA, rootB, m)
	if sim != 1.0 {
		t.Errorf("fully mapped chawathe = %f, want 1.0", sim)
	}
}

func TestChawatheSimilarityEmpty(t *testing.T) {
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")
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
		{"a nil", nil, testutil.Leaf("id", "x"), false},
		{"b nil", testutil.Leaf("id", "x"), nil, false},
		{"same leaf", testutil.Leaf("id", "x"), testutil.Leaf("id", "x"), true},
		{"diff label", testutil.Leaf("id", "x"), testutil.Leaf("id", "y"), false},
		{"diff type", testutil.Leaf("id", "x"), testutil.Leaf("str", "x"), false},
		{
			"same tree",
			testutil.Node("call", "", testutil.Leaf("id", "f")),
			testutil.Node("call", "", testutil.Leaf("id", "f")),
			true,
		},
		{
			"diff children count",
			testutil.Node("call", "", testutil.Leaf("id", "f")),
			testutil.Node("call", "", testutil.Leaf("id", "f"), testutil.Leaf("id", "g")),
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
	// StructureIsomorphic ignores label differences.
	a := testutil.Node("call", "", testutil.Leaf("id", "x"))
	b := testutil.Node("call", "", testutil.Leaf("id", "y"))
	if !StructureIsomorphic(a, b) {
		t.Error("same structure should be StructureIsomorphic")
	}

	// StructureIsomorphic returns false for different structures.
	c := testutil.Node("call", "", testutil.Leaf("id", "x"), testutil.Leaf("id", "y"))
	if StructureIsomorphic(a, c) {
		t.Error("different structure should not be StructureIsomorphic")
	}
}

func TestPostOrder(t *testing.T) {
	leaf := testutil.Leaf("id", "x")
	root := testutil.Node("call", "", leaf)
	order := PostOrder(root)

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != leaf || order[1] != root {
		t.Error("post-order should visit child before parent")
	}
}

func TestPreOrder(t *testing.T) {
	leaf := testutil.Leaf("id", "x")
	root := testutil.Node("call", "", leaf)
	order := PreOrder(root)

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != root || order[1] != leaf {
		t.Error("pre-order should visit parent before child")
	}
}

func TestNearestMatchedAncestor(t *testing.T) {
	grandchild := testutil.Leaf("id", "x")
	child := testutil.Node("call", "", grandchild)
	root := testutil.Node("func", "", child)

	m := NewMapping()
	m.Add(root, testutil.Leaf("func", ""))

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
	grandchild := testutil.Leaf("id", "x")
	child := testutil.Node("call", "", grandchild)
	root := testutil.Node("func", "", child)

	m := NewMapping()
	m.Add(testutil.Leaf("func", ""), root)

	got := NearestMatchedAncestor(grandchild, m, true)
	if got != root {
		t.Errorf("expected root on dst side, got %v", got)
	}
}

func TestAncestorNameSimilarity(t *testing.T) {
	// Trees share overlapping identifier children.
	a := testutil.Node("func", "", testutil.Leaf("identifier", "foo"), testutil.Leaf("id", "x"))
	b := testutil.Node("func", "", testutil.Leaf("identifier", "foo"), testutil.Leaf("id", "y"))
	leaf1 := a.Children[1]
	leaf2 := b.Children[1]

	overlap := AncestorNameSimilarity(leaf1, leaf2)
	if overlap != 1 {
		t.Errorf("expected overlap=1, got %d", overlap)
	}
}

func TestAncestorNameSimilarityNil(t *testing.T) {
	if AncestorNameSimilarity(nil, testutil.Leaf("id", "x")) != 0 {
		t.Error("nil input should return 0")
	}
}
