package serialize

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// SchemaVersion defines the stable, versioned JSON output format version.
const SchemaVersion = "v1"

// LineAlignmentPair matches a source file line to a destination file line.
// A line index of -1 indicates a filler line.
type LineAlignmentPair struct {
	LeftLine  int `json:"left_line"`
	RightLine int `json:"right_line"`
}

// Envelope wraps the serialized actions list with a schema version.
type Envelope struct {
	Version         string              `json:"version"`
	Actions         []Action            `json:"actions"`
	LineAlignment   []LineAlignmentPair `json:"line_alignment,omitempty"`
	LeftHighlights  []HighlightSpan     `json:"left_highlights,omitempty"`
	RightHighlights []HighlightSpan     `json:"right_highlights,omitempty"`
}

// Action represents a serialized edit-script action.
// The absence of the "subtree" field always indicates false.
type Action struct {
	Action        string   `json:"action"` // "insert", "delete", "update", "move"
	Node          *NodeRef `json:"node"`
	Parent        *NodeRef `json:"parent,omitempty"`
	Position      *int     `json:"position,omitempty"`
	OldParent     *NodeRef `json:"old_parent,omitempty"`
	OldPosition   *int     `json:"old_position,omitempty"`
	OldValue      string   `json:"old_value,omitempty"`
	NewValue      string   `json:"new_value,omitempty"`
	Subtree       *bool    `json:"subtree,omitempty"`
	DestNode      *NodeRef `json:"dest_node,omitempty"`
	DestStartByte *uint32  `json:"dest_start_byte,omitempty"`
	DestEndByte   *uint32  `json:"dest_end_byte,omitempty"`
	GroupID       string   `json:"group_id,omitempty"`
}

// NodeRef is a stable and self-describing reference to an AST node.
type NodeRef struct {
	Tree      string `json:"tree"` // "before" or "after"
	Path      []int  `json:"path"`
	Type      string `json:"type"`
	Label     string `json:"label,omitempty"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
}

// BuildLineDiffEnvelope creates a line-level diff fallback when tree-sitter can't parse a file.
func BuildLineDiffEnvelope(srcBytes, dstBytes []byte) *Envelope {
	alignment := AlignLines(srcBytes, dstBytes, nil, nil, nil, nil)

	offsetsSrc := BuildLineIndex(srcBytes)
	offsetsDst := BuildLineIndex(dstBytes)

	getBounds := func(lineIdx int, offsets []int, maxLen int) (uint32, uint32) {
		end := uint32(maxLen)
		if lineIdx+1 < len(offsets) {
			end = uint32(offsets[lineIdx+1])
		}
		return uint32(offsets[lineIdx]), end
	}

	var actionsList []Action

	for _, pair := range alignment {
		if pair.RightLine == -1 && pair.LeftLine != -1 {
			start, end := getBounds(pair.LeftLine, offsetsSrc, len(srcBytes))
			actionsList = append(actionsList, Action{
				Action: "delete",
				Node: &NodeRef{
					Tree:      "before",
					StartByte: start,
					EndByte:   end,
				},
			})
		} else if pair.LeftLine == -1 && pair.RightLine != -1 {
			start, end := getBounds(pair.RightLine, offsetsDst, len(dstBytes))
			actionsList = append(actionsList, Action{
				Action: "insert",
				Node: &NodeRef{
					Tree:      "after",
					StartByte: start,
					EndByte:   end,
				},
			})
		}
	}

	env := &Envelope{
		Version:       SchemaVersion,
		Actions:       actionsList,
		LineAlignment: alignment,
	}
	env.LeftHighlights = BuildHighlightSpans(srcBytes, env.Actions, "left")
	env.RightHighlights = BuildHighlightSpans(dstBytes, env.Actions, "right")
	return env
}

// BuildEnvelope bundles the edit script, AST mappings, and metadata into a unified envelope.
func BuildEnvelope(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode, srcBytes, dstBytes []byte) (*Envelope, error) {
	if es == nil {
		return nil, fmt.Errorf("edit script is nil")
	}

	env := Envelope{
		Version:       SchemaVersion,
		Actions:       make([]Action, 0, es.Size()),
		LineAlignment: AlignLines(srcBytes, dstBytes, es, ms, srcRoot, dstRoot),
	}

	for _, a := range es.Actions() {
		var ja Action
		ja.Action = a.Type.String()

		switch a.Type {
		case actions.Insert:
			if a.Node == nil {
				return nil, fmt.Errorf("insert action has nil Node")
			}
			nodeRef, err := makeNodeRef(a.Node, "after")
			if err != nil {
				return nil, fmt.Errorf("failed to build node reference for insert: %w", err)
			}
			if !a.Subtree {
				adjustRangeForHeader(a.Node, &nodeRef.StartByte, &nodeRef.EndByte)
			}
			ja.Node = nodeRef

			if a.Node.Parent == nil {
				return nil, fmt.Errorf("inserted node %s has nil Parent", a.Node.Type)
			}
			parentRef, err := makeNodeRef(a.Node.Parent, "after")
			if err != nil {
				return nil, fmt.Errorf("failed to build parent reference for insert: %w", err)
			}
			ja.Parent = parentRef

			pos := a.Position
			ja.Position = &pos
			if a.Subtree {
				st := true
				ja.Subtree = &st
			}

		case actions.Delete:
			if a.Node == nil {
				return nil, fmt.Errorf("delete action has nil Node")
			}
			nodeRef, err := makeNodeRef(a.Node, "before")
			if err != nil {
				return nil, fmt.Errorf("failed to build node reference for delete: %w", err)
			}
			if !a.Subtree {
				adjustRangeForHeader(a.Node, &nodeRef.StartByte, &nodeRef.EndByte)
			}
			ja.Node = nodeRef
			if a.Subtree {
				st := true
				ja.Subtree = &st
			}

		case actions.Update:
			if a.Node == nil {
				return nil, fmt.Errorf("update action has nil Node")
			}
			nodeRef, err := makeNodeRef(a.Node, "before")
			if err != nil {
				return nil, fmt.Errorf("failed to build node reference for update: %w", err)
			}
			ja.Node = nodeRef
			ja.OldValue = a.Node.Label
			ja.NewValue = a.Value

			// Resolve the mapped destination node to reference in the after tree.
			if ms != nil {
				if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
					destRef, err := makeNodeRef(destNodeDst, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build dest reference for update: %w", err)
					}
					ja.DestNode = destRef
				}
			}

		case actions.Move:
			if a.Node == nil {
				return nil, fmt.Errorf("move action has nil Node")
			}
			nodeRef, err := makeNodeRef(a.Node, "before")
			if err != nil {
				return nil, fmt.Errorf("failed to build node reference for move: %w", err)
			}
			if !a.Subtree {
				adjustRangeForHeader(a.Node, &nodeRef.StartByte, &nodeRef.EndByte)
			}
			ja.Node = nodeRef

			if a.Parent == nil {
				return nil, fmt.Errorf("move action has nil Parent")
			}

			// Resolve parent in the destination (after) tree.
			var newParentDst *treesitter.ASTNode
			var pos int
			resolved := false

			if ms != nil {
				if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
					newParentDst = destNodeDst.Parent
					if p := destNodeDst.ChildIndex(); p >= 0 {
						pos = p
						resolved = true
					}
				}
			}

			if !resolved {
				// Fallback: keep the action's original parent and position.
				if a.Parent.Root() == dstRoot.Root() {
					newParentDst = a.Parent
				} else if ms != nil {
					newParentDst = ms.Src()[a.Parent]
				}
				pos = a.Position
			}

			if newParentDst == nil {
				return nil, fmt.Errorf("failed to resolve move parent %s in destination tree", a.Parent.Type)
			}

			parentRef, err := makeNodeRef(newParentDst, "after")
			if err != nil {
				return nil, fmt.Errorf("failed to build parent reference for move: %w", err)
			}
			ja.Parent = parentRef
			ja.Position = &pos

			// Old parent is a.Node.Parent in the before tree.
			if a.Node.Parent == nil {
				return nil, fmt.Errorf("moved node %s has nil old Parent", a.Node.Type)
			}
			oldParentRef, err := makeNodeRef(a.Node.Parent, "before")
			if err != nil {
				return nil, fmt.Errorf("failed to build old parent reference for move: %w", err)
			}
			ja.OldParent = oldParentRef

			oldPos := a.Node.ChildIndex()
			if oldPos == -1 {
				return nil, fmt.Errorf("moved node %s not found in its old parent's children", a.Node.Type)
			}
			ja.OldPosition = &oldPos

			if a.Subtree {
				st := true
				ja.Subtree = &st
			}

			// Resolve the mapped destination node for the destination byte range.
			if ms != nil {
				if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
					startByte := destNodeDst.StartByte
					endByte := destNodeDst.EndByte
					if !a.Subtree {
						adjustRangeForHeader(destNodeDst, &startByte, &endByte)
					}
					ja.DestStartByte = &startByte
					ja.DestEndByte = &endByte
				}
			}
		}

		if a.GroupID != "" {
			ja.GroupID = a.GroupID
		}

		env.Actions = append(env.Actions, ja)
	}

	env.LeftHighlights = BuildHighlightSpans(srcBytes, env.Actions, "left")
	env.RightHighlights = BuildHighlightSpans(dstBytes, env.Actions, "right")

	return &env, nil
}

// Marshal serializes the diff result into an indented JSON byte slice.
func Marshal(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode, srcBytes, dstBytes []byte) ([]byte, error) {
	env, err := BuildEnvelope(es, ms, srcRoot, dstRoot, srcBytes, dstBytes)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(env, "", "  ")
}

func makeNodeRef(n *treesitter.ASTNode, treeName string) (*NodeRef, error) {
	if n == nil {
		return nil, nil
	}
	path := getIndexPath(n)
	if path == nil && n.Parent != nil {
		return nil, fmt.Errorf("node %s of tree %s has broken parent link", n.Type, treeName)
	}
	return &NodeRef{
		Tree:      treeName,
		Path:      path,
		Type:      n.Type,
		Label:     n.Label,
		StartByte: n.StartByte,
		EndByte:   n.EndByte,
	}, nil
}

func getIndexPath(node *treesitter.ASTNode) []int {
	var path []int
	curr := node
	for curr.Parent != nil {
		idx := curr.ChildIndex()
		if idx == -1 {
			return nil
		}
		path = append(path, idx)
		curr = curr.Parent
	}
	slices.Reverse(path)
	return path
}

// adjustRangeForHeader limits the node's range to its header by cutting off at the first code block.
func adjustRangeForHeader(n *treesitter.ASTNode, start, end *uint32) {
	if n == nil || start == nil || end == nil {
		return
	}
	for _, child := range n.Children {
		if child.Type == "block" || child.Type == "statement_block" {
			if child.StartByte > *start && child.StartByte < *end {
				*end = child.StartByte
				break
			}
		}
	}
}
