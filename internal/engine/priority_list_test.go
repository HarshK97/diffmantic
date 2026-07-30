package engine

import (
	"math"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestPriorityListPushAndPeek(t *testing.T) {
	l := NewPriorityList()

	// PeekMax returns MinInt32 for an empty list.
	if got := PeekMax(l); got != math.MinInt32 {
		t.Errorf("empty PeekMax = %d, want MinInt32", got)
	}

	leaf := testutil.Leaf("id", "x")         // height 1
	inner := testutil.Node("call", "", leaf) // height 2
	root := testutil.Node("func", "", inner) // height 3

	Push(leaf, l)
	Push(root, l)
	Push(inner, l)

	if got := PeekMax(l); got != 3 {
		t.Errorf("PeekMax = %d, want 3", got)
	}
}

func TestPriorityListPop(t *testing.T) {
	l := NewPriorityList()

	leaf := testutil.Leaf("id", "x")
	root := testutil.Node("func", "", testutil.Node("call", "", leaf))

	Push(root, l)
	Push(leaf, l)

	// Pop returns nodes with the highest height.
	popped := Pop(l)
	if len(popped) != 1 || popped[0] != root {
		t.Error("Pop should return root (height 3)")
	}

	// Second Pop returns the leaf.
	popped = Pop(l)
	if len(popped) != 1 || popped[0] != leaf {
		t.Error("second Pop should return leaf (height 1)")
	}

	// Pop returns nil when empty.
	if Pop(l) != nil {
		t.Error("Pop on empty list should return nil")
	}
}

func TestPriorityListSameHeight(t *testing.T) {
	l := NewPriorityList()
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")

	Push(a, l)
	Push(b, l)

	popped := Pop(l)
	if len(popped) != 2 {
		t.Errorf("Pop should return both leaves, got %d", len(popped))
	}
}

func TestOpen(t *testing.T) {
	l := NewPriorityList()
	c1 := testutil.Leaf("id", "x")
	c2 := testutil.Leaf("id", "y")
	root := testutil.Node("call", "", c1, c2)

	Open(root, l)

	if PeekMax(l) != 1 {
		t.Errorf("after Open, PeekMax should be 1 (children are leaves), got %d", PeekMax(l))
	}
	popped := Pop(l)
	if len(popped) != 2 {
		t.Errorf("Open should push both children, got %d", len(popped))
	}
}
