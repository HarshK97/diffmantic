package engine

import (
	"fmt"
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

func TestTopDown_AmbiguityPreorderTieBreaker(t *testing.T) {
	// If dst has duplicate candidate subtrees at different lines, match the stationary
	// row 20 candidate first, even if traversal opened row 50 earlier.
	srcLeaf := testutil.NodeAtRC("identifier", "score", 20, 5)
	srcExpr := testutil.Tree(testutil.NodeAtRC("expression_list", "", 20, 5), srcLeaf)
	srcRoot := testutil.Node("function_declaration", "myFunc", srcExpr)

	dstLeafFar := testutil.NodeAtRC("identifier", "score", 50, 5)
	dstExprFar := testutil.Tree(testutil.NodeAtRC("expression_list", "", 50, 5), dstLeafFar)

	dstLeafClose := testutil.NodeAtRC("identifier", "score", 20, 5)
	dstExprClose := testutil.Tree(testutil.NodeAtRC("expression_list", "", 20, 5), dstLeafClose)

	// Put the farther node first so traversal would visit it first without preorder sorting.
	dstRoot := testutil.Node("function_declaration", "myFunc", dstExprFar, dstExprClose)

	m := NewMapping()
	TopDown(srcRoot, dstRoot, 2, m, nil)

	if m.Src()[srcExpr] != dstExprClose {
		t.Errorf("TopDown matched srcExpr to %v, want dstExprClose at row 20", m.Src()[srcExpr])
	}
}

func TestTopDown_IdentifierlessScopeIsolation(t *testing.T) {
	t.Run("rejects identifier-less subtree across different function scopes", func(t *testing.T) {
		srcExit := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("false", "false")),
			),
		)
		srcDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcA"),
			testutil.Node("block", "", srcExit),
		)
		srcDecl.Language = "go"

		dstExit := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("false", "false")),
			),
		)
		dstDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcB"),
			testutil.Node("block", "", dstExit),
		)
		dstDecl.Language = "go"

		m := NewMapping()
		TopDown(srcDecl, dstDecl, 2, m, nil)

		if m.Has(srcExit) {
			t.Errorf("expected identifier-less subtree not to match across funcA and funcB, got %v", m.Src()[srcExit])
		}
	})

	t.Run("matches identifier-less subtree within same function scope", func(t *testing.T) {
		srcExit := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("false", "false")),
			),
		)
		srcDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcA"),
			testutil.Node("block", "", srcExit),
		)
		srcDecl.Language = "go"

		dstExit := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("false", "false")),
			),
		)
		dstDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcA"),
			testutil.Node("block", "", dstExit),
		)
		dstDecl.Language = "go"

		m := NewMapping()
		TopDown(srcDecl, dstDecl, 2, m, nil)

		if m.Src()[srcExit] != dstExit {
			t.Errorf("expected identifier-less subtree to match within same function scope funcA, got %v", m.Src()[srcExit])
		}
	})

	t.Run("matches subtree containing identifier across different function scopes", func(t *testing.T) {
		srcSub := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("identifier", "err")),
			),
		)
		srcDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcA"),
			testutil.Node("block", "", srcSub),
		)
		srcDecl.Language = "go"

		dstSub := testutil.Node("block", "",
			testutil.Node("return_statement", "",
				testutil.Node("expression_list", "", testutil.Leaf("identifier", "err")),
			),
		)
		dstDecl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "funcB"),
			testutil.Node("block", "", dstSub),
		)
		dstDecl.Language = "go"

		m := NewMapping()
		TopDown(srcDecl, dstDecl, 2, m, nil)

		if m.Src()[srcSub] != dstSub {
			t.Errorf("expected subtree with identifier to match across funcA and funcB, got %v", m.Src()[srcSub])
		}
	})
}

func TestTopDown_DirectParentPrecedenceOverFallback(t *testing.T) {
	// Candidate A has a direct parent match, while Candidate B only matches via
	// a high-scoring outer loop. Make sure TopDown picks A over B.
	srcLeafA := testutil.NodeAtRC("identifier", "sim", 10, 5)
	srcExprA := testutil.Node("expression_list", "", srcLeafA)
	srcExprA.StartRow = 10
	srcExprA.StartCol = 5
	srcParentA := testutil.Node("short_var_declaration", "", srcExprA)
	srcParentA.StartRow = 10
	srcParentA.StartCol = 1

	srcLeafB := testutil.NodeAtRC("identifier", "sim", 20, 5)
	srcExprB := testutil.Node("expression_list", "", srcLeafB)
	srcExprB.StartRow = 20
	srcExprB.StartCol = 5
	srcParentB := testutil.Node("assignment_statement", "", srcExprB)
	srcParentB.StartRow = 20
	srcParentB.StartCol = 1

	srcFnName := testutil.NodeAtRC("identifier", "myFunc", 1, 1)
	srcForLoop := testutil.Node("for_statement", "", srcParentA, srcParentB)
	srcBlock := testutil.Node("block", "", srcForLoop)
	srcRoot := testutil.Node("function_declaration", "", srcFnName, srcBlock)
	srcRoot.Language = "go"

	dstLeaf := testutil.NodeAtRC("identifier", "sim", 10, 5)
	dstExpr := testutil.Node("expression_list", "", dstLeaf)
	dstExpr.StartRow = 10
	dstExpr.StartCol = 5
	dstParent := testutil.Node("short_var_declaration", "", dstExpr)
	dstParent.StartRow = 10
	dstParent.StartCol = 1

	dstFnName := testutil.NodeAtRC("identifier", "myFunc", 1, 1)
	dstForLoop := testutil.Node("for_statement", "", dstParent)
	dstBlock := testutil.Node("block", "", dstForLoop)
	dstRoot := testutil.Node("function_declaration", "", dstFnName, dstBlock)
	dstRoot.Language = "go"

	m := NewMapping()
	m.Add(srcFnName, dstFnName)
	// Give parentA and dstParent a shared mapped node so direct parent Dice is non-zero.
	extraSrc := testutil.NodeAtRC("identifier", "common", 10, 20)
	extraDst := testutil.NodeAtRC("identifier", "common", 10, 20)
	srcParentA.Children = append(srcParentA.Children, extraSrc)
	extraSrc.Parent = srcParentA
	dstParent.Children = append(dstParent.Children, extraDst)
	extraDst.Parent = dstParent
	m.Add(extraSrc, extraDst)

	// Pad parentA with unmapped nodes to keep its direct Dice low (~0.16).
	for i := range 5 {
		u1 := testutil.NodeAtRC("identifier", fmt.Sprintf("u1_%d", i), 10, uint32(30+i))
		srcParentA.Children = append(srcParentA.Children, u1)
		u1.Parent = srcParentA

		u2 := testutil.NodeAtRC("identifier", fmt.Sprintf("u2_%d", i), 10, uint32(30+i))
		dstParent.Children = append(dstParent.Children, u2)
		u2.Parent = dstParent
	}

	// Stuff the loop with mapped children so the ancestor fallback (Dice * 0.5 ≈ 0.45) easily beats parentA's direct score.
	for i := range 15 {
		r1 := testutil.NodeAtRC("identifier", fmt.Sprintf("loop_%d", i), uint32(50+i), 1)
		r2 := testutil.NodeAtRC("identifier", fmt.Sprintf("loop_%d", i), uint32(50+i), 1)
		srcForLoop.Children = append(srcForLoop.Children, r1)
		r1.Parent = srcForLoop
		dstForLoop.Children = append(dstForLoop.Children, r2)
		r2.Parent = dstForLoop
		m.Add(r1, r2)
	}

	srcRoot.ComputeHashes()
	dstRoot.ComputeHashes()

	TopDown(srcRoot, dstRoot, 2, m, nil)

	if m.Src()[srcExprA] != dstExpr {
		t.Errorf("TopDown matched srcExprA to %v, want dstExpr", m.Src()[srcExprA])
	}
	if m.Src()[srcExprB] != nil {
		t.Errorf("TopDown matched srcExprB to %v, want nil", m.Src()[srcExprB])
	}
}
