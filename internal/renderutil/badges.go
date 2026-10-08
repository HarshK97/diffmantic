package renderutil

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// MoveBadges contains line-level arrow annotations and hunk header descriptions for moves.
type MoveBadges struct {
	SrcLineBadges      map[int]string // Source line index -> " ➔ L{dst}"
	DstLineBadges      map[int]string // Destination line index -> " ⤹ L{src}"
	SrcLineBadgeColors map[int]int    // Source line index -> Color index (0: Teal, 1: Mauve, 2: Sapphire)
	DstLineBadgeColors map[int]int    // Destination line index -> Color index (0: Teal, 1: Mauve, 2: Sapphire)
	HunkHeaders        map[int]string // Hunk index -> " func Foo() (moved to L{dst})"
}

// BuildMoveBadges computes directional line badges and hunk header move annotations.
func BuildMoveBadges(
	actions []serialize.Action,
	hunks []Interval,
	pairs []serialize.LineAlignmentPair,
	srcOffsets, dstOffsets []int,
	srcLines, dstLines []string,
	r *rules.Rules,
	promoteDeclarations bool,
) MoveBadges {
	meta := MoveBadges{
		SrcLineBadges:      make(map[int]string),
		DstLineBadges:      make(map[int]string),
		SrcLineBadgeColors: make(map[int]int),
		DstLineBadgeColors: make(map[int]int),
		HunkHeaders:        make(map[int]string),
	}
	if len(hunks) == 0 || len(actions) == 0 {
		return meta
	}

	srcLineToHunk := make(map[int]int)
	dstLineToHunk := make(map[int]int)
	for hIdx, h := range hunks {
		for p := h.Start; p <= h.End && p < len(pairs); p++ {
			pair := pairs[p]
			if pair.LeftLine >= 0 {
				srcLineToHunk[pair.LeftLine] = hIdx
			}
			if pair.RightLine >= 0 {
				dstLineToHunk[pair.RightLine] = hIdx
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
		hSrc, hDst     int
		inSrc, inDst   bool
		isDecl         bool
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
		sEnd, _ := serialize.ByteToLineCol(srcOffsets, a.Node.EndByte)

		var dStart, dEnd int
		var dStartByteLine uint32
		if a.DestStartByte != nil && a.DestEndByte != nil {
			dStartByteLine = *a.DestStartByte
			dStart, _ = serialize.ByteToLineCol(dstOffsets, *a.DestStartByte)
			dEnd, _ = serialize.ByteToLineCol(dstOffsets, *a.DestEndByte)
		} else if a.DestNode != nil {
			dStartByteLine = a.DestNode.StartByte
			dStart, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.StartByte)
			dEnd, _ = serialize.ByteToLineCol(dstOffsets, a.DestNode.EndByte)
		} else {
			continue
		}

		hSrc, inSrcHunk := srcLineToHunk[sStart]
		hDst, inDstHunk := dstLineToHunk[dStart]

		isDecl := r != nil && r.IsDeclaration(a.Node.Type)
		isBlock := r != nil && r.IsBlock(a.Node.Type)
		isMultiLine := sEnd > sStart || dEnd > dStart
		isStatement := r.IsStatement(a.Node.Type) || (r != nil && r.IsCall(a.Node.Type))
		if !isDecl && !isBlock && !isMultiLine && !isStatement {
			continue
		}
		// Same-hunk one-liners can see their destination on screen, so skip the badge.
		if !isDecl && !isBlock && !isMultiLine && inSrcHunk && inDstHunk && hSrc == hDst {
			continue
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
			hSrc:           hSrc,
			hDst:           hDst,
			inSrc:          inSrcHunk,
			inDst:          inDstHunk,
			isDecl:         isDecl,
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

	for cIdx, c := range candidates {
		suppressSrc := false
		suppressDst := false

		// Don't badge children if their enclosing parent move already has a badge and they stayed inside it.
		for _, p := range candidates[:cIdx] {
			enclosesSrc := p.sStartByte <= c.sStartByte && c.sEndByte <= p.sEndByte &&
				(p.sStartByte < c.sStartByte || c.sEndByte < p.sEndByte)
			enclosesDst := p.dStartByte <= c.dStartByte && c.dEndByte <= p.dEndByte &&
				(p.dStartByte < c.dStartByte || c.dEndByte < p.dEndByte)

			if enclosesSrc && enclosesDst {
				if p.hasSrcBadge {
					suppressSrc = true
				}
				if p.hasDstBadge {
					suppressDst = true
				}
				if suppressSrc && suppressDst {
					break
				}
			}
		}

		_, isDeleted := deletedStartBytes[c.act.Node.StartByte]
		_, isInserted := insertedStartBytes[c.dStartByteLine]

		if promoteDeclarations && c.isDecl {
			// Top-level declaration moves are summarized directly in the hunk header
			sig := ExtractDeclarationSignature(c.act.Node, srcLines, c.sStart, c.sEnd)
			if sig == "declaration" {
				sig = ExtractDeclarationSignature(c.act.Node, dstLines, c.dStart, c.dEnd)
			}
			if c.inSrc && sig != "" && !isDeleted && !suppressSrc {
				if _, exists := meta.HunkHeaders[c.hSrc]; !exists {
					meta.HunkHeaders[c.hSrc] = fmt.Sprintf(" %s (moved to L%d)", sig, c.dStart+1)
					c.hasSrcBadge = true
				}
			}
			if c.inDst && sig != "" && !isInserted && !suppressDst {
				if _, exists := meta.HunkHeaders[c.hDst]; !exists {
					meta.HunkHeaders[c.hDst] = fmt.Sprintf(" %s (moved from L%d)", sig, c.sStart+1)
					c.hasDstBadge = true
				}
			}
		} else {
			// Sub-block or nested moves show a directional badge on the opening line
			// (or top-level declarations when promoteDeclarations is false in SBS)
			if c.inSrc && c.sStart >= 0 && c.sStart < len(srcLines) && !isDeleted && !suppressSrc {
				if _, exists := meta.SrcLineBadges[c.sStart]; !exists {
					meta.SrcLineBadges[c.sStart] = fmt.Sprintf(" ➔ L%d", c.dStart+1)
					meta.SrcLineBadgeColors[c.sStart] = c.act.MoveColorIndex
					c.hasSrcBadge = true
				}
			}
			if c.inDst && c.dStart >= 0 && c.dStart < len(dstLines) && !isInserted && !suppressDst {
				if _, exists := meta.DstLineBadges[c.dStart]; !exists {
					meta.DstLineBadges[c.dStart] = fmt.Sprintf(" ⤹ L%d", c.sStart+1)
					meta.DstLineBadgeColors[c.dStart] = c.act.MoveColorIndex
					c.hasDstBadge = true
				}
			}
		}
	}

	return meta
}

// ExtractDeclarationSignature extracts the declaration signature line from lines,
// trimming leading decorators, comments, and trailing braces.
func ExtractDeclarationSignature(node *serialize.NodeRef, lines []string, sStartLine, sEndLine int) string {
	if sStartLine >= 0 && sStartLine < len(lines) {
		maxLine := sEndLine
		if maxLine < sStartLine || maxLine >= len(lines) {
			maxLine = sStartLine
		}
		for lineIdx := sStartLine; lineIdx <= maxLine; lineIdx++ {
			line := strings.TrimSpace(lines[lineIdx])
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "@") || strings.HasPrefix(line, "#[") || strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || strings.HasPrefix(line, "*") || strings.HasPrefix(line, "#") {
				continue
			}
			line = strings.TrimSuffix(line, " {")
			line = strings.TrimSuffix(line, "{")
			line = strings.TrimSuffix(line, ";")
			line = strings.TrimSpace(line)
			if line != "" {
				runes := []rune(line)
				if len(runes) > 80 {
					return string(runes[:77]) + "..."
				}
				return line
			}
		}
	}
	if node != nil && node.Label != "" {
		return node.Label
	}
	if node != nil && node.Type != "" {
		return node.Type
	}
	return "declaration"
}
