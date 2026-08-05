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

func TestDescendants(t *testing.T) {
	c1 := &ASTNode{Type: "id", Label: "a"}
	c2 := &ASTNode{Type: "id", Label: "b"}
	root := &ASTNode{Type: "call", Children: []*ASTNode{c1, c2}}

	desc := root.Descendants()
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
	if desc[0] != c1 || desc[1] != c2 {
		t.Error("descendants not in expected order")
	}
}

func TestDescendantsNested(t *testing.T) {
	leaf := &ASTNode{Type: "id", Label: "x"}
	mid := &ASTNode{Type: "call", Children: []*ASTNode{leaf}}
	root := &ASTNode{Type: "func", Children: []*ASTNode{mid}}

	desc := root.Descendants()
	if len(desc) != 2 {
		t.Fatalf("want 2 descendants, got %d", len(desc))
	}
}

func TestPostOrder(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.PostOrder() != nil {
		t.Error("nilNode.PostOrder() should return nil")
	}
	leaf := &ASTNode{Type: "id", Label: "x"}
	root := &ASTNode{Type: "call", Children: []*ASTNode{leaf}}
	order := root.PostOrder()

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != leaf || order[1] != root {
		t.Error("post-order should visit child before parent")
	}
}

func TestPreOrder(t *testing.T) {
	var nilNode *ASTNode
	if nilNode.PreOrder() != nil {
		t.Error("nilNode.PreOrder() should return nil")
	}
	leaf := &ASTNode{Type: "id", Label: "x"}
	root := &ASTNode{Type: "call", Children: []*ASTNode{leaf}}
	order := root.PreOrder()

	if len(order) != 2 {
		t.Fatalf("want 2 nodes, got %d", len(order))
	}
	if order[0] != root || order[1] != leaf {
		t.Error("pre-order should visit parent before child")
	}
}
