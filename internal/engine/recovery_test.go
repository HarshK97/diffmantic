package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestSimpleRecoveryLabelMatch(t *testing.T) {
	// SimpleRecovery maps isomorphic children of mapped parents.
	srcChild := testutil.Leaf("id", "x")
	srcRoot := testutil.Node("block", "", srcChild)
	dstChild := testutil.Leaf("id", "x")
	dstRoot := testutil.Node("block", "", dstChild)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("isomorphic child should be recovered")
	}
	if m.Src()[srcChild] != dstChild {
		t.Error("child should map to corresponding dst child")
	}
}

func TestSimpleRecoveryStructureMatch(t *testing.T) {
	// SimpleRecovery maps children with matching structures but different labels.
	srcChild := testutil.Node("call", "", testutil.Leaf("id", "a"))
	srcRoot := testutil.Node("block", "", srcChild)
	dstChild := testutil.Node("call", "", testutil.Leaf("id", "b"))
	dstRoot := testutil.Node("block", "", dstChild)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("structurally isomorphic child should be recovered")
	}
}

func TestSimpleRecoveryUniqueType(t *testing.T) {
	// SimpleRecovery pairs nodes with unique types.
	srcIf := testutil.Node("if_stmt", "", testutil.Leaf("cond", "a"))
	srcLet := testutil.Leaf("let", "x")
	srcRoot := testutil.Node("block", "", srcIf, srcLet)

	dstIf := testutil.Node("if_stmt", "", testutil.Leaf("cond", "b"))
	dstLet := testutil.Leaf("let", "y")
	dstRoot := testutil.Node("block", "", dstIf, dstLet)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcIf) {
		t.Error("unique-type if_stmt should be paired")
	}
	if m.Src()[srcIf] != dstIf {
		t.Error("if_stmt should map to corresponding dst if_stmt")
	}
}

func TestSimpleRecoveryNoChildren(t *testing.T) {
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")
	m := NewMapping()
	m.Add(a, b)
	SimpleRecovery(a, b, m)

	if len(m.Pairs) != 1 {
		t.Errorf("leaf recovery should add no new pairs, got %d", len(m.Pairs))
	}
}

func TestUniqueTypePairs(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("str", "hello")
	b1 := testutil.Leaf("id", "y")
	b2 := testutil.Leaf("str", "world")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1, b2},
	)
	if len(pairs) != 2 {
		t.Errorf("want 2 unique-type pairs, got %d", len(pairs))
	}
}

func TestUniqueTypePairsAmbiguous(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("id", "y")
	b1 := testutil.Leaf("id", "z")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1},
	)
	if len(pairs) != 0 {
		t.Errorf("ambiguous type should not pair, got %d", len(pairs))
	}
}

func TestSimpleRecoveryStationaryNeighbors(t *testing.T) {
	leftSrc := testutil.Leaf("id", "a")
	midSrc := testutil.Leaf("id", "b")
	rightSrc := testutil.Leaf("id", "c")
	srcRoot := testutil.Node("block", "", leftSrc, midSrc, rightSrc)

	leftDst := testutil.Leaf("id", "a")
	midDst := testutil.Leaf("id", "b")
	rightDst := testutil.Leaf("id", "c")
	dstRoot := testutil.Node("block", "", leftDst, midDst, rightDst)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(leftSrc, leftDst)
	m.Add(rightSrc, rightDst)

	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(midSrc) {
		t.Fatal("stationary middle child was not recovered")
	}
	if m.Src()[midSrc] != midDst {
		t.Errorf("got %v, want %v", m.Src()[midSrc], midDst)
	}
}
