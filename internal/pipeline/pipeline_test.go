package pipeline

import (
	"slices"
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
		testutil.AssertMapped(t, res.SrcAST, res.DstAST, srcMappings,
			testutil.NodeQuery{Type: "function_definition", Line: 5},
			testutil.NodeQuery{Type: "function_definition", Line: 5},
		)

		// 2. Surrounding statements: macro call and adjacent statements must map in place
		testutil.AssertMapped(t, res.SrcAST, res.DstAST, srcMappings,
			testutil.NodeQuery{Type: "call_expression", Line: 7},
			testutil.NodeQuery{Type: "call_expression", Line: 7},
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
    local <del>add_undefined</del> = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    return format(intermediate, add_undefined)
end
`)
	testutil.AssertTagged(t, dst, toTestSpans(res.Envelope.RightHighlights), `function build_node(declaration)
    local <ins>add_undefined_for_parameter</ins> = declaration and is_parameter(declaration)
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
