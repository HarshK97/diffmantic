package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// Collapse cleans up fine-grained actions in the edit script by folding
// fully inserted or deleted children into subtree actions, and dropping redundant
// scaffolding and wrapper actions.
func Collapse(
	es *actions.EditScript,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
) *actions.EditScript {
	if es == nil || es.Size() == 0 || ms == nil {
		return es
	}
	es = normalizeCrossScopeNonStructuralMoves(es, ms)
	es = normalizeControlFlowMoves(es, ms)
	es = normalizeOrphanedCallArgumentMoves(es, ms)
	es = normalizeOrphanedDeclarationParameterMoves(es, ms)
	es = normalizeOrphanedOperatorMoves(es, ms)
	es = normalizeStationaryWrapperMoves(es, ms)
	es = normalizeWrapperDelimiterChanges(es, ms)

	actionsSlice := es.Actions()
	actionPtrs := make([]*actions.Action, len(actionsSlice))
	for i := range actionsSlice {
		actionPtrs[i] = &actionsSlice[i]
	}

	inserted := make(map[*treesitter.ASTNode]*actions.Action)
	deleted := make(map[*treesitter.ASTNode]*actions.Action)
	moved := make(map[*treesitter.ASTNode]*actions.Action)
	updated := make(map[*treesitter.ASTNode]*actions.Action)
	suppressed := make(map[*actions.Action]bool)

	for _, a := range actionPtrs {
		switch a.Type {
		case actions.Insert:
			if prev, ok := inserted[a.Node]; ok {
				suppressed[prev] = true
			}
			inserted[a.Node] = a
		case actions.Delete:
			if prev, ok := deleted[a.Node]; ok {
				suppressed[prev] = true
			}
			deleted[a.Node] = a
		case actions.Move:
			if prev, ok := moved[a.Node]; ok {
				suppressed[prev] = true
			}
			moved[a.Node] = a
		case actions.Update:
			if prev, ok := updated[a.Node]; ok {
				suppressed[prev] = true
			}
			updated[a.Node] = a
		}
	}

	// Fold child inserts into parent subtree inserts when the whole branch is new.
	for _, parent := range dstRoot.PostOrder() {
		if act, ok := inserted[parent]; ok && len(parent.Children) > 0 {
			allChildrenInserted := true
			for _, child := range parent.Children {
				childAct, ok := inserted[child]
				if !ok || suppressed[childAct] {
					allChildrenInserted = false
					break
				}
				if len(child.Children) > 0 && !childAct.Subtree {
					allChildrenInserted = false
					break
				}
			}

			if allChildrenInserted {
				KillChildren(parent, inserted, suppressed)
				act.Subtree = true
			}
		}
	}

	// Fold child deletes into parent subtree deletes when the whole branch was removed.
	for _, parent := range srcRoot.PostOrder() {
		if act, ok := deleted[parent]; ok && len(parent.Children) > 0 {
			allChildrenDeleted := true
			for _, child := range parent.Children {
				childAct, ok := deleted[child]
				if !ok || suppressed[childAct] {
					allChildrenDeleted = false
					break
				}
				if len(child.Children) > 0 && !childAct.Subtree {
					allChildrenDeleted = false
					break
				}
			}

			if allChildrenDeleted {
				KillChildren(parent, deleted, suppressed)
				act.Subtree = true
			}
		}
	}

	// A Move action on a parent can only be a subtree move if all its descendants moved with it.
	for parent, act := range moved {
		if act.Subtree && len(parent.Children) > 0 {
			dstNode := act.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[parent]
			}
			if dstNode == nil {
				act.Subtree = false
				continue
			}
			for _, d := range parent.Descendants() {
				if dst, ok := ms.Src()[d]; ok {
					if !dstNode.Contains(dst) && dst != dstNode {
						act.Subtree = false
						break
					}
				}
			}
			if act.Subtree {
				for _, d := range dstNode.Descendants() {
					if src, ok := ms.Dst()[d]; ok {
						if !parent.Contains(src) && src != parent {
							act.Subtree = false
							break
						}
					}
				}
			}
		}
	}

	suppressSurvivingContainers(deleted, ms.Src(), suppressed)
	suppressSurvivingContainers(inserted, ms.Dst(), suppressed)

	suppressInlineParentRedundancy(actionPtrs, ms, inserted, deleted, suppressed)

	result := actions.NewEditScript()
	for _, a := range actionPtrs {
		if !suppressed[a] {
			result.Add(*a)
		}
	}
	return result
}

// KillChildren marks all descendant actions as suppressed under a collapsed subtree.
func KillChildren(
	parent *treesitter.ASTNode,
	actionMap map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for _, child := range parent.Children {
		if act, ok := actionMap[child]; ok {
			suppressed[act] = true
		}
		if child.IsScaffolding() {
			KillChildren(child, actionMap, suppressed)
		}
	}
}

// If a child action already covers the deletion, insertion, or move destination on a line, drop
// its single-line parent wrappers so we don't highlight the same line twice or mask moved nodes.
// Multi-line subtree actions are kept since they span past the single line.
func suppressInlineParentRedundancy(
	actionPtrs []*actions.Action,
	ms *engine.Mapping,
	inserted, deleted map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for _, a := range actionPtrs {
		if suppressed[a] || a.Node == nil {
			continue
		}

		switch a.Type {
		case actions.Insert, actions.Delete:
			node := a.Node
			if node.StartRow != node.EndRow {
				continue
			}

			actionMap := inserted
			if a.Type == actions.Delete {
				actionMap = deleted
			}

			for parent := node.Parent; parent != nil; parent = parent.Parent {
				if parent.StartRow != parent.EndRow || parent.StartRow != node.StartRow {
					break
				}
				// If parent is a pair or adds outer syntax (e.g. parentheses, brackets, keywords, or trailing delimiters) outside its children, do not suppress it.
				r := rules.Get(parent.GetLanguage())
				if (r != nil && r.IsPair(parent.Type)) || (len(parent.Children) > 0 && (parent.StartByte < parent.Children[0].StartByte || parent.EndByte > parent.Children[len(parent.Children)-1].EndByte)) {
					continue
				}
				parentAct := actionMap[parent]
				if parentAct != nil && !suppressed[parentAct] && !parentAct.Subtree {
					suppressed[parentAct] = true
				}
			}

		case actions.Move:
			// 1. Destination-side suppression (suppress phantom Insert wrappers)
			dstNode := a.DestNode
			if dstNode == nil && ms != nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil && dstNode.StartRow == dstNode.EndRow {
				suppressCoextensiveWrappers(dstNode, inserted, suppressed)
			}

			// 2. Source-side suppression (suppress phantom Delete wrappers)
			srcNode := a.Node
			if srcNode != nil && srcNode.StartRow == srcNode.EndRow {
				suppressCoextensiveWrappers(srcNode, deleted, suppressed)
			}
		}
	}
}

// suppressCoextensiveWrappers climbs single-line parents and suppresses wrapper actions
// that are truly coextensive with the child (exact same byte range).
func suppressCoextensiveWrappers(
	node *treesitter.ASTNode,
	actionMap map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for parent := node.Parent; parent != nil; parent = parent.Parent {
		if parent.StartRow != parent.EndRow || parent.StartRow != node.StartRow {
			break
		}
		r := rules.Get(parent.GetLanguage())
		if r != nil && r.IsPair(parent.Type) {
			continue
		}
		// Strict opening invariant: parent must not start before child (protects {hash}, [array], (expr))
		if parent.StartByte == node.StartByte {
			// Only suppress if parent is truly coextensive (exact same EndByte) and is a single-child or scaffolding wrapper
			if parent.EndByte == node.EndByte && (len(parent.Children) <= 1 || parent.IsScaffolding()) {
				parentAct := actionMap[parent]
				if parentAct != nil && !suppressed[parentAct] && !parentAct.Subtree {
					suppressed[parentAct] = true
				}
			}
		}
	}
}

// isDelimitedContainer reports whether n is enclosed by syntax delimiters like braces or parens, or is a key-value pair.
func isDelimitedContainer(n *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	r := rules.Get(n.GetLanguage())
	if r == nil {
		return false
	}
	if r.IsBlock(n.Type) || r.IsUnordered(n.Type) || r.IsIndexed(n.Type) || r.IsPair(n.Type) {
		return true
	}
	// Only treat a wrapper as delimited if it has brackets or parens on both ends.
	// Single-ended wrappers (like prefix casts or unary &x) should stay collapsible.
	if r.IsWrapper(n.Type) && len(n.Children) > 0 {
		first := n.Children[0]
		last := n.Children[len(n.Children)-1]
		if n.StartByte < first.StartByte && n.EndByte > last.EndByte {
			return true
		}
	}
	return false
}

// isReceiverMapped checks if the innermost receiver in a method chain is mapped.
func isReceiverMapped(n *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) bool {
	curr := n
	for curr != nil && len(curr.Children) > 0 {
		curr = curr.Children[0]
		if _, ok := m[curr]; ok {
			return true
		}
	}
	return false
}

// suppressSurvivingContainers drops container delete/insert actions when direct children survived
// and modified tokens already have their own actions, or if it's a 1:1 wrapper. Delimited containers
// and declarations are kept so delimiters and keywords stay highlighted.
func suppressSurvivingContainers(
	actionMap map[*treesitter.ASTNode]*actions.Action,
	targetMap map[*treesitter.ASTNode]*treesitter.ASTNode,
	suppressed map[*actions.Action]bool,
) {
	for parent, act := range actionMap {
		r := rules.Get(parent.GetLanguage())
		if len(parent.Children) == 0 || isDelimitedContainer(parent) || (r != nil && r.IsDeclaration(parent.Type) && !r.IsWrapper(parent.Type)) {
			continue
		}
		// If parent has leading or trailing syntax (e.g. keywords, semicolons, delimiters) outside its children, preserve it.
		hasOuterSyntax := len(parent.Children) > 0 && (parent.StartByte < parent.Children[0].StartByte || parent.EndByte > parent.Children[len(parent.Children)-1].EndByte)
		if hasOuterSyntax {
			continue
		}
		hasMapped := false
		for _, child := range parent.Children {
			if _, ok := targetMap[child]; ok {
				hasMapped = true
				break
			}
			if r != nil && r.IsCall(parent.Type) {
				for _, arg := range child.Children {
					if _, ok := targetMap[arg]; ok {
						hasMapped = true
						break
					}
				}
			}
		}
		if !hasMapped && isReceiverMapped(parent, targetMap) {
			hasMapped = true
		}
		hasDescAction := false
		for _, d := range parent.Descendants() {
			if childAct, ok := actionMap[d]; ok && !suppressed[childAct] {
				hasDescAction = true
				break
			}
		}
		allChildrenMapped := true
		for _, child := range parent.Children {
			if _, ok := targetMap[child]; !ok {
				allChildrenMapped = false
				break
			}
		}
		transparentWrapper := allChildrenMapped && (parent.IsScaffolding() || (len(parent.Children) == 1 &&
			parent.StartByte == parent.Children[0].StartByte &&
			parent.EndByte == parent.Children[0].EndByte &&
			parent.IsWrapper()))

		if (hasMapped && hasDescAction) || transparentWrapper {
			suppressed[act] = true
		}
	}
}
