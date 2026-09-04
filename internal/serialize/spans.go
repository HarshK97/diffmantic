package serialize

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// HighlightSpan is a visual column range to color on a specific line.
type HighlightSpan struct {
	Line      int     `json:"line"`
	StartCol  int     `json:"start_col"`
	EndCol    int     `json:"end_col"`
	Action    string  `json:"action"` // "insert", "delete", "update", "move"
	ActionRef *Action `json:"-"`
}

func (s HighlightSpan) MarshalJSON() ([]byte, error) {
	return json.Marshal([]any{s.Line, s.StartCol, s.EndCol, s.Action})
}

func (s *HighlightSpan) UnmarshalJSON(b []byte) error {
	if err := json.Unmarshal(b, &[]any{&s.Line, &s.StartCol, &s.EndCol, &s.Action}); err != nil {
		return fmt.Errorf("invalid highlight span tuple: %w", err)
	}
	return nil
}

type internalSpan struct {
	startCol int
	endCol   int
	action   string
	actRef   *Action
}

// DelimiterSpan tracks a closing delimiter (like a brace or bracket) for UI highlights.
type DelimiterSpan struct {
	StartByte uint32
	EndByte   uint32
	Action    string // "insert", "delete", "move"
	ActionRef *Action
}

// BuildHighlightSpans returns pre-merged highlight spans for one side of a diff.
// It splits byte ranges across newlines and merges adjacent spans of the same
// action when separated only by small punctuation or whitespace gaps.
// Pass extraSpans to include closing delimiter highlights without adding extra actions.
func BuildHighlightSpans(fileBytes []byte, actions []Action, side string, extraSpans ...DelimiterSpan) []HighlightSpan {
	if (len(actions) == 0 && len(extraSpans) == 0) || len(fileBytes) == 0 {
		return nil
	}

	lineIndex := BuildLineIndex(fileBytes)
	spansByLine := make(map[int][]internalSpan)

	for i := range actions {
		a := &actions[i]
		switch a.Action {
		case "delete":
			if side == "left" && a.Node != nil {
				addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte, a.Node.EndByte, "delete", a)
			}

		case "insert":
			if side == "right" && a.Node != nil {
				addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte, a.Node.EndByte, "insert", a)
			}

		case "update":
			if side == "left" && a.Node != nil {
				addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte, a.Node.EndByte, "update", a)
			}
			if side == "right" && a.DestNode != nil {
				addSpan(spansByLine, lineIndex, fileBytes, a.DestNode.StartByte, a.DestNode.EndByte, "update", a)
			}

		case "move":
			actType := "move"
			if side == "left" && a.Node != nil {
				addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte, a.Node.EndByte, actType, a)
			}
			if side == "right" && a.DestStartByte != nil && a.DestEndByte != nil {
				addSpan(spansByLine, lineIndex, fileBytes, *a.DestStartByte, *a.DestEndByte, actType, a)
			}
		}
	}

	for _, extra := range extraSpans {
		if (side == "left" && (extra.Action == "delete" || extra.Action == "move")) ||
			(side == "right" && (extra.Action == "insert" || extra.Action == "move")) {
			addSpan(spansByLine, lineIndex, fileBytes, extra.StartByte, extra.EndByte, extra.Action, extra.ActionRef)
		}
	}

	var result []HighlightSpan

	lines := slices.Sorted(maps.Keys(spansByLine))

	for _, line := range lines {
		lineSpans := spansByLine[line]
		if len(lineSpans) == 0 {
			continue
		}

		groups := make(map[string][]internalSpan)
		for _, s := range lineSpans {
			groups[s.action] = append(groups[s.action], s)
		}

		var lineMerged []internalSpan
		for _, actStr := range []string{"delete", "insert", "update", "move"} {
			gSpans := groups[actStr]
			if len(gSpans) == 0 {
				continue
			}
			slices.SortFunc(gSpans, func(a, b internalSpan) int {
				return cmp.Or(
					cmp.Compare(a.startCol, b.startCol),
					cmp.Compare(a.endCol, b.endCol),
				)
			})

			curr := gSpans[0]
			for i := 1; i < len(gSpans); i++ {
				next := gSpans[i]

				if next.startCol >= curr.startCol && next.endCol <= curr.endCol {
					if next.actRef != curr.actRef {
						lineMerged = append(lineMerged, next)
					}
					continue
				}

				canMerge := false
				gap := next.startCol - curr.endCol
				if gap <= 3 {
					onlyNonChars := true
					if gap > 0 && line < len(lineIndex) {
						lineStart := lineIndex[line]
						gapStart := lineStart + curr.endCol
						gapEnd := lineStart + next.startCol
						if gapStart < len(fileBytes) && gapEnd <= len(fileBytes) {
							onlyNonChars = isOnlyNonCharacters(fileBytes[gapStart:gapEnd])
						}
					}

					if onlyNonChars {
						if actStr == "update" || actStr == "move" {
							if curr.actRef != nil && next.actRef != nil {
								if curr.actRef.GroupID != "" && curr.actRef.GroupID == next.actRef.GroupID {
									canMerge = true
								} else if actStr == "update" {
									canMerge = sharesLineage(curr.actRef.Parent, next.actRef.Parent)
								} else {
									canMerge = sharesLineage(curr.actRef.Parent, next.actRef.Parent) &&
										sharesLineage(curr.actRef.OldParent, next.actRef.OldParent)
								}
							}
						} else {
							canMerge = true
						}
					}
				}

				if canMerge {
					if next.endCol > curr.endCol {
						// When curr is an inner sub-span coaligned at the start with
						// a wider container next (e.g. curr: 0..3, next: 0..18),
						// preserve curr before expanding to the outer container.
						if curr.startCol == next.startCol && next.actRef != nil && curr.actRef != nil && next.actRef != curr.actRef {
							lineMerged = append(lineMerged, curr)
						}

						// Adopt the wider actRef so the merged span's
						// AST length reflects the true outer container,
						// preventing a small inner node from falsely winning
						// layering priority over overlapping spans of
						// other action types.
						if next.actRef != nil && curr.actRef != nil {
							currNodeLen := nodeLen(curr.actRef, side)
							nextNodeLen := nodeLen(next.actRef, side)
							if nextNodeLen > currNodeLen {
								curr.actRef = next.actRef
							}
						}
						curr.endCol = next.endCol
					}
				} else {
					lineMerged = append(lineMerged, curr)
					curr = next
				}
			}
			lineMerged = append(lineMerged, curr)
		}

		slices.SortFunc(lineMerged, func(a, b internalSpan) int {
			return cmp.Or(
				cmp.Compare(a.startCol, b.startCol),
				cmp.Compare(a.endCol, b.endCol),
				cmp.Compare(a.action, b.action),
			)
		})

		for _, mSpan := range lineMerged {
			result = append(result, HighlightSpan{
				Line:      line,
				StartCol:  mSpan.startCol,
				EndCol:    mSpan.endCol,
				Action:    mSpan.action,
				ActionRef: mSpan.actRef,
			})
		}
	}

	return result
}

func addSpan(spans map[int][]internalSpan, lineIndex []int, fileBytes []byte, startByte, endByte uint32, actStr string, action *Action) {
	ForEachLineSpan(lineIndex, fileBytes, startByte, endByte, func(line, sc, ec int) {
		if line < len(lineIndex) {
			lineStart := lineIndex[line]
			sByte := lineStart + sc
			eByte := lineStart + ec
			if sByte < len(fileBytes) && eByte <= len(fileBytes) {
				if sc > 0 {
					for sByte < eByte && (fileBytes[sByte] == ' ' || fileBytes[sByte] == '\t') {
						sByte++
						sc++
					}
				}
				for eByte > sByte && (fileBytes[eByte-1] == ' ' || fileBytes[eByte-1] == '\t' || fileBytes[eByte-1] == '\r' || fileBytes[eByte-1] == '\n') {
					eByte--
					ec--
				}
			}
		}
		if ec > sc {
			spans[line] = append(spans[line], internalSpan{
				startCol: sc,
				endCol:   ec,
				action:   actStr,
				actRef:   action,
			})
		}
	})
}

func isOnlyNonCharacters(b []byte) bool {
	if bytes.ContainsAny(b, "<>=+-*/%!&|^~?") {
		return false
	}
	for _, c := range b {
		if treesitter.IsWordChar(c) {
			return false
		}
	}
	return true
}

func nodeRefsEqual(n1, n2 *NodeRef) bool {
	if n1 == n2 {
		return true
	}
	if n1 == nil || n2 == nil {
		return false
	}
	return n1.Tree == n2.Tree && n1.StartByte == n2.StartByte && n1.EndByte == n2.EndByte && n1.Type == n2.Type
}

func isAncestorRef(a, b *NodeRef) bool {
	if a == nil || b == nil {
		return false
	}
	return a.Tree == b.Tree && a.StartByte <= b.StartByte && a.EndByte >= b.EndByte
}

func sharesLineage(n1, n2 *NodeRef) bool {
	return nodeRefsEqual(n1, n2) || isAncestorRef(n1, n2) || isAncestorRef(n2, n1)
}

// nodeLen returns the AST byte range of an action on the specified side.
func nodeLen(a *Action, side string) int {
	if a == nil {
		return 0
	}
	if side == "left" {
		if a.Node != nil {
			return int(a.Node.EndByte - a.Node.StartByte)
		}
	} else {
		if a.DestStartByte != nil && a.DestEndByte != nil {
			return int(*a.DestEndByte - *a.DestStartByte)
		}
		if a.DestNode != nil {
			return int(a.DestNode.EndByte - a.DestNode.StartByte)
		}
		if a.Node != nil {
			return int(a.Node.EndByte - a.Node.StartByte)
		}
	}
	return 0
}
