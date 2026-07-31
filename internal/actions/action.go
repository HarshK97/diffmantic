package actions

import (
	"fmt"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type ActionType int

const (
	Insert ActionType = iota
	Delete
	Update
	Move
)

var actionTypeNames = [...]string{"insert", "delete", "update", "move"}

func (t ActionType) String() string {
	if uint(t) < uint(len(actionTypeNames)) {
		return actionTypeNames[t]
	}
	return "unknown"
}

type Action struct {
	Type     ActionType
	Node     *treesitter.ASTNode
	Parent   *treesitter.ASTNode
	Position int
	Value    string
	Subtree  bool
	GroupID  string
}

func (a Action) String() string {
	switch a.Type {
	case Insert:
		return fmt.Sprintf("%s %s → %s at %d",
			a.Type, NodeToString(a.Node), NodeToString(a.Parent), a.Position)
	case Delete:
		return fmt.Sprintf("%s %s", a.Type, NodeToString(a.Node))
	case Update:
		return fmt.Sprintf("%s %s → %q", a.Type, NodeToString(a.Node), a.Value)
	case Move:
		return fmt.Sprintf("%s %s → %s at %d",
			a.Type, NodeToString(a.Node), NodeToString(a.Parent), a.Position)
	default:
		return fmt.Sprintf("%s %s", a.Type, NodeToString(a.Node))
	}
}

func NodeToString(n *treesitter.ASTNode) string {
	if n == nil {
		return "<nil>"
	}
	pos := fmt.Sprintf("[%d:%d-%d:%d]", n.StartRow+1, n.StartCol, n.EndRow+1, n.EndCol)
	if n.Label != "" {
		return fmt.Sprintf("%s: %s %s", n.Type, n.Label, pos)
	}
	return fmt.Sprintf("%s %s", n.Type, pos)
}
