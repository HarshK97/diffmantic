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

func TestPipeline_Postprocess_CrossBranchMove(t *testing.T) {
	t.Run("Go", func(t *testing.T) {
		src := []byte(`package main

func handle(ready bool) {
	prepare()
	if ready {
		processTask()
	}
	finish()
}
`)
		dst := []byte(`package main

func handle(ready bool) {
	prepare()
	if ready {
		validate()
	} else {
		processTask()
	}
	finish()
}
`)
		res, err := Run(src, dst, "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func handle(ready bool) {
	prepare()
	if ready <del>{</del>
		<mov>processTask()</mov>
	<del>}</del>
	finish()
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func handle(ready bool) {
	prepare()
	if ready <ins>{</ins>
<ins>		validate()</ins>
<ins>	} else {</ins>
		<mov>processTask()</mov>
	<ins>}</ins>
	finish()
}
`)
	})

	t.Run("Rust", func(t *testing.T) {
		src := []byte(`fn handle(ready: bool) {
    prepare();
    if ready {
        process_task();
    }
    finish();
}
`)
		dst := []byte(`fn handle(ready: bool) {
    prepare();
    if ready {
        validate();
    } else {
        process_task();
    }
    finish();
}
`)
		res, err := Run(src, dst, "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `fn handle(ready: bool) {
    prepare();
    if ready <del>{</del>
        <mov>process_task();</mov>
    <del>}</del>
    finish();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `fn handle(ready: bool) {
    prepare();
    if ready <ins>{</ins>
<ins>        validate();</ins>
<ins>    } else {</ins>
        <mov>process_task();</mov>
    <ins>}</ins>
    finish();
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := []byte(`function handle(ready: boolean) {
    prepare();
    if (ready) {
        processTask();
    }
    finish();
}
`)
		dst := []byte(`function handle(ready: boolean) {
    prepare();
    if (ready) {
        validate();
    } else {
        processTask();
    }
    finish();
}
`)
		res, err := Run(src, dst, "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `function handle(ready: boolean) {
    prepare();
    if (ready) <del>{</del>
        <mov>processTask();</mov>
    <del>}</del>
    finish();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `function handle(ready: boolean) {
    prepare();
    if (ready) <ins>{</ins>
<ins>        validate();</ins>
<ins>    } else {</ins>
        <mov>processTask();</mov>
    <ins>}</ins>
    finish();
}
`)
	})

	t.Run("Python", func(t *testing.T) {
		src := []byte(`def handle(ready: bool):
    prepare()
    if ready:
        process_task()
    finish()
`)
		dst := []byte(`def handle(ready: bool):
    prepare()
    if ready:
        validate()
    else:
        process_task()
    finish()
`)
		res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `def handle(ready: bool):
    prepare()
    if ready:
        <mov>process_task()</mov>
    finish()
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `def handle(ready: bool):
    prepare()
    if ready:
        <ins>validate()</ins>
    <ins>else:</ins>
        <mov>process_task()</mov>
    finish()
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := []byte(`class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) {
            processTask();
        }
        finish();
    }
}
`)
		dst := []byte(`class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) {
            validate();
        } else {
            processTask();
        }
        finish();
    }
}
`)
		res, err := Run(src, dst, "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) <del>{</del>
            <mov>processTask();</mov>
        <del>}</del>
        finish();
    }
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) <ins>{</ins>
<ins>            validate();</ins>
<ins>        } else {</ins>
            <mov>processTask();</mov>
        <ins>}</ins>
        finish();
    }
}
`)
	})
}

func TestPipeline_Postprocess_StationaryIfElseIfLadderMoves(t *testing.T) {
	t.Run("C", func(t *testing.T) {
		src := []byte(`void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
		dst := []byte(`void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } else if (x == 2) {
        do_two();
    } else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
		res, err := Run(src, dst, "test.c", "test.c", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } <ins>else if (x == 2) {</ins>
<ins>        do_two();</ins>
<ins>    }</ins> else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
	})

	t.Run("Python", func(t *testing.T) {
		src := []byte(`def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 3:
        do_three()
    do_done()
`)
		dst := []byte(`def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 2:
        do_two()
    elif x == 3:
        do_three()
    do_done()
`)
		res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 3:
        do_three()
    do_done()
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    <ins>elif x == 2:</ins>
<ins>        do_two()</ins>
    elif x == 3:
        do_three()
    do_done()
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := []byte(`function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
		dst := []byte(`function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } else if (x === 2) {
        doTwo();
    } else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
		res, err := Run(src, dst, "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } <ins>else if (x === 2) {</ins>
<ins>        doTwo();</ins>
<ins>    }</ins> else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
	})

	t.Run("Go", func(t *testing.T) {
		src := []byte(`package main

func evaluateOption() {
	doInit()
	if x == 1 {
		doOne()
	} else if x == 3 {
		doThree()
	}
	doDone()
}
`)
		dst := []byte(`package main

func evaluateOption() {
	doInit()
	if x == 1 {
		doOne()
	} else if x == 2 {
		doTwo()
	} else if x == 3 {
		doThree()
	}
	doDone()
}
`)
		res, err := Run(src, dst, "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func evaluateOption() {
	doInit()
	if x == 1 {
		doOne()
	} else if x == 3 {
		doThree()
	}
	doDone()
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func evaluateOption() {
	doInit()
	if x == 1 {
		doOne()
	} else <ins>if x == 2 {</ins>
<ins>		doTwo()</ins>
<ins>	} else</ins> if x == 3 {
		doThree()
	}
	doDone()
}
`)
	})

	t.Run("Rust", func(t *testing.T) {
		src := []byte(`fn evaluate_option() {
    do_init();
    if x == 1 {
        do_one();
    } else if x == 3 {
        do_three();
    }
    do_done();
}
`)
		dst := []byte(`fn evaluate_option() {
    do_init();
    if x == 1 {
        do_one();
    } else if x == 2 {
        do_two();
    } else if x == 3 {
        do_three();
    }
    do_done();
}
`)
		res, err := Run(src, dst, "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `fn evaluate_option() {
    do_init();
    if x == 1 {
        do_one();
    } else if x == 3 {
        do_three();
    }
    do_done();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `fn evaluate_option() {
    do_init();
    if x == 1 {
        do_one();
    } <ins>else if x == 2 {</ins>
<ins>        do_two();</ins>
<ins>    }</ins> else if x == 3 {
        do_three();
    }
    do_done();
}
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := []byte(`class Evaluator {
    void evaluateOption() {
        doInit();
        if (x == 1) {
            doOne();
        } else if (x == 3) {
            doThree();
        }
        doDone();
    }
}
`)
		dst := []byte(`class Evaluator {
    void evaluateOption() {
        doInit();
        if (x == 1) {
            doOne();
        } else if (x == 2) {
            doTwo();
        } else if (x == 3) {
            doThree();
        }
        doDone();
    }
}
`)
		res, err := Run(src, dst, "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `class Evaluator {
    void evaluateOption() {
        doInit();
        if (x == 1) {
            doOne();
        } else if (x == 3) {
            doThree();
        }
        doDone();
    }
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `class Evaluator {
    void evaluateOption() {
        doInit();
        if (x == 1) {
            doOne();
        } else <ins>if (x == 2) {</ins>
<ins>            doTwo();</ins>
<ins>        } else</ins> if (x == 3) {
            doThree();
        }
        doDone();
    }
}
`)
	})

	t.Run("Zig", func(t *testing.T) {
		src := []byte(`fn evaluateOption() void {
    doInit();
    if (x == 1) {
        doOne();
    } else if (x == 3) {
        doThree();
    }
    doDone();
}
`)
		dst := []byte(`fn evaluateOption() void {
    doInit();
    if (x == 1) {
        doOne();
    } else if (x == 2) {
        doTwo();
    } else if (x == 3) {
        doThree();
    }
    doDone();
}
`)
		res, err := Run(src, dst, "test.zig", "test.zig", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `fn evaluateOption() void {
    doInit();
    if (x == 1) {
        doOne();
    } else if (x == 3) {
        doThree();
    }
    doDone();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `fn evaluateOption() void {
    doInit();
    if (x == 1) {
        doOne();
    } <ins>else if (x == 2) {</ins>
<ins>        doTwo();</ins>
<ins>    }</ins> else if (x == 3) {
        doThree();
    }
    doDone();
}
`)
	})
}

func TestPipeline_Postprocess_StationaryFluentMethodChains(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := []byte(`fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                .use_value_delimiter(false)
                .takes_value(true)
                .overrides_with("style")
        );
    config.validate();
}
`)
		dst := []byte(`fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                .overrides_with("style")
        );
    config.validate();
}
`)
		res, err := Run(src, dst, "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                <del>.use_value_delimiter(false)</del>
                <del>.takes_value(true)</del>
                .overrides_with("style")
        );
    config.validate();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                .overrides_with("style")
        );
    config.validate();
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := []byte(`function buildQuery() {
    const config = initConfig();
    const query = new QueryBuilder("users")
        .select("id", "name")
        .where("active", true)
        .limit(10)
        .offset(20)
        .orderBy("created_at");
    config.validate();
}
`)
		dst := []byte(`function buildQuery() {
    const config = initConfig();
    const query = new QueryBuilder("users")
        .select("id", "name")
        .where("active", true)
        .orderBy("created_at");
    config.validate();
}
`)
		res, err := Run(src, dst, "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `function buildQuery() {
    const config = initConfig();
    const query = new QueryBuilder("users")
        .select("id", "name")
        .where("active", true)
        <del>.limit(10)</del>
        <del>.offset(20)</del>
        .orderBy("created_at");
    config.validate();
}
`)
		testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `function buildQuery() {
    const config = initConfig();
    const query = new QueryBuilder("users")
        .select("id", "name")
        .where("active", true)
        .orderBy("created_at");
    config.validate();
}
`)
	})
}

func TestPipeline_Postprocess_RustTailExpressionSemicolonTransitionWithDrift(t *testing.T) {
	src := []byte(`fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: &mut u32) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id)
}
`)
	dst := []byte(`fn added_upstream_helper() {
    let _ = 1;
}

fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: &mut NonZeroU32) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id);
}
`)
	res, err := Run(src, dst, "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: <del>&mut u32</del>) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id)
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `<ins>fn added_upstream_helper() {</ins>
<ins>    let _ = 1;</ins>
<ins>}</ins>

fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: <ins>&mut NonZeroU32</ins>) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id)<ins>;</ins>
}
`)
}

func TestPipeline_Postprocess_SameLineCrossStatementMove_ThresholdBypass(t *testing.T) {
	src := `package main

func foo() {
	name1 := getDeclarationName(s1)
	if name1 == "" {
		name1 = getClosureDeclarationName(s1, r)
	}
	name2 := getDeclarationName(s2)
	if name2 == "" {
		name2 = getClosureDeclarationName(s2, r)
	}
}
`

	dst := `package main

func foo() {
	r = cmp.Or(r, rulesFor(t1))
	scope1 := getScopeName(s1.Parent)
	scope2 := getScopeName(s2.Parent)
	sameScope := (scope1 == "" && scope2 == "") || (scope1 != "" && scope2 != "" && scope1 == scope2)

	name1 := cmp.Or(getDeclarationName(s1), getClosureDeclarationName(s1, r))
	name2 := cmp.Or(getDeclarationName(s2), getClosureDeclarationName(s2, r))
}
`

	res, err := Run([]byte(src), []byte(dst), "a.go", "b.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `package main

func foo() {
	name1 := getDeclarationName(s1)
	<del>if name1 == "" {</del>
<del>		name1 = getClosureDeclarationName(s1, r)</del>
<del>	}</del>
	name2 := getDeclarationName(s2)
	<del>if name2 == "" {</del>
<del>		name2 = getClosureDeclarationName(s2, r)</del>
<del>	}</del>
}
`

	expectedRight := `package main

func foo() {
	<ins>r = cmp.Or(r, rulesFor(t1))</ins>
	<ins>scope1 := getScopeName(s1.Parent)</ins>
	<ins>scope2 := getScopeName(s2.Parent)</ins>
	<ins>sameScope := (scope1 == "" && scope2 == "") || (scope1 != "" && scope2 != "" && scope1 == scope2)</ins>

	name1 := <ins>cmp.Or(</ins>getDeclarationName(s1)<ins>, getClosureDeclarationName(s1, r))</ins>
	name2 := <ins>cmp.Or(</ins>getDeclarationName(s2)<ins>, getClosureDeclarationName(s2, r))</ins>
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}

func TestPipeline_Postprocess_SameLineCrossStatementMove_ThresholdBypass_Rust(t *testing.T) {
	src := `fn foo() {
    let mut name1 = get_declaration_name(s1);
    if name1.is_empty() {
        name1 = get_closure_declaration_name(s1, r);
    }
    let mut name2 = get_declaration_name(s2);
    if name2.is_empty() {
        name2 = get_closure_declaration_name(s2, r);
    }
}
`
	dst := `fn foo() {
    r = resolve_rules(t1);
    let scope1 = get_scope_name(&s1);
    let scope2 = get_scope_name(&s2);
    let same_scope = scope1 == scope2;

    let name1 = get_declaration_name(s1).or_else(|| get_closure_declaration_name(s1, r));
    let name2 = get_declaration_name(s2).or_else(|| get_closure_declaration_name(s2, r));
}
`
	res, err := Run([]byte(src), []byte(dst), "a.rs", "b.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `fn foo() {
    let <del>mut</del> name1 = get_declaration_name(s1);
    <del>if name1.is_empty() {</del>
<del>        name1 = get_closure_declaration_name(s1, r);</del>
<del>    }</del>
    let <del>mut</del> name2 = get_declaration_name(s2);
    <del>if name2.is_empty() {</del>
<del>        name2 = get_closure_declaration_name(s2, r);</del>
<del>    }</del>
}
`

	expectedRight := `fn foo() {
    <ins>r = resolve_rules(t1);</ins>
    <ins>let scope1 = get_scope_name(&s1);</ins>
    <ins>let scope2 = get_scope_name(&s2);</ins>
    <ins>let same_scope = scope1 == scope2;</ins>

    let name1 = get_declaration_name(s1)<ins>.or_else(|| get_closure_declaration_name(s1, r))</ins>;
    let name2 = get_declaration_name(s2)<ins>.or_else(|| get_closure_declaration_name(s2, r))</ins>;
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}

func TestPipeline_Postprocess_BinaryExprChainExpansion_Visual(t *testing.T) {
	src := `package main

func scorePartner(src, dst *ASTNode) int {
	isDecl := (r != nil && (r.IsDeclaration(src.Type) || r.IsDeclaration(dst.Type))) ||
		(r == nil && (rules.IsDeclaration(src.Type) || rules.IsDeclaration(dst.Type)))
	return 0
}
`

	dst := `package main

func scorePartner(src, dst *ASTNode) int {
	isDecl := (r != nil && (r.IsDeclaration(src.Type) || r.IsLocalVarDeclaration(src.Type) || r.IsDeclaration(dst.Type) || r.IsLocalVarDeclaration(dst.Type))) ||
		(r == nil && (rules.IsDeclaration(src.Type) || rules.IsLocalVarDeclaration(src.Type) || rules.IsDeclaration(dst.Type) || rules.IsLocalVarDeclaration(dst.Type)))
	return 0
}
`

	res, err := Run([]byte(src), []byte(dst), "a.go", "b.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `package main

func scorePartner(src, dst *ASTNode) int {
	isDecl := (r != nil && (r.IsDeclaration(src.Type) || r.IsDeclaration(dst.Type))) ||
		(r == nil && (rules.IsDeclaration(src.Type) || rules.IsDeclaration(dst.Type)))
	return 0
}
`

	expectedRight := `package main

func scorePartner(src, dst *ASTNode) int {
	isDecl := (r != nil && (r.IsDeclaration(src.Type) <ins>|| r.IsLocalVarDeclaration(src.Type)</ins> || r.IsDeclaration(dst.Type) <ins>|| r.IsLocalVarDeclaration(dst.Type)</ins>)) ||
		(r == nil && (rules.IsDeclaration(src.Type) <ins>|| rules.IsLocalVarDeclaration(src.Type)</ins> || rules.IsDeclaration(dst.Type) <ins>|| rules.IsLocalVarDeclaration(dst.Type)</ins>))
	return 0
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}

func TestPipeline_Postprocess_BooleanExprNesting_Visual(t *testing.T) {
	t.Run("RightSide", func(t *testing.T) {
		src := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	for enc != nil && r != nil && r.IsImport(enc.Type) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		dst := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	// Skip imports so symbols inside them resolve their scope to the enclosing module or file.
	for enc != nil && ((r != nil && r.IsImport(enc.Type)) || (r == nil && rules.IsImport(enc.Type))) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		res, err := Run([]byte(src), []byte(dst), "a.go", "b.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		expectedLeft := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	for enc != nil <upd>&&</upd> r != nil && r.IsImport(enc.Type) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		expectedRight := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	<ins>// Skip imports so symbols inside them resolve their scope to the enclosing module or file.</ins>
	for enc != nil && <ins>((</ins>r != nil <ins>&&</ins> r.IsImport(enc.Type)<ins>)</ins> <upd>||</upd> <ins>(r == nil && rules.IsImport(enc.Type)))</ins> {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
	})

	t.Run("LeftSide", func(t *testing.T) {
		src := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	for enc != nil && r != nil && r.IsImport(enc.Type) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		dst := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	// Skip imports so symbols inside them resolve their scope to the enclosing module or file.
	for enc != nil && ((r == nil && rules.IsImport(enc.Type)) || (r != nil && r.IsImport(enc.Type))) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		res, err := Run([]byte(src), []byte(dst), "a.go", "b.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		expectedLeft := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	for enc != nil <upd>&&</upd> r != nil && r.IsImport(enc.Type) {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		expectedRight := `package engine

func getScopeName(n *ASTNode) string {
	enc := GetEnclosingDeclaration(n)
	<ins>// Skip imports so symbols inside them resolve their scope to the enclosing module or file.</ins>
	for enc != nil && <ins>((r == nil && rules.IsImport(enc.Type))</ins> <upd>||</upd> <ins>(</ins>r != nil <ins>&&</ins> r.IsImport(enc.Type)<ins>))</ins> {
		enc = GetEnclosingDeclaration(enc)
	}
	return ""
}
`

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
	})
}

func TestPipeline_Postprocess_BinaryExprChainExpansion_Zig_Visual(t *testing.T) {
	src := `fn scorePartner(src: *Node, dst: *Node) bool {
    const is_decl = (r != null and (isDeclaration(src) or isDeclaration(dst))) or
        (r == null and (rules.isDeclaration(src) or rules.isDeclaration(dst)));
    return is_decl;
}
`

	dst := `fn scorePartner(src: *Node, dst: *Node) bool {
    const is_decl = (r != null and (isDeclaration(src) or isLocalVar(src) or isDeclaration(dst) or isLocalVar(dst))) or
        (r == null and (rules.isDeclaration(src) or rules.isLocalVar(src) or rules.isDeclaration(dst) or rules.isLocalVar(dst)));
    return is_decl;
}
`

	res, err := Run([]byte(src), []byte(dst), "a.zig", "b.zig", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `fn scorePartner(src: *Node, dst: *Node) bool {
    const is_decl = (r != null and (isDeclaration(src) or isDeclaration(dst))) or
        (r == null and (rules.isDeclaration(src) or rules.isDeclaration(dst)));
    return is_decl;
}
`

	expectedRight := `fn scorePartner(src: *Node, dst: *Node) bool {
    const is_decl = (r != null and (isDeclaration(src) <ins>or isLocalVar(src)</ins> or isDeclaration(dst) <ins>or isLocalVar(dst)</ins>)) or
        (r == null and (rules.isDeclaration(src) <ins>or rules.isLocalVar(src)</ins> or rules.isDeclaration(dst) <ins>or rules.isLocalVar(dst)</ins>));
    return is_decl;
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}
