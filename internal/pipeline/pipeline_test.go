package pipeline

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestPipeline_LineLimitFallback(t *testing.T) {
	// Build a small Go file with 12 lines
	var lines []string
	for i := 0; i < 12; i++ {
		lines = append(lines, "package main")
	}
	content := []byte(strings.Join(lines, "\n"))

	// With maxLines=10 and 12-line file, it should fall back to line diff (no AST).
	res, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SrcAST != nil || res.DstAST != nil {
		t.Errorf("expected AST nil on line limit fallback, got non-nil")
	}
	if res.Envelope == nil {
		t.Fatal("expected non-nil Envelope on line limit fallback")
	}
	if res.Envelope.FallbackReason != serialize.FallbackReasonLineLimit {
		t.Errorf("expected FallbackReason %q, got %q", serialize.FallbackReasonLineLimit, res.Envelope.FallbackReason)
	}
	expectedLineAnnotation := serialize.FormatFallbackAnnotationLineLimit(12)
	if res.Envelope.FallbackAnnotation != expectedLineAnnotation {
		t.Errorf("expected FallbackAnnotation %q, got %q", expectedLineAnnotation, res.Envelope.FallbackAnnotation)
	}

	// With DisableLineLimit=true, AST should parse regardless.
	resUnlimited, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines:  10,
		DisableLineLimit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error with disabled limit: %v", err)
	}
	if resUnlimited.SrcAST == nil {
		t.Errorf("expected AST parsed when line limit disabled")
	}

	// With higher limit of 20, file has fewer lines - should parse as AST.
	resHigher, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error with higher limit: %v", err)
	}
	if resHigher.SrcAST == nil {
		t.Errorf("expected AST parsed when file is under line limit")
	}
}

func TestPipeline_FileSizeLimitFallback(t *testing.T) {
	content := []byte(strings.Repeat("// Comment line in go file\n", 80))

	res, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileSize: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SrcAST != nil || res.DstAST != nil {
		t.Errorf("expected AST nil on file size limit fallback, got non-nil")
	}
	if res.Envelope == nil {
		t.Fatal("expected non-nil Envelope on file size fallback")
	}
	if res.Envelope.FallbackReason != serialize.FallbackReasonFileSizeLimit {
		t.Errorf("expected FallbackReason %q, got %q", serialize.FallbackReasonFileSizeLimit, res.Envelope.FallbackReason)
	}
	expectedAnnotation := serialize.FormatFallbackAnnotationSizeLimit(len(content))
	if res.Envelope.FallbackAnnotation != expectedAnnotation {
		t.Errorf("expected FallbackAnnotation %q, got %q", expectedAnnotation, res.Envelope.FallbackAnnotation)
	}

	resUnlimited, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileSize:   1000,
		DisableSizeLimit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error with disabled size limit: %v", err)
	}
	if resUnlimited.SrcAST == nil {
		t.Errorf("expected AST parsed when size limit disabled")
	}

	// Large non-AST files (like .txt) shouldn't show limit banners.
	resUnsupported, err := Run(content, content, "README.txt", "README.txt", DiffOptions{
		MaxASTFileSize: 1000,
	})
	if err != nil {
		t.Fatalf("unexpected error for unsupported large file: %v", err)
	}
	if resUnsupported.Envelope.FallbackReason != serialize.FallbackReasonUnsupportedLanguage {
		t.Errorf("expected FallbackReason %q for large unsupported file, got %q",
			serialize.FallbackReasonUnsupportedLanguage, resUnsupported.Envelope.FallbackReason)
	}
	if resUnsupported.Envelope.FallbackAnnotation != "" {
		t.Errorf("expected empty FallbackAnnotation for unsupported large file, got %q",
			resUnsupported.Envelope.FallbackAnnotation)
	}
}

func TestPipeline_ConflictFallback(t *testing.T) {
	conflictContent := []byte("<<<<<<<\nfoo\n=======\nbar\n>>>>>>>\n")
	res, err := Run(conflictContent, conflictContent, "main.go", "main.go", DiffOptions{})
	if err != nil {
		t.Fatalf("unexpected error on conflict file: %v", err)
	}
	if res.SrcAST != nil || res.DstAST != nil {
		t.Errorf("expected AST nil on conflict fallback, got non-nil")
	}
	if res.Envelope == nil {
		t.Fatal("expected non-nil Envelope on conflict fallback")
	}
	if res.Envelope.FallbackReason != serialize.FallbackReasonConflict {
		t.Errorf("expected FallbackReason %q, got %q", serialize.FallbackReasonConflict, res.Envelope.FallbackReason)
	}
	if res.Envelope.FallbackAnnotation != serialize.FallbackAnnotationConflict {
		t.Errorf("expected FallbackAnnotation %q, got %q", serialize.FallbackAnnotationConflict, res.Envelope.FallbackAnnotation)
	}
}

func TestPipeline_UnchangedCommentsRetainedOnLineShift(t *testing.T) {
	src := []byte(`package main

// Foo does foo things.
func Foo() {
	// Step 1: initialize
	x := 1
	_ = x
}

// Bar does bar things.
func Bar() {
	// Step 2: finalize
	y := 2
	_ = y
}
`)

	// dst inserts a 30-line function between Foo and Bar to trigger line distance > 25
	var filler []string
	filler = append(filler, "// NewFunction does extra work.", "func NewFunction() {")
	for i := 0; i < 28; i++ {
		filler = append(filler, "\tz := 0\n\t_ = z")
	}
	filler = append(filler, "}")

	dst := []byte(`package main

// Foo does foo things.
func Foo() {
	// Step 1: initialize
	x := 1
	_ = x
}

` + strings.Join(filler, "\n") + `

// Bar does bar things.
func Bar() {
	// Step 2: finalize
	y := 2
	_ = y
}
`)

	res, err := Run(src, dst, "a.go", "b.go", DiffOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that comments in Foo and Bar are NOT deleted or inserted.
	for _, act := range res.Envelope.Actions {
		if act.Node != nil && act.Node.Type == "comment" {
			if strings.Contains(act.Node.Label, "Foo does foo") ||
				strings.Contains(act.Node.Label, "Step 1") ||
				strings.Contains(act.Node.Label, "Bar does bar") ||
				strings.Contains(act.Node.Label, "Step 2") {
				t.Errorf("spurious comment action %s on %q", act.Action, act.Node.Label)
			}
		}
	}
}

func toTestSpans(spans []serialize.HighlightSpan) []testutil.Span {
	res := make([]testutil.Span, len(spans))
	for i, s := range spans {
		res[i] = testutil.Span{
			Line:       s.Line,
			StartCol:   s.StartCol,
			EndCol:     s.EndCol,
			Action:     s.Action,
			ColorIndex: s.ColorIndex,
		}
	}
	return res
}

func TestPipeline_ParseErrorFallbackBoundary(t *testing.T) {
	validGo := []byte("package main\n\nfunc main() {\n\tx := 10\n}\n")
	brokenGo := []byte("package main\n\nfunc main() {\n\t@@@\n}\n")
	plainText := []byte("This is plain text documentation.\nNot Go code at all.\n")

	t.Run("SyntaxErrorTriggersParseErrorFallback", func(t *testing.T) {
		res, err := Run(validGo, brokenGo, "a.go", "b.go", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult != nil {
			t.Error("expected MatchResult to be nil on parse error fallback")
		}
		if res.SrcAST != nil || res.DstAST != nil {
			t.Errorf("expected SrcAST and DstAST to be nil on fallback, got src=%v, dst=%v", res.SrcAST, res.DstAST)
		}
		if res.Envelope.FallbackReason != serialize.FallbackReasonParseError {
			t.Errorf("expected FallbackReason %q, got %q", serialize.FallbackReasonParseError, res.Envelope.FallbackReason)
		}
		if res.Envelope.FallbackAnnotation != serialize.FallbackAnnotationParseError {
			t.Errorf("expected FallbackAnnotation %q, got %q", serialize.FallbackAnnotationParseError, res.Envelope.FallbackAnnotation)
		}
	})

	t.Run("AsymmetricUnsupportedVsSupportedDoesNotTriggerParseError", func(t *testing.T) {
		res, err := Run(plainText, validGo, "README.txt", "main.go", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult != nil {
			t.Error("expected MatchResult to be nil on cross-language diff")
		}
		if res.Envelope.FallbackReason != serialize.FallbackReasonUnsupportedLanguage {
			t.Errorf("expected FallbackReason %q on asymmetric diff, got %q",
				serialize.FallbackReasonUnsupportedLanguage, res.Envelope.FallbackReason)
		}
		if res.Envelope.FallbackAnnotation != "" {
			t.Errorf("expected empty FallbackAnnotation on asymmetric diff, got annotation=%q", res.Envelope.FallbackAnnotation)
		}
	})

	t.Run("BothUnsupportedLanguages", func(t *testing.T) {
		res, err := Run(plainText, plainText, "README.txt", "CHANGES.txt", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult != nil {
			t.Error("expected MatchResult to be nil on unsupported language diff")
		}
		if res.Envelope.FallbackReason != serialize.FallbackReasonUnsupportedLanguage {
			t.Errorf("expected FallbackReason %q on unsupported language diff, got %q",
				serialize.FallbackReasonUnsupportedLanguage, res.Envelope.FallbackReason)
		}
		if res.Envelope.FallbackAnnotation != "" {
			t.Errorf("expected empty FallbackAnnotation on unsupported language diff, got annotation=%q", res.Envelope.FallbackAnnotation)
		}
	})

	t.Run("MismatchedLanguagesDoNotTriggerParseError", func(t *testing.T) {
		pyCode := []byte("def hello():\n    print('hi')\n")
		res, err := Run(validGo, pyCode, "main.go", "script.py", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult != nil {
			t.Error("expected MatchResult to be nil on mismatched language diff")
		}
		if res.Envelope.FallbackReason != serialize.FallbackReasonMismatchedLanguages {
			t.Errorf("expected FallbackReason %q on mismatched language diff, got %q",
				serialize.FallbackReasonMismatchedLanguages, res.Envelope.FallbackReason)
		}
		if res.Envelope.FallbackAnnotation != "" {
			t.Errorf("expected empty FallbackAnnotation on mismatched language diff, got annotation=%q", res.Envelope.FallbackAnnotation)
		}
	})

	t.Run("EmptyFileInheritsPeerLanguageForValidCode", func(t *testing.T) {
		res, err := Run([]byte{}, validGo, "/dev/null", "main.go", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult == nil {
			t.Error("expected AST MatchResult when new file has valid code")
		}
		if res.Envelope.FallbackReason != "" {
			t.Errorf("expected empty FallbackReason for valid new file, got %q", res.Envelope.FallbackReason)
		}
	})

	t.Run("EmptyFileInheritsPeerLanguageForBrokenCode", func(t *testing.T) {
		res, err := Run([]byte{}, brokenGo, "/dev/null", "main.go", DiffOptions{ParseErrorLimit: 0})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult != nil {
			t.Error("expected MatchResult to be nil on broken new file")
		}
		if res.Envelope.FallbackReason != serialize.FallbackReasonParseError {
			t.Errorf("expected FallbackReason %q on broken new file, got %q", serialize.FallbackReasonParseError, res.Envelope.FallbackReason)
		}
	})

	t.Run("MissingTokenRecoveryRetainsStructuralDiff", func(t *testing.T) {
		oldC := []byte(`void logger(const char *msg) {
    (void)msg;
}

int main(void) {
    int total = 0;
    FOR_ALL_TAB_WINDOWS(tp, wp) {
        total += 10;
    }
    return total;
}
`)
		newC := []byte(`void logger(const char *msg) {
    (void)msg;
}

int main(void) {
    int total = 0;
    FOR_ALL_TAB_WINDOWS(tp, wp) {
        total += 20;
    }
    return total;
}
`)
		res, err := Run(oldC, newC, "old.c", "new.c", DiffOptions{
			ParseErrorLimit: 0,
			EnvelopeOpts: serialize.EnvelopeOptions{
				IncludeHighlights: true,
				IncludeActions:    true,
			},
		})
		if err != nil {
			t.Fatalf("unexpected Run error: %v", err)
		}
		if res.MatchResult == nil {
			t.Fatal("expected non-nil MatchResult for C macro with missing tokens under ParseErrorLimit=0")
		}
		if res.Envelope.FallbackReason != "" {
			t.Errorf("expected empty FallbackReason, got %q", res.Envelope.FallbackReason)
		}
		if res.Envelope.FallbackAnnotation != "" {
			t.Errorf("expected empty FallbackAnnotation, got %q", res.Envelope.FallbackAnnotation)
		}

		// Whole-snippet verification:
		srcMappings := res.MatchResult.Mappings.Src()

		// 1. Enclosing context: main function definitions must map in place
		testutil.AssertMappedText(t, res.SrcAST, res.DstAST, oldC, newC, srcMappings,
			"int main(void)",
			"int main(void)",
			"function_definition",
		)

		// 2. Surrounding statements: macro call and adjacent statements must map in place
		testutil.AssertMappedText(t, res.SrcAST, res.DstAST, oldC, newC, srcMappings,
			"FOR_ALL_TAB_WINDOWS(tp, wp)",
			"FOR_ALL_TAB_WINDOWS(tp, wp)",
			"call_expression",
		)

		// 3. Highlights: only '10' -> '20' should take an update highlight
		testutil.AssertTagged(t, string(oldC), toTestSpans(res.Envelope.LeftHighlights), `void logger(const char *msg) {
    (void)msg;
}

int main(void) {
    int total = 0;
    FOR_ALL_TAB_WINDOWS(tp, wp) {
        total += <upd>10</upd>;
    }
    return total;
}
`)
		testutil.AssertTagged(t, string(newC), toTestSpans(res.Envelope.RightHighlights), `void logger(const char *msg) {
    (void)msg;
}

int main(void) {
    int total = 0;
    FOR_ALL_TAB_WINDOWS(tp, wp) {
        total += <upd>20</upd>;
    }
    return total;
}
`)
	})
}

func TestPipeline_CommentTriviaMoveHighlighting(t *testing.T) {
	oldSrc := `package main

// CalculateTax computes order tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`
	newSrc := `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

// CalculateTax computes order tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
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
	env := res.Envelope

	// Only the function should show as moved, without extra comment actions.
	if len(env.Actions) != 1 {
		t.Fatalf("expected exactly 1 action in env.Actions, got %d: %+v", len(env.Actions), env.Actions)
	}
	if env.Actions[0].Action != "move" {
		t.Errorf("expected move action, got %s", env.Actions[0].Action)
	}

	// Both sides of the attached comment should highlight as moved.
	testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

<mov>// CalculateTax computes order tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`)
	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

<mov>// CalculateTax computes order tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>
`)

	// Line alignments should stay strictly monotonic.
	lastLeft := -1
	lastRight := -1
	for _, pair := range env.LineAlignment {
		if pair.LeftLine != -1 && pair.RightLine != -1 {
			if pair.LeftLine < lastLeft || pair.RightLine < lastRight {
				t.Errorf("detected alignment inversion: pair (%d, %d) preceded by (%d, %d)",
					pair.LeftLine, pair.RightLine, lastLeft, lastRight)
			}
			lastLeft = pair.LeftLine
			lastRight = pair.RightLine
		}
	}
}

func TestPipeline_ForceBinaryAndForceText(t *testing.T) {
	t.Run("ForceBinaryOverridesPlainText", func(t *testing.T) {
		src := []byte("package main\n\nfunc A() int { return 1 }\n")
		dst := []byte("package main\n\nfunc A() int { return 2 }\n")

		res, err := Run(src, dst, "a.go", "b.go", DiffOptions{
			ForceBinary: true,
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if !res.IsBinary {
			t.Errorf("expected res.IsBinary to be true with ForceBinary: true")
		}
		if res.Envelope == nil || !res.Envelope.IsBinary {
			t.Errorf("expected res.Envelope.IsBinary to be true")
		}
	})

	t.Run("ForceTextOverridesNULBytes", func(t *testing.T) {
		src := []byte("hello\x00world\nline 2\n")
		dst := []byte("hello\x00world\nline 2 modified\n")

		resDefault, err := Run(src, dst, "data.bin", "data.bin", DiffOptions{})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if !resDefault.IsBinary {
			t.Errorf("expected default NUL buffer to be detected as binary")
		}

		resForceText, err := Run(src, dst, "data.bin", "data.bin", DiffOptions{
			ForceText: true,
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}
		if resForceText.IsBinary {
			t.Errorf("expected ForceText: true to bypass binary detection")
		}
		if resForceText.Envelope == nil || resForceText.Envelope.IsBinary {
			t.Errorf("expected envelope not to be binary with ForceText: true")
		}
	})
}

func TestPipelineVisual_DeletedImportCommentAnchor(t *testing.T) {
	t.Run("Rust_DocCommentAboveDeletedImport", func(t *testing.T) {
		src := `/// This module applies rewrite rules to the command line options.
/// 1. Express deprecated usages in the new non-deprecated form.
use std::process;

use structopt::clap;
use crate::cli;
`
		dst := `/// This module applies rewrite rules to the command line options.
/// 1. Express deprecated usages in the new non-deprecated form.
use structopt::clap;
use crate::cli;
`
		res, err := Run([]byte(src), []byte(dst), "test.rs", "test.rs", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `/// This module applies rewrite rules to the command line options.
/// 1. Express deprecated usages in the new non-deprecated form.
<del>use std::process;</del>

use structopt::clap;
use crate::cli;
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `/// This module applies rewrite rules to the command line options.
/// 1. Express deprecated usages in the new non-deprecated form.
use structopt::clap;
use crate::cli;
`)
	})

	t.Run("Go_GroupedImportCommentAboveDeletedSpec", func(t *testing.T) {
		src := `package main

import (
	// Format and I/O utilities
	"fmt"
	"io"
	"os"
)

func main() {
	_ = io.EOF
	_ = os.Args
}
`
		dst := `package main

import (
	// Format and I/O utilities
	"io"
	"os"
)

func main() {
	_ = io.EOF
	_ = os.Args
}
`
		res, err := Run([]byte(src), []byte(dst), "test.go", "test.go", DiffOptions{
			EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
		})
		if err != nil {
			t.Fatalf("Run failed: %v", err)
		}

		testutil.AssertTagged(t, src, toTestSpans(res.Envelope.LeftHighlights), `package main

import (
	// Format and I/O utilities
	<del>"fmt"</del>
	"io"
	"os"
)

func main() {
	_ = io.EOF
	_ = os.Args
}
`)
		testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `package main

import (
	// Format and I/O utilities
	"io"
	"os"
)

func main() {
	_ = io.EOF
	_ = os.Args
}
`)
	})
}
