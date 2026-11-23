package actions

import (
	"fmt"
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestActionNames(t *testing.T) {
	tests := []struct {
		aType ActionType
		want  string
	}{
		{Insert, "insert"},
		{Delete, "delete"},
		{Update, "update"},
		{Move, "move"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.aType.String(); got != tt.want {
				t.Errorf("ActionType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestActionFields(t *testing.T) {
	ast, err := treesitter.Parse([]byte("package main\n\nfunc foo() {}\n"), "main.go")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	parent := ast
	node := ast.Children[len(ast.Children)-1]

	ins := Action{
		Type:     Insert,
		Node:     node,
		Parent:   parent,
		Position: 3,
	}
	if ins.Type != Insert || ins.Node != node || ins.Parent != parent || ins.Position != 3 {
		t.Error("Insert action fields mismatch")
	}

	del := Action{
		Type: Delete,
		Node: node,
	}
	if del.Type != Delete || del.Node != node {
		t.Error("Delete action fields mismatch")
	}

	upd := Action{
		Type:  Update,
		Node:  node,
		Value: "bar",
	}
	if upd.Type != Update || upd.Node != node || upd.Value != "bar" {
		t.Error("Update action fields mismatch")
	}

	mv := Action{
		Type:     Move,
		Node:     node,
		Parent:   parent,
		Position: 5,
	}
	if mv.Type != Move || mv.Node != node || mv.Parent != parent || mv.Position != 5 {
		t.Error("Move action fields mismatch")
	}
}

func TestEditScriptAddAndSize(t *testing.T) {
	es := NewEditScript()
	if es.Size() != 0 {
		t.Fatalf("empty script size = %d", es.Size())
	}

	ast, err := treesitter.Parse([]byte("x = 1\n"), "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	node := ast.Children[0]
	es.Add(Action{Type: Delete, Node: node})
	es.Add(Action{Type: Delete, Node: node})

	if es.Size() != 2 {
		t.Fatalf("script size = %d, want 2", es.Size())
	}

	actions := es.Actions()
	if len(actions) != 2 {
		t.Fatalf("actions slice len = %d, want 2", len(actions))
	}
}

func TestNodeToString(t *testing.T) {
	code := []byte("package main\n\nfunc foo() {\n\treturn\n}\n")
	ast, err := treesitter.Parse(code, "test.go")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	identNode, err := testutil.FindNodeByText(ast, code, "foo")
	if err != nil {
		t.Fatalf("failed to locate identNode: %v", err)
	}
	returnNode, err := testutil.FindNodeByText(ast, code, "return")
	if err != nil {
		t.Fatalf("failed to locate returnNode: %v", err)
	}
	goRules := rules.Get("go")
	var blockNode *treesitter.ASTNode
	for p := returnNode.Parent; p != nil; p = p.Parent {
		if goRules.IsBlock(p.Type) {
			blockNode = p
			break
		}
	}
	if blockNode == nil {
		t.Fatal("failed to locate blockNode")
	}

	got := NodeToString(identNode)
	want := fmt.Sprintf("%s: %s [%d:%d-%d:%d]", identNode.Type, identNode.Label, identNode.StartRow+1, identNode.StartCol, identNode.EndRow+1, identNode.EndCol)
	if got != want {
		t.Errorf("NodeToString = %q, want %q", got, want)
	}

	got2 := NodeToString(blockNode)
	want2 := fmt.Sprintf("%s [%d:%d-%d:%d]", blockNode.Type, blockNode.StartRow+1, blockNode.StartCol, blockNode.EndRow+1, blockNode.EndCol)
	if got2 != want2 {
		t.Errorf("NodeToString no-label = %q, want %q", got2, want2)
	}

	if gotNil := NodeToString(nil); gotNil != "<nil>" {
		t.Errorf("NodeToString nil = %q, want \"<nil>\"", gotNil)
	}
}

func TestDeepCopyTree(t *testing.T) {
	code := []byte("def foo():\n    x = 1\n\ndef bar():\n    pass\n")
	root, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	cs := &chawatheState{}
	cs.init(root, root, engine.NewMapping())

	if cs.cpySrc == nil || cs.cpySrc.orig != root {
		t.Fatal("root copy wrong")
	}
	if len(cs.cpySrc.children) != len(root.Children) {
		t.Fatalf("root children = %d, want %d", len(cs.cpySrc.children), len(root.Children))
	}
	if cs.origToCopy[root] != cs.cpySrc {
		t.Fatal("origToCopy[root] wrong")
	}
	if cs.cpySrc.orig != root {
		t.Fatal("copyToOrig[root] wrong")
	}

	cs.cpySrc.label = "mutated"
	if root.Label == "mutated" {
		t.Fatal("deep copy is not independent")
	}
}

func TestInsertChild(t *testing.T) {
	code := []byte("a = 1\nb = 2\nc = 3\n")
	ast, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	cs := &chawatheState{}
	cs.init(ast, ast, engine.NewMapping())
	parent := cs.cpySrc
	if len(parent.children) < 3 {
		t.Fatalf("expected at least 3 children, got %d", len(parent.children))
	}
	c1 := parent.children[0]
	c2 := parent.children[1]
	c3 := parent.children[2]

	parent.children = nil
	c1.parent = nil
	c2.parent = nil
	c3.parent = nil

	insertChild(parent, c1, 0)
	insertChild(parent, c3, 1)
	insertChild(parent, c2, 1)

	if len(parent.children) != 3 {
		t.Fatalf("children count = %d, want 3", len(parent.children))
	}
	if parent.children[0] != c1 || parent.children[1] != c2 || parent.children[2] != c3 {
		t.Fatalf("children order wrong")
	}
	if c2.parent != parent {
		t.Fatal("inserted child parent not set")
	}
}

func TestPositionInParent(t *testing.T) {
	code := []byte("a = 1\nb = 2\nc = 3\n")
	parent, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(parent.Children) < 3 {
		t.Fatalf("expected at least 3 children, got %d", len(parent.Children))
	}
	c1 := parent.Children[0]
	c2 := parent.Children[1]
	c3 := parent.Children[2]

	if p := c1.ChildIndex(); p != 0 {
		t.Errorf("c1.ChildIndex() = %d, want 0", p)
	}
	if p := c2.ChildIndex(); p != 1 {
		t.Errorf("c2.ChildIndex() = %d, want 1", p)
	}
	if p := c3.ChildIndex(); p != 2 {
		t.Errorf("c3.ChildIndex() = %d, want 2", p)
	}

	// Root node has no parent, so ChildIndex() returns -1
	if p := parent.ChildIndex(); p != -1 {
		t.Errorf("parent.ChildIndex() = %d, want -1", p)
	}
}

func TestBFS(t *testing.T) {
	code := []byte("def f():\n    pass\n")
	root, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	nodes := root.LevelOrder()
	if len(nodes) != root.Size() {
		t.Fatalf("bfs returned %d nodes, want %d", len(nodes), root.Size())
	}
	if nodes[0] != root {
		t.Fatalf("bfs root should be first, got %v", nodes[0])
	}

	// In BFS, every non-root node's parent must appear before it in the sequence
	for i, n := range nodes {
		if n.Parent != nil {
			parentIdx := slices.Index(nodes, n.Parent)
			if parentIdx == -1 || parentIdx >= i {
				t.Errorf("node %s at index %d has parent at index %d", n.Type, i, parentIdx)
			}
		}
	}
}

func TestPostOrder(t *testing.T) {
	code := []byte("def f():\n    pass\n")
	root, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	nodes := root.PostOrder()
	if len(nodes) != root.Size() {
		t.Fatalf("postOrder returned %d nodes, want %d", len(nodes), root.Size())
	}
	if nodes[len(nodes)-1] != root {
		t.Fatalf("postOrder root should be last, got %v", nodes[len(nodes)-1])
	}

	// In PostOrder, every child must appear before its parent in the sequence
	for i, n := range nodes {
		for _, child := range n.Children {
			childIdx := slices.Index(nodes, child)
			if childIdx == -1 || childIdx >= i {
				t.Errorf("parent %s at index %d has child %s at index %d", n.Type, i, child.Type, childIdx)
			}
		}
	}
}
