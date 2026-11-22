package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestSimpleRecoveryLabelMatch(t *testing.T) {
	// SimpleRecovery maps isomorphic children of mapped parents.
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() { a() }"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() { a() }"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	dstRoot, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcChild, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "expression_statement", Line: 2})
	dstChild, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "expression_statement", Line: 2})

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
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() { a() }"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() { b() }"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	dstRoot, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcChild, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "expression_statement", Line: 2})

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("structurally isomorphic child should be recovered")
	}
}

func TestSimpleRecoveryUniqueType(t *testing.T) {
	// SimpleRecovery pairs nodes with unique types.
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tif cond { a() }\n\tvar x int\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tif cond { b() }\n\tvar y int\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	dstRoot, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcIf, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	dstIf, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if_statement", Line: 3})

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
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() { _ = x }"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() { _ = y }"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	b, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	m := NewMapping()
	m.Add(a, b)
	SimpleRecovery(a, b, m)

	if len(m.Pairs) != 1 {
		t.Errorf("leaf recovery should add no new pairs, got %d", len(m.Pairs))
	}
}

func TestUniqueTypePairs(t *testing.T) {
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = 10\n\t_ = \"hello\"\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = 20\n\t_ = \"world\"\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a1, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "int_literal", Line: 3})
	a2, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "interpreted_string_literal", Line: 4})
	b1, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "int_literal", Line: 3})
	b2, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "interpreted_string_literal", Line: 4})

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
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() { _ = x; _ = y }"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() { _ = z }"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a1, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})
	a2, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 4})
	b1, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2})

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
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\ta()\n\tb()\n\tc()\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\ta()\n\tb()\n\tc()\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	dstRoot, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	leftSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "expression_statement", Line: 3})
	midSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "expression_statement", Line: 4})
	rightSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "expression_statement", Line: 5})
	leftDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "expression_statement", Line: 3})
	midDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "expression_statement", Line: 4})
	rightDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "expression_statement", Line: 5})

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
	srcTree, err := treesitter.Parse([]byte("package main\nvar headerStyle Style"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nvar rulesObj Rules"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	v1, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "var_declaration", Line: 2})
	v2, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "var_declaration", Line: 2})

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
	srcTree, err := treesitter.Parse([]byte("package main\nvar count int"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nvar count int64"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	v1, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "var_declaration", Line: 2})
	v2, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "var_declaration", Line: 2})

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
	src := "package main\nfunc f() {\n\tif x == nil || x.Parent == nil || z == nil {}\n}"
	dst := "package main\nfunc f() {\n\tif v == nil {}\n}"
	srcTree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	compoundSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3, Nth: 1})
	c3, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3, Nth: 5})
	simpleDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})

	nilSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "nil", Line: 3, Nth: 3})
	nilDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "nil", Line: 3})

	m := NewMapping()
	m.Add(nilSrc, nilDst)

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
	src := "package main\nfunc f() {\n\tif v == nil {}\n}"
	dst := "package main\nfunc f() {\n\tif x == nil || x.Parent == nil || z == nil {}\n}"
	srcTree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	simpleSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	compoundDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 3, Nth: 1})
	c3, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 3, Nth: 5})

	nilSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "nil", Line: 3})
	nilDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "nil", Line: 3, Nth: 3})

	m := NewMapping()
	m.Add(nilSrc, nilDst)

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
	srcTree, err := treesitter.Parse([]byte("const x = <span>Recent Admissions</span>;"), "test.tsx")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("const y = <i></i>;"), "test.tsx")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	spanElem, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "jsx_element", Line: 1})
	iElem, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "jsx_element", Line: 1})

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{spanElem}, []*treesitter.ASTNode{iElem}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate JSX tags (span vs i), got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairs_BlindSingletonTrapStatement(t *testing.T) {
	src1 := "package main\nfunc f() {\n\tfor item := range list {\n\t\toldBody()\n\t}\n}"
	dst1 := "package main\nfunc f() {\n\tfor i := 0; i < n; i++ {\n\t\tnewBody()\n\t}\n}"
	tree1, _ := treesitter.Parse([]byte(src1), "test.go")
	tree2, _ := treesitter.Parse([]byte(dst1), "test.go")
	forRange, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "for_statement", Line: 3})
	forThreeClause, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "for_statement", Line: 3})

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{forRange}, []*treesitter.ASTNode{forThreeClause}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate statements with 0 affinity, got %d pairs", len(pairs))
	}

	src2 := "package main\nfunc f() {\n\tfor i < n {\n\t\tbodyA()\n\t}\n}"
	dst2 := "package main\nfunc f() {\n\tfor i < n {\n\t\tbodyB()\n\t}\n}"
	tree3, _ := treesitter.Parse([]byte(src2), "test.go")
	tree4, _ := treesitter.Parse([]byte(dst2), "test.go")
	for1, _ := testutil.FindNode(tree3, testutil.NodeQuery{Type: "for_statement", Line: 3})
	for2, _ := testutil.FindNode(tree4, testutil.NodeQuery{Type: "for_statement", Line: 3})

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
	src := `package main
import "sort"
func f() {
	sort.SliceStable(scored, func() { return dice })
}
`
	dst := `package main
import "slices"
func f() {
	slices.SortStableFunc(scored, func() { return cmp.Or(dice) })
}
`
	tree1, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	tree2, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	outerCall1, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "call_expression", Line: 4})
	outerCall2, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "call_expression", Line: 4, Nth: 1})
	innerCmpCall, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "call_expression", Line: 4, Nth: 2})

	r := rules.Get("go")
	m := NewMapping()
	cand := findUnwrappedDstExpressionCandidate(outerCall1, outerCall2, m, r)
	if cand == innerCmpCall {
		t.Errorf("findUnwrappedDstExpressionCandidate should not cross closure/block/statement boundary to return innerCmpCall")
	}
}

func TestUniqueTypePairs_ClosureCallUsesContainerThreshold(t *testing.T) {
	src1 := `package main
func f() {
	sortSlice(scored, func() {
		check(a1, a2, a3, a4)
	})
}
`
	src2 := `package main
func f() {
	sortStableFunc(scored, func() {
		verify(a1, b2, b3, b4, b5, b6)
	})
}
`
	tree1, err := treesitter.Parse([]byte(src1), "test.go")
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	tree2, err := treesitter.Parse([]byte(src2), "test.go")
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}

	call1, _ := testutil.FindNode(tree1, testutil.NodeQuery{Type: "call_expression", Line: 3})
	call2, _ := testutil.FindNode(tree2, testutil.NodeQuery{Type: "call_expression", Line: 3})

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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn set_widths()", "fn set_widths()")

		// 2. Surrounding Sequence: opt.available_terminal_width assignment statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"opt.available_terminal_width = Term::stdout().size().1;",
			"opt.available_terminal_width = term_stdout.size().1;")

		// 3. Target: unrelated let declarations don't pair across statements.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"let empty_builtin_features = HashMap::new();")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"let term_stdout = Term::stdout();")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func setWidths()", "func setWidths()")

		// 2. Surrounding Sequence: availableTerminalWidth assignment statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"opt.availableTerminalWidth = TermStdout().size().one",
			"opt.availableTerminalWidth = termStdout.size().one")

		// 3. Target: unrelated short var declarations don't pair across statements.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"emptyBuiltinFeatures := HashMapNew()")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"termStdout := TermStdout()")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn compute()", "fn compute()")

		// 2. Surrounding Sequence: process(value) statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"process(old_value);", "process(new_value);")

		// 3. Target: in-place renamed let_declaration maps across versions.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let old_value = calculate_total(a, b);", "let new_value = calculate_total(a, b);")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function setWidths()", "function setWidths()")

		// 2. Surrounding Sequence: availableTerminalWidth expression statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"opt.availableTerminalWidth = Term.stdout().size().one;",
			"opt.availableTerminalWidth = termStdout.size().one;")

		// 3. Target: unrelated lexical declarations don't pair across statements.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"const emptyBuiltinFeatures = HashMap.create();")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"const termStdout = Term.stdout();")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function compute()", "function compute()")

		// 2. Surrounding Sequence: process call statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"process(oldValue);", "process(newValue);")

		// 3. Target: in-place renamed lexical_declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"const oldValue = calculateTotal(a, b);", "const newValue = calculateTotal(a, b);")
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

	part := NewLinePartition(srcBytes, dstBytes)
	mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	// 1. Enclosing Context: evaluate function maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"func evaluate(val int) int", "func evaluate(val int) int")

	// 2. Surrounding Sequence: startMetrics and endMetrics map in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"startMetrics()", "startMetrics()")
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"endMetrics()", "endMetrics()")

	// 3. Target: if_statement maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"if val > 0 {\n\t\treturn val * 2\n\t}", "if err := validate(val); val > 0 {\n\t\treturn val * 2\n\t}", "if_statement")

	// 4. Condition: val > 0 maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"val > 0", "val > 0")

	// 5. Stationary inner return statement maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"return val * 2", "return val * 2")

	// 6. The newly added init statement in dstIf must remain unmapped.
	testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
		"err := validate(val)")
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

	part := NewLinePartition(srcBytes, dstBytes)
	mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	// 1. Enclosing function handleBranch maps in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"func handleBranch(flag bool, val int)", "func handleBranch(flag bool, val int)")

	// 2. Surrounding Sequence: header, footer, and if_statement map in place.
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"header()", "header()")
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"footer()", "footer()")
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"if flag", "if flag", "if_statement")

	// 3. Moved statements across branches map to each other (cross-branch move).
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"compute(val)", "compute(val)")
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"fallback()", "fallback()")
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: enclosing function maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn test_logging(val: &str)", "fn test_logging(val: &str)")

		// 2. Surrounding Sequence: anchor statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"setup_env();", "setup_env();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"cleanup_env();", "cleanup_env();")

		// 3. Target Sandwiched Statement:
		// print! statement must map to write! statement across the shifted index interval.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"print!(\"value: {}\", val);", "write!(io::stdout(), \"value: {}\", val);")

		// 4. Stationary Operands: format string literal "value: {}" maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"value: {}"`, `"value: {}"`)
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: enclosing function, its parameters, and body block map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn write_assets(target_dir: &Path, current_version: &str) -> Result<()>",
			"fn write_assets(target_dir: &Path, current_version: &str) -> Result<()>")

		// 2. Surrounding Sequence: anchor statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let _ = std::fs::create_dir_all(target_dir);", "let _ = std::fs::create_dir_all(target_dir);")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;",
			"crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"Ok(())", "Ok(())")

		// 3. Target Sandwiched Statements:
		// Statement 1: print! -> write!
		// Statement 2: println! -> writeln!
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"print!(\"Writing metadata to folder {} ... \", target_dir.to_string_lossy());",
			"write!(io::stdout(), \"Writing metadata to folder {} ... \", target_dir.to_string_lossy())?;", "expression_statement")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"println!(\"okay\");",
			"writeln!(io::stdout(), \"okay\")?;", "expression_statement")

		// 4. Transparent Wrapper Unwrapping:
		// The inner macro_invocations must map across the try_expression wrapper.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"print!(\"Writing metadata to folder {} ... \", target_dir.to_string_lossy())",
			"write!(io::stdout(), \"Writing metadata to folder {} ... \", target_dir.to_string_lossy())", "macro_invocation")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"println!(\"okay\")",
			"writeln!(io::stdout(), \"okay\")", "macro_invocation")

		// Outer try_expressions must remain unmapped so they become inserted wrappers.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"write!(io::stdout(), \"Writing metadata to folder {} ... \", target_dir.to_string_lossy())?", "try_expression")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"writeln!(io::stdout(), \"okay\")?", "try_expression")

		// 5. Stationary Operands:
		// String literal "okay" must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"okay"`, `"okay"`)
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: fn generate_config_file maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn generate_config_file() -> Result<()>", "fn generate_config_file() -> Result<()>")

		// 2. Surrounding Sequence: if_statement and inner statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if config_file.is_file()", "if config_file.is_file()", "if_expression")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"io::stdout().flush()?;", "io::stdout().flush()?;")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let mut decision = String::new();", "let mut decision = String::new();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"io::stdin().read_line(&mut decision)?;", "io::stdin().read_line(&mut decision)?;")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"Ok(())", "Ok(())")

		// 3. Multi-statement Leading Interval Pairing:
		// println! and print! statements in the leading interval must map 1:1 to writeln! and write!.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"println!(\"A config file already exists at: {}\", config_file.to_string_lossy());",
			"writeln!(io::stdout(), \"A config file already exists at: {}\", config_file.to_string_lossy())?;", "expression_statement")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"print!(\"Overwrite? (y/N): \");",
			"write!(io::stdout(), \"Overwrite? (y/N): \")?;", "expression_statement")

		// 4. Stationary Operands: format string literals map in place without disruption.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"A config file already exists at: {}"`, `"A config file already exists at: {}"`)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"Overwrite? (y/N): "`, `"Overwrite? (y/N): "`)
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: run function declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"pub fn run(writer: anytype) !void", "pub fn run(writer: anytype) !void")

		// 2. Surrounding Sequence: init, anchor, and finish statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"init();", "init();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"anchor();", "anchor();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"finish();", "finish();")

		// 3. Target Sandwiched Statements:
		// writer.print statement maps to try writer.write statement.
		// writer.println statement maps to try writer.writeln statement.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`writer.print("foo\n", .{});`, `try writer.write("foo\n", .{});`)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`writer.println("bar\n", .{});`, `try writer.writeln("bar\n", .{});`)

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expressions map across try_expression wrappers.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`writer.print("foo\n", .{})`, `writer.write("foo\n", .{})`)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`writer.println("bar\n", .{})`, `writer.writeln("bar\n", .{})`)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			`try writer.write("foo\n", .{})`, "try_expression")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			`try writer.writeln("bar\n", .{})`, "try_expression")

		// 5. Stationary Operands:
		// Format string literals "foo\n" and "bar\n" map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"foo\n"`, `"foo\n"`)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"bar\n"`, `"bar\n"`)
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: handle_transaction function maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"void handle_transaction(Database* db)", "void handle_transaction(Database* db)")

		// 2. Surrounding Sequence: surrounding statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"db_begin(db);", "db_begin(db);")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"db_commit(db);", "db_commit(db);")

		// 3. Target Sandwiched Statement:
		// db_flush_wal statement maps in place across parenthesized expression.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"db_flush_wal(db);", "(db_flush_wal(db));")

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expression maps across the parenthesized_expression wrapper.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"db_flush_wal(db)", "db_flush_wal(db)")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"(db_flush_wal(db))", "parenthesized_expression")

		// 5. Stationary Operands:
		// db argument identifier maps in place.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "db", Occurrence: 3},
			testutil.TextQuery{Text: "db", Occurrence: 3})
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: syncSession function declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"async function syncSession(client)", "async function syncSession(client)")

		// 2. Surrounding Sequence: connect and disconnect statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.connect();", "client.connect();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.disconnect();", "client.disconnect();")

		// 3. Target Sandwiched Statement:
		// fetchUpdates statement maps across await_expression.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.fetchUpdates();", "await client.fetchUpdates();")

		// 4. Transparent Wrapper Unwrapping:
		// Inner call_expression maps across the await_expression wrapper.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.fetchUpdates()", "client.fetchUpdates()")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"await client.fetchUpdates()", "await_expression")
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

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: sync_session function definition maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"async def sync_session(client):", "async def sync_session(client):")

		// 2. Surrounding Sequence: connect and disconnect statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.connect()", "client.connect()")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.disconnect()", "client.disconnect()")

		// 3. Target Sandwiched Statement:
		// fetch_updates statement maps across await wrapper.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.fetch_updates()", "await client.fetch_updates()", "expression_statement")

		// 4. Transparent Wrapper Unwrapping:
		// Inner call maps across the await wrapper.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"client.fetch_updates()", "client.fetch_updates()", "call")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"await client.fetch_updates()", "await")
	})
}
