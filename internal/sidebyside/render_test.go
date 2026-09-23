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
	// Narrow fallback emits inline diff with @@ headers
	if !strings.Contains(out, "@@") {
		t.Errorf("expected inline diff header '@@' on narrow fallback, got:\n%s", out)
	}
}

func TestRenderFileBanner(t *testing.T) {
	var buf bytes.Buffer

	// 1. With in-place updates: [+12 -4 ~3]
	err := RenderFileBanner("pkg/auth/token.go", 12, 4, 3, false, &buf)
	if err != nil {
		t.Fatalf("RenderFileBanner failed: %v", err)
	}
	want := "━━━ pkg/auth/token.go ━━━ [+12 -4 ~3] ━━━\n"
	if buf.String() != want {
		t.Errorf("RenderFileBanner with updates: got %q, want %q", buf.String(), want)
	}

	// 2. Without in-place updates: [+28 -0]
	buf.Reset()
	err = RenderFileBanner("pkg/auth/token.go", 28, 0, 0, false, &buf)
	if err != nil {
		t.Fatalf("RenderFileBanner failed: %v", err)
	}
	wantZeroUpd := "━━━ pkg/auth/token.go ━━━ [+28 -0] ━━━\n"
	if buf.String() != wantZeroUpd {
		t.Errorf("RenderFileBanner zero updates: got %q, want %q", buf.String(), wantZeroUpd)
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
		TerminalWidth:     100,
		ContextLines:      2,
		LineNumbers:       true,
		Color:             false,
		AdaptiveThreshold: 6,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	// Large insert block (8+ lines) renders full-width inline with double gutters
	if !strings.Contains(out, "      7  func NewBigFunction() error {") {
		t.Errorf("expected inserted function in full-width output with double gutter:\n%s", out)
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
		TerminalWidth:     100,
		ContextLines:      3,
		LineNumbers:       true,
		Color:             false,
		AdaptiveThreshold: 6,
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

func TestRender_AdaptiveHybridLayout_MixedHunkWithPairedAndMonolithicBlock(t *testing.T) {
	// Mixed hunk: paired edit at top, followed by 10-line contiguous insert at bottom
	src := "package main\n\nfunc A() {\n\tport := 8080\n\tprintln(port)\n}\n"
	dst := "package main\n\nfunc A() {\n\tport := 9090\n\tprintln(port)\n}\n\nfunc NewBigHandler() {\n\t// Line 1\n\t// Line 2\n\t// Line 3\n\t// Line 4\n\t// Line 5\n\t// Line 6\n\t// Line 7\n\tprintln(\"done\")\n}\n"

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
		TerminalWidth:     100,
		ContextLines:      3,
		LineNumbers:       true,
		Color:             false,
		AdaptiveThreshold: 6,
	}

	err = Render("a.go", "b.go", []byte(src), []byte(dst), dr.Envelope, opts, &buf)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "port := 8080") || !strings.Contains(out, "port := 9090") {
		t.Errorf("expected paired edit in output:\n%s", out)
	}
	if !strings.Contains(out, "      8  func NewBigHandler() {") {
		t.Errorf("expected full-width inline insert for big handler:\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_WrappingTokenEdit(t *testing.T) {
	// A wide line with a small token edit (e.g. removing ", th") that wraps in 50/50 SBS
	// should adaptively switch to full-width inline.
	src := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, th, noPager)\n}\n")
	dst := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, noPager)\n}\n")

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
	opts.TerminalWidth = 100 // codeWidth is ~45, while the line is ~95 chars
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	// Should render as full-width inline with double number gutters
	if !strings.Contains(out, "  4          runFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB,") {
		t.Errorf("expected full-width inline delete line, got:\n%s", out)
	}
	if !strings.Contains(out, "      4      runFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB,") {
		t.Errorf("expected full-width inline insert line, got:\n%s", out)
	}
	if !strings.Contains(out, " ..      th, noPager)") {
		t.Errorf("expected continuation gutter for delete line, got:\n%s", out)
	}
	if !strings.Contains(out, "     ..  noPager)") {
		t.Errorf("expected continuation gutter for insert line, got:\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_NonWrappingTokenEdit(t *testing.T) {
	// A short line with a token edit (e.g. timeout = 10 -> timeout = 20)
	// that fits easily in 50/50 SBS should remain in side-by-side mode.
	src := []byte("package main\n\nfunc Run() {\n\ttimeout := 10\n\t_ = timeout\n}\n")
	dst := []byte("package main\n\nfunc Run() {\n\ttimeout := 20\n\t_ = timeout\n}\n")

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
	opts.TerminalWidth = 100 // codeWidth is ~45, short line is ~15 chars
	opts.Color = false

	var buf bytes.Buffer
	if err := Render("main.go", "main.go", src, dst, dr.Envelope, opts, &buf); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	out := buf.String()
	// Should NOT have "-" or "+" gutters on line numbers
	if strings.Contains(out, "-   4 ") || strings.Contains(out, "+   4 ") {
		t.Errorf("expected side-by-side layout, not full-width inline, got:\n%s", out)
	}
	if !strings.Contains(out, "timeout := 10") || !strings.Contains(out, "timeout := 20") {
		t.Errorf("expected token edits in output:\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_ForceSBS_WrappingTokenEdit(t *testing.T) {
	// Wide line that would normally switch to inline, but ForceSideBySide is set.
	src := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, th, noPager)\n}\n")
	dst := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, noPager)\n}\n")

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
	if !strings.Contains(out, "..") {
		t.Errorf("expected force-sbs to wrap and have continuation gutter '..', got:\n%s", out)
	}
}

func TestRender_AdaptiveHybridLayout_NoLineNumbers_ShowsSymbols(t *testing.T) {
	// When LineNumbers is false, symbols '-' and '+' must be rendered since there are no line numbers.
	src := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, th, noPager)\n}\n")
	dst := []byte("package main\n\nfunc Run() {\n\trunFileDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, noPager)\n}\n")

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
	// Should have "- " and "+ " symbols because line numbers are disabled
	if !strings.Contains(out, "- ") {
		t.Errorf("expected '- ' symbol when line numbers disabled, got:\n%s", out)
	}
	if !strings.Contains(out, "+ ") {
		t.Errorf("expected '+ ' symbol when line numbers disabled, got:\n%s", out)
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

func TestRender_AdaptiveHybridLayout_SubBlockGrouping(t *testing.T) {
	// A wide line with a small token edit triggers full-width hybrid layout for the segment.
	// Within that segment, 3 lines of filter(...) are replaced by 1 line of reduce(...).
	// Verifies that all 3 deleted lines are rendered before the 1 replacement line.
	src := []byte("function OverviewPanel({ events, registrations, team, crew, partners, sections, onToggleSection }: any) {\n" +
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

	// All 3 deleted lines must appear before the 1 inserted line.
	idxDel1 := strings.Index(out, "const confirmedCount = (registrations || []).filter(")
	idxDel2 := strings.Index(out, "(r: any) => r.status === 'CONFIRMED' || r.status === 'ATTENDED',")
	idxDel3 := strings.Index(out, ").length")
	idxIns := strings.Index(out, "const confirmedCount = (events || []).reduce(")

	if idxDel1 == -1 || idxDel2 == -1 || idxDel3 == -1 || idxIns == -1 {
		t.Fatalf("missing expected text in output:\n%s", out)
	}

	if idxDel1 >= idxDel2 || idxDel2 >= idxDel3 || idxDel3 >= idxIns {
		t.Errorf("expected all 3 deleted lines to precede inserted line, but got relative order:\n%s", out)
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

func TestSliceLineToChunks_StandaloneInsertFallback(t *testing.T) {
	scratch := &RenderScratch{}
	line := "    newStandaloneMethod()"

	// 0 spans on standalone insert should color unspanned tokens as InsertFg
	chunks := scratch.SliceLineToChunks(line, "", nil, 100, renderutil.LineContextStandaloneInsert, false, true)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	got := string(chunks[0])
	insToken := color.InsertFg + "newStandaloneMethod()"
	if !strings.Contains(got, insToken) {
		t.Errorf("standalone insert line should fallback to InsertFg on unspanned tokens, got:\n%q", got)
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
