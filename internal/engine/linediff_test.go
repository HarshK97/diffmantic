package engine

import "testing"

func TestLineDiff(t *testing.T) {
	linesA := []string{"a", "b", "c", "d"}
	linesB := []string{"a", "x", "c", "y", "d"}

	matchedA := LineDiff(linesA, linesB)

	if matchedA[0] != 0 {
		t.Errorf("expected A[0] to match B[0]")
	}
	if _, ok := matchedA[1]; ok {
		t.Errorf("expected A[1] ('b') to not match")
	}
	if matchedA[2] != 2 {
		t.Errorf("expected A[2] to match B[2]")
	}
	if matchedA[3] != 4 {
		t.Errorf("expected A[3] to match B[4], got matchedA[3]=%d", matchedA[3])
	}
}
