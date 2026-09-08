// Package color defines static 16-color ANSI escape sequences and diff action classifications.
package color

// ActionKind represents the semantic category of an AST diff action or hunk line.
type ActionKind int

const (
	ActionDelete ActionKind = iota
	ActionInsert
	ActionUpdate
	ActionMove
	ActionMoveUpdate
)

// String returns the canonical name for an action kind.
func (k ActionKind) String() string {
	switch k {
	case ActionDelete:
		return "delete"
	case ActionInsert:
		return "insert"
	case ActionUpdate:
		return "update"
	case ActionMove:
		return "move"
	case ActionMoveUpdate:
		return "move_update"
	default:
		return "unknown"
	}
}

// Terminal text formatting escape sequences.
const (
	Reset     = "\x1b[0m"
	Bold      = "\x1b[1m"
	Dim       = "\x1b[2m"
	Italic    = "\x1b[3m"
	Underline = "\x1b[4m"
)

// Standard 16-color ANSI foreground escape sequences.
const (
	InsertFg  = "\x1b[32m"
	DeleteFg  = "\x1b[31m"
	MoveFg    = "\x1b[36m"
	UpdateFg  = "\x1b[33m"
	HeaderFg  = "\x1b[35m"
	OverlayFg = "\x1b[90m"
	SurfaceFg = "\x1b[90m"
	TextFg    = "\x1b[39m"
)
