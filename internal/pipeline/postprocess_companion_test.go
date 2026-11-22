package pipeline

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
)

// TestPipeline_Postprocess_ClosureExtractionInlining verifies visual highlights when
// an expression is extracted into an inner arrow function closure within the same function scope.
func TestPipeline_Postprocess_ClosureExtractionInlining(t *testing.T) {
	src := []byte(`function processItems(items: number[]) {
    precedingInit();
    const currTotal = items.reduce((a, b) => a + b, 0);
    succeedingDone(currTotal);
}
`)
	dst := []byte(`function processItems(items: number[]) {
    precedingInit();

    step01();
    step02();
    step03();
    step04();
    step05();
    step06();
    step07();
    step08();
    step09();
    step10();
    step11();

    const getMetrics = () => items.reduce((a, b) => a + b, 0);
    succeedingDone(getMetrics());
}
`)

	res, err := Run(src, dst, "pipeline.ts", "pipeline.ts", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `function processItems(items: number[]) {
    precedingInit();
    const <upd>currTotal</upd> = items.reduce((a, b) => a + b, 0);
    succeedingDone(<del>currTotal</del>);
}
`)

	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `function processItems(items: number[]) {
    precedingInit();

    <ins>step01();</ins>
    <ins>step02();</ins>
    <ins>step03();</ins>
    <ins>step04();</ins>
    <ins>step05();</ins>
    <ins>step06();</ins>
    <ins>step07();</ins>
    <ins>step08();</ins>
    <ins>step09();</ins>
    <ins>step10();</ins>
    <ins>step11();</ins>

    const <upd>getMetrics</upd> = <ins>() =></ins> items.reduce((a, b) => a + b, 0);
    succeedingDone(<ins>getMetrics()</ins>);
}
`)
}

// TestPipeline_Postprocess_TSXSelfClosingTagConversion verifies visual highlights when
// an empty pair of JSX/TSX opening/closing tags is converted into a self-closing element.
func TestPipeline_Postprocess_TSXSelfClosingTagConversion(t *testing.T) {
	src := []byte(`import React from 'react';

export function Header() {
	return <Button title="Submit"></Button>;
}
`)
	dst := []byte(`import React from 'react';

export function Header() {
	return <Button title="Submit" />;
}
`)

	res, err := Run(src, dst, "test.tsx", "test.tsx", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}

	// The tag name and attribute should remain unchanged context, while opening/closing delimiters adapt.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `import React from 'react';

export function Header() {
	return <Button title="Submit"<del>></Button></del>;
}
`)

	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `import React from 'react';

export function Header() {
	return <Button title="Submit" <ins>/></ins>;
}
`)
}

// TestPipeline_Postprocess_LISMoveGrouping verifies that monotonic cohorts moving
// together receive cohesive move group tagging while reordered elements receive distinct move styling.
func TestPipeline_Postprocess_LISMoveGrouping(t *testing.T) {
	src := []byte(`package main

func target() {
	println("prologue")
}

func source() {
	stmtA()
	stmtB()
	stmtC()
	stmtD()
}
`)
	dst := []byte(`package main

func target() {
	println("prologue")
	stmtD()
	stmtA()
	stmtB()
	stmtC()
}

func source() {
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}

	// All four statements should be recognized as moves, with stmtA, stmtB, stmtC sharing
	// a cohesive monotonic cohort move badge/color, and stmtD receiving a separate move color.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func target() {
	println("prologue")
}

func source() {
	<mov>stmtA()</mov>
	<mov>stmtB()</mov>
	<mov>stmtC()</mov>
	<mov1>stmtD()</mov1>
}
`)

	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func target() {
	println("prologue")
	<mov1>stmtD()</mov1>
	<mov>stmtA()</mov>
	<mov>stmtB()</mov>
	<mov>stmtC()</mov>
}

func source() {
}
`)
}

// TestPipeline_Postprocess_MultilineExpressionUnwrapping verifies that unwrapping a
// multiline boolean or helper expression preserves stationary inner operands without spurious moves.
func TestPipeline_Postprocess_MultilineExpressionUnwrapping(t *testing.T) {
	src := []byte(`package main

func check(n *ASTNode, r *Rules) bool {
	if (r != nil && (r.IsOperatorLiteral(c) || r.IsPunctuation(c))) ||
		(r == nil && (rules.IsOperatorLiteral(c) || rules.IsPunctuation(c))) {
		return true
	}
	return false
}
`)
	dst := []byte(`package main

func check(n *ASTNode, r *Rules) bool {
	if r.IsOperatorLiteral(c) || r.IsPunctuation(c) {
		return true
	}
	return false
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected pipeline error: %v", err)
	}

	// The surviving operand r.IsOperatorLiteral(c) || r.IsPunctuation(c) should stay context,
	// while unwrapped outer conditional expressions are deleted.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func check(n *ASTNode, r *Rules) bool {
	if <del>(r != nil && (</del>r.IsOperatorLiteral(c) || r.IsPunctuation(c)<del>)) ||</del>
		<del>(r == nil && (rules.IsOperatorLiteral(c) || rules.IsPunctuation(c)))</del> {
		return true
	}
	return false
}
`)

	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func check(n *ASTNode, r *Rules) bool {
	if r.IsOperatorLiteral(c) || r.IsPunctuation(c) {
		return true
	}
	return false
}
`)
}
