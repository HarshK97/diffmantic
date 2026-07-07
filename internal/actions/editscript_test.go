package actions

import (
	"bytes"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestEditScriptActionsSlice(t *testing.T) {
	es := NewEditScript()
	a := Action{Type: Update, Node: &treesitter.ASTNode{Type: "id", Label: "x"}, Value: "y"}
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
	FprintActions(&buf, nil)
	if buf.String() != "(no edit actions)\n" {
		t.Errorf("nil script output = %q", buf.String())
	}
}

func TestFprintActionsContainsDetails(t *testing.T) {
	// Verify subtree and move details appear in formatted output.
	es := NewEditScript()
	node := &treesitter.ASTNode{Type: "block", Label: ""}
	parent := &treesitter.ASTNode{Type: "func"}
	es.Add(Action{Type: Insert, Node: node, Parent: parent, Position: 2, Subtree: true})
	es.Add(Action{Type: Move, Node: node, Parent: parent, Position: 1})
	es.Add(Action{Type: Update, Node: &treesitter.ASTNode{Type: "id", Label: "old"}, Value: "new"})

	var buf bytes.Buffer
	FprintActions(&buf, es)
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

func TestBuildJSONSubtreeFlag(t *testing.T) {
	es := NewEditScript()
	node := &treesitter.ASTNode{Type: "block"}
	es.Add(Action{Type: Insert, Node: node, Parent: &treesitter.ASTNode{Type: "func"}, Position: 0, Subtree: true})

	ms := engine.NewMapping()
	out := buildJSONOutput(es, ms)

	if len(out.Actions) != 1 {
		t.Fatalf("want 1 action, got %d", len(out.Actions))
	}
	if out.Actions[0].Subtree == nil || !*out.Actions[0].Subtree {
		t.Error("subtree flag should be set to true")
	}
}

func TestBuildJSONNoSubtreeOmitted(t *testing.T) {
	es := NewEditScript()
	node := &treesitter.ASTNode{Type: "id", Label: "x"}
	es.Add(Action{Type: Delete, Node: node, Subtree: false})

	ms := engine.NewMapping()
	out := buildJSONOutput(es, ms)

	if out.Actions[0].Subtree != nil {
		t.Error("subtree=false should be omitted (nil pointer)")
	}
}
