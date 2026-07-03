package postprocess

import (
	"fmt"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type groupKey struct {
	oldParent *treesitter.ASTNode
	newParent *treesitter.ASTNode
}

// GroupMoves assigns grouping metadata (GroupID) to Move actions that share
// the exact same source parent and destination parent context.
// Bare aliased literals are excluded from group membership.
func GroupMoves(es *actions.EditScript) *actions.EditScript {
	if es == nil {
		return nil
	}

	actionsSlice := es.Actions()
	if len(actionsSlice) == 0 {
		return es
	}

	// Group indices of Move actions by (oldParent, newParent)
	groups := make(map[groupKey][]int)
	var keyOrder []groupKey

	for i, act := range actionsSlice {
		if act.Type == actions.Move {
			if act.Node == nil || act.Node.Parent == nil || act.Parent == nil {
				continue
			}
			k := groupKey{
				oldParent: act.Node.Parent,
				newParent: act.Parent,
			}
			if _, exists := groups[k]; !exists {
				keyOrder = append(keyOrder, k)
			}
			groups[k] = append(groups[k], i)
		}
	}

	groupIDMap := make(map[int]string)
	groupCounter := 1

	for _, k := range keyOrder {
		indices := groups[k]
		var nonLiteralIndices []int
		for _, idx := range indices {
			act := actionsSlice[idx]
			if !isBareAliasedLiteral(act.Node) {
				nonLiteralIndices = append(nonLiteralIndices, idx)
			}
		}

		if len(nonLiteralIndices) >= 2 {
			gid := fmt.Sprintf("group-%d", groupCounter)
			groupCounter++
			for _, idx := range nonLiteralIndices {
				groupIDMap[idx] = gid
			}
		}
	}

	result := actions.NewEditScript()
	for i, act := range actionsSlice {
		if gid, ok := groupIDMap[i]; ok {
			act.GroupID = gid
		}
		result.Add(act)
	}

	return result
}
