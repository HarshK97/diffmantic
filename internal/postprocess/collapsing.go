package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func Collapse(
	es *actions.EditScript,
	ms *engine.Mapping,
	srcRoot, dstRoot *treesitter.ASTNode,
) *actions.EditScript {
	actionsSlice := es.Actions()
	actionPtrs := make([]*actions.Action, len(actionsSlice))
	for i := range actionsSlice {
		actionPtrs[i] = &actionsSlice[i]
	}

	inserted := make(map[*treesitter.ASTNode]*actions.Action)
	deleted := make(map[*treesitter.ASTNode]*actions.Action)
	moved := make(map[*treesitter.ASTNode]*actions.Action)
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
		}
	}

	// Collapse/Clean Inserts bottom-up on the destination tree
	for _, parent := range postOrder(dstRoot) {
		if act, ok := inserted[parent]; ok && len(parent.Children) > 0 {
			allChildrenInserted := true
			for _, child := range parent.Children {
				childAct, ok := inserted[child]
				if !ok || suppressed[childAct] {
					allChildrenInserted = false
					break
				}
			}

			if allChildrenInserted {
				KillChildren(parent, inserted, suppressed)
				act.Subtree = true
			} else {
				KillParent(act, suppressed)
			}
		}
	}

	// Collapse/Clean Deletes bottom-up on the source tree
	for _, parent := range postOrder(srcRoot) {
		if act, ok := deleted[parent]; ok && len(parent.Children) > 0 {
			allChildrenDeleted := true
			for _, child := range parent.Children {
				childAct, ok := deleted[child]
				if !ok || suppressed[childAct] {
					allChildrenDeleted = false
					break
				}
			}

			if allChildrenDeleted {
				KillChildren(parent, deleted, suppressed)
				act.Subtree = true
			} else {
				KillParent(act, suppressed)
			}
		}
	}

	// Collapse/Clean Moves bottom-up on the source tree
	for _, parentSrc := range postOrder(srcRoot) {
		if act, ok := moved[parentSrc]; ok && len(parentSrc.Children) > 0 {
			allChildrenMovedTogether := true
			for _, childSrc := range parentSrc.Children {
				childAct, ok := moved[childSrc]
				if !ok || suppressed[childAct] {
					allChildrenMovedTogether = false
					break
				}
				dstParent := ms.Src()[parentSrc]
				if dstParent == nil || childAct.Parent != dstParent {
					allChildrenMovedTogether = false
					break
				}
			}

			if allChildrenMovedTogether {
				KillChildren(parentSrc, moved, suppressed)
				act.Subtree = true
			} else {
				KillParent(act, suppressed)
			}
		}
	}

	result := actions.NewEditScript()
	for _, a := range actionPtrs {
		if !suppressed[a] {
			result.Add(*a)
		}
	}
	return result
}

func KillChildren(
	parent *treesitter.ASTNode,
	actionMap map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for _, child := range parent.Children {
		if act, ok := actionMap[child]; ok {
			suppressed[act] = true
		}
	}
}

func KillParent(act *actions.Action, suppressed map[*actions.Action]bool) {
	suppressed[act] = true
}

func postOrder(n *treesitter.ASTNode) []*treesitter.ASTNode {
	if n == nil {
		return nil
	}
	var res []*treesitter.ASTNode
	for _, child := range n.Children {
		res = append(res, postOrder(child)...)
	}
	res = append(res, n)
	return res
}
