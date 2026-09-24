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
	ActionMove2
	ActionMoveUpdate2
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
	case ActionMove, ActionMove1, ActionMove2:
		return "move"
	case ActionMoveUpdate, ActionMoveUpdate1, ActionMoveUpdate2:
		return "move_update"
	default:
		return "unknown"
	}
}

// MoveActionKindForSlot maps a slot index (0, 1, 2) and update flag to an ActionKind.
func MoveActionKindForSlot(slot int, isUpdate bool) ActionKind {
	slot = max(0, slot) % 3
	if isUpdate {
		switch slot {
		case 1:
			return ActionMoveUpdate1
		case 2:
			return ActionMoveUpdate2
		default:
			return ActionMoveUpdate
		}
	}
	switch slot {
	case 1:
		return ActionMove1
	case 2:
		return ActionMove2
	default:
		return ActionMove
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
	Move0Fg   = "\x1b[36m" // Slot 0: Cyan / Teal
	Move1Fg   = "\x1b[35m" // Slot 1: Magenta / Mauve
	Move2Fg   = "\x1b[94m" // Slot 2: Bright Blue / Sapphire
	UpdateFg  = "\x1b[33m"
	HeaderFg  = "\x1b[35m"
	OverlayFg = "\x1b[90m"
	SurfaceFg = "\x1b[90m"
	TextFg    = "\x1b[39m"
)

// movePaletteFg holds the colors used to tell adjacent or concurrent moves apart.
var movePaletteFg = [...]string{
	Move0Fg,
	Move1Fg,
	Move2Fg,
}

// MoveFgForSlot returns the ANSI foreground sequence for a given move color slot.
func MoveFgForSlot(slot int) string {
	switch max(0, slot) % len(movePaletteFg) {
	case 1:
		return Move1Fg
	case 2:
		return Move2Fg
	default:
		return Move0Fg
	}
}
