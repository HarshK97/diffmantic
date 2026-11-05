package engine

import (
	"slices"
	"testing"
)

func TestTrimPrefixSuffix(t *testing.T) {
	t.Run("Identical", func(t *testing.T) {
		a := []string{"a", "b", "c"}
		b := []string{"a", "b", "c"}
		prefix, endA, endB := TrimSliceBounds(a, b)
		if prefix != 3 {
			t.Errorf("expected prefix 3, got %d", prefix)
		}
		if prefix <= endA || prefix <= endB {
			t.Errorf("expected middle to be empty, got endA=%d, endB=%d", endA, endB)
		}
	})

	t.Run("CompletelyDifferent", func(t *testing.T) {
		a := []string{"a", "b"}
		b := []string{"x", "y", "z"}
		prefix, endA, endB := TrimSliceBounds(a, b)
		if prefix != 0 {
			t.Errorf("expected prefix 0, got %d", prefix)
		}
		if endA != 1 || endB != 2 {
			t.Errorf("expected endA=1, endB=2, got %d, %d", endA, endB)
		}
	})

	t.Run("EmptyInputs", func(t *testing.T) {
		var a, b []string
		prefix, endA, endB := TrimSliceBounds(a, b)
		if prefix != 0 || endA != -1 || endB != -1 {
			t.Errorf("expected 0, -1, -1, got %d, %d, %d", prefix, endA, endB)
		}

		a = []string{"x"}
		prefix, endA, endB = TrimSliceBounds(a, b)
		if prefix != 0 || endA != 0 || endB != -1 {
			t.Errorf("expected 0, 0, -1, got %d, %d, %d", prefix, endA, endB)
		}
	})

	t.Run("PrefixAndSuffixWithMiddle", func(t *testing.T) {
		a := []string{"head1", "head2", "midA1", "midA2", "tail1", "tail2"}
		b := []string{"head1", "head2", "midB1", "tail1", "tail2"}
		prefix, endA, endB := TrimSliceBounds(a, b)
		if prefix != 2 {
			t.Errorf("expected prefix 2, got %d", prefix)
		}
		if endA != 3 || endB != 2 {
			t.Errorf("expected endA=3, endB=2, got %d, %d", endA, endB)
		}
		if !slices.Equal(a[prefix:endA+1], []string{"midA1", "midA2"}) {
			t.Errorf("unexpected middle a: %v", a[prefix:endA+1])
		}
		if !slices.Equal(b[prefix:endB+1], []string{"midB1"}) {
			t.Errorf("unexpected middle b: %v", b[prefix:endB+1])
		}
	})

	t.Run("IndexBasedTrimBounds", func(t *testing.T) {
		src := []int{10, 20, 30, 40, 50}
		dst := []int{10, 20, 99, 50}
		prefix, endA, endB := TrimBounds(len(src), len(dst), func(i, j int) bool {
			return src[i] == dst[j]
		})
		if prefix != 2 {
			t.Errorf("expected prefix 2, got %d", prefix)
		}
		if endA != 3 || endB != 2 {
			t.Errorf("expected endA=3, endB=2, got endA=%d, endB=%d", endA, endB)
		}
	})
}

func TestCombineMatches(t *testing.T) {
	prefix := 2
	middle := [][2]int{{0, 1}, {2, 3}}
	endA := 4
	endB := 5
	lenA := 7

	combined := CombineMatches(nil, prefix, middle, endA, endB, lenA)
	expected := [][2]int{
		{0, 0},
		{1, 1},
		{2, 3},
		{4, 5},
		{5, 6},
		{6, 7},
	}

	if !slices.Equal(combined, expected) {
		t.Fatalf("expected %+v, got %+v", expected, combined)
	}
}

func TestSeqAlign_Uniform(t *testing.T) {
	scratch := AcquireSeqScratch()
	defer ReleaseSeqScratch(scratch)

	t.Run("Empty", func(t *testing.T) {
		matches := scratch.AlignUniform(0, 5, func(i, j int) bool { return true })
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
	})

	t.Run("SimpleLCS", func(t *testing.T) {
		a := []string{"A", "B", "C", "D"}
		b := []string{"A", "X", "C", "Y", "D"}
		matches := scratch.AlignUniform(len(a), len(b), func(i, j int) bool {
			return a[i] == b[j]
		})
		expected := [][2]int{{0, 0}, {2, 2}, {3, 4}}
		if !slices.Equal(matches, expected) {
			t.Errorf("expected %+v, got %+v", expected, matches)
		}
	})

	t.Run("NoMatches", func(t *testing.T) {
		a := []string{"1", "2"}
		b := []string{"3", "4"}
		matches := scratch.AlignUniform(len(a), len(b), func(i, j int) bool {
			return a[i] == b[j]
		})
		if len(matches) != 0 {
			t.Errorf("expected 0 matches, got %d", len(matches))
		}
	})
}

func TestSeqAlign_Weighted(t *testing.T) {
	scratch := AcquireSeqScratch()
	defer ReleaseSeqScratch(scratch)

	t.Run("CustomWeights", func(t *testing.T) {
		a := []string{"X", "Y"}
		b := []string{"Y", "X"}

		weight := func(i, j int) int {
			if a[i] == b[j] {
				if a[i] == "X" {
					return 10
				}
				return 1
			}
			return 0
		}

		matches := scratch.AlignWeighted(len(a), len(b), weight, false)
		expected := [][2]int{{0, 1}}
		if !slices.Equal(matches, expected) {
			t.Errorf("expected %+v, got %+v", expected, matches)
		}
	})

	t.Run("DiagonalTieBreaking", func(t *testing.T) {
		weight := func(i, j int) int {
			return 1
		}

		matches := scratch.AlignWeighted(1, 3, weight, true)
		expected := [][2]int{{0, 0}}
		if !slices.Equal(matches, expected) {
			t.Errorf("expected tie-break match %+v, got %+v", expected, matches)
		}
	})
}

func TestSeqAlign_ZeroAlloc(t *testing.T) {
	scratch := AcquireSeqScratch()
	defer ReleaseSeqScratch(scratch)

	a := []string{"func", "foo", "(", "x", "int", ")", "int", "{"}
	b := []string{"func", "foo", "(", "x", "int", ",", "y", "int", ")", "int", "{"}

	eq := func(i, j int) bool {
		return a[i] == b[j]
	}

	// Warm up
	_ = scratch.AlignUniform(len(a), len(b), eq)

	allocs := testing.AllocsPerRun(100, func() {
		_ = scratch.AlignUniform(len(a), len(b), eq)
	})
	if allocs > 0 {
		t.Fatalf("expected 0 allocs/op, got %f", allocs)
	}
}

func TestSeqScratch_PoolCapping(t *testing.T) {
	scratch := AcquireSeqScratch()
	hugeCells := MaxPooledSeqDPCapacity + 1000
	scratch.ensureDP(hugeCells)
	if cap(scratch.dp) < hugeCells {
		t.Fatalf("expected dp capacity >= %d, got %d", hugeCells, cap(scratch.dp))
	}

	ReleaseSeqScratch(scratch)
	if cap(scratch.dp) > MaxPooledSeqDPCapacity {
		t.Errorf("expected released scratch capacity <= %d, got %d", MaxPooledSeqDPCapacity, cap(scratch.dp))
	}
}
