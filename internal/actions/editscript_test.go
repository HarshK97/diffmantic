package actions

import (
	"bytes"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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
	code := []byte("def foo():\n    x = 1\n")
	ast, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	fnNode := ast.Children[0]

	idNode, err := testutil.FindNodeByText(ast, code, "x")
	if err != nil {
		t.Fatalf("failed to find identifier x: %v", err)
	}

	pyRules := rules.Get("python")
	var blockNode *treesitter.ASTNode
	for p := idNode.Parent; p != nil; p = p.Parent {
		if pyRules.IsBlock(p.Type) {
			blockNode = p
			break
		}
	}
	if blockNode == nil {
		t.Fatal("failed to find block node in parsed snippet")
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
