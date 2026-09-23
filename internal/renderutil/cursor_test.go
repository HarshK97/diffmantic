package renderutil

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestSpanCursor_DisjointMapping(t *testing.T) {
	spans := []serialize.HighlightSpan{
		{StartCol: 4, EndCol: 8, Action: "update"},
		{StartCol: 12, EndCol: 16, Action: "move"},
		{StartCol: 20, EndCol: 22, Action: "delete"},
		{StartCol: 25, EndCol: 30, Action: "insert"},
		{StartCol: 35, EndCol: 40, Action: "move_update"},
	}

	cursor := NewSpanCursor(spans)
	if !cursor.HasSpans() {
		t.Errorf("expected HasSpans to report true")
	}

	tests := []struct {
		col  int
		want color.ActionKind
	}{
		// Before first span
		{col: 0, want: color.ActionNone},
		{col: 3, want: color.ActionNone},

		// First span: [4, 8) update
		{col: 4, want: color.ActionUpdate},
		{col: 6, want: color.ActionUpdate},
		{col: 7, want: color.ActionUpdate},
		{col: 8, want: color.ActionNone}, // EndCol is exclusive

		// Gap [8, 12)
		{col: 9, want: color.ActionNone},
		{col: 11, want: color.ActionNone},

		// Second span: [12, 16) move
		{col: 12, want: color.ActionMove},
		{col: 15, want: color.ActionMove},
		{col: 16, want: color.ActionNone},

		// Gap [16, 20)
		{col: 18, want: color.ActionNone},

		// Third span: [20, 22) delete
		{col: 20, want: color.ActionDelete},
		{col: 21, want: color.ActionDelete},
		{col: 22, want: color.ActionNone},

		// Fourth span: [25, 30) insert
		{col: 25, want: color.ActionInsert},
		{col: 29, want: color.ActionInsert},
		{col: 30, want: color.ActionNone},

		// Fifth span: [35, 40) move_update
		{col: 35, want: color.ActionMoveUpdate},
		{col: 39, want: color.ActionMoveUpdate},
		{col: 40, want: color.ActionNone},

		// Past end of all spans
		{col: 41, want: color.ActionNone},
		{col: 100, want: color.ActionNone},
	}

	for _, tt := range tests {
		got := cursor.ActionAt(tt.col)
		if got != tt.want {
			t.Errorf("ActionAt(%d) = %v, want %v", tt.col, got, tt.want)
		}
	}
}

func TestSpanCursor_EmptySpans(t *testing.T) {
	cursor := NewSpanCursor(nil)
	if cursor.HasSpans() {
		t.Errorf("expected HasSpans to report false for nil spans")
	}

	for col := range 20 {
		if got := cursor.ActionAt(col); got != color.ActionNone {
			t.Errorf("ActionAt(%d) on empty spans = %v, want ActionNone", col, got)
		}
	}
}

func TestSpanCursor_ZeroAllocation(t *testing.T) {
	spans := []serialize.HighlightSpan{
		{StartCol: 2, EndCol: 6, Action: "update"},
		{StartCol: 10, EndCol: 15, Action: "insert"},
	}

	allocs := testing.AllocsPerRun(100, func() {
		cursor := NewSpanCursor(spans)
		for col := range 20 {
			_ = cursor.ActionAt(col)
		}
	})

	if allocs != 0 {
		t.Errorf("expected 0 allocations in streaming SpanCursor loop, got %v", allocs)
	}
}
