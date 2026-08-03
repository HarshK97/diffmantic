package engine

import (
	"reflect"
	"testing"
)

func TestLineDiff(t *testing.T) {
	tests := []struct {
		name     string
		linesA   []string
		linesB   []string
		expected map[int]int
	}{
		{
			name:     "basic match with insertions and deletions",
			linesA:   []string{"a", "b", "c", "d"},
			linesB:   []string{"a", "x", "c", "y", "d"},
			expected: map[int]int{0: 0, 2: 2, 3: 4},
		},
		{
			name:     "identical files",
			linesA:   []string{"one", "two", "three"},
			linesB:   []string{"one", "two", "three"},
			expected: map[int]int{0: 0, 1: 1, 2: 2},
		},
		{
			name:     "completely different files",
			linesA:   []string{"a", "b"},
			linesB:   []string{"x", "y"},
			expected: map[int]int{},
		},
		{
			name:     "empty inputs",
			linesA:   []string{},
			linesB:   []string{},
			expected: map[int]int{},
		},
		{
			name:     "prefix and suffix trim with middle changes",
			linesA:   []string{"head", "mid_old_1", "mid_old_2", "tail"},
			linesB:   []string{"head", "mid_new_1", "tail"},
			expected: map[int]int{0: 0, 3: 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := LineDiff(tt.linesA, tt.linesB)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("LineDiff() = %v, want %v", got, tt.expected)
			}
		})
	}
}
