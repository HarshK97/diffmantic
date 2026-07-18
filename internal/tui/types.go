package tui

type model struct {
	width  int
	height int
	ready  bool

	srcFile  string
	dstFile  string
	srcLines []string
	dstLines []string

	scrollY      int
	scrollXLeft  int
	scrollXRight int

	srcHighlights *highlights
	dstHighlights *highlights
	allChanges    []int // Sorted list of modified lines so we can jump between them using n/N

	digitBuffer string // count prefix for Vim keys
}

const (
	titleBarHeight  = 1
	statusBarHeight = 1
	gutterPadding   = 1 // padding between line numbers and text
	dividerWidth    = 1 // width of the vertical pane separator
)
