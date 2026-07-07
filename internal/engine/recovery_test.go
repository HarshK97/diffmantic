package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestSimpleRecoveryLabelMatch(t *testing.T) {
	// Parent pair matched; children are isomorphic → should be recovered.
	srcChild := mkLeaf("id", "x")
	srcRoot := mkNode("block", "", srcChild)
	dstChild := mkLeaf("id", "x")
	dstRoot := mkNode("block", "", dstChild)

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
	// Same structure, different labels → structure LCS should recover.
	srcChild := mkNode("call", "", mkLeaf("id", "a"))
	srcRoot := mkNode("block", "", srcChild)
	dstChild := mkNode("call", "", mkLeaf("id", "b"))
	dstRoot := mkNode("block", "", dstChild)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	SimpleRecovery(srcRoot, dstRoot, m)

	if !m.Has(srcChild) {
		t.Error("structurally isomorphic child should be recovered")
	}
}

func TestSimpleRecoveryUniqueType(t *testing.T) {
	// Unique-type pairing: only one "if_stmt" on each side.
	srcIf := mkNode("if_stmt", "", mkLeaf("cond", "a"))
	srcLet := mkLeaf("let", "x")
	srcRoot := mkNode("block", "", srcIf, srcLet)

	dstIf := mkNode("if_stmt", "", mkLeaf("cond", "b"))
	dstLet := mkLeaf("let", "y")
	dstRoot := mkNode("block", "", dstIf, dstLet)

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
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")
	m := NewMapping()
	m.Add(a, b)
	SimpleRecovery(a, b, m)

	if len(m.Pairs) != 1 {
		t.Errorf("leaf recovery should add no new pairs, got %d", len(m.Pairs))
	}
}

func TestUniqueTypePairs(t *testing.T) {
	a1 := mkLeaf("id", "x")
	a2 := mkLeaf("str", "hello")
	b1 := mkLeaf("id", "y")
	b2 := mkLeaf("str", "world")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1, b2},
	)
	if len(pairs) != 2 {
		t.Errorf("want 2 unique-type pairs, got %d", len(pairs))
	}
}

func TestUniqueTypePairsAmbiguous(t *testing.T) {
	a1 := mkLeaf("id", "x")
	a2 := mkLeaf("id", "y")
	b1 := mkLeaf("id", "z")

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{a1, a2},
		[]*treesitter.ASTNode{b1},
	)
	if len(pairs) != 0 {
		t.Errorf("ambiguous type should not pair, got %d", len(pairs))
	}
}
