// Package inline prints the inline diff.
package inline

// RenderOptions controls how we render the inline diff.
type RenderOptions struct {
	// Color turns ANSI colors on or off.
	Color bool
	// ContextLines is how much context we show — defaults to 3 lines.
	ContextLines int
	// LineNumbers shows line numbers in the gutter.
	LineNumbers bool
	// DisableAnnotations hides the little move badges.
	DisableAnnotations bool
	// Wrap turns on soft wrapping to fit the terminal.
	Wrap bool
	// TerminalWidth is the wrap column — 0 means auto-detect or off.
	TerminalWidth int
	// TabWidth is spaces per tab — defaults to 4.
	TabWidth int
}
