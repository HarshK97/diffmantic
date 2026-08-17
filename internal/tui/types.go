package tui

import (
	"sync"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/charmbracelet/bubbles/textinput"
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

	digitBuffer    string // Buffer for vim-style count prefixes
	pendingZ       bool   // Waiting for the second key in a 'z' combo
	pendingBracket string // Waiting for second key in a '[' or ']' combo

	folds        []fold
	virtualLines []virtualLine
	vchanges     []int // Change indices mapped to visible virtual lines

	cursorY    int    // index of active virtual line (0-indexed)
	cursorX    int    // horizontal column position in visual spaces (0-indexed)
	activePane string // "left" or "right"

	srcSyntax map[int][]syntaxSpan
	dstSyntax map[int][]syntaxSpan

	inspectOpen    bool                // whether the expanded inspect panel is visible
	inspectActions []*serialize.Action // actions at the current cursor position

	searchActive   bool
	searchQuery    string
	searchMatches  []searchMatch
	searchMatchIdx int
	textinput      textinput.Model

	helpOpen bool

	// Git integration fields
	gitMode            bool
	gitItems           []gitTreeItem
	gitCursorY         int
	gitTreeOpen        bool
	gitCommitOpen      bool
	gitCommitInput     textinput.Model
	gitSelectedFileIdx int
	repoPath           string
	refA               string
	refB               string
	gitStagedOnly      bool
	pathFilter         string
	conflictWarning    string
	gitCache           *gitCache
}

// gitCacheKey uniquely identifies a cached diff entry by path and staged state.
type gitCacheKey struct {
	path     string
	isStaged bool
}

// gitCache coordinates thread-safe diff caching, worker pool concurrency (sem),
// and epoch-based invalidation to drop lagging background diff computations.
type gitCache struct {
	mu      sync.RWMutex
	epoch   uint64
	sem     chan struct{}
	entries map[gitCacheKey]gitDiffCacheEntry
}

type gitTreeItem struct {
	isHeader   bool
	headerText string

	// File details (if not header)
	path      string
	oldPath   string
	status    string
	rawStatus string
	isStaged  bool
}

type gitDiffCacheEntry struct {
	srcBytes []byte
	dstBytes []byte
	env      *serialize.Envelope
	isBinary bool
	computed bool
}

const (
	titleBarHeight     = 1
	statusBarHeight    = 1
	gutterPadding      = 4
	dividerWidth       = 1
	foldContext        = 3 // Unchanged lines to keep visible around changes
	inspectPanelHeight = 4
)
