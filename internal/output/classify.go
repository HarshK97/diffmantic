package output
import (
	"fmt"
	"sort"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)
func isDeclarationLike(nodeType string) bool {
	switch nodeType {
	case "function_declaration", "method_declaration", "type_declaration", "const_declaration", "var_declaration",
		"class_specifier", "struct_specifier", "union_specifier", "enum_specifier", "function_definition",
		"declaration", "namespace_definition", "template_declaration", "linkage_specification",
		"preproc_include", "preproc_def", "preproc_function_def", "preproc_call":
		return true
	default:
		return false
	}
}
func isStatementOrDeclaration(nodeType string) bool {
	if isDeclarationLike(nodeType) {
		return true
	}
	switch nodeType {
	case "short_var_declaration", "assignment_statement", "expression_statement",
		"if_statement", "for_statement", "for_range_loop", "while_statement", "do_statement",
		"return_statement", "break_statement", "continue_statement",
		"import_declaration", "package_clause", "comment":
		return true
	}
	return false
}
func isAnyParentMovedOrUpdated(n *treesitter.ASTNode, c2o map[*treesitter.ASTNode]*treesitter.ASTNode, movedOrUpdated map[*treesitter.ASTNode]bool) bool {
	curr := n.Parent
	for curr != nil {
		if movedOrUpdated[curr] {
			return true
		}
		if orig, ok := c2o[curr]; ok && movedOrUpdated[orig] {
			return true
		}
		curr = curr.Parent
	}
	return false
}
func shouldIncludeHunk(a engine.Action, c2o map[*treesitter.ASTNode]*treesitter.ASTNode, movedOrUpdated map[*treesitter.ASTNode]bool) bool {
	node := a.Node
	if a.Kind == engine.ActionInsert || a.Kind == engine.ActionInsertTree {
		node = a.T2Ref
	}
	if node == nil {
		return false
	}
	if !isStatementOrDeclaration(node.Type) {
		if a.Kind != engine.ActionUpdate {
			return false
		}
	}
	// Noise suppression for moves:
	if a.Kind == engine.ActionMove && node.Label == "" {
		if isAnyParentMovedOrUpdated(node, c2o, movedOrUpdated) {
			return false
		}
	}
	return true
}
// Classify converts a raw EditScript into a flat slice of Hunks.
// Each engine.Action becomes one Hunk with source-level line ranges.
func Classify(es *engine.EditScript) []Hunk {
	if es == nil {
		return nil
	}
	// Build map of moved or updated nodes for noise suppression
	movedOrUpdated := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions {
		if a.Kind == engine.ActionMove || a.Kind == engine.ActionUpdate {
			if a.Node != nil {
				movedOrUpdated[a.Node] = true
			}
			if a.T2Ref != nil {
				movedOrUpdated[a.T2Ref] = true
			}
		}
	}
	var hunks []Hunk
	for _, a := range es.Actions {
		if !shouldIncludeHunk(a, es.CopyToOrig, movedOrUpdated) {
			continue
		}
		var h Hunk
		switch a.Kind {
		case engine.ActionInsert:
			h = classifyInsert(a)
		case engine.ActionInsertTree:
			h = classifyInsertTree(a)
		case engine.ActionDelete:
			h = classifyDelete(a, es.CopyToOrig)
		case engine.ActionDeleteTree:
			h = classifyDeleteTree(a, es.CopyToOrig)
		case engine.ActionUpdate:
			h = classifyUpdate(a, es.CopyToOrig)
		case engine.ActionMove:
			h = classifyMove(a, es.CopyToOrig)
			// Filter structural-only MOVs: same line range → no visual change.
			if h.SrcStartLine == h.DstStartLine && h.SrcEndLine == h.DstEndLine {
				continue
			}
		}
		hunks = append(hunks, h)
	}
	return hunks
}
// classifyInsert maps an INS action to a Hunk.
func classifyInsert(a engine.Action) Hunk {
	dst := a.T2Ref
	return Hunk{
		Kind:         ChangeInsert,
		DstStartLine: rowToLine(dst.StartRow),
		DstEndLine:   rowToLine(dst.EndRow),
		Summary:      fmt.Sprintf("inserted %s %s", dst.Type, labelStr(dst)),
		NodeType:     dst.Type,
	}
}
// classifyInsertTree maps an INS_TREE action to a Hunk.
// Uses the T2Ref which spans the full subtree.
func classifyInsertTree(a engine.Action) Hunk {
	dst := a.T2Ref
	return Hunk{
		Kind:         ChangeInsert,
		DstStartLine: rowToLine(dst.StartRow),
		DstEndLine:   rowToLine(dst.EndRow),
		Summary:      fmt.Sprintf("inserted %s %s", dst.Type, labelStr(dst)),
		NodeType:     dst.Type,
	}
}
// classifyDelete maps a DEL action to a Hunk.
func classifyDelete(a engine.Action, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) Hunk {
	src := resolveOriginal(a.Node, c2o)
	return Hunk{
		Kind:         ChangeDelete,
		SrcStartLine: rowToLine(src.StartRow),
		SrcEndLine:   rowToLine(src.EndRow),
		Summary:      fmt.Sprintf("deleted %s %s", src.Type, labelStr(src)),
		NodeType:     src.Type,
	}
}
// classifyDeleteTree maps a DEL_TREE action to a Hunk.
func classifyDeleteTree(a engine.Action, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) Hunk {
	src := resolveOriginal(a.Node, c2o)
	return Hunk{
		Kind:         ChangeDelete,
		SrcStartLine: rowToLine(src.StartRow),
		SrcEndLine:   rowToLine(src.EndRow),
		Summary:      fmt.Sprintf("deleted %s %s", src.Type, labelStr(src)),
		NodeType:     src.Type,
	}
}
// classifyUpdate maps a UPD action to a Hunk.
func classifyUpdate(a engine.Action, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) Hunk {
	src := resolveOriginal(a.Node, c2o)
	dst := a.T2Ref
	return Hunk{
		Kind:         ChangeUpdate,
		SrcStartLine: rowToLine(src.StartRow),
		SrcEndLine:   rowToLine(src.EndRow),
		DstStartLine: rowToLine(dst.StartRow),
		DstEndLine:   rowToLine(dst.EndRow),
		Summary:      fmt.Sprintf("%s changed: %q → %q", src.Type, src.Label, dst.Label),
		NodeType:     src.Type,
	}
}
// classifyMove maps a MOV action to a Hunk.
func classifyMove(a engine.Action, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) Hunk {
	src := resolveOriginal(a.Node, c2o)
	dst := a.T2Ref
	return Hunk{
		Kind:         ChangeMove,
		SrcStartLine: rowToLine(src.StartRow),
		SrcEndLine:   rowToLine(src.EndRow),
		DstStartLine: rowToLine(dst.StartRow),
		DstEndLine:   rowToLine(dst.EndRow),
		Summary:      fmt.Sprintf("moved %s %s", src.Type, labelStr(src)),
		NodeType:     src.Type,
	}
}
// Coalesce merges adjacent/overlapping hunks of the same kind that
// share a common parent in the AST (AST-level primary). If two hunks
// are different kinds but overlap on lines, they are merged as a
// line-level fallback.
func Coalesce(hunks []Hunk) []Hunk {
	if len(hunks) <= 1 {
		return hunks
	}
	sort.SliceStable(hunks, func(i, j int) bool {
		if hunks[i].Kind != hunks[j].Kind {
			return hunks[i].Kind < hunks[j].Kind
		}
		if hunks[i].SrcStartLine != hunks[j].SrcStartLine {
			return hunks[i].SrcStartLine < hunks[j].SrcStartLine
		}
		return hunks[i].DstStartLine < hunks[j].DstStartLine
	})
	// Pass 1: AST-level - merge same-Kind hunks with overlapping/adjacent ranges.
	merged := []Hunk{hunks[0]}
	for _, h := range hunks[1:] {
		last := &merged[len(merged)-1]
		if last.Kind == h.Kind && rangesOverlap(last, &h) {
			mergeInto(last, &h)
		} else {
			merged = append(merged, h)
		}
	}
	// Pass 2: Line-level fallback - merge same-kind hunks that overlap
	// when sorted by position (catches hunks that weren't adjacent in
	// the kind-sorted pass 1).
	sort.SliceStable(merged, func(i, j int) bool {
		si := effectiveStart(merged[i])
		sj := effectiveStart(merged[j])
		if si != sj {
			return si < sj
		}
		return merged[i].Kind < merged[j].Kind
	})
	final := []Hunk{merged[0]}
	for _, h := range merged[1:] {
		last := &final[len(final)-1]
		if last.Kind == h.Kind && rangesOverlap(last, &h) {
			mergeInto(last, &h)
		} else {
			final = append(final, h)
		}
	}
	return final
}
// resolveOriginal follows the copy→original map. If the node is not
// in the map (e.g. it was freshly inserted), return it as-is.
func resolveOriginal(n *treesitter.ASTNode, c2o map[*treesitter.ASTNode]*treesitter.ASTNode) *treesitter.ASTNode {
	if orig, ok := c2o[n]; ok {
		return orig
	}
	return n
}
// rowToLine converts a 0-indexed tree-sitter row to a 1-indexed line.
func rowToLine(row uint32) int { return int(row) + 1 }
func labelStr(n *treesitter.ASTNode) string {
	if n.Label != "" {
		return fmt.Sprintf("%q", n.Label)
	}
	return ""
}
// rangesOverlap returns true if two hunks have overlapping or
// adjacent line ranges (on either the src or dst side).
func rangesOverlap(a, b *Hunk) bool {
	if a.Kind == ChangeUpdate || a.Kind == ChangeMove {
		return linesOverlap(a.SrcStartLine, a.SrcEndLine, b.SrcStartLine, b.SrcEndLine) &&
			linesOverlap(a.DstStartLine, a.DstEndLine, b.DstStartLine, b.DstEndLine)
	}
	return linesOverlap(a.SrcStartLine, a.SrcEndLine, b.SrcStartLine, b.SrcEndLine) ||
		linesOverlap(a.DstStartLine, a.DstEndLine, b.DstStartLine, b.DstEndLine)
}
func linesOverlap(aStart, aEnd, bStart, bEnd int) bool {
	if aStart == 0 || bStart == 0 {
		return false
	}
	return aStart <= bEnd+1 && bStart <= aEnd+1
}
func effectiveStart(h Hunk) int {
	if h.SrcStartLine > 0 {
		return h.SrcStartLine
	}
	return h.DstStartLine
}
func mergeInto(dst, src *Hunk) {
	srcSpan := func(h *Hunk) int {
		if h.SrcEndLine > 0 && h.SrcStartLine > 0 {
			return h.SrcEndLine - h.SrcStartLine
		}
		if h.DstEndLine > 0 && h.DstStartLine > 0 {
			return h.DstEndLine - h.DstStartLine
		}
		return 0
	}
	spanSrc := srcSpan(src)
	spanDst := srcSpan(dst)
	if src.SrcStartLine > 0 {
		if dst.SrcStartLine == 0 || src.SrcStartLine < dst.SrcStartLine {
			dst.SrcStartLine = src.SrcStartLine
		}
		if src.SrcEndLine > dst.SrcEndLine {
			dst.SrcEndLine = src.SrcEndLine
		}
	}
	if src.DstStartLine > 0 {
		if dst.DstStartLine == 0 || src.DstStartLine < dst.DstStartLine {
			dst.DstStartLine = src.DstStartLine
		}
		if src.DstEndLine > dst.DstEndLine {
			dst.DstEndLine = src.DstEndLine
		}
	}
	if spanSrc > spanDst {
		dst.Summary = src.Summary
		dst.NodeType = src.NodeType
	}
}
