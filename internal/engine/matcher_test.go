package engine

import (
	"bytes"
	"os"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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
	p1 := testutil.Node("key_value_pair", "", k1, val1)
	srcRoot := testutil.Node("root", "", p1)
	srcRoot.Language = "go"

	k2 := testutil.Leaf("string", "\"priority\"")
	val2 := testutil.Node("object", "", testutil.Leaf("string", "\"b\""))
	p2 := testutil.Node("key_value_pair", "", k2, val2)
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
