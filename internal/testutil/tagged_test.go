package testutil

import (
	"strings"
	"testing"
)

func TestFormatTagged_BasicActions(t *testing.T) {
	src := `package main

func process(target string) {
	oldHelper("first", target)
}
`
	spans := []Span{
		{Line: 3, StartCol: 1, EndCol: 10, Action: "update"},
		{Line: 3, StartCol: 11, EndCol: 18, Action: "update"},
	}

	expected := `package main

func process(target string) {
	<upd>oldHelper</upd>(<upd>"first"</upd>, target)
}
`
	AssertTagged(t, src, spans, expected)
}

func TestFormatTagged_MoveColorIndices(t *testing.T) {
	src := `stmtA()
stmtB()
stmtC()
stmtD()
`
	spans := []Span{
		{Line: 0, StartCol: 0, EndCol: 7, Action: "move", ColorIndex: 0},
		{Line: 1, StartCol: 0, EndCol: 7, Action: "move", ColorIndex: 1},
		{Line: 2, StartCol: 0, EndCol: 7, Action: "move_update", ColorIndex: 0},
		{Line: 3, StartCol: 0, EndCol: 7, Action: "move_update", ColorIndex: 2},
	}

	expected := `<mov>stmtA()</mov>
<mov1>stmtB()</mov1>
<mUpd>stmtC()</mUpd>
<mUpd2>stmtD()</mUpd2>
`
	AssertTagged(t, src, spans, expected)
}

func TestFormatTagged_AdjacentSpanMerging(t *testing.T) {
	t.Run("SameActionAndColorMerges", func(t *testing.T) {
		src := `foobar`
		spans := []Span{
			{Line: 0, StartCol: 0, EndCol: 3, Action: "delete"},
			{Line: 0, StartCol: 3, EndCol: 6, Action: "delete"},
		}
		expected := `<del>foobar</del>`
		AssertTagged(t, src, spans, expected)
	})

	t.Run("DifferentColorDoesNotMerge", func(t *testing.T) {
		src := `foobar`
		spans := []Span{
			{Line: 0, StartCol: 0, EndCol: 3, Action: "move", ColorIndex: 0},
			{Line: 0, StartCol: 3, EndCol: 6, Action: "move", ColorIndex: 1},
		}
		expected := `<mov>foo</mov><mov1>bar</mov1>`
		AssertTagged(t, src, spans, expected)
	})

	t.Run("DifferentActionDoesNotMerge", func(t *testing.T) {
		src := `foobar`
		spans := []Span{
			{Line: 0, StartCol: 0, EndCol: 3, Action: "delete"},
			{Line: 0, StartCol: 3, EndCol: 6, Action: "insert"},
		}
		expected := `<del>foo</del><ins>bar</ins>`
		AssertTagged(t, src, spans, expected)
	})
}

func TestFormatTagged_HTMLEscaping(t *testing.T) {
	// Source code containing literal <del> and <ins> in JSX/HTML, plus backslash variants
	src := `<div><del>old</del> and <ins>new</ins></div>
let escaped = "\<del>literal\</del>";
<div className="container">keep</div>
`
	spans := []Span{
		// Highlight just the word "old" (from col 10 to 13)
		{Line: 0, StartCol: 10, EndCol: 13, Action: "delete"},
	}

	expected := `<div>\<del><del>old</del>\</del> and \<ins>new\</ins></div>
let escaped = "\\<del>literal\\</del>";
<div className="container">keep</div>
`
	AssertTagged(t, src, spans, expected)
}

func TestFormatTagged_EmptyLineBleedSpan(t *testing.T) {
	// If a multi-line deletion bleeds onto an empty line, a zero-width span [0..0] must
	// NOT be silently swallowed; it must visibly render as <del></del> to catch the bleed.
	src := "func foo() {\n\n}\n"
	spans := []Span{
		{Line: 1, StartCol: 0, EndCol: 0, Action: "delete"},
	}
	expected := "func foo() {\n<del></del>\n}\n"
	AssertTagged(t, src, spans, expected)
}

func TestFormatTagged_MultibyteUTF8(t *testing.T) {
	// "fn målet() {" in UTF-8:
	// 'f' (1 byte), 'n' (1 byte), ' ' (1 byte) -> col 3
	// 'm' (1 byte), 'å' (2 bytes: 0xC3, 0xA5), 'l' (1 byte), 'e' (1 byte), 't' (1 byte) -> len("målet") = 6 bytes
	// So "målet" spans byte offsets [3..9].
	src := `fn målet(変数: i32) {
	ret();
}
`
	// Highlight "målet" (bytes 3..9) as update
	// Highlight "変数" in UTF-8: '変' (3 bytes), '数' (3 bytes) -> 6 bytes, preceded by '(' (1 byte)
	// byte offset of '(' is 9, so '変数' starts at 10 and ends at 16.
	spans := []Span{
		{Line: 0, StartCol: 3, EndCol: 9, Action: "update"},
		{Line: 0, StartCol: 10, EndCol: 16, Action: "insert"},
	}

	expected := `fn <upd>målet</upd>(<ins>変数</ins>: i32) {
	ret();
}
`
	AssertTagged(t, src, spans, expected)
}

type fakeTB struct {
	testing.TB
	failed   bool
	fatalMsg string
}

func (f *fakeTB) Helper() {}
func (f *fakeTB) Fatalf(format string, args ...any) {
	f.failed = true
	f.fatalMsg = format
}

func TestFormatTagged_OverlappingSpansPanicsOrFatals(t *testing.T) {
	src := `some code line`
	spans := []Span{
		{Line: 0, StartCol: 0, EndCol: 7, Action: "delete"},
		{Line: 0, StartCol: 5, EndCol: 12, Action: "update"}, // overlaps 5..7
	}

	ft := &fakeTB{}
	FormatTagged(ft, src, spans)

	if !ft.failed {
		t.Fatalf("expected fatal error on overlapping spans, but none occurred")
	}
	if !strings.Contains(ft.fatalMsg, "overlapping spans") {
		t.Errorf("expected error message to mention 'overlapping spans', got: %s", ft.fatalMsg)
	}
}
