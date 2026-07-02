package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestCollapseDivergence(t *testing.T) {
	// Construct source tree:
	// P (block)
	//   C (identifier)
	cSrc := &treesitter.ASTNode{Type: "block", StartByte: 10, EndByte: 20}
	pSrc := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 100, Children: []*treesitter.ASTNode{cSrc}}
	pSrc.Language = "python"
	cSrc.Parent = pSrc

	// Construct destination tree:
	// Q (block)
	//   R (block)
	//     D (block)
	dDst := &treesitter.ASTNode{Type: "block", StartByte: 60, EndByte: 70}
	rDst := &treesitter.ASTNode{Type: "block", StartByte: 50, EndByte: 150, Children: []*treesitter.ASTNode{dDst}}
	dDst.Parent = rDst
	qDst := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 200, Children: []*treesitter.ASTNode{rDst}}
	qDst.Language = "python"
	rDst.Parent = qDst

	// Mappings:
	// pSrc -> qDst
	// cSrc -> dDst
	// Note that this mapping is depth-inconsistent: dDst.Parent (rDst) != qDst
	ms := engine.NewMapping()
	ms.Add(pSrc, qDst)
	ms.Add(cSrc, dDst)

	// Construct EditScript:
	// P moves to Q
	// C moves with Parent = qDst (manually set, diverging from dDst.Parent = rDst)
	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   qDst,
		Position: 0,
	})
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     cSrc,
		Parent:   qDst, // Manually set to match P's destination to trigger the divergence
		Position: 0,
	})

	// Run Collapse
	collapsed := Collapse(es, ms, pSrc, qDst)

	// Since we set C's Parent to qDst, it matches dstParent (qDst) in the parent-equality check.
	// Therefore, Collapse thinks they moved to the same parent and collapses them!
	// C's Move action is suppressed, and P's Move action is marked as Subtree: true.
	if collapsed.Size() != 1 {
		t.Fatalf("expected collapsed edit script to have size 1, got %d", collapsed.Size())
	}

	collapsedActions := collapsed.Actions()
	pAction := collapsedActions[0]
	if pAction.Node != pSrc {
		t.Errorf("expected action node to be pSrc, got %v", pAction.Node)
	}
	if !pAction.Subtree {
		t.Errorf("expected action to be a subtree move, but Subtree is false")
	}

	// This demonstrates the divergence: the mapping was depth-inconsistent (dDst.Parent != qDst),
	// but the check passed and collapsed them anyway because the edit script had C's Parent set to Q.
}

func TestRefinedParentSuppression(t *testing.T) {
	// (a) boolean operator suppression (real Move child triggers suppression)
	t.Run("boolean-operator-suppression", func(t *testing.T) {
		// Destination Tree:
		// Parent: boolean_operator (Insert)
		//   Child 1: not_operator (Insert)
		//   Child 2: logical_operator_literal (Insert)
		//   Child 3: not_operator (Move)
		
		parent := &treesitter.ASTNode{Type: "boolean_operator", StartByte: 0, EndByte: 100}
		parent.Language = "python"
		
		c1 := &treesitter.ASTNode{Type: "not_operator", StartByte: 0, EndByte: 40, Parent: parent}
		c2 := &treesitter.ASTNode{Type: "logical_operator_literal", StartByte: 41, EndByte: 44, Parent: parent}
		c3 := &treesitter.ASTNode{Type: "not_operator", StartByte: 45, EndByte: 100, Parent: parent}
		parent.Children = []*treesitter.ASTNode{c1, c2, c3}
		
		// Source Tree:
		// We have an old node for c3 to map from
		c3Src := &treesitter.ASTNode{Type: "not_operator", StartByte: 50, EndByte: 105}
		c3Src.Language = "python"
		
		ms := engine.NewMapping()
		ms.Add(c3Src, c3)
		
		es := actions.NewEditScript()
		// Parent, c1, and c2 are inserted
		pAct := actions.Action{Type: actions.Insert, Node: parent}
		c1Act := actions.Action{Type: actions.Insert, Node: c1}
		c2Act := actions.Action{Type: actions.Insert, Node: c2}
		// c3 is moved
		c3Act := actions.Action{Type: actions.Move, Node: c3Src, Parent: parent}
		
		es.Add(pAct)
		es.Add(c1Act)
		es.Add(c2Act)
		es.Add(c3Act)
		
		collapsed := Collapse(es, ms, c3Src, parent)
		
		// We expect the parent's Insert action to be suppressed.
		// c1, c2, and c3 actions should survive.
		// So total actions = 3 (c1, c2, c3).
		if collapsed.Size() != 3 {
			t.Errorf("expected 3 actions, got %d", collapsed.Size())
		}
		
		// Ensure the parent action is suppressed (not in the script)
		for _, a := range collapsed.Actions() {
			if a.Node == parent {
				t.Error("expected parent boolean_operator Insert action to be suppressed, but it is not")
			}
		}
	})

	// (b) assignment no suppression (only bare aliased-literal Move child, must NOT trigger suppression)
	t.Run("assignment-no-suppression", func(t *testing.T) {
		// Destination Tree:
		// Parent: assignment (Insert)
		//   Child 1: identifier (Insert)
		//   Child 2: assignment_operator_literal (Move)
		//   Child 3: call (Insert)
		
		parent := &treesitter.ASTNode{Type: "assignment", StartByte: 0, EndByte: 100}
		parent.Language = "python"
		
		c1 := &treesitter.ASTNode{Type: "identifier", StartByte: 0, EndByte: 10, Parent: parent}
		c2 := &treesitter.ASTNode{Type: "assignment_operator_literal", StartByte: 11, EndByte: 12, Parent: parent}
		c3 := &treesitter.ASTNode{Type: "call", StartByte: 13, EndByte: 100, Parent: parent}
		parent.Children = []*treesitter.ASTNode{c1, c2, c3}
		
		// Source Tree:
		c2Src := &treesitter.ASTNode{Type: "assignment_operator_literal", StartByte: 20, EndByte: 21}
		c2Src.Language = "python"
		
		ms := engine.NewMapping()
		ms.Add(c2Src, c2)
		
		es := actions.NewEditScript()
		pAct := actions.Action{Type: actions.Insert, Node: parent}
		c1Act := actions.Action{Type: actions.Insert, Node: c1}
		c2Act := actions.Action{Type: actions.Move, Node: c2Src, Parent: parent}
		c3Act := actions.Action{Type: actions.Insert, Node: c3}
		
		es.Add(pAct)
		es.Add(c1Act)
		es.Add(c2Act)
		es.Add(c3Act)
		
		collapsed := Collapse(es, ms, c2Src, parent)
		
		// Normalizing the c2 Move action allows the parent assignment to collapse.
		// Expected surviving actions: parent Insert (Subtree: true) and source child c2Src Delete.
		if collapsed.Size() != 2 {
			t.Errorf("expected 2 actions, got %d", collapsed.Size())
		}
		
		foundParentSubtree := false
		foundSourceDelete := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert && a.Subtree {
				foundParentSubtree = true
			}
			if a.Node == c2Src && a.Type == actions.Delete {
				foundSourceDelete = true
			}
		}
		if !foundParentSubtree {
			t.Error("expected parent assignment Insert action to survive with Subtree: true")
		}
		if !foundSourceDelete {
			t.Error("expected source child c2Src Delete action to survive")
		}
	})

	// (c) case with zero Move/Update children at all (existing allChildrenInserted=true path)
	t.Run("allChildrenInserted-true", func(t *testing.T) {
		// Destination Tree:
		// Parent: boolean_operator (Insert)
		//   Child 1: not_operator (Insert)
		//   Child 2: logical_operator_literal (Insert)
		//   Child 3: not_operator (Insert)
		
		parent := &treesitter.ASTNode{Type: "boolean_operator", StartByte: 0, EndByte: 100}
		parent.Language = "python"
		
		c1 := &treesitter.ASTNode{Type: "not_operator", StartByte: 0, EndByte: 40, Parent: parent}
		c2 := &treesitter.ASTNode{Type: "logical_operator_literal", StartByte: 41, EndByte: 44, Parent: parent}
		c3 := &treesitter.ASTNode{Type: "not_operator", StartByte: 45, EndByte: 100, Parent: parent}
		parent.Children = []*treesitter.ASTNode{c1, c2, c3}
		
		ms := engine.NewMapping()
		
		es := actions.NewEditScript()
		pAct := actions.Action{Type: actions.Insert, Node: parent}
		c1Act := actions.Action{Type: actions.Insert, Node: c1}
		c2Act := actions.Action{Type: actions.Insert, Node: c2}
		c3Act := actions.Action{Type: actions.Insert, Node: c3}
		
		es.Add(pAct)
		es.Add(c1Act)
		es.Add(c2Act)
		es.Add(c3Act)
		
		collapsed := Collapse(es, ms, nil, parent)
		
		// All children Insert actions should be suppressed, parent Insert action survives with Subtree = true.
		// Total actions = 1
		if collapsed.Size() != 1 {
			t.Errorf("expected 1 action (parent subtree), got %d", collapsed.Size())
		}
		
		collapsedActions := collapsed.Actions()
		if collapsedActions[0].Node != parent || !collapsedActions[0].Subtree {
			t.Errorf("expected parent action to survive with Subtree: true, got %+v", collapsedActions[0])
		}
	})

	// (d) verify that child suppression for an UNRELATED reason (e.g. duplicate action suppression)
	// STILL disqualifies allChildrenInserted as required.
	t.Run("unrelated-suppression-disqualifies-allChildrenInserted", func(t *testing.T) {
		parent := &treesitter.ASTNode{Type: "call", StartByte: 0, EndByte: 100}
		parent.Language = "python"
		
		c1 := &treesitter.ASTNode{Type: "identifier", StartByte: 0, EndByte: 10, Parent: parent}
		c2 := &treesitter.ASTNode{Type: "argument_list", StartByte: 11, EndByte: 100, Parent: parent}
		gc1 := &treesitter.ASTNode{Type: "string", StartByte: 12, EndByte: 50, Parent: c2}
		c2.Children = []*treesitter.ASTNode{gc1}
		parent.Children = []*treesitter.ASTNode{c1, c2}
		
		ms := engine.NewMapping()
		
		es := actions.NewEditScript()
		pAct := actions.Action{Type: actions.Insert, Node: parent}
		c1Act := actions.Action{Type: actions.Insert, Node: c1}
		c2Act := actions.Action{Type: actions.Insert, Node: c2}
		gc1Act := actions.Action{Type: actions.Insert, Node: gc1}
		
		es.Add(pAct)
		es.Add(c1Act)
		es.Add(c2Act)
		es.Add(gc1Act)
		
		collapsed := Collapse(es, ms, nil, parent)
		
		// Here c2 has all children inserted (gc1), so c2 performs subtree collapse and KillChildren suppresses gc1Act.
		// When parent is evaluated, c1 is inserted, c2 is inserted (and not suppressed by contentMove).
		// So parent call performs subtree collapse as expected.
		foundSubtree := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Subtree {
				foundSubtree = true
			}
		}
		if !foundSubtree {
			t.Error("expected parent call to perform Subtree collapse")
		}
	})
}

