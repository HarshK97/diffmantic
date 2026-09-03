package inline

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
)

func fullEnvelopeOpts() serialize.EnvelopeOptions {
	return serialize.EnvelopeOptions{
		IncludeActions:    true,
		IncludeAlignment:  true,
		IncludeHighlights: true,
	}
}

func TestRender_BasicDiffs(t *testing.T) {
	tests := []struct {
		name         string
		srcFile      string
		dstFile      string
		srcContent   string
		dstContent   string
		opts         RenderOptions
		wantContains []string
		wantEmpty    bool
	}{
		{
			name:       "Identical files return empty string",
			srcFile:    "a.go",
			dstFile:    "b.go",
			srcContent: "func main() {\n\tprintln(\"hello\")\n}\n",
			dstContent: "func main() {\n\tprintln(\"hello\")\n}\n",
			opts:       RenderOptions{Color: false, ContextLines: 3},
			wantEmpty:  true,
		},
		{
			name:       "Simple single-line edit without line numbers",
			srcFile:    "old.go",
			dstFile:    "new.go",
			srcContent: "package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n",
			dstContent: "package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n",
			opts:       RenderOptions{Color: false, ContextLines: 3, LineNumbers: false},
			wantContains: []string{
				"--- a/old.go\n",
				"+++ b/new.go\n",
				"-\tprintln(\"hello\")\n",
				"+\tprintln(\"world\")\n",
				"@@ ",
			},
		},
		{
			name:       "Insert only with dev null header",
			srcFile:    "/dev/null",
			dstFile:    "b.txt",
			srcContent: "",
			dstContent: "line1\nline2\n",
			opts:       RenderOptions{Color: false, ContextLines: 3, LineNumbers: false},
			wantContains: []string{
				"--- /dev/null\n",
				"+++ b/b.txt\n",
				"@@ -0,0 +1,2 @@\n",
				"+line1\n+line2\n",
			},
		},
		{
			name:       "Delete only with dev null header",
			srcFile:    "a.txt",
			dstFile:    "/dev/null",
			srcContent: "line1\nline2\n",
			dstContent: "",
			opts:       RenderOptions{Color: false, ContextLines: 3, LineNumbers: false},
			wantContains: []string{
				"--- a/a.txt\n",
				"+++ /dev/null\n",
				"@@ -1,2 +0,0 @@\n",
				"-line1\n-line2\n",
			},
		},
		{
			name:       "Zero context lines",
			srcFile:    "a.txt",
			dstFile:    "b.txt",
			srcContent: "line1\nline2\nline3\nline4\n",
			dstContent: "line1\nline2_mod\nline3\nline4\n",
			opts:       RenderOptions{Color: false, ContextLines: 0, LineNumbers: false},
			wantContains: []string{
				"--- a/a.txt\n",
				"+++ b/b.txt\n",
				"@@ -2 +2 @@\n",
				"-line2\n",
				"+line2_mod\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dr, err := pipeline.Run([]byte(tt.srcContent), []byte(tt.dstContent), tt.srcFile, tt.dstFile, pipeline.DiffOptions{
				EnvelopeOpts: fullEnvelopeOpts(),
			})
			if err != nil {
				t.Fatalf("pipeline.Run failed: %v", err)
			}

			got := Render(tt.srcFile, tt.dstFile, []byte(tt.srcContent), []byte(tt.dstContent), dr.Envelope, tt.opts, nil)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("expected empty string, got:\n%s", got)
				}
				return
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("missing expected substring %q in output:\n%s", want, got)
				}
			}
		})
	}
}

func TestRender_LineNumbersGutter(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n")

	dr, err := pipeline.Run(src, dst, "old.go", "new.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("old.go", "new.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true}, nil)

	if !strings.Contains(got, "  4     │ -\tprintln(\"hello\")") {
		t.Errorf("expected source line number 4 with '-' prefix in gutter, got:\n%s", got)
	}
	if !strings.Contains(got, "      4 │ +\tprintln(\"world\")") {
		t.Errorf("expected destination line number 4 with '+' prefix in gutter, got:\n%s", got)
	}
	if !strings.Contains(got, "  1   1 │  package main") {
		t.Errorf("expected context line numbers 1 1 with space prefix in gutter, got:\n%s", got)
	}
}

func TestRender_ColorOutput(t *testing.T) {
	src := []byte("package main\nfunc foo() int { return 1 }\n")
	dst := []byte("package main\nfunc foo() int { return 2 }\n")

	dr, err := pipeline.Run(src, dst, "foo.go", "foo.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	th := theme.CatppuccinMochaTheme()
	colored := Render("foo.go", "foo.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true}, th)
	plain := Render("foo.go", "foo.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true}, th)

	if !strings.Contains(colored, "\x1b[") {
		t.Errorf("expected ANSI escape sequences in colored output:\n%q", colored)
	}
	if strings.Contains(plain, "\x1b[") {
		t.Errorf("did not expect ANSI escape sequences in plain output:\n%q", plain)
	}
}

func TestRender_TokenLevelHighlighting(t *testing.T) {
	src := []byte("func handle() {\n\tc.Writer.WriteHeader(404)\n}\n")
	dst := []byte("func handle() {\n\tc.Writer.setStatus(404)\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	th := theme.CatppuccinMochaTheme()
	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true}, th)

	// Check ANSI escape codes for move, update, and move_update token highlights.
	if !strings.Contains(got, "137;179;250") && !strings.Contains(got, "147;226;213") && !strings.Contains(got, "249;226;175") {
		t.Errorf("expected token-level highlight colors in output:\n%q", got)
	}
}

func TestRender_IntraHunkMoveSuppression(t *testing.T) {
	src := []byte("func handle404(w http.ResponseWriter, req *http.Request) {\n\tif engine.handlers404 == nil {\n\t\thttp.NotFound(c.Writer, c.Req)\n\t} else {\n\t\tc.Writer.WriteHeader(404)\n\t}\n}\n")
	dst := []byte("func handle404(w http.ResponseWriter, req *http.Request) {\n\tc.Writer.setStatus(404)\n\tc.Next()\n\tif !c.Writer.Written() {\n\t\tc.String(404, \"404 page not found\")\n\t}\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true}, nil)

	// Tier 1: Intra-hunk moves must have zero right-margin quoted token dumps or arrows
	if strings.Contains(got, "'c.Writer'") || strings.Contains(got, "←") {
		t.Errorf("expected no quoted token move annotations in output:\n%s", got)
	}
}

func TestRender_CrossHunkMoveBadges(t *testing.T) {
	src := []byte("line1\nline2\n")
	dst := []byte("line2\nline1\n")

	startDst := uint32(6)
	endDst := uint32(11)

	env := &serialize.Envelope{
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: -1},
			{LeftLine: 1, RightLine: 0},
			{LeftLine: -1, RightLine: 1},
		},
		Actions: []serialize.Action{
			{
				Action: "move",
				Node: &serialize.NodeRef{
					Tree:      "before",
					Type:      "line",
					StartByte: 0,
					EndByte:   5,
				},
				DestStartByte: &startDst,
				DestEndByte:   &endDst,
			},
		},
	}

	got := Render("a.txt", "b.txt", src, dst, env, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false}, nil)

	if !strings.Contains(got, " ➔ L2") {
		t.Errorf("expected ' ➔ L2' micro-badge on deletion line, got:\n%s", got)
	}
	if !strings.Contains(got, " ⤹ L1") {
		t.Errorf("expected ' ⤹ L1' micro-badge on insertion line, got:\n%s", got)
	}
}

func TestRender_DeclarationMoveHunkHeader(t *testing.T) {
	src := []byte("func Alpha() {\n}\n\nfunc Target() {\n}\n")
	dst := []byte("func Target() {\n}\n\nfunc Alpha() {\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false}, nil)
	if !strings.Contains(got, "func Alpha() (moved to L") && !strings.Contains(got, "func Alpha() (moved from L") {
		t.Errorf("expected declaration move signature in hunk header, got:\n%s", got)
	}
}

func TestRender_EOFNewlineChanges(t *testing.T) {
	// Case 1: src has no trailing newline, dst has trailing newline
	src1 := []byte("line1\nline2")
	dst1 := []byte("line1\nline2\n")

	dr1, err := pipeline.Run(src1, dst1, "a.txt", "b.txt", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got1 := Render("a.txt", "b.txt", src1, dst1, dr1.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: false}, nil)
	if !strings.Contains(got1, "\\ No newline at end of file") {
		t.Errorf("expected '\\ No newline at end of file' when src lacks trailing newline, got:\n%s", got1)
	}

	// Case 2: src has trailing newline, dst has no trailing newline
	src2 := []byte("line1\nline2\n")
	dst2 := []byte("line1\nline2")

	dr2, err := pipeline.Run(src2, dst2, "a.txt", "b.txt", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got2 := Render("a.txt", "b.txt", src2, dst2, dr2.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: false}, nil)
	if !strings.Contains(got2, "\\ No newline at end of file") {
		t.Errorf("expected '\\ No newline at end of file' when dst lacks trailing newline, got:\n%s", got2)
	}
}

func TestRender_MidFileInsertOnlyHunkHeader(t *testing.T) {
	src := []byte("line1\nline2\nline3\n")
	dst := []byte("line1\nline_inserted_a\nline_inserted_b\nline2\nline3\n")

	dr, err := pipeline.Run(src, dst, "a.txt", "b.txt", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.txt", "b.txt", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false}, nil)
	if !strings.Contains(got, "@@ -1,0 +2,2 @@") {
		t.Errorf("expected hunk header '@@ -1,0 +2,2 @@', got:\n%s", got)
	}
}

func TestRender_MultiByteRuneIntegrity(t *testing.T) {
	src := []byte("var greeting = \"こんにちは世界\"\nvar status = \"🚀 running\"\n")
	dst := []byte("var greeting = \"こんばんは世界\"\nvar status = \"✨ complete\"\n")

	dr, err := pipeline.Run(src, dst, "a.js", "b.js", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.js", "b.js", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true}, nil)

	if strings.ContainsRune(got, '\uFFFD') {
		t.Errorf("found Unicode replacement character \uFFFD in output:\n%s", got)
	}
	if !utf8.ValidString(got) {
		t.Errorf("output is not valid UTF-8:\n%s", got)
	}
	if !strings.Contains(got, "こんばんは世界") {
		t.Errorf("expected Japanese text in output:\n%s", got)
	}
}

func TestExtractDeclarationSignature_Truncation(t *testing.T) {
	lines := []string{
		"@[some_decorator]",
		"// doc comment",
		"func VeryLongFunctionNameWithLotsOfParametersAndGenericTypesThatExceedsTheEightyColumnLimit() {",
	}
	sig := extractDeclarationSignature(lines, 0, len(lines)-1, nil)
	if !strings.HasSuffix(sig, "...") {
		t.Errorf("expected ellipsis truncation for long signature, got: %q", sig)
	}
	if strings.Contains(sig, "@") || strings.Contains(sig, "//") {
		t.Errorf("expected decorator and comments to be bypassed, got: %q", sig)
	}
}

func TestRender_StartOfFileInsertOnlyHunkHeader(t *testing.T) {
	src := []byte("existing line\n")
	dst := []byte("new top line\nexisting line\n")

	dr, err := pipeline.Run(src, dst, "a.txt", "b.txt", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.txt", "b.txt", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false}, nil)
	if !strings.Contains(got, "@@ -1,0 +1 @@") {
		t.Errorf("expected hunk header '@@ -1,0 +1 @@', got:\n%s", got)
	}
}

func TestRender_GutterPolarityAlignment(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tprintln(\"world\")\n}\n")

	dr, err := pipeline.Run(src, dst, "old.go", "new.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	// Colored mode: gutter conveys polarity via color; redundant '-' and '+' are omitted to keep code flush at column 0
	gotColor := Render("old.go", "new.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true}, nil)
	if strings.Contains(gotColor, "│ -\t") || strings.Contains(gotColor, "│ +\t") {
		t.Errorf("expected flush code column without redundant +/- in colored gutter mode, got:\n%s", gotColor)
	}

	// Monochrome mode: '-' and '+' are strictly preserved
	gotMono := Render("old.go", "new.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true}, nil)
	if !strings.Contains(gotMono, "│ -\tprintln(\"hello\")") {
		t.Errorf("expected '-' in monochrome gutter mode, got:\n%s", gotMono)
	}
	if !strings.Contains(gotMono, "│ +\tprintln(\"world\")") {
		t.Errorf("expected '+' in monochrome gutter mode, got:\n%s", gotMono)
	}
}
