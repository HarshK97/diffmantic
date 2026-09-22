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
		nil,
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
		nil,
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

func TestUniqueTypePairsDeclarationNoSemanticOverlap(t *testing.T) {
	// Two var_declarations that share only the keyword "var" must NOT pair.
	v1 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "headerStyle"),
		testutil.Leaf("type_identifier", "Style"),
	)
	root1 := testutil.Node("source_file", "", v1)
	root1.Language = "go"

	v2 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "rulesObj"),
		testutil.Leaf("type_identifier", "Rules"),
	)
	root2 := testutil.Node("source_file", "", v2)
	root2.Language = "go"

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{v1},
		[]*treesitter.ASTNode{v2},
		nil,
	)
	if len(pairs) != 0 {
		t.Fatalf("declarations sharing only keywords should not pair, got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairsDeclarationWithSemanticOverlap(t *testing.T) {
	// Two var_declarations that share a semantic identifier ("count") should pair.
	v1 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "count"),
		testutil.Leaf("type_identifier", "int"),
	)
	root1 := testutil.Node("source_file", "", v1)
	root1.Language = "go"

	v2 := testutil.Node("var_declaration", "",
		testutil.Leaf("var", "var"),
		testutil.Leaf("identifier", "count"),
		testutil.Leaf("type_identifier", "int64"),
	)
	root2 := testutil.Node("source_file", "", v2)
	root2.Language = "go"

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{v1},
		[]*treesitter.ASTNode{v2},
		nil,
	)
	if len(pairs) != 1 {
		t.Fatalf("declarations sharing semantic identifier should pair, got %d pairs", len(pairs))
	}
}

func TestUniqueTypePairs_UnwrappedCondition(t *testing.T) {
	// src is a chained || while dst is a single check.
	// uniqueTypePairs should match the inner z == nil, not the root ||.
	c1 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "x"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c2 := testutil.Node("binary_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "x"),
			testutil.Leaf("field_identifier", "Parent"),
		),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c3 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "z"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c12 := testutil.Node("binary_expression", "",
		c1,
		testutil.Leaf("logical_operator_literal", "||"),
		c2,
	)
	compoundSrc := testutil.Node("binary_expression", "",
		c12,
		testutil.Leaf("logical_operator_literal", "||"),
		c3,
	)
	compoundSrc.Language = "go"

	simpleDst := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "v"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	simpleDst.Language = "go"

	m := NewMapping()
	m.Add(c3.Children[2], simpleDst.Children[2])

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{compoundSrc},
		[]*treesitter.ASTNode{simpleDst},
		m,
	)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0][0] != c3 {
		t.Errorf("expected inner c3 (z == nil) to be paired, got %v", pairs[0][0])
	}
	if pairs[0][1] != simpleDst {
		t.Errorf("expected simpleDst (v == nil) to be paired, got %v", pairs[0][1])
	}
}

func TestUniqueTypePairs_WrappedCondition(t *testing.T) {
	// Flipped case: src is simple and dst is the chained ||.
	// It should still pair v == nil with the inner z == nil.
	simpleSrc := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "v"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	simpleSrc.Language = "go"

	c1 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "x"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c2 := testutil.Node("binary_expression", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "x"),
			testutil.Leaf("field_identifier", "Parent"),
		),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c3 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "z"),
		testutil.Leaf("comparison_operator_literal", "=="),
		testutil.Leaf("nil", "nil"),
	)
	c12 := testutil.Node("binary_expression", "",
		c1,
		testutil.Leaf("logical_operator_literal", "||"),
		c2,
	)
	compoundDst := testutil.Node("binary_expression", "",
		c12,
		testutil.Leaf("logical_operator_literal", "||"),
		c3,
	)
	compoundDst.Language = "go"

	m := NewMapping()
	m.Add(simpleSrc.Children[2], c3.Children[2])

	pairs := uniqueTypePairs(
		[]*treesitter.ASTNode{simpleSrc},
		[]*treesitter.ASTNode{compoundDst},
		m,
	)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 pair, got %d", len(pairs))
	}
	if pairs[0][0] != simpleSrc {
		t.Errorf("expected simpleSrc (v == nil) to be paired, got %v", pairs[0][0])
	}
	if pairs[0][1] != c3 {
		t.Errorf("expected inner c3 (z == nil) to be paired, got %v", pairs[0][1])
	}
}
