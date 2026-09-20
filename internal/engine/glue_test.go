package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestRevokeOrphanedGlue(t *testing.T) {
	// Parent with orphaned glue: operator matched, sibling subtree unmatched.
	parent := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "m"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Node("call_expression", "", testutil.Leaf("identifier", "NewMapping")),
	)
	glue, unmatched := parent.Children[1], parent.Children[2]
	dstParent := testutil.Node("other_construct", "",
		testutil.Leaf("identifier", "y"),
		testutil.Leaf("assignment_operator_literal", ":="),
	)
	m := NewMapping()
	m.Add(glue, dstParent.Children[1])
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
	srcScat := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "a"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "b"),
	)
	dstScat := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "c"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "d"),
	)
	m5.Add(srcScat.Children[0], testutil.Leaf("identifier", "elsewhere"))
	m5.Add(srcScat.Children[1], dstScat.Children[1])
	m5.Add(srcScat.Children[2], testutil.Leaf("identifier", "elsewhere2"))
	RevokeOrphanedGlue(m5)
	if m5.Has(srcScat.Children[1]) {
		t.Fatal("reparented glue with scattered siblings should be revoked")
	}
	// Stationary glue (same container): kept even with unmatched sibling.
	dstSame := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "m"),
		testutil.Leaf("assignment_operator_literal", ":="),
	)
	m4 := NewMapping()
	m4.Add(parent, dstSame)
	m4.Add(glue, dstSame.Children[1])
	RevokeOrphanedGlue(m4)
	if !m4.Has(glue) {
		t.Fatal("stationary glue should be kept")
	}

	// Fully matched construct: glue kept.
	m2 := NewMapping()
	parent2 := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "m"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "x"),
	)
	dstFull := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "m"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "x"),
	)
	m2.Add(parent2, dstFull)
	m2.Add(parent2.Children[0], dstFull.Children[0])
	m2.Add(parent2.Children[1], dstFull.Children[1])
	m2.Add(parent2.Children[2], dstFull.Children[2])
	RevokeOrphanedGlue(m2)
	if !m2.Has(parent2.Children[1]) {
		t.Fatal("glue in fully matched construct should be kept")
	}

	// Moving container with zero non-glue children matched:
	// Reparented across blocks, only glue matched -> revoked.
	srcBlock := testutil.Node("block", "")
	dstBlock := testutil.Node("block", "")
	srcMoving := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "srcVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "oldVal"),
	)
	srcMoving.Parent = srcBlock
	dstMoving := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "dstVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "newVal"),
	)
	dstMoving.Parent = dstBlock
	mMoving := NewMapping()
	mMoving.Add(srcMoving, dstMoving)
	mMoving.Add(srcMoving.Children[1], dstMoving.Children[1])
	RevokeOrphanedGlue(mMoving)
	if mMoving.Has(srcMoving) {
		t.Fatal("moving container with zero non-glue matches should be revoked")
	}
	if mMoving.Has(srcMoving.Children[1]) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Moving container with only a bare identifier or single-child wrapper matched:
	// bare tokens (height = 1) do not have structural mass to anchor a moving container across blocks.
	srcMoveLeaf := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "commonVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "oldVal"),
	)
	srcMoveLeaf.Parent = srcBlock
	dstMoveLeaf := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "commonVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "newVal"),
	)
	dstMoveLeaf.Parent = dstBlock
	mRevokeLeaf := NewMapping()
	mRevokeLeaf.Add(srcMoveLeaf, dstMoveLeaf)
	mRevokeLeaf.Add(srcMoveLeaf.Children[0], dstMoveLeaf.Children[0])
	mRevokeLeaf.Add(srcMoveLeaf.Children[1], dstMoveLeaf.Children[1])
	RevokeOrphanedGlue(mRevokeLeaf)
	if mRevokeLeaf.Has(srcMoveLeaf) {
		t.Fatal("moving container anchored solely by glue and bare identifier should be revoked")
	}
	if mRevokeLeaf.Has(srcMoveLeaf.Children[1]) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Single-child wrapper (expression_list -> identifier, effectiveHeight = 1): revoked.
	srcLHSWrap := testutil.Node("expression_list", "", testutil.Leaf("identifier", "m"))
	srcLHSWrap.Language = "go"
	dstLHSWrap := testutil.Node("expression_list", "", testutil.Leaf("identifier", "m"))
	dstLHSWrap.Language = "go"
	srcMoveWrap := testutil.Node("short_var_declaration", "",
		srcLHSWrap,
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "oldVal"),
	)
	srcMoveWrap.Language = "go"
	srcMoveWrap.Parent = srcBlock
	dstMoveWrap := testutil.Node("short_var_declaration", "",
		dstLHSWrap,
		testutil.Leaf("assignment_operator_literal", ":="),
		testutil.Leaf("identifier", "newVal"),
	)
	dstMoveWrap.Language = "go"
	dstMoveWrap.Parent = dstBlock
	mRevokeWrap := NewMapping()
	mRevokeWrap.Add(srcMoveWrap, dstMoveWrap)
	mRevokeWrap.Add(srcLHSWrap, dstLHSWrap)
	mRevokeWrap.Add(srcLHSWrap.Children[0], dstLHSWrap.Children[0])
	mRevokeWrap.Add(srcMoveWrap.Children[1], dstMoveWrap.Children[1])
	RevokeOrphanedGlue(mRevokeWrap)
	if mRevokeWrap.Has(srcMoveWrap) {
		t.Fatal("moving container with only a 1-child wrapper LHS matched should be revoked")
	}
	if mRevokeWrap.Has(srcMoveWrap.Children[1]) {
		t.Fatal("glue inside revoked moving container should be revoked")
	}

	// Moving container with matched compound child (effectiveHeight >= 2, e.g. RHS call_expression):
	// Reparented across blocks, compound RHS and glue matched -> preserved.
	srcRHSCall := testutil.Node("call_expression", "",
		testutil.Leaf("identifier", "compute"),
		testutil.Node("argument_list", "", testutil.Leaf("identifier", "x")),
	)
	dstRHSCall := testutil.Node("call_expression", "",
		testutil.Leaf("identifier", "compute"),
		testutil.Node("argument_list", "", testutil.Leaf("identifier", "x")),
	)
	srcMoveOK := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "oldVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		srcRHSCall,
	)
	srcMoveOK.Parent = srcBlock
	dstMoveOK := testutil.Node("short_var_declaration", "",
		testutil.Leaf("identifier", "newVar"),
		testutil.Leaf("assignment_operator_literal", ":="),
		dstRHSCall,
	)
	dstMoveOK.Parent = dstBlock
	mPreserve := NewMapping()
	mPreserve.Add(srcMoveOK, dstMoveOK)
	mPreserve.Add(srcRHSCall, dstRHSCall)
	mPreserve.Add(srcMoveOK.Children[1], dstMoveOK.Children[1])
	RevokeOrphanedGlue(mPreserve)
	if !mPreserve.Has(srcMoveOK) {
		t.Fatal("moving container with matched compound child (height >= 2) should be preserved")
	}
	if !mPreserve.Has(srcMoveOK.Children[1]) {
		t.Fatal("glue inside valid moving container with compound child should be preserved")
	}

	// Keyword glue recognition: if keyword across moving blocks with no non-glue matches.
	srcIf := testutil.Node("if_statement", "",
		testutil.Leaf("if", "if"),
		testutil.Leaf("identifier", "condA"),
	)
	srcIf.Children[0].IsKeyword = true
	srcIf.Parent = srcBlock
	dstIf := testutil.Node("if_statement", "",
		testutil.Leaf("if", "if"),
		testutil.Leaf("identifier", "condB"),
	)
	dstIf.Children[0].IsKeyword = true
	dstIf.Parent = dstBlock
	mKw := NewMapping()
	mKw.Add(srcIf, dstIf)
	mKw.Add(srcIf.Children[0], dstIf.Children[0])
	RevokeOrphanedGlue(mKw)
	if mKw.Has(srcIf) {
		t.Fatal("moving if_statement anchored solely by if keyword should be revoked")
	}
	if mKw.Has(srcIf.Children[0]) {
		t.Fatal("if keyword glue should be revoked")
	}
}
