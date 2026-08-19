package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// Collapse cleans up fine-grained actions in the edit script by folding
// fully inserted or deleted children into subtree actions, and dropping redundant
// scaffolding and wrapper actions.
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

	// Scaffolding nodes (like statement_list or block) shouldn't emit separate
	// actions if their parent already handles them. We run this after subtree
	// collapsing so child suppressions don't prevent parents from becoming subtrees.
	suppressRedundantScaffolding(dstRoot, inserted, suppressed)
	suppressRedundantScaffolding(srcRoot, deleted, suppressed)
	suppressRedundantScaffolding(srcRoot, moved, suppressed)

	suppressInlineParentRedundancy(actionPtrs, inserted, deleted, suppressed)

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

func suppressRedundantScaffolding(
	root *treesitter.ASTNode,
	actionMap map[*treesitter.ASTNode]*actions.Action,
	suppressed map[*actions.Action]bool,
) {
	for _, node := range root.PostOrder() {
		if !node.IsScaffolding() || node.Parent == nil {
			continue
		}
		sAct, ok := actionMap[node]
		if !ok || suppressed[sAct] || sAct.Subtree {
			continue
		}
		if pAct, ok := actionMap[node.Parent]; ok && !suppressed[pAct] {
			suppressed[sAct] = true
		}
	}
}

// If a child action already covers the deletion or insertion on a line, drop
// its single-line parent wrappers so we don't highlight the same line twice.
// Multi-line subtree actions are kept since they span past the single line.
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

		actionMap := inserted
		if a.Type == actions.Delete {
			actionMap = deleted
		}

		for parent := node.Parent; parent != nil; parent = parent.Parent {
			if parent.StartRow != parent.EndRow || parent.StartRow != node.StartRow {
				break
			}
			parentAct := actionMap[parent]
			if parentAct != nil && !suppressed[parentAct] && !parentAct.Subtree {
				suppressed[parentAct] = true
			}
		}
	}
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
