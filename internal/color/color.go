// Package color defines static 16-color ANSI escape sequences and diff action classifications.
package color

// ActionKind represents the semantic category of an AST diff action or hunk line.
type ActionKind int

const (
	ActionNone ActionKind = iota - 1
	ActionDelete
	ActionInsert
	ActionUpdate
	ActionMove
	ActionMoveUpdate
	ActionMove1
	ActionMoveUpdate1
)

// String returns the canonical name for an action kind.
func (k ActionKind) String() string {
	switch k {
	case ActionNone:
		return "none"
	case ActionDelete:
		return "delete"
	case ActionInsert:
		return "insert"
	case ActionUpdate:
		return "update"
	case ActionMove, ActionMove1:
		return "move"
	case ActionMoveUpdate, ActionMoveUpdate1:
		return "move_update"
	default:
		return "unknown"
	}
}

// MoveActionKindForSlot maps a slot index (0, 1) and update flag to an ActionKind.
func MoveActionKindForSlot(slot int, isUpdate bool) ActionKind {
	slot = max(0, slot) % 2
	if isUpdate {
		if slot == 1 {
			return ActionMoveUpdate1
		}
		return ActionMoveUpdate
	}
	if slot == 1 {
		return ActionMove1
	}
	return ActionMove
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
	Move0Fg   = "\x1b[36m" // Slot 0: Cyan / Teal
	Move1Fg   = "\x1b[35m" // Slot 1: Magenta / Mauve
	UpdateFg  = "\x1b[33m"
	HeaderFg  = "\x1b[1;94m" // Bold Bright Blue
	OverlayFg = "\x1b[90m"
	SurfaceFg = "\x1b[90m"
	TextFg    = "\x1b[39m"
)

// movePaletteFg holds the colors used to tell adjacent or concurrent moves apart.
var movePaletteFg = [...]string{
	Move0Fg,
	Move1Fg,
}

// MoveFgForSlot returns the ANSI foreground sequence for a given move color slot.
func MoveFgForSlot(slot int) string {
	return movePaletteFg[max(0, slot)%len(movePaletteFg)]
}
