package postprocess

import (
	"cmp"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// isStationaryMove reports whether a move stayed in place, accounting for transparent
// wrappers, expression shifts, and container provenance.
func isStationaryMove(node, dstNode *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	if node == nil || dstNode == nil || node.Parent == nil || dstNode.Parent == nil {
		return false
	}
	srcParent := node.Parent
	dstParent := dstNode.Parent

	// If direct parents match, the node moved within the same container (like swapped arguments).
	if ms.Src()[srcParent] == dstParent {
		return false
	}

	if isStationaryExpressionMove(node, dstNode, ms, r, nil) {
		return true
	}

	// Can't be stationary if it hopped containers, like when dstParent mapped to an
	// old container that never held this node.
	if mappedDstParent := ms.Dst()[dstParent]; mappedDstParent != nil && !mappedDstParent.Contains(node) {
		return false
	}
	if mappedSrcParent := ms.Src()[srcParent]; mappedSrcParent != nil && !mappedSrcParent.Contains(dstNode) {
		return false
	}

	hasPos := (node.EndByte > 0 || node.StartRow > 0 || node.EndRow > 0) &&
		(dstNode.EndByte > 0 || dstNode.StartRow > 0 || dstNode.EndRow > 0)
	if hasPos && node.StartRow == dstNode.StartRow && node.EndRow == dstNode.EndRow &&
		node.StartCol == dstNode.StartCol && node.EndCol == dstNode.EndCol {
		if sameScopeDeclaration(node, dstNode, ms, r) {
			return true
		}
	}

	sameLine := node.StartRow == dstNode.StartRow

	// Walk up past transparent wrappers to find the enclosing container.
	srcBase := srcParent
	srcChild := node
	dstBase := dstParent
	dstChild := dstNode

	canUnwrap := func(base, child *treesitter.ASTNode, isDst bool) bool {
		if base == nil || base.Parent == nil {
			return false
		}
		br := rules.Get(base.GetLanguage())
		isCall := (br != nil && br.IsCall(base.Type)) || (br == nil && rules.IsCall(base.Type))
		if isCall && child.ChildIndex() != 0 && !sameLine {
			return false
		}
		if base.Parent != nil {
			parentIsCall := (br != nil && br.IsCall(base.Parent.Type)) || (br == nil && rules.IsCall(base.Parent.Type))
			if parentIsCall && base.ChildIndex() != 0 && !sameLine {
				return false
			}
		}

		if base.IsWrapper() {
			return true
		}
		hasMapping := ms.Has(base)
		if isDst {
			hasMapping = ms.HasDst(base)
		}
		if !hasMapping && canUnwrapSubExpression(base) {
			if isDst {
				return base.ChildIndex() == 0 || ms.Dst()[base.Parent] == srcBase || ms.Dst()[base.Parent] == srcParent
			}
			return base.ChildIndex() == 0 || ms.Src()[base.Parent] == dstBase || ms.Src()[base.Parent] == dstParent
		}
		if canUnwrapSubExpression(base) {
			if isDst {
				return ms.Dst()[base] != srcBase && ms.Dst()[base.Parent] == srcBase
			}
			return ms.Src()[base] != dstBase && ms.Src()[base.Parent] == dstBase
		}
		return false
	}

	const maxUnwrapDepth = 3
	srcDepth := 0
	for canUnwrap(srcBase, srcChild, false) && srcBase.Parent != nil && srcDepth < maxUnwrapDepth {
		srcChild = srcBase
		srcBase = srcBase.Parent
		srcDepth++
	}

	dstDepth := 0
	for canUnwrap(dstBase, dstChild, true) && dstBase.Parent != nil && dstDepth < maxUnwrapDepth {
		dstChild = dstBase
		dstBase = dstBase.Parent
		dstDepth++
	}

	// Different unwrap depths mean code moved into or out of nesting instead of staying put.
	if srcDepth > 0 && dstDepth > 0 && max(srcDepth, dstDepth)-min(srcDepth, dstDepth) > 1 {
		return false
	}

	isStationaryPos := srcChild.ChildIndex() == dstChild.ChildIndex() ||
		(srcChild.ChildIndex() == len(srcBase.Children)-1 && dstChild.ChildIndex() == len(dstBase.Children)-1)
	if (srcBase != srcParent || dstBase != dstParent) && ms.Src()[srcBase] == dstBase {
		if isStationaryPos {
			return true
		}
		if (node.IsKeyword || (r != nil && r.IsKeyword(node.Type, node.Label))) && sameScopeDeclaration(srcBase, dstBase, ms, r) {
			return true
		}
	}
	return false
}

// normalizeStationaryWrapperMoves drops false-positive Move actions when a wrapper
// (like friend_declaration or template_declaration) is added or removed around code that stayed in place.
func normalizeStationaryWrapperMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := cmp.Or(a.DestNode, ms.Src()[a.Node])
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				if isStationaryMove(a.Node, dstNode, ms, r) {
					continue
				}
			}
		}
		result.Add(a)
	}
	return result
}

// isStationaryExpressionMove reports whether a Move between expressions in the
// same statement only regroups operators or parens without reordering the
// underlying operands.
func isStationaryExpressionMove(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) bool {
	if src == nil || dst == nil || ms == nil || r == nil {
		return false
	}
	if len(src.Children) == 0 || len(dst.Children) == 0 {
		return false
	}
	if !r.IsExpression(src.Type) || !r.IsExpression(dst.Type) {
		return false
	}

	srcStmt := engine.FindEnclosingStatement(src, r)
	dstStmt := engine.FindEnclosingStatement(dst, r)
	if !isStatementStationary(srcStmt, dstStmt, ms, r) {
		return false
	}
	// An expression isn't stationary if it shifted to a different line inside the statement.
	if int(src.StartRow)-int(srcStmt.StartRow) != int(dst.StartRow)-int(dstStmt.StartRow) {
		return false
	}

	// FindEnclosingStatement returns the node itself if it sits directly under a block,
	// so check the parents to make sure neither side is a nested block of the statement.
	if engine.FindEnclosingStatement(src.Parent, r) != srcStmt || engine.FindEnclosingStatement(dst.Parent, r) != dstStmt {
		return false
	}

	var mappedSrcLeaves []*treesitter.ASTNode
	hasSurvivingPayloadInDst := false
	for _, d := range src.Descendants() {
		if len(d.Children) > 0 {
			continue
		}
		if _, isEv := evicted[d]; isEv {
			continue
		}
		if engine.FindEnclosingStatement(d, r) != srcStmt {
			continue
		}
		if dDst, ok := ms.Src()[d]; ok {
			if !dstStmt.Contains(dDst) || engine.FindEnclosingStatement(dDst, r) != dstStmt {
				return false
			}
			if isPayloadLeaf(d, r) && dst.Contains(dDst) {
				hasSurvivingPayloadInDst = true
			}
			mappedSrcLeaves = append(mappedSrcLeaves, d)
		}
	}
	if !hasSurvivingPayloadInDst {
		return false
	}

	for _, dDst := range dst.Descendants() {
		if len(dDst.Children) > 0 {
			continue
		}
		if engine.FindEnclosingStatement(dDst, r) != dstStmt {
			continue
		}
		if dSrc, ok := ms.Dst()[dDst]; ok {
			if _, isEv := evicted[dSrc]; isEv {
				continue
			}
			if !srcStmt.Contains(dSrc) || engine.FindEnclosingStatement(dSrc, r) != srcStmt {
				return false
			}
		}
	}

	var allMappedStmtLeaves []*treesitter.ASTNode
	var collectStmtLeaves func(n *treesitter.ASTNode)
	collectStmtLeaves = func(n *treesitter.ASTNode) {
		for _, c := range n.Children {
			if len(c.Children) == 0 {
				if _, isEv := evicted[c]; isEv {
					continue
				}
				if engine.FindEnclosingStatement(c, r) != srcStmt {
					continue
				}
				if cDst, ok := ms.Src()[c]; ok && dstStmt.Contains(cDst) && engine.FindEnclosingStatement(cDst, r) == dstStmt {
					allMappedStmtLeaves = append(allMappedStmtLeaves, c)
				}
				continue
			}
			if r.IsBlock(c.Type) || r.IsCaseClause(c.Type) || engine.FindEnclosingStatement(c, r) != srcStmt {
				continue
			}
			collectStmtLeaves(c)
		}
	}
	collectStmtLeaves(srcStmt)

	// Bail if any operand crossed over another token in the statement.
	for _, d := range mappedSrcLeaves {
		dDst := ms.Src()[d]
		for _, o := range allMappedStmtLeaves {
			if o == d {
				continue
			}
			oDst := ms.Src()[o]
			if d.StartByte < o.StartByte && dDst.StartByte > oDst.StartByte {
				return false
			}
			if d.StartByte > o.StartByte && dDst.StartByte < oDst.StartByte {
				return false
			}
		}
	}

	return true
}

// isStatementStationary reports whether a matched statement stayed in place
// structurally within its enclosing scope (ignoring line drift from edits above it).
func isStatementStationary(srcStmt, dstStmt *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	if srcStmt == nil || dstStmt == nil || ms == nil || ms.Src()[srcStmt] != dstStmt {
		return false
	}
	if !sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
		return false
	}
	srcDecl := srcStmt.EnclosingContainerDeclaration(r)
	curSrc, curDst := srcStmt, dstStmt
	for curSrc != nil && curDst != nil && curSrc != srcDecl {
		srcParent := curSrc.Parent
		dstParent := curDst.Parent
		if srcParent == nil && dstParent == nil {
			break
		}
		if srcParent == nil || dstParent == nil || ms.Src()[srcParent] != dstParent {
			return false
		}
		for _, sib := range srcParent.Children {
			if sib == curSrc {
				continue
			}
			if sibDst := ms.Get(sib); sibDst != nil && sibDst.Parent == dstParent {
				if (curSrc.StartByte < sib.StartByte) != (curDst.StartByte < sibDst.StartByte) {
					return false
				}
			}
		}
		curSrc = srcParent
		curDst = dstParent
	}
	return true
}

// areChainedStatements reports whether base and parent belong to the same
// statement chain (like an if-else-if ladder).
func areChainedStatements(base, parent *treesitter.ASTNode, r *rules.Rules) bool {
	if base == nil || parent == nil {
		return false
	}
	isStmt := (r != nil && r.IsStatement(base.Type)) || (r == nil && rules.IsStatement(base.Type))
	if !isStmt {
		return false
	}
	if r != nil {
		return r.AreTypesEquivalent(base.Type, parent.Type)
	}
	return base.Type == parent.Type
}

// canUnwrapSubExpression reports whether n is a lightweight wrapper (like parens
// or an if-else chain) that code can stay stationary across.
func canUnwrapSubExpression(n *treesitter.ASTNode) bool {
	if n == nil || n.Parent == nil {
		return false
	}
	r := rules.Get(n.GetLanguage())
	if r != nil {
		if r.IsPair(n.Type) || r.IsUnordered(n.Type) || r.IsBlock(n.Type) {
			return false
		}
		if r.IsDeclaration(n.Type) && !r.IsWrapper(n.Type) {
			return false
		}
		if r.IsStatement(n.Type) && !r.IsWrapper(n.Type) && !r.IsExpressionStatement(n.Type) {
			if !areChainedStatements(n, n.Parent, r) {
				return false
			}
		}
	} else {
		if rules.IsBlock(n.Type) {
			return false
		}
		if rules.IsStatement(n.Type) && !rules.IsExpressionStatement(n.Type) {
			if !areChainedStatements(n, n.Parent, nil) {
				return false
			}
		}
	}
	return true
}

// normalizeWrapperDelimiterChanges emits non-subtree Insert or Delete actions when a mapped
// wrapper container (such as argument_list) gains or loses outer delimiter punctuation
// (such as optional parentheses in Ruby) around surviving mapped children.
func normalizeWrapperDelimiterChanges(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	hasAction := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions() {
		if a.Node != nil {
			hasAction[a.Node] = true
		}
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		result.Add(a)
	}

	for _, p := range ms.Pairs {
		srcNode, dstNode := p.Src, p.Dst
		if srcNode == nil || dstNode == nil {
			continue
		}
		if hasAction[srcNode] || hasAction[dstNode] {
			continue
		}
		r := rules.Get(srcNode.GetLanguage())
		if r == nil || !r.IsWrapper(srcNode.Type) || !r.IsWrapper(dstNode.Type) {
			continue
		}
		if len(srcNode.Children) == 0 || len(dstNode.Children) == 0 {
			continue
		}

		srcHasDelims := srcNode.StartByte < srcNode.Children[0].StartByte || srcNode.EndByte > srcNode.Children[len(srcNode.Children)-1].EndByte
		dstHasDelims := dstNode.StartByte < dstNode.Children[0].StartByte || dstNode.EndByte > dstNode.Children[len(dstNode.Children)-1].EndByte

		if !srcHasDelims && dstHasDelims {
			result.Add(actions.Action{
				Type:     actions.Insert,
				Node:     dstNode,
				Parent:   dstNode.Parent,
				Position: dstNode.ChildIndex(),
				Subtree:  false,
			})
		} else if srcHasDelims && !dstHasDelims {
			result.Add(actions.Action{
				Type:    actions.Delete,
				Node:    srcNode,
				Parent:  srcNode.Parent,
				Subtree: false,
			})
		}
	}

	return result
}

// sameScopeDeclaration reports whether src and dst reside within corresponding
// (mapped) enclosing container declarations (functions, methods, classes, structs).
func sameScopeDeclaration(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	if src == nil || dst == nil || ms == nil {
		return false
	}
	if r == nil {
		r = rules.Get(src.GetLanguage())
	}
	if r == nil {
		return false
	}
	srcDecl := src.EnclosingContainerDeclaration(r)
	dstDecl := dst.EnclosingContainerDeclaration(r)
	if srcDecl == nil && dstDecl == nil {
		return true
	}
	if srcDecl == nil || dstDecl == nil {
		return false
	}
	return ms.Src()[srcDecl] == dstDecl
}
