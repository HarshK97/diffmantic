package tui

import (
	"sort"

	"github.com/HarshK97/diffmantic/internal/serialize"
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

	srcIndex := buildLineIndex(srcBytes)
	dstIndex := buildLineIndex(dstBytes)

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
	srcHL.changeLines = sortedKeys(srcHL.tinted)
	dstHL.changeLines = sortedKeys(dstHL.tinted)

	return srcHL, dstHL
}

// Split a byte range into line-by-line highlights. Ranges can cross line boundaries.
func addHighlight(hl *highlights, lineIndex []int, fileBytes []byte, startByte, endByte uint32, kind actionKind, action *serialize.Action) {
	if startByte >= endByte {
		return
	}
	startLine, startCol := byteToLineCol(lineIndex, startByte)
	endLine, endCol := byteToLineCol(lineIndex, endByte)

	for line := startLine; line <= endLine; line++ {
		var sc, ec int

		if line == startLine {
			sc = startCol
		} else {
			sc = 0
		}

		if line == endLine {
			ec = endCol
		} else {
			if line < len(lineIndex)-1 {
				lineLen := lineIndex[line+1] - lineIndex[line]
				// Don't highlight the trailing newline so we avoid coloring empty space.
				if lineLen > 0 && line+1 < len(lineIndex) {
					bytePos := lineIndex[line] + lineLen - 1
					if bytePos < len(fileBytes) && fileBytes[bytePos] == '\n' {
						lineLen--
					}
				}
				ec = lineLen
			} else {
				ec = len(fileBytes) - lineIndex[line]
			}
		}

		if ec > sc {
			hl.spans[line] = append(hl.spans[line], span{startCol: sc, endCol: ec, kind: kind, action: action})
		}

		// If a line has multiple changes, color the whole line using the most important one.
		if existing, ok := hl.tinted[line]; !ok || kind < existing {
			hl.tinted[line] = kind
		}
	}
}

// buildLineIndex maps out where each line starts in the file.
func buildLineIndex(data []byte) []int {
	index := []int{0}
	for i, b := range data {
		if b == '\n' {
			index = append(index, i+1)
		}
	}
	return index
}

// byteToLineCol converts a global byte offset into a 0-indexed line and column.
func byteToLineCol(lineIndex []int, offset uint32) (line, col int) {
	off := int(offset)
	line = sort.Search(len(lineIndex), func(i int) bool {
		return lineIndex[i] > off
	}) - 1
	if line < 0 {
		line = 0
	}
	col = off - lineIndex[line]
	return line, col
}

func sortedKeys(m map[int]actionKind) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func mergeAllSpans(hl *highlights, fileBytes []byte) {
	lineIndex := buildLineIndex(fileBytes)
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
				if gap <= 5 {
					// Check if the gap is just spaces, tabs, or punctuation.
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
								if curr.kind == kindUpdate {
									canMerge = nodeRefsEqual(curr.action.Parent, next.action.Parent)
								} else {
									// Moves need to share both the old and new parents.
									canMerge = nodeRefsEqual(curr.action.Parent, next.action.Parent) &&
										nodeRefsEqual(curr.action.OldParent, next.action.OldParent)
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
	for _, c := range b {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			return false
		}
	}
	return true
}

func nodeRefsEqual(n1, n2 *serialize.NodeRef) bool {
	if n1 == nil && n2 == nil {
		return true
	}
	if n1 == nil || n2 == nil {
		return false
	}
	if len(n1.Path) != len(n2.Path) {
		return false
	}
	for i := range n1.Path {
		if n1.Path[i] != n2.Path[i] {
			return false
		}
	}
	return true
}
