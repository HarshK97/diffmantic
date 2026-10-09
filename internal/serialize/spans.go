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
	startCol       int
	endCol         int
	action         string
	actRef         *Action
	moveColorIndex int
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
	skipNestedMove := nestedMoveActions(actions)

	for i := range actions {
		a := &actions[i]
		switch a.Action {
		case "delete":
			if side == "left" && a.Node != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, a.Parent, a.ASTNode)
				addSpan(spansByLine, lineIndex, fileBytes, sb, eb, "delete", a)
			}

		case "insert":
			if side == "right" && a.Node != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, a.Parent, a.ASTNode)
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
			if skipNestedMove[i] {
				continue
			}
			// Delimiters (like commas) around moved nodes aren't part of the move itself.
			// Delete them on the left and insert them on the right so they don't get painted as moves.
			if side == "left" && a.Node != nil {
				sb, eb := absorbSyntacticDelimiters(fileBytes, a.Node.StartByte, a.Node.EndByte, a.OldParent, a.ASTNode)
				if sb < a.Node.StartByte {
					addSpan(spansByLine, lineIndex, fileBytes, sb, a.Node.StartByte, "delete", a)
				}
				addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte, a.Node.EndByte, "move", a)
				if eb > a.Node.EndByte {
					addSpan(spansByLine, lineIndex, fileBytes, a.Node.EndByte, eb, "delete", a)
				}
			}
			if side == "right" {
				if dStart, dEnd, ok := actionDestBytes(a); ok {
					sb, eb := absorbSyntacticDelimiters(fileBytes, dStart, dEnd, a.Parent, a.DestASTNode)
					if sb < dStart {
						addSpan(spansByLine, lineIndex, fileBytes, sb, dStart, "insert", a)
					}
					addSpan(spansByLine, lineIndex, fileBytes, dStart, dEnd, "move", a)
					if eb > dEnd {
						addSpan(spansByLine, lineIndex, fileBytes, dEnd, eb, "insert", a)
					}
				}
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
				maxGap := 3
				if gap <= maxGap {
					onlyNonChars := true
					if gap > 0 && line < len(lineIndex) {
						lineStart := lineIndex[line]
						gapStart := lineStart + curr.endCol
						gapEnd := lineStart + next.startCol
						if gapStart < len(fileBytes) && gapEnd <= len(fileBytes) {
							gapBytes := fileBytes[gapStart:gapEnd]
							onlyNonChars = isOnlyNonCharacters(gapBytes)
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

		lineMerged = partitionLineSpans(lineMerged, side, lineIndex, fileBytes)

		slices.SortFunc(lineMerged, func(a, b internalSpan) int {
			return cmp.Or(
				cmp.Compare(a.startCol, b.startCol),
				cmp.Compare(a.endCol, b.endCol),
				cmp.Compare(a.action, b.action),
			)
		})

		for _, mSpan := range lineMerged {
			result = append(result, HighlightSpan{
				Line:       line,
				StartCol:   mSpan.startCol,
				EndCol:     mSpan.endCol,
				Action:     mSpan.action,
				ColorIndex: mSpan.moveColorIndex,
				ActionRef:  mSpan.actRef,
			})
		}
	}

	return result
}

// isActionMultiLine reports whether an action's node spans multiple lines of code on the given diff side.
// It ignores trailing whitespace and newlines so container headers ending at a newline or indent
// (like "{\n\t\t") aren't treated as multi-line moves.
func isActionMultiLine(act *Action, lineIndex []int, fileBytes []byte, side string) bool {
	if act == nil || len(lineIndex) == 0 {
		return false
	}
	var startByte, endByte uint32
	switch side {
	case "left":
		if act.Node == nil {
			return false
		}
		startByte = act.Node.StartByte
		endByte = act.Node.EndByte
	case "right":
		if act.DestStartByte != nil && act.DestEndByte != nil {
			startByte = *act.DestStartByte
			endByte = *act.DestEndByte
		} else if act.DestNode != nil {
			startByte = act.DestNode.StartByte
			endByte = act.DestNode.EndByte
		} else {
			return false
		}
	default:
		return false
	}

	for endByte > startByte && int(endByte) <= len(fileBytes) && endByte > 0 {
		b := fileBytes[endByte-1]
		if b == ' ' || b == '\t' || b == '\r' || b == '\n' {
			endByte--
		} else {
			break
		}
	}

	sStart, _ := ByteToLineCol(lineIndex, startByte)
	sEnd, endCol := ByteToLineCol(lineIndex, endByte)
	if endCol == 0 && sEnd > sStart {
		sEnd--
	}
	return sEnd > sStart
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

// nestedMoveActions drops moves nested inside a larger move across both sides.
// Intact relocated blocks collapse to a single span, while moves that split
// away or unbundle keep their own spans so both sides stay in sync.
func nestedMoveActions(actions []Action) map[int]bool {
	type moveRange struct {
		idx     int
		sStart  uint32
		sEnd    uint32
		dStart  uint32
		dEnd    uint32
		hasDest bool
	}
	type byteInterval struct {
		s uint32
		e uint32
	}

	var moves []moveRange
	var deletes []byteInterval
	var inserts []byteInterval

	for i := range actions {
		a := &actions[i]
		switch a.Action {
		case "delete":
			if a.Node != nil && a.Node.EndByte > a.Node.StartByte {
				deletes = append(deletes, byteInterval{s: a.Node.StartByte, e: a.Node.EndByte})
			}
		case "insert":
			if a.Node != nil && a.Node.EndByte > a.Node.StartByte {
				inserts = append(inserts, byteInterval{s: a.Node.StartByte, e: a.Node.EndByte})
			}
		case "move":
			if a.Node == nil || a.Node.EndByte <= a.Node.StartByte {
				continue
			}
			dStart, dEnd, hasDest := actionDestBytes(a)
			moves = append(moves, moveRange{
				idx:     i,
				sStart:  a.Node.StartByte,
				sEnd:    a.Node.EndByte,
				dStart:  dStart,
				dEnd:    dEnd,
				hasDest: hasDest,
			})
		}
	}

	// Sort by total span size descending so outermost moves are processed first.
	slices.SortFunc(moves, func(a, b moveRange) int {
		aLen := a.sEnd - a.sStart
		if a.hasDest {
			aLen += a.dEnd - a.dStart
		}
		bLen := b.sEnd - b.sStart
		if b.hasDest {
			bLen += b.dEnd - b.dStart
		}
		return cmp.Or(
			cmp.Compare(bLen, aLen),
			cmp.Compare(a.sStart, b.sStart),
		)
	})

	skip := make(map[int]bool)
	var kept []moveRange

	for _, r := range moves {
		contained := false
		for _, k := range kept {
			srcContained := k.sStart <= r.sStart && k.sEnd >= r.sEnd
			dstContained := true
			if k.hasDest && r.hasDest {
				dstContained = k.dStart <= r.dStart && k.dEnd >= r.dEnd
			} else if k.hasDest != r.hasDest {
				dstContained = false
			}

			if srcContained && dstContained {
				// If an inserted or deleted container sits between k and r, it takes precedence
				// over the outer move. Keep r so the container doesn't swallow it.
				hasInterveningDel := slices.ContainsFunc(deletes, func(del byteInterval) bool {
					return del.s <= r.sStart && del.e >= r.sEnd && k.sStart <= del.s && k.sEnd >= del.e
				})
				hasInterveningIns := k.hasDest && r.hasDest && slices.ContainsFunc(inserts, func(ins byteInterval) bool {
					return ins.s <= r.dStart && ins.e >= r.dEnd && k.dStart <= ins.s && k.dEnd >= ins.e
				})

				if !hasInterveningDel && !hasInterveningIns {
					contained = true
					break
				}
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

func actionDestBytes(a *Action) (uint32, uint32, bool) {
	if a == nil {
		return 0, 0, false
	}
	if a.DestStartByte != nil && a.DestEndByte != nil && *a.DestEndByte > *a.DestStartByte {
		return *a.DestStartByte, *a.DestEndByte, true
	}
	if a.DestNode != nil && a.DestNode.EndByte > a.DestNode.StartByte {
		return a.DestNode.StartByte, a.DestNode.EndByte, true
	}
	return 0, 0, false
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
		if dStart, dEnd, ok := actionDestBytes(a); ok {
			return int(dEnd - dStart)
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
func partitionLineSpans(spans []internalSpan, side string, lineIndex []int, fileBytes []byte) []internalSpan {
	resolveMoveColor := func(act *Action) int {
		if act == nil {
			return 0
		}
		if len(lineIndex) > 0 && isActionMultiLine(act, lineIndex, fileBytes, side) {
			return 0
		}
		return act.MoveColorIndex
	}

	if len(spans) == 0 {
		return spans
	}
	if len(spans) == 1 {
		if spans[0].action == "move" {
			spans[0].moveColorIndex = resolveMoveColor(spans[0].actRef)
		}
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
		for i := range spans {
			if spans[i].action == "move" {
				spans[i].moveColorIndex = resolveMoveColor(spans[i].actRef)
			}
		}
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
			winner   *internalSpan
			moveSpan *internalSpan
		)
		for j := range spans {
			sp := &spans[j]
			if sp.startCol <= segStart && sp.endCol >= segEnd {
				if sp.action == "move" {
					if moveSpan == nil || spanASTLength(*sp, side) < spanASTLength(*moveSpan, side) {
						moveSpan = sp
					}
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
			colorIdx := 0
			if action == "move" {
				colorIdx = resolveMoveColor(winner.actRef)
			} else if action == "update" && moveSpan != nil {
				action = "move_update"
				colorIdx = resolveMoveColor(moveSpan.actRef)
			}
			segments = append(segments, internalSpan{
				startCol:       segStart,
				endCol:         segEnd,
				action:         action,
				actRef:         winner.actRef,
				moveColorIndex: colorIdx,
			})
		}
	}

	if len(segments) == 0 {
		return spans
	}
	var coalesced []internalSpan
	cur := segments[0]
	for i := 1; i < len(segments); i++ {
		canCoalesce := segments[i].action == cur.action && segments[i].startCol == cur.endCol
		if canCoalesce && (cur.action == "move" || cur.action == "move_update") {
			// Don't merge move segments across different actions or color slots.
			if cur.actRef != segments[i].actRef || cur.moveColorIndex != segments[i].moveColorIndex {
				canCoalesce = false
			}
		}
		if canCoalesce {
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
func absorbSyntacticDelimiters(fileBytes []byte, startByte, endByte uint32, parent *NodeRef, node *treesitter.ASTNode) (uint32, uint32) {
	if parent == nil || len(fileBytes) == 0 {
		return startByte, endByte
	}
	if startByte > endByte {
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

	absorbedTrailing := false
	hasTrailingTrivia := false

	// Prefer CST trivia so comments between code and punctuation don't get swallowed.
	if node != nil {
		if trivia := node.SyntaxTrivia(); trivia != nil && trivia.TrailingEnd > trivia.TrailingStart {
			hasTrailingTrivia = true
			tStart := min(trivia.TrailingStart, parentEnd)
			tEnd := min(trivia.TrailingEnd, parentEnd)
			if tEnd > tStart && tStart >= endByte {
				isOnlyWS := true
				for i := endByte; i < tStart; i++ {
					b := fileBytes[i]
					if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
						isOnlyWS = false
						break
					}
				}
				if isOnlyWS {
					endByte = tEnd
					absorbedTrailing = true
					if rules.IsDelimitedContainer(parent.Type) {
						for endByte < parentEnd && (fileBytes[endByte] == ' ' || fileBytes[endByte] == '\t') {
							endByte++
						}
					}
				}
			}
		}
	}

	// Fall back to scanning member operators (., ->, ::) so chained access doesn't leave dangling dots.
	if !absorbedTrailing && endByte < parentEnd {
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

	// Delimited containers: grab adjacent commas so deleting an element cleans up its separator.
	if rules.IsDelimitedContainer(parent.Type) {
		// Trailing comma fallback: eat the comma after the item if CST trivia didn't already catch it.
		if !absorbedTrailing && endByte < parentEnd {
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

		// Leading comma: if this is the last element, absorb the preceding comma so we don't leave a trailing separator.
		if !absorbedTrailing && !hasTrailingTrivia && startByte > parent.StartByte {
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

	// When a switch or match pattern changes, pull in the arrow or colon so it renders as a single edit.
	if !absorbedTrailing && rules.IsCaseClause(parent.Type) {
		if endByte < parentEnd {
			idx := endByte
			for idx < parentEnd && (fileBytes[idx] == ' ' || fileBytes[idx] == '\t') {
				idx++
			}
			matched := false
			if idx+2 <= parentEnd && fileBytes[idx] == '=' && fileBytes[idx+1] == '>' {
				idx += 2
				matched = true
			} else if idx+2 <= parentEnd && fileBytes[idx] == '-' && fileBytes[idx+1] == '>' {
				idx += 2
				matched = true
			} else if idx < parentEnd && fileBytes[idx] == ':' {
				idx++
				matched = true
			}
			if matched {
				for idx < parentEnd && (fileBytes[idx] == ' ' || fileBytes[idx] == '\t') {
					idx++
				}
				endByte = idx
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
