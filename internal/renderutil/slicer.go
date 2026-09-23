package renderutil

import (
	"bytes"
	"strings"
	"unicode/utf8"

	"github.com/HarshK97/diffmantic/internal/color"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/mattn/go-runewidth"
)

// BadgeAnchor controls where the move badge is visually anchored on wrapped lines.
type BadgeAnchor uint8

const (
	// BadgeAnchorRow0 docks the badge on visual Chunk 0 and deducts badge width upfront (Side-by-Side).
	BadgeAnchorRow0 BadgeAnchor = iota
	// BadgeAnchorLastRow appends the badge to the final visual chunk (Inline).
	BadgeAnchorLastRow
)

// LineContext distinguishes paired aligned lines from single-sided additions or deletions.
type LineContext uint8

const (
	// LineContextAligned represents an aligned line pair (LeftLine >= 0 && RightLine >= 0).
	LineContextAligned LineContext = iota
	// LineContextStandaloneDelete represents an unpaired deletion (LeftLine >= 0 && RightLine == -1).
	LineContextStandaloneDelete
	// LineContextStandaloneInsert represents an unpaired insertion (LeftLine == -1 && RightLine >= 0).
	LineContextStandaloneInsert
)

// SliceConfig configures line slicing, wrapping, and highlight styling.
type SliceConfig struct {
	TargetWidth      int
	TabWidth         int
	ColorMode        bool
	PadToTargetWidth bool
	BadgeAnchor      BadgeAnchor
	Context          LineContext
}

// Slicer formats and slices lines into display chunks with syntax and action highlighting.
type Slicer struct {
	curChunk []byte
}

type tokenInfo struct {
	byteLen      int
	displayWidth int
	isSpace      bool
}

const (
	styleUnset  color.ActionKind = -99
	styleLeadWS color.ActionKind = -98
	stylePlain  color.ActionKind = -1
)

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

func resolveStyle(offset int, firstNonWS int, cursor *SpanCursor, cfg SliceConfig) color.ActionKind {
	spanKind := cursor.ActionAt(offset)
	if spanKind != color.ActionNone {
		return spanKind
	}
	if offset < firstNonWS {
		return styleLeadWS
	}
	switch cfg.Context {
	case LineContextStandaloneDelete:
		return color.ActionDelete
	case LineContextStandaloneInsert:
		return color.ActionInsert
	default:
		return stylePlain
	}
}

func appendTransition(b []byte, targetStyle, activeStyle color.ActionKind) []byte {
	if targetStyle == styleLeadWS {
		if activeStyle != styleUnset && activeStyle != styleLeadWS {
			return append(b, color.Reset...)
		}
		return b
	}

	b = append(b, color.Reset...)
	switch targetStyle {
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
	default: // stylePlain
		return append(b, color.TextFg...)
	}
}

// SliceLineToChunks formats a single line of text with syntax/action spans into display rows.
func (s *Slicer) SliceLineToChunks(
	lineText string,
	badgeText string,
	spans []serialize.HighlightSpan,
	cfg SliceConfig,
) [][]byte {
	lineText = strings.TrimSuffix(lineText, "\r")

	targetWidth := cfg.TargetWidth
	if cfg.PadToTargetWidth && targetWidth <= 0 {
		targetWidth = 10
	}

	tabWidth := cfg.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}

	firstNonWS := len(lineText) - len(strings.TrimLeft(lineText, " \t"))

	badgeWidth := 0
	if badgeText != "" {
		badgeWidth = runewidth.StringWidth(badgeText)
	}

	maxTextWidth := targetWidth
	if cfg.BadgeAnchor == BadgeAnchorRow0 && targetWidth > 0 && badgeWidth > 0 {
		maxTextWidth = targetWidth - badgeWidth
		if maxTextWidth <= 0 {
			maxTextWidth = targetWidth
			badgeText = ""
			badgeWidth = 0
		}
	}

	var chunks [][]byte
	curChunk := s.curChunk[:0]
	curDisplayCol := 0
	activeStyle := styleUnset
	cursor := NewSpanCursor(spans)
	byteOffset := 0

	finishChunk := func(isFirstChunk bool) {
		if cfg.ColorMode && activeStyle != styleUnset && activeStyle != styleLeadWS {
			curChunk = append(curChunk, color.Reset...)
			activeStyle = styleUnset
		}

		if isFirstChunk && cfg.BadgeAnchor == BadgeAnchorRow0 && badgeText != "" {
			if cfg.ColorMode {
				curChunk = append(curChunk, color.MoveFg+badgeText+color.Reset...)
			} else {
				curChunk = append(curChunk, badgeText...)
			}
			curDisplayCol += badgeWidth
		}

		if cfg.PadToTargetWidth && targetWidth > 0 {
			if cfg.ColorMode && activeStyle != styleUnset {
				curChunk = append(curChunk, color.Reset...)
				activeStyle = styleUnset
			}
			for curDisplayCol < targetWidth {
				curChunk = append(curChunk, ' ')
				curDisplayCol++
			}
		}

		chunks = append(chunks, bytes.Clone(curChunk))
		curChunk = curChunk[:0]
		curDisplayCol = 0
		maxTextWidth = targetWidth
		activeStyle = styleUnset
	}

	for byteOffset < len(lineText) {
		tok := nextToken(lineText, byteOffset)
		if tok.byteLen == 0 {
			break
		}

		// Fast path: a space is always one display column wide.
		if tok.isSpace && lineText[byteOffset] == ' ' {
			if maxTextWidth > 0 && curDisplayCol+1 > maxTextWidth {
				finishChunk(len(chunks) == 0)
				activeStyle = styleUnset
				if !cfg.PadToTargetWidth {
					for byteOffset < len(lineText) && (lineText[byteOffset] == ' ' || lineText[byteOffset] == '\t') {
						byteOffset++
					}
				} else {
					byteOffset += tok.byteLen
				}
				continue
			}

			targetStyle := resolveStyle(byteOffset, firstNonWS, &cursor, cfg)
			if cfg.ColorMode && targetStyle != activeStyle {
				curChunk = appendTransition(curChunk, targetStyle, activeStyle)
				activeStyle = targetStyle
			}
			curChunk = append(curChunk, ' ')
			curDisplayCol++
			byteOffset += tok.byteLen
			continue
		}

		// Expand tabs to the next tab stop using the current column offset.
		if tok.isSpace && lineText[byteOffset] == '\t' {
			if targetWidth <= 0 {
				targetStyle := resolveStyle(byteOffset, firstNonWS, &cursor, cfg)
				if cfg.ColorMode && targetStyle != activeStyle {
					curChunk = appendTransition(curChunk, targetStyle, activeStyle)
					activeStyle = targetStyle
				}
				curChunk = append(curChunk, '\t')
				byteOffset += tok.byteLen
				continue
			}

			tabSpaces := tabWidth - (curDisplayCol % tabWidth)

			if maxTextWidth > 0 && curDisplayCol+tabSpaces > maxTextWidth {
				finishChunk(len(chunks) == 0)
				activeStyle = styleUnset
				tabSpaces = tabWidth
			}

			targetStyle := resolveStyle(byteOffset, firstNonWS, &cursor, cfg)
			if cfg.ColorMode && targetStyle != activeStyle {
				curChunk = appendTransition(curChunk, targetStyle, activeStyle)
				activeStyle = targetStyle
			}

			for range tabSpaces {
				curChunk = append(curChunk, ' ')
			}
			curDisplayCol += tabSpaces
			byteOffset += tok.byteLen
			continue
		}

		if maxTextWidth > 0 && curDisplayCol+tok.displayWidth > maxTextWidth {
			if tok.displayWidth > targetWidth {
				// Break mid-word rune-by-rune when the token is wider than the entire column.
				currRuneOffset := byteOffset
				for currRuneOffset < byteOffset+tok.byteLen {
					r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
					rWidth := max(1, runewidth.RuneWidth(r))

					if maxTextWidth > 0 && curDisplayCol+rWidth > maxTextWidth {
						finishChunk(len(chunks) == 0)
						activeStyle = styleUnset
					}

					targetStyle := resolveStyle(currRuneOffset, firstNonWS, &cursor, cfg)
					if cfg.ColorMode && targetStyle != activeStyle {
						curChunk = appendTransition(curChunk, targetStyle, activeStyle)
						activeStyle = targetStyle
					}

					curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
					curDisplayCol += rWidth
					currRuneOffset += rLen
				}
				byteOffset += tok.byteLen
				continue
			}

			// Overflow: push token to the next row and start fresh.
			finishChunk(len(chunks) == 0)
			activeStyle = styleUnset
		}

		currRuneOffset := byteOffset
		for currRuneOffset < byteOffset+tok.byteLen {
			r, rLen := utf8.DecodeRuneInString(lineText[currRuneOffset:])
			rWidth := max(1, runewidth.RuneWidth(r))

			targetStyle := resolveStyle(currRuneOffset, firstNonWS, &cursor, cfg)
			if cfg.ColorMode && targetStyle != activeStyle {
				curChunk = appendTransition(curChunk, targetStyle, activeStyle)
				activeStyle = targetStyle
			}

			curChunk = append(curChunk, lineText[currRuneOffset:currRuneOffset+rLen]...)
			curDisplayCol += rWidth
			currRuneOffset += rLen
		}
		byteOffset += tok.byteLen
	}

	if len(chunks) == 0 || curDisplayCol > 0 || len(curChunk) > 0 {
		finishChunk(len(chunks) == 0)
	}

	if cfg.BadgeAnchor == BadgeAnchorLastRow && badgeText != "" {
		var badgeBytes []byte
		if cfg.ColorMode {
			badgeBytes = []byte(color.MoveFg + badgeText + color.Reset)
		} else {
			badgeBytes = []byte(badgeText)
		}
		if len(chunks) > 0 {
			last := len(chunks) - 1
			chunks[last] = append(chunks[last], badgeBytes...)
		} else {
			chunks = append(chunks, badgeBytes)
		}
	}

	s.curChunk = curChunk[:0]
	return chunks
}
