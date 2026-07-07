package engine

import (
	"math"
	"testing"
)

func TestPriorityListPushAndPeek(t *testing.T) {
	l := NewPriorityList()

	// Empty list should return MinInt32.
	if got := PeekMax(l); got != math.MinInt32 {
		t.Errorf("empty PeekMax = %d, want MinInt32", got)
	}

	leaf := mkLeaf("id", "x")           // height 1
	inner := mkNode("call", "", leaf)    // height 2
	root := mkNode("func", "", inner)    // height 3

	Push(leaf, l)
	Push(root, l)
	Push(inner, l)

	if got := PeekMax(l); got != 3 {
		t.Errorf("PeekMax = %d, want 3", got)
	}
}

func TestPriorityListPop(t *testing.T) {
	l := NewPriorityList()

	leaf := mkLeaf("id", "x")
	root := mkNode("func", "", mkNode("call", "", leaf))

	Push(root, l)
	Push(leaf, l)

	// Pop should return the highest-height nodes.
	popped := Pop(l)
	if len(popped) != 1 || popped[0] != root {
		t.Error("Pop should return root (height 3)")
	}

	// Next pop should be the leaf.
	popped = Pop(l)
	if len(popped) != 1 || popped[0] != leaf {
		t.Error("second Pop should return leaf (height 1)")
	}

	// Empty pop returns nil.
	if Pop(l) != nil {
		t.Error("Pop on empty list should return nil")
	}
}

func TestPriorityListSameHeight(t *testing.T) {
	l := NewPriorityList()
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")

	Push(a, l)
	Push(b, l)

	popped := Pop(l)
	if len(popped) != 2 {
		t.Errorf("Pop should return both leaves, got %d", len(popped))
	}
}

func TestOpen(t *testing.T) {
	l := NewPriorityList()
	c1 := mkLeaf("id", "x")
	c2 := mkLeaf("id", "y")
	root := mkNode("call", "", c1, c2)

	Open(root, l)

	if PeekMax(l) != 1 {
		t.Errorf("after Open, PeekMax should be 1 (children are leaves), got %d", PeekMax(l))
	}
	popped := Pop(l)
	if len(popped) != 2 {
		t.Errorf("Open should push both children, got %d", len(popped))
	}
}
