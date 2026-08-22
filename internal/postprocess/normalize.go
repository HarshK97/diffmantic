package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

const commentSimilarityThreshold = 0.7

// Converts spurious moves on bare literals (like "=", "+", "and") and isolated keywords
// into delete/insert pairs. We only split moves when the literal or keyword matched across
// unrelated parent contexts. If the parent moved along with the node, keep the move so
// parent collapsing still works.
func isSpuriousMoveCandidate(node *treesitter.ASTNode) bool {
	if node == nil {
		return false
	}
	if node.IsKeyword || isPureKeywordTree(node) {
		return true
	}
	if isBareAliasedLiteral(node) {
		return true
	}
	switch node.Type {
	case "type", "type_identifier", "primitive_type", "placeholder_type_specifier",
		"integer", "float", "string", "true", "false", "none", "nil",
		"identifier", "field_identifier", "property_identifier":
		return true
	}
	return false
}

func isPureKeywordTree(node *treesitter.ASTNode) bool {
	if node == nil || len(node.Children) == 0 {
		return false
	}
	leaves := node.Leaves()
	if len(leaves) == 0 {
		return false
	}
	for _, l := range leaves {
		if !l.IsKeyword {
			return false
		}
	}
	return true
}

func removeSubtreeMappings(node *treesitter.ASTNode, ms *engine.Mapping) {
	for _, n := range node.PreOrder() {
		ms.Remove(n)
	}
}

func splitMoveToDeleteInsert(result *actions.EditScript, srcNode, dstNode *treesitter.ASTNode) {
	result.Add(actions.Action{
		Type: actions.Delete,
		Node: srcNode,
	})
	if dstNode != nil {
		result.Add(actions.Action{
			Type:     actions.Insert,
			Node:     dstNode,
			Parent:   dstNode.Parent,
			Position: dstNode.ChildIndex(),
		})
	}
}

func isCommentASTNode(node *treesitter.ASTNode, rulesSrc, rulesDst *treesitter.Rules) bool {
	if node == nil {
		return false
	}
	if node.Type == "comment" {
		return true
	}
	if rulesSrc != nil && rulesSrc.IsComment(node.Type) {
		return true
	}
	if rulesDst != nil && rulesDst.IsComment(node.Type) {
		return true
	}
	if r := treesitter.GetRules(node.GetLanguage()); r != nil && r.IsComment(node.Type) {
		return true
	}
	return false
}

func normalizeBareLiteralMoves(es *actions.EditScript, ms *engine.Mapping, roots ...*treesitter.ASTNode) *actions.EditScript {
	if ms == nil {
		return es
	}

	var rulesSrc, rulesDst *treesitter.Rules
	if len(roots) > 0 && roots[0] != nil {
		rulesSrc = treesitter.GetRules(roots[0].GetLanguage())
	}
	if len(roots) > 1 && roots[1] != nil {
		rulesDst = treesitter.GetRules(roots[1].GetLanguage())
	}

	convertedSrc := make(map[*treesitter.ASTNode]bool)
	convertedDst := make(map[*treesitter.ASTNode]bool)

	type moveDecision struct {
		shouldNormalize bool
		dstNode         *treesitter.ASTNode
	}
	decisions := make(map[*treesitter.ASTNode]moveDecision)

	// Pre-map destination nodes that have move actions
	movedDstNodes := make(map[*treesitter.ASTNode]*actions.Action)
	actionsSlice := es.Actions()
	for i := range actionsSlice {
		a := &actionsSlice[i]
		if a.Type == actions.Move && a.Node != nil {
			if dstNode := ms.Src()[a.Node]; dstNode != nil {
				movedDstNodes[dstNode] = a
			}
		}
	}

	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil || !isSpuriousMoveCandidate(a.Node) {
			continue
		}
		dstNode := ms.Src()[a.Node]
		if dstNode == nil {
			continue
		}
		srcParent := a.Node.Parent
		var dstParentMapped *treesitter.ASTNode
		if srcParent != nil {
			dstParentMapped = ms.Src()[srcParent]
		}
		sameParent := dstParentMapped != nil && a.Parent == dstParentMapped
		if isOrphanedSplitKeyword(a.Node, a.Parent, ms) || (!sameParent && !hasMovedSiblingFromSameParent(a.Node, a.Parent, srcParent, movedDstNodes)) {
			decisions[a.Node] = moveDecision{
				shouldNormalize: true,
				dstNode:         dstNode,
			}
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
			if a.Node == nil {
				continue
			}
			isComment := isCommentASTNode(a.Node, rulesSrc, rulesDst)
			if !isComment && a.DestNode == nil && (ms == nil || ms.Src()[a.Node] == nil) {
				// The node is unmapped (e.g. because its ancestor was normalized and broke the mapping),
				// so any Move action on it is invalid/redundant and should be dropped.
				continue
			}

			// Normalize candidate spurious moves (operators, literals, types, identifiers)
			// when matched across unrelated parent contexts.
			if dec, ok := decisions[a.Node]; ok && dec.shouldNormalize {
				// Break mappings for this subtree so they don't generate spurious move actions.
				removeSubtreeMappings(a.Node, ms)
				splitMoveToDeleteInsert(result, a.Node, dec.dstNode)
				continue
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
func normalizeCommentMoves(es *actions.EditScript, ms *engine.Mapping, roots ...*treesitter.ASTNode) *actions.EditScript {
	if es == nil {
		return es
	}

	var rulesSrc, rulesDst *treesitter.Rules
	if len(roots) > 0 && roots[0] != nil {
		rulesSrc = treesitter.GetRules(roots[0].GetLanguage())
	}
	if len(roots) > 1 && roots[1] != nil {
		rulesDst = treesitter.GetRules(roots[1].GetLanguage())
	}

	isComment := func(node *treesitter.ASTNode) bool {
		return isCommentASTNode(node, rulesSrc, rulesDst)
	}

	commentMovedFuzzy := make(map[*treesitter.ASTNode]bool)
	commentMovedConverted := make(map[*treesitter.ASTNode]bool)
	commentMovedConvertedDst := make(map[*treesitter.ASTNode]bool)

	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil && isComment(a.Node) {
			dstNode := a.DestNode
			if dstNode == nil && ms != nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
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

		if a.Type == actions.Update && isComment(a.Node) {
			if commentMovedConverted[a.Node] || commentMovedConvertedDst[a.Node] {
				continue
			}
		}

		if a.Type == actions.Move && isComment(a.Node) {
			if commentMovedFuzzy[a.Node] {
				result.Add(a)
				continue
			}

			if commentMovedConverted[a.Node] {
				dstNode := a.DestNode
				if dstNode == nil && ms != nil {
					dstNode = ms.Src()[a.Node]
				}
				if ms != nil {
					removeSubtreeMappings(a.Node, ms)
				}
				splitMoveToDeleteInsert(result, a.Node, dstNode)
				continue
			}
		}

		result.Add(a)
	}
	return result
}

func hasMovedSiblingFromSameParent(
	node *treesitter.ASTNode,
	parent *treesitter.ASTNode,
	srcParent *treesitter.ASTNode,
	movedDstNodes map[*treesitter.ASTNode]*actions.Action,
) bool {
	if node == nil || srcParent == nil || parent == nil {
		return false
	}
	for _, child := range parent.Children {
		if act, ok := movedDstNodes[child]; ok && act != nil && act.Node != nil && act.Node != node {
			if act.Node.Parent == srcParent {
				return true
			}
		}
	}
	return false
}

func isOrphanedSplitKeyword(node *treesitter.ASTNode, dstParent *treesitter.ASTNode, ms *engine.Mapping) bool {
	if node == nil || ms == nil || (!node.IsKeyword && !isPureKeywordTree(node)) {
		return false
	}
	srcParent := node.Parent
	if srcParent == nil || dstParent == nil {
		return true
	}
	hasSemanticSiblingInDst := false
	for _, child := range srcParent.Children {
		if child == node || child.IsKeyword || child.IsBracketOrParen() {
			continue
		}
		dstChild := ms.Src()[child]
		if dstChild == nil {
			continue
		}
		if dstChild.Parent != dstParent {
			return true
		}
		hasSemanticSiblingInDst = true
	}
	if !hasSemanticSiblingInDst && hasNonKeywordChildren(srcParent) {
		return true
	}
	return false
}

func hasNonKeywordChildren(n *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	for _, c := range n.Children {
		if !c.IsKeyword && !c.IsBracketOrParen() {
			return true
		}
	}
	return false
}
