package actions

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// --- test helpers ---

func makeNode(typ, label string, row, col uint32) *treesitter.ASTNode {
	return &treesitter.ASTNode{
		Type:     typ,
		Label:    label,
		StartRow: row,
		StartCol: col,
		EndRow:   row,
		EndCol:   col + uint32(len(label)),
	}
}

func makeTree(parent *treesitter.ASTNode, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	parent.Children = children
	for _, c := range children {
		c.Parent = parent
	}
	return parent
}

// --- Action type tests ---

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
		if got := tt.aType.String(); got != tt.want {
			t.Errorf("ActionType.String() = %q, want %q", got, tt.want)
		}
	}
}

func TestActionFields(t *testing.T) {
	node := makeNode("identifier", "foo", 1, 0)
	parent := makeNode("block", "", 0, 0)

	// Insert Action
	ins := Action{
		Type:     Insert,
		Node:     node,
		Parent:   parent,
		Position: 3,
	}
	if ins.Type != Insert || ins.Node != node || ins.Parent != parent || ins.Position != 3 {
		t.Error("Insert action fields mismatch")
	}

	// Delete Action
	del := Action{
		Type: Delete,
		Node: node,
	}
	if del.Type != Delete || del.Node != node {
		t.Error("Delete action fields mismatch")
	}

	// Update Action
	upd := Action{
		Type:  Update,
		Node:  node,
		Value: "bar",
	}
	if upd.Type != Update || upd.Node != node || upd.Value != "bar" {
		t.Error("Update action fields mismatch")
	}

	// Move Action
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

// --- EditScript tests ---

func TestEditScriptAddAndSize(t *testing.T) {
	es := NewEditScript()
	if es.Size() != 0 {
		t.Fatalf("empty script size = %d", es.Size())
	}

	node := makeNode("id", "x", 0, 0)
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

// --- NodeToString tests ---

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

	// Without label.
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

	// Nil node.
	if gotNil := NodeToString(nil); gotNil != "<nil>" {
		t.Errorf("NodeToString nil = %q, want \"<nil>\"", gotNil)
	}
}

// --- Tree helpers tests ---

func TestDeepCopyTree(t *testing.T) {
	root := makeNode("module", "", 0, 0)
	child1 := makeNode("function", "foo", 1, 0)
	child2 := makeNode("function", "bar", 5, 0)
	makeTree(root, child1, child2)
	leaf := makeNode("identifier", "x", 2, 4)
	makeTree(child1, leaf)

	cr := deepCopyTree(root)

	if cr.root.Type != "module" {
		t.Fatal("root type wrong")
	}
	if len(cr.root.Children) != 2 {
		t.Fatalf("root children = %d, want 2", len(cr.root.Children))
	}
	if cr.root.Children[0].Label != "foo" {
		t.Fatal("child1 label wrong")
	}
	if len(cr.root.Children[0].Children) != 1 {
		t.Fatal("child1 should have 1 child")
	}

	if cr.origToCopy[root] != cr.root {
		t.Fatal("origToCopy[root] wrong")
	}
	if cr.copyToOrig[cr.root] != root {
		t.Fatal("copyToOrig[root] wrong")
	}

	cr.root.Label = "mutated"
	if root.Label != "" {
		t.Fatal("deep copy is not independent")
	}
}

func TestFakeTree(t *testing.T) {
	child := makeNode("module", "", 0, 0)
	fake := newFakeTree(child)

	if fake.Type != fakeTreeType {
		t.Errorf("FakeTree type = %q, want %q", fake.Type, fakeTreeType)
	}
	if len(fake.Children) != 1 || fake.Children[0] != child {
		t.Fatal("FakeTree should wrap child")
	}
	if child.Parent != fake {
		t.Fatal("child.Parent should be fake")
	}
}

func TestInsertChild(t *testing.T) {
	parent := makeNode("block", "", 0, 0)
	c1 := makeNode("a", "", 0, 0)
	c2 := makeNode("b", "", 0, 0)
	c3 := makeNode("c", "", 0, 0)

	insertChild(parent, c1, 0)
	insertChild(parent, c3, 1)
	insertChild(parent, c2, 1)

	if len(parent.Children) != 3 {
		t.Fatalf("children count = %d, want 3", len(parent.Children))
	}
	if parent.Children[0] != c1 || parent.Children[1] != c2 || parent.Children[2] != c3 {
		t.Fatalf("children order wrong")
	}
	if c2.Parent != parent {
		t.Fatal("inserted child parent not set")
	}
}

func TestPositionInParent(t *testing.T) {
	parent := makeNode("block", "", 0, 0)
	c1 := makeNode("a", "", 0, 0)
	c2 := makeNode("b", "", 0, 0)
	c3 := makeNode("c", "", 0, 0)
	makeTree(parent, c1, c2, c3)

	if p := positionInParent(c1); p != 0 {
		t.Errorf("positionInParent(c1) = %d, want 0", p)
	}
	if p := positionInParent(c2); p != 1 {
		t.Errorf("positionInParent(c2) = %d, want 1", p)
	}
	if p := positionInParent(c3); p != 2 {
		t.Errorf("positionInParent(c3) = %d, want 2", p)
	}

	orphan := makeNode("orphan", "", 0, 0)
	if p := positionInParent(orphan); p != -1 {
		t.Errorf("positionInParent(orphan) = %d, want -1", p)
	}
}

func TestBFS(t *testing.T) {
	root := makeNode("a", "", 0, 0)
	b := makeNode("b", "", 0, 0)
	c := makeNode("c", "", 0, 0)
	d := makeNode("d", "", 0, 0)
	makeTree(root, b, c)
	makeTree(b, d)

	nodes := bfs(root)
	if len(nodes) != 4 {
		t.Fatalf("bfs returned %d nodes, want 4", len(nodes))
	}
	types := ""
	for _, n := range nodes {
		types += n.Type
	}
	if types != "abcd" {
		t.Errorf("bfs order = %q, want %q", types, "abcd")
	}
}

func TestPostOrder(t *testing.T) {
	root := makeNode("a", "", 0, 0)
	b := makeNode("b", "", 0, 0)
	c := makeNode("c", "", 0, 0)
	d := makeNode("d", "", 0, 0)
	makeTree(root, b, c)
	makeTree(b, d)

	nodes := postOrder(root)
	if len(nodes) != 4 {
		t.Fatalf("postOrder returned %d nodes, want 4", len(nodes))
	}
	types := ""
	for _, n := range nodes {
		types += n.Type
	}
	if types != "dbca" {
		t.Errorf("postOrder order = %q, want %q", types, "dbca")
	}
}
