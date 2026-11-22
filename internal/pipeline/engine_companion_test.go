package pipeline

import (
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
