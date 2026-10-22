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

func groupingParent(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r := rules.Get(n.GetLanguage())
	stmt := engine.FindEnclosingStatement(n, r)
	curr := n.Parent
	for curr != nil && curr.Parent != nil {
		if curr == stmt {
			return curr
		}
		if curr.StartRow != n.StartRow {
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
	stmt := engine.FindEnclosingStatement(n, r)
	curr := n
	for curr != nil && curr.Parent != nil {
		if curr == stmt {
			return curr
		}
		if curr.StartRow != n.StartRow {
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
			newP := act.Parent
			var dstNode *treesitter.ASTNode
			if act.DestNode != nil {
				dstNode = act.DestNode
				if act.DestNode.Parent != nil {
					newP = act.DestNode.Parent
				}
			} else if ms != nil && ms.Src() != nil && act.Node != nil {
				dstNode = ms.Src()[act.Node]
				if dstNode != nil && dstNode.Parent != nil {
					newP = dstNode.Parent
				}
			}
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

	// Pass the container's group ID down to child delimiter blocks.
	for i, act := range actionsSlice {
		if act.Type == actions.Move && act.Node != nil && groupIDMap[i] == "" {
			r := rules.Get(act.Node.GetLanguage())
			if r != nil && r.IsBlock(act.Node.Type) && act.Node.Parent != nil {
				pIdx := slices.IndexFunc(actionsSlice, func(pAct actions.Action) bool {
					return pAct.Type == actions.Move && pAct.Node == act.Node.Parent
				})
				if pIdx >= 0 {
					if parentGid := groupIDMap[pIdx]; parentGid != "" {
						groupIDMap[i] = parentGid
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
func extractLIS(pool []moveItem) []moveItem {
	n := len(pool)
	if n == 0 {
		return nil
	}
	length := make([]int, n)
	prev := make([]int, n)
	bestLen := 0
	bestIdx := -1

	for i := range n {
		length[i] = 1
		prev[i] = -1
		for j := range i {
			if pool[j].srcStart < pool[i].srcStart && pool[j].dstStart < pool[i].dstStart {
				if length[j]+1 > length[i] {
					length[i] = length[j] + 1
					prev[i] = j
				}
			}
		}
		if length[i] > bestLen {
			bestLen = length[i]
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
