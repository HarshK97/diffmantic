package testutil

import (
	"fmt"
	"strings"
	"testing"
)

// LineAlignmentPair matches a source file line to a destination file line.
// A line index of -1 indicates a void/filler row.
type LineAlignmentPair struct {
	LeftLine  int
	RightLine int
}

// LineQuery specifies how to identify a line in source text by its content.
type LineQuery struct {
	Text       string
	Occurrence int // 1-based occurrence index; 0 means "must be unique" (exactly 1 match)
}

// Line returns a query for a line whose trimmed content matches text.
// If text appears 0 times or > 1 times, the assertion fails loudly.
func Line(text string) LineQuery {
	return LineQuery{Text: strings.TrimSpace(text), Occurrence: 0}
}

// Nth returns a query for the n-th (1-based) occurrence of a line whose trimmed content matches text.
// If fewer than n occurrences exist, the assertion fails loudly.
func Nth(text string, n int) LineQuery {
	if n < 1 {
		panic(fmt.Sprintf("testutil.Nth: occurrence index must be >= 1, got %d", n))
	}
	return LineQuery{Text: strings.TrimSpace(text), Occurrence: n}
}

func resolveLineIndex(t testing.TB, lines []string, q LineQuery, side string) int {
	t.Helper()
	var matches []int
	for idx, l := range lines {
		if strings.TrimSpace(l) == q.Text {
			matches = append(matches, idx)
		}
	}

	if len(matches) == 0 {
		t.Fatalf("testutil.%s line query %q matched 0 lines in source", side, q.Text)
		return -1
	}

	if q.Occurrence == 0 {
		if len(matches) > 1 {
			t.Fatalf("testutil.%s line query %q is ambiguous (matched %d lines at 0-based indices %v); use testutil.Nth(%q, n)",
				side, q.Text, len(matches), matches, q.Text)
			return -1
		}
		return matches[0]
	}

	if q.Occurrence > len(matches) {
		t.Fatalf("testutil.%s line query %q requested occurrence %d, but only %d exist (found at indices %v)",
			side, q.Text, q.Occurrence, len(matches), matches)
		return -1
	}

	return matches[q.Occurrence-1]
}

// AssertLinesAligned asserts that leftQuery on the left file and rightQuery on the right file
// are paired together in the alignment grid.
//
// Both leftQuery and rightQuery are strictly validated: if either text does not exist or is ambiguous,
// the test fails loudly rather than passing for the wrong reason.
func AssertLinesAligned(t testing.TB, leftSrc, rightSrc string, grid []LineAlignmentPair, leftQuery, rightQuery LineQuery) {
	t.Helper()
	leftLines := strings.Split(leftSrc, "\n")
	rightLines := strings.Split(rightSrc, "\n")

	leftIdx := resolveLineIndex(t, leftLines, leftQuery, "left")
	rightIdx := resolveLineIndex(t, rightLines, rightQuery, "right")
	if leftIdx < 0 || rightIdx < 0 {
		return
	}

	for _, p := range grid {
		if p.LeftLine == leftIdx && p.RightLine == rightIdx {
			return // Successfully aligned
		}
	}

	// Not aligned: find what each side was actually paired with for a helpful failure message
	actualLeftPartner := -1
	actualRightPartner := -1
	for _, p := range grid {
		if p.LeftLine == leftIdx {
			actualLeftPartner = p.RightLine
		}
		if p.RightLine == rightIdx {
			actualRightPartner = p.LeftLine
		}
	}

	formatPartner := func(idx int) string {
		if idx < 0 {
			return "void"
		}
		return fmt.Sprintf("line %d", idx+1)
	}

	t.Fatalf("testutil.AssertLinesAligned failed: expected left line %d %q to align with right line %d %q,\n"+
		"but left was paired with %s (right), and right was paired with %s (left)",
		leftIdx+1, leftQuery.Text, rightIdx+1, rightQuery.Text,
		formatPartner(actualLeftPartner), formatPartner(actualRightPartner))
}

// AssertLinesNotAligned asserts that leftQuery on the left file and rightQuery on the right file
// are NOT paired together in the alignment grid.
//
// Crucial: Both leftQuery and rightQuery go through the exact same resolveLineIndex validation,
// preventing tests with mistyped text from passing vacuously.
func AssertLinesNotAligned(t testing.TB, leftSrc, rightSrc string, grid []LineAlignmentPair, leftQuery, rightQuery LineQuery) {
	t.Helper()
	leftLines := strings.Split(leftSrc, "\n")
	rightLines := strings.Split(rightSrc, "\n")

	leftIdx := resolveLineIndex(t, leftLines, leftQuery, "left")
	rightIdx := resolveLineIndex(t, rightLines, rightQuery, "right")
	if leftIdx < 0 || rightIdx < 0 {
		return
	}

	for _, p := range grid {
		if p.LeftLine == leftIdx && p.RightLine == rightIdx {
			t.Fatalf("testutil.AssertLinesNotAligned failed: left line %d %q and right line %d %q were unexpectedly aligned",
				leftIdx+1, leftQuery.Text, rightIdx+1, rightQuery.Text)
		}
	}
}
