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

func TestPipeline_VarDeclToShortVarDeclInPlace(t *testing.T) {
	src := []byte(`package main

func f() {
	var candidates []*moveCandidate
	for i := range actions {
		_ = i
	}
}
`)
	dst := []byte(`package main

func f() {
	candidates := make([]*moveCandidate, 0, len(actions))
	for i := range actions {
		_ = i
	}
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Don't blow away the whole var declaration as a monolithic delete/insert subtree.
	for _, a := range res.Envelope.Actions {
		if a.Action == "delete" && a.Node.Type == "var_declaration" && a.Subtree != nil && *a.Subtree {
			t.Errorf("expected var_declaration to not be deleted as an entire subtree, got: %+v", a)
		}
	}

	// Delete just the "var" keyword; keep the identifier and type slice matched in place.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func f() {
	<del>var</del> candidates []*moveCandidate
	for i := range actions {
		_ = i
	}
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func f() {
	candidates <ins>:= make(</ins>[]*moveCandidate<ins>, 0, len(actions))</ins>
	for i := range actions {
		_ = i
	}
}
`)
}

func TestPipeline_StationaryVarDeclarations(t *testing.T) {
	t.Run("Grouped", func(t *testing.T) {
		src := `package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		errMu     sync.Mutex
		workerErr error
		readerErr error
	)
	setErr := func(err error) {
		errMu.Lock()
		if readerErr == nil {
			readerErr = err
		}
		errMu.Unlock()
	}
	getErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return readerErr
	}
	failReader := func(err error) {
		setErr(err)
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`
		dst := `package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		workerErr error
		readerErr error
	)
	failReader := func(err error) {
		readerErr = err
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`

		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		<del>errMu     sync.Mutex</del>
		workerErr error
		readerErr error
	)
	<del>setErr := func(err error) {</del>
<del>		errMu.Lock()</del>
<del>		if readerErr == nil {</del>
<del>			readerErr = err</del>
<del>		}</del>
<del>		errMu.Unlock()</del>
<del>	}</del>
	<del>getErr := func() error {</del>
<del>		errMu.Lock()</del>
<del>		defer errMu.Unlock()</del>
<del>		return readerErr</del>
<del>	}</del>
	failReader := func(err error) {
		<del>setErr(err)</del>
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		workerErr error
		readerErr error
	)
	failReader := func(err error) {
		<ins>readerErr = err</ins>
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`)
	})

	t.Run("Collection", func(t *testing.T) {
		src := `package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: -1},
	{LeftLine: 4, RightLine: -1},
	{LeftLine: -1, RightLine: 1},
}
`
		dst := `package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: 1},
	{LeftLine: 4, RightLine: -1},
}
`

		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: <del>-</del>1},
	{LeftLine: 4, RightLine: -1},
	<del>{LeftLine: -1, RightLine: 1},</del>
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: 1},
	{LeftLine: 4, RightLine: -1},
}
`)
	})
}

func TestPipeline_ReferenceExpressionWrapperStationary(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn format_commit() {
    let prefix = "commit: ";
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, repo);
    options::set::set_options(&mut opt, git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`
		dst := `fn format_commit() {
    let prefix = "commit: ";
    let shift = 42;
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, &repo);
    options::set::set_options(&mut opt, &mut git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`
		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn format_commit() {
    let prefix = "commit: ";
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, repo);
    options::set::set_options(&mut opt, git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn format_commit() {
    let prefix = "commit: ";
    <ins>let shift = 42;</ins>
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, <ins>&</ins>repo);
    options::set::set_options(&mut opt, <ins>&mut</ins> git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`)
	})
}

func TestPipeline_GenericTypeWrapperUnwrapStationary(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn slide_to_prev_node(nodes: &[&Syntax], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`
		dst := `fn slide_to_prev_node<'a>(nodes: &[&'a Syntax<'a>], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`
		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn slide_to_prev_node(nodes: &[&Syntax], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn slide_to_prev_node<ins><'a></ins>(nodes: &[&<ins>'a</ins> Syntax<ins><'a></ins>], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `function slideToPrevNode(nodes: Syntax[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`
		dst := `function slideToPrevNode<T>(nodes: Promise<Syntax>[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`
		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function slideToPrevNode(nodes: Syntax[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function slideToPrevNode<ins><T></ins>(nodes: <ins>Promise<</ins>Syntax<ins>></ins>[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := `class Parser {
    void slideToPrevNode(List<Syntax> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`
		dst := `class Parser {
    <T> void slideToPrevNode(List<Optional<Syntax>> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`
		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Parser {
    void slideToPrevNode(List<Syntax> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Parser {
    <ins><T></ins> void slideToPrevNode(List<ins><Optional</ins><Syntax><ins>></ins> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`)
	})
}

func TestPipeline_StationaryMultiLineWrappedArgument(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(&matched_lines));
    finish_processing();
}
`
		dst := `pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(&match_preceding_blanks(
        &matched_lines,
        lhs_lines,
        rhs_lines,
    )));
    finish_processing();
}
`
		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(&matched_lines));
    finish_processing();
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(<ins>&match_preceding_blanks(</ins>
        &matched_lines<ins>,</ins>
        <ins>lhs_lines,</ins>
        <ins>rhs_lines,</ins>
    <ins>)</ins>));
    finish_processing();
}
`)
	})

	t.Run("Go", func(t *testing.T) {
		src := `package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(&matched))
	finishProcessing()
}
`
		dst := `package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(matchPrecedingBlanks(
		&matched,
		lhs,
		rhs,
	)))
	finishProcessing()
}
`
		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(&matched))
	finishProcessing()
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(<ins>matchPrecedingBlanks(</ins>
		&matched<ins>,</ins>
		<ins>lhs,</ins>
		<ins>rhs,</ins>
	<ins>)</ins>))
	finishProcessing()
}
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := `class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(data.matched));
        finishProcessing();
    }
}
`
		dst := `class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(matchPrecedingBlanks(
            data.matched,
            lhs,
            rhs
        )));
        finishProcessing();
    }
}
`
		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(data.matched));
        finishProcessing();
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(<ins>matchPrecedingBlanks(</ins>
            data.matched<ins>,</ins>
            <ins>lhs,</ins>
            <ins>rhs</ins>
        <ins>)</ins>));
        finishProcessing();
    }
}
`)
	})
}
