package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// A styled foreground color span for a single line.
type syntaxSpan struct {
	startCol int
	endCol   int
	color    lipgloss.Color
}

// Parse a file with Tree-sitter and return per-line syntax color spans. Returns nil if the language isn't supported.
func highlightSyntax(filename string, source []byte) map[int][]syntaxSpan {
	if len(source) == 0 {
		return nil
	}

	base := filepath.Base(filename)
	entry := grammars.DetectLanguage(base)
	if entry == nil || entry.HighlightQuery == "" {
		return nil
	}

	lang := entry.Language()
	if lang == nil {
		return nil
	}

	var opts []gotreesitter.HighlighterOption
	if entry.TokenSourceFactory != nil {
		opts = append(opts, gotreesitter.WithTokenSourceFactory(func(src []byte) gotreesitter.TokenSource {
			return entry.TokenSourceFactory(src, lang)
		}))
	}

	h, err := gotreesitter.NewHighlighter(lang, entry.HighlightQuery, opts...)
	if err != nil {
		return nil
	}

	ranges := h.Highlight(source)
	if len(ranges) == 0 {
		return nil
	}

	// Index line start offsets for converting bytes to line numbers.
	lineIndex := buildLineIndex(source)

	result := make(map[int][]syntaxSpan)

	for _, r := range ranges {
		color := captureColor(r.Capture)
		if color == "" {
			continue
		}

		startLine, startCol := byteToLineCol(lineIndex, r.StartByte)
		endLine, endCol := byteToLineCol(lineIndex, r.EndByte)

		for line := startLine; line <= endLine; line++ {
			sc := 0
			if line == startLine {
				sc = startCol
			}

			var ec int
			if line == endLine {
				ec = endCol
			} else {
				// The span extends to the end of the line.
				if line+1 < len(lineIndex) {
					lineLen := lineIndex[line+1] - lineIndex[line]
					if lineLen > 0 {
						bytePos := lineIndex[line] + lineLen - 1
						if bytePos < len(source) && source[bytePos] == '\n' {
							lineLen--
						}
					}
					ec = lineLen
				} else {
					ec = len(source) - lineIndex[line]
				}
			}

			if ec > sc {
				result[line] = append(result[line], syntaxSpan{
					startCol: sc,
					endCol:   ec,
					color:    color,
				})
			}
		}
	}

	return result
}

// captureColor maps Tree-sitter capture names to Catppuccin Mocha foreground colors.
// It falls back to parent categories (e.g. "function.method" matches "function").
func captureColor(capture string) lipgloss.Color {
	// Try exact match first, then prefix.
	if c, ok := captureColorMap[capture]; ok {
		return c
	}

	// Prefix match: "function.method" → "function", "constant.builtin" → "constant"
	for {
		idx := strings.LastIndex(capture, ".")
		if idx < 0 {
			break
		}
		capture = capture[:idx]
		if c, ok := captureColorMap[capture]; ok {
			return c
		}
	}

	return ""
}

var captureColorMap = map[string]lipgloss.Color{
	// Keywords
	"keyword":     colorMauve,
	"conditional": colorMauve,
	"repeat":      colorMauve,
	"include":     colorMauve,
	"exception":   colorMauve,

	// Functions
	"function": colorBlue,
	"method":   colorBlue,

	// Strings and literals
	"string":  colorGreen,
	"escape":  colorPink,
	"number":  colorPeach,
	"boolean": colorPeach,
	"float":   colorPeach,

	// Types
	"type":        colorYellow,
	"constructor": colorYellow,

	// Constants
	"constant": colorPeach,

	// Variables and properties
	"variable":  "",
	"property":  colorLavender,
	"field":     colorLavender,
	"attribute": colorLavender,

	// Operators and punctuation
	"operator":    colorSky,
	"punctuation": colorOverlay0,

	// Comments
	"comment": colorOverlay0,

	// Tags (HTML/XML)
	"tag":      colorMauve,
	"embedded": colorRed,

	// Other
	"label":     colorTeal,
	"namespace": colorRosewater,
	"error":     colorRed,
}
