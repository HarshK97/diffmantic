package actions

import (
	"encoding/json"
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

// --- Simplify tests ---

func TestSimplifyCollapseInserts(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}

	dst := &treesitter.ASTNode{Type: "module"}
	dstFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: dst, StartRow: 1}
	dst.Children = []*treesitter.ASTNode{dstFoo}
	dstX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: dstFoo, StartRow: 2}
	dstY := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: dstFoo, StartRow: 3}
	dstFoo.Children = []*treesitter.ASTNode{dstX, dstY}

	ms := engine.NewMapping()
	ms.Add(src, dst)

	es := GenerateEditScript(src, dst, ms)
	simplified := Simplify(es)

	// Since dstFoo and all its descendants are inserted, the root of this subtree (dstFoo)
	// should be marked as Subtree=true, and dstX/dstY inserts should be suppressed.
	hasSubtreeInsert := false
	var insertCount int
	for _, a := range simplified.Actions() {
		if a.Type == Insert {
			insertCount++
			if a.Node == dstFoo && a.Subtree {
				hasSubtreeInsert = true
			}
		}
	}

	if !hasSubtreeInsert {
		t.Error("expected simplified insert action with Subtree=true for dstFoo")
	}
	if insertCount != 1 {
		t.Errorf("expected exactly 1 insert action in simplified script, got %d", insertCount)
	}
}

func TestSimplifyCollapseDeletes(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}
	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo", Parent: src, StartRow: 1}
	src.Children = []*treesitter.ASTNode{srcFoo}
	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: srcFoo, StartRow: 2}
	srcY := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: srcFoo, StartRow: 3}
	srcFoo.Children = []*treesitter.ASTNode{srcX, srcY}

	dst := &treesitter.ASTNode{Type: "module"}

	ms := engine.NewMapping()
	ms.Add(src, dst)

	es := GenerateEditScript(src, dst, ms)
	simplified := Simplify(es)

	hasSubtreeDelete := false
	var deleteCount int
	for _, a := range simplified.Actions() {
		if a.Type == Delete {
			deleteCount++
			if a.Node == srcFoo && a.Subtree {
				hasSubtreeDelete = true
			}
		}
	}

	if !hasSubtreeDelete {
		t.Error("expected simplified delete action with Subtree=true for srcFoo")
	}
	if deleteCount != 1 {
		t.Errorf("expected exactly 1 delete action in simplified script, got %d", deleteCount)
	}
}

// --- JSON output tests ---

func TestToJSON(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}
	srcId := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: src, StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 1}
	src.Children = []*treesitter.ASTNode{srcId}

	dst := &treesitter.ASTNode{Type: "module"}
	dstId := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: dst, StartRow: 1, StartCol: 0, EndRow: 1, EndCol: 1}
	dst.Children = []*treesitter.ASTNode{dstId}

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcId, dstId)

	es := GenerateEditScript(src, dst, ms)

	data, err := ToJSON(es, ms)
	if err != nil {
		t.Fatalf("ToJSON error: %v", err)
	}

	var out map[string]interface{}
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := out["matches"]; !ok {
		t.Error("JSON missing 'matches' key")
	}
	if _, ok := out["actions"]; !ok {
		t.Error("JSON missing 'actions' key")
	}

	actionsArr := out["actions"].([]interface{})
	found := false
	for _, raw := range actionsArr {
		act := raw.(map[string]interface{})
		if act["action"] == "update" {
			found = true
			if _, ok := act["label"]; !ok {
				t.Error("update action missing 'label' field")
			}
		}
	}
	if !found {
		t.Error("expected update action in JSON output")
	}
}

func TestWriteJSON(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}
	dst := &treesitter.ASTNode{Type: "module"}

	ms := engine.NewMapping()
	ms.Add(src, dst)

	es := GenerateEditScript(src, dst, ms)

	var buf strings.Builder
	if err := WriteJSON(&buf, es, ms); err != nil {
		t.Fatalf("WriteJSON error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "matches") || !strings.Contains(output, "actions") {
		t.Error("WriteJSON output missing expected keys")
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
