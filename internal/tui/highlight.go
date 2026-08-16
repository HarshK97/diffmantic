package tui

import (
	"maps"
	"slices"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

// actionKind dictates how a line or span is colored in the TUI.
type actionKind int

const (
	kindDelete actionKind = iota
	kindInsert
	kindUpdate
	kindMove
	kindMoveUpdate
)

// span is a 0-indexed byte column range to highlight on a single line.
type span struct {
	startCol int
	endCol   int
	kind     actionKind
	action   *serialize.Action
}

// highlights tracks highlighted spans and changed lines for one pane.
type highlights struct {
	spans       map[int][]span
	tinted      map[int]actionKind
	changeLines []int // Sorted list of edited lines
}

func buildHighlights(leftSpans, rightSpans []serialize.HighlightSpan) (srcHL, dstHL *highlights) {
	return populateHL(leftSpans), populateHL(rightSpans)
}

func populateHL(hlSpans []serialize.HighlightSpan) *highlights {
	hl := &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}
	for _, s := range hlSpans {
		k := parseActionKind(s.Action)
		hl.spans[s.Line] = append(hl.spans[s.Line], span{
			startCol: s.StartCol,
			endCol:   s.EndCol,
			kind:     k,
			action:   s.ActionRef,
		})
		if existing, ok := hl.tinted[s.Line]; !ok || k < existing {
			hl.tinted[s.Line] = k
		}
	}
	hl.changeLines = slices.Sorted(maps.Keys(hl.tinted))
	return hl
}

func parseActionKind(act string) actionKind {
	switch act {
	case "delete":
		return kindDelete
	case "insert":
		return kindInsert
	case "update":
		return kindUpdate
	case "move":
		return kindMove
	default:
		return kindUpdate
	}
}
