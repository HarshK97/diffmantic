package postprocess

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestCollapseDivergence(t *testing.T) {
	src := []byte("package main\nfunc f() {\n\touterCall(innerCall())\n}\n")
	dst := []byte("package main\nfunc g() {\n\twrapperCall(outerCall(innerCall()))\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func g()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	pSrc, err := testutil.FindNodeByText(srcTree, src, "outerCall(innerCall())", "call_expression")
	if err != nil {
		t.Fatalf("find pSrc: %v", err)
	}
	cSrc, err := testutil.FindNodeByText(srcTree, src, "innerCall()", "call_expression")
	if err != nil {
		t.Fatalf("find cSrc: %v", err)
	}

	wrapDst, err := testutil.FindNodeByText(dstTree, dst, "wrapperCall(outerCall(innerCall()))", "call_expression")
	if err != nil {
		t.Fatalf("find wrapDst: %v", err)
	}
	qDst, err := testutil.FindNodeByText(dstTree, dst, "outerCall(innerCall())", "call_expression")
	if err != nil {
		t.Fatalf("find qDst: %v", err)
	}
	dDst, err := testutil.FindNodeByText(dstTree, dst, "innerCall()", "call_expression")
	if err != nil {
		t.Fatalf("find dDst: %v", err)
	}

	outerIDSrc, err := testutil.FindNodeByText(srcTree, src, "outerCall", "identifier")
	if err != nil {
		t.Fatalf("find outerIDSrc: %v", err)
	}
	outerIDDst, err := testutil.FindNodeByText(dstTree, dst, "outerCall", "identifier")
	if err != nil {
		t.Fatalf("find outerIDDst: %v", err)
	}
	innerIDSrc, err := testutil.FindNodeByText(srcTree, src, "innerCall", "identifier")
	if err != nil {
		t.Fatalf("find innerIDSrc: %v", err)
	}
	innerIDDst, err := testutil.FindNodeByText(dstTree, dst, "innerCall", "identifier")
	if err != nil {
		t.Fatalf("find innerIDDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(pSrc, qDst)
	ms.Add(cSrc, dDst)
	ms.Add(outerIDSrc, outerIDDst)
	ms.Add(innerIDSrc, innerIDDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		DestNode: qDst,
		Parent:   wrapDst,
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
	src := []byte("package main\nfunc f() {\n\texecA()\n\texecB()\n}\n")
	dst := []byte("package main\nfunc f() {\n\tcallTarget(execA(), execB())\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	pSrc, err := testutil.FindNodeByText(srcTree, src, "execA()", "call_expression")
	if err != nil {
		t.Fatalf("find pSrc: %v", err)
	}
	otherSrc, err := testutil.FindNodeByText(srcTree, src, "execB()", "call_expression")
	if err != nil {
		t.Fatalf("find otherSrc: %v", err)
	}

	qDst, err := testutil.FindNodeByText(dstTree, dst, "callTarget(execA(), execB())", "call_expression")
	if err != nil {
		t.Fatalf("find qDst: %v", err)
	}
	cDst, err := testutil.FindNodeByText(dstTree, dst, "execA()", "call_expression")
	if err != nil {
		t.Fatalf("find cDst: %v", err)
	}
	otherDst, err := testutil.FindNodeByText(dstTree, dst, "execB()", "call_expression")
	if err != nil {
		t.Fatalf("find otherDst: %v", err)
	}

	aIDSrc, err := testutil.FindNodeByText(srcTree, src, "execA", "identifier")
	if err != nil {
		t.Fatalf("find aIDSrc: %v", err)
	}
	aIDDst, err := testutil.FindNodeByText(dstTree, dst, "execA", "identifier")
	if err != nil {
		t.Fatalf("find aIDDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(pSrc, qDst)
	ms.Add(cDst, cDst)
	ms.Add(aIDSrc, aIDDst)
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
	src := []byte("x = not a and not b\n")
	tree, err := treesitter.Parse(src, "test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	parent, err := testutil.FindNodeByText(tree, src, "not a and not b", "boolean_operator")
	if err != nil {
		t.Fatalf("find parent: %v", err)
	}

	ms := engine.NewMapping()

	es := actions.NewEditScript()
	for _, n := range parent.PostOrder() {
		es.Add(actions.Action{Type: actions.Insert, Node: n})
	}

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
	src := []byte("def f():\n    x = 1\n    return x\n")
	tree, err := treesitter.Parse(src, "test.py")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	parent, err := testutil.FindNodeByText(tree, src, "x = 1\n    return x", "block")
	if err != nil {
		t.Fatalf("find parent: %v", err)
	}

	ms := engine.NewMapping()

	es := actions.NewEditScript()
	for _, n := range parent.PostOrder() {
		es.Add(actions.Action{Type: actions.Delete, Node: n})
	}

	collapsed := Collapse(es, ms, parent, nil)

	if collapsed.Size() != 1 {
		t.Fatalf("expected 1 action (parent subtree delete), got %d", collapsed.Size())
	}

	t.Run("scaffolding-insert-survives-when-subtree-true", func(t *testing.T) {
		srcGo := []byte("package main\nfunc f(x int) {}\n")
		dstGo := []byte("package main\nfunc g(x int) {\n\ty := 1\n}\n")

		srcAST, err := treesitter.Parse(srcGo, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstGo, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		parent, err := testutil.FindNodeByText(dstAST, dstGo, "func g(x int) {\n\ty := 1\n}", "function_declaration")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}
		sNode, err := testutil.FindNodeByText(dstAST, dstGo, "{\n\ty := 1\n}", "block")
		if err != nil {
			t.Fatalf("find sNode: %v", err)
		}
		unrelatedChild, err := testutil.FindNodeByText(dstAST, dstGo, "x int", "parameter_declaration")
		if err != nil {
			t.Fatalf("find unrelatedChild: %v", err)
		}
		unrelatedSrc, err := testutil.FindNodeByText(srcAST, srcGo, "x int", "parameter_declaration")
		if err != nil {
			t.Fatalf("find unrelatedSrc: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(unrelatedSrc, unrelatedChild)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: parent})
		for _, n := range sNode.PostOrder() {
			es.Add(actions.Action{Type: actions.Insert, Node: n})
		}

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
			if a.Node != sNode && a.Type == actions.Insert && sNode.Contains(a.Node) {
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

func TestInlineParentSuppression(t *testing.T) {
	t.Run("binary_expression-delete-kills-parent", func(t *testing.T) {
		src := []byte("package main\nfunc f() { _ = queryErr == sql.ErrNoRows }\n")
		dst := []byte("package main\nfunc f() { _ = errors.Is(queryErr, sql.ErrNoRows) }\n")

		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find fnSrc: %v", err)
		}
		fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find fnDst: %v", err)
		}

		binExpr, err := testutil.FindNodeByText(srcTree, src, "queryErr == sql.ErrNoRows", "binary_expression")
		if err != nil {
			t.Fatalf("find binExpr: %v", err)
		}
		ident, err := testutil.FindNodeByText(srcTree, src, "queryErr", "identifier")
		if err != nil {
			t.Fatalf("find ident: %v", err)
		}
		op, err := testutil.FindNodeByText(srcTree, src, "==", "comparison_operator_literal")
		if err != nil {
			t.Fatalf("find op: %v", err)
		}
		selSrc, err := testutil.FindNodeByText(srcTree, src, "sql.ErrNoRows", "selector_expression")
		if err != nil {
			t.Fatalf("find selSrc: %v", err)
		}
		selDst, err := testutil.FindNodeByText(dstTree, dst, "sql.ErrNoRows", "selector_expression")
		if err != nil {
			t.Fatalf("find selDst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(fnSrc, fnDst)
		ms.Add(selSrc, selDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: ident})
		es.Add(actions.Action{Type: actions.Delete, Node: op})
		es.Add(actions.Action{Type: actions.Delete, Node: binExpr})
		es.Add(actions.Action{Type: actions.Move, Node: selSrc, DestNode: selDst, Parent: selDst, Position: 0})

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

	t.Run("call_expression-insert-kills-parent", func(t *testing.T) {
		src := []byte("package main\nfunc f() { _ = queryErr == sql.ErrNoRows }\n")
		dst := []byte("package main\nfunc f() { _ = errors.Is(queryErr, sql.ErrNoRows) }\n")

		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find fnSrc: %v", err)
		}
		fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find fnDst: %v", err)
		}

		call, err := testutil.FindNodeByText(dstTree, dst, "errors.Is(queryErr, sql.ErrNoRows)", "call_expression")
		if err != nil {
			t.Fatalf("find call: %v", err)
		}
		sel, err := testutil.FindNodeByText(dstTree, dst, "errors.Is", "selector_expression")
		if err != nil {
			t.Fatalf("find sel: %v", err)
		}
		argList, err := testutil.FindNodeByText(dstTree, dst, "(queryErr, sql.ErrNoRows)", "argument_list")
		if err != nil {
			t.Fatalf("find argList: %v", err)
		}
		argIdent, err := testutil.FindNodeByText(dstTree, dst, "queryErr", "identifier")
		if err != nil {
			t.Fatalf("find argIdent: %v", err)
		}
		movedSelDst, err := testutil.FindNodeByText(dstTree, dst, "sql.ErrNoRows", "selector_expression")
		if err != nil {
			t.Fatalf("find movedSelDst: %v", err)
		}
		movedSelSrc, err := testutil.FindNodeByText(srcTree, src, "sql.ErrNoRows", "selector_expression")
		if err != nil {
			t.Fatalf("find movedSelSrc: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(fnSrc, fnDst)
		ms.Add(movedSelSrc, movedSelDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: call})
		es.Add(actions.Action{Type: actions.Insert, Node: sel, Subtree: true})
		es.Add(actions.Action{Type: actions.Insert, Node: argList})
		es.Add(actions.Action{Type: actions.Insert, Node: argIdent})
		es.Add(actions.Action{Type: actions.Move, Node: movedSelSrc, DestNode: movedSelDst, Parent: argList, Position: 1})

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

	t.Run("subtree-true-parent-not-killed", func(t *testing.T) {
		src := []byte("foo(x)\n")
		tree, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		parent, err := testutil.FindNodeByText(tree, src, "foo(x)", "call")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}
		child, err := testutil.FindNodeByText(tree, src, "x", "identifier")
		if err != nil {
			t.Fatalf("find child: %v", err)
		}

		ms := engine.NewMapping()
		es := actions.NewEditScript()
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

	t.Run("multiline-parent-not-killed", func(t *testing.T) {
		src := []byte("def foo():\n    x = 1\n")
		tree, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		parent, err := testutil.FindNodeByText(tree, src, "def foo():\n    x = 1", "function_definition")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}
		child, err := testutil.FindNodeByText(tree, src, "x", "identifier")
		if err != nil {
			t.Fatalf("find child: %v", err)
		}

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

	t.Run("different-line-parent-not-killed", func(t *testing.T) {
		src := []byte("foo()\nbar()\n")
		tree, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}

		parent, err := testutil.FindNodeByText(tree, src, "foo()", "call")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}
		child, err := testutil.FindNodeByText(tree, src, "bar", "identifier")
		if err != nil {
			t.Fatalf("find child: %v", err)
		}

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

	t.Run("moved-child-suppresses-inline-parent-insert", func(t *testing.T) {
		src := []byte("a = 1;\n")
		dst := []byte("b = 1;\n")

		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcChild, err := testutil.FindNodeByText(srcTree, src, "a = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find srcChild: %v", err)
		}
		dstChild, err := testutil.FindNodeByText(dstTree, dst, "b = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find dstChild: %v", err)
		}
		parent, err := testutil.FindNodeByText(dstTree, dst, "b = 1;", "expression_statement")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}

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

	t.Run("wider-container-insert-preserved-around-moved-child", func(t *testing.T) {
		src := []byte("const x = { a: 1 };\n")
		dst := []byte("const y = { a: 1 };\n")

		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcChild, err := testutil.FindNodeByText(srcTree, src, "a: 1", "pair")
		if err != nil {
			t.Fatalf("find srcChild: %v", err)
		}
		dstChild, err := testutil.FindNodeByText(dstTree, dst, "a: 1", "pair")
		if err != nil {
			t.Fatalf("find dstChild: %v", err)
		}
		parent, err := testutil.FindNodeByText(dstTree, dst, "{ a: 1 }", "object")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}

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

	t.Run("moved-child-suppresses-inline-parent-insert-with-semicolon", func(t *testing.T) {
		src := []byte("a = 1;\n")
		dst := []byte("b = 1;\n")

		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcChild, err := testutil.FindNodeByText(srcTree, src, "a = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find srcChild: %v", err)
		}
		dstChild, err := testutil.FindNodeByText(dstTree, dst, "b = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find dstChild: %v", err)
		}
		parent, err := testutil.FindNodeByText(dstTree, dst, "b = 1;", "expression_statement")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}

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

	t.Run("moved-child-suppresses-source-side-parent-delete-with-semicolon", func(t *testing.T) {
		src := []byte("a = 1;\n")
		dst := []byte("b = 1;\n")

		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcChild, err := testutil.FindNodeByText(srcTree, src, "a = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find srcChild: %v", err)
		}
		srcParent, err := testutil.FindNodeByText(srcTree, src, "a = 1;", "expression_statement")
		if err != nil {
			t.Fatalf("find srcParent: %v", err)
		}
		dstChild, err := testutil.FindNodeByText(dstTree, dst, "b = 1", "assignment_expression")
		if err != nil {
			t.Fatalf("find dstChild: %v", err)
		}

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

	t.Run("multi-child-container-not-suppressed-on-move", func(t *testing.T) {
		src := []byte("foo();\n")
		dst := []byte("foo() + bar();\n")

		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcLeft, err := testutil.FindNodeByText(srcTree, src, "foo()", "call_expression")
		if err != nil {
			t.Fatalf("find srcLeft: %v", err)
		}
		dstLeft, err := testutil.FindNodeByText(dstTree, dst, "foo()", "call_expression")
		if err != nil {
			t.Fatalf("find dstLeft: %v", err)
		}
		dstBin, err := testutil.FindNodeByText(dstTree, dst, "foo() + bar()", "binary_expression")
		if err != nil {
			t.Fatalf("find dstBin: %v", err)
		}

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

	t.Run("all-mapped-multi-child-container-suppressed-on-move", func(t *testing.T) {
		src := []byte("use foo::bar::{Hunk};\n")
		dst := []byte("use foo::bar::Hunk;\n")

		srcTree, err := treesitter.Parse(src, "src.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcScopedUseList, err := testutil.FindNodeByText(srcTree, src, "foo::bar::{Hunk}", "scoped_use_list")
		if err != nil {
			t.Fatalf("find srcScopedUseList: %v", err)
		}
		srcPrefix, err := testutil.FindNodeByText(srcTree, src, "foo::bar", "scoped_identifier")
		if err != nil {
			t.Fatalf("find srcPrefix: %v", err)
		}
		srcHunk, err := testutil.FindNodeByText(srcTree, src, "Hunk", "identifier")
		if err != nil {
			t.Fatalf("find srcHunk: %v", err)
		}

		dstOuter, err := testutil.FindNodeByText(dstTree, dst, "foo::bar::Hunk", "scoped_identifier")
		if err != nil {
			t.Fatalf("find dstOuter: %v", err)
		}
		dstPrefix, err := testutil.FindNodeByText(dstTree, dst, "foo::bar", "scoped_identifier")
		if err != nil {
			t.Fatalf("find dstPrefix: %v", err)
		}
		dstHunk, err := testutil.FindNodeByText(dstTree, dst, "Hunk", "identifier")
		if err != nil {
			t.Fatalf("find dstHunk: %v", err)
		}

		srcScopedUseList.Parent = nil
		dstOuter.Parent = nil

		ms := engine.NewMapping()
		ms.Add(srcPrefix, dstPrefix)
		ms.Add(srcHunk, dstHunk)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: dstOuter})
		es.Add(actions.Action{Type: actions.Move, Node: srcHunk, DestNode: dstHunk, Parent: dstOuter, Position: 1})

		collapsed := Collapse(es, ms, srcScopedUseList, dstOuter)

		outerSurvives := false
		moveSurvives := false
		for _, a := range collapsed.Actions() {
			if a.Node == dstOuter && a.Type == actions.Insert {
				outerSurvives = true
			}
			if a.Node == srcHunk && a.Type == actions.Move {
				moveSurvives = true
			}
		}
		if outerSurvives {
			t.Error("expected outer scoped_identifier Insert to be suppressed when all children are mapped")
		}
		if !moveSurvives {
			t.Error("expected child Hunk Move action to survive")
		}
	})

	t.Run("all-mapped-multi-child-container-suppressed-on-move-end-to-end", func(t *testing.T) {
		oldSrc := "use foo::bar::{Hunk};\n"
		newSrc := "use foo::bar::Hunk;\n"
		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		collapsed := Collapse(es, mr.Mappings, oldAST, newAST)

		for _, a := range collapsed.Actions() {
			if a.Type == actions.Insert && a.Node.Type == "scoped_identifier" {
				t.Errorf("expected outer scoped_identifier Insert to be suppressed, but found: %+v", a)
			}
		}
	})
}

func TestParentMoveWithDeletedDescendant(t *testing.T) {
	src := []byte("def f():\n    x = 1\n    y = 2\n")
	dst := []byte("def f():\n    x = 1\n")

	srcTree, err := treesitter.Parse(src, "src.py")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.py")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "def f():", "function_definition")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "def f():", "function_definition")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	pSrc, err := testutil.FindNodeByText(srcTree, src, "x = 1\n    y = 2", "block")
	if err != nil {
		t.Fatalf("find pSrc: %v", err)
	}
	c1Src, err := testutil.FindNodeByText(srcTree, src, "x = 1", "expression_statement")
	if err != nil {
		t.Fatalf("find c1Src: %v", err)
	}
	c2Src, err := testutil.FindNodeByText(srcTree, src, "y = 2", "expression_statement")
	if err != nil {
		t.Fatalf("find c2Src: %v", err)
	}

	pDst, err := testutil.FindNodeByText(dstTree, dst, "x = 1", "block")
	if err != nil {
		t.Fatalf("find pDst: %v", err)
	}
	c1Dst, err := testutil.FindNodeByText(dstTree, dst, "x = 1", "expression_statement")
	if err != nil {
		t.Fatalf("find c1Dst: %v", err)
	}

	xIDSrc, err := testutil.FindNodeByTextQuery(srcTree, src, testutil.TextQuery{Text: "x", Type: "identifier", Occurrence: 1})
	if err != nil {
		t.Fatalf("find xIDSrc: %v", err)
	}
	xIDDst, err := testutil.FindNodeByTextQuery(dstTree, dst, testutil.TextQuery{Text: "x", Type: "identifier", Occurrence: 1})
	if err != nil {
		t.Fatalf("find xIDDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(pSrc, pDst)
	ms.Add(c1Src, c1Dst)
	ms.Add(xIDSrc, xIDDst)

	es := actions.NewEditScript()
	pMove := actions.Action{
		Type:     actions.Move,
		Node:     pSrc,
		Parent:   fnDst,
		Position: 0,
	}
	c1Move := actions.Action{
		Type:     actions.Move,
		Node:     c1Src,
		Parent:   pDst,
		Position: 0,
	}
	c2Delete := actions.Action{
		Type: actions.Delete,
		Node: c2Src,
	}
	es.Add(pMove)
	es.Add(c1Move)
	es.Add(c2Delete)

	collapsed := Collapse(es, ms, pSrc, fnDst)

	pMoveSurvives := false
	c1MoveSurvives := false
	for _, act := range collapsed.Actions() {
		if act.Node == pSrc && act.Type == actions.Move {
			pMoveSurvives = true
		}
		if act.Node == c1Src && act.Type == actions.Move {
			c1MoveSurvives = true
		}
	}

	if pMoveSurvives {
		t.Error("expected parent block Move to be suppressed since one child (c2Src) was deleted")
	}
	if !c1MoveSurvives {
		t.Error("expected child c1Src Move to survive")
	}
}

func TestCollapseWrapperContainerSuppression(t *testing.T) {
	src := []byte("package main\nfunc f() {\n\tvar fc func()\n}\n")
	dst := []byte("package main\nfunc f() {\n\tfc := func() {}\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	oldBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tvar fc func()\n}", "block")
	if err != nil {
		t.Fatalf("find oldBlock: %v", err)
	}
	oldVarDecl, err := testutil.FindNodeByText(srcTree, src, "var fc func()", "var_declaration")
	if err != nil {
		t.Fatalf("find oldVarDecl: %v", err)
	}
	oldVarKeyword, err := testutil.FindNodeByText(srcTree, src, "var", "var")
	if err != nil {
		t.Fatalf("find oldVarKeyword: %v", err)
	}
	oldVarSpec, err := testutil.FindNodeByText(srcTree, src, "fc func()", "var_spec")
	if err != nil {
		t.Fatalf("find oldVarSpec: %v", err)
	}
	oldID, err := testutil.FindNodeByText(srcTree, src, "fc", "identifier")
	if err != nil {
		t.Fatalf("find oldID: %v", err)
	}

	newBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tfc := func() {}\n}", "block")
	if err != nil {
		t.Fatalf("find newBlock: %v", err)
	}
	newShortVar, err := testutil.FindNodeByText(dstTree, dst, "fc := func() {}", "short_var_declaration")
	if err != nil {
		t.Fatalf("find newShortVar: %v", err)
	}
	newExprList, err := testutil.FindNodeByText(dstTree, dst, "fc", "expression_list")
	if err != nil {
		t.Fatalf("find newExprList: %v", err)
	}
	newID, err := testutil.FindNodeByText(dstTree, dst, "fc", "identifier")
	if err != nil {
		t.Fatalf("find newID: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(oldBlock, newBlock)
	ms.Add(oldVarSpec, newShortVar)
	ms.Add(oldID, newID)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: oldVarKeyword})
	es.Add(actions.Action{Type: actions.Delete, Node: oldVarDecl})
	es.Add(actions.Action{Type: actions.Insert, Node: newExprList})

	collapsed := Collapse(es, ms, oldBlock, newBlock)
	if collapsed.Size() != 1 {
		t.Fatalf("expected 1 action (Delete(var)), got %d", collapsed.Size())
	}
	act := collapsed.Actions()[0]
	if act.Type != actions.Delete || act.Node != oldVarKeyword {
		t.Fatalf("expected Delete(var) to survive, got %+v", act)
	}
}

func TestCollapseBlockDelimitersPreserved(t *testing.T) {
	src := []byte("void f() { foo(); }\n")
	dst := []byte("void f() {\n  {\n    bar();\n    foo();\n  }\n}\n")

	srcTree, err := treesitter.Parse(src, "src.c")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.c")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	oldStmt, err := testutil.FindNodeByText(srcTree, src, "foo();", "expression_statement")
	if err != nil {
		t.Fatalf("find oldStmt: %v", err)
	}
	newBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n    bar();\n    foo();\n  }", "compound_statement")
	if err != nil {
		t.Fatalf("find newBlock: %v", err)
	}
	newInsertedChild, err := testutil.FindNodeByText(dstTree, dst, "bar();", "expression_statement")
	if err != nil {
		t.Fatalf("find newInsertedChild: %v", err)
	}
	newMovedChild, err := testutil.FindNodeByText(dstTree, dst, "foo();", "expression_statement")
	if err != nil {
		t.Fatalf("find newMovedChild: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(oldStmt, newMovedChild)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: newBlock})
	es.Add(actions.Action{Type: actions.Insert, Node: newInsertedChild})
	es.Add(actions.Action{Type: actions.Move, Node: oldStmt, DestNode: newMovedChild})

	collapsed := Collapse(es, ms, oldStmt, newBlock)
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
	src := []byte("package main\nfunc f() {\n\tcall()\n\trow--\n\tcol--\n}\n")
	dst := []byte("package main\nfunc g() {\n\tcall()\n}\nfunc f() {\n\tcallNew()\n\trow--\n\tcol--\n\tx = 1\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tcall()\n\trow--\n\tcol--\n}", "block")
	if err != nil {
		t.Fatalf("find srcBlock: %v", err)
	}
	callSrc, err := testutil.FindNodeByText(srcTree, src, "call()", "expression_statement")
	if err != nil {
		t.Fatalf("find callSrc: %v", err)
	}
	decRowSrc, err := testutil.FindNodeByText(srcTree, src, "row--", "dec_statement")
	if err != nil {
		t.Fatalf("find decRowSrc: %v", err)
	}
	decColSrc, err := testutil.FindNodeByText(srcTree, src, "col--", "dec_statement")
	if err != nil {
		t.Fatalf("find decColSrc: %v", err)
	}

	otherCallDst, err := testutil.FindNodeByText(dstTree, dst, "call()", "expression_statement")
	if err != nil {
		t.Fatalf("find otherCallDst: %v", err)
	}
	dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tcallNew()\n\trow--\n\tcol--\n\tx = 1\n}", "block")
	if err != nil {
		t.Fatalf("find dstBlock: %v", err)
	}
	decRowDst, err := testutil.FindNodeByText(dstTree, dst, "row--", "dec_statement")
	if err != nil {
		t.Fatalf("find decRowDst: %v", err)
	}
	decColDst, err := testutil.FindNodeByText(dstTree, dst, "col--", "dec_statement")
	if err != nil {
		t.Fatalf("find decColDst: %v", err)
	}

	identRowSrc, err := testutil.FindNodeByText(srcTree, src, "row", "identifier")
	if err != nil {
		t.Fatalf("find identRowSrc: %v", err)
	}
	identRowDst, err := testutil.FindNodeByText(dstTree, dst, "row", "identifier")
	if err != nil {
		t.Fatalf("find identRowDst: %v", err)
	}
	identColSrc, err := testutil.FindNodeByText(srcTree, src, "col", "identifier")
	if err != nil {
		t.Fatalf("find identColSrc: %v", err)
	}
	identColDst, err := testutil.FindNodeByText(dstTree, dst, "col", "identifier")
	if err != nil {
		t.Fatalf("find identColDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
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
	src := []byte("package main\nfunc f() {\n\tif true {\n\t\tfor {\n\t\t\tcall()\n\t\t}\n\t}\n\tother()\n}\n")
	dst := []byte("package main\nfunc f() {\n\tif true {\n\t\tfor {\n\t\t\tcall()\n\t\t\tother()\n\t\t}\n\t}\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	pSrc, err := testutil.FindNodeByText(srcTree, src, "if true {\n\t\tfor {\n\t\t\tcall()\n\t\t}\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find pSrc: %v", err)
	}
	cSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\t\tfor {\n\t\t\tcall()\n\t\t}\n\t}", "block")
	if err != nil {
		t.Fatalf("find cSrc: %v", err)
	}
	gcSrc, err := testutil.FindNodeByText(srcTree, src, "for {\n\t\t\tcall()\n\t\t}", "for_statement")
	if err != nil {
		t.Fatalf("find gcSrc: %v", err)
	}
	forBlockSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\t\t\tcall()\n\t\t}", "block")
	if err != nil {
		t.Fatalf("find forBlockSrc: %v", err)
	}
	leafSrc, err := testutil.FindNodeByText(srcTree, src, "call()", "expression_statement")
	if err != nil {
		t.Fatalf("find leafSrc: %v", err)
	}
	leafIDSrc, err := testutil.FindNodeByText(srcTree, src, "call", "identifier")
	if err != nil {
		t.Fatalf("find leafIDSrc: %v", err)
	}
	otherSrc, err := testutil.FindNodeByText(srcTree, src, "other()", "expression_statement")
	if err != nil {
		t.Fatalf("find otherSrc: %v", err)
	}

	pDst, err := testutil.FindNodeByText(dstTree, dst, "if true {\n\t\tfor {\n\t\t\tcall()\n\t\t\tother()\n\t\t}\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find pDst: %v", err)
	}
	cDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\t\tfor {\n\t\t\tcall()\n\t\t\tother()\n\t\t}\n\t}", "block")
	if err != nil {
		t.Fatalf("find cDst: %v", err)
	}
	gcDst, err := testutil.FindNodeByText(dstTree, dst, "for {\n\t\t\tcall()\n\t\t\tother()\n\t\t}", "for_statement")
	if err != nil {
		t.Fatalf("find gcDst: %v", err)
	}
	forBlockDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\t\t\tcall()\n\t\t\tother()\n\t\t}", "block")
	if err != nil {
		t.Fatalf("find forBlockDst: %v", err)
	}
	leafDst, err := testutil.FindNodeByText(dstTree, dst, "call()", "expression_statement")
	if err != nil {
		t.Fatalf("find leafDst: %v", err)
	}
	leafIDDst, err := testutil.FindNodeByText(dstTree, dst, "call", "identifier")
	if err != nil {
		t.Fatalf("find leafIDDst: %v", err)
	}
	otherDst, err := testutil.FindNodeByText(dstTree, dst, "other()", "expression_statement")
	if err != nil {
		t.Fatalf("find otherDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(pSrc, pDst)
	ms.Add(cSrc, cDst)
	ms.Add(gcSrc, gcDst)
	ms.Add(forBlockSrc, forBlockDst)
	ms.Add(leafSrc, leafDst)
	ms.Add(leafIDSrc, leafIDDst)
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
	src := []byte("package main\nfunc f() error {\n\treturn targetErr\n}\n")
	dst := []byte("package main\nfunc g() error {\n\treturn targetErr\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	stmt1, err := testutil.FindNodeByText(srcTree, src, "return targetErr", "return_statement")
	if err != nil {
		t.Fatalf("find stmt1: %v", err)
	}
	stmt2, err := testutil.FindNodeByText(dstTree, dst, "return targetErr", "return_statement")
	if err != nil {
		t.Fatalf("find stmt2: %v", err)
	}
	leafErr1, err := testutil.FindNodeByText(srcTree, src, "targetErr", "identifier")
	if err != nil {
		t.Fatalf("find leafErr1: %v", err)
	}
	leafErr2, err := testutil.FindNodeByText(dstTree, dst, "targetErr", "identifier")
	if err != nil {
		t.Fatalf("find leafErr2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcTree, dstTree)
	ms.Add(leafErr1, leafErr2) // leaf mapped cross-statement while statements themselves are unmapped

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: stmt1})
	es.Add(actions.Action{Type: actions.Insert, Node: stmt2})
	es.Add(actions.Action{Type: actions.Move, Node: leafErr1, DestNode: leafErr2})

	collapsed := Collapse(es, ms, srcTree, dstTree)

	for _, act := range collapsed.Actions() {
		if act.Type == actions.Move && act.Node == leafErr1 {
			t.Errorf("expected isolated leaf Move on 'targetErr' to be demoted to del+ins, but Move survived")
		}
	}
}

func TestHollowBlockMoveSuppression_Tier3B(t *testing.T) {
	src := []byte("package main\nfunc f() {\n\tif cond1 {\n\t\tcheckAuth(user, token)\n\t\tstmt1()\n\t\tstmt2()\n\t\tstmt3()\n\t}\n}\n")
	dst := []byte("package main\nfunc f() {\n\tif cond2 {\n\t\tcheckAuth(user, token)\n\t\tnewStmt1()\n\t\tnewStmt2()\n\t\tnewStmt3()\n\t}\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	ifSrc, err := testutil.FindNodeByText(srcTree, src, "if cond1 {\n\t\tcheckAuth(user, token)\n\t\tstmt1()\n\t\tstmt2()\n\t\tstmt3()\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find ifSrc: %v", err)
	}
	blockSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\t\tcheckAuth(user, token)\n\t\tstmt1()\n\t\tstmt2()\n\t\tstmt3()\n\t}", "block")
	if err != nil {
		t.Fatalf("find blockSrc: %v", err)
	}
	authGuardSrc, err := testutil.FindNodeByText(srcTree, src, "checkAuth(user, token)", "expression_statement")
	if err != nil {
		t.Fatalf("find authGuardSrc: %v", err)
	}

	ifDst, err := testutil.FindNodeByText(dstTree, dst, "if cond2 {\n\t\tcheckAuth(user, token)\n\t\tnewStmt1()\n\t\tnewStmt2()\n\t\tnewStmt3()\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find ifDst: %v", err)
	}
	blockDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\t\tcheckAuth(user, token)\n\t\tnewStmt1()\n\t\tnewStmt2()\n\t\tnewStmt3()\n\t}", "block")
	if err != nil {
		t.Fatalf("find blockDst: %v", err)
	}
	authGuardDst, err := testutil.FindNodeByText(dstTree, dst, "checkAuth(user, token)", "expression_statement")
	if err != nil {
		t.Fatalf("find authGuardDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(blockSrc, blockDst)
	ms.Add(authGuardSrc, authGuardDst)
	for i := range authGuardSrc.Children {
		ms.Add(authGuardSrc.Children[i], authGuardDst.Children[i])
		for j := range authGuardSrc.Children[i].Children {
			ms.Add(authGuardSrc.Children[i].Children[j], authGuardDst.Children[i].Children[j])
		}
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

func TestHollowBlockMoveSuppression_DemotesUnderThresholdChild(t *testing.T) {
	src := []byte("package main\nfunc RecoverGlueAnchors() {\n\tif true {\n\t\tm.Add(kw1, kw2)\n\t}\n}\n")
	dst := []byte("package main\nfunc RecoverGlueAnchors() {\n\tif true {\n\t\toldStmt()\n\t}\n}\n" +
		strings.Repeat("\n", 300) +
		"func pairKeywordGlue() {\n\tif true {\n\t\tm.Add(kw1, kw2)\n\t}\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	ifSrc, err := testutil.FindNodeByText(srcTree, src, "if true {\n\t\tm.Add(kw1, kw2)\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find ifSrc: %v", err)
	}
	blockSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\t\tm.Add(kw1, kw2)\n\t}", "block")
	if err != nil {
		t.Fatalf("find blockSrc: %v", err)
	}
	callSrc, err := testutil.FindNodeByText(srcTree, src, "m.Add(kw1, kw2)", "expression_statement")
	if err != nil {
		t.Fatalf("find callSrc: %v", err)
	}

	ifDst, err := testutil.FindNodeByText(dstTree, dst, "if true {\n\t\tm.Add(kw1, kw2)\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find ifDst: %v", err)
	}
	blockDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\t\tm.Add(kw1, kw2)\n\t}", "block")
	if err != nil {
		t.Fatalf("find blockDst: %v", err)
	}
	callDst, err := testutil.FindNodeByText(dstTree, dst, "m.Add(kw1, kw2)", "expression_statement")
	if err != nil {
		t.Fatalf("find callDst: %v", err)
	}

	ms := engine.NewMapping()
	srcNodes := blockSrc.PostOrder()
	dstNodes := blockDst.PostOrder()
	for i := range srcNodes {
		ms.Add(srcNodes[i], dstNodes[i])
	}

	actionPtrs := []*actions.Action{
		{Type: actions.Delete, Node: ifSrc},
		{Type: actions.Insert, Node: ifDst},
		{Type: actions.Move, Node: blockSrc, DestNode: blockDst, Subtree: true},
	}
	moved := map[*treesitter.ASTNode]*actions.Action{
		blockSrc: actionPtrs[2],
	}
	inserted := map[*treesitter.ASTNode]*actions.Action{
		ifDst: actionPtrs[1],
	}
	deleted := map[*treesitter.ASTNode]*actions.Action{
		ifSrc: actionPtrs[0],
	}
	suppressed := make(map[*actions.Action]bool)

	suppressHollowBlockMoves(&actionPtrs, moved, inserted, deleted, suppressed, ms)

	hasActiveMove := slices.ContainsFunc(actionPtrs, func(a *actions.Action) bool {
		return !suppressed[a] && a.Type == actions.Move && a.Node == callSrc
	})
	if hasActiveMove {
		t.Errorf("expected under-threshold promoted child move on callSrc to be demoted, but Move survived")
	}
	testutil.AssertUnmappedText(t, srcTree, src, ms.Src(), "m.Add(kw1, kw2)", "expression_statement")
	if deleted[callSrc] == nil || deleted[callSrc].Type != actions.Delete {
		t.Errorf("expected replacement Delete action for callSrc, got nil or wrong type")
	}
	if inserted[callDst] == nil || inserted[callDst].Type != actions.Insert {
		t.Errorf("expected replacement Insert action for callDst, got nil or wrong type")
	}
}

func TestNearAtomicSubtreeCollapsing_Tier3C(t *testing.T) {
	// Source container has 10 statements: 9 deleted, 1 low-mass "return nil" matched.
	var srcLines []string
	srcLines = append(srcLines, "package main\nfunc f() error {")
	for i := 1; i <= 9; i++ {
		srcLines = append(srcLines, fmt.Sprintf("\twork%d()", i))
	}
	srcLines = append(srcLines, "\treturn nil\n}\n")
	src := []byte(strings.Join(srcLines, "\n"))

	var dstLines []string
	dstLines = append(dstLines, "package main\nfunc f() error {")
	for i := 1; i <= 9; i++ {
		dstLines = append(dstLines, fmt.Sprintf("\tnewWork%d()", i))
	}
	dstLines = append(dstLines, "\treturn nil\n}\n")
	dst := []byte(strings.Join(dstLines, "\n"))

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcBlockText := strings.TrimSpace(string(src)[strings.Index(string(src), "{"):])
	dstBlockText := strings.TrimSpace(string(dst)[strings.Index(string(dst), "{"):])

	blockSrc, err := testutil.FindNodeByText(srcTree, src, srcBlockText, "block")
	if err != nil {
		t.Fatalf("find blockSrc: %v", err)
	}
	blockDst, err := testutil.FindNodeByText(dstTree, dst, dstBlockText, "block")
	if err != nil {
		t.Fatalf("find blockDst: %v", err)
	}

	retNilSrc, err := testutil.FindNodeByText(srcTree, src, "return nil", "return_statement")
	if err != nil {
		t.Fatalf("find retNilSrc: %v", err)
	}
	retNilDst, err := testutil.FindNodeByText(dstTree, dst, "return nil", "return_statement")
	if err != nil {
		t.Fatalf("find retNilDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(retNilSrc, retNilDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
	for i := 1; i <= 9; i++ {
		stmt, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("work%d()", i), "expression_statement")
		if err != nil {
			t.Fatalf("find src work%d: %v", i, err)
		}
		es.Add(actions.Action{Type: actions.Delete, Node: stmt, Subtree: true})
	}
	es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
	for i := 1; i <= 9; i++ {
		stmt, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("newWork%d()", i), "expression_statement")
		if err != nil {
			t.Fatalf("find dst newWork%d: %v", i, err)
		}
		es.Add(actions.Action{Type: actions.Insert, Node: stmt, Subtree: true})
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
		src := []byte("package main\nfunc f() error {\n\toldWork1()\n\toldWork2()\n\toldWork3()\n\treturn nil\n}\n")
		dst := []byte("package main\nfunc f() error {\n\tnewWork1()\n\tnewWork2()\n\tnewWork3()\n\treturn nil\n}\n")

		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		blockSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\toldWork1()\n\toldWork2()\n\toldWork3()\n\treturn nil\n}", "block")
		if err != nil {
			t.Fatalf("find blockSrc: %v", err)
		}
		blockDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\tnewWork1()\n\tnewWork2()\n\tnewWork3()\n\treturn nil\n}", "block")
		if err != nil {
			t.Fatalf("find blockDst: %v", err)
		}

		retNilSrc, err := testutil.FindNodeByText(srcTree, src, "return nil", "return_statement")
		if err != nil {
			t.Fatalf("find retNilSrc: %v", err)
		}
		retNilDst, err := testutil.FindNodeByText(dstTree, dst, "return nil", "return_statement")
		if err != nil {
			t.Fatalf("find retNilDst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(retNilSrc, retNilDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
		for i := 1; i <= 3; i++ {
			stmt, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("oldWork%d()", i), "expression_statement")
			if err != nil {
				t.Fatalf("find oldWork%d: %v", i, err)
			}
			es.Add(actions.Action{Type: actions.Delete, Node: stmt, Subtree: true})
		}
		es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
		for i := 1; i <= 3; i++ {
			stmt, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("newWork%d()", i), "expression_statement")
			if err != nil {
				t.Fatalf("find newWork%d: %v", i, err)
			}
			es.Add(actions.Action{Type: actions.Insert, Node: stmt, Subtree: true})
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
		src := []byte("package main\nfunc f() error {\n\toldWork1()\n\toldWork2()\n\treturn nil\n}\n")
		dst := []byte("package main\nfunc f() error {\n\tnewWork1()\n\tnewWork2()\n\treturn nil\n}\n")

		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		blockSrc, err := testutil.FindNodeByText(srcTree, src, "{\n\toldWork1()\n\toldWork2()\n\treturn nil\n}", "block")
		if err != nil {
			t.Fatalf("find blockSrc: %v", err)
		}
		blockDst, err := testutil.FindNodeByText(dstTree, dst, "{\n\tnewWork1()\n\tnewWork2()\n\treturn nil\n}", "block")
		if err != nil {
			t.Fatalf("find blockDst: %v", err)
		}

		retNilSrc, err := testutil.FindNodeByText(srcTree, src, "return nil", "return_statement")
		if err != nil {
			t.Fatalf("find retNilSrc: %v", err)
		}
		retNilDst, err := testutil.FindNodeByText(dstTree, dst, "return nil", "return_statement")
		if err != nil {
			t.Fatalf("find retNilDst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(retNilSrc, retNilDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Delete, Node: blockSrc})
		for i := 1; i <= 2; i++ {
			stmt, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("oldWork%d()", i), "expression_statement")
			if err != nil {
				t.Fatalf("find oldWork%d: %v", i, err)
			}
			es.Add(actions.Action{Type: actions.Delete, Node: stmt, Subtree: true})
		}
		es.Add(actions.Action{Type: actions.Insert, Node: blockDst})
		for i := 1; i <= 2; i++ {
			stmt, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("newWork%d()", i), "expression_statement")
			if err != nil {
				t.Fatalf("find newWork%d: %v", i, err)
			}
			es.Add(actions.Action{Type: actions.Insert, Node: stmt, Subtree: true})
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

func TestPromoteOrphanedChildren_SkipsHollowUndelimitedWrapper(t *testing.T) {
	src := []byte("package main\nfunc f() {\n\topDeleted = true\n\tcheck(a > 0)\n}\n")
	dst := []byte("package main\nfunc f() {\n\topDeleted := slices.ContainsFunc(items, func() bool { return a > 0 })\n}\n")

	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	fnSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnSrc: %v", err)
	}
	fnDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find fnDst: %v", err)
	}

	assignSrc, err := testutil.FindNodeByText(srcTree, src, "opDeleted = true", "assignment_statement")
	if err != nil {
		t.Fatalf("find assignSrc: %v", err)
	}
	lhsListSrc, err := testutil.FindNodeByText(srcTree, src, "opDeleted", "expression_list")
	if err != nil {
		t.Fatalf("find lhsListSrc: %v", err)
	}
	lhsIdentSrc, err := testutil.FindNodeByText(srcTree, src, "opDeleted", "identifier")
	if err != nil {
		t.Fatalf("find lhsIdentSrc: %v", err)
	}
	eqSrc, err := testutil.FindNodeByText(srcTree, src, "=", "assignment_operator_literal")
	if err != nil {
		t.Fatalf("find eqSrc: %v", err)
	}
	trueLitSrc, err := testutil.FindNodeByText(srcTree, src, "true", "true")
	if err != nil {
		t.Fatalf("find trueLitSrc: %v", err)
	}
	rhsListSrc, err := testutil.FindNodeByText(srcTree, src, "true", "expression_list")
	if err != nil {
		t.Fatalf("find rhsListSrc: %v", err)
	}

	shortVarDst, err := testutil.FindNodeByText(dstTree, dst, "opDeleted := slices.ContainsFunc(items, func() bool { return a > 0 })", "short_var_declaration")
	if err != nil {
		t.Fatalf("find shortVarDst: %v", err)
	}
	lhsListDst, err := testutil.FindNodeByText(dstTree, dst, "opDeleted", "expression_list")
	if err != nil {
		t.Fatalf("find lhsListDst: %v", err)
	}
	lhsIdentDst, err := testutil.FindNodeByText(dstTree, dst, "opDeleted", "identifier")
	if err != nil {
		t.Fatalf("find lhsIdentDst: %v", err)
	}
	colonEqDst, err := testutil.FindNodeByText(dstTree, dst, ":=", "assignment_operator_literal")
	if err != nil {
		t.Fatalf("find colonEqDst: %v", err)
	}
	callDst, err := testutil.FindNodeByText(dstTree, dst, "slices.ContainsFunc(items, func() bool { return a > 0 })", "call_expression")
	if err != nil {
		t.Fatalf("find callDst: %v", err)
	}
	rhsListDst, err := testutil.FindNodeByText(dstTree, dst, "slices.ContainsFunc(items, func() bool { return a > 0 })", "expression_list")
	if err != nil {
		t.Fatalf("find rhsListDst: %v", err)
	}

	condSrc, err := testutil.FindNodeByText(srcTree, src, "a > 0", "binary_expression")
	if err != nil {
		t.Fatalf("find condSrc: %v", err)
	}
	condDst, err := testutil.FindNodeByText(dstTree, dst, "a > 0", "binary_expression")
	if err != nil {
		t.Fatalf("find condDst: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(fnSrc, fnDst)
	ms.Add(assignSrc, shortVarDst)
	ms.Add(lhsListSrc, lhsListDst)
	ms.Add(lhsIdentSrc, lhsIdentDst)
	ms.Add(eqSrc, colonEqDst)
	ms.Add(condSrc, condDst) // cross-mapped into callDst from outside assignSrc, triggering Subtree demotion
	ms.Add(rhsListSrc, rhsListDst)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     assignSrc,
		DestNode: shortVarDst,
		Parent:   shortVarDst,
		Subtree:  true,
	})
	es.Add(actions.Action{Type: actions.Update, Node: eqSrc, DestNode: colonEqDst})
	es.Add(actions.Action{Type: actions.Delete, Node: trueLitSrc})
	es.Add(actions.Action{Type: actions.Insert, Node: callDst, Subtree: true})

	collapsed := Collapse(es, ms, assignSrc, shortVarDst)

	for _, act := range collapsed.Actions() {
		if act.Type == actions.Move && act.Node == rhsListSrc {
			t.Errorf("expected hollow undelimited wrapper rhsListSrc NOT to be promoted to Move, got %+v", act)
		}
	}
	testutil.AssertUnmappedText(t, srcTree, src, ms.Src(), "true", "expression_list")
}

func TestMultilineExpressionUnwrapping_SurvivingOperandPreserved(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	for _, c := range n.Children {
		if (r != nil && (r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type))) ||
			(r == nil && (rules.IsOperatorLiteral(c.Type) || rules.IsPunctuation(c.Type))) {
			return c.Label
		}
	}
	return ""
}
`
		newSrc := `package main

func getBinaryOperatorLabel(n *ASTNode, r *Rules) string {
	if n == nil {
		return ""
	}
	r = cmp.Or(r, rules.Get(""))
	for _, c := range n.Children {
		if r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type) {
			return c.Label
		}
	}
	return ""
}
`
		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if oldAST.HasError || oldAST.ParseErrorCount > 0 {
			t.Fatalf("oldAST has parse errors: %d", oldAST.ParseErrorCount)
		}
		if newAST.HasError || newAST.ParseErrorCount > 0 {
			t.Fatalf("newAST has parse errors: %d", newAST.ParseErrorCount)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		ms := mr.Mappings

		// 1. Enclosing function declaration mapped
		testutil.AssertMappedText(t, oldAST, newAST, []byte(oldSrc), []byte(newSrc), ms.Src(),
			"func getBinaryOperatorLabel", "func getBinaryOperatorLabel", "function_declaration",
		)

		// 2. Surrounding statements mapped in place
		testutil.AssertMappedText(t, oldAST, newAST, []byte(oldSrc), []byte(newSrc), ms.Src(),
			"for _, c := range n.Children", "for _, c := range n.Children", "for_statement",
		)
		testutil.AssertMappedText(t, oldAST, newAST, []byte(oldSrc), []byte(newSrc), ms.Src(),
			"return c.Label", "return c.Label", "return_statement",
		)

		// 3. Target stationary condition mapped
		testutil.AssertMappedText(t, oldAST, newAST, []byte(oldSrc), []byte(newSrc), ms.Src(),
			"r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type)",
			"r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type)",
			"binary_expression",
		)

		// 4. Edit script postprocessing must suppress the outer binary_expression delete
		es := actions.GenerateEditScript(oldAST, newAST, ms)
		postprocessed := Run(es, ms, oldAST, newAST)

		oldOuterCond, err := testutil.FindNodeByText(oldAST, []byte(oldSrc), "(r != nil && (r.IsOperatorLiteral(c.Type) || r.IsPunctuation(c.Type))) ||", "binary_expression")
		if err != nil {
			t.Fatalf("failed to locate old outer binary_expression: %v", err)
		}
		for _, a := range postprocessed.Actions() {
			if a.Node == oldOuterCond && a.Type == actions.Delete {
				t.Fatalf("expected outer binary_expression Delete action to be suppressed because inner operand is matched, got action: %+v", a)
			}
		}
	})
}
