package engine

import (
	"bytes"
	"testing"
)

func TestMatchIdenticalTrees(t *testing.T) {
	// Identical trees → all nodes mapped, no structural changes.
	src := mkNode("func", "main",
		mkNode("block", "",
			mkLeaf("id", "x"),
			mkLeaf("id", "y"),
		),
	)
	dst := mkNode("func", "main",
		mkNode("block", "",
			mkLeaf("id", "x"),
			mkLeaf("id", "y"),
		),
	)

	r := Match(src, dst)
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
	// Same structure, one leaf label differs → should still map structurally.
	src := mkNode("func", "main", mkLeaf("id", "x"))
	dst := mkNode("func", "main", mkLeaf("id", "y"))

	r := Match(src, dst)
	if r == nil {
		t.Fatal("Match returned nil")
	}
	if !r.Mappings.Has(src) {
		t.Error("root should be mapped")
	}
}

func TestMatchRootAlwaysMapped(t *testing.T) {
	// Even completely different trees should map roots.
	src := mkNode("func", "a", mkLeaf("id", "x"))
	dst := mkNode("func", "b", mkLeaf("str", "hello"))

	r := Match(src, dst)
	if !r.Mappings.Has(src) {
		t.Error("src root should always be mapped")
	}
	if !r.Mappings.HasDst(dst) {
		t.Error("dst root should always be mapped")
	}
}

func TestMatchSingleLeaves(t *testing.T) {
	src := mkLeaf("id", "x")
	dst := mkLeaf("id", "x")
	r := Match(src, dst)
	if !r.Mappings.Has(src) || r.Mappings.Src()[src] != dst {
		t.Error("single identical leaves should be mapped")
	}
}

func TestMatchPairsPreOrder(t *testing.T) {
	// Mappings.Pairs should be in pre-order of the src tree.
	c1 := mkLeaf("id", "x")
	c2 := mkLeaf("id", "y")
	src := mkNode("block", "", c1, c2)
	dst := mkNode("block", "", mkLeaf("id", "x"), mkLeaf("id", "y"))

	r := Match(src, dst)
	if len(r.Mappings.Pairs) < 3 {
		t.Fatalf("expected at least 3 pairs, got %d", len(r.Mappings.Pairs))
	}
	if r.Mappings.Pairs[0].Src != src {
		t.Error("first pair should be the root (pre-order)")
	}
}

func TestFprintMappingsNil(t *testing.T) {
	var buf bytes.Buffer
	FprintMappings(&buf, nil)
	if buf.String() != "(no mappings)\n" {
		t.Errorf("unexpected output for nil: %q", buf.String())
	}
}

func TestFprintMappingsEmpty(t *testing.T) {
	var buf bytes.Buffer
	r := &MatchResult{Mappings: NewMapping()}
	FprintMappings(&buf, r)
	if buf.String() != "(no mappings found)\n" {
		t.Errorf("unexpected output for empty: %q", buf.String())
	}
}

func TestTopDownUnambiguous(t *testing.T) {
	// Unique isomorphic subtree → directly mapped in top-down.
	src := mkNode("root", "",
		mkNode("call", "", mkLeaf("id", "f")),
	)
	dst := mkNode("root", "",
		mkNode("call", "", mkLeaf("id", "f")),
	)

	m := TopDown(src, dst, 2)
	srcCall := src.Children[0]
	if !m.Has(srcCall) {
		t.Error("unambiguous isomorphic subtree should be mapped by TopDown")
	}
}

func TestBottomUpWithPriorMapping(t *testing.T) {
	// BottomUp should match containers with matched children.
	srcLeaf := mkLeaf("id", "x")
	srcBlock := mkNode("block", "", srcLeaf)
	srcRoot := mkNode("func", "", srcBlock)

	dstLeaf := mkLeaf("id", "x")
	dstBlock := mkNode("block", "", dstLeaf)
	dstRoot := mkNode("func", "", dstBlock)

	m := NewMapping()
	m.Add(srcLeaf, dstLeaf)

	BottomUp(srcRoot, dstRoot, m, 0.5)

	if !m.Has(srcBlock) {
		t.Error("BottomUp should match block containing matched leaf")
	}
}
