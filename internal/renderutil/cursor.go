// Package renderutil provides shared terminal rendering, highlighting, and hunk calculation primitives.
package renderutil

import (
	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
)

// SpanCursor provides zero-allocation streaming lookup over sorted disjoint spans.
type SpanCursor struct {
	spans []serialize.HighlightSpan
	idx   int
}

// NewSpanCursor initializes a cursor over pre-sorted, pairwise-disjoint spans.
func NewSpanCursor(spans []serialize.HighlightSpan) SpanCursor {
	return SpanCursor{spans: spans}
}

// ActionAt returns the highlight action active at byte offset col.
// col must not decrease between calls; the cursor never rewinds.
func (c *SpanCursor) ActionAt(col int) color.ActionKind {
	for c.idx < len(c.spans) && col >= c.spans[c.idx].EndCol {
		c.idx++
	}
	if c.idx < len(c.spans) && col >= c.spans[c.idx].StartCol {
		sp := c.spans[c.idx]
		return parseActionKind(sp.Action, sp.ColorIndex)
	}
	return color.ActionNone
}

// HasSpans reports whether the cursor contains any highlight spans.
func (c *SpanCursor) HasSpans() bool {
	return len(c.spans) > 0
}

func parseActionKind(action string, colorIndex int) color.ActionKind {
	switch action {
	case "delete", "DELETE":
		return color.ActionDelete
	case "insert", "INSERT":
		return color.ActionInsert
	case "update", "UPDATE":
		return color.ActionUpdate
	case "move", "MOVE":
		return color.MoveActionKindForSlot(colorIndex, false)
	case "move_update", "MOVE_UPDATE":
		return color.MoveActionKindForSlot(colorIndex, true)
	default:
		return color.ActionNone
	}
}
