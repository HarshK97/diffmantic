package testutil

import (
	"strings"
	"testing"
)

func TestAssertAlignmentGrid_Success(t *testing.T) {
	leftSrc := `func process() {
	var buf bytes.Buffer
	if a || b {

	}
}
`
	rightSrc := `func process() {
	if a || b {

	}
	var buf bytes.Buffer
}
`
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},  // func process() {
		{LeftLine: 1, RightLine: -1}, // var buf bytes.Buffer │ ∅
		{LeftLine: 2, RightLine: 1},  // if a || b {
		{LeftLine: 3, RightLine: 2},  // [blank line] │ [blank line]
		{LeftLine: 4, RightLine: 3},  // }
		{LeftLine: -1, RightLine: 4}, // ∅ │ var buf bytes.Buffer
		{LeftLine: 5, RightLine: 5},  // }
	}

	expected := `
func process() {     │ func process() {
	var buf bytes.Buffer │ ∅
	if a || b {         │ 	if a || b {
	                    │ 	
	}                   │ 	}
∅                    │ 	var buf bytes.Buffer
}                    │ }
`
	AssertAlignmentGrid(t, leftSrc, rightSrc, grid, expected)
}

func TestAssertAlignmentGrid_FailureOnMismatch(t *testing.T) {
	leftSrc := `lineA`
	rightSrc := `lineB`
	grid := []LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
	}

	// Expected claims right is ∅, but actual is lineB
	expected := `lineA │ ∅`
	ft := &fakeTB{}
	AssertAlignmentGrid(ft, leftSrc, rightSrc, grid, expected)

	if !ft.failed {
		t.Fatalf("expected AssertAlignmentGrid to fail on cell mismatch")
	}
	if !strings.Contains(ft.fatalMsg, "mismatch") {
		t.Errorf("expected failure message to report mismatch, got: %s", ft.fatalMsg)
	}
}
