package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestSimpleRecoveryLabelMatch(t *testing.T) {
	// SimpleRecovery maps isomorphic children of mapped parents.
	src := []byte("package main\nfunc f() { a() }")
	dst := []byte("package main\nfunc f() { a() }")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNodeByText(srcTree, src, "{ a() }")
	dstRoot, _ := testutil.FindNodeByText(dstTree, dst, "{ a() }")

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	testutil.AssertMappedText(t, srcTree, dstTree, src, dst, m.Src(), "a()", "a()")
}

func TestSimpleRecoveryStructureMatch(t *testing.T) {
	// SimpleRecovery maps children with matching structures but different labels.
	src := []byte("package main\nfunc f() { a() }")
	dst := []byte("package main\nfunc f() { b() }")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNodeByText(srcTree, src, "{ a() }")
	dstRoot, _ := testutil.FindNodeByText(dstTree, dst, "{ b() }")

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	testutil.AssertMappedText(t, srcTree, dstTree, src, dst, m.Src(), "a()", "b()")
}

func TestSimpleRecoveryUniqueType(t *testing.T) {
	// SimpleRecovery pairs nodes with unique types.
	src := []byte("package main\nfunc f() {\n\tif cond { a() }\n\tvar x int\n}")
	dst := []byte("package main\nfunc f() {\n\tif cond { b() }\n\tvar y int\n}")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNodeByText(srcTree, src, "{\n\tif cond { a() }\n\tvar x int\n}")
	dstRoot, _ := testutil.FindNodeByText(dstTree, dst, "{\n\tif cond { b() }\n\tvar y int\n}")

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	testutil.AssertMappedText(t, srcTree, dstTree, src, dst, m.Src(), "if cond { a() }", "if cond { b() }")
}

func TestSimpleRecoveryNoChildren(t *testing.T) {
	src := []byte("package main\nfunc f() { _ = x }")
	dst := []byte("package main\nfunc f() { _ = y }")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a, _ := testutil.FindNodeByText(srcTree, src, "x")
	b, _ := testutil.FindNodeByText(dstTree, dst, "y")
	m := NewMapping()
	m.Add(a, b)
	SimpleRecovery(a, b, m)

	if len(m.Pairs) != 1 {
		t.Errorf("leaf recovery should add no new pairs, got %d", len(m.Pairs))
	}
}

func TestUniqueTypePairs(t *testing.T) {
	src := []byte("package main\nfunc f() {\n\t_ = 10\n\t_ = \"hello\"\n}")
	dst := []byte("package main\nfunc f() {\n\t_ = 20\n\t_ = \"world\"\n}")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a1, _ := testutil.FindNodeByText(srcTree, src, "10")
	a2, _ := testutil.FindNodeByText(srcTree, src, "\"hello\"")
	b1, _ := testutil.FindNodeByText(dstTree, dst, "20")
	b2, _ := testutil.FindNodeByText(dstTree, dst, "\"world\"")

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
	src := []byte("package main\nfunc f() { _ = x; _ = y }")
	dst := []byte("package main\nfunc f() { _ = z }")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	a1, _ := testutil.FindNodeByText(srcTree, src, "x")
	a2, _ := testutil.FindNodeByText(srcTree, src, "y")
	b1, _ := testutil.FindNodeByText(dstTree, dst, "z")

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
	src := []byte("package main\nfunc f() {\n\ta()\n\tb()\n\tc()\n}")
	dst := []byte("package main\nfunc f() {\n\ta()\n\tb()\n\tc()\n}")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcRoot, _ := testutil.FindNodeByText(srcTree, src, "{\n\ta()\n\tb()\n\tc()\n}")
	dstRoot, _ := testutil.FindNodeByText(dstTree, dst, "{\n\ta()\n\tb()\n\tc()\n}")
	leftSrc, _ := testutil.FindNodeByText(srcTree, src, "a()")
	rightSrc, _ := testutil.FindNodeByText(srcTree, src, "c()")
	leftDst, _ := testutil.FindNodeByText(dstTree, dst, "a()")
	rightDst, _ := testutil.FindNodeByText(dstTree, dst, "c()")

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(leftSrc, leftDst)
	m.Add(rightSrc, rightDst)

	SimpleRecovery(srcRoot, dstRoot, m)

	testutil.AssertMappedText(t, srcTree, dstTree, src, dst, m.Src(), "b()", "b()")
}

func TestUniqueTypePairsDeclarationNoSemanticOverlap(t *testing.T) {
	src := []byte("package main\nvar headerStyle Style")
	dst := []byte("package main\nvar rulesObj Rules")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	v1, _ := testutil.FindNodeByText(srcTree, src, "var headerStyle Style")
	v2, _ := testutil.FindNodeByText(dstTree, dst, "var rulesObj Rules")

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
	src := []byte("package main\nvar count int")
	dst := []byte("package main\nvar count int64")
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	v1, _ := testutil.FindNodeByText(srcTree, src, "var count int")
	v2, _ := testutil.FindNodeByText(dstTree, dst, "var count int64")

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
	srcBytes := []byte(src)
	dstBytes := []byte(dst)
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	compoundSrc, _ := testutil.FindNodeByText(srcTree, srcBytes, "x == nil || x.Parent == nil || z == nil")
	c3, _ := testutil.FindNodeByText(srcTree, srcBytes, "z == nil")
	simpleDst, _ := testutil.FindNodeByText(dstTree, dstBytes, "v == nil")

	nilSrc, err := testutil.FindNodeByTextQuery(srcTree, srcBytes, testutil.TextQuery{Text: "nil", Occurrence: 3})
	if err != nil {
		t.Fatalf("find nilSrc: %v", err)
	}
	nilDst, err := testutil.FindNodeByText(simpleDst, dstBytes, "nil")
	if err != nil {
		t.Fatalf("find nilDst: %v", err)
	}

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
	srcBytes := []byte(src)
	dstBytes := []byte(dst)
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	simpleSrc, _ := testutil.FindNodeByText(srcTree, srcBytes, "v == nil")
	compoundDst, _ := testutil.FindNodeByText(dstTree, dstBytes, "x == nil || x.Parent == nil || z == nil")
	c3, _ := testutil.FindNodeByText(dstTree, dstBytes, "z == nil")

	nilSrc, err := testutil.FindNodeByText(simpleSrc, srcBytes, "nil")
	if err != nil {
		t.Fatalf("find nilSrc: %v", err)
	}
	nilDst, err := testutil.FindNodeByTextQuery(dstTree, dstBytes, testutil.TextQuery{Text: "nil", Occurrence: 3})
	if err != nil {
		t.Fatalf("find nilDst: %v", err)
	}

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
	src := []byte("const x = <span>Recent Admissions</span>;")
	dst := []byte("const y = <i></i>;")
	srcTree, err := treesitter.Parse(src, "test.tsx")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.tsx")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	spanElem, _ := testutil.FindNodeByText(srcTree, src, "<span>Recent Admissions</span>")
	iElem, _ := testutil.FindNodeByText(dstTree, dst, "<i></i>")

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{spanElem}, []*treesitter.ASTNode{iElem}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate JSX tags (span vs i), got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairs_BlindSingletonTrapStatement(t *testing.T) {
	src1 := []byte("package main\nfunc f() {\n\tfor item := range list {\n\t\toldBody()\n\t}\n}")
	dst1 := []byte("package main\nfunc f() {\n\tfor i := 0; i < n; i++ {\n\t\tnewBody()\n\t}\n}")
	tree1, _ := treesitter.Parse(src1, "test.go")
	tree2, _ := treesitter.Parse(dst1, "test.go")
	forRange, _ := testutil.FindNodeByText(tree1, src1, "for item := range list {\n\t\toldBody()\n\t}")
	forThreeClause, _ := testutil.FindNodeByText(tree2, dst1, "for i := 0; i < n; i++ {\n\t\tnewBody()\n\t}")

	m := NewMapping()
	pairs := uniqueTypePairs([]*treesitter.ASTNode{forRange}, []*treesitter.ASTNode{forThreeClause}, m)
	if len(pairs) != 0 {
		t.Errorf("uniqueTypePairs should NOT pair disparate statements with 0 affinity, got %d pairs", len(pairs))
	}

	src2 := []byte("package main\nfunc f() {\n\tfor i < n {\n\t\tbodyA()\n\t}\n}")
	dst2 := []byte("package main\nfunc f() {\n\tfor i < n {\n\t\tbodyB()\n\t}\n}")
	tree3, _ := treesitter.Parse(src2, "test.go")
	tree4, _ := treesitter.Parse(dst2, "test.go")
	for1, _ := testutil.FindNodeByText(tree3, src2, "for i < n {\n\t\tbodyA()\n\t}")
	for2, _ := testutil.FindNodeByText(tree4, dst2, "for i < n {\n\t\tbodyB()\n\t}")

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
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			testutil.TextQuery{Text: "shared := calc()", Occurrence: 1},
			testutil.TextQuery{Text: "shared := calc()", Occurrence: 1},
		)
		testutil.AssertUnmappedTextQuery(t, srcAST, []byte(src), mr.Mappings.Src(),
			testutil.TextQuery{Text: "shared := calc()", Occurrence: 2},
		)

		// Whole-snippet verification: func Run, both if blocks, and finishB map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func Run(mode int)", "func Run(mode int)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"if mode == 1", "if mode == 1",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"if mode == 2", "if mode == 2",
		)

		// Verify finishB statement in mode == 2 maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"finishB()", "finishB()",
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
	srcBytes := []byte(src)
	dstBytes := []byte(dst)
	tree1, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	tree2, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	outerCall1, _ := testutil.FindNodeByText(tree1, srcBytes, "sort.SliceStable(scored, func() { return dice })")
	outerCall2, _ := testutil.FindNodeByText(tree2, dstBytes, "slices.SortStableFunc(scored, func() { return cmp.Or(dice) })")
	innerCmpCall, _ := testutil.FindNodeByText(tree2, dstBytes, "cmp.Or(dice)")

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
	src1Bytes := []byte(src1)
	src2Bytes := []byte(src2)
	tree1, err := treesitter.Parse(src1Bytes, "test.go")
	if err != nil {
		t.Fatalf("parse 1: %v", err)
	}
	tree2, err := treesitter.Parse(src2Bytes, "test.go")
	if err != nil {
		t.Fatalf("parse 2: %v", err)
	}

	call1, _ := testutil.FindNodeByText(tree1, src1Bytes, "sortSlice(scored, func() {\n\t\tcheck(a1, a2, a3, a4)\n\t})")
	call2, _ := testutil.FindNodeByText(tree2, src2Bytes, "sortStableFunc(scored, func() {\n\t\tverify(a1, b2, b3, b4, b5, b6)\n\t})")

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
	testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"sortSliceStable(scored, func(i, j int) bool {",
		"cmpOr(",
	)
	testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
		"cmpOr(",
	)

	// Whole-snippet verification: func sortItems, expression statement, and closure function literal map in place
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"func sortItems(scored any)", "func sortItems(scored any)",
	)
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"sortSliceStable(scored, func(i, j int) bool {", "sortStableFunc(scored, func(a, b any) int {",
	)
	testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
		"func(i, j int) bool", "func(a, b any) int",
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
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "len(item.Payload) > 0", Occurrence: 1},
			testutil.TextQuery{Text: "len(item.Payload) > 0", Occurrence: 1},
		)

		// Defect assertion: on main without sibling absorption rejection, uniqueTypePairs
		// erroneously paired the deleted second operand (srcSecondOp) to dstInnerWrapper.
		testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"len(item.Payload) > 0 && len(item.Payload) > 0",
			"len(item.Payload) > 0 && hasPayload",
		)
		testutil.AssertUnmappedTextQuery(t, srcAST, srcBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "len(item.Payload) > 0", Occurrence: 2},
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"len(item.Payload) > 0 && hasPayload",
		)

		// Whole-snippet verification: enclosing type and function declarations, surrounding statements, and returns map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"type Item struct", "type Item struct",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func check(item Item, hasPayload, hasMapped bool) bool", "func check(item Item, hasPayload, hasMapped bool) bool",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if len(item.Payload) > 0", "if len(item.Payload) > 0",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return true", "return true",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return false", "return false",
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func formatBadge", "func formatBadge",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"(lastChunkStr, badgeText string)", "(lastChunkStr, badgeText string)",
		)

		// 2. Surrounding Sequence: Preceding short_var_declaration, if_statement, and returns
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`prefix := "badge:"`, `prefix := "badge:"`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if !hasSuffix", "if !hasSuffix",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return false", "return false",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return true", "return true",
		)

		// 3. Target, Containers & Defect Invariant:
		// bOldInner (MoveFg+badgeText) maps to bDst2 (Nth 2 in dst)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"color.MoveFg+badgeText", "color.MoveFg+badgeText",
		)
		// bOldOuter (... + Reset) maps to bDst3 (Nth 1 in dst)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"color.MoveFg+badgeText+color.Reset", "color.Dim+color.Italic+color.MoveFg+badgeText+color.Reset",
		)

		// Defect assertions: bOldInner must NOT map to bDst1 (cross-side operator mismatch)
		testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"color.MoveFg+badgeText",
			"color.Dim+color.Italic",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"color.Dim+color.Italic",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"color.Dim+color.Italic+color.MoveFg",
		)

		// Call and argument_list map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"hasSuffix(lastChunkStr, color.MoveFg+badgeText+color.Reset)",
			"hasSuffix(lastChunkStr, color.Dim+color.Italic+color.MoveFg+badgeText+color.Reset)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"(lastChunkStr, color.MoveFg+badgeText+color.Reset)",
			"(lastChunkStr, color.Dim+color.Italic+color.MoveFg+badgeText+color.Reset)",
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func TestRevokeOrphanedGlue(t *testing.T)",
			"func TestRevokeOrphanedGlue(t *testing.T)",
		)

		// 2. Surrounding Sequence: stmt1 := ... and stmt1.Children = ... and bin1.Parent = stmt1
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`stmt1 := testutil.Node("if_statement", "")`,
			`stmt1 := testutil.Node("if_statement", "")`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"stmt1.Children = []*treesitter.ASTNode{bin1}",
			"stmt1.Children = []*treesitter.ASTNode{bin1}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"bin1.Parent = stmt1",
			"bin1.Parent = stmt1",
		)

		// 3. Target & Containers: bin1 outer call and argument_list
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`testutil.Node("binary_expression", "",`,
			`testutil.Node("binary_expression", "",`,
		)

		// 4. Sibling Calls: testutil.Leaf(...) <-> testutil.NodeAt(...)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`testutil.Leaf("identifier", "sText")`,
			`testutil.NodeAt("identifier", "sText", 0, 5)`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`testutil.Leaf("logical_operator_literal", "||")`,
			`testutil.NodeAt("logical_operator_literal", "||", 6, 8)`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`testutil.Leaf("identifier", "oldCond")`,
			`testutil.NodeAt("identifier", "oldCond", 9, 16)`,
		)

		// 5. Stationary Operands: strings inside sibling calls
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"sText"`, `"sText"`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"||"`, `"||"`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`"oldCond"`, `"oldCond"`,
		)

		// 6. Granular evolution: new numeric offsets inserted and unmapped
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "0", "int_literal")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "5", "int_literal")
		testutil.AssertUnmappedTextQuery(t, dstAST, dstBytes, mr.Mappings.Dst(), testutil.TextQuery{Text: "6", Occurrence: 1, Type: "int_literal"})
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "8", "int_literal")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "9", "int_literal")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "16", "int_literal")
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags function
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function checkFlags(context: Context, node: Node)",
			"function checkFlags(context: Context, node: Node)",
		)

		// 2. Surrounding Sequence: const valid and return valid
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"const valid = isValid(node);",
			"const valid = isValid(node);",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return valid;",
			"return valid;",
		)

		// 3. Target & Containers: if statement, unary ! and parenthesized_expression
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if (!", "if (!",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"!(context.flags & Flags.AllowUnresolved",
			"!(context.flags & Flags.AllowUnresolved",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"(!(context.flags & Flags.AllowUnresolved",
			"(!(context.flags & Flags.AllowUnresolved",
		)

		// 4. Stationary Operands: isEntityName(node) maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"isEntityName(node)",
			"isEntityName(node)",
		)

		// Deleted middle term hasDynamicName must be unmapped in src
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"hasDynamicName(node)",
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
		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		srcAST, err := treesitter.Parse(srcBytes, "test.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: checkFlags method
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"boolean checkFlags(Context context, Node node)",
			"boolean checkFlags(Context context, Node node)",
		)

		// 2. Surrounding Sequence: boolean valid and return valid
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"boolean valid = isValid(node);",
			"boolean valid = isValid(node);",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return valid;",
			"return valid;",
		)

		// 3. Target: if statement and outer parenthesized condition
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if (!((context.flags", "if (!((context.flags",
		)

		// 4. Stationary Operands: call isEntityName
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"isEntityName(node)",
			"isEntityName(node)",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"hasDynamicName(node)",
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		srcAST, err := treesitter.Parse(srcBytes, "config.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "config.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: struct Config maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"struct Config", "struct Config",
		)

		// 2. Surrounding Sequence: hyperlinks and line_numbers_zero_style map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"hyperlinks", "hyperlinks", "field_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"line_numbers_zero_style", "line_numbers_zero_style", "field_declaration",
		)

		// 3. Target: Deleted line_numbers_plus_style must NOT match inserted inline_hint_style.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"line_numbers_plus_style", "field_declaration",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"inline_hint_style", "field_declaration",
		)

		// 4. Same name with different type: pub foo: Style -> pub foo: String should map.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"foo", "foo", "field_declaration",
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		srcAST, err := treesitter.Parse(srcBytes, "color.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "color.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Surrounding sequence: HashMap and Color must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"use std::collections::HashMap;", "use std::collections::HashMap;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"use ansi_term::Color;", "use ansi_term::Color;",
		)

		// 2. Target: Deleted use std::process must NOT match inserted use crate::fatal.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"use std::process;",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"use crate::fatal;",
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		srcAST, err := treesitter.Parse(srcBytes, "config.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "config.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: type Config struct maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"type Config struct", "type Config struct",
		)

		// 2. Surrounding sequence: Hyperlinks and Numbers map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"Hyperlinks bool", "Hyperlinks bool",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"Numbers    bool", "Numbers    bool",
		)

		// 3. Target: Deleted OldField must NOT match inserted NewField.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"OldField   Style",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"NewField   Style",
		)

		// 4. Same name with different type: Foo Style -> Foo string maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"Foo        Style", "Foo        string",
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, nil)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"use crate::syntect_color;", "use crate::syntect_utils;",
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

		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "config.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "config.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: fn make_config maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn make_config(opt: Opt) -> Config",
			"fn make_config(opt: Opt) -> Config",
		)

		// 2. Surrounding sequence: commit_style, color_only, line_numbers_zero_style, line_buffer_size map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"commit_style", "commit_style",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"color_only: opt.color_only", "color_only: opt.color_only",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"line_numbers_zero_style", "line_numbers_zero_style",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"line_buffer_size: opt.line_buffer_size", "line_buffer_size: opt.line_buffer_size",
		)

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"default_language: opt.default_language",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"hunk_label",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"inline_hint_style",
		)

		// 4. Evolving line numbers pairs:
		// line_numbers_left_format -> line_numbers_format
		// line_numbers_left_style -> line_numbers_style_leftright
		// line_numbers_minus_style -> line_numbers_style_minusplus
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"line_numbers_left_format: opt.line_numbers_left_format",
			"line_numbers_format: LeftRight::new(",
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "line_numbers_left_style", Type: "shorthand_field_initializer"},
			testutil.TextQuery{Text: "line_numbers_style_leftright: LeftRight::new("},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "line_numbers_minus_style", Type: "shorthand_field_initializer"},
			testutil.TextQuery{Text: "line_numbers_style_minusplus: MinusPlus::new("},
		)

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"line_numbers_plus_style",
			"shorthand_field_initializer",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"line_numbers_right_format: opt.line_numbers_right_format",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"line_numbers_right_style",
			"shorthand_field_initializer",
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "config.js")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "config.js")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function buildConfig maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function buildConfig(opt)", "function buildConfig(opt)",
		)

		// 2. Destructuring pattern: lineNumbers and sideBySide map in place.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "lineNumbers", Occurrence: 1},
			testutil.TextQuery{Text: "lineNumbers", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "sideBySide", Occurrence: 1},
			testutil.TextQuery{Text: "sideBySide", Occurrence: 1},
		)

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"commitStyle", "commitStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"colorOnly: opt.colorOnly", "colorOnly: opt.colorOnly",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersZeroStyle", "lineNumbersZeroStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineBufferSize: opt.lineBufferSize", "lineBufferSize: opt.lineBufferSize",
		)

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"defaultLanguage: opt.defaultLanguage",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"hunkLabel", "shorthand_property_identifier",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"inlineHintStyle", "shorthand_property_identifier",
		)

		// 5. Evolving line numbers pairs:
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftFormat: opt.lineNumbersLeftFormat",
			"lineNumbersFormat: formatLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftStyle",
			"lineNumbersStyleLeftRight: styleLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersMinusStyle",
			"lineNumbersStyleMinusPlus: styleMinusPlus(",
		)

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersPlusStyle", "shorthand_property_identifier",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersRightFormat: opt.lineNumbersRightFormat",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersRightStyle", "shorthand_property_identifier",
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "config.ts")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "config.ts")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function configure maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function configure(opt: Opts)", "function configure(opt: Opts)",
		)

		// 2. Destructuring pattern: flag and mode map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"flag", "flag",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"mode", "mode",
		)

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"commitStyle", "commitStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"colorOnly: opt.colorOnly", "colorOnly: opt.colorOnly",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersZeroStyle", "lineNumbersZeroStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineBufferSize: opt.lineBufferSize", "lineBufferSize: opt.lineBufferSize",
		)

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"defaultLanguage: opt.defaultLanguage",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"hunkLabel", "shorthand_property_identifier",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"inlineHintStyle", "shorthand_property_identifier",
		)

		// 5. Evolving line numbers pairs:
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftFormat: opt.lineNumbersLeftFormat",
			"lineNumbersFormat: formatLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftStyle",
			"lineNumbersStyleLeftRight: styleLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersMinusStyle",
			"lineNumbersStyleMinusPlus: styleMinusPlus(",
		)

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersPlusStyle", "shorthand_property_identifier",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersRightFormat: opt.lineNumbersRightFormat",
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersRightStyle", "shorthand_property_identifier",
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "component.tsx")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "component.tsx")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function ConfigPanel maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"export function ConfigPanel", "export function ConfigPanel",
		)

		// 2. Destructuring pattern: visible and theme map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"visible", "visible",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"theme", "theme",
		)

		// 3. Surrounding sequence: commitStyle, colorOnly, lineNumbersZeroStyle, lineBufferSize map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"commitStyle", "commitStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"colorOnly: props.colorOnly", "colorOnly: props.colorOnly",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersZeroStyle", "lineNumbersZeroStyle", "shorthand_property_identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineBufferSize: props.lineBufferSize", "lineBufferSize: props.lineBufferSize",
		)

		// 4. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"defaultLanguage: props.defaultLanguage",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"hunkLabel", "shorthand_property_identifier",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"inlineHintStyle", "shorthand_property_identifier",
		)

		// 5. Evolving line numbers pairs:
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftFormat: props.lineNumbersLeftFormat",
			"lineNumbersFormat: formatLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftStyle",
			"lineNumbersStyleLeftRight: styleLeftRight(",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersMinusStyle",
			"lineNumbersStyleMinusPlus: styleMinusPlus(",
		)

		// 6. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersPlusStyle", "shorthand_property_identifier",
		)
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		srcAST, err := treesitter.Parse(srcBytes, "ConfigService.java")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "ConfigService.java")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing class and method map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"class ConfigService", "class ConfigService",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"public void configure() {}", "public void configure() {}",
		)

		// 2. Surrounding sequence: prefix, lineNumbersZeroStyle, and lineBufferSize map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"prefix = \"app.config\"", "prefix = \"app.config\"",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersZeroStyle = \"zero\"", "lineNumbersZeroStyle = \"zero\"",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineBufferSize = \"1024\"", "lineBufferSize = \"1024\"",
		)

		// 3. Newly added disjoint fields in dst must NOT be stolen by old fields.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"defaultLanguage = \"rust\"",
		)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(),
			"hunkLabel = \"label\"",
		)

		// 4. Evolving line numbers pairs:
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"lineNumbersLeftFormat = \"left\"",
			"lineNumbersFormat = \"leftRight\"",
		)

		// 5. Consolidated trailing fields in src must be cleanly unmapped (deleted):
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			"lineNumbersLeftStyle = \"style\"",
		)
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Surrounding sequence maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "CommitMeta", "CommitMeta")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "FileMeta", "FileMeta")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), testutil.TextQuery{Text: "HunkZero", Occurrence: 1}, testutil.TextQuery{Text: "HunkZero", Occurrence: 1})
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "Unknown", "Unknown")

		// 2. Target variant evolves in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "HunkHeader", "HunkHeader(String, String)", "enum_variant")

		// 3. Added variant is unmapped in src (inserted)
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "HunkPlusWrapped")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.c")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.c")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "STATE_COMMIT_META", "STATE_COMMIT_META")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "STATE_FILE_META", "STATE_FILE_META")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), testutil.TextQuery{Text: "STATE_HUNK_ZERO", Occurrence: 1}, testutil.TextQuery{Text: "STATE_HUNK_ZERO", Occurrence: 1})
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "STATE_UNKNOWN", "STATE_UNKNOWN")

		// Disjoint variant names must not match across non-identical identifiers
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "STATE_OLD_DELETED_HEADER")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "STATE_NEW_INSERTED_HEADER = 42")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "CommitMeta", "CommitMeta")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "FileMeta", "FileMeta")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), testutil.TextQuery{Text: "HunkZero", Occurrence: 1}, testutil.TextQuery{Text: "HunkZero", Occurrence: 1})
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "Unknown", "Unknown")

		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "OldDeletedHeader")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "NewInsertedHeader = 100")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "Test.java")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "Test.java")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "CommitMeta", "CommitMeta")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "FileMeta", "FileMeta")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), testutil.TextQuery{Text: "HunkZero", Occurrence: 1}, testutil.TextQuery{Text: "HunkZero", Occurrence: 1})
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "Unknown", "Unknown")

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "HunkHeader", "HunkHeader(42)", "enum_constant")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "HunkPlusWrapped")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "CommitMeta = 1", "CommitMeta = 1")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "FileMeta = 2", "FileMeta = 2")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "HunkZero = 4", "HunkZero = 4")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "Unknown = 5", "Unknown = 5")

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "HunkHeader", "HunkHeader = 42")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "HunkPlusWrapped")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Unchanged parameter maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "config: &config::Config", "config: &config::Config")

		// 2. Differing parameters with different names and types must NOT cross-match
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "minus_number: Option<usize>")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "line_numbers: &MinusPlus<Option<usize>>")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Unchanged parameter maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "items []string", "items []string")

		// 2. Disjoint parameter names must not match across non-identical identifiers
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "oldLimit int")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "newLimit bool")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "items []string", "items ...string")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "totalCount int")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "batchSize float64")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.cpp")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "const char* name", "const char* name")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "int totalCount")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "long batchSize")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.java")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.java")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "String name", "String name")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "int totalCount")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "long batchSize")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "name: string", "name: string")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "totalCount: number")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "batchSize: string")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.py")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.py")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "name: str", "name: str")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "total_count: int")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "batch_size: str")
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
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.zig")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.zig")
		if err != nil {
			t.Fatal(err)
		}
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "name: []const u8", "name: []const u8")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "totalCount: i32")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "batchSize: f64")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "fn t()", "fn t()")

		// 2. Surrounding sequence: format payload container and unchanged field names
		// map in place without order disruption.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "prefix:", "prefix:")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "placeholder:", "placeholder:")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "Some", "Some")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "suffix:", "suffix:")

		// 3. Target & containers: neither empty `.into()` frame can steal ("nm");
		// the rewritten payload container pairs instead.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "(\"nm\")", "(Placeholder::NumberMinus)")
		testutil.AssertUnmappedTextQuery(t, dstAST, dstBytes, mr.Mappings.Dst(), testutil.TextQuery{Text: "()", Occurrence: 2, Type: "token_tree"})
		testutil.AssertUnmappedTextQuery(t, dstAST, dstBytes, mr.Mappings.Dst(), testutil.TextQuery{Text: "()", Occurrence: 3, Type: "token_tree"})
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"fn format_commit_line_captures_with_osc8_commit_hyperlink", "fn format_commit_line_captures_with_osc8_commit_hyperlink")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let commit = captures.get(2).unwrap().as_str();", "let commit = captures.get(2).unwrap().as_str();")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"format!", "format!")

		// 2. Surrounding Sequence: surrounding tokens and stationary arguments map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"url = format_github_commit_url(commit, github_repo),", "url = format_github_commit_url(commit, github_repo),")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"commit = commit,", "commit = commit,")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"suffix = captures.get(3).unwrap().as_str(),", "suffix = captures.get(3).unwrap().as_str(),")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"osc = \"\\x1b]\",", "osc = \"\\x1b]\",")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"st = \"\\x1b\\\\\"", "st = \"\\x1b\\\\\"")

		// 3. Target & Containers (the defect invariant):
		// Unrelated identifiers like as_str and map must never pair.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "map")

		// unwrap pairs with unwrap_or in place.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "unwrap", Occurrence: 2},
			testutil.TextQuery{Text: "unwrap_or", Occurrence: 1},
		)
		// as_str pairs with the closure's as_str.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "as_str", Occurrence: 2},
			testutil.TextQuery{Text: "as_str", Occurrence: 2},
		)
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "lazy_static!", "lazy_static!")

		// 2. Surrounding Sequence: static, ref, LINE_NUMBERS_PLACEHOLDER_REGEX, and Regex (type) map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "static ref", "static ref")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "LINE_NUMBERS_PLACEHOLDER_REGEX", "LINE_NUMBERS_PLACEHOLDER_REGEX")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "Regex", Occurrence: 1},
			testutil.TextQuery{Text: "Regex", Occurrence: 1},
		)

		// 3. Target & Containers (defect invariant):
		// The old token_tree of Regex::new ("abc") MUST NOT pair with the new token_tree of format::make_placeholder_regex.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "(\n        \"abc\"\n    )")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "(&[\"nm\", \"np\"])")

		// Old new and unwrap identifiers must remain unmapped (destined for deletion).
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "new")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "unwrap")

		// New format and make_placeholder_regex identifiers must remain unmapped (destined for insertion).
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "format")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "make_placeholder_regex")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "fn test_two_plus_lines()", "fn test_two_plus_lines()")

		// 2. Surrounding sequence: following statements should keep their mapping.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let output = run_delta(TWO_PLUS_LINES_DIFF, &config);",
			"let output = run_delta(TWO_PLUS_LINES_DIFF, &config);")

		// 3. Target construct: let config = make_config_from_args(...) should map across versions.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"let config = make_config_from_args(&[\"--side-by-side\", \"--width\", \"40\"]);",
			"let config = make_config_from_args(&[\n        \"--side-by-side\",\n        \"--width\",\n        \"41\",\n        \"--line-fill-method=spaces\",\n    ]);")

		// 4. Stationary operands: the config identifier should stay mapped.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "config", Occurrence: 1},
			testutil.TextQuery{Text: "config", Occurrence: 1})

		// 5. Inserted statement: new let sac shouldn't map back to anything in old code.
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "let sac = strip_ansi_codes;")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "fn parse_options()", "fn parse_options()")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "if condition", "if condition")

		// 2. Surrounding statements: let statements around the if block shouldn't drift.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "let x = 1;", "let x = 1;")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "let y = 2;", "let y = 2;")

		// 3. Target & containers: eprintln! should map to fatal(), token_tree should map to arguments, and process::exit should stay unmapped (deleted).
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"eprintln!(\"Invalid command-line option: --foo\");", "fatal(\"Invalid command-line option: --foo\");")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "process::exit(1);")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"(\"Invalid command-line option: --foo\")", "(\"Invalid command-line option: --foo\")")

		// 4. Stationary operands: the error message string literal should map in place across the macro and call.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"\"Invalid command-line option: --foo\"", "\"Invalid command-line option: --foo\"")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "pub fn parse_options() void", "pub fn parse_options() void")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "if (condition)", "if (condition)")

		// 2. Surrounding statements: const statements around the if block shouldn't drift.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "const x = 1;", "const x = 1;")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "const y = 2;", "const y = 2;")

		// 3. Target & containers: @panic should map to fatal(), while process.exit stays unmapped (deleted).
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"@panic(\"Invalid command-line option: --foo\");", "fatal(\"Invalid command-line option: --foo\");")
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "process.exit(1);")

		// 4. Stationary operands: the panic message string should map in place across @panic and fatal.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"\"Invalid command-line option: --foo\"", "\"Invalid command-line option: --foo\"")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "fn make_config", "fn make_config")
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			testutil.TextQuery{Text: "Config {", Occurrence: 2},
			testutil.TextQuery{Text: "Config {", Occurrence: 2})

		// 2. Surrounding Sequence: preceding tab_width and succeeding zero_style fields map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "tab_width: opt.tab_width,", "tab_width: opt.tab_width,")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "zero_style: opt.zero_style,", "zero_style: opt.zero_style,")

		// 3. Target & Containers:
		// Target pair truncation_symbol maps to truncation_symbol, along with key field_identifier.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "truncation_symbol: \"→\".to_string(),", "truncation_symbol: format!(\"{}→{}\", 1, 2),")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "truncation_symbol", "truncation_symbol")

		// Replaced value: call_expression ("→".to_string()) and arguments (()) must remain unmapped,
		// and macro_invocation (format!(...)) and token_tree must remain unmapped.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "\"→\".to_string()")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "format!(\"{}→{}\", 1, 2)")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "function makeConfig", "function makeConfig")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "return {", "return {")

		// 2. Surrounding Sequence: preceding tabWidth and succeeding zeroStyle pairs map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "tabWidth: opt.tabWidth,", "tabWidth: opt.tabWidth,")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "zeroStyle: opt.zeroStyle,", "zeroStyle: opt.zeroStyle,")

		// 3. Target & Containers: truncationSymbol pair maps across both trees.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "truncationSymbol: opt.getSymbol(),", "truncationSymbol: `${1}→${2}`,")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "truncationSymbol", "truncationSymbol")

		// Replaced value: call_expression (opt.getSymbol()) and arguments (()) remain unmapped,
		// and template_string remains unmapped.
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "opt.getSymbol()")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "`${1}→${2}`")
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
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "function App()", "function App()")

		// 2. Surrounding Sequence: preceding variant and succeeding disabled attributes map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "variant=\"primary\"", "variant=\"primary\"")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "disabled={false}", "disabled={false}")

		// 3. Target & Containers: onClick JSX attribute maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "onClick={handleClick()}", "onClick={() => handle()}")
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), "onClick", "onClick")

		// Replaced expression: handleClick() call should remain unmapped (deleted),
		// and () => handle() arrow function should remain unmapped (inserted).
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(), "handleClick()")
		testutil.AssertUnmappedText(t, dstAST, dstBytes, mr.Mappings.Dst(), "() => handle()")
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
