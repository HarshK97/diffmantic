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
const SchemaVersion = "1.0.0"

// Envelope wraps the serialized actions list with a schema version.
type Envelope struct {
	Version string   `json:"version"`
	Actions []Action `json:"actions"`
}

// Action represents a serialized edit-script action.
// The absence of the "subtree" field always indicates false.
type Action struct {
	Action      string   `json:"action"` // "insert", "delete", "update", "move"
	Node        *NodeRef `json:"node"`
	Parent      *NodeRef `json:"parent,omitempty"`
	Position    *int     `json:"position,omitempty"`
	OldParent   *NodeRef `json:"old_parent,omitempty"`
	OldPosition *int     `json:"old_position,omitempty"`
	OldValue    string   `json:"old_value,omitempty"`
	NewValue    string   `json:"new_value,omitempty"`
	Subtree       *bool    `json:"subtree,omitempty"`
	DestNode      *NodeRef `json:"dest_node,omitempty"`
	DestStartByte *uint32  `json:"dest_start_byte,omitempty"`
	DestEndByte   *uint32  `json:"dest_end_byte,omitempty"`
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

// Marshal converts an actions.EditScript and the matching engine.Mapping into a JSON byte slice.
func Marshal(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode) ([]byte, error) {
	if es == nil {
		return nil, fmt.Errorf("edit script is nil")
	}

	env := Envelope{
		Version: SchemaVersion,
		Actions: make([]Action, 0, es.Size()),
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

			// Resolve mapped destination node in target (after) tree
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
			ja.Node = nodeRef

			if a.Parent == nil {
				return nil, fmt.Errorf("move action has nil Parent")
			}

			// Resolve parent in the destination (after) tree.
			// a.Parent in memory could be in the before tree (if it was an existing parent)
			// or already in the after tree (if it was a newly inserted parent node).
			var newParentDst *treesitter.ASTNode
			if findRoot(a.Parent) == findRoot(dstRoot) {
				newParentDst = a.Parent
			} else if ms != nil {
				newParentDst = ms.Src()[a.Parent]
			}
			if newParentDst == nil {
				return nil, fmt.Errorf("failed to resolve move parent %s in destination tree", a.Parent.Type)
			}

			parentRef, err := makeNodeRef(newParentDst, "after")
			if err != nil {
				return nil, fmt.Errorf("failed to build parent reference for move: %w", err)
			}
			ja.Parent = parentRef
			pos := a.Position
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

			// Find old position in the before tree
			oldPos := -1
			for i, child := range a.Node.Parent.Children {
				if child == a.Node {
					oldPos = i
					break
				}
			}
			if oldPos == -1 {
				return nil, fmt.Errorf("moved node %s not found in its old parent's children", a.Node.Type)
			}
			ja.OldPosition = &oldPos

			if a.Subtree {
				st := true
				ja.Subtree = &st
			}

			// Resolve mapped destination node in target (after) tree for byte location
			if ms != nil {
				if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
					startByte := destNodeDst.StartByte
					endByte := destNodeDst.EndByte
					ja.DestStartByte = &startByte
					ja.DestEndByte = &endByte
				}
			}
		}

		env.Actions = append(env.Actions, ja)
	}

	return json.MarshalIndent(env, "", "  ")
}

// Unmarshal reconstructs an actions.EditScript from the JSON data, resolving node
// references against the provided source (before) and destination (after) AST roots.
func Unmarshal(data []byte, srcRoot, dstRoot *treesitter.ASTNode) (*actions.EditScript, error) {
	var env Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	es := actions.NewEditScript()

	for _, ja := range env.Actions {
		var a actions.Action

		switch ja.Action {
		case "insert":
			a.Type = actions.Insert
		case "delete":
			a.Type = actions.Delete
		case "update":
			a.Type = actions.Update
		case "move":
			a.Type = actions.Move
		default:
			return nil, fmt.Errorf("unknown action type: %q", ja.Action)
		}

		// Resolve Node
		if ja.Node != nil {
			var err error
			if ja.Node.Tree == "before" {
				a.Node, err = findNodeByPath(srcRoot, ja.Node.Path)
			} else if ja.Node.Tree == "after" {
				a.Node, err = findNodeByPath(dstRoot, ja.Node.Path)
			} else {
				return nil, fmt.Errorf("unknown tree name for node: %q", ja.Node.Tree)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to resolve node: %w", err)
			}
		}

		// Resolve Parent (if present)
		if ja.Parent != nil {
			var err error
			if ja.Parent.Tree == "before" {
				a.Parent, err = findNodeByPath(srcRoot, ja.Parent.Path)
			} else if ja.Parent.Tree == "after" {
				a.Parent, err = findNodeByPath(dstRoot, ja.Parent.Path)
			} else {
				return nil, fmt.Errorf("unknown tree name for parent: %q", ja.Parent.Tree)
			}
			if err != nil {
				return nil, fmt.Errorf("failed to resolve parent: %w", err)
			}
		}

		// Resolve Position
		if ja.Position != nil {
			a.Position = *ja.Position
		}

		// Resolve Value
		if ja.Action == "update" {
			a.Value = ja.NewValue
		}

		// Resolve Subtree
		if ja.Subtree != nil {
			a.Subtree = *ja.Subtree
		}

		es.Add(a)
	}

	return es, nil
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
		idx := -1
		for i, child := range curr.Parent.Children {
			if child == curr {
				idx = i
				break
			}
		}
		if idx == -1 {
			return nil
		}
		path = append(path, idx)
		curr = curr.Parent
	}
	slices.Reverse(path)
	return path
}

func findNodeByPath(root *treesitter.ASTNode, path []int) (*treesitter.ASTNode, error) {
	curr := root
	for _, idx := range path {
		if idx < 0 || idx >= len(curr.Children) {
			return nil, fmt.Errorf("invalid path index %d for node %s with %d children", idx, curr.Type, len(curr.Children))
		}
		curr = curr.Children[idx]
	}
	return curr, nil
}

func findRoot(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	curr := n
	for curr.Parent != nil {
		curr = curr.Parent
	}
	return curr
}
