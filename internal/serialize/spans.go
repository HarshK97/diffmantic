package serialize

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// HighlightSpan is a visual column range to color on a specific line.
type HighlightSpan struct {
	Line       int     `json:"line"`
	StartCol   int     `json:"start_col"`
	EndCol     int     `json:"end_col"`
	Action     string  `json:"action"` // "insert", "delete", "update", "move"
	ColorIndex int     `json:"-"`
	ActionRef  *Action `json:"-"`
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

	// Only the outermost move of a relocated block gets spans. The nested
	// ones would paint the same teal twice.
	skipNestedMove := nestedMoveActions(actions, side)

	for i := range actions {
		a := &actions[i]
		switch a.Action {
		case "delete":
			if side == "left" && a.Node != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, a.Parent)
				addSpan(spansByLine, lineIndex, fileBytes, sb, eb, "delete", a)
			}

		case "insert":
			if side == "right" && a.Node != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, a.Parent)
				addSpan(spansByLine, lineIndex, fileBytes, sb, eb, "insert", a)
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
			if skipNestedMove[i] {
				continue
			}
			if side == "left" && a.Node != nil {
				parent := a.OldParent
				if parent == nil {
					parent = a.Parent
				}
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, parent)
				addSpan(spansByLine, lineIndex, fileBytes, sb, eb, actType, a)
			}
			if side == "right" && a.DestStartByte != nil && a.DestEndByte != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, *a.DestStartByte, *a.DestEndByte, a.Parent)
				addSpan(spansByLine, lineIndex, fileBytes, sb, eb, actType, a)
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
						closeTrailingDelimiter(&next, line, lineIndex, fileBytes)
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
								if curr.actRef.MoveColorIndex != next.actRef.MoveColorIndex {
									canMerge = false
								} else if curr.actRef.GroupID != "" && curr.actRef.GroupID == next.actRef.GroupID {
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
						// Preserve curr if next is a wider container starting at the same column.
						if curr.startCol == next.startCol && next.actRef != nil && curr.actRef != nil && next.actRef != curr.actRef {
							closeTrailingDelimiter(&curr, line, lineIndex, fileBytes)
							lineMerged = append(lineMerged, curr)
						}

						// Use the wider action reference so larger containers don't lose layering priority.
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
					closeTrailingDelimiter(&curr, line, lineIndex, fileBytes)
					lineMerged = append(lineMerged, curr)
					curr = next
				}
			}
			closeTrailingDelimiter(&curr, line, lineIndex, fileBytes)
			lineMerged = append(lineMerged, curr)
		}

		lineMerged = partitionLineSpans(lineMerged, side)

		slices.SortFunc(lineMerged, func(a, b internalSpan) int {
			return cmp.Or(
				cmp.Compare(a.startCol, b.startCol),
				cmp.Compare(a.endCol, b.endCol),
				cmp.Compare(a.action, b.action),
			)
		})

		for _, mSpan := range lineMerged {
			colorIdx := 0
			if mSpan.actRef != nil {
				colorIdx = mSpan.actRef.MoveColorIndex
			}
			result = append(result, HighlightSpan{
				Line:       line,
				StartCol:   mSpan.startCol,
				EndCol:     mSpan.endCol,
				Action:     mSpan.action,
				ColorIndex: colorIdx,
				ActionRef:  mSpan.actRef,
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

// nestedMoveActions finds moves buried inside a bigger move on the same side.
// Skipping them keeps a relocated block to one span instead of one per token.
func nestedMoveActions(actions []Action, side string) map[int]bool {
	type byteRange struct {
		idx int
		s   uint32
		e   uint32
	}
	var ranges []byteRange
	for i := range actions {
		a := &actions[i]
		if a.Action != "move" {
			continue
		}
		var s, e uint32
		var ok bool
		if side == "left" && a.Node != nil {
			s, e, ok = a.Node.StartByte, a.Node.EndByte, true
		} else if side == "right" {
			if a.DestStartByte != nil && a.DestEndByte != nil {
				s, e, ok = *a.DestStartByte, *a.DestEndByte, true
			} else if a.DestNode != nil {
				s, e, ok = a.DestNode.StartByte, a.DestNode.EndByte, true
			}
		}
		if !ok || e <= s {
			continue
		}
		ranges = append(ranges, byteRange{idx: i, s: s, e: e})
	}
	// Plain O(n^2) scan, there are never enough moves per file for this to matter.
	slices.SortFunc(ranges, func(a, b byteRange) int {
		return cmp.Or(
			cmp.Compare(b.e-b.s, a.e-a.s),
			cmp.Compare(a.s, b.s),
		)
	})
	skip := make(map[int]bool)
	var kept []byteRange
	for _, r := range ranges {
		contained := false
		for _, k := range kept {
			if k.s <= r.s && k.e >= r.e {
				contained = true
				break
			}
		}
		if contained {
			skip[r.idx] = true
			continue
		}
		kept = append(kept, r)
	}
	return skip
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

// spanASTLength returns the AST byte range for a span on the given side.
// Falls back to column width when no AST node is attached so spans can still be compared.
func spanASTLength(sp internalSpan, side string) int {
	if n := nodeLen(sp.actRef, side); n > 0 {
		return n
	}
	return sp.endCol - sp.startCol
}

// parseSpanActionPriority assigns tiebreaker priorities when two spans have identical AST lengths.
// Higher values win: move_update (5) > update (4) > move (3) > insert (2) > delete (1).
func parseSpanActionPriority(act string) int {
	switch act {
	case "move_update":
		return 5
	case "update":
		return 4
	case "move":
		return 3
	case "insert":
		return 2
	case "delete":
		return 1
	default:
		return 0
	}
}

// partitionLineSpans resolves cross-action overlaps for a single line's spans.
func partitionLineSpans(spans []internalSpan, side string) []internalSpan {
	if len(spans) <= 1 {
		return spans
	}

	var hasOverlap bool
checkOverlap:
	for i := range len(spans) {
		for j := i + 1; j < len(spans); j++ {
			if spans[i].startCol < spans[j].endCol && spans[j].startCol < spans[i].endCol {
				hasOverlap = true
				break checkOverlap
			}
		}
	}
	if !hasOverlap {
		return spans
	}

	boundaries := make(map[int]struct{})
	for _, sp := range spans {
		boundaries[sp.startCol] = struct{}{}
		boundaries[sp.endCol] = struct{}{}
	}
	sortedBounds := slices.Sorted(maps.Keys(boundaries))

	var segments []internalSpan
	for i := range len(sortedBounds) - 1 {
		segStart := sortedBounds[i]
		segEnd := sortedBounds[i+1]
		if segStart >= segEnd {
			continue
		}

		var (
			winner  *internalSpan
			hasMove bool
		)
		for j := range spans {
			sp := &spans[j]
			if sp.startCol <= segStart && sp.endCol >= segEnd {
				if sp.action == "move" {
					hasMove = true
				}
				if winner == nil {
					winner = sp
					continue
				}
				wAstLen := spanASTLength(*winner, side)
				sAstLen := spanASTLength(*sp, side)
				candWidth := sp.endCol - sp.startCol
				wCandWidth := winner.endCol - winner.startCol
				if sAstLen < wAstLen ||
					(sAstLen == wAstLen && candWidth < wCandWidth) ||
					(sAstLen == wAstLen && candWidth == wCandWidth && parseSpanActionPriority(sp.action) > parseSpanActionPriority(winner.action)) {
					winner = sp
				}
			}
		}
		if winner != nil {
			action := winner.action
			if action == "update" && hasMove {
				action = "move_update"
			}
			segments = append(segments, internalSpan{
				startCol: segStart,
				endCol:   segEnd,
				action:   action,
				actRef:   winner.actRef,
			})
		}
	}

	if len(segments) == 0 {
		return spans
	}
	var coalesced []internalSpan
	cur := segments[0]
	for i := 1; i < len(segments); i++ {
		if segments[i].action == cur.action && segments[i].startCol == cur.endCol {
			cur.endCol = segments[i].endCol
		} else {
			coalesced = append(coalesced, cur)
			cur = segments[i]
		}
	}
	coalesced = append(coalesced, cur)
	return coalesced
}

// absorbSyntacticDelimiters expands a span to cover adjacent member connectors
// (., ->, ::) or sequence commas so punctuation doesn't get left unhighlighted.
func absorbSyntacticDelimiters(fileBytes []byte, startByte, endByte uint32, parent *NodeRef) (uint32, uint32) {
	if parent == nil || len(fileBytes) == 0 {
		return startByte, endByte
	}
	if startByte < parent.StartByte || endByte > parent.EndByte {
		return startByte, endByte
	}
	fileLen := uint32(len(fileBytes))
	if startByte > fileLen || endByte > fileLen {
		return startByte, endByte
	}
	parentEnd := parent.EndByte
	if parentEnd > fileLen {
		parentEnd = fileLen
	}

	// Member connectors (e.g. "gin.", "ptr->", "std::"):
	if endByte < parentEnd {
		if endByte+2 <= parentEnd && fileBytes[endByte] == ':' && fileBytes[endByte+1] == ':' {
			endByte += 2
		} else if endByte+2 <= parentEnd && fileBytes[endByte] == '-' && fileBytes[endByte+1] == '>' {
			endByte += 2
		} else if fileBytes[endByte] == '.' {
			notNextDot := (endByte+1 >= fileLen || fileBytes[endByte+1] != '.')
			notPrevDot := (endByte == 0 || fileBytes[endByte-1] != '.')
			if notNextDot && notPrevDot {
				endByte++
			}
		}
	}

	if startByte > parent.StartByte {
		if startByte >= parent.StartByte+2 && fileBytes[startByte-2] == ':' && fileBytes[startByte-1] == ':' {
			startByte -= 2
		} else if startByte >= parent.StartByte+2 && fileBytes[startByte-2] == '-' && fileBytes[startByte-1] == '>' {
			startByte -= 2
		} else if fileBytes[startByte-1] == '.' {
			notPrevDot := (startByte < 2 || fileBytes[startByte-2] != '.')
			notNextDot := (startByte >= fileLen || fileBytes[startByte] != '.')
			if notPrevDot && notNextDot {
				startByte--
			}
		}
	}

	// Commas inside delimited containers (argument lists, arrays, parameters):
	if rules.IsDelimitedContainer(parent.Type) {
		absorbedTrailing := false

		// Trailing comma (e.g. "a," in "foo(a, b)" or multiline entries):
		if endByte < parentEnd {
			idx := endByte
			for idx < parentEnd && (fileBytes[idx] == ' ' || fileBytes[idx] == '\t') {
				idx++
			}
			if idx < parentEnd && fileBytes[idx] == ',' {
				idx++
				for idx < parentEnd && (fileBytes[idx] == ' ' || fileBytes[idx] == '\t') {
					idx++
				}
				endByte = idx
				absorbedTrailing = true
			}
		}

		// Leading comma (e.g. ", c" in "foo(a, b, c)"):
		if !absorbedTrailing && startByte > parent.StartByte {
			idx := startByte
			for idx > parent.StartByte && (fileBytes[idx-1] == ' ' || fileBytes[idx-1] == '\t') {
				idx--
			}
			if idx > parent.StartByte && fileBytes[idx-1] == ',' {
				idx--
				startByte = idx
			}
		}
	}

	return startByte, endByte
}

// closeTrailingDelimiter expands s to include matching closing brackets or parentheses
// when s contains unclosed opening delimiters.
func closeTrailingDelimiter(s *internalSpan, line int, lineIndex []int, fileBytes []byte) {
	if line >= len(lineIndex) {
		return
	}
	lineStart := lineIndex[line]
	sByte := lineStart + s.startCol
	eByte := lineStart + s.endCol
	if sByte >= eByte || sByte < 0 || eByte >= len(fileBytes) {
		return
	}
	for sByte < len(fileBytes) && eByte < len(fileBytes) {
		sub := fileBytes[sByte:eByte]
		nextChar := fileBytes[eByte]
		if nextChar == ')' && bytes.Count(sub, []byte("(")) > bytes.Count(sub, []byte(")")) {
			s.endCol++
			eByte++
		} else if nextChar == ']' && bytes.Count(sub, []byte("[")) > bytes.Count(sub, []byte("]")) {
			s.endCol++
			eByte++
		} else if nextChar == '}' && bytes.Count(sub, []byte("{")) > bytes.Count(sub, []byte("}")) {
			s.endCol++
			eByte++
		} else {
			break
		}
	}
}
