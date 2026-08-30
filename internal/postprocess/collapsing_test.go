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
	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   qDst,
		Position: 0,
		Subtree:  true,
	})

	// Run Collapse
	collapsed := Collapse(es, ms, pSrc, qDst)

	if collapsed.Size() != 1 {
		t.Fatalf("expected collapsed edit script to have size 1, got %d", collapsed.Size())
	}

	collapsedActions := collapsed.Actions()
	survivingAction := collapsedActions[0]
	if survivingAction.Node != pSrc {
		t.Errorf("expected surviving action node to be pSrc, got %v", survivingAction.Node)
	}
	if !survivingAction.Subtree {
		t.Errorf("expected surviving action to be a subtree move, but Subtree is false")
	}
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
			Type: "call_expression", StartByte: 0, EndByte: 13,
			Children: []*treesitter.ASTNode{mappedChild, s},
		}
		parent.Language = "go"
		mappedChild.Parent = parent
		s.Parent = parent

		mappedSrc := &treesitter.ASTNode{Type: "identifier", StartByte: 100, EndByte: 110}
		mappedSrc.Language = "go"

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
	// S's Insert MUST survive with Subtree:true (NOT suppressed by the new rule).
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
		parent.Language = "go"
		unrelatedChild.Parent = parent
		sChild.Parent = sNode
		sNode.Parent = parent

		unrelatedSrc := &treesitter.ASTNode{Type: "decorator", StartByte: 100, EndByte: 110}
		unrelatedSrc.Language = "go"

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
	// Both S and S2 should be suppressed: independently evaluated at each level.
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
		parent.Language = "go"
		mappedChild.Parent = parent
		mappedChild2.Parent = s
		s2.Parent = s
		s.Parent = parent

		mappedSrc := &treesitter.ASTNode{Type: "identifier", StartByte: 200, EndByte: 209}
		mappedSrc.Language = "go"
		mappedSrc2 := &treesitter.ASTNode{Type: "identifier", StartByte: 210, EndByte: 219}
		mappedSrc2.Language = "go"

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

// TestInlineParentSuppression covers suppressInlineParentRedundancy: when a
// parent Insert/Delete and a child Insert/Delete both live on the same source
// line, the parent action is redundant and must be suppressed. The child
// action (more specific) survives.
func TestInlineParentSuppression(t *testing.T) {
	// (a) go_4_error_handling L26 delete shape:
	// binary_expression (Delete, inline) with children identifier (Delete,
	// inline) and comparison_operator_literal (Delete, inline). A third child
	// is a Move, so allChildrenDeleted fails and the parent Delete survives
	// the existing collapse pass. The inline-redundancy pass must kill the
	// parent binary_expression Delete; the two leaf Deletes survive.
	t.Run("binary_expression-delete-kills-parent", func(t *testing.T) {
		ident := &treesitter.ASTNode{Type: "identifier", Label: "err", StartByte: 608, EndByte: 611, StartRow: 25, EndRow: 25}
		op := &treesitter.ASTNode{Type: "comparison_operator_literal", Label: "==", StartByte: 612, EndByte: 614, StartRow: 25, EndRow: 25}
		// third child is the source of a Move (not deleted), so allChildrenDeleted fails
		selSrc := &treesitter.ASTNode{Type: "selector_expression", StartByte: 615, EndByte: 628, StartRow: 25, EndRow: 25}
		selSrc.Language = "go"
		selDst := &treesitter.ASTNode{Type: "selector_expression", StartByte: 634, EndByte: 647, StartRow: 25, EndRow: 25}
		binExpr := &treesitter.ASTNode{
			Type:      "binary_expression",
			StartByte: 608, EndByte: 628,
			StartRow: 25, EndRow: 25,
			Children: []*treesitter.ASTNode{ident, op, selSrc},
		}
		binExpr.Language = "go"
		ident.Parent = binExpr
		op.Parent = binExpr
		selSrc.Parent = binExpr

		ms := engine.NewMapping()
		ms.Add(selSrc, selDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: ident})
		es.Add(actions.Action{Type: actions.Delete, Node: op})
		es.Add(actions.Action{Type: actions.Delete, Node: binExpr})
		es.Add(actions.Action{Type: actions.Move, Node: selSrc, Parent: selDst, Position: 0})

		collapsed := Collapse(es, ms, binExpr, selDst)

		binSurvives := false
		identSurvives := false
		opSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == binExpr && a.Type == actions.Delete {
				binSurvives = true
			}
			if a.Node == ident && a.Type == actions.Delete {
				identSurvives = true
			}
			if a.Node == op && a.Type == actions.Delete {
				opSurvives = true
			}
			if a.Node == selSrc && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if binSurvives {
			t.Error("expected inline parent binary_expression Delete to be suppressed")
		}
		if !identSurvives || !opSurvives {
			t.Errorf("expected child identifier and operator Deletes to survive; ident=%v op=%v", identSurvives, opSurvives)
		}
		if !moveSurvives {
			t.Error("expected unrelated Move to survive")
		}
	})

	// (b) go_4_error_handling L26 insert shape:
	// call_expression (Insert, inline, no subtree) with direct children
	// selector_expression (Insert, inline, subtree:true, the "errors.Is"
	// function part) and argument_list (scaffolding). Inside argument_list
	// there is an inserted identifier "err" AND a Move destination
	// selector_expression (for "sql.ErrNoRows"). The Move grandchild makes
	// allChildrenInserted fail for argument_list, and because it is a
	// grandchild (not a direct child of call_expression) the existing
	// hasMoveOrUpdateChild check does not fire on call_expression either,
	// so call_expression survives as a non-subtree Insert. The inline pass
	// must then kill call_expression because its direct child sel is an
	// inline Insert on the same line.
	t.Run("call_expression-insert-kills-parent", func(t *testing.T) {
		sel := &treesitter.ASTNode{Type: "selector_expression", StartByte: 619, EndByte: 628, StartRow: 25, EndRow: 25}
		argList := &treesitter.ASTNode{Type: "argument_list", StartByte: 628, EndByte: 648, StartRow: 25, EndRow: 25}
		call := &treesitter.ASTNode{
			Type:      "call_expression",
			StartByte: 619, EndByte: 648,
			StartRow: 25, EndRow: 25,
			Children: []*treesitter.ASTNode{sel, argList},
		}
		call.Language = "go"
		sel.Parent = call
		argList.Parent = call

		// Inside argument_list: an inserted identifier AND a Move destination.
		// The Move destination is what prevents the existing collapse pass from
		// collapsing call_expression to subtree:true.
		argIdent := &treesitter.ASTNode{Type: "identifier", Label: "err", StartByte: 629, EndByte: 632, StartRow: 25, EndRow: 25, Parent: argList}
		movedSelDst := &treesitter.ASTNode{Type: "selector_expression", StartByte: 634, EndByte: 647, StartRow: 25, EndRow: 25, Parent: argList}
		argList.Children = []*treesitter.ASTNode{argIdent, movedSelDst}

		// Source-side node for the Move (sql.ErrNoRows selector_expression).
		movedSelSrc := &treesitter.ASTNode{Type: "selector_expression", StartByte: 615, EndByte: 628, StartRow: 25, EndRow: 25}
		movedSelSrc.Language = "go"

		ms := engine.NewMapping()
		ms.Add(movedSelSrc, movedSelDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: call})
		es.Add(actions.Action{Type: actions.Insert, Node: sel, Subtree: true})
		es.Add(actions.Action{Type: actions.Insert, Node: argList})
		es.Add(actions.Action{Type: actions.Insert, Node: argIdent})
		es.Add(actions.Action{Type: actions.Move, Node: movedSelSrc, Parent: argList, Position: 1})

		collapsed := Collapse(es, ms, movedSelSrc, call)

		callSurvives := false
		selSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == call && a.Type == actions.Insert {
				callSurvives = true
			}
			if a.Node == sel && a.Type == actions.Insert {
				selSurvives = true
			}
		}
		if callSurvives {
			t.Error("expected inline parent call_expression Insert to be suppressed")
		}
		if !selSurvives {
			t.Error("expected child selector_expression Insert (subtree:true) to survive")
		}
	})

	// (c) Guard: parent with Subtree:true must NEVER be killed, even if it is
	// inline and a child action exists on the same line.
	t.Run("subtree-true-parent-not-killed", func(t *testing.T) {
		child := &treesitter.ASTNode{Type: "identifier", Label: "x", StartByte: 5, EndByte: 6, StartRow: 10, EndRow: 10}
		parent := &treesitter.ASTNode{
			Type:      "call",
			StartByte: 0, EndByte: 10,
			StartRow: 10, EndRow: 10,
			Children: []*treesitter.ASTNode{child},
		}
		parent.Language = "python"
		child.Parent = parent

		ms := engine.NewMapping()
		es := actions.NewEditScript()
		// Manually mark parent as Subtree:true to test the guard directly.
		es.Add(actions.Action{Type: actions.Insert, Node: parent, Subtree: true})
		es.Add(actions.Action{Type: actions.Insert, Node: child})

		collapsed := Collapse(es, ms, nil, parent)

		parentSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
		}
		if !parentSurvives {
			t.Error("expected Subtree:true parent Insert to survive (never killed by inline pass)")
		}
	})

	// (d) Guard: multi-line parent with an inline child on one of its lines
	// must NOT be killed.
	t.Run("multiline-parent-not-killed", func(t *testing.T) {
		child := &treesitter.ASTNode{Type: "identifier", Label: "x", StartByte: 5, EndByte: 6, StartRow: 10, EndRow: 10}
		parent := &treesitter.ASTNode{
			Type:      "function_definition",
			StartByte: 0, EndByte: 200,
			StartRow: 9, EndRow: 15, // spans lines 9-15
			Children: []*treesitter.ASTNode{child},
		}
		parent.Language = "python"
		child.Parent = parent

		ms := engine.NewMapping()
		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: child})

		collapsed := Collapse(es, ms, nil, parent)

		parentSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
		}
		if !parentSurvives {
			t.Error("expected multi-line parent Insert to survive (inline pass must not kill it)")
		}
	})

	// (e) Guard: inline parent on a DIFFERENT line than the child must NOT be
	// killed.
	t.Run("different-line-parent-not-killed", func(t *testing.T) {
		child := &treesitter.ASTNode{Type: "identifier", Label: "x", StartByte: 50, EndByte: 51, StartRow: 11, EndRow: 11}
		parent := &treesitter.ASTNode{
			Type:      "call",
			StartByte: 0, EndByte: 10,
			StartRow: 10, EndRow: 10, // inline, but on a different line
			Children: []*treesitter.ASTNode{child},
		}
		parent.Language = "python"
		child.Parent = parent

		ms := engine.NewMapping()
		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Insert, Node: child})

		collapsed := Collapse(es, ms, nil, parent)

		parentSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
		}
		if !parentSurvives {
			t.Error("expected parent Insert on a different line to survive")
		}
	})

	// (f) Move destination on the same line suppresses single-line parent wrapper Insert
	t.Run("moved-child-suppresses-inline-parent-insert", func(t *testing.T) {
		srcChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 634, EndByte: 654,
			StartRow: 34, EndRow: 34,
		}
		srcChild.Language = "javascript"

		dstChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 630, EndByte: 650,
			StartRow: 34, EndRow: 34,
		}
		parent := &treesitter.ASTNode{
			Type:      "expression_statement",
			StartByte: 630, EndByte: 650,
			StartRow: 34, EndRow: 34,
			Children: []*treesitter.ASTNode{dstChild},
		}
		parent.Language = "javascript"
		dstChild.Parent = parent

		ms := engine.NewMapping()
		ms.Add(srcChild, dstChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Move, Node: srcChild, Parent: parent, Position: 0})

		collapsed := Collapse(es, ms, srcChild, parent)

		parentSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
			if a.Node == srcChild && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if parentSurvives {
			t.Error("expected parent expression_statement Insert to be suppressed by inline Move child")
		}
		if !moveSurvives {
			t.Error("expected child assignment_expression Move to survive")
		}
	})

	// (g) Wider parent container (e.g. hash braces "{...}") is NOT suppressed
	// so the newly added braces/syntax retain their Insert highlight.
	t.Run("wider-container-insert-preserved-around-moved-child", func(t *testing.T) {
		srcChild := &treesitter.ASTNode{
			Type:      "pair",
			StartByte: 10, EndByte: 20,
			StartRow: 28, EndRow: 28,
		}
		srcChild.Language = "ruby"

		dstChild := &treesitter.ASTNode{
			Type:      "pair",
			StartByte: 11, EndByte: 21,
			StartRow: 28, EndRow: 28,
		}
		// Parent hash spans bytes 10..22 (wider due to { and })
		parent := &treesitter.ASTNode{
			Type:      "hash",
			StartByte: 10, EndByte: 22,
			StartRow: 28, EndRow: 28,
			Children: []*treesitter.ASTNode{dstChild},
		}
		parent.Language = "ruby"
		dstChild.Parent = parent

		ms := engine.NewMapping()
		ms.Add(srcChild, dstChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Move, Node: srcChild, Parent: parent, Position: 0})

		collapsed := Collapse(es, ms, srcChild, parent)

		parentSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
			if a.Node == srcChild && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if !parentSurvives {
			t.Error("expected wider parent hash Insert to survive (preserve container braces)")
		}
		if !moveSurvives {
			t.Error("expected child pair Move to survive")
		}
	})

	// (h) Trailing statement terminator (semicolon) in single-child expression_statement
	// must STILL suppress the parent wrapper Insert even though parent.EndByte > dstChild.EndByte.
	t.Run("moved-child-suppresses-inline-parent-insert-with-semicolon", func(t *testing.T) {
		srcChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 634, EndByte: 654,
			StartRow: 34, EndRow: 34,
		}
		srcChild.Language = "javascript"

		dstChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 630, EndByte: 650,
			StartRow: 34, EndRow: 34,
		}
		// Parent expression_statement includes trailing semicolon (630..651)
		parent := &treesitter.ASTNode{
			Type:      "expression_statement",
			StartByte: 630, EndByte: 651,
			StartRow: 34, EndRow: 34,
			Children: []*treesitter.ASTNode{dstChild},
		}
		parent.Language = "javascript"
		dstChild.Parent = parent

		ms := engine.NewMapping()
		ms.Add(srcChild, dstChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		es.Add(actions.Action{Type: actions.Move, Node: srcChild, Parent: parent, Position: 0})

		collapsed := Collapse(es, ms, srcChild, parent)

		parentSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == parent && a.Type == actions.Insert {
				parentSurvives = true
			}
			if a.Node == srcChild && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if parentSurvives {
			t.Error("expected parent expression_statement Insert with trailing semicolon to be suppressed")
		}
		if !moveSurvives {
			t.Error("expected child assignment_expression Move to survive")
		}
	})

	// (i) Bidirectional symmetry: source-side single-line statement wrapper Delete is suppressed
	// when child node is moved out.
	t.Run("moved-child-suppresses-source-side-parent-delete", func(t *testing.T) {
		srcChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 100, EndByte: 120,
			StartRow: 10, EndRow: 10,
		}
		srcChild.Language = "javascript"

		srcParent := &treesitter.ASTNode{
			Type:      "expression_statement",
			StartByte: 100, EndByte: 121,
			StartRow: 10, EndRow: 10,
			Children: []*treesitter.ASTNode{srcChild},
		}
		srcParent.Language = "javascript"
		srcChild.Parent = srcParent

		dstChild := &treesitter.ASTNode{
			Type:      "assignment_expression",
			StartByte: 200, EndByte: 220,
			StartRow: 20, EndRow: 20,
		}
		dstChild.Language = "javascript"

		ms := engine.NewMapping()
		ms.Add(srcChild, dstChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: srcParent})
		es.Add(actions.Action{Type: actions.Move, Node: srcChild, Parent: dstChild, Position: 0})

		collapsed := Collapse(es, ms, srcParent, dstChild)

		srcParentSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == srcParent && a.Type == actions.Delete {
				srcParentSurvives = true
			}
			if a.Node == srcChild && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if srcParentSurvives {
			t.Error("expected source-side parent expression_statement Delete to be suppressed")
		}
		if !moveSurvives {
			t.Error("expected child Move action to survive")
		}
	})

	// (j) Multi-child non-scaffolding container (e.g. binary_expression) is NOT suppressed
	// when one child is moved into it.
	t.Run("multi-child-container-not-suppressed-on-move", func(t *testing.T) {
		srcLeft := &treesitter.ASTNode{Type: "call_expression", StartByte: 10, EndByte: 20, StartRow: 5, EndRow: 5}
		srcLeft.Language = "javascript"

		dstLeft := &treesitter.ASTNode{Type: "call_expression", StartByte: 50, EndByte: 60, StartRow: 15, EndRow: 15}
		dstOp := &treesitter.ASTNode{Type: "+", StartByte: 61, EndByte: 62, StartRow: 15, EndRow: 15}
		dstRight := &treesitter.ASTNode{Type: "call_expression", StartByte: 63, EndByte: 73, StartRow: 15, EndRow: 15}
		dstBin := &treesitter.ASTNode{
			Type:      "binary_expression",
			StartByte: 50, EndByte: 73,
			StartRow: 15, EndRow: 15,
			Children: []*treesitter.ASTNode{dstLeft, dstOp, dstRight},
		}
		dstBin.Language = "javascript"
		dstLeft.Parent = dstBin
		dstOp.Parent = dstBin
		dstRight.Parent = dstBin

		ms := engine.NewMapping()
		ms.Add(srcLeft, dstLeft)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: dstBin})
		es.Add(actions.Action{Type: actions.Move, Node: srcLeft, Parent: dstBin, Position: 0})

		collapsed := Collapse(es, ms, srcLeft, dstBin)

		binSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == dstBin && a.Type == actions.Insert {
				binSurvives = true
			}
			if a.Node == srcLeft && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if !binSurvives {
			t.Error("expected multi-child binary_expression Insert to survive (not suppressed)")
		}
		if !moveSurvives {
			t.Error("expected child Move action to survive")
		}
	})
}

func TestParentMoveWithDeletedDescendant(t *testing.T) {
	// Construct source tree:
	// pSrc (block)
	//   c1Src (identifier)
	//   c2Src (identifier)
	c1Src := &treesitter.ASTNode{Type: "identifier", Label: "x", StartByte: 10, EndByte: 15}
	c2Src := &treesitter.ASTNode{Type: "identifier", Label: "y", StartByte: 16, EndByte: 20}
	pSrc := &treesitter.ASTNode{
		Type:      "block",
		StartByte: 0, EndByte: 100,
		Children: []*treesitter.ASTNode{c1Src, c2Src},
	}
	pSrc.Language = "python"
	c1Src.Parent = pSrc
	c2Src.Parent = pSrc

	// Construct destination tree:
	// qDst (parent container)
	//   pDst (block)
	//     c1Dst (identifier)
	c1Dst := &treesitter.ASTNode{Type: "identifier", Label: "x", StartByte: 10, EndByte: 15}
	pDst := &treesitter.ASTNode{
		Type:      "block",
		StartByte: 0, EndByte: 50,
		Children: []*treesitter.ASTNode{c1Dst},
	}
	qDst := &treesitter.ASTNode{
		Type:      "block",
		StartByte: 0, EndByte: 150,
		Children: []*treesitter.ASTNode{pDst},
	}
	pDst.Parent = qDst
	c1Dst.Parent = pDst
	qDst.Language = "python"

	// Mappings:
	// pSrc -> pDst
	// c1Src -> c1Dst
	// c2Src is unmapped (deleted)
	ms := engine.NewMapping()
	ms.Add(pSrc, pDst)
	ms.Add(c1Src, c1Dst)

	// Construct EditScript:
	// - pSrc moves under qDst (subtree move)
	// - c2Src is deleted
	es := actions.NewEditScript()
	pMove := actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   qDst,
		Position: 0,
		Subtree:  true,
	}
	c2Delete := actions.Action{
		Type: actions.Delete,
		Node: c2Src,
	}
	es.Add(pMove)
	es.Add(c2Delete)

	// Run Collapse
	collapsed := Collapse(es, ms, pSrc, qDst)

	pMoveSurvives := false
	c2DeleteSurvives := false
	for _, act := range collapsed.Actions() {
		if act.Node == pSrc && act.Type == actions.Move {
			pMoveSurvives = true
		}
		if act.Node == c2Src && act.Type == actions.Delete {
			c2DeleteSurvives = true
		}
	}

	if !pMoveSurvives {
		t.Error("expected parent block Move to survive as subtree move")
	}
	if !c2DeleteSurvives {
		t.Error("expected child c2Src Delete to survive")
	}
}

func TestScaffoldingDeleteSuppression(t *testing.T) {
	// P=block (Delete, scaffolding) -> S=statement_list (Delete, scaffolding) -> C=expression_statement (Delete)
	// S's Delete should be suppressed as redundant when under parent block Delete.
	stmt := &treesitter.ASTNode{Type: "expression_statement", StartByte: 10, EndByte: 30}
	stmtList := &treesitter.ASTNode{
		Type: "statement_list", StartByte: 10, EndByte: 30,
		Children: []*treesitter.ASTNode{stmt},
	}
	block := &treesitter.ASTNode{
		Type: "block", StartByte: 0, EndByte: 35,
		Children: []*treesitter.ASTNode{stmtList},
	}
	block.Language = "go"
	stmt.Parent = stmtList
	stmtList.Parent = block

	ms := engine.NewMapping()
	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: block})
	es.Add(actions.Action{Type: actions.Delete, Node: stmtList})
	es.Add(actions.Action{Type: actions.Delete, Node: stmt})

	collapsed := Collapse(es, ms, block, nil)

	blockSurvives := false
	stmtListSurvives := false
	for _, a := range collapsed.Actions() {
		if a.Node == block && a.Type == actions.Delete {
			blockSurvives = true
		}
		if a.Node == stmtList && a.Type == actions.Delete {
			stmtListSurvives = true
		}
	}

	if !blockSurvives {
		t.Error("expected parent block Delete action to survive")
	}
	if stmtListSurvives {
		t.Error("expected child statement_list Delete action to be suppressed as redundant scaffolding")
	}
}

func TestScaffoldingMoveSuppression(t *testing.T) {
	// P=call (Move) -> S=argument_list (Move, scaffolding)
	s := &treesitter.ASTNode{Type: "argument_list", StartByte: 11, EndByte: 20}
	parent := &treesitter.ASTNode{
		Type: "call_expression", StartByte: 0, EndByte: 20,
		Children: []*treesitter.ASTNode{s},
	}
	parent.Language = "go"
	s.Parent = parent

	dstS := &treesitter.ASTNode{Type: "argument_list", StartByte: 111, EndByte: 120}
	dstParent := &treesitter.ASTNode{
		Type: "call_expression", StartByte: 100, EndByte: 120,
		Children: []*treesitter.ASTNode{dstS},
	}
	dstParent.Language = "go"
	dstS.Parent = dstParent

	ms := engine.NewMapping()
	ms.Add(parent, dstParent)
	ms.Add(s, dstS)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: parent, Parent: dstParent, Position: 0})
	es.Add(actions.Action{Type: actions.Move, Node: s, Parent: dstParent, Position: 0})

	collapsed := Collapse(es, ms, parent, dstParent)

	pSurvives := false
	sSurvives := false
	for _, a := range collapsed.Actions() {
		if a.Node == parent && a.Type == actions.Move {
			pSurvives = true
		}
		if a.Node == s && a.Type == actions.Move {
			sSurvives = true
		}
	}

	if !pSurvives {
		t.Error("expected parent call_expression Move to survive")
	}
	if sSurvives {
		t.Error("expected argument_list Move to be suppressed as redundant child scaffolding")
	}
}

func TestMultiLevelInlineDeleteSuppression(t *testing.T) {
	// Hierarchy on line 133:
	// expression_statement (Delete, L133)
	//   call_expression (not deleted because argument_list moved)
	//     selector_expression (Delete, L133)
	// expression_statement should be suppressed by selector_expression because
	// selector_expression is on the exact same line and is the specific deleted function.
	sel := &treesitter.ASTNode{Type: "selector_expression", StartByte: 10, EndByte: 25, StartRow: 133, EndRow: 133}
	argList := &treesitter.ASTNode{Type: "argument_list", StartByte: 25, EndByte: 40, StartRow: 133, EndRow: 133}
	call := &treesitter.ASTNode{
		Type: "call_expression", StartByte: 10, EndByte: 40, StartRow: 133, EndRow: 133,
		Children: []*treesitter.ASTNode{sel, argList},
	}
	exprStmt := &treesitter.ASTNode{
		Type: "expression_statement", StartByte: 10, EndByte: 40, StartRow: 133, EndRow: 133,
		Children: []*treesitter.ASTNode{call},
	}
	exprStmt.Language = "go"
	call.Parent = exprStmt
	sel.Parent = call
	argList.Parent = call

	ms := engine.NewMapping()
	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: exprStmt})
	es.Add(actions.Action{Type: actions.Delete, Node: sel})

	collapsed := Collapse(es, ms, exprStmt, nil)

	exprSurvives := false
	selSurvives := false
	for _, a := range collapsed.Actions() {
		if a.Node == exprStmt && a.Type == actions.Delete {
			exprSurvives = true
		}
		if a.Node == sel && a.Type == actions.Delete {
			selSurvives = true
		}
	}

	if exprSurvives {
		t.Error("expected multi-level inline ancestor expression_statement Delete to be suppressed")
	}
	if !selSurvives {
		t.Error("expected leaf selector_expression Delete to survive")
	}
}
