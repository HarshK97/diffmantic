package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestCollapseDivergence(t *testing.T) {
	cSrc := &treesitter.ASTNode{Type: "block", StartByte: 10, EndByte: 20}
	pSrc := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 100, Children: []*treesitter.ASTNode{cSrc}}
	pSrc.Language = "python"
	cSrc.Parent = pSrc

	dDst := &treesitter.ASTNode{Type: "block", StartByte: 60, EndByte: 70}
	rDst := &treesitter.ASTNode{Type: "block", StartByte: 50, EndByte: 150, Children: []*treesitter.ASTNode{dDst}}
	dDst.Parent = rDst
	qDst := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 200, Children: []*treesitter.ASTNode{rDst}}
	qDst.Language = "python"
	rDst.Parent = qDst

	ms := engine.NewMapping()
	ms.Add(pSrc, qDst)
	ms.Add(cSrc, dDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   qDst,
		Position: 0,
		Subtree:  true,
	})

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

func TestSubtreeMoveWithExternalDescendantInDest(t *testing.T) {
	pSrc := &treesitter.ASTNode{Type: "parenthesized_expression", StartByte: 10, EndByte: 40}
	pSrc.Language = "java"
	cSrc := &treesitter.ASTNode{Type: "method_invocation", StartByte: 11, EndByte: 39, Parent: pSrc}
	cSrc.Language = "java"
	pSrc.Children = []*treesitter.ASTNode{cSrc}

	otherSrc := &treesitter.ASTNode{Type: "binary_expression", StartByte: 100, EndByte: 150}
	otherSrc.Language = "java"

	qDst := &treesitter.ASTNode{Type: "parenthesized_expression", StartByte: 200, EndByte: 300}
	qDst.Language = "java"
	cDst := &treesitter.ASTNode{Type: "method_invocation", StartByte: 201, EndByte: 229, Parent: qDst}
	cDst.Language = "java"
	otherDst := &treesitter.ASTNode{Type: "binary_expression", StartByte: 235, EndByte: 285, Parent: qDst}
	otherDst.Language = "java"
	qDst.Children = []*treesitter.ASTNode{cDst, otherDst}

	ms := engine.NewMapping()
	ms.Add(pSrc, qDst)
	ms.Add(cSrc, cDst)
	ms.Add(otherSrc, otherDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		DestNode: qDst,
		Parent:   qDst,
		Position: 0,
		Subtree:  true,
	})

	collapsed := Collapse(es, ms, pSrc, qDst)
	if collapsed.Size() != 2 {
		t.Fatalf("expected collapsed edit script size 2, got %d", collapsed.Size())
	}
	if collapsed.Actions()[0].Subtree {
		t.Errorf("expected Subtree to be demoted to false because qDst contains otherDst from outside pSrc")
	}
}

func TestSubtreeInsertCollapsing(t *testing.T) {
	parent := &treesitter.ASTNode{Type: "boolean_operator", StartByte: 0, EndByte: 100}
	parent.Language = "python"

	c1 := &treesitter.ASTNode{Type: "not_operator", StartByte: 0, EndByte: 40, Parent: parent}
	c2 := &treesitter.ASTNode{Type: "logical_operator_literal", StartByte: 41, EndByte: 44, Parent: parent}
	c3 := &treesitter.ASTNode{Type: "not_operator", StartByte: 45, EndByte: 100, Parent: parent}
	parent.Children = []*treesitter.ASTNode{c1, c2, c3}

	ms := engine.NewMapping()

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: parent})
	es.Add(actions.Action{Type: actions.Insert, Node: c1})
	es.Add(actions.Action{Type: actions.Insert, Node: c2})
	es.Add(actions.Action{Type: actions.Insert, Node: c3})

	collapsed := Collapse(es, ms, nil, parent)

	if collapsed.Size() != 1 {
		t.Fatalf("expected 1 action (parent subtree), got %d", collapsed.Size())
	}

	collapsedActions := collapsed.Actions()
	if collapsedActions[0].Node != parent || !collapsedActions[0].Subtree {
		t.Errorf("expected parent action to survive with Subtree: true, got %+v", collapsedActions[0])
	}
}

func TestSubtreeDeleteCollapsing(t *testing.T) {
	parent := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 100}
	parent.Language = "python"

	c1 := &treesitter.ASTNode{Type: "expression_statement", StartByte: 0, EndByte: 40, Parent: parent}
	c2 := &treesitter.ASTNode{Type: "return_statement", StartByte: 41, EndByte: 100, Parent: parent}
	parent.Children = []*treesitter.ASTNode{c1, c2}

	ms := engine.NewMapping()

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: parent})
	es.Add(actions.Action{Type: actions.Delete, Node: c1})
	es.Add(actions.Action{Type: actions.Delete, Node: c2})

	collapsed := Collapse(es, ms, parent, nil)

	if collapsed.Size() != 1 {
		t.Fatalf("expected 1 action (parent subtree delete), got %d", collapsed.Size())
	}
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

	collapsedActions := collapsed.Actions()
	if collapsedActions[0].Node != parent || !collapsedActions[0].Subtree {
		t.Errorf("expected parent action to survive with Subtree: true, got %+v", collapsedActions[0])
	}
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
		srcChild.Language = "javascript"

		dstChild := &treesitter.ASTNode{
			Type:      "pair",
			StartByte: 11, EndByte: 21,
			StartRow: 28, EndRow: 28,
		}
		// Parent object spans bytes 10..22 (wider due to { and })
		parent := &treesitter.ASTNode{
			Type:      "object",
			StartByte: 10, EndByte: 22,
			StartRow: 28, EndRow: 28,
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
		if !parentSurvives {
			t.Error("expected wider parent hash Insert to survive (preserve container braces)")
		}
		if !moveSurvives {
			t.Error("expected child pair Move to survive")
		}
	})

	// (h) Trailing semicolons shouldn't keep single-child wrappers alive when their child moves.
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

	// (i) Symmetrical delete: source-side single-line statement wrappers with trailing
	// semicolons get dropped too when their child moves.
	t.Run("moved-child-suppresses-source-side-parent-delete-with-semicolon", func(t *testing.T) {
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
			StartRow: 10, EndRow: 10,
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
			t.Error("expected source-side parent expression_statement Delete with trailing semicolon to be suppressed")
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

		dstLeft := &treesitter.ASTNode{Type: "call_expression", StartByte: 50, EndByte: 60, StartRow: 5, EndRow: 5}
		dstOp := &treesitter.ASTNode{Type: "+", StartByte: 61, EndByte: 62, StartRow: 5, EndRow: 5}
		dstRight := &treesitter.ASTNode{Type: "call_expression", StartByte: 63, EndByte: 73, StartRow: 5, EndRow: 5}
		dstBin := &treesitter.ASTNode{
			Type:      "binary_expression",
			StartByte: 50, EndByte: 73,
			StartRow: 5, EndRow: 5,
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

	ms := engine.NewMapping()
	ms.Add(pSrc, pDst)
	ms.Add(c1Src, c1Dst)

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

func TestCollapseWrapperContainerSuppression(t *testing.T) {
	oldStmtList := mkNode("statement_list", "")
	oldStmtList.Language = "go"
	oldVarDecl := mkNode("var_declaration", "")
	oldVarDecl.Language = "go"
	oldVarKeyword := mkNode("var", "var")
	oldVarKeyword.Language = "go"
	oldVarSpec := mkNode("var_spec", "")
	oldVarSpec.Language = "go"

	oldID := mkNode("identifier", "fc")
	oldID.Language = "go"
	oldVarSpec.Children = []*treesitter.ASTNode{oldID}
	oldID.Parent = oldVarSpec

	oldStmtList.Children = []*treesitter.ASTNode{oldVarDecl}
	oldVarDecl.Parent = oldStmtList
	oldVarDecl.Children = []*treesitter.ASTNode{oldVarKeyword, oldVarSpec}
	oldVarKeyword.Parent = oldVarDecl
	oldVarSpec.Parent = oldVarDecl

	newStmtList := mkNode("statement_list", "")
	newStmtList.Language = "go"
	newShortVar := mkNode("short_var_declaration", "")
	newShortVar.Language = "go"
	newExprList := mkNode("expression_list", "")
	newExprList.Language = "go"
	newID := mkNode("identifier", "fc")
	newID.Language = "go"

	newStmtList.Children = []*treesitter.ASTNode{newShortVar}
	newShortVar.Parent = newStmtList
	newExprList.Children = []*treesitter.ASTNode{newID}
	newID.Parent = newExprList
	newShortVar.Children = []*treesitter.ASTNode{newExprList}
	newExprList.Parent = newShortVar

	ms := engine.NewMapping()
	ms.Add(oldStmtList, newStmtList)
	ms.Add(oldVarSpec, newShortVar)
	ms.Add(oldID, newID)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: oldVarKeyword})
	es.Add(actions.Action{Type: actions.Delete, Node: oldVarDecl})
	es.Add(actions.Action{Type: actions.Insert, Node: newExprList})

	collapsed := Collapse(es, ms, oldStmtList, newStmtList)
	if collapsed.Size() != 1 {
		t.Fatalf("expected 1 action (Delete(var)), got %d", collapsed.Size())
	}
	act := collapsed.Actions()[0]
	if act.Type != actions.Delete || act.Node != oldVarKeyword {
		t.Fatalf("expected Delete(var) to survive, got %+v", act)
	}
}

func TestCollapseBlockDelimitersPreserved(t *testing.T) {
	oldStmt := mkNode("expression_statement", "foo();")
	oldStmt.Language = "c"
	oldStmt.StartRow = 0
	oldStmt.EndRow = 0

	newBlock := mkNode("compound_statement", "{\n  bar();\n  foo();\n}")
	newBlock.Language = "c"
	newBlock.StartRow = 0
	newBlock.EndRow = 2

	newInsertedChild := mkNode("expression_statement", "bar();")
	newInsertedChild.Language = "c"
	newInsertedChild.StartRow = 1
	newInsertedChild.EndRow = 1

	newMovedChild := mkNode("expression_statement", "foo();")
	newMovedChild.Language = "c"
	newMovedChild.StartRow = 2
	newMovedChild.EndRow = 2

	newBlock.Children = []*treesitter.ASTNode{newInsertedChild, newMovedChild}
	newInsertedChild.Parent = newBlock
	newMovedChild.Parent = newBlock

	ms := engine.NewMapping()
	ms.Add(oldStmt, newMovedChild)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: newBlock})
	es.Add(actions.Action{Type: actions.Insert, Node: newInsertedChild})
	es.Add(actions.Action{Type: actions.Move, Node: oldStmt, DestNode: newMovedChild})

	collapsed := Collapse(es, ms, oldStmt, newBlock)
	// compound_statement shouldn't be dropped because '{' and '}' are actual delimiters.
	hasBlockInsert := false
	for _, a := range collapsed.Actions() {
		if a.Type == actions.Insert && a.Node == newBlock {
			hasBlockInsert = true
			break
		}
	}
	if !hasBlockInsert {
		t.Fatalf("expected compound_statement Insert action to survive for block delimiters, got actions: %+v", collapsed.Actions())
	}
}

func TestDiscreteChildPromotionOnSubtreeDemotion(t *testing.T) {
	srcBlock := &treesitter.ASTNode{Type: "block", StartByte: 0, EndByte: 100, Language: "go"}

	callSrc := &treesitter.ASTNode{Type: "call_expression", StartByte: 1, EndByte: 40, Parent: srcBlock, Language: "go"}
	callSrcChild := &treesitter.ASTNode{Type: "identifier", StartByte: 2, EndByte: 10, Parent: callSrc, Language: "go"}
	callSrc.Children = []*treesitter.ASTNode{callSrcChild}

	decRowSrc := &treesitter.ASTNode{Type: "dec_statement", StartByte: 45, EndByte: 50, Parent: srcBlock, Language: "go"}
	identRowSrc := &treesitter.ASTNode{Type: "identifier", Label: "row", StartByte: 45, EndByte: 48, Parent: decRowSrc, Language: "go"}
	decRowSrc.Children = []*treesitter.ASTNode{identRowSrc}

	decColSrc := &treesitter.ASTNode{Type: "dec_statement", StartByte: 55, EndByte: 60, Parent: srcBlock, Language: "go"}
	identColSrc := &treesitter.ASTNode{Type: "identifier", Label: "col", StartByte: 55, EndByte: 58, Parent: decColSrc, Language: "go"}
	decColSrc.Children = []*treesitter.ASTNode{identColSrc}

	srcBlock.Children = []*treesitter.ASTNode{callSrc, decRowSrc, decColSrc}

	otherCallDst := &treesitter.ASTNode{Type: "other_call", StartByte: 600, EndByte: 650, Language: "go"}

	dstBlock := &treesitter.ASTNode{Type: "block", StartByte: 200, EndByte: 350, Language: "go"}

	callDst := &treesitter.ASTNode{Type: "call_expression", StartByte: 201, EndByte: 240, Parent: dstBlock, Language: "go"}

	decRowDst := &treesitter.ASTNode{Type: "dec_statement", StartByte: 245, EndByte: 250, Parent: dstBlock, Language: "go"}
	identRowDst := &treesitter.ASTNode{Type: "identifier", Label: "row", StartByte: 245, EndByte: 248, Parent: decRowDst, Language: "go"}
	decRowDst.Children = []*treesitter.ASTNode{identRowDst}

	decColDst := &treesitter.ASTNode{Type: "dec_statement", StartByte: 255, EndByte: 260, Parent: dstBlock, Language: "go"}
	identColDst := &treesitter.ASTNode{Type: "identifier", Label: "col", StartByte: 255, EndByte: 258, Parent: decColDst, Language: "go"}
	decColDst.Children = []*treesitter.ASTNode{identColDst}

	assignDst := &treesitter.ASTNode{Type: "assignment_statement", StartByte: 265, EndByte: 280, Parent: dstBlock, Language: "go"}

	dstBlock.Children = []*treesitter.ASTNode{callDst, decRowDst, decColDst, assignDst}

	ms := engine.NewMapping()
	ms.Add(srcBlock, dstBlock)
	ms.Add(callSrc, otherCallDst) // Map callSrc outside dstBlock so srcBlock loses Subtree: true.
	ms.Add(decRowSrc, decRowDst)
	ms.Add(identRowSrc, identRowDst)
	ms.Add(decColSrc, decColDst)
	ms.Add(identColSrc, identColDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcBlock,
		DestNode: dstBlock,
		Parent:   dstBlock,
		Position: 0,
		Subtree:  true,
	})

	collapsed := Collapse(es, ms, srcBlock, dstBlock)

	actionsList := collapsed.Actions()
	if len(actionsList) != 3 {
		t.Fatalf("expected collapsed edit script to have size 3, got %d", len(actionsList))
	}

	var pMove, rowMove, colMove *actions.Action
	for i := range actionsList {
		a := &actionsList[i]
		switch a.Node {
		case srcBlock:
			pMove = a
		case decRowSrc:
			rowMove = a
		case decColSrc:
			colMove = a
		}
	}

	if pMove == nil || pMove.Subtree {
		t.Errorf("expected parent block Move with Subtree: false, got %+v", pMove)
	}
	if rowMove == nil || rowMove.Type != actions.Move || rowMove.DestNode != decRowDst || !rowMove.Subtree {
		t.Errorf("expected decRowSrc child Move with Subtree: true and DestNode: decRowDst, got %+v", rowMove)
	}
	if colMove == nil || colMove.Type != actions.Move || colMove.DestNode != decColDst || !colMove.Subtree {
		t.Errorf("expected decColSrc child Move with Subtree: true and DestNode: decColDst, got %+v", colMove)
	}
}

func TestPromoteOrphanedChildrenRecursiveNestedContainer(t *testing.T) {
	// if -> block -> for -> call_expression, where for gains a second child in dst.
	leafSrc := &treesitter.ASTNode{Type: "call_expression", StartByte: 30, EndByte: 50, Language: "go"}
	gcSrc := &treesitter.ASTNode{Type: "for_statement", StartByte: 20, EndByte: 60, Children: []*treesitter.ASTNode{leafSrc}, Language: "go"}
	leafSrc.Parent = gcSrc
	cSrc := &treesitter.ASTNode{Type: "block", StartByte: 10, EndByte: 70, Children: []*treesitter.ASTNode{gcSrc}, Language: "go"}
	gcSrc.Parent = cSrc
	pSrc := &treesitter.ASTNode{Type: "if_statement", StartByte: 0, EndByte: 80, Children: []*treesitter.ASTNode{cSrc}, Language: "go"}
	cSrc.Parent = pSrc

	otherSrc := &treesitter.ASTNode{Type: "expression_statement", StartByte: 200, EndByte: 220, Language: "go"}

	leafDst := &treesitter.ASTNode{Type: "call_expression", StartByte: 130, EndByte: 150, Language: "go"}
	otherDst := &treesitter.ASTNode{Type: "expression_statement", StartByte: 155, EndByte: 175, Language: "go"}
	gcDst := &treesitter.ASTNode{Type: "for_statement", StartByte: 120, EndByte: 180, Children: []*treesitter.ASTNode{leafDst, otherDst}, Language: "go"}
	leafDst.Parent = gcDst
	otherDst.Parent = gcDst
	cDst := &treesitter.ASTNode{Type: "block", StartByte: 110, EndByte: 190, Children: []*treesitter.ASTNode{gcDst}, Language: "go"}
	gcDst.Parent = cDst
	pDst := &treesitter.ASTNode{Type: "if_statement", StartByte: 100, EndByte: 200, Children: []*treesitter.ASTNode{cDst}, Language: "go"}
	cDst.Parent = pDst

	ms := engine.NewMapping()
	ms.Add(pSrc, pDst)
	ms.Add(cSrc, cDst)
	ms.Add(gcSrc, gcDst)
	ms.Add(leafSrc, leafDst)
	ms.Add(otherSrc, otherDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		DestNode: pDst,
		Parent:   pDst,
		Position: 0,
		Subtree:  true,
	})

	collapsed := Collapse(es, ms, pSrc, pDst)
	actionsList := collapsed.Actions()

	var pMove, cMove, gcMove, leafMove *actions.Action
	for i := range actionsList {
		a := &actionsList[i]
		switch a.Node {
		case pSrc:
			pMove = a
		case cSrc:
			cMove = a
		case gcSrc:
			gcMove = a
		case leafSrc:
			leafMove = a
		}
	}

	if pMove == nil || pMove.Subtree {
		t.Errorf("expected pSrc Move with Subtree: false, got %+v", pMove)
	}
	if cMove == nil || cMove.Subtree {
		t.Errorf("expected cSrc Move with Subtree: false, got %+v", cMove)
	}
	if gcMove == nil || gcMove.Subtree {
		t.Errorf("expected gcSrc (nested for_statement) Move with Subtree: false, got %+v", gcMove)
	}
	if leafMove == nil || leafMove.Type != actions.Move || leafMove.DestNode != leafDst {
		t.Errorf("expected leafSrc Move to leafDst, got %+v", leafMove)
	}
}

func TestLeafMoveMobilityDemotion_Tier3A(t *testing.T) {
	// Source: stmt1 deleted (contains isolated leaf "err")
	leafErr1 := testutil.Leaf("identifier", "err")
	stmt1 := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), leafErr1)
	stmt1.Language = "go"
	block1 := testutil.Node("block", "", stmt1)
	block1.Language = "go"
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	// Destination: stmt2 inserted (contains leaf "err")
	leafErr2 := testutil.Leaf("identifier", "err")
	stmt2 := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), leafErr2)
	stmt2.Language = "go"
	block2 := testutil.Node("block", "", stmt2)
	block2.Language = "go"
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	ms := engine.NewMapping()
	ms.Add(root1, root2)
	ms.Add(block1, block2)
	ms.Add(leafErr1, leafErr2) // leaf mapped cross-statement while statements themselves are unmapped

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: stmt1})
	es.Add(actions.Action{Type: actions.Insert, Node: stmt2})
	es.Add(actions.Action{Type: actions.Move, Node: leafErr1, DestNode: leafErr2})

	collapsed := Collapse(es, ms, root1, root2)

	for _, act := range collapsed.Actions() {
		if act.Type == actions.Move && act.Node == leafErr1 {
			t.Errorf("expected isolated leaf Move on 'err' to be demoted to del+ins, but Move survived")
		}
	}
}

func TestHollowBlockMoveSuppression_Tier3B(t *testing.T) {
	// Source: if_statement with 4 statements in its block.
	// Only 1 statement (auth guard) moves; other 3 are deleted.
	authGuardSrc := testutil.Node("expression_statement", "",
		testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "checkAuth"),
			testutil.Leaf("identifier", "user"),
			testutil.Leaf("identifier", "token"),
		),
	)
	authGuardSrc.Language = "go"
	authGuardSrc.StartRow = 2
	authGuardSrc.EndRow = 2
	authGuardSrc.StartByte = 20
	authGuardSrc.EndByte = 50

	stmtS1 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "stmt1"))
	stmtS1.Language = "go"
	stmtS1.StartRow = 3
	stmtS1.EndRow = 3

	stmtS2 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "stmt2"))
	stmtS2.Language = "go"
	stmtS2.StartRow = 4
	stmtS2.EndRow = 4

	stmtS3 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "stmt3"))
	stmtS3.Language = "go"
	stmtS3.StartRow = 5
	stmtS3.EndRow = 5

	blockSrc := testutil.Node("block", "", authGuardSrc, stmtS1, stmtS2, stmtS3)
	blockSrc.Language = "go"
	blockSrc.StartRow = 1
	blockSrc.EndRow = 6
	blockSrc.StartByte = 15
	blockSrc.EndByte = 90

	ifSrc := testutil.Node("if_statement", "", testutil.Leaf("if", "if"), blockSrc)
	ifSrc.Language = "go"
	ifSrc.StartRow = 1
	ifSrc.EndRow = 6

	// Destination: new if_statement with new condition and 4 statements in its block.
	authGuardDst := testutil.Node("expression_statement", "",
		testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "checkAuth"),
			testutil.Leaf("identifier", "user"),
			testutil.Leaf("identifier", "token"),
		),
	)
	authGuardDst.Language = "go"
	authGuardDst.StartRow = 12
	authGuardDst.EndRow = 12
	authGuardDst.StartByte = 120
	authGuardDst.EndByte = 150

	stmtD1 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newStmt1"))
	stmtD1.Language = "go"
	stmtD1.StartRow = 13
	stmtD1.EndRow = 13

	stmtD2 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newStmt2"))
	stmtD2.Language = "go"
	stmtD2.StartRow = 14
	stmtD2.EndRow = 14

	stmtD3 := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newStmt3"))
	stmtD3.Language = "go"
	stmtD3.StartRow = 15
	stmtD3.EndRow = 15

	blockDst := testutil.Node("block", "", authGuardDst, stmtD1, stmtD2, stmtD3)
	blockDst.Language = "go"
	blockDst.StartRow = 11
	blockDst.EndRow = 16
	blockDst.StartByte = 115
	blockDst.EndByte = 190

	ifDst := testutil.Node("if_statement", "", testutil.Leaf("if", "if"), blockDst)
	ifDst.Language = "go"
	ifDst.StartRow = 11
	ifDst.EndRow = 16

	ms := engine.NewMapping()
	ms.Add(blockSrc, blockDst)
	ms.Add(authGuardSrc, authGuardDst)
	ms.Add(authGuardSrc.Children[0], authGuardDst.Children[0])
	for i := range authGuardSrc.Children[0].Children {
		ms.Add(authGuardSrc.Children[0].Children[i], authGuardDst.Children[0].Children[i])
	}

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: ifSrc})
	es.Add(actions.Action{Type: actions.Insert, Node: ifDst})
	es.Add(actions.Action{Type: actions.Move, Node: blockSrc, DestNode: blockDst})
	es.Add(actions.Action{Type: actions.Move, Node: authGuardSrc, DestNode: authGuardDst})

	collapsed := Collapse(es, ms, ifSrc, ifDst)

	blockMoveSurvives := false
	authGuardMoveSurvives := false
	blockDeleteExists := false
	blockInsertExists := false

	for _, act := range collapsed.Actions() {
		if act.Node == blockSrc && act.Type == actions.Move {
			blockMoveSurvives = true
		}
		if act.Node == authGuardSrc && act.Type == actions.Move {
			authGuardMoveSurvives = true
		}
		if act.Node == blockSrc && act.Type == actions.Delete {
			blockDeleteExists = true
		}
		if act.Node == blockDst && act.Type == actions.Insert {
			blockInsertExists = true
		}
	}

	if blockMoveSurvives {
		t.Errorf("hollow block move should be suppressed on blockSrc")
	}
	if !authGuardMoveSurvives {
		t.Errorf("discrete auth guard move should be preserved inside hollow block")
	}
	if !blockDeleteExists || !blockInsertExists {
		t.Errorf("hollow block should demote to discrete Delete and Insert actions on delimiters")
	}
}

func TestNearAtomicSubtreeCollapsing_Tier3C(t *testing.T) {
	// Source container has 10 statements: 9 deleted, 1 low-mass "return nil" matched.
	var srcChildren []*treesitter.ASTNode
	for range 9 {
		stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "work"))
		stmt.Language = "go"
		srcChildren = append(srcChildren, stmt)
	}
	retNilSrc := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
	retNilSrc.Language = "go"
	srcChildren = append(srcChildren, retNilSrc)

	blockSrc := testutil.Node("block", "", srcChildren...)
	blockSrc.Language = "go"

	// Destination container has 10 statements: 9 new, 1 matched "return nil".
	var dstChildren []*treesitter.ASTNode
	for range 9 {
		stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newWork"))
		stmt.Language = "go"
		dstChildren = append(dstChildren, stmt)
	}
	retNilDst := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
	retNilDst.Language = "go"
	dstChildren = append(dstChildren, retNilDst)

	blockDst := testutil.Node("block", "", dstChildren...)
	blockDst.Language = "go"

	ms := engine.NewMapping()
	ms.Add(retNilSrc, retNilDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
	for _, c := range srcChildren[:9] {
		es.Add(actions.Action{Type: actions.Delete, Node: c, Subtree: true})
	}
	es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
	for _, c := range dstChildren[:9] {
		es.Add(actions.Action{Type: actions.Insert, Node: c, Subtree: true})
	}

	collapsed := Collapse(es, ms, blockSrc, blockDst)

	var blockSrcDelete, blockDstInsert *actions.Action
	for _, act := range collapsed.Actions() {
		a := act
		if a.Node == blockSrc && a.Type == actions.Delete {
			blockSrcDelete = &a
		}
		if a.Node == blockDst && a.Type == actions.Insert {
			blockDstInsert = &a
		}
	}

	if blockSrcDelete == nil || !blockSrcDelete.Subtree {
		t.Errorf("expected blockSrc to collapse into Delete{Subtree: true}, got %+v", blockSrcDelete)
	}
	if blockDstInsert == nil || !blockDstInsert.Subtree {
		t.Errorf("expected blockDst to collapse into Insert{Subtree: true}, got %+v", blockDstInsert)
	}
}

func TestNearAtomicSubtreeCollapsing_Threshold75Percent(t *testing.T) {
	t.Run("four-statements-one-survivor-75-percent-churn-collapses", func(t *testing.T) {
		// 4 statements: 3 deleted, 1 low-mass return nil (retention 1/4 = 25%, churn 75%)
		var srcChildren []*treesitter.ASTNode
		for range 3 {
			stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "oldWork"))
			stmt.Language = "go"
			srcChildren = append(srcChildren, stmt)
		}
		retNilSrc := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
		retNilSrc.Language = "go"
		srcChildren = append(srcChildren, retNilSrc)

		blockSrc := testutil.Node("block", "", srcChildren...)
		blockSrc.Language = "go"

		var dstChildren []*treesitter.ASTNode
		for range 3 {
			stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newWork"))
			stmt.Language = "go"
			dstChildren = append(dstChildren, stmt)
		}
		retNilDst := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
		retNilDst.Language = "go"
		dstChildren = append(dstChildren, retNilDst)

		blockDst := testutil.Node("block", "", dstChildren...)
		blockDst.Language = "go"

		ms := engine.NewMapping()
		ms.Add(retNilSrc, retNilDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
		for _, c := range srcChildren[:3] {
			es.Add(actions.Action{Type: actions.Delete, Node: c, Subtree: true})
		}
		es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
		for _, c := range dstChildren[:3] {
			es.Add(actions.Action{Type: actions.Insert, Node: c, Subtree: true})
		}

		collapsed := Collapse(es, ms, blockSrc, blockDst)

		var blockSrcDelete, blockDstInsert *actions.Action
		for _, act := range collapsed.Actions() {
			a := act
			if a.Node == blockSrc && a.Type == actions.Delete {
				blockSrcDelete = &a
			}
			if a.Node == blockDst && a.Type == actions.Insert {
				blockDstInsert = &a
			}
		}

		if blockSrcDelete == nil || !blockSrcDelete.Subtree {
			t.Errorf("expected 4-statement blockSrc with 75%% churn to collapse into Delete{Subtree: true}")
		}
		if blockDstInsert == nil || !blockDstInsert.Subtree {
			t.Errorf("expected 4-statement blockDst with 75%% churn to collapse into Insert{Subtree: true}")
		}
	})

	t.Run("three-statements-one-survivor-below-threshold-preserved", func(t *testing.T) {
		// 3 statements: 2 deleted, 1 return nil (retention 1/3 = 33.3%, churn 66.7% < 75%)
		var srcChildren []*treesitter.ASTNode
		for range 2 {
			stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "oldWork"))
			stmt.Language = "go"
			srcChildren = append(srcChildren, stmt)
		}
		retNilSrc := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
		retNilSrc.Language = "go"
		srcChildren = append(srcChildren, retNilSrc)

		blockSrc := testutil.Node("block", "", srcChildren...)
		blockSrc.Language = "go"

		var dstChildren []*treesitter.ASTNode
		for range 2 {
			stmt := testutil.Node("expression_statement", "", testutil.Leaf("identifier", "newWork"))
			stmt.Language = "go"
			dstChildren = append(dstChildren, stmt)
		}
		retNilDst := testutil.Node("return_statement", "", testutil.Leaf("return", "return"), testutil.Leaf("nil", "nil"))
		retNilDst.Language = "go"
		dstChildren = append(dstChildren, retNilDst)

		blockDst := testutil.Node("block", "", dstChildren...)
		blockDst.Language = "go"

		ms := engine.NewMapping()
		ms.Add(retNilSrc, retNilDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
		for _, c := range srcChildren[:2] {
			es.Add(actions.Action{Type: actions.Delete, Node: c, Subtree: true})
		}
		es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
		for _, c := range dstChildren[:2] {
			es.Add(actions.Action{Type: actions.Insert, Node: c, Subtree: true})
		}

		collapsed := Collapse(es, ms, blockSrc, blockDst)

		var blockSrcDelete *actions.Action
		for _, act := range collapsed.Actions() {
			a := act
			if a.Node == blockSrc && a.Type == actions.Delete {
				blockSrcDelete = &a
			}
		}

		if blockSrcDelete != nil && blockSrcDelete.Subtree {
			t.Errorf("expected 3-statement blockSrc with 66.7%% churn NOT to collapse into Subtree: true")
		}
	})
}
