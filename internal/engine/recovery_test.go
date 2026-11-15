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

		// In src, row 5 (line 6) is "shared := calc()" inside "mode == 1".
		// In src, row 10 (line 11) is "shared := calc()" inside "mode == 2".
		// In dst, row 5 (line 6) is "shared := calc()" inside "mode == 1".
		// With reclaimInParentIsomorphicChildren, srcShared1 (inside mode == 1) maps in-place to dstShared.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 6},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 6},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 11},
		)

		// Whole-snippet verification: func Run, both if blocks, and finishB map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 9},
			testutil.NodeQuery{Type: "if_statement", Line: 10},
		)

		// Verify finishB statement in mode == 2 maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 12},
			testutil.NodeQuery{Type: "expression_statement", Line: 11},
		)
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

	// On main without the boundary guard and container Dice threshold,
	// uniqueTypePairs and findBestExpressionMatch mapped srcOuterCall to dstInnerCmpCall.
	testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
		testutil.NodeQuery{Type: "call_expression", Line: 4},
		testutil.NodeQuery{Type: "call_expression", Line: 5},
	)
	testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
		testutil.NodeQuery{Type: "call_expression", Line: 5},
	)

	// Whole-snippet verification: func sortItems, expression statement, and closure function literal map in place
	testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
		testutil.NodeQuery{Type: "function_declaration", Line: 3},
		testutil.NodeQuery{Type: "function_declaration", Line: 3},
	)
	testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
		testutil.NodeQuery{Type: "expression_statement", Line: 4},
		testutil.NodeQuery{Type: "expression_statement", Line: 4},
	)
	testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
		testutil.NodeQuery{Type: "func_literal", Line: 4},
		testutil.NodeQuery{Type: "func_literal", Line: 4},
	)
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

		// Target assertion: the stationary first operand must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 2},
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 3},
		)

		// Defect assertion: on main without sibling absorption rejection, uniqueTypePairs
		// erroneously paired the deleted second operand (srcSecondOp) to dstInnerWrapper.
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 3},
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 2},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 3},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 2},
		)

		// Whole-snippet verification: enclosing type and function declarations, surrounding statements, and returns map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "type_declaration", Line: 3},
			testutil.NodeQuery{Type: "type_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 7},
			testutil.NodeQuery{Type: "function_declaration", Line: 7},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 8},
			testutil.NodeQuery{Type: "if_statement", Line: 8},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 9},
			testutil.NodeQuery{Type: "return_statement", Line: 9},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 11},
			testutil.NodeQuery{Type: "return_statement", Line: 11},
		)
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

		// 1. Enclosing Context: Function declaration, parameter list
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
		)

		// 2. Surrounding Sequence: Preceding short_var_declaration, if_statement, and returns
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 5},
			testutil.NodeQuery{Type: "if_statement", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 6},
			testutil.NodeQuery{Type: "return_statement", Line: 6},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 8},
			testutil.NodeQuery{Type: "return_statement", Line: 8},
		)

		// 3. Target, Containers & Defect Invariant:
		// bOldInner (MoveFg+badgeText) maps to bDst2 (Nth 2 in dst)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 2},
		)
		// bOldOuter (... + Reset) maps to bDst3 (Nth 1 in dst)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 1},
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 1},
		)

		// Defect assertions: bOldInner must NOT map to bDst1 (cross-side operator mismatch)
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 3},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 3},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5, Nth: 4},
		)

		// Call and argument_list map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5},
			testutil.NodeQuery{Type: "call_expression", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "argument_list", Line: 5},
			testutil.NodeQuery{Type: "argument_list", Line: 5},
		)
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

		// 1. Enclosing Context: Function declaration
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// 2. Surrounding Sequence: stmt1 := ... and stmt1.Children = ... and bin1.Parent = stmt1
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 5},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "assignment_statement", Line: 11},
			testutil.NodeQuery{Type: "assignment_statement", Line: 11},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "assignment_statement", Line: 12},
			testutil.NodeQuery{Type: "assignment_statement", Line: 12},
		)

		// 3. Target & Containers: bin1 outer call and argument_list
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 6},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "argument_list", Line: 6},
			testutil.NodeQuery{Type: "argument_list", Line: 6},
		)

		// 4. Sibling Calls: testutil.Leaf(...) <-> testutil.NodeAt(...)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 7},
			testutil.NodeQuery{Type: "call_expression", Line: 7},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 8},
			testutil.NodeQuery{Type: "call_expression", Line: 8},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 9},
			testutil.NodeQuery{Type: "call_expression", Line: 9},
		)

		// 5. Stationary Operands: strings inside sibling calls
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 7, Nth: 1},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 7, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 7, Nth: 2},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 7, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 8, Nth: 1},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 8, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 8, Nth: 2},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 8, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 9, Nth: 1},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 9, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 9, Nth: 2},
			testutil.NodeQuery{Type: "interpreted_string_literal", Line: 9, Nth: 2},
		)

		// 6. Granular evolution: new numeric offsets inserted and unmapped
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 7, Nth: 1},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 7, Nth: 2},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 8, Nth: 1},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 8, Nth: 2},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 9, Nth: 1},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "int_literal", Line: 9, Nth: 2},
		)
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags function
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Surrounding Sequence: const valid and return valid
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 6},
			testutil.NodeQuery{Type: "return_statement", Line: 6},
		)

		// 3. Target & Containers: if statement, unary ! and parenthesized_expression
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 3},
			testutil.NodeQuery{Type: "if_statement", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "unary_expression", Line: 3},
			testutil.NodeQuery{Type: "unary_expression", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 3, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 3, Nth: 2},
		)

		// 4. Stationary Operands: isEntityName(node) maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
		)

		// Deleted middle term hasDynamicName must be unmapped in src
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
		)
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags method
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
		)

		// 2. Surrounding Sequence: boolean valid and return valid
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 7},
			testutil.NodeQuery{Type: "return_statement", Line: 7},
		)

		// 3. Target: if statement and outer parenthesized condition
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 4, Nth: 1},
			testutil.NodeQuery{Type: "parenthesized_expression", Line: 4, Nth: 1},
		)

		// 4. Stationary Operands: call isEntityName
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 4, Nth: 2},
			testutil.NodeQuery{Type: "method_invocation", Line: 4, Nth: 1},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 4, Nth: 1},
		)
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: struct Config maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "struct_item", Line: 1},
			testutil.NodeQuery{Type: "struct_item", Line: 1},
		)

		// 2. Surrounding Sequence: hyperlinks and line_numbers_zero_style map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 2},
			testutil.NodeQuery{Type: "field_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 5},
			testutil.NodeQuery{Type: "field_declaration", Line: 4},
		)

		// 3. Target: Deleted line_numbers_plus_style must NOT match inserted inline_hint_style.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 4},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "field_declaration", Line: 5},
		)

		// 4. Same name with different type: pub foo: Style -> pub foo: String should map.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 3},
			testutil.NodeQuery{Type: "field_declaration", Line: 3},
		)
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Surrounding sequence: HashMap and Color must map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 1},
			testutil.NodeQuery{Type: "use_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
		)

		// 2. Target: Deleted use std::process must NOT match inserted use crate::fatal.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
		)
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: type Config struct maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "type_declaration", Line: 3},
			testutil.NodeQuery{Type: "type_declaration", Line: 3},
		)

		// 2. Surrounding sequence: Hyperlinks and Numbers map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 4},
			testutil.NodeQuery{Type: "field_declaration", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 7},
			testutil.NodeQuery{Type: "field_declaration", Line: 6},
		)

		// 3. Target: Deleted OldField must NOT match inserted NewField.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 6},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "field_declaration", Line: 7},
		)

		// 4. Same name with different type: Foo Style -> Foo string maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_declaration", Line: 5},
			testutil.NodeQuery{Type: "field_declaration", Line: 5},
		)
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
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), nil)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 10},
			testutil.NodeQuery{Type: "use_declaration", Line: 10},
		)
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

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: fn make_config maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: commit_style, color_only, line_numbers_zero_style, line_buffer_size map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 4},
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_initializer", Line: 5},
			testutil.NodeQuery{Type: "field_initializer", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 12},
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 21},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_initializer", Line: 13},
			testutil.NodeQuery{Type: "field_initializer", Line: 22},
		)

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "field_initializer", Line: 6},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 7},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 8},
		)

		// 4. Evolving line numbers pairs:
		// line_numbers_left_format -> line_numbers_format
		// line_numbers_left_style -> line_numbers_style_leftright
		// line_numbers_minus_style -> line_numbers_style_minusplus
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_initializer", Line: 6},
			testutil.NodeQuery{Type: "field_initializer", Line: 9},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 7},
			testutil.NodeQuery{Type: "field_initializer", Line: 13},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 8},
			testutil.NodeQuery{Type: "field_initializer", Line: 17},
		)

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 9},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_initializer", Line: 10},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_field_initializer", Line: 11},
		)
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

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function buildConfig maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Destructuring pattern: lineNumbers and sideBySide map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 1},
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 2},
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 2},
		)

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 4},
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 5},
			testutil.NodeQuery{Type: "pair", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 12},
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 21},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 13},
			testutil.NodeQuery{Type: "pair", Line: 22},
		)

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "pair", Line: 6},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 7},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 8},
		)

		// 5. Evolving line numbers pairs:
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 6},
			testutil.NodeQuery{Type: "pair", Line: 9},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 7},
			testutil.NodeQuery{Type: "pair", Line: 13},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 8},
			testutil.NodeQuery{Type: "pair", Line: 17},
		)

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 9},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 10},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 11},
		)
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

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function configure maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 6},
			testutil.NodeQuery{Type: "function_declaration", Line: 6},
		)

		// 2. Destructuring pattern: flag and mode map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 7, Nth: 1},
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 7, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 7, Nth: 2},
			testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 7, Nth: 2},
		)

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 9},
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 9},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 10},
			testutil.NodeQuery{Type: "pair", Line: 10},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 17},
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 26},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 18},
			testutil.NodeQuery{Type: "pair", Line: 27},
		)

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "pair", Line: 11},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 12},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 13},
		)

		// 5. Evolving line numbers pairs:
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 11},
			testutil.NodeQuery{Type: "pair", Line: 14},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 12},
			testutil.NodeQuery{Type: "pair", Line: 18},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 13},
			testutil.NodeQuery{Type: "pair", Line: 22},
		)

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 14},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "pair", Line: 15},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 16},
		)
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

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function ConfigPanel maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})

		// 2. Destructuring pattern: visible and theme map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 1}, testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 2}, testutil.NodeQuery{Type: "shorthand_property_identifier_pattern", Line: 2, Nth: 2})

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 4}, testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 5}, testutil.NodeQuery{Type: "pair", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 10}, testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 18})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 11}, testutil.NodeQuery{Type: "pair", Line: 19})

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "pair", Line: 6})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 7})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 8})

		// 5. Evolving line numbers pairs:
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 6}, testutil.NodeQuery{Type: "pair", Line: 9})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 7}, testutil.NodeQuery{Type: "pair", Line: 12})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 8}, testutil.NodeQuery{Type: "pair", Line: 15})

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "shorthand_property_identifier", Line: 9})
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

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing class and method map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "class_declaration", Line: 1}, testutil.NodeQuery{Type: "class_declaration", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "method_declaration", Line: 2}, testutil.NodeQuery{Type: "method_declaration", Line: 2})

		// 2. Surrounding sequence: prefix, lineNumbersZeroStyle, and lineBufferSize map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "element_value_pair", Line: 3}, testutil.NodeQuery{Type: "element_value_pair", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "element_value_pair", Line: 6}, testutil.NodeQuery{Type: "element_value_pair", Line: 7})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "element_value_pair", Line: 7}, testutil.NodeQuery{Type: "element_value_pair", Line: 8})

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "element_value_pair", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "element_value_pair", Line: 5})

		// 4. Evolving line numbers pairs:
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "element_value_pair", Line: 4}, testutil.NodeQuery{Type: "element_value_pair", Line: 6})

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "element_value_pair", Line: 5})
	})
}

func TestEnumVariantEvolution_NoFalseMatch(t *testing.T) {
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_variant", Line: 3}, testutil.NodeQuery{Type: "enum_variant", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_variant", Line: 4}, testutil.NodeQuery{Type: "enum_variant", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_variant", Line: 6}, testutil.NodeQuery{Type: "enum_variant", Line: 6})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_variant", Line: 9}, testutil.NodeQuery{Type: "enum_variant", Line: 11})

		// 2. Target variant evolves in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_variant", Line: 5}, testutil.NodeQuery{Type: "enum_variant", Line: 5})

		// 3. Added variant is unmapped in src (inserted)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "enum_variant", Line: 14})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 2}, testutil.NodeQuery{Type: "enumerator", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 3}, testutil.NodeQuery{Type: "enumerator", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 5}, testutil.NodeQuery{Type: "enumerator", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 6}, testutil.NodeQuery{Type: "enumerator", Line: 6})

		// Disjoint variant names must not match across non-identical identifiers
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "enumerator", Line: 4})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 4}, testutil.NodeQuery{Type: "enumerator", Line: 4})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 2}, testutil.NodeQuery{Type: "enumerator", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 3}, testutil.NodeQuery{Type: "enumerator", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 5}, testutil.NodeQuery{Type: "enumerator", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 6}, testutil.NodeQuery{Type: "enumerator", Line: 6})

		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "enumerator", Line: 4})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enumerator", Line: 4}, testutil.NodeQuery{Type: "enumerator", Line: 4})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_constant", Line: 2}, testutil.NodeQuery{Type: "enum_constant", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_constant", Line: 3}, testutil.NodeQuery{Type: "enum_constant", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_constant", Line: 5}, testutil.NodeQuery{Type: "enum_constant", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_constant", Line: 6}, testutil.NodeQuery{Type: "enum_constant", Line: 6})

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_constant", Line: 4}, testutil.NodeQuery{Type: "enum_constant", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "enum_constant", Line: 9})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_assignment", Line: 2}, testutil.NodeQuery{Type: "enum_assignment", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_assignment", Line: 3}, testutil.NodeQuery{Type: "enum_assignment", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_assignment", Line: 5}, testutil.NodeQuery{Type: "enum_assignment", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "enum_assignment", Line: 6}, testutil.NodeQuery{Type: "enum_assignment", Line: 6})

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "property_identifier", Line: 4}, testutil.NodeQuery{Type: "enum_assignment", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "property_identifier", Line: 9})
	})
}

func TestParameterDeclarationEvolution_NoFalseMatch(t *testing.T) {
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 11}, testutil.NodeQuery{Type: "parameter", Line: 7})

		// 2. Differing parameters with different names and types must NOT cross-match
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 6})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "parameter", Line: 6})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 6}, testutil.NodeQuery{Type: "parameter", Line: 6})

		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 8}, testutil.NodeQuery{Type: "parameter", Line: 4})
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 6}, testutil.NodeQuery{Type: "parameter_declaration", Line: 6})

		// 2. Disjoint parameter names must not match across non-identical identifiers
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4}, testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 5}, testutil.NodeQuery{Type: "variadic_parameter_declaration", Line: 5})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 4}, testutil.NodeQuery{Type: "parameter_declaration", Line: 4})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 3}, testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 3})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 1})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 1}, testutil.NodeQuery{Type: "parameter_declaration", Line: 1, Nth: 1})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 3}, testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 3})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 1})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 1}, testutil.NodeQuery{Type: "formal_parameter", Line: 2, Nth: 1})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 3}, testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 3})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 1})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 1}, testutil.NodeQuery{Type: "required_parameter", Line: 1, Nth: 1})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 3}, testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 3})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 1})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 1}, testutil.NodeQuery{Type: "typed_parameter", Line: 1, Nth: 1})
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

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 3}, testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 3})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 1})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 1}, testutil.NodeQuery{Type: "parameter", Line: 1, Nth: 1})
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

		// 1. Enclosing context: brace bodies of vec![Struct { ... }] map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 3}, testutil.NodeQuery{Type: "function_item", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 6, Nth: 2}, testutil.NodeQuery{Type: "token_tree", Line: 6, Nth: 2})

		// 2. Surrounding sequence: format payload container and unchanged field names
		// map in place without order disruption.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 5}, testutil.NodeQuery{Type: "token_tree", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 7}, testutil.NodeQuery{Type: "identifier", Line: 7, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 2}, testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 9}, testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 1})

		// 3. Target & containers: neither empty `.into()` frame can steal ("nm");
		// the rewritten payload container pairs instead.
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 8}, testutil.NodeQuery{Type: "token_tree", Line: 7})
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 8}, testutil.NodeQuery{Type: "token_tree", Line: 9})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 8}, testutil.NodeQuery{Type: "token_tree", Line: 8})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "token_tree", Line: 7})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "token_tree", Line: 9})
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

		// 1. Enclosing Context: function item, format! macro invocation, and let statement map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 5}, testutil.NodeQuery{Type: "let_declaration", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "macro_invocation", Line: 6}, testutil.NodeQuery{Type: "macro_invocation", Line: 6})

		// 2. Surrounding Sequence: surrounding tokens and stationary arguments map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 8, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 11, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 11, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 13, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 13, Nth: 1})

		// 3. Target & Containers (the defect invariant):
		// Unrelated identifiers like as_str and map must never pair.
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 5}, testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 4})

		// unwrap pairs with unwrap_or in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 4}, testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 8})
		// as_str pairs with the closure's as_str.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 5}, testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 7})
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

		// 1. Enclosing Context: macro_invocation and the outer macro token_tree map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "macro_invocation", Line: 1}, testutil.NodeQuery{Type: "macro_invocation", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 1}, testutil.NodeQuery{Type: "token_tree", Line: 1})

		// 2. Surrounding Sequence: static, ref, LINE_NUMBERS_PLACEHOLDER_REGEX, and Regex (type) map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "static", Line: 2}, testutil.NodeQuery{Type: "static", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2}, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 3}, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 3})

		// 3. Target & Containers (defect invariant):
		// The old token_tree of Regex::new ("abc") MUST NOT pair with the new token_tree of format::make_placeholder_regex.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 2, Nth: 1})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "token_tree", Line: 2, Nth: 1})

		// Old new and unwrap identifiers must remain unmapped (destined for deletion).
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 5})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 5})

		// New format and make_placeholder_regex identifiers must remain unmapped (destined for insertion).
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 5})
	})
}

func TestBlockStatementRecovery_MultipleTypedSiblings(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `#[test]
fn test_two_plus_lines() {
    let config = make_config_from_args(&["--side-by-side", "--width", "40"]);
    let output = run_delta(TWO_PLUS_LINES_DIFF, &config);
    let mut lines = output.lines().skip(7);
    let (line_1, line_2) = (lines.next().unwrap(), lines.next().unwrap());
    assert_eq!("│    │              │ 1  │a = 1", strip_ansi_codes(line_1));
    assert_eq!("│    │              │ 2  │b = 2", strip_ansi_codes(line_2));
}
`
		newSrc := `#[test]
fn test_two_plus_lines() {
    let config = make_config_from_args(&[
        "--side-by-side",
        "--width",
        "41",
        "--line-fill-method=spaces",
    ]);
    let output = run_delta(TWO_PLUS_LINES_DIFF, &config);
    let mut lines = output.lines().skip(crate::config::HEADER_LEN);
    let (line_1, line_2) = (lines.next().unwrap(), lines.next().unwrap());
    let sac = strip_ansi_codes; // alias to help with cargo fmt-ing:
    assert_eq!("│    │              │ 1  │a = 1         ", sac(line_1));
    assert_eq!("│    │              │ 2  │b = 234567    ", sac(line_2));
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

		// 1. Enclosing context: the function and body block should map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 2}, testutil.NodeQuery{Type: "function_item", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "block", Line: 2}, testutil.NodeQuery{Type: "block", Line: 2})

		// 2. Surrounding sequence: following statements should keep their mapping.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 4}, testutil.NodeQuery{Type: "let_declaration", Line: 9})

		// 3. Target construct: let config = make_config_from_args(...) should map across versions.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 3}, testutil.NodeQuery{Type: "let_declaration", Line: 3})

		// 4. Stationary operands: the config identifier should stay mapped.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1}, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})

		// 5. Inserted statement: new let sac shouldn't map back to anything in old code.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "let_declaration", Line: 12})
	})
}

func TestRustMacroCallEquivalenceRecovery(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules.Get(rust) returned nil")
		}

		if !r.AreTypesEquivalent("call_expression", "macro_invocation") {
			t.Errorf("expected call_expression and macro_invocation to match in rust rules")
		}
		if !r.AreTypesEquivalent("arguments", "token_tree") {
			t.Errorf("expected arguments and token_tree to match in rust rules")
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn parse_options() {
    let x = 1;
    if condition {
        eprintln!("Invalid command-line option: --foo");
        process::exit(1);
    }
    let y = 2;
}
`
		newSrc := `fn parse_options() {
    let x = 1;
    if condition {
        fatal("Invalid command-line option: --foo");
    }
    let y = 2;
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

		// 1. Enclosing context: the function and if expression should stay mapped in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "if_expression", Line: 3}, testutil.NodeQuery{Type: "if_expression", Line: 3})

		// 2. Surrounding statements: let statements around the if block shouldn't drift.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 2}, testutil.NodeQuery{Type: "let_declaration", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 7}, testutil.NodeQuery{Type: "let_declaration", Line: 6})

		// 3. Target & containers: eprintln! should map to fatal(), token_tree should map to arguments, and process::exit should stay unmapped (deleted).
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "macro_invocation", Line: 4}, testutil.NodeQuery{Type: "call_expression", Line: 4})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "token_tree", Line: 4}, testutil.NodeQuery{Type: "arguments", Line: 4})

		// 4. Stationary operands: the error message string literal should map in place across the macro and call.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "string_literal", Line: 4}, testutil.NodeQuery{Type: "string_literal", Line: 4})
	})
}

func TestZigBuiltinCallEquivalenceRecovery(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("zig")
		if r == nil {
			t.Fatal("rules.Get(zig) returned nil")
		}

		if !r.AreTypesEquivalent("builtin_function", "call_expression") {
			t.Errorf("expected builtin_function and call_expression to match in zig rules")
		}
		if !r.AreTypesEquivalent("builtin_identifier", "identifier") {
			t.Errorf("expected builtin_identifier and identifier to match in zig rules")
		}
		if !r.IsCall("builtin_function") {
			t.Errorf("expected builtin_function to be recognized as a call in zig rules")
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `pub fn parse_options() void {
    const x = 1;
    if (condition) {
        @panic("Invalid command-line option: --foo");
        process.exit(1);
    }
    const y = 2;
}
`
		newSrc := `pub fn parse_options() void {
    const x = 1;
    if (condition) {
        fatal("Invalid command-line option: --foo");
    }
    const y = 2;
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.zig")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.zig")
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

		// 1. Enclosing context: the function and if statement should stay mapped in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "if_statement", Line: 3}, testutil.NodeQuery{Type: "if_statement", Line: 3})

		// 2. Surrounding statements: const statements around the if block shouldn't drift.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "variable_declaration", Line: 2}, testutil.NodeQuery{Type: "variable_declaration", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "variable_declaration", Line: 7}, testutil.NodeQuery{Type: "variable_declaration", Line: 6})

		// 3. Target & containers: @panic should map to fatal(), while process.exit stays unmapped (deleted).
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "builtin_function", Line: 4}, testutil.NodeQuery{Type: "call_expression", Line: 4})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 5})

		// 4. Stationary operands: the panic message string should map in place across @panic and fatal.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "string", Line: 4}, testutil.NodeQuery{Type: "string", Line: 4})
	})
}

// TestRustPairValueCallToMacroReplacement verifies that replacing a method call with
// a macro invocation inside a struct field initializer bypasses Zhang-Shasha, cleanly
// deleting the call expression without leaving call parentheses unhighlighted.
func TestRustPairValueCallToMacroReplacement(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: "→".to_string(),
        zero_style: opt.zero_style,
    }
}
`
		newSrc := `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: format!("{}→{}", 1, 2),
        zero_style: opt.zero_style,
    }
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

		// 1. Enclosing Context: enclosing function and its body block map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "struct_expression", Line: 2}, testutil.NodeQuery{Type: "struct_expression", Line: 2})

		// 2. Surrounding Sequence: preceding tab_width and succeeding zero_style fields map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "field_initializer", Line: 3}, testutil.NodeQuery{Type: "field_initializer", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "field_initializer", Line: 5}, testutil.NodeQuery{Type: "field_initializer", Line: 5})

		// 3. Target & Containers:
		// Target pair truncation_symbol maps to truncation_symbol, along with key field_identifier.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "field_initializer", Line: 4}, testutil.NodeQuery{Type: "field_initializer", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "field_identifier", Line: 4, Nth: 1}, testutil.NodeQuery{Type: "field_identifier", Line: 4, Nth: 1})

		// Replaced value: call_expression ("→".to_string()) and arguments (()) must remain unmapped,
		// and macro_invocation (format!(...)) and token_tree must remain unmapped.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 4})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "arguments", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "macro_invocation", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "token_tree", Line: 4})
	})
}

// TestTypeScriptPairValueCallReplacement verifies that replacing a call expression
// with a template string inside an object literal pair bypasses Zhang-Shasha,
// cleanly aligning the pair keys while deleting the call.
func TestTypeScriptPairValueCallReplacement(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `function makeConfig(opt: Options): Config {
    return {
        tabWidth: opt.tabWidth,
        truncationSymbol: opt.getSymbol(),
        zeroStyle: opt.zeroStyle,
    };
}
`
		newSrc := "function makeConfig(opt: Options): Config {\n" +
			"    return {\n" +
			"        tabWidth: opt.tabWidth,\n" +
			"        truncationSymbol: `${1}→${2}`,\n" +
			"        zeroStyle: opt.zeroStyle,\n" +
			"    };\n" +
			"}\n"
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
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
		// 1. Enclosing Context: function makeConfig maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "object", Line: 2}, testutil.NodeQuery{Type: "object", Line: 2})

		// 2. Surrounding Sequence: preceding tabWidth and succeeding zeroStyle pairs map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 3}, testutil.NodeQuery{Type: "pair", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 5}, testutil.NodeQuery{Type: "pair", Line: 5})

		// 3. Target & Containers: truncationSymbol pair maps across both trees.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "pair", Line: 4}, testutil.NodeQuery{Type: "pair", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "property_identifier", Line: 4, Nth: 1}, testutil.NodeQuery{Type: "property_identifier", Line: 4, Nth: 1})

		// Replaced value: call_expression (opt.getSymbol()) and arguments (()) remain unmapped,
		// and template_string remains unmapped.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 4})
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "arguments", Line: 4})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "template_string", Line: 4})
	})
}

// TestTSXJSXAttributeValueReplacement verifies that replacing a call expression
// with an arrow function inside a JSX attribute bypasses Zhang-Shasha, cleanly
// deleting the call without falsely matching the nested arrow function body.
func TestTSXJSXAttributeValueReplacement(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `function App() {
    return (
        <Button
            variant="primary"
            onClick={handleClick()}
            disabled={false}
        />
    );
}
`
		newSrc := `function App() {
    return (
        <Button
            variant="primary"
            onClick={() => handle()}
            disabled={false}
        />
    );
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "component.tsx")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "component.tsx")
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
		// 1. Enclosing Context: App function declaration maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})

		// 2. Surrounding Sequence: preceding variant and succeeding disabled attributes map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "jsx_attribute", Line: 4}, testutil.NodeQuery{Type: "jsx_attribute", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "jsx_attribute", Line: 6}, testutil.NodeQuery{Type: "jsx_attribute", Line: 6})

		// 3. Target & Containers: onClick JSX attribute maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "jsx_attribute", Line: 5}, testutil.NodeQuery{Type: "jsx_attribute", Line: 5})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "property_identifier", Line: 5, Nth: 1}, testutil.NodeQuery{Type: "property_identifier", Line: 5, Nth: 1})

		// Replaced expression: handleClick() call should remain unmapped (deleted),
		// and () => handle() arrow function should remain unmapped (inserted).
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 5})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "arrow_function", Line: 5})
	})
}

func TestRecovery_LocalVarDeclarationTheft_FullEngine(t *testing.T) {
	t.Run("Rust_UnrelatedLetDeclarationsAcrossStatements_Rejected", func(t *testing.T) {
		oldSrc := `fn set_widths() {
    opt.available_terminal_width = Term::stdout().size().1;
    let empty_builtin_features = HashMap::new();
}
`
		newSrc := `fn set_widths() {
    let term_stdout = Term::stdout();
    opt.available_terminal_width = term_stdout.size().1;
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

		// 1. Enclosing Context: enclosing function maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})

		// 2. Surrounding Sequence: opt.available_terminal_width assignment statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 2}, testutil.NodeQuery{Type: "expression_statement", Line: 3})

		// 3. Target: unrelated let declarations don't pair across statements.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 3})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "let_declaration", Line: 2})
	})

	t.Run("Go_UnrelatedShortVarDeclarationsAcrossStatements_Rejected", func(t *testing.T) {
		oldSrc := `package main

func setWidths() {
	opt.availableTerminalWidth = TermStdout().size().one
	emptyBuiltinFeatures := HashMapNew()
}
`
		newSrc := `package main

func setWidths() {
	termStdout := TermStdout()
	opt.availableTerminalWidth = termStdout.size().one
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.go")
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

		// 1. Enclosing Context: func setWidths maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 3}, testutil.NodeQuery{Type: "function_declaration", Line: 3})

		// 2. Surrounding Sequence: availableTerminalWidth assignment statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "assignment_statement", Line: 4}, testutil.NodeQuery{Type: "assignment_statement", Line: 5})

		// 3. Target: unrelated short var declarations don't pair across statements.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "short_var_declaration", Line: 5})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "short_var_declaration", Line: 4})
	})

	t.Run("Rust_LegitimateInPlaceLocalVarRename_Maps", func(t *testing.T) {
		oldSrc := `fn compute() {
    let old_value = calculate_total(a, b);
    process(old_value);
}
`
		newSrc := `fn compute() {
    let new_value = calculate_total(a, b);
    process(new_value);
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

		// 1. Enclosing Context: function item maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})

		// 2. Surrounding Sequence: process(value) statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 3}, testutil.NodeQuery{Type: "expression_statement", Line: 3})

		// 3. Target: in-place renamed let_declaration maps across versions.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 2}, testutil.NodeQuery{Type: "let_declaration", Line: 2})
	})

	t.Run("TypeScript_UnrelatedLexicalDeclarationsAcrossStatements_Rejected", func(t *testing.T) {
		oldSrc := `function setWidths() {
    opt.availableTerminalWidth = Term.stdout().size().one;
    const emptyBuiltinFeatures = HashMap.create();
}
`
		newSrc := `function setWidths() {
    const termStdout = Term.stdout();
    opt.availableTerminalWidth = termStdout.size().one;
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
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

		// 1. Enclosing Context: enclosing function maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})

		// 2. Surrounding Sequence: availableTerminalWidth expression statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 2}, testutil.NodeQuery{Type: "expression_statement", Line: 3})

		// 3. Target: unrelated lexical declarations don't pair across statements.
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "lexical_declaration", Line: 3})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "lexical_declaration", Line: 2})
	})

	t.Run("TypeScript_LegitimateInPlaceConstRename_Maps", func(t *testing.T) {
		oldSrc := `function compute() {
    const oldValue = calculateTotal(a, b);
    process(oldValue);
}
`
		newSrc := `function compute() {
    const newValue = calculateTotal(a, b);
    process(newValue);
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
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

		// 1. Enclosing Context: enclosing function maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_declaration", Line: 1}, testutil.NodeQuery{Type: "function_declaration", Line: 1})

		// 2. Surrounding Sequence: process call statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 3}, testutil.NodeQuery{Type: "expression_statement", Line: 3})

		// 3. Target: in-place renamed lexical_declaration maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "lexical_declaration", Line: 2}, testutil.NodeQuery{Type: "lexical_declaration", Line: 2})
	})
}

func TestRecovery_IfInitConditionFastPath_FullEngine(t *testing.T) {
	oldSrc := `package main

func evaluate(val int) int {
	startMetrics()
	if val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`
	newSrc := `package main

func evaluate(val int) int {
	startMetrics()
	if err := validate(val); val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`
	srcBytes := []byte(oldSrc)
	dstBytes := []byte(newSrc)

	srcAST, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	if srcAST.HasError || srcAST.ParseErrorCount > 0 {
		t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.go")
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
	revMapped := mr.Mappings.Dst()

	// 1. Enclosing Context: evaluate function maps in place.
	var srcFn, dstFn *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "evaluate") {
			srcFn = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "evaluate") {
			dstFn = n
			break
		}
	}
	if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
		t.Fatalf("expected evaluate fn to map in place, got src=%v dst=%v mapped=%v", srcFn, dstFn, mapped[srcFn])
	}

	// 2. Surrounding Sequence: startMetrics and endMetrics map in place.
	var srcStart, dstStart, srcEnd, dstEnd *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "startMetrics") {
				srcStart = n
			} else if strings.Contains(text, "endMetrics") {
				srcEnd = n
			}
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "startMetrics") {
				dstStart = n
			} else if strings.Contains(text, "endMetrics") {
				dstEnd = n
			}
		}
	}
	if srcStart == nil || dstStart == nil || mapped[srcStart] != dstStart {
		t.Fatalf("expected startMetrics to map in place, got %v", mapped[srcStart])
	}
	if srcEnd == nil || dstEnd == nil || mapped[srcEnd] != dstEnd {
		t.Fatalf("expected endMetrics to map in place, got %v", mapped[srcEnd])
	}

	// 3. Target: if_statement maps in place.
	var srcIf, dstIf *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if n.Type == "if_statement" {
			srcIf = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if n.Type == "if_statement" {
			dstIf = n
			break
		}
	}
	if srcIf == nil || dstIf == nil || mapped[srcIf] != dstIf {
		t.Fatalf("expected if_statement to map in place, got src=%v dst=%v mapped=%v", srcIf, dstIf, mapped[srcIf])
	}

	// 4. Condition: val > 0 maps in place.
	srcCond := srcIf.ChildByFieldName("condition")
	dstCond := dstIf.ChildByFieldName("condition")
	if srcCond == nil || dstCond == nil || mapped[srcCond] != dstCond {
		t.Fatalf("expected condition (val > 0) to map in place, got srcCond=%v dstCond=%v mapped=%v", srcCond, dstCond, mapped[srcCond])
	}

	// 5. Stationary inner return statement maps in place.
	var srcRet, dstRet *treesitter.ASTNode
	for _, n := range srcIf.Descendants() {
		if r.IsJumpStatement(n.Type) {
			srcRet = n
			break
		}
	}
	for _, n := range dstIf.Descendants() {
		if r.IsJumpStatement(n.Type) {
			dstRet = n
			break
		}
	}
	if srcRet == nil || dstRet == nil || mapped[srcRet] != dstRet {
		t.Fatalf("expected return statement inside if to map in place, got srcRet=%v dstRet=%v mapped=%v", srcRet, dstRet, mapped[srcRet])
	}

	// 6. The newly added init statement in dstIf must not steal an existing mapping.
	for _, n := range dstIf.Children {
		if r.IsLocalVarDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "validate") {
			if revMapped[n] != nil {
				t.Errorf("expected inserted init statement to remain unmapped, got %v", revMapped[n])
			}
		}
	}
}

func TestRecovery_CrossBranchStatementMove_FullEngine(t *testing.T) {
	oldSrc := `package main

func handleBranch(flag bool, val int) {
	header()
	if flag {
		fallback()
	} else {
		compute(val)
		cleanup()
	}
	footer()
}
`
	newSrc := `package main

func handleBranch(flag bool, val int) {
	header()
	if flag {
		compute(val)
	} else {
		fallback()
		cleanup()
	}
	footer()
}
`
	srcBytes := []byte(oldSrc)
	dstBytes := []byte(newSrc)

	srcAST, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	if srcAST.HasError || srcAST.ParseErrorCount > 0 {
		t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.go")
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

	// 1. Enclosing function handleBranch maps in place.
	var srcFn, dstFn *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "handleBranch") {
			srcFn = n
			break
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "handleBranch") {
			dstFn = n
			break
		}
	}
	if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
		t.Fatalf("expected handleBranch fn to map in place, got %v", mapped[srcFn])
	}

	// 2. Surrounding Sequence: header, footer, and if_statement map in place.
	var srcHeader, dstHeader, srcFooter, dstFooter, srcIf, dstIf *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "header") {
				srcHeader = n
			} else if strings.Contains(text, "footer") {
				srcFooter = n
			}
		}
		if n.Type == "if_statement" {
			srcIf = n
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "header") {
				dstHeader = n
			} else if strings.Contains(text, "footer") {
				dstFooter = n
			}
		}
		if n.Type == "if_statement" {
			dstIf = n
		}
	}
	if srcHeader == nil || dstHeader == nil || mapped[srcHeader] != dstHeader {
		t.Fatalf("expected header to map in place, got %v", mapped[srcHeader])
	}
	if srcFooter == nil || dstFooter == nil || mapped[srcFooter] != dstFooter {
		t.Fatalf("expected footer to map in place, got %v", mapped[srcFooter])
	}
	if srcIf == nil || dstIf == nil || mapped[srcIf] != dstIf {
		t.Fatalf("expected if_statement to map in place, got %v", mapped[srcIf])
	}

	// 3. Moved statements across branches map to each other (cross-branch move).
	var srcCompute, dstCompute, srcFallback, dstFallback *treesitter.ASTNode
	for _, n := range srcAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "compute") {
				srcCompute = n
			} else if strings.Contains(text, "fallback") {
				srcFallback = n
			}
		}
	}
	for _, n := range dstAST.Descendants() {
		if r.IsStatement(n.Type) {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "compute") {
				dstCompute = n
			} else if strings.Contains(text, "fallback") {
				dstFallback = n
			}
		}
	}
	if srcCompute == nil || dstCompute == nil || mapped[srcCompute] != dstCompute {
		t.Fatalf("expected compute statement to map across branches, got %v", mapped[srcCompute])
	}
	if srcFallback == nil || dstFallback == nil || mapped[srcFallback] != dstFallback {
		t.Fatalf("expected fallback statement to map across branches, got %v", mapped[srcFallback])
	}
}

func TestSandwichedStatementShiftedIntervalRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn test_logging(val: &str) {
    setup_env();
    print!("value: {}", val);
    cleanup_env();
}
`
		newSrc := `fn test_logging(val: &str) {
    init_tracing();
    setup_env();
    write!(io::stdout(), "value: {}", val);
    cleanup_env();
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

		// 1. Enclosing Context: enclosing function maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "test_logging") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "test_logging") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Fatalf("expected test_logging fn to map in place, got src=%v dst=%v mapped=%v", srcFn, dstFn, mapped[srcFn])
		}

		// 2. Surrounding Sequence: anchor statements map in place.
		var srcSetup, dstSetup *treesitter.ASTNode
		var srcCleanup, dstCleanup *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) {
				if strings.Contains(text, "setup_env") {
					srcSetup = n
				} else if strings.Contains(text, "cleanup_env") {
					srcCleanup = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) {
				if strings.Contains(text, "setup_env") {
					dstSetup = n
				} else if strings.Contains(text, "cleanup_env") {
					dstCleanup = n
				}
			}
		}

		if srcSetup == nil || dstSetup == nil || mapped[srcSetup] != dstSetup {
			t.Fatalf("expected setup_env() statement to map in place, got %v", mapped[srcSetup])
		}
		if srcCleanup == nil || dstCleanup == nil || mapped[srcCleanup] != dstCleanup {
			t.Fatalf("expected cleanup_env() statement to map in place, got %v", mapped[srcCleanup])
		}

		// 3. Target Sandwiched Statement:
		// print! statement must map to write! statement across the shifted index interval.
		var srcPrintStmt, dstWriteStmt *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) && strings.HasPrefix(text, "print!") {
				srcPrintStmt = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) && strings.HasPrefix(text, "write!") {
				dstWriteStmt = n
			}
		}

		if srcPrintStmt == nil || dstWriteStmt == nil || mapped[srcPrintStmt] != dstWriteStmt {
			t.Fatalf("expected print! expression_statement to map to write! expression_statement across shifted interval, got %v", mapped[srcPrintStmt])
		}

		// 4. Stationary Operands: format string literal "value: {}" maps in place.
		var srcFmtStr, dstFmtStr *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "value: {}") {
				srcFmtStr = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "value: {}") {
				dstFmtStr = n
				break
			}
		}
		if srcFmtStr == nil || dstFmtStr == nil || mapped[srcFmtStr] != dstFmtStr {
			t.Errorf("expected 'value: {}' format string to map in place, got %v", mapped[srcFmtStr])
		}
	})
}

func TestRustSandwichedStatementTryWrapperRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    print!("Writing metadata to folder {} ... ", target_dir.to_string_lossy());
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    println!("okay");
    Ok(())
}
`
		newSrc := `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    write!(io::stdout(), "Writing metadata to folder {} ... ", target_dir.to_string_lossy())?;
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    writeln!(io::stdout(), "okay")?;
    Ok(())
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

		// 1. Enclosing Context: enclosing function, its parameters, and body block map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})

		// 2. Surrounding Sequence: anchor statements map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 2}, testutil.NodeQuery{Type: "let_declaration", Line: 2})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 4}, testutil.NodeQuery{Type: "expression_statement", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 6}, testutil.NodeQuery{Type: "call_expression", Line: 6})

		// 3. Target Sandwiched Statements:
		// Statement 1: print! -> write!
		// Statement 2: println! -> writeln!
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 3}, testutil.NodeQuery{Type: "expression_statement", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 5}, testutil.NodeQuery{Type: "expression_statement", Line: 5})

		// 4. Transparent Wrapper Unwrapping:
		// The inner macro_invocations must map across the try_expression wrapper.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "macro_invocation", Line: 3}, testutil.NodeQuery{Type: "macro_invocation", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "macro_invocation", Line: 5}, testutil.NodeQuery{Type: "macro_invocation", Line: 5})

		// Outer try_expressions must remain unmapped so they become inserted wrappers.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "try_expression", Line: 3})
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(), testutil.NodeQuery{Type: "try_expression", Line: 5})

		// 5. Stationary Operands:
		// String literal "okay" must map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "string_literal", Line: 5}, testutil.NodeQuery{Type: "string_literal", Line: 5})
	})
}

func TestRustMultiStatementIntervalRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `fn generate_config_file() -> Result<()> {
    let config_file = config_file();
    if config_file.is_file() {
        println!("A config file already exists at: {}", config_file.to_string_lossy());
        print!("Overwrite? (y/N): ");
        io::stdout().flush()?;
        let mut decision = String::new();
        io::stdin().read_line(&mut decision)?;
    }
    Ok(())
}
`
		newSrc := `fn generate_config_file() -> Result<()> {
    let config_file = config_file();
    if config_file.is_file() {
        writeln!(io::stdout(), "A config file already exists at: {}", config_file.to_string_lossy())?;
        write!(io::stdout(), "Overwrite? (y/N): ")?;
        io::stdout().flush()?;
        let mut decision = String::new();
        io::stdin().read_line(&mut decision)?;
    }
    Ok(())
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

		// 1. Enclosing Context: fn generate_config_file maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "function_item", Line: 1}, testutil.NodeQuery{Type: "function_item", Line: 1})

		// 2. Surrounding Sequence: if_statement and inner statements map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "if_expression", Line: 3}, testutil.NodeQuery{Type: "if_expression", Line: 3})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 6}, testutil.NodeQuery{Type: "expression_statement", Line: 6})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "let_declaration", Line: 7}, testutil.NodeQuery{Type: "let_declaration", Line: 7})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 8}, testutil.NodeQuery{Type: "expression_statement", Line: 8})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "call_expression", Line: 10}, testutil.NodeQuery{Type: "call_expression", Line: 10})

		// 3. Multi-statement Leading Interval Pairing:
		// println! and print! statements in the leading interval must map 1:1 to writeln! and write!.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 4}, testutil.NodeQuery{Type: "expression_statement", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "expression_statement", Line: 5}, testutil.NodeQuery{Type: "expression_statement", Line: 5})

		// 4. Stationary Operands: format string literals map in place without disruption.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "string_literal", Line: 4}, testutil.NodeQuery{Type: "string_literal", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(), testutil.NodeQuery{Type: "string_literal", Line: 5}, testutil.NodeQuery{Type: "string_literal", Line: 5})
	})
}

func TestZigSandwichedStatementTryWrapperRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `pub fn run(writer: anytype) !void {
    init();
    writer.print("foo\n", .{});
    anchor();
    writer.println("bar\n", .{});
    finish();
}
`
		newSrc := `pub fn run(writer: anytype) !void {
    init();
    try writer.write("foo\n", .{});
    anchor();
    try writer.writeln("bar\n", .{});
    finish();
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.zig")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.zig")
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
		revMapped := mr.Mappings.Dst()

		// 1. Enclosing Context: run function declaration maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "run") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "run") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Fatalf("expected run fn to map in place, got src=%v dst=%v mapped=%v", srcFn, dstFn, mapped[srcFn])
		}

		// 2. Surrounding Sequence: init, anchor, and finish statements map in place.
		var srcInit, dstInit *treesitter.ASTNode
		var srcAnchor, dstAnchor *treesitter.ASTNode
		var srcFinish, dstFinish *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := strings.TrimSpace(string(srcBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "init(") && r.IsStatement(n.Type) {
				srcInit = n
			} else if strings.HasPrefix(text, "anchor(") && r.IsStatement(n.Type) {
				srcAnchor = n
			} else if strings.HasPrefix(text, "finish(") && r.IsStatement(n.Type) {
				srcFinish = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := strings.TrimSpace(string(dstBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "init(") && r.IsStatement(n.Type) {
				dstInit = n
			} else if strings.HasPrefix(text, "anchor(") && r.IsStatement(n.Type) {
				dstAnchor = n
			} else if strings.HasPrefix(text, "finish(") && r.IsStatement(n.Type) {
				dstFinish = n
			}
		}

		if srcInit == nil || dstInit == nil || mapped[srcInit] != dstInit {
			t.Errorf("expected init statement to map in place, got %v", mapped[srcInit])
		}
		if srcAnchor == nil || dstAnchor == nil || mapped[srcAnchor] != dstAnchor {
			t.Errorf("expected anchor statement to map in place, got %v", mapped[srcAnchor])
		}
		if srcFinish == nil || dstFinish == nil || mapped[srcFinish] != dstFinish {
			t.Errorf("expected finish statement to map in place, got %v", mapped[srcFinish])
		}

		// 3. Target Sandwiched Statements:
		// writer.print statement maps to try writer.write statement.
		// writer.println statement maps to try writer.writeln statement.
		var srcPrintStmt, dstWriteStmt *treesitter.ASTNode
		var srcPrintlnStmt, dstWritelnStmt *treesitter.ASTNode
		var srcPrintCall, dstWriteCall *treesitter.ASTNode
		var srcPrintlnCall, dstWritelnCall *treesitter.ASTNode
		var dstTry1, dstTry2 *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) {
				if strings.Contains(text, "writer.print(") {
					srcPrintStmt = n
				} else if strings.Contains(text, "writer.println(") {
					srcPrintlnStmt = n
				}
			}
			if r.IsCall(n.Type) {
				if strings.Contains(text, "writer.print(") {
					srcPrintCall = n
				} else if strings.Contains(text, "writer.println(") {
					srcPrintlnCall = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if r.IsStatement(n.Type) {
				if strings.Contains(text, "writer.write(") {
					dstWriteStmt = n
				} else if strings.Contains(text, "writer.writeln(") {
					dstWritelnStmt = n
				}
			}
			if r.IsCall(n.Type) {
				if strings.Contains(text, "writer.write(") {
					dstWriteCall = n
				} else if strings.Contains(text, "writer.writeln(") {
					dstWritelnCall = n
				}
			}
			if r.IsWrapper(n.Type) && n.Type == "try_expression" {
				if dstTry1 == nil {
					dstTry1 = n
				} else {
					dstTry2 = n
				}
			}
		}

		if srcPrintStmt == nil || dstWriteStmt == nil || mapped[srcPrintStmt] != dstWriteStmt {
			t.Fatalf("expected writer.print expression_statement to map in place, got %v", mapped[srcPrintStmt])
		}
		if srcPrintlnStmt == nil || dstWritelnStmt == nil || mapped[srcPrintlnStmt] != dstWritelnStmt {
			t.Fatalf("expected writer.println expression_statement to map in place, got %v", mapped[srcPrintlnStmt])
		}

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expressions map across try_expression wrappers.
		if srcPrintCall == nil || dstWriteCall == nil || mapped[srcPrintCall] != dstWriteCall {
			t.Errorf("expected writer.print call_expression to map across try wrapper, got %v", mapped[srcPrintCall])
		}
		if srcPrintlnCall == nil || dstWritelnCall == nil || mapped[srcPrintlnCall] != dstWritelnCall {
			t.Errorf("expected writer.println call_expression to map across try wrapper, got %v", mapped[srcPrintlnCall])
		}
		if dstTry1 != nil && revMapped[dstTry1] != nil {
			t.Errorf("expected dst try_expression 1 to remain unmapped, got %v", revMapped[dstTry1])
		}
		if dstTry2 != nil && revMapped[dstTry2] != nil {
			t.Errorf("expected dst try_expression 2 to remain unmapped, got %v", revMapped[dstTry2])
		}

		// 5. Stationary Operands:
		// Format string literals "foo\n" and "bar\n" map in place.
		var srcFoo, dstFoo *treesitter.ASTNode
		var srcBar, dstBar *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "foo") && (n.Type == "string" || n.Type == "string_content") {
				srcFoo = n
			} else if strings.Contains(text, "bar") && (n.Type == "string" || n.Type == "string_content") {
				srcBar = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "foo") && (n.Type == "string" || n.Type == "string_content") {
				dstFoo = n
			} else if strings.Contains(text, "bar") && (n.Type == "string" || n.Type == "string_content") {
				dstBar = n
			}
		}

		if srcFoo != nil && dstFoo != nil && mapped[srcFoo] != dstFoo {
			t.Errorf("expected 'foo' string to map in place, got %v", mapped[srcFoo])
		}
		if srcBar != nil && dstBar != nil && mapped[srcBar] != dstBar {
			t.Errorf("expected 'bar' string to map in place, got %v", mapped[srcBar])
		}
	})
}

func TestCParenthesizedStatementRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `void handle_transaction(Database* db) {
    db_begin(db);
    db_flush_wal(db);
    db_commit(db);
}
`
		newSrc := `void handle_transaction(Database* db) {
    db_begin(db);
    (db_flush_wal(db));
    db_commit(db);
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.c")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.c")
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
		revMapped := mr.Mappings.Dst()

		// 1. Enclosing Context: handle_transaction function maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "handle_transaction") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "handle_transaction") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Fatalf("expected handle_transaction fn to map in place, got %v", mapped[srcFn])
		}

		// 2. Surrounding Sequence: surrounding statements map in place.
		var srcBegin, dstBegin *treesitter.ASTNode
		var srcCommit, dstCommit *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := string(srcBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "db_begin") && r.IsStatement(n.Type) {
				srcBegin = n
			} else if strings.Contains(text, "db_commit") && r.IsStatement(n.Type) {
				srcCommit = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := string(dstBytes[n.StartByte:n.EndByte])
			if strings.Contains(text, "db_begin") && r.IsStatement(n.Type) {
				dstBegin = n
			} else if strings.Contains(text, "db_commit") && r.IsStatement(n.Type) {
				dstCommit = n
			}
		}

		if srcBegin == nil || dstBegin == nil || mapped[srcBegin] != dstBegin {
			t.Errorf("expected db_begin statement to map in place, got %v", mapped[srcBegin])
		}
		if srcCommit == nil || dstCommit == nil || mapped[srcCommit] != dstCommit {
			t.Errorf("expected db_commit statement to map in place, got %v", mapped[srcCommit])
		}

		// 3. Target Sandwiched Statement:
		// db_flush_wal statement maps in place across parenthesized expression.
		var srcFlushStmt, dstFlushStmt *treesitter.ASTNode
		var srcFlushCall, dstFlushCall *treesitter.ASTNode
		var dstParen *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "db_flush_wal") {
				srcFlushStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "db_flush_wal") {
				srcFlushCall = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "db_flush_wal") {
				dstFlushStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "db_flush_wal") {
				dstFlushCall = n
			}
			if r.IsWrapper(n.Type) && n.Type == "parenthesized_expression" {
				dstParen = n
			}
		}

		if srcFlushStmt == nil || dstFlushStmt == nil || mapped[srcFlushStmt] != dstFlushStmt {
			t.Fatalf("expected db_flush_wal expression_statement to map in place, got %v", mapped[srcFlushStmt])
		}

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expression maps across the parenthesized_expression wrapper.
		if srcFlushCall == nil || dstFlushCall == nil || mapped[srcFlushCall] != dstFlushCall {
			t.Errorf("expected db_flush_wal call_expression to map across paren wrapper, got %v", mapped[srcFlushCall])
		}
		if dstParen != nil && revMapped[dstParen] != nil {
			t.Errorf("expected dst parenthesized_expression to remain unmapped, got %v", revMapped[dstParen])
		}

		// 5. Stationary Operands:
		// db argument identifier maps in place.
		var srcArg, dstArg *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsIdentifier(n.Type) && string(srcBytes[n.StartByte:n.EndByte]) == "db" && n.Parent == srcFlushCall {
				srcArg = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsIdentifier(n.Type) && string(dstBytes[n.StartByte:n.EndByte]) == "db" && n.Parent == dstFlushCall {
				dstArg = n
				break
			}
		}
		if srcArg != nil && dstArg != nil && mapped[srcArg] != dstArg {
			t.Errorf("expected 'db' identifier argument to map in place, got %v", mapped[srcArg])
		}
	})
}

func TestJavaScriptSandwichedStatementAwaitWrapperRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `async function syncSession(client) {
    client.connect();
    client.fetchUpdates();
    client.disconnect();
}
`
		newSrc := `async function syncSession(client) {
    client.connect();
    await client.fetchUpdates();
    client.disconnect();
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.js")
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
		revMapped := mr.Mappings.Dst()

		// 1. Enclosing Context: syncSession function declaration maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "syncSession") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "syncSession") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Fatalf("expected syncSession fn to map in place, got %v", mapped[srcFn])
		}

		// 2. Surrounding Sequence: connect and disconnect statements map in place.
		var srcConn, dstConn *treesitter.ASTNode
		var srcDisc, dstDisc *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := strings.TrimSpace(string(srcBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "client.connect") && r.IsStatement(n.Type) {
				srcConn = n
			} else if strings.HasPrefix(text, "client.disconnect") && r.IsStatement(n.Type) {
				srcDisc = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := strings.TrimSpace(string(dstBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "client.connect") && r.IsStatement(n.Type) {
				dstConn = n
			} else if strings.HasPrefix(text, "client.disconnect") && r.IsStatement(n.Type) {
				dstDisc = n
			}
		}

		if srcConn == nil || dstConn == nil || mapped[srcConn] != dstConn {
			t.Errorf("expected connect statement to map in place, got %v", mapped[srcConn])
		}
		if srcDisc == nil || dstDisc == nil || mapped[srcDisc] != dstDisc {
			t.Errorf("expected disconnect statement to map in place, got %v", mapped[srcDisc])
		}

		// 3. Target Sandwiched Statement:
		// fetchUpdates statement maps across await_expression.
		var srcFetchStmt, dstFetchStmt *treesitter.ASTNode
		var srcFetchCall, dstFetchCall *treesitter.ASTNode
		var dstAwait *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "fetchUpdates") {
				srcFetchStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "fetchUpdates") {
				srcFetchCall = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "fetchUpdates") {
				dstFetchStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "fetchUpdates") {
				dstFetchCall = n
			}
			if r.IsWrapper(n.Type) && n.Type == "await_expression" {
				dstAwait = n
			}
		}

		if srcFetchStmt == nil || dstFetchStmt == nil || mapped[srcFetchStmt] != dstFetchStmt {
			t.Fatalf("expected fetchUpdates expression_statement to map in place, got %v", mapped[srcFetchStmt])
		}

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expression maps across the await_expression wrapper.
		if srcFetchCall == nil || dstFetchCall == nil || mapped[srcFetchCall] != dstFetchCall {
			t.Errorf("expected fetchUpdates call_expression to map across await wrapper, got %v", mapped[srcFetchCall])
		}
		if dstAwait != nil && revMapped[dstAwait] != nil {
			t.Errorf("expected dst await_expression to remain unmapped, got %v", revMapped[dstAwait])
		}
	})
}

func TestPythonSandwichedStatementAwaitWrapperRecovery(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `async def sync_session(client):
    client.connect()
    client.fetch_updates()
    client.disconnect()
`
		newSrc := `async def sync_session(client):
    client.connect()
    await client.fetch_updates()
    client.disconnect()
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "test.py")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src snippet must parse cleanly: hasError=%v errors=%d", srcAST.HasError, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.py")
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
		revMapped := mr.Mappings.Dst()

		// 1. Enclosing Context: sync_session function definition maps in place.
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "sync_session") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "sync_session") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mapped[srcFn] != dstFn {
			t.Fatalf("expected sync_session fn to map in place, got %v", mapped[srcFn])
		}

		// 2. Surrounding Sequence: connect and disconnect statements map in place.
		var srcConn, dstConn *treesitter.ASTNode
		var srcDisc, dstDisc *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			text := strings.TrimSpace(string(srcBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "client.connect") && r.IsStatement(n.Type) {
				srcConn = n
			} else if strings.HasPrefix(text, "client.disconnect") && r.IsStatement(n.Type) {
				srcDisc = n
			}
		}
		for _, n := range dstAST.Descendants() {
			text := strings.TrimSpace(string(dstBytes[n.StartByte:n.EndByte]))
			if strings.HasPrefix(text, "client.connect") && r.IsStatement(n.Type) {
				dstConn = n
			} else if strings.HasPrefix(text, "client.disconnect") && r.IsStatement(n.Type) {
				dstDisc = n
			}
		}

		if srcConn == nil || dstConn == nil || mapped[srcConn] != dstConn {
			t.Errorf("expected connect statement to map in place, got %v", mapped[srcConn])
		}
		if srcDisc == nil || dstDisc == nil || mapped[srcDisc] != dstDisc {
			t.Errorf("expected disconnect statement to map in place, got %v", mapped[srcDisc])
		}

		// 3. Target Sandwiched Statement:
		// fetch_updates statement maps across await wrapper.
		var srcFetchStmt, dstFetchStmt *treesitter.ASTNode
		var srcFetchCall, dstFetchCall *treesitter.ASTNode
		var dstAwait *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "fetch_updates") {
				srcFetchStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(srcBytes[n.StartByte:n.EndByte]), "fetch_updates") {
				srcFetchCall = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "fetch_updates") {
				dstFetchStmt = n
			}
			if r.IsCall(n.Type) && strings.Contains(string(dstBytes[n.StartByte:n.EndByte]), "fetch_updates") {
				dstFetchCall = n
			}
			if r.IsWrapper(n.Type) && n.Type == "await" {
				dstAwait = n
			}
		}

		if srcFetchStmt == nil || dstFetchStmt == nil || mapped[srcFetchStmt] != dstFetchStmt {
			t.Fatalf("expected fetch_updates expression_statement to map in place, got %v", mapped[srcFetchStmt])
		}

		// 4. Transparent Wrapper Unwrapping:
		// Inner call maps across the await wrapper.
		if srcFetchCall == nil || dstFetchCall == nil || mapped[srcFetchCall] != dstFetchCall {
			t.Errorf("expected fetch_updates call to map across await wrapper, got %v", mapped[srcFetchCall])
		}
		if dstAwait != nil && revMapped[dstAwait] != nil {
			t.Errorf("expected dst await to remain unmapped, got %v", revMapped[dstAwait])
		}
	})
}
