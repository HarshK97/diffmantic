package engine

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestSimpleRecoveryLabelMatch(t *testing.T) {
	// SimpleRecovery maps isomorphic children of mapped parents.
	srcChild := testutil.Leaf("id", "x")
	srcRoot := testutil.Node("block", "", srcChild)
	dstChild := testutil.Leaf("id", "x")
	dstRoot := testutil.Node("block", "", dstChild)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("isomorphic child should be recovered")
	}
	if m.Src()[srcChild] != dstChild {
		t.Error("child should map to corresponding dst child")
	}
}

func TestSimpleRecoveryStructureMatch(t *testing.T) {
	// SimpleRecovery maps children with matching structures but different labels.
	srcChild := testutil.Node("call", "", testutil.Leaf("id", "a"))
	srcRoot := testutil.Node("block", "", srcChild)
	dstChild := testutil.Node("call", "", testutil.Leaf("id", "b"))
	dstRoot := testutil.Node("block", "", dstChild)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("structurally isomorphic child should be recovered")
	}
}

func TestSimpleRecoveryUniqueType(t *testing.T) {
	// SimpleRecovery pairs nodes with unique types.
	srcIf := testutil.Node("if_stmt", "", testutil.Leaf("cond", "a"))
	srcLet := testutil.Leaf("let", "x")
	srcRoot := testutil.Node("block", "", srcIf, srcLet)

	dstIf := testutil.Node("if_stmt", "", testutil.Leaf("cond", "b"))
	dstLet := testutil.Leaf("let", "y")
	dstRoot := testutil.Node("block", "", dstIf, dstLet)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcIf) {
		t.Error("unique-type if_stmt should be paired")
	}
	if m.Src()[srcIf] != dstIf {
		t.Error("if_stmt should map to corresponding dst if_stmt")
	}
}

func TestSimpleRecoveryNoChildren(t *testing.T) {
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")
	m := NewMapping()
	m.Add(a, b)
	SimpleRecovery(a, b, m)

	if len(m.Pairs) != 1 {
		t.Errorf("leaf recovery should add no new pairs, got %d", len(m.Pairs))
	}
}

func TestUniqueTypePairs(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("str", "hello")
	b1 := testutil.Leaf("id", "y")
	b2 := testutil.Leaf("str", "world")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1, b2},
		nil,
	)
	if len(pairs) != 2 {
		t.Errorf("want 2 unique-type pairs, got %d", len(pairs))
	}
}

func TestUniqueTypePairsAmbiguous(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("id", "y")
	b1 := testutil.Leaf("id", "z")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1},
		nil,
	)
	if len(pairs) != 0 {
		t.Errorf("ambiguous type should not pair, got %d", len(pairs))
	}
}

func TestSimpleRecoveryStationaryNeighbors(t *testing.T) {
	leftSrc := testutil.Leaf("id", "a")
	midSrc := testutil.Leaf("id", "b")
	rightSrc := testutil.Leaf("id", "c")
	srcRoot := testutil.Node("block", "", leftSrc, midSrc, rightSrc)

	leftDst := testutil.Leaf("id", "a")
	midDst := testutil.Leaf("id", "b")
	rightDst := testutil.Leaf("id", "c")
	dstRoot := testutil.Node("block", "", leftDst, midDst, rightDst)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(leftSrc, leftDst)
	m.Add(rightSrc, rightDst)

	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(midSrc) {
		t.Fatal("stationary middle child was not recovered")
	}
	if m.Src()[midSrc] != midDst {
		t.Errorf("got %v, want %v", m.Src()[midSrc], midDst)
	}
}

func TestUniqueTypePairsDeclarationNoSemanticOverlap(t *testing.T) {
	// Two var_declarations that share only the keyword "var" must NOT pair.
	v1 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "headerStyle"),
		testutil.Leaf("type_identifier", "Style"),
	)
	root1 := testutil.Node("source_file", "", v1)
	root1.Language = "go"

	v2 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "rulesObj"),
		testutil.Leaf("type_identifier", "Rules"),
	)
	root2 := testutil.Node("source_file", "", v2)
	root2.Language = "go"

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{v1},
		[]*treesitter.ASTNode{v2},
		nil,
	)
	if len(pairs) != 0 {
		t.Fatalf("declarations sharing only keywords should not pair, got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairsDeclarationWithSemanticOverlap(t *testing.T) {
	// Two var_declarations that share a semantic identifier ("count") should pair.
	v1 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "count"),
		testutil.Leaf("type_identifier", "int"),
	)
	root1 := testutil.Node("source_file", "", v1)
	root1.Language = "go"

	v2 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "count"),
		testutil.Leaf("type_identifier", "int64"),
	)
	root2 := testutil.Node("source_file", "", v2)
	root2.Language = "go"

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{v1},
		[]*treesitter.ASTNode{v2},
		nil,
	)
	if len(pairs) != 1 {
		t.Fatalf("declarations sharing semantic identifier should pair, got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairs_UnwrappedCondition(t *testing.T) {
	// src is a chained || while dst is a single check.
	// uniqueTypePairs should match the inner z == nil, not the root ||.
	c1 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "x"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c2 := testutil.Node("binary_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "x"),
			testutil.Leaf("field_identifier", "Parent"),
		),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c3 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "z"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c12 := testutil.Node("binary_expression", "",
		c1,
		testutil.Leaf("logical_operator_literal", "||"),
		c2,
	)
	compoundSrc := testutil.Node("binary_expression", "",
		c12,
		testutil.Leaf("logical_operator_literal", "||"),
		c3,
	)
	compoundSrc.Language = "go"

	simpleDst := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "v"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	simpleDst.Language = "go"

	m := NewMapping()
	m.Add(c3.Children[2], simpleDst.Children[2])

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{compoundSrc},
		[]*treesitter.ASTNode{simpleDst},
		m,
	)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0][0] != c3 {
		t.Errorf("expected inner c3 (z == nil) to be paired, got %v", pairs[0][0])
	}
	if pairs[0][1] != simpleDst {
		t.Errorf("expected simpleDst (v == nil) to be paired, got %v", pairs[0][1])
	}
}

func TestUniqueTypePairs_WrappedCondition(t *testing.T) {
	// Flipped case: src is simple and dst is the chained ||.
	// It should still pair v == nil with the inner z == nil.
	simpleSrc := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "v"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	simpleSrc.Language = "go"

	c1 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "x"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c2 := testutil.Node("binary_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "x"),
			testutil.Leaf("field_identifier", "Parent"),
		),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c3 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "z"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c12 := testutil.Node("binary_expression", "",
		c1,
		testutil.Leaf("logical_operator_literal", "||"),
		c2,
	)
	compoundDst := testutil.Node("binary_expression", "",
		c12,
		testutil.Leaf("logical_operator_literal", "||"),
		c3,
	)
	compoundDst.Language = "go"

	m := NewMapping()
	m.Add(simpleSrc.Children[2], c3.Children[2])

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{simpleSrc},
		[]*treesitter.ASTNode{compoundDst},
		m,
	)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0][0] != simpleSrc {
		t.Errorf("expected simpleSrc (v == nil) to be paired, got %v", pairs[0][0])
	}
	if pairs[0][1] != c3 {
		t.Errorf("expected inner c3 (z == nil) to be paired, got %v", pairs[0][1])
	}
}

func TestUniqueTypePairsDisparateJSXTags(t *testing.T) {
	spanOpen := testutil.Node("jsx_opening_element", "",
		testutil.Leaf("comparison_operator_literal", "<"),
		testutil.Leaf("identifier", "span"),
		testutil.Leaf("comparison_operator_literal", ">"),
	)
	spanText := testutil.Leaf("jsx_text", "Recent Admissions")
	spanClose := testutil.Node("jsx_closing_element", "",
		testutil.Leaf("jsx_closing_tag_lt_slash", "</"),
		testutil.Leaf("identifier", "span"),
		testutil.Leaf("comparison_operator_literal", ">"),
	)
	spanElem := testutil.Node("jsx_element", "", spanOpen, spanText, spanClose)
	spanElem.Language = "tsx"

	iOpen := testutil.Node("jsx_opening_element", "",
		testutil.Leaf("comparison_operator_literal", "<"),
		testutil.Leaf("identifier", "i"),
		testutil.Leaf("comparison_operator_literal", ">"),
	)
	iClose := testutil.Node("jsx_closing_element", "",
		testutil.Leaf("jsx_closing_tag_lt_slash", "</"),
		testutil.Leaf("identifier", "i"),
		testutil.Leaf("comparison_operator_literal", ">"),
	)
	iElem := testutil.Node("jsx_element", "", iOpen, iClose)
	iElem.Language = "tsx"

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{spanElem}, []*treesitter.ASTNode{iElem}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate JSX tags ('span' vs 'i'), got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairs_BlindSingletonTrapStatement(t *testing.T) {
	// Two for_statements with completely different headers and zero mapped descendants.
	// Source: for item := range list { ... }
	forRange := testutil.Node("for_statement", "",
		testutil.Leaf("for", "for"),
		testutil.Leaf("identifier", "item"),
		testutil.Leaf("identifier", "list"),
		testutil.Node("block", "", testutil.Leaf("identifier", "oldBody")),
	)
	forRange.Language = "go"

	// Destination: for i := 0; i < n; i++ { ... }
	forThreeClause := testutil.Node("for_statement", "",
		testutil.Leaf("for", "for"),
		testutil.Leaf("identifier", "i"),
		testutil.Leaf("int_literal", "0"),
		testutil.Leaf("identifier", "n"),
		testutil.Leaf("operator", "++"),
		testutil.Node("block", "", testutil.Leaf("identifier", "newBody")),
	)
	forThreeClause.Language = "go"

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{forRange}, []*treesitter.ASTNode{forThreeClause}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate statements with 0 affinity, got %d pairs", len(pairs))
	}

	// Two for_statements with isomorphic headers should still pair even with an empty mapping.
	for1 := testutil.Node("for_statement", "",
		testutil.Leaf("for", "for"),
		testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "i"),
			testutil.Leaf("comparison_operator_literal", "<"),
			testutil.Leaf("identifier", "n"),
		),
		testutil.Node("block", "", testutil.Leaf("identifier", "bodyA")),
	)
	for1.Language = "go"

	for2 := testutil.Node("for_statement", "",
		testutil.Leaf("for", "for"),
		testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "i"),
			testutil.Leaf("comparison_operator_literal", "<"),
			testutil.Leaf("identifier", "n"),
		),
		testutil.Node("block", "", testutil.Leaf("identifier", "bodyB")),
	)
	for2.Language = "go"

	m2 := NewMapping()
	pairs2 := uniqueTypePairs([]*treesitter.ASTNode{for1}, []*treesitter.ASTNode{for2}, m2)
	if len(pairs2) != 1 {
		t.Errorf("uniqueTypePairs SHOULD pair statements with isomorphic headers, got %d pairs", len(pairs2))
	}
}

func TestRecover_ReclaimInParentIsomorphicChildren(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Both source blocks share an identical statement. TopDown paired block2's copy
		// with dstBlock1. When block1 maps to dstBlock1, Recover should reclaim stmt2
		// for block1's own statement.
		stmt1A := testutil.Node("short_var_declaration", "",
			testutil.Leaf("identifier", "shared"),
			testutil.Leaf("assignment_operator_literal", ":="),
			testutil.Node("call_expression", "",
				testutil.Leaf("identifier", "calc"),
			),
		)
		stmt1B := testutil.Node("short_var_declaration", "",
			testutil.Leaf("identifier", "shared"),
			testutil.Leaf("assignment_operator_literal", ":="),
			testutil.Node("call_expression", "",
				testutil.Leaf("identifier", "calc"),
			),
		)
		stmt2 := testutil.Node("short_var_declaration", "",
			testutil.Leaf("identifier", "shared"),
			testutil.Leaf("assignment_operator_literal", ":="),
			testutil.Node("call_expression", "",
				testutil.Leaf("identifier", "calc"),
			),
		)

		block1 := testutil.Node("block", "", stmt1A)
		block2 := testutil.Node("block", "", stmt1B)
		srcRoot := testutil.Node("block", "", block1, block2)
		srcRoot.Language = "go"

		dstBlock1 := testutil.Node("block", "", stmt2)
		dstRoot := testutil.Node("block", "", dstBlock1)
		dstRoot.Language = "go"

		m := NewMapping()
		// Premature cross-parent match: stmt1B -> stmt2.
		addIsomorphicPairs(stmt1B, stmt2, m)
		m.Add(block1, dstBlock1)

		Recover(block1, dstBlock1, m)

		if m.Src()[stmt1A] != stmt2 {
			t.Errorf("expected Recover to reclaim stmt2 for in-parent child stmt1A, got %v", m.Src()[stmt1A])
		}
		if m.Has(stmt1B) {
			t.Errorf("expected foreign duplicate stmt1B to be unmapped after reclaim")
		}
		if m.Has(stmt1B.Children[0]) {
			t.Errorf("expected descendants of foreign duplicate stmt1B to be unmapped after reclaim")
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func Run(mode int) {
	if mode == 1 {
		runA("oldArg")
		shared := calc()
	}

	if mode == 2 {
		helper()
		shared := calc()
		finishB()
	}
}
`
		dst := `package main

func Run(mode int) {
	if mode == 1 {
		helper()
		shared := calc()
		runA("newArg")
	}

	if mode == 2 {
		finishB()
	}
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Line rows are 0-indexed in Tree-sitter:
		// In src, row 5 (line 6) is "shared := calc()" inside "mode == 1".
		// In src, row 10 (line 11) is "shared := calc()" inside "mode == 2".
		// In dst, row 5 (line 6) is "shared := calc()" inside "mode == 1".
		var srcShared1, srcShared2, dstShared *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "short_var_declaration" {
				switch n.StartRow {
				case 5:
					srcShared1 = n
				case 10:
					srcShared2 = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "short_var_declaration" && n.StartRow == 5 {
				dstShared = n
			}
		}

		if srcShared1 == nil || srcShared2 == nil || dstShared == nil {
			t.Fatalf("failed to find short_var_declaration nodes: srcShared1=%v, srcShared2=%v, dstShared=%v", srcShared1, srcShared2, dstShared)
		}

		// With reclaimInParentIsomorphicChildren, srcShared1 (inside mode == 1) maps in-place to dstShared.
		// On main without the fix, TopDown paired foreign srcShared2 (mode == 2) with dstShared.
		if mr.Mappings.Src()[srcShared1] != dstShared {
			t.Errorf("expected in-parent statement (row 5) to map to dstShared, got %v", mr.Mappings.Src()[srcShared1])
		}
		if mr.Mappings.Has(srcShared2) {
			t.Errorf("expected foreign duplicate (row 10) to remain unmapped, got %v", mr.Mappings.Src()[srcShared2])
		}
		if mr.Mappings.Has(srcShared2.Children[0]) {
			t.Errorf("expected descendants of foreign duplicate (row 10) to remain unmapped")
		}

		// Whole-snippet verification: func Run, both if blocks, and finishB map in place
		var srcRun, dstRun, srcIf1, dstIf1, srcIf2, dstIf2 *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "function_declaration" {
				srcRun = n
			}
			if n.Type == "if_statement" {
				switch n.StartRow {
				case 3:
					srcIf1 = n
				case 8:
					srcIf2 = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "function_declaration" {
				dstRun = n
			}
			if n.Type == "if_statement" {
				switch n.StartRow {
				case 3:
					dstIf1 = n
				case 9:
					dstIf2 = n
				}
			}
		}
		if srcRun == nil || dstRun == nil || srcIf1 == nil || dstIf1 == nil || srcIf2 == nil || dstIf2 == nil {
			t.Fatalf("missing control structures: srcRun=%v, dstRun=%v, srcIf1=%v, dstIf1=%v, srcIf2=%v, dstIf2=%v",
				srcRun, dstRun, srcIf1, dstIf1, srcIf2, dstIf2)
		}
		if mr.Mappings.Src()[srcRun] != dstRun {
			t.Errorf("expected func Run to map in place, got %v", mr.Mappings.Src()[srcRun])
		}
		if mr.Mappings.Src()[srcIf1] != dstIf1 {
			t.Errorf("expected if mode == 1 to map in place, got %v", mr.Mappings.Src()[srcIf1])
		}
		if mr.Mappings.Src()[srcIf2] != dstIf2 {
			t.Errorf("expected if mode == 2 to map in place, got %v", mr.Mappings.Src()[srcIf2])
		}

		// Verify finishB statement in mode == 2 maps in place
		var srcFinishB, dstFinishB *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "expression_statement" && strings.Contains(string([]byte(src)[n.StartByte:n.EndByte]), "finishB") {
				srcFinishB = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "expression_statement" && strings.Contains(string([]byte(dst)[n.StartByte:n.EndByte]), "finishB") {
				dstFinishB = n
			}
		}
		if srcFinishB == nil || dstFinishB == nil {
			t.Fatalf("missing finishB statements: srcFinishB=%v, dstFinishB=%v", srcFinishB, dstFinishB)
		}
		if mr.Mappings.Src()[srcFinishB] != dstFinishB {
			t.Errorf("expected finishB statement to map to dstFinishB, got %v", mr.Mappings.Src()[srcFinishB])
		}
	})
}

func TestFindBestExpressionMatch_StopsAtClosureAndBlockBoundary(t *testing.T) {
	// Outer call `sort.SliceStable(scored, func() { return dice })` vs
	// `slices.SortStableFunc(scored, func() { return cmp.Or(dice) })`.
	// Expression unwrapping must NOT tunnel through `func_literal` -> `block` -> `return_statement`
	// to hijack the outer call onto the inner `cmp.Or(dice)` call.
	outerCall1 := testutil.Node("call_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "sort"),
			testutil.Leaf("field_identifier", "SliceStable"),
		),
		testutil.Node("argument_list", "",
			testutil.Leaf("identifier", "scored"),
			testutil.Node("func_literal", "",
				testutil.Node("block", "",
					testutil.Node("return_statement", "",
						testutil.Leaf("field_identifier", "dice"),
					),
				),
			),
		),
	)
	outerCall1.Language = "go"

	innerCmpCall := testutil.Node("call_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "cmp"),
			testutil.Leaf("field_identifier", "Or"),
		),
		testutil.Node("argument_list", "",
			testutil.Leaf("field_identifier", "dice"),
		),
	)
	innerCmpCall.Language = "go"

	outerCall2 := testutil.Node("call_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "slices"),
			testutil.Leaf("field_identifier", "SortStableFunc"),
		),
		testutil.Node("argument_list", "",
			testutil.Leaf("identifier", "scored"),
			testutil.Node("func_literal", "",
				testutil.Node("block", "",
					testutil.Node("return_statement", "",
						innerCmpCall,
					),
				),
			),
		),
	)
	outerCall2.Language = "go"

	r := rules.Get("go")
	m := NewMapping()
	cand := findUnwrappedDstExpressionCandidate(outerCall1, outerCall2, m, r)
	if cand == innerCmpCall {
		t.Errorf("findUnwrappedDstExpressionCandidate should not cross closure/block/statement boundary to return innerCmpCall")
	}
}

func TestUniqueTypePairs_ClosureCallUsesContainerThreshold(t *testing.T) {
	// Calls that wrap a closure contain a nested block inside their argument list.
	// uniqueTypePairs should treat them as containers (needing >= 0.50 Dice)
	// instead of flat expressions (0.25 overlap).
	call1 := testutil.Node("call_expression", "",
		testutil.Leaf("identifier", "sortSlice"),
		testutil.Node("argument_list", "",
			testutil.Leaf("identifier", "scored"),
			testutil.Node("func_literal", "",
				testutil.Node("block", "",
					testutil.Leaf("identifier", "a1"),
					testutil.Leaf("identifier", "a2"),
					testutil.Leaf("identifier", "a3"),
					testutil.Leaf("identifier", "a4"),
				),
			),
		),
	)
	call1.Language = "go"

	call2 := testutil.Node("call_expression", "",
		testutil.Leaf("identifier", "sortStableFunc"),
		testutil.Node("argument_list", "",
			testutil.Leaf("identifier", "scored"),
			testutil.Node("func_literal", "",
				testutil.Node("block", "",
					testutil.Leaf("identifier", "a1"),
					testutil.Leaf("identifier", "b2"),
					testutil.Leaf("identifier", "b3"),
					testutil.Leaf("identifier", "b4"),
					testutil.Leaf("identifier", "b5"),
					testutil.Leaf("identifier", "b6"),
				),
			),
		),
	)
	call2.Language = "go"

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{call1}, []*treesitter.ASTNode{call2}, m)
	if len(pairs) != 0 {
		t.Errorf("expected closure-wrapping calls with < 0.50 container Dice to be rejected by uniqueTypePairs, got %d pairs", len(pairs))
	}

	pairsRev := uniqueTypePairs([]*treesitter.ASTNode{call2}, []*treesitter.ASTNode{call1}, m)
	if len(pairsRev) != 0 {
		t.Errorf("expected reverse closure-wrapping call pair to be rejected by uniqueTypePairs, got %d pairs", len(pairsRev))
	}
}

func TestRecovery_ClosureCallExpressionUnwrapping_FullEngine(t *testing.T) {
	// Replacing sortSliceStable with sortStableFunc returning cmpOr.
	// Recovery and uniqueTypePairs must not tunnel through the closure boundary to pair
	// the outer sortSliceStable call onto the inner cmpOr call.
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

	part := NewLinePartition(srcBytes, dstBytes)
	mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	var srcOuterCall *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if n.Type == "call_expression" && strings.HasPrefix(string(srcBytes[n.StartByte:n.EndByte]), "sortSliceStable") {
			srcOuterCall = n
			break
		}
	}

	var dstInnerCmpCall *treesitter.ASTNode
	for _, n := range dstAST.Descendants() {
		if n.Type == "call_expression" && strings.HasPrefix(string(dstBytes[n.StartByte:n.EndByte]), "cmpOr") {
			dstInnerCmpCall = n
			break
		}
	}

	if srcOuterCall == nil || dstInnerCmpCall == nil {
		t.Fatalf("failed to locate target calls: srcOuterCall=%v, dstInnerCmpCall=%v", srcOuterCall, dstInnerCmpCall)
	}

	// On main without the boundary guard and container Dice threshold,
	// uniqueTypePairs and findBestExpressionMatch mapped srcOuterCall to dstInnerCmpCall.
	if mapped := mr.Mappings.Src()[srcOuterCall]; mapped == dstInnerCmpCall {
		t.Errorf("outer sortSliceStable call was erroneously mapped to inner cmpOr call across closure boundary")
	}
	if mr.Mappings.Dst()[dstInnerCmpCall] != nil {
		t.Errorf("inner cmpOr call should remain unmapped, got %v", mr.Mappings.Dst()[dstInnerCmpCall])
	}

	// Whole-snippet verification: func sortItems, expression statement, and closure function literal map in place
	var srcFunc, dstFunc, srcExprStmt, dstExprStmt, srcClosure, dstClosure *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		switch n.Type {
		case "function_declaration":
			srcFunc = n
		case "expression_statement":
			srcExprStmt = n
		case "func_literal":
			srcClosure = n
		}
	}
	for _, n := range dstAST.Descendants() {
		switch n.Type {
		case "function_declaration":
			dstFunc = n
		case "expression_statement":
			dstExprStmt = n
		case "func_literal":
			dstClosure = n
		}
	}

	if srcFunc == nil || dstFunc == nil || srcExprStmt == nil || dstExprStmt == nil || srcClosure == nil || dstClosure == nil {
		t.Fatalf("missing nodes: srcFunc=%v, dstFunc=%v, srcExprStmt=%v, dstExprStmt=%v, srcClosure=%v, dstClosure=%v",
			srcFunc, dstFunc, srcExprStmt, dstExprStmt, srcClosure, dstClosure)
	}
	if mr.Mappings.Src()[srcFunc] != dstFunc {
		t.Errorf("expected func sortItems to map in place, got %v", mr.Mappings.Src()[srcFunc])
	}
	if mr.Mappings.Src()[srcExprStmt] != dstExprStmt {
		t.Errorf("expected expression statement to map in place, got %v", mr.Mappings.Src()[srcExprStmt])
	}
	if mr.Mappings.Src()[srcClosure] != dstClosure {
		t.Errorf("expected closure function literal to map in place, got %v", mr.Mappings.Src()[srcClosure])
	}
}

func TestUniqueTypePairs_RejectsSiblingAbsorptionInExpressions(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// When an operand in (A && B) already matched inside an outer (A && Y), make sure
		// uniqueTypePairs doesn't pair the surviving sibling B to that outer expression
		// just because they're both binary expressions.
		opA := testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "len"),
			testutil.Leaf(">", ">"),
			testutil.Leaf("int", "0"),
		)
		opA.Language = "go"

		opB := testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "len"),
			testutil.Leaf(">", ">"),
			testutil.Leaf("int", "0"),
		)
		opB.Language = "go"

		t1 := testutil.Node("binary_expression", "",
			opA,
			testutil.Leaf("&&", "&&"),
			opB,
		)
		t1.Language = "go"

		opAPrime := testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "len"),
			testutil.Leaf(">", ">"),
			testutil.Leaf("int", "0"),
		)
		opAPrime.Language = "go"

		innerC := testutil.Node("binary_expression", "",
			opAPrime,
			testutil.Leaf("&&", "&&"),
			testutil.Leaf("identifier", "hasPayload"),
		)
		innerC.Language = "go"

		t2 := testutil.Node("binary_expression", "",
			innerC,
			testutil.Leaf("&&", "&&"),
			testutil.Leaf("identifier", "hasMapped"),
		)
		t2.Language = "go"

		m := NewMapping()
		m.Add(t1, t2)
		m.Add(opA, opAPrime)

		pairs := uniqueTypePairs([]*treesitter.ASTNode{opB}, []*treesitter.ASTNode{innerC}, m)
		if len(pairs) != 0 {
			t.Errorf("expected opB not to pair with innerC (sibling absorption conflict), got %d pairs", len(pairs))
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		// When a duplicate/chained binary expression operand is deleted and the surviving sibling
		// is absorbed into an intermediate wrapper, uniqueTypePairs must not pair the deleted
		// operand with the intermediate wrapper.
		src := `package main

type Item struct {
	Payload []byte
}

func check(item Item, hasPayload, hasMapped bool) bool {
	if len(item.Payload) > 0 && len(item.Payload) > 0 {
		return true
	}
	return false
}
`
		dst := `package main

type Item struct {
	Payload []byte
}

func check(item Item, hasPayload, hasMapped bool) bool {
	if len(item.Payload) > 0 && hasPayload && hasMapped {
		return true
	}
	return false
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		// Locate if_statements and conditions
		var srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				srcIf = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				dstIf = n
				break
			}
		}
		if srcIf == nil || dstIf == nil {
			t.Fatalf("missing if_statement: srcIf=%v, dstIf=%v", srcIf, dstIf)
		}

		// Condition of srcIf is `len(item.Payload) > 0 && len(item.Payload) > 0`
		// Condition of dstIf is `(len(item.Payload) > 0 && hasPayload) && hasMapped`
		srcCond := srcIf.Children[1]
		dstCond := dstIf.Children[1]
		if !r.IsExpression(srcCond.Type) || !r.IsExpression(dstCond.Type) {
			t.Fatalf("expected expression conditions: src=%v, dst=%v", srcCond.Type, dstCond.Type)
		}

		srcFirstOp := srcCond.Children[0]  // len(item.Payload) > 0 (left)
		srcSecondOp := srcCond.Children[2] // len(item.Payload) > 0 (right)

		dstInnerWrapper := dstCond.Children[0]    // len(item.Payload) > 0 && hasPayload
		dstFirstOp := dstInnerWrapper.Children[0] // len(item.Payload) > 0

		// Target assertion: the stationary first operand must map in place
		if mr.Mappings.Src()[srcFirstOp] != dstFirstOp {
			t.Errorf("expected stationary first operand to map in place, got %v", mr.Mappings.Src()[srcFirstOp])
		}

		// Defect assertion: on main without sibling absorption rejection, uniqueTypePairs
		// erroneously paired the deleted second operand (srcSecondOp) to dstInnerWrapper.
		if mapped := mr.Mappings.Src()[srcSecondOp]; mapped == dstInnerWrapper {
			t.Errorf("deleted second operand was erroneously mapped to intermediate wrapper dstInnerWrapper")
		}
		if mr.Mappings.Has(srcSecondOp) {
			t.Errorf("expected deleted second operand to remain unmapped, got %v", mr.Mappings.Src()[srcSecondOp])
		}
		if mr.Mappings.Dst()[dstInnerWrapper] != nil {
			t.Errorf("expected intermediate wrapper dstInnerWrapper to remain unmapped, got %v", mr.Mappings.Dst()[dstInnerWrapper])
		}

		// Whole-snippet verification: enclosing type and function declarations, surrounding statements, and returns map in place
		var srcTypeDecl, dstTypeDecl, srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				switch n.Type {
				case "type_declaration":
					srcTypeDecl = n
				case "function_declaration":
					srcFunc = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				switch n.Type {
				case "type_declaration":
					dstTypeDecl = n
				case "function_declaration":
					dstFunc = n
				}
			}
		}
		if srcTypeDecl == nil || dstTypeDecl == nil || srcFunc == nil || dstFunc == nil {
			t.Fatalf("missing declarations: srcTypeDecl=%v, dstTypeDecl=%v, srcFunc=%v, dstFunc=%v",
				srcTypeDecl, dstTypeDecl, srcFunc, dstFunc)
		}
		if mr.Mappings.Src()[srcTypeDecl] != dstTypeDecl {
			t.Errorf("expected type Item to map in place, got %v", mr.Mappings.Src()[srcTypeDecl])
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected func check to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}
		if mr.Mappings.Src()[srcIf] != dstIf {
			t.Errorf("expected if_statement to map in place, got %v", mr.Mappings.Src()[srcIf])
		}

		// Both return statements must map in place
		var srcReturns, dstReturns []*treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsJumpStatement(n.Type) {
				srcReturns = append(srcReturns, n)
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsJumpStatement(n.Type) {
				dstReturns = append(dstReturns, n)
			}
		}
		if len(srcReturns) != 2 || len(dstReturns) != 2 {
			t.Fatalf("expected 2 return_statements in src and dst, got src=%d, dst=%d", len(srcReturns), len(dstReturns))
		}
		if mr.Mappings.Src()[srcReturns[0]] != dstReturns[0] {
			t.Errorf("expected return true to map in place, got %v", mr.Mappings.Src()[srcReturns[0]])
		}
		if mr.Mappings.Src()[srcReturns[1]] != dstReturns[1] {
			t.Errorf("expected return false to map in place, got %v", mr.Mappings.Src()[srcReturns[1]])
		}
	})
}
