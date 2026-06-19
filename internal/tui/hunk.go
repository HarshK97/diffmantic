package tui

import (
	"encoding/json"
)

// ChangeKind is the user-visible change type.
type ChangeKind int

const (
	ChangeInsert ChangeKind = iota // new code in T2
	ChangeDelete                   // removed code from T1
	ChangeUpdate                   // modified leaf value
	ChangeMove                     // subtree relocated
)

func (k ChangeKind) String() string {
	switch k {
	case ChangeInsert:
		return "insert"
	case ChangeDelete:
		return "delete"
	case ChangeUpdate:
		return "update"
	case ChangeMove:
		return "move"
	default:
		return "unknown"
	}
}

// MarshalJSON serializes ChangeKind as a string for JSON output.
func (k ChangeKind) MarshalJSON() ([]byte, error) {
	return json.Marshal(k.String())
}

// Hunk represents one contiguous region of change, ready for
// display in the TUI. All line numbers are 1-indexed.
type Hunk struct {
	Kind ChangeKind `json:"kind"`

	SrcStartLine int `json:"src_start_line,omitempty"`
	SrcEndLine   int `json:"src_end_line,omitempty"`

	DstStartLine int `json:"dst_start_line,omitempty"`
	DstEndLine   int `json:"dst_end_line,omitempty"`

	Summary string `json:"summary"`

	NodeType string `json:"node_type"`
}
