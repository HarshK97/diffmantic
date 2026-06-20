package tui
import (
	"sort"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)
func normalizeVisualHunks(hunks []Hunk, mappings *engine.Mapping) []Hunk {
	visual := make([]Hunk, 0, len(hunks)+1)
	for _, h := range hunks {
		if h.Kind == ChangeMove && h.SrcStartLine == h.DstStartLine {
			continue
		}
		visual = append(visual, h)
	}
	for _, pair := range normalizedVisualMovePairs(mappings) {
		if hasMoveForPair(visual, pair) {
			continue
		}
		visual = append(visual, Hunk{
			Kind:         ChangeMove,
			SrcStartLine: int(pair.Src.StartRow) + 1,
			SrcEndLine:   int(pair.Src.EndRow) + 1,
			DstStartLine: int(pair.Dst.StartRow) + 1,
			DstEndLine:   int(pair.Dst.EndRow) + 1,
			Summary:      "moved " + pair.Src.Type,
			NodeType:     pair.Src.Type,
		})
	}
	sort.SliceStable(visual, func(i, j int) bool {
		a, b := effectiveVisualStart(visual[i]), effectiveVisualStart(visual[j])
		if a != b {
			return a < b
		}
		return visual[i].Kind < visual[j].Kind
	})
	return visual
}
func effectiveVisualStart(h Hunk) int {
	if h.SrcStartLine > 0 {
		return h.SrcStartLine
	}
	return h.DstStartLine
}
func hasMoveForPair(hunks []Hunk, pair engine.MappingPair) bool {
	srcStart, srcEnd := int(pair.Src.StartRow)+1, int(pair.Src.EndRow)+1
	dstStart, dstEnd := int(pair.Dst.StartRow)+1, int(pair.Dst.EndRow)+1
	for _, h := range hunks {
		if h.Kind != ChangeMove {
			continue
		}
		if h.SrcStartLine == srcStart && h.SrcEndLine == srcEnd &&
			h.DstStartLine == dstStart && h.DstEndLine == dstEnd {
			return true
		}
	}
	return false
}
func normalizedVisualMovePairs(mappings *engine.Mapping) []engine.MappingPair {
	if mappings == nil {
		return nil
	}
	bestByParent := make(map[parentPair]engine.MappingPair)
	bestDelta := make(map[parentPair]int)
	for _, pair := range mappings.Pairs {
		if !isVisualMoveCandidate(pair, mappings) {
			continue
		}
		key := parentPair{src: pair.Src.Parent, dst: pair.Dst.Parent}
		delta := absInt(int(pair.Dst.StartRow) - int(pair.Src.StartRow))
		if current, ok := bestDelta[key]; !ok || delta > current {
			bestByParent[key] = pair
			bestDelta[key] = delta
		}
	}
	out := make([]engine.MappingPair, 0, len(bestByParent))
	for _, pair := range bestByParent {
		out = append(out, pair)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Src.StartRow < out[j].Src.StartRow
	})
	return out
}
type parentPair struct {
	src *treesitter.ASTNode
	dst *treesitter.ASTNode
}
func isVisualMoveCandidate(pair engine.MappingPair, mappings *engine.Mapping) bool {
	src, dst := pair.Src, pair.Dst
	if src == nil || dst == nil || src.Parent == nil || dst.Parent == nil {
		return false
	}
	if src.Type != dst.Type || !isDeclarationLike(src.Type) {
		return false
	}
	if src.StartRow == dst.StartRow {
		return false
	}
	return siblingOrderChanged(pair, mappings)
}
func isDeclarationLike(nodeType string) bool {
	switch nodeType {
	case "function_declaration", "method_declaration", "type_declaration", "const_declaration", "var_declaration":
		return true
	default:
		return false
	}
}
func siblingOrderChanged(pair engine.MappingPair, mappings *engine.Mapping) bool {
	srcIndex := childIndex(pair.Src)
	dstIndex := childIndex(pair.Dst)
	for _, other := range mappings.Pairs {
		if other.Src == pair.Src || other.Dst == pair.Dst {
			continue
		}
		if other.Src == nil || other.Dst == nil || other.Src.Parent != pair.Src.Parent || other.Dst.Parent != pair.Dst.Parent {
			continue
		}
		otherSrcIndex := childIndex(other.Src)
		otherDstIndex := childIndex(other.Dst)
		if (srcIndex < otherSrcIndex && dstIndex > otherDstIndex) ||
			(srcIndex > otherSrcIndex && dstIndex < otherDstIndex) {
			return true
		}
	}
	return false
}
func childIndex(n *treesitter.ASTNode) int {
	if n == nil || n.Parent == nil {
		return 0
	}
	for i, child := range n.Parent.Children {
		if child == n {
			return i
		}
	}
	return 0
}
func absInt(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
