package serialize

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// SchemaVersion defines the stable, versioned JSON output format version.
const SchemaVersion = "v1"

// LineAlignmentPair matches a source file line to a destination file line.
// A line index of -1 indicates a filler line.
type LineAlignmentPair struct {
	LeftLine  int `json:"left_line"`
	RightLine int `json:"right_line"`
}

func (p LineAlignmentPair) MarshalJSON() ([]byte, error) {
	return json.Marshal([2]int{p.LeftLine, p.RightLine})
}

func (p *LineAlignmentPair) UnmarshalJSON(b []byte) error {
	var arr [2]int
	if err := json.Unmarshal(b, &arr); err != nil {
		return fmt.Errorf("invalid line alignment pair tuple: %w", err)
	}
	p.LeftLine = arr[0]
	p.RightLine = arr[1]
	return nil
}

// EnvelopeOptions selects which sections to include in the JSON envelope.
type EnvelopeOptions struct {
	IncludeActions    bool
	IncludeAlignment  bool
	IncludeHighlights bool
}

// Envelope wraps the serialized actions list with a schema version.
type Envelope struct {
	Version         string              `json:"version"`
	Actions         []Action            `json:"actions,omitempty"`
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
	Type      string `json:"type"`
	Label     string `json:"label,omitempty"`
	StartByte uint32 `json:"start_byte"`
	EndByte   uint32 `json:"end_byte"`
}

// BuildLineDiffEnvelopeWithOptions builds a fallback line-diff envelope when tree-sitter parsing fails or is skipped.
func BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes []byte, opts EnvelopeOptions) *Envelope {
	env := &Envelope{
		Version: SchemaVersion,
	}

	alignment := AlignLines(srcBytes, dstBytes)
	if opts.IncludeAlignment {
		env.LineAlignment = alignment
	}

	if opts.IncludeActions || opts.IncludeHighlights {
		offsetsSrc := BuildLineIndex(srcBytes)
		offsetsDst := BuildLineIndex(dstBytes)

		getBounds := func(lineIdx int, offsets []int, maxLen int) (uint32, uint32) {
			end := uint32(maxLen)
			if lineIdx+1 < len(offsets) {
				end = uint32(offsets[lineIdx+1])
			}
			return uint32(offsets[lineIdx]), end
		}

		srcLines := strings.Split(string(srcBytes), "\n")
		dstLines := strings.Split(string(dstBytes), "\n")

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
			} else if pair.LeftLine != -1 && pair.RightLine != -1 {
				// Paired lines with differing text turn into a delete on the left and insert on the right.
				if pair.LeftLine < len(srcLines) && pair.RightLine < len(dstLines) && srcLines[pair.LeftLine] != dstLines[pair.RightLine] {
					startSrc, endSrc := getBounds(pair.LeftLine, offsetsSrc, len(srcBytes))
					startDst, endDst := getBounds(pair.RightLine, offsetsDst, len(dstBytes))
					actionsList = append(actionsList, Action{
						Action: "delete",
						Node: &NodeRef{
							Tree:      "before",
							StartByte: startSrc,
							EndByte:   endSrc,
						},
					})
					actionsList = append(actionsList, Action{
						Action: "insert",
						Node: &NodeRef{
							Tree:      "after",
							StartByte: startDst,
							EndByte:   endDst,
						},
					})
				}
			}
		}

		if opts.IncludeActions {
			env.Actions = actionsList
		}

		if opts.IncludeHighlights {
			env.LeftHighlights = BuildHighlightSpans(srcBytes, actionsList, "left")
			env.RightHighlights = BuildHighlightSpans(dstBytes, actionsList, "right")
		}
	}

	return env
}

// BuildEnvelopeWithOptions packages the edit script, AST mappings, and UI metadata into an Envelope based on opts.
func BuildEnvelopeWithOptions(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode, srcBytes, dstBytes []byte, opts EnvelopeOptions) (*Envelope, error) {
	if es == nil {
		return nil, fmt.Errorf("edit script is nil")
	}

	env := Envelope{
		Version: SchemaVersion,
	}

	if opts.IncludeAlignment {
		env.LineAlignment = AlignLines(srcBytes, dstBytes)
	}

	var actionsList []Action
	type pendingFooter struct {
		span        DelimiterSpan
		actionIndex int
	}
	var leftFooters []pendingFooter
	var rightFooters []pendingFooter
	if opts.IncludeActions || opts.IncludeHighlights {
		actionsList = make([]Action, 0, es.Size())
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
					hasFooter, fStart, fEnd := adjustRangeForContainer(a.Node, &nodeRef.StartByte, &nodeRef.EndByte, dstBytes)
					if hasFooter {
						rightFooters = append(rightFooters, pendingFooter{
							span: DelimiterSpan{
								StartByte: fStart,
								EndByte:   fEnd,
								Action:    "insert",
							},
							actionIndex: len(actionsList),
						})
					}
				}
				ja.Node = nodeRef

				if a.Node.Parent != nil {
					parentRef, err := makeNodeRef(a.Node.Parent, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build parent reference for insert: %w", err)
					}
					ja.Parent = parentRef
				}

				ja.Position = new(a.Position)
				if a.Subtree {
					ja.Subtree = new(true)
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
					hasFooter, fStart, fEnd := adjustRangeForContainer(a.Node, &nodeRef.StartByte, &nodeRef.EndByte, srcBytes)
					if hasFooter {
						leftFooters = append(leftFooters, pendingFooter{
							span: DelimiterSpan{
								StartByte: fStart,
								EndByte:   fEnd,
								Action:    "delete",
							},
							actionIndex: len(actionsList),
						})
					}
				}
				ja.Node = nodeRef

				if a.Node.Parent != nil {
					parentRef, err := makeNodeRef(a.Node.Parent, "before")
					if err != nil {
						return nil, fmt.Errorf("failed to build parent reference for delete: %w", err)
					}
					ja.Parent = parentRef
				}

				ja.Position = new(a.Position)
				if a.Subtree {
					ja.Subtree = new(true)
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

				var destNodeDst *treesitter.ASTNode
				if a.DestNode != nil {
					destNodeDst = a.DestNode
				} else if ms != nil {
					destNodeDst = ms.Src()[a.Node]
				}

				if destNodeDst != nil {
					destRef, err := makeNodeRef(destNodeDst, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build dest_node reference for update: %w", err)
					}
					ja.DestNode = destRef
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
					hasLeftFooter, lfStart, lfEnd := adjustRangeForContainer(a.Node, &nodeRef.StartByte, &nodeRef.EndByte, srcBytes)
					if hasLeftFooter {
						leftFooters = append(leftFooters, pendingFooter{
							span: DelimiterSpan{
								StartByte: lfStart,
								EndByte:   lfEnd,
								Action:    "move",
							},
							actionIndex: len(actionsList),
						})
					}
				}
				ja.Node = nodeRef

				parent := a.Parent
				if parent == nil && a.DestNode != nil && a.DestNode.Parent != nil {
					parent = a.DestNode.Parent
				}

				if parent == nil {
					return nil, fmt.Errorf("move action for node %s has nil Parent", a.Node.Type)
				}

				var newParentDst *treesitter.ASTNode
				if dstRoot != nil && parent.Root() == dstRoot.Root() {
					newParentDst = parent
				} else if ms != nil {
					newParentDst = ms.Src()[parent]
				}

				if newParentDst == nil && ms != nil && ms.Src() != nil {
					curr := parent
					for curr != nil {
						if mapped := ms.Src()[curr]; mapped != nil {
							newParentDst = mapped
							break
						}
						curr = curr.Parent
					}
				}

				if newParentDst == nil {
					if a.DestNode != nil && a.DestNode.Parent != nil {
						newParentDst = a.DestNode.Parent
					} else {
						return nil, fmt.Errorf("failed to resolve move parent %s in destination tree", parent.Type)
					}
				}

				parentRef, err := makeNodeRef(newParentDst, "after")
				if err != nil {
					return nil, fmt.Errorf("failed to build parent reference for move: %w", err)
				}
				ja.Parent = parentRef
				ja.Position = new(a.Position)

				if a.Node.Parent != nil {
					oldParentRef, err := makeNodeRef(a.Node.Parent, "before")
					if err != nil {
						return nil, fmt.Errorf("failed to build old parent reference for move: %w", err)
					}
					ja.OldParent = oldParentRef
				}

				oldPos := a.Node.ChildIndex()
				if oldPos == -1 {
					oldPos = 0
				}
				ja.OldPosition = new(oldPos)

				if a.Subtree {
					ja.Subtree = new(true)
				}

				if a.DestNode != nil {
					startByte := a.DestNode.StartByte
					endByte := a.DestNode.EndByte
					if !a.Subtree {
						hasRightFooter, rfStart, rfEnd := adjustRangeForContainer(a.DestNode, &startByte, &endByte, dstBytes)
						if hasRightFooter {
							rightFooters = append(rightFooters, pendingFooter{
								span: DelimiterSpan{
									StartByte: rfStart,
									EndByte:   rfEnd,
									Action:    "move",
								},
								actionIndex: len(actionsList),
							})
						}
					}
					ja.DestStartByte = &startByte
					ja.DestEndByte = &endByte
					destRef, err := makeNodeRef(a.DestNode, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build dest_node reference for move: %w", err)
					}
					ja.DestNode = destRef
				} else if ms != nil {
					if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
						startByte := destNodeDst.StartByte
						endByte := destNodeDst.EndByte
						if !a.Subtree {
							hasRightFooter, rfStart, rfEnd := adjustRangeForContainer(destNodeDst, &startByte, &endByte, dstBytes)
							if hasRightFooter {
								rightFooters = append(rightFooters, pendingFooter{
									span: DelimiterSpan{
										StartByte: rfStart,
										EndByte:   rfEnd,
										Action:    "move",
									},
									actionIndex: len(actionsList),
								})
							}
						}
						ja.DestStartByte = new(startByte)
						ja.DestEndByte = new(endByte)
					}
				}
			}

			if a.GroupID != "" {
				ja.GroupID = a.GroupID
			}

			actionsList = append(actionsList, ja)
		}
	}

	var leftDelims []DelimiterSpan
	if len(leftFooters) > 0 {
		leftDelims = make([]DelimiterSpan, len(leftFooters))
		for i, f := range leftFooters {
			leftDelims[i] = f.span
			if f.actionIndex < len(actionsList) {
				leftDelims[i].ActionRef = &actionsList[f.actionIndex]
			}
		}
	}
	var rightDelims []DelimiterSpan
	if len(rightFooters) > 0 {
		rightDelims = make([]DelimiterSpan, len(rightFooters))
		for i, f := range rightFooters {
			rightDelims[i] = f.span
			if f.actionIndex < len(actionsList) {
				rightDelims[i].ActionRef = &actionsList[f.actionIndex]
			}
		}
	}

	if opts.IncludeActions {
		env.Actions = actionsList
	}

	if opts.IncludeHighlights {
		env.LeftHighlights = BuildHighlightSpans(srcBytes, actionsList, "left", leftDelims...)
		env.RightHighlights = BuildHighlightSpans(dstBytes, actionsList, "right", rightDelims...)
	}

	return &env, nil
}

// MarshalWithOptions formats the diff envelope as indented JSON using the given options.
func MarshalWithOptions(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode, srcBytes, dstBytes []byte, opts EnvelopeOptions) ([]byte, error) {
	env, err := BuildEnvelopeWithOptions(es, ms, srcRoot, dstRoot, srcBytes, dstBytes, opts)
	if err != nil {
		return nil, err
	}
	return json.MarshalIndent(env, "", "  ")
}

func makeNodeRef(n *treesitter.ASTNode, treeName string) (*NodeRef, error) {
	if n == nil {
		return nil, nil
	}
	return &NodeRef{
		Tree:      treeName,
		Type:      n.Type,
		Label:     n.Label,
		StartByte: n.StartByte,
		EndByte:   n.EndByte,
	}, nil
}

// adjustRangeForContainer limits the node to its opening header and reports any closing delimiter span (e.g. closing brace).
func adjustRangeForContainer(n *treesitter.ASTNode, start, end *uint32, fileBytes []byte) (hasFooter bool, footerStart, footerEnd uint32) {
	if n == nil || start == nil || end == nil {
		return false, 0, 0
	}
	r := rules.Get(n.GetLanguage())
	for _, child := range n.Children {
		if r != nil && r.IsBlock(child.Type) {
			if child.StartByte > *start && child.StartByte < *end {
				origEnd := *end
				*end = child.StartByte
				if child.EndByte < origEnd && !isIndentationConstruct(n, child) && isClosingDelimiter(fileBytes, child.EndByte, origEnd) {
					return true, child.EndByte, origEnd
				}
				return false, 0, 0
			}
		}
	}
	if len(n.Children) > 0 && n.StartRow != n.EndRow {
		var firstBodyChild *treesitter.ASTNode
		for _, child := range n.Children {
			if child.StartRow > n.StartRow && child.StartByte > *start && child.StartByte < *end {
				firstBodyChild = child
				break
			}
		}
		if firstBodyChild != nil {
			origEnd := *end
			*end = firstBodyChild.StartByte
			lastChild := n.Children[len(n.Children)-1]
			if lastChild.EndByte > firstBodyChild.StartByte && lastChild.EndByte < origEnd {
				if !isIndentationConstruct(n, nil) && isClosingDelimiter(fileBytes, lastChild.EndByte, origEnd) {
					return true, lastChild.EndByte, origEnd
				}
			}
		}
	}
	return false, 0, 0
}

func isIndentationConstruct(n, child *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	lang := n.GetLanguage()
	if lang != "python" && lang != "yaml" && lang != "toml" {
		return false
	}
	r := rules.Get(lang)
	if child != nil && r.IsBlock(child.Type) {
		return true
	}
	return !r.IsWrapper(n.Type)
}

func isClosingDelimiter(fileBytes []byte, start, end uint32) bool {
	if fileBytes == nil || start >= end || end > uint32(len(fileBytes)) {
		return false
	}
	trimmed := bytes.TrimSpace(fileBytes[start:end])
	if len(trimmed) == 0 {
		return false
	}
	if bytes.Contains(trimmed, []byte("//")) || bytes.Contains(trimmed, []byte("/*")) ||
		bytes.Contains(trimmed, []byte("#")) || bytes.Contains(trimmed, []byte("--")) ||
		bytes.Contains(trimmed, []byte("<!--")) {
		return false
	}
	tok := bytes.Trim(trimmed, ",; \t\r\n")
	return bytes.Equal(tok, []byte("}")) || bytes.Equal(tok, []byte("]")) ||
		bytes.Equal(tok, []byte(")")) || bytes.Equal(tok, []byte("end"))
}
