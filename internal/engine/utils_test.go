package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestHeight(t *testing.T) {
	src := `package main
func f() {
	x := 1
	return
}
`
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	leaf, err := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	if err != nil {
		t.Fatalf("find leaf: %v", err)
	}
	retStmt, err := testutil.FindNode(tree, testutil.NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find retStmt: %v", err)
	}
	decl, err := testutil.FindNode(tree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	if err != nil {
		t.Fatalf("find decl: %v", err)
	}

	tests := []struct {
		name string
		node *treesitter.ASTNode
		want int
	}{
		{"nil node", nil, 0},
		{"single leaf", leaf, 1},
		{"parent with leaf", retStmt, 2},
		{"deep tree", decl, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Height(tt.node); got != tt.want {
				t.Errorf("Height(%s) = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestDice(t *testing.T) {
	srcA := "package main\nfunc f() { a(x, y) }"
	srcB := "package main\nfunc g() { a(x, y) }"
	treeA, err := treesitter.Parse([]byte(srcA), "test.go")
	if err != nil {
		t.Fatalf("parse A: %v", err)
	}
	treeB, err := treesitter.Parse([]byte(srcB), "test.go")
	if err != nil {
		t.Fatalf("parse B: %v", err)
	}

	callA, _ := testutil.FindNode(treeA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	callB, _ := testutil.FindNode(treeB, testutil.NodeQuery{Type: "call_expression", Line: 2})

	descA := callA.Descendants()
	descB := callB.Descendants()
	m := make(map[*treesitter.ASTNode]*treesitter.ASTNode, len(descA))
	for i := range descA {
		m[descA[i]] = descB[i]
	}
	d := Dice(callA, callB, m)
	if d != 1.0 {
		t.Errorf("fully mapped dice = %f, want 1.0", d)
	}
}

func TestDiceNoMapping(t *testing.T) {
	srcA := "package main\nfunc f() { a(x) }"
	srcB := "package main\nfunc g() { a(y) }"
	treeA, _ := treesitter.Parse([]byte(srcA), "test.go")
	treeB, _ := treesitter.Parse([]byte(srcB), "test.go")

	callA, _ := testutil.FindNode(treeA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	callB, _ := testutil.FindNode(treeB, testutil.NodeQuery{Type: "call_expression", Line: 2})

	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}
	d := Dice(callA, callB, m)
	if d != 0.0 {
		t.Errorf("empty mapping dice = %f, want 0.0", d)
	}
}

func TestDiceEmptyTrees(t *testing.T) {
	src := "package main\nfunc f() { _ = x; _ = y }"
	tree, _ := treesitter.Parse([]byte(src), "test.go")
	x, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	y, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 4})
	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}

	// Dice is 0.0 for leaves because they have no descendants.
	d := Dice(x, y, m)
	if d != 0.0 {
		t.Errorf("leaf dice = %f, want 0.0", d)
	}
}

func TestChawatheSimilarity(t *testing.T) {
	srcA := "package main\nfunc f() { a(x) }"
	srcB := "package main\nfunc g() { a(x) }"
	treeA, _ := treesitter.Parse([]byte(srcA), "test.go")
	treeB, _ := treesitter.Parse([]byte(srcB), "test.go")

	callA, _ := testutil.FindNode(treeA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	callB, _ := testutil.FindNode(treeB, testutil.NodeQuery{Type: "call_expression", Line: 2})

	descA := callA.Descendants()
	descB := callB.Descendants()
	m := make(map[*treesitter.ASTNode]*treesitter.ASTNode, len(descA))
	for i := range descA {
		m[descA[i]] = descB[i]
	}
	sim := ChawatheSimilarity(callA, callB, m)
	if sim != 1.0 {
		t.Errorf("fully mapped chawathe = %f, want 1.0", sim)
	}
}

func TestChawatheSimilarityEmpty(t *testing.T) {
	src := "package main\nfunc f() { _ = x; _ = y }"
	tree, _ := treesitter.Parse([]byte(src), "test.go")
	x, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	y, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 4})
	m := map[*treesitter.ASTNode]*treesitter.ASTNode{}
	sim := ChawatheSimilarity(x, y, m)
	if sim != 0.0 {
		t.Errorf("empty chawathe = %f, want 0.0", sim)
	}
}

func TestIsomorphic(t *testing.T) {
	src := "package main\nfunc f() { a(x); a(x); b(x); a(x, y) }"
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	callAX1, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 1})
	callAX2, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 2})
	callBX, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 3})
	callAXY, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 4})

	idA1, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2}) // a
	idA2, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 4}) // a
	idB, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 6})  // b
	pkgMain, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "package_identifier", Line: 1})

	tests := []struct {
		name string
		a, b *treesitter.ASTNode
		want bool
	}{
		{"both nil", nil, nil, true},
		{"a nil", nil, idA1, false},
		{"b nil", idA1, nil, false},
		{"same leaf", idA1, idA2, true},
		{"diff label", idA1, idB, false},
		{"diff type", idA1, pkgMain, false},
		{"same tree", callAX1, callAX2, true},
		{"diff label tree", callAX1, callBX, false},
		{"diff children count", callAX1, callAXY, false},
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

	src := "package main\nfunc f() { a(x); b(y); a(x, y) }"
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	callAX, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 1})
	callBY, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 2})
	callAXY, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 3})
	idA, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

	if StructureIsomorphic(nil, idA) {
		t.Error("nil and non-nil should not be StructureIsomorphic")
	}

	// StructureIsomorphic ignores label differences.
	if !StructureIsomorphic(callAX, callBY) {
		t.Error("same structure should be StructureIsomorphic")
	}

	// StructureIsomorphic returns false for different structures.
	if StructureIsomorphic(callAX, callAXY) {
		t.Error("different structure should not be StructureIsomorphic")
	}
}

func TestNearestMatchedAncestor(t *testing.T) {
	src := "package main\nfunc f() { x() }"
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	root, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	grandchild, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

	m := NewMapping()
	m.Add(root, root)

	// Grandchild nearest matched ancestor should be root.
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
	src := "package main\nfunc f() { x() }"
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	root, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	grandchild, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

	m := NewMapping()
	m.Add(root, root)

	got := NearestMatchedAncestor(grandchild, m, true)
	if got != root {
		t.Errorf("expected root on dst side, got %v", got)
	}
}

func TestAncestorNameSimilarity(t *testing.T) {
	srcA := "package main\nfunc foo() { x() }"
	srcB := "package main\nfunc foo() { y() }"
	treeA, _ := treesitter.Parse([]byte(srcA), "test.go")
	treeB, _ := treesitter.Parse([]byte(srcB), "test.go")

	leaf1, _ := testutil.FindNode(treeA, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	leaf2, _ := testutil.FindNode(treeB, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

	overlap := AncestorNameSimilarity(leaf1, leaf2)
	if overlap != 1 {
		t.Errorf("expected overlap=1, got %d", overlap)
	}
}

func TestAncestorNameSimilarityPairKey(t *testing.T) {
	srcA := `const obj1 = { priority: { min: 1 } };`
	srcB := `let obj2 = { priority: { max: 1 } };`
	treeA, err := treesitter.Parse([]byte(srcA), "test.js")
	if err != nil {
		t.Fatalf("parse A: %v", err)
	}
	treeB, err := treesitter.Parse([]byte(srcB), "test.js")
	if err != nil {
		t.Fatalf("parse B: %v", err)
	}

	leaf1, _ := testutil.FindNode(treeA, testutil.NodeQuery{Type: "property_identifier", Line: 1, Nth: 2}) // min
	leaf2, _ := testutil.FindNode(treeB, testutil.NodeQuery{Type: "property_identifier", Line: 1, Nth: 2}) // max

	overlap := AncestorNameSimilarity(leaf1, leaf2)
	if overlap != 1 {
		t.Errorf("expected overlap=1 for pair key priority, got %d", overlap)
	}
}

func TestAncestorNameSimilarityNil(t *testing.T) {
	src := "package main\nfunc f() { _ = x }"
	tree, _ := treesitter.Parse([]byte(src), "test.go")
	leaf, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

	if AncestorNameSimilarity(nil, leaf) != 0 {
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
	src1 := `d1 = {414: ("request_uri_too_large",)}`
	src2 := `d2 = {422: ("unprocessable_entity",)}`
	src3 := `d3 = {414: ("request_uri_too_large", "uri_too_long")}`

	t1Tree, err := treesitter.Parse([]byte(src1), "test.py")
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	t2Tree, err := treesitter.Parse([]byte(src2), "test.py")
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}
	t3Tree, err := treesitter.Parse([]byte(src3), "test.py")
	if err != nil {
		t.Fatalf("parse 3: %v", err)
	}

	t1, _ := testutil.FindNode(t1Tree, testutil.NodeQuery{Type: "pair", Line: 1})
	t2, _ := testutil.FindNode(t2Tree, testutil.NodeQuery{Type: "pair", Line: 1})
	t3, _ := testutil.FindNode(t3Tree, testutil.NodeQuery{Type: "pair", Line: 1})

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
		src := "const val = 1;"
		tree, _ := treesitter.Parse([]byte(src), "test.js")
		orphan, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 1})
		if got := NodePairRole(orphan); got != PairRoleNone {
			t.Errorf("NodePairRole(orphan) = %v, want %v", got, PairRoleNone)
		}
	})

	t.Run("javascript pair key and value", func(t *testing.T) {
		src := `const obj = { name: "service" };`
		tree, err := treesitter.Parse([]byte(src), "test.js")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		key, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "property_identifier", Line: 1})
		val, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "string", Line: 1})

		if got := NodePairRole(key); got != PairRoleKey {
			t.Errorf("NodePairRole(key) = %v, want %v", got, PairRoleKey)
		}
		if got := NodePairRole(val); got != PairRoleValue {
			t.Errorf("NodePairRole(val) = %v, want %v", got, PairRoleValue)
		}
	})

	t.Run("non-pair node in hierarchy", func(t *testing.T) {
		src := `const arr = [item1, item2];`
		tree, err := treesitter.Parse([]byte(src), "test.js")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		item1, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 1, Nth: 2})

		if got := NodePairRole(item1); got != PairRoleNone {
			t.Errorf("NodePairRole(item1) = %v, want %v", got, PairRoleNone)
		}
	})
}

func TestCompatiblePairRoles(t *testing.T) {
	src1 := `const obj1 = { k1: "v1" };`
	src2 := `const obj2 = { k2: "v2" };`
	src3 := `const arr = [plain];`

	tree1, _ := treesitter.Parse([]byte(src1), "test.js")
	tree2, _ := treesitter.Parse([]byte(src2), "test.js")
	tree3, _ := treesitter.Parse([]byte(src3), "test.js")

	key1, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "property_identifier", Line: 1})
	val1, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "string", Line: 1})
	key2, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "property_identifier", Line: 1})
	val2, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "string", Line: 1})
	nonPairNode, _ := testutil.FindNode(tree3, testutil.NodeQuery{Type: "identifier", Line: 1, Nth: 2})

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

func TestGetKeyLabel(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		if got := getKeyLabel(nil); got != "" {
			t.Errorf("getKeyLabel(nil) = %q, want empty", got)
		}
	})

	t.Run("call with identifier in go", func(t *testing.T) {
		src := `package main
func f() { println("hello") }`
		tree, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		call, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 2})
		if got := getKeyLabel(call); got != "println" {
			t.Errorf("getKeyLabel(call) = %q, want %q", got, "println")
		}
	})

	t.Run("call with identifier in c", func(t *testing.T) {
		src := `void f() { doWork(); }`
		tree, err := treesitter.Parse([]byte(src), "test.c")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		call, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 1})
		if got := getKeyLabel(call); got != "doWork" {
			t.Errorf("getKeyLabel(call) = %q, want %q", got, "doWork")
		}
	})

	t.Run("call with property_identifier in javascript", func(t *testing.T) {
		src := `fetchData();`
		tree, err := treesitter.Parse([]byte(src), "test.js")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		call, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "call_expression", Line: 1})
		if got := getKeyLabel(call); got != "fetchData" {
			t.Errorf("getKeyLabel(call) = %q, want %q", got, "fetchData")
		}
	})

	t.Run("pair with direct key label", func(t *testing.T) {
		src := `const obj = { "endpoint": "https://api.example.com" };`
		tree, err := treesitter.Parse([]byte(src), "test.js")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		pair, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "pair", Line: 1})
		if got := getKeyLabel(pair); got != `"endpoint"` {
			t.Errorf("getKeyLabel(pair) = %q, want %q", got, `"endpoint"`)
		}
	})

	t.Run("pair with identifier key label", func(t *testing.T) {
		src := `const obj = { serviceName: "auth" };`
		tree, err := treesitter.Parse([]byte(src), "test.js")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		pair, _ := testutil.FindNode(tree, testutil.NodeQuery{Type: "pair", Line: 1})
		if got := getKeyLabel(pair); got != "serviceName" {
			t.Errorf("getKeyLabel(pair) = %q, want %q", got, "serviceName")
		}
	})
}

func TestFindEnclosingStatement_CaseClause(t *testing.T) {
	src := `package main
func f() {
	switch x {
	default:
		matchDeclarations()
	}
}
`
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	stmt, err := testutil.FindNode(tree, testutil.NodeQuery{Type: "expression_statement", Line: 5})
	if err != nil {
		t.Fatalf("find expr stmt: %v", err)
	}

	got := FindEnclosingStatement(stmt, nil)
	if got != stmt {
		t.Errorf("FindEnclosingStatement(stmt inside default_case) = %v (%s), want %v (%s)", got, got.Type, stmt, stmt.Type)
	}

	innerID, err := testutil.FindNode(tree, testutil.NodeQuery{Type: "identifier", Line: 5})
	if err != nil {
		t.Fatalf("find inner id: %v", err)
	}
	gotInner := FindEnclosingStatement(innerID, nil)
	if gotInner != stmt {
		t.Errorf("FindEnclosingStatement(inner identifier) = %v (%s), want enclosing stmt %v (%s)", gotInner, gotInner.Type, stmt, stmt.Type)
	}
}

func TestIsQuotedLiteral(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{`""`, true},
		{`":"`, true},
		{`"hello world"`, true},
		{`''`, true},
		{`'a'`, true},
		{"``", true},
		{"`raw string`", true},
		{"", false},
		{`"`, false},
		{`'`, false},
		{"`", false},
		{`"mismatched'`, false},
		{"identifier", false},
		{":", false},
		{",", false},
		{";", false},
		{`"unclosed`, false},
		{`unopened"`, false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isQuotedLiteral(tt.input); got != tt.want {
				t.Errorf("isQuotedLiteral(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLeafSimilarity_QuotedLiterals(t *testing.T) {
	src1 := `package main
func f() {
	getContent(dstFile, ":")
}
`
	src2 := `package main
func f() {
	getContent(dstFile, "")
}
`
	tree1, err := treesitter.Parse([]byte(src1), "test.go")
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	tree2, err := treesitter.Parse([]byte(src2), "test.go")
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}

	call1, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "call_expression", Line: 3})
	call2, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "call_expression", Line: 3})

	if sim := LeafSimilarity(call1, call1); sim != 1.0 {
		t.Errorf("LeafSimilarity(call1, call1) = %f, want 1.0", sim)
	}
	if sim := LeafSimilarity(call2, call2); sim != 1.0 {
		t.Errorf("LeafSimilarity(call2, call2) = %f, want 1.0", sim)
	}

	simDiff := LeafSimilarity(call1, call2)
	if simDiff >= 1.0 || simDiff == 0.0 {
		t.Errorf("LeafSimilarity(call1, call2) = %f, want between 0 and 1.0", simDiff)
	}
	if simDiff > 0.8 {
		t.Errorf("LeafSimilarity(call1, call2) = %f, expected < 0.8 due to distinct quoted literals", simDiff)
	}
}

func TestAncestorNameSimilarity_ScopeBounded(t *testing.T) {
	src1 := `package main
var fileLevelGlobal = 1
func doWork() {
	x()
}
`
	src2 := `package main
var fileLevelGlobal = 1
func doWork() {
	y()
}
`
	tree1, err := treesitter.Parse([]byte(src1), "test.go")
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	tree2, err := treesitter.Parse([]byte(src2), "test.go")
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}

	x, err := testutil.FindNode(tree1, testutil.NodeQuery{Type: "identifier", Line: 4})
	if err != nil {
		t.Fatalf("find x: %v", err)
	}
	y, err := testutil.FindNode(tree2, testutil.NodeQuery{Type: "identifier", Line: 4})
	if err != nil {
		t.Fatalf("find y: %v", err)
	}

	sim := AncestorNameSimilarity(x, y)
	if sim != 1 {
		t.Errorf("AncestorNameSimilarity(x, y) = %d, want 1 (only doWork, bounded at function_declaration)", sim)
	}
}

func findFirstNode(root *treesitter.ASTNode, predicate func(*treesitter.ASTNode) bool) *treesitter.ASTNode {
	for _, n := range root.PreOrder() {
		if predicate(n) {
			return n
		}
	}
	return nil
}
