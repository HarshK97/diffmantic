package serialize

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
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
	IncludeActions        bool
	IncludeAlignment      bool
	IncludeHighlights     bool
	CommentLineMappings   map[int]int
	ShowStyleChanges      bool
	SignificantWhitespace bool
}

// FallbackReason classifies why Diffmantic degraded to line-diff mode.
type FallbackReason string

const (
	FallbackReasonNone                FallbackReason = ""
	FallbackReasonParseError          FallbackReason = "parse_error"
	FallbackReasonFileSizeLimit       FallbackReason = "file_size_limit"
	FallbackReasonLineLimit           FallbackReason = "line_limit"
	FallbackReasonConflict            FallbackReason = "merge_conflict"
	FallbackReasonMergeConflict                      = FallbackReasonConflict
	FallbackReasonUnsupportedLanguage FallbackReason = "unsupported_language"
	FallbackReasonMismatchedLanguages FallbackReason = "mismatched_languages"
)

// FallbackAnnotationParseError is the terminal badge text for parse errors.
const FallbackAnnotationParseError = "[parse error, falling back to line diff]"

// FallbackAnnotationConflict is the terminal badge text for merge conflicts.
const FallbackAnnotationConflict = "[merge conflict markers, falling back to line diff]"

// FormatFallbackAnnotationSizeLimit returns the terminal badge text when file size exceeds DIFFM_SIZE_LIMIT.
func FormatFallbackAnnotationSizeLimit(sizeBytes int) string {
	return fmt.Sprintf("[%s exceeded DIFFM_SIZE_LIMIT, falling back to line diff]", formatByteSize(sizeBytes))
}

// FormatFallbackAnnotationLineLimit returns the terminal badge text when line count exceeds DIFFM_LINE_LIMIT.
func FormatFallbackAnnotationLineLimit(lines int) string {
	return fmt.Sprintf("[%s lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]", formatLineCount(lines))
}

func formatByteSize(n int) string {
	if n < 1024 {
		return fmt.Sprintf("%d B", n)
	}
	if n < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	}
	return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
}

func formatLineCount(n int) string {
	in := strconv.Itoa(n)
	if len(in) <= 3 {
		return in
	}
	var b strings.Builder
	b.Grow(len(in) + (len(in)-1)/3)
	for i, r := range in {
		if i > 0 && (len(in)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteByte(byte(r))
	}
	return b.String()
}

// Envelope wraps the serialized actions list with a schema version.
type Envelope struct {
	Version            string              `json:"version"`
	IsBinary           bool                `json:"is_binary,omitempty"`
	FallbackReason     FallbackReason      `json:"fallback_reason,omitempty"`
	FallbackAnnotation string              `json:"fallback_annotation,omitempty"`
	Actions            []Action            `json:"actions,omitempty"`
	LineAlignment      []LineAlignmentPair `json:"line_alignment,omitempty"`
	LeftHighlights     []HighlightSpan     `json:"left_highlights,omitempty"`
	RightHighlights    []HighlightSpan     `json:"right_highlights,omitempty"`
}

// IsFallback reports whether the envelope represents a line-diff fallback rather than a clean AST diff.
func (e *Envelope) IsFallback() bool {
	return e == nil || e.FallbackReason != FallbackReasonNone || e.FallbackAnnotation != ""
}

// Action is a single AST edit operation. Omitting "subtree" on the wire implicitly means false.
type Action struct {
	Action            string              `json:"action"` // "insert", "delete", "update", "move"
	Node              *NodeRef            `json:"node"`
	Parent            *NodeRef            `json:"parent,omitempty"`
	Position          *int                `json:"position,omitempty"`
	OldParent         *NodeRef            `json:"old_parent,omitempty"`
	OldPosition       *int                `json:"old_position,omitempty"`
	OldValue          string              `json:"old_value,omitempty"`
	NewValue          string              `json:"new_value,omitempty"`
	Subtree           *bool               `json:"subtree,omitempty"`
	DestNode          *NodeRef            `json:"dest_node,omitempty"`
	DestStartByte     *uint32             `json:"dest_start_byte,omitempty"`
	DestEndByte       *uint32             `json:"dest_end_byte,omitempty"`
	GroupID           string              `json:"group_id,omitempty"`
	MoveColorIndex    int                 `json:"-"`
	OrigStartByte     uint32              `json:"-"`
	OrigEndByte       uint32              `json:"-"`
	DestOrigStartByte uint32              `json:"-"`
	DestOrigEndByte   uint32              `json:"-"`
	ASTNode           *treesitter.ASTNode `json:"-"`
	DestASTNode       *treesitter.ASTNode `json:"-"`
	SrcLineCount      int                 `json:"-"`
	DstLineCount      int                 `json:"-"`
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

	alignment := AlignLines(srcBytes, dstBytes, nil, nil)
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
		var leftHighlights []HighlightSpan
		var rightHighlights []HighlightSpan

		var scratch *engine.WordDiffScratch
		if opts.IncludeHighlights {
			scratch = engine.AcquireWordDiffScratch()
			defer engine.ReleaseWordDiffScratch(scratch)
		}

		trimLineEndCol := func(lineText string) int {
			return len(strings.TrimRight(lineText, " \t\r\n"))
		}

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
				if opts.IncludeHighlights && pair.LeftLine >= 0 && pair.LeftLine < len(srcLines) {
					endCol := trimLineEndCol(srcLines[pair.LeftLine])
					if endCol > 0 {
						leftHighlights = append(leftHighlights, HighlightSpan{
							Line:     pair.LeftLine,
							StartCol: 0,
							EndCol:   endCol,
							Action:   "delete",
						})
					}
				}
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
				if opts.IncludeHighlights && pair.RightLine >= 0 && pair.RightLine < len(dstLines) {
					endCol := trimLineEndCol(dstLines[pair.RightLine])
					if endCol > 0 {
						rightHighlights = append(rightHighlights, HighlightSpan{
							Line:     pair.RightLine,
							StartCol: 0,
							EndCol:   endCol,
							Action:   "insert",
						})
					}
				}
			} else if pair.LeftLine != -1 && pair.RightLine != -1 {
				// Paired lines with differing text turn into a delete on the left and insert on the right.
				if pair.LeftLine >= 0 && pair.RightLine >= 0 && pair.LeftLine < len(srcLines) && pair.RightLine < len(dstLines) && srcLines[pair.LeftLine] != dstLines[pair.RightLine] {
					sText := srcLines[pair.LeftLine]
					dText := dstLines[pair.RightLine]
					if !opts.ShowStyleChanges && EqualIgnoringStyle(sText, dText, opts.SignificantWhitespace) {
						continue
					}
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

					if scratch != nil {
						for _, op := range scratch.Diff(sText, dText) {
							if op.Action != "insert" {
								leftHighlights = append(leftHighlights, HighlightSpan{
									Line:     pair.LeftLine,
									StartCol: op.SrcStart,
									EndCol:   op.SrcEnd,
									Action:   op.Action,
								})
							}
							if op.Action != "delete" {
								rightHighlights = append(rightHighlights, HighlightSpan{
									Line:     pair.RightLine,
									StartCol: op.DstStart,
									EndCol:   op.DstEnd,
									Action:   op.Action,
								})
							}
						}
					}
				}
			}
		}

		if opts.IncludeActions {
			env.Actions = actionsList
		}

		if opts.IncludeHighlights {
			env.LeftHighlights = leftHighlights
			env.RightHighlights = rightHighlights
		}
	}

	return env
}

// BuildEnvelopeWithOptions translates raw diff actions and line alignments into the finalized JSON payload.
func BuildEnvelopeWithOptions(es *actions.EditScript, ms *engine.Mapping, srcRoot, dstRoot *treesitter.ASTNode, srcBytes, dstBytes []byte, opts EnvelopeOptions) (*Envelope, error) {
	if es == nil {
		return nil, fmt.Errorf("edit script is nil")
	}

	env := Envelope{
		Version: SchemaVersion,
	}

	if opts.IncludeAlignment {
		env.LineAlignment = AlignLines(srcBytes, dstBytes, ms, es, opts.CommentLineMappings)
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
			if a.Node != nil {
				ja.OrigStartByte = a.Node.StartByte
				ja.OrigEndByte = a.Node.EndByte
				ja.ASTNode = a.Node
			}

			switch a.Type {
			case actions.Insert:
				if a.Node == nil {
					return nil, fmt.Errorf("insert action has nil Node")
				}
				ja.DestOrigStartByte = a.Node.StartByte
				ja.DestOrigEndByte = a.Node.EndByte
				nodeRef, err := makeNodeRef(a.Node, "after")
				if err != nil {
					return nil, fmt.Errorf("failed to build node reference for insert: %w", err)
				}
				if !a.Subtree {
					if hasFooter, fStart, fEnd := adjustRangeForContainer(a.Node, &nodeRef.StartByte, &nodeRef.EndByte, dstBytes); hasFooter {
						// When the enclosed block moved from elsewhere, its closing brace
						// belongs to that move, so skip emitting a synthetic insert footer.
						r := rules.Get(a.Node.GetLanguage())
						enclosedBlock := findDirectOrEnclosedBlock(a.Node, r)
						blockMapped := enclosedBlock != nil && ms != nil && ms.Dst()[enclosedBlock] != nil
						if !blockMapped {
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
				}
				ja.Node = nodeRef

				if a.Node.Parent != nil {
					parentRef, err := makeNodeRef(a.Node.Parent, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build parent reference for insert: %w", err)
					}
					ja.Parent = parentRef
				}

				ja.Position = ptr(a.Position)
				if a.Subtree {
					ja.Subtree = ptr(true)
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
					if hasFooter, fStart, fEnd := adjustRangeForContainer(a.Node, &nodeRef.StartByte, &nodeRef.EndByte, srcBytes); hasFooter {
						// When the enclosed block moved to elsewhere, its closing brace
						// belongs to that move, so skip emitting a synthetic delete footer.
						r := rules.Get(a.Node.GetLanguage())
						enclosedBlock := findDirectOrEnclosedBlock(a.Node, r)
						blockMapped := enclosedBlock != nil && ms != nil && ms.Src()[enclosedBlock] != nil
						if !blockMapped {
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
				}
				ja.Node = nodeRef

				if a.Node.Parent != nil {
					parentRef, err := makeNodeRef(a.Node.Parent, "before")
					if err != nil {
						return nil, fmt.Errorf("failed to build parent reference for delete: %w", err)
					}
					ja.Parent = parentRef
				}

				ja.Position = ptr(a.Position)
				if a.Subtree {
					ja.Subtree = ptr(true)
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
					ja.DestOrigStartByte = destNodeDst.StartByte
					ja.DestOrigEndByte = destNodeDst.EndByte
					ja.DestASTNode = destNodeDst
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
				ja.Position = ptr(a.Position)

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
				ja.OldPosition = ptr(oldPos)

				if a.Subtree {
					ja.Subtree = ptr(true)
				}

				if a.DestNode != nil {
					ja.DestOrigStartByte = a.DestNode.StartByte
					ja.DestOrigEndByte = a.DestNode.EndByte
					ja.DestASTNode = a.DestNode
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
					ja.DestStartByte = ptr(startByte)
					ja.DestEndByte = ptr(endByte)
					destRef, err := makeNodeRef(a.DestNode, "after")
					if err != nil {
						return nil, fmt.Errorf("failed to build dest_node reference for move: %w", err)
					}
					ja.DestNode = destRef
				} else if ms != nil {
					if destNodeDst := ms.Src()[a.Node]; destNodeDst != nil {
						ja.DestOrigStartByte = destNodeDst.StartByte
						ja.DestOrigEndByte = destNodeDst.EndByte
						ja.DestASTNode = destNodeDst
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
						ja.DestStartByte = ptr(startByte)
						ja.DestEndByte = ptr(endByte)
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
	if opts.IncludeHighlights && ms != nil {
		extraLeft, extraRight := collectMappedTrailingDelimiters(ms, srcRoot, srcBytes, dstBytes)
		leftDelims = append(leftDelims, extraLeft...)
		rightDelims = append(rightDelims, extraRight...)
	}

	deletedCommentBytes := make(map[uint32]bool)
	insertedCommentBytes := make(map[uint32]bool)
	for _, a := range actionsList {
		if a.Action == "delete" && a.ASTNode != nil && a.ASTNode.IsComment() {
			deletedCommentBytes[a.ASTNode.StartByte] = true
		} else if a.Action == "insert" && a.ASTNode != nil && a.ASTNode.IsComment() {
			insertedCommentBytes[a.ASTNode.StartByte] = true
		}
	}

	for actIdx := range actionsList {
		ja := &actionsList[actIdx]
		if ja.Action != "move" && ja.Action != "move_update" {
			continue
		}

		// Let leading doc comments share the block's move highlight if both sides have them.
		if ja.ASTNode != nil && ja.DestASTNode != nil && len(ja.ASTNode.LeadingTrivia) > 0 && len(ja.DestASTNode.LeadingTrivia) > 0 {
			for _, tr := range ja.ASTNode.LeadingTrivia {
				if tr != nil && (tr.IsComment || rules.IsComment(tr.Type)) && !deletedCommentBytes[tr.StartByte] {
					leftDelims = append(leftDelims, DelimiterSpan{
						StartByte: tr.StartByte,
						EndByte:   tr.EndByte,
						Action:    "move",
						ActionRef: ja,
					})
				}
			}

			for _, tr := range ja.DestASTNode.LeadingTrivia {
				if tr != nil && (tr.IsComment || rules.IsComment(tr.Type)) && !insertedCommentBytes[tr.StartByte] {
					rightDelims = append(rightDelims, DelimiterSpan{
						StartByte: tr.StartByte,
						EndByte:   tr.EndByte,
						Action:    "move",
						ActionRef: ja,
					})
				}
			}
		}
	}

	hasMoves := slices.ContainsFunc(actionsList, func(a Action) bool {
		return a.Action == "move" || a.Action == "move_update"
	})
	if hasMoves {
		offsetsSrc := BuildLineIndex(srcBytes)
		offsetsDst := BuildLineIndex(dstBytes)
		for i := range actionsList {
			a := &actionsList[i]
			if a.Action == "move" || a.Action == "move_update" {
				a.SrcLineCount = actionLineCount(a, offsetsSrc, srcBytes, "left")
				a.DstLineCount = actionLineCount(a, offsetsDst, dstBytes, "right")
			}
		}
		AssignMoveColors(actionsList, offsetsSrc, offsetsDst)
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

// AssignMoveColors assigns move colors (0: Cyan, 1: Magenta) to moved actions:
//   - Descendants moving with a parent inherit that parent's color.
//   - Adjacent moves shifting together in the same order share a color so lines don't alternate colors for no reason.
//   - Swaps, reordered moves, or moves nested inside another take different colors so they stand out.
//   - Moves diverging from the same parent or converging near the same destination take different colors, unless one moved into the other's body block.
func AssignMoveColors(actions []Action, srcOffsets, dstOffsets []int) {
	var moves []moveInterval
	for i := range actions {
		a := &actions[i]
		if a.Action != "move" && a.Action != "move_update" {
			continue
		}
		var sStart, sEnd, dStart, dEnd int
		sStartByte, sEndByte, hasSrc := getSrcBytes(a)
		if hasSrc {
			sStart, _ = ByteToLineCol(srcOffsets, sStartByte)
			sEnd, _ = ByteToLineCol(srcOffsets, sEndByte)
		}
		dStartByte, dEndByte, hasDst := getDestBytes(a)
		if hasDst {
			dStart, _ = ByteToLineCol(dstOffsets, dStartByte)
			dEnd, _ = ByteToLineCol(dstOffsets, dEndByte)
		}
		moves = append(moves, moveInterval{
			actIdx: i,
			sStart: sStart,
			sEnd:   sEnd,
			dStart: dStart,
			dEnd:   dEnd,
			hasSrc: hasSrc,
			hasDst: hasDst,
		})
	}

	if len(moves) <= 1 {
		return
	}

	type groupInfo struct {
		id          string
		moves       []moveInterval
		totalSpan   int
		minSrcStart int
	}

	groupMap := make(map[string]*groupInfo)
	for _, m := range moves {
		curAct := &actions[m.actIdx]
		if curAct.GroupID == "" {
			continue
		}
		gi, exists := groupMap[curAct.GroupID]
		if !exists {
			gi = &groupInfo{
				id:          curAct.GroupID,
				minSrcStart: m.sStart,
			}
			groupMap[curAct.GroupID] = gi
		}
		gi.moves = append(gi.moves, m)
		span := 0
		if curAct.Node != nil {
			span = int(curAct.Node.EndByte - curAct.Node.StartByte)
		}
		gi.totalSpan += span
		gi.minSrcStart = min(gi.minSrcStart, m.sStart)
	}

	groups := slices.Collect(maps.Values(groupMap))
	slices.SortFunc(groups, func(a, b *groupInfo) int {
		return cmp.Or(
			cmp.Compare(len(b.moves), len(a.moves)),
			cmp.Compare(b.totalSpan, a.totalSpan),
			cmp.Compare(a.minSrcStart, b.minSrcStart),
			cmp.Compare(a.id, b.id),
		)
	})

	isColored := make([]bool, len(actions))
	groupColors := make(map[string]int)

	// Pass 1: Color dominant groups first so the main block gets slot 0 (Cyan).
	for _, g := range groups {
		var used [2]bool
		for _, m := range g.moves {
			curAct := &actions[m.actIdx]
			for _, otherG := range groups {
				otherColor, scheduled := groupColors[otherG.id]
				if !scheduled || otherG.id == g.id {
					continue
				}
				for _, otherM := range otherG.moves {
					otherAct := &actions[otherM.actIdx]
					if isHierarchicalCoMover(curAct, otherAct) {
						continue
					}
					if movesInterfere(m, otherM, curAct, otherAct) {
						if otherColor < len(used) {
							used[otherColor] = true
						}
					}
				}
			}
		}

		slot := slices.Index(used[:], false)
		if slot < 0 {
			slot = 0
		}
		groupColors[g.id] = slot
		for _, m := range g.moves {
			actions[m.actIdx].MoveColorIndex = slot
			isColored[m.actIdx] = true
		}
	}

	// Pass 2: Color leftover outliers and ungrouped moves, picking whatever slot doesn't clash nearby.
	for _, cur := range moves {
		curAct := &actions[cur.actIdx]
		if isColored[cur.actIdx] {
			continue
		}

		// Inherit the container's move color unless this statement swapped order
		// with a sibling inside the container.
		if pIdx := slices.IndexFunc(moves, func(prev moveInterval) bool {
			parentAct := &actions[prev.actIdx]
			return isColored[prev.actIdx] &&
				(isHierarchicalCoMover(curAct, parentAct) || isDestinationContainerBodyCoMover(curAct, parentAct)) &&
				isContainerInOrderCoMover(cur, curAct, prev, parentAct, moves, actions, isColored)
		}); pIdx >= 0 {
			c := actions[moves[pIdx].actIdx].MoveColorIndex
			curAct.MoveColorIndex = c
			isColored[cur.actIdx] = true
			continue
		}

		var used [2]bool
		for _, prev := range moves {
			if !isColored[prev.actIdx] {
				continue
			}
			prevAct := &actions[prev.actIdx]
			if curAct.GroupID != "" && curAct.GroupID == prevAct.GroupID {
				continue
			}
			if isChildCoMover(curAct, prevAct) || isChildCoMover(prevAct, curAct) {
				continue
			}
			if (isContainerBodyCoMover(curAct, prevAct) && isContainerInOrderCoMover(cur, curAct, prev, prevAct, moves, actions, isColored)) ||
				(isContainerBodyCoMover(prevAct, curAct) && isContainerInOrderCoMover(prev, prevAct, cur, curAct, moves, actions, isColored)) {
				continue
			}

			if movesInterfere(cur, prev, curAct, prevAct) {
				if prevAct.MoveColorIndex < len(used) {
					used[prevAct.MoveColorIndex] = true
				}
			}
		}

		slot := slices.Index(used[:], false)
		if slot < 0 {
			slot = 0
		}
		curAct.MoveColorIndex = slot
		isColored[cur.actIdx] = true
	}
}

type moveInterval struct {
	actIdx int
	sStart int
	sEnd   int
	dStart int
	dEnd   int
	hasSrc bool
	hasDst bool
}

// movesInterfere determines whether two moves should take distinct colors.
func movesInterfere(cur, prev moveInterval, curAct, prevAct *Action) bool {
	sOverlap := max(cur.sStart, prev.sStart) <= min(cur.sEnd, prev.sEnd)+2
	dOverlap := max(cur.dStart, prev.dStart) <= min(cur.dEnd, prev.dEnd)+2

	if !sOverlap && !dOverlap {
		return false
	}

	// Case 1: Both source and destination overlap within 2 lines.
	if sOverlap && dOverlap {
		// Contrasting colors highlight nested wrappers or swapped positions.
		return movesNestedOrReordered(cur, prev, curAct, prevAct)
	}

	// Case 2: Diverging moves (nearby source lines, but distant destination lines).
	// They only clash if they diverge from the same parent or container, or if one
	// moves out of an enclosing move.
	if sOverlap && !dOverlap {
		return sameSourceParent(curAct, prevAct) || isEnclosingMove(curAct, prevAct) || isEnclosingMove(prevAct, curAct)
	}

	// Case 3: Converging moves (distant source lines, but nearby destination lines).
	// They clash so converging moves don't look like one chunk, unless one moved into the other's body block or both shifted together from nearby lines.
	if !sOverlap && dOverlap {
		return !isDestinationContainerBodyCoMover(curAct, prevAct) &&
			!isDestinationContainerBodyCoMover(prevAct, curAct) &&
			!isLockstepCohort(cur, prev, curAct, prevAct)
	}

	return false
}

// movesNestedOrReordered reports whether one move encloses the other or their relative order flipped.
func movesNestedOrReordered(cur, prev moveInterval, curAct, prevAct *Action) bool {
	if isEnclosingMove(curAct, prevAct) || isEnclosingMove(prevAct, curAct) {
		return true
	}
	return moveStartsBefore(getSrcBytes, curAct, prevAct, cur.sStart, prev.sStart) !=
		moveStartsBefore(getDestBytes, curAct, prevAct, cur.dStart, prev.dStart)
}

// isLockstepCohort reports whether two converging moves came from nearby lines and shifted together in order,
// so unnested sibling branches keep the same color.
func isLockstepCohort(cur, prev moveInterval, curAct, prevAct *Action) bool {
	const (
		// Allow up to 4 lines of shift difference when unnesting removes slightly different wrapper headers.
		maxCohortDeltaDiffLines = 4
		// Cap the source gap at 8 lines so unrelated distant moves with similar offsets don't merge.
		maxCohortSourceGapLines = 8
	)

	if !cur.hasSrc || !cur.hasDst || !prev.hasSrc || !prev.hasDst {
		return false
	}
	if movesNestedOrReordered(cur, prev, curAct, prevAct) {
		return false
	}

	deltaCur := cur.dStart - cur.sStart
	deltaPrev := prev.dStart - prev.sStart
	// Reject unshifted moves so converging moves onto a stationary-line sibling still contrast.
	if deltaCur == 0 || deltaPrev == 0 || (deltaCur > 0) != (deltaPrev > 0) {
		return false
	}

	return max(deltaCur, deltaPrev)-min(deltaCur, deltaPrev) <= maxCohortDeltaDiffLines &&
		max(cur.sStart, prev.sStart)-min(cur.sEnd, prev.sEnd) <= maxCohortSourceGapLines
}

// isEnclosingMove reports whether outer encloses inner in source or destination bytes.
func isEnclosingMove(outer, inner *Action) bool {
	if outer == nil || inner == nil {
		return false
	}
	if sOuterStart, sOuterEnd, okOuter := getSrcBytes(outer); okOuter {
		if sInnerStart, sInnerEnd, okInner := getSrcBytes(inner); okInner {
			if sOuterStart <= sInnerStart && sOuterEnd >= sInnerEnd && (sOuterStart < sInnerStart || sOuterEnd > sInnerEnd) {
				return true
			}
		}
	}
	if dOuterStart, dOuterEnd, okOuter := getDestBytes(outer); okOuter {
		if dInnerStart, dInnerEnd, okInner := getDestBytes(inner); okInner {
			if dOuterStart <= dInnerStart && dOuterEnd >= dInnerEnd && (dOuterStart < dInnerStart || dOuterEnd > dInnerEnd) {
				return true
			}
		}
	}
	return false
}

// getSrcBytes returns the source byte span for a, keeping OrigEndByte for clipped containers.
func getSrcBytes(a *Action) (uint32, uint32, bool) {
	if a == nil || a.Node == nil {
		return 0, 0, false
	}
	start := a.Node.StartByte
	end := a.Node.EndByte
	if a.OrigEndByte > start {
		end = max(end, a.OrigEndByte)
	}
	return start, end, true
}

// getDestBytes returns the destination byte span for a, keeping DestOrigEndByte for clipped containers.
func getDestBytes(a *Action) (uint32, uint32, bool) {
	if a == nil {
		return 0, 0, false
	}
	var start, end uint32
	found := false
	if a.DestStartByte != nil && a.DestEndByte != nil {
		start = *a.DestStartByte
		end = *a.DestEndByte
		found = true
	} else if a.DestNode != nil {
		start = a.DestNode.StartByte
		end = a.DestNode.EndByte
		found = true
	}
	if !found {
		return 0, 0, false
	}
	if a.DestOrigEndByte > start {
		end = max(end, a.DestOrigEndByte)
	}
	return start, end, true
}

// moveStartsBefore reports whether actA starts before actB using byte offsets when available, or line indices otherwise.
func moveStartsBefore(getBytes func(*Action) (uint32, uint32, bool), actA, actB *Action, lineA, lineB int) bool {
	if aStart, _, okA := getBytes(actA); okA {
		if bStart, _, okB := getBytes(actB); okB {
			return aStart < bStart
		}
	}
	return lineA < lineB
}

// canonicalMoveParentNode skips wrapper and expression statement nodes to find the enclosing block or construct parent.
func canonicalMoveParentNode(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	p := n.Parent
	r := rules.Get(n.GetLanguage())
	for p != nil && r != nil && (r.IsWrapper(p.Type) || r.IsExpressionStatement(p.Type)) && !r.IsBlock(p.Type) && !r.IsDeclaration(p.Type) {
		p = p.Parent
	}
	return p
}

// sameSourceParent reports whether actA and actB share the same enclosing parent in the source tree.
func sameSourceParent(actA, actB *Action) bool {
	if actA == nil || actB == nil {
		return true
	}
	if actA.ASTNode != nil && actB.ASTNode != nil {
		pA := canonicalMoveParentNode(actA.ASTNode)
		pB := canonicalMoveParentNode(actB.ASTNode)
		return pA != nil && pA == pB
	}
	if actA.OldParent != nil && actB.OldParent != nil {
		return actA.OldParent.StartByte == actB.OldParent.StartByte &&
			actA.OldParent.Type == actB.OldParent.Type
	}
	return true
}

// isDestinationContainerBodyCoMover reports whether child moved into parent's body block in the destination tree.
func isDestinationContainerBodyCoMover(child, parent *Action) bool {
	if child == nil || parent == nil || parent.DestASTNode == nil {
		return false
	}
	dstParent := parent.DestASTNode
	r := rules.Get(dstParent.GetLanguage())
	if r == nil {
		return false
	}
	dstBlock := getDirectOrEnclosedBlock(dstParent, r)
	if dstBlock == nil {
		return false
	}
	if child.DestASTNode != nil {
		return dstBlock.Contains(child.DestASTNode)
	}
	if child.DestStartByte != nil && child.DestEndByte != nil {
		return dstBlock.StartByte <= *child.DestStartByte && dstBlock.EndByte >= *child.DestEndByte
	}
	return false
}

// isHierarchicalCoMover reports whether either action is a child or body descendant of the other in both trees.
func isHierarchicalCoMover(a, b *Action) bool {
	return isChildCoMover(a, b) || isChildCoMover(b, a) || isContainerBodyCoMover(a, b) || isContainerBodyCoMover(b, a)
}

// isContainerBodyCoMover reports whether child moved inside parent's body block across both trees.
func isContainerBodyCoMover(child, parent *Action) bool {
	if child == nil || parent == nil || child.ASTNode == nil || parent.ASTNode == nil {
		return false
	}
	r := rules.Get(parent.ASTNode.GetLanguage())
	if r == nil {
		return false
	}
	srcBlock := getDirectOrEnclosedBlock(parent.ASTNode, r)
	if srcBlock == nil || !srcBlock.Contains(child.ASTNode) {
		return false
	}
	return isDestinationContainerBodyCoMover(child, parent)
}

// isContainerInOrderCoMover checks whether curAct kept its relative order with siblings
// inside parentAct. If it swapped order, it's an outlier and shouldn't share the container's color.
func isContainerInOrderCoMover(cur moveInterval, curAct *Action, prev moveInterval, parentAct *Action, moves []moveInterval, actions []Action, isColored []bool) bool {
	if isChildCoMover(curAct, parentAct) || isChildCoMover(parentAct, curAct) || !isContainerBodyCoMover(curAct, parentAct) {
		return true
	}

	for _, m := range moves {
		if m.actIdx == cur.actIdx || m.actIdx == prev.actIdx {
			continue
		}
		otherAct := &actions[m.actIdx]
		if !isContainerBodyCoMover(otherAct, parentAct) {
			continue
		}
		sOrder := moveStartsBefore(getSrcBytes, curAct, otherAct, cur.sStart, m.sStart)
		dOrder := moveStartsBefore(getDestBytes, curAct, otherAct, cur.dStart, m.dStart)
		if sOrder != dOrder {
			if isColored[m.actIdx] && otherAct.MoveColorIndex == parentAct.MoveColorIndex {
				return false
			}
			deltaParent := prev.dStart - prev.sStart
			deltaCur := cur.dStart - cur.sStart
			deltaOther := m.dStart - m.sStart
			diffCur := max(deltaCur-deltaParent, deltaParent-deltaCur)
			diffOther := max(deltaOther-deltaParent, deltaParent-deltaOther)
			if diffCur > diffOther || (diffCur == diffOther && moveStartsBefore(getDestBytes, otherAct, curAct, m.dStart, cur.dStart)) {
				return false
			}
		}
	}
	return true
}

// getDirectOrEnclosedBlock returns n if it is already a block, or searches for an enclosed block child.
func getDirectOrEnclosedBlock(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil || r == nil {
		return nil
	}
	if r.IsBlock(n.Type) {
		return n
	}
	return findDirectOrEnclosedBlock(n, r)
}

// isChildCoMover reports whether child is a direct child of parent in both the before and after trees.
func isChildCoMover(child, parent *Action) bool {
	if child == nil || parent == nil || child.OldParent == nil || parent.Node == nil {
		return false
	}
	if child.OldParent.StartByte != parent.Node.StartByte || child.OldParent.Type != parent.Node.Type {
		return false
	}
	if child.Parent == nil {
		return false
	}
	if parent.DestASTNode != nil {
		return child.Parent.StartByte == parent.DestASTNode.StartByte && child.Parent.Type == parent.DestASTNode.Type
	}
	if parent.DestNode != nil {
		return child.Parent.StartByte == parent.DestNode.StartByte && child.Parent.Type == parent.DestNode.Type
	}
	if parent.DestStartByte != nil {
		if child.Parent.StartByte != *parent.DestStartByte {
			return false
		}
		var r *rules.Rules
		if parent.ASTNode != nil {
			r = rules.Get(parent.ASTNode.GetLanguage())
		}
		return engine.TypesMatch(child.Parent.Type, parent.Node.Type, r)
	}
	return false
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

// adjustRangeForContainer clips a container action to its header line and extracts its closing delimiter so multiline blocks don't highlight inner children.
func adjustRangeForContainer(n *treesitter.ASTNode, start, end *uint32, fileBytes []byte) (hasFooter bool, footerStart, footerEnd uint32) {
	if n == nil || start == nil || end == nil {
		return false, 0, 0
	}
	r := rules.Get(n.GetLanguage())
	if block := findDirectOrEnclosedBlock(n, r); block != nil {
		if block.StartByte > *start && block.StartByte < *end {
			origEnd := *end
			*end = block.StartByte
			if len(block.LeadingTrivia) > 0 && block.LeadingTrivia[0].StartByte > *start && block.LeadingTrivia[0].StartByte < *end {
				*end = block.LeadingTrivia[0].StartByte
			}
			if bt := block.SyntaxTrivia(); bt != nil && bt.ClosingEnd > bt.ClosingStart && !isIndentationConstruct(n, block) {
				return true, bt.ClosingStart, bt.ClosingEnd
			}
			if block.EndByte < origEnd && !isIndentationConstruct(n, block) && isClosingDelimiter(fileBytes, block.EndByte, origEnd) {
				return true, block.EndByte, origEnd
			}
			return false, 0, 0
		}
	}
	if len(n.Children) > 0 {
		if r != nil && r.IsIndexed(n.Type) && len(n.Children) >= 2 {
			receiver := n.Children[0]
			firstIndex := n.Children[1]
			lastIndex := n.Children[len(n.Children)-1]
			if firstIndex.StartByte > receiver.EndByte && firstIndex.StartByte <= *end {
				origEnd := *end
				*start = receiver.EndByte
				*end = firstIndex.StartByte
				if len(firstIndex.LeadingTrivia) > 0 && firstIndex.LeadingTrivia[0].StartByte > *start && firstIndex.LeadingTrivia[0].StartByte < *end {
					*end = firstIndex.LeadingTrivia[0].StartByte
				}
				if trivia := n.SyntaxTrivia(); trivia != nil && trivia.ClosingEnd > trivia.ClosingStart && !isIndentationConstruct(n, nil) {
					return true, trivia.ClosingStart, trivia.ClosingEnd
				}
				if lastIndex.EndByte > firstIndex.StartByte && lastIndex.EndByte < origEnd {
					if !isIndentationConstruct(n, nil) && isClosingDelimiter(fileBytes, lastIndex.EndByte, origEnd) {
						return true, lastIndex.EndByte, origEnd
					}
				}
				return false, 0, 0
			}
		}

		if r != nil && r.IsWrapper(n.Type) && !r.IsDeclaration(n.Type) {
			firstChild := n.Children[0]
			if firstChild.StartByte > *start && firstChild.StartByte < *end {
				origEnd := *end
				*end = firstChild.StartByte
				if len(firstChild.LeadingTrivia) > 0 && firstChild.LeadingTrivia[0].StartByte > *start && firstChild.LeadingTrivia[0].StartByte < *end {
					*end = firstChild.LeadingTrivia[0].StartByte
				}
				if trivia := n.SyntaxTrivia(); trivia != nil && trivia.ClosingEnd > trivia.ClosingStart && !isIndentationConstruct(n, nil) {
					return true, trivia.ClosingStart, trivia.ClosingEnd
				}
				lastChild := n.Children[len(n.Children)-1]
				if lastChild.EndByte > firstChild.StartByte && lastChild.EndByte < origEnd {
					if !isIndentationConstruct(n, nil) && isClosingDelimiter(fileBytes, lastChild.EndByte, origEnd) {
						return true, lastChild.EndByte, origEnd
					}
				}
				return false, 0, 0
			}
			// For wrappers starting with a prefix identifier (like tuple_struct_pattern or class_pattern), the delimiter begins after it.
			if firstChild.StartByte == *start && firstChild.EndByte < *end {
				*start = firstChild.EndByte
				if len(n.Children) <= 1 {
					return false, 0, 0
				}
				firstInner := n.Children[1]
				if firstInner.StartByte <= *start || firstInner.StartByte > *end {
					return false, 0, 0
				}
				origEnd := *end
				*end = firstInner.StartByte
				if len(firstInner.LeadingTrivia) > 0 && firstInner.LeadingTrivia[0].StartByte > *start && firstInner.LeadingTrivia[0].StartByte < *end {
					*end = firstInner.LeadingTrivia[0].StartByte
				}
				if trivia := n.SyntaxTrivia(); trivia != nil && trivia.ClosingEnd > trivia.ClosingStart && !isIndentationConstruct(n, nil) {
					return true, trivia.ClosingStart, trivia.ClosingEnd
				}
				lastChild := n.Children[len(n.Children)-1]
				if lastChild.EndByte > firstInner.StartByte && lastChild.EndByte < origEnd {
					if !isIndentationConstruct(n, nil) && isClosingDelimiter(fileBytes, lastChild.EndByte, origEnd) {
						return true, lastChild.EndByte, origEnd
					}
				}
				return false, 0, 0
			}
		}
		if n.StartRow != n.EndRow {
			var firstBodyChild *treesitter.ASTNode
			for _, child := range n.Children {
				isBody := child.StartRow > n.StartRow || (r != nil && (r.IsDeclaration(child.Type) || r.IsBlock(child.Type)))
				if isBody && child.StartByte > *start && child.StartByte < *end {
					firstBodyChild = child
					break
				}
			}
			if firstBodyChild == nil {
				for _, child := range n.Children {
					if child.EndRow > n.StartRow && child.StartByte > *start && child.StartByte < *end &&
						findDirectOrEnclosedBlock(child, r) != nil {
						firstBodyChild = child
						break
					}
				}
			}
			if firstBodyChild != nil {
				origEnd := *end
				*end = firstBodyChild.StartByte
				if len(firstBodyChild.LeadingTrivia) > 0 && firstBodyChild.LeadingTrivia[0].StartByte > *start && firstBodyChild.LeadingTrivia[0].StartByte < *end {
					*end = firstBodyChild.LeadingTrivia[0].StartByte
				}
				if trivia := n.SyntaxTrivia(); trivia != nil && trivia.ClosingEnd > trivia.ClosingStart && !isIndentationConstruct(n, nil) {
					return true, trivia.ClosingStart, trivia.ClosingEnd
				}
				lastChild := n.Children[len(n.Children)-1]
				if lastChild.EndByte > firstBodyChild.StartByte && lastChild.EndByte < origEnd {
					if !isIndentationConstruct(n, nil) && isClosingDelimiter(fileBytes, lastChild.EndByte, origEnd) {
						return true, lastChild.EndByte, origEnd
					}
				}
				return false, 0, 0
			}
		}
	}
	return false, 0, 0
}

// findDirectOrEnclosedBlock finds an immediate block child, or walks down single-line opening
// wrappers of a multiline call (e.g. arguments, closures) to locate an enclosed block.
func findDirectOrEnclosedBlock(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil || r == nil {
		return nil
	}
	for _, child := range n.Children {
		if r.IsBlock(child.Type) {
			return child
		}
	}
	if n.StartRow != n.EndRow && isCallOrCallStatement(n, r) {
		for curr := n; curr != nil && len(curr.Children) > 0; {
			var next *treesitter.ASTNode
			for _, child := range curr.Children {
				if r.IsBlock(child.Type) {
					return child
				}
				if child.StartRow == n.StartRow && child.EndRow == n.EndRow {
					next = child
				}
			}
			curr = next
		}
	}
	return nil
}

// isCallOrCallStatement reports whether n is a call expression or an expression statement
// wrapping a call (optionally followed by a trailing semicolon).
func isCallOrCallStatement(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || r == nil || r.IsBlock(n.Type) || r.IsDeclaration(n.Type) {
		return false
	}
	if r.IsCall(n.Type) {
		return true
	}
	if len(n.Children) == 1 {
		return r.IsCall(n.Children[0].Type)
	}
	if len(n.Children) == 2 {
		return r.IsCall(n.Children[0].Type) && n.Children[0].EndRow == n.EndRow
	}
	return false
}

func isIndentationConstruct(n, child *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	lang := n.GetLanguage()
	if lang != "python" {
		return false
	}
	r := rules.Get(lang)
	if child != nil && r.IsBlock(child.Type) {
		return true
	}
	if r.IsWrapper(n.Type) || r.IsIndexed(n.Type) {
		return false
	}
	return true
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

// collectMappedTrailingDelimiters compares syntax trivia trailing separators across
// mapped AST pairs and emits delete or insert delimiter spans when a stationary node
// gains or loses a comma or semicolon.
func collectMappedTrailingDelimiters(
	ms *engine.Mapping,
	srcRoot *treesitter.ASTNode,
	srcBytes, dstBytes []byte,
) (left, right []DelimiterSpan) {
	if ms == nil || (len(srcBytes) == 0 && len(dstBytes) == 0) {
		return nil, nil
	}

	visitPair := func(srcNode, dstNode *treesitter.ASTNode) {
		if srcNode == nil || dstNode == nil {
			return
		}
		srcTok, sStart, sEnd, srcOk := ownTrailingSeparator(srcNode, srcBytes)
		dstTok, dStart, dEnd, dstOk := ownTrailingSeparator(dstNode, dstBytes)

		if srcOk && (!dstOk || dstTok != srcTok) && !hasMatchingTrailingSeparator(srcNode, dstNode, srcTok, ms.Src(), dstBytes) {
			left = append(left, DelimiterSpan{
				StartByte: sStart,
				EndByte:   sEnd,
				Action:    "delete",
			})
		}
		if dstOk && (!srcOk || srcTok != dstTok) && !hasMatchingTrailingSeparator(dstNode, srcNode, dstTok, ms.Dst(), srcBytes) {
			right = append(right, DelimiterSpan{
				StartByte: dStart,
				EndByte:   dEnd,
				Action:    "insert",
			})
		}
	}

	if srcRoot != nil {
		for _, srcNode := range srcRoot.PreOrder() {
			if dstNode := ms.Src()[srcNode]; dstNode != nil {
				visitPair(srcNode, dstNode)
			}
		}
	} else if len(ms.Pairs) > 0 {
		for _, p := range ms.Pairs {
			visitPair(p.Src, p.Dst)
		}
	} else {
		for srcNode, dstNode := range ms.Src() {
			visitPair(srcNode, dstNode)
		}
	}

	slices.SortFunc(left, func(a, b DelimiterSpan) int {
		return cmp.Or(
			cmp.Compare(a.StartByte, b.StartByte),
			cmp.Compare(a.EndByte, b.EndByte),
		)
	})
	slices.SortFunc(right, func(a, b DelimiterSpan) int {
		return cmp.Or(
			cmp.Compare(a.StartByte, b.StartByte),
			cmp.Compare(a.EndByte, b.EndByte),
		)
	})
	return left, right
}

// ownTrailingSeparator returns the syntax trivia trailing separator token (such as a comma)
// and byte range directly attached to n, absorbing trailing horizontal whitespace when inside
// a delimited container.
func ownTrailingSeparator(n *treesitter.ASTNode, fileBytes []byte) (tok string, start, end uint32, ok bool) {
	if n == nil || len(fileBytes) == 0 {
		return "", 0, 0, false
	}
	trivia := n.SyntaxTrivia()
	if trivia == nil || trivia.TrailingEnd <= trivia.TrailingStart {
		return "", 0, 0, false
	}
	fileLen := uint32(len(fileBytes))
	parentEnd := fileLen
	if n.Parent != nil && n.Parent.EndByte < parentEnd {
		parentEnd = n.Parent.EndByte
	}
	tStart := min(trivia.TrailingStart, parentEnd)
	tEnd := min(trivia.TrailingEnd, parentEnd)
	if tEnd <= tStart || tStart < n.EndByte {
		return "", 0, 0, false
	}
	tok = string(fileBytes[tStart:tEnd])
	if n.Parent != nil && rules.IsDelimitedContainer(n.Parent.Type) {
		for tEnd < parentEnd && (fileBytes[tEnd] == ' ' || fileBytes[tEnd] == '\t') {
			tEnd++
		}
	}
	return tok, tStart, tEnd, true
}

// hasMatchingTrailingSeparator checks whether partner already owns the separator through
// an ancestor wrapper or enclosing container slot, avoiding false comma diffs when expressions
// are wrapped or unwrapped inside the same container.
func hasMatchingTrailingSeparator(
	node, partner *treesitter.ASTNode,
	tok string,
	parentMap map[*treesitter.ASTNode]*treesitter.ASTNode,
	partnerBytes []byte,
) bool {
	if node == nil || partner == nil || tok == "" {
		return false
	}
	for anc := partner.Parent; anc != nil && anc.EndByte == partner.EndByte; anc = anc.Parent {
		if ancTok, _, _, ok := ownTrailingSeparator(anc, partnerBytes); ok && ancTok == tok {
			return true
		}
	}
	if node.Parent != nil && parentMap != nil {
		if mappedParent := parentMap[node.Parent]; mappedParent != nil && mappedParent.Contains(partner) {
			for anc := partner; anc != nil && anc != mappedParent; anc = anc.Parent {
				if anc.Parent == mappedParent {
					if ancTok, _, _, ok := ownTrailingSeparator(anc, partnerBytes); ok && ancTok == tok {
						return true
					}
					break
				}
			}
		}
	}
	return false
}

// ptr returns a pointer to v. It exists because tree-sitter-go does not yet support Go's new(value)
// syntax, so ptr() lets us take the address of literals and expressions inline without temporary
// variables or tripping upstream Tree-sitter grammar errors.
func ptr[T any](v T) *T {
	return &v
}
