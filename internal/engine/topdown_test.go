package engine

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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

func TestTopDown_ParentLeafSimilarityTieBreaker(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Two if-statements under the same function share identical `{ continue }` bodies.
		// With 0 mapped descendants and matching lineage, parent leaf similarity breaks
		// the tie in favor of the modified condition over the newly inserted one.
		srcCond := testutil.Node("unary_expression", "",
			testutil.Node("call_expression", "",
				testutil.Node("identifier", "shouldDemoteMove"),
				testutil.Node("argument_list", "",
					testutil.Node("identifier", "dstNode"),
					testutil.Node("identifier", "ms"),
					testutil.Node("identifier", "r"),
				),
			),
		)
		srcBody := testutil.Node("block", "",
			testutil.Node("continue_statement", "", testutil.Node("continue", "continue")),
		)
		srcIf := testutil.Node("if_statement", "", testutil.Node("if", "if"), srcCond, srcBody)
		srcIf.StartByte = 100
		srcRoot := testutil.Node("function_declaration", "",
			testutil.Node("identifier", "normalizeMovesByStructure"),
			testutil.Node("block", "", srcIf),
		)
		srcRoot.Language = "go"

		dstEarlyCond := testutil.Node("short_var_declaration", "",
			testutil.Node("identifier", "already"),
			testutil.Node("identifier", "toDemote"),
		)
		dstEarlyBody := testutil.Node("block", "",
			testutil.Node("continue_statement", "", testutil.Node("continue", "continue")),
		)
		dstEarlyIf := testutil.Node("if_statement", "", testutil.Node("if", "if"), dstEarlyCond, dstEarlyBody)
		dstEarlyIf.StartByte = 50

		dstLateCond := testutil.Node("unary_expression", "",
			testutil.Node("call_expression", "",
				testutil.Node("identifier", "shouldDemoteMove"),
				testutil.Node("argument_list", "",
					testutil.Node("identifier", "dstNode"),
					testutil.Node("identifier", "ms"),
					testutil.Node("identifier", "r"),
					testutil.Node("identifier", "evicted"),
				),
			),
		)
		dstLateBody := testutil.Node("block", "",
			testutil.Node("continue_statement", "", testutil.Node("continue", "continue")),
		)
		dstLateIf := testutil.Node("if_statement", "", testutil.Node("if", "if"), dstLateCond, dstLateBody)
		dstLateIf.StartByte = 150

		dstRoot := testutil.Node("function_declaration", "",
			testutil.Node("identifier", "normalizeMovesByStructure"),
			testutil.Node("block", "", dstEarlyIf, dstLateIf),
		)
		dstRoot.Language = "go"

		srcRoot.ComputeHashes()
		dstRoot.ComputeHashes()

		m := NewMapping()
		TopDown(srcRoot, dstRoot, 2, m, nil)

		if m.Src()[srcBody] != dstLateBody {
			t.Fatalf("TopDown matched srcBody to %v, want dstLateBody (%v)", m.Src()[srcBody], dstLateBody)
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func process(items []int) {
	for _, x := range items {
		if shouldDemoteMove(x, dstNode, ms, r) {
			continue
		}
	}
}
`
		dst := `package main

func process(items []int) {
	for _, x := range items {
		if already := toDemote[x]; already {
			continue
		}
		if shouldDemoteMove(x, dstNode, ms, r, evicted) {
			continue
		}
	}
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		var srcIf, srcBody *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "if_statement" {
				srcIf = n
				for _, c := range n.Children {
					if c.Type == "block" {
						srcBody = c
					}
				}
			}
		}

		var dstEarlyIf, dstLateIf, dstLateBody *treesitter.ASTNode
		for _, n := range dstAST.Descendants() {
			if n.Type == "if_statement" {
				if dstEarlyIf == nil {
					dstEarlyIf = n
				} else {
					dstLateIf = n
					for _, c := range n.Children {
						if c.Type == "block" {
							dstLateBody = c
						}
					}
				}
			}
		}

		if srcIf == nil || srcBody == nil || dstEarlyIf == nil || dstLateIf == nil || dstLateBody == nil {
			t.Fatalf("failed to locate AST nodes: srcIf=%v srcBody=%v dstEarlyIf=%v dstLateIf=%v dstLateBody=%v",
				srcIf, srcBody, dstEarlyIf, dstLateIf, dstLateBody)
		}

		if matched := mr.Mappings.Src()[srcBody]; matched != dstLateBody {
			t.Fatalf("expected srcBody to match dstLateBody (%v), got %v", dstLateBody, matched)
		}
		if matched := mr.Mappings.Src()[srcIf]; matched != dstLateIf {
			t.Fatalf("expected srcIf to match dstLateIf (%v), got %v", dstLateIf, matched)
		}

		// Whole-snippet verification: func process and for loop map in place, early if is cleanly unmapped
		var srcFunc, dstFunc, srcFor, dstFor *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			switch n.Type {
			case "function_declaration":
				srcFunc = n
			case "for_statement":
				srcFor = n
			}
		}
		for _, n := range dstAST.Descendants() {
			switch n.Type {
			case "function_declaration":
				dstFunc = n
			case "for_statement":
				dstFor = n
			}
		}

		if srcFunc == nil || dstFunc == nil || srcFor == nil || dstFor == nil {
			t.Fatalf("missing nodes: srcFunc=%v, dstFunc=%v, srcFor=%v, dstFor=%v", srcFunc, dstFunc, srcFor, dstFor)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected func process to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}
		if mr.Mappings.Src()[srcFor] != dstFor {
			t.Errorf("expected for_statement to map in place, got %v", mr.Mappings.Src()[srcFor])
		}
		if mr.Mappings.Dst()[dstEarlyIf] != nil {
			t.Errorf("expected newly inserted early if to remain unmapped, got %v", mr.Mappings.Dst()[dstEarlyIf])
		}

		// Verify continue statement in srcIf maps to continue in dstLateIf
		var srcCont, dstCont *treesitter.ASTNode
		for _, n := range srcIf.Descendants() {
			if n.Type == "continue_statement" {
				srcCont = n
				break
			}
		}
		for _, n := range dstLateIf.Descendants() {
			if n.Type == "continue_statement" {
				dstCont = n
				break
			}
		}
		if srcCont == nil || dstCont == nil {
			t.Fatalf("missing continue statements: srcCont=%v, dstCont=%v", srcCont, dstCont)
		}
		if mr.Mappings.Src()[srcCont] != dstCont {
			t.Errorf("expected continue statement to map in place, got %v", mr.Mappings.Src()[srcCont])
		}
	})
}

func TestTopDown_DelimitedContainerBoundaryGuard(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Source collection has two structs: S1 ("staging", SSL: true) and S2 ("backup", SSL: false).
		s1Host := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "Host"),
			testutil.Node("interpreted_string_literal", `"staging.internal"`),
		)
		s1SSL := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "SSL"),
			testutil.Node("true", "true"),
		)
		s1Elem := testutil.Node("literal_value", "", s1Host, s1SSL)

		s2Host := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "Host"),
			testutil.Node("interpreted_string_literal", `"backup.internal"`),
		)
		s2SSL := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "SSL"),
			testutil.Node("false", "false"),
		)
		s2Elem := testutil.Node("literal_value", "", s2Host, s2SSL)

		outer1 := testutil.Node("literal_value", "", s1Elem, s2Elem)

		// Destination collection has only D1 ("backup", SSL: true).
		d1Host := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "Host"),
			testutil.Node("interpreted_string_literal", `"backup.internal"`),
		)
		d1SSL := testutil.Node("keyed_element", "",
			testutil.Node("identifier", "SSL"),
			testutil.Node("true", "true"),
		)
		d1Elem := testutil.Node("literal_value", "", d1Host, d1SSL)

		outer2 := testutil.Node("literal_value", "", d1Elem)

		root1 := testutil.Node("source_file", "", outer1)
		root1.Language = "go"
		root2 := testutil.Node("source_file", "", outer2)
		root2.Language = "go"

		r := rulesFor(root1)

		m := NewMapping()

		if s1Pairs := collectContainerPairs(s1Elem, r, nil); len(s1Pairs) != 2 {
			t.Errorf("expected 2 pairs for s1Elem, got %d", len(s1Pairs))
		}

		// S1's SSL: true matches D1's SSL: true, but S2 shares the "backup.internal" Host key with D1.
		// S1 must not greedily steal D1 when S2 is a superior match.
		if !hasDelimitedContainerConflict(s1SSL, d1SSL, m, r) {
			t.Errorf("expected conflict between s1 (staging) and d1 (backup) due to unextinguished peer s2 (backup)")
		}

		// S2's SSL: false should still be allowed to match D1 because they share the Host key.
		if hasDelimitedContainerConflict(s2SSL, d1SSL, m, r) {
			t.Errorf("expected no conflict between s2 (backup) and d1 (backup)")
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		runCase := func(t *testing.T, name string, src, dst []byte, stagingRow, backupRowSrc, backupRowDst uint32) {
			t.Run(name, func(t *testing.T) {
				oldTree, err := treesitter.Parse(src, "old.go")
				if err != nil {
					t.Fatal(err)
				}
				newTree, err := treesitter.Parse(dst, "new.go")
				if err != nil {
					t.Fatal(err)
				}
				r := rulesFor(oldTree)

				part := NewLinePartition(src, dst)
				mr := Match(oldTree, newTree, src, dst, part)
				m := mr.Mappings

				// Enclosing declarations must stay anchored in place.
				var oldServerDecl, newServerDecl *treesitter.ASTNode
				var oldVarDecl, newVarDecl *treesitter.ASTNode
				for _, n := range oldTree.PreOrder() {
					if r.IsDeclaration(n.Type) {
						switch n.StartRow {
						case 2:
							oldServerDecl = n
						case 8:
							oldVarDecl = n
						}
					}
				}
				for _, n := range newTree.PreOrder() {
					if r.IsDeclaration(n.Type) {
						switch n.StartRow {
						case 2:
							newServerDecl = n
						case 8:
							newVarDecl = n
						}
					}
				}
				if oldServerDecl == nil || newServerDecl == nil || m.Src()[oldServerDecl] != newServerDecl {
					t.Errorf("expected Server struct declaration to map in place, got %v", m.Src()[oldServerDecl])
				}
				if oldVarDecl == nil || newVarDecl == nil || m.Src()[oldVarDecl] != newVarDecl {
					t.Errorf("expected var servers declaration to map in place, got %v", m.Src()[oldVarDecl])
				}

				// Unchanged earlier elements must stay mapped in place.
				var oldPrimary, newPrimary *treesitter.ASTNode
				for _, n := range oldTree.PreOrder() {
					if r.IsPair(n.Type) && n.StartRow == 9 && len(n.Children) >= 2 && getKeyLabel(n) == "Host" {
						oldPrimary = n
						break
					}
				}
				for _, n := range newTree.PreOrder() {
					if r.IsPair(n.Type) && n.StartRow == 9 && len(n.Children) >= 2 && getKeyLabel(n) == "Host" {
						newPrimary = n
						break
					}
				}
				if oldPrimary == nil || newPrimary == nil || m.Src()[oldPrimary] != newPrimary {
					t.Errorf("expected primary.internal to map in place, got %v", m.Src()[oldPrimary])
				}

				// Staging was deleted; it must not map into the updated backup element.
				for n1, n2 := range m.Src() {
					if n1.StartRow == stagingRow && n2.StartRow == backupRowDst {
						t.Errorf("expected deleted staging element at row %d to not map to new backup element at row %d (node %s -> %s)",
							n1.StartRow, n2.StartRow, n1.Type, n2.Type)
					}
				}

				// Stationary fields inside the updated element should map in place.
				var oldBackupHost, newBackupHost *treesitter.ASTNode
				var oldBackupPort, newBackupPort *treesitter.ASTNode
				for _, n := range oldTree.PreOrder() {
					if r.IsPair(n.Type) && n.StartRow == backupRowSrc && len(n.Children) >= 2 {
						if getKeyLabel(n) == "Host" {
							oldBackupHost = n
						} else if getKeyLabel(n) == "Port" {
							oldBackupPort = n
						}
					}
				}
				for _, n := range newTree.PreOrder() {
					if r.IsPair(n.Type) && n.StartRow == backupRowDst && len(n.Children) >= 2 {
						if getKeyLabel(n) == "Host" {
							newBackupHost = n
						} else if getKeyLabel(n) == "Port" {
							newBackupPort = n
						}
					}
				}
				if oldBackupHost == nil || newBackupHost == nil || m.Src()[oldBackupHost] != newBackupHost {
					t.Errorf("expected backup Host pair to map in place, got %v", m.Src()[oldBackupHost])
				}
				if oldBackupPort == nil || newBackupPort == nil || m.Src()[oldBackupPort] != newBackupPort {
					t.Errorf("expected backup Port pair to map in place, got %v", m.Src()[oldBackupPort])
				}
			})
		}

		// Case 1: Updated element appears before deleted element in source.
		srcStd := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: false},
	{Host: "staging.internal", Port: 9090, SSL: true},
}
`)
		dstStd := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: true},
}
`)
		runCase(t, "StandardOrder_DeletedAfterUpdated", srcStd, dstStd, 11, 10, 10)

		// Case 2: Deleted element appears first; top-down matching must not greedily claim the updated element.
		srcInv := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "staging.internal", Port: 9090, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: false},
}
`)
		dstInv := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: true},
}
`)
		runCase(t, "InvertedOrder_DeletedBeforeUpdated", srcInv, dstInv, 10, 11, 10)
	})
}

// Sibling statements and functions with similar shapes shouldn't invert their matches
// across container declarations or steal statements from sibling scopes.
func TestTopDown_SiblingStatementMonotonicity(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`fn f1() void {}

fn f2() void {
    x += 1;
}
`)
		newSrc := []byte(`fn f1() void {
    foo();
    x += 1;
}

fn f2() void {
    foo();
    x += 1;
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "old.zig")
		if err != nil {
			t.Fatalf("parse old.zig: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "new.zig")
		if err != nil {
			t.Fatalf("parse new.zig: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		r := rulesFor(oldTree)
		if r == nil {
			t.Fatal("rulesFor(oldTree) returned nil")
		}

		// Locate function declarations and blocks in both trees.
		var oldF1, oldF2, oldF1Block, oldF2Block, oldXStmt *treesitter.ASTNode
		for _, n := range oldTree.Children {
			if r.IsContainerDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Type == "identifier" && c.Label == "f1" {
						oldF1 = n
						for _, bc := range n.Children {
							if r.IsBlock(bc.Type) {
								oldF1Block = bc
							}
						}
					}
					if c.Type == "identifier" && c.Label == "f2" {
						oldF2 = n
						for _, bc := range n.Children {
							if r.IsBlock(bc.Type) {
								oldF2Block = bc
								for _, stmt := range bc.Children {
									if r.IsStatement(stmt.Type) {
										oldXStmt = stmt
									}
								}
							}
						}
					}
				}
			}
		}

		var newF1, newF2, newF1Block, newF2Block *treesitter.ASTNode
		var newFoo1, newX1, newFoo2, newX2 *treesitter.ASTNode
		for _, n := range newTree.Children {
			if r.IsContainerDeclaration(n.Type) {
				for _, c := range n.Children {
					if c.Type == "identifier" && c.Label == "f1" {
						newF1 = n
						for _, bc := range n.Children {
							if r.IsBlock(bc.Type) {
								newF1Block = bc
								for _, stmt := range bc.Children {
									if r.IsStatement(stmt.Type) {
										if newFoo1 == nil {
											newFoo1 = stmt
										} else {
											newX1 = stmt
										}
									}
								}
							}
						}
					}
					if c.Type == "identifier" && c.Label == "f2" {
						newF2 = n
						for _, bc := range n.Children {
							if r.IsBlock(bc.Type) {
								newF2Block = bc
								for _, stmt := range bc.Children {
									if r.IsStatement(stmt.Type) {
										if newFoo2 == nil {
											newFoo2 = stmt
										} else {
											newX2 = stmt
										}
									}
								}
							}
						}
					}
				}
			}
		}

		if oldF1 == nil || oldF2 == nil || oldXStmt == nil {
			t.Fatalf("failed to locate old AST nodes: f1=%v, f2=%v, xStmt=%v", oldF1, oldF2, oldXStmt)
		}
		if newF1 == nil || newF2 == nil || newFoo1 == nil || newX1 == nil || newFoo2 == nil || newX2 == nil {
			t.Fatalf("failed to locate new AST nodes: f1=%v, f2=%v, foo1=%v, x1=%v, foo2=%v, x2=%v",
				newF1, newF2, newFoo1, newX1, newFoo2, newX2)
		}

		// 1. Enclosing context: functions map in place
		if got := mr.Mappings.Src()[oldF1]; got != newF1 {
			t.Errorf("expected old f1 to map to new f1, got %v", got)
		}
		if got := mr.Mappings.Src()[oldF2]; got != newF2 {
			t.Errorf("expected old f2 to map to new f2, got %v", got)
		}

		// 2. Surrounding sequence: blocks map in place
		if oldF1Block != nil && newF1Block != nil {
			if got := mr.Mappings.Src()[oldF1Block]; got != newF1Block {
				t.Errorf("expected old f1 block to map to new f1 block, got %v", got)
			}
		}
		if oldF2Block != nil && newF2Block != nil {
			if got := mr.Mappings.Src()[oldF2Block]; got != newF2Block {
				t.Errorf("expected old f2 block to map to new f2 block, got %v", got)
			}
		}

		// 3. Stationary target: x += 1 inside f2 maps monotonically in place to f2
		if got := mr.Mappings.Src()[oldXStmt]; got != newX2 {
			t.Errorf("expected old f2 statement to map in place to new f2 statement (%v), got %v", newX2, got)
		}

		// 4. Non-theft assertion: newly inserted statements in f1 are unmapped
		if got := mr.Mappings.Dst()[newX1]; got != nil {
			t.Errorf("expected new f1 statement to be unmapped (not stolen from f2), got %v", got)
		}
		if got := mr.Mappings.Dst()[newFoo1]; got != nil {
			t.Errorf("expected new f1 foo call to be unmapped, got %v", got)
		}
		if got := mr.Mappings.Dst()[newFoo2]; got != nil {
			t.Errorf("expected new f2 foo call to be unmapped, got %v", got)
		}
	})
}

func TestTopDown_ImmediateStatementAffinity(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Synthetic AST: two adjacent declarations sharing an identical literal value (false).
		// Inverted order in dst: flagB comes first, flagA comes second.
		// Statement affinity must anchor each literal to its own declaration without being tricked
		// by physical document order.
		idSrcA := testutil.Node("identifier", "flagA")
		litSrcA := testutil.Leaf("false", "false")
		declSrcA := testutil.Node("short_var_declaration", "", idSrcA, litSrcA)

		idSrcB := testutil.Node("identifier", "flagB")
		litSrcB := testutil.Leaf("false", "false")
		declSrcB := testutil.Node("short_var_declaration", "", idSrcB, litSrcB)

		blockSrc := testutil.Node("block", "", declSrcA, declSrcB)
		fnSrc := testutil.Node("function_declaration", "", testutil.Leaf("f", "f"), blockSrc)
		rootSrc := testutil.Node("source_file", "", fnSrc)
		rootSrc.Language = "go"
		treesitter.EnsureIndex(rootSrc)

		// In destination: flagB first, flagA second
		idDstB := testutil.Node("identifier", "flagB")
		litDstB := testutil.Leaf("false", "false")
		declDstB := testutil.Node("short_var_declaration", "", idDstB, litDstB)

		idDstA := testutil.Node("identifier", "flagA")
		litDstA := testutil.Leaf("false", "false")
		declDstA := testutil.Node("short_var_declaration", "", idDstA, litDstA)

		blockDst := testutil.Node("block", "", declDstB, declDstA)
		fnDst := testutil.Node("function_declaration", "", testutil.Leaf("f", "f"), blockDst)
		rootDst := testutil.Node("source_file", "", fnDst)
		rootDst.Language = "go"
		treesitter.EnsureIndex(rootDst)

		m := NewMapping()
		TopDown(rootSrc, rootDst, 1, m, nil)

		if got := m.Get(litSrcA); got != litDstA {
			t.Errorf("flagA literal 'false' mapped to %v, want flagA literal 'false' %v", got, litDstA)
		}
		if got := m.Get(litSrcB); got != litDstB {
			t.Errorf("flagB literal 'false' mapped to %v, want flagB literal 'false' %v", got, litDstB)
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package main

func f() {
	{
		iter := slice("1", "b")
		expect(1, iter.get(0))
		expectStrings("b", iter.get(1))
	}
}
`)

		dst := []byte(`package main

func f() {
	{
		iter := slice("1", "2")
		expect(1, iter.get(0))
		expect(2, iter.get(1))
	}

	{
		iter := slice("1", "2", "b")
		expect(1, iter.get(0))
		expect(2, iter.get(1))
		expectStrings("b", iter.get(2))
	}
}
`)

		oldTree, err := treesitter.Parse(src, "old.go")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(dst, "new.go")
		if err != nil {
			t.Fatal(err)
		}

		r := rulesFor(oldTree)
		part := NewLinePartition(src, dst)
		mr := Match(oldTree, newTree, src, dst, part)
		m := mr.Mappings

		// 1. Enclosing Context: Function declaration must map in place.
		var oldFn, newFn *treesitter.ASTNode
		for _, n := range oldTree.PreOrder() {
			if r.IsDeclaration(n.Type) {
				oldFn = n
				break
			}
		}
		for _, n := range newTree.PreOrder() {
			if r.IsDeclaration(n.Type) {
				newFn = n
				break
			}
		}
		if oldFn == nil || newFn == nil || m.Src()[oldFn] != newFn {
			t.Fatalf("expected function f to map in place, got %v", m.Src()[oldFn])
		}

		// Locate inner blocks in old and new trees.
		// oldTree has 1 inner block; newTree has 2 inner blocks.
		var oldBlock *treesitter.ASTNode
		for _, n := range oldTree.PreOrder() {
			if r.IsBlock(n.Type) && n.Parent != nil && r.IsBlock(n.Parent.Type) {
				oldBlock = n
				break
			}
		}
		var newBlock1, newBlock2 *treesitter.ASTNode
		for _, n := range newTree.PreOrder() {
			if r.IsBlock(n.Type) && n.Parent != nil && r.IsBlock(n.Parent.Type) {
				if newBlock1 == nil {
					newBlock1 = n
				} else if newBlock2 == nil {
					newBlock2 = n
					break
				}
			}
		}
		if oldBlock == nil || newBlock1 == nil || newBlock2 == nil {
			t.Fatalf("failed to locate inner blocks: old=%v new1=%v new2=%v", oldBlock, newBlock1, newBlock2)
		}

		// 2. Target & Containers:
		// Newly prepended block 1 must NOT steal oldBlock. oldBlock must map to newBlock2 where "b" survives.
		if m.Src()[oldBlock] == newBlock1 {
			t.Errorf("oldBlock erroneously mapped to newly prepended newBlock1")
		}
		if m.Src()[oldBlock] != newBlock2 {
			t.Errorf("expected oldBlock to map to newBlock2, got %v", m.Src()[oldBlock])
		}

		// 3. Surrounding Sequence & Target Statement:
		// In diffm_main, expectStrings in oldBlock was stolen by newBlock1's expect(2, iter.get(1)).
		// In diffm_branch, expectStrings is correctly anchored inside newBlock2.
		var oldExpectStrings *treesitter.ASTNode
		for _, n := range oldBlock.PreOrder() {
			if r.IsCall(n.Type) && bytes.Contains(src[n.StartByte:n.EndByte], []byte("expectStrings")) {
				oldExpectStrings = n
				break
			}
		}
		if oldExpectStrings == nil {
			t.Fatalf("failed to locate expectStrings call in oldBlock: %v", oldExpectStrings)
		}
		mappedCall := m.Src()[oldExpectStrings]
		if mappedCall == nil {
			t.Fatalf("expected expectStrings in oldBlock to be mapped")
		}
		if newBlock1.Contains(mappedCall) {
			t.Errorf("expectStrings was erroneously stolen by newly prepended newBlock1 (diffm_main bug)")
		}
		if !newBlock2.Contains(mappedCall) {
			t.Errorf("expected expectStrings to be mapped inside newBlock2, got %v", mappedCall)
		}

		// 4. Stationary Operands:
		// expect(1, iter.get(0)) in oldBlock must stay mapped to newBlock2's expect(1, iter.get(0)).
		var oldExpect1, newExpect1InBlock2 *treesitter.ASTNode
		for _, n := range oldBlock.PreOrder() {
			if r.IsCall(n.Type) && bytes.Contains(src[n.StartByte:n.EndByte], []byte("expect(1")) {
				oldExpect1 = n
				break
			}
		}
		for _, n := range newBlock2.PreOrder() {
			if r.IsCall(n.Type) && bytes.Contains(dst[n.StartByte:n.EndByte], []byte("expect(1")) {
				newExpect1InBlock2 = n
				break
			}
		}
		if oldExpect1 == nil || newExpect1InBlock2 == nil {
			t.Fatalf("failed to locate expect(1) calls: old=%v new=%v", oldExpect1, newExpect1InBlock2)
		}
		if m.Src()[oldExpect1] != newExpect1InBlock2 {
			t.Errorf("expected expect(1) to stay mapped in-place to newBlock2, got %v", m.Src()[oldExpect1])
		}
	})
}
