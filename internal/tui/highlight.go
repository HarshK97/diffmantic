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

	for _, a := range actions {
		switch a.Action {
		case "delete":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindDelete)
			}

		case "insert":
			if a.Node != nil {
				addHighlight(dstHL, dstIndex, dstBytes, a.Node.StartByte, a.Node.EndByte, kindInsert)
			}

		case "update":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindUpdate)
			}
			if a.DestNode != nil {
				addHighlight(dstHL, dstIndex, dstBytes, a.DestNode.StartByte, a.DestNode.EndByte, kindUpdate)
			}

		case "move":
			if a.Node != nil {
				addHighlight(srcHL, srcIndex, srcBytes, a.Node.StartByte, a.Node.EndByte, kindMove)
			}
			if a.DestStartByte != nil && a.DestEndByte != nil {
				addHighlight(dstHL, dstIndex, dstBytes, *a.DestStartByte, *a.DestEndByte, kindMove)
			}
		}
	}

	// Track edited lines so the user can jump between them with n/N.
	srcHL.changeLines = sortedKeys(srcHL.tinted)
	dstHL.changeLines = sortedKeys(dstHL.tinted)

	return srcHL, dstHL
}

// addHighlight converts a raw byte range (which can cover multiple lines) into individual line highlights.
func addHighlight(hl *highlights, lineIndex []int, fileBytes []byte, startByte, endByte uint32, kind actionKind) {
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
			hl.spans[line] = append(hl.spans[line], span{startCol: sc, endCol: ec, kind: kind})
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

// mergedChangeLines combines and sorts edited lines from both files to let us step through them.
func mergedChangeLines(src, dst *highlights) []int {
	seen := make(map[int]bool)
	for _, l := range src.changeLines {
		seen[l] = true
	}
	for _, l := range dst.changeLines {
		seen[l] = true
	}
	merged := make([]int, 0, len(seen))
	for l := range seen {
		merged = append(merged, l)
	}
	sort.Ints(merged)
	return merged
}
