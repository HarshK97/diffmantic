package postprocess

import (
	"cmp"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

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
	toDrop := make(map[*treesitter.ASTNode]struct{})
	demotedDescendants := make(map[*treesitter.ASTNode]struct{})
	evicted := make(map[*treesitter.ASTNode]struct{})

	explicitMoves := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			explicitMoves[a.Node] = true
			dstNode := cmp.Or(a.DestNode, ms.Src()[a.Node])
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				if r != nil && isStationaryExpressionMove(a.Node, dstNode, ms, r, nil) {
					toDrop[a.Node] = struct{}{}
				}
			}
		}
	}

	// Protect small sibling moves travelling with a qualified anchor.
	cohortProtected := buildCohortProtected(es, ms)

	const maxDemotionRounds = 3
	for range maxDemotionRounds {
		changed := false
		for _, a := range es.Actions() {
			if a.Type != actions.Move || a.Node == nil {
				continue
			}
			if _, ok := toDrop[a.Node]; ok {
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
			// Demote isolated leaf moves jumping across deleted/inserted statements.
			// A lone leaf shouldn't show as a move when its enclosing statement was rewritten.
			if isIsolatedLeafCrossStatementMove(a.Node, dstNode, ms, r) {
				toDemote[a.Node] = dstNode
				evicted[a.Node] = struct{}{}
				changed = true
				continue
			}
			if cohortProtected[a.Node] {
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
	var toRemove map[*treesitter.ASTNode]struct{}
	if len(toDemote) > 0 {
		toRemove = make(map[*treesitter.ASTNode]struct{}, len(toDemote)*2)
	}
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
			if _, ok := toDrop[a.Node]; ok {
				continue
			}
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
			// Queue evicted children for batch unmapping so later passes don't treat
			// them as matched. If the parent demotion wasn't a full-subtree delete/insert,
			// emit individual actions so they don't get lost from the edit script.
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
				toRemove[d] = struct{}{}
			}
			toRemove[a.Node] = struct{}{}
		default:
			result.Add(a)
		}
	}

	if len(toRemove) > 0 {
		ms.RemoveSet(toRemove)
	}

	return result
}

// isSubtreeDemotion checks whether demoting this move replaces the entire subtree,
// or just re-frames delimiters like () or {}.
func isSubtreeDemotion(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) (deleteSubtree, insertSubtree bool) {
	if src == nil || dst == nil || ms == nil {
		return false, false
	}
	isDelim := isDelimitedOrBlockContainer(src, r)

	hasSurvivingOutside := false
	for _, d := range src.Descendants() {
		if dDst, ok := ms.Src()[d]; ok && !dst.Contains(dDst) {
			hasSurvivingOutside = true
			break
		}
	}
	hasSurvivingInside := hasSurvivingMappedLeaves(src, dst, ms.Src(), evicted, r)
	deleteSubtree = len(src.Children) > 0 && (!isDelim || !hasSurvivingInside) && !hasSurvivingOutside

	hasSurvivingSrcOutside := false
	for _, d := range dst.Descendants() {
		if dSrc, ok := ms.Dst()[d]; ok && !src.Contains(dSrc) {
			hasSurvivingSrcOutside = true
			break
		}
	}
	isDstDelim := isDelimitedOrBlockContainer(dst, r)

	hasDstSurvivingInside := hasSurvivingMappedLeaves(dst, src, ms.Dst(), evicted, r)
	insertSubtree = len(dst.Children) > 0 && (!isDstDelim || !hasDstSurvivingInside) && !hasSurvivingSrcOutside

	return deleteSubtree, insertSubtree
}

// isIsolatedLeafCrossStatementMove checks if src is a leaf whose surrounding
// statement was rewritten, preventing misleading single-token moves.
func isIsolatedLeafCrossStatementMove(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	if src == nil || dst == nil || ms == nil || len(src.Children) > 0 {
		return false
	}
	if src.Parent != nil && dst.Parent != nil {
		// Sibling reordering under the same mapped parent is legitimate.
		if ms.Src()[src.Parent] == dst.Parent {
			return false
		}
		// Preserve one-hop container unwraps and reparents.
		if src.Parent.Parent != nil && ms.Src()[src.Parent.Parent] == dst.Parent {
			return false
		}
		if dst.Parent.Parent != nil && ms.Src()[src.Parent] == dst.Parent.Parent {
			return false
		}
	}
	srcStmt := engine.FindEnclosingStatement(src, r)
	dstStmt := engine.FindEnclosingStatement(dst, r)
	if srcStmt == nil || dstStmt == nil || srcStmt == src || dstStmt == dst {
		return false
	}
	isStmt1 := (r != nil && r.IsStatement(srcStmt.Type)) || (r == nil && rules.IsStatement(srcStmt.Type))
	isStmt2 := (r != nil && r.IsStatement(dstStmt.Type)) || (r == nil && rules.IsStatement(dstStmt.Type))
	if !isStmt1 || !isStmt2 {
		return false
	}
	// If the enclosing statements are mapped to each other, the statement wasn't rewritten.
	if ms.Src()[srcStmt] == dstStmt {
		return false
	}
	return true
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

	// Keep the move when an intermediate wrapper around src was removed
	// and src now sits directly inside the same matched outer container.
	if src.Parent != nil && dst.Parent != nil && src.Parent.Parent != nil && sameScopeDeclaration(src, dst, ms, r) {
		if !ms.Has(src.Parent) && ms.Src()[src.Parent.Parent] == dst.Parent {
			return false
		}
	}

	// Sibling relocation: src.Parent mapped to dst.Parent within the same enclosing declaration.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent && sameScopeDeclaration(src, dst, ms, r) {
		return false
	}

	// One-hop container reparent: a value was wrapped in a new node (like a key-value pair) under the same container.
	if src.Parent != nil && dst.Parent != nil && sameScopeDeclaration(src, dst, ms, r) {
		mappedSrcParent := ms.Src()[src.Parent]
		if mappedSrcParent != nil && dst.Parent.Parent == mappedSrcParent &&
			(r.IsPair(dst.Parent.Type) || r.IsWrapper(dst.Parent.Type)) {
			return false
		}
	}

	isDecl := r.IsDeclaration(src.Type) || r.IsDeclaration(dst.Type)
	isBlock := r.IsBlock(src.Type) || r.IsBlock(dst.Type)
	var srcBody, dstBody *treesitter.ASTNode
	if !isDecl && !isBlock {
		srcBody = findBodyBlock(src, r)
		dstBody = findBodyBlock(dst, r)
	}
	// For compound statements with a body (loops, conditionals), check how much of the
	// interior body survived. If the body was gutted or replaced across functions,
	// don't let a matching header turn it into a false move.
	if srcBody != nil || dstBody != nil {
		if srcBody == nil || dstBody == nil {
			return true
		}
		bodyRetention := computeMoveRetention(srcBody, dstBody, ms, r, evicted)
		if bodyRetention == 0.0 {
			return true
		}
		if !sameScopeDeclaration(src, dst, ms, r) && bodyRetention < 0.25 {
			return true
		}
	}

	isDelimContainer := isDelimitedOrBlockContainer(src, r) || isDelimitedOrBlockContainer(dst, r)
	isStructuralContainer := isDelimContainer || (!isDecl && (srcBody != nil || dstBody != nil))

	retention := -1.0

	// Don't move an empty container (like () or {}) when none of its contents move with it into dst.
	if isStructuralContainer && len(src.Children) > 0 {
		if !hasSurvivingMappedLeaves(src, dst, ms.Src(), evicted, r) {
			return true
		}

		// Cross-scope container moves need at least 25% leaf retention so matching
		// shells or boilerplate don't get paired across functions.
		if isDelimContainer && !sameScopeDeclaration(src, dst, ms, r) {
			if retention < 0 {
				retention = computeMoveRetention(src, dst, ms, r, evicted)
			}
			if retention < 0.25 {
				return true
			}
		}
	}

	// If an enclosing demotion evicted all mapped tokens here, or if this expression
	// only mapped hollow wrappers with no surviving payload leaves, drop it so we
	// don't leave an empty move shell behind.
	if !isDecl && len(src.Children) > 0 && hasPayloadLeaves(src, r) && hasMappedDescendantIn(src, dst, ms) {
		if !hasSurvivingMappedLeaves(src, dst, ms.Src(), evicted, r) {
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
	if isStructuralContainer {
		if retention < 0 {
			retention = computeMoveRetention(src, dst, ms, r, evicted)
		}
		score = max(int(float64(score)*retention), 1)
	}

	// Anchor protection: don't let a demotion cascade drag down a solid node
	// if its own retention was fine before the cascade.
	const anchorScoreFloor = 30
	if score >= anchorScoreFloor && len(evicted) > 0 {
		if retention < 0 {
			retention = computeMoveRetention(src, dst, ms, r, evicted)
		}
		preEvictionRetention := computeMoveRetention(src, dst, ms, r, nil)
		if retention < preEvictionRetention && preEvictionRetention >= 0.5 {
			return false
		}
	}

	return score < threshold
}
