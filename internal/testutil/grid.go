package testutil

import (
	"strings"
	"testing"
)

// AssertAlignmentGrid verifies that grid aligns leftSrc and rightSrc according to expected.
//
// Rules enforced:
//  1. Delimiter: Uses '│' (\u2502) to avoid collisions with '||' or '|' in code.
//  2. Void Rows: Voids are represented by '∅' (\u2205). An empty cell represents a real blank line.
//  3. Whitespace: Leading tabs and spaces are preserved as indentation. Trailing whitespace per cell
//     and exactly one space after '│' are stripped.
func AssertAlignmentGrid(t testing.TB, leftSrc, rightSrc string, grid []LineAlignmentPair, expected string) {
	t.Helper()

	// Parse expected lines
	rawExpectedLines := strings.Split(expected, "\n")
	// Trim single leading/trailing empty line from raw multiline literals
	if len(rawExpectedLines) > 0 && strings.TrimSpace(rawExpectedLines[0]) == "" {
		rawExpectedLines = rawExpectedLines[1:]
	}
	if len(rawExpectedLines) > 0 && strings.TrimSpace(rawExpectedLines[len(rawExpectedLines)-1]) == "" {
		rawExpectedLines = rawExpectedLines[:len(rawExpectedLines)-1]
	}

	type cellPair struct {
		left  string
		right string
	}

	expectedCells := make([]cellPair, len(rawExpectedLines))
	maxExpectedLeft := 0

	for i, l := range rawExpectedLines {
		parts := strings.SplitN(l, "│", 2)
		if len(parts) != 2 {
			t.Fatalf("testutil.AssertAlignmentGrid: expected line %d is missing '│' delimiter: %q", i+1, l)
		}
		lCell := strings.TrimRight(parts[0], " \t")
		rRest := strings.TrimPrefix(parts[1], " ") // strip exactly one separator space after '│' if present
		rCell := strings.TrimRight(rRest, " \t")
		expectedCells[i] = cellPair{left: lCell, right: rCell}
		if len(lCell) > maxExpectedLeft {
			maxExpectedLeft = len(lCell)
		}
	}

	leftLines := strings.Split(leftSrc, "\n")
	rightLines := strings.Split(rightSrc, "\n")

	actualCells := make([]cellPair, len(grid))
	maxActualLeft := 0

	for i, p := range grid {
		l := "∅"
		if p.LeftLine >= 0 && p.LeftLine < len(leftLines) {
			l = leftLines[p.LeftLine]
		}
		r := "∅"
		if p.RightLine >= 0 && p.RightLine < len(rightLines) {
			r = rightLines[p.RightLine]
		}
		actualCells[i] = cellPair{left: l, right: r}
		if len(l) > maxActualLeft {
			maxActualLeft = len(l)
		}
	}

	// Compare cell by cell
	matched := len(expectedCells) == len(actualCells)
	if matched {
		for i := 0; i < len(expectedCells); i++ {
			if expectedCells[i].left != actualCells[i].left || expectedCells[i].right != actualCells[i].right {
				matched = false
				break
			}
		}
	}

	if !matched {
		colWidth := max(maxExpectedLeft, maxActualLeft)
		var expSB strings.Builder
		for i, c := range expectedCells {
			pad := colWidth - len(c.left)
			if pad < 0 {
				pad = 0
			}
			expSB.WriteString(c.left)
			expSB.WriteString(strings.Repeat(" ", pad))
			expSB.WriteString(" │ ")
			expSB.WriteString(c.right)
			if i < len(expectedCells)-1 {
				expSB.WriteString("\n")
			}
		}

		var actSB strings.Builder
		for i, c := range actualCells {
			pad := colWidth - len(c.left)
			if pad < 0 {
				pad = 0
			}
			actSB.WriteString(c.left)
			actSB.WriteString(strings.Repeat(" ", pad))
			actSB.WriteString(" │ ")
			actSB.WriteString(c.right)
			if i < len(actualCells)-1 {
				actSB.WriteString("\n")
			}
		}

		diff := DiffStrings(expSB.String(), actSB.String())
		t.Fatalf("testutil.AssertAlignmentGrid mismatch (-want +got):\n%s", diff)
	}
}
