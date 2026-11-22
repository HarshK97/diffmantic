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
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestNormalizeMovesByStructure(t *testing.T) {
	ms := engine.NewMapping()

	t.Run("demotes boilerplate if block across distant scope", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\tif err != nil {\n\t\treturn\n\t}\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func g() {\n\tif err != nil {\n\t\treturn\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcIf, err := testutil.FindNodeByText(srcTree, src, "if err != nil {\n\t\treturn\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find srcIf: %v", err)
		}
		dstIf, err := testutil.FindNodeByText(dstTree, dst, "if err != nil {\n\t\treturn\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find dstIf: %v", err)
		}

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcIf, DestNode: dstIf})

		result := normalizeMovesByStructure(es, ms)
		// Boilerplate if-block across 490 lines should be demoted to delete+insert
		if result.Size() != 2 {
			t.Fatalf("expected 2 actions (delete+insert) for demoted boilerplate, got %d", result.Size())
		}
		if result.Actions()[0].Type != actions.Delete || result.Actions()[1].Type != actions.Insert {
			t.Errorf("expected delete then insert, got %v and %v", result.Actions()[0].Type, result.Actions()[1].Type)
		}
	})

	t.Run("preserves declaration move", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func foo() {}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func bar() {}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func foo() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func bar() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcFunc, DestNode: dstFunc})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected declaration move to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("suppresses orphaned update when paired move is demoted", func(t *testing.T) {
		// Chawathe emits Update + Move on the same node when labels differ.
		// When the Move is demoted, the Update must also be suppressed.
		src := []byte("const x = 'main';\n")
		dst := []byte(strings.Repeat("\n", 500) + "const y = '2.0.0';\n")
		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcVal, err := testutil.FindNodeByText(srcTree, src, "'main'", "string")
		if err != nil {
			t.Fatalf("find srcVal: %v", err)
		}
		dstVal, err := testutil.FindNodeByText(dstTree, dst, "'2.0.0'", "string")
		if err != nil {
			t.Fatalf("find dstVal: %v", err)
		}

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Update, Node: srcVal, DestNode: dstVal})
		es.Add(actions.Action{Type: actions.Move, Node: srcVal, DestNode: dstVal})

		result := normalizeMovesByStructure(es, ms)
		for _, a := range result.Actions() {
			if a.Type == actions.Update {
				t.Error("expected orphaned Update to be suppressed when paired Move is demoted")
			}
		}
		// Should have Delete + Insert from the demoted Move, no Update.
		if result.Size() != 2 {
			t.Errorf("expected 2 actions (delete+insert), got %d", result.Size())
		}
	})

	t.Run("preserves sibling relocation within same parent", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tcall(argAlpha, argBeta)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		srcArg, err := testutil.FindNodeByText(srcTree, src, "argAlpha", "identifier")
		if err != nil {
			t.Fatalf("find srcArg: %v", err)
		}
		dstArg, err := testutil.FindNodeByText(srcTree, src, "argBeta", "identifier")
		if err != nil {
			t.Fatalf("find dstArg: %v", err)
		}
		parent := srcArg.Parent

		ms2 := engine.NewMapping()
		ms2.Add(srcFunc, srcFunc)
		ms2.Add(parent, parent)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcArg, DestNode: dstArg})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected sibling relocation to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves one-hop container-preserving reparent", func(t *testing.T) {
		// Value wrapped in a brand-new wrapper node, but the matched container is intact.
		src := []byte("package main\nfunc f() {\n\tcall(valItem)\n}\n")
		dst := []byte("package main\nfunc f() {\n\tcall((valItem))\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		srcContainer, err := testutil.FindNodeByText(srcTree, src, "(valItem)", "argument_list")
		if err != nil {
			t.Fatalf("find srcContainer: %v", err)
		}
		dstContainer, err := testutil.FindNodeByText(dstTree, dst, "((valItem))", "argument_list")
		if err != nil {
			t.Fatalf("find dstContainer: %v", err)
		}
		srcVal, err := testutil.FindNodeByText(srcTree, src, "valItem", "identifier")
		if err != nil {
			t.Fatalf("find srcVal: %v", err)
		}
		dstVal, err := testutil.FindNodeByText(dstTree, dst, "valItem", "identifier")
		if err != nil {
			t.Fatalf("find dstVal: %v", err)
		}

		ms2 := engine.NewMapping()
		ms2.Add(srcFunc, dstFunc)
		ms2.Add(srcContainer, dstContainer)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcVal, DestNode: dstVal})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected one-hop reparent to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves intra-scope move with moderate drift", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func foo() {\n\tstmtA()\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 29) + "func foo() {\n\tstmtA()\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func foo()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func foo()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		srcStmt, err := testutil.FindNodeByText(srcTree, src, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms2 := engine.NewMapping()
		ms2.Add(srcFunc, dstFunc)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected intra-scope move with drift=20 to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("demotes cross-scope move with large drift and no mapped src decl", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f1() {\n\tstmtA()\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 199) + "func f2() {\n\tstmtA()\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcStmt, err := testutil.FindNodeByText(srcTree, src, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		// No parent mapping: cross-scope with no mapped enclosing decl.
		ms2 := engine.NewMapping()

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() == 1 && result.Actions()[0].Type == actions.Move {
			t.Error("expected cross-scope move with large drift and no mapped decl to be demoted")
		}
	})

	t.Run("demotes small inlined expression across different statements", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 18) + "func init() {\n\tpairCount := len(pairs)\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 23) + "func init() {\n\ts := len(pairs)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFn, err := testutil.FindNodeByText(srcTree, src, "func init()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFn: %v", err)
		}
		dstFn, err := testutil.FindNodeByText(dstTree, dst, "func init()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFn: %v", err)
		}
		srcCall, err := testutil.FindNodeByText(srcTree, src, "len(pairs)", "call_expression")
		if err != nil {
			t.Fatalf("find srcCall: %v", err)
		}
		dstCall, err := testutil.FindNodeByText(dstTree, dst, "len(pairs)", "call_expression")
		if err != nil {
			t.Fatalf("find dstCall: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFn, dstFn)
		ms.Add(srcCall, dstCall)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcCall, DestNode: dstCall})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 2 {
			t.Fatalf("expected 2 actions (delete+insert) for small inlined expression, got %d", result.Size())
		}
		if result.Actions()[0].Type != actions.Delete || result.Actions()[1].Type != actions.Insert {
			t.Errorf("expected delete then insert, got %v and %v", result.Actions()[0].Type, result.Actions()[1].Type)
		}
	})

	t.Run("preserves substantial inlined call across different statements", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 18) + `func init() {
	v := format(
		a1, a2, a3, a4, a5,
		a6, a7, a8, a9, a10,
	)
}
`)
		dst := []byte("package main\n" + strings.Repeat("\n", 23) + `func init() {
	res := format(
		a1, a2, a3, a4, a5,
		a6, a7, a8, a9, a10,
	)
}
`)
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFn, err := testutil.FindNodeByText(srcTree, src, "func init()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFn: %v", err)
		}
		dstFn, err := testutil.FindNodeByText(dstTree, dst, "func init()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFn: %v", err)
		}
		srcCall, err := testutil.FindNodeByText(srcTree, src, "format(\n\t\ta1, a2, a3, a4, a5,\n\t\ta6, a7, a8, a9, a10,\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find srcCall: %v", err)
		}
		dstCall, err := testutil.FindNodeByText(dstTree, dst, "format(\n\t\ta1, a2, a3, a4, a5,\n\t\ta6, a7, a8, a9, a10,\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find dstCall: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFn, dstFn)
		ms.Add(srcCall, dstCall)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcCall, DestNode: dstCall})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Fatalf("expected 1 Move action for substantial inlined call, got %d actions", result.Size())
		}
	})

	t.Run("non-move actions pass through unchanged", func(t *testing.T) {
		src := []byte("package main\nvar xVar int\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		node, err := testutil.FindNodeByText(srcTree, src, "xVar", "identifier")
		if err != nil {
			t.Fatalf("find node: %v", err)
		}

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: node})
		es.Add(actions.Action{Type: actions.Delete, Node: node})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 2 {
			t.Errorf("expected non-move actions to pass through, got %d", result.Size())
		}
	})

	t.Run("nested move inside demoted ancestor move is suppressed", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\tvalFunc()\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func g() {\n\tvalFunc()\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		parentSrc, err := testutil.FindNodeByText(srcTree, src, "valFunc()", "expression_statement")
		if err != nil {
			t.Fatalf("find parentSrc: %v", err)
		}
		childSrc, err := testutil.FindNodeByText(srcTree, src, "valFunc()", "call_expression")
		if err != nil {
			t.Fatalf("find childSrc: %v", err)
		}
		parentDst, err := testutil.FindNodeByText(dstTree, dst, "valFunc()", "expression_statement")
		if err != nil {
			t.Fatalf("find parentDst: %v", err)
		}
		childDst, err := testutil.FindNodeByText(dstTree, dst, "valFunc()", "call_expression")
		if err != nil {
			t.Fatalf("find childDst: %v", err)
		}

		msNested := engine.NewMapping()
		msNested.Add(parentSrc, parentDst)
		msNested.Add(childSrc, childDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})
		es.Add(actions.Action{Type: actions.Move, Node: childSrc, DestNode: childDst})

		result := normalizeMovesByStructure(es, msNested)
		if result.Size() != 2 {
			t.Fatalf("expected exactly 2 actions (Delete+Insert) after ancestor demotion, got %d", result.Size())
		}
		if idx := slices.IndexFunc(result.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move
		}); idx != -1 {
			a := result.Actions()[idx]
			t.Fatalf("expected no move actions to survive demotion of ancestor move, got %s on %s", a.Type, a.Node.Type)
		}
	})

	t.Run("nested move inside demoted ancestor wrapper is not suppressed and demotes to delete and insert", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\t_ = (checkNode)\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func g() {\n\t_ = (checkNode)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		parentSrc, err := testutil.FindNodeByText(srcTree, src, "(checkNode)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find parentSrc: %v", err)
		}
		childSrc, err := testutil.FindNodeByText(srcTree, src, "checkNode", "identifier")
		if err != nil {
			t.Fatalf("find childSrc: %v", err)
		}
		parentDst, err := testutil.FindNodeByText(dstTree, dst, "(checkNode)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find parentDst: %v", err)
		}
		childDst, err := testutil.FindNodeByText(dstTree, dst, "checkNode", "identifier")
		if err != nil {
			t.Fatalf("find childDst: %v", err)
		}

		msNested := engine.NewMapping()
		msNested.Add(parentSrc, parentDst)
		msNested.Add(childSrc, childDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})
		es.Add(actions.Action{Type: actions.Move, Node: childSrc, DestNode: childDst})

		result := normalizeMovesByStructure(es, msNested)
		foundChildDelete := false
		foundChildInsert := false
		for _, a := range result.Actions() {
			if a.Node == childSrc && a.Type == actions.Delete {
				foundChildDelete = true
			}
			if a.Node == childDst && a.Type == actions.Insert {
				foundChildInsert = true
			}
		}
		if !foundChildDelete || !foundChildInsert {
			t.Errorf("expected childSrc to be demoted to delete and insert, got actions: %+v", result.Actions())
		}
	})

	t.Run("descendants without own move are evicted and demoted to full subtree delete and insert", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 99) + "func f() {\n" + strings.Repeat("\n", 9) + "\ttargetVar = min(v1, v2)\n" + strings.Repeat("\n", 39) + "}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 99) + "func f() {\n" + strings.Repeat("\n", 8) + "\tresVar := min(v1, v2)\n" + strings.Repeat("\n", 40) + "}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		declSrc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find declSrc: %v", err)
		}
		declDst, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find declDst: %v", err)
		}
		parentSrc, err := testutil.FindNodeByText(srcTree, src, "min(v1, v2)", "expression_list")
		if err != nil {
			t.Fatalf("find parentSrc: %v", err)
		}
		parentDst, err := testutil.FindNodeByText(dstTree, dst, "min(v1, v2)", "expression_list")
		if err != nil {
			t.Fatalf("find parentDst: %v", err)
		}
		childSrc, err := testutil.FindNodeByText(srcTree, src, "min(v1, v2)", "call_expression")
		if err != nil {
			t.Fatalf("find childSrc: %v", err)
		}
		childDst, err := testutil.FindNodeByText(dstTree, dst, "min(v1, v2)", "call_expression")
		if err != nil {
			t.Fatalf("find childDst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(declSrc, declDst)
		ms.Add(parentSrc, parentDst)
		ms.Add(childSrc, childDst)

		// Only parentSrc has a Move action in es (childSrc moves implicitly).
		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})

		result := normalizeMovesByStructure(es, ms)

		if ms.Has(childSrc) {
			t.Errorf("expected childSrc to be evicted from mapping, but still present")
		}
		if ms.HasDst(childDst) {
			t.Errorf("expected childDst to be evicted from mapping destination, but still present")
		}

		foundDeleteSubtree := false
		foundInsertSubtree := false
		for _, a := range result.Actions() {
			if a.Node == parentSrc && a.Type == actions.Delete && a.Subtree {
				foundDeleteSubtree = true
			}
			if a.Node == parentDst && a.Type == actions.Insert && a.Subtree {
				foundInsertSubtree = true
			}
			if a.Type == actions.Move {
				t.Errorf("unexpected move action survived: %+v", a)
			}
		}
		if !foundDeleteSubtree || !foundInsertSubtree {
			t.Errorf("expected parentSrc to demote with Subtree: true, got actions: %+v", result.Actions())
		}
	})
}

func TestShouldDemoteMove(t *testing.T) {
	r := rules.Get("go")

	t.Run("returns false for sibling relocation", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tcall(argAlpha, argBeta)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		srcNode, err := testutil.FindNodeByText(srcTree, src, "argAlpha", "identifier")
		if err != nil {
			t.Fatalf("find src: %v", err)
		}
		dstNode, err := testutil.FindNodeByText(srcTree, src, "argBeta", "identifier")
		if err != nil {
			t.Fatalf("find dst: %v", err)
		}
		parent := srcNode.Parent

		ms := engine.NewMapping()
		ms.Add(srcFunc, srcFunc)
		ms.Add(parent, parent)

		if shouldDemoteMove(srcNode, dstNode, ms, r) {
			t.Error("expected sibling relocation to not be demoted")
		}
	})

	t.Run("demotes sibling relocation across different enclosing scopes", func(t *testing.T) {
		src := []byte("package main\nfunc funcA() {\n\t_ = varAlpha + 1\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func funcB() {\n\t_ = varBeta + 1\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		parent1, err := testutil.FindNodeByText(srcTree, src, "varAlpha + 1", "binary_expression")
		if err != nil {
			t.Fatalf("find parent1: %v", err)
		}
		srcNode, err := testutil.FindNodeByText(srcTree, src, "varAlpha", "identifier")
		if err != nil {
			t.Fatalf("find srcNode: %v", err)
		}
		parent2, err := testutil.FindNodeByText(dstTree, dst, "varBeta + 1", "binary_expression")
		if err != nil {
			t.Fatalf("find parent2: %v", err)
		}
		dstNode, err := testutil.FindNodeByText(dstTree, dst, "varBeta", "identifier")
		if err != nil {
			t.Fatalf("find dstNode: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(parent1, parent2)

		if !shouldDemoteMove(srcNode, dstNode, ms, r) {
			t.Error("expected sibling relocation across different enclosing functions to be demoted")
		}
	})

	t.Run("returns false for declaration move", func(t *testing.T) {
		src := []byte("package main\nfunc foo() {}\n")
		dst := []byte("package main\nfunc bar() {}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func foo() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func bar() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}

		ms := engine.NewMapping()

		if shouldDemoteMove(srcFunc, dstFunc, ms, r) {
			t.Error("expected declaration move to not be demoted")
		}
	})

	t.Run("returns true for boilerplate if block across distant scope", func(t *testing.T) {
		src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\tif err != nil {\n\t\treturn\n\t}\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 499) + "func g() {\n\tif err != nil {\n\t\treturn\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcIf, err := testutil.FindNodeByText(srcTree, src, "if err != nil {\n\t\treturn\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find srcIf: %v", err)
		}
		dstIf, err := testutil.FindNodeByText(dstTree, dst, "if err != nil {\n\t\treturn\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find dstIf: %v", err)
		}

		ms := engine.NewMapping()

		if !shouldDemoteMove(srcIf, dstIf, ms, r) {
			t.Error("expected boilerplate if block across distant scope to be demoted")
		}
	})

	t.Run("returns false for nil arguments", func(t *testing.T) {
		src := []byte("package main\nvar xVar int\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		node, err := testutil.FindNodeByText(srcTree, src, "xVar", "identifier")
		if err != nil {
			t.Fatalf("find node: %v", err)
		}

		ms := engine.NewMapping()
		if shouldDemoteMove(nil, node, ms, r) {
			t.Error("expected false for nil src")
		}
		if shouldDemoteMove(node, nil, ms, r) {
			t.Error("expected false for nil dst")
		}
		if shouldDemoteMove(node, node, nil, r) {
			t.Error("expected false for nil mapping")
		}
		if shouldDemoteMove(node, node, ms, nil) {
			t.Error("expected false for nil rules")
		}
	})

	t.Run("returns true for cross-scope move with asymmetric destination mass", func(t *testing.T) {
		// Large src call with 40 arguments, tiny dst call with 1 argument across distant scopes (line 10 vs 350)
		srcArgs := make([]string, 40)
		for i := range srcArgs {
			srcArgs[i] = fmt.Sprintf("arg%d", i)
		}
		src := []byte("package main\nfunc funcA() {\n\tbigCall(" + strings.Join(srcArgs, ", ") + ")\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 350) + "func funcB() {\n\tsmallCall(argX)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcCall, err := testutil.FindNodeByText(srcTree, src, "bigCall("+strings.Join(srcArgs, ", ")+")", "call_expression")
		if err != nil {
			t.Fatalf("find srcCall: %v", err)
		}
		dstCall, err := testutil.FindNodeByText(dstTree, dst, "smallCall(argX)", "call_expression")
		if err != nil {
			t.Fatalf("find dstCall: %v", err)
		}

		ms := engine.NewMapping()
		if !shouldDemoteMove(srcCall, dstCall, ms, r) {
			t.Error("expected cross-scope move with tiny destination to be demoted via bidirectional scoring")
		}
	})

	t.Run("returns false for cross-scope move with substantial mass on both ends", func(t *testing.T) {
		// Substantial mass (80 arguments across multiple lines) on both ends clears the cross-scope drift penalty.
		args := make([]string, 80)
		for i := range args {
			args[i] = fmt.Sprintf("arg%d", i)
		}
		src := []byte("package main\nfunc funcA() {\n\tbigCall(\n\t\t" + strings.Join(args, ",\n\t\t") + ",\n\t)\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 350) + "func funcB() {\n\tbigCall(\n\t\t" + strings.Join(args, ",\n\t\t") + ",\n\t)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcCall, err := testutil.FindNodeByText(srcTree, src, "bigCall(\n\t\t"+strings.Join(args, ",\n\t\t")+",\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find srcCall: %v", err)
		}
		dstCall, err := testutil.FindNodeByText(dstTree, dst, "bigCall(\n\t\t"+strings.Join(args, ",\n\t\t")+",\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find dstCall: %v", err)
		}

		ms := engine.NewMapping()
		if shouldDemoteMove(srcCall, dstCall, ms, r) {
			t.Error("expected cross-scope move with substantial mass on both ends to be preserved")
		}
	})

	t.Run("demotes wrapper/block moves when no child moves with them into destination", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\t_ = (a && b)\n}\n")
		dst := []byte("package main\nfunc g() {\n\t_ = (c && d)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcParen, err := testutil.FindNodeByText(srcTree, src, "(a && b)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find srcParen: %v", err)
		}
		dstParen, err := testutil.FindNodeByText(dstTree, dst, "(c && d)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find dstParen: %v", err)
		}
		srcBin, err := testutil.FindNodeByText(srcTree, src, "a && b", "binary_expression")
		if err != nil {
			t.Fatalf("find srcBin: %v", err)
		}
		dstBin, err := testutil.FindNodeByText(dstTree, dst, "c && d", "binary_expression")
		if err != nil {
			t.Fatalf("find dstBin: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcParen, dstParen)
		ms.Add(srcBin, dstBin)

		// Only the wrapper and internal binary_expression match, no leaf expressions/identifiers.
		if !shouldDemoteMove(srcParen, dstParen, ms, r) {
			t.Error("expected empty wrapper move with no substantive child to be demoted")
		}
	})

	t.Run("preserves wrapper move when substantive leaf moves with it into destination", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tcall((fooItem))\n}\n")
		dst := []byte("package main\nfunc f() {\n\tcall((fooItem))\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		parent1, err := testutil.FindNodeByText(srcTree, src, "((fooItem))", "argument_list")
		if err != nil {
			t.Fatalf("find parent1: %v", err)
		}
		parent2, err := testutil.FindNodeByText(dstTree, dst, "((fooItem))", "argument_list")
		if err != nil {
			t.Fatalf("find parent2: %v", err)
		}
		srcParen, err := testutil.FindNodeByText(srcTree, src, "(fooItem)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find srcParen: %v", err)
		}
		dstParen, err := testutil.FindNodeByText(dstTree, dst, "(fooItem)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find dstParen: %v", err)
		}
		srcIdent, err := testutil.FindNodeByText(srcTree, src, "fooItem", "identifier")
		if err != nil {
			t.Fatalf("find srcIdent: %v", err)
		}
		dstIdent, err := testutil.FindNodeByText(dstTree, dst, "fooItem", "identifier")
		if err != nil {
			t.Fatalf("find dstIdent: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)
		ms.Add(parent1, parent2)
		ms.Add(srcParen, dstParen)
		ms.Add(srcIdent, dstIdent)

		if shouldDemoteMove(srcParen, dstParen, ms, r) {
			t.Error("expected wrapper move with surviving substantive child to be preserved")
		}
	})

	t.Run("demotes cross-scope container move with low retention", func(t *testing.T) {
		srcLines := make([]string, 20)
		dstLines := make([]string, 20)
		for i := range 20 {
			srcLines[i] = fmt.Sprintf("\t\t_ = src_%02d", i)
			dstLines[i] = fmt.Sprintf("\t\t_ = dst_%02d", i)
		}
		src := []byte("package main\nfunc f1() {\n\t{\n" + strings.Join(srcLines, "\n") + "\n\t}\n}\n")
		dst := []byte("package main\nfunc f2() {\n\t{\n" + strings.Join(dstLines, "\n") + "\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n"+strings.Join(srcLines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n"+strings.Join(dstLines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}
		src0, err := testutil.FindNodeByText(srcTree, src, "src_00", "identifier")
		if err != nil {
			t.Fatalf("find src0: %v", err)
		}
		dst0, err := testutil.FindNodeByText(dstTree, dst, "dst_00", "identifier")
		if err != nil {
			t.Fatalf("find dst0: %v", err)
		}
		src1, err := testutil.FindNodeByText(srcTree, src, "src_01", "identifier")
		if err != nil {
			t.Fatalf("find src1: %v", err)
		}
		dst1, err := testutil.FindNodeByText(dstTree, dst, "dst_01", "identifier")
		if err != nil {
			t.Fatalf("find dst1: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		// Map only 2 out of 20 leaves (10% retention < 25%).
		ms.Add(src0, dst0)
		ms.Add(src1, dst1)

		if !shouldDemoteMove(srcBlock, dstBlock, ms, r) {
			t.Error("expected cross-scope container move with low retention (10%) to be demoted")
		}
	})

	t.Run("preserves cross-scope container move with high retention", func(t *testing.T) {
		lines := make([]string, 20)
		for i := range 20 {
			lines[i] = fmt.Sprintf("\t\tval_%02d++", i)
		}
		src := []byte("package main\nfunc f1() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 25) + "func f2() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		// Map 18 out of 20 leaves (90% retention >= 25%).
		for i := range 18 {
			sNode, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("val_%02d", i), "identifier")
			if err != nil {
				t.Fatalf("find val_%02d: %v", i, err)
			}
			dNode, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("val_%02d", i), "identifier")
			if err != nil {
				t.Fatalf("find val_%02d: %v", i, err)
			}
			ms.Add(sNode, dNode)
		}

		if shouldDemoteMove(srcBlock, dstBlock, ms, r) {
			t.Error("expected cross-scope container move with high retention (90%) to be preserved")
		}
	})

	t.Run("anchor protection preserves high-value node despite cascade eviction", func(t *testing.T) {
		lines := make([]string, 20)
		for i := range 20 {
			lines[i] = fmt.Sprintf("\t\t_ = xVar_%02d", i)
		}
		src := []byte("package main\nfunc handlerA() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 200) + "func handlerB() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		srcLeaves := make([]*treesitter.ASTNode, 20)
		dstLeaves := make([]*treesitter.ASTNode, 20)
		for i := range 20 {
			sNode, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("xVar_%02d", i), "identifier")
			if err != nil {
				t.Fatalf("find xVar_%02d: %v", i, err)
			}
			dNode, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("xVar_%02d", i), "identifier")
			if err != nil {
				t.Fatalf("find xVar_%02d: %v", i, err)
			}
			srcLeaves[i] = sNode
			dstLeaves[i] = dNode
			ms.Add(sNode, dNode)
		}

		// Evict 8 of 20 leaves: retention drops from 100% to 60%.
		evicted := make(map[*treesitter.ASTNode]struct{})
		for i := range 8 {
			evicted[srcLeaves[i]] = struct{}{}
			evicted[dstLeaves[i]] = struct{}{}
		}

		if shouldDemoteMove(srcBlock, dstBlock, ms, r, evicted) {
			t.Error("expected anchor move (score >= 30 with intrinsic retention 100%) to resist cascade demotion")
		}
	})

	t.Run("anchor protection does not exempt node when cascade caused no retention loss", func(t *testing.T) {
		lines := make([]string, 8)
		for i := range 8 {
			lines[i] = fmt.Sprintf("\t\t_ = xVal%d", i)
		}
		src := []byte("package main\nfunc handlerA() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		dst := []byte("package main\n" + strings.Repeat("\n", 200) + "func handlerB() {\n\t{\n" + strings.Join(lines, "\n") + "\n\t}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n"+strings.Join(lines, "\n")+"\n\t}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		for i := range 8 {
			sNode, err := testutil.FindNodeByText(srcTree, src, fmt.Sprintf("xVal%d", i), "identifier")
			if err != nil {
				t.Fatalf("find xVal%d: %v", i, err)
			}
			dNode, err := testutil.FindNodeByText(dstTree, dst, fmt.Sprintf("xVal%d", i), "identifier")
			if err != nil {
				t.Fatalf("find xVal%d: %v", i, err)
			}
			ms.Add(sNode, dNode)
		}

		unrelatedCode := []byte("package main\nvar unrelatedVar int\n")
		unrelatedTree, err := treesitter.Parse(unrelatedCode, "unrelated.go")
		if err != nil {
			t.Fatalf("parse unrelated: %v", err)
		}
		unrelatedNode, err := testutil.FindNodeByText(unrelatedTree, unrelatedCode, "unrelatedVar", "identifier")
		if err != nil {
			t.Fatalf("find unrelated: %v", err)
		}
		evicted := map[*treesitter.ASTNode]struct{}{unrelatedNode: {}}

		// Node had 0 cascade degradation (retention == preEvictionRetention == 1.0),
		// so anchor protection must not exempt it from score < threshold demotion.
		if !shouldDemoteMove(srcBlock, dstBlock, ms, r, evicted) {
			t.Error("expected move below threshold without cascade degradation to be demoted")
		}
	})
}

func TestNormalizeMovesByStructure_ScopedEviction(t *testing.T) {
	// Container t1 moves across statements to dst1, but contains a substantial child call c1 mapped to c2 elsewhere.
	src := []byte("package main\n" + strings.Repeat("\n", 9) + `func f() {
	_ = (format(
		a1, a2, a3, a4, a5,
		a6, a7, a8, a9, a10,
	))
}
`)
	dst := []byte("package main\n" + strings.Repeat("\n", 49) + `func f() {
	_ = (empty())
` + strings.Repeat("\n", 50) + `	format(
		a1, a2, a3, a4, a5,
		a6, a7, a8, a9, a10,
	)
}
`)
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find srcFunc: %v", err)
	}
	dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find dstFunc: %v", err)
	}

	callText := "format(\n\t\ta1, a2, a3, a4, a5,\n\t\ta6, a7, a8, a9, a10,\n\t)"
	c1, err := testutil.FindNodeByText(srcTree, src, callText, "call_expression")
	if err != nil {
		t.Fatalf("find c1: %v", err)
	}
	t1, err := testutil.FindNodeByText(srcTree, src, "("+callText+")", "parenthesized_expression")
	if err != nil {
		t.Fatalf("find t1: %v", err)
	}
	dst1, err := testutil.FindNodeByText(dstTree, dst, "(empty())", "parenthesized_expression")
	if err != nil {
		t.Fatalf("find dst1: %v", err)
	}
	c2, err := testutil.FindNodeByText(dstTree, dst, callText, "call_expression")
	if err != nil {
		t.Fatalf("find c2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcFunc, dstFunc)
	ms.Add(t1, dst1)
	ms.Add(c1, c2)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: t1, DestNode: dst1})
	es.Add(actions.Action{Type: actions.Move, Node: c1, DestNode: c2})

	result := normalizeMovesByStructure(es, ms)

	if ms.Src()[c1] != c2 {
		t.Errorf("expected mapping of survivor child c1 to be preserved, got %v", ms.Src()[c1])
	}

	var c1Moved bool
	for _, a := range result.Actions() {
		if a.Node == c1 && a.Type == actions.Move {
			c1Moved = true
		}
		if a.Node == t1 && a.Type == actions.Delete && a.Subtree {
			t.Errorf("expected demoted container delete to have Subtree: false, got true")
		}
	}
	if !c1Moved {
		t.Errorf("expected survivor child Move action to be preserved")
	}
}

func TestNormalizeMovesByStructure_EmitsDeleteForEvictedDescendantsWhenNonSubtree(t *testing.T) {
	// Moving c1 out of t1 (and c0 into dst1) keeps the demoted container's Delete
	// and Insert non-subtree. Evicted children like opSrc/opDst that stayed inside
	// the container still need their own Delete and Insert actions.
	src := []byte("package main\n" + strings.Repeat("\n", 9) + `func f() {
	_ = survivorVar && true
}
func f0() {
	_ = survivor2Var
}
`)
	dst := []byte("package main\n" + strings.Repeat("\n", 49) + `func g() {
	_ = survivor2Var && true
}
func g2() {
` + strings.Repeat("\n", 29) + `	_ = survivorVar
}
`)
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	t1, err := testutil.FindNodeByText(srcTree, src, "survivorVar && true", "binary_expression")
	if err != nil {
		t.Fatalf("find t1: %v", err)
	}
	c1, err := testutil.FindNodeByText(srcTree, src, "survivorVar", "identifier")
	if err != nil {
		t.Fatalf("find c1: %v", err)
	}
	opSrc, err := testutil.FindNodeByText(srcTree, src, "true", "true")
	if err != nil {
		t.Fatalf("find opSrc: %v", err)
	}
	c0, err := testutil.FindNodeByText(srcTree, src, "survivor2Var", "identifier")
	if err != nil {
		t.Fatalf("find c0: %v", err)
	}

	dst1, err := testutil.FindNodeByText(dstTree, dst, "survivor2Var && true", "binary_expression")
	if err != nil {
		t.Fatalf("find dst1: %v", err)
	}
	c3, err := testutil.FindNodeByText(dstTree, dst, "survivor2Var", "identifier")
	if err != nil {
		t.Fatalf("find c3: %v", err)
	}
	opDst, err := testutil.FindNodeByText(dstTree, dst, "true", "true")
	if err != nil {
		t.Fatalf("find opDst: %v", err)
	}
	c2, err := testutil.FindNodeByText(dstTree, dst, "survivorVar", "identifier")
	if err != nil {
		t.Fatalf("find c2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(t1, dst1)
	ms.Add(opSrc, opDst)
	ms.Add(c1, c2)
	ms.Add(c0, c3)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: t1, DestNode: dst1})
	es.Add(actions.Action{Type: actions.Move, Node: c1, DestNode: c2})
	es.Add(actions.Action{Type: actions.Move, Node: c0, DestNode: c3})

	result := normalizeMovesByStructure(es, ms)

	opDeleted := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == opSrc && a.Type == actions.Delete
	})
	opInserted := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == opDst && a.Type == actions.Insert
	})

	if !opDeleted {
		t.Errorf("normalizeMovesByStructure() missing Delete for opSrc; got actions = %+v", result.Actions())
	}
	if !opInserted {
		t.Errorf("normalizeMovesByStructure() missing Insert for opDst; got actions = %+v", result.Actions())
	}
	if ms.Has(opSrc) {
		t.Error("normalizeMovesByStructure() kept opSrc in mapping, want evicted")
	}
}

func TestNormalizeMovesByStructure_TrivialJumpBlockDemoted(t *testing.T) {
	// A breakaway trivial jump block ({ return }) in another scope should not move on its own.
	src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\t{\n\t\treturn\n\t}\n}\n")
	dst := []byte("package main\n" + strings.Repeat("\n", 99) + "func g() {\n\t{\n\t\treturn\n\t}\n}\n")
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	block1, err := testutil.FindNodeByText(srcTree, src, "{\n\t\treturn\n\t}", "block")
	if err != nil {
		t.Fatalf("find block1: %v", err)
	}
	retStmt1, err := testutil.FindNodeByText(srcTree, src, "return", "return_statement")
	if err != nil {
		t.Fatalf("find retStmt1: %v", err)
	}
	block2, err := testutil.FindNodeByText(dstTree, dst, "{\n\t\treturn\n\t}", "block")
	if err != nil {
		t.Fatalf("find block2: %v", err)
	}
	retStmt2, err := testutil.FindNodeByText(dstTree, dst, "return", "return_statement")
	if err != nil {
		t.Fatalf("find retStmt2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(block1, block2)
	ms.Add(retStmt1, retStmt2)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: block1, DestNode: block2})

	// Boilerplate penalty knocks the score to 1 (below the threshold of 20), forcing delete+insert.
	result := normalizeMovesByStructure(es, ms)

	hasMove := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block1 && a.Type == actions.Move
	})
	hasDelete := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block1 && a.Type == actions.Delete
	})
	hasInsert := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block2 && a.Type == actions.Insert
	})

	if hasMove {
		t.Errorf("expected breakaway trivial jump block move to be demoted, but Move action survived")
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected trivial jump block to be demoted to Delete+Insert, got delete=%v, insert=%v", hasDelete, hasInsert)
	}
}

func TestNormalizeMovesByStructure_HollowLoopDemotedAcrossScopes(t *testing.T) {
	// Even if the loop header matches across functions, drop the move if the body was completely wiped out.
	src := []byte("package main\n" + strings.Repeat("\n", 159) + "func f() {\n\tfor idxVar := range actList {\n\t\tdeadLogic()\n\t}\n}\n")
	dst := []byte("package main\n" + strings.Repeat("\n", 475) + "func g() {\n\tfor idxVar := range actList {\n\t\tbrandNewLogic()\n\t}\n}\n")
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	forStmt1, err := testutil.FindNodeByText(srcTree, src, "for idxVar := range actList {\n\t\tdeadLogic()\n\t}", "for_statement")
	if err != nil {
		t.Fatalf("find forStmt1: %v", err)
	}
	rangeClause1, err := testutil.FindNodeByText(srcTree, src, "idxVar := range actList", "range_clause")
	if err != nil {
		t.Fatalf("find rangeClause1: %v", err)
	}
	srcA, err := testutil.FindNodeByText(srcTree, src, "idxVar", "identifier")
	if err != nil {
		t.Fatalf("find srcA: %v", err)
	}
	srcActions, err := testutil.FindNodeByText(srcTree, src, "actList", "identifier")
	if err != nil {
		t.Fatalf("find srcActions: %v", err)
	}

	forStmt2, err := testutil.FindNodeByText(dstTree, dst, "for idxVar := range actList {\n\t\tbrandNewLogic()\n\t}", "for_statement")
	if err != nil {
		t.Fatalf("find forStmt2: %v", err)
	}
	rangeClause2, err := testutil.FindNodeByText(dstTree, dst, "idxVar := range actList", "range_clause")
	if err != nil {
		t.Fatalf("find rangeClause2: %v", err)
	}
	dstA, err := testutil.FindNodeByText(dstTree, dst, "idxVar", "identifier")
	if err != nil {
		t.Fatalf("find dstA: %v", err)
	}
	dstActions, err := testutil.FindNodeByText(dstTree, dst, "actList", "identifier")
	if err != nil {
		t.Fatalf("find dstActions: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(forStmt1, forStmt2)
	ms.Add(rangeClause1, rangeClause2)
	ms.Add(srcA, dstA)
	ms.Add(srcActions, dstActions)
	// Only the loop header is mapped, body has 0% retention.

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: forStmt1, DestNode: forStmt2})

	result := normalizeMovesByStructure(es, ms)

	if slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == forStmt1 && a.Type == actions.Move
	}) {
		t.Errorf("expected hollow loop move with 0%% body retention to be demoted, but Move survived")
	}
}

func TestNormalizeMovesByStructure_LoopExtractionPreserved(t *testing.T) {
	// Moving a loop out to a helper function should stay a Move when its body logic survives.
	src := []byte(`package main
func f() {
	for idxVar := range listItems {
		stepOne()
		stepTwo()
		stepThree()
		stepFour()
		stepFive()
		stepSix()
	}
}
`)
	dst := []byte(`package main
func helper() {
	for idxVar := range listItems {
		stepOne()
		stepTwo()
		stepThree()
		stepFour()
		stepFive()
		stepSix()
	}
}
`)
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	forStmt1, err := testutil.FindNodeByText(srcTree, src, "for idxVar := range listItems {\n\t\tstepOne()\n\t\tstepTwo()\n\t\tstepThree()\n\t\tstepFour()\n\t\tstepFive()\n\t\tstepSix()\n\t}", "for_statement")
	if err != nil {
		t.Fatalf("find forStmt1: %v", err)
	}
	forStmt2, err := testutil.FindNodeByText(dstTree, dst, "for idxVar := range listItems {\n\t\tstepOne()\n\t\tstepTwo()\n\t\tstepThree()\n\t\tstepFour()\n\t\tstepFive()\n\t\tstepSix()\n\t}", "for_statement")
	if err != nil {
		t.Fatalf("find forStmt2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(forStmt1, forStmt2)
	srcNodes := forStmt1.Descendants()
	dstNodes := forStmt2.Descendants()
	for i := range min(len(srcNodes), len(dstNodes)) {
		ms.Add(srcNodes[i], dstNodes[i])
	}

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: forStmt1, DestNode: forStmt2})

	result := normalizeMovesByStructure(es, ms)

	if !slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == forStmt1 && a.Type == actions.Move
	}) {
		t.Errorf("expected extracted loop with 100%% surviving body to be preserved as Move")
	}
}

func TestNormalizeMovesByStructure_DemotesHollowExpressionMoveWhenDescendantsEvicted(t *testing.T) {
	src := []byte(`package test
func isDeclarationHeader(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	enc := GetEnclosingDeclaration(n)
	if enc == nil {
		return false
	}
	body := findBodyBlock(enc, r)
	if body != nil && (body == n || body.Contains(n)) {
		return false
	}
	return true
}
`)

	dst := []byte(`package test
func isDeclarationHeader(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r == nil {
		r = rulesFor(n)
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isContainer := (r != nil && r.IsContainerDeclaration(curr.Type)) ||
			(r == nil && rules.IsContainerDeclaration(curr.Type))
		if isContainer {
			body := findBodyBlock(curr, r)
			if body == nil {
				return false
			}
			return body != n && !body.Contains(n)
		}
	}
	return false
}
`)

	srcAST, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
	script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

	hasHollowMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
		return a.Type == actions.Move && a.Node.Type == "binary_expression"
	})
	if hasHollowMove {
		t.Errorf("expected binary_expression move to be demoted after descendants were evicted, but found an active Move")
	}
}

func TestNormalizeMoves_DemotesHollowCallAndArgumentList(t *testing.T) {
	// When sort.SliceStable(...) falsely matches cmp.Or(...) and only their empty
	// argument lists get mapped, drop both moves so we don't leave an empty "()" move behind.
	src := []byte(`package main
func f() {
	sort.SliceStable(scored, func() bool { return true })
}
`)
	dst := []byte(`package main
func g() bool {
	return cmp.Or(xVar, yVar)
}
`)
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcRoot, err := testutil.FindNodeByText(srcTree, src, "{\n\tsort.SliceStable(scored, func() bool { return true })\n}", "block")
	if err != nil {
		t.Fatalf("find srcRoot: %v", err)
	}
	dstRoot, err := testutil.FindNodeByText(dstTree, dst, "{\n\treturn cmp.Or(xVar, yVar)\n}", "block")
	if err != nil {
		t.Fatalf("find dstRoot: %v", err)
	}
	srcCall, err := testutil.FindNodeByText(srcTree, src, "sort.SliceStable(scored, func() bool { return true })", "call_expression")
	if err != nil {
		t.Fatalf("find srcCall: %v", err)
	}
	dstCall, err := testutil.FindNodeByText(dstTree, dst, "cmp.Or(xVar, yVar)", "call_expression")
	if err != nil {
		t.Fatalf("find dstCall: %v", err)
	}
	srcArgList, err := testutil.FindNodeByText(srcTree, src, "(scored, func() bool { return true })", "argument_list")
	if err != nil {
		t.Fatalf("find srcArgList: %v", err)
	}
	dstArgList, err := testutil.FindNodeByText(dstTree, dst, "(xVar, yVar)", "argument_list")
	if err != nil {
		t.Fatalf("find dstArgList: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcRoot, dstRoot)
	ms.Add(srcCall, dstCall)
	ms.Add(srcArgList, dstArgList)

	script := actions.GenerateEditScript(srcRoot, dstRoot, ms)
	normalized := Run(script, ms, srcRoot, dstRoot)

	for _, a := range normalized.Actions() {
		if a.Type == actions.Move && (a.Node == srcCall || a.Node == srcArgList) {
			t.Errorf("expected hollow %s move to be demoted, but Move survived", a.Node.Type)
		}
	}
	if ms.Has(srcCall) || ms.Has(srcArgList) {
		t.Errorf("expected hollow call_expression and argument_list to be evicted from mapping")
	}
}

func TestNormalizeMoves_ClosureCallAndArgumentList_FullEngine(t *testing.T) {
	// Full pipeline: migrating from sortSliceStable to sortStableFunc returning cmpOr.
	// Postprocess must not emit a Move action on the outer call or argument list.
	src := `package main

func sortItems(scored any) {
	sortSliceStable(scored, func(i, j int) bool {
		if a != b {
			return a > b
		}
		return dice > ancSim
	})
}
`
	dst := `package main

func sortItems(scored any) {
	sortStableFunc(scored, func(a, b any) int {
		return cmpOr(
			compare(dice),
			compare(ancSim),
		)
	})
}
`
	srcBytes := []byte(src)
	dstBytes := []byte(dst)

	srcAST, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	part := engine.NewLinePartition(srcBytes, dstBytes)
	mr := engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
	normalized := Run(script, mr.Mappings, srcAST, dstAST)

	for _, a := range normalized.Actions() {
		// 1. Zero move actions anywhere in the snippet
		if a.Type == actions.Move {
			t.Errorf("expected zero move actions across entire snippet, but found move on %s: %v", a.Node.Type, a)
		}
		// 2. Package and function declaration headers must remain untouched context
		if a.Node != nil && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
			t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
		}
	}
}

func TestDecomposedExpressionMoveDemotion(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package test
func f(rawType string, fn Node) {
	startTimer()
	if rawType == "ERROR" || (fn.Flags&FlatNodeError != 0) || (fn.Flags&FlatNodeMissing != 0) {
		errCount++
	}
	logSummary()
}
`)
		dst := []byte(`package test
func f(rawType string, fn Node) {
	startTimer()
	if rawType == "ERROR" || (fn.Flags&FlatNodeError != 0) {
		errorCount++
	} else if (fn.Flags&FlatNodeMissing != 0) || strings.HasPrefix(rawType, "MISSING") {
		missingCount++
	}
	logSummary()
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// Clauses that stayed on the outer if statement should stay as context.
		// Only the clause extracted into the else-if should be a Move.
		var moveActions []actions.Action
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				moveActions = append(moveActions, a)
			}
		}

		if len(moveActions) != 1 {
			t.Fatalf("expected exactly 1 move action for the extracted clause, got %d: %+v", len(moveActions), moveActions)
		}

		movedNode := moveActions[0].Node
		movedText := string(src[movedNode.StartByte:movedNode.EndByte])
		if movedText != "(fn.Flags&FlatNodeMissing != 0)" {
			t.Errorf("expected moved node text to be '(fn.Flags&FlatNodeMissing != 0)', got %q", movedText)
		}

		// The moved clause shouldn't have any deleted children.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Delete && movedNode.Contains(a.Node) {
				t.Errorf("unexpected Delete action on descendant of moved node: %s %s", a.Node.Type, string(src[a.Node.StartByte:a.Node.EndByte]))
			}
		}
	})
}

func TestDecomposedExpressionMovePromotion_NestedClauseRecursed(t *testing.T) {
	src := []byte(`package test
func f(idx int) {
	initCount()
	if idx > 0 && idx+1 < len(t1.Children) && idx+1 < len(t2.Children) {
		foo()
	}
	finish()
}
`)
	dst := []byte(`package test
func f(idx int) {
	initCount()
	if idx > 0 && idx < len(t2.Children) {
		foo()
	}
	if idx+1 < len(t1.Children) && idx+1 < len(t2.Children) {
		bar()
	}
	finish()
}
`)

	srcAST, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
	script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

	// Both idx+1 < len(t1.Children) and idx+1 < len(t2.Children) should survive as moves.
	moves := make(map[string]bool)
	for _, a := range normalized.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			txt := string(src[a.Node.StartByte:a.Node.EndByte])
			moves[txt] = true
		}
	}

	if !moves["idx+1 < len(t1.Children)"] {
		t.Errorf("expected idx+1 < len(t1.Children) to be preserved as a move, moves: %+v", moves)
	}
	if !moves["idx+1 < len(t2.Children)"] {
		t.Errorf("expected idx+1 < len(t2.Children) to be preserved as a move, moves: %+v", moves)
	}
}

func TestNormalize_StationaryMoveProtection(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`function validate(annotation: any, node: any): boolean {
    const isReady = checkReady();
    if (annotation && getTypeFromTypeNode(node)) {
        return true;
    }
    return false;
}
`)
		dst := []byte(`function validate(annotation: any, node: any): boolean {
    const isReady = checkReady();
    if (annotation) {
        return true;
    }
    return false;
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing function declaration mapped
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"function validate", "function validate", "function_declaration",
		)

		// 2. Surrounding statements mapped in place
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"const isReady = checkReady();", "const isReady = checkReady();", "lexical_declaration",
		)
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"return false;", "return false;", "return_statement",
		)
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"if (annotation", "if (annotation", "if_statement",
		)

		// 3. Target stationary operand mapped
		testutil.AssertMappedTextQuery(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			testutil.TextQuery{Text: "annotation", Occurrence: 2},
			testutil.TextQuery{Text: "annotation", Occurrence: 2},
		)

		srcAnnotation, err := testutil.FindNodeByTextQuery(srcAST.Root(), src, testutil.TextQuery{Text: "annotation", Occurrence: 2})
		if err != nil {
			t.Fatalf("failed to locate annotation identifier: %v", err)
		}

		// 4. Check that postprocessing keeps annotation as context instead of turning it into a delete/insert
		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Node == srcAnnotation {
				t.Errorf("expected annotation to be preserved as context without action, got %v", a.Type)
			}
		}
		if !ms.Has(srcAnnotation) {
			t.Errorf("expected srcAnnotation to remain in mapping after postprocessing")
		}
	})
}

func TestNormalize_BinaryChainOperandStationaryProtection(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`function processTypes(type: any, host: any, context: any, typeNode: any, addUndefined: any): any {
    const originalType = type;
    if (clone) {
        if (addUndefined && !someType(getTypeFromTypeNode(context, typeNode), t => !!(t.flags & TypeFlags.Undefined))) {
            return factory.createUnionTypeNode([clone, factory.createKeywordTypeNode(SyntaxKind.UndefinedKeyword)]);
        }
        return clone;
    }
    return originalType;
}
`)
		dst := []byte(`function processTypes(type: any, host: any, context: any, typeNode: any, addUndefined: any): any {
    const originalType = type;
    if (clone) {
        // explicitly add | undefined if it is missing from input type nodes
        if (addUndefined && containsNonMissingUndefinedType(type) && !someType(getTypeFromTypeNode(context, typeNode), t => !!(t.flags & TypeFlags.Undefined))) {
            return factory.createUnionTypeNode([clone, factory.createKeywordTypeNode(SyntaxKind.UndefinedKeyword)]);
        }
        return clone;
    }
    return originalType;
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing Context: Function declaration maps in place
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"function processTypes", "function processTypes", "function_declaration",
		)

		// 2. Surrounding Sequence: Preceding and succeeding statements map in place
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"const originalType = type;", "const originalType = type;", "lexical_declaration",
		)
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"return originalType;", "return originalType;", "return_statement",
		)
		testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			"if (clone)", "if (clone)", "if_statement",
		)

		// 3. Target stationary operand mapped
		testutil.AssertMappedTextQuery(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
			testutil.TextQuery{Text: "addUndefined", Occurrence: 2},
			testutil.TextQuery{Text: "addUndefined", Occurrence: 2},
		)

		srcAddUndefined, err := testutil.FindNodeByTextQuery(srcAST.Root(), src, testutil.TextQuery{Text: "addUndefined", Occurrence: 2})
		if err != nil {
			t.Fatalf("failed to locate addUndefined identifier in src: %v", err)
		}
		dstAddUndefined, err := testutil.FindNodeByTextQuery(dstAST.Root(), dst, testutil.TextQuery{Text: "addUndefined", Occurrence: 2})
		if err != nil {
			t.Fatalf("failed to locate addUndefined identifier in dst: %v", err)
		}

		// 4. Check that postprocessing keeps addUndefined as context without delete/insert/move
		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Node == srcAddUndefined || a.Node == dstAddUndefined {
				t.Errorf("expected addUndefined to be preserved as context without action, got %v (%s)", a.Type, a.Node.Type)
			}
		}
		if !ms.Has(srcAddUndefined) {
			t.Errorf("expected srcAddUndefined to remain in mapping after postprocessing")
		}
	})
}

func TestAttributeMoveDemotion(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("rust")
		src := []byte(`#[allow(clippy::comparison_chain)]
fn foo() {}
`)
		dst := []byte(`#[derive(Debug)]
struct Bar;
`)
		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		var srcAttr, dstAttr *treesitter.ASTNode
		for _, n := range srcAST.Root().PreOrder() {
			if r.IsAttribute(n.Type) {
				srcAttr = n
				break
			}
		}
		for _, n := range dstAST.Root().PreOrder() {
			if r.IsAttribute(n.Type) {
				dstAttr = n
				break
			}
		}
		if srcAttr == nil || dstAttr == nil {
			t.Fatalf("failed to locate attributes in ASTs")
		}

		ms := engine.NewMapping()
		ms.Add(srcAST.Root(), dstAST.Root())

		if !shouldDemoteMove(srcAttr, dstAttr, ms, r) {
			t.Errorf("expected cross-scope attribute move to be demoted by shouldDemoteMove")
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust_AttributeDoesNotMoveAcrossDeclarations", func(t *testing.T) {
			src := []byte(`pub struct Panel {
    pub width: usize,
}

#[allow(clippy::comparison_chain)]
fn helper_old(x: usize) -> usize {
    x + 1
}
`)
			dst := []byte(`#[derive(Debug)]
pub struct Panel {
    pub width: usize,
}

fn helper_new(y: usize) -> usize {
    y + 1
}
`)

			srcAST, err := treesitter.Parse(src, "test.rs")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "test.rs")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}
			if srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
			}
			if dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			ms := matchResult.Mappings

			// 1. Surrounding context: Panel struct maps in place
			testutil.AssertMappedText(t, srcAST.Root(), dstAST.Root(), src, dst, ms.Src(),
				"pub struct Panel", "pub struct Panel", "struct_item",
			)

			srcAttr, err := testutil.FindNodeByText(srcAST.Root(), src, "#[allow(clippy::comparison_chain)]", "attribute_item")
			if err != nil {
				t.Fatalf("failed to locate src attribute: %v", err)
			}
			dstAttr, err := testutil.FindNodeByText(dstAST.Root(), dst, "#[derive(Debug)]", "attribute_item")
			if err != nil {
				t.Fatalf("failed to locate dst attribute: %v", err)
			}

			// 2. The attribute on helper_old must not move over to Panel
			script := actions.GenerateEditScript(srcAST, dstAST, ms)
			normalized := Run(script, ms, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && (a.Node == srcAttr || a.Node == dstAttr) {
					t.Errorf("expected attribute NOT to be a move across declarations, got Move on %s", a.Node.Type)
				}
			}
		})
	})
}

func TestNormalizeMoves_RustLifetimeDoesNotMoveAcrossParameters(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`fn format_and_paint_line_number_field<'a>(
    format_data: &[LineNumberPlaceholderData<'a>],
    style: &Style,
    min_field_width: usize,
) -> usize {
    let mut ansi_strings = Vec::new();
    let mut suffix = "";
    ansi_strings.push(suffix);
    min_field_width
}
`)
		dst := []byte(`fn format_and_paint_line_number_field<'a>(
    line_numbers_data: &'a LineNumbersData,
    style: &Style,
    min_field_width: usize,
) -> usize {
    let mut ansi_strings = Vec::new();
    let mut suffix = "";
    ansi_strings.push(suffix);
    min_field_width
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing Context: Function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"fn format_and_paint_line_number_field", "fn format_and_paint_line_number_field", "function_item",
		)

		// 2. Surrounding Sequence: style and min_field_width parameters map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"style: &Style", "style: &Style", "parameter",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"min_field_width: usize", "min_field_width: usize", "parameter",
		)

		// 3. Body statements map in place without order disruption
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"let mut ansi_strings = Vec::new();", "let mut ansi_strings = Vec::new();", "let_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			`let mut suffix = "";`, `let mut suffix = "";`, "let_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"ansi_strings.push(suffix);", "ansi_strings.push(suffix);", "expression_statement",
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, src, dst, ms.Src(),
			testutil.TextQuery{Text: "min_field_width", Occurrence: 2},
			testutil.TextQuery{Text: "min_field_width", Occurrence: 2},
		)

		// 4. Target: srcParam1Lifetime must NOT be emitted as a Move across rewritten parameters
		srcParam1Lifetime, err := testutil.FindNodeByTextQuery(srcAST, src, testutil.TextQuery{Text: "'a", Occurrence: 2})
		if err != nil {
			t.Fatalf("failed to locate src lifetime: %v", err)
		}

		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcParam1Lifetime {
				t.Fatalf("expected lifetime 'a NOT to be a spurious move across rewritten parameters, got Move on %s [%d-%d]",
					a.Node.Type, a.Node.StartByte, a.Node.EndByte)
			}
		}
	})
}

func TestNormalizeMoves_TypeScriptTypeDoesNotMoveAcrossParameters(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`function formatAndPaint(
    formatData: Array<string>,
    style: Style,
    minFieldWidth: number,
): number {
    let ansiStrings: string[] = [];
    let suffix = "";
    ansiStrings.push(suffix);
    return minFieldWidth;
}
`)
		dst := []byte(`function formatAndPaint(
    lineNumbersData: string,
    style: Style,
    minFieldWidth: number,
): number {
    let ansiStrings: string[] = [];
    let suffix = "";
    ansiStrings.push(suffix);
    return minFieldWidth;
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing Context: Function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"function formatAndPaint", "function formatAndPaint", "function_declaration",
		)

		// 2. Surrounding Sequence: style and minFieldWidth parameters map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"style: Style", "style: Style", "required_parameter",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"minFieldWidth: number", "minFieldWidth: number", "required_parameter",
		)

		// 3. Body statements map in place without order disruption
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"let ansiStrings: string[] = [];", "let ansiStrings: string[] = [];", "lexical_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			`let suffix = "";`, `let suffix = "";`, "lexical_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"ansiStrings.push(suffix);", "ansiStrings.push(suffix);", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"return minFieldWidth;", "return minFieldWidth;", "return_statement",
		)

		// 4. Target: string type in parameter 1 must NOT be emitted as a Move across rewritten parameters
		srcParam1String, err := testutil.FindNodeByTextQuery(srcAST, src, testutil.TextQuery{Text: "string", Occurrence: 1})
		if err != nil {
			t.Fatalf("failed to locate string type in parameter 1: %v", err)
		}

		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcParam1String {
				t.Fatalf("expected type 'string' NOT to be a spurious move across rewritten parameters, got Move on %s [%d-%d]",
					a.Node.Type, a.Node.StartByte, a.Node.EndByte)
			}
		}
	})
}

func TestNormalizeMoves_JavaTypeDoesNotMoveAcrossParameters(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`class StatusHandler {
    void formatAndPaint(
        List<String> formatData,
        Style style,
        int minFieldWidth
    ) {
        String msg = "status";
        doInitial();
        finalize(msg);
    }
}
`)
		dst := []byte(`class StatusHandler {
    void formatAndPaint(
        String lineNumbersData,
        Style style,
        int minFieldWidth
    ) {
        String msg = "status";
        doInitial();
        finalize(msg);
    }
}
`)

		srcAST, err := treesitter.Parse(src, "test.java")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.java")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing Context: method declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"void formatAndPaint", "void formatAndPaint", "method_declaration",
		)

		// 2. Surrounding Sequence: style and minFieldWidth parameters map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"Style style", "Style style", "formal_parameter",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"int minFieldWidth", "int minFieldWidth", "formal_parameter",
		)

		// 3. Body statements map in place without order disruption
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			`String msg = "status";`, `String msg = "status";`, "local_variable_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"doInitial();", "doInitial();", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"finalize(msg);", "finalize(msg);", "expression_statement",
		)

		// 4. Target: String type in parameter 1 must NOT be emitted as a Move across rewritten parameters
		srcParam1String, err := testutil.FindNodeByTextQuery(srcAST, src, testutil.TextQuery{Text: "String", Occurrence: 1})
		if err != nil {
			t.Fatalf("failed to locate String type in parameter 1: %v", err)
		}

		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcParam1String {
				t.Fatalf("expected type 'String' NOT to be a spurious move across rewritten parameters, got Move on %s [%d-%d]",
					a.Node.Type, a.Node.StartByte, a.Node.EndByte)
			}
		}
	})
}

func TestNormalizeMoves_ZigTypeDoesNotMoveAcrossParameters(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`pub fn formatAndPaint(
    format_data: []const u8,
    style: Style,
    min_field_width: usize,
) usize {
    const x = 1;
    doInitial();
    return min_field_width;
}
`)
		dst := []byte(`pub fn formatAndPaint(
    line_numbers_data: u8,
    style: Style,
    min_field_width: usize,
) usize {
    const x = 1;
    doInitial();
    return min_field_width;
}
`)

		srcAST, err := treesitter.Parse(src, "test.zig")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.zig")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		ms := matchResult.Mappings

		// 1. Enclosing Context: function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"pub fn formatAndPaint", "pub fn formatAndPaint", "function_declaration",
		)

		// 2. Surrounding Sequence: style and min_field_width parameters map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"style: Style", "style: Style", "parameter",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"min_field_width: usize", "min_field_width: usize", "parameter",
		)

		// 3. Body statements map in place without order disruption
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"const x = 1;", "const x = 1;", "variable_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"doInitial();", "doInitial();", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, ms.Src(),
			"return min_field_width;", "return min_field_width;",
		)

		// 4. Target: u8 primitive type in parameter 1 must NOT be emitted as a Move across rewritten parameters
		srcParam1U8, err := testutil.FindNodeByText(srcAST, src, "u8", "builtin_type")
		if err != nil {
			t.Fatalf("failed to locate u8 type in parameter 1: %v", err)
		}

		script := actions.GenerateEditScript(srcAST, dstAST, ms)
		normalized := Run(script, ms, srcAST, dstAST)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcParam1U8 {
				t.Fatalf("expected type 'u8' NOT to be a spurious move across rewritten parameters, got Move on %s [%d-%d]",
					a.Node.Type, a.Node.StartByte, a.Node.EndByte)
			}
		}
	})
}
