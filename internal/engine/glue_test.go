package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

func TestRecoverGlueAnchors_BinaryExpression_RHSMatched(t *testing.T) {
	lhsOld := testutil.Leaf("identifier", "oldVar")
	opOld := testutil.Leaf("comparison_operator_literal", "==")
	rhs := testutil.Leaf("identifier", "commonVal")
	srcExpr := testutil.Node("binary_expression", "", lhsOld, opOld, rhs)

	lhsNew := testutil.Leaf("identifier", "newVar")
	opNew := testutil.Leaf("comparison_operator_literal", "==")
	rhsDst := testutil.Leaf("identifier", "commonVal")
	dstExpr := testutil.Node("binary_expression", "", lhsNew, opNew, rhsDst)

	m := NewMapping()
	m.Add(rhs, rhsDst)

	RecoverGlueAnchors(m)

	if !m.Has(srcExpr) || m.Src()[srcExpr] != dstExpr {
		t.Fatalf("parent binary_expression should be mapped to dstExpr, got %v", m.Src()[srcExpr])
	}
	if !m.Has(opOld) || m.Src()[opOld] != opNew {
		t.Fatalf("operator == should be mapped to opNew, got %v", m.Src()[opOld])
	}
	if !m.Has(lhsOld) || m.Src()[lhsOld] != lhsNew {
		t.Fatalf("LHS oldVar should be recovered and mapped to newVar, got %v", m.Src()[lhsOld])
	}
}

func TestRecoverGlueAnchors_BinaryExpression_LHSMatched(t *testing.T) {
	lhs := testutil.Leaf("identifier", "commonVal")
	opOld := testutil.Leaf("comparison_operator_literal", "==")
	rhsOld := testutil.Leaf("identifier", "oldRHS")
	srcExpr := testutil.Node("binary_expression", "", lhs, opOld, rhsOld)

	lhsDst := testutil.Leaf("identifier", "commonVal")
	opNew := testutil.Leaf("comparison_operator_literal", "==")
	rhsNew := testutil.Leaf("identifier", "newRHS")
	dstExpr := testutil.Node("binary_expression", "", lhsDst, opNew, rhsNew)

	m := NewMapping()
	m.Add(lhs, lhsDst)

	RecoverGlueAnchors(m)

	if !m.Has(srcExpr) || m.Src()[srcExpr] != dstExpr {
		t.Fatalf("parent binary_expression should be mapped to dstExpr, got %v", m.Src()[srcExpr])
	}
	if !m.Has(opOld) || m.Src()[opOld] != opNew {
		t.Fatalf("operator == should be mapped to opNew, got %v", m.Src()[opOld])
	}
	if !m.Has(rhsOld) || m.Src()[rhsOld] != rhsNew {
		t.Fatalf("RHS oldRHS should be recovered and mapped to newRHS, got %v", m.Src()[rhsOld])
	}
}

func TestRecoverGlueAnchors_BinaryExpression_OppositeSidesNotMatched(t *testing.T) {
	// x = y (y is on RHS) vs y = z (y is on LHS)
	xOld := testutil.Leaf("identifier", "x")
	xOld.StartByte = 0
	opOld := testutil.Leaf("=", "=")
	opOld.StartByte = 2
	yOld := testutil.Leaf("identifier", "y")
	yOld.StartByte = 4
	srcExpr := testutil.Node("assignment_expression", "", xOld, opOld, yOld)

	yNew := testutil.Leaf("identifier", "y")
	yNew.StartByte = 0
	opNew := testutil.Leaf("=", "=")
	opNew.StartByte = 2
	zNew := testutil.Leaf("identifier", "z")
	zNew.StartByte = 4
	dstExpr := testutil.Node("assignment_expression", "", yNew, opNew, zNew)

	m := NewMapping()
	m.Add(yOld, yNew)

	RecoverGlueAnchors(m)

	if m.Has(srcExpr) || m.HasDst(dstExpr) {
		t.Fatalf("assignment expressions should not be paired when matched operand sits on opposite sides of operator")
	}
}

func TestRecoverGlueAnchors_ControlFlow_BodyMatched(t *testing.T) {
	kwOld := testutil.Leaf("if", "if")
	kwOld.IsKeyword = true
	condOld := testutil.Leaf("identifier", "oldCond")
	bodyOld := testutil.Node("block", "", testutil.Leaf("identifier", "stmt1"))
	srcIf := testutil.Node("if_statement", "", kwOld, condOld, bodyOld)

	kwNew := testutil.Leaf("if", "if")
	kwNew.IsKeyword = true
	condNew := testutil.Leaf("identifier", "newCond")
	bodyNew := testutil.Node("block", "", testutil.Leaf("identifier", "stmt1"))
	dstIf := testutil.Node("if_statement", "", kwNew, condNew, bodyNew)

	m := NewMapping()
	m.Add(bodyOld, bodyNew)

	RecoverGlueAnchors(m)

	if !m.Has(srcIf) || m.Src()[srcIf] != dstIf {
		t.Fatalf("parent if_statement should be mapped to dstIf, got %v", m.Src()[srcIf])
	}
	if !m.Has(kwOld) || m.Src()[kwOld] != kwNew {
		t.Fatalf("if keyword should be mapped to kwNew, got %v", m.Src()[kwOld])
	}
	if !m.Has(condOld) || m.Src()[condOld] != condNew {
		t.Fatalf("condition oldCond should be recovered and mapped to newCond, got %v", m.Src()[condOld])
	}
}

func TestRecoverGlueAnchors_BoilerplateGuard(t *testing.T) {
	kwOld := testutil.Leaf("if", "if")
	kwOld.IsKeyword = true
	condOld := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "err"),
		testutil.Leaf("comparison_operator_literal", "!="),
		testutil.Leaf("identifier", "nil"),
	)
	bodyOld := testutil.Node("block", "", testutil.Leaf("identifier", "stmtA"))
	srcIf := testutil.Node("if_statement", "", kwOld, condOld, bodyOld)
	srcIf.StartRow = 10

	kwNew := testutil.Leaf("if", "if")
	kwNew.IsKeyword = true
	condNew := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "err"),
		testutil.Leaf("comparison_operator_literal", "!="),
		testutil.Leaf("identifier", "nil"),
	)
	bodyNew := testutil.Node("block", "", testutil.Leaf("identifier", "stmtB"))
	dstIf := testutil.Node("if_statement", "", kwNew, condNew, bodyNew)
	dstIf.StartRow = 80

	m := NewMapping()
	m.Add(condOld, condNew)

	RecoverGlueAnchors(m)

	if m.Has(srcIf) {
		t.Fatalf("boilerplate err != nil condition across distant lines should NOT claim foreign if_statement")
	}
}

func TestIsOperatorGlue(t *testing.T) {
	r := rules.Get("go")
	tests := []struct {
		name string
		node *treesitter.ASTNode
		want bool
	}{
		{"assign", testutil.Leaf("=", "="), true},
		{"short_var", testutil.Leaf(":=", ":="), true},
		{"equal", testutil.Leaf("==", "=="), true},
		{"not_equal", testutil.Leaf("!=", "!="), true},
		{"plus", testutil.Leaf("+", "+"), true},
		{"plus_assign", testutil.Leaf("+=", "+="), true},
		{"bitwise_and", testutil.Leaf("&", "&"), true},
		{"logical_or", testutil.Leaf("||", "||"), true},
		{"less_than", testutil.Leaf("<", "<"), true},
		{"operator_literal", testutil.Leaf("comparison_operator_literal", "=="), true},
		{"identifier_err", testutil.Leaf("identifier", "err"), false},
		{"identifier_foo", testutil.Leaf("identifier", "foo"), false},
		{"colon_alone", testutil.Leaf(":", ":"), false},
		{"comma_delimiter", testutil.Leaf(",", ","), false},
		{"semicolon_delimiter", testutil.Leaf(";", ";"), false},
		{"paren_left", testutil.Leaf("(", "("), false},
		{"brace_right", testutil.Leaf("}", "}"), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isOperatorGlue(tc.node, r); got != tc.want {
				t.Errorf("isOperatorGlue(%v) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestIsBoilerplateCondition(t *testing.T) {
	rGo := rules.Get("go")
	rPy := rules.Get("python")
	rJava := rules.Get("java")
	rRust := rules.Get("rust")

	tests := []struct {
		name string
		cond *treesitter.ASTNode
		r    *rules.Rules
		want bool
	}{
		{
			name: "go_err_not_nil",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "err"),
				testutil.Leaf("!=", "!="),
				testutil.Leaf("identifier", "nil"),
			),
			r:    rGo,
			want: true,
		},
		{
			name: "go_single_var_ok",
			cond: testutil.Leaf("identifier", "ok"),
			r:    rGo,
			want: true,
		},
		{
			name: "go_negated_var",
			cond: testutil.Node("unary_expression", "",
				testutil.Leaf("!", "!"),
				testutil.Leaf("identifier", "ok"),
			),
			r:    rGo,
			want: true,
		},
		{
			name: "go_compare_zero",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "count"),
				testutil.Leaf("==", "=="),
				testutil.Leaf("int_literal", "0"),
			),
			r:    rGo,
			want: true,
		},
		{
			name: "java_null_check",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "ptr"),
				testutil.Leaf("==", "=="),
				testutil.Leaf("null_literal", "null"),
			),
			r:    rJava,
			want: true,
		},
		{
			name: "python_none_check",
			cond: testutil.Node("comparison", "",
				testutil.Leaf("identifier", "val"),
				testutil.Leaf("is", "is"),
				testutil.Leaf("none", "None"),
			),
			r:    rPy,
			want: true,
		},
		{
			name: "rust_none_check",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "opt"),
				testutil.Leaf("!=", "!="),
				testutil.Leaf("identifier", "None"),
			),
			r:    rRust,
			want: true,
		},
		{
			name: "c_null_check",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "ptr"),
				testutil.Leaf("!=", "!="),
				testutil.Leaf("identifier", "NULL"),
			),
			r:    rules.Get("c"),
			want: true,
		},
		{
			name: "js_undefined_check",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "arg"),
				testutil.Leaf("!==", "!=="),
				testutil.Leaf("identifier", "undefined"),
			),
			r:    rules.Get("javascript"),
			want: true,
		},
		{
			name: "two_variables_compare",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "a"),
				testutil.Leaf("==", "=="),
				testutil.Leaf("identifier", "b"),
			),
			r:    rGo,
			want: false,
		},
		{
			name: "two_variables_ordered",
			cond: testutil.Node("binary_expression", "",
				testutil.Leaf("identifier", "min"),
				testutil.Leaf("<", "<"),
				testutil.Leaf("identifier", "max"),
			),
			r:    rGo,
			want: false,
		},
		{
			name: "compound_call_condition",
			cond: testutil.Node("binary_expression", "",
				testutil.Node("call_expression", "",
					testutil.Leaf("identifier", "len"),
					testutil.Leaf("identifier", "items"),
				),
				testutil.Leaf(">", ">"),
				testutil.Leaf("int_literal", "0"),
			),
			r:    rGo,
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isBoilerplateCondition(tc.cond, tc.r); got != tc.want {
				t.Errorf("isBoilerplateCondition(%s) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestRevokeOrphanedGlue_RelocatedControlFlowAnchoredSolelyByKeyword(t *testing.T) {
	kwOld := testutil.Leaf("if", "if")
	kwOld.IsKeyword = true
	condOld := testutil.Node("call_expression", "", testutil.Leaf("identifier", "isErr"))
	bodyOld := testutil.Node("block", "", testutil.Leaf("identifier", "ret"))
	srcIf := testutil.Node("if_statement", "", kwOld, condOld, bodyOld)
	srcIf.StartRow = 10

	kwNew := testutil.Leaf("if", "if")
	kwNew.IsKeyword = true
	unaryCond := testutil.Node("unary_expression", "", testutil.Leaf("!", "!"), testutil.Node("call_expression", "", testutil.Leaf("identifier", "isErr")))
	bodyNew := testutil.Node("block", "", testutil.Leaf("identifier", "logErr"))
	dstIf := testutil.Node("if_statement", "", kwNew, unaryCond, bodyNew)
	dstIf.StartRow = 46

	outerBlockSrc := testutil.Node("block", "", srcIf)
	outerBlockDst := testutil.Node("block", "", dstIf)

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
	kwOld := testutil.Leaf("if", "if")
	kwOld.IsKeyword = true
	condOld := testutil.Node("call_expression", "", testutil.Leaf("identifier", "isErr"))
	bodyOld := testutil.Node("block", "", testutil.Leaf("identifier", "ret"))
	srcIf := testutil.Node("if_statement", "", kwOld, condOld, bodyOld)
	srcIf.StartRow = 10

	kwNew := testutil.Leaf("if", "if")
	kwNew.IsKeyword = true
	unaryCond := testutil.Node("unary_expression", "", testutil.Leaf("!", "!"), testutil.Node("call_expression", "", testutil.Leaf("identifier", "isErr")))
	bodyNew := testutil.Node("block", "", testutil.Leaf("identifier", "logErr"))
	dstIf := testutil.Node("if_statement", "", kwNew, unaryCond, bodyNew)
	dstIf.StartRow = 11

	outerBlockSrc := testutil.Node("block", "", srcIf)
	outerBlockDst := testutil.Node("block", "", dstIf)

	m := NewMapping()
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
