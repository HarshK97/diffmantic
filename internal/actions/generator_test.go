package actions

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
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

	srcNodes := src.LevelOrder()
	dstNodes := dst.LevelOrder()
	ms := engine.NewMapping()
	for i := range srcNodes {
		ms.Add(srcNodes[i], dstNodes[i])
	}

	es := GenerateEditScript(src, dst, ms)
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

	var dstY *treesitter.ASTNode
	for _, n := range dst.PostOrder() {
		if n.Label == "y" {
			dstY = n
			break
		}
	}
	if dstY == nil {
		t.Fatal("failed to find identifier y in dst")
	}

	matchResult := engine.Match(src, dst, srcCode, dstCode, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Insert && a.Node == dstY {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Insert action for dstY")
		for _, a := range es.Actions() {
			t.Logf("  %s: node=%v", a.Type, a.Node)
		}
	}
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

	var srcY *treesitter.ASTNode
	for _, n := range src.PostOrder() {
		if n.Label == "y" {
			srcY = n
			break
		}
	}
	if srcY == nil {
		t.Fatal("failed to find identifier y in src")
	}

	matchResult := engine.Match(src, dst, srcCode, dstCode, nil)
	es := GenerateEditScript(src, dst, matchResult.Mappings)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Delete && a.Node == srcY {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Delete action for srcY")
		for _, a := range es.Actions() {
			t.Logf("  %s: node=%v", a.Type, a.Node)
		}
	}
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

	var srcID, dstID *treesitter.ASTNode
	for _, n := range src.PostOrder() {
		if n.Label == "old_val" {
			srcID = n
			break
		}
	}
	for _, n := range dst.PostOrder() {
		if n.Label == "new_val" {
			dstID = n
			break
		}
	}
	if srcID == nil || dstID == nil {
		t.Fatalf("failed to find identifiers: srcID=%v, dstID=%v", srcID, dstID)
	}

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcID, dstID)

	es := GenerateEditScript(src, dst, ms)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Update && a.Node == srcID && a.Value == "new_val" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Update action changing 'old_val' to 'new_val'")
		for _, a := range es.Actions() {
			t.Logf("  %s: node=%v val=%q", a.Type, a.Node, a.Value)
		}
	}
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
