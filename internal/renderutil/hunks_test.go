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

func TestMaxVisibleLine(t *testing.T) {
	t.Run("EmptyHunksOrPairs", func(t *testing.T) {
		if got := MaxVisibleLine(nil, nil); got != 0 {
			t.Errorf("expected 0 for nil input, got %d", got)
		}
	})

	t.Run("ExtractsMaxAcrossHunks", func(t *testing.T) {
		pairs := []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
			{LeftLine: 1, RightLine: 1},
			{LeftLine: 2, RightLine: 2},
			{LeftLine: 3, RightLine: 3},
			{LeftLine: 4, RightLine: -1},
			{LeftLine: -1, RightLine: 4},
			{LeftLine: 50, RightLine: 52},
		}
		// Hunk 1 covers pairs 0..2 (max line 3)
		hunk1 := []Interval{{Start: 0, End: 2}}
		if got := MaxVisibleLine(hunk1, pairs); got != 3 {
			t.Errorf("expected max visible line 3, got %d", got)
		}

		// Hunk 2 covers pairs 6..6 (max line 53)
		hunk2 := []Interval{{Start: 0, End: 2}, {Start: 6, End: 6}}
		if got := MaxVisibleLine(hunk2, pairs); got != 53 {
			t.Errorf("expected max visible line 53, got %d", got)
		}
	})
}

func TestLineDisplayWidth(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		tabWidth int
		want     int
	}{
		{
			name:     "ASCII plain text",
			text:     "func A() int {",
			tabWidth: 4,
			want:     14,
		},
		{
			name:     "Leading tab with tabWidth 4",
			text:     "\treturn 1",
			tabWidth: 4,
			want:     12, // 4 spaces + 8 chars
		},
		{
			name:     "Multiple tabs with tabWidth 8",
			text:     "\ta\tb",
			tabWidth: 8,
			want:     17, // 8 (tab 1) + 1 ('a') + 7 (tab 2 to col 16) + 1 ('b') = 17
		},
		{
			name:     "Full-width CJK characters",
			text:     "你好世界",
			tabWidth: 4,
			want:     8, // 4 runes * 2 width = 8
		},
		{
			name:     "Trailing CRLF stripped",
			text:     "val = 42\r\n",
			tabWidth: 4,
			want:     8,
		},
		{
			name:     "Negative or zero tabWidth defaults to 4",
			text:     "\tx",
			tabWidth: 0,
			want:     5, // 4 spaces + 1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LineDisplayWidth(tt.text, tt.tabWidth)
			if got != tt.want {
				t.Errorf("LineDisplayWidth(%q, %d) = %d, want %d", tt.text, tt.tabWidth, got, tt.want)
			}
		})
	}
}

func TestMaxVisibleLineDisplayWidth(t *testing.T) {
	t.Run("EmptyHunksOrPairs", func(t *testing.T) {
		if got := MaxVisibleLineDisplayWidth(nil, nil, nil, nil, nil, nil, 4); got != 0 {
			t.Errorf("expected 0 for empty input, got %d", got)
		}
	})

	t.Run("InspectsVisibleHunksOnly", func(t *testing.T) {
		srcLines := []string{
			"short", // line 0 (width 5)
			"very long line in file that is outside hunk", // line 1 (width 43)
			"medium line", // line 2 (width 11)
		}
		dstLines := []string{
			"short new", // line 0 (width 9)
			"very long line in file that is outside hunk", // line 1 (width 43)
			"medium line modified",                        // line 2 (width 20)
		}
		pairs := []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
			{LeftLine: 1, RightLine: 1},
			{LeftLine: 2, RightLine: 2},
		}

		// Hunk only covers pair 0: max width is max(5, 9) = 9
		hunk0 := []Interval{{Start: 0, End: 0}}
		if got := MaxVisibleLineDisplayWidth(hunk0, pairs, srcLines, dstLines, nil, nil, 4); got != 9 {
			t.Errorf("expected max width 9, got %d", got)
		}

		// Hunk covers pairs 0 and 2: max width is max(5, 9, 11, 20) = 20
		hunk0And2 := []Interval{{Start: 0, End: 0}, {Start: 2, End: 2}}
		if got := MaxVisibleLineDisplayWidth(hunk0And2, pairs, srcLines, dstLines, nil, nil, 4); got != 20 {
			t.Errorf("expected max width 20, got %d", got)
		}
	})

	t.Run("IncludesBadgeWidth", func(t *testing.T) {
		srcLines := []string{"stmt := 1"} // width 9
		dstLines := []string{"stmt := 1"} // width 9
		pairs := []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
		}
		hunks := []Interval{{Start: 0, End: 0}}

		srcBadges := map[int]string{0: " ➔ L40"} // width 6
		dstBadges := map[int]string{0: " ↳ L10"} // width 6

		got := MaxVisibleLineDisplayWidth(hunks, pairs, srcLines, dstLines, srcBadges, dstBadges, 4)
		// 9 + 6 = 15
		if got != 15 {
			t.Errorf("expected max width 15 with badge, got %d", got)
		}
	})

	t.Run("NegativeHunkStartDoesNotPanic", func(t *testing.T) {
		srcLines := []string{"foo"}
		dstLines := []string{"bar"}
		pairs := []serialize.LineAlignmentPair{
			{LeftLine: 0, RightLine: 0},
		}
		negativeHunk := []Interval{{Start: -5, End: 0}}
		if got := MaxVisibleLineDisplayWidth(negativeHunk, pairs, srcLines, dstLines, nil, nil, 4); got != 3 {
			t.Errorf("expected max width 3, got %d", got)
		}
	})
}
