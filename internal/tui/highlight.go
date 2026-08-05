package tui

import (
	"bytes"
	"maps"
	"slices"
	"sort"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// actionKind is the type of edit (insert, delete, move, update) that dictates how we color the line.
type actionKind int

const (
	kindDelete actionKind = iota
	kindInsert
	kindUpdate
	kindMove
)

// span is a range of columns to highlight in a single line.
// Columns are 0-indexed byte offsets from the beginning of the line.
type span struct {
	startCol int
	endCol   int
	kind     actionKind
	action   *serialize.Action
}

// highlights tracks all highlighted spans and changed lines for one file.
type highlights struct {
	spans       map[int][]span
	tinted      map[int]actionKind
	changeLines []int // Sorted list of edited lines
}

// buildHighlights converts byte ranges from actions into inline highlights for the old and new files.
func buildHighlights(srcBytes, dstBytes []byte, actions []serialize.Action) (srcHL, dstHL *highlights) {
	srcHL = &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}
	dstHL = &highlights{
		spans:  make(map[int][]span),
		tinted: make(map[int]actionKind),
	}

	srcIndex := serialize.BuildLineIndex(srcBytes)
	dstIndex := serialize.BuildLineIndex(dstBytes)

	for i := range actions {
		a := &actions[i]
		switch a.Action {
		case "delete":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindDelete, a)
			}

		case "insert":
			if a.Node != nil {
				addHighlight(dstHL, dstIndex, dstBytes, a.Node.StartByte, a.Node.EndByte, kindInsert, a)
			}

		case "update":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindUpdate, a)
			}
			if a.DestNode != nil {
				addHighlight(dstHL, dstIndex, dstBytes, a.DestNode.StartByte, a.DestNode.EndByte, kindUpdate, a)
			}

		case "move":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindMove, a)
			}
			if a.DestStartByte != nil && a.DestEndByte != nil {
				addHighlight(dstHL, dstIndex, dstBytes, *a.DestStartByte, *a.DestEndByte, kindMove, a)
			}
		}
	}

	mergeAllSpans(srcHL, srcBytes)
	mergeAllSpans(dstHL, dstBytes)

	// Track edited lines so the user can jump between them with n/N.
	srcHL.changeLines = slices.Sorted(maps.Keys(srcHL.tinted))
	dstHL.changeLines = slices.Sorted(maps.Keys(dstHL.tinted))

	return srcHL, dstHL
}

// Break a byte range into line-by-line highlights.
func addHighlight(hl *highlights, lineIndex []int, fileBytes []byte, startByte, endByte uint32, kind actionKind, action *serialize.Action) {
	serialize.ForEachLineSpan(lineIndex, fileBytes, startByte, endByte, func(line, sc, ec int) {
		hl.spans[line] = append(hl.spans[line], span{startCol: sc, endCol: ec, kind: kind, action: action})
		if existing, ok := hl.tinted[line]; !ok || kind < existing {
			hl.tinted[line] = kind
		}
	})
}

func mergeAllSpans(hl *highlights, fileBytes []byte) {
	lineIndex := serialize.BuildLineIndex(fileBytes)
	for line, lineSpans := range hl.spans {
		if len(lineSpans) <= 1 {
			continue
		}

		// Sort spans by their starting columns.
		sort.Slice(lineSpans, func(i, j int) bool {
			return lineSpans[i].startCol < lineSpans[j].startCol
		})

		var merged []span
		curr := lineSpans[0]

		for i := 1; i < len(lineSpans); i++ {
			next := lineSpans[i]
			canMerge := false

			if curr.kind == next.kind {
				gap := next.startCol - curr.endCol
				if gap <= 3 {
					// Check if the gap contains only non-word characters like spaces or punctuation.
					onlyNonChars := true
					if gap > 0 && line < len(lineIndex) {
						lineStart := lineIndex[line]
						gapStart := lineStart + curr.endCol
						gapEnd := lineStart + next.startCol
						if gapStart < len(fileBytes) && gapEnd <= len(fileBytes) {
							onlyNonChars = isOnlyNonCharacters(fileBytes[gapStart:gapEnd])
						}
					}

					if onlyNonChars {
						// For updates and moves, make sure they share the same parents in the tree.
						if curr.kind == kindUpdate || curr.kind == kindMove {
							if curr.action != nil && next.action != nil {
								if curr.action.GroupID != "" && curr.action.GroupID == next.action.GroupID {
									canMerge = true
								} else if curr.kind == kindUpdate {
									canMerge = sharesLineage(curr.action.Parent, next.action.Parent)
								} else {
									// Moves must share both their original and destination lineage.
									canMerge = sharesLineage(curr.action.Parent, next.action.Parent) &&
										sharesLineage(curr.action.OldParent, next.action.OldParent)
								}
							}
						} else {
							// We can always merge inserts and deletes if there are no word characters in the gap.
							canMerge = true
						}
					}
				}
			}

			if canMerge {
				// Extend current span to cover the next one.
				if next.endCol > curr.endCol {
					curr.endCol = next.endCol
				}
			} else {
				merged = append(merged, curr)
				curr = next
			}
		}
		merged = append(merged, curr)
		hl.spans[line] = merged
	}
}

func isOnlyNonCharacters(b []byte) bool {
	if bytes.ContainsAny(b, "<>=+-*/%!&|^~?") {
		return false
	}
	for _, c := range b {
		if treesitter.IsWordChar(c) {
			return false
		}
	}
	return true
}

func nodeRefsEqual(n1, n2 *serialize.NodeRef) bool {
	if n1 == nil || n2 == nil {
		return n1 == nil && n2 == nil
	}
	return slices.Equal(n1.Path, n2.Path)
}

func isAncestorRef(a, b *serialize.NodeRef) bool {
	if a == nil || b == nil {
		return false
	}
	return len(b.Path) >= len(a.Path) && slices.Equal(a.Path, b.Path[:len(a.Path)])
}

func sharesLineage(n1, n2 *serialize.NodeRef) bool {
	return nodeRefsEqual(n1, n2) || isAncestorRef(n1, n2) || isAncestorRef(n2, n1)
}
