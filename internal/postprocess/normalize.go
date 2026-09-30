package postprocess

import (
	"cmp"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

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
			if dstNode != nil && a.Node.Parent != nil && dstNode.Parent != nil {
				hasPos := (a.Node.EndByte > 0 || a.Node.StartRow > 0 || a.Node.EndRow > 0) &&
					(dstNode.EndByte > 0 || dstNode.StartRow > 0 || dstNode.EndRow > 0)
				if hasPos && a.Node.StartRow == dstNode.StartRow && a.Node.EndRow == dstNode.EndRow && a.Node.StartCol == dstNode.StartCol && a.Node.EndCol == dstNode.EndCol {
					continue
				}
				srcParent := a.Node.Parent
				dstParent := dstNode.Parent

				// If direct parents match, the node moved within the same container (like swapped arguments).
				if ms.Src()[srcParent] == dstParent {
					result.Add(a)
					continue
				}

				r := rules.Get(a.Node.GetLanguage())
				if isStationaryExpressionMove(a.Node, dstNode, ms, r, nil) {
					continue
				}

				sameLine := a.Node.StartRow == dstNode.StartRow

				// Walk up past transparent wrappers to find the enclosing container.
				srcBase := srcParent
				srcChild := a.Node
				dstBase := dstParent
				dstChild := dstNode

				canUnwrap := func(base, child *treesitter.ASTNode, isDst bool) bool {
					if base == nil || base.Parent == nil {
						return false
					}
					r := rules.Get(base.GetLanguage())
					isCall := (r != nil && r.IsCall(base.Type)) || (r == nil && rules.IsCall(base.Type))
					if isCall && child.ChildIndex() != 0 && !sameLine {
						return false
					}
					if base.Parent != nil {
						parentIsCall := (r != nil && r.IsCall(base.Parent.Type)) || (r == nil && rules.IsCall(base.Parent.Type))
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
					return false
				}

				for canUnwrap(srcBase, srcChild, false) && srcBase.Parent != nil {
					srcChild = srcBase
					srcBase = srcBase.Parent
				}

				for canUnwrap(dstBase, dstChild, true) && dstBase.Parent != nil {
					dstChild = dstBase
					dstBase = dstBase.Parent
				}

				if (srcBase != srcParent || dstBase != dstParent) && ms.Src()[srcBase] == dstBase && srcChild.ChildIndex() == dstChild.ChildIndex() {
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
	if srcStmt == nil || dstStmt == nil || ms.Src()[srcStmt] != dstStmt {
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

// canUnwrapSubExpression checks if n is a lightweight wrapper or sub-expression
// that code can stay stationary across, rather than a distinct semantic container.
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
	}
	return true
}

// findBodyBlock returns the primary consequence/body block of a control-flow statement.
func findBodyBlock(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		isBlock := (r != nil && r.IsBlock(c.Type)) || (r == nil && rules.IsBlock(c.Type))
		if isBlock {
			return c
		}
	}
	return nil
}

// findCall returns the first call expression found within a statement subtree, or nil.
func findCall(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	if r != nil && r.IsCall(n.Type) {
		return n
	}
	for _, c := range n.Children {
		if found := findCall(c, r); found != nil {
			return found
		}
	}
	return nil
}

// isTerminatingStatement reports whether stmt is a control-flow jump (return, break, etc.)
// or an execution-terminating call (os.Exit, panic, sys.exit, etc.).
func isTerminatingStatement(stmt *treesitter.ASTNode, r *rules.Rules) bool {
	if stmt == nil || r == nil {
		return false
	}
	if r.IsJumpStatement(stmt.Type) {
		return true
	}
	call := findCall(stmt, r)
	if call == nil {
		return false
	}
	path := treesitter.CalleePath(call)
	return r.IsTerminalCallPath(path)
}

// isFillerCallStatement reports whether stmt is a standalone call without its
// own control flow. These can sit next to an exit call (like a log before os.Exit)
// without turning the block into real business logic.
func isFillerCallStatement(stmt *treesitter.ASTNode, r *rules.Rules) bool {
	if stmt == nil || r == nil {
		return false
	}
	if r.IsCall(stmt.Type) {
		return true
	}
	if len(stmt.Children) != 1 {
		return false
	}
	return r.IsCall(stmt.Children[0].Type)
}

// isTrivialJumpBody reports whether a block is just early-exit boilerplate
// (returns, breaks, or os.Exit/panic, plus any preceding print or log calls).
func isTrivialJumpBody(body *treesitter.ASTNode, r *rules.Rules) bool {
	if body == nil {
		return true
	}
	stmtCount := 0
	nonFillerCount := 0
	hasTerminator := false
	check := func(s *treesitter.ASTNode) {
		stmtCount++
		switch {
		case isTerminatingStatement(s, r):
			hasTerminator = true
		case !isFillerCallStatement(s, r):
			nonFillerCount++
		}
	}
	for _, c := range body.Children {
		isPunct := (r != nil && r.IsPunctuation(c.Type)) || (r == nil && rules.IsPunctuation(c.Type))
		if !isPunct {
			check(c)
		}
	}
	return stmtCount > 0 && nonFillerCount == 0 && hasTerminator
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
	if src == nil || dst == nil || ms == nil || r == nil {
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

// subtreeHeight returns the maximum depth of a subtree.
func subtreeHeight(n *treesitter.ASTNode) int {
	if n == nil || len(n.Children) == 0 {
		return 0
	}
	maxChild := 0
	for _, c := range n.Children {
		maxChild = max(maxChild, subtreeHeight(c))
	}
	return maxChild + 1
}

// subtreeLines returns the line span (EndRow - StartRow) of a node.
func subtreeLines(n *treesitter.ASTNode) uint32 {
	if n == nil {
		return 0
	}
	if n.EndRow >= n.StartRow {
		return n.EndRow - n.StartRow
	}
	return 0
}

// isTokenNode reports whether n is a bare token (operator, punctuation, type, or identifier).
func isTokenNode(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || r == nil {
		return false
	}
	return r.IsOperatorLiteral(n.Type) || r.IsPunctuation(n.Type) ||
		r.IsType(n.Type) || r.IsIdentifier(n.Type)
}

// isPayloadLeaf reports whether d is a non-punctuation, non-operator, non-keyword leaf.
func isPayloadLeaf(d *treesitter.ASTNode, r *rules.Rules) bool {
	if len(d.Children) > 0 || d.IsKeyword {
		return false
	}
	if (r != nil && (r.IsPunctuation(d.Type) || r.IsOperatorLiteral(d.Type))) ||
		(r == nil && (rules.IsPunctuation(d.Type) || rules.IsOperatorLiteral(d.Type))) {
		return false
	}
	if (r != nil && r.IsKeyword(d.Type, d.Label)) || (r == nil && rules.IsKeyword(d.Type, d.Label)) {
		return false
	}
	return true
}

// moveStructuralScore scores how structurally significant a node is.
//
//	S = BaseSize + 2*Height + 3*LineSpan + RoleBonus - BoilerplatePenalty
//
// Bare tokens are clamped to S=1. Declarations receive +40. Boilerplate bodies receive -20.
func moveStructuralScore(node *treesitter.ASTNode, r *rules.Rules, ms *engine.Mapping) int {
	if node == nil || r == nil {
		return 0
	}

	// Token clamp: bare tokens, operators, punctuation, types, identifiers.
	if isTokenNode(node, r) {
		return 1
	}

	size := node.Size()
	height := subtreeHeight(node)
	lines := subtreeLines(node)

	// If most of a container was deleted, score it by its surviving nodes so a
	// gutted block doesn't look like a real move.
	if ms != nil && (r.IsBlock(node.Type) || r.IsWrapper(node.Type) || r.IsDelimitedContainer(node.Type)) {
		surviving := 0
		totalLeaves := 0
		for _, d := range node.Descendants() {
			if !isPayloadLeaf(d, r) {
				continue
			}
			totalLeaves++
			if ms.Has(d) || ms.HasDst(d) {
				surviving++
			}
		}
		if totalLeaves > 0 && surviving < totalLeaves {
			ratio := float64(surviving) / float64(totalLeaves)
			size = min(size, surviving*3)
			height = int(float64(height) * ratio)
			lines = uint32(float64(lines) * ratio)
		}
	}

	score := size + 2*height + 3*int(lines)

	// Role bonus.
	if r.IsDeclaration(node.Type) {
		score += 40
	} else if r.IsBlock(node.Type) {
		score += 10
	}

	// Boilerplate penalty: bodies consisting entirely of terminating statements.
	if body := findBodyBlock(node, r); body != nil && isTrivialJumpBody(body, r) {
		score -= 20
	}

	score = max(score, 1)
	return score
}

// requiredMoveThreshold computes the dynamic threshold T for a Move action
// based on construct mobility, scope preservation, and line distance.
func requiredMoveThreshold(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) int {
	if src == nil || dst == nil || ms == nil || r == nil {
		return 50
	}

	// Intra-container reorder: src.Parent mapped to dst.Parent within the same enclosing declaration.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent && sameScopeDeclaration(src, dst, ms, r) {
		return 1
	}

	// Cross-key guard: a value sitting under one key in a key-value pair
	// (keyed_element, pair) and a value under a *different*, unmapped key
	// should never be treated as a trivial same-line shift, even if they
	// happen to land on the same row. Otherwise a clamped bare token (S=1)
	// can "move" from one struct field to an unrelated one.
	crossKey := src.Parent != nil && dst.Parent != nil && r.IsPair(src.Parent.Type) && r.IsPair(dst.Parent.Type)

	lineDist := int(src.StartRow) - int(dst.StartRow)
	if lineDist < 0 {
		lineDist = -lineDist
	}

	// Same-line shifts.
	if lineDist == 0 {
		if crossKey {
			return 5
		}
		return 1
	}

	// Only apply T=20 when both sides are declarations. If only src is a
	// declaration, dst could be any arbitrary node and steal stubs or forward
	// declarations across distant files.
	if r.IsDeclaration(src.Type) && r.IsDeclaration(dst.Type) {
		return 20
	}

	srcStmt := engine.FindEnclosingStatement(src, r)
	dstStmt := engine.FindEnclosingStatement(dst, r)

	// Moves within the same function or declaration.
	if sameScopeDeclaration(src, dst, ms, r) {
		// Moving an expression across statements (like inlining a variable) needs a higher
		// threshold (T=20) so small helpers and field accesses don't show up as moves.
		if (src != srcStmt || dst != dstStmt) && ms.Get(srcStmt) != dstStmt {
			return 20
		}
		// Statements or reorders inside the same statement use mild distance scaling.
		// The floor of 4 lets small expressions like condition calls (S=4) still match.
		return 4 + lineDist/5
	}

	// Moves within 10 lines get mild scaling, but only if the source function
	// wasn't deleted.
	if lineDist < 10 {
		srcDecl := src.EnclosingContainerDeclaration(r)
		if srcDecl != nil && ms.Src()[srcDecl] != nil {
			return 4 + lineDist/5
		}
	}

	// Cross-scope statements: severe distance penalty.
	return 50 + lineDist/10
}

// normalizeMovesByStructure demotes Move actions whose structural score is below
// their distance-based threshold into separate Delete + Insert actions, and drops
// the demoted nodes from the mapping to stop spurious updates downstream.
func normalizeMovesByStructure(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	// Pass 1: Find moves to demote and collect nodes to evict from the mapping.
	// We have to do this first because Chawathe emits Update actions before Move,
	// and we need to drop those orphaned updates in Pass 2.
	toDemote := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	demotedDescendants := make(map[*treesitter.ASTNode]struct{})
	evicted := make(map[*treesitter.ASTNode]struct{})

	explicitMoves := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			explicitMoves[a.Node] = true
		}
	}

	for {
		changed := false
		for _, a := range es.Actions() {
			if a.Type != actions.Move || a.Node == nil {
				continue
			}
			if _, ok := demotedDescendants[a.Node]; ok {
				continue
			}
			if _, already := toDemote[a.Node]; already {
				continue
			}
			dstNode := cmp.Or(a.DestNode, ms.Src()[a.Node])
			if dstNode == nil {
				continue
			}
			r := rules.Get(a.Node.GetLanguage())
			if r == nil {
				continue
			}
			if !shouldDemoteMove(a.Node, dstNode, ms, r, evicted) {
				continue
			}
			toDemote[a.Node] = dstNode
			evicted[a.Node] = struct{}{}
			changed = true

			// Children without their own Move only moved as part of a.Node. Evict them
			// first so isSubtreeDemotion doesn't count them as surviving inside dstNode.
			for _, d := range a.Node.Descendants() {
				if dDst, ok := ms.Src()[d]; ok && dstNode.Contains(dDst) {
					if !explicitMoves[d] {
						demotedDescendants[d] = struct{}{}
						evicted[d] = struct{}{}
					}
				}
			}

			deleteSubtree, insertSubtree := isSubtreeDemotion(a.Node, dstNode, ms, r, evicted)
			if deleteSubtree && insertSubtree {
				for _, d := range a.Node.Descendants() {
					if dDst, ok := ms.Src()[d]; ok && dstNode.Contains(dDst) {
						demotedDescendants[d] = struct{}{}
						evicted[d] = struct{}{}
					}
				}
			}
		}
		if !changed {
			break
		}
	}

	// Pass 2: Rebuild the edit script: demote flagged moves to delete+insert,
	// and drop any orphaned updates or nested moves inside those subtrees.
	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Node == nil {
			result.Add(a)
			continue
		}

		switch a.Type {
		case actions.Update:
			// Drop updates on nodes that are getting deleted anyway.
			if _, ok := evicted[a.Node]; ok {
				continue
			}
			result.Add(a)
		case actions.Move:
			// Skip nested moves inside an ancestor that's already turned into a subtree delete+insert.
			if _, ok := demotedDescendants[a.Node]; ok {
				continue
			}
			dstNode, shouldDemote := toDemote[a.Node]
			if !shouldDemote {
				result.Add(a)
				continue
			}

			r := rules.Get(a.Node.GetLanguage())
			deleteSubtree, insertSubtree := isSubtreeDemotion(a.Node, dstNode, ms, r, evicted)

			result.Add(actions.Action{
				Type:    actions.Delete,
				Node:    a.Node,
				Parent:  a.Node.Parent,
				Subtree: deleteSubtree,
			})
			result.Add(actions.Action{
				Type:     actions.Insert,
				Node:     dstNode,
				Parent:   dstNode.Parent,
				Position: dstNode.ChildIndex(),
				Subtree:  insertSubtree,
			})
			// Unmap evicted children so later passes don't treat them as matched. If the
			// parent demotion wasn't a full-subtree delete/insert, emit individual actions
			// for them here so they don't vanish from the edit script.
			for _, d := range a.Node.Descendants() {
				dDst, ok := ms.Src()[d]
				if !ok || !dstNode.Contains(dDst) {
					continue
				}
				if _, isEv := evicted[d]; !isEv {
					continue
				}
				if !deleteSubtree {
					result.Add(actions.Action{
						Type:   actions.Delete,
						Node:   d,
						Parent: d.Parent,
					})
				}
				if !insertSubtree {
					result.Add(actions.Action{
						Type:     actions.Insert,
						Node:     dDst,
						Parent:   dDst.Parent,
						Position: dDst.ChildIndex(),
					})
				}
				ms.Remove(d)
			}
			ms.Remove(a.Node)
		default:
			result.Add(a)
		}
	}
	return result
}

// isDelimitedOrBlockContainer checks if a node is a wrapper, block, or delimited container.
func isDelimitedOrBlockContainer(nodeType string, r *rules.Rules) bool {
	if r != nil {
		return r.IsWrapper(nodeType) || r.IsBlock(nodeType) || r.IsDelimitedContainer(nodeType)
	}
	return rules.IsWrapper(nodeType) || rules.IsBlock(nodeType) || rules.IsDelimitedContainer(nodeType)
}

// isSubtreeDemotion checks whether demoting this move replaces the entire subtree,
// or just re-frames delimiters like () or {}.
func isSubtreeDemotion(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) (deleteSubtree, insertSubtree bool) {
	if src == nil || dst == nil || ms == nil {
		return false, false
	}
	isDelim := isDelimitedOrBlockContainer(src.Type, r)

	hasSurvivingOutside := false
	for _, d := range src.Descendants() {
		if dDst, ok := ms.Src()[d]; ok && !dst.Contains(dDst) {
			hasSurvivingOutside = true
			break
		}
	}
	hasSurvivingInside := hasSurvivingMappedDescendants(src, dst, ms, evicted, r)
	deleteSubtree = len(src.Children) > 0 && (!isDelim || !hasSurvivingInside) && !hasSurvivingOutside

	hasSurvivingSrcOutside := false
	for _, d := range dst.Descendants() {
		if dSrc, ok := ms.Dst()[d]; ok && !src.Contains(dSrc) {
			hasSurvivingSrcOutside = true
			break
		}
	}
	isDstDelim := isDelimitedOrBlockContainer(dst.Type, r)

	hasDstSurvivingInside := false
	for _, d2 := range dst.Descendants() {
		if !isPayloadLeaf(d2, r) {
			continue
		}
		if d1, ok := ms.Dst()[d2]; ok && src.Contains(d1) {
			if _, isEv := evicted[d1]; isEv {
				continue
			}
			hasDstSurvivingInside = true
			break
		}
	}
	insertSubtree = len(dst.Children) > 0 && (!isDstDelim || !hasDstSurvivingInside) && !hasSurvivingSrcOutside

	return deleteSubtree, insertSubtree
}

// hasSurvivingMappedDescendants reports whether any real payload leaves in src mapped to dst without being evicted.
func hasSurvivingMappedDescendants(src, dst *treesitter.ASTNode, ms *engine.Mapping, evicted map[*treesitter.ASTNode]struct{}, r *rules.Rules) bool {
	if src == nil || dst == nil || ms == nil {
		return false
	}
	for _, d1 := range src.Descendants() {
		if _, isEvicted := evicted[d1]; isEvicted {
			continue
		}
		if !isPayloadLeaf(d1, r) {
			continue
		}
		if d2, ok := ms.Src()[d1]; ok {
			if dst.Contains(d2) {
				return true
			}
		}
	}
	return false
}

// computeMoveRetention returns the fraction of payload leaves preserved between
// src and dst in [0.0, 1.0], counting updated leaves at half weight.
func computeMoveRetention(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) float64 {
	if src == nil || dst == nil || ms == nil {
		return 1.0
	}

	nSrc := 0
	mExact := 0
	mUpdated := 0
	for _, d1 := range src.Descendants() {
		if !isPayloadLeaf(d1, r) {
			continue
		}
		nSrc++
		if _, isEv := evicted[d1]; isEv {
			continue
		}
		if d2, ok := ms.Src()[d1]; ok && dst.Contains(d2) {
			if d1.Label == d2.Label {
				mExact++
			} else {
				mUpdated++
			}
		}
	}

	nDst := 0
	for _, d2 := range dst.Descendants() {
		if !isPayloadLeaf(d2, r) {
			continue
		}
		nDst++
	}

	nMax := max(nSrc, nDst)
	if nMax == 0 {
		return 1.0
	}

	effectiveSurviving := float64(mExact) + 0.5*float64(mUpdated)
	return effectiveSurviving / float64(nMax)
}

// shouldDemoteMove reports whether a Move action should be demoted to Delete+Insert
// based on structural significance scoring and scope-aware threshold comparison.
func shouldDemoteMove(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evictedOpt ...map[*treesitter.ASTNode]struct{}) bool {
	if src == nil || dst == nil || ms == nil || r == nil {
		return false
	}

	var evicted map[*treesitter.ASTNode]struct{}
	if len(evictedOpt) > 0 {
		evicted = evictedOpt[0]
	}

	// Leave stationary expression moves alone here so normalizeStationaryWrapperMoves
	// can drop the Move in the next pass without evicting the mapping.
	if isStationaryExpressionMove(src, dst, ms, r, evicted) {
		return false
	}

	// Sibling relocation: src.Parent mapped to dst.Parent within the same enclosing declaration
	// means the parent container is intact and the child was simply reordered within it.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent && sameScopeDeclaration(src, dst, ms, r) {
		return false
	}

	// One-hop container-preserving reparent: the value got wrapped in a brand-new
	// node (e.g. a bare element promoted into a freshly-keyed field) but its
	// structurally-matched container never actually changed.
	if src.Parent != nil && dst.Parent != nil && sameScopeDeclaration(src, dst, ms, r) {
		mappedSrcParent := ms.Src()[src.Parent]
		if mappedSrcParent != nil && dst.Parent.Parent == mappedSrcParent &&
			(r.IsPair(dst.Parent.Type) || r.IsWrapper(dst.Parent.Type)) {
			return false
		}
	}

	isDelimContainer := isDelimitedOrBlockContainer(src.Type, r) || isDelimitedOrBlockContainer(dst.Type, r)
	retention := 1.0
	if isDelimContainer {
		// Don't move an empty container (like () or {}) when none of its contents move with it into dst.
		if len(src.Children) > 0 && !hasSurvivingMappedDescendants(src, dst, ms, evicted, r) {
			return true
		}

		retention = computeMoveRetention(src, dst, ms, r, evicted)

		// Cross-scope container moves need at least 25% leaf retention so matching
		// shells or boilerplate don't get paired across functions.
		if len(src.Children) > 0 && !sameScopeDeclaration(src, dst, ms, r) && retention < 0.25 {
			return true
		}
	}

	score := moveStructuralScore(src, r, ms)
	threshold := requiredMoveThreshold(src, dst, ms, r)
	// When moving across functions or scopes, require the destination to have
	// enough mass on its own so a tiny snippet doesn't match a deleted block.
	if !sameScopeDeclaration(src, dst, ms, r) && !r.IsDeclaration(src.Type) {
		score = min(score, moveStructuralScore(dst, r, ms))
	}

	// Scale container scores by leaf retention so heavily rewritten blocks don't clear the threshold.
	if isDelimContainer {
		score = max(int(float64(score)*retention), 1)
	}
	return score < threshold
}
