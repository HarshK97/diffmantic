package renderutil

import (
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func TestSlicer_SBS_GridAlignment_And_PaddingUnwind(t *testing.T) {
	slicer := &Slicer{}

	lineText := "func ComputeHash(seed uint32) int64"
	badgeText := " ➔ L42"

	spans := []serialize.HighlightSpan{
		{StartCol: 5, EndCol: 16, Action: "update"}, // "ComputeHash"
	}

	targetWidth := 25
	cfg := SliceConfig{
		TargetWidth:      targetWidth,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: true,
		Context:          LineContextAligned,
	}

	chunks := slicer.SliceLineToChunks(lineText, badgeText, spans, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	chunk0Str := string(chunks[0])
	chunk0Width := runewidth.StringWidth(stripANSI(chunk0Str))
	if chunk0Width != targetWidth {
		t.Errorf("chunk 0 display width = %d, want %d", chunk0Width, targetWidth)
	}
	if strings.Contains(chunk0Str, "➔ L42") {
		t.Errorf("expected chunk 0 to NOT contain move badge, got: %q", chunk0Str)
	}

	// SGR Pre-Padding Unwind: color.Reset must precede padding spaces to prevent trailing color bleed.
	if !strings.HasSuffix(chunk0Str, " ") {
		t.Errorf("expected chunk 0 to be padded with trailing spaces, got: %q", chunk0Str)
	}
	resetIdx := strings.LastIndex(chunk0Str, color.Reset)
	spaceIdx := strings.LastIndex(chunk0Str, " ")
	if resetIdx == -1 || resetIdx > spaceIdx {
		t.Errorf("expected color.Reset before trailing padding spaces, got: %q", chunk0Str)
	}

	// Chunk 1 must be padded to full targetWidth and contain the badge
	chunk1Str := string(chunks[1])
	chunk1Width := runewidth.StringWidth(stripANSI(chunk1Str))
	if chunk1Width != targetWidth {
		t.Errorf("chunk 1 display width = %d, want %d", chunk1Width, targetWidth)
	}
	if !strings.Contains(chunk1Str, "➔ L42") {
		t.Errorf("expected chunk 1 to contain move badge, got: %q", chunk1Str)
	}
}

func TestSlicer_SBS_LastRowBadge_GridAlignment_And_Padding(t *testing.T) {
	slicer := &Slicer{}

	lineText := "if prevLayout == HunkLayoutFullWidthInline || seg.layout == HunkLayoutFullWidthInline {"
	badgeText := " ➔ L205"

	targetWidth := 60
	cfg := SliceConfig{
		TargetWidth:      targetWidth,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: true,
		Context:          LineContextAligned,
	}

	chunks := slicer.SliceLineToChunks(lineText, badgeText, nil, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for long wrapped line, got %d", len(chunks))
	}

	chunk0Str := string(chunks[0])
	if strings.Contains(chunk0Str, "➔ L205") {
		t.Errorf("expected chunk 0 to NOT contain move badge, got: %q", chunk0Str)
	}
	if runewidth.StringWidth(stripANSI(chunk0Str)) != targetWidth {
		t.Errorf("chunk 0 display width = %d, want %d", runewidth.StringWidth(stripANSI(chunk0Str)), targetWidth)
	}

	lastIdx := len(chunks) - 1
	lastChunkStr := string(chunks[lastIdx])
	if !strings.Contains(lastChunkStr, "➔ L205") {
		t.Errorf("expected last chunk to contain move badge, got: %q", lastChunkStr)
	}
	if runewidth.StringWidth(stripANSI(lastChunkStr)) != targetWidth {
		t.Errorf("last chunk display width = %d, want %d", runewidth.StringWidth(stripANSI(lastChunkStr)), targetWidth)
	}

	plainLastChunk := stripANSI(lastChunkStr)
	braceIdx := strings.Index(plainLastChunk, "{")
	badgeIdx := strings.Index(plainLastChunk, "➔ L205")
	if braceIdx == -1 || badgeIdx == -1 || badgeIdx < braceIdx {
		t.Errorf("expected move badge after '{', got: %q", plainLastChunk)
	}
}

func TestSlicer_BadgeOverflow_WrapsToDedicatedRow(t *testing.T) {
	slicer := &Slicer{}

	// lineText takes 20 columns, targetWidth is 25.
	// badgeText is 7 columns (" ➔ L205").
	// 20 + 7 = 27 > 25, so the badge cannot fit on Chunk 0 and wraps to Chunk 1.
	lineText := "let totalScore = 100"
	badgeText := " ➔ L205"

	targetWidth := 25
	cfg := SliceConfig{
		TargetWidth:      targetWidth,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: true,
		Context:          LineContextAligned,
	}

	chunks := slicer.SliceLineToChunks(lineText, badgeText, nil, cfg)
	if len(chunks) != 2 {
		t.Fatalf("expected exactly 2 chunks (text on chunk 0, badge on chunk 1), got %d", len(chunks))
	}

	chunk0Str := string(chunks[0])
	if strings.Contains(chunk0Str, "➔ L205") {
		t.Errorf("expected chunk 0 to NOT contain move badge, got: %q", chunk0Str)
	}
	if runewidth.StringWidth(stripANSI(chunk0Str)) != targetWidth {
		t.Errorf("chunk 0 display width = %d, want %d", runewidth.StringWidth(stripANSI(chunk0Str)), targetWidth)
	}

	chunk1Str := string(chunks[1])
	if !strings.Contains(chunk1Str, "➔ L205") {
		t.Errorf("expected chunk 1 to contain move badge, got: %q", chunk1Str)
	}
	if runewidth.StringWidth(stripANSI(chunk1Str)) != targetWidth {
		t.Errorf("chunk 1 display width = %d, want %d", runewidth.StringWidth(stripANSI(chunk1Str)), targetWidth)
	}
}

func TestSlicer_Inline_LastRowBadge(t *testing.T) {
	slicer := &Slicer{}

	lineText := "very long line that wraps across multiple visual rows in inline diff format"
	badgeText := " ➔ L99"

	cfg := SliceConfig{
		TargetWidth:      30,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: false,
		Context:          LineContextAligned,
	}

	chunks := slicer.SliceLineToChunks(lineText, badgeText, nil, cfg)
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for long wrapped line, got %d", len(chunks))
	}

	chunk0Str := string(chunks[0])
	if strings.Contains(chunk0Str, "➔ L99") {
		t.Errorf("inline mode: chunk 0 should NOT contain badge: %q", chunk0Str)
	}

	lastChunkStr := string(chunks[len(chunks)-1])
	if !strings.Contains(lastChunkStr, "➔ L99") {
		t.Errorf("inline mode: last chunk should contain badge: %q", lastChunkStr)
	}
	if !strings.HasSuffix(lastChunkStr, color.MoveFg+badgeText+color.Reset) {
		t.Errorf("expected last chunk to end with styled badge, got: %q", lastChunkStr)
	}
}

func TestSlicer_MultibyteUTF8Boundary(t *testing.T) {
	slicer := &Slicer{}

	// "你好世界" (Hello World in Chinese) - each character is 3 bytes, display width 2
	lineText := "你好世界"
	// Span highlighting "好世" (bytes 3 to 9) as "update"
	spans := []serialize.HighlightSpan{
		{StartCol: 3, EndCol: 9, Action: "update"},
	}

	cfg := SliceConfig{
		TargetWidth:      50,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: false,
		Context:          LineContextAligned,
	}

	chunks := slicer.SliceLineToChunks(lineText, "", spans, cfg)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}

	chunkBytes := chunks[0]
	// Verify that the output is valid UTF-8
	if !utf8.Valid(chunkBytes) {
		t.Fatalf("sliced output contains invalid UTF-8 byte sequences: %q", chunkBytes)
	}

	chunkStr := string(chunkBytes)
	// Verify highlight sequence appears strictly before '好'
	haoIdx := strings.Index(chunkStr, "好")
	if haoIdx == -1 {
		t.Fatalf("expected '好' in output: %q", chunkStr)
	}
	if !strings.Contains(chunkStr[:haoIdx], color.UpdateFg) {
		t.Errorf("expected UpdateFg before '好', got %q", chunkStr)
	}

	// Verify reset sequence appears strictly after '世' and before '界'
	shiIdx := strings.Index(chunkStr, "世")
	jieIdx := strings.Index(chunkStr, "界")
	between := chunkStr[shiIdx+len("世") : jieIdx]
	if !strings.Contains(between, color.Reset) {
		t.Errorf("expected Reset between '世' and '界', got %q", between)
	}
}

func TestSlicer_NeutralContext_WhitespaceAligned(t *testing.T) {
	slicer := &Slicer{}

	lineText := "    field := value" // 4 spaces of indentation

	// 1. Aligned pair with 0 spans: verify unhighlighted line renders with neutral context styling
	alignedCfg := SliceConfig{
		TargetWidth:      50,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: false,
		Context:          LineContextAligned,
	}

	alignedChunks := slicer.SliceLineToChunks(lineText, "", nil, alignedCfg)
	alignedStr := string(alignedChunks[0])

	// Indentation must NOT be colored
	if strings.Contains(alignedStr[:4], "\x1b[") {
		t.Errorf("leading indentation should not contain ANSI codes: %q", alignedStr)
	}
	// Aligned line must NOT have DeleteFg or InsertFg
	if strings.Contains(alignedStr, color.DeleteFg) {
		t.Errorf("aligned line with 0 spans must not contain DeleteFg (wall-of-red defect): %q", alignedStr)
	}
	if strings.Contains(alignedStr, color.InsertFg) {
		t.Errorf("aligned line with 0 spans must not contain InsertFg (wall-of-green defect): %q", alignedStr)
	}
	// Must contain neutral TextFg
	if !strings.Contains(alignedStr, color.TextFg) {
		t.Errorf("aligned line with 0 spans must contain neutral TextFg: %q", alignedStr)
	}

	// 2. Standalone deletion with 0 spans: must use DeleteFg after indentation
	delCfg := SliceConfig{
		TargetWidth:      50,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: false,
		Context:          LineContextStandaloneDelete,
	}
	delChunks := slicer.SliceLineToChunks(lineText, "", nil, delCfg)
	delStr := string(delChunks[0])
	if strings.Contains(delStr[:4], "\x1b[") {
		t.Errorf("standalone delete: leading indentation should not contain ANSI codes: %q", delStr)
	}
	if !strings.Contains(delStr, color.DeleteFg) {
		t.Errorf("standalone delete with 0 spans must contain DeleteFg: %q", delStr)
	}

	// 3. Standalone insertion with 0 spans: must use InsertFg after indentation
	insCfg := SliceConfig{
		TargetWidth:      50,
		TabWidth:         4,
		ColorMode:        true,
		PadToTargetWidth: false,
		Context:          LineContextStandaloneInsert,
	}
	insChunks := slicer.SliceLineToChunks(lineText, "", nil, insCfg)
	insStr := string(insChunks[0])
	if strings.Contains(insStr[:4], "\x1b[") {
		t.Errorf("standalone insert: leading indentation should not contain ANSI codes: %q", insStr)
	}
	if !strings.Contains(insStr, color.InsertFg) {
		t.Errorf("standalone insert with 0 spans must contain InsertFg: %q", insStr)
	}
}

func TestIsPunctuationOrWhitespace(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{input: "", want: true},
		{input: "   \t\n  ", want: true},
		{input: "}", want: true},
		{input: ");", want: true},
		{input: "]", want: true},
		{input: ",", want: true},
		{input: "};", want: true},
		{input: " := ", want: true},
		{input: "foo", want: false},
		{input: "123", want: false},
		{input: "_", want: false},
		{input: "_,", want: false},
		{input: "\tfoo()", want: false},
		{input: "\tbar()", want: false},
		{input: " // comment with letters", want: false},
	}

	for _, tc := range tests {
		got := IsPunctuationOrWhitespace(tc.input)
		if got != tc.want {
			t.Errorf("IsPunctuationOrWhitespace(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestShouldStyleStandalone(t *testing.T) {
	spans := []serialize.HighlightSpan{
		{StartCol: 0, EndCol: 3, Action: "delete"},
	}

	// Paired lines never take standalone styling.
	if ShouldStyleStandalone(false, spans, "foo()") {
		t.Errorf("expected paired line with spans to NOT take standalone styling")
	}
	if ShouldStyleStandalone(false, nil, "}") {
		t.Errorf("expected paired line without spans to NOT take standalone styling")
	}

	// Unpaired line with active diff spans takes standalone styling.
	if !ShouldStyleStandalone(true, spans, "foo()") {
		t.Errorf("expected unpaired line with spans to take standalone styling")
	}

	// Unpaired line with zero spans and delimiters/punctuation takes standalone styling.
	if !ShouldStyleStandalone(true, nil, "}\n") {
		t.Errorf("expected unpaired line with delimiters to take standalone styling")
	}

	// Unpaired line with zero spans and substantive code remains neutral context.
	if ShouldStyleStandalone(true, nil, "foo()") {
		t.Errorf("expected unpaired line with substantive code and 0 spans to NOT take standalone styling")
	}

	// Unpaired line with blank identifier remains neutral context.
	if ShouldStyleStandalone(true, nil, "_,") {
		t.Errorf("expected unpaired line with blank identifier and 0 spans to NOT take standalone styling")
	}
}
