package pipeline

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/renderutil"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestPipeline_TSXUnorderedAttributesReordering(t *testing.T) {
	src := []byte("<Component a=\"1\" b=\"2\" />\n")
	dst := []byte("<Component b=\"2\" a=\"1\" />\n")

	res, err := Run(src, dst, "Component.tsx", "Component.tsx", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{IncludeHighlights: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unordered containers must emit zero highlight tags when attributes are reordered
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), string(src))
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), string(dst))

	if len(res.Envelope.LeftHighlights) != 0 {
		t.Errorf("expected 0 left highlights, got %d", len(res.Envelope.LeftHighlights))
	}
	if len(res.Envelope.RightHighlights) != 0 {
		t.Errorf("expected 0 right highlights, got %d", len(res.Envelope.RightHighlights))
	}
	if len(res.Envelope.Actions) != 0 {
		t.Errorf("expected 0 actions for unordered attribute reorder, got %d", len(res.Envelope.Actions))
	}
}

func TestPipeline_PythonStatementSwap(t *testing.T) {
	var py1Lines, py2Lines []string
	py1Lines = append(py1Lines, "def process():", "    step_swap()")
	py2Lines = append(py2Lines, "def process():")
	for i := 1; i <= 20; i++ {
		stmt := fmt.Sprintf("    work_%02d()", i)
		py1Lines = append(py1Lines, stmt)
		py2Lines = append(py2Lines, stmt)
	}
	py1Lines = append(py1Lines, "    done()", "")
	py2Lines = append(py2Lines, "    step_swap()", "    done()", "")

	src := []byte(strings.Join(py1Lines, "\n"))
	dst := []byte(strings.Join(py2Lines, "\n"))

	res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify single statement <mov> tag on both sides
	var expectedLeft, expectedRight []string
	expectedLeft = append(expectedLeft, "def process():", "    <mov>step_swap()</mov>")
	expectedRight = append(expectedRight, "def process():")
	for i := 1; i <= 20; i++ {
		stmt := fmt.Sprintf("    work_%02d()", i)
		expectedLeft = append(expectedLeft, stmt)
		expectedRight = append(expectedRight, stmt)
	}
	expectedLeft = append(expectedLeft, "    done()", "")
	expectedRight = append(expectedRight, "    <mov>step_swap()</mov>", "    done()", "")

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), strings.Join(expectedLeft, "\n"))
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), strings.Join(expectedRight, "\n"))

	// Verify exactly 1 Move action
	var moveCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "move" {
			moveCount++
		}
	}
	if moveCount != 1 {
		t.Fatalf("expected 1 move action, got %d", moveCount)
	}

	// Verify directional badges
	badges := buildPipelineBadges(res, 3, "python")

	badgeSrc, hasSrc := badges.SrcLineBadges[1]
	if !hasSrc || !strings.Contains(badgeSrc, "→") {
		t.Errorf("expected SrcLineBadges[1] to contain '→', got %q", badgeSrc)
	}
	badgeDst, hasDst := badges.DstLineBadges[21]
	if !hasDst || !strings.Contains(badgeDst, "↳") {
		t.Errorf("expected DstLineBadges[21] to contain '↳', got %q", badgeDst)
	}
}

func TestPipeline_IsolatedLeafInsert(t *testing.T) {
	src := []byte("foo(1)\n")
	dst := []byte("foo(1, 2)\n")

	res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), "foo(1)\n")
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), "foo(1<ins>, 2</ins>)\n")
}

func TestPipeline_IsolatedLeafDelete(t *testing.T) {
	src := []byte("foo(1, 2)\n")
	dst := []byte("foo(1)\n")

	res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), "foo(1<del>, 2</del>)\n")
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), "foo(1)\n")
}

func TestPipeline_IsolatedLeafUpdate(t *testing.T) {
	src := []byte("x = 1\n")
	dst := []byte("x = 2\n")

	res, err := Run(src, dst, "test.py", "test.py", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), "x = <upd>1</upd>\n")
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), "x = <upd>2</upd>\n")
}

func TestPipeline_JSUnorderedProperties(t *testing.T) {
	src := []byte("const x = {\n\ta: 1,\n\tb: 2,\n};\n")
	dst := []byte("const x = {\n\tb: 2,\n\ta: 1,\n};\n")

	res, err := Run(src, dst, "test.js", "test.js", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unordered containers must emit zero highlight tags when properties with trailing commas are reordered
	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), string(src))
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), string(dst))

	if len(res.Envelope.LeftHighlights) != 0 {
		t.Errorf("expected 0 left highlights, got %d", len(res.Envelope.LeftHighlights))
	}
	if len(res.Envelope.RightHighlights) != 0 {
		t.Errorf("expected 0 right highlights, got %d", len(res.Envelope.RightHighlights))
	}
	if len(res.Envelope.Actions) != 0 {
		t.Errorf("expected 0 actions for unordered property reorder, got %d", len(res.Envelope.Actions))
	}
}

func TestPipeline_GoStructFieldReordering(t *testing.T) {
	src := []byte(`package main

type Config struct {
	Timeout int
	Retries int
	Name    string
}
`)
	dst := []byte(`package main

type Config struct {
	Name    string
	Timeout int
	Retries int
}
`)

	res, err := Run(src, dst, "config.go", "config.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

type Config struct {
	Timeout int
	Retries int
	<mov>Name    string</mov>
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

type Config struct {
	<mov>Name    string</mov>
	Timeout int
	Retries int
}
`)

	var moveCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "move" {
			moveCount++
		}
	}
	if moveCount != 1 {
		t.Fatalf("expected 1 move action for struct field reorder, got %d", moveCount)
	}
}

func TestPipeline_SubtreeInsert(t *testing.T) {
	src := []byte(`package main

func existing() {
	println("existing")
}
`)
	dst := []byte(`package main

func existing() {
	println("existing")
}

func added() {
	println("new function")
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), string(src))
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func existing() {
	println("existing")
}

<ins>func added() {</ins>
<ins>	println("new function")</ins>
<ins>}</ins>
`)

	var insertCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "insert" {
			insertCount++
		}
	}
	if insertCount == 0 {
		t.Fatal("expected at least 1 insert action for subtree insert")
	}
}

func TestPipeline_SubtreeDelete(t *testing.T) {
	src := []byte(`package main

func existing() {
	println("existing")
}

func deleted() {
	println("old function")
}
`)
	dst := []byte(`package main

func existing() {
	println("existing")
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func existing() {
	println("existing")
}

<del>func deleted() {</del>
<del>	println("old function")</del>
<del>}</del>
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), string(dst))

	var deleteCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "delete" {
			deleteCount++
		}
	}
	if deleteCount == 0 {
		t.Fatal("expected at least 1 delete action for subtree delete")
	}
}

func TestPipeline_GoStatementSwap(t *testing.T) {
	var go1Lines, go2Lines []string
	go1Lines = append(go1Lines, "package main", "", "func process() {", "\tstepSwap()")
	go2Lines = append(go2Lines, "package main", "", "func process() {")
	for i := 1; i <= 20; i++ {
		stmt := fmt.Sprintf("\twork%02d()", i)
		go1Lines = append(go1Lines, stmt)
		go2Lines = append(go2Lines, stmt)
	}
	go1Lines = append(go1Lines, "\tdone()", "}", "")
	go2Lines = append(go2Lines, "\tstepSwap()", "\tdone()", "}", "")

	src := []byte(strings.Join(go1Lines, "\n"))
	dst := []byte(strings.Join(go2Lines, "\n"))

	res, err := Run(src, dst, "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify single statement <mov> tag on both sides
	var expectedLeft, expectedRight []string
	expectedLeft = append(expectedLeft, "package main", "", "func process():", "\t<mov>stepSwap()</mov>")
	_ = expectedLeft // adjust for Go syntax below
	expectedLeft = []string{"package main", "", "func process() {", "\t<mov>stepSwap()</mov>"}
	expectedRight = []string{"package main", "", "func process() {"}
	for i := 1; i <= 20; i++ {
		stmt := fmt.Sprintf("\twork%02d()", i)
		expectedLeft = append(expectedLeft, stmt)
		expectedRight = append(expectedRight, stmt)
	}
	expectedLeft = append(expectedLeft, "\tdone()", "}", "")
	expectedRight = append(expectedRight, "\t<mov>stepSwap()</mov>", "\tdone()", "}", "")

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), strings.Join(expectedLeft, "\n"))
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), strings.Join(expectedRight, "\n"))

	// Verify exactly 1 Move action
	var moveCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "move" {
			moveCount++
		}
	}
	if moveCount != 1 {
		t.Fatalf("expected 1 move action, got %d", moveCount)
	}

	// Verify directional badges
	srcLines := strings.Split(string(src), "\n")
	dstLines := strings.Split(string(dst), "\n")
	srcOffsets := serialize.BuildLineIndex(src)
	dstOffsets := serialize.BuildLineIndex(dst)

	isPairChanged := make([]bool, len(res.Envelope.LineAlignment))
	for i, pair := range res.Envelope.LineAlignment {
		if pair.LeftLine == 3 || pair.RightLine == 23 {
			isPairChanged[i] = true
		}
	}
	changeIntervals := renderutil.BuildChangeIntervals(isPairChanged)
	hunks := renderutil.MergeHunks(changeIntervals, len(res.Envelope.LineAlignment), 3)

	goRules := rules.Get("go")
	badges := renderutil.BuildMoveBadges(res.Envelope.Actions, hunks, res.Envelope.LineAlignment, srcOffsets, dstOffsets, srcLines, dstLines, goRules)

	badgeSrc, hasSrc := badges.SrcLineBadges[3]
	if !hasSrc || !strings.Contains(badgeSrc, "→") {
		t.Errorf("expected SrcLineBadges[3] to contain '→', got %q", badgeSrc)
	}
	badgeDst, hasDst := badges.DstLineBadges[23]
	if !hasDst || !strings.Contains(badgeDst, "↳") {
		t.Errorf("expected DstLineBadges[23] to contain '↳', got %q", badgeDst)
	}
}

func TestPipeline_FunctionRenaming(t *testing.T) {
	src := []byte(`package main

func processItems(items []int) int {
	return len(items)
}
`)
	dst := []byte(`package main

func processAllItems(items []int) int {
	return len(items)
}
`)

	res, err := Run(src, dst, "main.go", "main.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func <upd>processItems</upd>(items []int) int {
	return len(items)
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func <upd>processAllItems</upd>(items []int) int {
	return len(items)
}
`)

	var updateCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "update" {
			updateCount++
		}
	}
	if updateCount != 1 {
		t.Fatalf("expected 1 update action for function rename, got %d", updateCount)
	}
}

func TestPipeline_ListElementReordering(t *testing.T) {
	src := []byte(`package main

func f() {
	items := []string{
		"alpha",
		"beta",
		"gamma",
	}
	_ = items
}
`)
	dst := []byte(`package main

func f() {
	items := []string{
		"gamma",
		"alpha",
		"beta",
	}
	_ = items
}
`)

	res, err := Run(src, dst, "test.go", "test.go", DiffOptions{
		EnvelopeOpts: serialize.EnvelopeOptions{
			IncludeHighlights: true,
			IncludeActions:    true,
			IncludeAlignment:  true,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	testutil.AssertTagged(t, string(src), toTestSpans(res.Envelope.LeftHighlights), `package main

func f() {
	items := []string{
		"alpha",
		"beta",
		<mov>"gamma"</mov><del>,</del>
	}
	_ = items
}
`)
	testutil.AssertTagged(t, string(dst), toTestSpans(res.Envelope.RightHighlights), `package main

func f() {
	items := []string{
		<mov>"gamma"</mov><ins>,</ins>
		"alpha",
		"beta",
	}
	_ = items
}
`)

	var moveCount int
	for _, a := range res.Envelope.Actions {
		if a.Action == "move" {
			moveCount++
		}
	}
	if moveCount != 1 {
		t.Fatalf("expected 1 move action for list element reorder, got %d", moveCount)
	}
}
