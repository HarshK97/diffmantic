package engine

import (
	"bytes"
	"os"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func parseSnippet(t *testing.T, code, filename string) *treesitter.ASTNode {
	t.Helper()
	ast, err := treesitter.Parse([]byte(code), filename)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	return ast
}

func findNodeByType(root *treesitter.ASTNode, nodeType string) *treesitter.ASTNode {
	for _, n := range root.PreOrder() {
		if n.Type == nodeType {
			return n
		}
	}
	return nil
}

func findAllNodesByType(root *treesitter.ASTNode, nodeType string) []*treesitter.ASTNode {
	var result []*treesitter.ASTNode
	for _, n := range root.PreOrder() {
		if n.Type == nodeType {
			result = append(result, n)
		}
	}
	return result
}

func TestMatchIdenticalTrees(t *testing.T) {
	// Identical trees map all nodes without structural changes.
	code := "package main\n\nfunc main() {\n\tx := 1\n\ty := 2\n}\n"
	src := parseSnippet(t, code, "test.go")
	dst := parseSnippet(t, code, "test.go")

	r := Match(src, dst, []byte(code), []byte(code), nil)
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
	srcCode := "package main\n\nvar x = 1\n"
	dstCode := "package main\n\nvar y = 1\n"
	src := parseSnippet(t, srcCode, "test.go")
	dst := parseSnippet(t, dstCode, "test.go")

	r := Match(src, dst, []byte(srcCode), []byte(dstCode), nil)
	if r == nil {
		t.Fatal("Match returned nil")
	}
	if !r.Mappings.Has(src) {
		t.Error("root should be mapped")
	}
}

func TestMatchRootAlwaysMapped(t *testing.T) {
	// Match always maps root nodes, even for different trees.
	srcCode := "package main\n\nfunc a() int {\n\treturn 1\n}\n"
	dstCode := "package main\n\nfunc b() string {\n\treturn \"hello\"\n}\n"
	src := parseSnippet(t, srcCode, "test.go")
	dst := parseSnippet(t, dstCode, "test.go")

	r := Match(src, dst, []byte(srcCode), []byte(dstCode), nil)
	if !r.Mappings.Has(src) {
		t.Error("src root should always be mapped")
	}
	if !r.Mappings.HasDst(dst) {
		t.Error("dst root should always be mapped")
	}
}

func TestMatchSingleLeaves(t *testing.T) {
	code := "x"
	tree1 := parseSnippet(t, code, "test.py")
	tree2 := parseSnippet(t, code, "test.py")
	src := tree1.Leaves()[0]
	dst := tree2.Leaves()[0]
	r := Match(src, dst, []byte(code), []byte(code), nil)
	if !r.Mappings.Has(src) || r.Mappings.Src()[src] != dst {
		t.Error("single identical leaves should be mapped")
	}
}

func TestMatchPairsPreOrder(t *testing.T) {
	// Mappings.Pairs follows pre-order traversal.
	code := "package main\n\nfunc main() {\n\tx := 1\n\ty := 2\n}\n"
	src := parseSnippet(t, code, "test.go")
	dst := parseSnippet(t, code, "test.go")

	r := Match(src, dst, []byte(code), []byte(code), nil)
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
	code := "package main\n\nfunc main() {\n\tf()\n}\n"
	src := parseSnippet(t, code, "test.go")
	dst := parseSnippet(t, code, "test.go")

	m := NewMapping()
	part := NewLinePartition([]byte(code), []byte(code))
	TopDown(src, dst, 2, m, part)

	r := rules.Get("go")
	var srcCall *treesitter.ASTNode
	for _, n := range src.PreOrder() {
		if r.IsCall(n.Type) {
			srcCall = n
			break
		}
	}
	if srcCall == nil {
		t.Fatal("failed to find call node in src")
	}
	if !m.Has(srcCall) {
		t.Error("unambiguous isomorphic subtree should be mapped by TopDown")
	}
}

func TestBottomUpWithPriorMapping(t *testing.T) {
	// BottomUp maps parents of mapped children.
	srcCode := "package main\n\nfunc foo() {\n\tx := 1\n}\n"
	dstCode := "package main\n\nfunc bar() {\n\tx := 1\n}\n"
	srcRoot := parseSnippet(t, srcCode, "test.go")
	dstRoot := parseSnippet(t, dstCode, "test.go")

	r := rules.Get("go")
	var srcBlock, dstBlock *treesitter.ASTNode
	for _, n := range srcRoot.PreOrder() {
		if r.IsBlock(n.Type) {
			srcBlock = n
			break
		}
	}
	for _, n := range dstRoot.PreOrder() {
		if r.IsBlock(n.Type) {
			dstBlock = n
			break
		}
	}
	if srcBlock == nil || dstBlock == nil {
		t.Fatalf("failed to find blocks: src=%v dst=%v", srcBlock, dstBlock)
	}

	m := NewMapping()
	srcLeaves := srcBlock.Leaves()
	dstLeaves := dstBlock.Leaves()
	for i := range srcLeaves {
		if i < len(dstLeaves) && srcLeaves[i].Label == dstLeaves[i].Label {
			m.Add(srcLeaves[i], dstLeaves[i])
		}
	}

	BottomUp(srcRoot, dstRoot, m, 0.5)

	if !m.Has(srcBlock) {
		t.Error("BottomUp should match block containing matched leaf")
	}
}

func TestMatchUnmatchedLeaves(t *testing.T) {
	// Two distinct function blocks with identical leaf names ("count").
	srcCode := `package main
func fa() {
	count := 1
	_ = count
}
func fb() {
	count := 2
	_ = count
}
`
	srcRoot := parseSnippet(t, srcCode, "test.go")
	dstRoot := parseSnippet(t, srcCode, "test.go")

	var srcFuncs, dstFuncs []*treesitter.ASTNode
	for _, n := range srcRoot.Children {
		if n.Type == "function_declaration" {
			srcFuncs = append(srcFuncs, n)
		}
	}
	for _, n := range dstRoot.Children {
		if n.Type == "function_declaration" {
			dstFuncs = append(dstFuncs, n)
		}
	}
	if len(srcFuncs) != 2 || len(dstFuncs) != 2 {
		t.Fatalf("expected 2 funcs each, got %d and %d", len(srcFuncs), len(dstFuncs))
	}

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(srcFuncs[0], dstFuncs[0])
	m.Add(srcFuncs[1], dstFuncs[1])

	MatchUnmatchedLeaves(srcRoot, dstRoot, m, nil)

	for _, l := range srcFuncs[0].Leaves() {
		if m.Has(l) {
			dstL := m.Src()[l]
			if !dstFuncs[0].Contains(dstL) {
				t.Errorf("leaf %v from fa() mapped outside dst fa(): %v", l, dstL)
			}
		}
	}
	for _, l := range srcFuncs[1].Leaves() {
		if m.Has(l) {
			dstL := m.Src()[l]
			if !dstFuncs[1].Contains(dstL) {
				t.Errorf("leaf %v from fb() mapped outside dst fb(): %v", l, dstL)
			}
		}
	}
}

func TestRollupMatchedContainers(t *testing.T) {
	pyCode := "d = {414: \"request_uri_too_large\"}\n"
	srcRoot := parseSnippet(t, pyCode, "test.py")
	dstRoot := parseSnippet(t, pyCode, "test.py")

	dict1 := findNodeByType(srcRoot, "dictionary")
	dict2 := findNodeByType(dstRoot, "dictionary")
	pair1 := findNodeByType(srcRoot, "pair")
	pair2 := findNodeByType(dstRoot, "pair")
	if dict1 == nil || dict2 == nil || pair1 == nil || pair2 == nil {
		t.Fatalf("failed to find dictionary or pair: dict1=%v pair1=%v", dict1, pair1)
	}

	m := NewMapping()
	m.Add(dict1, dict2)
	for i := range pair1.Leaves() {
		if i < len(pair2.Leaves()) {
			m.Add(pair1.Leaves()[i], pair2.Leaves()[i])
		}
	}

	RollupMatchedContainers(dict1, dict2, m)

	if !m.Has(pair1) || m.Src()[pair1] != pair2 {
		t.Errorf("RollupMatchedContainers should pair pair1 -> pair2, got %v", m.Src()[pair1])
	}
}

func TestMatchUnmatchedLeavesUnderUnmatchedContainer(t *testing.T) {
	pyCode := "d = {414: \"request_uri_too_large\"}\n"
	srcRoot := parseSnippet(t, pyCode, "test.py")
	dstRoot := parseSnippet(t, pyCode, "test.py")

	dict1 := findNodeByType(srcRoot, "dictionary")
	dict2 := findNodeByType(dstRoot, "dictionary")
	pair1 := findNodeByType(srcRoot, "pair")
	pair2 := findNodeByType(dstRoot, "pair")
	if dict1 == nil || dict2 == nil || pair1 == nil || pair2 == nil {
		t.Fatalf("failed to find dictionary or pair: dict1=%v pair1=%v", dict1, pair1)
	}

	m := NewMapping()
	m.Add(dict1, dict2)

	MatchUnmatchedLeaves(dict1, dict2, m, nil)

	k1, k2 := pair1.Leaves()[0], pair2.Leaves()[0]
	v1, v2 := pair1.Leaves()[len(pair1.Leaves())-1], pair2.Leaves()[len(pair2.Leaves())-1]

	if !m.Has(k1) || m.Src()[k1] != k2 {
		t.Errorf("k1 should match k2 under matched dictionary ancestor, got %v", m.Src()[k1])
	}
	if !m.Has(v1) || m.Src()[v1] != v2 {
		t.Errorf("v1 should match v2 under matched dictionary ancestor, got %v", m.Src()[v1])
	}
}

func TestMatchUnmatchedLeavesIgnoresKeywords(t *testing.T) {
	goCode := "package main\n\nfunc f() {\n\tif true {\n\t\tprintln(1)\n\t}\n}\n"
	srcRoot := parseSnippet(t, goCode, "test.go")
	dstRoot := parseSnippet(t, goCode, "test.go")

	p1 := findNodeByType(srcRoot, "if_statement")
	p2 := findNodeByType(dstRoot, "if_statement")
	if p1 == nil || p2 == nil {
		t.Fatalf("failed to find if_statement: p1=%v p2=%v", p1, p2)
	}

	var kw1, kw2 *treesitter.ASTNode
	for _, l := range p1.Leaves() {
		if l.IsKeyword && l.Label == "if" {
			kw1 = l
			break
		}
	}
	for _, l := range p2.Leaves() {
		if l.IsKeyword && l.Label == "if" {
			kw2 = l
			break
		}
	}
	if kw1 == nil || kw2 == nil {
		t.Fatalf("failed to find if keyword: kw1=%v kw2=%v", kw1, kw2)
	}

	m := NewMapping()
	m.Add(srcRoot, dstRoot)
	m.Add(p1, p2)

	MatchUnmatchedLeaves(srcRoot, dstRoot, m, nil)
	if m.Has(kw1) {
		t.Errorf("MatchUnmatchedLeaves should ignore keywords, but kw1 was matched")
	}

	MatchContainerKeywords(srcRoot, dstRoot, m)

	if !m.Has(kw1) || m.Src()[kw1] != kw2 {
		t.Errorf("MatchContainerKeywords should map kw1 to kw2 under mapped parents, got %v", m.Src()[kw1])
	}
}

func TestMatchPairValues(t *testing.T) {
	srcCode := "package main\n\nvar _ = map[string]string{\"priority\": \"a\"}\n"
	dstCode := "package main\n\nvar _ = map[string]string{\"priority\": \"b\"}\n"
	srcRoot := parseSnippet(t, srcCode, "test.go")
	dstRoot := parseSnippet(t, dstCode, "test.go")

	p1 := findNodeByType(srcRoot, "keyed_element")
	p2 := findNodeByType(dstRoot, "keyed_element")
	if p1 == nil || p2 == nil {
		t.Fatalf("failed to find keyed_element: p1=%v p2=%v", p1, p2)
	}
	val1 := p1.Children[len(p1.Children)-1]
	val2 := p2.Children[len(p2.Children)-1]

	m := NewMapping()
	m.Add(p1, p2)

	matchPairValues(srcRoot, dstRoot, m)

	if !m.Has(val1) || m.Src()[val1] != val2 {
		t.Errorf("matchPairValues should map val1 to val2, got %v", m.Src()[val1])
	}
}

func TestMatchPairKeyNameAffinity(t *testing.T) {
	// Old pair: "priority": { ... }
	// New pair 1: "priority": { ... }
	// New pair 2: "oneOf": [ ... ]
	srcCode := "const obj = {\"priority\": {}};\n"
	dstCode := "const obj = {\"priority\": {}, \"oneOf\": []};\n"
	srcObj := parseSnippet(t, srcCode, "test.js")
	dstObj := parseSnippet(t, dstCode, "test.js")

	pairOld := findFirstNode(srcObj, func(n *treesitter.ASTNode) bool {
		return n.Type == "pair" && getKeyLabel(n) == "\"priority\""
	})
	pairNew1 := findFirstNode(dstObj, func(n *treesitter.ASTNode) bool {
		return n.Type == "pair" && getKeyLabel(n) == "\"priority\""
	})
	if pairOld == nil || pairNew1 == nil {
		t.Fatalf("failed to find pair nodes: pairOld=%v, pairNew1=%v", pairOld, pairNew1)
	}

	r := Match(srcObj, dstObj, []byte(srcCode), []byte(dstCode), nil)
	if r == nil || r.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	if r.Mappings.Src()[pairOld] != pairNew1 {
		t.Errorf("pairOld ('priority') should match pairNew1 ('priority'), got %v", r.Mappings.Src()[pairOld])
	}
}

func TestMatchPairValuesDisparateJSXTags(t *testing.T) {
	srcCode := "const old = <span className=\"text-[11px] font-semibold text-gray-400\" />;\n"
	dstCode := `const next = (
  <>
    <i className="fa-solid fa-clipboard-user text-gray-400" />
    <span className="text-[11px] font-semibold text-gray-400 ml-1" />
  </>
);
`
	srcRoot := parseSnippet(t, srcCode, "test.tsx")
	dstRoot := parseSnippet(t, dstCode, "test.tsx")

	attrOld := findFirstNode(srcRoot, func(n *treesitter.ASTNode) bool { return n.Type == "jsx_attribute" })
	dstAttrs := findAllNodesByType(dstRoot, "jsx_attribute")
	if attrOld == nil || len(dstAttrs) < 2 {
		t.Fatalf("failed to find jsx_attributes: attrOld=%v dstAttrs=%d", attrOld, len(dstAttrs))
	}
	attrNew1 := dstAttrs[0]
	attrNew2 := dstAttrs[1]

	m := NewMapping()
	m.Add(srcRoot, dstRoot)

	matchPairValues(srcRoot, dstRoot, m)

	if m.Src()[attrOld] == attrNew1 {
		t.Errorf("attrOld ('span') should NOT match attrNew1 ('i') across disparate tags")
	}
	if m.Src()[attrOld] != attrNew2 {
		t.Errorf("attrOld ('span') should match attrNew2 ('span'), got %v", m.Src()[attrOld])
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
	// Whole-snippet verification: enclosing function_definition and body block map in place.
	testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
		testutil.NodeQuery{Type: "function_definition", Line: 96},
		testutil.NodeQuery{Type: "function_definition", Line: 96},
	)
	testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
		testutil.NodeQuery{Type: "block", Line: 97},
		testutil.NodeQuery{Type: "block", Line: 97},
	)
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
	testutil.AssertUnmapped(t, dstAST, res.Mappings.Dst(),
		testutil.NodeQuery{Type: "return_statement", Line: 450},
	)
}

func TestMatchUnmatchedLeaves_IsolatedGlueGuard(t *testing.T) {
	// Two separate statements under an enclosing block.
	// In T1: stmt1: "x := 1"
	// In T2: stmt2: "y := 2"
	// Both have ":=" (assignment_operator_literal).
	// Because stmt1 and stmt2 are distinct statements under a block,
	// parentMatched is false and siblingScore is 0.
	// The isolated glue guard should skip ":=" from matching across statements.

	srcCode := "package main\n\nfunc f() {\n\tx := 1\n}\n"
	dstCode := "package main\n\nfunc f() {\n\ty := 2\n}\n"
	root1 := parseSnippet(t, srcCode, "test.go")
	root2 := parseSnippet(t, dstCode, "test.go")

	r := rules.Get("go")
	block1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	block2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if block1 == nil || block2 == nil {
		t.Fatalf("failed to find blocks: block1=%v, block2=%v", block1, block2)
	}

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	leafOp1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return n.Label == ":=" })
	if leafOp1 == nil {
		t.Fatal("failed to find ':=' leaf in block1")
	}

	if m.Has(leafOp1) {
		t.Errorf("expected isolated statement-level glue ':=' not to match across statements, got %v", m.Src()[leafOp1])
	}
}

func TestMatchUnmatchedLeaves_ExpressionExemption(t *testing.T) {
	// Intra-expression operators should be exempt from the glue guard
	// when parent expression types match.
	// T1: "a + b" inside binary_expression
	// T2: "c + d" inside binary_expression
	srcCode := "package main\n\nfunc f() {\n\t_ = a + b\n}\n"
	dstCode := "package main\n\nfunc f() {\n\t_ = c + d\n}\n"
	root1 := parseSnippet(t, srcCode, "test.go")
	root2 := parseSnippet(t, dstCode, "test.go")

	r := rules.Get("go")
	block1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	block2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if block1 == nil || block2 == nil {
		t.Fatalf("failed to find blocks: block1=%v, block2=%v", block1, block2)
	}

	stmt1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return r.IsStatement(n.Type) })
	stmt2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return r.IsStatement(n.Type) })
	if stmt1 == nil || stmt2 == nil {
		t.Fatalf("failed to find statements: stmt1=%v, stmt2=%v", stmt1, stmt2)
	}

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)
	m.Add(stmt1, stmt2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	leafOp1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return n.Label == "+" })
	leafOp2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return n.Label == "+" })
	if leafOp1 == nil || leafOp2 == nil {
		t.Fatalf("failed to find '+' operators: op1=%v, op2=%v", leafOp1, leafOp2)
	}

	if !m.Has(leafOp1) || m.Src()[leafOp1] != leafOp2 {
		t.Errorf("expected intra-expression '+' to match via expression exemption, got %v", m.Src()[leafOp1])
	}
}

func TestEffectiveDepthTo(t *testing.T) {
	r := rules.Get("go")
	code := `package main

func f(x, y int) {
	x = 1
	foo(x, y)
}
`
	root := parseSnippet(t, code, "test.go")

	// 1. Transparent wrapper (expression_list with 1 child in `x = 1`) should not increment depth.
	assignStmt := findFirstNode(root, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	if assignStmt == nil {
		t.Fatal("failed to find assignment_statement")
	}
	leaf1 := findFirstNode(assignStmt, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	depth1 := effectiveDepthTo(leaf1, assignStmt, r)
	if depth1 != 1 {
		t.Errorf("expected effectiveDepthTo with single-child wrapper to be 1, got %d", depth1)
	}

	// 2. Multi-child container (argument_list with 2 children in `foo(x, y)`) is not transparent.
	callExpr := findFirstNode(root, func(n *treesitter.ASTNode) bool { return r.IsCall(n.Type) })
	if callExpr == nil {
		t.Fatal("failed to find call_expression")
	}
	leaf2 := findFirstNode(callExpr, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	depth2 := effectiveDepthTo(leaf2, callExpr, r)
	if depth2 != 2 {
		t.Errorf("expected effectiveDepthTo with multi-child container to be 2, got %d", depth2)
	}

	// 3. Declaration immunity (function_declaration is NOT transparent).
	fnDecl := findFirstNode(root, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	if fnDecl == nil {
		t.Fatal("failed to find function_declaration")
	}
	leafIdent := findFirstNode(fnDecl, func(n *treesitter.ASTNode) bool { return n.Label == "f" })
	depth3 := effectiveDepthTo(leafIdent, root, r)
	if depth3 != 2 {
		t.Errorf("expected effectiveDepthTo with declaration ancestor to be 2 (immune), got %d", depth3)
	}

	// 4. Block immunity (block is NOT transparent).
	block := findFirstNode(root, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if block == nil {
		t.Fatal("failed to find block")
	}
	depthBlock := effectiveDepthTo(block, root, r)
	if depthBlock != 2 {
		t.Errorf("expected effectiveDepthTo with block ancestor to be 2 (immune), got %d", depthBlock)
	}
}

func TestIsTransparentWrapper(t *testing.T) {
	r := rules.Get("go")
	code := `package main

func f() {
	x := 1
	a, b := 1, 2
	_ = x; _ = a; _ = b
}
`
	root := parseSnippet(t, code, "test.go")

	// Single child wrapper -> true (expression_list in x := 1)
	stmt1 := findFirstNode(root, func(n *treesitter.ASTNode) bool {
		return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "x" }) != nil
	})
	if stmt1 == nil {
		t.Fatal("failed to find stmt1")
	}
	w1 := stmt1.Children[0] // expression_list with 1 child
	if !isTransparentWrapper(w1, r) {
		t.Errorf("expected expression_list with 1 child to be transparent wrapper")
	}

	// Multi-child wrapper -> false (expression_list in a, b := 1, 2)
	stmt2 := findFirstNode(root, func(n *treesitter.ASTNode) bool {
		return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "a" }) != nil
	})
	if stmt2 == nil {
		t.Fatal("failed to find stmt2")
	}
	w2 := stmt2.Children[0] // expression_list with 2 children
	if isTransparentWrapper(w2, r) {
		t.Errorf("expected expression_list with 2 children NOT to be transparent wrapper")
	}

	// Single child declaration -> false (immune)
	fnDecl := findFirstNode(root, func(n *treesitter.ASTNode) bool { return r.IsDeclaration(n.Type) })
	if isTransparentWrapper(fnDecl, r) {
		t.Errorf("expected function_declaration NOT to be transparent wrapper (scope immunity)")
	}

	// Single child block -> false (immune)
	block := findFirstNode(root, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if isTransparentWrapper(block, r) {
		t.Errorf("expected block NOT to be transparent wrapper (scope immunity)")
	}

	// Nil -> false
	if isTransparentWrapper(nil, r) {
		t.Errorf("expected nil NOT to be transparent wrapper")
	}
}

func TestAreSiblingsMatched(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\tfoo(c1, c2)\n\tsrc := 1\n\t_ = src\n}\n"
	code2 := "package main\n\nfunc f() {\n\tfoo(c1, c2)\n\tsrc := 1\n\t_ = src\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	m := NewMapping()

	// 1. Direct match
	leafA1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "foo" })
	leafA2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "foo" })
	if leafA1 == nil || leafA2 == nil {
		t.Fatal("failed to find foo leaves")
	}
	m.Add(leafA1, leafA2)
	if !areSiblingsMatched(leafA1, leafA2, m) {
		t.Errorf("expected direct leaf match to return true")
	}

	// 2. Unwrapping single-child container to mapped child
	leafB1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "src" })
	leafB2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "src" })
	if leafB1 == nil || leafB2 == nil {
		t.Fatal("failed to find src leaves")
	}
	m.Add(leafB1, leafB2)
	wrapB1 := leafB1.Parent // expression_list
	wrapB2 := leafB2.Parent // expression_list
	if !areSiblingsMatched(wrapB1, wrapB2, m) {
		t.Errorf("expected single-child container unwrapping to match inner leaves")
	}

	// 3. Multi-child container should not match unmapped wrapper
	call1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	call2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	argList1 := findFirstNode(call1, func(n *treesitter.ASTNode) bool { return n.Type == "argument_list" })
	argList2 := findFirstNode(call2, func(n *treesitter.ASTNode) bool { return n.Type == "argument_list" })
	c1_1 := findFirstNode(argList1, func(n *treesitter.ASTNode) bool { return n.Label == "c1" })
	c2_1 := findFirstNode(argList1, func(n *treesitter.ASTNode) bool { return n.Label == "c2" })
	c1_2 := findFirstNode(argList2, func(n *treesitter.ASTNode) bool { return n.Label == "c1" })
	c2_2 := findFirstNode(argList2, func(n *treesitter.ASTNode) bool { return n.Label == "c2" })
	if c1_1 == nil || c2_1 == nil || c1_2 == nil || c2_2 == nil {
		t.Fatal("failed to find argument leaves")
	}
	m.Add(c1_1, c1_2)
	m.Add(c2_1, c2_2)
	if areSiblingsMatched(argList1, argList2, m) {
		t.Errorf("expected multi-child container NOT to match when container itself is unmapped")
	}
}

func TestMatchUnmatchedLeaves_ContainerWrappedSiblingScore(t *testing.T) {
	// In Go: short_var_declaration has expression_list (wrapping identifier) as child 0.
	// When identifier is mapped, the adjacent operator ':=' must receive siblingScore = 500
	// through container-aware sibling matching, and match cleanly.
	srcCode := "package main\n\nfunc f() {\n\tsrc := 1\n}\n"
	dstCode := "package main\n\nfunc f() {\n\tsrc := 2\n}\n"
	root1 := parseSnippet(t, srcCode, "test.go")
	root2 := parseSnippet(t, dstCode, "test.go")

	r := rules.Get("go")
	block1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	block2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if block1 == nil || block2 == nil {
		t.Fatalf("failed to find blocks")
	}

	leafSrc1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return n.Label == "src" })
	leafSrc2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return n.Label == "src" })
	op1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return n.Label == ":=" })
	op2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return n.Label == ":=" })
	if leafSrc1 == nil || leafSrc2 == nil || op1 == nil || op2 == nil {
		t.Fatalf("failed to find nodes")
	}

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)
	m.Add(leafSrc1, leafSrc2)

	MatchUnmatchedLeaves(root1, root2, m, nil)

	if !m.Has(op1) || m.Src()[op1] != op2 {
		t.Errorf("expected ':=' to match via container-aware sibling score, got %v", m.Src()[op1])
	}
}

func TestMatchPairValues_HighSimilarityPreferred(t *testing.T) {
	// When multiple div tags share the same 'className' attribute, match the one
	// with similar class names (> 0.5) and reject completely different ones.
	srcCode := `const old = (
  <>
    <div className="grid grid-cols-1 lg:grid-cols-3 gap-6" />
    <div className="lg:col-span-2 bg-white rounded-3xl p-6 border border-gray-100 shadow-sm flex flex-col items-center" />
  </>
);
`
	dstCode := `const next = <div className="bg-white rounded-3xl p-6 border border-gray-100 shadow-sm flex flex-col items-center" />;
`
	root1 := parseSnippet(t, srcCode, "test.tsx")
	root2 := parseSnippet(t, dstCode, "test.tsx")

	attrs1 := findAllNodesByType(root1, "jsx_attribute")
	if len(attrs1) < 2 {
		t.Fatalf("expected at least 2 jsx_attribute in root1, got %d", len(attrs1))
	}
	attrOld1 := attrs1[0]
	attrOld2 := attrs1[1]

	attrNew := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "jsx_attribute" })
	if attrNew == nil {
		t.Fatal("failed to find jsx_attribute in root2")
	}

	m := NewMapping()
	m.Add(root1, root2)

	matchPairValues(root1, root2, m)

	// 'grid' shouldn't match 'bg-white ...'
	if m.Has(attrOld1) {
		t.Errorf("attrOld1 (grid) should not match attrNew, got mapped to %v", m.Src()[attrOld1])
	}
	// 'lg:col-span-2 bg-white ...' matches 'bg-white ...' (>90% similar)
	if !m.Has(attrOld2) || m.Src()[attrOld2] != attrNew {
		t.Errorf("attrOld2 should match attrNew via similarity, got %v", m.Src()[attrOld2])
	}
}

func TestRecoverCohortSiblings(t *testing.T) {
	t.Run("recovers unique unmatched sibling statement across declarations", func(t *testing.T) {
		srcCode := `package main

func fnOld1() {
	targetLabels := 1
	doHeavyWork()
}

func fnOld2() {
	doOther()
}
`
		dstCode := `package main

func fnOld2() {
	doOther()
	targetLabels := 1
	doHeavyWork()
}
`
		rootOld := parseSnippet(t, srcCode, "test.go")
		rootNew := parseSnippet(t, dstCode, "test.go")

		setupOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "targetLabels" }) != nil
		})
		anchorOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})
		otherOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doOther" }) != nil
		})

		fnOld2 := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "function_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "fnOld2" }) != nil
		})
		fnNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "function_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "fnOld2" }) != nil
		})

		setupNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "targetLabels" }) != nil
		})
		anchorNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})
		otherNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doOther" }) != nil
		})

		if setupOld == nil || anchorOld == nil || otherOld == nil || fnOld2 == nil || fnNew == nil || setupNew == nil || anchorNew == nil || otherNew == nil {
			t.Fatalf("failed to locate statements in ASTs: setupOld=%v, anchorOld=%v, otherOld=%v, fnOld2=%v, fnNew=%v, setupNew=%v, anchorNew=%v, otherNew=%v",
				setupOld, anchorOld, otherOld, fnOld2, fnNew, setupNew, anchorNew, otherNew)
		}

		m := NewMapping()
		m.Add(fnOld2, fnNew)
		m.Add(otherOld, otherNew)
		m.Add(anchorOld, anchorNew)

		RecoverCohortSiblings(rootOld, rootNew, m)

		if !m.Has(setupOld) {
			t.Fatalf("expected setupOld to be recovered into mapping")
		}
		if m.Src()[setupOld] != setupNew {
			t.Errorf("expected setupOld to map to setupNew, got %v", m.Src()[setupOld])
		}
		for i := range setupOld.Children {
			child := setupOld.Children[i]
			if !m.Has(child) {
				t.Errorf("expected child %v to be paired", child)
			}
		}
	})

	t.Run("skips ambiguous candidate in target block", func(t *testing.T) {
		srcCode := `package main

func fnOld() {
	x := 1
	doHeavyWork()
}
`
		dstCode := `package main

func fnNew() {
	x := 1
	x := 1
	doHeavyWork()
}
`
		rootOld := parseSnippet(t, srcCode, "test.go")
		rootNew := parseSnippet(t, dstCode, "test.go")

		setupOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })
		anchorOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})
		anchorNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})

		m := NewMapping()
		m.Add(anchorOld, anchorNew)

		RecoverCohortSiblings(rootOld, rootNew, m)

		if m.Has(setupOld) {
			t.Errorf("expected ambiguous candidate not to be paired, but was mapped to %v", m.Src()[setupOld])
		}
	})

	t.Run("preserves bijectivity with multiple source candidates", func(t *testing.T) {
		srcCode := `package main

func fnOld() {
	x := 1
	x := 1
	doHeavyWork()
}
`
		dstCode := `package main

func fnNew() {
	x := 1
	doHeavyWork()
}
`
		rootOld := parseSnippet(t, srcCode, "test.go")
		rootNew := parseSnippet(t, dstCode, "test.go")

		declsOld := findAllNodesByType(rootOld, "short_var_declaration")
		if len(declsOld) < 2 {
			t.Fatalf("expected at least 2 short_var_declarations in rootOld, got %d", len(declsOld))
		}
		setupOld1 := declsOld[0]
		setupOld2 := declsOld[1]

		anchorOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})
		setupNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })
		anchorNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool {
			return n.Type == "expression_statement" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "doHeavyWork" }) != nil
		})

		m := NewMapping()
		m.Add(anchorOld, anchorNew)

		RecoverCohortSiblings(rootOld, rootNew, m)

		mappedCount := 0
		if m.Has(setupOld1) && m.Src()[setupOld1] == setupNew {
			mappedCount++
		}
		if m.Has(setupOld2) && m.Src()[setupOld2] == setupNew {
			mappedCount++
		}
		if mappedCount != 1 {
			t.Errorf("expected exactly 1 source node mapped to setupNew, got %d", mappedCount)
		}
	})

	t.Run("safely handles nil inputs and non-block containers", func(t *testing.T) {
		RecoverCohortSiblings(nil, nil, nil)

		m := NewMapping()
		RecoverCohortSiblings(nil, nil, m)

		srcCode := "package main\n\nfunc fn1() {\n\t_ = x + y\n}\n"
		dstCode := "package main\n\nfunc fn2() {\n\t_ = x + y\n}\n"
		rootOld := parseSnippet(t, srcCode, "test.go")
		rootNew := parseSnippet(t, dstCode, "test.go")

		leftOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
		leftNew := findFirstNode(rootNew, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
		rightOld := findFirstNode(rootOld, func(n *treesitter.ASTNode) bool { return n.Label == "y" })

		m.Add(leftOld, leftNew)
		RecoverCohortSiblings(rootOld, rootNew, m)

		if m.Has(rightOld) {
			t.Errorf("expected non-block child rightOld not to be paired by RecoverCohortSiblings")
		}
	})
}

func TestMatchUnmatchedLeaves_StatementBoundary_ShortVarDeclaration(t *testing.T) {
	// Identical identifiers like "err" inside completely different short_var_declarations
	// in the same block shouldn't bind across statements when Dice similarity is low.
	srcCode := `package main

func f() {
	err := doDatabaseQuery()
	_ = err
}
`
	dstCode := `package main

func f() {
	err := renderTemplateOutput()
	_ = err
}
`
	rootSrc := parseSnippet(t, srcCode, "test.go")
	rootDst := parseSnippet(t, dstCode, "test.go")

	r := rules.Get("go")
	blockSrc := findFirstNode(rootSrc, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	blockDst := findFirstNode(rootDst, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
	if blockSrc == nil || blockDst == nil {
		t.Fatal("failed to find blocks")
	}

	stmtSrc := findFirstNode(blockSrc, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })
	stmtDst := findFirstNode(blockDst, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })
	if stmtSrc == nil || stmtDst == nil {
		t.Fatal("failed to find short_var_declarations")
	}
	idSrc := findFirstNode(stmtSrc, func(n *treesitter.ASTNode) bool { return n.Label == "err" })

	m := NewMapping()
	m.Add(rootSrc, rootDst)
	m.Add(blockSrc, blockDst)

	MatchUnmatchedLeaves(rootSrc, rootDst, m, nil)

	if m.Has(idSrc) {
		t.Errorf("expected 'err' across unrelated short_var_declarations to be rejected by statement boundary invariant, got mapped to %v", m.Src()[idSrc])
	}
}

func TestRecoverCohortSiblings_RelocatedCohort(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func OldLogic() {
	isDelim := check(10)
	hasOutside := false
	for _, item := range items {
		if item < 0 {
			break
		}
		handle(item)
	}
	deleteSubtree := compute(true)
	hasSrcOutside := false
	process(isDelim, hasOutside, deleteSubtree, hasSrcOutside)
}

func OtherCaller() {
	println("hello")
}
`
		dst := `package main

func OldLogic() {
	println("hello")
}

func ExtractedHelper() {
	isDelim := check(20)
	hasOutside := false
	for _, item := range items {
		if item < 0 {
			break
		}
		handle(item)
	}
	deleteSubtree := compute(false)
	hasSrcOutside := false
	process(isDelim, hasOutside, deleteSubtree, hasSrcOutside)
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

		// Whole-snippet verification: enclosing declaration OldLogic maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// Cohort anchors: isDelim and deleteSubtree
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 8},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 12},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 16},
		)

		// Whole-snippet verification: all 4 intermediate statements in ExtractedHelper are mapped
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 5},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 9},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "for_statement", Line: 6},
			testutil.NodeQuery{Type: "for_statement", Line: 10},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 13},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 17},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 14},
			testutil.NodeQuery{Type: "expression_statement", Line: 18},
		)

		// Verify println("hello") moved from OtherCaller to OldLogic
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 18},
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
		)
	})
}

func TestRecoverCohortSiblings_SingleAnchorClosureExtraction(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package main

func processItems(items []int) {
	precedingInit()
	currCommon := common(items)
	currDice := dice(items)
	currAffinity := affinity(items)
	succeedingDone(currCommon, currDice, currAffinity)
}
`)
		dst := []byte(`package main

func processItems(items []int) {
	precedingInit()
	var currCommon int
	var currDice float64
	var currAffinity float64
	step01()
	step02()
	step03()
	step04()
	step05()
	step06()
	step07()
	step08()
	step09()
	step10()
	step11()
	ensureMetrics := func() {
		currCommon = common(items)
		currDice = dice(items)
		currAffinity = affinity(items)
	}
	ensureMetrics()
	succeedingDone(currCommon, currDice, currAffinity)
}
`)
		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("snippets must parse cleanly: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Whole-snippet verification: enclosing declaration processItems and parameter_list map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
		)

		// Surrounding statements stay outside closure and map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 8},
			testutil.NodeQuery{Type: "call_expression", Line: 25},
		)

		// Extracted cohort statements map from short_var_declaration to assignment_statement in closure
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 5},
			testutil.NodeQuery{Type: "assignment_statement", Line: 20},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 6},
			testutil.NodeQuery{Type: "assignment_statement", Line: 21},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 7},
			testutil.NodeQuery{Type: "assignment_statement", Line: 22},
		)

		// RHS call expressions map to destination calls inside closure
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5},
			testutil.NodeQuery{Type: "call_expression", Line: 20},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 21},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 7},
			testutil.NodeQuery{Type: "call_expression", Line: 22},
		)
	})
}

func TestGetClosureDeclarationName(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		code := `package main

func f() {
	alignWindow := func() {
		internalSecret := 1
		_ = internalSecret
	}
	a, b := 1, 2
	_ = a
	_ = b
}
`
		root := parseSnippet(t, code, "test.go")

		// 1. short_var_declaration with expression_list LHS and func_literal RHS
		decl := findFirstNode(root, func(n *treesitter.ASTNode) bool {
			return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "alignWindow" }) != nil
		})
		if decl == nil {
			t.Fatal("failed to find alignWindow declaration")
		}

		r := rulesFor(decl)
		name := getClosureDeclarationName(decl, r)
		if name != "alignWindow" {
			t.Errorf("expected closure declaration name 'alignWindow', got %q", name)
		}

		// 2. Multi-variable scalar declaration: a, b := 1, 2
		// Scalar assignments like a, b := 1, 2 shouldn't count as closure scopes.
		tupleDecl := findFirstNode(root, func(n *treesitter.ASTNode) bool {
			return n.Type == "short_var_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "a" }) != nil
		})
		if tupleDecl == nil {
			t.Fatal("failed to find tuple declaration")
		}

		tupleR := rulesFor(tupleDecl)
		tupleName := getClosureDeclarationName(tupleDecl, tupleR)
		if tupleName != "" {
			t.Errorf("expected empty declaration name for scalar variable declaration, got %q", tupleName)
		}

		// 3. RHS func_literal itself has no declaration name
		fnLit := findFirstNode(decl, func(n *treesitter.ASTNode) bool { return n.Type == "func_literal" })
		if fnLit == nil {
			t.Fatal("failed to find func_literal")
		}
		if fnName := getDeclarationName(fnLit); fnName != "" {
			t.Errorf("expected func_literal to have empty declaration name, got %q", fnName)
		}
	})
}

func TestLinediffCrossMoveInversion_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package p

func Process() {
	setup()
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if cond1 && i > 0 {
				dist := max(i-j, j-i)
				if dist < 10 {
					bestDist = dist
				}
				matched[i] = dist
				total += dist
			}
		}
	}
}

func Batch() {
	worker := func() {
		for i := 0; i < 10; i++ {
			for j := 0; j < 10; j++ {
				if cond1 && i > 0 {
					dist := max(i-j, j-i)
					if dist < 10 {
						bestDist = dist
					}
					matched[i] = dist
					total += dist
				}
			}
		}
	}
	worker()
}
`

		newSrc := `package p

func Process() {
	setup()
	fallback()
}

func worker() {
	for i := 0; i < 10; i++ {
		for j := 0; j < 10; j++ {
			if cond2 && i > 0 {
				dist := max(i-j, j-i)
				if dist < 10 {
					bestDist = dist
				}
				matched[i] = dist
				total += dist
			}
		}
	}
}

func Batch() {
	worker()
}
`
		rootA, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
		res := Match(rootA, rootB, []byte(oldSrc), []byte(newSrc), part)
		if res == nil || res.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Whole-snippet verification 1: enclosing declarations map in place
		testutil.AssertMapped(t, rootA, rootB, res.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, rootA, rootB, res.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 19},
			testutil.NodeQuery{Type: "function_declaration", Line: 23},
		)

		// Whole-snippet verification 2: surrounding statements map in place
		testutil.AssertMapped(t, rootA, rootB, res.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)

		// Whole-snippet verification 3: Process()'s loop must not map to worker() in B
		testutil.AssertNotMapped(t, rootA, rootB, res.Mappings.Src(),
			testutil.NodeQuery{Type: "for_statement", Line: 5},
			testutil.NodeQuery{Type: "for_statement", Line: 9},
		)

		// The loop in worker() must originate from Batch's closure, never stolen from Process
		testutil.AssertMapped(t, rootA, rootB, res.Mappings.Src(),
			testutil.NodeQuery{Type: "for_statement", Line: 21},
			testutil.NodeQuery{Type: "for_statement", Line: 9},
		)
	})
}

func TestMatch_UnwrappedConditionIdentifier(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`function serialize(annotation: any, type: any) {
    if (annotation && getTypeFromTypeNode(annotation) === type) {
        return tryReuse(annotation);
    }
}
`)
		dst := []byte(`function serialize(annotation: any, type: any) {
    if (annotation) {
        return tryReuse(annotation);
    }
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := NewLinePartition(src, dst)
		res := Match(srcAST, dstAST, src, dst, part)

		// 1. Enclosing context: function serialize
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Surrounding sequence: return statement
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 3},
			testutil.NodeQuery{Type: "return_statement", Line: 3},
		)

		// 3. Target: condition annotation identifiers
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 1},
			testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 1},
		)
	})

	t.Run("FullEngine_Go", func(t *testing.T) {
		src := []byte(`package main

func serialize(annotation bool, t bool) bool {
	if annotation && getTypeFromTypeNode(annotation) {
		return tryReuse(annotation)
	}
	return false
}
`)
		dst := []byte(`package main

func serialize(annotation bool, t bool) bool {
	if annotation {
		return tryReuse(annotation)
	}
	return false
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := NewLinePartition(src, dst)
		res := Match(srcAST, dstAST, src, dst, part)

		// 1. Enclosing context: function serialize
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// 2. Surrounding sequence: return false
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 7},
			testutil.NodeQuery{Type: "return_statement", Line: 7},
		)

		// 3. Target: condition annotation identifier
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 1},
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 1},
		)
	})

	t.Run("FullEngine_Python", func(t *testing.T) {
		src := []byte(`def serialize(annotation, type_val):
    valid = check(annotation)
    if annotation and getTypeFromTypeNode(annotation):
        return tryReuse(annotation)
    return valid
`)
		dst := []byte(`def serialize(annotation, type_val):
    valid = check(annotation)
    if annotation:
        return tryReuse(annotation)
    return valid
`)

		srcAST, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.py")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := NewLinePartition(src, dst)
		res := Match(srcAST, dstAST, src, dst, part)

		// 1. Enclosing context: function serialize
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "function_definition", Line: 1},
			testutil.NodeQuery{Type: "function_definition", Line: 1},
		)

		// 2. Surrounding sequence: valid = check(annotation) and return valid
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 2},
			testutil.NodeQuery{Type: "expression_statement", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 5},
			testutil.NodeQuery{Type: "return_statement", Line: 5},
		)

		// 3. Target: condition annotation identifier
		testutil.AssertMapped(t, srcAST, dstAST, res.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1},
		)
	})
}

func TestMatch_RustControlFlowStatementBoundaries(t *testing.T) {
	source := []byte(`fn process(values: &[i32], count: i32, ready: bool) {
    for value in values {
        consume(*value);
    }
    if ready {
        consume(count);
    }
    loop {
        consume(count);
        break;
    }
    while count > 0 {
        consume(count);
    }
    match count {
        0 => consume(0),
        _ => consume(count),
    };
}
`)
	src, err := treesitter.Parse(source, "test.rs")
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	dst, err := treesitter.Parse(source, "test.rs")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	if src.HasError || dst.HasError {
		t.Fatalf("parse errors: source=%d, destination=%d", src.ParseErrorCount, dst.ParseErrorCount)
	}
	mr := Match(src, dst, source, source, NewLinePartition(source, source))
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil mappings")
	}
	r := rulesFor(src)
	for _, typ := range []string{"for_expression", "if_expression", "loop_expression", "while_expression", "match_expression"} {
		t.Run(typ, func(t *testing.T) {
			control := findFirstNode(src.Root(), func(n *treesitter.ASTNode) bool {
				return n.Type == typ
			})
			if control == nil {
				t.Fatal("control-flow expression not found")
			}
			if !r.IsStatement(control.Type) {
				t.Fatal("Rust control-flow expressions are statements when used directly in a block")
			}
			var condition *treesitter.ASTNode
			if typ == "loop_expression" {
				condition = control
			} else {
				condition = findFirstNode(control, func(n *treesitter.ASTNode) bool {
					return n.Label == "ready" || n.Label == "count" || n.Label == "value"
				})
			}
			want := control
			if control.Parent != nil && r.IsExpressionStatement(control.Parent.Type) {
				want = control.Parent
			}
			if got := FindEnclosingStatement(condition, r); got != want {
				t.Errorf("condition belongs to the control-flow statement, got %v (%s), want %v (%s)", got, got.Type, want, want.Type)
			}
			testutil.AssertMappedText(t, src.Root(), dst.Root(), source, source, mr.Mappings.Src(),
				string(source[control.StartByte:control.EndByte]),
				string(source[control.StartByte:control.EndByte]),
				typ,
			)
		})
	}
}

func TestMatch_ExpressionWrappingSameScope(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    match pagerflags.split_first() {
        Some((_, _)) => {
            let mut process = Command::new("less");
            process.arg("-R");
            process.arg("-F");
            process.arg("-X");
            process.arg("-K");
            process.arg("-i");
            process.env("LESSCHARSET", "UTF-8");
            process.env("LESSHISTFILE", "/dev/null");
            Ok(process)
        }
        None => Ok(Command::new("cat")),
    };
    process.wait()
}
`)
		newSrc := []byte(`fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    Ok(match pagerflags.split_first() {
        Some((_, _)) => {
            _make_process_from_less_path()
        }
        None => Command::new("cat"),
    });
    process.wait()
}

fn _make_process_from_less_path() -> Command {
    let mut process = Command::new("less");
    process.arg("-R");
    process.arg("-F");
    process.arg("-X");
    process.arg("-K");
    process.arg("-i");
    process.env("LESSCHARSET", "UTF-8");
    process.env("LESSHISTFILE", "/dev/null");
    process
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("oldSrc has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("newSrc has parse errors: %d", dstAST.ParseErrorCount)
		}

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// The enclosing function must stay in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// The preceding statement must stay in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)

		// The statement after the wrapped expression must stay in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 17},
			testutil.NodeQuery{Type: "call_expression", Line: 9},
		)

		// The match expression must stay inside the new Ok wrapper.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_expression", Line: 3},
			testutil.NodeQuery{Type: "match_expression", Line: 3},
		)

		// The new wrapper must not claim an inner Ok call or its identifier.
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
		)
		testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1},
		)

		// The match operand must stay in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3},
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 2},
		)
	})
}

func TestRecoverCohortSiblings_UnqualifiedUseDeclaration_FullEngine(t *testing.T) {
	t.Run("FullEngine_Rust", func(t *testing.T) {
		src := []byte(`pub mod integration_test_utils {
    use bytelines::ByteLines;
    use itertools;
    use crate::ansi;

    pub fn make_options() -> i32 {
        let default_limit = 42;
        default_limit
    }
}
`)
		dst := []byte(`use bytelines::ByteLines;
use itertools;
use crate::ansi;

pub fn make_options() -> i32 {
    let default_limit = 42;
    default_limit
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context / surrounding function: make_options maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"pub fn make_options() -> i32", "pub fn make_options() -> i32")

		// 2. Stationary function body statement default_limit maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let default_limit = 42;", "let default_limit = 42;")

		// 3. Sibling compound imports move alongside
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"use bytelines::ByteLines;", "use bytelines::ByteLines;")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"use crate::ansi;", "use crate::ansi;")

		// 4. Target unqualified import: use itertools; must map across scopes
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"use itertools;", "use itertools;")
	})

	t.Run("FullEngine_Cpp", func(t *testing.T) {
		src := []byte(`namespace integration_test_utils {
    using std::string;
    using std::vector;
    using std::map;

    int make_options() {
        int default_limit = 42;
        return default_limit;
    }
}
`)
		dst := []byte(`using std::string;
using std::vector;
using std::map;

int make_options() {
    int default_limit = 42;
    return default_limit;
}
`)

		srcAST, err := treesitter.Parse(src, "test.cpp")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.cpp")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context / surrounding function: make_options maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"int make_options()", "int make_options()")

		// 2. Stationary function body statement default_limit maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"int default_limit = 42;", "int default_limit = 42;")

		// 3. Sibling using declarations move alongside
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"using std::string;", "using std::string;")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"using std::vector;", "using std::vector;")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"using std::map;", "using std::map;")
	})

	t.Run("FullEngine_TypeScript", func(t *testing.T) {
		src := []byte(`namespace integration_test_utils {
    import ByteLines = bytelines.ByteLines;
    import ansi = crate.ansi;
    import itertools = crate.itertools;

    export function make_options(): number {
        let default_limit = 42;
        return default_limit;
    }
}
`)
		dst := []byte(`import ByteLines = bytelines.ByteLines;
import ansi = crate.ansi;
import itertools = crate.itertools;

export function make_options(): number {
    let default_limit = 42;
    return default_limit;
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context / surrounding function: make_options maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"export function make_options(): number", "export function make_options(): number")

		// 2. Stationary function body statement default_limit maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let default_limit = 42;", "let default_limit = 42;")

		// 3. Sibling import aliases move alongside
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"import ByteLines = bytelines.ByteLines;", "import ByteLines = bytelines.ByteLines;")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"import ansi = crate.ansi;", "import ansi = crate.ansi;")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"import itertools = crate.itertools;", "import itertools = crate.itertools;")
	})

	t.Run("FullEngine_Python", func(t *testing.T) {
		src := []byte(`class IntegrationTestUtils:
    from os import path
    import itertools
    from sys import version

    def make_options():
        default_limit = 42
        return default_limit
`)
		dst := []byte(`from os import path
import itertools
from sys import version

def make_options():
    default_limit = 42
    return default_limit
`)

		srcAST, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.py")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context / surrounding function: make_options maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"def make_options():", "def make_options():")

		// 2. Stationary function body statement default_limit maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"default_limit = 42", "default_limit = 42")

		// 3. Sibling imports move alongside
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"from os import path", "from os import path")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"import itertools", "import itertools")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"from sys import version", "from sys import version")
	})
}

func TestMatchUnmatchedLeaves_FieldMutualExclusion_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`def configure_session():
    setup_logger()
    options = {
        "token": 100,
        "active": True,
    }
    teardown_logger(options)
`)
		newSrc := []byte(`def configure_session():
    setup_logger()
    options = {
        "active": True,
        "auth_key": "token",
    }
    teardown_logger(options)
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.py")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.py")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing function declaration must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"def configure_session():", "def configure_session():")

		// 2. Surrounding sequence (setup_logger, teardown_logger) must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setup_logger()", "setup_logger()")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardown_logger(options)", "teardown_logger(options)")

		// 3. Target assignment container options = { ... } must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			`"active": True`, `"active": True`)

		// 4. A dictionary key named "token" shouldn't match a value named "token".
		testutil.AssertNotMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			`"token"`, `"token"`)
	})
}
