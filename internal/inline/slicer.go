// Package inline prints the inline diff.
package inline

import (
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

type tokenInfo struct {
	byteLen      int
	displayWidth int
	isSpace      bool
}

func nextToken(s string, offset int) tokenInfo {
	if offset >= len(s) {
		return tokenInfo{}
	}
	r, runeLen := utf8.DecodeRuneInString(s[offset:])
	if r == ' ' {
		return tokenInfo{byteLen: runeLen, displayWidth: 1, isSpace: true}
	}
	if r == '\t' {
		return tokenInfo{byteLen: runeLen, displayWidth: 1, isSpace: true}
	}

	byteLen := 0
	displayWidth := 0
	curr := offset
	for curr < len(s) {
		ch, sz := utf8.DecodeRuneInString(s[curr:])
		if ch == ' ' || ch == '\t' {
			break
		}
		rw := runewidth.RuneWidth(ch)
		byteLen += sz
		displayWidth += rw
		curr += sz
		if (ch == ',' || ch == ';') && curr < len(s) {
			break
		}
	}
	return tokenInfo{byteLen: byteLen, displayWidth: displayWidth, isSpace: false}
}

func appendSGRTransition(b []byte, hKind int, isDeleteLine, isContextLine bool) []byte {
	b = append(b, color.Reset...)
	if isContextLine {
		return b
	}

	if hKind == -1 {
		if isDeleteLine {
			return append(b, color.DeleteFg...)
		}
		return append(b, color.InsertFg...)
	}

	switch color.ActionKind(hKind) {
	case color.ActionDelete:
		return append(b, color.DeleteFg...)
	case color.ActionInsert:
		return append(b, color.InsertFg...)
	case color.ActionUpdate:
		return append(b, color.Bold+color.UpdateFg...)
	case color.ActionMove:
		return append(b, color.Bold+color.MoveFg...)
	case color.ActionMoveUpdate:
		return append(b, color.Bold+color.Underline+color.UpdateFg...)
	default:
		return append(b, color.TextFg...)
	}
}

func formatContinuationGutter(numWidth int) string {
	blankPad := strings.Repeat(" ", numWidth)
	return blankPad + " " + blankPad + "  "
}

// sliceInlineLine wraps a single line to fit the terminal width, keeping
// highlights intact. It returns one chunk per visual row.
func sliceInlineLine(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	targetWidth int,
	tabWidth int,
	pane string,
	kind lineKind,
	colorMode bool,
	scratch *inlineScratch,
	peerDepicts bool,
) [][]byte {
	if targetWidth <= 0 {
		targetWidth = 10
	}
	if tabWidth <= 0 {
		tabWidth = 4
	}

	isDeleteLine := (kind == kindDelete)
	isContextLine := (kind == kindContext)

	var colHighlight []int
	hasFineGrained := false
	if !isContextLine && len(spans) > 0 {
		colHighlight, _ = resolveColHighlights(lineText, spans, pane, scratch)
		hasFineGrained = true
	}

	if peerDepicts && !hasFineGrained {
		isContextLine = true
	}

	firstNonWS := len(lineText) - len(strings.TrimLeft(lineText, " \t"))

	var chunks [][]byte
	var curChunk []byte
	curDisplayCol := 0

	maxTextWidth := targetWidth

	activeKind := -2 // -2 = nothing yet, -1 = plain text, >=0 = specific highlight
	byteOffset := 0

	finishChunk := func() {
		// Reset colors before we split the row.
		if colorMode && activeKind != -2 {
			curChunk = append(curChunk, color.Reset...)
		}

		chunks = append(chunks, curChunk)
		curChunk = nil
		curDisplayCol = 0
		activeKind = -2
	}

	for byteOffset < len(lineText) {
		tok := nextToken(lineText, byteOffset)
		if tok.byteLen == 0 {
			break
		}

		// handle a single space
		if tok.isSpace && lineText[byteOffset] == ' ' {
			if curDisplayCol+1 > maxTextWidth {
				// Space would overflow, wrap and drop any extra spaces/tabs
				// so the next row doesn't start with whitespace.
				finishChunk()
				for byteOffset < len(lineText) && (lineText[byteOffset] == ' ' || lineText[byteOffset] == '\t') {
					byteOffset++
				}
				continue
			}
			if colorMode {
				hKind := -1
				if byteOffset < len(colHighlight) {
					hKind = colHighlight[byteOffset]
				}
				if hasFineGrained && hKind == -1 {
					if activeKind != -2 {
						curChunk = append(curChunk, color.Reset...)
						activeKind = -2
					}
				} else if hKind != -1 || byteOffset >= firstNonWS {
					if hKind != activeKind {
						curChunk = appendSGRTransition(curChunk, hKind, isDeleteLine, isContextLine)
						activeKind = hKind
					}
				} else if activeKind != -2 && activeKind != -1 {
					curChunk = append(curChunk, color.Reset...)
					activeKind = -2
				}
			}
			curChunk = append(curChunk, ' ')
			curDisplayCol++
			byteOffset += tok.byteLen
			continue
		}

		// handle a tab
		if tok.isSpace && lineText[byteOffset] == '\t' {
			tabSpaces := tabWidth - (curDisplayCol % tabWidth)
			if tabSpaces == 0 {
				tabSpaces = tabWidth
			}

			if curDisplayCol+tabSpaces > maxTextWidth {
				// Tab doesn't fit — wrap and reset the tab stop.
				finishChunk()
				tabSpaces = tabWidth
			}

			if colorMode {
				hKind := -1
				if byteOffset < len(colHighlight) {
					hKind = colHighlight[byteOffset]
				}
				if hasFineGrained && hKind == -1 {
					if activeKind != -2 {
						curChunk = append(curChunk, color.Reset...)
						activeKind = -2
					}
				} else if hKind != -1 || byteOffset >= firstNonWS {
					if hKind != activeKind {
						curChunk = appendSGRTransition(curChunk, hKind, isDeleteLine, isContextLine)
						activeKind = hKind
					}
				} else if activeKind != -2 && activeKind != -1 {
					curChunk = append(curChunk, color.Reset...)
					activeKind = -2
				}
			}

			for range tabSpaces {
				curChunk = append(curChunk, ' ')
			}
			curDisplayCol += tabSpaces
			byteOffset += tok.byteLen
			continue
		}

		if curDisplayCol+tok.displayWidth > maxTextWidth {
			// Token doesn't fit on this row.
			if tok.displayWidth > targetWidth {
				// Too long for any row — cut it rune by rune.
				currRuneOffset := byteOffset
				for currRuneOffset < byteOffset+tok.byteLen {
					r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
					rWidth := max(1, runewidth.RuneWidth(r))

					if curDisplayCol+rWidth > maxTextWidth {
						finishChunk()
					}

					if colorMode {
						hKind := -1
						if currRuneOffset < len(colHighlight) {
							hKind = colHighlight[currRuneOffset]
						}
						if hasFineGrained && hKind == -1 {
							if activeKind != -2 {
								curChunk = append(curChunk, color.Reset...)
								activeKind = -2
							}
						} else if hKind != activeKind {
							curChunk = appendSGRTransition(curChunk, hKind, isDeleteLine, isContextLine)
							activeKind = hKind
						}
					}

					curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
					curDisplayCol += rWidth
					currRuneOffset += rLen
				}
				byteOffset += tok.byteLen
				continue
			}

			// It fits on the next row — wrap at the word boundary.
			finishChunk()
		}

		// Paint the token with its highlight.
		currRuneOffset := byteOffset
		for currRuneOffset < byteOffset+tok.byteLen {
			r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
			rWidth := max(1, runewidth.RuneWidth(r))

			if colorMode {
				hKind := -1
				if currRuneOffset < len(colHighlight) {
					hKind = colHighlight[currRuneOffset]
				}
				if hasFineGrained && hKind == -1 {
					if activeKind != -2 {
						curChunk = append(curChunk, color.Reset...)
						activeKind = -2
					}
				} else if hKind != activeKind {
					curChunk = appendSGRTransition(curChunk, hKind, isDeleteLine, isContextLine)
					activeKind = hKind
				}
			}

			curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
			curDisplayCol += rWidth
			currRuneOffset += rLen
		}
		byteOffset += tok.byteLen
	}

	// Flush what's left.
	if len(chunks) == 0 || curDisplayCol > 0 || len(curChunk) > 0 {
		finishChunk()
	}

	// Badge goes at the end of the whole line (last visual row), not the
	// first — that way it doesn't steal width from the first row and end
	// up mid-sentence.
	if badgeText != "" {
		if len(chunks) > 0 {
			last := len(chunks) - 1
			if colorMode {
				chunks[last] = append(chunks[last], color.Italic+color.OverlayFg+badgeText+color.Reset...)
			} else {
				chunks[last] = append(chunks[last], badgeText...)
			}
		} else {
			var b []byte
			if colorMode {
				b = append(b, color.Italic+color.OverlayFg+badgeText+color.Reset...)
			} else {
				b = append(b, badgeText...)
			}
			chunks = append(chunks, b)
		}
	}

	return chunks
}
