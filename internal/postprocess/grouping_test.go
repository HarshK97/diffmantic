package postprocess

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

func TestGroupMovesPropagateToBlock(t *testing.T) {
	funcBody := &treesitter.ASTNode{Type: "block", Language: "go"}
	newFuncBody := &treesitter.ASTNode{Type: "block", Language: "go"}

	stmt := &treesitter.ASTNode{Type: "if_statement", Language: "go"}
	setParentAndRange(stmt, funcBody, 10, 50)

	stmt2 := &treesitter.ASTNode{Type: "for_statement", Language: "go"}
	setParentAndRange(stmt2, funcBody, 51, 90)

	stmtBlock := &treesitter.ASTNode{Type: "block", Language: "go"}
	setParentAndRange(stmtBlock, stmt, 20, 50)

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

		// Outer package and function declarations shouldn't have any edit actions.
		for _, a := range normalized.Actions() {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// println("prologue") didn't move, so it maps in place.
		var srcPrologue, dstPrologue *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "println") {
				srcPrologue = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "println") {
				dstPrologue = n
				break
			}
		}
		if srcPrologue == nil || dstPrologue == nil {
			t.Fatal("failed to find println() call nodes")
		}
		if matchResult.Mappings.Src()[srcPrologue] != dstPrologue {
			t.Errorf("expected stationary println() to map in place, got %v", matchResult.Mappings.Src()[srcPrologue])
		}

		// Argument literal "prologue" maps in place.
		var srcArg, dstArg *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsFlattened(n.Type) && string(src[n.StartByte:n.EndByte]) == `"prologue"` {
				srcArg = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsFlattened(n.Type) && string(dst[n.StartByte:n.EndByte]) == `"prologue"` {
				dstArg = n
				break
			}
		}
		if srcArg == nil || dstArg == nil {
			t.Fatal("failed to find 'prologue' literal nodes")
		}
		if matchResult.Mappings.Src()[srcArg] != dstArg {
			t.Errorf("expected literal 'prologue' to map in place, got %v", matchResult.Mappings.Src()[srcArg])
		}

		// Statements that moved in order stay in group-1; only hoisted stmtD is ungrouped.
		groupMap := make(map[string]string)
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && r.IsStatement(a.Node.Type) {
				text := strings.TrimSpace(string(src[a.Node.StartByte:a.Node.EndByte]))
				groupMap[text] = a.GroupID
			}
		}

		for _, stmt := range []string{"stmtA()", "stmtB()", "stmtC()"} {
			if gid, ok := groupMap[stmt]; !ok || gid != "group-1" {
				t.Errorf("expected in-order statement %s to have GroupID group-1, got %q", stmt, gid)
			}
		}

		if gid, ok := groupMap["stmtD()"]; !ok || gid != "" {
			t.Errorf("expected out-of-order statement stmtD() to have empty GroupID, got %q", gid)
		}
	})
}

func TestGroupMoves_LIS_SunkStatementUngrouped(t *testing.T) {
	oldParent := &treesitter.ASTNode{Type: "block"}
	newParent := &treesitter.ASTNode{Type: "block"}

	// Statement 1 was moved to the end:
	// src: 1(10..20), 2(30..40), 3(50..60)
	// dst: 2(10..20), 3(30..40), 1(50..60)
	src1 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(src1, oldParent, 10, 20)
	dst1 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(dst1, newParent, 50, 60)

	src2 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(src2, oldParent, 30, 40)
	dst2 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(dst2, newParent, 10, 20)

	src3 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(src3, oldParent, 50, 60)
	dst3 := &treesitter.ASTNode{Type: "expression_statement"}
	setParentAndRange(dst3, newParent, 30, 40)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: src1, DestNode: dst1, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: src2, DestNode: dst2, Parent: newParent})
	es.Add(actions.Action{Type: actions.Move, Node: src3, DestNode: dst3, Parent: newParent})

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
	root := &treesitter.ASTNode{Type: "module", Language: "python"}
	fn := &treesitter.ASTNode{Type: "function_definition", Language: "python"}
	setParentAndRange(fn, root, 0, 100)

	// In Python, block has no delimiters and shares StartRow with its first statement
	block := &treesitter.ASTNode{Type: "block", Language: "python", StartRow: 1, EndRow: 2}
	setParentAndRange(block, fn, 20, 100)

	stmt1 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 1, EndRow: 1}
	setParentAndRange(stmt1, block, 24, 50)

	stmt2 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 2, EndRow: 2}
	setParentAndRange(stmt2, block, 54, 80)

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
	// Source tree: if cond: a = 1; b = 2 (all on row 0)
	srcRoot := &treesitter.ASTNode{Type: "module", Language: "python", StartRow: 0, EndRow: 0}
	srcIf := &treesitter.ASTNode{Type: "if_statement", Language: "python", StartRow: 0, EndRow: 0}
	setParentAndRange(srcIf, srcRoot, 0, 30)

	srcBlock := &treesitter.ASTNode{Type: "block", Language: "python", StartRow: 0, EndRow: 0}
	setParentAndRange(srcBlock, srcIf, 9, 30)

	srcStmt1 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 0, EndRow: 0}
	setParentAndRange(srcStmt1, srcBlock, 9, 18)

	srcStmt2 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 0, EndRow: 0}
	setParentAndRange(srcStmt2, srcBlock, 20, 29)

	// Destination tree: target block
	dstRoot := &treesitter.ASTNode{Type: "module", Language: "python", StartRow: 0, EndRow: 10}
	dstBlock := &treesitter.ASTNode{Type: "block", Language: "python", StartRow: 1, EndRow: 3}
	setParentAndRange(dstBlock, dstRoot, 10, 80)

	dstStmt1 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 1, EndRow: 1}
	setParentAndRange(dstStmt1, dstBlock, 15, 24)

	dstStmt2 := &treesitter.ASTNode{Type: "expression_statement", Language: "python", StartRow: 2, EndRow: 2}
	setParentAndRange(dstStmt2, dstBlock, 29, 38)

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
	for _, a := range normalized.Actions() {
		if a.Node != nil && r.IsContainerDeclaration(a.Node.Type) {
			t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
		}
	}

	// 2. Preceding stationary statement: print("prologue") maps in place.
	var srcPrologue, dstPrologue *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "prologue") {
			srcPrologue = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "prologue") {
			dstPrologue = n
			break
		}
	}
	if srcPrologue == nil || dstPrologue == nil {
		t.Fatal("failed to find prologue call nodes")
	}
	if matchResult.Mappings.Src()[srcPrologue] != dstPrologue {
		t.Errorf("expected prologue call to map in place, got %v", matchResult.Mappings.Src()[srcPrologue])
	}

	// 3. Succeeding stationary statement: print("epilogue") maps in place.
	var srcEpilogue, dstEpilogue *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "epilogue") {
			srcEpilogue = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "epilogue") {
			dstEpilogue = n
			break
		}
	}
	if srcEpilogue == nil || dstEpilogue == nil {
		t.Fatal("failed to find epilogue call nodes")
	}
	if matchResult.Mappings.Src()[srcEpilogue] != dstEpilogue {
		t.Errorf("expected epilogue call to map in place, got %v", matchResult.Mappings.Src()[srcEpilogue])
	}

	// 4. Stationary argument literals ("prologue", "epilogue") map in place.
	for _, argName := range []string{`"prologue"`, `"epilogue"`} {
		var sArg, dArg *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsFlattened(n.Type) && string(src[n.StartByte:n.EndByte]) == argName {
				sArg = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsFlattened(n.Type) && string(dst[n.StartByte:n.EndByte]) == argName {
				dArg = n
				break
			}
		}
		if sArg == nil || dArg == nil {
			t.Fatalf("failed to find literal %s", argName)
		}
		if matchResult.Mappings.Src()[sArg] != dArg {
			t.Errorf("expected literal %s to map in place, got %v", argName, matchResult.Mappings.Src()[sArg])
		}
	}

	// 5. Target statements moved_one() and moved_two() move together and receive identical GroupID "group-1".
	groupMap := make(map[string]string)
	for _, a := range normalized.Actions() {
		if a.Type == actions.Move && a.Node != nil && r.IsStatement(a.Node.Type) {
			text := strings.TrimSpace(string(src[a.Node.StartByte:a.Node.EndByte]))
			groupMap[text] = a.GroupID
		}
	}

	for _, stmt := range []string{"moved_one()", "moved_two()"} {
		if gid, ok := groupMap[stmt]; !ok || gid != "group-1" {
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

		for _, a := range normalized.Actions() {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		var srcPrepare, dstPrepare *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "prepare") {
				srcPrepare = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "prepare") {
				dstPrepare = n
				break
			}
		}
		if srcPrepare == nil || dstPrepare == nil {
			t.Fatal("failed to find prepare() call nodes")
		}
		if matchResult.Mappings.Src()[srcPrepare] != dstPrepare {
			t.Errorf("expected stationary prepare() to map in place, got %v", matchResult.Mappings.Src()[srcPrepare])
		}

		groupMap := make(map[string]string)
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && r.IsStatement(a.Node.Type) {
				text := strings.TrimSpace(string(src[a.Node.StartByte:a.Node.EndByte]))
				groupMap[text] = a.GroupID
			}
		}

		ifText := "if count > limit {\n\t\treturn flush()\n\t}"
		if gid, ok := groupMap[ifText]; !ok || gid == "" {
			t.Errorf("expected dominant if statement to have non-empty GroupID, got %q (all groups: %+v)", gid, groupMap)
		}
		if gid, ok := groupMap["count++"]; !ok || gid != "" {
			t.Errorf("expected reordered smaller statement count++ to have empty GroupID, got %q", gid)
		}
	})
}
