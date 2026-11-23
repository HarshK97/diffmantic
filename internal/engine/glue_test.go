package engine

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestRevokeOrphanedGlue(t *testing.T) {
	// Parent with orphaned glue: operator matched, sibling subtree unmatched.
	srcBytes := []byte("package main\nfunc f() {\n\tm := NewMapping()\n}")
	dstBytes := []byte("package main\nfunc g() {\n\ty := 1\n}")
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	glue, err := testutil.FindNodeByText(srcTree, srcBytes, ":=")
	if err != nil {
		t.Fatalf("find glue: %v", err)
	}
	dstGlue, err := testutil.FindNodeByText(dstTree, dstBytes, ":=")
	if err != nil {
		t.Fatalf("find dstGlue: %v", err)
	}
	m := NewMapping()
	m.Add(glue, dstGlue)
	RevokeOrphanedGlue(m)
	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), ":=")
	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), "NewMapping()")

	// Reparented glue revoked even when siblings mapped elsewhere
	// (scattered matches in repetitive code): glue must never move.
	m5 := NewMapping()
	srcScatBytes := []byte("package main\nfunc f() {\n\ta := b\n}")
	dstScatBytes := []byte("package main\nfunc g() {\n\tc := d\n}")
	elsewhereBytes := []byte("package main\nfunc h() {\n\t_ = elsewhere\n\t_ = elsewhere2\n}")
	srcScatTree, err := treesitter.Parse(srcScatBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcScat: %v", err)
	}
	dstScatTree, err := treesitter.Parse(dstScatBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstScat: %v", err)
	}
	elsewhereTree, err := treesitter.Parse(elsewhereBytes, "test.go")
	if err != nil {
		t.Fatalf("parse elsewhere: %v", err)
	}
	srcScatA, _ := testutil.FindNodeByText(srcScatTree, srcScatBytes, "a")
	srcScatGlue, _ := testutil.FindNodeByText(srcScatTree, srcScatBytes, ":=")
	srcScatB, _ := testutil.FindNodeByText(srcScatTree, srcScatBytes, "b")
	dstScatGlue, _ := testutil.FindNodeByText(dstScatTree, dstScatBytes, ":=")
	ew1, _ := testutil.FindNodeByText(elsewhereTree, elsewhereBytes, "elsewhere")
	ew2, _ := testutil.FindNodeByText(elsewhereTree, elsewhereBytes, "elsewhere2")

	m5.Add(srcScatA, ew1)
	m5.Add(srcScatGlue, dstScatGlue)
	m5.Add(srcScatB, ew2)
	RevokeOrphanedGlue(m5)
	testutil.AssertUnmappedText(t, srcScatTree, srcScatBytes, m5.Src(), ":=")

	// Stationary glue (same container): kept even with unmatched sibling.
	srcSameBytes := []byte("package main\nfunc f() {\n\tm := NewMapping()\n}")
	dstSameBytes := []byte("package main\nfunc f() {\n\tm := 1\n}")
	srcSameTree, err := treesitter.Parse(srcSameBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcSame: %v", err)
	}
	dstSameTree, err := treesitter.Parse(dstSameBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstSame: %v", err)
	}
	srcFunc, _ := testutil.FindNodeByText(srcSameTree, srcSameBytes, "func f() {\n\tm := NewMapping()\n}")
	dstFunc, _ := testutil.FindNodeByText(dstSameTree, dstSameBytes, "func f() {\n\tm := 1\n}")
	srcBlock, _ := testutil.FindNodeByText(srcSameTree, srcSameBytes, "{\n\tm := NewMapping()\n}")
	dstBlock, _ := testutil.FindNodeByText(dstSameTree, dstSameBytes, "{\n\tm := 1\n}")
	srcDecl, _ := testutil.FindNodeByText(srcSameTree, srcSameBytes, "m := NewMapping()")
	dstDecl, _ := testutil.FindNodeByText(dstSameTree, dstSameBytes, "m := 1")
	srcSameGlue, _ := testutil.FindNodeByText(srcSameTree, srcSameBytes, ":=")
	dstSameGlue, _ := testutil.FindNodeByText(dstSameTree, dstSameBytes, ":=")
	m4 := NewMapping()
	m4.Add(srcFunc, dstFunc)
	m4.Add(srcBlock, dstBlock)
	m4.Add(srcDecl, dstDecl)
	m4.Add(srcSameGlue, dstSameGlue)
	RevokeOrphanedGlue(m4)
	testutil.AssertMappedText(t, srcSameTree, dstSameTree, srcSameBytes, dstSameBytes, m4.Src(), ":=", ":=")

	// Fully matched construct: glue kept.
	m2 := NewMapping()
	srcFullBytes := []byte("package main\nfunc f() {\n\tm := x\n}")
	dstFullBytes := []byte("package main\nfunc f() {\n\tm := x\n}")
	srcFullTree, err := treesitter.Parse(srcFullBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcFull: %v", err)
	}
	dstFullTree, err := treesitter.Parse(dstFullBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstFull: %v", err)
	}
	srcFullDecl, _ := testutil.FindNodeByText(srcFullTree, srcFullBytes, "m := x")
	dstFullDecl, _ := testutil.FindNodeByText(dstFullTree, dstFullBytes, "m := x")
	srcFullM, _ := testutil.FindNodeByText(srcFullTree, srcFullBytes, "m")
	dstFullM, _ := testutil.FindNodeByText(dstFullTree, dstFullBytes, "m")
	srcFullGlue, _ := testutil.FindNodeByText(srcFullTree, srcFullBytes, ":=")
	dstFullGlue, _ := testutil.FindNodeByText(dstFullTree, dstFullBytes, ":=")
	srcFullX, _ := testutil.FindNodeByText(srcFullTree, srcFullBytes, "x")
	dstFullX, _ := testutil.FindNodeByText(dstFullTree, dstFullBytes, "x")

	m2.Add(srcFullDecl, dstFullDecl)
	m2.Add(srcFullM, dstFullM)
	m2.Add(srcFullGlue, dstFullGlue)
	m2.Add(srcFullX, dstFullX)
	RevokeOrphanedGlue(m2)
	testutil.AssertMappedText(t, srcFullTree, dstFullTree, srcFullBytes, dstFullBytes, m2.Src(), ":=", ":=")

	// Moving container with zero non-glue children matched:
	// Reparented across blocks, only glue matched -> revoked.
	srcMoveBytes := []byte("package main\nfunc f() {\n\tsrcVar := oldVal\n}")
	dstMoveBytes := []byte("package main\nfunc g() {\n\tdstVar := newVal\n}")
	srcMoveTree, err := treesitter.Parse(srcMoveBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcMove: %v", err)
	}
	dstMoveTree, err := treesitter.Parse(dstMoveBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstMove: %v", err)
	}
	srcMoveDecl, _ := testutil.FindNodeByText(srcMoveTree, srcMoveBytes, "srcVar := oldVal")
	dstMoveDecl, _ := testutil.FindNodeByText(dstMoveTree, dstMoveBytes, "dstVar := newVal")
	srcMoveGlue, _ := testutil.FindNodeByText(srcMoveTree, srcMoveBytes, ":=")
	dstMoveGlue, _ := testutil.FindNodeByText(dstMoveTree, dstMoveBytes, ":=")
	mMoving := NewMapping()
	mMoving.Add(srcMoveDecl, dstMoveDecl)
	mMoving.Add(srcMoveGlue, dstMoveGlue)
	RevokeOrphanedGlue(mMoving)
	testutil.AssertUnmappedText(t, srcMoveTree, srcMoveBytes, mMoving.Src(), "srcVar := oldVal")
	testutil.AssertUnmappedText(t, srcMoveTree, srcMoveBytes, mMoving.Src(), ":=")

	// Moving container with only a bare identifier or single-child wrapper matched:
	// bare tokens (height = 1) do not have structural mass to anchor a moving container across blocks.
	srcMoveLeafBytes := []byte("package main\nfunc f() {\n\tcommonVar := oldVal\n}")
	dstMoveLeafBytes := []byte("package main\nfunc g() {\n\tcommonVar := newVal\n}")
	srcMoveLeafTree, err := treesitter.Parse(srcMoveLeafBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcMoveLeaf: %v", err)
	}
	dstMoveLeafTree, err := treesitter.Parse(dstMoveLeafBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstMoveLeaf: %v", err)
	}
	srcLeafDecl, _ := testutil.FindNodeByText(srcMoveLeafTree, srcMoveLeafBytes, "commonVar := oldVal")
	dstLeafDecl, _ := testutil.FindNodeByText(dstMoveLeafTree, dstMoveLeafBytes, "commonVar := newVal")
	srcCommonVar, _ := testutil.FindNodeByText(srcMoveLeafTree, srcMoveLeafBytes, "commonVar")
	dstCommonVar, _ := testutil.FindNodeByText(dstMoveLeafTree, dstMoveLeafBytes, "commonVar")
	srcLeafGlue, _ := testutil.FindNodeByText(srcMoveLeafTree, srcMoveLeafBytes, ":=")
	dstLeafGlue, _ := testutil.FindNodeByText(dstMoveLeafTree, dstMoveLeafBytes, ":=")
	mRevokeLeaf := NewMapping()
	mRevokeLeaf.Add(srcLeafDecl, dstLeafDecl)
	mRevokeLeaf.Add(srcCommonVar, dstCommonVar)
	mRevokeLeaf.Add(srcLeafGlue, dstLeafGlue)
	RevokeOrphanedGlue(mRevokeLeaf)
	testutil.AssertUnmappedText(t, srcMoveLeafTree, srcMoveLeafBytes, mRevokeLeaf.Src(), "commonVar := oldVal")
	testutil.AssertUnmappedText(t, srcMoveLeafTree, srcMoveLeafBytes, mRevokeLeaf.Src(), ":=")

	// Single-child wrapper (expression_list -> identifier, effectiveHeight = 1): revoked.
	srcLHSWrap, _ := testutil.FindNodeByTextQuery(srcMoveLeafTree, srcMoveLeafBytes, testutil.TextQuery{Text: "commonVar", Type: "expression_list"})
	dstLHSWrap, _ := testutil.FindNodeByTextQuery(dstMoveLeafTree, dstMoveLeafBytes, testutil.TextQuery{Text: "commonVar", Type: "expression_list"})
	mRevokeWrap := NewMapping()
	mRevokeWrap.Add(srcLeafDecl, dstLeafDecl)
	mRevokeWrap.Add(srcLHSWrap, dstLHSWrap)
	mRevokeWrap.Add(srcCommonVar, dstCommonVar)
	mRevokeWrap.Add(srcLeafGlue, dstLeafGlue)
	RevokeOrphanedGlue(mRevokeWrap)
	testutil.AssertUnmappedText(t, srcMoveLeafTree, srcMoveLeafBytes, mRevokeWrap.Src(), "commonVar := oldVal")
	testutil.AssertUnmappedText(t, srcMoveLeafTree, srcMoveLeafBytes, mRevokeWrap.Src(), ":=")

	// Moving container with matched compound child (effectiveHeight >= 2, e.g. RHS call_expression):
	// Reparented across blocks, compound RHS and glue matched -> preserved.
	srcCompoundBytes := []byte("package main\nfunc f() {\n\toldVar := compute(x)\n}")
	dstCompoundBytes := []byte("package main\nfunc g() {\n\tnewVar := compute(x)\n}")
	srcCompoundTree, err := treesitter.Parse(srcCompoundBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcCompound: %v", err)
	}
	dstCompoundTree, err := treesitter.Parse(dstCompoundBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstCompound: %v", err)
	}
	srcCompDecl, _ := testutil.FindNodeByText(srcCompoundTree, srcCompoundBytes, "oldVar := compute(x)")
	dstCompDecl, _ := testutil.FindNodeByText(dstCompoundTree, dstCompoundBytes, "newVar := compute(x)")
	srcRHSWrap, _ := testutil.FindNodeByTextQuery(srcCompoundTree, srcCompoundBytes, testutil.TextQuery{Text: "compute(x)", Type: "expression_list"})
	dstRHSWrap, _ := testutil.FindNodeByTextQuery(dstCompoundTree, dstCompoundBytes, testutil.TextQuery{Text: "compute(x)", Type: "expression_list"})
	srcRHSCall, _ := testutil.FindNodeByText(srcCompoundTree, srcCompoundBytes, "compute(x)")
	dstRHSCall, _ := testutil.FindNodeByText(dstCompoundTree, dstCompoundBytes, "compute(x)")
	srcCompGlue, _ := testutil.FindNodeByText(srcCompoundTree, srcCompoundBytes, ":=")
	dstCompGlue, _ := testutil.FindNodeByText(dstCompoundTree, dstCompoundBytes, ":=")
	mPreserve := NewMapping()
	mPreserve.Add(srcCompDecl, dstCompDecl)
	mPreserve.Add(srcRHSWrap, dstRHSWrap)
	mPreserve.Add(srcRHSCall, dstRHSCall)
	mPreserve.Add(srcCompGlue, dstCompGlue)
	RevokeOrphanedGlue(mPreserve)
	testutil.AssertMappedText(t, srcCompoundTree, dstCompoundTree, srcCompoundBytes, dstCompoundBytes, mPreserve.Src(), "oldVar := compute(x)", "newVar := compute(x)")
	testutil.AssertMappedText(t, srcCompoundTree, dstCompoundTree, srcCompoundBytes, dstCompoundBytes, mPreserve.Src(), ":=", ":=")

	// Keyword glue recognition: if keyword across moving blocks with no non-glue matches.
	srcIfBytes := []byte("package main\nfunc f() {\n\tif condA {}\n}")
	dstIfBytes := []byte("package main\nfunc g() {\n\tif condB {}\n}")
	srcIfTree, err := treesitter.Parse(srcIfBytes, "test.go")
	if err != nil {
		t.Fatalf("parse srcIf: %v", err)
	}
	dstIfTree, err := treesitter.Parse(dstIfBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dstIf: %v", err)
	}
	srcIf, _ := testutil.FindNodeByText(srcIfTree, srcIfBytes, "if condA {}")
	dstIf, _ := testutil.FindNodeByText(dstIfTree, dstIfBytes, "if condB {}")
	srcKw, _ := testutil.FindNodeByTextQuery(srcIfTree, srcIfBytes, testutil.TextQuery{Text: "if", Type: "if"})
	dstKw, _ := testutil.FindNodeByTextQuery(dstIfTree, dstIfBytes, testutil.TextQuery{Text: "if", Type: "if"})
	mKw := NewMapping()
	mKw.Add(srcIf, dstIf)
	mKw.Add(srcKw, dstKw)
	RevokeOrphanedGlue(mKw)
	testutil.AssertUnmappedText(t, srcIfTree, srcIfBytes, mKw.Src(), "if condA {}")
	testutil.AssertUnmappedText(t, srcIfTree, srcIfBytes, mKw.Src(), "if", "if")

	// A moved statement with only leaf tokens (like let far_future = 10_000_000)
	// should be preserved when both sides are isomorphic.
	srcIsoBytes := []byte("fn f() {\n\tlet far_future = 10_000_000;\n}")
	dstIsoBytes := []byte("fn g() {\n\tlet far_future = 10_000_000;\n}")
	srcIsoTree, err := treesitter.Parse(srcIsoBytes, "test.rs")
	if err != nil {
		t.Fatalf("parse srcIso: %v", err)
	}
	dstIsoTree, err := treesitter.Parse(dstIsoBytes, "test.rs")
	if err != nil {
		t.Fatalf("parse dstIso: %v", err)
	}
	srcLet, _ := testutil.FindNodeByText(srcIsoTree, srcIsoBytes, "let far_future = 10_000_000;")
	dstLet, _ := testutil.FindNodeByText(dstIsoTree, dstIsoBytes, "let far_future = 10_000_000;")
	mIso := NewMapping()
	mIso.Add(srcLet, dstLet)
	for i := range srcLet.Children {
		if i < len(dstLet.Children) {
			mIso.Add(srcLet.Children[i], dstLet.Children[i])
		}
	}
	RevokeOrphanedGlue(mIso)
	testutil.AssertMappedText(t, srcIsoTree, dstIsoTree, srcIsoBytes, dstIsoBytes, mIso.Src(), "let far_future = 10_000_000;", "let far_future = 10_000_000;")
	testutil.AssertMappedText(t, srcIsoTree, dstIsoTree, srcIsoBytes, dstIsoBytes, mIso.Src(), "let", "let", "let")

	// A moved statement should also stay mapped if all non-glue children match,
	// even when the destination adds a keyword like mut.
	dstMutBytes := []byte("fn g() {\n\tlet mut far_future = 10_000_000;\n}")
	dstMutTree, err := treesitter.Parse(dstMutBytes, "test.rs")
	if err != nil {
		t.Fatalf("parse dstMut: %v", err)
	}
	dstMutLet, _ := testutil.FindNodeByText(dstMutTree, dstMutBytes, "let mut far_future = 10_000_000;")
	srcLetKw, _ := testutil.FindNodeByTextQuery(srcIsoTree, srcIsoBytes, testutil.TextQuery{Text: "let", Type: "let"})
	dstLetKw, _ := testutil.FindNodeByTextQuery(dstMutTree, dstMutBytes, testutil.TextQuery{Text: "let", Type: "let"})
	srcIdent, _ := testutil.FindNodeByText(srcIsoTree, srcIsoBytes, "far_future")
	dstIdent, _ := testutil.FindNodeByText(dstMutTree, dstMutBytes, "far_future")
	srcEq, _ := testutil.FindNodeByText(srcIsoTree, srcIsoBytes, "=")
	dstEq, _ := testutil.FindNodeByText(dstMutTree, dstMutBytes, "=")
	srcInt, _ := testutil.FindNodeByText(srcIsoTree, srcIsoBytes, "10_000_000")
	dstInt, _ := testutil.FindNodeByText(dstMutTree, dstMutBytes, "10_000_000")

	mAllNonGlue := NewMapping()
	mAllNonGlue.Add(srcLet, dstMutLet)
	mAllNonGlue.Add(srcLetKw, dstLetKw)
	mAllNonGlue.Add(srcIdent, dstIdent)
	mAllNonGlue.Add(srcEq, dstEq)
	mAllNonGlue.Add(srcInt, dstInt)
	RevokeOrphanedGlue(mAllNonGlue)
	testutil.AssertMappedText(t, srcIsoTree, dstMutTree, srcIsoBytes, dstMutBytes, mAllNonGlue.Src(), "let far_future = 10_000_000;", "let mut far_future = 10_000_000;")
}

func TestRecoverGlueAnchors_BinaryExpression_RHSMatched(t *testing.T) {
	srcBytes := []byte("package main\nfunc f() {\n\t_ = oldVar == commonVal\n}")
	dstBytes := []byte("package main\nfunc f() {\n\t_ = newVar == commonVal\n}")
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNodeByText(srcTree, srcBytes, "func f() {\n\t_ = oldVar == commonVal\n}", "function_declaration")
	dstFunc, _ := testutil.FindNodeByText(dstTree, dstBytes, "func f() {\n\t_ = newVar == commonVal\n}", "function_declaration")
	srcRHS, _ := testutil.FindNodeByText(srcTree, srcBytes, "commonVal", "identifier")
	dstRHS, _ := testutil.FindNodeByText(dstTree, dstBytes, "commonVal", "identifier")

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcRHS, dstRHS)

	RecoverGlueAnchors(m)

	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"oldVar == commonVal",
		"newVar == commonVal",
		"binary_expression",
	)
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"oldVar",
		"newVar",
		"identifier",
	)
}

func TestRecoverGlueAnchors_BinaryExpression_LHSMatched(t *testing.T) {
	srcBytes := []byte("package main\nfunc f() {\n\t_ = commonVal == oldRHS\n}")
	dstBytes := []byte("package main\nfunc f() {\n\t_ = commonVal == newRHS\n}")
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNodeByText(srcTree, srcBytes, "func f() {\n\t_ = commonVal == oldRHS\n}", "function_declaration")
	dstFunc, _ := testutil.FindNodeByText(dstTree, dstBytes, "func f() {\n\t_ = commonVal == newRHS\n}", "function_declaration")
	srcLHS, _ := testutil.FindNodeByText(srcTree, srcBytes, "commonVal", "identifier")
	dstLHS, _ := testutil.FindNodeByText(dstTree, dstBytes, "commonVal", "identifier")

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcLHS, dstLHS)

	RecoverGlueAnchors(m)

	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"commonVal == oldRHS",
		"commonVal == newRHS",
		"binary_expression",
	)
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"oldRHS",
		"newRHS",
		"identifier",
	)
}

func TestRecoverGlueAnchors_BinaryExpression_OppositeSidesNotMatched(t *testing.T) {
	// x = y (y is on RHS) vs y = z (y is on LHS)
	srcBytes := []byte("package main\nfunc f() {\n\tx = y\n}")
	dstBytes := []byte("package main\nfunc f() {\n\ty = z\n}")
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcY, _ := testutil.FindNodeByText(srcTree, srcBytes, "y", "identifier")
	dstY, _ := testutil.FindNodeByText(dstTree, dstBytes, "y", "identifier")

	m := NewMapping()
	m.Add(srcY, dstY)

	RecoverGlueAnchors(m)

	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), "x = y", "assignment_statement")
	testutil.AssertUnmappedText(t, dstTree, dstBytes, m.Dst(), "y = z", "assignment_statement")
}

func TestRecoverGlueAnchors_ControlFlow_BodyMatched(t *testing.T) {
	srcBytes := []byte("package main\nfunc f() {\n\tif oldCond {\n\t\tstmt1()\n\t}\n}")
	dstBytes := []byte("package main\nfunc f() {\n\tif newCond {\n\t\tstmt1()\n\t}\n}")
	srcTree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNodeByText(srcTree, srcBytes, "func f() {\n\tif oldCond {\n\t\tstmt1()\n\t}\n}", "function_declaration")
	dstFunc, _ := testutil.FindNodeByText(dstTree, dstBytes, "func f() {\n\tif newCond {\n\t\tstmt1()\n\t}\n}", "function_declaration")
	srcBody, _ := testutil.FindNodeByText(srcTree, srcBytes, "{\n\t\tstmt1()\n\t}", "block")
	dstBody, _ := testutil.FindNodeByText(dstTree, dstBytes, "{\n\t\tstmt1()\n\t}", "block")

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcBody, dstBody)

	RecoverGlueAnchors(m)

	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"if oldCond {\n\t\tstmt1()\n\t}",
		"if newCond {\n\t\tstmt1()\n\t}",
		"if_statement",
	)
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(),
		"oldCond",
		"newCond",
		"identifier",
	)
}

func TestRecoverGlueAnchors_BoilerplateGuard(t *testing.T) {
	src := "package main\nfunc f() {\n\tif err != nil {\n\t\tstmtA()\n\t}\n}"
	dst := "package main\nfunc f() {\n" + strings.Repeat("\t// line padding\n", 70) + "\tif err != nil {\n\t\tstmtB()\n\t}\n}"
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
	srcCond, _ := testutil.FindNodeByText(srcTree, srcBytes, "err != nil")
	dstCond, _ := testutil.FindNodeByText(dstTree, dstBytes, "err != nil")

	m := NewMapping()
	m.Add(srcCond, dstCond)

	RecoverGlueAnchors(m)

	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), "if err != nil {\n\t\tstmtA()\n\t}")
}

func TestIsOperatorGlue(t *testing.T) {
	r := rules.Get("go")
	src := `package main
func ops() {
	a = b
	c := b
	_ = a == b
	_ = a != b
	_ = a + b
	a += b
	_ = a & b
	_ = a || b
	_ = a < b
	_ = err
	_ = foo
	_ = s[1:2]
	_ = []int{1, 2}
}
`
	srcBytes := []byte(src)
	tree, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}

	tests := []struct {
		name  string
		query testutil.TextQuery
		want  bool
	}{
		{"assign", testutil.TextQuery{Text: "=", Occurrence: 1}, true},
		{"short_var", testutil.TextQuery{Text: ":="}, true},
		{"equal", testutil.TextQuery{Text: "=="}, true},
		{"not_equal", testutil.TextQuery{Text: "!="}, true},
		{"plus", testutil.TextQuery{Text: "+", Occurrence: 1}, true},
		{"plus_assign", testutil.TextQuery{Text: "+="}, true},
		{"bitwise_and", testutil.TextQuery{Text: "&"}, true},
		{"logical_or", testutil.TextQuery{Text: "||"}, true},
		{"less_than", testutil.TextQuery{Text: "<"}, true},
		{"identifier_err", testutil.TextQuery{Text: "err"}, false},
		{"identifier_foo", testutil.TextQuery{Text: "foo"}, false},
		{"slice_expression_has_children", testutil.TextQuery{Text: "s[1:2]"}, false},
		{"composite_literal_has_children", testutil.TextQuery{Text: "[]int{1, 2}"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, err := testutil.FindNodeByTextQuery(tree, srcBytes, tc.query)
			if err != nil {
				t.Fatalf("find node %s: %v", tc.name, err)
			}
			if got := isOperatorGlue(node, r); got != tc.want {
				t.Errorf("isOperatorGlue(%v) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestIsBoilerplateCondition(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		ext      string
		condText string
		want     bool
	}{
		{
			name:     "go_err_not_nil",
			src:      "package main\nfunc f() {\n\tif err != nil {}\n}",
			ext:      "test.go",
			condText: "err != nil",
			want:     true,
		},
		{
			name:     "go_single_var_ok",
			src:      "package main\nfunc f() {\n\tif ok {}\n}",
			ext:      "test.go",
			condText: "ok",
			want:     true,
		},
		{
			name:     "go_negated_var",
			src:      "package main\nfunc f() {\n\tif !ok {}\n}",
			ext:      "test.go",
			condText: "!ok",
			want:     true,
		},
		{
			name:     "go_compare_zero",
			src:      "package main\nfunc f() {\n\tif count == 0 {}\n}",
			ext:      "test.go",
			condText: "count == 0",
			want:     true,
		},
		{
			name:     "java_null_check",
			src:      "class C {\n\tvoid f() {\n\t\tif (ptr == null) {}\n\t}\n}",
			ext:      "test.java",
			condText: "ptr == null",
			want:     true,
		},
		{
			name:     "python_none_check",
			src:      "def f():\n    if val is None:\n        pass",
			ext:      "test.py",
			condText: "val is None",
			want:     true,
		},
		{
			name:     "rust_none_check",
			src:      "fn f() {\n\tif opt != None {}\n}",
			ext:      "test.rs",
			condText: "opt != None",
			want:     true,
		},
		{
			name:     "c_null_check",
			src:      "void f() {\n\tif (ptr != NULL) {}\n}",
			ext:      "test.c",
			condText: "ptr != NULL",
			want:     true,
		},
		{
			name:     "js_undefined_check",
			src:      "function f() {\n\tif (arg !== undefined) {}\n}",
			ext:      "test.js",
			condText: "arg !== undefined",
			want:     true,
		},
		{
			name:     "two_variables_compare",
			src:      "package main\nfunc f() {\n\tif a == b {}\n}",
			ext:      "test.go",
			condText: "a == b",
			want:     false,
		},
		{
			name:     "two_variables_ordered",
			src:      "package main\nfunc f() {\n\tif min < max {}\n}",
			ext:      "test.go",
			condText: "min < max",
			want:     false,
		},
		{
			name:     "compound_call_condition",
			src:      "package main\nfunc f() {\n\tif len(items) > 0 {}\n}",
			ext:      "test.go",
			condText: "len(items) > 0",
			want:     false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srcBytes := []byte(tc.src)
			tree, err := treesitter.Parse(srcBytes, tc.ext)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			cond, err := testutil.FindNodeByText(tree, srcBytes, tc.condText)
			if err != nil {
				t.Fatalf("find cond %s: %v", tc.name, err)
			}
			r := rulesFor(cond)
			if got := isBoilerplateCondition(cond, r); got != tc.want {
				t.Errorf("isBoilerplateCondition(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRevokeOrphanedGlue_RelocatedControlFlowAnchoredSolelyByKeyword(t *testing.T) {
	src := "package main\nfunc f() {\n\tif isErr() {\n\t\treturn\n\t}\n}"
	dst := "package main\nfunc f() {\n" + strings.Repeat("\t// line padding\n", 35) + "\tif !isErr() {\n\t\tlogErr()\n\t}\n}"
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

	outerBlockSrc, _ := testutil.FindNodeByText(srcTree, srcBytes, "{\n\tif isErr() {\n\t\treturn\n\t}\n}")
	outerBlockDst, _ := testutil.FindNodeByText(dstTree, dstBytes, dst[strings.Index(dst, "{\n\t//"):])
	srcIf, _ := testutil.FindNodeByText(srcTree, srcBytes, "if isErr() {\n\t\treturn\n\t}")
	dstIf, _ := testutil.FindNodeByText(dstTree, dstBytes, "if !isErr() {\n\t\tlogErr()\n\t}")
	kwOld, _ := testutil.FindNodeByTextQuery(srcTree, srcBytes, testutil.TextQuery{Text: "if", Type: "if"})
	kwNew, _ := testutil.FindNodeByTextQuery(dstTree, dstBytes, testutil.TextQuery{Text: "if", Type: "if"})

	m := NewMapping()
	m.Add(outerBlockSrc, outerBlockDst)
	m.Add(srcIf, dstIf)
	m.Add(kwOld, kwNew)

	RevokeOrphanedGlue(m)

	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), "if isErr() {\n\t\treturn\n\t}")
	testutil.AssertUnmappedText(t, srcTree, srcBytes, m.Src(), "if", "if")
}

func TestRevokeOrphanedGlue_AdjacentControlFlowAnchoredByKeywordPreserved(t *testing.T) {
	src := "package main\nfunc f() {\n\tif isErr() {\n\t\treturn\n\t}\n}"
	dst := "package main\nfunc f() {\n\t_ = 1\n\tif !isErr() {\n\t\tlogErr()\n\t}\n}"
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

	srcFunc, _ := testutil.FindNodeByText(srcTree, srcBytes, "func f() {\n\tif isErr() {\n\t\treturn\n\t}\n}")
	dstFunc, _ := testutil.FindNodeByText(dstTree, dstBytes, "func f() {\n\t_ = 1\n\tif !isErr() {\n\t\tlogErr()\n\t}\n}")
	outerBlockSrc, _ := testutil.FindNodeByText(srcTree, srcBytes, "{\n\tif isErr() {\n\t\treturn\n\t}\n}")
	outerBlockDst, _ := testutil.FindNodeByText(dstTree, dstBytes, "{\n\t_ = 1\n\tif !isErr() {\n\t\tlogErr()\n\t}\n}")
	srcIf, _ := testutil.FindNodeByText(srcTree, srcBytes, "if isErr() {\n\t\treturn\n\t}")
	dstIf, _ := testutil.FindNodeByText(dstTree, dstBytes, "if !isErr() {\n\t\tlogErr()\n\t}")
	kwOld, _ := testutil.FindNodeByTextQuery(srcTree, srcBytes, testutil.TextQuery{Text: "if", Type: "if"})
	kwNew, _ := testutil.FindNodeByTextQuery(dstTree, dstBytes, testutil.TextQuery{Text: "if", Type: "if"})

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(outerBlockSrc, outerBlockDst)
	m.Add(srcIf, dstIf)
	m.Add(kwOld, kwNew)

	RevokeOrphanedGlue(m)

	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(), "if isErr() {\n\t\treturn\n\t}", "if !isErr() {\n\t\tlogErr()\n\t}")
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, m.Src(), "if", "if", "if")
}

func TestRevokeOrphanedGlue_BareJumpPreservedInMovingBlock(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func processOld(items []int) {
	for _, item := range items {
		if item < 0 {
			break
		}
		handle(item)
	}
}

func otherCaller() {
	println("hello")
}
`
		dst := `package main

func processOld(items []int) {
	println("hello")
}

func helperExtracted(items []int) {
	for _, item := range items {
		if item < 0 {
			break
		}
		handle(item)
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

		// Whole-snippet verification: all loop statements and outer functions accounted for
		// Enclosing function processOld maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func processOld(items []int)",
			"func processOld(items []int)",
		)

		// Deleted otherCaller and inserted helperExtracted
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"func otherCaller()",
		)
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"func helperExtracted(items []int)",
		)

		// Loop maps to extracted helper
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"for _, item := range items",
			"for _, item := range items",
		)

		// Target break_statement inside moved loop block is preserved and mapped
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"break",
			"break",
		)

		// Sibling statement handle(item) remains mapped
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"handle(item)",
			"handle(item)",
		)

		// println("hello") maps to moved caller
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			`println("hello")`,
			`println("hello")`,
		)
	})
}

func TestRevokeOrphanedGlue_MovingScalarSubExpressionPreserved(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func Process(x any) {
	if x != nil && validate(x) {
		println("valid")
	}
}

func Helper(x any) bool {
	return false
}
`
		dst := `package main

func Process(x any) {
	if Helper(x) {
		println("valid")
	}
}

func Helper(x any) bool {
	return x != nil && validate(x)
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

		// Whole-snippet verification: both functions and their statements map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func Process(x any)",
			"func Process(x any)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func Helper(x any) bool",
			"func Helper(x any) bool",
		)

		// Target moving scalar binary expression (x != nil) remains mapped
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"x != nil",
			"x != nil",
		)

		// Stationary println("valid")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			`println("valid")`,
			`println("valid")`,
		)

		// Return statement in Helper maps
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"return false",
			"return x != nil && validate(x)",
		)
	})
}

func TestBinaryOperatorGlue_SingleOperandMutation(t *testing.T) {
	r := rules.Get("go")
	if r == nil {
		t.Fatal("rules for go not found")
	}

	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func renderTest(left, right, lastSrcLineIdx, lastDstLineIdx int, sText, dText string, srcEndsWithNL, dstEndsWithNL bool) {
	isEOFLine := (left == lastSrcLineIdx && right == lastDstLineIdx)
	if sText != dText || (isEOFLine && srcEndsWithNL != dstEndsWithNL) {
		upd++
	}
}
`
		dst := `package main

func renderTest(left, right, lastSrcLineIdx, lastDstLineIdx int, sText, dText string, srcEndsWithNL, dstEndsWithNL bool) {
	isEOFLine := (left == lastSrcLineIdx && right == lastDstLineIdx)
	isEOFChange := isEOFLine && srcEndsWithNL != dstEndsWithNL
	if sText != dText || isEOFChange {
		if showStyleChanges || isEOFChange || !renderutil.EqualIgnoringStyle(sText, dText, significantWhitespace) {
			upd++
		}
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
		mr := Match(srcAST.Root(), dstAST.Root(), []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: outer func renderTest maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func renderTest",
			"func renderTest",
		)

		// 2. Surrounding sequence: isEOFLine declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"isEOFLine :=",
			"isEOFLine :=",
		)

		// 3. Stationary body: upd++ maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"upd++",
			"upd++",
		)

		// 4. Target operand: sText != dText must map to sText != dText
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"sText != dText",
			"sText != dText",
		)

		// 5. Outer binary expression: sText != dText || ... must NOT map to sText != dText || isEOFChange
		testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"sText != dText || (isEOFLine && srcEndsWithNL != dstEndsWithNL)",
			"sText != dText || isEOFChange",
		)
	})
}

// TestRevokeOrphanedGlue_NestedPathPayloadPreservedAcrossLineDrift verifies that trimming
// or expanding import paths (like Rust `use`, C++ `using`, Zig `usingnamespace`, or Java `import`)
// keeps the declaration mapped in place when upstream lines drift, rather than revoking it into
// delete and insert.
func TestRevokeOrphanedGlue_NestedPathPayloadPreservedAcrossLineDrift(t *testing.T) {
	t.Run("FullEngine_Rust", func(t *testing.T) {
		src := `fn top_helper() {
    let a = 1;
    let b = 2;
}

mod test_utils {
    use std::fs::remove_file;

    use crate::tests::integration_test_utils::integration_test_utils;

    #[test]
    fn test_diff_highlight_defaults() {
        let opt = 42;
    }
}
`
		dst := `fn top_helper() {
    let a = 1;
    let b = 2;
    let c = 3;
    let d = 4;
    let e = 5;
    let f = 6;
}

mod test_utils {
    use std::fs::remove_file;

    use crate::tests::integration_test_utils;

    #[test]
    fn test_diff_highlight_defaults() {
        let opt = 42;
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.ParseErrorCount != 0 || dstAST.ParseErrorCount != 0 {
			t.Fatalf("expected zero parse errors, got src=%d, dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Enclosing function, module, and declaration list must stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn top_helper()",
			"fn top_helper()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"mod test_utils",
			"mod test_utils",
			"mod_item",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use std::fs::remove_file;",
			"use std::fs::remove_file;",
			"declaration_list",
		)

		// Surrounding statements must stay mapped without disruption.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use std::fs::remove_file;",
			"use std::fs::remove_file;",
			"use_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn test_diff_highlight_defaults()",
			"fn test_diff_highlight_defaults()",
		)

		// Target use declaration must stay mapped despite upstream line drift.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::tests::integration_test_utils::integration_test_utils;",
			"use crate::tests::integration_test_utils;",
			"use_declaration",
		)

		// Shared path prefix must stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"crate::tests::integration_test_utils",
			"crate::tests::integration_test_utils",
			"scoped_identifier",
		)
	})

	t.Run("FullEngine_CPP", func(t *testing.T) {
		src := `void setup() {
    int a = 1;
    int b = 2;
}

namespace test_env {
    void init() {}
    using foo::bar::baz;
    void teardown() {}
}
`
		dst := `void setup() {
    int a = 1;
    int b = 2;
    int c = 3;
    int d = 4;
    int e = 5;
    int f = 6;
}

namespace test_env {
    void init() {}
    using bar::baz;
    void teardown() {}
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.cpp")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.cpp")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.ParseErrorCount != 0 || dstAST.ParseErrorCount != 0 {
			t.Fatalf("expected zero parse errors, got src=%d, dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Enclosing function, namespace, and declaration list must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void setup()",
			"void setup()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"namespace test_env",
			"namespace test_env",
			"namespace_definition",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void init() {}",
			"void init() {}",
			"declaration_list",
		)

		// Surrounding statements must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void init() {}",
			"void init() {}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void teardown() {}",
			"void teardown() {}",
		)

		// Target using declaration must stay mapped despite upstream line drift.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"using foo::bar::baz;",
			"using bar::baz;",
			"using_declaration",
		)

		// Shared qualified identifier must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"bar::baz",
			"bar::baz",
			"qualified_identifier",
		)
	})

	t.Run("FullEngine_Zig", func(t *testing.T) {
		src := `fn setup() void {
    const a = 1;
    const b = 2;
}

const test_mod = struct {
    fn before() void {}
    pub usingnamespace foo.bar.baz;
    fn after() void {}
};
`
		dst := `fn setup() void {
    const a = 1;
    const b = 2;
    const c = 3;
    const d = 4;
    const e = 5;
    const f = 6;
}

const test_mod = struct {
    fn before() void {}
    pub usingnamespace foo.bar;
    fn after() void {}
};
`
		srcAST, err := treesitter.Parse([]byte(src), "test.zig")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.zig")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.ParseErrorCount != 0 || dstAST.ParseErrorCount != 0 {
			t.Fatalf("expected zero parse errors, got src=%d, dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Enclosing function and module struct must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn setup() void",
			"fn setup() void",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"const test_mod = struct",
			"const test_mod = struct",
			"variable_declaration",
		)

		// Surrounding functions must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn before() void",
			"fn before() void",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn after() void",
			"fn after() void",
		)

		// Target usingnamespace declaration must stay mapped despite upstream line drift.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"pub usingnamespace foo.bar.baz;",
			"pub usingnamespace foo.bar;",
			"using_namespace_declaration",
		)

		// Shared field expression must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"foo.bar",
			"foo.bar",
			"field_expression",
		)
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		src := `// header comment 1
// header comment 2

package test_pkg;

import com.example.foo.Bar;

class TargetTest {
    void run() {}
}
`
		dst := `// header comment 1
// header comment 2
// header comment 3
// header comment 4
// header comment 5
// header comment 6

package test_pkg;

import com.example.foo;

class TargetTest {
    void run() {}
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
		if srcAST.ParseErrorCount != 0 || dstAST.ParseErrorCount != 0 {
			t.Fatalf("expected zero parse errors, got src=%d, dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Enclosing package and class declarations must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"package test_pkg;",
			"package test_pkg;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"class TargetTest",
			"class TargetTest",
		)

		// Target import declaration must stay mapped despite upstream line drift.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"import com.example.foo.Bar;",
			"import com.example.foo;",
			"import_declaration",
		)

		// Shared scoped identifier must stay mapped.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"com.example.foo",
			"com.example.foo",
			"scoped_identifier",
		)
	})
}
