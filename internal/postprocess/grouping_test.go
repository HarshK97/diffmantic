package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestGroupMoves(t *testing.T) {
	srcBytes := []byte("package main\nfunc f() {\n\tstmtA()\n\tstmtB()\n\tstmtC()\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n\tstmtA()\n\tstmtB()\n\tstmtC()\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	c1, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtA()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmtA: %v", err)
	}
	c2, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtB()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmtB: %v", err)
	}
	c3, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtC()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmtC: %v", err)
	}
	newParent, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\tstmtA()\n\tstmtB()\n\tstmtC()\n}", "block")
	if err != nil {
		t.Fatalf("find newParent: %v", err)
	}

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
	srcBytes := []byte("def old_fn(arg_one, arg_two):\n    pass\n")
	dstBytes := []byte("def new_fn(arg_one, arg_two):\n    pass\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.py")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.py")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	n1, err := testutil.FindNodeByText(srcTree, srcBytes, "arg_one", "identifier")
	if err != nil {
		t.Fatalf("find arg_one: %v", err)
	}
	n2, err := testutil.FindNodeByText(srcTree, srcBytes, "arg_two", "identifier")
	if err != nil {
		t.Fatalf("find arg_two: %v", err)
	}
	newParent, err := testutil.FindNodeByText(dstTree, dstBytes, "def new_fn(arg_one, arg_two):\n    pass", "function_definition")
	if err != nil {
		t.Fatalf("find new_fn: %v", err)
	}

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
	srcBytes := []byte("package main\nfunc f() {\n\tstmtA()\n\tstmtB()\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n\tstmtB()\n\tstmtA()\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcA, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtA()", "expression_statement")
	if err != nil {
		t.Fatalf("find srcA: %v", err)
	}
	srcB, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtB()", "expression_statement")
	if err != nil {
		t.Fatalf("find srcB: %v", err)
	}
	dstA, err := testutil.FindNodeByText(dstTree, dstBytes, "stmtA()", "expression_statement")
	if err != nil {
		t.Fatalf("find dstA: %v", err)
	}
	dstB, err := testutil.FindNodeByText(dstTree, dstBytes, "stmtB()", "expression_statement")
	if err != nil {
		t.Fatalf("find dstB: %v", err)
	}
	newParent, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\tstmtB()\n\tstmtA()\n}", "block")
	if err != nil {
		t.Fatalf("find newParent: %v", err)
	}

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
	srcBytes := []byte("package main\nfunc f() {\n\t{\n\t\tstmtA()\n\t\tstmtB()\n\t}\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n\tstmtA()\n\tstmtB()\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	outer, err := testutil.FindNodeByText(srcTree, srcBytes, "{\n\t\tstmtA()\n\t\tstmtB()\n\t}", "block")
	if err != nil {
		t.Fatalf("find outer: %v", err)
	}
	inner1, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtA()", "expression_statement")
	if err != nil {
		t.Fatalf("find inner1: %v", err)
	}
	inner2, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtB()", "expression_statement")
	if err != nil {
		t.Fatalf("find inner2: %v", err)
	}
	newParent, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\tstmtA()\n\tstmtB()\n}", "block")
	if err != nil {
		t.Fatalf("find newParent: %v", err)
	}

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
	srcBytes := []byte("package main\nfunc f() {\n\tstmtA()\n\tstmtB()\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n\tstmtA()\n\tstmtB()\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	n1, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtA()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmtA: %v", err)
	}
	n2, err := testutil.FindNodeByText(srcTree, srcBytes, "stmtB()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmtB: %v", err)
	}
	newParent, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\tstmtA()\n\tstmtB()\n}", "block")
	if err != nil {
		t.Fatalf("find newParent: %v", err)
	}

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

func TestGroupMovesPropagateToBlock(t *testing.T) {
	srcBytes := []byte("package main\nfunc f() {\n\tif true {\n\t\tfoo()\n\t}\n\tbar()\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	stmt, err := testutil.FindNodeByText(srcTree, srcBytes, "if true {\n\t\tfoo()\n\t}", "if_statement")
	if err != nil {
		t.Fatalf("find if: %v", err)
	}
	stmt2, err := testutil.FindNodeByText(srcTree, srcBytes, "bar()", "expression_statement")
	if err != nil {
		t.Fatalf("find bar: %v", err)
	}
	stmtBlock, err := testutil.FindNodeByText(srcTree, srcBytes, "{\n\t\tfoo()\n\t}", "block")
	if err != nil {
		t.Fatalf("find if block: %v", err)
	}
	newFuncBody, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n}", "block")
	if err != nil {
		t.Fatalf("find new block: %v", err)
	}

	es := actions.NewEditScript()
	// stmt and stmt2 share funcBody as parent -> get grouped into group-1
	es.Add(actions.Action{Type: actions.Move, Node: stmt, Parent: newFuncBody})
	es.Add(actions.Action{Type: actions.Move, Node: stmt2, Parent: newFuncBody})
	// stmtBlock has stmt as parent
	es.Add(actions.Action{Type: actions.Move, Node: stmtBlock, Parent: stmt})

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()
	if len(acts) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(acts))
	}
	if acts[0].GroupID != "group-1" {
		t.Errorf("expected stmt to have GroupID group-1, got %q", acts[0].GroupID)
	}
	if acts[2].GroupID != "group-1" {
		t.Errorf("expected child stmtBlock to inherit GroupID group-1, got %q", acts[2].GroupID)
	}
}

func TestGroupMoves_LIS_PreservesMonotonicCohortWhenOneIsReordered(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		src := []byte(`package main

func foo() {
	println("prologue")
}

func bar() {
	stmtA()
	stmtB()
	stmtC()
	stmtD()
}
`)
		dst := []byte(`package main

func foo() {
	println("prologue")
	stmtD()
	stmtA()
	stmtB()
	stmtC()
}

func bar() {
}
`)

		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// Whole-Snippet Verification:
		// 1. Enclosing context declarations (foo, bar) map in place with no edit actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"func foo()", "func foo()", "function_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"func bar()", "func bar()", "function_declaration",
		)
		for _, a := range normalized.Actions() {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Stationary statements and literals map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			`println("prologue")`, `println("prologue")`, "call_expression",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			`"prologue"`, `"prologue"`, "interpreted_string_literal",
		)

		// Statements that moved in order stay in group-1; only hoisted stmtD is ungrouped.
		findMoveGroupID := func(node *treesitter.ASTNode) (string, bool) {
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && (a.Node.ID == node.ID || a.Node.Contains(node)) {
					return a.GroupID, true
				}
			}
			return "", false
		}

		for _, stmt := range []string{"stmtA()", "stmtB()", "stmtC()"} {
			node, err := testutil.FindNodeByText(srcAST, src, stmt)
			if err != nil {
				t.Fatalf("find node for %s: %v", stmt, err)
			}
			if gid, ok := findMoveGroupID(node); !ok || gid != "group-1" {
				t.Errorf("expected in-order statement %s to have GroupID group-1, got %q", stmt, gid)
			}
		}

		nodeD, err := testutil.FindNodeByText(srcAST, src, "stmtD()")
		if err != nil {
			t.Fatalf("find node for stmtD(): %v", err)
		}
		if gid, ok := findMoveGroupID(nodeD); !ok || gid != "" {
			t.Errorf("expected out-of-order statement stmtD() to have empty GroupID, got %q", gid)
		}
	})
}

func TestGroupMoves_LIS_SunkStatementUngrouped(t *testing.T) {
	// Statement 1 was moved to the end:
	// src: stmt1(), stmt2(), stmt3()
	// dst: stmt2(), stmt3(), stmt1()
	srcBytes := []byte("package main\nfunc f() {\n\tstmt1()\n\tstmt2()\n\tstmt3()\n}\n")
	dstBytes := []byte("package main\nfunc g() {\n\tstmt2()\n\tstmt3()\n\tstmt1()\n}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	src1, err := testutil.FindNodeByText(srcTree, srcBytes, "stmt1()", "expression_statement")
	if err != nil {
		t.Fatalf("find src1: %v", err)
	}
	src2, err := testutil.FindNodeByText(srcTree, srcBytes, "stmt2()", "expression_statement")
	if err != nil {
		t.Fatalf("find src2: %v", err)
	}
	src3, err := testutil.FindNodeByText(srcTree, srcBytes, "stmt3()", "expression_statement")
	if err != nil {
		t.Fatalf("find src3: %v", err)
	}
	dst1, err := testutil.FindNodeByText(dstTree, dstBytes, "stmt1()", "expression_statement")
	if err != nil {
		t.Fatalf("find dst1: %v", err)
	}
	dst2, err := testutil.FindNodeByText(dstTree, dstBytes, "stmt2()", "expression_statement")
	if err != nil {
		t.Fatalf("find dst2: %v", err)
	}
	dst3, err := testutil.FindNodeByText(dstTree, dstBytes, "stmt3()", "expression_statement")
	if err != nil {
		t.Fatalf("find dst3: %v", err)
	}
	dstBlock, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\tstmt2()\n\tstmt3()\n\tstmt1()\n}", "block")
	if err != nil {
		t.Fatalf("find dstBlock: %v", err)
	}

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: src1, DestNode: dst1, Parent: dstBlock})
	es.Add(actions.Action{Type: actions.Move, Node: src2, DestNode: dst2, Parent: dstBlock})
	es.Add(actions.Action{Type: actions.Move, Node: src3, DestNode: dst3, Parent: dstBlock})

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()

	// 2 and 3 stay in relative order, so they form the group.
	if acts[1].GroupID != "group-1" || acts[2].GroupID != "group-1" {
		t.Errorf("expected cohort 2, 3 to have group-1, got %q, %q", acts[1].GroupID, acts[2].GroupID)
	}
	// 1 moved out of order to the end, so it stays ungrouped.
	if acts[0].GroupID != "" {
		t.Errorf("expected sunk statement 1 to have empty GroupID, got %q", acts[0].GroupID)
	}
}

func TestGroupingParent_DelimiterlessBlocks(t *testing.T) {
	code := []byte("def f():\n    x = 1\n    y = 2\n")
	tree, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("parse python: %v", err)
	}

	stmt1, err := testutil.FindNodeByText(tree, code, "x = 1", "expression_statement")
	if err != nil {
		t.Fatalf("find stmt1: %v", err)
	}
	stmt2, err := testutil.FindNodeByText(tree, code, "y = 2", "expression_statement")
	if err != nil {
		t.Fatalf("find stmt2: %v", err)
	}
	block, err := testutil.FindNodeByText(tree, code, "x = 1\n    y = 2", "block")
	if err != nil {
		t.Fatalf("find block: %v", err)
	}

	p1 := groupingParent(stmt1)
	p2 := groupingParent(stmt2)

	if p1 != block {
		t.Errorf("expected stmt1 groupingParent to be block, got %v", p1)
	}
	if p2 != block {
		t.Errorf("expected stmt2 groupingParent to be block, got %v", p2)
	}
}

func TestGrouping_SingleLineCompoundStatements(t *testing.T) {
	src := []byte("if cond: a = 1; b = 2\n")
	dst := []byte("def f():\n    a = 1\n    b = 2\n")
	srcTree, err := treesitter.Parse(src, "src.py")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.py")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcStmt1, err := testutil.FindNodeByText(srcTree, src, "a = 1", "expression_statement")
	if err != nil {
		t.Fatalf("find srcStmt1: %v", err)
	}
	srcStmt2, err := testutil.FindNodeByText(srcTree, src, "b = 2", "expression_statement")
	if err != nil {
		t.Fatalf("find srcStmt2: %v", err)
	}
	dstStmt1, err := testutil.FindNodeByText(dstTree, dst, "a = 1", "expression_statement")
	if err != nil {
		t.Fatalf("find dstStmt1: %v", err)
	}
	dstStmt2, err := testutil.FindNodeByText(dstTree, dst, "b = 2", "expression_statement")
	if err != nil {
		t.Fatalf("find dstStmt2: %v", err)
	}
	dstBlock, err := testutil.FindNodeByText(dstTree, dst, "a = 1\n    b = 2", "block")
	if err != nil {
		t.Fatalf("find dstBlock: %v", err)
	}

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: srcStmt1, DestNode: dstStmt1, Parent: dstBlock})
	es.Add(actions.Action{Type: actions.Move, Node: srcStmt2, DestNode: dstStmt2, Parent: dstBlock})

	grouped := GroupMoves(es, nil)
	acts := grouped.Actions()
	if len(acts) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(acts))
	}
	if acts[0].GroupID != "group-1" || acts[1].GroupID != "group-1" {
		t.Errorf("expected both single-line compound statements to unify under group-1, got %q and %q", acts[0].GroupID, acts[1].GroupID)
	}
}

func TestPython_MoveGrouping_FullEngine(t *testing.T) {
	r := rules.Get("python")
	if r == nil {
		t.Fatal("rules for python not found")
	}

	src := []byte(`def source_fn():
    print("prologue")
    moved_one()
    moved_two()
    print("epilogue")

def dest_fn():
    pass
`)
	dst := []byte(`def source_fn():
    print("prologue")
    print("epilogue")

def dest_fn():
    moved_one()
    moved_two()
`)

	srcAST, err := treesitter.Parse(src, "module.py")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "module.py")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition(src, dst)
	matchResult := engine.Match(srcAST, dstAST, src, dst, part)
	if matchResult == nil || matchResult.Mappings == nil {
		t.Fatal("Match returned nil mappings")
	}

	script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

	// Whole-Snippet Verification:
	// 1. Enclosing context declarations (source_fn, dest_fn) map in place with no edit actions.
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		"def source_fn():", "def source_fn():", "function_definition",
	)
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		"def dest_fn():", "def dest_fn():", "function_definition",
	)
	for _, a := range normalized.Actions() {
		if a.Node != nil && r.IsContainerDeclaration(a.Node.Type) {
			t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
		}
	}

	// 2. Preceding stationary statement: print("prologue") maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		`print("prologue")`, `print("prologue")`, "call",
	)

	// 3. Succeeding stationary statement: print("epilogue") maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		`print("epilogue")`, `print("epilogue")`, "call",
	)

	// 4. Stationary argument literals ("prologue", "epilogue") map in place.
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		`"prologue"`, `"prologue"`, "string",
	)
	testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
		`"epilogue"`, `"epilogue"`, "string",
	)

	// 5. Target statements moved_one() and moved_two() move together and receive identical GroupID "group-1".
	findMoveGroupID := func(node *treesitter.ASTNode) (string, bool) {
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && (a.Node.ID == node.ID || a.Node.Contains(node)) {
				return a.GroupID, true
			}
		}
		return "", false
	}

	for _, stmt := range []string{"moved_one()", "moved_two()"} {
		node, err := testutil.FindNodeByText(srcAST, src, stmt)
		if err != nil {
			t.Fatalf("find node for %s: %v", stmt, err)
		}
		if gid, ok := findMoveGroupID(node); !ok || gid != "group-1" {
			t.Errorf("expected co-moving Python statement %s to have GroupID group-1, got %q", stmt, gid)
		}
	}
}

func TestGroupMoves_LIS_TiedLengthBreaksByCodeSpan(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		src := []byte(`package main

func process() error {
	prepare()
	count++
	if count > limit {
		return flush()
	}
	return nil
}
`)
		dst := []byte(`package main

func process() error {
	prepare()
	if checkCondition() {
		if count > limit {
			return flush()
		}
		count++
		return nil
	}
	return nil
}
`)

		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// Whole-Snippet Verification:
		// 1. Enclosing context declaration maps in place with no edit actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"func process() error", "func process() error", "function_declaration",
		)
		for _, a := range normalized.Actions() {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Preceding stationary statement: prepare() maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			`prepare()`, `prepare()`, "call_expression",
		)

		findMoveGroupID := func(node *treesitter.ASTNode) (string, bool) {
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && (a.Node.ID == node.ID || a.Node.Contains(node)) {
					return a.GroupID, true
				}
			}
			return "", false
		}

		ifNode, err := testutil.FindNodeByText(srcAST, src, "if count > limit {\n\t\treturn flush()\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find if statement: %v", err)
		}
		if gid, ok := findMoveGroupID(ifNode); !ok || gid == "" {
			t.Errorf("expected dominant if statement to have non-empty GroupID, got %q", gid)
		}

		countNode, err := testutil.FindNodeByText(srcAST, src, "count++", "inc_statement")
		if err != nil {
			t.Fatalf("find count++: %v", err)
		}
		if gid, ok := findMoveGroupID(countNode); !ok || gid != "" {
			t.Errorf("expected reordered smaller statement count++ to have empty GroupID, got %q", gid)
		}
	})
}
