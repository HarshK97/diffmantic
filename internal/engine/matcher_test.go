package engine

import (
	"bytes"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
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

	r := Match(src, dst, nil, nil)
	if r == nil || r.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	srcNodes := PreOrder(src)
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

	r := Match(src, dst, nil, nil)
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

	r := Match(src, dst, nil, nil)
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
	r := Match(src, dst, nil, nil)
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

	r := Match(src, dst, nil, nil)
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
