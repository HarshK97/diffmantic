package actions

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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
	node := testutil.NodeAtRC("identifier", "foo", 1, 0)
	parent := testutil.NodeAtRC("block", "", 0, 0)

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

	node := testutil.NodeAtRC("id", "x", 0, 0)
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
	n := &treesitter.ASTNode{
		Type:     "identifier",
		Label:    "foo",
		StartRow: 1, // displays as 2
		StartCol: 4,
		EndRow:   1,
		EndCol:   7,
	}
	got := NodeToString(n)
	want := "identifier: foo [2:4-2:7]"
	if got != want {
		t.Errorf("NodeToString = %q, want %q", got, want)
	}

	n2 := &treesitter.ASTNode{
		Type:     "block",
		StartRow: 0,
		StartCol: 0,
		EndRow:   5,
		EndCol:   1,
	}
	got2 := NodeToString(n2)
	want2 := "block [1:0-6:1]"
	if got2 != want2 {
		t.Errorf("NodeToString no-label = %q, want %q", got2, want2)
	}

	if gotNil := NodeToString(nil); gotNil != "<nil>" {
		t.Errorf("NodeToString nil = %q, want \"<nil>\"", gotNil)
	}
}

func TestDeepCopyTree(t *testing.T) {
	root := testutil.NodeAtRC("module", "", 0, 0)
	root.ID = 0
	child1 := testutil.NodeAtRC("function", "foo", 1, 0)
	child1.ID = 1
	child2 := testutil.NodeAtRC("function", "bar", 5, 0)
	child2.ID = 2
	testutil.Tree(root, child1, child2)
	leaf := testutil.NodeAtRC("identifier", "x", 2, 4)
	leaf.ID = 3
	testutil.Tree(child1, leaf)

	cs := &chawatheState{}
	cs.init(root, root, engine.NewMapping())

	if cs.cpySrc.nodeType != "module" {
		t.Fatal("root type wrong")
	}
	if len(cs.cpySrc.children) != 2 {
		t.Fatalf("root children = %d, want 2", len(cs.cpySrc.children))
	}
	if cs.cpySrc.children[0].label != "foo" {
		t.Fatal("child1 label wrong")
	}
	if len(cs.cpySrc.children[0].children) != 1 {
		t.Fatal("child1 should have 1 child")
	}

	if cs.origToCopy[root] != cs.cpySrc {
		t.Fatal("origToCopy[root] wrong")
	}
	if cs.cpySrc.orig != root {
		t.Fatal("copyToOrig[root] wrong")
	}

	cs.cpySrc.label = "mutated"
	if root.Label != "" {
		t.Fatal("deep copy is not independent")
	}
}

func TestInsertChild(t *testing.T) {
	parent := &cnode{nodeType: "block"}
	c1 := &cnode{nodeType: "a"}
	c2 := &cnode{nodeType: "b"}
	c3 := &cnode{nodeType: "c"}

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
	parent := testutil.NodeAtRC("block", "", 0, 0)
	c1 := testutil.NodeAtRC("a", "", 0, 0)
	c2 := testutil.NodeAtRC("b", "", 0, 0)
	c3 := testutil.NodeAtRC("c", "", 0, 0)
	testutil.Tree(parent, c1, c2, c3)

	if p := c1.ChildIndex(); p != 0 {
		t.Errorf("c1.ChildIndex() = %d, want 0", p)
	}
	if p := c2.ChildIndex(); p != 1 {
		t.Errorf("c2.ChildIndex() = %d, want 1", p)
	}
	if p := c3.ChildIndex(); p != 2 {
		t.Errorf("c3.ChildIndex() = %d, want 2", p)
	}

	orphan := testutil.NodeAtRC("orphan", "", 0, 0)
	if p := orphan.ChildIndex(); p != -1 {
		t.Errorf("orphan.ChildIndex() = %d, want -1", p)
	}
}

func TestBFS(t *testing.T) {
	root := testutil.NodeAtRC("a", "", 0, 0)
	b := testutil.NodeAtRC("b", "", 0, 0)
	c := testutil.NodeAtRC("c", "", 0, 0)
	d := testutil.NodeAtRC("d", "", 0, 0)
	testutil.Tree(root, b, c)
	testutil.Tree(b, d)

	nodes := root.LevelOrder()
	if len(nodes) != 4 {
		t.Fatalf("bfs returned %d nodes, want 4", len(nodes))
	}
	var types strings.Builder
	for _, n := range nodes {
		types.WriteString(n.Type)
	}
	if types.String() != "abcd" {
		t.Errorf("bfs order = %q, want %q", types.String(), "abcd")
	}
}

func TestPostOrder(t *testing.T) {
	root := testutil.NodeAtRC("a", "", 0, 0)
	b := testutil.NodeAtRC("b", "", 0, 0)
	c := testutil.NodeAtRC("c", "", 0, 0)
	d := testutil.NodeAtRC("d", "", 0, 0)
	testutil.Tree(root, b, c)
	testutil.Tree(b, d)

	nodes := root.PostOrder()
	if len(nodes) != 4 {
		t.Fatalf("postOrder returned %d nodes, want 4", len(nodes))
	}
	var types strings.Builder
	for _, n := range nodes {
		types.WriteString(n.Type)
	}
	if types.String() != "dbca" {
		t.Errorf("postOrder order = %q, want %q", types.String(), "dbca")
	}
}
