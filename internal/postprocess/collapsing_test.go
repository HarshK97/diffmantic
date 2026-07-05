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

func TestScaffoldingInsertSuppression(t *testing.T) {
	// (a) black_18_black/tornado_11_http1connection shape:
	// P=call (Insert, Subtree:false) -> S=argument_list (Insert, Subtree:false, IsScaffolding)
	// S's Insert should be suppressed as redundant, P's Insert survives.
	t.Run("scaffolding-insert-suppressed-under-insert-parent", func(t *testing.T) {
		s := &treesitter.ASTNode{Type: "argument_list", StartByte: 11, EndByte: 13}
		mappedChild := &treesitter.ASTNode{Type: "identifier", StartByte: 0, EndByte: 10}
		parent := &treesitter.ASTNode{
			Type: "call", StartByte: 0, EndByte: 13,
			Children: []*treesitter.ASTNode{mappedChild, s},
		}
		parent.Language = "python"
		mappedChild.Parent = parent
		s.Parent = parent

		mappedSrc := &treesitter.ASTNode{Type: "identifier", StartByte: 100, EndByte: 110}
		mappedSrc.Language = "python"

		ms := engine.NewMapping()
		ms.Add(mappedSrc, mappedChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: s})

		collapsed := Collapse(es, ms, mappedSrc, parent)

		pSurvives := false
		sSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				pSurvives = true
			}
			if a.Node == s && a.Type == actions.Insert {
				sSurvives = true
			}
		}
		if !pSurvives {
			t.Error("expected parent (call) Insert action to survive")
		}
		if sSurvives {
			t.Error("expected argument_list Insert action to be suppressed")
		}
	})

	// (b) information-loss guard case:
	// P=function_definition (Insert, Subtree:false, fails due to unrelated sibling)
	// S=block (Insert, Subtree:true, all S's own children are clean Inserts)
	// S's Insert MUST survive with Subtree:true — NOT suppressed by the new rule.
	t.Run("scaffolding-insert-survives-when-subtree-true", func(t *testing.T) {
		unrelatedChild := &treesitter.ASTNode{Type: "decorator", StartByte: 0, EndByte: 10}
		sChild := &treesitter.ASTNode{Type: "expression_statement", StartByte: 30, EndByte: 50}
		sNode := &treesitter.ASTNode{
			Type: "block", StartByte: 25, EndByte: 65,
			Children: []*treesitter.ASTNode{sChild},
		}
		parent := &treesitter.ASTNode{
			Type: "function_definition", StartByte: 0, EndByte: 70,
			Children: []*treesitter.ASTNode{unrelatedChild, sNode},
		}
		parent.Language = "python"
		unrelatedChild.Parent = parent
		sChild.Parent = sNode
		sNode.Parent = parent

		unrelatedSrc := &treesitter.ASTNode{Type: "decorator", StartByte: 100, EndByte: 110}
		unrelatedSrc.Language = "python"

		ms := engine.NewMapping()
		ms.Add(unrelatedSrc, unrelatedChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: sNode})
		es.Add(actions.Action{Type: actions.Insert, Node: sChild})

		collapsed := Collapse(es, ms, unrelatedSrc, parent)

		findsNode := false
		sNodeSubtree := false
		findsChild := false
		for _, a := range collapsed.Actions() {
			if a.Node == sNode && a.Type == actions.Insert {
				findsNode = true
				if a.Subtree {
					sNodeSubtree = true
				}
			}
			if a.Node == sChild && a.Type == actions.Insert {
				findsChild = true
			}
		}
		if !findsNode {
			t.Error("expected block Insert action to survive")
		}
		if !sNodeSubtree {
			t.Error("expected block Insert to have Subtree:true")
		}
		if findsChild {
			t.Error("expected block's child Insert to be suppressed (by Subtree collapse)")
		}
	})

	// (c) 2-level scaffolding depth case:
	// P=call (Insert) -> S=argument_list (Insert, Subtree:false, scaffolding)
	//   -> S2=argument_list (Insert, Subtree:false, scaffolding)
	// Both S and S2 should be suppressed — independently evaluated at each level.
	t.Run("scaffolding-insert-suppressed-recursive-depth", func(t *testing.T) {
		mappedChild := &treesitter.ASTNode{Type: "identifier", StartByte: 0, EndByte: 9}
		mappedChild2 := &treesitter.ASTNode{Type: "identifier", StartByte: 10, EndByte: 19}
		s2 := &treesitter.ASTNode{Type: "argument_list", StartByte: 20, EndByte: 22}
		s := &treesitter.ASTNode{
			Type: "argument_list", StartByte: 10, EndByte: 22,
			Children: []*treesitter.ASTNode{mappedChild2, s2},
		}
		parent := &treesitter.ASTNode{
			Type: "call", StartByte: 0, EndByte: 22,
			Children: []*treesitter.ASTNode{mappedChild, s},
		}
		parent.Language = "python"
		mappedChild.Parent = parent
		mappedChild2.Parent = s
		s2.Parent = s
		s.Parent = parent

		mappedSrc := &treesitter.ASTNode{Type: "identifier", StartByte: 200, EndByte: 209}
		mappedSrc.Language = "python"
		mappedSrc2 := &treesitter.ASTNode{Type: "identifier", StartByte: 210, EndByte: 219}
		mappedSrc2.Language = "python"

		ms := engine.NewMapping()
		ms.Add(mappedSrc, mappedChild)
		ms.Add(mappedSrc2, mappedChild2)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: s})
		es.Add(actions.Action{Type: actions.Insert, Node: s2})

		collapsed := Collapse(es, ms, mappedSrc, parent)

		pSurvives := false
		sSurvives := false
		s2Survives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				pSurvives = true
			}
			if a.Node == s && a.Type == actions.Insert {
				sSurvives = true
			}
			if a.Node == s2 && a.Type == actions.Insert {
				s2Survives = true
			}
		}
		if !pSurvives {
			t.Error("expected parent (call) Insert action to survive")
		}
		if sSurvives {
			t.Error("expected S (argument_list) Insert action to be suppressed")
		}
		if s2Survives {
			t.Error("expected S2 (argument_list) Insert action to be suppressed")
		}
	})

	// (d) Subtree:true/KillChildren unaffected for scaffolding nodes:
	// A scaffolding node with ALL children cleanly Inserted still achieves
	// Subtree:true and suppresses its children via KillChildren. Our new rule
	// must NOT interfere because Subtree=true guards the suppression check.
	t.Run("scaffolding-subtree-true-killchildren-unaffected", func(t *testing.T) {
		sChild := &treesitter.ASTNode{Type: "expression_statement", StartByte: 20, EndByte: 50}
		sNode := &treesitter.ASTNode{
			Type: "block", StartByte: 10, EndByte: 60,
			Children: []*treesitter.ASTNode{sChild},
		}
		mappedChild := &treesitter.ASTNode{Type: "decorator", StartByte: 0, EndByte: 8}
		parent := &treesitter.ASTNode{
			Type: "function_definition", StartByte: 0, EndByte: 70,
			Children: []*treesitter.ASTNode{mappedChild, sNode},
		}
		parent.Language = "python"
		mappedChild.Parent = parent
		sChild.Parent = sNode
		sNode.Parent = parent

		mappedSrc := &treesitter.ASTNode{Type: "decorator", StartByte: 100, EndByte: 108}
		mappedSrc.Language = "python"

		ms := engine.NewMapping()
		ms.Add(mappedSrc, mappedChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: sNode})
		es.Add(actions.Action{Type: actions.Insert, Node: sChild})

		collapsed := Collapse(es, ms, mappedSrc, parent)

		sSurvives := false
		sSubtree := false
		childSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == sNode && a.Type == actions.Insert {
				sSurvives = true
				if a.Subtree {
					sSubtree = true
				}
			}
			if a.Node == sChild && a.Type == actions.Insert {
				childSurvives = true
			}
		}
		if !sSurvives {
			t.Error("expected block (scaffolding) Insert to survive when all children Inserted")
		}
		if !sSubtree {
			t.Error("expected block Insert to have Subtree:true")
		}
		if childSurvives {
			t.Error("expected block child Insert to be suppressed by KillChildren")
		}
	})
}

func TestContentMoveSuppressedChildResolution(t *testing.T) {
	// 1. Structural pattern with active sibling inserts: P (parenthesized_expression) -> A (boolean_operator) -> [B (Move), C1 (Insert), C2 (Insert)]
	// Must NOT grant the pass to P because A has active Insert children C1, C2.
	t.Run("multi-child-wrapper-denies-pass", func(t *testing.T) {
		p := &treesitter.ASTNode{Type: "parenthesized_expression", StartByte: 0, EndByte: 200}
		p.Language = "python"

		a := &treesitter.ASTNode{Type: "boolean_operator", StartByte: 10, EndByte: 190, Parent: p}
		p.Children = []*treesitter.ASTNode{a}

		bSrc := &treesitter.ASTNode{Type: "comparison_operator", StartByte: 1000, EndByte: 1020}
		bSrc.Language = "python"
		bDst := &treesitter.ASTNode{Type: "comparison_operator", StartByte: 10, EndByte: 50, Parent: a}

		c1 := &treesitter.ASTNode{Type: "logical_operator_literal", StartByte: 51, EndByte: 55, Parent: a, Label: "or"}
		c2 := &treesitter.ASTNode{Type: "comparison_operator", StartByte: 56, EndByte: 190, Parent: a}

		a.Children = []*treesitter.ASTNode{bDst, c1, c2}

		ms := engine.NewMapping()
		ms.Add(bSrc, bDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: p})
		es.Add(actions.Action{Type: actions.Insert, Node: a})
		es.Add(actions.Action{Type: actions.Move, Node: bSrc, Parent: a, Position: 0})
		es.Add(actions.Action{Type: actions.Insert, Node: c1})
		es.Add(actions.Action{Type: actions.Insert, Node: c2})

		collapsed := Collapse(es, ms, bSrc, p)

		// p must NOT claim Subtree = true, and c1, c2 must survive as active insert actions!
		pSubtree := false
		c1Found := false
		c2Found := false
		for _, act := range collapsed.Actions() {
			if act.Node == p && act.Subtree {
				pSubtree = true
			}
			if act.Node == c1 {
				c1Found = true
			}
			if act.Node == c2 {
				c2Found = true
			}
		}
		if pSubtree {
			t.Errorf("expected parenthesized_expression NOT to claim Subtree: true")
		}
		if !c1Found || !c2Found {
			t.Errorf("expected c1 ('or') and c2 ('comparison_operator') to survive, got c1Found=%v, c2Found=%v", c1Found, c2Found)
		}
	})

	// 2. Pure structural wrapper pattern: P (generic_type) -> A (type_parameter) -> [B (Move)]
	// Must STILL GRANT the pass to P because A has NO active Insert children.
	t.Run("single-child-wrapper-grants-pass", func(t *testing.T) {
		p := &treesitter.ASTNode{Type: "generic_type", StartByte: 0, EndByte: 100}
		p.Language = "python"

		c0 := &treesitter.ASTNode{Type: "identifier", StartByte: 0, EndByte: 10, Parent: p, Label: "List"}
		a := &treesitter.ASTNode{Type: "type_parameter", StartByte: 11, EndByte: 100, Parent: p}
		p.Children = []*treesitter.ASTNode{c0, a}

		bSrc := &treesitter.ASTNode{Type: "type", StartByte: 500, EndByte: 520}
		bSrc.Language = "python"
		bDst := &treesitter.ASTNode{Type: "type", StartByte: 12, EndByte: 99, Parent: a}
		a.Children = []*treesitter.ASTNode{bDst}

		ms := engine.NewMapping()
		ms.Add(bSrc, bDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: p})
		es.Add(actions.Action{Type: actions.Insert, Node: c0})
		es.Add(actions.Action{Type: actions.Insert, Node: a})
		es.Add(actions.Action{Type: actions.Move, Node: bSrc, Parent: a, Position: 0})

		collapsed := Collapse(es, ms, bSrc, p)

		pSubtree := false
		for _, act := range collapsed.Actions() {
			if act.Node == p && act.Subtree {
				pSubtree = true
			}
		}
		if !pSubtree {
			t.Errorf("expected generic_type to grant pass and claim Subtree: true")
		}
	})
}


