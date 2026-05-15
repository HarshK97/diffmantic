package output

import (
	"encoding/json"
	"fmt"
	"os"
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

// PrintHunks displays the coalesced hunks in a human-readable table.
func PrintHunks(hunks []Hunk) {
	if len(hunks) == 0 {
		fmt.Println("\n(no hunks)")
		return
	}

	fmt.Printf("\n%-4s  %-8s  %-15s  %-15s  %s\n",
		"#", "Kind", "File A Lines", "File B Lines", "Summary")
	fmt.Println("────────────────────────────────────────────────────────────────────────────────────")

	for i, h := range hunks {
		srcRange := "-"
		if h.SrcStartLine > 0 {
			if h.SrcStartLine == h.SrcEndLine {
				srcRange = fmt.Sprintf("L%d", h.SrcStartLine)
			} else {
				srcRange = fmt.Sprintf("L%d–L%d", h.SrcStartLine, h.SrcEndLine)
			}
		}

		dstRange := "-"
		if h.DstStartLine > 0 {
			if h.DstStartLine == h.DstEndLine {
				dstRange = fmt.Sprintf("L%d", h.DstStartLine)
			} else {
				dstRange = fmt.Sprintf("L%d–L%d", h.DstStartLine, h.DstEndLine)
			}
		}

		fmt.Printf("%-4d  %-8s  %-15s  %-15s  %s\n",
			i+1, h.Kind, srcRange, dstRange, h.Summary)
	}
	fmt.Printf("\nTotal hunks: %d\n", len(hunks))
}

// PrintHunksJSON writes the hunks as a JSON array to stdout.
func PrintHunksJSON(hunks []Hunk) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(hunks)
}
