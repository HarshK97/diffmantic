package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// FilterPunctuation removes noisy punctuation edits or converts move/update actions on punctuation into separate delete and insert steps.
func FilterPunctuation(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil {
		return nil
	}

	activeNodes := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions() {
		if a.Node != nil {
			activeNodes[a.Node] = true
		}
	}

	filtered := actions.NewEditScript()

	for _, a := range es.Actions() {
		if a.Node == nil {
			filtered.Add(a)
			continue
		}

		if isDelimiterPunctuation(a.Node) {
			continue
		}

		// Skip bracket and paren moves unless the surrounding block is also changing.
		if (a.Type == actions.Move || a.Type == actions.Update) && isStructuralPunctuation(a.Node) && !hasActiveContainer(a.Node, activeNodes) {
			continue
		}

		if (a.Type == actions.Move || a.Type == actions.Update) && isStrictPunctuation(a.Node) {
			splitMoveToDeleteInsert(filtered, a.Node, ms.Src()[a.Node])
		} else {
			filtered.Add(a)
		}
	}

	return filtered
}

func hasActiveContainer(n *treesitter.ASTNode, activeNodes map[*treesitter.ASTNode]bool) bool {
	if n == nil || n.Parent == nil {
		return false
	}
	return activeNodes[n.Parent] || (n.Parent.Parent != nil && activeNodes[n.Parent.Parent])
}

func isDelimiterPunctuation(n *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	lang := n.GetLanguage()
	if lang != "" {
		r := rules.Get(lang)
		if r != nil {
			return r.IsDelimiter(n.Type, n.Label)
		}
	}
	return rules.IsDelimiter(n.Type, n.Label)
}

// isStrictPunctuation checks if a node is punctuation, ignoring structural boundaries like braces and parens.
func isStrictPunctuation(n *treesitter.ASTNode) bool {
	if !engine.IsTrivialLeaf(n) {
		return false
	}
	return !n.IsBracketOrParen()
}
