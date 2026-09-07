package tui

import (
	"github.com/HarshK97/diffmantic/internal/theme"
	"github.com/charmbracelet/lipgloss"
)

// syntaxSpan holds the visual color range for a single line.
type syntaxSpan struct {
	startCol int
	endCol   int
	color    lipgloss.Color
}

// highlightSyntax runs Tree-sitter on source and maps matches to per-line color spans. Returns nil if unsupported.
func highlightSyntax(filename string, source []byte, themeOpt ...*theme.Theme) map[int][]syntaxSpan {
	return nil
}
