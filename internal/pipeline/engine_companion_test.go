package pipeline

import (
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestPipeline_Recovery_IfInitConditionFastPath(t *testing.T) {
	src := `package main

func evaluate(val int) int {
	startMetrics()
	if val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`
	dst := `package main

func evaluate(val int) int {
	startMetrics()
	if err := validate(val); val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`
	res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func evaluate(val int) int {
	startMetrics()
	if val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func evaluate(val int) int {
	startMetrics()
	if <ins>err := validate(val);</ins> val > 0 {
		return val * 2
	}
	endMetrics()
	return 0
}
`)
}

func TestPipeline_Recovery_CrossBranchStatementMove(t *testing.T) {
	src := `package main

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
	dst := `package main

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
	res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func handleBranch(flag bool, val int) {
	header()
	if flag {
		<mov1>fallback()</mov1>
	} else {
		<mov>compute(val)</mov>
		cleanup()
	}
	footer()
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func handleBranch(flag bool, val int) {
	header()
	if flag {
		<mov>compute(val)</mov>
	} else {
		<mov1>fallback()</mov1>
		cleanup()
	}
	footer()
}
`)
}

func TestPipeline_SandwichedStatementShiftedIntervalRecovery(t *testing.T) {
	src := `fn test_logging(val: &str) {
    setup_env();
    print!("value: {}", val);
    cleanup_env();
}
`
	dst := `fn test_logging(val: &str) {
    init_tracing();
    setup_env();
    write!(io::stdout(), "value: {}", val);
    cleanup_env();
}
`
	res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn test_logging(val: &str) {
    setup_env();
    <upd>print</upd>!("value: {}", val);
    cleanup_env();
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn test_logging(val: &str) {
    <ins>init_tracing();</ins>
    setup_env();
    <upd>write</upd>!(<ins>io::stdout(),</ins> "value: {}", val);
    cleanup_env();
}
`)
}

func TestPipeline_RustSandwichedStatementTryWrapperRecovery(t *testing.T) {
	src := `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    print!("Writing metadata to folder {} ... ", target_dir.to_string_lossy());
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    println!("okay");
    Ok(())
}
`
	dst := `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    write!(io::stdout(), "Writing metadata to folder {} ... ", target_dir.to_string_lossy())?;
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    writeln!(io::stdout(), "okay")?;
    Ok(())
}
`
	res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    <upd>print</upd>!("Writing metadata to folder {} ... ", target_dir.to_string_lossy());
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    <upd>println</upd>!("okay");
    Ok(())
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn write_assets(target_dir: &Path, current_version: &str) -> Result<()> {
    let _ = std::fs::create_dir_all(target_dir);
    <upd>write</upd>!(<ins>io::stdout(),</ins> "Writing metadata to folder {} ... ", target_dir.to_string_lossy())<ins>?</ins>;
    crate::assets_metadata::AssetsMetadata::new(current_version).save_to_folder(target_dir)?;
    <upd>writeln</upd>!(<ins>io::stdout(),</ins> "okay")<ins>?</ins>;
    Ok(())
}
`)
}

func TestPipeline_RustMultiStatementIntervalRecovery(t *testing.T) {
	src := `fn generate_config_file() -> Result<()> {
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
	dst := `fn generate_config_file() -> Result<()> {
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
	res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn generate_config_file() -> Result<()> {
    let config_file = config_file();
    if config_file.is_file() {
        <upd>println</upd>!("A config file already exists at: {}", config_file.to_string_lossy());
        <upd>print</upd>!("Overwrite? (y/N): ");
        io::stdout().flush()?;
        let mut decision = String::new();
        io::stdin().read_line(&mut decision)?;
    }
    Ok(())
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn generate_config_file() -> Result<()> {
    let config_file = config_file();
    if config_file.is_file() {
        <upd>writeln</upd>!(<ins>io::stdout(),</ins> "A config file already exists at: {}", config_file.to_string_lossy())<ins>?</ins>;
        <upd>write</upd>!(<ins>io::stdout(),</ins> "Overwrite? (y/N): ")<ins>?</ins>;
        io::stdout().flush()?;
        let mut decision = String::new();
        io::stdin().read_line(&mut decision)?;
    }
    Ok(())
}
`)
}

func TestPipeline_ZigSandwichedStatementTryWrapperRecovery(t *testing.T) {
	src := `pub fn run(writer: anytype) !void {
    init();
    writer.print("foo\n", .{});
    anchor();
    writer.println("bar\n", .{});
    finish();
}
`
	dst := `pub fn run(writer: anytype) !void {
    init();
    try writer.write("foo\n", .{});
    anchor();
    try writer.writeln("bar\n", .{});
    finish();
}
`
	res, err := Run([]byte(src), []byte(dst), "test.zig", "test.zig", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `pub fn run(writer: anytype) !void {
    init();
    writer.<upd>print</upd>("foo\n", .{});
    anchor();
    writer.<upd>println</upd>("bar\n", .{});
    finish();
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `pub fn run(writer: anytype) !void {
    init();
    <ins>try</ins> writer.<upd>write</upd>("foo\n", .{});
    anchor();
    <ins>try</ins> writer.<upd>writeln</upd>("bar\n", .{});
    finish();
}
`)
}

func TestPipeline_CParenthesizedStatementRecovery(t *testing.T) {
	src := `void handle_transaction(Database* db) {
    db_begin(db);
    db_flush_wal(db);
    db_commit(db);
}
`
	dst := `void handle_transaction(Database* db) {
    db_begin(db);
    (db_flush_wal(db));
    db_commit(db);
}
`
	res, err := Run([]byte(src), []byte(dst), "test.c", "test.c", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `void handle_transaction(Database* db) {
    db_begin(db);
    db_flush_wal(db);
    db_commit(db);
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `void handle_transaction(Database* db) {
    db_begin(db);
    <ins>(</ins>db_flush_wal(db)<ins>)</ins>;
    db_commit(db);
}
`)
}

func TestPipeline_JavaScriptSandwichedStatementAwaitWrapperRecovery(t *testing.T) {
	src := `async function syncSession(client) {
    client.connect();
    client.fetchUpdates();
    client.disconnect();
}
`
	dst := `async function syncSession(client) {
    client.connect();
    await client.fetchUpdates();
    client.disconnect();
}
`
	res, err := Run([]byte(src), []byte(dst), "test.js", "test.js", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `async function syncSession(client) {
    client.connect();
    client.fetchUpdates();
    client.disconnect();
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `async function syncSession(client) {
    client.connect();
    <ins>await</ins> client.fetchUpdates();
    client.disconnect();
}
`)
}

func TestPipeline_PythonSandwichedStatementAwaitWrapperRecovery(t *testing.T) {
	src := `async def sync_session(client):
    client.connect()
    client.fetch_updates()
    client.disconnect()
`
	dst := `async def sync_session(client):
    client.connect()
    await client.fetch_updates()
    client.disconnect()
`
	res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `async def sync_session(client):
    client.connect()
    client.fetch_updates()
    client.disconnect()
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `async def sync_session(client):
    client.connect()
    <ins>await</ins> client.fetch_updates()
    client.disconnect()
`)
}

func TestPipeline_MatchUnmatchedLeaves_FieldMutualExclusion(t *testing.T) {
	src := `def configure_session():
    setup_logger()
    options = {
        "token": 100,
        "active": True,
    }
    teardown_logger(options)
`
	dst := `def configure_session():
    setup_logger()
    options = {
        "active": True,
        "auth_key": "token",
    }
    teardown_logger(options)
`
	res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `def configure_session():
    setup_logger()
    options = {
        <upd>"token"</upd>: <del>100</del>,
        "active": True,
    }
    teardown_logger(options)
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `def configure_session():
    setup_logger()
    options = {
        "active": True,
        <upd>"auth_key"</upd>: <ins>"token"</ins>,
    }
    teardown_logger(options)
`)
}

func TestPipeline_RevokeOrphanedGlue_BareJumpPreservedInMovingBlock(t *testing.T) {
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
	res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func processOld(items []int) {
	<mov>for _, item := range items {</mov>
<mov>		if item < 0 {</mov>
<mov>			break</mov>
<mov>		}</mov>
<mov>		handle(item)</mov>
<mov>	}</mov>
}

<del>func otherCaller() {</del>
<del>	println("hello")</del>
<del>}</del>
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func processOld(items []int) {
	<ins>println("hello")</ins>
}

<ins>func helperExtracted(items []int) {</ins>
	<mov>for _, item := range items {</mov>
<mov>		if item < 0 {</mov>
<mov>			break</mov>
<mov>		}</mov>
<mov>		handle(item)</mov>
<mov>	}</mov>
<ins>}</ins>
`)
}

func TestPipeline_RevokeOrphanedGlue_MovingScalarSubExpressionPreserved(t *testing.T) {
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
	res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func Process(x any) {
	if <mov>x != nil && validate(x)</mov> {
		println("valid")
	}
}

func Helper(x any) bool {
	return <del>false</del>
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func Process(x any) {
	if <ins>Helper(x)</ins> {
		println("valid")
	}
}

func Helper(x any) bool {
	return <mov>x != nil && validate(x)</mov>
}
`)
}

func TestPipeline_BinaryOperatorGlue_SingleOperandMutation(t *testing.T) {
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
	res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func renderTest(left, right, lastSrcLineIdx, lastDstLineIdx int, sText, dText string, srcEndsWithNL, dstEndsWithNL bool) {
	isEOFLine := (left == lastSrcLineIdx && right == lastDstLineIdx)
	<mov>if </mov><del>sText != dText || (</del><mov1>isEOFLine && srcEndsWithNL != dstEndsWithNL</mov1><del>)</del><mov> {</mov>
<mov>		upd++</mov>
<mov>	}</mov>
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func renderTest(left, right, lastSrcLineIdx, lastDstLineIdx int, sText, dText string, srcEndsWithNL, dstEndsWithNL bool) {
	isEOFLine := (left == lastSrcLineIdx && right == lastDstLineIdx)
	<ins>isEOFChange :=</ins> <mov1>isEOFLine && srcEndsWithNL != dstEndsWithNL</mov1>
	<ins>if sText != dText || isEOFChange {</ins>
		<mov>if </mov><ins>showStyleChanges || isEOFChange || !renderutil.EqualIgnoringStyle(sText, dText, significantWhitespace)</ins><mov> {</mov>
<mov>			upd++</mov>
<mov>		}</mov>
	<ins>}</ins>
}
`)
}

func TestPipeline_IfElseBlockArgumentAdditionRecovery(t *testing.T) {
	src := []byte(`package main

func f() {
	if layout == HunkLayoutDualColumn {
		renderSideBySideHunk(w, h, filteredPairs, srcLines, dstLines, srcLineBadges, dstLineBadges, leftSpansByLine, rightSpansByLine, numWidth, codeWidth, opts, scratch, sep, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
	} else {
		renderSingleColumnHunk(w, h, filteredPairs, isPairChanged, srcLines, dstLines, srcLineBadges, dstLineBadges, leftSpansByLine, rightSpansByLine, numWidth, termWidth, opts, scratch, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx, layout)
	}
}
`)
	dst := []byte(`package main

func f() {
	if layout == HunkLayoutDualColumn {
		renderSideBySideHunk(w, h, filteredPairs, srcLines, dstLines, srcLineBadges, dstLineBadges, srcLineBadgeColors, dstLineBadgeColors, leftSpansByLine, rightSpansByLine, numWidth, codeWidth, opts, scratch, sep, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
	} else {
		renderSingleColumnHunk(w, h, filteredPairs, isPairChanged, srcLines, dstLines, srcLineBadges, dstLineBadges, srcLineBadgeColors, dstLineBadgeColors, leftSpansByLine, rightSpansByLine, numWidth, termWidth, opts, scratch, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx, layout)
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

	// Only arguments were added, so nothing on the left should be deleted.
	if len(res.Envelope.LeftHighlights) != 0 {
		t.Errorf("expected 0 left highlights, got %d: %+v", len(res.Envelope.LeftHighlights), res.Envelope.LeftHighlights)
	}

	for _, a := range res.Envelope.Actions {
		if a.Action != "insert" || a.Node.Type != "identifier" {
			t.Errorf("expected only identifier insert actions, got: %+v", a)
		}
	}

	// Expect insert highlights for the new arguments on lines 4 and 6.
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func f() {
	if layout == HunkLayoutDualColumn {
		renderSideBySideHunk(w, h, filteredPairs, srcLines, dstLines, srcLineBadges, dstLineBadges, <ins>srcLineBadgeColors, dstLineBadgeColors,</ins> leftSpansByLine, rightSpansByLine, numWidth, codeWidth, opts, scratch, sep, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx)
	} else {
		renderSingleColumnHunk(w, h, filteredPairs, isPairChanged, srcLines, dstLines, srcLineBadges, dstLineBadges, <ins>srcLineBadgeColors, dstLineBadgeColors,</ins> leftSpansByLine, rightSpansByLine, numWidth, termWidth, opts, scratch, srcEndsWithNL, dstEndsWithNL, lastSrcLineIdx, lastDstLineIdx, layout)
	}
}
`)
}

func TestPipeline_IfInitStatementMoveOut(t *testing.T) {
	src := []byte(`package main

func f() {
	if body := findBodyBlock(node, r); body != nil && isTrivialJumpBody(body, r) {
		score -= 20
	}
}
`)
	dst := []byte(`package main

func f() {
	body := node
	if !r.IsBlock(node.Type) {
		body = findBodyBlock(node, r)
	}
	if body != nil && isTrivialJumpBody(body, r) {
		score -= 20
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

	// Initializer statement moves out with := updated to =.
	foundMove := slices.ContainsFunc(res.Envelope.Actions, func(a serialize.Action) bool {
		return a.Action == "move" && a.Node.Type == "short_var_declaration"
	})
	foundUpdate := slices.ContainsFunc(res.Envelope.Actions, func(a serialize.Action) bool {
		return a.Action == "update" && a.Node.Type == "assignment_operator_literal"
	})
	if !foundMove {
		t.Error("expected move action on short_var_declaration when moving out of if statement")
	}
	if !foundUpdate {
		t.Error("expected update action on assignment operator (:= -> =)")
	}

	// The now-redundant semicolon in the if header should be highlighted as deleted.
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func f() {
	if <mov>body </mov><mUpd>:=</mUpd><mov> findBodyBlock(node, r)</mov><del>;</del> body != nil && isTrivialJumpBody(body, r) {
		score -= 20
	}
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func f() {
	<ins>body := node</ins>
	<ins>if !r.IsBlock(node.Type) {</ins>
		<mov>body </mov><mUpd>=</mUpd><mov> findBodyBlock(node, r)</mov>
	<ins>}</ins>
	if body != nil && isTrivialJumpBody(body, r) {
		score -= 20
	}
}
`)
}

func TestPipeline_ShortenedCompoundIdentifierImports(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `#[cfg(test)]
mod tests {
    use crate::ansi::{self, strip_ansi_codes};
    use crate::cli::InspectRawLines;
    use crate::delta::State;
    use crate::style;
    use crate::tests::ansi_test_utils::ansi_test_utils;
    use crate::tests::integration_test_utils::integration_test_utils;
    use crate::tests::test_utils::test_utils;

    #[test]
    fn test_added_file() {
        let config = integration_test_utils::make_config_from_args(&[]);
        let output = integration_test_utils::run_delta(ADDED_FILE_INPUT, &config);
        let output = strip_ansi_codes(&output);
        assert!(output.contains("\nadded: a.py\n"));
    }
}
`
		dst := `#[cfg(test)]
mod tests {
    use crate::ansi::{self, strip_ansi_codes};
    use crate::cli::InspectRawLines;
    use crate::delta::State;
    use crate::style;
    use crate::tests::ansi_test_utils::ansi_test_utils;
    use crate::tests::integration_test_utils;
    use crate::tests::test_utils;

    #[test]
    fn test_added_file() {
        let config = integration_test_utils::make_config_from_args(&[]);
        let output = integration_test_utils::run_delta(ADDED_FILE_INPUT, &config);
        let output = strip_ansi_codes(&output);
        assert!(output.contains("\nadded: a.py\n"));
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `#[cfg(test)]
mod tests {
    use crate::ansi::{self, strip_ansi_codes};
    use crate::cli::InspectRawLines;
    use crate::delta::State;
    use crate::style;
    use crate::tests::ansi_test_utils::ansi_test_utils;
    use crate::tests::integration_test_utils<del>::integration_test_utils</del>;
    use crate::tests::test_utils<del>::test_utils</del>;

    #[test]
    fn test_added_file() {
        let config = integration_test_utils::make_config_from_args(&[]);
        let output = integration_test_utils::run_delta(ADDED_FILE_INPUT, &config);
        let output = strip_ansi_codes(&output);
        assert!(output.contains("\nadded: a.py\n"));
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), dst)
	})

	t.Run("Java", func(t *testing.T) {
		src := `package com.example;

import com.example.foo.bar.baz;
import com.example.alpha.beta.gamma;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`
		dst := `package com.example;

import com.example.foo.bar;
import com.example.alpha.beta;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "Main.java", "Main.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package com.example;

import com.example.foo.bar<del>.baz</del>;
import com.example.alpha.beta<del>.gamma</del>;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), dst)
	})

	t.Run("Lua", func(t *testing.T) {
		src := `function process()
    foo.bar.baz()
    alpha.beta.gamma()
    local x = 1
end
`
		dst := `function process()
    foo.bar()
    alpha.beta()
    local x = 1
end
`

		res, err := Run([]byte(src), []byte(dst), "test.lua", "test.lua", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function process()
    foo.bar<del>.baz</del>()
    alpha.beta<del>.gamma</del>()
    local x = 1
end
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), dst)
	})

	t.Run("Zig", func(t *testing.T) {
		src := `pub fn setup() void {
    const a = @import("foo").bar.baz;
    const b = @import("alpha").beta.gamma;
    const c = 1;
}
`
		dst := `pub fn setup() void {
    const a = @import("foo").bar;
    const b = @import("alpha").beta;
    const c = 1;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.zig", "test.zig", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `pub fn setup() void {
    const a = @import("foo").bar<del>.baz</del>;
    const b = @import("alpha").beta<del>.gamma</del>;
    const c = 1;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), dst)
	})
}

func TestPipeline_HollowEnclosingCall_WrappedMacroAndCallEvolution(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `pub fn parse_color(s: &str) {
    let die = || {
        eprintln!("Invalid color or style attribute: {}", s);
        process::exit(1);
    };
    println!("color: {}", s);
}
`
		dst := `pub fn parse_color(s: &str) {
    let die = || {
        fatal(format!("Invalid color or style attribute: {}", s));
    };
    println!("color: {}", s);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `pub fn parse_color(s: &str) {
    let die = || {
        <upd>eprintln</upd>!("Invalid color or style attribute: {}", s);
        <del>process::exit(1);</del>
    };
    println!("color: {}", s);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `pub fn parse_color(s: &str) {
    let die = || {
        <ins>fatal(</ins><upd>format</upd>!("Invalid color or style attribute: {}", s)<ins>)</ins>;
    };
    println!("color: {}", s);
}
`)
	})

	t.Run("Go", func(t *testing.T) {
		src := `package main

func process(s string) {
	render("Invalid attribute: " + s)
	println(s)
}
`
		dst := `package main

func process(s string) {
	die(format("Invalid attribute: " + s))
	println(s)
}
`

		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func process(s string) {
	<upd>render</upd>("Invalid attribute: " + s)
	println(s)
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func process(s string) {
	<ins>die(</ins><upd>format</upd>("Invalid attribute: " + s)<ins>)</ins>
	println(s)
}
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := `class Test {
    void process(String s) {
        render("Invalid attribute: " + s);
        println(s);
    }
}
`
		dst := `class Test {
    void process(String s) {
        die(format("Invalid attribute: " + s));
        println(s);
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Test {
    void process(String s) {
        <upd>render</upd>("Invalid attribute: " + s);
        println(s);
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Test {
    void process(String s) {
        <ins>die(</ins><upd>format</upd>("Invalid attribute: " + s)<ins>)</ins>;
        println(s);
    }
}
`)
	})

	t.Run("Zig", func(t *testing.T) {
		src := `fn process(s: []const u8) void {
    render("Invalid attribute: {s}", .{s});
    println(s);
}
`
		dst := `fn process(s: []const u8) void {
    die(format("Invalid attribute: {s}", .{s}));
    println(s);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.zig", "test.zig", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn process(s: []const u8) void {
    <del>render</del>("Invalid attribute: {s}", .{s});
    println(s);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn process(s: []const u8) void {
    <ins>die(format("Invalid attribute: {s}", .{s}));</ins>
    println(s);
}
`)
	})
}

func TestPipeline_PairValueReplacement(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: "→".to_string(),
        zero_style: opt.zero_style,
    }
}
`
		dst := `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: format!("{}→{}", 1, 2),
        zero_style: opt.zero_style,
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: <del>"→".to_string()</del>,
        zero_style: opt.zero_style,
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn make_config(opt: &Options) -> Config {
    Config {
        tab_width: opt.tab_width,
        truncation_symbol: <ins>format!("{}→{}", 1, 2)</ins>,
        zero_style: opt.zero_style,
    }
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `function makeConfig(opt: Options): Config {
    return {
        tabWidth: opt.tabWidth,
        truncationSymbol: opt.getSymbol(),
        zeroStyle: opt.zeroStyle,
    };
}
`
		dst := "function makeConfig(opt: Options): Config {\n" +
			"    return {\n" +
			"        tabWidth: opt.tabWidth,\n" +
			"        truncationSymbol: `${1}→${2}`,\n" +
			"        zeroStyle: opt.zeroStyle,\n" +
			"    };\n" +
			"}\n"

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function makeConfig(opt: Options): Config {
    return {
        tabWidth: opt.tabWidth,
        truncationSymbol: <del>opt.getSymbol()</del>,
        zeroStyle: opt.zeroStyle,
    };
}
`)
		expectedDst := "function makeConfig(opt: Options): Config {\n" +
			"    return {\n" +
			"        tabWidth: opt.tabWidth,\n" +
			"        truncationSymbol: <ins>`${1}→${2}`</ins>,\n" +
			"        zeroStyle: opt.zeroStyle,\n" +
			"    };\n" +
			"}\n"
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedDst)
	})

	t.Run("TSX", func(t *testing.T) {
		src := `function App() {
    return (
        <Button
            variant="primary"
            onClick={handleClick()}
            disabled={false}
        />
    );
}
`
		dst := `function App() {
    return (
        <Button
            variant="primary"
            onClick={() => handle()}
            disabled={false}
        />
    );
}
`

		res, err := Run([]byte(src), []byte(dst), "component.tsx", "component.tsx", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function App() {
    return (
        <Button
            variant="primary"
            onClick={<del>handleClick()</del>}
            disabled={false}
        />
    );
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function App() {
    return (
        <Button
            variant="primary"
            onClick={<ins>() => handle()</ins>}
            disabled={false}
        />
    );
}
`)
	})
}

func TestPipeline_Lua_ModuleVariableAndMethod(t *testing.T) {
	src := `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    return vim.fs.normalize(path)
end
`
	dst := `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    local p = vim.fs.normalize(path)
    return p
end
`

	res, err := Run([]byte(src), []byte(dst), "fs.lua", "fs.lua", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    return <mov>vim.fs.normalize(path)</mov>
end
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    <ins>local p =</ins> <mov>vim.fs.normalize(path)</mov>
    return <ins>p</ins>
end
`)
}

func TestPipeline_Lua_LocalVarExclusion(t *testing.T) {
	src := `function build_node(declaration)
    local add_undefined = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    return format(intermediate, add_undefined)
end
`
	dst := `function build_node(declaration)
    local add_undefined_for_parameter = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    local add_undefined = add_undefined_for_parameter or check_optional(declaration)
    return format(intermediate, add_undefined)
end
`

	res, err := Run([]byte(src), []byte(dst), "test.lua", "test.lua", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function build_node(declaration)
    local <upd>add_undefined</upd> = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    return format(intermediate, add_undefined)
end
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function build_node(declaration)
    local <upd>add_undefined_for_parameter</upd> = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    <ins>local add_undefined = add_undefined_for_parameter or check_optional(declaration)</ins>
    return format(intermediate, add_undefined)
end
`)
}

func TestPipeline_Lua_ElseStatementAndFunctionCall(t *testing.T) {
	src := `function handle(val)
    local start = 1
    track_event(val)
    if val > 10 then
        process(val)
    else
        fallback(val)
    end
    local end_val = 2
end
`
	dst := `function handle(val)
    local start = 1
    track_event(val)
    if val > 10 then
        process(val)
    else
        log_fallback()
        fallback(val)
    end
    local end_val = 2
end
`

	res, err := Run([]byte(src), []byte(dst), "test.lua", "test.lua", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), src)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function handle(val)
    local start = 1
    track_event(val)
    if val > 10 then
        process(val)
    else
        <ins>log_fallback()</ins>
        fallback(val)
    end
    local end_val = 2
end
`)
}

func TestPipeline_Lua_ClosureFunctionDefinition(t *testing.T) {
	src := `local function processItems(items)
    precedingInit()
    local currTotal = total(items)
    local currDouble = twice(currTotal)
    local currAffinity = score(currDouble)
    succeedingDone(currTotal, currDouble, currAffinity)
end
`
	dst := `local function processItems(items)
    precedingInit()
    local currTotal = 0
    local currDouble = 0
    local currAffinity = 0

    step01()
    step02()
    step03()
    step04()
    step05()
    step06()
    step07()
    step08()
    step09()
    step10()
    step11()

    local ensureMetrics = function()
        currTotal = total(items)
        currDouble = twice(currTotal)
        currAffinity = score(currDouble)
    end
    ensureMetrics()
    succeedingDone(currTotal, currDouble, currAffinity)
end
`

	res, err := Run([]byte(src), []byte(dst), "main.lua", "main.lua", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `local function processItems(items)
    precedingInit()
    local <mov>currTotal = total(items)</mov>
    local <mov>currDouble = twice(currTotal)</mov>
    local <mov>currAffinity = score(currDouble)</mov>
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `local function processItems(items)
    precedingInit()
    local <ins>currTotal = 0</ins>
    local <ins>currDouble = 0</ins>
    local <ins>currAffinity = 0</ins>

    <ins>step01()</ins>
    <ins>step02()</ins>
    <ins>step03()</ins>
    <ins>step04()</ins>
    <ins>step05()</ins>
    <ins>step06()</ins>
    <ins>step07()</ins>
    <ins>step08()</ins>
    <ins>step09()</ins>
    <ins>step10()</ins>
    <ins>step11()</ins>

    <ins>local ensureMetrics = function()</ins>
<ins>        </ins><mov>currTotal = total(items)</mov>
<ins>        </ins><mov>currDouble = twice(currTotal)</mov>
<ins>        </ins><mov>currAffinity = score(currDouble)</mov>
<ins>    end</ins>
    <ins>ensureMetrics()</ins>
    succeedingDone(currTotal, currDouble, currAffinity)
end
`)
}

func TestPipeline_ChainedMethodCalleeCompatibility(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
                .takes_value(true)
        );
    finalize(flags);
}
`
		dst := `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg(
            Arg::new("nonprintable-notation")
                .long("nonprintable-notation")
                .hide_default_value(true)
        )
        .arg(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
        );
    finalize(flags);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
                <del>.takes_value(true)</del>
        );
    finalize(flags);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg<ins>(</ins>
<ins>            Arg::new("nonprintable-notation")</ins>
<ins>                .long("nonprintable-notation")</ins>
<ins>                .hide_default_value(true)</ins>
<ins>        )</ins>
        <ins>.arg</ins>(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
        );
    finalize(flags);
}
`)
	})
}

func TestPipeline_SplitStatements(t *testing.T) {
	t.Run("Go", func(t *testing.T) {
		src := `package test

func f() {
	setupContext()
	if !condA() && condB() {
		action1()
	}
	finalizeContext()
}
`
		dst := `package test

func f() {
	setupContext()
	if condA() || !otherCheck() {
		return
	}
	if condB() {
		action2()
	}
	finalizeContext()
}
`

		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package test

func f() {
	setupContext()
	<del>if !</del><mov>condA()</mov> <del>&& condB() {</del>
<del>		action1()</del>
<del>	}</del>
	finalizeContext()
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package test

func f() {
	setupContext()
	<ins>if</ins> <mov>condA()</mov> <ins>|| !otherCheck() {</ins>
<ins>		return</ins>
<ins>	}</ins>
	<ins>if condB() {</ins>
<ins>		action2()</ins>
<ins>	}</ins>
	finalizeContext()
}
`)
	})

	t.Run("GoBidirectional", func(t *testing.T) {
		src := `package test

func f() {
	setupContext()
	res := checkA() || checkB()
	finalizeContext(res)
}
`
		dst := `package test

func f() {
	setupContext()
	if checkA() {
		return
	}
	other := checkB()
	res := other || defaultVal()
	finalizeContext(res)
}
`

		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package test

func f() {
	setupContext()
	res := <mov>checkA()</mov> <del>|| checkB()</del>
	finalizeContext(res)
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package test

func f() {
	setupContext()
	<ins>if</ins> <mov>checkA()</mov> <ins>{</ins>
<ins>		return</ins>
<ins>	}</ins>
	<ins>other := checkB()</ins>
	res := <ins>other || defaultVal()</ins>
	finalizeContext(res)
}
`)
	})
}

func TestPipeline_IndentedBlockEvolution(t *testing.T) {
	t.Run("Python", func(t *testing.T) {
		src := `def process_records(flag: bool, count: int) -> None:
    total = 0
    if flag:
        total += 1
    else:
        if count > 0:
            handle_batch(count)
            total += count
        finish_records()
    record_metric(total)
`
		dst := `def process_records(flag: bool, count: int) -> None:
    total = 0
    if flag:
        total += 1
    else:
        if count > 0:
            if check_extra_threshold(count):
                split_and_process(count)
                log_debug_batch(count)
                record_entry(1)
                record_entry(2)
                record_entry(3)
                record_entry(4)
            elif check_secondary_threshold(count):
                record_fallback(count)
                apply_single_entry(count)
                record_entry(5)
                record_entry(6)
            else:
                notify_worker_pool(count)
                flush_intermediate_state(count)
            handle_batch(count)
            total += count
        record_unchanged(count)
    record_metric(total)
`

		res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `def process_records(flag: bool, count: int) -> None:
    total = 0
    if flag:
        total += 1
    else:
        if count > 0:
            handle_batch(count)
            total += count
        <del>finish_records()</del>
    record_metric(total)
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `def process_records(flag: bool, count: int) -> None:
    total = 0
    if flag:
        total += 1
    else:
        if count > 0:
            <ins>if check_extra_threshold(count):</ins>
<ins>                split_and_process(count)</ins>
<ins>                log_debug_batch(count)</ins>
<ins>                record_entry(1)</ins>
<ins>                record_entry(2)</ins>
<ins>                record_entry(3)</ins>
<ins>                record_entry(4)</ins>
<ins>            elif check_secondary_threshold(count):</ins>
<ins>                record_fallback(count)</ins>
<ins>                apply_single_entry(count)</ins>
<ins>                record_entry(5)</ins>
<ins>                record_entry(6)</ins>
<ins>            else:</ins>
<ins>                notify_worker_pool(count)</ins>
<ins>                flush_intermediate_state(count)</ins>
            handle_batch(count)
            total += count
        <ins>record_unchanged(count)</ins>
    record_metric(total)
`)
	})
}

func TestPipeline_DefStatements(t *testing.T) {
	t.Run("Python", func(t *testing.T) {
		src := `def process(total_count: int, old_limit: int, name: str):
    res = 0
    return res
`
		dst := `def process(batch_size: str, new_limit: float, name: str):
    res = 0
    return res
`

		res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `def process(<del>total_count: int, old_limit: int,</del> name: str):
    res = 0
    return res
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `def process(<ins>batch_size: str, new_limit: float,</ins> name: str):
    res = 0
    return res
`)
	})

	t.Run("PythonStationaryLadder", func(t *testing.T) {
		src := `def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 3:
        do_three()
    do_done()
`
		dst := `def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 2:
        do_two()
    elif x == 3:
        do_three()
    do_done()
`

		res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 3:
        do_three()
    do_done()
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `def evaluate_option():
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
}

func TestPipeline_MethodInvocationEvolution(t *testing.T) {
	t.Run("Java", func(t *testing.T) {
		src := `class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> push(paint(format(num2, spec, width, opt, config)));
            default -> unreachable();
        }
    }
}
`
		dst := `class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> {
                push(paint(format(new_num2, spec, width, opt, config)));
            }
            default -> unreachable();
        }
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> push(paint(format(<upd>num2</upd>, spec, width, opt, config)));
            default -> unreachable();
        }
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> <ins>{</ins>
                push(paint(format(<upd>new_num2</upd>, spec, width, opt, config)));
            <ins>}</ins>
            default -> unreachable();
        }
    }
}
`)
	})
}

func TestPipeline_ParameterDeclarationEvolution(t *testing.T) {
	t.Run("Java", func(t *testing.T) {
		src := `class Service {
    int process(int totalCount, double oldLimit, String name) {
        int res = 0;
        return res;
    }
}
`
		dst := `class Service {
    int process(long batchSize, float newLimit, String name) {
        int res = 0;
        return res;
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Service {
    int process(<del>int totalCount, double oldLimit,</del> String name) {
        int res = 0;
        return res;
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Service {
    int process(<ins>long batchSize, float newLimit,</ins> String name) {
        int res = 0;
        return res;
    }
}
`)
	})

	t.Run("Cpp", func(t *testing.T) {
		src := `int process(int totalCount, double oldLimit, const char* name) {
    int res = 0;
    return res;
}
`
		dst := `int process(long batchSize, float newLimit, const char* name) {
    int res = 0;
    return res;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.cpp", "test.cpp", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `int process(<del>int totalCount, double oldLimit,</del> const char* name) {
    int res = 0;
    return res;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `int process(<ins>long batchSize, float newLimit,</ins> const char* name) {
    int res = 0;
    return res;
}
`)
	})
}

func TestPipeline_FuncCallEvolution(t *testing.T) {
	t.Run("C", func(t *testing.T) {
		src := `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        printf("Invalid commit-regex: %s. Must be valid regex.\n", pattern);
        exit(1);
    }
    printf("Ready: %s\n", pattern);
}
`
		dst := `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        fatal(format_error("Invalid commit-regex: %s. Must be valid regex.\n", pattern));
    }
    printf("Ready: %s\n", pattern);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.c", "test.c", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        <upd>printf</upd>("Invalid commit-regex: %s. Must be valid regex.\n", pattern);
        <del>exit(1);</del>
    }
    printf("Ready: %s\n", pattern);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        <ins>fatal(</ins><upd>format_error</upd>("Invalid commit-regex: %s. Must be valid regex.\n", pattern)<ins>)</ins>;
    }
    printf("Ready: %s\n", pattern);
}
`)
	})
}

func TestPipeline_NamespaceUsingDeclaration(t *testing.T) {
	t.Run("Cpp", func(t *testing.T) {
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

		res, err := Run([]byte(src), []byte(dst), "test.cpp", "test.cpp", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `void setup() {
    int a = 1;
    int b = 2;
}

namespace test_env {
    void init() {}
    using <del>foo::</del>bar::baz;
    void teardown() {}
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `void setup() {
    int a = 1;
    int b = 2;
    <ins>int c = 3;</ins>
    <ins>int d = 4;</ins>
    <ins>int e = 5;</ins>
    <ins>int f = 6;</ins>
}

namespace test_env {
    void init() {}
    using bar::baz;
    void teardown() {}
}
`)
	})
}

func TestPipeline_TemplateCallAndRangeFor(t *testing.T) {
	t.Run("Cpp", func(t *testing.T) {
		src := `#include <vector>

void process() {
    int start = 0;
    std::vector<int> items = {1, 2, 3};
    for (const auto& val : items) {
        start += val;
    }
    if (int count = items.size(); count > 0) {
        start += count;
    }
    int end = 100;
}
`
		dst := `#include <vector>

void process() {
    int start = 0;
    std::vector<int> items = {1, 2, 3};
    for (const auto& val : items) {
        start += val * 2;
    }
    if (int count = items.size(); count > 0) {
        start += count;
    }
    int end = 100;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.cpp", "test.cpp", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `#include <vector>

void process() {
    int start = 0;
    std::vector<int> items = {1, 2, 3};
    for (const auto& val : items) {
        start += val;
    }
    if (int count = items.size(); count > 0) {
        start += count;
    }
    int end = 100;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `#include <vector>

void process() {
    int start = 0;
    std::vector<int> items = {1, 2, 3};
    for (const auto& val : items) {
        start += val <ins>* 2</ins>;
    }
    if (int count = items.size(); count > 0) {
        start += count;
    }
    int end = 100;
}
`)
	})
}

func TestPipeline_BlockWrappedCaseClauseMatching(t *testing.T) {
	t.Run("TypeScript", func(t *testing.T) {
		src := `function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2:
            return push(paint(format(num2, spec, width, opt, config)));
        default:
            return unreachable();
    }
}
`
		dst := `function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2: {
            return push(paint(format(new_num2, spec, width, opt, config)));
        }
        default:
            return unreachable();
    }
}
`

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2:
            return push(paint(format(<upd>num2</upd>, spec, width, opt, config)));
        default:
            return unreachable();
    }
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2: <ins>{</ins>
            return push(paint(format(<upd>new_num2</upd>, spec, width, opt, config)));
        <ins>}</ins>
        default:
            return unreachable();
    }
}
`)
	})
}

func TestPipeline_ParameterDeclarationEvolution_TypeScript(t *testing.T) {
	t.Run("TypeScript", func(t *testing.T) {
		src := `function process(totalCount: number, oldLimit: number, name: string): number {
    let res = 0;
    return res;
}
`
		dst := `function process(batchSize: string, newLimit: boolean, name: string): number {
    let res = 0;
    return res;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function process(<del>totalCount: number, oldLimit: number,</del> name: string): number {
    let res = 0;
    return res;
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function process(<ins>batchSize: string, newLimit: boolean,</ins> name: string): number {
    let res = 0;
    return res;
}
`)
	})
}

func TestPipeline_RollupMatchedContainers_ElseClause(t *testing.T) {
	t.Run("JavaScript", func(t *testing.T) {
		src := `function processRecords(flag, count) {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handleBatch(count);
            total += count;
        }
        finishRecords();
    }
    recordMetric(total);
}
`
		dst := `function processRecords(flag, count) {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            if (checkExtraThreshold(count)) {
                splitAndProcess(count);
                logDebugBatch(count);
                recordEntry(1);
                recordEntry(2);
                recordEntry(3);
                recordEntry(4);
            } else if (checkSecondaryThreshold(count)) {
                recordFallback(count);
                applySingleEntry(count);
                recordEntry(5);
                recordEntry(6);
            } else {
                notifyWorkerPool(count);
                flushIntermediateState(count);
            }
            handleBatch(count);
            total += count;
        }
        recordUnchanged(count);
    }
    recordMetric(total);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.js", "test.js", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function processRecords(flag, count) {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handleBatch(count);
            total += count;
        }
        <del>finishRecords();</del>
    }
    recordMetric(total);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function processRecords(flag, count) {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            <ins>if (checkExtraThreshold(count)) {</ins>
<ins>                splitAndProcess(count);</ins>
<ins>                logDebugBatch(count);</ins>
<ins>                recordEntry(1);</ins>
<ins>                recordEntry(2);</ins>
<ins>                recordEntry(3);</ins>
<ins>                recordEntry(4);</ins>
<ins>            } else if (checkSecondaryThreshold(count)) {</ins>
<ins>                recordFallback(count);</ins>
<ins>                applySingleEntry(count);</ins>
<ins>                recordEntry(5);</ins>
<ins>                recordEntry(6);</ins>
<ins>            } else {</ins>
<ins>                notifyWorkerPool(count);</ins>
<ins>                flushIntermediateState(count);</ins>
<ins>            }</ins>
            handleBatch(count);
            total += count;
        }
        <ins>recordUnchanged(count);</ins>
    }
    recordMetric(total);
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handleBatch(count);
            total += count;
        }
        finishRecords();
    }
    recordMetric(total);
}
`
		dst := `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            if (checkExtraThreshold(count)) {
                splitAndProcess(count);
                logDebugBatch(count);
                recordEntry(1);
                recordEntry(2);
                recordEntry(3);
                recordEntry(4);
            } else if (checkSecondaryThreshold(count)) {
                recordFallback(count);
                applySingleEntry(count);
                recordEntry(5);
                recordEntry(6);
            } else {
                notifyWorkerPool(count);
                flushIntermediateState(count);
            }
            handleBatch(count);
            total += count;
        }
        recordUnchanged(count);
    }
    recordMetric(total);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handleBatch(count);
            total += count;
        }
        <del>finishRecords();</del>
    }
    recordMetric(total);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            <ins>if (checkExtraThreshold(count)) {</ins>
<ins>                splitAndProcess(count);</ins>
<ins>                logDebugBatch(count);</ins>
<ins>                recordEntry(1);</ins>
<ins>                recordEntry(2);</ins>
<ins>                recordEntry(3);</ins>
<ins>                recordEntry(4);</ins>
<ins>            } else if (checkSecondaryThreshold(count)) {</ins>
<ins>                recordFallback(count);</ins>
<ins>                applySingleEntry(count);</ins>
<ins>                recordEntry(5);</ins>
<ins>                recordEntry(6);</ins>
<ins>            } else {</ins>
<ins>                notifyWorkerPool(count);</ins>
<ins>                flushIntermediateState(count);</ins>
<ins>            }</ins>
            handleBatch(count);
            total += count;
        }
        <ins>recordUnchanged(count);</ins>
    }
    recordMetric(total);
}
`)
	})

	t.Run("Zig", func(t *testing.T) {
		src := `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handle_batch(count);
            total += count;
        }
        finish_records();
    }
    record_metric(total);
}
`
		dst := `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            switch (as_singleton_list(count)) {
                1 => {
                    var split = split_records(count);
                    if (split > 1) {
                        process_all(&split);
                    } else {
                        record_fallback(count);
                    }
                },
                else => {
                    record_fallback(count);
                },
            }
            handle_batch(count);
            total += count;
        }
        record_unchanged(count);
    }
    record_metric(total);
}
`

		res, err := Run([]byte(src), []byte(dst), "test.zig", "test.zig", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            handle_batch(count);
            total += count;
        }
        <del>finish_records();</del>
    }
    record_metric(total);
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    if (flag) {
        total += 1;
    } else {
        if (count > 0) {
            <ins>switch (as_singleton_list(count)) {</ins>
<ins>                1 => {</ins>
<ins>                    var split = split_records(count);</ins>
<ins>                    if (split > 1) {</ins>
<ins>                        process_all(&split);</ins>
<ins>                    } else {</ins>
<ins>                        record_fallback(count);</ins>
<ins>                    }</ins>
<ins>                },</ins>
<ins>                else => {</ins>
<ins>                    record_fallback(count);</ins>
<ins>                },</ins>
<ins>            }</ins>
            handle_batch(count);
            total += count;
        }
        <ins>record_unchanged(count);</ins>
    }
    record_metric(total);
}
`)
	})
}

func TestPipeline_SameLineParameterDeclarationMoves(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn format_and_paint<'a>(
    format_data: &[PlaceholderData<'a>],
    style: &Style,
) {}
`
		dst := `fn format_and_paint<'a>(
    line_numbers_data: &'a NumbersData,
    style: &Style,
) {}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn format_and_paint<'a>(
    <del>format_data: &[PlaceholderData<'a>],</del>
    style: &Style,
) {}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn format_and_paint<'a>(
    <ins>line_numbers_data: &'a NumbersData,</ins>
    style: &Style,
) {}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `function formatAndPaint(
    formatData: Array<string>,
    style: Style,
) {}
`
		dst := `function formatAndPaint(
    lineNumbersData: string,
    style: Style,
) {}
`

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function formatAndPaint(
    <del>formatData: Array<string>,</del>
    style: Style,
) {}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function formatAndPaint(
    <ins>lineNumbersData: string,</ins>
    style: Style,
) {}
`)
	})

	t.Run("Java", func(t *testing.T) {
		src := `class Test {
    void formatAndPaint(
        List<String> formatData,
        Style style
    ) {}
}
`
		dst := `class Test {
    void formatAndPaint(
        String lineNumbersData,
        Style style
    ) {}
}
`

		res, err := Run([]byte(src), []byte(dst), "test.java", "test.java", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `class Test {
    void formatAndPaint(
        <del>List<String> formatData,</del>
        Style style
    ) {}
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `class Test {
    void formatAndPaint(
        <ins>String lineNumbersData,</ins>
        Style style
    ) {}
}
`)
	})

	t.Run("Zig", func(t *testing.T) {
		src := `fn formatAndPaint(
    format_data: []const u8,
    style: Style,
) void {}
`
		dst := `fn formatAndPaint(
    line_numbers_data: u8,
    style: Style,
) void {}
`

		res, err := Run([]byte(src), []byte(dst), "test.zig", "test.zig", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn formatAndPaint(
    <del>format_data: []const u8,</del>
    style: Style,
) void {}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn formatAndPaint(
    <ins>line_numbers_data: u8,</ins>
    style: Style,
) void {}
`)
	})
}

func TestPipeline_LocalVarDeclarationTheftRecovery(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn set_widths() {
    opt.available_terminal_width = Term::stdout().size().1;
    let empty_builtin_features = HashMap::new();
}
`
		dst := `fn set_widths() {
    let term_stdout = Term::stdout();
    opt.available_terminal_width = term_stdout.size().1;
}
`
		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn set_widths() {
    opt.available_terminal_width = <mov>Term::stdout()</mov><del>.</del>size().1;
    <del>let empty_builtin_features = HashMap::new();</del>
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn set_widths() {
    <ins>let term_stdout = </ins><mov>Term::stdout()</mov><ins>;</ins>
    opt.available_terminal_width = <ins>term_stdout.</ins>size().1;
}
`)
	})

	t.Run("Go", func(t *testing.T) {
		src := `package main

func setWidths() {
	opt.availableTerminalWidth = TermStdout().size().one
	emptyBuiltinFeatures := HashMapNew()
}
`
		dst := `package main

func setWidths() {
	termStdout := TermStdout()
	opt.availableTerminalWidth = termStdout.size().one
}
`
		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func setWidths() {
	opt.availableTerminalWidth = <mov>TermStdout()</mov><del>.</del>size().one
	<del>emptyBuiltinFeatures := HashMapNew()</del>
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func setWidths() {
	<ins>termStdout :=</ins> <mov>TermStdout()</mov>
	opt.availableTerminalWidth = <ins>termStdout.</ins>size().one
}
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `function setWidths() {
    opt.availableTerminalWidth = Term.stdout().size().one;
    const emptyBuiltinFeatures = HashMap.create();
}
`
		dst := `function setWidths() {
    const termStdout = Term.stdout();
    opt.availableTerminalWidth = termStdout.size().one;
}
`
		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `function setWidths() {
    opt.availableTerminalWidth = <mov>Term.stdout()</mov><del>.</del>size().one;
    <del>const emptyBuiltinFeatures = HashMap.create();</del>
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function setWidths() {
    <ins>const termStdout = </ins><mov>Term.stdout()</mov><ins>;</ins>
    opt.availableTerminalWidth = <ins>termStdout.</ins>size().one;
}
`)
	})
}

func TestPipeline_RecoverCohortSiblings_UnqualifiedUseDeclaration(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `pub mod integration_test_utils {
    use bytelines::ByteLines;
    use itertools;
    use crate::ansi;

    pub fn make_options() -> i32 {
        let default_limit = 42;
        default_limit
    }
}
`
		dst := `use bytelines::ByteLines;
use itertools;
use crate::ansi;

pub fn make_options() -> i32 {
    let default_limit = 42;
    default_limit
}
`

		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `<del>pub mod integration_test_utils {</del>
    <mov>use bytelines::ByteLines;</mov>
    <mov>use itertools;</mov>
    <mov>use crate::ansi;</mov>

    <mov>pub fn make_options() -> i32 {</mov>
<mov>        let default_limit = 42;</mov>
<mov>        default_limit</mov>
<mov>    }</mov>
<del>}</del>
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `<mov>use bytelines::ByteLines;</mov>
<mov>use itertools;</mov>
<mov>use crate::ansi;</mov>

<mov>pub fn make_options() -> i32 {</mov>
<mov>    let default_limit = 42;</mov>
<mov>    default_limit</mov>
<mov>}</mov>
`)
	})

	t.Run("Cpp", func(t *testing.T) {
		src := `namespace integration_test_utils {
    using std::string;
    using std::vector;
    using std::map;

    int make_options() {
        int default_limit = 42;
        return default_limit;
    }
}
`
		dst := `using std::string;
using std::vector;
using std::map;

int make_options() {
    int default_limit = 42;
    return default_limit;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.cpp", "test.cpp", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `<del>namespace integration_test_utils {</del>
    <mov>using std::string;</mov>
    <mov>using std::vector;</mov>
    <mov>using std::map;</mov>

    <mov>int make_options() {</mov>
<mov>        int default_limit = 42;</mov>
<mov>        return default_limit;</mov>
<mov>    }</mov>
<del>}</del>
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `<mov>using std::string;</mov>
<mov>using std::vector;</mov>
<mov>using std::map;</mov>

<mov>int make_options() {</mov>
<mov>    int default_limit = 42;</mov>
<mov>    return default_limit;</mov>
<mov>}</mov>
`)
	})

	t.Run("TypeScript", func(t *testing.T) {
		src := `namespace integration_test_utils {
    import ByteLines = bytelines.ByteLines;
    import ansi = crate.ansi;
    import itertools = crate.itertools;

    export function make_options(): number {
        let default_limit = 42;
        return default_limit;
    }
}
`
		dst := `import ByteLines = bytelines.ByteLines;
import ansi = crate.ansi;
import itertools = crate.itertools;

export function make_options(): number {
    let default_limit = 42;
    return default_limit;
}
`

		res, err := Run([]byte(src), []byte(dst), "test.ts", "test.ts", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `<del>namespace integration_test_utils {</del>
    <mov>import ByteLines = bytelines.ByteLines;</mov>
    <mov>import ansi = crate.ansi;</mov>
    <mov>import itertools = crate.itertools;</mov>

    <mov>export function make_options(): number {</mov>
<mov>        let default_limit = 42;</mov>
<mov>        return default_limit;</mov>
<mov>    }</mov>
<del>}</del>
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `<mov>import ByteLines = bytelines.ByteLines;</mov>
<mov>import ansi = crate.ansi;</mov>
<mov>import itertools = crate.itertools;</mov>

<mov>export function make_options(): number {</mov>
<mov>    let default_limit = 42;</mov>
<mov>    return default_limit;</mov>
<mov>}</mov>
`)
	})

	t.Run("Python", func(t *testing.T) {
		src := `class IntegrationTestUtils:
    from os import path
    import itertools
    from sys import version

    def make_options():
        default_limit = 42
        return default_limit
`
		dst := `from os import path
import itertools
from sys import version

def make_options():
    default_limit = 42
    return default_limit
`

		res, err := Run([]byte(src), []byte(dst), "test.py", "test.py", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `<del>class IntegrationTestUtils:</del>
    <mov>from os import path</mov>
    <mov>import itertools</mov>
    <mov>from sys import version</mov>

    <mov>def make_options():</mov>
<mov>        default_limit = 42</mov>
<mov>        return default_limit</mov>
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `<mov>from os import path</mov>
<mov>import itertools</mov>
<mov>from sys import version</mov>

<mov>def make_options():</mov>
<mov>    default_limit = 42</mov>
<mov>    return default_limit</mov>
`)
	})
}

func TestPipeline_ChainedMethodCall_WrapperUpdateTheft(t *testing.T) {
	t.Run("Rust", func(t *testing.T) {
		src := `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.contains_id("diff") && self.matches.get_flag("diff") {
            _ => None,
        },
    };
    self.flush_buffer();
}
`
		dst := `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.try_contains_id("diff").unwrap_or_default()
            && self.matches.get_flag("diff")
        {
            _ => None,
        },
    };
    self.flush_buffer();
}
`
		res, err := Run([]byte(src), []byte(dst), "app.rs", "app.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.<upd>contains_id</upd>("diff") && self.matches.get_flag("diff") {
            _ => None,
        },
    };
    self.flush_buffer();
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.<upd>try_contains_id</upd>("diff")<ins>.unwrap_or_default()</ins>
            && self.matches.get_flag("diff")
        {
            _ => None,
        },
    };
    self.flush_buffer();
}
`)
	})

	t.Run("Go", func(t *testing.T) {
		src := `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.ContainsID("diff") && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}
`
		dst := `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.TryContainsID("diff").UnwrapOrDefault() && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}
`
		res, err := Run([]byte(src), []byte(dst), "app.go", "app.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.<upd>ContainsID</upd>("diff") && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.<upd>TryContainsID</upd>("diff")<ins>.UnwrapOrDefault()</ins> && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}
`)
	})

	t.Run("Lua", func(t *testing.T) {
		src := `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:contains_id("diff") and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`
		dst := `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:try_contains_id("diff"):unwrap_or_default() and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`
		res, err := Run([]byte(src), []byte(dst), "app.lua", "app.lua", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:<upd>contains_id</upd>("diff") and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:<upd>try_contains_id</upd>("diff")<ins>:unwrap_or_default()</ins> and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`)
	})
}

func TestPipeline_Rust_ScopedUseListModification(t *testing.T) {
	src := `use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};
use std::str::FromStr;

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, get_args_from_env_var},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`
	dst := `use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, get_args_from_env_opts_var, get_args_from_env_vars},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`
	res, err := Run([]byte(src), []byte(dst), "app.rs", "app.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};
<del>use std::str::FromStr;</del>

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, <upd>get_args_from_env_var</upd>},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, <ins>get_args_from_env_opts_var,</ins> <upd>get_args_from_env_vars</upd>},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`)
}

func TestPipeline_InlinedVar_BinaryChain_NoSpuriousMoves(t *testing.T) {
	src := `package main

func check(r *Rules, n1 *Node) {
	var isIgnored bool
	isStmtOrDecl := (r != nil && (r.IsStatement(n1.Type) || r.IsDeclaration(n1.Type))) ||
		(r == nil && (rules.IsStatement(n1.Type) || rules.IsDeclaration(n1.Type)))
	if r != nil {
		isIgnored = r.IsDelimiter(n1.Type, n1.Label) ||
			r.IsOperatorLiteral(n1.Type) || r.IsIdentifier(n1.Type) ||
			(r.IsWrapper(n1.Type) && !isStmtOrDecl) || r.IsBlock(n1.Type) || r.IsFlattened(n1.Type) ||
			r.IsPunctuation(n1.Type) || r.IsPunctuation(n1.Label)
	} else {
		isIgnored = rules.IsDelimiter(n1.Type, n1.Label) ||
			rules.IsOperatorLiteral(n1.Type) || rules.IsIdentifier(n1.Type) ||
			(rules.IsWrapper(n1.Type) && !isStmtOrDecl) || rules.IsBlock(n1.Type) || rules.IsFlattened(n1.Type) ||
			rules.IsPunctuation(n1.Type) || rules.IsPunctuation(n1.Label)
	}
}
`

	dst := `package main

func check(r *Rules, n1 *Node) {
	var isIgnored bool
	if r != nil {
		isIgnored = r.IsDelimiter(n1.Type, n1.Label) ||
			r.IsOperatorLiteral(n1.Type) || r.IsIdentifier(n1.Type) ||
			(r.IsWrapper(n1.Type) && !r.IsStatement(n1.Type) && !r.IsDeclaration(n1.Type)) ||
			r.IsBlock(n1.Type) || r.IsFlattened(n1.Type) ||
			r.IsPunctuation(n1.Type) || r.IsPunctuation(n1.Label)
	} else {
		isIgnored = rules.IsDelimiter(n1.Type, n1.Label) ||
			rules.IsOperatorLiteral(n1.Type) || rules.IsIdentifier(n1.Type) ||
			(rules.IsWrapper(n1.Type) && !rules.IsStatement(n1.Type) && !rules.IsDeclaration(n1.Type)) ||
			rules.IsBlock(n1.Type) || rules.IsFlattened(n1.Type) ||
			rules.IsPunctuation(n1.Type) || rules.IsPunctuation(n1.Label)
	}
}
`

	res, err := Run([]byte(src), []byte(dst), "a.go", "b.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `package main

func check(r *Rules, n1 *Node) {
	var isIgnored bool
	<del>isStmtOrDecl := (r != nil && (r.IsStatement(n1.Type) || r.IsDeclaration(n1.Type))) ||</del>
<del>		(r == nil && (rules.IsStatement(n1.Type) || rules.IsDeclaration(n1.Type)))</del>
	if r != nil {
		isIgnored = r.IsDelimiter(n1.Type, n1.Label) ||
			r.IsOperatorLiteral(n1.Type) || r.IsIdentifier(n1.Type) ||
			(r.IsWrapper(n1.Type) && !<del>isStmtOrDecl</del>) || r.IsBlock(n1.Type) || r.IsFlattened(n1.Type) ||
			r.IsPunctuation(n1.Type) || r.IsPunctuation(n1.Label)
	} else {
		isIgnored = rules.IsDelimiter(n1.Type, n1.Label) ||
			rules.IsOperatorLiteral(n1.Type) || rules.IsIdentifier(n1.Type) ||
			(rules.IsWrapper(n1.Type) && !<del>isStmtOrDecl</del>) || rules.IsBlock(n1.Type) || rules.IsFlattened(n1.Type) ||
			rules.IsPunctuation(n1.Type) || rules.IsPunctuation(n1.Label)
	}
}
`

	expectedRight := `package main

func check(r *Rules, n1 *Node) {
	var isIgnored bool
	if r != nil {
		isIgnored = r.IsDelimiter(n1.Type, n1.Label) ||
			r.IsOperatorLiteral(n1.Type) || r.IsIdentifier(n1.Type) ||
			(r.IsWrapper(n1.Type) <ins>&& !r.IsStatement(n1.Type)</ins> && !<ins>r.IsDeclaration(n1.Type)</ins>) ||
			r.IsBlock(n1.Type) || r.IsFlattened(n1.Type) ||
			r.IsPunctuation(n1.Type) || r.IsPunctuation(n1.Label)
	} else {
		isIgnored = rules.IsDelimiter(n1.Type, n1.Label) ||
			rules.IsOperatorLiteral(n1.Type) || rules.IsIdentifier(n1.Type) ||
			(rules.IsWrapper(n1.Type) <ins>&& !rules.IsStatement(n1.Type)</ins> && !<ins>rules.IsDeclaration(n1.Type)</ins>) ||
			rules.IsBlock(n1.Type) || rules.IsFlattened(n1.Type) ||
			rules.IsPunctuation(n1.Type) || rules.IsPunctuation(n1.Label)
	}
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}

func TestPipeline_InlinedVar_BinaryChain_Rust(t *testing.T) {
	src := `fn check(r: Option<&Rules>, n: &Node) {
	let is_stmt_or_decl = (r.is_some() && (is_stmt(n) || is_decl(n))) ||
		(r.is_none() && (is_stmt_legacy(n) || is_decl_legacy(n)));
	let is_ignored = is_delim(n) || is_op(n) || is_ident(n) ||
		(is_wrapper(n) && !is_stmt_or_decl) || is_block(n) || is_punct(n);
}
`

	dst := `fn check(r: Option<&Rules>, n: &Node) {
	let is_ignored = is_delim(n) || is_op(n) || is_ident(n) ||
		(is_wrapper(n) && !is_stmt(n) && !is_decl(n)) || is_block(n) || is_punct(n);
}
`

	res, err := Run([]byte(src), []byte(dst), "a.rs", "b.rs", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	expectedLeft := `fn check(r: Option<&Rules>, n: &Node) {
	<del>let is_stmt_or_decl = (r.is_some() && (is_stmt(n) || is_decl(n))) ||</del>
<del>		(r.is_none() && (is_stmt_legacy(n) || is_decl_legacy(n)));</del>
	let is_ignored = is_delim(n) || is_op(n) || is_ident(n) ||
		(is_wrapper(n) && !<del>is_stmt_or_decl</del>) || is_block(n) || is_punct(n);
}
`

	expectedRight := `fn check(r: Option<&Rules>, n: &Node) {
	let is_ignored = is_delim(n) || is_op(n) || is_ident(n) ||
		(is_wrapper(n) && !<ins>is_stmt(n) && !is_decl(n)</ins>) || is_block(n) || is_punct(n);
}
`

	testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), expectedLeft)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), expectedRight)
}
