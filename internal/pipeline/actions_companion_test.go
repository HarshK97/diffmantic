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
	srcLines := strings.Split(string(src), "\n")
	dstLines := strings.Split(string(dst), "\n")
	srcOffsets := serialize.BuildLineIndex(src)
	dstOffsets := serialize.BuildLineIndex(dst)

	isPairChanged := make([]bool, len(res.Envelope.LineAlignment))
	for i, pair := range res.Envelope.LineAlignment {
		if pair.LeftLine == 1 || pair.RightLine == 21 {
			isPairChanged[i] = true
		}
	}
	changeIntervals := renderutil.BuildChangeIntervals(isPairChanged)
	hunks := renderutil.MergeHunks(changeIntervals, len(res.Envelope.LineAlignment), 3)

	pyRules := rules.Get("python")
	badges := renderutil.BuildMoveBadges(res.Envelope.Actions, hunks, res.Envelope.LineAlignment, srcOffsets, dstOffsets, srcLines, dstLines, pyRules)

	badgeSrc, hasSrc := badges.SrcLineBadges[1]
	if !hasSrc || !strings.Contains(badgeSrc, "➔") {
		t.Errorf("expected SrcLineBadges[1] to contain '➔', got %q", badgeSrc)
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
