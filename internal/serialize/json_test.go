package serialize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// helper to build parent-child relations properly
func setParentAndRange(node, parent *treesitter.ASTNode, startByte, endByte uint32) {
	node.Parent = parent
	node.StartByte = startByte
	node.EndByte = endByte
	if parent != nil {
		parent.Children = append(parent.Children, node)
	}
}

func TestRoundTrip(t *testing.T) {
	// Let's build a source (before) tree:
	// module
	//  ├── function (foo)
	//  │    ├── identifier (x)
	//  │    └── identifier (y)
	//  └── function (bar)
	src := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(src, nil, 0, 100)

	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(srcFoo, src, 10, 50)

	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(srcX, srcFoo, 15, 20)

	srcY := &treesitter.ASTNode{Type: "identifier", Label: "y"}
	setParentAndRange(srcY, srcFoo, 25, 30)

	srcBar := &treesitter.ASTNode{Type: "function", Label: "bar"}
	setParentAndRange(srcBar, src, 60, 90)

	// Let's build a destination (after) tree:
	// module
	//  ├── function (foo)
	//  │    └── identifier (x)  <-- y deleted
	//  └── function (bar)
	//       ├── identifier (z)  <-- z inserted
	//       └── identifier (x)  <-- x moved here from foo
	dst := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(dst, nil, 0, 100)

	dstFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(dstFoo, dst, 10, 40)

	dstX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(dstX, dstFoo, 15, 20)

	dstBar := &treesitter.ASTNode{Type: "function", Label: "bar"}
	setParentAndRange(dstBar, dst, 50, 95)

	dstZ := &treesitter.ASTNode{Type: "identifier", Label: "z"}
	setParentAndRange(dstZ, dstBar, 55, 60)

	dstMovedX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(dstMovedX, dstBar, 65, 70)

	// Create Mappings:
	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcFoo, dstFoo)
	ms.Add(srcBar, dstBar)
	ms.Add(srcX, dstMovedX) // Move mapping: srcX -> dstMovedX

	// Generate some simulated actions
	es := actions.NewEditScript()

	// 1. Insert action (insert dstZ under dstBar at position 0)
	es.Add(actions.Action{
		Type:     actions.Insert,
		Node:     dstZ,
		Parent:   srcBar, // parent in src tree
		Position: 0,
		Subtree:  false,
	})

	// 2. Delete action (delete srcY)
	es.Add(actions.Action{
		Type:    actions.Delete,
		Node:    srcY,
		Subtree: true, // test subtree delete
	})

	// 3. Update action (update srcBar from "bar" to "new_bar")
	es.Add(actions.Action{
		Type:  actions.Update,
		Node:  srcBar,
		Value: "new_bar",
	})

	// 4. Move action (move srcX to dstBar at position 1)
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcX,
		Parent:   srcBar, // new parent in src tree
		Position: 1,
		Subtree:  false,
	})

	// Perform serialization
	jsonData, err := Marshal(es, ms, src, dst, make([]byte, src.EndByte), make([]byte, dst.EndByte))
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	t.Logf("Generated JSON:\n%s", string(jsonData))

	// Verify JSON structure and options:
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if parsed["version"] != SchemaVersion {
		t.Errorf("expected version %q, got %v", SchemaVersion, parsed["version"])
	}

	acts, ok := parsed["actions"].([]interface{})
	if !ok {
		t.Fatalf("actions field missing or not an array")
	}

	if len(acts) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(acts))
	}

	// 1. Check Insert action fields and tree tags
	act1 := acts[0].(map[string]interface{})
	if act1["action"] != "insert" {
		t.Errorf("expected first action to be insert, got %v", act1["action"])
	}
	node1 := act1["node"].(map[string]interface{})
	if node1["tree"] != "after" {
		t.Errorf("insert node should be 'after', got %v", node1["tree"])
	}
	parent1 := act1["parent"].(map[string]interface{})
	if parent1["tree"] != "after" {
		t.Errorf("insert parent should be 'after', got %v", parent1["tree"])
	}
	if act1["subtree"] != nil {
		t.Errorf("insert subtree=false should be omitted, got %v", act1["subtree"])
	}

	// 2. Check Delete action fields, tree tags and subtree
	act2 := acts[1].(map[string]interface{})
	if act2["action"] != "delete" {
		t.Errorf("expected second action to be delete, got %v", act2["action"])
	}
	node2 := act2["node"].(map[string]interface{})
	if node2["tree"] != "before" {
		t.Errorf("delete node should be 'before', got %v", node2["tree"])
	}
	if act2["subtree"] != true {
		t.Errorf("delete subtree should be true, got %v", act2["subtree"])
	}

	// 3. Check Update action fields and subtree absence
	act3 := acts[2].(map[string]interface{})
	if act3["action"] != "update" {
		t.Errorf("expected third action to be update, got %v", act3["action"])
	}
	if act3["subtree"] != nil {
		t.Errorf("update should never contain subtree field, got %v", act3["subtree"])
	}
	if act3["old_value"] != "bar" || act3["new_value"] != "new_bar" {
		t.Errorf("update values mismatch: old=%v new=%v", act3["old_value"], act3["new_value"])
	}

	// 4. Check Move action fields, tree tags
	act4 := acts[3].(map[string]interface{})
	if act4["action"] != "move" {
		t.Errorf("expected fourth action to be move, got %v", act4["action"])
	}
	node4 := act4["node"].(map[string]interface{})
	if node4["tree"] != "before" {
		t.Errorf("move node should be 'before', got %v", node4["tree"])
	}
	parent4 := act4["parent"].(map[string]interface{})
	if parent4["tree"] != "after" {
		t.Errorf("move parent should be 'after', got %v", parent4["tree"])
	}
	oldParent4 := act4["old_parent"].(map[string]interface{})
	if oldParent4["tree"] != "before" {
		t.Errorf("move old_parent should be 'before', got %v", oldParent4["tree"])
	}
	if int(act4["old_position"].(float64)) != 0 {
		t.Errorf("move old_position should be 0, got %v", act4["old_position"])
	}
	if int(act4["position"].(float64)) != 1 {
		t.Errorf("move position should be 1, got %v", act4["position"])
	}

	// Unmarshal back and compare actions
	unmarshaled, err := Unmarshal(jsonData, src, dst)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if unmarshaled.Size() != es.Size() {
		t.Fatalf("size mismatch: expected %d, got %d", es.Size(), unmarshaled.Size())
	}

	for i := 0; i < es.Size(); i++ {
		orig := es.Actions()[i]
		unm := unmarshaled.Actions()[i]

		if orig.Type != unm.Type {
			t.Errorf("[%d] Type mismatch: original=%v, unmarshaled=%v", i, orig.Type, unm.Type)
		}
		if unm.Node == nil {
			t.Errorf("[%d] Resolved Node is nil", i)
		} else {
			if unm.Node.Type != orig.Node.Type || unm.Node.Label != orig.Node.Label {
				t.Errorf("[%d] Node mismatch: original=%v, unmarshaled=%v", i, orig.Node, unm.Node)
			}
		}

		if unm.Subtree != orig.Subtree {
			t.Errorf("[%d] Subtree mismatch: original=%v, unmarshaled=%v", i, orig.Subtree, unm.Subtree)
		}

		switch orig.Type {
		case actions.Insert:
			// In original Action struct, Parent was srcBar. But in JSON it was serialized with tag "after",
			// so the unmarshaled action parent resolves to dstBar. Let's verify it resolves to dstBar.
			if unm.Parent != dstBar {
				t.Errorf("[%d] Insert Parent mismatch: expected %v, got %v", i, dstBar, unm.Parent)
			}
			if unm.Position != orig.Position {
				t.Errorf("[%d] Insert Position mismatch: original=%v, unmarshaled=%v", i, orig.Position, unm.Position)
			}

		case actions.Delete:
			// No extra fields

		case actions.Update:
			if unm.Value != orig.Value {
				t.Errorf("[%d] Update Value mismatch: original=%q, unmarshaled=%q", i, orig.Value, unm.Value)
			}

		case actions.Move:
			// In original Action, Parent was srcBar. In JSON it was serialized as dstBar,
			// so unmarshaled resolves to dstBar.
			if unm.Parent != dstBar {
				t.Errorf("[%d] Move Parent mismatch: expected %v, got %v", i, dstBar, unm.Parent)
			}
			if unm.Position != orig.Position {
				t.Errorf("[%d] Move Position mismatch: original=%v, unmarshaled=%v", i, orig.Position, unm.Position)
			}
		}
	}
}

func TestNilEditScript(t *testing.T) {
	_, err := Marshal(nil, nil, nil, nil, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "edit script is nil") {
		t.Errorf("expected error for nil edit script, got %v", err)
	}
}

func TestMarshalErrorUnmappedMoveParent(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(src, nil, 0, 100)

	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(srcFoo, src, 10, 50)

	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(srcX, srcFoo, 15, 20)

	dst := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(dst, nil, 0, 100)

	// Create an empty mapping
	ms := engine.NewMapping()

	es := actions.NewEditScript()
	// Move action where the new parent srcFoo is NOT mapped in ms.
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcX,
		Parent:   srcFoo,
		Position: 0,
	})

	_, err := Marshal(es, ms, src, dst, make([]byte, src.EndByte), make([]byte, dst.EndByte))
	if err == nil {
		t.Fatal("expected Marshal to fail due to unmapped move parent, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to resolve move parent") {
		t.Errorf("expected error message to mention 'failed to resolve move parent', got: %v", err)
	}
}
