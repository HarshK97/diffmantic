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

// buildHighlights converts serialized actions into inline highlights for both panes.
func buildHighlights(srcBytes, dstBytes []byte, actions []serialize.Action) (srcHL, dstHL *highlights) {
	srcHL = &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}
	dstHL = &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}

	leftSpans := serialize.BuildHighlightSpans(srcBytes, actions, "left")
	rightSpans := serialize.BuildHighlightSpans(dstBytes, actions, "right")

	for _, s := range leftSpans {
		k := parseActionKind(s.Action)
		srcHL.spans[s.Line] = append(srcHL.spans[s.Line], span{
			startCol: s.StartCol,
			endCol:   s.EndCol,
			kind:     k,
			action:   s.ActionRef,
		})
		if existing, ok := srcHL.tinted[s.Line]; !ok || k < existing {
			srcHL.tinted[s.Line] = k
		}
	}

	for _, s := range rightSpans {
		k := parseActionKind(s.Action)
		dstHL.spans[s.Line] = append(dstHL.spans[s.Line], span{
			startCol: s.StartCol,
			endCol:   s.EndCol,
			kind:     k,
			action:   s.ActionRef,
		})
		if existing, ok := dstHL.tinted[s.Line]; !ok || k < existing {
			dstHL.tinted[s.Line] = k
		}
	}

	// Track edited lines so the user can jump between them with n/N.
	srcHL.changeLines = slices.Sorted(maps.Keys(srcHL.tinted))
	dstHL.changeLines = slices.Sorted(maps.Keys(dstHL.tinted))

	return srcHL, dstHL
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
