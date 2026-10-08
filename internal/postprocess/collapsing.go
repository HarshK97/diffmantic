package postprocess

import (
	"cmp"
	"slices"

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
	es = normalizeMovesByStructure(es, ms)
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

	// Tier 3B: Hollow Block Move Suppression
	suppressHollowBlockMoves(&actionPtrs, moved, inserted, deleted, suppressed, ms)

	// Tier 3C: Near-Atomic Subtree Collapsing
	collapseNearAtomicContainers(&actionPtrs, inserted, deleted, moved, suppressed, ms, srcRoot, dstRoot)

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
	demoted := make(map[*treesitter.ASTNode]bool)
	for parent, act := range moved {
		if act.Subtree && len(parent.Children) > 0 {
			dstNode := cmp.Or(act.DestNode, ms.Src()[parent])
			if dstNode == nil {
				act.Subtree = false
				continue
			}
			if !canMoveAsSubtree(parent, dstNode, ms) {
				act.Subtree = false
				demoted[parent] = true
			}
		}
	}

	for _, a := range actionPtrs {
		if !suppressed[a] && a.Type == actions.Move && demoted[a.Node] {
			dstNode := cmp.Or(a.DestNode, ms.Src()[a.Node])
			if dstNode == nil {
				continue
			}
			actionPtrs = promoteOrphanedChildren(a.Node, dstNode, ms, moved, deleted, actionPtrs)
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

// suppressCoextensiveWrappers walks single-line parents and drops wrapper actions
// that share the child's start byte, like single-child statements with trailing
// semicolons or scaffolding.
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
		// Parent can't start before the child: keeps opening delimiters like {hash}, [array], or (expr) intact.
		if parent.StartByte == node.StartByte &&
			(len(parent.Children) <= 1 || parent.IsScaffolding() || parent.EndByte == node.EndByte) {
			parentAct := actionMap[parent]
			if parentAct != nil && !suppressed[parentAct] && !parentAct.Subtree {
				suppressed[parentAct] = true
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

// Peek through single-child wrappers so we don't suppress an outer container
// when its wrapped inner expression survived and was matched.
func hasMappedChildOrWrapper(n *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	for _, child := range n.Children {
		if _, ok := m[child]; ok {
			return true
		}
		if r != nil && r.IsWrapper(child.Type) {
			curr := child
			for curr != nil && len(curr.Children) == 1 && r.IsWrapper(curr.Type) {
				curr = curr.Children[0]
				if _, ok := m[curr]; ok {
					return true
				}
			}
		}
	}
	if r != nil && r.IsCall(n.Type) {
		for _, child := range n.Children {
			for _, arg := range child.Children {
				if _, ok := m[arg]; ok {
					return true
				}
			}
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
		hasMapped := hasMappedChildOrWrapper(parent, targetMap, r) || isReceiverMapped(parent, targetMap)
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

// canMoveAsSubtree reports whether every mapped descendant inside src maps into
// dst and vice versa, so the move can stay a single subtree action.
func canMoveAsSubtree(src, dst *treesitter.ASTNode, ms *engine.Mapping) bool {
	for _, d := range src.Descendants() {
		if dDst, ok := ms.Src()[d]; ok && !dst.Contains(dDst) && dDst != dst {
			return false
		}
	}
	for _, d := range dst.Descendants() {
		if dSrc, ok := ms.Dst()[d]; ok && !src.Contains(dSrc) && dSrc != src {
			return false
		}
	}
	return true
}

// promoteOrphanedChildren gives mapped children their own Move actions when a
// parent move loses Subtree: true. Chawathe skips child moves when the parent
// moves as a unit, so without this they'd drop back to plain context.
func promoteOrphanedChildren(
	parent, dstNode *treesitter.ASTNode,
	ms *engine.Mapping,
	moved, deleted map[*treesitter.ASTNode]*actions.Action,
	actionPtrs []*actions.Action,
) []*actions.Action {
	for _, childSrc := range parent.Children {
		if childSrc.IsAnonymous() {
			continue
		}
		if moved[childSrc] != nil || deleted[childSrc] != nil {
			continue
		}
		childDst, ok := ms.Src()[childSrc]
		if !ok || childDst == nil {
			continue
		}
		if !dstNode.Contains(childDst) && childDst != dstNode {
			continue
		}

		subtree := len(childSrc.Children) > 0 && canMoveAsSubtree(childSrc, childDst, ms)

		newAct := &actions.Action{
			Type:     actions.Move,
			Node:     childSrc,
			DestNode: childDst,
			Parent:   dstNode,
			Position: childDst.ChildIndex(),
			Subtree:  subtree,
		}
		actionPtrs = append(actionPtrs, newAct)
		moved[childSrc] = newAct

		if !subtree && len(childSrc.Children) > 0 {
			actionPtrs = promoteOrphanedChildren(childSrc, childDst, ms, moved, deleted, actionPtrs)
		}
	}
	return actionPtrs
}

// isHollowBlockMove checks if a container block move is hollow, meaning the
// surrounding statement header was rewritten or deleted, and fewer than 2
// statements (or under 50% of the block) survived.
func isHollowBlockMove(
	act *actions.Action,
	ms *engine.Mapping,
	r *rules.Rules,
	moved map[*treesitter.ASTNode]*actions.Action,
) bool {
	if act == nil || act.Node == nil || ms == nil {
		return false
	}
	src := act.Node
	isBlock := (r != nil && r.IsBlock(src.Type)) || (r == nil && rules.IsBlock(src.Type))
	if !isBlock {
		return false
	}
	dst := act.DestNode
	if dst == nil {
		dst = ms.Src()[src]
	}
	if dst == nil {
		return false
	}

	if src.Parent == nil || dst.Parent == nil {
		return false
	}

	isStmtOrDecl := func(n *treesitter.ASTNode) bool {
		if n == nil {
			return false
		}
		if r != nil {
			return r.IsStatement(n.Type) || r.IsDeclaration(n.Type)
		}
		return rules.IsStatement(n.Type) || rules.IsDeclaration(n.Type)
	}

	if !isStmtOrDecl(src.Parent) && !isStmtOrDecl(dst.Parent) {
		return false
	}

	// 1. Did the parent statement/declaration move together with the block?
	if parentAct, ok := moved[src.Parent]; ok {
		pDst := parentAct.DestNode
		if pDst == nil {
			pDst = ms.Src()[src.Parent]
		}
		if pDst == dst.Parent {
			return false // Co-moving parent statement: keep block move.
		}
	}

	// 2. Body Statement Retention Threshold

	var sSrc, sDst []*treesitter.ASTNode
	for _, c := range src.Children {
		if isStmtOrDecl(c) {
			sSrc = append(sSrc, c)
		}
	}
	for _, c := range dst.Children {
		if isStmtOrDecl(c) {
			sDst = append(sDst, c)
		}
	}

	mCount := 0
	for _, s := range sSrc {
		if partner, ok := ms.Src()[s]; ok && slices.Contains(sDst, partner) {
			mCount++
		}
	}

	maxCount := max(len(sSrc), len(sDst))
	var rRet float64
	if maxCount > 0 {
		rRet = float64(mCount) / float64(maxCount)
	}

	return mCount < 2 || rRet < 0.50
}

func suppressHollowBlockMoves(
	actionPtrs *[]*actions.Action,
	moved, inserted, deleted map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
	ms *engine.Mapping,
) {
	var toDemote []*actions.Action
	for _, a := range *actionPtrs {
		if a.Type != actions.Move || suppressed[a] || a.Node == nil {
			continue
		}
		r := rules.Get(a.Node.GetLanguage())
		if isHollowBlockMove(a, ms, r, moved) {
			toDemote = append(toDemote, a)
		}
	}

	for _, a := range toDemote {
		dstNode := a.DestNode
		if dstNode == nil && ms != nil {
			dstNode = ms.Src()[a.Node]
		}
		if dstNode == nil {
			continue
		}

		// Promote discrete child moves inside the block before demoting the container.
		*actionPtrs = promoteOrphanedChildren(a.Node, dstNode, ms, moved, deleted, *actionPtrs)

		suppressed[a] = true
		delete(moved, a.Node)

		if ms != nil {
			ms.Remove(a.Node)
		}

		delAct := &actions.Action{
			Type:    actions.Delete,
			Node:    a.Node,
			Parent:  a.Node.Parent,
			Subtree: false,
		}
		*actionPtrs = append(*actionPtrs, delAct)
		deleted[a.Node] = delAct

		insAct := &actions.Action{
			Type:     actions.Insert,
			Node:     dstNode,
			Parent:   dstNode.Parent,
			Position: dstNode.ChildIndex(),
			Subtree:  false,
		}
		*actionPtrs = append(*actionPtrs, insAct)
		inserted[dstNode] = insAct
	}
}

// isLowMassStatement checks if n is a small statement (size <= 4) with no nested blocks.
func isLowMassStatement(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	isStmt := (r != nil && (r.IsStatement(n.Type) || r.IsDeclaration(n.Type))) || (r == nil && (rules.IsStatement(n.Type) || rules.IsDeclaration(n.Type)))
	if !isStmt {
		return false
	}
	if n.Size() > 4 {
		return false
	}
	for _, c := range n.Children {
		isBlock := (r != nil && r.IsBlock(c.Type)) || (r == nil && rules.IsBlock(c.Type))
		if isBlock {
			return false
		}
	}
	return true
}

// collapseNearAtomicContainers collapses blocks with >= 75% churn into atomic subtree
// delete/insert when the only surviving children are trivial low-mass statements (like return nil).
func collapseNearAtomicContainers(
	actionPtrs *[]*actions.Action,
	inserted, deleted, moved map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
) {
	if ms == nil || srcRoot == nil || dstRoot == nil {
		return
	}

	isStmtOrDecl := func(n *treesitter.ASTNode, r *rules.Rules) bool {
		if n == nil {
			return false
		}
		if r != nil {
			return r.IsStatement(n.Type) || r.IsDeclaration(n.Type)
		}
		return rules.IsStatement(n.Type) || rules.IsDeclaration(n.Type)
	}

	for _, parent := range srcRoot.PostOrder() {
		delAct, ok := deleted[parent]
		if !ok || suppressed[delAct] || len(parent.Children) == 0 {
			continue
		}
		r := rules.Get(parent.GetLanguage())
		isBlock := (r != nil && r.IsBlock(parent.Type)) || (r == nil && rules.IsBlock(parent.Type))
		if !isBlock {
			continue
		}

		var stmts []*treesitter.ASTNode
		var surviving []*treesitter.ASTNode
		for _, c := range parent.Children {
			if isStmtOrDecl(c, r) {
				stmts = append(stmts, c)
				if childAct, isDel := deleted[c]; !isDel || suppressed[childAct] {
					surviving = append(surviving, c)
				}
			}
		}

		if len(stmts) < 2 || len(surviving) == 0 {
			continue
		}

		rRet := float64(len(surviving)) / float64(len(stmts))
		if rRet > 0.25 {
			continue
		}

		allLowMass := true
		for _, s := range surviving {
			if !isLowMassStatement(s, r) {
				allLowMass = false
				break
			}
		}
		if !allLowMass {
			continue
		}

		// Demote surviving low-mass matches into independent delete and insert.
		for _, s := range surviving {
			sDst := ms.Src()[s]
			if sDst != nil {
				for _, d := range s.Descendants() {
					ms.Remove(d)
				}
				ms.Remove(s)
				if mAct, isMoved := moved[s]; isMoved {
					suppressed[mAct] = true
					delete(moved, s)
				}
				sIns := &actions.Action{
					Type:     actions.Insert,
					Node:     sDst,
					Parent:   sDst.Parent,
					Position: sDst.ChildIndex(),
					Subtree:  len(sDst.Children) > 0,
				}
				*actionPtrs = append(*actionPtrs, sIns)
				inserted[sDst] = sIns
			}
			sDel := &actions.Action{
				Type:    actions.Delete,
				Node:    s,
				Parent:  parent,
				Subtree: len(s.Children) > 0,
			}
			*actionPtrs = append(*actionPtrs, sDel)
			deleted[s] = sDel
		}
	}

	for _, parent := range dstRoot.PostOrder() {
		insAct, ok := inserted[parent]
		if !ok || suppressed[insAct] || len(parent.Children) == 0 {
			continue
		}
		r := rules.Get(parent.GetLanguage())
		isBlock := (r != nil && r.IsBlock(parent.Type)) || (r == nil && rules.IsBlock(parent.Type))
		if !isBlock {
			continue
		}

		var stmts []*treesitter.ASTNode
		var surviving []*treesitter.ASTNode
		for _, c := range parent.Children {
			if isStmtOrDecl(c, r) {
				stmts = append(stmts, c)
				if childAct, isIns := inserted[c]; !isIns || suppressed[childAct] {
					surviving = append(surviving, c)
				}
			}
		}

		if len(stmts) < 2 || len(surviving) == 0 {
			continue
		}

		rRet := float64(len(surviving)) / float64(len(stmts))
		if rRet > 0.25 {
			continue
		}

		allLowMass := true
		for _, s := range surviving {
			if !isLowMassStatement(s, r) {
				allLowMass = false
				break
			}
		}
		if !allLowMass {
			continue
		}

		for _, s := range surviving {
			sSrc := ms.Dst()[s]
			if sSrc != nil {
				for _, d := range sSrc.Descendants() {
					ms.Remove(d)
				}
				ms.Remove(sSrc)
				if mAct, isMoved := moved[sSrc]; isMoved {
					suppressed[mAct] = true
					delete(moved, sSrc)
				}
				sDel := &actions.Action{
					Type:    actions.Delete,
					Node:    sSrc,
					Parent:  sSrc.Parent,
					Subtree: len(sSrc.Children) > 0,
				}
				*actionPtrs = append(*actionPtrs, sDel)
				deleted[sSrc] = sDel
			}
			sIns := &actions.Action{
				Type:     actions.Insert,
				Node:     s,
				Parent:   parent,
				Position: s.ChildIndex(),
				Subtree:  len(s.Children) > 0,
			}
			*actionPtrs = append(*actionPtrs, sIns)
			inserted[s] = sIns
		}
	}
}
