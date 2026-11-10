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

func TestRecovery_CrossSideOperatorGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package renderutil

func formatBadge(lastChunkStr, badgeText string) bool {
	prefix := "badge:"
	if !hasSuffix(lastChunkStr, color.MoveFg+badgeText+color.Reset) {
		return false
	}
	return true
}
`
		dst := `package renderutil

func formatBadge(lastChunkStr, badgeText string) bool {
	prefix := "badge:"
	if !hasSuffix(lastChunkStr, color.Dim+color.Italic+color.MoveFg+badgeText+color.Reset) {
		return false
	}
	return true
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

		// 1. Enclosing Context: Function declaration, parameter list, and parameter identifiers
		var srcFunc, dstFunc, srcParams, dstParams *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Label == "formatBadge" {
						srcFunc = n
						break
					}
				}
			}
		}
		if srcFunc != nil {
			for _, c := range srcFunc.Children {
				if r.IsDelimitedContainer(c.Type) {
					srcParams = c
					break
				}
			}
		}

		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Label == "formatBadge" {
						dstFunc = n
						break
					}
				}
			}
		}
		if dstFunc != nil {
			for _, c := range dstFunc.Children {
				if r.IsDelimitedContainer(c.Type) {
					dstParams = c
					break
				}
			}
		}
		if srcFunc == nil || dstFunc == nil || srcParams == nil || dstParams == nil {
			t.Fatalf("missing func or params: srcFunc=%v, dstFunc=%v, srcParams=%v, dstParams=%v",
				srcFunc, dstFunc, srcParams, dstParams)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected function formatBadge to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}
		if mr.Mappings.Src()[srcParams] != dstParams {
			t.Errorf("expected parameter_list to map in place, got %v", mr.Mappings.Src()[srcParams])
		}

		// 2. Surrounding Sequence: Preceding short_var_declaration, if_statement, and returns
		var srcPrefixStmt, dstPrefixStmt, srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && n.StartRow == 3 {
				srcPrefixStmt = n
			}
			if r.IsStatement(n.Type) && n.StartRow == 4 {
				srcIf = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && n.StartRow == 3 {
				dstPrefixStmt = n
			}
			if r.IsStatement(n.Type) && n.StartRow == 4 {
				dstIf = n
			}
		}
		if srcPrefixStmt == nil || dstPrefixStmt == nil || srcIf == nil || dstIf == nil {
			t.Fatalf("missing statements: srcPrefix=%v, dstPrefix=%v, srcIf=%v, dstIf=%v",
				srcPrefixStmt, dstPrefixStmt, srcIf, dstIf)
		}
		if mr.Mappings.Src()[srcPrefixStmt] != dstPrefixStmt {
			t.Errorf("expected prefix := \"badge:\" to map in place, got %v", mr.Mappings.Src()[srcPrefixStmt])
		}
		if mr.Mappings.Src()[srcIf] != dstIf {
			t.Errorf("expected if_statement to map in place, got %v", mr.Mappings.Src()[srcIf])
		}

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
			t.Fatalf("expected 2 returns in src and dst, got src=%d, dst=%d", len(srcReturns), len(dstReturns))
		}
		if mr.Mappings.Src()[srcReturns[0]] != dstReturns[0] {
			t.Errorf("expected return false to map in place, got %v", mr.Mappings.Src()[srcReturns[0]])
		}
		if mr.Mappings.Src()[srcReturns[1]] != dstReturns[1] {
			t.Errorf("expected return true to map in place, got %v", mr.Mappings.Src()[srcReturns[1]])
		}

		// 3. Target, Containers & Defect Invariant:
		// In src:
		//   bOldOuter: (color.MoveFg + badgeText) + color.Reset
		//   bOldInner: color.MoveFg + badgeText
		// In dst:
		//   bDst0: color.Dim + color.Italic (inserted)
		//   bDst1: bDst0 + color.MoveFg (inserted candidate where MoveFg crossed to RHS)
		//   bDst2: bDst1 + badgeText (stationary match for bOldInner)
		//   bDst3: bDst2 + color.Reset (stationary match for bOldOuter)
		var bOldOuter, bOldInner *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsExpression(n.Type) && len(n.Children) == 3 && r.IsOperatorLiteral(n.Children[1].Type) {
				if n.Children[2].Label == "badgeText" {
					bOldInner = n
				}
				if n.Children[2].Label == "Reset" || (len(n.Children[2].Children) > 0 && n.Children[2].Children[len(n.Children[2].Children)-1].Label == "Reset") {
					bOldOuter = n
				}
			}
		}

		var bDst0, bDst1, bDst2, bDst3 *treesitter.ASTNode
		for _, n := range dstAST.Descendants() {
			if r.IsExpression(n.Type) && len(n.Children) == 3 && r.IsOperatorLiteral(n.Children[1].Type) {
				if n.Children[0].Label == "Dim" || (len(n.Children[0].Children) > 0 && n.Children[0].Children[len(n.Children[0].Children)-1].Label == "Dim") {
					bDst0 = n
				}
				if n.Children[2].Label == "MoveFg" || (len(n.Children[2].Children) > 0 && n.Children[2].Children[len(n.Children[2].Children)-1].Label == "MoveFg") {
					bDst1 = n
				}
				if n.Children[2].Label == "badgeText" {
					bDst2 = n
				}
				if n.Children[2].Label == "Reset" || (len(n.Children[2].Children) > 0 && n.Children[2].Children[len(n.Children[2].Children)-1].Label == "Reset") {
					bDst3 = n
				}
			}
		}

		if bOldInner == nil || bOldOuter == nil || bDst0 == nil || bDst1 == nil || bDst2 == nil || bDst3 == nil {
			t.Fatalf("failed to locate binary expressions: bOldInner=%v, bOldOuter=%v, bDst0=%v, bDst1=%v, bDst2=%v, bDst3=%v",
				bOldInner, bOldOuter, bDst0, bDst1, bDst2, bDst3)
		}

		if mr.Mappings.Src()[bOldInner] != bDst2 {
			t.Errorf("expected bOldInner (MoveFg+badgeText) to map to bDst2, got %v", mr.Mappings.Src()[bOldInner])
		}
		if mr.Mappings.Src()[bOldOuter] != bDst3 {
			t.Errorf("expected bOldOuter (... + Reset) to map to bDst3, got %v", mr.Mappings.Src()[bOldOuter])
		}

		// Defect assertions: on main without the guard, uniqueTypePairs mistakenly paired bOldInner with bDst1.
		if mapped := mr.Mappings.Src()[bOldInner]; mapped == bDst1 {
			t.Errorf("bOldInner was erroneously mapped to bDst1 (cross-side operator mismatch)")
		}
		if mr.Mappings.Dst()[bDst1] != nil {
			t.Errorf("expected intermediate candidate bDst1 to remain unmapped, got %v", mr.Mappings.Dst()[bDst1])
		}
		if mr.Mappings.Dst()[bDst0] != nil {
			t.Errorf("expected newly inserted bDst0 to remain unmapped, got %v", mr.Mappings.Dst()[bDst0])
		}

		// 4. Stationary Operands & Operators:
		srcMoveFg := bOldInner.Children[0]
		srcPlusInner := bOldInner.Children[1]
		srcBadge := bOldInner.Children[2]

		dstMoveFg := bDst1.Children[2]
		dstPlusInner := bDst2.Children[1]
		dstBadge := bDst2.Children[2]

		if mr.Mappings.Src()[srcBadge] != dstBadge {
			t.Errorf("expected badgeText to map in place, got %v", mr.Mappings.Src()[srcBadge])
		}
		if mr.Mappings.Src()[srcMoveFg] != dstMoveFg {
			t.Errorf("expected MoveFg to map in place, got %v", mr.Mappings.Src()[srcMoveFg])
		}
		if mr.Mappings.Src()[srcPlusInner] != dstPlusInner {
			t.Errorf("expected '+' before badgeText to map to corresponding '+', got %v", mr.Mappings.Src()[srcPlusInner])
		}

		srcPlusOuter := bOldOuter.Children[1]
		srcReset := bOldOuter.Children[2]
		dstPlusOuter := bDst3.Children[1]
		dstReset := bDst3.Children[2]

		if mr.Mappings.Src()[srcReset] != dstReset {
			t.Errorf("expected color.Reset to map in place, got %v", mr.Mappings.Src()[srcReset])
		}
		if mr.Mappings.Src()[srcPlusOuter] != dstPlusOuter {
			t.Errorf("expected '+' before Reset to map in place, got %v", mr.Mappings.Src()[srcPlusOuter])
		}

		var srcCall, dstCall, srcArgs, dstArgs *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) {
				srcCall = n
				for _, c := range n.Children {
					if r.IsDelimitedContainer(c.Type) {
						srcArgs = c
					}
				}
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) {
				dstCall = n
				for _, c := range n.Children {
					if r.IsDelimitedContainer(c.Type) {
						dstArgs = c
					}
				}
				break
			}
		}
		if srcCall == nil || dstCall == nil || srcArgs == nil || dstArgs == nil {
			t.Fatalf("missing call or args: srcCall=%v, dstCall=%v, srcArgs=%v, dstArgs=%v",
				srcCall, dstCall, srcArgs, dstArgs)
		}
		if mr.Mappings.Src()[srcCall] != dstCall {
			t.Errorf("expected hasSuffix call to map in place, got %v", mr.Mappings.Src()[srcCall])
		}
		if mr.Mappings.Src()[srcArgs] != dstArgs {
			t.Errorf("expected argument_list to map in place, got %v", mr.Mappings.Src()[srcArgs])
		}
	})
}

func TestSiblingRecovery_TwoTier(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package engine

func TestRevokeOrphanedGlue(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		stmt1 := testutil.Node("if_statement", "")
		bin1 := testutil.Node("binary_expression", "",
			testutil.Leaf("identifier", "sText"),
			testutil.Leaf("logical_operator_literal", "||"),
			testutil.Leaf("identifier", "oldCond"),
		)
		stmt1.Children = []*treesitter.ASTNode{bin1}
		bin1.Parent = stmt1
	})
}
`
		dst := `package engine

func TestRevokeOrphanedGlue(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		stmt1 := testutil.Node("if_statement", "")
		bin1 := testutil.Node("binary_expression", "",
			testutil.NodeAt("identifier", "sText", 0, 5),
			testutil.NodeAt("logical_operator_literal", "||", 6, 8),
			testutil.NodeAt("identifier", "oldCond", 9, 16),
		)
		stmt1.Children = []*treesitter.ASTNode{bin1}
		bin1.Parent = stmt1
	})
}
`
		srcBytes := []byte(src)
		dstBytes := []byte(dst)

		srcAST, err := treesitter.Parse(srcBytes, "glue_test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "glue_test.go")
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

		// 1. Enclosing Context: Function declaration, parameters, t.Run call and closure
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Label == "TestRevokeOrphanedGlue" {
						srcFunc = n
						break
					}
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Label == "TestRevokeOrphanedGlue" {
						dstFunc = n
						break
					}
				}
			}
		}
		if srcFunc == nil || dstFunc == nil {
			t.Fatalf("missing TestRevokeOrphanedGlue: src=%v, dst=%v", srcFunc, dstFunc)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected function TestRevokeOrphanedGlue to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}

		// 2. Surrounding Sequence: stmt1 := ... and stmt1.Children = ... and bin1.Parent = stmt1
		var srcStmt1, dstStmt1, srcAssignChild, dstAssignChild, srcAssignParent, dstAssignParent *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) {
				switch n.StartRow {
				case 4:
					srcStmt1 = n
				case 10:
					srcAssignChild = n
				case 11:
					srcAssignParent = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) {
				switch n.StartRow {
				case 4:
					dstStmt1 = n
				case 10:
					dstAssignChild = n
				case 11:
					dstAssignParent = n
				}
			}
		}
		if srcStmt1 == nil || dstStmt1 == nil || srcAssignChild == nil || dstAssignChild == nil || srcAssignParent == nil || dstAssignParent == nil {
			t.Fatalf("missing surrounding statements: stmt1=(%v,%v), child=(%v,%v), parent=(%v,%v)",
				srcStmt1, dstStmt1, srcAssignChild, dstAssignChild, srcAssignParent, dstAssignParent)
		}
		if mr.Mappings.Src()[srcStmt1] != dstStmt1 {
			t.Errorf("expected stmt1 declaration to map in place, got %v", mr.Mappings.Src()[srcStmt1])
		}
		if mr.Mappings.Src()[srcAssignChild] != dstAssignChild {
			t.Errorf("expected stmt1.Children assignment to map in place, got %v", mr.Mappings.Src()[srcAssignChild])
		}
		if mr.Mappings.Src()[srcAssignParent] != dstAssignParent {
			t.Errorf("expected bin1.Parent assignment to map in place, got %v", mr.Mappings.Src()[srcAssignParent])
		}

		// 3. Target & Containers: bin1 outer call and argument_list
		var srcBinCall, dstBinCall, srcBinArgs, dstBinArgs *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) && n.StartRow == 5 {
				srcBinCall = n
				for _, c := range n.Children {
					if r.IsDelimitedContainer(c.Type) {
						srcBinArgs = c
					}
				}
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && n.StartRow == 5 {
				dstBinCall = n
				for _, c := range n.Children {
					if r.IsDelimitedContainer(c.Type) {
						dstBinArgs = c
					}
				}
				break
			}
		}
		if srcBinCall == nil || dstBinCall == nil || srcBinArgs == nil || dstBinArgs == nil {
			t.Fatalf("missing bin1 call or argument list: srcCall=%v, dstCall=%v, srcArgs=%v, dstArgs=%v",
				srcBinCall, dstBinCall, srcBinArgs, dstBinArgs)
		}
		if mr.Mappings.Src()[srcBinCall] != dstBinCall {
			t.Errorf("expected outer testutil.Node call to map in place, got %v", mr.Mappings.Src()[srcBinCall])
		}
		if mr.Mappings.Src()[srcBinArgs] != dstBinArgs {
			t.Errorf("expected outer argument_list to map in place, got %v", mr.Mappings.Src()[srcBinArgs])
		}

		// 4. Sibling Calls: testutil.Leaf(...) <-> testutil.NodeAt(...)
		var srcSiblingCalls, dstSiblingCalls []*treesitter.ASTNode
		for _, c := range srcBinArgs.Children {
			if r.IsCall(c.Type) {
				srcSiblingCalls = append(srcSiblingCalls, c)
			}
		}
		for _, c := range dstBinArgs.Children {
			if r.IsCall(c.Type) {
				dstSiblingCalls = append(dstSiblingCalls, c)
			}
		}
		if len(srcSiblingCalls) != 3 || len(dstSiblingCalls) != 3 {
			t.Fatalf("expected 3 sibling calls on both sides, got src=%d, dst=%d",
				len(srcSiblingCalls), len(dstSiblingCalls))
		}

		for i := range 3 {
			sCall := srcSiblingCalls[i]
			dCall := dstSiblingCalls[i]
			if mr.Mappings.Src()[sCall] != dCall {
				t.Errorf("expected sibling call %d to map in place, got %v, want %v",
					i, mr.Mappings.Src()[sCall], dCall)
			}
		}

		// 5. Stationary Operands: "identifier", "sText", "logical_operator_literal", "||", "oldCond"
		stationaryTokens := []string{`"binary_expression"`, `""`, `"identifier"`, `"sText"`, `"logical_operator_literal"`, `"||"`, `"oldCond"`}
		for _, tok := range stationaryTokens {
			var sNode, dNode *treesitter.ASTNode
			for _, n := range srcBinCall.Descendants() {
				if n.Label == tok {
					sNode = n
					break
				}
			}
			for _, n := range dstBinCall.Descendants() {
				if n.Label == tok {
					dNode = n
					break
				}
			}
			if sNode == nil || dNode == nil {
				t.Fatalf("token %s missing: src=%v, dst=%v", tok, sNode, dNode)
			}
			if mr.Mappings.Src()[sNode] != dNode {
				t.Errorf("expected stationary token %s to map in place, got %v", tok, mr.Mappings.Src()[sNode])
			}
		}

		// 6. Granular evolution: "Leaf" and "NodeAt" callees update, new numeric offsets inserted
		for _, offsetStr := range []string{"0", "5", "6", "8", "9", "16"} {
			var offNode *treesitter.ASTNode
			for _, n := range dstBinCall.Descendants() {
				if n.Label == offsetStr {
					offNode = n
					break
				}
			}
			if offNode == nil {
				t.Fatalf("offset token %s missing in dst", offsetStr)
			}
			if mr.Mappings.Dst()[offNode] != nil {
				t.Errorf("inserted offset token %s should be unmapped, got %v", offsetStr, mr.Mappings.Dst()[offNode])
			}
		}
	})
}

func TestRecovery_ParenthesizedConditionNoOperatorUnwrap(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `function checkFlags(context: Context, node: Node) {
    const valid = isValid(node);
    if (!(context.flags & Flags.AllowUnresolved && hasDynamicName(node) && isEntityName(node))) {
        return fallback();
    }
    return valid;
}
`
		dst := `function checkFlags(context: Context, node: Node) {
    const valid = isValid(node);
    if (!(context.flags & Flags.AllowUnresolved && isEntityName(node))) {
        return fallback();
    }
    return valid;
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.ts")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags function
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "checkFlags" {
				srcFunc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "checkFlags" {
				dstFunc = n
				break
			}
		}
		if srcFunc == nil || dstFunc == nil || mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Fatalf("expected function checkFlags to map in place, got src=%v dst=%v", srcFunc, dstFunc)
		}

		// 2. Surrounding Sequence: const valid and return valid
		var srcValid, dstValid *treesitter.ASTNode
		var srcReturn, dstReturn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsLocalVarDeclaration(n.Type) {
				srcValid = n
			}
			if r.IsJumpStatement(n.Type) && n.StartRow == 5 {
				srcReturn = n
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsLocalVarDeclaration(n.Type) {
				dstValid = n
			}
			if r.IsJumpStatement(n.Type) && n.StartRow == 5 {
				dstReturn = n
			}
		}
		if srcValid == nil || dstValid == nil || mr.Mappings.Src()[srcValid] != dstValid {
			t.Errorf("expected preceding const valid statement to map in place")
		}
		if srcReturn == nil || dstReturn == nil || mr.Mappings.Src()[srcReturn] != dstReturn {
			t.Errorf("expected succeeding return valid statement to map in place")
		}

		// 3. Target & Containers: if statement, unary ! and parenthesized_expression
		var srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				srcIf = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				dstIf = n
				break
			}
		}
		if srcIf == nil || dstIf == nil || mr.Mappings.Src()[srcIf] != dstIf {
			t.Fatalf("expected if_statement to map in place, got src=%v dst=%v", srcIf, dstIf)
		}

		var srcParen, dstParen *treesitter.ASTNode
		var srcUnary, dstUnary *treesitter.ASTNode
		for _, n := range srcIf.PreOrder() {
			if n.Type == "parenthesized_expression" {
				srcParen = n
			}
			if n.Type == "unary_expression" {
				srcUnary = n
			}
		}
		for _, n := range dstIf.PreOrder() {
			if n.Type == "parenthesized_expression" {
				dstParen = n
			}
			if n.Type == "unary_expression" {
				dstUnary = n
			}
		}
		if srcUnary == nil || dstUnary == nil || mr.Mappings.Src()[srcUnary] != dstUnary {
			t.Errorf("expected unary expression ! to map in place")
		}
		if srcParen == nil || dstParen == nil || mr.Mappings.Src()[srcParen] != dstParen {
			t.Errorf("expected parenthesized_expression to map in place")
		}

		// 4. Stationary Operands: context.flags & Flags.AllowUnresolved and isEntityName(node)
		var srcEntityCall, dstEntityCall *treesitter.ASTNode
		var srcDynCall *treesitter.ASTNode
		for _, n := range srcIf.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isEntityName" {
				srcEntityCall = n
			}
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "hasDynamicName" {
				srcDynCall = n
			}
		}
		for _, n := range dstIf.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isEntityName" {
				dstEntityCall = n
			}
		}
		if srcEntityCall == nil || dstEntityCall == nil || mr.Mappings.Src()[srcEntityCall] != dstEntityCall {
			t.Errorf("expected call isEntityName to map in place")
		}

		// Deleted middle term hasDynamicName must be unmapped in dst
		if srcDynCall == nil || mr.Mappings.Src()[srcDynCall] != nil {
			t.Errorf("expected deleted call hasDynamicName to be unmapped in dst, got %v", mr.Mappings.Src()[srcDynCall])
		}
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		src := `class FlagsChecker {
    boolean checkFlags(Context context, Node node) {
        boolean valid = isValid(node);
        if (!((context.flags & Flags.AllowUnresolved) != 0 && hasDynamicName(node) && isEntityName(node))) {
            return fallback();
        }
        return valid;
    }
}
`
		dst := `class FlagsChecker {
    boolean checkFlags(Context context, Node node) {
        boolean valid = isValid(node);
        if (!((context.flags & Flags.AllowUnresolved) != 0 && isEntityName(node))) {
            return fallback();
        }
        return valid;
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags method
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "checkFlags" {
				srcFunc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "checkFlags" {
				dstFunc = n
				break
			}
		}
		if srcFunc == nil || dstFunc == nil || mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Fatalf("expected method checkFlags to map in place, got src=%v dst=%v", srcFunc, dstFunc)
		}

		// 2. Surrounding Sequence: boolean valid and return valid
		var srcValid, dstValid *treesitter.ASTNode
		var srcReturn, dstReturn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsLocalVarDeclaration(n.Type) {
				srcValid = n
			}
			if r.IsJumpStatement(n.Type) && n.StartRow == 6 {
				srcReturn = n
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsLocalVarDeclaration(n.Type) {
				dstValid = n
			}
			if r.IsJumpStatement(n.Type) && n.StartRow == 6 {
				dstReturn = n
			}
		}
		if srcValid == nil || dstValid == nil || mr.Mappings.Src()[srcValid] != dstValid {
			t.Errorf("expected preceding boolean valid statement to map in place")
		}
		if srcReturn == nil || dstReturn == nil || mr.Mappings.Src()[srcReturn] != dstReturn {
			t.Errorf("expected succeeding return valid statement to map in place")
		}

		// 3. Target: if statement and outer parenthesized condition
		var srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				srcIf = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsStatement(n.Type) && n.Type == "if_statement" {
				dstIf = n
				break
			}
		}
		if srcIf == nil || dstIf == nil || mr.Mappings.Src()[srcIf] != dstIf {
			t.Fatalf("expected if_statement to map in place, got src=%v dst=%v", srcIf, dstIf)
		}

		var srcParen, dstParen *treesitter.ASTNode
		for _, n := range srcIf.Children {
			if n.Type == "parenthesized_expression" {
				srcParen = n
				break
			}
		}
		for _, n := range dstIf.Children {
			if n.Type == "parenthesized_expression" {
				dstParen = n
				break
			}
		}
		if srcParen == nil || dstParen == nil || mr.Mappings.Src()[srcParen] != dstParen {
			t.Errorf("expected outer parenthesized_expression to map in place")
		}

		// 4. Stationary Operands: call isEntityName
		var srcEntityCall, dstEntityCall *treesitter.ASTNode
		var srcDynCall *treesitter.ASTNode
		for _, n := range srcIf.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isEntityName" {
				srcEntityCall = n
			}
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "hasDynamicName" {
				srcDynCall = n
			}
		}
		for _, n := range dstIf.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isEntityName" {
				dstEntityCall = n
			}
		}
		if srcEntityCall == nil || dstEntityCall == nil || mr.Mappings.Src()[srcEntityCall] != dstEntityCall {
			t.Errorf("expected call isEntityName to map in place")
		}
		if srcDynCall == nil || mr.Mappings.Src()[srcDynCall] != nil {
			t.Errorf("expected deleted call hasDynamicName to be unmapped in dst, got %v", mr.Mappings.Src()[srcDynCall])
		}
	})
}

func TestStructFieldReplacement_NoFalseMove(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `struct Config {
    pub hyperlinks: bool,
    pub foo: Style,
    pub line_numbers_plus_style: Style,
    pub line_numbers_zero_style: Style,
}
`
		dst := `struct Config {
    pub hyperlinks: bool,
    pub foo: String,
    pub line_numbers_zero_style: Style,
    pub inline_hint_style: Style,
}
`

		srcAST, err := treesitter.Parse([]byte(src), "config.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "config.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: struct Config maps in place.
		var srcStruct, dstStruct *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Config" {
				srcStruct = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Config" {
				dstStruct = n
				break
			}
		}
		if srcStruct == nil || dstStruct == nil || mr.Mappings.Src()[srcStruct] != dstStruct {
			t.Fatalf("expected struct Config to map in place, got src=%v dst=%v", srcStruct, dstStruct)
		}

		// 2. Surrounding Sequence: hyperlinks and line_numbers_zero_style map in place.
		var srcHyper, dstHyper *treesitter.ASTNode
		var srcZero, dstZero *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				switch getDeclarationName(n) {
				case "hyperlinks":
					srcHyper = n
				case "line_numbers_zero_style":
					srcZero = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				switch getDeclarationName(n) {
				case "hyperlinks":
					dstHyper = n
				case "line_numbers_zero_style":
					dstZero = n
				}
			}
		}
		if srcHyper == nil || dstHyper == nil || mr.Mappings.Src()[srcHyper] != dstHyper {
			t.Errorf("expected hyperlinks field to map in place")
		}
		if srcZero == nil || dstZero == nil || mr.Mappings.Src()[srcZero] != dstZero {
			t.Errorf("expected line_numbers_zero_style field to map in place")
		}

		// 3. Target: Deleted line_numbers_plus_style must NOT match inserted inline_hint_style.
		var srcPlus, dstInline *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "line_numbers_plus_style" {
				srcPlus = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "inline_hint_style" {
				dstInline = n
				break
			}
		}
		if srcPlus == nil {
			t.Fatal("source line_numbers_plus_style field not found")
		}
		if dstInline == nil {
			t.Fatal("destination inline_hint_style field not found")
		}
		if mapped := mr.Mappings.Src()[srcPlus]; mapped != nil {
			t.Errorf("expected line_numbers_plus_style to be unmapped, got mapped to %s %q",
				mapped.Type, getDeclarationName(mapped))
		}
		if mapped := mr.Mappings.Dst()[dstInline]; mapped != nil {
			t.Errorf("expected inline_hint_style to be unmapped in src, got mapped from %s %q",
				mapped.Type, getDeclarationName(mapped))
		}

		// 4. Same name with different type: pub foo: Style -> pub foo: String should map.
		var srcFoo, dstFoo *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "foo" {
				srcFoo = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "foo" {
				dstFoo = n
				break
			}
		}
		if srcFoo == nil || dstFoo == nil || mr.Mappings.Src()[srcFoo] != dstFoo {
			t.Errorf("expected field foo to map across type update")
		}
	})
}

func TestImportReplacement_NoFalseMove(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `use std::collections::HashMap;
use std::process;
use ansi_term::Color;
`
		dst := `use std::collections::HashMap;
use crate::fatal;
use ansi_term::Color;
`

		srcAST, err := treesitter.Parse([]byte(src), "color.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "color.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Surrounding sequence: HashMap and Color must map in place.
		var srcHashMap, dstHashMap *treesitter.ASTNode
		var srcColor, dstColor *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				txt := string([]byte(src)[n.StartByte:n.EndByte])
				if strings.Contains(txt, "HashMap") {
					srcHashMap = n
				}
				if strings.Contains(txt, "Color") {
					srcColor = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				txt := string([]byte(dst)[n.StartByte:n.EndByte])
				if strings.Contains(txt, "HashMap") {
					dstHashMap = n
				}
				if strings.Contains(txt, "Color") {
					dstColor = n
				}
			}
		}
		if srcHashMap == nil || dstHashMap == nil || mr.Mappings.Src()[srcHashMap] != dstHashMap {
			t.Errorf("expected use std::collections::HashMap to map in place")
		}
		if srcColor == nil || dstColor == nil || mr.Mappings.Src()[srcColor] != dstColor {
			t.Errorf("expected use ansi_term::Color to map in place")
		}

		// 2. Target: Deleted use std::process must NOT match inserted use crate::fatal.
		var srcProcess, dstFatal *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && strings.Contains(string([]byte(src)[n.StartByte:n.EndByte]), "process") {
				srcProcess = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && strings.Contains(string([]byte(dst)[n.StartByte:n.EndByte]), "fatal") {
				dstFatal = n
				break
			}
		}
		if srcProcess == nil {
			t.Fatal("source use std::process not found")
		}
		if dstFatal == nil {
			t.Fatal("destination use crate::fatal not found")
		}
		if mapped := mr.Mappings.Src()[srcProcess]; mapped != nil {
			t.Errorf("expected use std::process to be unmapped, got mapped to %s (%s)",
				mapped.Type, string([]byte(dst)[mapped.StartByte:mapped.EndByte]))
		}
		if mapped := mr.Mappings.Dst()[dstFatal]; mapped != nil {
			t.Errorf("expected use crate::fatal to be unmapped in src, got mapped from %s (%s)",
				mapped.Type, string([]byte(src)[mapped.StartByte:mapped.EndByte]))
		}
	})
}

func TestGoStructFieldReplacement_NoFalseMove(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

type Config struct {
	Hyperlinks bool
	Foo        Style
	OldField   Style
	Numbers    bool
}
`
		dst := `package main

type Config struct {
	Hyperlinks bool
	Foo        string
	Numbers    bool
	NewField   Style
}
`

		srcAST, err := treesitter.Parse([]byte(src), "config.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "config.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: type Config struct maps in place.
		var srcConfig, dstConfig *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Config" {
				srcConfig = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Config" {
				dstConfig = n
				break
			}
		}
		if srcConfig == nil || dstConfig == nil || mr.Mappings.Src()[srcConfig] != dstConfig {
			t.Fatalf("expected type Config to map in place")
		}

		// 2. Surrounding sequence: Hyperlinks and Numbers map in place.
		var srcHyper, dstHyper *treesitter.ASTNode
		var srcNumbers, dstNumbers *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				switch getDeclarationName(n) {
				case "Hyperlinks":
					srcHyper = n
				case "Numbers":
					srcNumbers = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				switch getDeclarationName(n) {
				case "Hyperlinks":
					dstHyper = n
				case "Numbers":
					dstNumbers = n
				}
			}
		}
		if srcHyper == nil || dstHyper == nil || mr.Mappings.Src()[srcHyper] != dstHyper {
			t.Errorf("expected Hyperlinks field to map in place")
		}
		if srcNumbers == nil || dstNumbers == nil || mr.Mappings.Src()[srcNumbers] != dstNumbers {
			t.Errorf("expected Numbers field to map in place")
		}

		// 3. Target: Deleted OldField must NOT match inserted NewField.
		var srcOld, dstNew *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "OldField" {
				srcOld = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "NewField" {
				dstNew = n
				break
			}
		}
		if srcOld == nil {
			t.Fatal("source OldField not found")
		}
		if dstNew == nil {
			t.Fatal("destination NewField not found")
		}
		if mapped := mr.Mappings.Src()[srcOld]; mapped != nil {
			t.Errorf("expected OldField to be unmapped, got mapped to %s %q",
				mapped.Type, getDeclarationName(mapped))
		}
		if mapped := mr.Mappings.Dst()[dstNew]; mapped != nil {
			t.Errorf("expected NewField to be unmapped in src, got mapped from %s %q",
				mapped.Type, getDeclarationName(mapped))
		}

		// 4. Same name with different type: Foo Style -> Foo string maps in place.
		var srcFoo, dstFoo *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Foo" {
				srcFoo = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "Foo" {
				dstFoo = n
				break
			}
		}
		if srcFoo == nil || dstFoo == nil || mr.Mappings.Src()[srcFoo] != dstFoo {
			t.Errorf("expected field Foo to map across type update")
		}
	})
}

func TestColorImportsRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `use std::collections::HashMap;
use std::process;
use std::str::FromStr;

use ansi_term::Color;
use lazy_static::lazy_static;
use syntect::highlighting::Color as SyntectColor;

use crate::bat_utils::terminal::to_ansi_color;
use crate::syntect_color;

pub fn parse_color() {}
`
		newSrc := `use std::collections::HashMap;
use std::str::FromStr;

use ansi_term::Color;
use lazy_static::lazy_static;
use syntect::highlighting::Color as SyntectColor;

use crate::bat_utils::terminal::to_ansi_color;
use crate::fatal;
use crate::syntect_utils;

pub fn parse_color() {}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		r := rulesFor(srcAST)
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), nil)

		var srcSyn, dstSyn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && strings.Contains(getDeclarationName(n), "syntect_color") {
				srcSyn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && strings.Contains(getDeclarationName(n), "syntect_utils") {
				dstSyn = n
				break
			}
		}
		if srcSyn == nil || dstSyn == nil {
			t.Fatalf("syn nodes not found: src=%v, dst=%v", srcSyn, dstSyn)
		}
		if mr.Mappings.Src()[srcSyn] != dstSyn {
			t.Errorf("expected syntect_color to map to syntect_utils, got %v", mr.Mappings.Src()[srcSyn])
		}
	})
}

func TestDelimitedPairKeyAffinity_NoDisjointPairTheft(t *testing.T) {
	t.Run("FullEngine_Rust", func(t *testing.T) {
		oldSrc := `fn make_config(opt: Opt) -> Config {
    let _flag = true;
    Config {
        commit_style,
        color_only: opt.color_only,
        line_numbers_left_format: opt.line_numbers_left_format,
        line_numbers_left_style,
        line_numbers_minus_style,
        line_numbers_plus_style,
        line_numbers_right_format: opt.line_numbers_right_format,
        line_numbers_right_style,
        line_numbers_zero_style,
        line_buffer_size: opt.line_buffer_size,
    }
}
`
		newSrc := `fn make_config(opt: Opt) -> Config {
    let _flag = true;
    Config {
        commit_style,
        color_only: opt.color_only,
        default_language: opt.default_language,
        hunk_label,
        inline_hint_style,
        line_numbers_format: LeftRight::new(
            opt.line_numbers_left_format,
            opt.line_numbers_right_format,
        ),
        line_numbers_style_leftright: LeftRight::new(
            line_numbers_left_style,
            line_numbers_right_style,
        ),
        line_numbers_style_minusplus: MinusPlus::new(
            line_numbers_minus_style,
            line_numbers_plus_style,
        ),
        line_numbers_zero_style,
        line_buffer_size: opt.line_buffer_size,
    }
}
`

		srcAST, err := treesitter.Parse([]byte(oldSrc), "config.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "config.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: fn make_config maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "make_config" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "make_config" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected fn make_config to map in place")
		}

		// 2. Surrounding sequence: commit_style, color_only, line_numbers_zero_style, line_buffer_size map in place.
		findPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key {
					return n
				}
			}
			return nil
		}

		for _, stableKey := range []string{"commit_style", "color_only", "line_numbers_zero_style", "line_buffer_size"} {
			srcNode := findPairByKey(srcAST, stableKey)
			dstNode := findPairByKey(dstAST, stableKey)
			if srcNode == nil || dstNode == nil {
				t.Fatalf("stable pair %q not found: src=%v, dst=%v", stableKey, srcNode, dstNode)
			}
			if mr.Mappings.Src()[srcNode] != dstNode {
				t.Errorf("expected stable pair %q to map in place, got %v", stableKey, mr.Mappings.Src()[srcNode])
			}
		}

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		for _, addedKey := range []string{"default_language", "hunk_label", "inline_hint_style"} {
			dstNode := findPairByKey(dstAST, addedKey)
			if dstNode == nil {
				t.Fatalf("added pair %q not found in dst", addedKey)
			}
			if mr.Mappings.Dst()[dstNode] != nil {
				t.Errorf("expected added pair %q to remain unmapped, got mapped from %v (%s)",
					addedKey, mr.Mappings.Dst()[dstNode], getKeyLabel(mr.Mappings.Dst()[dstNode]))
			}
		}

		// 4. Evolving line numbers pairs:
		// line_numbers_left_format -> line_numbers_format
		// line_numbers_left_style -> line_numbers_style_leftright
		// line_numbers_minus_style -> line_numbers_style_minusplus
		leftFormatSrc := findPairByKey(srcAST, "line_numbers_left_format")
		formatDst := findPairByKey(dstAST, "line_numbers_format")
		if leftFormatSrc == nil || formatDst == nil || mr.Mappings.Src()[leftFormatSrc] != formatDst {
			t.Errorf("expected line_numbers_left_format to map to line_numbers_format, got %v", mr.Mappings.Src()[leftFormatSrc])
		}

		leftStyleSrc := findPairByKey(srcAST, "line_numbers_left_style")
		styleLeftRightDst := findPairByKey(dstAST, "line_numbers_style_leftright")
		if leftStyleSrc == nil || styleLeftRightDst == nil || mr.Mappings.Src()[leftStyleSrc] != styleLeftRightDst {
			t.Errorf("expected line_numbers_left_style to map to line_numbers_style_leftright, got %v", mr.Mappings.Src()[leftStyleSrc])
		}

		minusStyleSrc := findPairByKey(srcAST, "line_numbers_minus_style")
		styleMinusPlusDst := findPairByKey(dstAST, "line_numbers_style_minusplus")
		if minusStyleSrc == nil || styleMinusPlusDst == nil || mr.Mappings.Src()[minusStyleSrc] != styleMinusPlusDst {
			t.Errorf("expected line_numbers_minus_style to map to line_numbers_style_minusplus, got %v", mr.Mappings.Src()[minusStyleSrc])
		}

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		for _, delKey := range []string{"line_numbers_plus_style", "line_numbers_right_format", "line_numbers_right_style"} {
			srcNode := findPairByKey(srcAST, delKey)
			if srcNode == nil {
				t.Fatalf("deleted pair %q not found in src", delKey)
			}
			if mr.Mappings.Src()[srcNode] != nil {
				t.Errorf("expected deleted pair %q to remain unmapped, got mapped to %v (%s)",
					delKey, mr.Mappings.Src()[srcNode], getKeyLabel(mr.Mappings.Src()[srcNode]))
			}
		}
	})

	t.Run("FullEngine_JavaScript", func(t *testing.T) {
		oldSrc := `function buildConfig(opt) {
    const { lineNumbers, sideBySide } = opt;
    return {
        commitStyle,
        colorOnly: opt.colorOnly,
        lineNumbersLeftFormat: opt.lineNumbersLeftFormat,
        lineNumbersLeftStyle,
        lineNumbersMinusStyle,
        lineNumbersPlusStyle,
        lineNumbersRightFormat: opt.lineNumbersRightFormat,
        lineNumbersRightStyle,
        lineNumbersZeroStyle,
        lineBufferSize: opt.lineBufferSize,
    };
}
`
		newSrc := `function buildConfig(opt) {
    const { lineNumbers, sideBySide } = opt;
    return {
        commitStyle,
        colorOnly: opt.colorOnly,
        defaultLanguage: opt.defaultLanguage,
        hunkLabel,
        inlineHintStyle,
        lineNumbersFormat: formatLeftRight(
            opt.lineNumbersLeftFormat,
            opt.lineNumbersRightFormat,
        ),
        lineNumbersStyleLeftRight: styleLeftRight(
            lineNumbersLeftStyle,
            lineNumbersRightStyle,
        ),
        lineNumbersStyleMinusPlus: styleMinusPlus(
            lineNumbersMinusStyle,
            lineNumbersPlusStyle,
        ),
        lineNumbersZeroStyle,
        lineBufferSize: opt.lineBufferSize,
    };
}
`

		srcAST, err := treesitter.Parse([]byte(oldSrc), "config.js")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "config.js")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function buildConfig maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildConfig" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildConfig" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected function buildConfig to map in place")
		}

		// 2. Destructuring pattern: lineNumbers and sideBySide map in place.
		findPatternPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object_pattern" {
					return n
				}
			}
			return nil
		}
		for _, key := range []string{"lineNumbers", "sideBySide"} {
			sNode := findPatternPairByKey(srcAST, key)
			dNode := findPatternPairByKey(dstAST, key)
			if sNode == nil || dNode == nil || mr.Mappings.Src()[sNode] != dNode {
				t.Errorf("expected pattern pair %q to map in place, got %v", key, mr.Mappings.Src()[sNode])
			}
		}

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		findPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object" {
					return n
				}
			}
			return nil
		}

		for _, stableKey := range []string{"commitStyle", "colorOnly", "lineNumbersZeroStyle", "lineBufferSize"} {
			srcNode := findPairByKey(srcAST, stableKey)
			dstNode := findPairByKey(dstAST, stableKey)
			if srcNode == nil || dstNode == nil {
				t.Fatalf("stable pair %q not found: src=%v, dst=%v", stableKey, srcNode, dstNode)
			}
			if mr.Mappings.Src()[srcNode] != dstNode {
				t.Errorf("expected stable pair %q to map in place, got %v", stableKey, mr.Mappings.Src()[srcNode])
			}
		}

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		for _, addedKey := range []string{"defaultLanguage", "hunkLabel", "inlineHintStyle"} {
			dstNode := findPairByKey(dstAST, addedKey)
			if dstNode == nil {
				t.Fatalf("added pair %q not found in dst", addedKey)
			}
			if mr.Mappings.Dst()[dstNode] != nil {
				t.Errorf("expected added pair %q to remain unmapped, got mapped from %v (%s)",
					addedKey, mr.Mappings.Dst()[dstNode], getKeyLabel(mr.Mappings.Dst()[dstNode]))
			}
		}

		// 5. Evolving line numbers pairs:
		leftFormatSrc := findPairByKey(srcAST, "lineNumbersLeftFormat")
		formatDst := findPairByKey(dstAST, "lineNumbersFormat")
		if leftFormatSrc == nil || formatDst == nil || mr.Mappings.Src()[leftFormatSrc] != formatDst {
			t.Errorf("expected lineNumbersLeftFormat to map to lineNumbersFormat, got %v", mr.Mappings.Src()[leftFormatSrc])
		}

		leftStyleSrc := findPairByKey(srcAST, "lineNumbersLeftStyle")
		styleLeftRightDst := findPairByKey(dstAST, "lineNumbersStyleLeftRight")
		if leftStyleSrc == nil || styleLeftRightDst == nil || mr.Mappings.Src()[leftStyleSrc] != styleLeftRightDst {
			t.Errorf("expected lineNumbersLeftStyle to map to lineNumbersStyleLeftRight, got %v", mr.Mappings.Src()[leftStyleSrc])
		}

		minusStyleSrc := findPairByKey(srcAST, "lineNumbersMinusStyle")
		styleMinusPlusDst := findPairByKey(dstAST, "lineNumbersStyleMinusPlus")
		if minusStyleSrc == nil || styleMinusPlusDst == nil || mr.Mappings.Src()[minusStyleSrc] != styleMinusPlusDst {
			t.Errorf("expected lineNumbersMinusStyle to map to lineNumbersStyleMinusPlus, got %v", mr.Mappings.Src()[minusStyleSrc])
		}

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		for _, delKey := range []string{"lineNumbersPlusStyle", "lineNumbersRightFormat", "lineNumbersRightStyle"} {
			srcNode := findPairByKey(srcAST, delKey)
			if srcNode == nil {
				t.Fatalf("deleted pair %q not found in src", delKey)
			}
			if mr.Mappings.Src()[srcNode] != nil {
				t.Errorf("expected deleted pair %q to remain unmapped, got mapped to %v (%s)",
					delKey, mr.Mappings.Src()[srcNode], getKeyLabel(mr.Mappings.Src()[srcNode]))
			}
		}
	})

	t.Run("FullEngine_TypeScript", func(t *testing.T) {
		oldSrc := `interface Opts {
    colorOnly: boolean;
    lineBufferSize: number;
}

function configure(opt: Opts) {
    const { flag, mode } = opt;
    return {
        commitStyle,
        colorOnly: opt.colorOnly,
        lineNumbersLeftFormat: opt.lineNumbersLeftFormat,
        lineNumbersLeftStyle,
        lineNumbersMinusStyle,
        lineNumbersPlusStyle,
        lineNumbersRightFormat: opt.lineNumbersRightFormat,
        lineNumbersRightStyle,
        lineNumbersZeroStyle,
        lineBufferSize: opt.lineBufferSize,
    };
}
`
		newSrc := `interface Opts {
    colorOnly: boolean;
    lineBufferSize: number;
}

function configure(opt: Opts) {
    const { flag, mode } = opt;
    return {
        commitStyle,
        colorOnly: opt.colorOnly,
        defaultLanguage: opt.defaultLanguage,
        hunkLabel,
        inlineHintStyle,
        lineNumbersFormat: formatLeftRight(
            opt.lineNumbersLeftFormat,
            opt.lineNumbersRightFormat,
        ),
        lineNumbersStyleLeftRight: styleLeftRight(
            lineNumbersLeftStyle,
            lineNumbersRightStyle,
        ),
        lineNumbersStyleMinusPlus: styleMinusPlus(
            lineNumbersMinusStyle,
            lineNumbersPlusStyle,
        ),
        lineNumbersZeroStyle,
        lineBufferSize: opt.lineBufferSize,
    };
}
`

		srcAST, err := treesitter.Parse([]byte(oldSrc), "config.ts")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "config.ts")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function configure maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "configure" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "configure" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected function configure to map in place")
		}

		// 2. Destructuring pattern: flag and mode map in place.
		findPatternPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object_pattern" {
					return n
				}
			}
			return nil
		}
		for _, key := range []string{"flag", "mode"} {
			sNode := findPatternPairByKey(srcAST, key)
			dNode := findPatternPairByKey(dstAST, key)
			if sNode == nil || dNode == nil || mr.Mappings.Src()[sNode] != dNode {
				t.Errorf("expected pattern pair %q to map in place, got %v", key, mr.Mappings.Src()[sNode])
			}
		}

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		findPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object" {
					return n
				}
			}
			return nil
		}

		for _, stableKey := range []string{"commitStyle", "colorOnly", "lineNumbersZeroStyle", "lineBufferSize"} {
			srcNode := findPairByKey(srcAST, stableKey)
			dstNode := findPairByKey(dstAST, stableKey)
			if srcNode == nil || dstNode == nil {
				t.Fatalf("stable pair %q not found: src=%v, dst=%v", stableKey, srcNode, dstNode)
			}
			if mr.Mappings.Src()[srcNode] != dstNode {
				t.Errorf("expected stable pair %q to map in place, got %v", stableKey, mr.Mappings.Src()[srcNode])
			}
		}

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		for _, addedKey := range []string{"defaultLanguage", "hunkLabel", "inlineHintStyle"} {
			dstNode := findPairByKey(dstAST, addedKey)
			if dstNode == nil {
				t.Fatalf("added pair %q not found in dst", addedKey)
			}
			if mr.Mappings.Dst()[dstNode] != nil {
				t.Errorf("expected added pair %q to remain unmapped, got mapped from %v (%s)",
					addedKey, mr.Mappings.Dst()[dstNode], getKeyLabel(mr.Mappings.Dst()[dstNode]))
			}
		}

		// 5. Evolving line numbers pairs:
		leftFormatSrc := findPairByKey(srcAST, "lineNumbersLeftFormat")
		formatDst := findPairByKey(dstAST, "lineNumbersFormat")
		if leftFormatSrc == nil || formatDst == nil || mr.Mappings.Src()[leftFormatSrc] != formatDst {
			t.Errorf("expected lineNumbersLeftFormat to map to lineNumbersFormat, got %v", mr.Mappings.Src()[leftFormatSrc])
		}

		leftStyleSrc := findPairByKey(srcAST, "lineNumbersLeftStyle")
		styleLeftRightDst := findPairByKey(dstAST, "lineNumbersStyleLeftRight")
		if leftStyleSrc == nil || styleLeftRightDst == nil || mr.Mappings.Src()[leftStyleSrc] != styleLeftRightDst {
			t.Errorf("expected lineNumbersLeftStyle to map to lineNumbersStyleLeftRight, got %v", mr.Mappings.Src()[leftStyleSrc])
		}

		minusStyleSrc := findPairByKey(srcAST, "lineNumbersMinusStyle")
		styleMinusPlusDst := findPairByKey(dstAST, "lineNumbersStyleMinusPlus")
		if minusStyleSrc == nil || styleMinusPlusDst == nil || mr.Mappings.Src()[minusStyleSrc] != styleMinusPlusDst {
			t.Errorf("expected lineNumbersMinusStyle to map to lineNumbersStyleMinusPlus, got %v", mr.Mappings.Src()[minusStyleSrc])
		}

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		for _, delKey := range []string{"lineNumbersPlusStyle", "lineNumbersRightFormat", "lineNumbersRightStyle"} {
			srcNode := findPairByKey(srcAST, delKey)
			if srcNode == nil {
				t.Fatalf("deleted pair %q not found in src", delKey)
			}
			if mr.Mappings.Src()[srcNode] != nil {
				t.Errorf("expected deleted pair %q to remain unmapped, got mapped to %v (%s)",
					delKey, mr.Mappings.Src()[srcNode], getKeyLabel(mr.Mappings.Src()[srcNode]))
			}
		}
	})

	t.Run("FullEngine_TSX", func(t *testing.T) {
		oldSrc := `export function ConfigPanel(props: ConfigProps) {
    const { visible, theme } = props;
    const options = {
        commitStyle,
        colorOnly: props.colorOnly,
        lineNumbersLeftFormat: props.lineNumbersLeftFormat,
        lineNumbersLeftStyle,
        lineNumbersMinusStyle,
        lineNumbersPlusStyle,
        lineNumbersZeroStyle,
        lineBufferSize: props.lineBufferSize,
    };
    return <div data-options={options} />;
}
`
		newSrc := `export function ConfigPanel(props: ConfigProps) {
    const { visible, theme } = props;
    const options = {
        commitStyle,
        colorOnly: props.colorOnly,
        defaultLanguage: props.defaultLanguage,
        hunkLabel,
        inlineHintStyle,
        lineNumbersFormat: formatLeftRight(
            props.lineNumbersLeftFormat,
        ),
        lineNumbersStyleLeftRight: styleLeftRight(
            lineNumbersLeftStyle,
        ),
        lineNumbersStyleMinusPlus: styleMinusPlus(
            lineNumbersMinusStyle,
        ),
        lineNumbersZeroStyle,
        lineBufferSize: props.lineBufferSize,
    };
    return <div data-options={options} />;
}
`

		srcAST, err := treesitter.Parse([]byte(oldSrc), "component.tsx")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "component.tsx")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function ConfigPanel maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "ConfigPanel" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "ConfigPanel" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected function ConfigPanel to map in place")
		}

		// 2. Destructuring pattern: visible and theme map in place.
		findPatternPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object_pattern" {
					return n
				}
			}
			return nil
		}
		for _, key := range []string{"visible", "theme"} {
			sNode := findPatternPairByKey(srcAST, key)
			dNode := findPatternPairByKey(dstAST, key)
			if sNode == nil || dNode == nil || mr.Mappings.Src()[sNode] != dNode {
				t.Errorf("expected pattern pair %q to map in place, got %v", key, mr.Mappings.Src()[sNode])
			}
		}

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		findPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key && n.Parent != nil && n.Parent.Type == "object" {
					return n
				}
			}
			return nil
		}

		for _, stableKey := range []string{"commitStyle", "colorOnly", "lineNumbersZeroStyle", "lineBufferSize"} {
			srcNode := findPairByKey(srcAST, stableKey)
			dstNode := findPairByKey(dstAST, stableKey)
			if srcNode == nil || dstNode == nil {
				t.Fatalf("stable pair %q not found: src=%v, dst=%v", stableKey, srcNode, dstNode)
			}
			if mr.Mappings.Src()[srcNode] != dstNode {
				t.Errorf("expected stable pair %q to map in place, got %v", stableKey, mr.Mappings.Src()[srcNode])
			}
		}

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		for _, addedKey := range []string{"defaultLanguage", "hunkLabel", "inlineHintStyle"} {
			dstNode := findPairByKey(dstAST, addedKey)
			if dstNode == nil {
				t.Fatalf("added pair %q not found in dst", addedKey)
			}
			if mr.Mappings.Dst()[dstNode] != nil {
				t.Errorf("expected added pair %q to remain unmapped, got mapped from %v (%s)",
					addedKey, mr.Mappings.Dst()[dstNode], getKeyLabel(mr.Mappings.Dst()[dstNode]))
			}
		}

		// 5. Evolving line numbers pairs:
		leftFormatSrc := findPairByKey(srcAST, "lineNumbersLeftFormat")
		formatDst := findPairByKey(dstAST, "lineNumbersFormat")
		if leftFormatSrc == nil || formatDst == nil || mr.Mappings.Src()[leftFormatSrc] != formatDst {
			t.Errorf("expected lineNumbersLeftFormat to map to lineNumbersFormat, got %v", mr.Mappings.Src()[leftFormatSrc])
		}

		leftStyleSrc := findPairByKey(srcAST, "lineNumbersLeftStyle")
		styleLeftRightDst := findPairByKey(dstAST, "lineNumbersStyleLeftRight")
		if leftStyleSrc == nil || styleLeftRightDst == nil || mr.Mappings.Src()[leftStyleSrc] != styleLeftRightDst {
			t.Errorf("expected lineNumbersLeftStyle to map to lineNumbersStyleLeftRight, got %v", mr.Mappings.Src()[leftStyleSrc])
		}

		minusStyleSrc := findPairByKey(srcAST, "lineNumbersMinusStyle")
		styleMinusPlusDst := findPairByKey(dstAST, "lineNumbersStyleMinusPlus")
		if minusStyleSrc == nil || styleMinusPlusDst == nil || mr.Mappings.Src()[minusStyleSrc] != styleMinusPlusDst {
			t.Errorf("expected lineNumbersMinusStyle to map to lineNumbersStyleMinusPlus, got %v", mr.Mappings.Src()[minusStyleSrc])
		}

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		for _, delKey := range []string{"lineNumbersPlusStyle"} {
			srcNode := findPairByKey(srcAST, delKey)
			if srcNode == nil {
				t.Fatalf("deleted pair %q not found in src", delKey)
			}
			if mr.Mappings.Src()[srcNode] != nil {
				t.Errorf("expected deleted pair %q to remain unmapped, got mapped to %v (%s)",
					delKey, mr.Mappings.Src()[srcNode], getKeyLabel(mr.Mappings.Src()[srcNode]))
			}
		}
	})

	t.Run("FullEngine_Java_Annotation", func(t *testing.T) {
		oldSrc := `class ConfigService {
    @ConfigMapping(
        prefix = "app.config",
        lineNumbersLeftFormat = "left",
        lineNumbersLeftStyle = "style",
        lineNumbersZeroStyle = "zero",
        lineBufferSize = "1024"
    )
    public void configure() {}
}
`
		newSrc := `class ConfigService {
    @ConfigMapping(
        prefix = "app.config",
        defaultLanguage = "rust",
        hunkLabel = "label",
        lineNumbersFormat = "leftRight",
        lineNumbersZeroStyle = "zero",
        lineBufferSize = "1024"
    )
    public void configure() {}
}
`

		srcAST, err := treesitter.Parse([]byte(oldSrc), "ConfigService.java")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "ConfigService.java")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		r := rulesFor(srcAST)
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing class and method map in place.
		var srcClass, dstClass *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "ConfigService" {
				srcClass = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "ConfigService" {
				dstClass = n
				break
			}
		}
		if srcClass == nil || dstClass == nil || mr.Mappings.Src()[srcClass] != dstClass {
			t.Fatalf("expected class ConfigService to map in place")
		}

		// 2. Surrounding sequence: prefix, lineNumbersZeroStyle, and lineBufferSize map in place.
		findPairByKey := func(root *treesitter.ASTNode, key string) *treesitter.ASTNode {
			for _, n := range root.PreOrder() {
				if r.IsPair(n.Type) && getKeyLabel(n) == key {
					return n
				}
			}
			return nil
		}

		for _, stableKey := range []string{"prefix", "lineNumbersZeroStyle", "lineBufferSize"} {
			srcNode := findPairByKey(srcAST, stableKey)
			dstNode := findPairByKey(dstAST, stableKey)
			if srcNode == nil || dstNode == nil {
				t.Fatalf("stable pair %q not found: src=%v, dst=%v", stableKey, srcNode, dstNode)
			}
			if mr.Mappings.Src()[srcNode] != dstNode {
				t.Errorf("expected stable pair %q to map in place, got %v", stableKey, mr.Mappings.Src()[srcNode])
			}
		}

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		for _, addedKey := range []string{"defaultLanguage", "hunkLabel"} {
			dstNode := findPairByKey(dstAST, addedKey)
			if dstNode == nil {
				t.Fatalf("added pair %q not found in dst", addedKey)
			}
			if mr.Mappings.Dst()[dstNode] != nil {
				t.Errorf("expected added pair %q to remain unmapped, got mapped from %v (%s)",
					addedKey, mr.Mappings.Dst()[dstNode], getKeyLabel(mr.Mappings.Dst()[dstNode]))
			}
		}

		// 4. Evolving line numbers pairs:
		leftFormatSrc := findPairByKey(srcAST, "lineNumbersLeftFormat")
		formatDst := findPairByKey(dstAST, "lineNumbersFormat")
		if leftFormatSrc == nil || formatDst == nil || mr.Mappings.Src()[leftFormatSrc] != formatDst {
			t.Errorf("expected lineNumbersLeftFormat to map to lineNumbersFormat, got %v", mr.Mappings.Src()[leftFormatSrc])
		}

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		for _, delKey := range []string{"lineNumbersLeftStyle"} {
			srcNode := findPairByKey(srcAST, delKey)
			if srcNode == nil {
				t.Fatalf("deleted pair %q not found in src", delKey)
			}
			if mr.Mappings.Src()[srcNode] != nil {
				t.Errorf("expected deleted pair %q to remain unmapped, got mapped to %v (%s)",
					delKey, mr.Mappings.Src()[srcNode], getKeyLabel(mr.Mappings.Src()[srcNode]))
			}
		}
	})
}

func TestEnumVariantEvolution_NoFalseMatch(t *testing.T) {
	findVariant := func(root *treesitter.ASTNode, name string) *treesitter.ASTNode {
		for _, n := range root.PreOrder() {
			switch n.Type {
			case "enum_variant", "enumerator", "enum_constant", "enum_assignment":
				for _, c := range n.Children {
					if c.Label == name {
						return n
					}
				}
			case "property_identifier":
				if n.Label == name && n.Parent != nil && n.Parent.Type == "enum_body" {
					return n
				}
			}
		}
		return nil
	}

	t.Run("FullEngine_Rust", func(t *testing.T) {
		oldSrc := `#[derive(Clone, Debug, PartialEq)]
pub enum State {
    CommitMeta,
    FileMeta,
    HunkHeader,
    HunkZero,
    HunkMinus(Option<String>),
    HunkPlus(Option<String>),
    Unknown,
}
`
		newSrc := `#[derive(Clone, Debug, PartialEq)]
pub enum State {
    CommitMeta,
    FileMeta,
    HunkHeader(String, String),
    HunkZero,
    HunkMinus(Option<String>),
    HunkPlus(Option<String>),
    SubmoduleLog,
    SubmoduleShort(String),
    Unknown,
    HunkZeroWrapped,
    HunkMinusWrapped,
    HunkPlusWrapped,
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Surrounding sequence maps in place
		for _, name := range []string{"CommitMeta", "FileMeta", "HunkZero", "Unknown"} {
			s := findVariant(srcAST, name)
			d := findVariant(dstAST, name)
			if s == nil || d == nil || mr.Mappings.Src()[s] != d {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", name, s, d, mr.Mappings.Src()[s])
			}
		}

		// 2. Target variant evolves in place
		sHunkHeader := findVariant(srcAST, "HunkHeader")
		dHunkHeader := findVariant(dstAST, "HunkHeader")
		if sHunkHeader == nil || dHunkHeader == nil {
			t.Fatalf("HunkHeader not found: src=%v, dst=%v", sHunkHeader, dHunkHeader)
		}
		if mapped := mr.Mappings.Src()[sHunkHeader]; mapped != dHunkHeader {
			t.Errorf("expected HunkHeader to map in place, got %v", mapped)
		}

		// 3. Added variant is unmapped in src (inserted)
		dHunkPlus := findVariant(dstAST, "HunkPlusWrapped")
		if dHunkPlus == nil {
			t.Fatalf("HunkPlusWrapped not found in dst")
		}
		if mappedSrc := mr.Mappings.Dst()[dHunkPlus]; mappedSrc != nil {
			t.Errorf("expected HunkPlusWrapped to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_C", func(t *testing.T) {
		oldSrc := `enum State {
    STATE_COMMIT_META,
    STATE_FILE_META,
    STATE_OLD_DELETED_HEADER,
    STATE_HUNK_ZERO,
    STATE_UNKNOWN
};
`
		newSrc := `enum State {
    STATE_COMMIT_META,
    STATE_FILE_META,
    STATE_NEW_INSERTED_HEADER = 42,
    STATE_HUNK_ZERO,
    STATE_UNKNOWN,
    STATE_HUNK_ZERO_WRAPPED,
    STATE_HUNK_MINUS_WRAPPED,
    STATE_HUNK_PLUS_WRAPPED
};
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.c")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.c")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		for _, name := range []string{"STATE_COMMIT_META", "STATE_FILE_META", "STATE_HUNK_ZERO", "STATE_UNKNOWN"} {
			s := findVariant(srcAST, name)
			d := findVariant(dstAST, name)
			if s == nil || d == nil || mr.Mappings.Src()[s] != d {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", name, s, d, mr.Mappings.Src()[s])
			}
		}

		// Disjoint variant names must not match across non-identical identifiers
		sOld := findVariant(srcAST, "STATE_OLD_DELETED_HEADER")
		dNew := findVariant(dstAST, "STATE_NEW_INSERTED_HEADER")
		if sOld == nil || dNew == nil {
			t.Fatalf("variants not found: sOld=%v, dNew=%v", sOld, dNew)
		}
		if mapped := mr.Mappings.Src()[sOld]; mapped != nil {
			t.Errorf("expected STATE_OLD_DELETED_HEADER to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dNew]; mappedSrc != nil {
			t.Errorf("expected STATE_NEW_INSERTED_HEADER to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Cpp", func(t *testing.T) {
		oldSrc := `enum class State {
    CommitMeta,
    FileMeta,
    OldDeletedHeader,
    HunkZero,
    Unknown
};
`
		newSrc := `enum class State {
    CommitMeta,
    FileMeta,
    NewInsertedHeader = 100,
    HunkZero,
    Unknown,
    HunkZeroWrapped,
    HunkMinusWrapped,
    HunkPlusWrapped
};
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		for _, name := range []string{"CommitMeta", "FileMeta", "HunkZero", "Unknown"} {
			s := findVariant(srcAST, name)
			d := findVariant(dstAST, name)
			if s == nil || d == nil || mr.Mappings.Src()[s] != d {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", name, s, d, mr.Mappings.Src()[s])
			}
		}

		sOld := findVariant(srcAST, "OldDeletedHeader")
		dNew := findVariant(dstAST, "NewInsertedHeader")
		if sOld == nil || dNew == nil {
			t.Fatalf("variants not found: sOld=%v, dNew=%v", sOld, dNew)
		}
		if mapped := mr.Mappings.Src()[sOld]; mapped != nil {
			t.Errorf("expected OldDeletedHeader to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dNew]; mappedSrc != nil {
			t.Errorf("expected NewInsertedHeader to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		oldSrc := `public enum State {
    CommitMeta,
    FileMeta,
    HunkHeader,
    HunkZero,
    Unknown;
}
`
		newSrc := `public enum State {
    CommitMeta,
    FileMeta,
    HunkHeader(42),
    HunkZero,
    Unknown,
    HunkZeroWrapped,
    HunkMinusWrapped,
    HunkPlusWrapped;
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "Test.java")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "Test.java")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		for _, name := range []string{"CommitMeta", "FileMeta", "HunkZero", "Unknown"} {
			s := findVariant(srcAST, name)
			d := findVariant(dstAST, name)
			if s == nil || d == nil || mr.Mappings.Src()[s] != d {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", name, s, d, mr.Mappings.Src()[s])
			}
		}

		sHunkHeader := findVariant(srcAST, "HunkHeader")
		dHunkHeader := findVariant(dstAST, "HunkHeader")
		if sHunkHeader == nil || dHunkHeader == nil {
			t.Fatalf("HunkHeader not found: src=%v, dst=%v", sHunkHeader, dHunkHeader)
		}
		if mapped := mr.Mappings.Src()[sHunkHeader]; mapped != dHunkHeader {
			t.Errorf("expected HunkHeader to map in place, got %v", mapped)
		}

		dHunkPlus := findVariant(dstAST, "HunkPlusWrapped")
		if dHunkPlus == nil {
			t.Fatalf("HunkPlusWrapped not found in dst")
		}
		if mappedSrc := mr.Mappings.Dst()[dHunkPlus]; mappedSrc != nil {
			t.Errorf("expected HunkPlusWrapped to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_TypeScript", func(t *testing.T) {
		oldSrc := `enum State {
    CommitMeta = 1,
    FileMeta = 2,
    HunkHeader,
    HunkZero = 4,
    Unknown = 5,
}
`
		newSrc := `enum State {
    CommitMeta = 1,
    FileMeta = 2,
    HunkHeader = 42,
    HunkZero = 4,
    Unknown = 5,
    HunkZeroWrapped,
    HunkMinusWrapped,
    HunkPlusWrapped,
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		for _, name := range []string{"CommitMeta", "FileMeta", "HunkZero", "Unknown"} {
			s := findVariant(srcAST, name)
			d := findVariant(dstAST, name)
			if s == nil || d == nil || mr.Mappings.Src()[s] != d {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", name, s, d, mr.Mappings.Src()[s])
			}
		}

		sHunkHeader := findVariant(srcAST, "HunkHeader")
		dHunkHeader := findVariant(dstAST, "HunkHeader")
		if sHunkHeader == nil || dHunkHeader == nil {
			t.Fatalf("HunkHeader not found: src=%v, dst=%v", sHunkHeader, dHunkHeader)
		}
		if mapped := mr.Mappings.Src()[sHunkHeader]; mapped != dHunkHeader {
			t.Errorf("expected HunkHeader to map in place, got %v", mapped)
		}

		dHunkPlus := findVariant(dstAST, "HunkPlusWrapped")
		if dHunkPlus == nil {
			t.Fatalf("HunkPlusWrapped not found in dst")
		}
		if mappedSrc := mr.Mappings.Dst()[dHunkPlus]; mappedSrc != nil {
			t.Errorf("expected HunkPlusWrapped to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})
}

func TestParameterDeclarationEvolution_NoFalseMatch(t *testing.T) {
	findParam := func(root *treesitter.ASTNode, name string) *treesitter.ASTNode {
		for _, n := range root.PreOrder() {
			switch n.Type {
			case "parameter", "parameter_declaration", "variadic_parameter_declaration", "formal_parameter", "spread_parameter",
				"required_parameter", "optional_parameter", "typed_parameter", "default_parameter", "typed_default_parameter":
				for _, c := range n.Children {
					if c.Label == name {
						return n
					}
					for _, sub := range c.Children {
						if sub.Label == name {
							return n
						}
					}
				}
			}
		}
		return nil
	}

	t.Run("FullEngine_Rust", func(t *testing.T) {
		oldSrc := `#[allow(clippy::too_many_arguments)]
fn format_and_paint_line_number_field<'a>(
    format_data: &[LineNumberPlaceholderData<'a>],
    style: &Style,
    minus_number: Option<usize>,
    plus_number: Option<usize>,
    min_field_width: usize,
    minus_number_style: &Style,
    plus_number_style: &Style,
    plus_file: &str,
    config: &config::Config,
) -> Vec<String> {
    let mut ansi_strings = Vec::new();
    let mut suffix = "";
    ansi_strings.push(suffix.to_string());
    ansi_strings
}
`
		newSrc := `#[allow(clippy::too_many_arguments)]
fn format_and_paint_line_number_field<'a>(
    line_numbers_data: &'a LineNumbersData,
    side: MinusPlusIndex,
    styles: &MinusPlus<Style>,
    line_numbers: &MinusPlus<Option<usize>>,
    config: &config::Config,
) -> Vec<String> {
    let min_field_width = line_numbers_data.hunk_max_line_number_width;
    let mut ansi_strings = Vec::new();
    let mut suffix = "";
    ansi_strings.push(suffix.to_string());
    ansi_strings
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Unchanged parameter maps in place
		sConfig := findParam(srcAST, "config")
		dConfig := findParam(dstAST, "config")
		if sConfig == nil || dConfig == nil || mr.Mappings.Src()[sConfig] != dConfig {
			t.Errorf("expected config parameter to map in place, got src=%v dst=%v mapped=%v", sConfig, dConfig, mr.Mappings.Src()[sConfig])
		}

		// 2. Differing parameters with different names and types must NOT cross-match
		sPlus := findParam(srcAST, "plus_number")
		dLineNumbers := findParam(dstAST, "line_numbers")
		if sPlus == nil || dLineNumbers == nil {
			t.Fatalf("parameters not found: sPlus=%v, dLineNumbers=%v", sPlus, dLineNumbers)
		}
		if mapped := mr.Mappings.Src()[sPlus]; mapped != nil {
			t.Errorf("expected plus_number to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dLineNumbers]; mappedSrc != nil {
			t.Errorf("expected line_numbers to be unmapped in src, got mapped from %v", mappedSrc)
		}

		sMinusStyle := findParam(srcAST, "minus_number_style")
		dSide := findParam(dstAST, "side")
		if sMinusStyle == nil || dSide == nil {
			t.Fatalf("parameters not found: sMinusStyle=%v, dSide=%v", sMinusStyle, dSide)
		}
		if mr.Mappings.Src()[sMinusStyle] == dSide {
			t.Errorf("expected minus_number_style to not match side, got mapped to %v", dSide)
		}
	})

	t.Run("FullEngine_Go", func(t *testing.T) {
		oldSrc := `package main

func processItems(
	totalCount int,
	oldLimit int,
	items []string,
) int {
	count := len(items)
	return count
}
`
		newSrc := `package main

func processItems(
	batchSize float64,
	newLimit bool,
	items []string,
) int {
	count := len(items)
	return count
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Unchanged parameter maps in place
		sItems := findParam(srcAST, "items")
		dItems := findParam(dstAST, "items")
		if sItems == nil || dItems == nil || mr.Mappings.Src()[sItems] != dItems {
			t.Errorf("expected items parameter to map in place, got src=%v dst=%v mapped=%v", sItems, dItems, mr.Mappings.Src()[sItems])
		}

		// 2. Disjoint parameter names must not match across non-identical identifiers
		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Go_VariadicEvolution", func(t *testing.T) {
		oldSrc := `package main

func processItems(
	totalCount int,
	items []string,
) int {
	count := len(items)
	return count
}
`
		newSrc := `package main

func processItems(
	batchSize float64,
	items ...string,
) int {
	count := len(items)
	return count
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sItems := findParam(srcAST, "items")
		dItems := findParam(dstAST, "items")
		if sItems == nil || dItems == nil || mr.Mappings.Src()[sItems] != dItems {
			t.Errorf("expected items parameter to map in place across variadic evolution, got src=%v dst=%v mapped=%v", sItems, dItems, mr.Mappings.Src()[sItems])
		}

		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Cpp", func(t *testing.T) {
		oldSrc := `int process(int totalCount, double oldLimit, const char* name) {
    int res = 0;
    return res;
}
`
		newSrc := `int process(long batchSize, float newLimit, const char* name) {
    int res = 0;
    return res;
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sName := findParam(srcAST, "name")
		dName := findParam(dstAST, "name")
		if sName == nil || dName == nil || mr.Mappings.Src()[sName] != dName {
			t.Errorf("expected name parameter to map in place, got src=%v dst=%v mapped=%v", sName, dName, mr.Mappings.Src()[sName])
		}

		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		oldSrc := `class Service {
    int process(int totalCount, double oldLimit, String name) {
        int res = 0;
        return res;
    }
}
`
		newSrc := `class Service {
    int process(long batchSize, float newLimit, String name) {
        int res = 0;
        return res;
    }
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.java")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.java")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sName := findParam(srcAST, "name")
		dName := findParam(dstAST, "name")
		if sName == nil || dName == nil || mr.Mappings.Src()[sName] != dName {
			t.Errorf("expected name parameter to map in place, got src=%v dst=%v mapped=%v", sName, dName, mr.Mappings.Src()[sName])
		}

		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_TypeScript", func(t *testing.T) {
		oldSrc := `function process(totalCount: number, oldLimit: number, name: string): number {
    let res = 0;
    return res;
}
`
		newSrc := `function process(batchSize: string, newLimit: boolean, name: string): number {
    let res = 0;
    return res;
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sName := findParam(srcAST, "name")
		dName := findParam(dstAST, "name")
		if sName == nil || dName == nil || mr.Mappings.Src()[sName] != dName {
			t.Errorf("expected name parameter to map in place, got src=%v dst=%v mapped=%v", sName, dName, mr.Mappings.Src()[sName])
		}

		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Python", func(t *testing.T) {
		oldSrc := `def process(total_count: int, old_limit: int, name: str):
    res = 0
    return res
`
		newSrc := `def process(batch_size: str, new_limit: float, name: str):
    res = 0
    return res
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.py")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.py")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sName := findParam(srcAST, "name")
		dName := findParam(dstAST, "name")
		if sName == nil || dName == nil || mr.Mappings.Src()[sName] != dName {
			t.Errorf("expected name parameter to map in place, got src=%v dst=%v mapped=%v", sName, dName, mr.Mappings.Src()[sName])
		}

		sTotal := findParam(srcAST, "total_count")
		dBatch := findParam(dstAST, "batch_size")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected total_count to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batch_size to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})

	t.Run("FullEngine_Zig", func(t *testing.T) {
		oldSrc := `fn process(totalCount: i32, oldLimit: i32, name: []const u8) i32 {
    var res: i32 = 0;
    return res;
}
`
		newSrc := `fn process(batchSize: f64, newLimit: bool, name: []const u8) i32 {
    var res: i32 = 0;
    return res;
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.zig")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.zig")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		sName := findParam(srcAST, "name")
		dName := findParam(dstAST, "name")
		if sName == nil || dName == nil || mr.Mappings.Src()[sName] != dName {
			t.Errorf("expected name parameter to map in place, got src=%v dst=%v mapped=%v", sName, dName, mr.Mappings.Src()[sName])
		}

		sTotal := findParam(srcAST, "totalCount")
		dBatch := findParam(dstAST, "batchSize")
		if sTotal == nil || dBatch == nil {
			t.Fatalf("parameters not found: sTotal=%v, dBatch=%v", sTotal, dBatch)
		}
		if mapped := mr.Mappings.Src()[sTotal]; mapped != nil {
			t.Errorf("expected totalCount to remain unmapped, got mapped to %v", mapped)
		}
		if mappedSrc := mr.Mappings.Dst()[dBatch]; mappedSrc != nil {
			t.Errorf("expected batchSize to be unmapped in src, got mapped from %v", mappedSrc)
		}
	})
}

// TestCanPairDelimitedSiblings_EmptyDelimitedContainerGuard ensures empty
// delimited containers only match other empty containers.
// Without this check, the empty `()` of `"".into()` inside a Rust macro can steal
// the delimiter frame from `Some("nm")` instead of letting the rewritten payload
// `Some(Placeholder::NumberMinus)` pair with it.
func TestCanPairDelimitedSiblings_EmptyDelimitedContainerGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `struct D;

fn t() {
    assert_eq!(
        parse_line_number_format("{nm}"),
        vec![LinePlaceholderData {
            prefix: "",
            placeholder: Some("nm"),
            suffix: "",
        }]
    )
}
`
		newSrc := `struct D;

fn t() {
    assert_eq!(
        parse_line_number_format_with_default_regex("{nm}"),
        vec![FormatStringPlaceholderData {
            prefix: "".into(),
            placeholder: Some(Placeholder::NumberMinus),
            suffix: "".into(),
        }]
    )
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst snippet must parse cleanly: hasError=%v errors=%d", dstAST.HasError, dstAST.ParseErrorCount)
		}

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		mapped := mr.Mappings.Src()

		// Locate target nodes:
		//   srcArgs: ("nm") token_tree inside the brace body
		//   dstArgs: (Placeholder::NumberMinus) token_tree inside the brace body
		//   srcFmtArgs / dstFmtArgs: ("{nm}") payload container in the format call
		//   dstEmpty: the empty () token_tree nodes from `"".into()`
		var srcArgs, srcFmtArgs *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if !r.IsDelimitedContainer(n.Type) {
				continue
			}
			if len(n.Children) == 1 && n.Children[0].Label == `"nm"` {
				srcArgs = n
			}
			if len(n.Children) == 1 && n.Children[0].Label == `"{nm}"` {
				srcFmtArgs = n
			}
		}

		var dstArgs, dstFmtArgs *treesitter.ASTNode
		dstEmptyByRow := make(map[uint32]*treesitter.ASTNode)
		for _, n := range dstAST.Descendants() {
			if !r.IsDelimitedContainer(n.Type) {
				continue
			}
			if len(n.Children) == 2 && n.Children[0].Label == "Placeholder" {
				dstArgs = n
			}
			if len(n.Children) == 1 && n.Children[0].Label == `"{nm}"` {
				dstFmtArgs = n
			}
			if len(n.Children) == 0 {
				dstEmptyByRow[n.StartRow] = n
			}
		}
		dstEmptyPrefix := dstEmptyByRow[6]
		dstEmptySuffix := dstEmptyByRow[8]
		if srcArgs == nil || dstArgs == nil || srcFmtArgs == nil || dstFmtArgs == nil || dstEmptyPrefix == nil || dstEmptySuffix == nil {
			t.Fatalf("failed to locate token_tree nodes: srcArgs=%v dstArgs=%v srcFmtArgs=%v dstFmtArgs=%v dstEmptyPrefix=%v dstEmptySuffix=%v",
				srcArgs, dstArgs, srcFmtArgs, dstFmtArgs, dstEmptyPrefix, dstEmptySuffix)
		}
		srcBrace := srcArgs.Parent
		dstBrace := dstArgs.Parent
		if srcBrace == nil || dstBrace == nil || srcBrace.Type != dstBrace.Type {
			t.Fatalf("failed to locate brace bodies: srcBrace=%v dstBrace=%v", srcBrace, dstBrace)
		}

		// 1. Enclosing context: brace bodies of vec![Struct { ... }] map in place.
		if mapped[srcBrace] != dstBrace {
			t.Errorf("expected brace field body to map in place, got %v", mapped[srcBrace])
		}

		// 2. Surrounding sequence: format payload container and unchanged field names
		// map in place without order disruption.
		if mapped[srcFmtArgs] != dstFmtArgs {
			t.Errorf(`expected the ("{nm}") payload container to map in place, got %v`, mapped[srcFmtArgs])
		}
		for _, want := range []string{"prefix", "placeholder", "suffix", "Some"} {
			var srcN, dstN *treesitter.ASTNode
			for _, n := range srcBrace.Children {
				if n.Label == want {
					srcN = n
				}
			}
			for _, n := range dstBrace.Children {
				if n.Label == want {
					dstN = n
				}
			}
			if srcN == nil || dstN == nil || mapped[srcN] != dstN {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", want, srcN, dstN, mapped[srcN])
			}
		}

		// 3. Target & containers: neither empty `.into()` frame can steal ("nm");
		// the rewritten payload container pairs instead.
		if got := mapped[srcArgs]; got == dstEmptyPrefix || got == dstEmptySuffix {
			t.Errorf("empty `.into()` token_tree stole the (\"nm\") delimiter frame: srcArgs mapped to %v", got)
		}
		if mapped[srcArgs] != dstArgs {
			t.Errorf(`expected srcArgs ("nm") to pair with the rewritten payload container, got %v`, mapped[srcArgs])
		}
		if got := mr.Mappings.Dst()[dstEmptyPrefix]; got != nil {
			t.Errorf("expected prefix into() empty token_tree to remain unmapped, got %v", got)
		}
		if got := mr.Mappings.Dst()[dstEmptySuffix]; got != nil {
			t.Errorf("expected suffix into() empty token_tree to remain unmapped, got %v", got)
		}
	})
}

func TestBareIdentifierLastResortPairing(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn format_commit_line_captures_with_osc8_commit_hyperlink(
    captures: &Captures,
    github_repo: &str,
) -> String {
    let commit = captures.get(2).unwrap().as_str();
    format!(
        "{prefix}{osc}8;;{url}{st}{commit}{osc}8;;{st}{suffix}",
        url = format_github_commit_url(commit, github_repo),
        commit = commit,
        prefix = captures.get(1).unwrap().as_str(),
        suffix = captures.get(3).unwrap().as_str(),
        osc = "\x1b]",
        st = "\x1b\\"
    )
}
`
		newSrc := `fn format_commit_line_captures_with_osc8_commit_hyperlink(
    captures: &Captures,
    github_repo: &str,
) -> String {
    let commit = captures.get(2).unwrap().as_str();
    format!(
        "{prefix}{osc}8;;{url}{st}{commit}{osc}8;;{st}{suffix}",
        url = format_github_commit_url(commit, github_repo),
        commit = commit,
        prefix = captures.get(1).map(|m| m.as_str()).unwrap_or(""),
        suffix = captures.get(3).unwrap().as_str(),
        osc = "\x1b]",
        st = "\x1b\\"
    )
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst snippet must parse cleanly: hasError=%v errors=%d", dstAST.HasError, dstAST.ParseErrorCount)
		}

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		mapped := mr.Mappings.Src()

		// 1. Enclosing Context: function item, format! macro invocation, and let statement map in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Errorf("expected enclosing function to map in place, got src=%v dst=%v mapped=%v", srcFn, dstFn, mapped[srcFn])
		}

		var srcMacro, dstMacro *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "macro_invocation" {
				srcMacro = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "macro_invocation" {
				dstMacro = n
				break
			}
		}
		if srcMacro == nil || dstMacro == nil || mapped[srcMacro] != dstMacro {
			t.Errorf("expected format! macro invocation to map in place, got src=%v dst=%v mapped=%v", srcMacro, dstMacro, mapped[srcMacro])
		}

		// 2. Surrounding Sequence: surrounding tokens and stationary arguments map in place.
		for _, want := range []string{"url", "commit", "suffix", "osc", "st"} {
			var srcN, dstN *treesitter.ASTNode
			for _, n := range srcMacro.Descendants() {
				if r.IsIdentifier(n.Type) && n.Label == want {
					srcN = n
					break
				}
			}
			for _, n := range dstMacro.Descendants() {
				if r.IsIdentifier(n.Type) && n.Label == want {
					dstN = n
					break
				}
			}
			if srcN == nil || dstN == nil || mapped[srcN] != dstN {
				t.Errorf("expected %s to map in place, got src=%v dst=%v mapped=%v", want, srcN, dstN, mapped[srcN])
			}
		}

		// 3. Target & Containers (the defect invariant):
		// Locate target nodes on Row 9 (prefix = captures.get(1)...):
		//   oldUnwrap: identifier "unwrap" on Row 9
		//   oldAsStr: identifier "as_str" on Row 9
		//   newMap: identifier "map" on Row 9
		//   newAsStr: identifier "as_str" inside the closure on Row 9
		//   newUnwrapOr: identifier "unwrap_or" on Row 9
		var oldUnwrap, oldAsStr *treesitter.ASTNode
		for _, n := range srcMacro.Descendants() {
			if n.StartRow == 9 && r.IsIdentifier(n.Type) {
				switch n.Label {
				case "unwrap":
					oldUnwrap = n
				case "as_str":
					oldAsStr = n
				}
			}
		}
		var newMap, newAsStr, newUnwrapOr *treesitter.ASTNode
		for _, n := range dstMacro.Descendants() {
			if n.StartRow == 9 && r.IsIdentifier(n.Type) {
				switch n.Label {
				case "map":
					newMap = n
				case "as_str":
					newAsStr = n
				case "unwrap_or":
					newUnwrapOr = n
				}
			}
		}
		if oldUnwrap == nil || oldAsStr == nil || newMap == nil || newAsStr == nil || newUnwrapOr == nil {
			t.Fatalf("failed to locate target nodes: oldUnwrap=%v oldAsStr=%v newMap=%v newAsStr=%v newUnwrapOr=%v",
				oldUnwrap, oldAsStr, newMap, newAsStr, newUnwrapOr)
		}

		// Unrelated identifiers like as_str and map must never pair.
		if got := mapped[oldAsStr]; got == newMap {
			t.Errorf("uniqueTypePairs glued bare identifier as_str to unrelated map: mapped=%v", got)
		}
		if got := mr.Mappings.Dst()[newMap]; got != nil {
			t.Errorf("expected new map identifier to remain unmapped, got %v", got)
		}
		// unwrap pairs with unwrap_or in place.
		if mapped[oldUnwrap] != newUnwrapOr {
			t.Errorf("expected unwrap to pair with unwrap_or, got %v", mapped[oldUnwrap])
		}
		// as_str pairs with the closure's as_str.
		if mapped[oldAsStr] != newAsStr {
			t.Errorf("expected old as_str to pair with closure as_str, got %v", mapped[oldAsStr])
		}
	})
}

// TestCanPairDelimitedSiblings_CallerAffinityGuard guards against pairing disjoint
// delimited containers across unmapped or incompatible callers. When a function call
// in a macro is replaced by an unrelated call (e.g. Regex::new replaced by format::make_placeholder_regex),
// the old container and its delimiters must be deleted and the new container inserted,
// rather than falsely pairing the unrelated delimiter frames.
func TestCanPairDelimitedSiblings_CallerAffinityGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `lazy_static! {
    static ref LINE_NUMBERS_PLACEHOLDER_REGEX: Regex = Regex::new(
        "abc"
    )
    .unwrap();
}
`
		newSrc := `lazy_static! {
    static ref LINE_NUMBERS_PLACEHOLDER_REGEX: Regex = format::make_placeholder_regex(&["nm", "np"]);
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst snippet must parse cleanly: hasError=%v errors=%d", dstAST.HasError, dstAST.ParseErrorCount)
		}

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		mapped := mr.Mappings.Src()

		// 1. Enclosing Context: macro_invocation and the outer macro token_tree map in place.
		var srcMacro, dstMacro *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "macro_invocation" {
				srcMacro = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "macro_invocation" {
				dstMacro = n
				break
			}
		}
		if srcMacro == nil || dstMacro == nil || mapped[srcMacro] != dstMacro {
			t.Fatalf("expected macro_invocation to map in place, got src=%v dst=%v mapped=%v", srcMacro, dstMacro, mapped[srcMacro])
		}

		var srcOuterTree, dstOuterTree *treesitter.ASTNode
		for _, c := range srcMacro.Children {
			if r.IsDelimitedContainer(c.Type) {
				srcOuterTree = c
				break
			}
		}
		for _, c := range dstMacro.Children {
			if r.IsDelimitedContainer(c.Type) {
				dstOuterTree = c
				break
			}
		}
		if srcOuterTree == nil || dstOuterTree == nil || mapped[srcOuterTree] != dstOuterTree {
			t.Fatalf("expected outer macro token_tree to map in place, got src=%v dst=%v mapped=%v", srcOuterTree, dstOuterTree, mapped[srcOuterTree])
		}

		// 2. Surrounding Sequence: static, ref, LINE_NUMBERS_PLACEHOLDER_REGEX, and Regex (type) map in place.
		for _, want := range []string{"static", "ref", "LINE_NUMBERS_PLACEHOLDER_REGEX", "Regex"} {
			var srcN, dstN *treesitter.ASTNode
			for _, n := range srcOuterTree.Children {
				if n.Label == want {
					srcN = n
					break
				}
			}
			for _, n := range dstOuterTree.Children {
				if n.Label == want {
					dstN = n
					break
				}
			}
			if srcN == nil || dstN == nil || mapped[srcN] != dstN {
				t.Errorf("expected %q to map in place, got src=%v dst=%v mapped=%v", want, srcN, dstN, mapped[srcN])
			}
		}

		// 3. Target & Containers (defect invariant):
		// The old token_tree of Regex::new ("abc") MUST NOT pair with the new token_tree of format::make_placeholder_regex.
		var oldRegexArgs, newFormatArgs *treesitter.ASTNode
		for _, n := range srcOuterTree.Children {
			if r.IsDelimitedContainer(n.Type) && n.StartRow >= 1 && n.StartRow <= 3 {
				oldRegexArgs = n
				break
			}
		}
		for _, n := range dstOuterTree.Children {
			if r.IsDelimitedContainer(n.Type) && n.StartRow == 1 {
				newFormatArgs = n
				break
			}
		}
		if oldRegexArgs == nil || newFormatArgs == nil {
			t.Fatalf("failed to locate target delimiter frames: oldRegexArgs=%v newFormatArgs=%v", oldRegexArgs, newFormatArgs)
		}

		if got := mapped[oldRegexArgs]; got != nil {
			t.Errorf("expected old Regex::new argument token_tree to remain unmapped, but it mapped to %v", got)
		}
		if got := mr.Mappings.Dst()[newFormatArgs]; got != nil {
			t.Errorf("expected new format::make_placeholder_regex argument token_tree to remain unmapped, but it mapped from %v", got)
		}

		// Old Regex, new, and unwrap identifiers must remain unmapped (destined for deletion).
		for _, name := range []string{"new", "unwrap"} {
			for _, n := range srcOuterTree.Children {
				if r.IsIdentifier(n.Type) && n.Label == name {
					if got := mapped[n]; got != nil {
						t.Errorf("expected old %q to remain unmapped, got %v", name, got)
					}
				}
			}
		}

		// New format and make_placeholder_regex identifiers must remain unmapped (destined for insertion).
		for _, name := range []string{"format", "make_placeholder_regex"} {
			for _, n := range dstOuterTree.Children {
				if r.IsIdentifier(n.Type) && n.Label == name {
					if got := mr.Mappings.Dst()[n]; got != nil {
						t.Errorf("expected new %q to remain unmapped, got %v", name, got)
					}
				}
			}
		}
	})
}
