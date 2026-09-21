package engine

import (
	"bytes"
	"os"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestMatchIdenticalTrees(t *testing.T) {
	// Identical trees map all nodes without structural changes.
	src := testutil.Node("func", "main",
		testutil.Node("block", "",
			testutil.Leaf("id", "x"),
			testutil.Leaf("id", "y"),
		),
	)
	dst := testutil.Node("func", "main",
		testutil.Node("block", "",
			testutil.Leaf("id", "x"),
			testutil.Leaf("id", "y"),
		),
	)

	r := Match(src, dst, nil, nil, nil)
	if r == nil || r.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	srcNodes := src.PreOrder()
	for _, n := range srcNodes {
		if !r.Mappings.Has(n) {
			t.Errorf("node %s:%s not mapped in identical tree", n.Type, n.Label)
		}
	}
}

func TestMatchDifferentLeafLabels(t *testing.T) {
	// Changing one leaf label still maps the structure.
	src := testutil.Node("func", "main", testutil.Leaf("id", "x"))
	dst := testutil.Node("func", "main", testutil.Leaf("id", "y"))

	r := Match(src, dst, nil, nil, nil)
	if r == nil {
		t.Fatal("Match returned nil")
		return
	}
	if !r.Mappings.Has(src) {
		t.Error("root should be mapped")
	}
}

func TestMatchRootAlwaysMapped(t *testing.T) {
	// Match always maps root nodes, even for different trees.
	src := testutil.Node("func", "a", testutil.Leaf("id", "x"))
	dst := testutil.Node("func", "b", testutil.Leaf("str", "hello"))

	r := Match(src, dst, nil, nil, nil)
	if !r.Mappings.Has(src) {
		t.Error("src root should always be mapped")
	}
	if !r.Mappings.HasDst(dst) {
		t.Error("dst root should always be mapped")
	}
}

func TestMatchSingleLeaves(t *testing.T) {
	src := testutil.Leaf("id", "x")
	dst := testutil.Leaf("id", "x")
	r := Match(src, dst, nil, nil, nil)
	if !r.Mappings.Has(src) || r.Mappings.Src()[src] != dst {
		t.Error("single identical leaves should be mapped")
	}
}

func TestMatchPairsPreOrder(t *testing.T) {
	// Mappings.Pairs follows pre-order traversal.
	c1 := testutil.Leaf("id", "x")
	c2 := testutil.Leaf("id", "y")
	src := testutil.Node("block", "", c1, c2)
	dst := testutil.Node("block", "", testutil.Leaf("id", "x"), testutil.Leaf("id", "y"))

	r := Match(src, dst, nil, nil, nil)
	if len(r.Mappings.Pairs) < 3 {
		t.Fatalf("expected at least 3 pairs, got %d", len(r.Mappings.Pairs))
	}
	if r.Mappings.Pairs[0].Src != src {
		t.Error("first pair should be the root (pre-order)")
	}
}

func TestFprintMappingsNil(t *testing.T) {
	var buf bytes.Buffer
	_ = FprintMappings(&buf, nil)
	if buf.String() != "(no mappings)\n" {
		t.Errorf("unexpected output for nil: %q", buf.String())
	}
}

func TestFprintMappingsEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := &MatchResult{Mappings: NewMapping()}
	_ = FprintMappings(&buf, r)
	if buf.String() != "(no mappings found)\n" {
		t.Errorf("unexpected output for empty: %q", buf.String())
	}
}

func TestTopDownUnambiguous(t *testing.T) {
	// TopDown directly maps unique isomorphic subtrees.
	src := testutil.Node("root", "",
		testutil.Node("call", "", testutil.Leaf("id", "f")),
	)
	dst := testutil.Node("root", "",
		testutil.Node("call", "", testutil.Leaf("id", "f")),
	)

	m := NewMapping()
	TopDown(src, dst, 2, m, nil)
	srcCall := src.Children[0]
	if !m.Has(srcCall) {
		t.Error("unambiguous isomorphic subtree should be mapped by TopDown")
	}
}

func TestBottomUpWithPriorMapping(t *testing.T) {
	// BottomUp maps parents of mapped children.
	srcLeaf := testutil.Leaf("id", "x")
	srcBlock := testutil.Node("block", "", srcLeaf)
	srcRoot := testutil.Node("func", "", srcBlock)

	dstLeaf := testutil.Leaf("id", "x")
	dstBlock := testutil.Node("block", "", dstLeaf)
	dstRoot := testutil.Node("func", "", dstBlock)

	m := NewMapping()
	m.Add(srcLeaf, dstLeaf)

	BottomUp(srcRoot, dstRoot, m, 0.5)

	if !m.Has(srcBlock) {
		t.Error("BottomUp should match block containing matched leaf")
	}
}

func TestMatchUnmatchedLeaves(t *testing.T) {
	// Create two distinct parent blocks with identical leaf types/labels ("identifier", "count").
	l1 := testutil.Leaf("identifier", "count")
	l2 := testutil.Leaf("identifier", "count")
	p1 := testutil.Node("block_a", "", l1)
	p2 := testutil.Node("block_b", "", l2)
	srcRoot := testutil.Node("root", "", p1, p2)

	r1 := testutil.Leaf("identifier", "count")
	r2 := testutil.Leaf("identifier", "count")
	q1 := testutil.Node("block_a", "", r1)
	q2 := testutil.Node("block_b", "", r2)
	dstRoot := testutil.Node("root", "", q1, q2)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(p1, q1)
	m.Add(p2, q2)

	// MatchUnmatchedLeaves should correctly pair l1 -> r1 and l2 -> r2 based on parent mapping.
	MatchUnmatchedLeaves(srcRoot, dstRoot, m, nil)

	if !m.Has(l1) || m.Src()[l1] != r1 {
		t.Errorf("l1 should be matched to r1 under parent block_a, got %v", m.Src()[l1])
	}
	if !m.Has(l2) || m.Src()[l2] != r2 {
		t.Errorf("l2 should be matched to r2 under parent block_b, got %v", m.Src()[l2])
	}
}

func TestRollupMatchedContainers(t *testing.T) {
	k1 := testutil.Leaf("integer", "414")
	v1 := testutil.Leaf("string", "\"request_uri_too_large\"")
	pair1 := testutil.Node("pair", "", k1, v1)
	dict1 := testutil.Node("dictionary", "", pair1)

	k2 := testutil.Leaf("integer", "414")
	v2 := testutil.Leaf("string", "\"request_uri_too_large\"")
	pair2 := testutil.Node("pair", "", k2, v2)
	dict2 := testutil.Node("dictionary", "", pair2)

	m := NewMapping()
	m.Add(dict1, dict2)
	m.Add(k1, k2)
	m.Add(v1, v2)

	RollupMatchedContainers(dict1, dict2, m)

	if !m.Has(pair1) || m.Src()[pair1] != pair2 {
		t.Errorf("RollupMatchedContainers should pair pair1 -> pair2, got %v", m.Src()[pair1])
	}
}

func TestMatchUnmatchedLeavesUnderUnmatchedContainer(t *testing.T) {
	k1 := testutil.Leaf("integer", "414")
	v1 := testutil.Leaf("string", "\"request_uri_too_large\"")
	pair1 := testutil.Node("pair", "", k1, v1)
	dict1 := testutil.Node("dictionary", "", pair1)

	k2 := testutil.Leaf("integer", "414")
	v2 := testutil.Leaf("string", "\"request_uri_too_large\"")
	pair2 := testutil.Node("pair", "", k2, v2)
	dict2 := testutil.Node("dictionary", "", pair2)

	m := NewMapping()
	m.Add(dict1, dict2)

	MatchUnmatchedLeaves(dict1, dict2, m, nil)

	if !m.Has(k1) || m.Src()[k1] != k2 {
		t.Errorf("k1 should match k2 under matched dictionary ancestor, got %v", m.Src()[k1])
	}
	if !m.Has(v1) || m.Src()[v1] != v2 {
		t.Errorf("v1 should match v2 under matched dictionary ancestor, got %v", m.Src()[v1])
	}
}

func TestMatchUnmatchedLeavesIgnoresKeywords(t *testing.T) {
	kw1 := testutil.Leaf("if", "if")
	kw1.IsKeyword = true
	p1 := testutil.Node("if_statement", "", kw1)
	srcRoot := testutil.Node("root", "", p1)

	kw2 := testutil.Leaf("if", "if")
	kw2.IsKeyword = true
	p2 := testutil.Node("if_statement", "", kw2)
	dstRoot := testutil.Node("root", "", p2)

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(p1, p2)

	MatchUnmatchedLeaves(srcRoot, dstRoot, m, nil)

	MatchContainerKeywords(srcRoot, dstRoot, m)

	if !m.Has(kw1) || m.Src()[kw1] != kw2 {
		t.Errorf("MatchContainerKeywords should map kw1 to kw2 under mapped parents, got %v", m.Src()[kw1])
	}
}

func TestMatchPairValues(t *testing.T) {
	k1 := testutil.Leaf("string", "\"priority\"")
	val1 := testutil.Node("object", "", testutil.Leaf("string", "\"a\""))
	p1 := testutil.Node("keyed_element", "", k1, val1)
	srcRoot := testutil.Node("root", "", p1)
	srcRoot.Language = "go"

	k2 := testutil.Leaf("string", "\"priority\"")
	val2 := testutil.Node("object", "", testutil.Leaf("string", "\"b\""))
	p2 := testutil.Node("keyed_element", "", k2, val2)
	dstRoot := testutil.Node("root", "", p2)

	m := NewMapping()
	m.Add(p1, p2)

	matchPairValues(srcRoot, dstRoot, m)

	if !m.Has(val1) || m.Src()[val1] != val2 {
		t.Errorf("matchPairValues should map val1 to val2, got %v", m.Src()[val1])
	}
}

func TestMatchPairKeyNameAffinity(t *testing.T) {
	// Old pair: "priority": { ... }
	kOld := testutil.Leaf("string", "\"priority\"")
	valOld := testutil.Node("object", "")
	pairOld := testutil.Node("pair", "", kOld, valOld)
	srcObj := testutil.Node("object", "", pairOld)

	// New pair 1: "priority": { ... }
	kNew1 := testutil.Leaf("string", "\"priority\"")
	valNew1 := testutil.Node("object", "")
	pairNew1 := testutil.Node("pair", "", kNew1, valNew1)

	// New pair 2: "oneOf": [ ... ]
	kNew2 := testutil.Leaf("string", "\"oneOf\"")
	valNew2 := testutil.Node("array", "")
	pairNew2 := testutil.Node("pair", "", kNew2, valNew2)

	dstObj := testutil.Node("object", "", pairNew1, pairNew2)

	r := Match(srcObj, dstObj, nil, nil, nil)
	if r == nil || r.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	if r.Mappings.Src()[pairOld] != pairNew1 {
		t.Errorf("pairOld ('priority') should match pairNew1 ('priority'), got %v", r.Mappings.Src()[pairOld])
	}
}

func TestBottomUpOuterAncestorPreservation(t *testing.T) {
	srcBytes, err := os.ReadFile("../../tests/testdata/lua_neovim_write_spec_refactor/old.lua")
	if err != nil {
		t.Fatal(err)
	}
	dstBytes, err := os.ReadFile("../../tests/testdata/lua_neovim_write_spec_refactor/new.lua")
	if err != nil {
		t.Fatal(err)
	}

	srcAST, err := treesitter.Parse(srcBytes, "test.lua")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.lua")
	if err != nil {
		t.Fatal(err)
	}

	res := Match(srcAST, dstAST, srcBytes, dstBytes, nil)
	// Find the function_definition at line 96 in src and verify its block maps to dst outer block.
	found := false
	for _, n := range srcAST.Descendants() {
		if n.Type == "function_definition" && n.StartRow == 95 {
			for _, child := range n.Children {
				if child.Type == "block" {
					found = true
					mappedBlock := res.Mappings.Src()[child]
					if mappedBlock == nil {
						t.Fatalf("src outer block at line 96 should be mapped")
					}
					if mappedBlock.StartRow != 96 {
						t.Errorf("mappedBlock.StartRow = %d, want 96", mappedBlock.StartRow)
					}
				}
			}
		}
	}
	if !found {
		t.Fatalf("target function_definition or block at row 95 not found in AST")
	}
}

func TestZipSpecChangeInsertedCallback(t *testing.T) {
	srcBytes, err := os.ReadFile("../../tests/testdata/lua_neovim_zip_spec_change/old.lua")
	if err != nil {
		t.Fatal(err)
	}
	dstBytes, err := os.ReadFile("../../tests/testdata/lua_neovim_zip_spec_change/new.lua")
	if err != nil {
		t.Fatal(err)
	}

	srcAST, err := treesitter.Parse(srcBytes, "test.lua")
	if err != nil {
		t.Fatal(err)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.lua")
	if err != nil {
		t.Fatal(err)
	}

	res := Match(srcAST, dstAST, srcBytes, dstBytes, nil)
	// The return statement inside vim.wait is newly added, so it shouldn't match
	// an existing return in an unrelated callback.
	found := false
	for _, n := range dstAST.Descendants() {
		if n.Type == "return_statement" && n.StartRow == 449 {
			found = true
			if src := res.Mappings.Dst()[n]; src != nil {
				t.Errorf("expected return vim.wait statement to be unmapped (insert), got mapped from %v", src)
			}
		}
	}
	if !found {
		t.Fatalf("target return_statement at row 449 not found")
	}
}

func TestMatchUnmatchedLeaves_IsolatedGlueGuard(t *testing.T) {
	// Two separate statements under an enclosing block.
	// In T1: stmt1: "x := 1"
	// In T2: stmt2: "y := 2"
	// Both have ":=" (assignment_operator_literal).
	// Because stmt1 and stmt2 are distinct statements under a block,
	// parentMatched is false and siblingScore is 0.
	// The isolated glue guard should skip ":=" from matching across statements.

	leafX := testutil.Leaf("identifier", "x")
	leafOp1 := testutil.Leaf("assignment_operator_literal", ":=")
	leafVal1 := testutil.Leaf("int_literal", "1")
	stmt1 := testutil.Node("short_var_declaration", "", leafX, leafOp1, leafVal1)
	stmt1.Language = "go"
	block1 := testutil.Node("block", "", stmt1)
	block1.Language = "go"
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	leafY := testutil.Leaf("identifier", "y")
	leafOp2 := testutil.Leaf("assignment_operator_literal", ":=")
	leafVal2 := testutil.Leaf("int_literal", "2")
	stmt2 := testutil.Node("short_var_declaration", "", leafY, leafOp2, leafVal2)
	stmt2.Language = "go"
	block2 := testutil.Node("block", "", stmt2)
	block2.Language = "go"
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	if m.Has(leafOp1) {
		t.Errorf("expected isolated statement-level glue ':=' not to match across statements, got %v", m.Src()[leafOp1])
	}
}

func TestMatchUnmatchedLeaves_ExpressionExemption(t *testing.T) {
	// Intra-expression operators should be exempt from the glue guard
	// when parent expression types match.
	// T1: "a + b" inside binary_expression
	// T2: "c + d" inside binary_expression
	leafA := testutil.Leaf("identifier", "a")
	leafOp1 := testutil.Leaf("arithmetic_operator_literal", "+")
	leafB := testutil.Leaf("identifier", "b")
	expr1 := testutil.Node("binary_expression", "", leafA, leafOp1, leafB)
	expr1.Language = "go"
	block1 := testutil.Node("block", "", expr1)
	block1.Language = "go"
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	leafC := testutil.Leaf("identifier", "c")
	leafOp2 := testutil.Leaf("arithmetic_operator_literal", "+")
	leafD := testutil.Leaf("identifier", "d")
	expr2 := testutil.Node("binary_expression", "", leafC, leafOp2, leafD)
	expr2.Language = "go"
	block2 := testutil.Node("block", "", expr2)
	block2.Language = "go"
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	if !m.Has(leafOp1) || m.Src()[leafOp1] != leafOp2 {
		t.Errorf("expected intra-expression '+' to match via expression exemption, got %v", m.Src()[leafOp1])
	}
}

func TestEffectiveDepthTo(t *testing.T) {
	r := rules.Get("go")

	// 1. Transparent wrapper (expression_list with 1 child) should not increment depth.
	leaf1 := testutil.Leaf("identifier", "x")
	exprList := testutil.Node("expression_list", "", leaf1)
	block1 := testutil.Node("block", "", exprList)

	depth1 := effectiveDepthTo(leaf1, block1, r)
	if depth1 != 1 {
		t.Errorf("expected effectiveDepthTo with single-child wrapper to be 1, got %d", depth1)
	}

	// 2. Multi-child container (argument_list with 2 children) is not transparent.
	leaf2 := testutil.Leaf("identifier", "x")
	leaf3 := testutil.Leaf("identifier", "y")
	argList := testutil.Node("argument_list", "", leaf2, leaf3)
	block2 := testutil.Node("block", "", argList)

	depth2 := effectiveDepthTo(leaf2, block2, r)
	if depth2 != 2 {
		t.Errorf("expected effectiveDepthTo with multi-child container to be 2, got %d", depth2)
	}

	// 3. Declaration immunity (function_declaration with 1 child is NOT transparent).
	leaf4 := testutil.Leaf("identifier", "x")
	funcDecl := testutil.Node("function_declaration", "", leaf4)
	srcFile := testutil.Node("source_file", "", funcDecl)

	depth3 := effectiveDepthTo(leaf4, srcFile, r)
	if depth3 != 2 {
		t.Errorf("expected effectiveDepthTo with declaration ancestor to be 2 (immune), got %d", depth3)
	}

	// 4. Block immunity (block with 1 child is NOT transparent).
	leaf5 := testutil.Leaf("identifier", "x")
	innerBlock := testutil.Node("block", "", leaf5)
	outerFile := testutil.Node("source_file", "", innerBlock)

	depth4 := effectiveDepthTo(leaf5, outerFile, r)
	if depth4 != 2 {
		t.Errorf("expected effectiveDepthTo with block ancestor to be 2 (immune), got %d", depth4)
	}
}

func TestIsTransparentWrapper(t *testing.T) {
	r := rules.Get("go")

	// Single child wrapper -> true
	w1 := testutil.Node("expression_list", "", testutil.Leaf("identifier", "x"))
	if !isTransparentWrapper(w1, r) {
		t.Errorf("expected expression_list with 1 child to be transparent wrapper")
	}

	// Multi-child wrapper -> false
	w2 := testutil.Node("expression_list", "", testutil.Leaf("identifier", "x"), testutil.Leaf("identifier", "y"))
	if isTransparentWrapper(w2, r) {
		t.Errorf("expected expression_list with 2 children NOT to be transparent wrapper")
	}

	// Single child declaration -> false (immune)
	d1 := testutil.Node("function_declaration", "", testutil.Leaf("identifier", "x"))
	if isTransparentWrapper(d1, r) {
		t.Errorf("expected function_declaration NOT to be transparent wrapper (scope immunity)")
	}

	// Single child block -> false (immune)
	b1 := testutil.Node("block", "", testutil.Leaf("identifier", "x"))
	if isTransparentWrapper(b1, r) {
		t.Errorf("expected block NOT to be transparent wrapper (scope immunity)")
	}

	// Nil -> false
	if isTransparentWrapper(nil, r) {
		t.Errorf("expected nil NOT to be transparent wrapper")
	}
}

func TestAreSiblingsMatched(t *testing.T) {
	m := NewMapping()

	// 1. Direct match
	leafA1 := testutil.Leaf("identifier", "foo")
	leafA2 := testutil.Leaf("identifier", "foo")
	m.Add(leafA1, leafA2)
	if !areSiblingsMatched(leafA1, leafA2, m) {
		t.Errorf("expected direct leaf match to return true")
	}

	// 2. Unwrapping single-child container to mapped child
	leafB1 := testutil.Leaf("identifier", "src")
	leafB2 := testutil.Leaf("identifier", "src")
	m.Add(leafB1, leafB2)
	wrapB1 := testutil.Node("expression_list", "", leafB1)
	wrapB2 := testutil.Node("expression_list", "", leafB2)
	if !areSiblingsMatched(wrapB1, wrapB2, m) {
		t.Errorf("expected single-child container unwrapping to match inner leaves")
	}

	// 3. Multi-child container should not match unmapped wrapper
	leafC1 := testutil.Leaf("identifier", "c1")
	leafC2 := testutil.Leaf("identifier", "c2")
	leafC3 := testutil.Leaf("identifier", "c1")
	leafC4 := testutil.Leaf("identifier", "c2")
	m.Add(leafC1, leafC3)
	m.Add(leafC2, leafC4)
	multi1 := testutil.Node("argument_list", "", leafC1, leafC2)
	multi2 := testutil.Node("argument_list", "", leafC3, leafC4)
	if areSiblingsMatched(multi1, multi2, m) {
		t.Errorf("expected multi-child container NOT to match when container itself is unmapped")
	}
}

func TestMatchUnmatchedLeaves_ContainerWrappedSiblingScore(t *testing.T) {
	// In Go: short_var_declaration has expression_list (wrapping identifier) as child 0.
	// When identifier is mapped, the adjacent operator ':=' must receive siblingScore = 500
	// through container-aware sibling matching, and match cleanly.
	leafSrc1 := testutil.Leaf("identifier", "src")
	wrapSrc1 := testutil.Node("expression_list", "", leafSrc1)
	op1 := testutil.Leaf("assignment_operator_literal", ":=")
	val1 := testutil.Leaf("int_literal", "1")
	wrapVal1 := testutil.Node("expression_list", "", val1)
	stmt1 := testutil.Node("short_var_declaration", "", wrapSrc1, op1, wrapVal1)
	stmt1.Language = "go"
	block1 := testutil.Node("block", "", stmt1)
	block1.Language = "go"
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	leafSrc2 := testutil.Leaf("identifier", "src")
	wrapSrc2 := testutil.Node("expression_list", "", leafSrc2)
	op2 := testutil.Leaf("assignment_operator_literal", ":=")
	val2 := testutil.Leaf("int_literal", "2")
	wrapVal2 := testutil.Node("expression_list", "", val2)
	stmt2 := testutil.Node("short_var_declaration", "", wrapSrc2, op2, wrapVal2)
	stmt2.Language = "go"
	block2 := testutil.Node("block", "", stmt2)
	block2.Language = "go"
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)
	m.Add(leafSrc1, leafSrc2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	if !m.Has(op1) || m.Src()[op1] != op2 {
		t.Errorf("expected ':=' to match via container-aware sibling score, got %v", m.Src()[op1])
	}
}
