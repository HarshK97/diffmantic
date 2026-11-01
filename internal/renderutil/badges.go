package renderutil

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// MoveBadges contains line-level arrow annotations for moves.
type MoveBadges struct {
	SrcLineBadges      map[int]string // Source line index -> " ➔ L{dst}"
	DstLineBadges      map[int]string // Destination line index -> " ↳ L{src}"
	SrcLineBadgeColors map[int]int    // Source line index -> Color index (0: Cyan, 1: Magenta)
	DstLineBadgeColors map[int]int    // Destination line index -> Color index (0: Cyan, 1: Magenta)
}

// BuildMoveBadges computes directional line badges for moves.
func BuildMoveBadges(
	actions []serialize.Action,
	hunks []Interval,
	pairs []serialize.LineAlignmentPair,
	srcOffsets, dstOffsets []int,
	srcLines, dstLines []string,
	r *rules.Rules,
) MoveBadges {
	meta := MoveBadges{
		SrcLineBadges:      make(map[int]string),
		DstLineBadges:      make(map[int]string),
		SrcLineBadgeColors: make(map[int]int),
		DstLineBadgeColors: make(map[int]int),
	}
	if len(hunks) == 0 || len(actions) == 0 {
		return meta
	}

	const maxSuppressAlignDist = 15

	srcLineToHunk := make(map[int]int, len(pairs))
	dstLineToHunk := make(map[int]int, len(pairs))
	srcLineToPairIdx := make(map[int]int, len(pairs))
	dstLineToPairIdx := make(map[int]int, len(pairs))
	for hIdx, h := range hunks {
		for p := h.Start; p <= h.End && p < len(pairs); p++ {
			pair := pairs[p]
			if pair.LeftLine >= 0 {
				srcLineToHunk[pair.LeftLine] = hIdx
				srcLineToPairIdx[pair.LeftLine] = p
			}
			if pair.RightLine >= 0 {
				dstLineToHunk[pair.RightLine] = hIdx
				dstLineToPairIdx[pair.RightLine] = p
			}
		}
	}

	deletedStartBytes := make(map[uint32]struct{}, len(actions))
	insertedStartBytes := make(map[uint32]struct{}, len(actions))
	for _, a := range actions {
		if a.Node == nil {
			continue
		}
		if a.Action == "delete" {
			deletedStartBytes[a.Node.StartByte] = struct{}{}
		}
		if a.Action == "insert" {
			insertedStartBytes[a.Node.StartByte] = struct{}{}
		}
	}

	type moveCandidate struct {
		act            *serialize.Action
		sStart, sEnd   int
		dStart, dEnd   int
		sStartByte     uint32
		sEndByte       uint32
		dStartByte     uint32
		dEndByte       uint32
		dStartByteLine uint32
		inSrc, inDst   bool
		alignedNear    bool
		hasSrcBadge    bool
		hasDstBadge    bool
	}

	candidates := make([]*moveCandidate, 0, len(actions))
	for i := range actions {
		a := &actions[i]
		if a.Action != "move" || a.Node == nil {
			continue
		}
		sStart, _ := serialize.ByteToLineCol(srcOffsets, a.Node.StartByte)
		sEndLineByte := a.Node.EndByte
		if a.OrigEndByte > a.OrigStartByte {
			sEndLineByte = a.OrigEndByte
		}
		sEnd, _ := serialize.ByteToLineCol(srcOffsets, sEndLineByte)

		var dStartByteLine, dEndLineByte uint32
		if a.DestStartByte != nil && a.DestEndByte != nil {
			dStartByteLine = *a.DestStartByte
			dEndLineByte = *a.DestEndByte
		} else if a.DestNode != nil {
			dStartByteLine = a.DestNode.StartByte
			dEndLineByte = a.DestNode.EndByte
		} else {
			continue
		}
		if a.DestOrigEndByte > a.DestOrigStartByte {
			dEndLineByte = a.DestOrigEndByte
		}
		dStart, _ := serialize.ByteToLineCol(dstOffsets, dStartByteLine)
		dEnd, _ := serialize.ByteToLineCol(dstOffsets, dEndLineByte)

		hSrc, inSrcHunk := srcLineToHunk[sStart]
		hDst, inDstHunk := dstLineToHunk[dStart]

		isDecl := (r != nil && r.IsDeclaration(a.Node.Type)) || (r == nil && rules.IsDeclaration(a.Node.Type))
		isBlock := (r != nil && r.IsBlock(a.Node.Type)) || (r == nil && rules.IsBlock(a.Node.Type))
		isMultiLine := sEnd > sStart || dEnd > dStart
		isStatement := (r != nil && (r.IsStatement(a.Node.Type) || r.IsCall(a.Node.Type))) || (r == nil && (rules.IsStatement(a.Node.Type) || rules.IsCall(a.Node.Type)))
		if !isDecl && !isBlock && !isMultiLine && !isStatement {
			continue
		}
		alignedNear := false
		if inSrcHunk && inDstHunk && hSrc == hDst {
			alignDist := srcLineToPairIdx[sStart] - dstLineToPairIdx[dStart]
			if alignDist < 0 {
				alignDist = -alignDist
			}
			alignedNear = alignDist <= maxSuppressAlignDist
		}

		sStartByte := a.OrigStartByte
		sEndByte := a.OrigEndByte
		if sStartByte == 0 && sEndByte == 0 {
			sStartByte = a.Node.StartByte
			sEndByte = a.Node.EndByte
		}

		dStartByte := a.DestOrigStartByte
		dEndByte := a.DestOrigEndByte
		if dStartByte == 0 && dEndByte == 0 {
			if a.DestStartByte != nil && a.DestEndByte != nil {
				dStartByte = *a.DestStartByte
				dEndByte = *a.DestEndByte
			} else if a.DestNode != nil {
				dStartByte = a.DestNode.StartByte
				dEndByte = a.DestNode.EndByte
			}
		}

		candidates = append(candidates, &moveCandidate{
			act:            a,
			sStart:         sStart,
			sEnd:           sEnd,
			dStart:         dStart,
			dEnd:           dEnd,
			sStartByte:     sStartByte,
			sEndByte:       sEndByte,
			dStartByte:     dStartByte,
			dEndByte:       dEndByte,
			dStartByteLine: dStartByteLine,
			inSrc:          inSrcHunk,
			inDst:          inDstHunk,
			alignedNear:    alignedNear,
		})
	}

	// Sort by AST byte span descending so outer enclosing containers claim badges first
	slices.SortFunc(candidates, func(a, b *moveCandidate) int {
		return cmp.Or(
			cmp.Compare(b.sEndByte-b.sStartByte, a.sEndByte-a.sStartByte),
			cmp.Compare(b.dEndByte-b.dStartByte, a.dEndByte-a.dStartByte),
			cmp.Compare(a.sStart, b.sStart),
		)
	})

	// If any move in a group spans multiple lines, collapse all member badges to Cyan (slot 0) so they match how the spans render.
	multiLineGroupsSrc := make(map[string]bool)
	multiLineGroupsDst := make(map[string]bool)
	for _, c := range candidates {
		if c.act.GroupID != "" {
			if c.sEnd > c.sStart {
				multiLineGroupsSrc[c.act.GroupID] = true
			}
			if c.dEnd > c.dStart {
				multiLineGroupsDst[c.act.GroupID] = true
			}
		}
	}

	// Only badge the first statement in a contiguous run of single-line moves within the same group.
	// Multiline compound statements (like if/for) always keep their badges as landmarks.
	suppressFlatFollower := make(map[*moveCandidate]bool)
	candidatesByGroup := make(map[string][]*moveCandidate)
	for _, c := range candidates {
		if c.act.GroupID != "" {
			candidatesByGroup[c.act.GroupID] = append(candidatesByGroup[c.act.GroupID], c)
		}
	}

	for _, grp := range candidatesByGroup {
		if len(grp) < 2 {
			continue
		}
		slices.SortFunc(grp, func(a, b *moveCandidate) int {
			return cmp.Or(
				cmp.Compare(a.sStart, b.sStart),
				cmp.Compare(a.sStartByte, b.sStartByte),
			)
		})

		for i := 1; i < len(grp); i++ {
			prev := grp[i-1]
			curr := grp[i]

			prevSingle := (prev.sStart == prev.sEnd) && (prev.dStart == prev.dEnd)
			currSingle := (curr.sStart == curr.sEnd) && (curr.dStart == curr.dEnd)
			if !prevSingle || !currSingle {
				continue
			}

			srcContiguous := (curr.sStart == prev.sEnd+1) ||
				(curr.sStart > prev.sEnd+1 && isLinesTriviaOnly(srcLines, prev.sEnd+1, curr.sStart-1))
			dstContiguous := (curr.dStart == prev.dEnd+1) ||
				(curr.dStart > prev.dEnd+1 && isLinesTriviaOnly(dstLines, prev.dEnd+1, curr.dStart-1))

			if srcContiguous && dstContiguous {
				suppressFlatFollower[curr] = true
			}
		}
	}

	for cIdx, c := range candidates {
		suppressSrc := c.alignedNear || suppressFlatFollower[c]
		suppressDst := c.alignedNear || suppressFlatFollower[c]

		// Don't badge children if their enclosing parent move already has a badge (or is nearby) and they stayed inside it.
		for _, p := range candidates[:cIdx] {
			enclosesSrc := p.sStartByte <= c.sStartByte && c.sEndByte <= p.sEndByte &&
				(p.sStartByte < c.sStartByte || c.sEndByte < p.sEndByte)
			enclosesDst := p.dStartByte <= c.dStartByte && c.dEndByte <= p.dEndByte &&
				(p.dStartByte < c.dStartByte || c.dEndByte < p.dEndByte)

			if enclosesSrc && enclosesDst {
				if p.hasSrcBadge || p.alignedNear {
					suppressSrc = true
				}
				if p.hasDstBadge || p.alignedNear {
					suppressDst = true
				}
				if suppressSrc && suppressDst {
					break
				}
			}
		}

		_, isDeleted := deletedStartBytes[c.act.Node.StartByte]
		_, isInserted := insertedStartBytes[c.dStartByteLine]

		if c.inSrc && c.sStart >= 0 && c.sStart < len(srcLines) && !isDeleted && !suppressSrc {
			if _, exists := meta.SrcLineBadges[c.sStart]; !exists {
				meta.SrcLineBadges[c.sStart] = fmt.Sprintf(" ➔ L%d", c.dStart+1)
				meta.SrcLineBadgeColors[c.sStart] = serialize.ResolveMoveColor(c.act, c.sEnd-c.sStart+1, multiLineGroupsSrc[c.act.GroupID])
				c.hasSrcBadge = true
			}
		}
		if c.inDst && c.dStart >= 0 && c.dStart < len(dstLines) && !isInserted && !suppressDst {
			if _, exists := meta.DstLineBadges[c.dStart]; !exists {
				meta.DstLineBadges[c.dStart] = fmt.Sprintf(" ↳ L%d", c.sStart+1)
				meta.DstLineBadgeColors[c.dStart] = serialize.ResolveMoveColor(c.act, c.dEnd-c.dStart+1, multiLineGroupsDst[c.act.GroupID])
				c.hasDstBadge = true
			}
		}
	}

	return meta
}

// isTriviaOrBlankLine reports whether line contains only whitespace or comments.
func isTriviaOrBlankLine(line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return true
	}
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "--")
}

// isLinesTriviaOnly reports whether all lines from startLine to endLine (inclusive) are empty or comments.
func isLinesTriviaOnly(lines []string, startLine, endLine int) bool {
	if startLine < 0 || endLine >= len(lines) {
		return false
	}
	for l := startLine; l <= endLine; l++ {
		if !isTriviaOrBlankLine(lines[l]) {
			return false
		}
	}
	return true
}
