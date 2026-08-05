package tui

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/charmbracelet/lipgloss"
	"github.com/odvcencio/gotreesitter"
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

	entry := treesitter.DetectGrammarEntry(filename)
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
	lineIndex := serialize.BuildLineIndex(source)

	result := make(map[int][]syntaxSpan)

	for _, r := range ranges {
		color := captureColor(r.Capture)
		if color == "" {
			continue
		}

		serialize.ForEachLineSpan(lineIndex, source, r.StartByte, r.EndByte, func(line, sc, ec int) {
			result[line] = append(result[line], syntaxSpan{
				startCol: sc,
				endCol:   ec,
				color:    color,
			})
		})
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
