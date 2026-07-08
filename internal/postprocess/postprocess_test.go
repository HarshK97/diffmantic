package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestRunNilEditScript(t *testing.T) {
	ms := engine.NewMapping()
	src := &treesitter.ASTNode{Type: "root"}
	dst := &treesitter.ASTNode{Type: "root"}

	result := Run(nil, ms, src, dst)
	if result != nil {
		t.Error("Run(nil) should return nil")
	}
}

func TestRunEmpty(t *testing.T) {
	es := actions.NewEditScript()
	ms := engine.NewMapping()
	src := &treesitter.ASTNode{Type: "root"}
	dst := &treesitter.ASTNode{Type: "root"}

	result := Run(es, ms, src, dst)
	if result == nil {
		t.Fatal("Run on empty script should not return nil")
	}
	if result.Size() != 0 {
		t.Errorf("empty script should produce 0 actions, got %d", result.Size())
	}
}

func TestRunPreservesInserts(t *testing.T) {
	// Simple insert actions should pass through Run.
	es := actions.NewEditScript()
	node := &treesitter.ASTNode{Type: "id", Label: "x"}
	parent := &treesitter.ASTNode{Type: "block"}
	es.Add(actions.Action{Type: actions.Insert, Node: node, Parent: parent, Position: 0})

	ms := engine.NewMapping()
	src := &treesitter.ASTNode{Type: "root"}
	dst := &treesitter.ASTNode{Type: "root"}

	result := Run(es, ms, src, dst)
	if result.Size() < 1 {
		t.Error("insert action should survive Run")
	}
}
