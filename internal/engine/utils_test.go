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
	if !StructureIsomorphic(nil, nil) {
		t.Error("both nil should be StructureIsomorphic")
	}
	if StructureIsomorphic(nil, testutil.Leaf("id", "x")) {
		t.Error("nil and non-nil should not be StructureIsomorphic")
	}

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

func TestAncestorNameSimilarityPairKey(t *testing.T) {
	// Ancestor pair keys (like in JSON/YAML) should contribute to similarity overlap.
	pair1 := testutil.Node("pair", "", testutil.Leaf("string", "\"priority\""), testutil.Node("object", "", testutil.Leaf("string", "min")))
	pair2 := testutil.Node("pair", "", testutil.Leaf("string", "\"priority\""), testutil.Node("object", "", testutil.Leaf("string", "max")))
	leaf1 := pair1.Children[1].Children[0]
	leaf2 := pair2.Children[1].Children[0]

	overlap := AncestorNameSimilarity(leaf1, leaf2)
	if overlap != 1 {
		t.Errorf("expected overlap=1 for pair key 'priority', got %d", overlap)
	}
}

func TestAncestorNameSimilarityNil(t *testing.T) {
	if AncestorNameSimilarity(nil, testutil.Leaf("id", "x")) != 0 {
		t.Error("nil input should return 0")
	}
}

func TestCommentSimilarity(t *testing.T) {
	tests := []struct {
		s1, s2 string
		match  bool
	}{
		{"// Comment", "// Comment 1", true},
		{"// Comment", "// Hello", false},
		{"# Comment", "// Comment 1", true},
		{"/* Comment */", "// Comment", true},
		{"// then Dice as a tie-breaker. Positional prior...", "// then Dice as a tie-breaker.", true},
		{"// leafLabels collects all...", "// then Dice as a tie-breaker.", false},
	}

	for _, tt := range tests {
		got := CommentSimilarity(tt.s1, tt.s2)
		match := got >= 0.4
		if match != tt.match {
			t.Errorf("CommentSimilarity(%q, %q) = %f; match = %v, want match = %v", tt.s1, tt.s2, got, match, tt.match)
		}
	}
}

func TestStructureIsomorphicComments(t *testing.T) {
	c1 := &treesitter.ASTNode{Type: "comment", Label: "// Comment"}
	c2 := &treesitter.ASTNode{Type: "comment", Label: "// Comment 1"}
	c3 := &treesitter.ASTNode{Type: "comment", Label: "// Hello"}

	if !StructureIsomorphic(c1, c2) {
		t.Error("expected c1 and c2 to be structurally isomorphic comments")
	}
	if StructureIsomorphic(c1, c3) {
		t.Error("expected c1 and c3 not to be structurally isomorphic comments")
	}
}

func TestLeafSimilarityAndHasLongLeafToken(t *testing.T) {
	t1 := testutil.Node("pair", "",
		testutil.Leaf("integer", "414"),
		testutil.Node("tuple", "", testutil.Leaf("string", "\"request_uri_too_large\"")),
	)
	t2 := testutil.Node("pair", "",
		testutil.Leaf("integer", "422"),
		testutil.Node("tuple", "", testutil.Leaf("string", "\"unprocessable_entity\"")),
	)
	t3 := testutil.Node("pair", "",
		testutil.Leaf("integer", "414"),
		testutil.Node("tuple", "", testutil.Leaf("string", "\"request_uri_too_large\""), testutil.Leaf("string", "\"uri_too_long\"")),
	)

	if !HasLongLeafToken(t1) {
		t.Error("expected t1 to have long leaf token")
	}
	if !HasLongLeafToken(t2) {
		t.Error("expected t2 to have long leaf token")
	}

	if sim := LeafSimilarity(t1, t2); sim != 0 {
		t.Errorf("LeafSimilarity(t1, t2) = %f, want 0", sim)
	}

	if sim := LeafSimilarity(t1, t3); sim <= 0 {
		t.Errorf("LeafSimilarity(t1, t3) = %f, want > 0", sim)
	}
}

func TestUnquote(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"empty string", "", ""},
		{"single quote char", "\"", "\""},
		{"single char", "a", "a"},
		{"double quotes empty", `""`, ""},
		{"single quotes empty", `''`, ""},
		{"backticks empty", "``", ""},
		{"double quotes text", `"hello world"`, "hello world"},
		{"single quotes text", `'hello world'`, "hello world"},
		{"backtick text", "`hello world`", "hello world"},
		{"mismatched quotes", `"hello'`, `"hello'`},
		{"inner quotes preserved", `"hello 'world'"`, "hello 'world'"},
		{"plain unquoted", "hello", "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Unquote(tt.in); got != tt.want {
				t.Errorf("Unquote(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNodePairRole(t *testing.T) {
	t.Run("nil node", func(t *testing.T) {
		if got := NodePairRole(nil); got != PairRoleNone {
			t.Errorf("NodePairRole(nil) = %v, want %v", got, PairRoleNone)
		}
	})

	t.Run("orphan node", func(t *testing.T) {
		orphan := testutil.Leaf("plain_scalar", "val")
		orphan.Language = "yaml"
		if got := NodePairRole(orphan); got != PairRoleNone {
			t.Errorf("NodePairRole(orphan) = %v, want %v", got, PairRoleNone)
		}
	})

	t.Run("yaml pair key and value", func(t *testing.T) {
		key := testutil.Leaf("plain_scalar", "name")
		val := testutil.Leaf("plain_scalar", "service")
		pair := testutil.Node("block_mapping_pair", "", key, val)
		root := testutil.Node("block_mapping", "", pair)
		root.Language = "yaml"

		if got := NodePairRole(key); got != PairRoleKey {
			t.Errorf("NodePairRole(key) = %v, want %v", got, PairRoleKey)
		}
		if got := NodePairRole(val); got != PairRoleValue {
			t.Errorf("NodePairRole(val) = %v, want %v", got, PairRoleValue)
		}
	})

	t.Run("yaml pair with scaffolding flow_node wrapper", func(t *testing.T) {
		keyLeaf := testutil.Leaf("plain_scalar", "image")
		keyWrap := testutil.Node("flow_node", "", keyLeaf)
		valLeaf := testutil.Leaf("double_quote_scalar", `"nginx"`)
		valWrap := testutil.Node("flow_node", "", valLeaf)
		pair := testutil.Node("block_mapping_pair", "", keyWrap, valWrap)
		root := testutil.Node("block_mapping", "", pair)
		root.Language = "yaml"

		if got := NodePairRole(keyLeaf); got != PairRoleKey {
			t.Errorf("NodePairRole(keyLeaf) = %v, want %v", got, PairRoleKey)
		}
		if got := NodePairRole(valLeaf); got != PairRoleValue {
			t.Errorf("NodePairRole(valLeaf) = %v, want %v", got, PairRoleValue)
		}
	})

	t.Run("non-pair node in hierarchy", func(t *testing.T) {
		item1 := testutil.Leaf("plain_scalar", "item1")
		item2 := testutil.Leaf("plain_scalar", "item2")
		root := testutil.Node("block_sequence", "", item1, item2)
		root.Language = "yaml"

		if got := NodePairRole(item1); got != PairRoleNone {
			t.Errorf("NodePairRole(item1) = %v, want %v", got, PairRoleNone)
		}
	})
}

func TestCompatiblePairRoles(t *testing.T) {
	key1 := testutil.Leaf("plain_scalar", "k1")
	val1 := testutil.Leaf("plain_scalar", "v1")
	pair1 := testutil.Node("block_mapping_pair", "", key1, val1)
	root1 := testutil.Node("block_mapping", "", pair1)
	root1.Language = "yaml"

	key2 := testutil.Leaf("plain_scalar", "k2")
	val2 := testutil.Leaf("plain_scalar", "v2")
	pair2 := testutil.Node("block_mapping_pair", "", key2, val2)
	root2 := testutil.Node("block_mapping", "", pair2)
	root2.Language = "yaml"

	nonPairNode := testutil.Leaf("plain_scalar", "plain")
	root3 := testutil.Node("block_sequence", "", nonPairNode)
	root3.Language = "yaml"

	tests := []struct {
		name string
		n1   *treesitter.ASTNode
		n2   *treesitter.ASTNode
		want bool
	}{
		{"both keys", key1, key2, true},
		{"both values", val1, val2, true},
		{"key and value", key1, val2, false},
		{"value and key", val1, key2, false},
		{"key and non-pair", key1, nonPairNode, true},
		{"value and non-pair", val1, nonPairNode, true},
		{"non-pair and non-pair", nonPairNode, nonPairNode, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CompatiblePairRoles(tt.n1, tt.n2); got != tt.want {
				t.Errorf("CompatiblePairRoles(%s, %s) = %v, want %v", tt.name, tt.name, got, tt.want)
			}
		})
	}
}
