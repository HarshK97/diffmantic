package serialize

import (
	"bytes"
	"maps"
	"slices"
	"sort"

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

type internalSpan struct {
	startCol int
	endCol   int
	action   string
	actRef   *Action
}

// BuildHighlightSpans returns pre-merged highlight spans for one side of a diff.
// It splits byte ranges across newlines and merges adjacent spans of the same
// action when separated only by small punctuation or whitespace gaps.
func BuildHighlightSpans(fileBytes []byte, actions []Action, side string) []HighlightSpan {
	if len(actions) == 0 || len(fileBytes) == 0 {
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
			sort.Slice(gSpans, func(i, j int) bool {
				if gSpans[i].startCol != gSpans[j].startCol {
					return gSpans[i].startCol < gSpans[j].startCol
				}
				return gSpans[i].endCol < gSpans[j].endCol
			})

			curr := gSpans[0]
			for i := 1; i < len(gSpans); i++ {
				next := gSpans[i]
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
						curr.endCol = next.endCol
					}
				} else {
					lineMerged = append(lineMerged, curr)
					curr = next
				}
			}
			lineMerged = append(lineMerged, curr)
		}

		sort.Slice(lineMerged, func(i, j int) bool {
			if lineMerged[i].startCol != lineMerged[j].startCol {
				return lineMerged[i].startCol < lineMerged[j].startCol
			}
			if lineMerged[i].endCol != lineMerged[j].endCol {
				return lineMerged[i].endCol < lineMerged[j].endCol
			}
			return lineMerged[i].action < lineMerged[j].action
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
		spans[line] = append(spans[line], internalSpan{
			startCol: sc,
			endCol:   ec,
			action:   actStr,
			actRef:   action,
		})
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
	return n1 == n2 || (n1 != nil && n2 != nil && slices.Equal(n1.Path, n2.Path))
}

func isAncestorRef(a, b *NodeRef) bool {
	if a == nil || b == nil {
		return false
	}
	return len(b.Path) >= len(a.Path) && slices.Equal(a.Path, b.Path[:len(a.Path)])
}

func sharesLineage(n1, n2 *NodeRef) bool {
	return nodeRefsEqual(n1, n2) || isAncestorRef(n1, n2) || isAncestorRef(n2, n1)
}
