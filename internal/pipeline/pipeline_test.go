package pipeline

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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
	if len(res.Envelope.LeftHighlights) != 1 {
		t.Fatalf("expected 1 left highlight, got %d: %+v", len(res.Envelope.LeftHighlights), res.Envelope.LeftHighlights)
	}
	hLeft := res.Envelope.LeftHighlights[0]
	if hLeft.Line != 2 || hLeft.Action != "delete" || hLeft.StartCol != 8 || hLeft.EndCol != 10 {
		t.Errorf("expected delete [8..10] on line 2, got: %+v", hLeft)
	}

	if len(res.Envelope.RightHighlights) != 1 {
		t.Fatalf("expected 1 right highlight, got %d: %+v", len(res.Envelope.RightHighlights), res.Envelope.RightHighlights)
	}
	hRight := res.Envelope.RightHighlights[0]
	if hRight.Line != 2 || hRight.Action != "insert" || hRight.StartCol != 8 || hRight.EndCol != 13 {
		t.Errorf("expected insert [8..13] on line 2, got: %+v", hRight)
	}
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
	if len(res.Envelope.RightHighlights) != 2 {
		t.Fatalf("expected exactly 2 right highlights, got %d: %+v", len(res.Envelope.RightHighlights), res.Envelope.RightHighlights)
	}
	if res.Envelope.RightHighlights[0].Line != 4 || res.Envelope.RightHighlights[0].Action != "insert" {
		t.Errorf("expected insert highlight on line 4, got: %+v", res.Envelope.RightHighlights[0])
	}
	if res.Envelope.RightHighlights[1].Line != 6 || res.Envelope.RightHighlights[1].Action != "insert" {
		t.Errorf("expected insert highlight on line 6, got: %+v", res.Envelope.RightHighlights[1])
	}
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
	foundVarDel := slices.ContainsFunc(res.Envelope.LeftHighlights, func(h serialize.HighlightSpan) bool {
		return h.Line == 3 && h.Action == "delete" && h.StartCol == 1 && h.EndCol == 4
	})
	if !foundVarDel {
		t.Errorf("expected 'var' to be deleted on line 3, left highlights: %+v", res.Envelope.LeftHighlights)
	}
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
	foundSemicolonDel := slices.ContainsFunc(res.Envelope.LeftHighlights, func(h serialize.HighlightSpan) bool {
		return h.Line == 3 && h.StartCol == 34 && h.EndCol == 35 && h.Action == "delete"
	})
	if !foundSemicolonDel {
		t.Errorf("expected trailing semicolon on line 3 to be deleted, highlights: %+v", res.Envelope.LeftHighlights)
	}
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
		r := rules.Get("c")
		srcMappings := res.MatchResult.Mappings.Src()

		hasIdent := func(node *treesitter.ASTNode, name string) bool {
			return slices.ContainsFunc(node.Descendants(), func(d *treesitter.ASTNode) bool {
				return r.IsIdentifier(d.Type) && d.Label == name
			})
		}

		// 1. Enclosing context: main function definitions must map in place
		var mainOld, mainNew *treesitter.ASTNode
		for _, n := range res.SrcAST.Descendants() {
			if r.IsDeclaration(n.Type) && hasIdent(n, "main") {
				mainOld = n
				break
			}
		}
		for _, n := range res.DstAST.Descendants() {
			if r.IsDeclaration(n.Type) && hasIdent(n, "main") {
				mainNew = n
				break
			}
		}
		if mainOld == nil || mainNew == nil {
			t.Fatalf("expected to locate main function declarations in both ASTs")
		}
		if srcMappings[mainOld] != mainNew {
			t.Errorf("expected main function declaration to map in place across ASTs")
		}

		// 2. Surrounding statements: macro call and adjacent statements must map in place
		var macroCallOld, macroCallNew *treesitter.ASTNode
		for _, n := range mainOld.Descendants() {
			if r.IsCall(n.Type) && hasIdent(n, "FOR_ALL_TAB_WINDOWS") {
				macroCallOld = n
				break
			}
		}
		for _, n := range mainNew.Descendants() {
			if r.IsCall(n.Type) && hasIdent(n, "FOR_ALL_TAB_WINDOWS") {
				macroCallNew = n
				break
			}
		}
		if macroCallOld == nil || macroCallNew == nil {
			t.Fatalf("expected to find FOR_ALL_TAB_WINDOWS call in both ASTs")
		}
		if srcMappings[macroCallOld] != macroCallNew {
			t.Errorf("expected FOR_ALL_TAB_WINDOWS call to map in place")
		}

		// 3. Highlights: only '10' -> '20' should take an update highlight
		var leftUpdates, rightUpdates []serialize.HighlightSpan
		for _, h := range res.Envelope.LeftHighlights {
			if h.Action == "update" {
				leftUpdates = append(leftUpdates, h)
			}
		}
		for _, h := range res.Envelope.RightHighlights {
			if h.Action == "update" {
				rightUpdates = append(rightUpdates, h)
			}
		}
		if len(leftUpdates) != 1 || len(rightUpdates) != 1 {
			t.Errorf("expected exactly 1 update highlight on each side, got left=%v right=%v", leftUpdates, rightUpdates)
		}
		if len(leftUpdates) == 1 && (leftUpdates[0].Line != 7 || leftUpdates[0].StartCol != 17 || leftUpdates[0].EndCol != 19) {
			t.Errorf("unexpected left update highlight bounds: %+v", leftUpdates[0])
		}
		if len(rightUpdates) == 1 && (rightUpdates[0].Line != 7 || rightUpdates[0].StartCol != 17 || rightUpdates[0].EndCol != 19) {
			t.Errorf("unexpected right update highlight bounds: %+v", rightUpdates[0])
		}
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
	hasLeftCommentMove := false
	for _, h := range env.LeftHighlights {
		if h.Line == 2 && h.Action == "move" {
			hasLeftCommentMove = true
		}
	}
	if !hasLeftCommentMove {
		t.Errorf("expected left comment on line 2 to have 'move' highlight, got: %+v", env.LeftHighlights)
	}

	hasRightCommentMove := false
	for _, h := range env.RightHighlights {
		if h.Line == 14 && h.Action == "move" {
			hasRightCommentMove = true
		}
	}
	if !hasRightCommentMove {
		t.Errorf("expected right comment on line 14 to have 'move' highlight, got: %+v", env.RightHighlights)
	}

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
