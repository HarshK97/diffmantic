package renderutil

import (
	"reflect"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestBuildChangeIntervals(t *testing.T) {
	tests := []struct {
		name          string
		isPairChanged []bool
		want          []Interval
	}{
		{
			name:          "Empty input",
			isPairChanged: nil,
			want:          nil,
		},
		{
			name:          "No changes",
			isPairChanged: []bool{false, false, false},
			want:          nil,
		},
		{
			name:          "Single changed pair",
			isPairChanged: []bool{true},
			want:          []Interval{{Start: 0, End: 0}},
		},
		{
			name:          "Contiguous change in middle",
			isPairChanged: []bool{false, true, true, true, false},
			want:          []Interval{{Start: 1, End: 3}},
		},
		{
			name:          "Multiple separated changes",
			isPairChanged: []bool{true, false, false, true, true, false, true},
			want:          []Interval{{Start: 0, End: 0}, {Start: 3, End: 4}, {Start: 6, End: 6}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildChangeIntervals(tt.isPairChanged)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("BuildChangeIntervals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMergeHunks(t *testing.T) {
	tests := []struct {
		name            string
		changeIntervals []Interval
		totalPairs      int
		contextLines    int
		want            []Interval
	}{
		{
			name:            "Empty change intervals",
			changeIntervals: nil,
			totalPairs:      10,
			contextLines:    3,
			want:            nil,
		},
		{
			name:            "Single interval expansion",
			changeIntervals: []Interval{{Start: 5, End: 7}},
			totalPairs:      20,
			contextLines:    3,
			want:            []Interval{{Start: 2, End: 10}},
		},
		{
			name:            "Clamping to bounds 0 and totalPairs-1",
			changeIntervals: []Interval{{Start: 1, End: 2}},
			totalPairs:      5,
			contextLines:    3,
			want:            []Interval{{Start: 0, End: 4}},
		},
		{
			name: "Overlapping hunks merge into one",
			changeIntervals: []Interval{
				{Start: 2, End: 3},
				{Start: 5, End: 6},
			},
			totalPairs:   20,
			contextLines: 2,
			// Hunk 0: [0, 5], Hunk 1: [3, 8] -> overlap merges to [0, 8]
			want: []Interval{{Start: 0, End: 8}},
		},
		{
			name: "Adjacent hunks merge",
			changeIntervals: []Interval{
				{Start: 2, End: 2},
				{Start: 5, End: 5},
			},
			totalPairs:   20,
			contextLines: 1,
			// Hunk 0: [1, 3], Hunk 1: [4, 6] -> adjacent (3+1 == 4) merges to [1, 6]
			want: []Interval{{Start: 1, End: 6}},
		},
		{
			name: "Well-separated hunks stay separate",
			changeIntervals: []Interval{
				{Start: 2, End: 3},
				{Start: 15, End: 16},
			},
			totalPairs:   20,
			contextLines: 2,
			want: []Interval{
				{Start: 0, End: 5},
				{Start: 13, End: 18},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeHunks(tt.changeIntervals, tt.totalPairs, tt.contextLines)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergeHunks() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeHunkRange(t *testing.T) {
	pairs := []serialize.LineAlignmentPair{
		{LeftLine: 0, RightLine: 0},
		{LeftLine: 1, RightLine: -1},
		{LeftLine: 2, RightLine: -1},
		{LeftLine: -1, RightLine: 1},
		{LeftLine: 3, RightLine: 2},
	}

	// Hunk covering middle changes [1..3]
	h := Interval{Start: 1, End: 3}
	srcStart, srcCount, dstStart, dstCount := ComputeHunkRange(h, pairs, true, true)
	if srcStart != 2 || srcCount != 2 {
		t.Errorf("expected srcStart=2, srcCount=2, got srcStart=%d, srcCount=%d", srcStart, srcCount)
	}
	if dstStart != 2 || dstCount != 1 {
		t.Errorf("expected dstStart=2, dstCount=1, got dstStart=%d, dstCount=%d", dstStart, dstCount)
	}

	// Pure addition hunk with no left lines
	pureAddPairs := []serialize.LineAlignmentPair{
		{LeftLine: -1, RightLine: 0},
		{LeftLine: -1, RightLine: 1},
	}
	hAdd := Interval{Start: 0, End: 1}
	srcStart, srcCount, dstStart, dstCount = ComputeHunkRange(hAdd, pureAddPairs, false, true)
	if srcStart != 0 || srcCount != 0 {
		t.Errorf("expected srcStart=0, srcCount=0, got srcStart=%d, srcCount=%d", srcStart, srcCount)
	}
	if dstStart != 1 || dstCount != 2 {
		t.Errorf("expected dstStart=1, dstCount=2, got dstStart=%d, dstCount=%d", dstStart, dstCount)
	}
}

func TestFormatHunkHeader(t *testing.T) {
	if got := FormatRange(1, 1); got != "1" {
		t.Errorf("FormatRange(1, 1) = %q, want '1'", got)
	}
	if got := FormatRange(1, 5); got != "1,5" {
		t.Errorf("FormatRange(1, 5) = %q, want '1,5'", got)
	}
	if got := FormatRange(1, 0); got != "1,0" {
		t.Errorf("FormatRange(1, 0) = %q, want '1,0'", got)
	}

	header := FormatHunkHeader(1, 5, 1, 6, " func Foo()")
	want := "@@ -1,5 +1,6 @@ func Foo()"
	if header != want {
		t.Errorf("FormatHunkHeader = %q, want %q", header, want)
	}
}
