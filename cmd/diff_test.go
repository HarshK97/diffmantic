package cmd

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/unified"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// TestMain clears user config and pager env vars so host settings don't break cmd tests.
func TestMain(m *testing.M) {
	for _, env := range os.Environ() {
		if k, _, ok := strings.Cut(env, "="); ok {
			if strings.HasPrefix(k, "DIFFM_") && !strings.HasPrefix(k, "DIFFM_TEST_") {
				_ = os.Unsetenv(k)
				continue
			}
			switch k {
			case "PAGER", "GIT_PAGER", "COLUMNS", "NO_COLOR", "CLICOLOR", "CLICOLOR_FORCE",
				"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE":
				_ = os.Unsetenv(k)
			}
		}
	}
	os.Exit(m.Run())
}

// resetRootCmdFlags resets rootCmd args and returns all registered flags to their default values and clean Changed state.
func resetRootCmdFlags() {
	rootCmd.SetArgs(nil)
	rootCmd.Flags().VisitAll(func(f *pflag.Flag) {
		_ = f.Value.Set(f.DefValue)
		f.Changed = false
	})
}

func TestRootCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{name: "format", shorthand: "f", defValue: ""},
		{name: "ignore-comments", shorthand: "C", defValue: "false"},
		{name: "parse-error-limit", shorthand: "e", defValue: "0"},
		{name: "ui", shorthand: "", defValue: "false"},
		{name: "full", shorthand: "", defValue: "false"},
		{name: "cached", shorthand: "", defValue: "false"},
		{name: "parse-tree", shorthand: "", defValue: "false"},
		{name: "cst", shorthand: "", defValue: "false"},
		{name: "tab-width", shorthand: "", defValue: "4"},
	}

	for _, tt := range flags {
		f := rootCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Fatalf("flag %q not registered on rootCmd", tt.name)
		}
		if tt.shorthand != "" && f.Shorthand != tt.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
		}
		if f.DefValue != tt.defValue {
			t.Errorf("flag %q default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestComputeDiffWithDevNull(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	devNull := os.DevNull
	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(devNull, tmpFile, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(/dev/null, sample.go) failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}

	res2, err := pipeline.RunFiles(tmpFile, devNull, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(sample.go, /dev/null) failed: %v", err)
	}
	if res2 == nil || res2.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope for deleted file")
	}
}

func TestComputeDiffUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.unknown")
	fileB := filepath.Join(dir, "b.unknown")

	_ = os.WriteFile(fileA, []byte("line 1\nline 2\nline 3\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("line 1\nline 2 modified\nline 3\nline 4\n"), 0o644)

	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles failed for unsupported files: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil result and envelope for unsupported file diff")
	}
	if len(res.Envelope.Actions) == 0 {
		t.Error("expected fallback line diff actions for unsupported files, got 0")
	}
}

func TestComputeDiffWithParseErrorLimit(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	fileB := filepath.Join(dir, "b.go")

	// File with syntax error (missing := RHS)
	_ = os.WriteFile(fileA, []byte("package main\n\nfunc foo() {\n\tx :=\n}\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc foo() {\n\tx := 10\n}\n"), 0o644)

	opts := serialize.EnvelopeOptions{IncludeActions: true}
	// Zero error limit should fall back to line diff
	resDefault, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 0, EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(0) failed: %v", err)
	}
	if resDefault.MatchResult != nil {
		t.Error("expected line diff fallback (nil MatchResult) when limit is 0")
	}
	if resDefault.Envelope == nil {
		t.Fatal("expected non-nil Envelope on parse error fallback")
	}
	if resDefault.Envelope.FallbackReason != serialize.FallbackReasonParseError {
		t.Errorf("expected FallbackReason %q, got %q", serialize.FallbackReasonParseError, resDefault.Envelope.FallbackReason)
	}
	if resDefault.Envelope.FallbackAnnotation != serialize.FallbackAnnotationParseError {
		t.Errorf("expected FallbackAnnotation %q, got %q", serialize.FallbackAnnotationParseError, resDefault.Envelope.FallbackAnnotation)
	}
	if resDefault.SrcAST != nil || resDefault.DstAST != nil {
		t.Errorf("expected SrcAST and DstAST to be nil on parse error fallback, got src=%v, dst=%v", resDefault.SrcAST, resDefault.DstAST)
	}

	// Tolerated errors should keep AST matching enabled
	resAllowed, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 5, EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(5) failed: %v", err)
	}
	if resAllowed.MatchResult == nil {
		t.Error("expected structural AST match result when limit is 5")
	}
	if resAllowed.Envelope != nil && (resAllowed.Envelope.FallbackReason != "" || resAllowed.Envelope.FallbackAnnotation != "") {
		t.Errorf("expected empty FallbackReason and FallbackAnnotation when errors allowed, got reason=%q, annotation=%q",
			resAllowed.Envelope.FallbackReason, resAllowed.Envelope.FallbackAnnotation)
	}
}

func TestRenderDiffResult_UnifiedPreservesFallbackAnnotationWhenShowBanner(t *testing.T) {
	src := []byte("line1\nline2\n")
	dst := []byte("line1\nline3\n")
	env := &serialize.Envelope{
		FallbackReason:     serialize.FallbackReasonParseError,
		FallbackAnnotation: serialize.FallbackAnnotationParseError,
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
			{LeftLine: 1, RightLine: -1},
			{LeftLine: -1, RightLine: 1},
		},
		Actions: []serialize.Action{
			{Action: "delete", Node: &serialize.NodeRef{Tree: "before", StartByte: 6, EndByte: 11}},
			{Action: "insert", Node: &serialize.NodeRef{Tree: "after", StartByte: 6, EndByte: 11}},
		},
	}
	dr := &pipeline.DiffResult{
		SrcBytes: src,
		DstBytes: dst,
		SrcFile:  "a.txt",
		DstFile:  "b.txt",
		Envelope: env,
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format:     "unified",
		showBanner: true, // Multi-file diff sets showBanner to true
		writer:     &buf,
		unifiedOpts: unified.RenderOptions{
			Color:        false,
			ContextLines: 3,
		},
	}

	if err := renderDiffResult("a.txt", "b.txt", src, dst, dr, rc); err != nil {
		t.Fatalf("renderDiffResult failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, serialize.FallbackAnnotationParseError) {
		t.Errorf("expected unified output to preserve fallback annotation when showBanner=true, got:\n%s", out)
	}
}

func TestIsFileOrDevNull(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.go")
	_ = os.WriteFile(tmpFile, []byte("package main\n"), 0o644)

	if !isFileOrDevNull(tmpFile) {
		t.Errorf("isFileOrDevNull(%q) = false, want true", tmpFile)
	}
	if !isFileOrDevNull(os.DevNull) {
		t.Errorf("isFileOrDevNull(%q) = false, want true", os.DevNull)
	}
	if isFileOrDevNull("/path/does/not/exist/surely.go") {
		t.Errorf("isFileOrDevNull(nonexistent) = true, want false")
	}
}

func TestDumpFiles(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	fileB := filepath.Join(dir, "b.go")

	if err := os.WriteFile(fileA, []byte("package main\n\nfunc A() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("package main\n\nfunc B() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// 1. Single file AST dump
	var bufA bytes.Buffer
	if err := dumpFiles(&bufA, []string{fileA}, false); err != nil {
		t.Fatalf("dumpFiles single AST failed: %v", err)
	}
	outA := bufA.String()
	if !strings.Contains(outA, "source_file") || !strings.Contains(outA, "identifier: \"A\"") {
		t.Errorf("unexpected single AST output:\n%s", outA)
	}

	// 2. Single file CST dump
	var bufCST bytes.Buffer
	if err := dumpFiles(&bufCST, []string{fileA}, true); err != nil {
		t.Fatalf("dumpFiles single CST failed: %v", err)
	}
	outCST := bufCST.String()
	if !strings.Contains(outCST, "source_file") || !strings.Contains(outCST, "package") {
		t.Errorf("unexpected single CST output:\n%s", outCST)
	}

	// 3. Multiple files dump with headers
	var bufMulti bytes.Buffer
	if err := dumpFiles(&bufMulti, []string{fileA, fileB}, false); err != nil {
		t.Fatalf("dumpFiles multi AST failed: %v", err)
	}
	outMulti := bufMulti.String()
	if !strings.Contains(outMulti, "=== "+fileA+" ===") || !strings.Contains(outMulti, "=== "+fileB+" ===") {
		t.Errorf("expected file headers in multi dump output, got:\n%s", outMulti)
	}
	if !strings.Contains(outMulti, "identifier: \"A\"") || !strings.Contains(outMulti, "identifier: \"B\"") {
		t.Errorf("expected both file ASTs in multi dump output, got:\n%s", outMulti)
	}
}

func TestCSTZeroArgsError(t *testing.T) {
	if os.Getenv("DIFFM_TEST_CST_ZERO_ARGS") == "1" {
		rootCmd.SetArgs([]string{"--cst"})
		_ = rootCmd.Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestCSTZeroArgsError$")
	cmd.Env = append(os.Environ(), "DIFFM_TEST_CST_ZERO_ARGS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected command to exit with error, but succeeded. Output: %s", string(out))
	}
	want := "Error: --cst requires at least one file argument"
	if !strings.Contains(string(out), want) {
		t.Errorf("output = %q, want substring %q", string(out), want)
	}
}

func TestParseTreeZeroArgsError(t *testing.T) {
	if os.Getenv("DIFFM_TEST_PARSE_TREE_ZERO_ARGS") == "1" {
		rootCmd.SetArgs([]string{"--parse-tree"})
		_ = rootCmd.Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestParseTreeZeroArgsError$")
	cmd.Env = append(os.Environ(), "DIFFM_TEST_PARSE_TREE_ZERO_ARGS=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected command to exit with error, but succeeded. Output: %s", string(out))
	}
	want := "Error: --parse-tree requires at least one file argument"
	if !strings.Contains(string(out), want) {
		t.Errorf("output = %q, want substring %q", string(out), want)
	}
}

func TestDumpFilesLanguageDetectionError(t *testing.T) {
	dir := t.TempDir()
	unknownFile := filepath.Join(dir, "file.unsupported_ext")
	if err := os.WriteFile(unknownFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	err := dumpFiles(&buf, []string{unknownFile}, false)
	if err == nil {
		t.Fatal("expected error for unsupported language, got nil")
	}
	expectedPrefix := "detecting language for " + unknownFile
	if !strings.Contains(err.Error(), expectedPrefix) {
		t.Errorf("expected error containing %q, got: %v", expectedPrefix, err)
	}
}

func TestRunParseTree(t *testing.T) {
	resetRootCmdFlags()

	t.Run("ZeroArgsReturnsTrue", func(t *testing.T) {
		hadErrors := runParseTree(rootCmd, nil, true, false)
		if !hadErrors {
			t.Error("expected runParseTree to return true (hadErrors) for zero arguments")
		}
	})

	t.Run("NonExistentFileReturnsTrue", func(t *testing.T) {
		hadErrors := runParseTree(rootCmd, []string{"/non/existent/path/never_there.go"}, true, false)
		if !hadErrors {
			t.Error("expected runParseTree to return true (hadErrors) for non-existent file")
		}
	})

	t.Run("ValidFileReturnsFalse", func(t *testing.T) {
		dir := t.TempDir()
		file := filepath.Join(dir, "valid.go")
		if err := os.WriteFile(file, []byte("package main\n\nfunc Hello() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		hadErrors := runParseTree(rootCmd, []string{file}, true, false)
		if hadErrors {
			t.Error("expected runParseTree to return false for valid file")
		}
	})
}

func TestRunFileDiff(t *testing.T) {
	resetRootCmdFlags()
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	fileB := filepath.Join(dir, "b.go")
	if err := os.WriteFile(fileA, []byte("package main\n\nfunc Foo() { println(1) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fileB, []byte("package main\n\nfunc Foo() { println(2) }\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("NonExistentFileAReturnsTrue", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, "/non/existent/a.go", fileB, "", "", "side-by-side", false, 0, 0, 0, true)
		if !hadErrors {
			t.Error("expected runFileDiff to return true (hadErrors) when fileA does not exist")
		}
	})

	t.Run("NonExistentFileBReturnsTrue", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, fileA, "/non/existent/b.go", "", "", "side-by-side", false, 0, 0, 0, true)
		if !hadErrors {
			t.Error("expected runFileDiff to return true (hadErrors) when fileB does not exist")
		}
	})

	t.Run("IdenticalFilesReturnsFalse", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, fileA, fileA, "", "", "side-by-side", false, 0, 0, 0, true)
		if hadErrors {
			t.Error("expected runFileDiff to return false for identical files")
		}
	})

	t.Run("ValidSideBySideDiffReturnsFalse", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, fileA, fileB, "", "", "side-by-side", false, 0, 0, 0, true)
		if hadErrors {
			t.Error("expected runFileDiff to return false for valid side-by-side diff")
		}
	})

	t.Run("ValidUnifiedDiffReturnsFalse", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, fileA, fileB, "", "", "unified", false, 0, 0, 0, true)
		if hadErrors {
			t.Error("expected runFileDiff to return false for valid unified diff")
		}
	})

	t.Run("ValidDevNullDiffReturnsFalse", func(t *testing.T) {
		hadErrors := runFileDiff(rootCmd, os.DevNull, fileB, "", "", "side-by-side", false, 0, 0, 0, true)
		if hadErrors {
			t.Error("expected runFileDiff to return false when diffing /dev/null against valid file")
		}
	})
}

func TestRootCmdAutocompletion(t *testing.T) {
	if rootCmd.ValidArgsFunction == nil {
		t.Fatal("expected rootCmd.ValidArgsFunction to be defined")
	}

	candidates, directive := rootCmd.ValidArgsFunction(rootCmd, nil, "")
	if directive != cobra.ShellCompDirectiveDefault {
		t.Errorf("expected directive ShellCompDirectiveDefault, got: %d", directive)
	}
	if len(candidates) == 0 {
		t.Error("expected non-empty git ref completion candidates")
	}

	candidates1, directive1 := rootCmd.ValidArgsFunction(rootCmd, []string{"main"}, "")
	if directive1 != cobra.ShellCompDirectiveDefault {
		t.Errorf("expected directive ShellCompDirectiveDefault, got: %d", directive1)
	}
	if len(candidates1) == 0 {
		t.Error("expected non-empty git ref completion candidates for second arg")
	}

	candidates2, directive2 := rootCmd.ValidArgsFunction(rootCmd, []string{"main", "HEAD"}, "")
	if len(candidates2) != 0 {
		t.Errorf("expected no candidates when 2 args provided, got: %v", candidates2)
	}
	if directive2 != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("expected ShellCompDirectiveNoFileComp, got: %d", directive2)
	}
}

func TestFlagAutocompletion(t *testing.T) {
	formatFn, ok := rootCmd.GetFlagCompletionFunc("format")
	if !ok || formatFn == nil {
		t.Fatal("expected flag completion function for --format")
	}
	formats, dir := formatFn(rootCmd, nil, "")
	if len(formats) != 5 || dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("unexpected --format completions: %v, directive: %d", formats, dir)
	}
	if !slices.Contains(formats, "auto") {
		t.Errorf("expected --format completions to contain \"auto\", got %v", formats)
	}

	colorFn, ok := rootCmd.GetFlagCompletionFunc("color")
	if !ok || colorFn == nil {
		t.Fatal("expected flag completion function for --color")
	}
	colors, dir := colorFn(rootCmd, nil, "")
	if len(colors) != 3 || dir != cobra.ShellCompDirectiveNoFileComp {
		t.Errorf("unexpected --color completions: %v, directive: %d", colors, dir)
	}
}

func TestGitExternal7ArgsProtocol(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "old.go")
	fileB := filepath.Join(dir, "new.go")

	_ = os.WriteFile(fileA, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n"), 0o644)

	// Git passes: path old-file old-hex old-mode new-file new-hex new-mode
	displayPath := "sample.go"
	oldHex := "1234567890123456789012345678901234567890"
	newHex := "0987654321098765432109876543210987654321"
	oldMode := "100644"
	newMode := "100644"

	cmd := rootCmd
	defer resetRootCmdFlags()
	cmd.SetArgs([]string{"-f", "json", displayPath, fileA, oldHex, oldMode, fileB, newHex, newMode})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("7-arg diff.external execution failed: %v", err)
	}

	// Verify the underlying pipeline produces a real diff with actions.
	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}
	if len(res.Envelope.Actions) == 0 {
		t.Error("expected at least 1 action in diff between old.go and new.go, got 0")
	}
}

func TestGitExternal7ArgsFailFast(t *testing.T) {
	if os.Getenv("DIFFM_TEST_7ARG_FAILFAST") == "1" {
		rootCmd.SetArgs([]string{"sample.go", "nonexistent_file_a", "oldhex", "100644", "nonexistent_file_b", "newhex", "100644"})
		_ = rootCmd.Execute()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestGitExternal7ArgsFailFast$")
	cmd.Env = append(os.Environ(), "DIFFM_TEST_7ARG_FAILFAST=1")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected command to exit with error, but succeeded. Output: %s", string(out))
	}
	want := "Error: invalid old-file for external diff driver: nonexistent_file_a"
	if !strings.Contains(string(out), want) {
		t.Errorf("output = %q, want substring %q", string(out), want)
	}
}

func TestGitExternal7ArgsIdenticalBlobHashFastPath(t *testing.T) {
	// Git invokes diff.external on mode-only changes with identical hashes; make sure
	// we exit 0 immediately without touching disk.
	defer resetRootCmdFlags()
	hex := "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	rootCmd.SetArgs([]string{"sample.go", "nonexistent_a", hex, "100644", "nonexistent_b", hex, "100755"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected 7-arg identical blob hash fast-path to succeed, got: %v", err)
	}
}

func TestGitExternal8And9Args(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "old.go")
	fileB := filepath.Join(dir, "new.go")

	if err := os.WriteFile(fileA, []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", fileA, err)
	}
	if err := os.WriteFile(fileB, []byte("package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n"), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", fileB, err)
	}

	oldHex := "1234567890123456789012345678901234567890"
	newHex := "0987654321098765432109876543210987654321"
	oldMode := "100644"
	newMode := "100644"

	cmd := rootCmd
	defer resetRootCmdFlags()

	// 8-arg protocol (e.g. rename without xfrm_msg)
	cmd.SetArgs([]string{"-f", "json", "old.go", fileA, oldHex, oldMode, fileB, newHex, newMode, "new.go"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("8-arg diff.external execution failed: %v", err)
	}

	// 9-arg protocol (e.g. Lazygit diffing an untracked file against /dev/null with --no-index)
	cmd.SetArgs([]string{"-f", "json", "/dev/null", "/dev/null", ".", ".", fileB, "0000000000000000000000000000000000000000", "100644", "new.go", "index 000000000..d8cead7bb\n"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("9-arg diff.external execution (untracked /dev/null) failed: %v", err)
	}

	// Added file (/dev/null source)
	cmd.SetArgs([]string{"-f", "json", "new.go", "/dev/null", ".", ".", fileB, newHex, newMode})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("7-arg diff.external execution (added file) failed: %v", err)
	}

	// Deleted file (/dev/null destination)
	cmd.SetArgs([]string{"-f", "json", "old.go", fileA, oldHex, oldMode, "/dev/null", ".", "."})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("7-arg diff.external execution (deleted file) failed: %v", err)
	}
}

func TestRunFileDiffIdenticalBytesFastPath(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	fileB := filepath.Join(dir, "b.go")

	content := []byte("package main\n\nfunc main() {}\n")
	_ = os.WriteFile(fileA, content, 0o644)
	_ = os.WriteFile(fileB, content, 0o644)

	defer resetRootCmdFlags()

	rootCmd.SetArgs([]string{"--no-pager", fileA, fileB})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected identical files diff to succeed, got: %v", err)
	}

	rootCmd.SetArgs([]string{"--no-pager", fileA, fileA})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("expected same file diff to succeed, got: %v", err)
	}

	// Verify via pipeline that identical files produce no diff actions.
	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}
	if len(res.Envelope.Actions) != 0 {
		t.Errorf("expected 0 actions for identical files, got %d", len(res.Envelope.Actions))
	}
}

func TestValidateNoMisplacedFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name: "valid positional args",
			args: []string{"main", "cmd/diff_test.go"},
		},
		{
			name: "stdin symbol",
			args: []string{"-"},
		},
		{
			name:    "flag with value after separator",
			args:    []string{"main", "cmd/diff_test.go", "--color=always"},
			wantErr: `flag "--color=always" cannot be placed after '--'`,
		},
		{
			name:    "short flag after separator",
			args:    []string{"main", "cmd/diff_test.go", "-f", "sbs"},
			wantErr: `flag "-f" cannot be placed after '--'`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNoMisplacedFlags(tt.args)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateNoMisplacedFlags(%v) error = %v, want nil", tt.args, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateNoMisplacedFlags(%v) error = %v, want substring %q", tt.args, err, tt.wantErr)
			}
		})
	}

	t.Run("existing file starting with dash", func(t *testing.T) {
		dashFile := filepath.Join(t.TempDir(), "-testfile.go")
		if err := os.WriteFile(dashFile, []byte("package main\n"), 0o644); err != nil {
			t.Fatalf("failed to write temp file: %v", err)
		}
		if err := validateNoMisplacedFlags([]string{dashFile}); err != nil {
			t.Errorf("validateNoMisplacedFlags(%v) error = %v, want nil", dashFile, err)
		}
	})
}
