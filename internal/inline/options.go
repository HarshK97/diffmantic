// Package inline renders AST-aware inline diffs.
package inline

// RenderOptions tunes how the inline diff is rendered.
type RenderOptions struct {
	// Color toggles ANSI color codes in the output.
	Color bool
	// ContextLines sets how many surrounding unchanged lines to show (defaults to 3).
	ContextLines int
	// LineNumbers prints line numbers in a side-by-side gutter.
	LineNumbers bool
	// DisableAnnotations suppresses AST move and token move ghost text annotations.
	DisableAnnotations bool
}
