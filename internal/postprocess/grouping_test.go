package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func setParentAndRange(node, parent *treesitter.ASTNode, startByte, endByte uint32) {
	node.Parent = parent
	node.StartByte = startByte
	node.EndByte = endByte
	if parent != nil {
		parent.Children = append(parent.Children, node)
	}
}

func TestGroupMoves(t *testing.T) {
	oldParent := &treesitter.ASTNode{Type: "if_statement"}
	newParent := &treesitter.ASTNode{Type: "block"}

	c1 := &treesitter.ASTNode{Type: "comparison_operator"}
	setParentAndRange(c1, oldParent, 10, 20)

	c2 := &treesitter.ASTNode{Type: "block"}
	setParentAndRange(c2, oldParent, 21, 30)

	c3 := &treesitter.ASTNode{Type: "assignment_operator_literal"}
	setParentAndRange(c3, oldParent, 31, 32)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: c1, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: c2, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: c3, Parent: newParent})

	grouped := GroupMoves(es)
	acts := grouped.Actions()

	if len(acts) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(acts))
	}

	if acts[0].GroupID != "group-1" {
		t.Errorf("expected c1 to have GroupID group-1, got %q", acts[0].GroupID)
	}
	if acts[1].GroupID != "group-1" {
		t.Errorf("expected c2 to have GroupID group-1, got %q", acts[1].GroupID)
	}
	if acts[2].GroupID != "" {
		t.Errorf("expected bare literal c3 to have empty GroupID, got %q", acts[2].GroupID)
	}
}

func TestGroupMovesSerialization(t *testing.T) {
	srcRoot := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(srcRoot, nil, 0, 100)

	oldParent := &treesitter.ASTNode{Type: "function_definition"}
	setParentAndRange(oldParent, srcRoot, 10, 90)

	n1 := &treesitter.ASTNode{Type: "identifier"}
	setParentAndRange(n1, oldParent, 15, 20)

	n2 := &treesitter.ASTNode{Type: "parameters"}
	setParentAndRange(n2, oldParent, 21, 30)

	dstRoot := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(dstRoot, nil, 0, 100)

	newParent := &treesitter.ASTNode{Type: "function_definition"}
	setParentAndRange(newParent, dstRoot, 10, 90)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: n1, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: n2, Parent: newParent})

	grouped := GroupMoves(es)
	data, err := serialize.Marshal(grouped, nil, srcRoot, dstRoot)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	reconstituted, err := serialize.Unmarshal(data, srcRoot, dstRoot)
	if err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	acts := reconstituted.Actions()
	if len(acts) != 2 {
		t.Fatalf("expected 2 unmarshaled actions, got %d", len(acts))
	}
	if acts[0].GroupID != "group-1" || acts[1].GroupID != "group-1" {
		t.Errorf("expected reconstituted actions to have GroupID group-1, got %q and %q", acts[0].GroupID, acts[1].GroupID)
	}
}
