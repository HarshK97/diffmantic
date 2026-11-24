package treesitter

import (
	"slices"
	"testing"
)

func TestASTNodeLabeling(t *testing.T) {
	pySrc := []byte(`
def test_func():
    cls()
    cls(name="test_str", val=123)
    a = 1 + 2
`)

	ast, err := Parse(pySrc, "test.py")
	if err != nil {
		t.Fatalf("failed to parse python snippet: %v", err)
	}

	// Helper to find all nodes matching type
	var findNodes func(n *ASTNode, targetType string) []*ASTNode
	findNodes = func(n *ASTNode, targetType string) []*ASTNode {
		if n == nil {
			return nil
		}
		var res []*ASTNode
		if n.Type == targetType {
			res = append(res, n)
		}
		for _, child := range n.Children {
			res = append(res, findNodes(child, targetType)...)
		}
		return res
	}

	// (a) Test empty argument_list gets Label == ""
	argLists := findNodes(ast, "argument_list")
	if len(argLists) < 2 {
		t.Fatalf("expected at least 2 argument_list nodes, found %d", len(argLists))
	}

	emptyArgList := argLists[0]
	if emptyArgList.Label != "" {
		t.Errorf("expected empty argument_list to have empty Label, got %q", emptyArgList.Label)
	}

	// (b) Test non-empty argument_list gets Label == ""
	nonEmptyArgList := argLists[1]
	if nonEmptyArgList.Label != "" {
		t.Errorf("expected non-empty argument_list to have empty Label, got %q", nonEmptyArgList.Label)
	}

	// (c) Test genuine leaf nodes get correct text labels
	identifiers := findNodes(ast, "identifier")
	if len(identifiers) == 0 {
		t.Fatal("expected identifier nodes, found 0")
	}
	foundTestFunc := slices.ContainsFunc(identifiers, func(n *ASTNode) bool {
		return n.Label == "test_func"
	})
	if !foundTestFunc {
		t.Error("expected to find identifier 'test_func' with label 'test_func'")
	}

	integers := findNodes(ast, "integer")
	if len(integers) == 0 {
		t.Fatal("expected integer nodes, found 0")
	}
	found123 := slices.ContainsFunc(integers, func(n *ASTNode) bool {
		return n.Label == "123"
	})
	if !found123 {
		t.Error("expected to find integer node with label '123'")
	}

	// (d) Test string literal gets label via isStringLiteralType path regardless of child count
	strings := findNodes(ast, "string")
	if len(strings) == 0 {
		t.Fatal("expected string nodes, found 0")
	}
	strNode := strings[0]
	if strNode.Label != `"test_str"` {
		t.Errorf("expected string node to have label %q, got %q", `"test_str"`, strNode.Label)
	}
}

func findFirstASTNode(n *ASTNode, pred func(n *ASTNode) bool) *ASTNode {
	if n == nil {
		return nil
	}
	if pred(n) {
		return n
	}
	for _, child := range n.Children {
		if found := findFirstASTNode(child, pred); found != nil {
			return found
		}
	}
	return nil
}

func TestDescendants(t *testing.T) {
	src := []byte("package main\nfunc f() { a(x, y) }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
	if call == nil {
		t.Fatal("call_expression not found")
	}

	desc := call.Descendants()
	if len(desc) == 0 {
		t.Fatalf("want non-empty descendants, got %d", len(desc))
	}
	pre := call.PreOrder()
	if len(desc) != len(pre)-1 {
		t.Fatalf("want %d descendants, got %d", len(pre)-1, len(desc))
	}
	for i, d := range desc {
		if d != pre[i+1] {
			t.Errorf("descendant at %d does not match pre-order: got %v, want %v", i, d, pre[i+1])
		}
	}
}

func TestDescendantsNested(t *testing.T) {
	src := []byte("package main\nfunc f() { x() }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fn := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "function_declaration" })
	if fn == nil {
		t.Fatal("function_declaration not found")
	}

	desc := fn.Descendants()
	if len(desc) < 3 {
		t.Fatalf("want at least 3 descendants, got %d", len(desc))
	}
	hasCall := slices.ContainsFunc(desc, func(n *ASTNode) bool { return n.Type == "call_expression" })
	hasX := slices.ContainsFunc(desc, func(n *ASTNode) bool { return n.Label == "x" })
	if !hasCall || !hasX {
		t.Error("nested descendants missing expected call_expression or identifier 'x'")
	}
}

func TestPostOrder(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.PostOrder() != nil {
		t.Error("nilNode.PostOrder() should return nil")
	}

	src := []byte("package main\nfunc f() { x() }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
	if call == nil {
		t.Fatal("call_expression not found")
	}

	order := call.PostOrder()
	if len(order) == 0 {
		t.Fatalf("want non-empty post-order traversal, got %d", len(order))
	}
	if order[len(order)-1] != call {
		t.Errorf("last node in post-order should be the root %v, got %v", call, order[len(order)-1])
	}
	pos := make(map[*ASTNode]int)
	for i, n := range order {
		pos[n] = i
	}
	for _, n := range order {
		for _, child := range n.Children {
			if pos[child] >= pos[n] {
				t.Errorf("child %v (pos %d) should visit before parent %v (pos %d) in post-order", child, pos[child], n, pos[n])
			}
		}
	}
}

func TestPreOrder(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.PreOrder() != nil {
		t.Error("nilNode.PreOrder() should return nil")
	}

	src := []byte("package main\nfunc f() { x() }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
	if call == nil {
		t.Fatal("call_expression not found")
	}

	order := call.PreOrder()
	if len(order) == 0 {
		t.Fatalf("want non-empty pre-order traversal, got %d", len(order))
	}
	if order[0] != call {
		t.Errorf("first node in pre-order should be the root %v, got %v", call, order[0])
	}
	pos := make(map[*ASTNode]int)
	for i, n := range order {
		pos[n] = i
	}
	for _, n := range order {
		for _, child := range n.Children {
			if pos[n] >= pos[child] {
				t.Errorf("parent %v (pos %d) should visit before child %v (pos %d) in pre-order", n, pos[n], child, pos[child])
			}
		}
	}
}

func TestLeaves(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.Leaves() != nil {
		t.Error("nilNode.Leaves() should return nil")
	}

	src := []byte("package main\nfunc f() { a(x, y) }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
	if call == nil {
		t.Fatal("call_expression not found")
	}

	leaves := call.Leaves()
	if len(leaves) < 3 {
		t.Fatalf("want at least 3 leaves for call, got %d", len(leaves))
	}
	for _, l := range leaves {
		if !l.IsLeafOrStringLiteral() {
			t.Errorf("node %v returned by Leaves() is not a leaf or string literal", l)
		}
	}
	hasA := slices.ContainsFunc(leaves, func(n *ASTNode) bool { return n.Label == "a" })
	hasX := slices.ContainsFunc(leaves, func(n *ASTNode) bool { return n.Label == "x" })
	hasY := slices.ContainsFunc(leaves, func(n *ASTNode) bool { return n.Label == "y" })
	if !hasA || !hasX || !hasY {
		t.Error("expected leaves to contain identifiers 'a', 'x', and 'y'")
	}
}

func TestLevelOrder(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.LevelOrder() != nil {
		t.Error("nilNode.LevelOrder() should return nil")
	}

	src := []byte("package main\nfunc f() { a(x, y) }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
	if call == nil {
		t.Fatal("call_expression not found")
	}

	order := call.LevelOrder()
	if len(order) == 0 {
		t.Fatalf("want non-empty level-order traversal, got %d", len(order))
	}
	if order[0] != call {
		t.Errorf("level-order should visit root first, got %v", order[0])
	}
	depth := make(map[*ASTNode]int)
	depth[call] = 0
	var calcDepth func(n *ASTNode, d int)
	calcDepth = func(n *ASTNode, d int) {
		depth[n] = d
		for _, c := range n.Children {
			calcDepth(c, d+1)
		}
	}
	calcDepth(call, 0)
	for i := 1; i < len(order); i++ {
		if depth[order[i]] < depth[order[i-1]] {
			t.Errorf("level-order violation: node at index %d has depth %d < node at %d depth %d",
				i, depth[order[i]], i-1, depth[order[i-1]])
		}
	}
}

func TestIsLeafOrStringLiteral(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.IsLeafOrStringLiteral() {
		t.Error("nilNode.IsLeafOrStringLiteral() should be false")
	}

	src := []byte("package main\nfunc f() { s := \"hello\" }\n")
	root, err := Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	leaf := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "identifier" && n.Label == "s" })
	if leaf == nil || !leaf.IsLeafOrStringLiteral() {
		t.Errorf("expected identifier 's' to be leaf or string literal: %v", leaf)
	}

	str := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "interpreted_string_literal" })
	if str == nil || !str.IsLeafOrStringLiteral() {
		t.Errorf("expected interpreted_string_literal to be leaf or string literal: %v", str)
	}

	parent := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "function_declaration" })
	if parent == nil || parent.IsLeafOrStringLiteral() {
		t.Errorf("expected function_declaration not to be leaf: %v", parent)
	}
}

func TestASTNodeKeywordsOnlyOnLeaves(t *testing.T) {
	src := []byte(`
def foo():
    if True:
        pass
`)
	ast, err := Parse(src, "test.py")
	if err != nil {
		t.Fatalf("failed to parse python snippet: %v", err)
	}

	for _, n := range ast.LevelOrder() {
		if !n.IsLeafOrStringLiteral() && n.IsKeyword {
			t.Errorf("non-leaf node %q should not be tagged as keyword", n.Type)
		}
	}
}

func TestCalleePath(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		lang     string
		expected string
	}{
		{
			name:     "bare identifier (exit)",
			src:      "package main\nfunc f() { exit() }\n",
			lang:     "test.go",
			expected: "exit",
		},
		{
			name:     "selector expression (os.Exit)",
			src:      "package main\nfunc f() { os.Exit(1) }\n",
			lang:     "test.go",
			expected: "os.Exit",
		},
		{
			name:     "member expression (process.exit)",
			src:      "function f() { process.exit(0); }\n",
			lang:     "test.js",
			expected: "process.exit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, err := Parse([]byte(tt.src), tt.lang)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			call := findFirstASTNode(root, func(n *ASTNode) bool { return n.Type == "call_expression" })
			if call == nil {
				t.Fatal("call_expression not found")
			}
			got := CalleePath(call)
			if got != tt.expected {
				t.Errorf("CalleePath() = %q, want %q", got, tt.expected)
			}
		})
	}

	t.Run("nil call node", func(t *testing.T) {
		if got := CalleePath(nil); got != "" {
			t.Errorf("CalleePath(nil) = %q, want empty string", got)
		}
	})
	t.Run("empty call node", func(t *testing.T) {
		var empty ASTNode
		if got := CalleePath(&empty); got != "" {
			t.Errorf("CalleePath(&empty) = %q, want empty string", got)
		}
	})
}

func TestEnclosingContainerDeclaration_Closures(t *testing.T) {
	goSrc := []byte(`
package main

func outer() {
	fn := func() {
		x := 42
		_ = x
	}
	_ = fn
}
`)
	ast, err := Parse(goSrc, "main.go")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	desc := ast.Descendants()
	idx := slices.IndexFunc(desc, func(n *ASTNode) bool {
		return n.Type == "int_literal"
	})
	if idx == -1 {
		t.Fatal("int_literal not found")
	}
	numLit := desc[idx]

	enclosing := numLit.EnclosingContainerDeclaration(nil)
	if enclosing == nil {
		t.Fatal("expected enclosing container declaration, got nil")
	}
	if enclosing.Type != "func_literal" {
		t.Errorf("expected enclosing container declaration to be func_literal, got %q", enclosing.Type)
	}
}
