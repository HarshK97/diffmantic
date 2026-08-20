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
	return populateHL(leftSpans, "left"), populateHL(rightSpans, "right")
}

func populateHL(hlSpans []serialize.HighlightSpan, pane string) *highlights {
	hl := &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}
	lineSmallestLen := make(map[int]int)
	for _, s := range hlSpans {
		k := parseActionKind(s.Action)
		sp := span{
			startCol: s.StartCol,
			endCol:   s.EndCol,
			kind:     k,
			action:   s.ActionRef,
		}
		hl.spans[s.Line] = append(hl.spans[s.Line], sp)

		astLen := getSpanASTLen(sp, pane)
		bestLen, ok := lineSmallestLen[s.Line]
		if !ok || astLen < bestLen || (astLen == bestLen && actionPriority(k) > actionPriority(hl.tinted[s.Line])) {
			lineSmallestLen[s.Line] = astLen
			hl.tinted[s.Line] = k
		}
	}
	hl.changeLines = slices.Sorted(maps.Keys(hl.tinted))
	return hl
}

func getSpanASTLen(s span, pane string) int {
	if s.action != nil {
		if pane == "left" {
			if s.action.Node != nil {
				return int(s.action.Node.EndByte - s.action.Node.StartByte)
			}
		} else {
			if s.action.DestStartByte != nil && s.action.DestEndByte != nil {
				return int(*s.action.DestEndByte - *s.action.DestStartByte)
			}
			if s.action.DestNode != nil {
				return int(s.action.DestNode.EndByte - s.action.DestNode.StartByte)
			}
			if s.action.Node != nil {
				return int(s.action.Node.EndByte - s.action.Node.StartByte)
			}
		}
	}
	return s.endCol - s.startCol
}

func actionPriority(k actionKind) int {
	switch k {
	case kindMoveUpdate:
		return 5
	case kindUpdate:
		return 4
	case kindInsert:
		return 3
	case kindMove:
		return 2
	case kindDelete:
		return 1
	default:
		return 0
	}
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
