package serialize

import "sort"

// BuildLineIndex returns byte offsets for the start of each line.
func BuildLineIndex(data []byte) []int {
	index := []int{0}
	for i, b := range data {
		if b == '\n' {
			index = append(index, i+1)
		}
	}
	return index
}

// ByteToLineCol maps a byte offset to a zero-indexed line and column.
func ByteToLineCol(lineIndex []int, offset uint32) (line, col int) {
	if len(lineIndex) == 0 {
		return 0, int(offset)
	}
	off := int(offset)
	line = max(sort.Search(len(lineIndex), func(i int) bool {
		return lineIndex[i] > off
	})-1, 0)
	col = off - lineIndex[line]
	return line, col
}

// ForEachLineSpan runs yield for every line that intersects [startByte, endByte].
func ForEachLineSpan(lineIndex []int, fileBytes []byte, startByte, endByte uint32, yield func(line, startCol, endCol int)) {
	if startByte >= endByte || len(lineIndex) == 0 {
		return
	}
	startLine, startCol := ByteToLineCol(lineIndex, startByte)
	endLine, endCol := ByteToLineCol(lineIndex, endByte)

	for line := startLine; line <= endLine; line++ {
		sc := 0
		if line == startLine {
			sc = startCol
		}

		var ec int
		if line == endLine {
			ec = endCol
		} else if line+1 < len(lineIndex) {
			lineLen := lineIndex[line+1] - lineIndex[line]
			if lineLen > 0 {
				bytePos := lineIndex[line] + lineLen - 1
				if bytePos < len(fileBytes) && fileBytes[bytePos] == '\n' {
					lineLen--
				}
			}
			ec = lineLen
		} else {
			ec = len(fileBytes) - lineIndex[line]
		}

		if ec > sc {
			yield(line, sc, ec)
		}
	}
}
