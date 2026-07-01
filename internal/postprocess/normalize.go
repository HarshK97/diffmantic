package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// normalizeBareLiteralMoves finds all Move actions in the edit script where the
// moved node is a bare aliased literal (such as "=", "and", "is not", "+") and
// converts them into a Delete action for the source node and an Insert action for
// the destination node.
//
// Refinement: We ONLY convert the Move if the match is spurious (i.e., the node
// was matched across unrelated parent contexts). If the node and its parent
// both moved coherently to the same destination context (a.Parent == ms.Src()[a.Node.Parent]),
// we preserve the Move so that parent Move-collapsing continues to work cleanly
// and avoids fragmenting coherent subtrees.
func isSpuriousMoveCandidate(node *treesitter.ASTNode) bool {
	if isBareAliasedLiteral(node) {
		return true
	}
	switch node.Type {
	case "type", "type_identifier", "primitive_type",
		"integer", "float", "string", "true", "false", "none", "nil",
		"identifier":
		return true
	}
	return false
}

func removeSubtreeMappings(node *treesitter.ASTNode, ms *engine.Mapping) {
	if node == nil {
		return
	}
	ms.Remove(node)
	for _, child := range node.Children {
		removeSubtreeMappings(child, ms)
	}
}

func normalizeBareLiteralMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if ms == nil {
		return es
	}
	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move {
			if a.Node == nil || ms.Src() == nil || ms.Src()[a.Node] == nil {
				// The node is unmapped (e.g. because its ancestor was normalized and broke the mapping),
				// so any Move action on it is invalid/redundant and should be dropped.
				continue
			}
		}

		// Normalize candidate spurious moves (operators, literals, types, identifiers)
		// when matched across unrelated parent contexts.
		if a.Type == actions.Move && isSpuriousMoveCandidate(a.Node) && a.Node.Type != "assignment" {
			srcParent := a.Node.Parent
			var dstParentMapped *treesitter.ASTNode
			if srcParent != nil && ms.Src() != nil {
				dstParentMapped = ms.Src()[srcParent]
			}

			if dstParentMapped == nil || a.Parent != dstParentMapped {
				var dstNode *treesitter.ASTNode
				if ms.Src() != nil {
					dstNode = ms.Src()[a.Node]
				}
				if dstNode != nil {
					// Break mappings for this subtree so they don't generate spurious move actions.
					removeSubtreeMappings(a.Node, ms)

					delAct := actions.Action{
						Type: actions.Delete,
						Node: a.Node,
					}
					result.Add(delAct)

					pos := -1
					if dstNode.Parent != nil {
						for idx, child := range dstNode.Parent.Children {
							if child == dstNode {
								pos = idx
								break
							}
						}
					}
					insAct := actions.Action{
						Type:     actions.Insert,
						Node:     dstNode,
						Parent:   dstNode.Parent,
						Position: pos,
					}
					result.Add(insAct)
					continue
				}
			}
		}
		result.Add(a)
	}
	return result
}

