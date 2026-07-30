package postprocess

import (
	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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

		// Skip commas and semicolons so they don't produce clutter in diffs.
		if a.Node.Type == "semicolon" || a.Node.Type == "comma" || a.Node.Label == ";" || a.Node.Label == "," {
			continue
		}

		// Skip bracket and paren moves unless the surrounding block is also changing.
		if (a.Type == actions.Move || a.Type == actions.Update) && isStructuralPunctuation(a.Node) && !hasActiveContainer(a.Node, activeNodes) {
			continue
		}

		if (a.Type == actions.Move || a.Type == actions.Update) && isStrictPunctuation(a.Node) {
			// Split punctuation moves into delete + insert.
			filtered.Add(actions.Action{
				Type: actions.Delete,
				Node: a.Node,
			})

			dstNode := ms.Dst()[a.Node]
			if dstNode != nil {
				filtered.Add(actions.Action{
					Type:     actions.Insert,
					Node:     dstNode,
					Parent:   a.Parent,
					Position: a.Position,
				})
			}
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

// isStrictPunctuation checks if a node is punctuation, ignoring structural boundaries like braces and parens.
func isStrictPunctuation(n *treesitter.ASTNode) bool {
	if !engine.IsTrivialLeaf(n) {
		return false
	}
	switch n.Label {
	case "(", ")", "{", "}", "[", "]":
		return false
	}
	return true
}
