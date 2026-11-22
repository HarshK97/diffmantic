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
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tm := NewMapping()\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\ty := 1\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	glue, err := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	if err != nil {
		t.Fatalf("find glue: %v", err)
	}
	unmatched, err := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "call_expression", Line: 3})
	if err != nil {
		t.Fatalf("find unmatched: %v", err)
	}
	dstGlue, err := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	if err != nil {
		t.Fatalf("find dstGlue: %v", err)
	}
	m := NewMapping()
	m.Add(glue, dstGlue)
	RevokeOrphanedGlue(m)
	if m.Has(glue) {
		t.Fatal("reparented orphaned glue should be revoked")
	}
	if m.Has(unmatched) {
		t.Fatal("unmatched sibling should stay unmapped")
	}

	// Reparented glue revoked even when siblings mapped elsewhere
	// (scattered matches in repetitive code): glue must never move.
	m5 := NewMapping()
	srcScatTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\ta := b\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcScat: %v", err)
	}
	dstScatTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\tc := d\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstScat: %v", err)
	}
	elsewhereTree, err := treesitter.Parse([]byte("package main\nfunc h() {\n\t_ = elsewhere\n\t_ = elsewhere2\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse elsewhere: %v", err)
	}
	srcScatA, _ := testutil.FindNode(srcScatTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	srcScatGlue, _ := testutil.FindNode(srcScatTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	srcScatB, _ := testutil.FindNode(srcScatTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	dstScatGlue, _ := testutil.FindNode(dstScatTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	ew1, _ := testutil.FindNode(elsewhereTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	ew2, _ := testutil.FindNode(elsewhereTree, testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 2})

	m5.Add(srcScatA, ew1)
	m5.Add(srcScatGlue, dstScatGlue)
	m5.Add(srcScatB, ew2)
	RevokeOrphanedGlue(m5)
	if m5.Has(srcScatGlue) {
		t.Fatal("reparented glue with scattered siblings should be revoked")
	}

	// Stationary glue (same container): kept even with unmatched sibling.
	srcSameTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tm := NewMapping()\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcSame: %v", err)
	}
	dstSameTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tm := 1\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstSame: %v", err)
	}
	srcFunc, _ := testutil.FindNode(srcSameTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	dstFunc, _ := testutil.FindNode(dstSameTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	srcBlock, _ := testutil.FindNode(srcSameTree, testutil.NodeQuery{Type: "block", Line: 2})
	dstBlock, _ := testutil.FindNode(dstSameTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcDecl, _ := testutil.FindNode(srcSameTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	dstDecl, _ := testutil.FindNode(dstSameTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	srcSameGlue, _ := testutil.FindNode(srcSameTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	dstSameGlue, _ := testutil.FindNode(dstSameTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	m4 := NewMapping()
	m4.Add(srcFunc, dstFunc)
	m4.Add(srcBlock, dstBlock)
	m4.Add(srcDecl, dstDecl)
	m4.Add(srcSameGlue, dstSameGlue)
	RevokeOrphanedGlue(m4)
	if !m4.Has(srcSameGlue) {
		t.Fatal("stationary glue should be kept")
	}

	// Fully matched construct: glue kept.
	m2 := NewMapping()
	srcFullTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tm := x\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcFull: %v", err)
	}
	dstFullTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tm := x\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstFull: %v", err)
	}
	srcFullDecl, _ := testutil.FindNode(srcFullTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	dstFullDecl, _ := testutil.FindNode(dstFullTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	srcFullM, _ := testutil.FindNode(srcFullTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	dstFullM, _ := testutil.FindNode(dstFullTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	srcFullGlue, _ := testutil.FindNode(srcFullTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	dstFullGlue, _ := testutil.FindNode(dstFullTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	srcFullX, _ := testutil.FindNode(srcFullTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	dstFullX, _ := testutil.FindNode(dstFullTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})

	m2.Add(srcFullDecl, dstFullDecl)
	m2.Add(srcFullM, dstFullM)
	m2.Add(srcFullGlue, dstFullGlue)
	m2.Add(srcFullX, dstFullX)
	RevokeOrphanedGlue(m2)
	if !m2.Has(srcFullGlue) {
		t.Fatal("glue in fully matched construct should be kept")
	}

	// Moving container with zero non-glue children matched:
	// Reparented across blocks, only glue matched -> revoked.
	srcMoveTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tsrcVar := oldVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcMove: %v", err)
	}
	dstMoveTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\tdstVar := newVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstMove: %v", err)
	}
	srcMoveDecl, _ := testutil.FindNode(srcMoveTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	dstMoveDecl, _ := testutil.FindNode(dstMoveTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	srcMoveGlue, _ := testutil.FindNode(srcMoveTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	dstMoveGlue, _ := testutil.FindNode(dstMoveTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	mMoving := NewMapping()
	mMoving.Add(srcMoveDecl, dstMoveDecl)
	mMoving.Add(srcMoveGlue, dstMoveGlue)
	RevokeOrphanedGlue(mMoving)
	if mMoving.Has(srcMoveDecl) {
		t.Fatal("moving container with zero non-glue matches should be revoked")
	}
	if mMoving.Has(srcMoveGlue) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Moving container with only a bare identifier or single-child wrapper matched:
	// bare tokens (height = 1) do not have structural mass to anchor a moving container across blocks.
	srcMoveLeafTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tcommonVar := oldVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcMoveLeaf: %v", err)
	}
	dstMoveLeafTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\tcommonVar := newVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstMoveLeaf: %v", err)
	}
	srcLeafDecl, _ := testutil.FindNode(srcMoveLeafTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	dstLeafDecl, _ := testutil.FindNode(dstMoveLeafTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	srcCommonVar, _ := testutil.FindNode(srcMoveLeafTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	dstCommonVar, _ := testutil.FindNode(dstMoveLeafTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})
	srcLeafGlue, _ := testutil.FindNode(srcMoveLeafTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	dstLeafGlue, _ := testutil.FindNode(dstMoveLeafTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	mRevokeLeaf := NewMapping()
	mRevokeLeaf.Add(srcLeafDecl, dstLeafDecl)
	mRevokeLeaf.Add(srcCommonVar, dstCommonVar)
	mRevokeLeaf.Add(srcLeafGlue, dstLeafGlue)
	RevokeOrphanedGlue(mRevokeLeaf)
	if mRevokeLeaf.Has(srcLeafDecl) {
		t.Fatal("moving container anchored solely by glue and bare identifier should be revoked")
	}
	if mRevokeLeaf.Has(srcLeafGlue) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Single-child wrapper (expression_list -> identifier, effectiveHeight = 1): revoked.
	srcLHSWrap, _ := testutil.FindNode(srcMoveLeafTree, testutil.NodeQuery{Type: "expression_list", Line: 3, Nth: 1})
	dstLHSWrap, _ := testutil.FindNode(dstMoveLeafTree, testutil.NodeQuery{Type: "expression_list", Line: 3, Nth: 1})
	mRevokeWrap := NewMapping()
	mRevokeWrap.Add(srcLeafDecl, dstLeafDecl)
	mRevokeWrap.Add(srcLHSWrap, dstLHSWrap)
	mRevokeWrap.Add(srcCommonVar, dstCommonVar)
	mRevokeWrap.Add(srcLeafGlue, dstLeafGlue)
	RevokeOrphanedGlue(mRevokeWrap)
	if mRevokeWrap.Has(srcLeafDecl) {
		t.Fatal("moving container with only a 1-child wrapper LHS matched should be revoked")
	}
	if mRevokeWrap.Has(srcLeafGlue) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Moving container with matched compound child (effectiveHeight >= 2, e.g. RHS call_expression):
	// Reparented across blocks, compound RHS and glue matched -> preserved.
	srcCompoundTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\toldVar := compute(x)\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcCompound: %v", err)
	}
	dstCompoundTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\tnewVar := compute(x)\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstCompound: %v", err)
	}
	srcCompDecl, _ := testutil.FindNode(srcCompoundTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	dstCompDecl, _ := testutil.FindNode(dstCompoundTree, testutil.NodeQuery{Type: "short_var_declaration", Line: 3})
	srcRHSWrap, _ := testutil.FindNode(srcCompoundTree, testutil.NodeQuery{Type: "expression_list", Line: 3, Nth: 2})
	dstRHSWrap, _ := testutil.FindNode(dstCompoundTree, testutil.NodeQuery{Type: "expression_list", Line: 3, Nth: 2})
	srcRHSCall, _ := testutil.FindNode(srcCompoundTree, testutil.NodeQuery{Type: "call_expression", Line: 3})
	dstRHSCall, _ := testutil.FindNode(dstCompoundTree, testutil.NodeQuery{Type: "call_expression", Line: 3})
	srcCompGlue, _ := testutil.FindNode(srcCompoundTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	dstCompGlue, _ := testutil.FindNode(dstCompoundTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3})
	mPreserve := NewMapping()
	mPreserve.Add(srcCompDecl, dstCompDecl)
	mPreserve.Add(srcRHSWrap, dstRHSWrap)
	mPreserve.Add(srcRHSCall, dstRHSCall)
	mPreserve.Add(srcCompGlue, dstCompGlue)
	RevokeOrphanedGlue(mPreserve)
	if !mPreserve.Has(srcCompDecl) {
		t.Fatal("moving container with matched compound child (height >= 2) should be preserved")
	}
	if !mPreserve.Has(srcCompGlue) {
		t.Fatal("glue inside valid moving container with compound child should be preserved")
	}

	// Keyword glue recognition: if keyword across moving blocks with no non-glue matches.
	srcIfTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tif condA {}\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse srcIf: %v", err)
	}
	dstIfTree, err := treesitter.Parse([]byte("package main\nfunc g() {\n\tif condB {}\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dstIf: %v", err)
	}
	srcIf, _ := testutil.FindNode(srcIfTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	dstIf, _ := testutil.FindNode(dstIfTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	srcKw, _ := testutil.FindNode(srcIfTree, testutil.NodeQuery{Type: "if", Line: 3})
	dstKw, _ := testutil.FindNode(dstIfTree, testutil.NodeQuery{Type: "if", Line: 3})
	mKw := NewMapping()
	mKw.Add(srcIf, dstIf)
	mKw.Add(srcKw, dstKw)
	RevokeOrphanedGlue(mKw)
	if mKw.Has(srcIf) {
		t.Fatal("moving if_statement anchored solely by if keyword should be revoked")
	}
	if mKw.Has(srcKw) {
		t.Fatal("if keyword glue should be revoked")
	}

	// A moved statement with only leaf tokens (like let far_future = 10_000_000)
	// should be preserved when both sides are isomorphic.
	srcIsoTree, err := treesitter.Parse([]byte("fn f() {\n\tlet far_future = 10_000_000;\n}"), "test.rs")
	if err != nil {
		t.Fatalf("parse srcIso: %v", err)
	}
	dstIsoTree, err := treesitter.Parse([]byte("fn g() {\n\tlet far_future = 10_000_000;\n}"), "test.rs")
	if err != nil {
		t.Fatalf("parse dstIso: %v", err)
	}
	srcLet, _ := testutil.FindNode(srcIsoTree, testutil.NodeQuery{Type: "let_declaration", Line: 2})
	dstLet, _ := testutil.FindNode(dstIsoTree, testutil.NodeQuery{Type: "let_declaration", Line: 2})
	mIso := NewMapping()
	mIso.Add(srcLet, dstLet)
	for i := range srcLet.Children {
		if i < len(dstLet.Children) {
			mIso.Add(srcLet.Children[i], dstLet.Children[i])
		}
	}
	RevokeOrphanedGlue(mIso)
	if !mIso.Has(srcLet) {
		t.Fatal("moving isomorphic container with leaf payload should be preserved")
	}
	if !mIso.Has(srcLet.Children[0]) {
		t.Fatal("glue inside moving isomorphic container should be preserved")
	}

	// A moved statement should also stay mapped if all non-glue children match,
	// even when the destination adds a keyword like mut.
	dstMutTree, err := treesitter.Parse([]byte("fn g() {\n\tlet mut far_future = 10_000_000;\n}"), "test.rs")
	if err != nil {
		t.Fatalf("parse dstMut: %v", err)
	}
	dstMutLet, _ := testutil.FindNode(dstMutTree, testutil.NodeQuery{Type: "let_declaration", Line: 2})
	srcLetKw, _ := testutil.FindNode(srcIsoTree, testutil.NodeQuery{Type: "let", Line: 2})
	dstLetKw, _ := testutil.FindNode(dstMutTree, testutil.NodeQuery{Type: "let", Line: 2})
	srcIdent, _ := testutil.FindNode(srcIsoTree, testutil.NodeQuery{Type: "identifier", Line: 2})
	dstIdent, _ := testutil.FindNode(dstMutTree, testutil.NodeQuery{Type: "identifier", Line: 2})
	srcEq, _ := testutil.FindNode(srcIsoTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 2})
	dstEq, _ := testutil.FindNode(dstMutTree, testutil.NodeQuery{Type: "assignment_operator_literal", Line: 2})
	srcInt, _ := testutil.FindNode(srcIsoTree, testutil.NodeQuery{Type: "integer_literal", Line: 2})
	dstInt, _ := testutil.FindNode(dstMutTree, testutil.NodeQuery{Type: "integer_literal", Line: 2})

	mAllNonGlue := NewMapping()
	mAllNonGlue.Add(srcLet, dstMutLet)
	mAllNonGlue.Add(srcLetKw, dstLetKw)
	mAllNonGlue.Add(srcIdent, dstIdent)
	mAllNonGlue.Add(srcEq, dstEq)
	mAllNonGlue.Add(srcInt, dstInt)
	RevokeOrphanedGlue(mAllNonGlue)
	if !mAllNonGlue.Has(srcLet) {
		t.Fatal("moving container with all non-glue children matched should be preserved")
	}
}

func TestRecoverGlueAnchors_BinaryExpression_RHSMatched(t *testing.T) {
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = oldVar == commonVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = newVar == commonVal\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	dstFunc, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	srcBin, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	dstBin, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	srcLHS, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	dstLHS, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	srcOp, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "comparison_operator_literal", Line: 3})
	dstOp, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "comparison_operator_literal", Line: 3})
	srcRHS, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3})
	dstRHS, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3})

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcRHS, dstRHS)

	RecoverGlueAnchors(m)

	if !m.Has(srcBin) || m.Src()[srcBin] != dstBin {
		t.Fatalf("parent binary_expression should be mapped to dstBin, got %v", m.Src()[srcBin])
	}
	if !m.Has(srcOp) || m.Src()[srcOp] != dstOp {
		t.Fatalf("operator == should be mapped to dstOp, got %v", m.Src()[srcOp])
	}
	if !m.Has(srcLHS) || m.Src()[srcLHS] != dstLHS {
		t.Fatalf("LHS oldVar should be recovered and mapped to newVar, got %v", m.Src()[srcLHS])
	}
}

func TestRecoverGlueAnchors_BinaryExpression_LHSMatched(t *testing.T) {
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = commonVal == oldRHS\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\t_ = commonVal == newRHS\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	dstFunc, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	srcBin, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	dstBin, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	srcLHS, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	dstLHS, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	srcOp, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "comparison_operator_literal", Line: 3})
	dstOp, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "comparison_operator_literal", Line: 3})
	srcRHS, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3})
	dstRHS, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3})

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcLHS, dstLHS)

	RecoverGlueAnchors(m)

	if !m.Has(srcBin) || m.Src()[srcBin] != dstBin {
		t.Fatalf("parent binary_expression should be mapped to dstBin, got %v", m.Src()[srcBin])
	}
	if !m.Has(srcOp) || m.Src()[srcOp] != dstOp {
		t.Fatalf("operator == should be mapped to dstOp, got %v", m.Src()[srcOp])
	}
	if !m.Has(srcRHS) || m.Src()[srcRHS] != dstRHS {
		t.Fatalf("RHS oldRHS should be recovered and mapped to newRHS, got %v", m.Src()[srcRHS])
	}
}

func TestRecoverGlueAnchors_BinaryExpression_OppositeSidesNotMatched(t *testing.T) {
	// x = y (y is on RHS) vs y = z (y is on LHS)
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tx = y\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\ty = z\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcAssign, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "assignment_statement", Line: 3})
	dstAssign, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "assignment_statement", Line: 3})
	srcY, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2})
	dstY, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1})

	m := NewMapping()
	m.Add(srcY, dstY)

	RecoverGlueAnchors(m)

	if m.Has(srcAssign) || m.HasDst(dstAssign) {
		t.Fatalf("assignment expressions should not be paired when matched operand sits on opposite sides of operator")
	}
}

func TestRecoverGlueAnchors_ControlFlow_BodyMatched(t *testing.T) {
	srcTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tif oldCond {\n\t\tstmt1()\n\t}\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte("package main\nfunc f() {\n\tif newCond {\n\t\tstmt1()\n\t}\n}"), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcFunc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	dstFunc, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	srcIf, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	dstIf, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	srcKw, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if", Line: 3})
	dstKw, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if", Line: 3})
	srcCond, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "identifier", Line: 3})
	dstCond, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "identifier", Line: 3})
	srcBody, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 3})
	dstBody, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 3})

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(srcBody, dstBody)

	RecoverGlueAnchors(m)

	if !m.Has(srcIf) || m.Src()[srcIf] != dstIf {
		t.Fatalf("parent if_statement should be mapped to dstIf, got %v", m.Src()[srcIf])
	}
	if !m.Has(srcKw) || m.Src()[srcKw] != dstKw {
		t.Fatalf("if keyword should be mapped to dstKw, got %v", m.Src()[srcKw])
	}
	if !m.Has(srcCond) || m.Src()[srcCond] != dstCond {
		t.Fatalf("condition oldCond should be recovered and mapped to newCond, got %v", m.Src()[srcCond])
	}
}

func TestRecoverGlueAnchors_BoilerplateGuard(t *testing.T) {
	src := "package main\nfunc f() {\n\tif err != nil {\n\t\tstmtA()\n\t}\n}"
	dst := "package main\nfunc f() {\n" + strings.Repeat("\t// line padding\n", 70) + "\tif err != nil {\n\t\tstmtB()\n\t}\n}"
	srcTree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}
	srcIf, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	srcCond, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "binary_expression", Line: 3})
	dstCond, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "binary_expression", Line: 73})

	m := NewMapping()
	m.Add(srcCond, dstCond)

	RecoverGlueAnchors(m)

	if m.Has(srcIf) {
		t.Fatalf("boilerplate err != nil condition across distant lines should NOT claim foreign if_statement")
	}
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
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}

	tests := []struct {
		name  string
		query testutil.NodeQuery
		want  bool
	}{
		{"assign", testutil.NodeQuery{Type: "assignment_operator_literal", Line: 3}, true},
		{"short_var", testutil.NodeQuery{Type: "assignment_operator_literal", Line: 4}, true},
		{"equal", testutil.NodeQuery{Type: "comparison_operator_literal", Line: 5}, true},
		{"not_equal", testutil.NodeQuery{Type: "comparison_operator_literal", Line: 6}, true},
		{"plus", testutil.NodeQuery{Type: "arithmetic_operator_literal", Line: 7}, true},
		{"plus_assign", testutil.NodeQuery{Type: "assignment_operator_literal", Line: 8}, true},
		{"bitwise_and", testutil.NodeQuery{Type: "bitwise_operator_literal", Line: 9}, true},
		{"logical_or", testutil.NodeQuery{Type: "logical_operator_literal", Line: 10}, true},
		{"less_than", testutil.NodeQuery{Type: "comparison_operator_literal", Line: 11}, true},
		{"identifier_err", testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 2}, false},
		{"identifier_foo", testutil.NodeQuery{Type: "identifier", Line: 13, Nth: 2}, false},
		{"slice_expression_has_children", testutil.NodeQuery{Type: "slice_expression", Line: 14}, false},
		{"composite_literal_has_children", testutil.NodeQuery{Type: "composite_literal", Line: 15}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			node, err := testutil.FindNode(tree, tc.query)
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
		name      string
		src       string
		ext       string
		queryType string
		line      int
		nth       int
		want      bool
	}{
		{
			name:      "go_err_not_nil",
			src:       "package main\nfunc f() {\n\tif err != nil {}\n}",
			ext:       "test.go",
			queryType: "binary_expression",
			line:      3,
			want:      true,
		},
		{
			name:      "go_single_var_ok",
			src:       "package main\nfunc f() {\n\tif ok {}\n}",
			ext:       "test.go",
			queryType: "identifier",
			line:      3,
			want:      true,
		},
		{
			name:      "go_negated_var",
			src:       "package main\nfunc f() {\n\tif !ok {}\n}",
			ext:       "test.go",
			queryType: "unary_expression",
			line:      3,
			want:      true,
		},
		{
			name:      "go_compare_zero",
			src:       "package main\nfunc f() {\n\tif count == 0 {}\n}",
			ext:       "test.go",
			queryType: "binary_expression",
			line:      3,
			want:      true,
		},
		{
			name:      "java_null_check",
			src:       "class C {\n\tvoid f() {\n\t\tif (ptr == null) {}\n\t}\n}",
			ext:       "test.java",
			queryType: "binary_expression",
			line:      3,
			want:      true,
		},
		{
			name:      "python_none_check",
			src:       "def f():\n    if val is None:\n        pass",
			ext:       "test.py",
			queryType: "comparison_operator",
			line:      2,
			want:      true,
		},
		{
			name:      "rust_none_check",
			src:       "fn f() {\n\tif opt != None {}\n}",
			ext:       "test.rs",
			queryType: "binary_expression",
			line:      2,
			want:      true,
		},
		{
			name:      "c_null_check",
			src:       "void f() {\n\tif (ptr != NULL) {}\n}",
			ext:       "test.c",
			queryType: "binary_expression",
			line:      2,
			want:      true,
		},
		{
			name:      "js_undefined_check",
			src:       "function f() {\n\tif (arg !== undefined) {}\n}",
			ext:       "test.js",
			queryType: "binary_expression",
			line:      2,
			want:      true,
		},
		{
			name:      "two_variables_compare",
			src:       "package main\nfunc f() {\n\tif a == b {}\n}",
			ext:       "test.go",
			queryType: "binary_expression",
			line:      3,
			want:      false,
		},
		{
			name:      "two_variables_ordered",
			src:       "package main\nfunc f() {\n\tif min < max {}\n}",
			ext:       "test.go",
			queryType: "binary_expression",
			line:      3,
			want:      false,
		},
		{
			name:      "compound_call_condition",
			src:       "package main\nfunc f() {\n\tif len(items) > 0 {}\n}",
			ext:       "test.go",
			queryType: "binary_expression",
			line:      3,
			want:      false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := treesitter.Parse([]byte(tc.src), tc.ext)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			cond, err := testutil.FindNode(tree, testutil.NodeQuery{Type: tc.queryType, Line: tc.line, Nth: tc.nth})
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
	srcTree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	outerBlockSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	outerBlockDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcIf, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	dstIf, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if_statement", Line: 38})
	kwOld, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if", Line: 3})
	kwNew, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if", Line: 38})

	m := NewMapping()
	m.Add(outerBlockSrc, outerBlockDst)
	m.Add(srcIf, dstIf)
	m.Add(kwOld, kwNew)

	RevokeOrphanedGlue(m)

	if m.Has(srcIf) {
		t.Fatal("relocated if_statement anchored solely by if keyword across lines must be revoked")
	}
	if m.Has(kwOld) {
		t.Fatal("if keyword of revoked if_statement must be revoked")
	}
}

func TestRevokeOrphanedGlue_AdjacentControlFlowAnchoredByKeywordPreserved(t *testing.T) {
	src := "package main\nfunc f() {\n\tif isErr() {\n\t\treturn\n\t}\n}"
	dst := "package main\nfunc f() {\n\t_ = 1\n\tif !isErr() {\n\t\tlogErr()\n\t}\n}"
	srcTree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcFunc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	dstFunc, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "function_declaration", Line: 2})
	outerBlockSrc, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "block", Line: 2})
	outerBlockDst, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "block", Line: 2})
	srcIf, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if_statement", Line: 3})
	dstIf, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if_statement", Line: 4})
	kwOld, _ := testutil.FindNode(srcTree, testutil.NodeQuery{Type: "if", Line: 3})
	kwNew, _ := testutil.FindNode(dstTree, testutil.NodeQuery{Type: "if", Line: 4})

	m := NewMapping()
	m.Add(srcFunc, dstFunc)
	m.Add(outerBlockSrc, outerBlockDst)
	m.Add(srcIf, dstIf)
	m.Add(kwOld, kwNew)

	RevokeOrphanedGlue(m)

	if !m.Has(srcIf) {
		t.Fatal("adjacent if_statement on nearby line in same block should be preserved")
	}
	if !m.Has(kwOld) {
		t.Fatal("if keyword of adjacent if_statement should be preserved")
	}
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
