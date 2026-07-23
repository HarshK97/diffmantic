package tui

import (
	"github.com/HarshK97/diffmantic/internal/serialize"
)

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
	lineAlignment []serialize.LineAlignmentPair
	allChanges    []int // Sorted list of all changed lines for n/N jumping

	digitBuffer string // Buffer for vim-style count prefixes
	pendingZ    bool   // Waiting for the second key in a 'z' combo

	folds        []fold
	virtualLines []virtualLine
	vchanges     []int // Change indices mapped to visible virtual lines

	cursorY    int    // index of active virtual line (0-indexed)
	cursorX    int    // horizontal column position in visual spaces (0-indexed)
	activePane string // "left" or "right"

	srcSyntax map[int][]syntaxSpan
	dstSyntax map[int][]syntaxSpan
}

const (
	titleBarHeight  = 1
	statusBarHeight = 1
	gutterPadding   = 1
	dividerWidth    = 1
	foldContext     = 3 // Unchanged lines to keep visible around changes
)
