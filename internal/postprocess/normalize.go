package postprocess

import (
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
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
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

// moveStructuralScore scores how structurally significant a node is.
//
//	S = BaseSize + 2*Height + 3*LineSpan + RoleBonus - BoilerplatePenalty
//
// Bare tokens are clamped to S=1. Declarations receive +40. Boilerplate bodies receive -20.
func moveStructuralScore(node *treesitter.ASTNode, r *rules.Rules) int {
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

	// Intra-container reorder: src.Parent mapped to dst.Parent.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent {
		return 1
	}

	// Cross-key guard: a value sitting under one key in a key-value pair
	// (keyed_element, pair) and a value under a *different*, unmapped key
	// should never be treated as a trivial same-line shift, even if they
	// happen to land on the same row. Otherwise a clamped bare token (S=1)
	// can "move" from one struct field to an unrelated one.
	crossKey := src.Parent != nil && dst.Parent != nil && r.IsPair(src.Parent.Type) && r.IsPair(dst.Parent.Type)

	lineDist := ms.AdjustedLineDistance(src, dst)

	// Same-line inline shifts.
	if lineDist == 0 {
		if crossKey {
			return 5
		}
		return 1
	}

	// Top-level declarations: zero distance penalty.
	if r.IsDeclaration(src.Type) {
		return 20
	}

	// Intra-scope statements: mild distance scaling. Floor of 4 so small
	// expressions like bare condition calls (S=4) can still clear the threshold.
	if sameScopeDeclaration(src, dst, ms, r) {
		return 4 + lineDist/5
	}

	// Nearby cross-scope moves (drift < 10) get mild scaling, but only when the
	// source function survived deletion. If it was deleted, AdjustedLineDistance
	// shrinks the gap and lets weak moves slip through.
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
	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil {
			continue
		}
		if _, ok := demotedDescendants[a.Node]; ok {
			continue
		}
		dstNode := a.DestNode
		if dstNode == nil {
			dstNode = ms.Src()[a.Node]
		}
		if dstNode == nil {
			continue
		}
		r := rules.Get(a.Node.GetLanguage())
		if r == nil {
			continue
		}
		if !shouldDemoteMove(a.Node, dstNode, ms, r) {
			continue
		}
		toDemote[a.Node] = dstNode
		for _, d := range a.Node.Descendants() {
			demotedDescendants[d] = struct{}{}
			evicted[d] = struct{}{}
		}
		evicted[a.Node] = struct{}{}
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
			demoteMoveToDelIns(result, ms, a.Node, dstNode)
		default:
			result.Add(a)
		}
	}
	return result
}

// shouldDemoteMove reports whether a Move action should be demoted to Delete+Insert
// based on structural significance scoring and scope-aware threshold comparison.
func shouldDemoteMove(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	if src == nil || dst == nil || ms == nil || r == nil {
		return false
	}
	// Sibling relocation: src.Parent mapped to dst.Parent means the parent
	// container is intact and the child was simply reordered within it.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent {
		return false
	}

	// One-hop container-preserving reparent: the value got wrapped in a brand-new
	// node (e.g. a bare element promoted into a freshly-keyed field) but its
	// structurally-matched container never actually changed.
	if src.Parent != nil && dst.Parent != nil {
		mappedSrcParent := ms.Src()[src.Parent]
		if mappedSrcParent != nil && dst.Parent.Parent == mappedSrcParent &&
			(r.IsPair(dst.Parent.Type) || r.IsWrapper(dst.Parent.Type)) {
			return false
		}
	}

	score := moveStructuralScore(src, r)
	threshold := requiredMoveThreshold(src, dst, ms, r)
	// When moving across functions or scopes, require the destination to have
	// enough mass on its own so a tiny snippet doesn't match a deleted block.
	if !sameScopeDeclaration(src, dst, ms, r) && !r.IsDeclaration(src.Type) {
		score = min(score, moveStructuralScore(dst, r))
	}
	return score < threshold
}

// demoteMoveToDelIns demotes a Move action into separate Delete and Insert actions
// and clears descendant mappings from the mapping store.
func demoteMoveToDelIns(result *actions.EditScript, ms *engine.Mapping, srcNode, dstNode *treesitter.ASTNode) {
	result.Add(actions.Action{
		Type:    actions.Delete,
		Node:    srcNode,
		Parent:  srcNode.Parent,
		Subtree: len(srcNode.Children) > 0,
	})
	result.Add(actions.Action{
		Type:     actions.Insert,
		Node:     dstNode,
		Parent:   dstNode.Parent,
		Position: dstNode.ChildIndex(),
		Subtree:  len(dstNode.Children) > 0,
	})
	for _, d := range srcNode.Descendants() {
		ms.Remove(d)
	}
	ms.Remove(srcNode)
}
