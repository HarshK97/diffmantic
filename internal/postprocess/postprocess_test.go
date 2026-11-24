package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestRunNilEditScript(t *testing.T) {
	ms := engine.NewMapping()
	src, err := treesitter.Parse([]byte("package main\n"), "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dst, err := treesitter.Parse([]byte("package main\n"), "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	result := Run(nil, ms, src, dst)
	if result != nil {
		t.Error("Run(nil) should return nil")
	}
}

func TestRunEmpty(t *testing.T) {
	es := actions.NewEditScript()
	ms := engine.NewMapping()
	src, err := treesitter.Parse([]byte("package main\n"), "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dst, err := treesitter.Parse([]byte("package main\n"), "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

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
	srcBytes := []byte("package main\n")
	dstBytes := []byte("package main\nfunc f() { x := 1 }\n")
	src, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	node, err := testutil.FindNodeByText(dst, dstBytes, "x", "identifier")
	if err != nil {
		t.Fatalf("find node: %v", err)
	}
	parent := node.Parent

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: node, Parent: parent, Position: 0})

	ms := engine.NewMapping()
	result := Run(es, ms, src, dst)
	if result.Size() < 1 {
		t.Error("insert action should survive Run")
	}
}
