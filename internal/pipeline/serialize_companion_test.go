package pipeline

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
)

func toPipelineTestPairs(grid []serialize.LineAlignmentPair) []testutil.LineAlignmentPair {
	res := make([]testutil.LineAlignmentPair, len(grid))
	for i, p := range grid {
		res[i] = testutil.LineAlignmentPair{
			LeftLine:  p.LeftLine,
			RightLine: p.RightLine,
		}
	}
	return res
}

func TestPipeline_AdaptiveMoveColor(t *testing.T) {
	// Companion for TestBuildHighlightSpans_AdaptiveMoveColor_FullEngine:
	// An isolated move takes color slot 0 (Cyan, <mov>). An adjacent move group
	// containing both a multiline if-statement and a single-line call keeps color slot 0
	// so the group shares one color.
	oldSrc := `package main

func process() {
	isolatedMove()
	stationaryA()
	stationaryB()
	if check() {
		doWork()
	}
	finish()
}
`
	newSrc := `package main

func process() {
	stationaryA()
	stationaryB()
	isolatedMove()
	if wrapper() {
		if check() {
			doWork()
		}
		finish()
	}
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `package main

func process() {
	<mov>isolatedMove()</mov>
	stationaryA()
	stationaryB()
	<mov>if check() {</mov>
<mov>		doWork()</mov>
<mov>	}</mov>
	<mov>finish()</mov>
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `package main

func process() {
	stationaryA()
	stationaryB()
	<mov>isolatedMove()</mov>
	<ins>if wrapper() {</ins>
		<mov>if check() {</mov>
<mov>			doWork()</mov>
<mov>		}</mov>
		<mov>finish()</mov>
	<ins>}</ins>
}
`)
}

func TestPipeline_MultilineStatementInsertCarveout_RustMacro(t *testing.T) {
	// Companion for TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine (Rust macro):
	// Multiline macro invocation preserves stationary string literal without delete/insert bleeding.
	oldSrc := `fn set_terminal_title_to(new_terminal_title: String) {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(io::stdout(), "{osc_command}{safe_title}{osc_end}").unwrap();
    io::stdout().flush().unwrap();
}
`
	newSrc := `fn set_terminal_title_to(new_terminal_title: String) -> Result<()> {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(
        io::stdout(),
        "{osc_command}{safe_title}{osc_end}"
    )?;
    io::stdout().flush()?;
    Ok(())
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "old.rs", "new.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `fn set_terminal_title_to(new_terminal_title: String) {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(io::stdout(), "{osc_command}{safe_title}{osc_end}")<del>.unwrap()</del>;
    <del>io::stdout().flush().unwrap();</del>
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `fn set_terminal_title_to(new_terminal_title: String) <ins>-> Result<()></ins> {
    let osc_command = "\x1b]0;";
    let osc_end = "\x07";
    let safe_title = bat::sanitize(&new_terminal_title);
    write!(
        io::stdout(),
        "{osc_command}{safe_title}{osc_end}"
    )<ins>?;</ins>
    <ins>io::stdout().flush()?;</ins>
    <ins>Ok(())</ins>
}
`)
}

func TestPipeline_MultilineStatementInsertCarveout_GoCall(t *testing.T) {
	// Companion for TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine (Go call):
	// Multiline call deletion does not bleed delete over stationary arguments.
	oldSrc := `package main

func bar() {
	foo(
		"arg1",
		"arg2",
	)
}
`
	newSrc := `package main

func bar() {
	result := foo(
		"arg1",
		"arg2",
	)
	_ = result
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "old.go", "new.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `package main

func bar() {
	foo(
		"arg1",
		"arg2",
	)
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `package main

func bar() {
	<ins>result :=</ins> foo(
		"arg1",
		"arg2",
	)
	<ins>_ = result</ins>
}
`)
}

func TestPipeline_MultilineStatementInsertCarveout_RustMatch(t *testing.T) {
	// Companion for TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine (Rust match):
	// Let declaration wrapping match expression does not bleed insert over stationary match arms.
	oldSrc := `fn main() {
    match shell.as_str() {
        "bash" => println!("{}", completions::BASH_COMPLETION),
        _ => unreachable!("No completion for shell '{shell}' available."),
    }
}
`
	newSrc := `fn main() {
    let completion = match shell.as_str() {
        "bash" => completions::BASH_COMPLETION,
        _ => unreachable!("No completion for shell '{shell}' available."),
    };
    writeln!(io::stdout(), "{completion}")?;
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "old.rs", "new.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `fn main() {
    match shell.as_str() {
        "bash" => <del>println!("{}", completions::BASH_COMPLETION),</del>
        _ => unreachable!("No completion for shell '{shell}' available."),
    }
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `fn main() {
    <ins>let completion =</ins> match shell.as_str() {
        "bash" => <ins>completions::BASH_COMPLETION,</ins>
        _ => unreachable!("No completion for shell '{shell}' available."),
    }<ins>;</ins>
    <ins>writeln!(io::stdout(), "{completion}")?;</ins>
}
`)
}

func TestPipeline_MultilineStatementInsertCarveout_CInitializer(t *testing.T) {
	// Companion for TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine (C initializer):
	// Initializer list carving out stationary child item without engulfing it in insert highlights.
	oldSrc := `void foo() {
    int item = 10;
    int a = item;
}
`
	newSrc := `void foo() {
    int item = 10;
    int a[] = { item };
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "old.c", "new.c", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `void foo() {
    int item = 10;
    int a = item;
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `void foo() {
    int item = 10;
    int a[] = <ins>{</ins> item <ins>}</ins>;
}
`)
}

func TestPipeline_MultilineStatementInsertCarveout_JavaArrayInitializer(t *testing.T) {
	// Companion for TestBuildHighlightSpans_MultilineStatementInsertCarveout_FullEngine (Java initializer):
	// Array initializer carving out stationary child item without engulfing it in insert highlights.
	oldSrc := `class Test {
    void run() {
        int item = 10;
        int a = item;
    }
}
`
	newSrc := `class Test {
    void run() {
        int item = 10;
        int[] a = { item };
    }
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "Test.java", "Test.java", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `class Test {
    void run() {
        int item = 10;
        int a = item;
    }
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `class Test {
    void run() {
        int item = 10;
        int<ins>[]</ins> a = <ins>{</ins> item <ins>}</ins>;
    }
}
`)
}

func TestPipeline_Tier2StationaryStatements_Alignment(t *testing.T) {
	// Companion for TestAlignLines_Tier2StationaryStatements_FullEngine:
	// var buf bytes.Buffer must be aligned as Tier 1 unchanged anchor and not displaced by moved if-block.
	oldSrc := `package streamdiff

import (
	"bytes"
	"context"
)

func processStreamJob(ctx context.Context, job *diffJob, cfg StreamConfig) {
	if err := ctx.Err(); err != nil {
		job.err = err
		return
	}

	var buf bytes.Buffer
	if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != "json" {
		job.out = buf.Bytes()
		return
	}

	dr, err := pipeline.Run(job.srcBytes, job.dstBytes, job.hunk.oldPath, job.hunk.newPath, cfg.DiffOpts)
	if err != nil || dr == nil {
		_ = dumpLines(&buf, job.hunk.lines)
		job.out = buf.Bytes()
		return
	}
}
`
	newSrc := `package streamdiff

import (
	"bytes"
	"context"
)

func processStreamJob(ctx context.Context, job *diffJob, cfg StreamConfig) {
	if err := ctx.Err(); err != nil {
		job.err = err
		return
	}

	if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != "json" {
		return
	}

	var buf bytes.Buffer

	dr, err := pipeline.Run(job.srcBytes, job.dstBytes, job.hunk.oldPath, job.hunk.newPath, cfg.DiffOpts)
	if err != nil || dr == nil {
		_ = dumpLines(&buf, job.hunk.lines)
		job.out = buf.Bytes()
		return
	}
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "stream_old.go", "stream_new.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testPairs := toPipelineTestPairs(res.Envelope.LineAlignment)
	testutil.AssertLinesAligned(t, oldSrc, newSrc, testPairs,
		testutil.Line("var buf bytes.Buffer"),
		testutil.Line("var buf bytes.Buffer"),
	)
	testutil.AssertLinesNotAligned(t, oldSrc, newSrc, testPairs,
		testutil.Line("var buf bytes.Buffer"),
		testutil.Line("if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != \"json\" {"),
	)
}

func TestPipeline_AssignMoveColors_DivergingMoves(t *testing.T) {
	// Companion for TestAssignMoveColors_FullEngine:
	// Diverging moves from nearby source get distinct colors (<mov> vs <mov1>).
	oldSrc := `package main

func process() {
	moveA()
	moveB()
	keep1()
	keep2()
	keep3()
	keep4()
}
`
	newSrc := `package main

func process() {
	keep1()
	if cond1 {
		moveA()
	}
	keep2()
	keep3()
	keep4()
	if cond2 {
		moveB()
	}
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `package main

func process() {
	<mov>moveA()</mov>
	<mov1>moveB()</mov1>
	keep1()
	keep2()
	keep3()
	keep4()
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `package main

func process() {
	keep1()
	<ins>if cond1 {</ins>
		<mov>moveA()</mov>
	<ins>}</ins>
	keep2()
	keep3()
	keep4()
	<ins>if cond2 {</ins>
		<mov1>moveB()</mov1>
	<ins>}</ins>
}
`)
}

func TestPipeline_AdjustRangeForContainer_ZigElseClause(t *testing.T) {
	// Companion for TestAdjustRangeForContainer_FullEngine:
	// Verify else_clause clamps to the prologue keyword rather than spanning the entire block.
	oldSrc := `fn f() void {
    a();
    b();
    c();
}
`
	newSrc := `fn f() void {
    if (cond) {
        x();
    } else {
        a();
        b();
        c();
    }
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.zig", "test.zig", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `fn f() void {
    <mov>a();</mov>
    <mov>b();</mov>
    <mov>c();</mov>
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `fn f() void {
    <ins>if (cond) {</ins>
<ins>        x();</ins>
<ins>    } else {</ins>
        <mov>a();</mov>
        <mov>b();</mov>
        <mov>c();</mov>
    <ins>}</ins>
}
`)
}
