package actions

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestChawatheIdenticalTrees(t *testing.T) {
	code := []byte("package main\n\nfunc foo() {\n\tx := 1\n}\n")
	src, err := treesitter.Parse(code, "main.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(code, "main.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(src, dst, code, code, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)
	if es.Size() != 0 {
		t.Errorf("identical trees should produce 0 actions, got %d", es.Size())
		for _, a := range es.Actions() {
			t.Logf("  %s: %s", a.Type, a.String())
		}
	}
}

func TestChawatheInsertLeaf(t *testing.T) {
	srcCode := []byte("package main\n\nfunc foo() {\n\tx := 1\n}\n")
	dstCode := []byte("package main\n\nfunc foo() {\n\tx := 1\n\ty := 2\n}\n")
	src, err := treesitter.Parse(srcCode, "main.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "main.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	dstY, err := testutil.FindNodeByText(dst, dstCode, "y")
	if err != nil {
		t.Fatalf("failed to find identifier y in dst: %v", err)
	}

	matchResult := engine.Match(src, dst, srcCode, dstCode, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)

	assertHasAction(t, es, Insert, dstY)
}

func TestChawatheDeleteLeaf(t *testing.T) {
	srcCode := []byte("package main\n\nfunc foo() {\n\tx := 1\n\ty := 2\n}\n")
	dstCode := []byte("package main\n\nfunc foo() {\n\tx := 1\n}\n")
	src, err := treesitter.Parse(srcCode, "main.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "main.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	srcY, err := testutil.FindNodeByText(src, srcCode, "y")
	if err != nil {
		t.Fatalf("failed to find identifier y in src: %v", err)
	}

	matchResult := engine.Match(src, dst, srcCode, dstCode, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)

	assertHasAction(t, es, Delete, srcY)
}

func TestChawatheUpdateLeaf(t *testing.T) {
	srcCode := []byte("old_val = 1\n")
	dstCode := []byte("new_val = 1\n")
	src, err := treesitter.Parse(srcCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	srcID, err := testutil.FindNodeByText(src, srcCode, "old_val")
	if err != nil {
		t.Fatalf("failed to find identifier old_val in src: %v", err)
	}

	matchResult := engine.Match(src, dst, srcCode, dstCode, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)

	assertHasUpdateAction(t, es, srcID, "new_val")
}

// --- PrintActions test ---

func TestPrintActions(t *testing.T) {
	ast, err := treesitter.Parse([]byte("foo = 1\n"), "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	es := NewEditScript()
	parent := ast
	node := ast.Children[0]
	es.Add(Action{Type: Insert, Node: node, Parent: parent, Position: 0})
	es.Add(Action{Type: Delete, Node: node})
	es.Add(Action{Type: Update, Node: node, Value: "bar"})

	var buf strings.Builder
	_ = FprintActions(&buf, es)
	output := buf.String()

	if !strings.Contains(output, "insert") {
		t.Error("output missing insert")
	}
	if !strings.Contains(output, "delete") {
		t.Error("output missing delete")
	}
	if !strings.Contains(output, "update") {
		t.Error("output missing update")
	}
	if !strings.Contains(output, "Total actions: 3") {
		t.Error("output missing total count")
	}
}

func TestPrintActionsEmpty(t *testing.T) {
	var buf strings.Builder
	_ = FprintActions(&buf, NewEditScript())
	if !strings.Contains(buf.String(), "no edit actions") {
		t.Error("empty script should print 'no edit actions'")
	}
}
