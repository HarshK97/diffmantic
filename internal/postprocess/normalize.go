package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

const commentSimilarityThreshold = 0.7

// normalizeBareLiteralMoves finds all Move actions in the edit script where the
// moved node is a bare aliased literal (such as "=", "and", "is not", "+") and
// converts them into a Delete action for the source node and an Insert action for
// the destination node.
//
// Only convert when the match is spurious — the node was matched across unrelated
// parent contexts. If the node and its parent both moved coherently (a.Parent ==
// ms.Src()[a.Node.Parent]), keep the Move so parent Move-collapsing still works.
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

	convertedSrc := make(map[*treesitter.ASTNode]bool)
	convertedDst := make(map[*treesitter.ASTNode]bool)

	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil || !isSpuriousMoveCandidate(a.Node) {
			continue
		}
		if ms.Src()[a.Node] == nil {
			continue
		}
		dstNode := ms.Src()[a.Node]
		srcParent := a.Node.Parent
		var dstParentMapped *treesitter.ASTNode
		if srcParent != nil {
			dstParentMapped = ms.Src()[srcParent]
		}
		if dstParentMapped == nil || a.Parent != dstParentMapped {
			convertedSrc[a.Node] = true
			convertedDst[dstNode] = true
		}
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Node != nil && a.Type == actions.Update && (convertedSrc[a.Node] || convertedDst[a.Node]) {
			continue
		}

		if a.Type == actions.Move {
			if a.Node == nil || ms.Src()[a.Node] == nil {
				// The node is unmapped (e.g. because its ancestor was normalized and broke the mapping),
				// so any Move action on it is invalid/redundant and should be dropped.
				continue
			}
		}

		// Normalize candidate spurious moves (operators, literals, types, identifiers)
		// when matched across unrelated parent contexts.
		if a.Type == actions.Move && isSpuriousMoveCandidate(a.Node) {
			srcParent := a.Node.Parent
			var dstParentMapped *treesitter.ASTNode
			if srcParent != nil {
				dstParentMapped = ms.Src()[srcParent]
			}

			if dstParentMapped == nil || a.Parent != dstParentMapped {
				dstNode := ms.Src()[a.Node]
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

// commentTextSimilarity returns a 0..1 Levenshtein ratio for two strings.
func commentTextSimilarity(s1, s2 string) float64 {
	if s1 == s2 {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}

	r1 := []rune(s1)
	r2 := []rune(s2)

	if len(r1) > len(r2) {
		r1, r2 = r2, r1
	}

	m := len(r1)
	n := len(r2)

	prev := make([]int, m+1)
	for j := 0; j <= m; j++ {
		prev[j] = j
	}

	for i := 1; i <= n; i++ {
		curr := make([]int, m+1)
		curr[0] = i
		for j := 1; j <= m; j++ {
			cost := 1
			if r1[j-1] == r2[i-1] {
				cost = 0
			}
			curr[j] = min(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev = curr
	}

	distance := prev[m]
	maxLen := n
	return 1.0 - float64(distance)/float64(maxLen)
}

// normalizeCommentMoves turns comment Moves into Delete+Insert, but only when
// the text differs enough (below commentSimilarityThreshold). Similar comments
// keep their Move and any paired Update.
func normalizeCommentMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if ms == nil {
		return es
	}

	commentMovedFuzzy := make(map[*treesitter.ASTNode]bool)
	commentMovedConverted := make(map[*treesitter.ASTNode]bool)
	commentMovedConvertedDst := make(map[*treesitter.ASTNode]bool)

	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil && a.Node.Type == "comment" {
			if dstNode := ms.Src()[a.Node]; dstNode != nil {
				sim := commentTextSimilarity(a.Node.Label, dstNode.Label)
				if sim >= commentSimilarityThreshold {
					commentMovedFuzzy[a.Node] = true
				} else {
					commentMovedConverted[a.Node] = true
					commentMovedConvertedDst[dstNode] = true
				}
			} else {
				commentMovedConverted[a.Node] = true
			}
		}
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Node == nil {
			result.Add(a)
			continue
		}

		if a.Type == actions.Update && a.Node.Type == "comment" {
			if commentMovedConverted[a.Node] || commentMovedConvertedDst[a.Node] {
				continue
			}
		}

		if a.Type == actions.Move && a.Node.Type == "comment" {
			if commentMovedFuzzy[a.Node] {
				result.Add(a)
				continue
			}

			if commentMovedConverted[a.Node] {
				dstNode := ms.Src()[a.Node]

				removeSubtreeMappings(a.Node, ms)

				delAct := actions.Action{
					Type: actions.Delete,
					Node: a.Node,
				}
				result.Add(delAct)

				if dstNode != nil {
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
				}
				continue
			}
		}

		result.Add(a)
	}
	return result
}


