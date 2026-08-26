package tui

import (
	"cmp"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/charmbracelet/lipgloss"
	"github.com/odvcencio/gotreesitter"
)

// syntaxSpan holds the visual color range for a single line.
type syntaxSpan struct {
	startCol int
	endCol   int
	color    lipgloss.Color
}

// highlightSyntax runs Tree-sitter on source and maps matches to per-line color spans. Returns nil if unsupported.
func highlightSyntax(filename string, source []byte, themeOpt ...*theme.Theme) map[int][]syntaxSpan {
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

	th := defaultTheme
	if len(themeOpt) > 0 {
		th = cmp.Or(themeOpt[0], defaultTheme)
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

	// Map byte offsets to line numbers.
	lineIndex := serialize.BuildLineIndex(source)

	result := make(map[int][]syntaxSpan)

	for _, r := range ranges {
		color := th.CaptureColor(r.Capture)
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
