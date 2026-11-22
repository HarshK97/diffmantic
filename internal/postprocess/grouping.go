package postprocess

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

type groupKey struct {
	oldParent *treesitter.ASTNode
	newParent *treesitter.ASTNode
}

// isCanonicalContainer reports whether n represents a statement container, block, or scope root.
func isCanonicalContainer(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if n.Parent == nil {
		return true
	}
	if r != nil {
		return r.IsBlock(n.Type) || r.IsCaseClause(n.Type) || r.IsContainerDeclaration(n.Type)
	}
	return rules.IsBlock(n.Type) || rules.IsCaseClause(n.Type) || rules.IsContainerDeclaration(n.Type)
}

func groupingParent(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r := rules.Get(n.GetLanguage())
	stmt := engine.FindEnclosingStatement(n, r)
	curr := n.Parent
	for curr != nil && curr.Parent != nil {
		// Stop at the enclosing statement so expressions in different statements don't group together.
		if curr == stmt {
			return curr
		}
		// For whole statements, stop at the enclosing block or function.
		if isCanonicalContainer(curr, r) {
			return curr
		}
		curr = curr.Parent
	}
	if curr != nil {
		return curr
	}
	return n.Parent
}

func groupingNewParent(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r := rules.Get(n.GetLanguage())
	// If we're already at a block or function, don't climb any further.
	if isCanonicalContainer(n, r) {
		return n
	}
	stmt := engine.FindEnclosingStatement(n, r)
	curr := n
	for curr != nil && curr.Parent != nil {
		if curr == stmt {
			return curr
		}
		if isCanonicalContainer(curr, r) {
			return curr
		}
		curr = curr.Parent
	}
	return n
}

type moveItem struct {
	index    int
	srcStart uint32
	srcEnd   uint32
	dstStart uint32
	dstEnd   uint32
	hasDst   bool
}

// resolveDestinationInfo resolves the target AST node and new parent for a move action.
func resolveDestinationInfo(act actions.Action, ms *engine.Mapping) (destNode, destParent *treesitter.ASTNode) {
	destParent = act.Parent
	if act.DestNode != nil {
		destNode = act.DestNode
		if act.DestNode.Parent != nil {
			destParent = act.DestNode.Parent
		}
	} else if ms != nil && ms.Src() != nil && act.Node != nil {
		destNode = ms.Src()[act.Node]
		if destNode != nil && destNode.Parent != nil {
			destParent = destNode.Parent
		}
	}
	return destNode, destParent
}

// isDestinationCoMover reports whether childAct and parentAct moved to the same destination container.
func isDestinationCoMover(childAct, parentAct actions.Action, ms *engine.Mapping) bool {
	parentDst, _ := resolveDestinationInfo(parentAct, ms)
	childDst, childDestParent := resolveDestinationInfo(childAct, ms)

	if parentDst != nil {
		if childDst != nil {
			return parentDst.Contains(childDst)
		}
		if childDestParent != nil {
			return parentDst == childDestParent || parentDst.Contains(childDestParent)
		}
		return false
	}

	// Fallback when destination AST mapping is unavailable (e.g. isolated edit script tests):
	if childDestParent != nil && parentAct.Node != nil {
		return childDestParent == parentAct.Node || (parentAct.DestNode != nil && childDestParent == parentAct.DestNode)
	}
	return false
}

// GroupMoves groups Move actions that share the same source and destination parents.
// Swapped moves and parent-child moves get separate group IDs (and colors)
// instead of sharing one.
func GroupMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil {
		return nil
	}

	actionsSlice := es.Actions()
	if len(actionsSlice) == 0 {
		return es
	}

	groups := make(map[groupKey][]moveItem)
	var keyOrder []groupKey

	for i, act := range actionsSlice {
		if act.Type == actions.Move {
			dstNode, newP := resolveDestinationInfo(act, ms)
			if act.Node == nil || act.Node.Parent == nil || newP == nil {
				continue
			}

			var dstStart, dstEnd uint32
			hasDst := false
			if dstNode != nil {
				dstStart = dstNode.StartByte
				dstEnd = dstNode.EndByte
				hasDst = true
			}

			k := groupKey{
				oldParent: groupingParent(act.Node),
				newParent: groupingNewParent(newP),
			}
			if _, exists := groups[k]; !exists {
				keyOrder = append(keyOrder, k)
			}
			groups[k] = append(groups[k], moveItem{
				index:    i,
				srcStart: act.Node.StartByte,
				srcEnd:   act.Node.EndByte,
				dstStart: dstStart,
				dstEnd:   dstEnd,
				hasDst:   hasDst,
			})
		}
	}

	groupIDMap := make(map[int]string, len(actionsSlice))
	groupCounter := 1

	for _, k := range keyOrder {
		items := groups[k]
		if len(items) < 2 {
			continue
		}

		// 1. Exclude container nodes that enclose other moving items in this group
		var nonContainers []moveItem
		for _, a := range items {
			isContainer := slices.ContainsFunc(items, func(b moveItem) bool {
				return a.index != b.index && a.srcStart <= b.srcStart && a.srcEnd >= b.srcEnd && (a.srcEnd-a.srcStart) > (b.srcEnd-b.srcStart)
			})
			if !isContainer {
				nonContainers = append(nonContainers, a)
			}
		}

		if len(nonContainers) < 2 {
			continue
		}

		// 2. Keep statements that moved in order grouped together (LIS) instead of dropping the whole group if one got reordered.
		hasAnyDst := slices.ContainsFunc(nonContainers, func(item moveItem) bool {
			return item.hasDst
		})
		if !hasAnyDst {
			if len(nonContainers) >= 2 {
				gid := fmt.Sprintf("group-%d", groupCounter)
				groupCounter++
				for _, item := range nonContainers {
					groupIDMap[item.index] = gid
				}
			}
			continue
		}

		resolved := make([]moveItem, 0, len(nonContainers))
		for _, item := range nonContainers {
			if item.hasDst {
				resolved = append(resolved, item)
			}
		}
		if len(resolved) < 2 {
			continue
		}

		slices.SortFunc(resolved, func(a, b moveItem) int {
			return cmp.Or(
				cmp.Compare(a.srcStart, b.srcStart),
				cmp.Compare(a.dstStart, b.dstStart),
				cmp.Compare(a.index, b.index),
			)
		})

		pool := slices.Clone(resolved)
		for len(pool) >= 2 {
			best := extractLIS(pool)
			if len(best) < 2 {
				break
			}
			gid := fmt.Sprintf("group-%d", groupCounter)
			groupCounter++
			bestIndices := make(map[int]struct{}, len(best))
			for _, item := range best {
				groupIDMap[item.index] = gid
				bestIndices[item.index] = struct{}{}
			}
			pool = slices.DeleteFunc(pool, func(item moveItem) bool {
				_, ok := bestIndices[item.index]
				return ok
			})
		}
	}

	// Copy the container's group ID to child blocks that moved with it.
	for i, act := range actionsSlice {
		if act.Type == actions.Move && act.Node != nil && groupIDMap[i] == "" {
			r := rules.Get(act.Node.GetLanguage())
			if r != nil && r.IsBlock(act.Node.Type) && act.Node.Parent != nil {
				for pIdx, pAct := range actionsSlice {
					if pAct.Type == actions.Move && pAct.Node == act.Node.Parent && isDestinationCoMover(act, pAct, ms) {
						if parentGid := groupIDMap[pIdx]; parentGid != "" {
							groupIDMap[i] = parentGid
							break
						}
					}
				}
			}
		}
	}

	// And copy the group ID up to outer containers that moved along with this block.
	for i, act := range actionsSlice {
		if act.Type == actions.Move && act.Node != nil && groupIDMap[i] == "" {
			r := rules.Get(act.Node.GetLanguage())
			for cIdx, cAct := range actionsSlice {
				if cAct.Type == actions.Move && cAct.Node != nil && cAct.Node.Parent == act.Node &&
					r != nil && r.IsBlock(cAct.Node.Type) && isDestinationCoMover(cAct, act, ms) {
					if childGid := groupIDMap[cIdx]; childGid != "" {
						groupIDMap[i] = childGid
						break
					}
				}
			}
		}
	}

	result := actions.NewEditScript()
	for i, act := range actionsSlice {
		if act.Type == actions.Move {
			act.GroupID = groupIDMap[i]
		}
		result.Add(act)
	}

	return result
}

// extractLIS finds the longest chain of moves that kept their relative order in the destination.
// When multiple chains tie for the maximum number of moves, it prefers the chain covering the
// greatest total code span across source and destination.
func extractLIS(pool []moveItem) []moveItem {
	n := len(pool)
	if n == 0 {
		return nil
	}
	length := make([]int, n)
	weight := make([]int, n)
	prev := make([]int, n)
	bestLen := 0
	bestWeight := 0
	bestIdx := -1

	itemWeight := func(item *moveItem) int {
		w := int(item.srcEnd - item.srcStart)
		if item.hasDst && item.dstEnd > item.dstStart {
			w += int(item.dstEnd - item.dstStart)
		}
		return w
	}

	for i := range n {
		length[i] = 1
		weight[i] = itemWeight(&pool[i])
		prev[i] = -1
		for j := range i {
			if pool[j].srcStart < pool[i].srcStart && pool[j].dstStart < pool[i].dstStart {
				newLen := length[j] + 1
				newWeight := weight[j] + itemWeight(&pool[i])
				if newLen > length[i] || (newLen == length[i] && newWeight > weight[i]) {
					length[i] = newLen
					weight[i] = newWeight
					prev[i] = j
				}
			}
		}
		if length[i] > bestLen || (length[i] == bestLen && weight[i] > bestWeight) {
			bestLen = length[i]
			bestWeight = weight[i]
			bestIdx = i
		}
	}

	if bestLen == 0 {
		return nil
	}
	best := make([]moveItem, bestLen)
	for curr, k := bestIdx, bestLen-1; curr != -1; curr = prev[curr] {
		best[k] = pool[curr]
		k--
	}
	return best
}
