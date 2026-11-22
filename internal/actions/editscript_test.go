package actions

import (
	"bytes"
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestEditScriptActionsSlice(t *testing.T) {
	ast, err := treesitter.Parse([]byte("x = 1\n"), "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	node := ast.Children[0]

	es := NewEditScript()
	a := Action{Type: Update, Node: node, Value: "y"}
	es.Add(a)

	acts := es.Actions()
	if len(acts) != 1 {
		t.Fatalf("want 1 action, got %d", len(acts))
	}
	if acts[0].Type != Update || acts[0].Value != "y" {
		t.Error("action fields should be preserved")
	}
}

func TestFprintActionsNil(t *testing.T) {
	var buf bytes.Buffer
	_ = FprintActions(&buf, nil)
	if buf.String() != "(no edit actions)\n" {
		t.Errorf("nil script output = %q", buf.String())
	}
}

func TestFprintActionsContainsDetails(t *testing.T) {
	// Verify subtree and move details appear in formatted output.
	ast, err := treesitter.Parse([]byte("def foo():\n    x = 1\n"), "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	fnNode := ast.Children[0]
	var blockNode, idNode *treesitter.ASTNode
	for _, n := range ast.Descendants() {
		if n.Type == "block" && blockNode == nil {
			blockNode = n
		}
		if n.Type == "identifier" && n.Label == "x" && idNode == nil {
			idNode = n
		}
	}
	if blockNode == nil || idNode == nil {
		t.Fatalf("failed to find child nodes in parsed snippet: block=%v, id=%v", blockNode, idNode)
	}

	es := NewEditScript()
	es.Add(Action{Type: Insert, Node: blockNode, Parent: fnNode, Position: 2, Subtree: true})
	es.Add(Action{Type: Move, Node: blockNode, Parent: fnNode, Position: 1})
	es.Add(Action{Type: Update, Node: idNode, Value: "new"})

	var buf bytes.Buffer
	_ = FprintActions(&buf, es)
	out := buf.String()

	if !bytes.Contains([]byte(out), []byte("[subtree]")) {
		t.Error("subtree insert should show [subtree]")
	}
	if !bytes.Contains([]byte(out), []byte("move")) {
		t.Error("should contain move action")
	}
	if !bytes.Contains([]byte(out), []byte(`val="new"`)) {
		t.Error("update should show value")
	}
	if !bytes.Contains([]byte(out), []byte("Total actions: 3")) {
		t.Error("should show total count")
	}
}
