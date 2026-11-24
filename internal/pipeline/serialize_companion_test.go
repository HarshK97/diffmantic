package pipeline

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

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

	testPairs := make([]testutil.LineAlignmentPair, len(res.Envelope.LineAlignment))
	for i, p := range res.Envelope.LineAlignment {
		testPairs[i] = testutil.LineAlignmentPair{
			LeftLine:  p.LeftLine,
			RightLine: p.RightLine,
		}
	}
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

func TestPipeline_GoImplicitLengthArrayWrapperTrimming(t *testing.T) {
	src := []byte("package main\n\nvar x = []string{}\n")
	dst := []byte("package main\n\nvar x = [...]string{}\n")

	res, err := Run(src, dst, "a.go", "b.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Changing []string to [...]string should only diff the brackets and dots,
	// leaving the inner type "string" untouched as unchanged context.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

var x = <del>[]</del>string{}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

var x = <ins>[...]</ins>string{}
`)
}

func TestPipeline_MultilineCallDeletionCarveout_C(t *testing.T) {
	oldSrc := `void test() {
    foo(
        "arg1",
        "arg2"
    );
}
`
	newSrc := `void test() {
    int res = foo(
        "arg1",
        "arg2"
    );
    return;
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

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `void test() {
    foo(
        "arg1",
        "arg2"
    )<del>;</del>
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `void test() {
    <ins>int res = foo(</ins>
<ins>        "arg1",</ins>
<ins>        "arg2"</ins>
<ins>    );</ins>
    <ins>return;</ins>
}
`)
}

func TestPipeline_MultilineCallDeletionCarveout_Java(t *testing.T) {
	oldSrc := `class Test {
    void run() {
        foo(
            "arg1",
            "arg2"
        );
    }
}
`
	newSrc := `class Test {
    void run() {
        int res = foo(
            "arg1",
            "arg2"
        );
        return;
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
        foo(
            "arg1",
            "arg2"
        )<del>;</del>
    }
}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `class Test {
    void run() {
        <ins>int res = foo(</ins>
<ins>            "arg1",</ins>
<ins>            "arg2"</ins>
<ins>        );</ins>
        <ins>return;</ins>
    }
}
`)
}

func TestPipeline_ContainerDelimiterMoveParity(t *testing.T) {
	oldSrc := `package main

func Process(condition bool) error {
	if condition {
		step1 := validate()
		step2 := transform(step1)
		step3 := persist(step2)
		notify(step3)
		logAudit(step3)
		return nil
	}
	return nil
}
`
	newSrc := `package main

func Process(condition bool) error {
	if condition {
		return handleCondition()
	}
	return nil
}

func handleCondition() error {
	step1 := validate()
	step2 := transform(step1)
	step3 := persist(step2)
	notify(step3)
	logAudit(step3)
	return nil
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

func Process(condition bool) error {
	<del>if condition</del> <mov>{</mov>
<mov>		step1 := validate()</mov>
<mov>		step2 := transform(step1)</mov>
<mov>		step3 := persist(step2)</mov>
<mov>		notify(step3)</mov>
<mov>		logAudit(step3)</mov>
<mov>		return nil</mov>
<mov>	}</mov>
	return nil
}
`)
	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `package main

func Process(condition bool) error {
	<ins>if condition {</ins>
<ins>		return handleCondition()</ins>
<ins>	}</ins>
	return nil
}

<ins>func handleCondition() error</ins> <mov>{</mov>
<mov>	step1 := validate()</mov>
<mov>	step2 := transform(step1)</mov>
<mov>	step3 := persist(step2)</mov>
<mov>	notify(step3)</mov>
<mov>	logAudit(step3)</mov>
<mov>	return nil</mov>
<mov>}</mov>
`)
}

func TestPipeline_AssignMoveColors_ExtractedBlockColorContrast(t *testing.T) {
	oldSrc := `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    match pagerflags.split_first() {
        Some((_, _)) => {
            let mut process = Command::new("less");
            process.arg("-R");
            process.arg("-F");
            process.arg("-X");
            process.arg("-K");
            process.arg("-i");
            process.env("LESSCHARSET", "UTF-8");
            process.env("LESSHISTFILE", "/dev/null");
            Ok(process)
        }
        None => Ok(Command::new("cat")),
    };
    process.wait()
}
`
	newSrc := `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    Ok(match pagerflags.split_first() {
        Some((_, _)) => {
            _make_process_from_less_path()
        }
        None => Command::new("cat"),
    });
    process.wait()
}

fn _make_process_from_less_path() -> Command {
    let mut process = Command::new("less");
    process.arg("-R");
    process.arg("-F");
    process.arg("-X");
    process.arg("-K");
    process.arg("-i");
    process.env("LESSCHARSET", "UTF-8");
    process.env("LESSHISTFILE", "/dev/null");
    process
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    <mov>match pagerflags.split_first() {</mov>
        <mov>Some((_, _)) =></mov> <mov1>{</mov1>
<mov1>            let mut process = Command::new("less");</mov1>
<mov1>            process.arg("-R");</mov1>
<mov1>            process.arg("-F");</mov1>
<mov1>            process.arg("-X");</mov1>
<mov1>            process.arg("-K");</mov1>
<mov1>            process.arg("-i");</mov1>
<mov1>            process.env("LESSCHARSET", "UTF-8");</mov1>
<mov1>            process.env("LESSHISTFILE", "/dev/null");</mov1>
<mov1>            </mov1><del>Ok(process)</del>
<mov1>        }</mov1>
        <mov>None => </mov><del>Ok(</del><mov>Command::new("cat")</mov><del>)</del><mov>,</mov>
    <mov>}</mov><del>;</del>
    process.wait()
}
`)
	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    <ins>Ok(</ins><mov>match pagerflags.split_first() {</mov>
        <mov>Some((_, _)) =></mov> <ins>{</ins>
<ins>            _make_process_from_less_path()</ins>
<ins>        }</ins>
        <mov>None => Command::new("cat"),</mov>
    <mov>}</mov><ins>)</ins>;
    process.wait()
}

<ins>fn _make_process_from_less_path() -> Command</ins> <mov1>{</mov1>
<mov1>    let mut process = Command::new("less");</mov1>
<mov1>    process.arg("-R");</mov1>
<mov1>    process.arg("-F");</mov1>
<mov1>    process.arg("-X");</mov1>
<mov1>    process.arg("-K");</mov1>
<mov1>    process.arg("-i");</mov1>
<mov1>    process.env("LESSCHARSET", "UTF-8");</mov1>
<mov1>    process.env("LESSHISTFILE", "/dev/null");</mov1>
    <ins>process</ins>
<mov1>}</mov1>
`)
}

func buildPipelineBadges(res *DiffResult, contextLines int, lang string) renderutil.MoveBadges {
	srcLines := strings.Split(string(res.SrcBytes), "\n")
	dstLines := strings.Split(string(res.DstBytes), "\n")
	srcOffsets := serialize.BuildLineIndex(res.SrcBytes)
	dstOffsets := serialize.BuildLineIndex(res.DstBytes)

	leftSpansByLine := make(map[int][]serialize.HighlightSpan)
	for _, s := range res.Envelope.LeftHighlights {
		leftSpansByLine[s.Line] = append(leftSpansByLine[s.Line], s)
	}
	rightSpansByLine := make(map[int][]serialize.HighlightSpan)
	for _, s := range res.Envelope.RightHighlights {
		rightSpansByLine[s.Line] = append(rightSpansByLine[s.Line], s)
	}

	isPairChanged := make([]bool, len(res.Envelope.LineAlignment))
	for i, pair := range res.Envelope.LineAlignment {
		var srcText, dstText string
		if pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
			srcText = srcLines[pair.LeftLine]
		}
		if pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
			dstText = dstLines[pair.RightLine]
		}
		var leftSpans, rightSpans []serialize.HighlightSpan
		if pair.LeftLine >= 0 {
			leftSpans = leftSpansByLine[pair.LeftLine]
		}
		if pair.RightLine >= 0 {
			rightSpans = rightSpansByLine[pair.RightLine]
		}
		if renderutil.IsPairChanged(pair, leftSpans, rightSpans, srcText, dstText, false, false, false) {
			isPairChanged[i] = true
		}
	}
	changeIntervals := renderutil.BuildChangeIntervals(isPairChanged)
	hunks := renderutil.MergeHunks(changeIntervals, len(res.Envelope.LineAlignment), contextLines)

	r := rules.Get(lang)
	return renderutil.BuildMoveBadges(res.Envelope.Actions, hunks, res.Envelope.LineAlignment, srcOffsets, dstOffsets, srcLines, dstLines, r)
}

func TestPipeline_MoveBadges_DeclarationMove_Go(t *testing.T) {
	// Companion for TestBuildMoveBadges_DeclarationMove:
	// A function declaration moves past stationary functions.
	// Separated by distant unchanged functions so source and destination hunks
	// are distinct (> 15 alignment lines apart), emitting directional line badges
	// with color index 0, and the moved function is highlighted as <mov>.
	oldSrc := `package main

func MovedFunc() {
	println("moved")
}

func Pad01() {}
func Pad02() {}
func Pad03() {}
func Pad04() {}
func Pad05() {}
func Pad06() {}
func Pad07() {}
func Pad08() {}
func Pad09() {}
func Pad10() {}
func Pad11() {}
func Pad12() {}
func Pad13() {}
func Pad14() {}
func Pad15() {}
func Pad16() {}
func Pad17() {}
func Pad18() {}
func Pad19() {}
func Pad20() {}
`
	newSrc := `package main

func Pad01() {}
func Pad02() {}
func Pad03() {}
func Pad04() {}
func Pad05() {}
func Pad06() {}
func Pad07() {}
func Pad08() {}
func Pad09() {}
func Pad10() {}
func Pad11() {}
func Pad12() {}
func Pad13() {}
func Pad14() {}
func Pad15() {}
func Pad16() {}
func Pad17() {}
func Pad18() {}
func Pad19() {}
func Pad20() {}

func MovedFunc() {
	println("moved")
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

<mov>func MovedFunc() {</mov>
<mov>	println("moved")</mov>
<mov>}</mov>

func Pad01() {}
func Pad02() {}
func Pad03() {}
func Pad04() {}
func Pad05() {}
func Pad06() {}
func Pad07() {}
func Pad08() {}
func Pad09() {}
func Pad10() {}
func Pad11() {}
func Pad12() {}
func Pad13() {}
func Pad14() {}
func Pad15() {}
func Pad16() {}
func Pad17() {}
func Pad18() {}
func Pad19() {}
func Pad20() {}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `package main

func Pad01() {}
func Pad02() {}
func Pad03() {}
func Pad04() {}
func Pad05() {}
func Pad06() {}
func Pad07() {}
func Pad08() {}
func Pad09() {}
func Pad10() {}
func Pad11() {}
func Pad12() {}
func Pad13() {}
func Pad14() {}
func Pad15() {}
func Pad16() {}
func Pad17() {}
func Pad18() {}
func Pad19() {}
func Pad20() {}

<mov>func MovedFunc() {</mov>
<mov>	println("moved")</mov>
<mov>}</mov>
`)

	badges := buildPipelineBadges(res, 3, "go")

	badgeSrc, hasSrc := badges.SrcLineBadges[2]
	if !hasSrc || !strings.Contains(badgeSrc, "➔ L24") {
		t.Errorf("expected SrcLineBadges[2] to contain '➔ L24', got %q", badgeSrc)
	}
	if color := badges.SrcLineBadgeColors[2]; color != 0 {
		t.Errorf("expected SrcLineBadgeColors[2] to be 0 (Cyan), got %d", color)
	}

	badgeDst, hasDst := badges.DstLineBadges[23]
	if !hasDst || !strings.Contains(badgeDst, "↳ L3") {
		t.Errorf("expected DstLineBadges[23] to contain '↳ L3', got %q", badgeDst)
	}
	if color := badges.DstLineBadgeColors[23]; color != 0 {
		t.Errorf("expected DstLineBadgeColors[23] to be 0 (Cyan), got %d", color)
	}
}

func TestPipeline_MoveBadges_SingleLineStatementMove_Go(t *testing.T) {
	// Companion for TestBuildMoveBadges_SingleLineStatementMove:
	// A single-line assignment is relocated past stationary statements and distant lines.
	oldSrc := `package main

func Foo() {
	if true {
		absorbed := false
	}
}
`
	newSrc := `package main

func Foo() {
	absorbed := false
	println("1")
	println("2")
	println("3")
	println("4")
	println("5")
	println("6")
	println("7")
	println("8")
	println("9")
	println("10")
	println("11")
	println("12")
	println("13")
	println("14")
	println("15")
	println("16")
	if true {
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

func Foo() {
	if true {
		<mov>absorbed := false</mov>
	}
}
`)

	badges := buildPipelineBadges(res, 3, "go")

	badgeSrc, hasSrc := badges.SrcLineBadges[4]
	if !hasSrc || !strings.Contains(badgeSrc, "➔ L4") {
		t.Errorf("expected SrcLineBadges[4] to contain '➔ L4', got %q", badgeSrc)
	}
	badgeDst, hasDst := badges.DstLineBadges[3]
	if !hasDst || !strings.Contains(badgeDst, "↳ L5") {
		t.Errorf("expected DstLineBadges[3] to contain '↳ L5', got %q", badgeDst)
	}
}

func TestPipeline_MoveBadges_HierarchicalSuppression_Rust(t *testing.T) {
	// Companion for TestBuildMoveBadges_HierarchicalSuppression:
	// When an enclosing declaration (impl block) moves past distant stationary functions,
	// only the outer impl block gets badged; inner methods must have their badges suppressed.
	oldSrc := `struct Service;

impl Service {
    fn handle_request(&self) {
        println!("handling");
    }
}

fn pad01() {}
fn pad02() {}
fn pad03() {}
fn pad04() {}
fn pad05() {}
fn pad06() {}
fn pad07() {}
fn pad08() {}
fn pad09() {}
fn pad10() {}
fn pad11() {}
fn pad12() {}
fn pad13() {}
fn pad14() {}
fn pad15() {}
fn pad16() {}
fn pad17() {}
fn pad18() {}
fn pad19() {}
fn pad20() {}
`
	newSrc := `struct Service;

fn pad01() {}
fn pad02() {}
fn pad03() {}
fn pad04() {}
fn pad05() {}
fn pad06() {}
fn pad07() {}
fn pad08() {}
fn pad09() {}
fn pad10() {}
fn pad11() {}
fn pad12() {}
fn pad13() {}
fn pad14() {}
fn pad15() {}
fn pad16() {}
fn pad17() {}
fn pad18() {}
fn pad19() {}
fn pad20() {}

impl Service {
    fn handle_request(&self) {
        println!("handling");
    }
}
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `struct Service;

<mov>impl Service {</mov>
<mov>    fn handle_request(&self) {</mov>
<mov>        println!("handling");</mov>
<mov>    }</mov>
<mov>}</mov>

fn pad01() {}
fn pad02() {}
fn pad03() {}
fn pad04() {}
fn pad05() {}
fn pad06() {}
fn pad07() {}
fn pad08() {}
fn pad09() {}
fn pad10() {}
fn pad11() {}
fn pad12() {}
fn pad13() {}
fn pad14() {}
fn pad15() {}
fn pad16() {}
fn pad17() {}
fn pad18() {}
fn pad19() {}
fn pad20() {}
`)

	testutil.AssertTagged(t, newSrc, toTestSpans(res.Envelope.RightHighlights), `struct Service;

fn pad01() {}
fn pad02() {}
fn pad03() {}
fn pad04() {}
fn pad05() {}
fn pad06() {}
fn pad07() {}
fn pad08() {}
fn pad09() {}
fn pad10() {}
fn pad11() {}
fn pad12() {}
fn pad13() {}
fn pad14() {}
fn pad15() {}
fn pad16() {}
fn pad17() {}
fn pad18() {}
fn pad19() {}
fn pad20() {}

<mov>impl Service {</mov>
<mov>    fn handle_request(&self) {</mov>
<mov>        println!("handling");</mov>
<mov>    }</mov>
<mov>}</mov>
`)

	badges := buildPipelineBadges(res, 3, "rust")

	// Outer impl block at source line 2 gets badged pointing to destination line 24 (1-based)
	badgeSrc, hasSrc := badges.SrcLineBadges[2]
	if !hasSrc || !strings.Contains(badgeSrc, "➔ L24") {
		t.Errorf("expected outer impl at SrcLineBadges[2] to contain '➔ L24', got %q", badgeSrc)
	}

	// Inner method at source line 3 MUST NOT get a redundant badge
	if badgeChild, hasChild := badges.SrcLineBadges[3]; hasChild {
		t.Errorf("expected inner method at SrcLineBadges[3] to be suppressed, got %q", badgeChild)
	}
}

func TestPipeline_MoveBadges_AlignmentDistanceThreshold_Go(t *testing.T) {
	// Companion for TestBuildMoveBadges_AlignmentDistanceThreshold:
	// A small move within <= 15 lines of alignment distance in the same hunk is
	// visually clear in side-by-side alignment, so arrow badges are suppressed.
	oldSrc := `package main

func process() {
	if cond {
		doWork()
	}
	step1()
	step2()
}
`
	newSrc := `package main

func process() {
	step1()
	step2()
	if cond {
		doWork()
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
	<mov>if cond {</mov>
<mov>		doWork()</mov>
<mov>	}</mov>
	step1()
	step2()
}
`)

	badges := buildPipelineBadges(res, 3, "go")

	// The move is within 5 lines in the same hunk, so badges are suppressed
	if badge, ok := badges.SrcLineBadges[3]; ok {
		t.Errorf("expected SrcLineBadges[3] to be suppressed due to alignment distance threshold, got %q", badge)
	}
	if badge, ok := badges.DstLineBadges[5]; ok {
		t.Errorf("expected DstLineBadges[5] to be suppressed due to alignment distance threshold, got %q", badge)
	}
}

func TestPipeline_MoveBadges_ColorSynchronization_TwoMoves_Python(t *testing.T) {
	// Companion for TestBuildMoveBadges_ColorSynchronization:
	// Two diverging moves across distant scopes receive distinct color indices (0: Cyan, 1: Magenta).
	// The directional badge colors must match the move span colors.
	oldSrc := `def process():
    move_a()
    move_b()
    keep01()
    keep02()
    keep03()
    keep04()
    keep05()
    keep06()
    keep07()
    keep08()
    keep09()
    keep10()
    keep11()
    keep12()
    keep13()
    keep14()
    keep15()
    keep16()
    keep17()
    keep18()
`
	newSrc := `def process():
    keep01()
    keep02()
    keep03()
    keep04()
    keep05()
    keep06()
    keep07()
    keep08()
    if cond1:
        move_a()
    keep09()
    keep10()
    keep11()
    keep12()
    keep13()
    keep14()
    keep15()
    keep16()
    keep17()
    keep18()
    if cond2:
        move_b()
`
	res, err := Run([]byte(oldSrc), []byte(newSrc), "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, oldSrc, toTestSpans(res.Envelope.LeftHighlights), `def process():
    <mov>move_a()</mov>
    <mov1>move_b()</mov1>
    keep01()
    keep02()
    keep03()
    keep04()
    keep05()
    keep06()
    keep07()
    keep08()
    keep09()
    keep10()
    keep11()
    keep12()
    keep13()
    keep14()
    keep15()
    keep16()
    keep17()
    keep18()
`)

	badges := buildPipelineBadges(res, 3, "python")

	// Check move 1 has color index 0 and badge color 0
	if color, ok := badges.SrcLineBadgeColors[1]; !ok || color != 0 {
		t.Errorf("expected SrcLineBadgeColors[1] to be color 0 (Cyan), got %d (has: %v)", color, ok)
	}

	// Check move 2 has color index 1 and badge color 1
	if color, ok := badges.SrcLineBadgeColors[2]; !ok || color != 1 {
		t.Errorf("expected SrcLineBadgeColors[2] to be color 1 (Magenta), got %d (has: %v)", color, ok)
	}
}
