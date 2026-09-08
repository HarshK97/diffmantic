package inline

import (
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
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

			got := Render(tt.srcFile, tt.dstFile, []byte(tt.srcContent), []byte(tt.dstContent), dr.Envelope, tt.opts)

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

	got := Render("old.go", "new.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true})

	if !strings.Contains(got, "  4      -\tprintln(\"hello\")") {
		t.Errorf("expected source line number 4 with '-' prefix in gutter, got:\n%s", got)
	}
	if !strings.Contains(got, "      4  +\tprintln(\"world\")") {
		t.Errorf("expected destination line number 4 with '+' prefix in gutter, got:\n%s", got)
	}
	if !strings.Contains(got, "  1   1   package main") {
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

	colored := Render("foo.go", "foo.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true})
	plain := Render("foo.go", "foo.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true})

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

	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true})

	// Check ANSI escape codes for move, update, and move_update token highlights.
	if !strings.Contains(got, color.MoveFg) && !strings.Contains(got, color.UpdateFg) {
		t.Errorf("expected token-level highlight colors in output:\n%q", got)
	}
}

func TestRender_Tier1_IntraHunkMoveCleanliness(t *testing.T) {
	src := []byte("func handle404(w http.ResponseWriter, req *http.Request) {\n\tif engine.handlers404 == nil {\n\t\thttp.NotFound(c.Writer, c.Req)\n\t} else {\n\t\tc.Writer.WriteHeader(404)\n\t}\n}\n")
	dst := []byte("func handle404(w http.ResponseWriter, req *http.Request) {\n\tc.Writer.setStatus(404)\n\tc.Next()\n\tif !c.Writer.Written() {\n\t\tc.String(404, \"404 page not found\")\n\t}\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true})

	// Under Tier 1, intra-hunk moves suppress right-margin ghost text completely.
	if strings.Contains(got, "←") || strings.Contains(got, "➔") || strings.Contains(got, "⤹") || strings.Contains(got, "moved to line") {
		t.Errorf("expected zero right-margin trailing ghost annotations for intra-hunk move, got:\n%s", got)
	}
	if !strings.Contains(got, "c.Writer.setStatus(404)") {
		t.Errorf("expected destination code in output, got:\n%s", got)
	}
}

func TestRender_Tier2_CrossHunkDeclarationMove(t *testing.T) {
	src := []byte("func Alpha() {\n}\n\nfunc Target() {\n}\n")
	dst := []byte("func Target() {\n}\n\nfunc Alpha() {\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	got := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false})

	// Check that POSIX hunk header contains moved context or clean lines.
	if strings.Contains(got, "←") {
		t.Errorf("expected zero legacy arrow annotations, got:\n%s", got)
	}
	if !strings.Contains(got, "func Alpha") {
		t.Errorf("expected 'func Alpha' in output, got:\n%s", got)
	}
}

func TestRender_Tier3_CrossHunkSubBlockMicroBadge(t *testing.T) {
	src := []byte("line1\nline2\nline3\nline4\n")
	dst := []byte("line3\nline4\nline5\nline1\nline2\n")

	startDst := uint32(18)
	endDst := uint32(29)

	env := &serialize.Envelope{
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: -1},
			{LeftLine: 1, RightLine: -1},
			{LeftLine: 2, RightLine: 0},
			{LeftLine: 3, RightLine: 1},
			{LeftLine: -1, RightLine: 2},
			{LeftLine: -1, RightLine: 3},
			{LeftLine: -1, RightLine: 4},
		},
		Actions: []serialize.Action{
			{
				Action: "move",
				Node: &serialize.NodeRef{
					Tree:      "before",
					Type:      "statement",
					StartByte: 0,
					EndByte:   11,
				},
				DestStartByte: &startDst,
				DestEndByte:   &endDst,
			},
		},
	}

	got := Render("a.txt", "b.txt", src, dst, env, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false})

	// Micro-badges should be bounded and appear on line 1 of moved sub-blocks across hunks.
	if strings.Contains(got, "← moved to line") {
		t.Errorf("expected legacy '← moved to line' to be removed, got:\n%s", got)
	}
	if strings.Contains(got, "➔ L") || strings.Contains(got, "⤹ L") {
		// Bounded micro-badge present
		if strings.Contains(got, "line2 ➔ L") {
			t.Errorf("expected micro-badge only on line 1, not subsequent line, got:\n%s", got)
		}
	}
}

func TestRender_DisableAnnotationsOption(t *testing.T) {
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
					Type:      "statement",
					StartByte: 0,
					EndByte:   5,
				},
				DestStartByte: &startDst,
				DestEndByte:   &endDst,
			},
		},
	}

	got := Render("a.txt", "b.txt", src, dst, env, RenderOptions{Color: false, ContextLines: 3, LineNumbers: false, DisableAnnotations: true})

	if strings.Contains(got, "➔") || strings.Contains(got, "⤹") || strings.Contains(got, "moved") {
		t.Errorf("expected zero move annotations when DisableAnnotations=true, got:\n%s", got)
	}
}

func TestExtractDeclarationSignature(t *testing.T) {
	lines := []string{
		"func (h *Header) MarshalXML(e *xml.Encoder, start xml.StartElement) error {",
		"\treturn nil",
		"}",
	}
	sig := extractDeclarationSignature(&serialize.NodeRef{Type: "function_declaration"}, lines, 0, 2)
	if sig != "func (h *Header) MarshalXML(e *xml.Encoder, start xml.StartElement) error" {
		t.Errorf("unexpected signature: %q", sig)
	}

	// Test decorator and comment bypassing
	linesWithDecorator := []string{
		"// Header comment",
		"@dataclass",
		"@app.route(\"/api/v1\")",
		"def handle_request(req):",
		"\tpass",
	}
	sigDec := extractDeclarationSignature(&serialize.NodeRef{Type: "function_definition"}, linesWithDecorator, 0, 4)
	if sigDec != "def handle_request(req):" {
		t.Errorf("expected decorator bypass, got %q", sigDec)
	}

	longLine := "func VeryLongFunctionNameToTestTruncationBehaviorAcrossBoundaries(withManyArgumentsA string, withManyArgumentsB int) error {"
	longLines := []string{longLine}
	sigLong := extractDeclarationSignature(&serialize.NodeRef{Type: "function_declaration"}, longLines, 0, 0)
	if len(sigLong) > 80 {
		t.Errorf("expected signature length <= 80, got %d (%q)", len(sigLong), sigLong)
	}
	if !strings.HasSuffix(sigLong, "...") {
		t.Errorf("expected ellipsis suffix for long signature, got %q", sigLong)
	}
}

func TestRender_Tier2_ModifiedRelocation(t *testing.T) {
	src := []byte("func Process() {\n\tstepA()\n\tstepB()\n}\n\nfunc Helper() {\n\tnoop()\n}\n")
	dst := []byte("func Helper() {\n\tnoop()\n}\n\nfunc Extra() {\n\tlog()\n}\n\nfunc Process() {\n\tstepA()\n\tstepB_modified()\n\tstepC_new()\n}\n")

	startDst := uint32(50)
	endDst := uint32(110)
	mutDst := uint32(75)

	env := &serialize.Envelope{
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: -1},
			{LeftLine: 1, RightLine: -1},
			{LeftLine: 2, RightLine: -1},
			{LeftLine: 3, RightLine: -1},
			{LeftLine: 4, RightLine: -1},
			{LeftLine: 5, RightLine: 0},
			{LeftLine: 6, RightLine: 1},
			{LeftLine: 7, RightLine: 2},
			{LeftLine: -1, RightLine: 3},
			{LeftLine: -1, RightLine: 4},
			{LeftLine: -1, RightLine: 5},
			{LeftLine: -1, RightLine: 6},
			{LeftLine: -1, RightLine: 7},
			{LeftLine: -1, RightLine: 8},
			{LeftLine: -1, RightLine: 9},
		},
		Actions: []serialize.Action{
			{
				Action: "move",
				Node: &serialize.NodeRef{
					Tree:      "before",
					Type:      "function_declaration",
					StartByte: 0,
					EndByte:   44,
				},
				DestStartByte: &startDst,
				DestEndByte:   &endDst,
			},
			{
				Action: "insert",
				Node: &serialize.NodeRef{
					Tree:      "after",
					Type:      "call_expression",
					StartByte: mutDst,
					EndByte:   mutDst + 10,
				},
			},
		},
	}

	got := Render("main.go", "main.go", src, dst, env, RenderOptions{Color: false, ContextLines: 1, LineNumbers: false})

	if !strings.Contains(got, "func Process") {
		t.Errorf("expected 'func Process' in output, got:\n%s", got)
	}
	if strings.Contains(got, "←") {
		t.Errorf("expected zero legacy arrow annotations, got:\n%s", got)
	}
}

func TestRender_Tier3_MultiMoveHunk(t *testing.T) {
	src := []byte("lineA\nlineB\nctx1\nctx2\nctx3\nctx4\nctx5\nctx6\nctx7\nctx8\n")
	dst := []byte("ctx1\nctx2\nctx3\nctx4\nctx5\nctx6\nctx7\nctx8\nlineB\nlineA\n")

	startDstB := uint32(40)
	endDstB := uint32(45)
	startDstA := uint32(46)
	endDstA := uint32(51)

	env := &serialize.Envelope{
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: -1},
			{LeftLine: 1, RightLine: -1},
			{LeftLine: 2, RightLine: 0},
			{LeftLine: 3, RightLine: 1},
			{LeftLine: 4, RightLine: 2},
			{LeftLine: 5, RightLine: 3},
			{LeftLine: 6, RightLine: 4},
			{LeftLine: 7, RightLine: 5},
			{LeftLine: 8, RightLine: 6},
			{LeftLine: 9, RightLine: 7},
			{LeftLine: -1, RightLine: 8},
			{LeftLine: -1, RightLine: 9},
		},
		Actions: []serialize.Action{
			{
				Action: "move",
				Node: &serialize.NodeRef{
					Tree:      "before",
					Type:      "statement",
					StartByte: 0,
					EndByte:   5,
				},
				DestStartByte: &startDstA,
				DestEndByte:   &endDstA,
			},
			{
				Action: "move",
				Node: &serialize.NodeRef{
					Tree:      "before",
					Type:      "statement",
					StartByte: 6,
					EndByte:   11,
				},
				DestStartByte: &startDstB,
				DestEndByte:   &endDstB,
			},
		},
	}

	got := Render("a.txt", "b.txt", src, dst, env, RenderOptions{Color: false, ContextLines: 1, LineNumbers: false})

	if strings.Contains(got, "← moved to line") {
		t.Errorf("expected zero legacy arrow annotations, got:\n%s", got)
	}
	if !strings.Contains(got, "-lineA ➔ L10") {
		t.Errorf("expected '-lineA ➔ L10' badge on lineA, got:\n%s", got)
	}
	if !strings.Contains(got, "-lineB ➔ L9") {
		t.Errorf("expected '-lineB ➔ L9' badge on lineB, got:\n%s", got)
	}
}

func TestRender_GutterPrefixOmissionInColorMode(t *testing.T) {
	src := []byte("func oldFunc() {\n\treturn 1\n}\n")
	dst := []byte("func newFunc() {\n\treturn 2\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	// 1. Color + LineNumbers -> Omit + and - prefixes
	colorWithLines := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true})
	// Should not have -func or +func in color mode
	if strings.Contains(colorWithLines, "-func") || strings.Contains(colorWithLines, "+func") {
		t.Errorf("expected +/- prefixes to be omitted when Color and LineNumbers are enabled, got:\n%s", colorWithLines)
	}

	// 2. Monochrome + LineNumbers -> Keep + and - prefixes
	plainWithLines := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 3, LineNumbers: true})
	if !strings.Contains(plainWithLines, "-func") || !strings.Contains(plainWithLines, "+func") {
		t.Errorf("expected +/- prefixes to be retained in monochrome LineNumbers mode, got:\n%s", plainWithLines)
	}

	// 3. Color + No LineNumbers -> Keep + and - prefixes
	colorNoLines := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: false})
	if !strings.Contains(colorNoLines, "-") || !strings.Contains(colorNoLines, "+") {
		t.Errorf("expected +/- prefixes to be retained when LineNumbers is false, got:\n%s", colorNoLines)
	}
}

func TestRender_UnmutatedTokenBaseColor(t *testing.T) {
	src := []byte("if err := e.EncodeToken(xml.EndElement{start.Name}); err != nil {\n\treturn err\n}\n")
	dst := []byte("if err := e.EncodeToken(xml.EndElement{Name: start.Name}); err != nil {\n\treturn err\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	rendered := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true})

	// Unmutated segments on fine-grained lines stay neutral (TextFg); only highlighted tokens get action colors
	if !strings.Contains(rendered, color.TextFg+"if err :=") {
		t.Errorf("expected unmutated prefix on delete line to be styled with TextFg, got:\n%q", rendered)
	}
	if strings.Contains(rendered, color.DeleteFg+"if err :=") {
		t.Errorf("expected unmutated prefix on delete line NOT to be styled with DeleteFg, got:\n%q", rendered)
	}
	if !strings.Contains(rendered, color.TextFg+"if err :=") {
		t.Errorf("expected unmutated prefix on insert line to be styled with TextFg, got:\n%q", rendered)
	}
}

func TestRender_UnchangedMultilineLineUsesBaseColor(t *testing.T) {
	src := []byte("func foo() {\n\tstart.Name = xml.Name{\"\", \"map\"}\n}\n")
	dst := []byte("func foo() {\n\tstart.Name = xml.Name{\n\t\tSpace: \"\",\n\t\tLocal: \"map\",\n\t}\n}\n")

	dr, err := pipeline.Run(src, dst, "a.go", "b.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	rendered := Render("a.go", "b.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 3, LineNumbers: true})

	// The container header is paired with the old single line, and the old
	// side carries the highlights (moved args). So the header renders plain.
	// It didn't change. The closing brace is a pure insert, so it keeps
	// InsertFg.
	if !strings.Contains(rendered, color.TextFg+"start.Name = xml.Name{") {
		t.Errorf("expected container header on paired inserted line to use TextFg color, got:\n%q", rendered)
	}
	if strings.Contains(rendered, color.InsertFg+"start.Name = xml.Name{") {
		t.Errorf("expected container header on paired inserted line NOT to use InsertFg color, got:\n%q", rendered)
	}
	// Closing brace `}` on inserted line also uses base InsertFg
	if !strings.Contains(rendered, color.InsertFg+"}") {
		t.Errorf("expected closing brace on inserted line to use InsertFg color, got:\n%q", rendered)
	}
}

func TestRender_BinaryFile(t *testing.T) {
	src := []byte("PNG\x00\x00\x01\x02")
	dst := []byte("PNG\x00\x00\x01\x03")

	dr, err := pipeline.Run(src, dst, "a.png", "b.png", pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	rendered := Render("a.png", "b.png", src, dst, dr.Envelope, RenderOptions{Color: false})
	if !strings.Contains(rendered, "Binary files a.png and b.png differ") {
		t.Errorf("expected binary differ message, got: %q", rendered)
	}
}

func TestRender_AppendAtEOF_NoPhantomContextLine(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tprintln(\"hello\")\n}\n\nfunc extra() {\n\tprintln(\"world\")\n}\n")

	dr, err := pipeline.Run(src, dst, "main.go", "main.go", pipeline.DiffOptions{
		EnvelopeOpts: fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	rendered := Render("main.go", "main.go", src, dst, dr.Envelope, RenderOptions{
		Color:        false,
		ContextLines: 3,
		LineNumbers:  false,
	})

	// Context should not overshoot source line count (5 lines).
	// With 3 context lines, it should be @@ -3,3 +3,7 @@ or similar, never @@ -3,4 ... @@
	if strings.Contains(rendered, "@@ -3,4") {
		t.Errorf("expected hunk to not overshoot source line count with phantom context line, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "+func extra() {") {
		t.Errorf("expected rendered output to contain inserted function, got:\n%s", rendered)
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

	got := Render("a.txt", "b.txt", src, dst, dr.Envelope, RenderOptions{Color: false, ContextLines: 0, LineNumbers: false})
	if !strings.Contains(got, "@@ -1,0 +1 @@") {
		t.Errorf("expected hunk header '@@ -1,0 +1 @@', got:\n%s", got)
	}
}

func TestRender_SubBlockGrouping(t *testing.T) {
	src := []byte("package main\n\nfunc Calc() {\n\ta := 1\n\tb := filter(\n\t\tx,\n\t\ty,\n\t)\n\tc := 3\n}\n")
	dst := []byte("package main\n\nfunc Calc() {\n\ta := 10\n\tb := reduce(x, y)\n\tc := 30\n}\n")

	env := &serialize.Envelope{
		LineAlignment: []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
			{LeftLine: 1, RightLine: 1},
			{LeftLine: 2, RightLine: 2},
			{LeftLine: 3, RightLine: 3},
			{LeftLine: 4, RightLine: 4},
			{LeftLine: 5, RightLine: -1},
			{LeftLine: 6, RightLine: -1},
			{LeftLine: 7, RightLine: -1},
			{LeftLine: 8, RightLine: 5},
			{LeftLine: 9, RightLine: 6},
		},
		LeftHighlights: []serialize.HighlightSpan{
			{Line: 3, StartCol: 1, EndCol: 7, Action: "delete"},
			{Line: 4, StartCol: 1, EndCol: 13, Action: "delete"},
			{Line: 5, StartCol: 1, EndCol: 5, Action: "delete"},
			{Line: 6, StartCol: 1, EndCol: 5, Action: "delete"},
			{Line: 7, StartCol: 1, EndCol: 3, Action: "delete"},
			{Line: 8, StartCol: 1, EndCol: 7, Action: "delete"},
		},
		RightHighlights: []serialize.HighlightSpan{
			{Line: 3, StartCol: 1, EndCol: 8, Action: "insert"},
			{Line: 4, StartCol: 1, EndCol: 19, Action: "insert"},
			{Line: 5, StartCol: 1, EndCol: 8, Action: "insert"},
		},
	}

	got := Render("a.go", "b.go", src, dst, env, RenderOptions{Color: false, ContextLines: 1, LineNumbers: false})

	// Deletions of b := filter(...) must complete before b := reduce(x, y) is inserted.
	// That is, all 3 lines of b := filter(...) must appear contiguously before +	b := reduce(x, y).
	expectedDelBlock := "-\tb := filter(\n-\t\tx,\n-\t\ty,\n-\t)\n+\tb := reduce(x, y)\n"
	if !strings.Contains(got, expectedDelBlock) {
		t.Errorf("expected sub-block to complete deletions before insertion:\nExpected block:\n%s\nGot:\n%s", expectedDelBlock, got)
	}
}

func TestRender_LineWrapping_ContinuationGutter(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tif format != \"\" && !slices.Contains([]string{\"json\", \"actions\", \"inline\"}, format) {\n\t\tprintln(format)\n\t}\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tif format != \"\" && !slices.Contains([]string{\"json\", \"actions\", \"tui\", \"inline\"}, format) {\n\t\tprintln(format)\n\t}\n}\n")

	dr, err := pipeline.Run(src, dst, "old.go", "new.go", pipeline.DiffOptions{
		ParseErrorLimit: 0,
		EnvelopeOpts:    fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         false,
		ContextLines:  1,
		LineNumbers:   true,
		Wrap:          true,
		TerminalWidth: 60,
		TabWidth:      4,
	}

	got := Render("old.go", "new.go", src, dst, dr.Envelope, opts)

	// Wrapped lines should have an empty gutter on continuation rows.
	lines := strings.Split(got, "\n")
	hasContinuationLine := false
	for _, l := range lines {
		if strings.HasPrefix(l, "          ") && len(strings.TrimSpace(l)) > 0 && !strings.Contains(l, "func") && !strings.Contains(l, "package") {
			hasContinuationLine = true
		}
	}

	if !hasContinuationLine {
		t.Fatalf("expected at least one continuation line in wrapped output:\n%s", got)
	}
}

func TestRender_LineWrapping_ColorMode(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tlongVar := \"This is a very long string that will definitely exceed the terminal width and wrap over multiple rows\"\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tlongVar := \"This is an updated very long string that will definitely exceed the terminal width and wrap over multiple rows\"\n}\n")

	dr, err := pipeline.Run(src, dst, "old.go", "new.go", pipeline.DiffOptions{
		ParseErrorLimit: 0,
		EnvelopeOpts:    fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         true,
		ContextLines:  1,
		LineNumbers:   true,
		Wrap:          true,
		TerminalWidth: 50,
		TabWidth:      4,
	}

	got := Render("old.go", "new.go", src, dst, dr.Envelope, opts)

	lines := strings.Split(got, "\n")
	// Continuation rows shouldn't have color codes in the gutter — it should be plain spaces.
	// With ~5 lines, numWidth is 3, so the gutter is 9 chars wide in color mode.
	const contGutterWidth = 9
	for _, l := range lines {
		if len(strings.TrimSpace(l)) == 0 {
			continue
		}
		// If the gutter's empty (no digits), it's a continuation row.
		if len(l) >= contGutterWidth && strings.HasPrefix(l, "   ") && !strings.Contains(l[:contGutterWidth], "1") && !strings.Contains(l[:contGutterWidth], "2") && !strings.Contains(l[:contGutterWidth], "3") {
			gutterRaw := l[:contGutterWidth]
			if strings.Contains(gutterRaw, "\x1b[") {
				t.Errorf("continuation gutter leaked ANSI escape into line-number area: %q", l)
			}
		}
	}
	// Wrapping should give us more physical lines than not wrapping.
	unwrapped := Render("old.go", "new.go", src, dst, dr.Envelope, RenderOptions{Color: true, ContextLines: 1, LineNumbers: true, Wrap: false, TerminalWidth: 50, TabWidth: 4})
	if strings.Count(got, "\n") <= strings.Count(unwrapped, "\n") {
		t.Errorf("expected wrapped output to have more physical lines than unwrapped")
	}
}

func TestRender_LineWrapping_DisabledWhenWrapFalse(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tlongVar := \"This is a very long string that will definitely exceed the terminal width and wrap over multiple rows\"\n}\n")
	dst := []byte("package main\n\nfunc main() {\n\tlongVar := \"This is an updated very long string that will definitely exceed the terminal width and wrap over multiple rows\"\n}\n")

	dr, err := pipeline.Run(src, dst, "old.go", "new.go", pipeline.DiffOptions{
		ParseErrorLimit: 0,
		EnvelopeOpts:    fullEnvelopeOpts(),
	})
	if err != nil {
		t.Fatalf("pipeline.Run failed: %v", err)
	}

	opts := RenderOptions{
		Color:         false,
		ContextLines:  1,
		LineNumbers:   true,
		Wrap:          false,
		TerminalWidth: 50,
		TabWidth:      4,
	}

	got := Render("old.go", "new.go", src, dst, dr.Envelope, opts)

	// Without wrapping, the whole string stays on one line.
	if !strings.Contains(got, "This is an updated very long string that will definitely exceed the terminal width and wrap over multiple rows") {
		t.Errorf("expected unwrapped line to contain full string, got:\n%s", got)
	}
}
