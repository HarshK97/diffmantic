package sidebyside

import (
	"bytes"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

func TestSliceLineToColumn_ASCII_And_Padding(t *testing.T) {
	scratch := &RenderScratch{}
	var buf bytes.Buffer

	err := scratch.SliceLineToColumn("hello world", "", nil, 20, renderutil.LineContextAligned, true, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "hello world         "
	if got != want {
		t.Errorf("SliceLineToColumn ASCII: got %q, want %q", got, want)
	}
	if len(got) != 20 {
		t.Errorf("expected width 20, got %d", len(got))
	}
}

func TestSliceLineToChunks_Wrapping(t *testing.T) {
	scratch := &RenderScratch{}

	// 1. Function signature wrapping without mid-word splits
	line := "func ComputeHash(seed uint32) int64"
	chunks := scratch.SliceLineToChunks(line, "", nil, 25, renderutil.LineContextAligned, true, false)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	chunk0 := string(chunks[0])
	chunk1 := string(chunks[1])

	if runewidth.StringWidth(chunk0) != 25 {
		t.Errorf("chunk0 width = %d, want 25: %q", runewidth.StringWidth(chunk0), chunk0)
	}
	if runewidth.StringWidth(chunk1) != 25 {
		t.Errorf("chunk1 width = %d, want 25: %q", runewidth.StringWidth(chunk1), chunk1)
	}

	if !strings.HasPrefix(chunk0, "func ComputeHash(seed") {
		t.Errorf("chunk0 = %q, want prefix 'func ComputeHash(seed'", chunk0)
	}
	if !strings.HasPrefix(chunk1, "uint32) int64") {
		t.Errorf("chunk1 = %q, want prefix 'uint32) int64'", chunk1)
	}

	// 2. Long comment wrapping cleanly at word boundaries
	comment := "// Dispatch sends a payload over HTTP with full context cancellation and retry logic."
	cChunks := scratch.SliceLineToChunks(comment, "", nil, 40, renderutil.LineContextAligned, true, false)
	for i, c := range cChunks {
		s := string(c)
		if runewidth.StringWidth(s) != 40 {
			t.Errorf("comment chunk %d width = %d, want 40: %q", i, runewidth.StringWidth(s), s)
		}
	}
	// "cancellation" and "logic." must remain whole words
	fullReconstructed := strings.TrimSpace(string(cChunks[0])) + " " + strings.TrimSpace(string(cChunks[1])) + " " + strings.TrimSpace(string(cChunks[2]))
	if !strings.Contains(fullReconstructed, "cancellation") || !strings.Contains(fullReconstructed, "logic.") {
		t.Errorf("words broken in reconstructed comment: %q", fullReconstructed)
	}

	// 3. Highlighted line wrapping across boundary retains insert color on chunk 1
	insLine := `_, err := fmt.Fprintf(w, "\x1b[38;2;%d;%d;%dmBinary files %s and %s differ\x1b[0m\n", yellowRGB[0])`
	spans := []serialize.HighlightSpan{
		{StartCol: 0, EndCol: len(insLine), Action: "insert"},
	}
	colorChunks := scratch.SliceLineToChunks(insLine, "", spans, 50, renderutil.LineContextStandaloneInsert, true, true)
	if len(colorChunks) < 2 {
		t.Fatalf("expected at least 2 color chunks, got %d", len(colorChunks))
	}
	expectedInsertSGR := color.InsertFg
	if !strings.Contains(string(colorChunks[1]), expectedInsertSGR) {
		t.Errorf("expected chunk 1 to contain insert color SGR code %q, got:\n%q", expectedInsertSGR, string(colorChunks[1]))
	}
}

func TestSliceLineToChunks_WideRune(t *testing.T) {
	scratch := &RenderScratch{}

	// "世界你好" (4 CJK runes, 8 display cols) with targetWidth 6
	// chunk0 gets 世(2)+界(4)+pad(2)=6 cols, chunk1 gets 你(2)+好(4)+pad(2)=6 cols
	chunks := scratch.SliceLineToChunks("世界你好", "", nil, 6, renderutil.LineContextAligned, true, false)
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks, got %d", len(chunks))
	}

	if runewidth.StringWidth(string(chunks[0])) != 6 {
		t.Errorf("chunk0 width = %d, want 6", runewidth.StringWidth(string(chunks[0])))
	}
	if runewidth.StringWidth(string(chunks[1])) != 6 {
		t.Errorf("chunk1 width = %d, want 6", runewidth.StringWidth(string(chunks[1])))
	}
}

func TestSliceLineToColumn_TabExpansion(t *testing.T) {
	scratch := &RenderScratch{}
	var buf bytes.Buffer

	// "\ta" -> tab expands to 4 spaces, 'a' at col 4 -> "    a" padded to 10
	err := scratch.SliceLineToColumn("\ta", "", nil, 10, renderutil.LineContextAligned, true, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	want := "    a     "
	if got != want {
		t.Errorf("SliceLineToColumn tab expansion: got %q, want %q", got, want)
	}
}

func TestSliceLineToColumn_CustomTabWidth(t *testing.T) {
	// TabWidth = 2: "\ta" -> 2 spaces + 'a' -> "  a       " (padded to 10)
	scratch2 := &RenderScratch{TabWidth: 2}
	var buf2 bytes.Buffer
	if err := scratch2.SliceLineToColumn("\ta", "", nil, 10, renderutil.LineContextAligned, true, false, &buf2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := buf2.String(), "  a       "; got != want {
		t.Errorf("TabWidth 2: got %q, want %q", got, want)
	}

	// TabWidth = 8: "\ta" -> 8 spaces + 'a' -> "        a " (padded to 10)
	scratch8 := &RenderScratch{TabWidth: 8}
	var buf8 bytes.Buffer
	if err := scratch8.SliceLineToColumn("\ta", "", nil, 10, renderutil.LineContextAligned, true, false, &buf8); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := buf8.String(), "        a "; got != want {
		t.Errorf("TabWidth 8: got %q, want %q", got, want)
	}
}

func TestSliceLineToColumn_MoveBadge(t *testing.T) {
	scratch := &RenderScratch{}
	var buf bytes.Buffer

	err := scratch.SliceLineToColumn("func Foo()", " ➔ L42", nil, 20, renderutil.LineContextAligned, true, false, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "➔ L42") {
		t.Errorf("expected badge '➔ L42', got %q", got)
	}
	if runewidth.StringWidth(got) != 20 {
		t.Errorf("expected total display width 20, got %d: %q", runewidth.StringWidth(got), got)
	}
}

func TestSliceLineToColumn_MoveBadge_MultiColor(t *testing.T) {
	scratch := &RenderScratch{}
	var buf bytes.Buffer

	// Slot 0 (Cyan): color.Move0Fg
	err := scratch.SliceLineToColumn("func Foo()", " ➔ L42", nil, 20, renderutil.LineContextAligned, true, true, &buf, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, color.Move0Fg+" ➔ L42"+color.Reset) {
		t.Errorf("expected badge with Move0Fg, got %q", got)
	}

	// Slot 1 (Magenta): color.Move1Fg
	buf.Reset()
	err = scratch.SliceLineToColumn("func Bar()", " ➔ L50", nil, 20, renderutil.LineContextAligned, true, true, &buf, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got = buf.String()
	if !strings.Contains(got, color.Move1Fg+" ➔ L50"+color.Reset) {
		t.Errorf("expected badge with Move1Fg, got %q", got)
	}
}

func TestRender_BasicSideBySide(t *testing.T) {
	src := "package main\n\nfunc A() int {\n\treturn 1\n}\n"
	dst := "package main\n\nfunc A() int {\n\treturn 2\n}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth:   100,
		ContextLines:    3,
		LineNumbers:     true,
		Color:           false,
		ForceSideBySide: true,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "return 1") || !strings.Contains(out, "return 2") {
		t.Errorf("expected both old and new code lines in output:\n%s", out)
	}
}

func TestRender_NarrowWidthFallback(t *testing.T) {
	src := "package main\n\nfunc A() int {\n\treturn 1\n}\n"
	dst := "package main\n\nfunc A() int {\n\treturn 2\n}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth:   60, // Narrow width (< 80) without force
		ContextLines:    3,
		LineNumbers:     true,
		Color:           false,
		ForceSideBySide: false,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render fallback failed: %v", err)
	}

	out := buf.String()
	// Narrow fallback emits unified diff with @@ headers
	if !strings.Contains(out, "@@") {
		t.Errorf("expected unified diff header '@@' on narrow fallback, got:\n%s", out)
	}
}

func TestRender_SideBySideFallbackAnnotation(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tx :=\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tx := 1\n}\n")

	env := &serialize.Envelope{
		FallbackReason:     serialize.FallbackReasonParseError,
		FallbackAnnotation: serialize.FallbackAnnotationParseError,
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
			{LeftLine: 1, RightLine: 1},
			{LeftLine: 2, RightLine: 2},
			{LeftLine: 3, RightLine: -1},
			{LeftLine: -1, RightLine: 3},
			{LeftLine: 4, RightLine: 4},
		},
		Actions: []serialize.Action{
			{Action: "delete", Node: &serialize.NodeRef{Tree: "before", StartByte: 29, EndByte: 35}},
			{Action: "insert", Node: &serialize.NodeRef{Tree: "after", StartByte: 29, EndByte: 37}},
		},
	}

	opts := RenderOptions{
		TerminalWidth:   120,
		ContextLines:    3,
		LineNumbers:     true,
		Color:           false,
		ForceSideBySide: true,
	}

	t.Run("DefaultEnabled", func(t *testing.T) {
		var buf bytes.Buffer
		err := Render("a.go", "b.go", src, dst, env, opts, &buf)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		out := buf.String()
		if !strings.Contains(out, "@@ -1,5 +1,5 @@ [parse error, falling back to line diff]") {
			t.Errorf("expected hunk header to contain fallback annotation, got:\n%s", out)
		}
	})

	t.Run("AnnotationsDisabled", func(t *testing.T) {
		optsDisabled := opts
		optsDisabled.DisableAnnotations = true
		var buf bytes.Buffer
		err := Render("a.go", "b.go", src, dst, env, optsDisabled, &buf)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		outDisabled := buf.String()
		if strings.Contains(outDisabled, "[parse error, falling back to line diff]") {
			t.Errorf("expected fallback annotation suppressed when DisableAnnotations=true, got:\n%s", outDisabled)
		}
	})

	t.Run("SuppressHunkFallbackAnnotation", func(t *testing.T) {
		optsSuppressed := opts
		optsSuppressed.SuppressHunkFallbackAnnotation = true
		var buf bytes.Buffer
		err := Render("a.go", "b.go", src, dst, env, optsSuppressed, &buf)
		if err != nil {
			t.Fatalf("Render failed: %v", err)
		}
		outSuppressed := buf.String()
		if strings.Contains(outSuppressed, "[parse error, falling back to line diff]") {
			t.Errorf("expected fallback annotation suppressed from hunk when SuppressHunkFallbackAnnotation=true, got:\n%s", outSuppressed)
		}
	})
}

func TestRender_DeclarationMove_CleanHunkHeaderWithLineBadges(t *testing.T) {
	src := []byte("package main\n\nfunc MovedFunc() {\n\tprintln(\"hello\")\n}\n\nfunc Anchor() {\n\tprintln(\"anchor\")\n}\n")
	dst := []byte("package main\n\nfunc Anchor() {\n\tprintln(\"anchor\")\n}\n\nfunc MovedFunc() {\n\tprintln(\"hello\")\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	err = Render("main.go", "main.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 1, LineNumbers: false}, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	got := buf.String()

	if strings.Contains(got, "moved to") || strings.Contains(got, "moved from") {
		t.Errorf("expected hunk headers to not contain 'moved to' or 'moved from', got:\n%s", got)
	}
	if !strings.Contains(got, "➔ L") {
		t.Errorf("expected line badge '➔ L' on moved source declaration, got:\n%s", got)
	}
	if !strings.Contains(got, "↳ L") {
		t.Errorf("expected line badge '↳ L' on moved destination declaration, got:\n%s", got)
	}
}

func TestRender_SideBySideDisableAnnotations_SuppressesMoveBadges(t *testing.T) {
	src := []byte("package main\n\nfunc MovedFunc() {\n\tprintln(\"hello\")\n}\n\nfunc Anchor() {\n\tprintln(\"anchor\")\n}\n")
	dst := []byte("package main\n\nfunc Anchor() {\n\tprintln(\"anchor\")\n}\n\nfunc MovedFunc() {\n\tprintln(\"hello\")\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{Color: false, ContextLines: 1, LineNumbers: false, DisableAnnotations: true}
	err = Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	got := buf.String()

	if strings.Contains(got, "➔") || strings.Contains(got, "↳") || strings.Contains(got, "moved") {
		t.Errorf("expected zero move annotations when DisableAnnotations=true, got:\n%s", got)
	}
}

func TestRenderFileBanner(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		insertions int
		deletions  int
		updates    int
		annotation string
		colorMode  bool
		want       string
	}{
		{
			name:       "with updates",
			path:       "pkg/auth/token.go",
			insertions: 12,
			deletions:  4,
			updates:    3,
			want:       "pkg/auth/token.go  [+12 -4 ~3]\n",
		},
		{
			name:       "without updates",
			path:       "pkg/auth/token.go",
			insertions: 28,
			deletions:  0,
			updates:    0,
			want:       "pkg/auth/token.go  [+28 -0]\n",
		},
		{
			name:       "binary files differ",
			path:       "pkg/auth/token.go",
			insertions: 0,
			deletions:  0,
			updates:    0,
			want:       "pkg/auth/token.go  [Binary files differ]\n",
		},
		{
			name:       "color mode",
			path:       "pkg/auth/token.go",
			insertions: 1,
			deletions:  2,
			updates:    0,
			colorMode:  true,
			want:       "\x1b[1m\x1b[1;94mpkg/auth/token.go\x1b[0m  [\x1b[32m+1\x1b[0m \x1b[31m-2\x1b[0m]\x1b[0m\n",
		},
		{
			name:       "with fallback annotation plain text",
			path:       "pkg/auth/token.go",
			insertions: 1,
			deletions:  2,
			updates:    0,
			annotation: serialize.FallbackAnnotationParseError,
			want:       "pkg/auth/token.go  [+1 -2]  [parse error, falling back to line diff]\n",
		},
		{
			name:       "with fallback annotation color mode",
			path:       "pkg/auth/token.go",
			insertions: 1,
			deletions:  2,
			updates:    0,
			annotation: serialize.FallbackAnnotationParseError,
			colorMode:  true,
			want:       "\x1b[1m\x1b[1;94mpkg/auth/token.go\x1b[0m  [\x1b[32m+1\x1b[0m \x1b[31m-2\x1b[0m]  \x1b[33m[parse error, falling back to line diff]\x1b[0m\x1b[0m\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := RenderFileBanner(tt.path, tt.insertions, tt.deletions, tt.updates, tt.annotation, tt.colorMode, &buf)
			if err != nil {
				t.Fatalf("RenderFileBanner() error = %v", err)
			}
			if got := buf.String(); got != tt.want {
				t.Errorf("RenderFileBanner() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountLineStats(t *testing.T) {
	src := "package main\n\nfunc A() int {\n\treturn 1\n}\n"
	dst := "package main\n\nfunc A() int {\n\treturn 2\n}\n\nfunc B() {}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	ins, del, upd := CountLineStats([]byte(src), []byte(dst), dr.Envelope)
	if ins == 0 && upd == 0 {
		t.Errorf("CountLineStats expected insertions and updates, got ins=%d, del=%d, upd=%d", ins, del, upd)
	}

	// Nil envelope safety
	ins0, del0, upd0 := CountLineStats(nil, nil, nil)
	if ins0 != 0 || del0 != 0 || upd0 != 0 {
		t.Errorf("CountLineStats on nil: got (%d, %d, %d), want (0, 0, 0)", ins0, del0, upd0)
	}
}

func TestRender_WrappingAndContinuationGutter(t *testing.T) {
	src := "package main\n\n// VeryLongOldCommentThatExceedsTheColumnWidthAndMustWrapCleanly\nfunc A() {}\n"
	dst := "package main\n\n// VeryLongNewCommentThatExceedsTheColumnWidthAndMustWrapCleanlyAcrossMultipleRows\nfunc A() {}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth:   90,
		ContextLines:    3,
		LineNumbers:     true,
		Color:           false,
		ForceSideBySide: true,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	// Must contain continuation gutter ".."
	if !strings.Contains(out, "..") {
		t.Errorf("expected continuation gutter '..' in wrapped output:\n%s", out)
	}
}

func TestRender_BinaryFile(t *testing.T) {
	src := []byte("PNG\x00\x00\x01\x02")
	dst := []byte("PNG\x00\x00\x01\x03")

	dr, err := pipeline.Run(src, dst, "a.png", "b.png", pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	if !dr.IsBinary {
		t.Fatal("expected IsBinary to be true for binary png data")
	}

	var buf bytes.Buffer
	opts := RenderOptions{Color: false}

	err = Render("a.png", "b.png", src, dst, dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed on binary: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Binary files a.png and b.png differ") {
		t.Errorf("expected binary message, got: %q", out)
	}
}

func TestRender_AdaptiveHybridLayout_MonolithicInsert(t *testing.T) {
	src := "package main\n\nfunc A() {\n\tprintln(\"hello\")\n}\n"
	dst := "package main\n\nfunc A() {\n\tprintln(\"hello\")\n}\n\nfunc NewBigFunction() error {\n\t// Line 1\n\t// Line 2\n\t// Line 3\n\t// Line 4\n\t// Line 5\n\t// Line 6\n\treturn nil\n}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth: 100,
		ContextLines:  2,
		LineNumbers:   true,
		Color:         false,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	// Pure addition hunk should render single-column without central divider
	if !strings.Contains(out, " ..   7 func NewBigFunction() error {") {
		t.Errorf("expected inserted function in single-column output with ' ..   7':\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_PairedModifications(t *testing.T) {
	src := "package main\n\nfunc A() {\n\tport := 8080\n\tprintln(port)\n}\n"
	dst := "package main\n\nfunc A() {\n\tport := 9090\n\tprintln(port)\n}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth: 100,
		ContextLines:  3,
		LineNumbers:   true,
		Color:         false,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "port := 8080") || !strings.Contains(out, "port := 9090") {
		t.Errorf("expected paired edit in output:\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_ForceSBS(t *testing.T) {
	src := "package main\n\nfunc A() {\n\tprintln(\"hello\")\n}\n"
	dst := "package main\n\nfunc A() {\n\tprintln(\"hello\")\n}\n\nfunc NewBigFunction() error {\n\t// Line 1\n\t// Line 2\n\t// Line 3\n\t// Line 4\n\t// Line 5\n\t// Line 6\n\treturn nil\n}\n"

	dr, err := pipeline.Run([]byte(src), []byte(dst), "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	var buf bytes.Buffer
	opts := RenderOptions{
		TerminalWidth:   100,
		ContextLines:    2,
		LineNumbers:     true,
		Color:           false,
		ForceSideBySide: true,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "NewBigFunction") {
		t.Errorf("expected NewBigFunction in output:\n%s", out)
	}
}

func TestRender_Hybrid_PureAdditionHunk(t *testing.T) {
	// A file where a function is added at the end
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n\nfunc B() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "@@") {
		t.Errorf("expected hunk header '@@', got:\n%s", out)
	}
	if !strings.Contains(out, " ..   7 func B() {") {
		t.Errorf("expected ' ..   7 func B() {', got:\n%s", out)
	}
}

func TestRender_Hybrid_PureDeletionHunk(t *testing.T) {
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n\nfunc B() {\n\treturn 2\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "@@") {
		t.Errorf("expected hunk header '@@', got:\n%s", out)
	}
	if !strings.Contains(out, "  7  .. func B() {") {
		t.Errorf("expected '  7  .. func B() {', got:\n%s", out)
	}
}

func TestRender_Hybrid_MixedHunk(t *testing.T) {
	// In-place edit has changes on both sides: renders dual-column SBS
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "@@") {
		t.Errorf("expected hunk header '@@', got:\n%s", out)
	}
	if !strings.Contains(out, "return 1") || !strings.Contains(out, "return 2") {
		t.Errorf("expected both sides in output:\n%s", out)
	}
}

func TestRender_Hybrid_ForceSideBySide(t *testing.T) {
	// Pure addition hunk, but ForceSideBySide is true
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n\nfunc B() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.ForceSideBySide = true
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "func B() {") {
		t.Errorf("expected func B() in output:\n%s", out)
	}
}

func TestRender_Hybrid_WholeFileAddition(t *testing.T) {
	src := []byte("")
	dst := []byte("package main\n\nfunc New() {\n\treturn\n}\n")

	dr, err := pipeline.Run(src, dst, "new.go", "new.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("new.go", "new.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "@@ -0,0 +1,6 @@") {
		t.Errorf("expected hunk header '@@ -0,0 +1,6 @@', got:\n%s", out)
	}
	if !strings.Contains(out, "  3 func New() {") {
		t.Errorf("expected single line number gutter '  3 func New() {', got:\n%s", out)
	}
}

func TestRender_Hybrid_WholeFileDeletion(t *testing.T) {
	src := []byte("package main\n\nfunc Old() {\n\treturn\n}\n")
	dst := []byte("")

	dr, err := pipeline.Run(src, dst, "old.go", "old.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("old.go", "old.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "@@ -1,6 +0,0 @@") {
		t.Errorf("expected hunk header '@@ -1,6 +0,0 @@', got:\n%s", out)
	}
	if !strings.Contains(out, "  3 func Old() {") {
		t.Errorf("expected single line number gutter '  3 func Old() {', got:\n%s", out)
	}
}

func TestRender_BoldLineNumbers_ColorMode(t *testing.T) {
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n\nfunc B() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	// 1. Single column hybrid with Color = true -> bold insert number
	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = true

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	out := buf.String()
	expectedBoldInsert := color.Bold + color.InsertFg
	if !strings.Contains(out, expectedBoldInsert) {
		t.Errorf("expected bold insert line number in single column, got:\n%s", out)
	}

	// 2. Dual column SBS with Color = true -> bold insert number in right gutter
	opts.ForceSideBySide = true
	var bufSBS bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &bufSBS); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	outSBS := bufSBS.String()
	if !strings.Contains(outSBS, expectedBoldInsert) {
		t.Errorf("expected bold insert line number in dual column SBS, got:\n%s", outSBS)
	}
}

func TestRender_Hybrid_NoLineNumbers_ShowsSymbols(t *testing.T) {
	src := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc A() {\n\treturn 1\n}\n\nfunc B() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.LineNumbers = false
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "+ func B() {") {
		t.Errorf("expected '+ func B() {' when line numbers are disabled, got:\n%s", out)
	}
}

func TestRender_CustomTabWidth(t *testing.T) {
	src := []byte("package main\n\nfunc Run() {\n\treturn 1\n}\n")
	dst := []byte("package main\n\nfunc Run() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	// With TabWidth = 2: "\treturn" has 2 spaces indent
	opts2 := DefaultOptions()
	opts2.TabWidth = 2
	opts2.Color = false
	opts2.ContextLines = 5

	var buf2 bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts2, &buf2); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	out2 := buf2.String()
	if !strings.Contains(out2, "  return") {
		t.Errorf("expected 2 spaces indent with TabWidth=2, got:\n%s", out2)
	}

	// With TabWidth = 8: "\treturn" has 8 spaces indent
	opts8 := DefaultOptions()
	opts8.TabWidth = 8
	opts8.Color = false
	opts8.ContextLines = 5

	var buf8 bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts8, &buf8); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	out8 := buf8.String()
	if !strings.Contains(out8, "        return") {
		t.Errorf("expected 8 spaces indent with TabWidth=8, got:\n%s", out8)
	}
}

func TestRender_SubBlockSideBySide(t *testing.T) {
	src := []byte("function OverviewPanel({ events,\n" +
		"\tregistrations, team, crew, partners,\n" +
		"\tsections, onToggleSection }: any) {\n" +
		"\tconst confirmedCount = (registrations || []).filter(\n" +
		"\t\t(r: any) => r.status === 'CONFIRMED' || r.status === 'ATTENDED',\n" +
		"\t).length\n" +
		"}\n")
	dst := []byte("function OverviewPanel({ events, team, crew, partners, sections, onToggleSection }: any) {\n" +
		"\tconst confirmedCount = (events || []).reduce((sum: number, e: any) => sum + (e.registrationCount || 0), 0)\n" +
		"}\n")

	dr, err := pipeline.Run(src, dst, "panel.ts", "panel.ts", pipeline.DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeActions:    true,
			IncludeAlignment:  true,
			IncludeHighlights: true,
		},
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := DefaultOptions()
	opts.TerminalWidth = 100
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("panel.ts", "panel.ts", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "filter(") || !strings.Contains(out, "reduce(") {
		t.Errorf("missing expected text in output:\n%s", out)
	}
}

func TestSliceLineToChunks_AlignedPairRetainsGranularHighlights(t *testing.T) {
	scratch := &RenderScratch{}
	line := "changeIntervals := buildChangeIntervals(isPairChanged)"
	// Spans only on "buildChangeIntervals" (cols 19..39)
	spans := []serialize.HighlightSpan{
		{StartCol: 19, EndCol: 39, Action: "delete"},
	}

	chunks := scratch.SliceLineToChunks(line, "", spans, 100, renderutil.LineContextAligned, false, true)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	got := string(chunks[0])
	// "changeIntervals := " must NOT be colored with DeleteFg
	delPrefix := color.DeleteFg + "changeIntervals"
	if strings.Contains(got, delPrefix) {
		t.Errorf("unspanned prefix should not take DeleteFg, got:\n%q", got)
	}
	// "buildChangeIntervals" must have DeleteFg
	delToken := color.DeleteFg + "buildChangeIntervals"
	if !strings.Contains(got, delToken) {
		t.Errorf("spanned token should take DeleteFg, got:\n%q", got)
	}
}

func TestSliceLineToChunks_StandaloneInsertNeutralContext(t *testing.T) {
	scratch := &RenderScratch{}
	line := "    newStandaloneMethod()"

	chunks := scratch.SliceLineToChunks(line, "", nil, 100, renderutil.LineContextStandaloneInsert, false, true)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	got := string(chunks[0])
	neutralToken := color.TextFg + "newStandaloneMethod()"
	if !strings.Contains(got, neutralToken) {
		t.Errorf("standalone insert line without spans should retain neutral TextFg on unspanned tokens, got:\n%q", got)
	}
}

func TestSliceLineToChunks_NoTrailingPaddingWhenDisabled(t *testing.T) {
	scratch := &RenderScratch{}
	line := "short line"

	chunks := scratch.SliceLineToChunks(line, "", nil, 80, renderutil.LineContextAligned, false, false)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	got := string(chunks[0])
	if got != "short line" {
		t.Errorf("expected no trailing padding, got %q (len %d)", got, len(got))
	}
}

func TestRender_UnpairedMatchedLineDoesNotColorTextAsDelete(t *testing.T) {
	src := []byte("func test() {\n\tfoo()\n\tbar()\n}\n")
	dst := []byte("func test() {\n\tfoo(); bar()\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         true,
		ContextLines:  3,
		LineNumbers:   true,
		TerminalWidth: 100,
	}
	var buf bytes.Buffer
	if err := Render("a.go", "b.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	rendered := buf.String()

	if strings.Contains(rendered, color.DeleteFg+"bar()") {
		t.Errorf("expected matched code on unpaired left line NOT to use DeleteFg, got:\n%s", rendered)
	}
}

func TestRender_UnpairedMatchedLineDoesNotColorTextAsInsert(t *testing.T) {
	src := []byte("func test() {\n\tfoo(); bar()\n}\n")
	dst := []byte("func test() {\n\tfoo()\n\tbar()\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         true,
		ContextLines:  3,
		LineNumbers:   true,
		TerminalWidth: 100,
	}
	var buf bytes.Buffer
	if err := Render("a.go", "b.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	rendered := buf.String()

	if strings.Contains(rendered, color.InsertFg+"bar()") {
		t.Errorf("expected matched code on unpaired right line NOT to use InsertFg, got:\n%s", rendered)
	}
}

func TestRender_UnpairedSplitLineDoesNotColorMatchedTokensAsInsert(t *testing.T) {
	src := []byte("body = '<p>' + msg + '. Redirecting to ' + u + '</p>'\n")
	dst := []byte("body = '<title>' + msg + '</title>'\n  + '<p>' + msg + '. Redirecting to ' + u + '</p>'\n")

	dr, err := pipeline.Run(src, dst, "a.js", "b.js", pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         true,
		ContextLines:  3,
		LineNumbers:   true,
		TerminalWidth: 120,
	}
	var buf bytes.Buffer
	if err := Render("a.js", "b.js", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	rendered := buf.String()

	if strings.Contains(rendered, color.InsertFg+". Redirecting to") {
		t.Errorf("expected matched code on unpaired right line NOT to use InsertFg, got:\n%s", rendered)
	}
}
