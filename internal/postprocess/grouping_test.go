package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
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

	grouped := GroupMoves(es, nil)
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
	if acts[2].GroupID != "group-1" {
		t.Errorf("expected c3 to have GroupID group-1, got %q", acts[2].GroupID)
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

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()
	if len(acts) != 2 {
		t.Fatalf("expected 2 grouped actions, got %d", len(acts))
	}
	if acts[0].GroupID != "group-1" || acts[1].GroupID != "group-1" {
		t.Errorf("expected grouped actions to have GroupID group-1, got %q and %q", acts[0].GroupID, acts[1].GroupID)
	}
}

func TestGroupMoves_SwappedOrderExcluded(t *testing.T) {
	oldParent := &treesitter.ASTNode{Type: "block"}
	newParent := &treesitter.ASTNode{Type: "block"}

	// a: src 10..20 -> dst 50..60
	srcA := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(srcA, oldParent, 10, 20)
	dstA := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(dstA, newParent, 50, 60)

	// b: src 30..40 -> dst 20..30 (inverted order relative to a)
	srcB := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(srcB, oldParent, 30, 40)
	dstB := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(dstB, newParent, 20, 30)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: srcA, DestNode: dstA, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: srcB, DestNode: dstB, Parent: newParent})

	grouped := GroupMoves(es, nil)
	for _, act := range grouped.Actions() {
		if act.GroupID != "" {
			t.Errorf("expected swapped action to have empty GroupID, got %q", act.GroupID)
		}
	}
}

func TestGroupMoves_ContainerExcluded(t *testing.T) {
	oldParent := &treesitter.ASTNode{Type: "block"}
	newParent := &treesitter.ASTNode{Type: "block"}

	// outer container enclosing inner1 and inner2
	outer := &treesitter.ASTNode{Type: "statement_list"}
	setParentAndRange(outer, oldParent, 10, 80)

	inner1 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(inner1, oldParent, 15, 30)

	inner2 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(inner2, oldParent, 40, 60)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: outer, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: inner1, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: inner2, Parent: newParent})

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()
	if acts[0].GroupID != "" {
		t.Errorf("expected enclosing container to be excluded from group, got %q", acts[0].GroupID)
	}
	if acts[1].GroupID != "group-1" || acts[2].GroupID != "group-1" {
		t.Errorf("expected inner items to be grouped as group-1, got %q and %q", acts[1].GroupID, acts[2].GroupID)
	}
}

func TestGroupMoves_UnresolvedDstNoFalseSwap(t *testing.T) {
	oldParent := &treesitter.ASTNode{Type: "block"}
	newParent := &treesitter.ASTNode{Type: "block"}

	n1 := &treesitter.ASTNode{Type: "identifier"}
	setParentAndRange(n1, oldParent, 10, 20)

	n2 := &treesitter.ASTNode{Type: "identifier"}
	setParentAndRange(n2, oldParent, 30, 40)

	// Neither has DestNode or ms; offsets default to 0 and must not falsely trigger swap detection
	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: n1, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: n2, Parent: newParent})

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()
	if acts[0].GroupID != "group-1" || acts[1].GroupID != "group-1" {
		t.Errorf("expected co-moving nodes without resolved dst to group as group-1, got %q and %q", acts[0].GroupID, acts[1].GroupID)
	}
}
