package actions

import (
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// hasAction reports whether the script contains an action with matching type and target node.
func hasAction(es *EditScript, aType ActionType, node *treesitter.ASTNode) bool {
	if es == nil {
		return false
	}
	return slices.ContainsFunc(es.Actions(), func(a Action) bool {
		return a.Type == aType && a.Node == node
	})
}

// hasUpdateAction reports whether the script contains an update action for node with target value.
func hasUpdateAction(es *EditScript, node *treesitter.ASTNode, value string) bool {
	if es == nil {
		return false
	}
	return slices.ContainsFunc(es.Actions(), func(a Action) bool {
		return a.Type == Update && a.Node == node && a.Value == value
	})
}

// assertHasAction asserts that the script contains an action with matching type and target node.
func assertHasAction(t *testing.T, es *EditScript, aType ActionType, node *treesitter.ASTNode) {
	t.Helper()
	if !hasAction(es, aType, node) {
		t.Errorf("expected %s action for node %v, got %v", aType, node, es.Actions())
	}
}

// assertHasUpdateAction asserts that the script contains an update action for node with target value.
func assertHasUpdateAction(t *testing.T, es *EditScript, node *treesitter.ASTNode, value string) {
	t.Helper()
	if !hasUpdateAction(es, node, value) {
		t.Errorf("expected Update action changing node %v to %q, got %v", node, value, es.Actions())
	}
}

// assertNoActions asserts that the script contains zero actions matching the target ActionType.
func assertNoActions(t *testing.T, es *EditScript, aType ActionType) {
	t.Helper()
	if es == nil {
		return
	}
	if idx := slices.IndexFunc(es.Actions(), func(a Action) bool { return a.Type == aType }); idx != -1 {
		a := es.Actions()[idx]
		t.Errorf("expected 0 %s actions, found at least one on node %v", aType, a.Node)
	}
}

// assertActionCount asserts that the script contains exactly expected actions of the given type and returns them.
func assertActionCount(t *testing.T, es *EditScript, aType ActionType, expected int) []Action {
	t.Helper()
	var matching []Action
	if es != nil {
		for _, a := range es.Actions() {
			if a.Type == aType {
				matching = append(matching, a)
			}
		}
	}
	if len(matching) != expected {
		t.Fatalf("expected %d %s actions, got %d (all actions: %v)", expected, aType, len(matching), es.Actions())
	}
	return matching
}
