// Package sidebyside renders aligned two-pane terminal diffs.
package sidebyside

// RenderOptions defines configuration for side-by-side rendering.
type RenderOptions struct {
	TerminalWidth      int  // Detected or overridden terminal width (default 0 = auto)
	ContextLines       int  // Context lines around hunks (default 3, -1 for full file)
	LineNumbers        bool // Show line number gutters
	Color              bool // Enable TrueColor ANSI escapes
	DisableAnnotations bool // Suppress AST move annotations and badges
	TabWidth           int  // Spaces per tab stop (default 4)
	ForceSideBySide    bool // Disable hybrid single-column layout and force 50/50 dual-column view
}

// DefaultOptions returns standard settings for interactive terminals.
func DefaultOptions() RenderOptions {
	return RenderOptions{
		TerminalWidth:      0,
		ContextLines:       3,
		LineNumbers:        true,
		Color:              true,
		DisableAnnotations: false,
		TabWidth:           4,
		ForceSideBySide:    false,
	}
}
