package actions

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// buildSimpleTree creates:
//
//	module
//	├── function: foo
//	│   ├── identifier: x
//	│   └── identifier: y
//	└── function: bar
//	    └── identifier: z
func buildSimpleTree(prefix string) *treesitter.ASTNode {
	module := &treesitter.ASTNode{Type: "module", StartRow: 0, EndRow: 10}
	foo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: module, StartRow: 1, EndRow: 4}
	bar := &treesitter.ASTNode{Type: "function", Label: "bar", Parent: module, StartRow: 5, EndRow: 8}
	module.Children = []*treesitter.ASTNode{foo, bar}

	x := &treesitter.ASTNode{Type: "identifier", Label: prefix + "x", Parent: foo, StartRow: 2, StartCol: 4, EndRow: 2, EndCol: 5}
	y := &treesitter.ASTNode{Type: "identifier", Label: prefix + "y", Parent: foo, StartRow: 3, StartCol: 4, EndRow: 3, EndCol: 5}
	foo.Children = []*treesitter.ASTNode{x, y}

	z := &treesitter.ASTNode{Type: "identifier", Label: prefix + "z", Parent: bar, StartRow: 6, StartCol: 4, EndRow: 6, EndCol: 5}
	bar.Children = []*treesitter.ASTNode{z}

	return module
}

func TestChawatheIdenticalTrees(t *testing.T) {
	src := buildSimpleTree("")
	dst := buildSimpleTree("")

	srcNodes := bfs(src)
	dstNodes := bfs(dst)
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
	// Src: module -> [function:foo -> [id:x]]
	src := &treesitter.ASTNode{Type: "module"}
	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: src}
	src.Children = []*treesitter.ASTNode{srcFoo}
	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: srcFoo, StartRow: 1}
	srcFoo.Children = []*treesitter.ASTNode{srcX}

	// Dst: module -> [function:foo -> [id:x, id:y]]
	dst := &treesitter.ASTNode{Type: "module"}
	dstFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: dst}
	dst.Children = []*treesitter.ASTNode{dstFoo}
	dstX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: dstFoo, StartRow: 1}
	dstY := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: dstFoo, StartRow: 2}
	dstFoo.Children = []*treesitter.ASTNode{dstX, dstY}

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcFoo, dstFoo)
	ms.Add(srcX, dstX)

	es := GenerateEditScript(src, dst, ms)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Insert && a.Node == dstY {
			found = true
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
	// Src: module -> [function:foo -> [id:x, id:y]]
	src := &treesitter.ASTNode{Type: "module"}
	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: src}
	src.Children = []*treesitter.ASTNode{srcFoo}
	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: srcFoo, StartRow: 1}
	srcY := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: srcFoo, StartRow: 2}
	srcFoo.Children = []*treesitter.ASTNode{srcX, srcY}

	// Dst: module -> [function:foo -> [id:x]]
	dst := &treesitter.ASTNode{Type: "module"}
	dstFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: dst}
	dst.Children = []*treesitter.ASTNode{dstFoo}
	dstX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: dstFoo, StartRow: 1}
	dstFoo.Children = []*treesitter.ASTNode{dstX}

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcFoo, dstFoo)
	ms.Add(srcX, dstX)

	es := GenerateEditScript(src, dst, ms)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Delete && a.Node == srcY {
			found = true
		}
	}
	if !found {
		t.Error("expected Delete action for srcY")
	}
}

func TestChawatheUpdateLeaf(t *testing.T) {
	// Src: module -> [id:old]
	src := &treesitter.ASTNode{Type: "module"}
	srcId := &treesitter.ASTNode{Type: "identifier", Label: "old", Parent: src, StartRow: 1}
	src.Children = []*treesitter.ASTNode{srcId}

	// Dst: module -> [id:new]
	dst := &treesitter.ASTNode{Type: "module"}
	dstId := &treesitter.ASTNode{Type: "identifier", Label: "new", Parent: dst, StartRow: 1}
	dst.Children = []*treesitter.ASTNode{dstId}

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcId, dstId)

	es := GenerateEditScript(src, dst, ms)

	found := false
	for _, a := range es.Actions() {
		if a.Type == Update && a.Node == srcId && a.Value == "new" {
			found = true
		}
	}
	if !found {
		t.Error("expected Update action changing 'old' to 'new'")
	}
}



// --- PrintActions test ---

func TestPrintActions(t *testing.T) {
	es := NewEditScript()
	node := &treesitter.ASTNode{Type: "identifier", Label: "foo", StartRow: 1}
	parent := &treesitter.ASTNode{Type: "block"}
	es.Add(Action{Type: Insert, Node: node, Parent: parent, Position: 0})
	es.Add(Action{Type: Delete, Node: node})
	es.Add(Action{Type: Update, Node: node, Value: "bar"})

	var buf strings.Builder
	FprintActions(&buf, es)
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
	FprintActions(&buf, NewEditScript())
	if !strings.Contains(buf.String(), "no edit actions") {
		t.Error("empty script should print 'no edit actions'")
	}
}
