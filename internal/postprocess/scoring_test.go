package postprocess

import (
	"fmt"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestIsTerminatingStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("return statement is terminating", func(t *testing.T) {
		stmt := mkNode("return_statement", "")
		stmt.Language = "go"
		if !isTerminatingStatement(stmt, r) {
			t.Error("expected return_statement to be terminating")
		}
	})

	t.Run("os.Exit call is terminating", func(t *testing.T) {
		operand := mkNode("identifier", "os")
		operand.Language = "go"
		field := mkNode("field_identifier", "Exit")
		field.Language = "go"
		selector := mkNode("selector_expression", "")
		selector.Language = "go"
		selector.Children = []*treesitter.ASTNode{operand, field}
		operand.Parent = selector
		field.Parent = selector

		arg := mkNode("interpreted_string_literal", "1")
		arg.Language = "go"
		args := mkNode("argument_list", "")
		args.Language = "go"
		args.Children = []*treesitter.ASTNode{arg}
		arg.Parent = args

		call := mkNode("call_expression", "")
		call.Language = "go"
		call.Children = []*treesitter.ASTNode{selector, args}
		selector.Parent = call
		args.Parent = call

		exprStmt := mkNode("expression_statement", "")
		exprStmt.Language = "go"
		exprStmt.Children = []*treesitter.ASTNode{call}
		call.Parent = exprStmt

		if !isTerminatingStatement(exprStmt, r) {
			t.Error("expected os.Exit call to be terminating")
		}
	})

	t.Run("regular expression is not terminating", func(t *testing.T) {
		stmt := mkNode("expression_statement", "")
		stmt.Language = "go"
		call := mkNode("call_expression", "")
		call.Language = "go"
		operand := mkNode("identifier", "fmt")
		operand.Language = "go"
		field := mkNode("field_identifier", "Println")
		field.Language = "go"
		selector := mkNode("selector_expression", "")
		selector.Language = "go"
		selector.Children = []*treesitter.ASTNode{operand, field}
		operand.Parent = selector
		field.Parent = selector
		call.Children = []*treesitter.ASTNode{selector}
		selector.Parent = call
		stmt.Children = []*treesitter.ASTNode{call}
		call.Parent = stmt

		if isTerminatingStatement(stmt, r) {
			t.Error("expected fmt.Println call to NOT be terminating")
		}
	})
}

func TestIsTrivialJumpBody(t *testing.T) {
	r := rules.Get("go")

	t.Run("body of only jump statements is trivial", func(t *testing.T) {
		openBrace := mkNode("{", "{")
		retStmt := mkNode("return_statement", "")
		closeBrace := mkNode("}", "}")
		body := mkNode("block", "", openBrace, retStmt, closeBrace)
		setLanguageRecursive(body, "go")

		if !isTrivialJumpBody(body, r) {
			t.Error("expected body of only a return statement to be trivial")
		}
	})

	// An error print followed by an exit is still boilerplate - the print
	// shouldn't keep it from being scored as a trivial exit block.
	t.Run("body pairing an error print with a terminating call is trivial", func(t *testing.T) {
		openBrace := mkNode("{", "{")
		fprintfSel := mkNode("selector_expression", "",
			mkNode("identifier", "fmt"),
			mkNode("field_identifier", "Fprintf"),
		)
		printCall := mkNode("call_expression", "", fprintfSel, mkNode("argument_list", ""))
		printStmt := mkNode("expression_statement", "", printCall)

		exitSel := mkNode("selector_expression", "",
			mkNode("identifier", "os"),
			mkNode("field_identifier", "Exit"),
		)
		exitCall := mkNode("call_expression", "", exitSel, mkNode("argument_list", ""))
		exitStmt := mkNode("expression_statement", "", exitCall)
		closeBrace := mkNode("}", "}")

		body := mkNode("block", "", openBrace, printStmt, exitStmt, closeBrace)
		setLanguageRecursive(body, "go")

		if !isTrivialJumpBody(body, r) {
			t.Error("expected print+os.Exit body to be trivial boilerplate")
		}
	})
}

func TestMoveStructuralScore(t *testing.T) {
	r := rules.Get("go")
	ms := engine.NewMapping()

	t.Run("bare token clamped to 1", func(t *testing.T) {
		op := mkNode("arithmetic_operator_literal", "+")
		op.Language = "go"
		score := moveStructuralScore(op, r, ms)
		if score != 1 {
			t.Errorf("expected token score 1, got %d", score)
		}
	})

	t.Run("small if block with trivial body", func(t *testing.T) {
		retStmt := mkNode("return_statement", "")
		retStmt.Language = "go"
		body := mkNode("block", "", retStmt)
		body.Language = "go"
		retStmt.Parent = body
		ifStmt := mkNode("if_statement", "")
		ifStmt.Language = "go"
		cond := mkNode("identifier", "err")
		cond.Language = "go"
		ifStmt.Children = []*treesitter.ASTNode{cond, body}
		cond.Parent = ifStmt
		body.Parent = ifStmt

		score := moveStructuralScore(ifStmt, r, ms)
		// Size ~4, height ~2, lines 0, boilerplate -20 → clamp to 1
		if score < 1 {
			t.Errorf("expected score >= 1, got %d", score)
		}
	})

	t.Run("declaration gets +40 bonus", func(t *testing.T) {
		decl := mkNode("function_declaration", "foo")
		decl.Language = "go"
		decl.Children = []*treesitter.ASTNode{mkNode("block", "")}
		decl.Children[0].Parent = decl

		score := moveStructuralScore(decl, r, ms)
		if score < 40 {
			t.Errorf("expected declaration score >= 40, got %d", score)
		}
	})

	t.Run("container with deleted contents discounts size by surviving mass", func(t *testing.T) {
		id1 := mkNode("identifier", "a")
		id2 := mkNode("identifier", "b")
		id3 := mkNode("identifier", "c")
		stmt1 := mkNode("expression_statement", "", id1)
		stmt2 := mkNode("expression_statement", "", id2)
		stmt3 := mkNode("expression_statement", "", id3)
		block := mkNode("block", "", stmt1, stmt2, stmt3)
		setLanguageRecursive(block, "go")

		emptyMs := engine.NewMapping()
		scoreEmpty := moveStructuralScore(block, r, emptyMs)

		partialMs := engine.NewMapping()
		partner := mkNode("identifier", "a")
		partner.Language = "go"
		partialMs.Add(id1, partner)
		scorePartial := moveStructuralScore(block, r, partialMs)

		if scorePartial <= scoreEmpty {
			t.Errorf("expected score with surviving child (%d) > empty score (%d)", scorePartial, scoreEmpty)
		}

		fullMs := engine.NewMapping()
		fullMs.Add(id1, partner)
		fullMs.Add(id2, partner)
		fullMs.Add(id3, partner)
		scoreFull := moveStructuralScore(block, r, fullMs)
		expectedScore := block.Size() + 2*subtreeHeight(block) + 10
		if scoreFull > expectedScore {
			t.Errorf("expected scoreFull (%d) not to exceed undiscounted score (%d)", scoreFull, expectedScore)
		}
	})
}

func TestRequiredMoveThreshold(t *testing.T) {
	r := rules.Get("go")
	ms := engine.NewMapping()

	t.Run("intra-container reorder returns 1", func(t *testing.T) {
		parent := mkNode("argument_list", "")
		parent.Language = "go"
		src := mkNode("identifier", "a")
		src.Language = "go"
		src.Parent = parent
		dst := mkNode("identifier", "b")
		dst.Language = "go"
		dst.Parent = parent
		parent.Children = []*treesitter.ASTNode{src, dst}

		ms.Add(parent, parent)

		threshold := requiredMoveThreshold(src, dst, ms, r)
		if threshold != 1 {
			t.Errorf("expected threshold 1 for intra-container reorder, got %d", threshold)
		}
	})

	t.Run("top-level declaration returns 20", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.StartRow = 10
		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.StartRow = 500

		threshold := requiredMoveThreshold(srcFunc, dstFunc, ms, r)
		if threshold != 20 {
			t.Errorf("expected threshold 20 for top-level declaration, got %d", threshold)
		}
	})

	// Don't let a bare literal move between different struct fields just
	// because they sit on the same line (e.g. `defValue: ""` vs `shorthand: ""`).
	t.Run("cross-key bare literal at same line requires threshold >= 5", func(t *testing.T) {
		srcVal := mkNode("string_literal", "\"\"")
		srcVal.Language = "go"
		srcPair := mkNode("keyed_element", "", mkNode("field_identifier", "defValue"), srcVal)
		srcPair.Language = "go"
		srcVal.Parent = srcPair

		dstVal := mkNode("string_literal", "\"\"")
		dstVal.Language = "go"
		dstPair := mkNode("keyed_element", "", mkNode("field_identifier", "shorthand"), dstVal)
		dstPair.Language = "go"
		dstVal.Parent = dstPair

		// Same line (ΔL = 0); srcPair is intentionally left unmapped to dstPair.
		srcVal.StartRow, srcVal.EndRow = 5, 5
		dstVal.StartRow, dstVal.EndRow = 5, 5

		threshold := requiredMoveThreshold(srcVal, dstVal, ms, r)
		if threshold < 5 {
			t.Errorf("expected threshold >= 5 for cross-key bare literal, got %d (a clamped S=1 token would survive as a Move)", threshold)
		}
	})

	t.Run("same-line inline shift returns 1", func(t *testing.T) {
		ms := engine.NewMapping()
		srcStmt := mkNode("expression_statement", "foo()")
		srcStmt.Language = "go"
		srcStmt.StartRow, srcStmt.EndRow = 10, 10
		dstStmt := mkNode("expression_statement", "foo()")
		dstStmt.Language = "go"
		dstStmt.StartRow, dstStmt.EndRow = 10, 10

		threshold := requiredMoveThreshold(srcStmt, dstStmt, ms, r)
		if threshold != 1 {
			t.Errorf("requiredMoveThreshold() = %d, want 1", threshold)
		}
	})

	t.Run("distant statement across scopes does not get same-line threshold of 1", func(t *testing.T) {
		// A surviving outer scope shouldn't cancel out line distance between distant functions.
		rootSrc := mkNode("source_file", "")
		rootSrc.Language = "go"
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.Parent = rootSrc
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcIf := mkNode("if_statement", "")
		srcIf.Language = "go"
		srcIf.StartRow = 10
		srcIf.Parent = srcBody

		rootDst := mkNode("source_file", "")
		rootDst.Language = "go"
		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.Parent = rootDst
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstIf := mkNode("if_statement", "")
		dstIf.Language = "go"
		dstIf.StartRow = 210
		dstIf.Parent = dstBody

		msDrift := engine.NewMapping()
		msDrift.Add(rootSrc, rootDst)

		threshold := requiredMoveThreshold(srcIf, dstIf, msDrift, r)
		if threshold <= 1 {
			t.Errorf("expected threshold > 1 for distant statement 200 lines away, got %d", threshold)
		}
		// 200 lines apart gets the cross-scope penalty: 50 + 200/10 = 70.
		if threshold < 50 {
			t.Errorf("expected threshold >= 50 for 200-line distant statement across scopes, got %d", threshold)
		}
	})

	t.Run("initializer statement moving out of if gets statement threshold", func(t *testing.T) {
		rootSrc := mkNode("source_file", "")
		rootSrc.Language = "go"
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.Parent = rootSrc
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcIfBody := mkNode("block", "")
		srcIfBody.Language = "go"
		srcInit := mkNode("short_var_declaration", "")
		srcInit.Language = "go"
		srcInit.StartRow = 10
		srcIf := mkNode("if_statement", "", srcInit, srcIfBody)
		srcIf.Language = "go"
		srcIf.StartRow = 10
		srcIf.Parent = srcBody
		srcInit.Parent = srcIf
		srcIfBody.Parent = srcIf

		rootDst := mkNode("source_file", "")
		rootDst.Language = "go"
		dstFunc := mkNode("function_declaration", "foo")
		dstFunc.Language = "go"
		dstFunc.Parent = rootDst
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstAssign := mkNode("assignment_statement", "")
		dstAssign.Language = "go"
		dstAssign.StartRow = 15
		dstAssign.Parent = dstBody

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)
		ms.Add(srcBody, dstBody)

		threshold := requiredMoveThreshold(srcInit, dstAssign, ms, r)
		// Distance is 5 lines (10 to 15). With 4 + lineDist/5, threshold should be 5, not 20.
		if threshold >= 20 {
			t.Errorf("expected threshold < 20 for moving initializer statement, got %d", threshold)
		}
		if threshold != 5 {
			t.Errorf("expected threshold = 5 for moving initializer statement (4 + 5/5), got %d", threshold)
		}
	})
}

func TestIsFillerCallStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("bare call is filler", func(t *testing.T) {
		sel := mkNode("selector_expression", "")
		sel.Language = "go"
		sel.Children = []*treesitter.ASTNode{mkNode("identifier", "fmt"), mkNode("field_identifier", "Println")}
		sel.Children[0].Parent = sel
		sel.Children[1].Parent = sel
		call := mkNode("call_expression", "", sel)
		call.Language = "go"
		sel.Parent = call
		if !isFillerCallStatement(call, r) {
			t.Error("expected bare call to be filler")
		}
	})

	t.Run("single-child wrapper around call is filler", func(t *testing.T) {
		sel := mkNode("selector_expression", "")
		sel.Language = "go"
		sel.Children = []*treesitter.ASTNode{mkNode("identifier", "fmt"), mkNode("field_identifier", "Println")}
		sel.Children[0].Parent = sel
		sel.Children[1].Parent = sel
		call := mkNode("call_expression", "", sel)
		call.Language = "go"
		sel.Parent = call
		exprStmt := mkNode("expression_statement", "", call)
		exprStmt.Language = "go"
		call.Parent = exprStmt
		if !isFillerCallStatement(exprStmt, r) {
			t.Error("expected single-child wrapper around call to be filler")
		}
	})

	t.Run("multi-child statement is not filler", func(t *testing.T) {
		ifStmt := mkNode("if_statement", "")
		ifStmt.Language = "go"
		ifStmt.Children = []*treesitter.ASTNode{mkNode("identifier", "err"), mkNode("block", "")}
		ifStmt.Children[0].Parent = ifStmt
		ifStmt.Children[1].Parent = ifStmt
		if isFillerCallStatement(ifStmt, r) {
			t.Error("expected multi-child statement to not be filler")
		}
	})

	t.Run("nil node is not filler", func(t *testing.T) {
		if isFillerCallStatement(nil, r) {
			t.Error("expected nil to not be filler")
		}
	})
}

func TestIsPayloadLeaf(t *testing.T) {
	r := rules.Get("go")

	t.Run("nil safety and non-leaf", func(t *testing.T) {
		if isPayloadLeaf(nil, r) {
			t.Errorf("expected nil node to not be payload leaf")
		}
		parent := mkNode("call_expression", "", mkNode("identifier", "foo"))
		if isPayloadLeaf(parent, r) {
			t.Errorf("expected node with children to not be payload leaf")
		}
	})

	t.Run("punctuation and operators rejected", func(t *testing.T) {
		punct := mkNode(";", ";")
		if isPayloadLeaf(punct, r) {
			t.Errorf("expected semicolon to not be payload leaf")
		}
		op := mkNode("arithmetic_operator_literal", "+")
		if isPayloadLeaf(op, r) {
			t.Errorf("expected operator to not be payload leaf")
		}
	})

	t.Run("keywords filtering and jump statement exception", func(t *testing.T) {
		funcKw := mkNode("func", "func")
		funcKw.IsKeyword = true
		if isPayloadLeaf(funcKw, r) {
			t.Errorf("expected 'func' keyword to not be payload leaf")
		}

		retKwBare := mkNode("return", "return")
		retKwBare.IsKeyword = true
		if isPayloadLeaf(retKwBare, r) {
			t.Errorf("expected bare 'return' without jump parent to not be payload leaf")
		}

		jumpStmt := mkNode("return_statement", "")
		retKwUnderJump := mkNode("return", "return")
		retKwUnderJump.IsKeyword = true
		retKwUnderJump.Parent = jumpStmt
		jumpStmt.Children = append(jumpStmt.Children, retKwUnderJump)

		if !isPayloadLeaf(retKwUnderJump, r) {
			t.Errorf("expected 'return' keyword inside return_statement to be payload leaf")
		}
	})

	t.Run("content leaves accepted", func(t *testing.T) {
		id := mkNode("identifier", "myVar")
		if !isPayloadLeaf(id, r) {
			t.Errorf("expected identifier to be payload leaf")
		}
		lit := mkNode("interpreted_string_literal", `"hello"`)
		if !isPayloadLeaf(lit, r) {
			t.Errorf("expected literal to be payload leaf")
		}
	})
}

func TestSurvivingAndSummarizeMappedLeaves(t *testing.T) {
	r := rules.Get("go")

	srcLeaf1 := mkNode("identifier", "foo")
	srcLeaf2 := mkNode("identifier", "bar")
	srcPunct := mkNode(",", ",")
	srcBlock := mkNode("block", "", srcLeaf1, srcLeaf2, srcPunct)
	srcLeaf1.Parent = srcBlock
	srcLeaf2.Parent = srcBlock
	srcPunct.Parent = srcBlock

	dstLeaf1 := mkNode("identifier", "foo") // exact
	dstLeaf2 := mkNode("identifier", "baz") // updated label
	dstBlock := mkNode("block", "", dstLeaf1, dstLeaf2)
	dstLeaf1.Parent = dstBlock
	dstLeaf2.Parent = dstBlock

	ms := engine.NewMapping()
	ms.Add(srcLeaf1, dstLeaf1)
	ms.Add(srcLeaf2, dstLeaf2)

	t.Run("hasSurvivingMappedLeaves detects mapped payload", func(t *testing.T) {
		if !hasSurvivingMappedLeaves(srcBlock, dstBlock, ms.Src(), nil, r) {
			t.Fatalf("expected surviving mapped leaves")
		}
		// Bidirectional symmetry check
		if !hasSurvivingMappedLeaves(dstBlock, srcBlock, ms.Dst(), nil, r) {
			t.Fatalf("expected surviving mapped leaves in reverse mapping")
		}
	})

	t.Run("hasSurvivingMappedLeaves respects eviction", func(t *testing.T) {
		evicted := map[*treesitter.ASTNode]struct{}{
			srcLeaf1: {},
			srcLeaf2: {},
		}
		if hasSurvivingMappedLeaves(srcBlock, dstBlock, ms.Src(), evicted, r) {
			t.Errorf("expected no surviving leaves when all payload leaves are evicted")
		}
		// In reverse direction, partner (source node) eviction is respected
		if hasSurvivingMappedLeaves(dstBlock, srcBlock, ms.Dst(), evicted, r) {
			t.Errorf("expected no surviving leaves in reverse when partners are evicted")
		}
	})

	t.Run("summarizeMappedLeaves counts exact and updated leaves", func(t *testing.T) {
		s := summarizeMappedLeaves(srcBlock, dstBlock, ms, r, nil)
		if s.total != 2 {
			t.Errorf("expected total=2 (ignoring comma), got %d", s.total)
		}
		if s.exact != 1 {
			t.Errorf("expected exact=1 ('foo'), got %d", s.exact)
		}
		if s.updated != 1 {
			t.Errorf("expected updated=1 ('bar' -> 'baz'), got %d", s.updated)
		}

		// Evicting one leaf adjusts exact count
		sEvicted := summarizeMappedLeaves(srcBlock, dstBlock, ms, r, map[*treesitter.ASTNode]struct{}{srcLeaf1: {}})
		if sEvicted.total != 2 {
			t.Errorf("expected total=2, got %d", sEvicted.total)
		}
		if sEvicted.exact != 0 {
			t.Errorf("expected exact=0 after evicting 'foo', got %d", sEvicted.exact)
		}
		if sEvicted.updated != 1 {
			t.Errorf("expected updated=1 for 'bar' -> 'baz', got %d", sEvicted.updated)
		}
	})
}

func TestIsDelimitedOrBlockContainer(t *testing.T) {
	r := rules.Get("go")

	if isDelimitedOrBlockContainer(nil, r) {
		t.Errorf("expected nil to return false")
	}

	blockNode := mkNode("block", "")
	if !isDelimitedOrBlockContainer(blockNode, r) {
		t.Errorf("expected block to return true")
	}

	argListNode := mkNode("argument_list", "")
	if !isDelimitedOrBlockContainer(argListNode, r) {
		t.Errorf("expected argument_list to return true")
	}

	identNode := mkNode("identifier", "x")
	if isDelimitedOrBlockContainer(identNode, r) {
		t.Errorf("expected identifier to return false")
	}

	// r == nil fallback
	if !isDelimitedOrBlockContainer(blockNode, nil) {
		t.Errorf("expected block with nil rules to return true")
	}
}

func TestBuildCohortProtected(t *testing.T) {
	t.Run("cohort with anchor is protected", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		anchorSrc := mkNode("for_statement", "")
		anchorSrc.Language = "go"
		anchorSrc.StartRow, anchorSrc.EndRow = 10, 25
		anchorSrc.Parent = srcBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorSrc
			anchorSrc.Children = append(anchorSrc.Children, child)
		}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody

		stmt2Src := mkNode("short_var_declaration", "y := 2")
		stmt2Src.Language = "go"
		stmt2Src.Parent = srcBody

		srcBody.Children = []*treesitter.ASTNode{anchorSrc, stmt1Src, stmt2Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		anchorDst := mkNode("for_statement", "")
		anchorDst.Language = "go"
		anchorDst.StartRow, anchorDst.EndRow = 10, 25
		anchorDst.Parent = dstBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorDst
			anchorDst.Children = append(anchorDst.Children, child)
		}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody

		stmt2Dst := mkNode("short_var_declaration", "y := 2")
		stmt2Dst.Language = "go"
		stmt2Dst.Parent = dstBody

		dstBody.Children = []*treesitter.ASTNode{anchorDst, stmt1Dst, stmt2Dst}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		for i := range anchorSrc.Children {
			ms.Add(anchorSrc.Children[i], anchorDst.Children[i])
		}
		ms.Add(stmt1Src, stmt1Dst)
		ms.Add(stmt2Src, stmt2Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: anchorSrc, DestNode: anchorDst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt2Src, DestNode: stmt2Dst})

		protected := buildCohortProtected(es, ms)
		if !protected[anchorSrc] || !protected[stmt1Src] || !protected[stmt2Src] {
			t.Errorf("expected all 3 cohort moves to be protected, got: %+v", protected)
		}

		result := normalizeMovesByStructure(es, ms)
		moveCount := 0
		for _, a := range result.Actions() {
			if a.Type == actions.Move {
				moveCount++
			}
		}
		if moveCount != 3 {
			t.Errorf("expected 3 moves to survive in protected cohort, got %d", moveCount)
		}
	})

	t.Run("cohort without qualifying anchor is not protected", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody

		stmt2Src := mkNode("short_var_declaration", "y := 2")
		stmt2Src.Language = "go"
		stmt2Src.Parent = srcBody

		stmt3Src := mkNode("short_var_declaration", "z := 3")
		stmt3Src.Language = "go"
		stmt3Src.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{stmt1Src, stmt2Src, stmt3Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody

		stmt2Dst := mkNode("short_var_declaration", "y := 2")
		stmt2Dst.Language = "go"
		stmt2Dst.Parent = dstBody

		stmt3Dst := mkNode("short_var_declaration", "z := 3")
		stmt3Dst.Language = "go"
		stmt3Dst.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{stmt1Dst, stmt2Dst, stmt3Dst}

		ms := engine.NewMapping()
		ms.Add(stmt1Src, stmt1Dst)
		ms.Add(stmt2Src, stmt2Dst)
		ms.Add(stmt3Src, stmt3Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt2Src, DestNode: stmt2Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt3Src, DestNode: stmt3Dst})

		protected := buildCohortProtected(es, ms)
		if len(protected) != 0 {
			t.Errorf("expected 0 protected nodes when no anchor qualifies, got %d", len(protected))
		}

		result := normalizeMovesByStructure(es, ms)
		for _, a := range result.Actions() {
			if a.Type == actions.Move {
				t.Errorf("expected small move without anchor to be demoted, but found Move for %v", a.Node.Label)
			}
		}
	})

	t.Run("cohort with fewer than 3 moves is not protected", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		anchorSrc := mkNode("for_statement", "")
		anchorSrc.Language = "go"
		anchorSrc.Parent = srcBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorSrc
			anchorSrc.Children = append(anchorSrc.Children, child)
		}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{anchorSrc, stmt1Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		anchorDst := mkNode("for_statement", "")
		anchorDst.Language = "go"
		anchorDst.Parent = dstBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorDst
			anchorDst.Children = append(anchorDst.Children, child)
		}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{anchorDst, stmt1Dst}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		for i := range anchorSrc.Children {
			ms.Add(anchorSrc.Children[i], anchorDst.Children[i])
		}
		ms.Add(stmt1Src, stmt1Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: anchorSrc, DestNode: anchorDst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})

		protected := buildCohortProtected(es, ms)
		if len(protected) != 0 {
			t.Errorf("expected 0 protected nodes for cohort size < 3, got %d", len(protected))
		}
	})
}

func TestBuildCohortProtected_ContiguousMonotonicRun(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Contiguous, monotonic, diverse statements without anchor: protected.
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		ms := engine.NewMapping()
		es := actions.NewEditScript()

		var srcStmts, dstStmts []*treesitter.ASTNode
		for i := range 12 {
			sSrc := mkNode("short_var_declaration", fmt.Sprintf("val%d := compute%d(arg%d, param%d)", i, i, i, i))
			sSrc.Language = "go"
			sSrc.Parent = srcBody
			sSrc.StartRow = uint32(10 + i*2)
			sSrc.EndRow = uint32(10 + i*2 + 1)

			for _, sym := range []string{fmt.Sprintf("val%d", i), ":=", fmt.Sprintf("compute%d", i), fmt.Sprintf("arg%d", i), fmt.Sprintf("param%d", i)} {
				leaf := mkNode("identifier", sym)
				leaf.Language = "go"
				leaf.Parent = sSrc
				sSrc.Children = append(sSrc.Children, leaf)
			}

			sDst := mkNode("short_var_declaration", fmt.Sprintf("val%d := compute%d(arg%d, param%d)", i, i, i, i))
			sDst.Language = "go"
			sDst.Parent = dstBody
			sDst.StartRow = uint32(200 + i*2)
			sDst.EndRow = uint32(200 + i*2 + 1)

			for _, sym := range []string{fmt.Sprintf("val%d", i), ":=", fmt.Sprintf("compute%d", i), fmt.Sprintf("arg%d", i), fmt.Sprintf("param%d", i)} {
				leaf := mkNode("identifier", sym)
				leaf.Language = "go"
				leaf.Parent = sDst
				sDst.Children = append(sDst.Children, leaf)
			}

			srcStmts = append(srcStmts, sSrc)
			dstStmts = append(dstStmts, sDst)
			ms.Add(sSrc, sDst)
			es.Add(actions.Action{Type: actions.Move, Node: sSrc, DestNode: sDst})
		}
		srcBody.Children = srcStmts
		dstBody.Children = dstStmts

		protected := buildCohortProtected(es, ms)
		if len(protected) != 12 {
			t.Errorf("expected all 12 contiguous statements to be protected by CMR, got %d", len(protected))
		}

		// Non-contiguous run in source: interrupted by deleted statement.
		srcBodyBroken := mkNode("block", "")
		srcBodyBroken.Language = "go"
		srcBodyBroken.Parent = srcFunc

		s0 := srcStmts[0]
		s0.Parent = srcBodyBroken
		sDeleted := mkNode("expression_statement", "cleanup()")
		sDeleted.Language = "go"
		sDeleted.Parent = srcBodyBroken
		s1 := srcStmts[1]
		s1.Parent = srcBodyBroken
		s2 := srcStmts[2]
		s2.Parent = srcBodyBroken
		s3 := srcStmts[3]
		s3.Parent = srcBodyBroken
		srcBodyBroken.Children = []*treesitter.ASTNode{s0, sDeleted, s1, s2, s3}

		esBroken := actions.NewEditScript()
		esBroken.Add(actions.Action{Type: actions.Move, Node: s0, DestNode: dstStmts[0]})
		esBroken.Add(actions.Action{Type: actions.Move, Node: s1, DestNode: dstStmts[1]})
		esBroken.Add(actions.Action{Type: actions.Move, Node: s2, DestNode: dstStmts[2]})
		esBroken.Add(actions.Action{Type: actions.Move, Node: s3, DestNode: dstStmts[3]})

		protectedBroken := buildCohortProtected(esBroken, ms)
		if len(protectedBroken) != 0 {
			t.Errorf("expected 0 protected nodes for non-contiguous run (interrupted by deleted statement), got %d", len(protectedBroken))
		}

		// Repetitive assertion boilerplate with low entropy should not be protected.
		srcBodyBoiler := mkNode("block", "")
		srcBodyBoiler.Language = "go"
		srcBodyBoiler.Parent = srcFunc
		dstBodyBoiler := mkNode("block", "")
		dstBodyBoiler.Language = "go"
		dstBodyBoiler.Parent = dstFunc

		esBoiler := actions.NewEditScript()
		msBoiler := engine.NewMapping()
		var boilerSrc, boilerDst []*treesitter.ASTNode

		for i := range 10 {
			bSrc := mkNode("expression_statement", "assert.NoError(t, err)")
			bSrc.Language = "go"
			bSrc.Parent = srcBodyBoiler
			bSrc.StartRow = uint32(10 + i)
			for _, sym := range []string{"assert", "NoError", "t", "err"} {
				leaf := mkNode("identifier", sym)
				leaf.Language = "go"
				leaf.Parent = bSrc
				bSrc.Children = append(bSrc.Children, leaf)
			}

			bDst := mkNode("expression_statement", "assert.NoError(t, err)")
			bDst.Language = "go"
			bDst.Parent = dstBodyBoiler
			bDst.StartRow = uint32(300 + i)
			for _, sym := range []string{"assert", "NoError", "t", "err"} {
				leaf := mkNode("identifier", sym)
				leaf.Language = "go"
				leaf.Parent = bDst
				bDst.Children = append(bDst.Children, leaf)
			}

			boilerSrc = append(boilerSrc, bSrc)
			boilerDst = append(boilerDst, bDst)
			msBoiler.Add(bSrc, bDst)
			esBoiler.Add(actions.Action{Type: actions.Move, Node: bSrc, DestNode: bDst})
		}
		srcBodyBoiler.Children = boilerSrc
		dstBodyBoiler.Children = boilerDst

		protectedBoiler := buildCohortProtected(esBoiler, msBoiler)
		if len(protectedBoiler) != 0 {
			t.Errorf("expected 0 protected nodes for low-entropy repetitive assertions, got %d", len(protectedBoiler))
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func OldHandler() {
	config := loadConfig("production", true)
	logger := initLogger(config.LogLevel, "stderr")
	client := connectDatabase(config.DBHost, config.DBPort, config.Timeout)
	session := client.CreateSession("admin", true)
	metrics := registerMetrics("service_v1", session.ID)
	worker := newWorkerPool(config.Concurrency, metrics)
	worker.Start(session.Context())
	cache := initCache(config.CacheSize, "lru")
	server := startServer(config.HTTPPort, router, cache)
	notifyStartup(logger, server.Address())
}

func Helper1() { println("h1") }
func Helper2() { println("h2") }
func Helper3() { println("h3") }
func Helper4() { println("h4") }
func Helper5() { println("h5") }
func Helper6() { println("h6") }
func Helper7() { println("h7") }
func Helper8() { println("h8") }
func Helper9() { println("h9") }
func Helper10() { println("h10") }
`
		dst := `package main

func OldHandler() {
	println("placeholder")
}

func Helper1() { println("h1") }
func Helper2() { println("h2") }
func Helper3() { println("h3") }
func Helper4() { println("h4") }
func Helper5() { println("h5") }
func Helper6() { println("h6") }
func Helper7() { println("h7") }
func Helper8() { println("h8") }
func Helper9() { println("h9") }
func Helper10() { println("h10") }

func NewExtractedHandler() {
	config := loadConfig("production", true)
	logger := initLogger(config.LogLevel, "stderr")
	client := connectDatabase(config.DBHost, config.DBPort, config.Timeout)
	session := client.CreateSession("admin", true)
	metrics := registerMetrics("service_v1", session.ID)
	worker := newWorkerPool(config.Concurrency, metrics)
	worker.Start(session.Context())
	cache := initCache(config.CacheSize, "lru")
	server := startServer(config.HTTPPort, router, cache)
	notifyStartup(logger, server.Address())
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := engine.NewLinePartition([]byte(src), []byte(dst))
		mr := engine.Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		es := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		es = Run(es, mr.Mappings, srcAST, dstAST)

		// All contiguous procedural statements should survive as moves.
		moveCount := 0
		for _, a := range es.Actions() {
			if a.Type == actions.Move && a.Node != nil && rules.Get("go").IsStatement(a.Node.Type) {
				moveCount++
			}
		}

		if moveCount < 10 {
			t.Errorf("expected at least 10 statement moves preserved by CMR, got %d", moveCount)
		}
	})
}
