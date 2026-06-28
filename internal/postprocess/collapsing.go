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
			}
		}
	}

	// Collapse/Clean Moves bottom-up on the source tree
	for _, parentSrc := range postOrder(srcRoot) {
		if act, ok := moved[parentSrc]; ok && len(parentSrc.Children) > 0 {
			allChildrenMovedToSameParent := true
			dstParent := ms.Src()[parentSrc]
			if dstParent == nil {
				allChildrenMovedToSameParent = false
			} else {
				for _, childSrc := range parentSrc.Children {
					childAct, ok := moved[childSrc]
					if !ok || suppressed[childAct] {
						allChildrenMovedToSameParent = false
						break
					}
					if childAct.Parent != dstParent {
						allChildrenMovedToSameParent = false
						break
					}
				}
			}

			if allChildrenMovedToSameParent {
				// Children all move under the same destination parent.
				// Now verify if their destination positions are contiguous.
				var destPositions []int
				for _, childSrc := range parentSrc.Children {
					childDst := ms.Src()[childSrc]
					pos := -1
					for idx, child := range dstParent.Children {
						if child == childDst {
							pos = idx
							break
						}
					}
					destPositions = append(destPositions, pos)
				}

				contiguous := true
				for i := 0; i < len(destPositions)-1; i++ {
					if destPositions[i+1] != destPositions[i]+1 {
						contiguous = false
						break
					}
				}

				if contiguous {
					// Pass both parent-equality and contiguity -> collapse
					KillChildren(parentSrc, moved, suppressed)
					act.Subtree = true
				} else {
					// Pass parent-equality but FAIL contiguity -> do NOT collapse.
					// Both parent move and children moves survive without suppression.
					act.Subtree = false
				}
			} else {
				// FAIL parent-equality -> Kill parent's move, children survive.
				// NOTE: This assumes parent-equality failure indicates a dissolved/rewrapped
				// identity (e.g. boolean operator nesting shifts) where suppressing the parent
				// move avoids misleading highlights of newly inserted sibling elements as moved.
				// This has only been validated against cases where parent-equality failure
				// correctly indicated a dissolved/rewrapped identity, not yet against a case
				// where it might fragment a legitimately-coherent parent-level container move.
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
