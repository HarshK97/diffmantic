package actions

import (
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// filterActions returns all actions in the slice matching the target ActionType.
func filterActions(actions []Action, aType ActionType) []Action {
	var filtered []Action
	for _, a := range actions {
		if a.Type == aType {
			filtered = append(filtered, a)
		}
	}
	return filtered
}

// hasAction returns whether the script contains an action with matching type and target node.
func hasAction(es *EditScript, aType ActionType, node *treesitter.ASTNode) bool {
	for _, a := range es.Actions() {
		if a.Type == aType && a.Node == node {
			return true
		}
	}
	return false
}

// hasUpdateAction returns whether the script contains an update action for node with target value.
func hasUpdateAction(es *EditScript, node *treesitter.ASTNode, value string) bool {
	for _, a := range es.Actions() {
		if a.Type == Update && a.Node == node && a.Value == value {
			return true
		}
	}
	return false
}
