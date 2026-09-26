package renderutil

import (
	"fmt"
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

	for _, a := range actions {
		if a.Action != "move" || a.Node == nil {
			continue
		}
		sStart, _ := serialize.ByteToLineCol(srcOffsets, a.Node.StartByte)
		sEnd, _ := serialize.ByteToLineCol(srcOffsets, a.Node.EndByte)

		var dStart, dEnd int
		var dStartByte uint32
		if a.DestStartByte != nil && a.DestEndByte != nil {
			dStartByte = *a.DestStartByte
			dStart, _ = serialize.ByteToLineCol(dstOffsets, *a.DestStartByte)
			dEnd, _ = serialize.ByteToLineCol(dstOffsets, *a.DestEndByte)
		} else if a.DestNode != nil {
			dStartByte = a.DestNode.StartByte
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
		isStatement := a.Node.Type == "statement" || strings.HasSuffix(a.Node.Type, "_statement") || (r != nil && r.IsCall(a.Node.Type))
		if !isDecl && !isBlock && !isMultiLine && !isStatement {
			continue
		}
		// Same-hunk one-liners can see their destination on screen, so skip the badge.
		if !isDecl && !isBlock && !isMultiLine && inSrcHunk && inDstHunk && hSrc == hDst {
			continue
		}

		_, isDeleted := deletedStartBytes[a.Node.StartByte]
		_, isInserted := insertedStartBytes[dStartByte]

		if promoteDeclarations && isDecl {
			// Top-level declaration moves are summarized directly in the hunk header
			sig := ExtractDeclarationSignature(a.Node, srcLines, sStart, sEnd)
			if sig == "declaration" {
				sig = ExtractDeclarationSignature(a.Node, dstLines, dStart, dEnd)
			}
			if inSrcHunk && sig != "" && !isDeleted {
				if _, exists := meta.HunkHeaders[hSrc]; !exists {
					meta.HunkHeaders[hSrc] = fmt.Sprintf(" %s (moved to L%d)", sig, dStart+1)
				}
			}
			if inDstHunk && sig != "" && !isInserted {
				if _, exists := meta.HunkHeaders[hDst]; !exists {
					meta.HunkHeaders[hDst] = fmt.Sprintf(" %s (moved from L%d)", sig, sStart+1)
				}
			}
		} else {
			// Sub-block or nested moves show a directional badge on the opening line
			// (or top-level declarations when promoteDeclarations is false in SBS)
			if inSrcHunk && sStart >= 0 && sStart < len(srcLines) && !isDeleted {
				if _, exists := meta.SrcLineBadges[sStart]; !exists {
					meta.SrcLineBadges[sStart] = fmt.Sprintf(" ➔ L%d", dStart+1)
					meta.SrcLineBadgeColors[sStart] = a.MoveColorIndex
				}
			}
			if inDstHunk && dStart >= 0 && dStart < len(dstLines) && !isInserted {
				if _, exists := meta.DstLineBadges[dStart]; !exists {
					meta.DstLineBadges[dStart] = fmt.Sprintf(" ⤹ L%d", sStart+1)
					meta.DstLineBadgeColors[dStart] = a.MoveColorIndex
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
