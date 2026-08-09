package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestTopDown(t *testing.T) {
	t.Run("Identical trees", func(t *testing.T) {
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

		m := NewMapping()
		TopDown(src, dst, 1, m, nil)

		for _, n := range src.PreOrder() {
			if !m.Has(n) {
				t.Errorf("expected node %s:%s to be mapped", n.Type, n.Label)
			}
		}
	})

	t.Run("Single leaf label change", func(t *testing.T) {
		srcLeaf1 := testutil.Leaf("id", "x")
		srcLeaf2 := testutil.Leaf("id", "y")
		src := testutil.Node("func", "main",
			testutil.Node("block", "", srcLeaf1, srcLeaf2),
		)

		dstLeaf1 := testutil.Leaf("id", "x")
		dstLeaf2 := testutil.Leaf("id", "z")
		dst := testutil.Node("func", "main",
			testutil.Node("block", "", dstLeaf1, dstLeaf2),
		)

		m := NewMapping()
		TopDown(src, dst, 1, m, nil)

		// TopDown matches isomorphic subtrees like srcLeaf1.
		if !m.Has(srcLeaf1) {
			t.Errorf("expected unchanged leaf to be mapped")
		}
		if m.Has(srcLeaf2) {
			t.Errorf("expected changed leaf to not be mapped by top-down")
		}
	})

	t.Run("Added leaf", func(t *testing.T) {
		srcLeaf := testutil.Leaf("id", "x")
		src := testutil.Node("block", "", srcLeaf)

		dstLeaf1 := testutil.Leaf("id", "x")
		dstLeaf2 := testutil.Leaf("id", "y")
		dst := testutil.Node("block", "", dstLeaf1, dstLeaf2)

		m := NewMapping()
		TopDown(src, dst, 1, m, nil)

		if !m.Has(srcLeaf) {
			t.Errorf("expected original leaf to be mapped")
		}
		if m.HasDst(dstLeaf2) {
			t.Errorf("expected added leaf to not be mapped")
		}
	})

	t.Run("Structurally isomorphic subtrees", func(t *testing.T) {
		srcSub := testutil.Node("call", "", testutil.Leaf("id", "f"))
		src := testutil.Node("func", "a", srcSub)

		dstSub := testutil.Node("call", "", testutil.Leaf("id", "f"))
		dst := testutil.Node("func", "b", dstSub)

		m := NewMapping()
		TopDown(src, dst, 1, m, nil)

		if !m.Has(srcSub) {
			t.Errorf("expected identical subtree to be mapped")
		}
		if m.Has(src) {
			t.Errorf("expected root with different label to not be mapped by top-down")
		}
	})
}

func TestBottomUp(t *testing.T) {
	t.Run("Fills gaps left by TopDown", func(t *testing.T) {
		srcLeaf := testutil.Leaf("id", "x")
		srcBlock := testutil.Node("block", "", srcLeaf)

		dstLeaf := testutil.Leaf("id", "x")
		dstBlock := testutil.Node("block", "", dstLeaf)

		// Simulate TopDown mapping the leaf.
		m := NewMapping()
		m.Add(srcLeaf, dstLeaf)

		BottomUp(srcBlock, dstBlock, m, 0.5)

		if !m.Has(srcBlock) {
			t.Errorf("expected BottomUp to map the parent block")
		}
	})

	t.Run("Single leaf rename", func(t *testing.T) {
		srcLeaf1 := testutil.Leaf("id", "x")
		srcLeaf2 := testutil.Leaf("id", "y")
		srcBlock := testutil.Node("block", "", srcLeaf1, srcLeaf2)
		srcRoot := testutil.Node("func", "main", srcBlock)

		dstLeaf1 := testutil.Leaf("id", "x")
		dstLeaf2 := testutil.Leaf("id", "z")
		dstBlock := testutil.Node("block", "", dstLeaf1, dstLeaf2)
		dstRoot := testutil.Node("func", "main", dstBlock)

		m := NewMapping()
		TopDown(srcRoot, dstRoot, 1, m, nil)
		BottomUp(srcRoot, dstRoot, m, 0.5)

		if !m.Has(srcRoot) {
			t.Errorf("expected root to be mapped after BottomUp")
		}
		if !m.Has(srcBlock) {
			t.Errorf("expected block to be mapped after BottomUp")
		}
	})

	t.Run("Completely different trees", func(t *testing.T) {
		srcLeaf := testutil.Leaf("id", "x")
		src := testutil.Node("func", "a", srcLeaf)

		dstLeaf := testutil.Leaf("str", "hello")
		dst := testutil.Node("func", "b", dstLeaf)

		m := NewMapping()
		TopDown(src, dst, 1, m, nil)
		BottomUp(src, dst, m, 0.5)

		// BottomUp might map the root as a fallback, but leaves shouldn't map.
		if m.Has(srcLeaf) {
			t.Errorf("expected completely different leaves to not be mapped")
		}
		if m.HasDst(dstLeaf) {
			t.Errorf("expected completely different leaves to not be mapped")
		}
	})
}

func TestTopDownUncomputedHashes(t *testing.T) {
	srcLeaf1 := testutil.Leaf("id", "x")
	srcLeaf2 := testutil.Leaf("id", "y")
	src := testutil.Node("func", "main",
		testutil.Node("block", "", srcLeaf1, srcLeaf2),
	)

	dstLeaf1 := testutil.Leaf("id", "x")
	dstLeaf2 := testutil.Leaf("id", "y")
	dst := testutil.Node("func", "main",
		testutil.Node("block", "", dstLeaf1, dstLeaf2),
	)

	// Reset hashes to 0 to make sure TopDown lazily computes missing hashes.
	for _, n := range src.PreOrder() {
		n.Hash = 0
	}
	for _, n := range dst.PreOrder() {
		n.Hash = 0
	}

	m := NewMapping()
	TopDown(src, dst, 1, m, nil)

	if !m.Has(srcLeaf1) || !m.Has(srcLeaf2) {
		t.Errorf("expected all isomorphic leaves to be mapped even when initial Hash is 0")
	}
}

func TestFindCandidatesWithCommonDescendants(t *testing.T) {
	d1 := testutil.Leaf("id", "x")
	t1 := testutil.Node("block", "", d1)

	d2 := testutil.Leaf("id", "x")
	t2 := testutil.Node("block", "", d2)

	m := NewMapping()
	m.Add(d1, d2)

	candidates := findCandidatesWithCommonDescendants(t1, m)
	if len(candidates) != 1 || candidates[0] != t2 {
		t.Errorf("findCandidatesWithCommonDescendants() = %v, want [%v]", candidates, t2)
	}
}
