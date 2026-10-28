// Package unified renders AST-aware unified diffs.
package unified

// RenderOptions controls how we render the unified diff.
type RenderOptions struct {
	// Color turns ANSI styling on or off.
	Color bool
	// ContextLines sets how many surrounding lines to show (default: 3).
	ContextLines int
	// LineNumbers shows source and destination line numbers in the gutter.
	LineNumbers bool
	// DisableAnnotations hides AST move and shift badges from line output.
	DisableAnnotations bool
	// SuppressHunkFallbackAnnotation stops hunk headers from repeating the fallback tag when the file banner already shows it.
	SuppressHunkFallbackAnnotation bool
	// Wrap enables soft line wrapping to fit the terminal width.
	Wrap bool
	// TerminalWidth is the wrap boundary (0 to auto-detect from the terminal).
	TerminalWidth int
	// TabWidth sets spaces per tab stop (default: 4).
	TabWidth int
	// GitDiff includes standard git diff, mode, and index headers.
	GitDiff bool
	// GitSrcOID overrides the source blob hash in the index line.
	GitSrcOID string
	// GitDstOID overrides the destination blob hash in the index line.
	GitDstOID string
	// FileMode is the octal file mode (default: "100644").
	FileMode string
}
