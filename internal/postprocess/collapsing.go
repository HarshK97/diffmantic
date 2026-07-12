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
	es = normalizeBareLiteralMoves(es, ms)
	es = normalizeCommentMoves(es, ms)

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
	contentMoveSuppressed := make(map[*actions.Action]bool)

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

	// Collapse/Clean Inserts bottom-up on the destination tree
	for _, parent := range postOrder(dstRoot) {
		if act, ok := inserted[parent]; ok && len(parent.Children) > 0 {
			allChildrenInserted := true
			for _, child := range parent.Children {
				childAct, ok := inserted[child]
				if !ok {
					allChildrenInserted = false
					break
				}
				if suppressed[childAct] {
					if contentMoveSuppressed[childAct] {
						hasActiveInsertChildren := false
						for _, gc := range child.Children {
							if gcAct, gcIns := inserted[gc]; gcIns && !suppressed[gcAct] {
								hasActiveInsertChildren = true
								break
							}
						}
						if hasActiveInsertChildren {
							allChildrenInserted = false
							break
						}
					} else {
						allChildrenInserted = false
						break
					}
				}
			}

			if allChildrenInserted {
				KillChildren(parent, inserted, suppressed)
				act.Subtree = true
			} else {
				// Check if at least one direct child is a Move or Update action of a non-literal content node.
				hasMoveOrUpdateChild := false
				for _, child := range parent.Children {
					if isBareAliasedLiteral(child) {
						continue
					}
					if _, isInserted := inserted[child]; !isInserted {
						srcNode := ms.Dst()[child]
						if srcNode != nil {
							isMove := false
							if moveAct, ok := moved[srcNode]; ok && !suppressed[moveAct] {
								isMove = true
							}
							isUpdate := false
							if updateAct, ok := updated[srcNode]; ok && !suppressed[updateAct] {
								isUpdate = true
							}

							if isMove || isUpdate {
								if nodeSimilarity(srcNode, child, ms) >= 0.5 {
									hasMoveOrUpdateChild = true
									break
								}
							}
						}
					}
				}

				if hasMoveOrUpdateChild {
					suppressed[act] = true
					contentMoveSuppressed[act] = true
				}
			}
		}

	}

	// Suppress redundant scaffolding Insert actions in a second pass, after
	// all Subtree:true/KillChildren determinations are finalized. This avoids
	// a cascade bug where prematurely suppressing a scaffolding child's Insert
	// prevents its parent from reaching Subtree:true.
	for _, node := range postOrder(dstRoot) {
		if node.IsScaffolding() {
			if sAct, ok := inserted[node]; ok && !suppressed[sAct] && !sAct.Subtree {
				if node.Parent != nil {
					if pAct, ok := inserted[node.Parent]; ok && !suppressed[pAct] {
						suppressed[sAct] = true
					}
				}
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
					childDst := ms.Src()[childSrc]
					if childDst == nil || childDst.Parent != dstParent || childAct.Parent != dstParent {
						allChildrenMovedToSameParent = false
						break
					}
					if len(childAct.Node.Children) > 0 && !childAct.Subtree {
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
				for _, pos := range destPositions {
					if pos < 0 {
						contiguous = false
						break
					}
				}
				if contiguous {
					for i := 0; i < len(destPositions)-1; i++ {
						if destPositions[i+1] != destPositions[i]+1 {
							contiguous = false
							break
						}
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
				suppressed[act] = true
			}
		}
	}

	suppressInlineParentRedundancy(actionPtrs, inserted, deleted, suppressed)

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
		if child.IsScaffolding() {
			KillChildren(child, actionMap, suppressed)
		}
	}
}

// suppressInlineParentRedundancy kills a parent Insert/Delete when an inline
// child of the same type already covers the same line. Subtree:true parents
// are never killed (they cover more than the line). Looks one level up only.
func suppressInlineParentRedundancy(
	actionPtrs []*actions.Action,
	inserted, deleted map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for _, a := range actionPtrs {
		if suppressed[a] || a.Node == nil {
			continue
		}
		if a.Type != actions.Insert && a.Type != actions.Delete {
			continue
		}
		node := a.Node
		if node.StartRow != node.EndRow {
			continue
		}
		parent := node.Parent
		if parent == nil {
			continue
		}
		var parentAct *actions.Action
		switch a.Type {
		case actions.Insert:
			parentAct = inserted[parent]
		case actions.Delete:
			parentAct = deleted[parent]
		}
		if parentAct == nil || suppressed[parentAct] {
			continue
		}
		if parentAct.Subtree {
			continue
		}
		if parent.StartRow != parent.EndRow {
			continue
		}
		if parent.StartRow != node.StartRow {
			continue
		}
		suppressed[parentAct] = true
	}
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

var genuineBareOperatorLiterals = map[string]bool{
	"comparison_operator_literal": true,
	"logical_operator_literal":    true,
	"assignment_operator_literal": true,
	"arithmetic_operator_literal": true,
	"bitwise_operator_literal":    true,
	"unary_operator_literal":      true,
	"channel_operator_literal":    true,
	"update_operator_literal":     true,
	"is_operator":                 true,
	"is_not_operator":             true,
}

func isBareAliasedLiteral(node *treesitter.ASTNode) bool {
	return genuineBareOperatorLiterals[node.Type]
}

func nodeSimilarity(src, dst *treesitter.ASTNode, ms *engine.Mapping) float64 {
	if len(src.Children) == 0 && len(dst.Children) == 0 {
		if src.Type == dst.Type && src.Label == dst.Label {
			return 1.0
		}
		return 0.0
	}
	return ms.DiceSrc(src, dst)
}
