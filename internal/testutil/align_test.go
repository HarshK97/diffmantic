package testutil

import (
	"strings"
	"testing"
)

func TestAssertLinesAligned_Success(t *testing.T) {
	leftSrc := `package main

func f() {
	var buf bytes.Buffer
	return
}
`
	rightSrc := `package main

func f() {
	if check() {
		return
	}
	var buf bytes.Buffer
}
`
	// In grid:
	// left line 3 ("var buf bytes.Buffer") pairs with right line 6 ("var buf bytes.Buffer")
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: -1, RightLine: 3},
		{LeftLine: -1, RightLine: 4},
		{LeftLine: -1, RightLine: 5},
		{LeftLine: 3, RightLine: 6},
		{LeftLine: 4, RightLine: -1},
		{LeftLine: 5, RightLine: 7},
	}

	AssertLinesAligned(t, leftSrc, rightSrc, grid,
		Line("var buf bytes.Buffer"),
		Line("var buf bytes.Buffer"),
	)
}

func TestAssertLinesAligned_Nth(t *testing.T) {
	leftSrc := `func f() {
	return nil
	return nil
}
`
	rightSrc := `func f() {
	return nil
	return nil
}
`
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: 1},
		{LeftLine: 2, RightLine: 2},
		{LeftLine: 3, RightLine: 3},
	}

	// Line("return nil") would fail because it appears twice
	ft := &fakeTB{}
	AssertLinesAligned(ft, leftSrc, rightSrc, grid,
		Line("return nil"),
		Line("return nil"),
	)
	if !ft.failed || !strings.Contains(ft.fatalMsg, "is ambiguous") {
		t.Fatalf("expected Line to fail on ambiguous match, got: failed=%v msg=%s", ft.failed, ft.fatalMsg)
	}

	// Nth(1) and Nth(2) should succeed cleanly
	AssertLinesAligned(t, leftSrc, rightSrc, grid,
		Nth("return nil", 1),
		Nth("return nil", 1),
	)
	AssertLinesAligned(t, leftSrc, rightSrc, grid,
		Nth("return nil", 2),
		Nth("return nil", 2),
	)
}

func TestAssertLinesNotAligned_VacuousPassPrevention(t *testing.T) {
	leftSrc := `var a = 1`
	rightSrc := `var b = 2`
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: -1},
		{LeftLine: -1, RightLine: 0},
	}

	// Mistyped query on right side MUST fail loudly rather than passing vacuously!
	ft := &fakeTB{}
	AssertLinesNotAligned(ft, leftSrc, rightSrc, grid,
		Line("var a = 1"),
		Line("var typo = 2"), // Does not exist!
	)

	if !ft.failed {
		t.Fatalf("expected AssertLinesNotAligned to fail loudly on nonexistent line, but it passed vacuously")
	}
	if !strings.Contains(ft.fatalMsg, "matched 0 lines") {
		t.Errorf("expected error to mention 'matched 0 lines', got: %s", ft.fatalMsg)
	}
}

func TestAssertLinesNotAligned_Success(t *testing.T) {
	leftSrc := `var old = 1`
	rightSrc := `var new = 2`
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: -1},
		{LeftLine: -1, RightLine: 0},
	}

	AssertLinesNotAligned(t, leftSrc, rightSrc, grid,
		Line("var old = 1"),
		Line("var new = 2"),
	)
}
