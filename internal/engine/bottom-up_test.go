package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestComputeAffinity_HardConstraints(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\t{\n\t\tx := 1\n\t\t_ = x\n\t}\n}\n"
	code2 := "package main\n\nfunc f() {\n\t{\n\t\tx := 1\n\t\t_ = x\n\t}\n\tfoo()\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	blocks1 := findAllNodesByType(root1, "block")
	blocks2 := findAllNodesByType(root2, "block")
	if len(blocks1) < 2 || len(blocks2) < 2 {
		t.Fatalf("expected at least 2 blocks each")
	}
	t1 := blocks1[1] // inner block
	c := blocks2[1]  // inner block
	leaf1 := findFirstNode(t1, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	leaf2 := findFirstNode(c, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	if leaf1 == nil || leaf2 == nil {
		t.Fatalf("expected x leaves")
	}

	m := NewMapping()

	// 1. Without common descendants, affinity should be -1.0
	scoreNoCommon := computeAffinity(t1, c, m, DefaultAffinityWeights, false)
	if scoreNoCommon != -1.0 {
		t.Errorf("expected -1.0 for no common descendants, got %f", scoreNoCommon)
	}

	m.Add(leaf1, leaf2)

	// 2. Already mapped destination candidate should return -1.0
	cMapped := blocks2[0] // outer block
	m.Add(t1, cMapped)
	scoreAlreadyMapped := computeAffinity(t1, cMapped, m, DefaultAffinityWeights, false)
	if scoreAlreadyMapped != -1.0 {
		t.Errorf("expected -1.0 for already mapped candidate, got %f", scoreAlreadyMapped)
	}

	// 3. Incompatible types should return -1.0
	m2 := NewMapping()
	m2.Add(leaf1, leaf2)
	cDiffType := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "expression_statement" })
	if cDiffType == nil {
		t.Fatal("failed to find expression_statement")
	}
	scoreDiffType := computeAffinity(t1, cDiffType, m2, DefaultAffinityWeights, false)
	if scoreDiffType != -1.0 {
		t.Errorf("expected -1.0 for incompatible types, got %f", scoreDiffType)
	}
}

func TestComputeAffinity_ScopeProximity(t *testing.T) {
	code1 := `package main

func fnA() {
	val := 1
	_ = val
}

func fnB() {
	val := 1
	_ = val
}
`
	code2 := `package main

func fnA() {
	val := 1
	_ = val
}

func fnB() {
	val := 1
	_ = val
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	funcs1 := findAllNodesByType(root1, "function_declaration")
	funcs2 := findAllNodesByType(root2, "function_declaration")
	if len(funcs1) < 2 || len(funcs2) < 2 {
		t.Fatalf("expected 2 functions")
	}
	scopeA1, scopeB1 := funcs1[0], funcs1[1]
	scopeA2, scopeB2 := funcs2[0], funcs2[1]

	b1 := findFirstNode(scopeA1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	b2 := findFirstNode(scopeA2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	b3 := findFirstNode(scopeB2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	l1 := findFirstNode(b1, func(n *treesitter.ASTNode) bool { return n.Label == "val" })
	l2 := findFirstNode(b2, func(n *treesitter.ASTNode) bool { return n.Label == "val" })
	l3 := findFirstNode(b3, func(n *treesitter.ASTNode) bool { return n.Label == "val" })

	m := NewMapping()
	m.Add(l1, l2)
	m.Add(scopeA1, scopeA2)

	scoreInScope := computeAffinity(b1, b2, m, DefaultAffinityWeights, false)

	mCross := NewMapping()
	mCross.Add(l1, l3)
	mCross.Add(scopeA1, scopeA2)
	mCross.Add(scopeB1, scopeB2)

	scoreCrossScope := computeAffinity(b1, b3, mCross, DefaultAffinityWeights, false)

	if scoreInScope <= scoreCrossScope {
		t.Errorf("expected in-scope affinity (%f) > cross-scope affinity (%f)", scoreInScope, scoreCrossScope)
	}
}

func TestComputeAffinity_PositionalAlignment(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\t_ = []int{a, b}\n}\n"
	code2 := "package main\n\nfunc f() {\n\t_ = []int{a, b}\n}\n"
	codeShifted := "package main\n\nfunc f() {\n\t_ = []int{b, a}\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")
	rootShifted := parseSnippet(t, codeShifted, "test.go")

	item1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	item2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	item2Shifted := findFirstNode(rootShifted, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	if item1 == nil || item2 == nil || item2Shifted == nil {
		t.Fatalf("failed to find 'a' nodes")
	}

	m := NewMapping()
	m.Add(item1, item2)

	scoreSamePos := computeAffinity(item1, item2, m, DefaultAffinityWeights, false)

	mShifted := NewMapping()
	mShifted.Add(item1, item2Shifted)

	scoreShifted := computeAffinity(item1, item2Shifted, mShifted, DefaultAffinityWeights, false)
	if scoreSamePos < scoreShifted {
		t.Errorf("expected same positional score (%f) >= shifted score (%f)", scoreSamePos, scoreShifted)
	}

	// Unordered parent container ignores positional differences
	parent1 := item1.Parent
	parent2 := item2.Parent
	if parent1 != nil && parent2 != nil {
		parent1.IsUnordered = true
		parent2.IsUnordered = true
		scoreUnordered := computeAffinity(item1, item2, m, DefaultAffinityWeights, false)
		if scoreUnordered < scoreSamePos {
			t.Errorf("expected unordered score (%f) to be neutral/maximal", scoreUnordered)
		}
	}
}

func TestComputeAffinity_KeyBonus(t *testing.T) {
	py1 := "d = {'name': 'alice'}\n"
	py2 := "d = {'name': 'alice'}\n"
	py3 := "d = {'other': 'alice'}\n"
	root1 := parseSnippet(t, py1, "test.py")
	root2 := parseSnippet(t, py2, "test.py")
	root3 := parseSnippet(t, py3, "test.py")

	pair1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "pair" })
	pair2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "pair" })
	pair3 := findFirstNode(root3, func(n *treesitter.ASTNode) bool { return n.Type == "pair" })
	if pair1 == nil || pair2 == nil || pair3 == nil {
		t.Fatalf("failed to find pair nodes")
	}

	v1 := pair1.Children[len(pair1.Children)-1]
	v2 := pair2.Children[len(pair2.Children)-1]
	v3 := pair3.Children[len(pair3.Children)-1]

	m := NewMapping()
	m.Add(v1, v2)

	scoreSameKey := computeAffinity(pair1, pair2, m, DefaultAffinityWeights, false)

	mDiff := NewMapping()
	mDiff.Add(v1, v3)
	scoreDiffKey := computeAffinity(pair1, pair3, mDiff, DefaultAffinityWeights, false)

	if scoreSameKey <= scoreDiffKey {
		t.Errorf("expected same key score (%f) > different key score (%f)", scoreSameKey, scoreDiffKey)
	}
}

func TestComputeAffinity_SmallSubtreeScopeGate(t *testing.T) {
	code1 := "package main\n\nfunc f1() {\n\t_ = x + y\n}\n\nfunc other() {}\n"
	code2 := "package main\n\nfunc f2() {\n\t_ = x + y\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	sub1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	sub2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	if sub1 == nil || sub2 == nil {
		t.Fatalf("failed to find binary_expression")
	}

	anc1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool {
		return n.Type == "function_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "f1" }) != nil
	})
	otherAnc := findFirstNode(root1, func(n *treesitter.ASTNode) bool {
		return n.Type == "function_declaration" && findFirstNode(n, func(c *treesitter.ASTNode) bool { return c.Label == "other" }) != nil
	})
	if anc1 == nil || otherAnc == nil {
		t.Fatalf("failed to find function declarations")
	}

	l1 := findFirstNode(sub1, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	l2 := findFirstNode(sub2, func(n *treesitter.ASTNode) bool { return n.Label == "x" })
	if l1 == nil || l2 == nil {
		t.Fatalf("failed to find 'x' leaves")
	}

	m := NewMapping()
	m.Add(l1, l2)
	m.Add(anc1, otherAnc)

	score := computeAffinity(sub1, sub2, m, DefaultAffinityWeights, false)
	if score != -1.0 {
		t.Errorf("expected -1.0 for small subtree crossing scope boundary, got %f", score)
	}
}

func TestComputeAffinity_DepthPenaltyFires(t *testing.T) {
	codeSame1 := "package main\n\nfunc f() {\n\t{\n\t\tshared := 1\n\t\tx := 2\n\t\t_ = shared\n\t\t_ = x\n\t}\n}\n"
	codeSame2 := "package main\n\nfunc f() {\n\t{\n\t\tshared := 1\n\t\tx := 2\n\t\t_ = shared\n\t\t_ = x\n\t}\n}\n"
	rootSame1 := parseSnippet(t, codeSame1, "test.go")
	rootSame2 := parseSnippet(t, codeSame2, "test.go")

	blocks1 := findAllNodesByType(rootSame1, "block")
	blocks2 := findAllNodesByType(rootSame2, "block")
	block1 := blocks1[1]
	block2 := blocks2[1]
	parent1 := findFirstNode(rootSame1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	parent2 := findFirstNode(rootSame2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })

	sharedLeaf1 := findFirstNode(block1, func(n *treesitter.ASTNode) bool { return n.Label == "shared" })
	sharedLeaf2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return n.Label == "shared" })

	mSame := NewMapping()
	mSame.Add(sharedLeaf1, sharedLeaf2)
	mSame.Add(parent1, parent2)
	scoreSameDepth := computeAffinity(block1, block2, mSame, DefaultAffinityWeights, false)

	// Cross-depth: block1Deep is nested under if inside parent1Deep.
	codeDeep := "package main\n\nfunc f() {\n\tif cond {\n\t\t{\n\t\t\tshared := 1\n\t\t\tx := 2\n\t\t\t_ = shared\n\t\t\t_ = x\n\t\t}\n\t}\n}\n"
	rootDeep := parseSnippet(t, codeDeep, "test.go")
	blocksDeep := findAllNodesByType(rootDeep, "block")
	block1Deep := blocksDeep[len(blocksDeep)-1]
	parent1Deep := findFirstNode(rootDeep, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	sharedLeaf1d := findFirstNode(block1Deep, func(n *treesitter.ASTNode) bool { return n.Label == "shared" })

	mDeep := NewMapping()
	mDeep.Add(sharedLeaf1d, sharedLeaf2)
	mDeep.Add(parent1Deep, parent2)
	scoreCrossDepth := computeAffinity(block1Deep, block2, mDeep, DefaultAffinityWeights, false)

	if scoreSameDepth <= scoreCrossDepth {
		t.Errorf("depth penalty did not fire: same-depth %f should exceed cross-depth %f", scoreSameDepth, scoreCrossDepth)
	}
}

func TestContestContainers_SiblingArbitration(t *testing.T) {
	code1 := `const a = [
  {"name": "early", "url": "url_match"},
  {"name": "stationary", "desc": "desc_match", "fileMatch": "file_match"}
];
`
	code2 := `const b = [
  {"name": "stationary_updated", "desc": "desc_match", "fileMatch": "file_match", "url": "url_match"}
];
`
	root1 := parseSnippet(t, code1, "test.js")
	root2 := parseSnippet(t, code2, "test.js")

	array1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "array" })
	array2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "array" })
	if array1 == nil || array2 == nil {
		t.Fatalf("expected arrays in root1 and root2")
	}

	objects1 := findAllNodesByType(array1, "object")
	if len(objects1) < 2 {
		t.Fatalf("expected at least 2 objects in root1, got %d", len(objects1))
	}
	t1Early := objects1[0]
	t1Stationary := objects1[1]

	t2 := findFirstNode(array2, func(n *treesitter.ASTNode) bool { return n.Type == "object" })
	if t2 == nil {
		t.Fatal("failed to find object in root2")
	}

	leafURL1 := findFirstNode(t1Early, func(n *treesitter.ASTNode) bool { return n.Label == `"url_match"` })
	leafURL2 := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Label == `"url_match"` })
	leafDesc1 := findFirstNode(t1Stationary, func(n *treesitter.ASTNode) bool { return n.Label == `"desc_match"` })
	leafDesc2 := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Label == `"desc_match"` })
	leafMatch1 := findFirstNode(t1Stationary, func(n *treesitter.ASTNode) bool { return n.Label == `"file_match"` })
	leafMatch2 := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Label == `"file_match"` })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(array1, array2)
	m.Add(leafURL1, leafURL2)
	m.Add(leafDesc1, leafDesc2)
	m.Add(leafMatch1, leafMatch2)

	// Simulate BottomUp greedy post-order: t1Early grabbed t2
	m.Add(t1Early, t2)

	ContestContainers(root1, root2, m, nil)

	// Sibling contest should reassign t2 from t1Early to t1Stationary
	if m.Get(t1Early) != nil {
		t.Errorf("expected t1Early to be unmapped after contest, got %v", m.Get(t1Early))
	}
	if m.Get(t1Stationary) != t2 {
		t.Errorf("expected t1Stationary to be mapped to t2, got %v", m.Get(t1Stationary))
	}
	if m.Dst()[leafURL2] != nil && m.Dst()[leafURL2] == leafURL1 {
		t.Errorf("expected leafURL2 mapping from t1Early to be freed")
	}
}

func TestContestContainers_VerticalHierarchy(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\tif true {\n\t\tpanic(\"throw\")\n\t}\n}\n"
	code2 := "package main\n\nfunc f() {\n\tpanic(\"throw\")\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	func1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	func2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	if func1 == nil || func2 == nil {
		t.Fatal("failed to find function_declaration")
	}

	blocks1 := findAllNodesByType(root1, "block")
	if len(blocks1) < 2 {
		t.Fatalf("expected at least 2 blocks in root1")
	}
	outer1 := blocks1[0]
	inner1 := blocks1[1]

	outer2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	if outer2 == nil {
		t.Fatal("failed to find block in root2")
	}

	innerThrow1 := findFirstNode(inner1, func(n *treesitter.ASTNode) bool { return n.Label == `"throw"` })
	innerThrow2 := findFirstNode(outer2, func(n *treesitter.ASTNode) bool { return n.Label == `"throw"` })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(func1, func2)
	m.Add(innerThrow1, innerThrow2)
	m.Add(inner1, outer2) // Inner block greedily stole outer function body

	ContestContainers(root1, root2, m, nil)

	if m.Get(inner1) != nil {
		t.Errorf("expected inner1 to be unmapped, got %v", m.Get(inner1))
	}
	if m.Get(outer1) != outer2 {
		t.Errorf("expected outer1 to be mapped to outer2, got %v", m.Get(outer1))
	}
}

func TestRollupMatchedContainers_UnmatchedBodyGuard(t *testing.T) {
	code1 := "package main\n\nfunc Foo(t int) {\n\tstatementInOld()\n}\n"
	code2 := "package main\n\nfunc Bar(t int) {\n\tstatementInNew()\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	f1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	f2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	p1 := findFirstNode(f1, func(n *treesitter.ASTNode) bool { return n.Type == "parameter_list" })
	p2 := findFirstNode(f2, func(n *treesitter.ASTNode) bool { return n.Type == "parameter_list" })

	m := NewMapping()
	m.Add(p1, p2)
	t1 := findFirstNode(p1, func(n *treesitter.ASTNode) bool { return n.Label == "t" })
	t2 := findFirstNode(p2, func(n *treesitter.ASTNode) bool { return n.Label == "t" })
	m.Add(t1, t2)

	RollupMatchedContainers(root1, root2, m)

	if m.Get(f1) != nil {
		t.Errorf("expected function_declaration with unmatched body not to roll up, but got mapped to %v", m.Get(f1))
	}
}

func TestRollupMatchedContainers_MassWeighting(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\tx = a + b\n}\n"
	code2 := "package main\n\nfunc f() {\n\tx = other\n\tx = a + b\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	func1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	func2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	block1 := findFirstNode(func1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	block2 := findFirstNode(func2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	decl1 := findAllNodesByType(root1, "assignment_statement")
	t1 := decl1[0] // x = a + b
	bin1 := findFirstNode(t1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

	decl2 := findAllNodesByType(root2, "assignment_statement")
	c2Wrong := decl2[0] // x = other
	c2Right := decl2[1] // x = a + b
	bin2 := findFirstNode(c2Right, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

	leafOp1 := findFirstNode(t1, func(n *treesitter.ASTNode) bool { return n.Label == "=" })
	leafOp2 := findFirstNode(c2Wrong, func(n *treesitter.ASTNode) bool { return n.Label == "=" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(func1, func2)
	m.Add(block1, block2)
	m.Add(leafOp1, leafOp2)
	addIsomorphicPairs(bin1, bin2, m)

	RollupMatchedContainers(root1, root2, m)

	if m.Get(t1) != c2Right {
		t.Fatalf("expected t1 to roll up to c2Right by structural mass, got %v", m.Get(t1))
	}
}

func TestRollupMatchedContainers_TieBreaking_ScopeAffinity(t *testing.T) {
	code1 := `package main

func func1() {
	x = 1
}
`
	code2 := `package main

func func1() {
	x = 1
}

func func2() {
	x = 1
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	funcs1 := findAllNodesByType(root1, "function_declaration")
	funcs2 := findAllNodesByType(root2, "function_declaration")
	func1Src := funcs1[0]
	func1Dst, func2Dst := funcs2[0], funcs2[1]

	t1 := findFirstNode(func1Src, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	candInScope := findFirstNode(func1Dst, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	candForeign := findFirstNode(func2Dst, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(func1Src, func1Dst)

	r := rules.Get("go")
	superior := isSuperiorCandidate(candInScope, candForeign, t1, m, r)
	if !superior {
		t.Errorf("expected candInScope to be superior to candForeign via Tier 1 Scope Affinity")
	}

	inferior := isSuperiorCandidate(candForeign, candInScope, t1, m, r)
	if inferior {
		t.Errorf("expected candForeign NOT to be superior to candInScope via Tier 1 Scope Affinity")
	}
}

func TestRollupMatchedContainers_TieBreaking_LineDistance(t *testing.T) {
	code1 := `package main

func f() {
	x = 1
}
`
	code2 := "package main\n\nfunc f() {\n\tx = 1\n" +
		strings.Repeat("\n", 20) +
		"\tx = 1\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	t1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	stmts2 := findAllNodesByType(root2, "assignment_statement")
	if len(stmts2) < 2 {
		t.Fatalf("expected 2 assignment_statements in root2")
	}
	candClose := stmts2[0]
	candFar := stmts2[1]

	m := NewMapping()
	m.Add(root1, root2)

	r := rules.Get("go")
	if !isSuperiorCandidate(candClose, candFar, t1, m, r) {
		t.Errorf("expected candClose to defeat candFar via Tier 2 Drift-Compensated Proximity")
	}
	if isSuperiorCandidate(candFar, candClose, t1, m, r) {
		t.Errorf("expected candFar to lose to candClose via Tier 2 Drift-Compensated Proximity")
	}
}

func TestRollupMatchedContainers_TieBreaking_NodeID(t *testing.T) {
	code1 := `package main

func f() {
	x = 1
}
`
	code2 := `package main

func f() {
	x = 1; x = 1
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	t1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	stmts2 := findAllNodesByType(root2, "assignment_statement")
	if len(stmts2) < 2 {
		t.Fatalf("expected 2 assignment_statements in root2")
	}
	candFirst := stmts2[0]
	candSecond := stmts2[1]

	if candFirst.ID >= candSecond.ID {
		t.Fatalf("expected candFirst.ID (%d) < candSecond.ID (%d)", candFirst.ID, candSecond.ID)
	}

	m := NewMapping()
	m.Add(root1, root2)

	r := rules.Get("go")
	if !isSuperiorCandidate(candFirst, candSecond, t1, m, r) {
		t.Errorf("expected candFirst to defeat candSecond via Tier 3 Pre-order Node ID")
	}
	if isSuperiorCandidate(candSecond, candFirst, t1, m, r) {
		t.Errorf("expected candSecond to lose to candFirst via Tier 3 Pre-order Node ID")
	}
}

func TestContestContainers_VerticalReclaim(t *testing.T) {
	code1 := `package main

func f() {
	if true {
		pairCount = len(pairs)
	}
	s.cpySrcToDst = make(map[key]val, pairCount)
}
`
	code2 := `package main

func f() {
	s.cpySrcToDst = make(map[key]val, len(pairs))
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	func1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	func2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	block1 := findFirstNode(func1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	block2 := findFirstNode(func2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	stmts1 := findAllNodesByType(block1, "assignment_statement")
	currentT1 := stmts1[0] // pairCount = len(pairs)
	candidate := stmts1[1] // s.cpySrcToDst = make(...)

	t2 := findFirstNode(block2, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(func1, func2)
	m.Add(block1, block2)

	lhsOld := findFirstNode(candidate, func(n *treesitter.ASTNode) bool { return n.Type == "selector_expression" })
	lhsNew := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Type == "selector_expression" })
	m.Add(lhsOld, lhsNew)
	for i := range lhsOld.Children {
		m.Add(lhsOld.Children[i], lhsNew.Children[i])
	}

	mapTypeOld := findFirstNode(candidate, func(n *treesitter.ASTNode) bool { return n.Type == "map_type" })
	mapTypeNew := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Type == "map_type" })
	m.Add(mapTypeOld, mapTypeNew)
	for i := range mapTypeOld.Children {
		m.Add(mapTypeOld.Children[i], mapTypeNew.Children[i])
	}

	// Simulate BottomUp greedily matching currentT1 to t2 before contest:
	m.Add(currentT1, t2)
	opDecomp := findFirstNode(currentT1, func(n *treesitter.ASTNode) bool { return n.Label == "=" })
	opNew := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Label == "=" })
	m.Add(opDecomp, opNew)
	rhsDecomp := findFirstNode(currentT1, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	rhsNew := findFirstNode(t2, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	m.Add(rhsDecomp, rhsNew)

	ContestContainers(root1, root2, m, nil)

	if got := m.Get(currentT1); got != nil {
		t.Errorf("m.Get(currentT1) = %v, want nil", got)
	}
	if got := m.Get(candidate); got != t2 {
		t.Fatalf("m.Get(candidate) = %v, want %v", got, t2)
	}
	opOld := findFirstNode(candidate, func(n *treesitter.ASTNode) bool { return n.Label == "=" })
	if got := m.Get(opOld); got != opNew {
		t.Errorf("m.Get(opOld) = %v, want %v", got, opNew)
	}
}

func TestRollupMatchedContainers_CrossStatementBoundaryGuard(t *testing.T) {
	code1 := `package main

func f() {
	if a && b {
	}
}
`
	code2 := `package main

func f() {
	if a && c {
	}
	if b && d {
	}
}
`
	codeOther := `package main

func other() {
	if true {
	}
	if true {
	}
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")
	rootOther := parseSnippet(t, codeOther, "test.go")

	block1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	block2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	ifStmt1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	cond1 := findFirstNode(ifStmt1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	body1 := findFirstNode(ifStmt1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	ifStmts2 := findAllNodesByType(root2, "if_statement")
	ifStmt2, ifStmt3 := ifStmts2[0], ifStmts2[1]
	cond2 := findFirstNode(ifStmt2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	body2 := findFirstNode(ifStmt2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	cond3 := findFirstNode(ifStmt3, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	body3 := findFirstNode(ifStmt3, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	otherStmts := findAllNodesByType(rootOther, "if_statement")
	otherStmtA, otherStmtB := otherStmts[0], otherStmts[1]
	otherBodyA := findFirstNode(otherStmtA, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	otherBodyB := findFirstNode(otherStmtB, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	childA1 := findFirstNode(cond1, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	childB1 := findFirstNode(cond1, func(n *treesitter.ASTNode) bool { return n.Label == "b" })
	childA2 := findFirstNode(cond2, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	childB2 := findFirstNode(cond3, func(n *treesitter.ASTNode) bool { return n.Label == "b" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)
	m.Add(otherStmtA, ifStmt2)
	m.Add(otherBodyA, body2)
	m.Add(otherStmtB, ifStmt3)
	m.Add(otherBodyB, body3)
	m.Add(childA1, childA2)
	m.Add(childB1, childB2)

	if !hasForeignBodyOwner(cond1, cond2, m, nil) {
		t.Errorf("expected hasForeignBodyOwner to reject cond1 vs cond2 across distinct statements")
	}

	mReverse := NewMapping()
	mReverse.Add(body1, body2)
	if !hasForeignBodyOwner(cond1, cond3, mReverse, nil) {
		t.Errorf("expected hasForeignBodyOwner to reject cond1 vs cond3 when body1 is mapped outside cond3")
	}

	RollupMatchedContainers(root1, root2, m)

	if m.Get(cond1) != nil {
		t.Errorf("expected cond1 NOT to roll up across distinct statement boundaries, but got %v", m.Get(cond1))
	}
}

func TestHasForeignBodyOwner_InitializerStatementPermitted(t *testing.T) {
	code1 := `package main

func f() {
	if init := 1; cond > 0 {
		body()
	}
}
`
	code2 := `package main

func f() {
	assign = 2
	if cond > 0 {
		body()
	}
}
`
	codeOther := `package main

func other() {
	if otherCond > 0 {
		otherBody()
	}
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")
	rootOther := parseSnippet(t, codeOther, "test.go")

	init1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })
	cond1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	body1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "block" && n.Parent.Type == "if_statement" })

	assign2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	body2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "block" && n.Parent.Type == "if_statement" })

	otherCond := findFirstNode(rootOther, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

	m := NewMapping()
	m.Add(body1, body2)

	// Moving an init statement outside an if statement shouldn't be blocked by foreign body checks.
	if hasForeignBodyOwner(init1, assign2, m, nil) {
		t.Errorf("expected hasForeignBodyOwner to permit initializer statement init1 vs assign2")
	}

	// But condition expressions tested against an unrelated construct must still be rejected.
	if !hasForeignBodyOwner(cond1, otherCond, m, nil) {
		t.Errorf("expected hasForeignBodyOwner to reject cond1 vs otherCond across foreign construct")
	}
}

func TestComputeAffinity_BodyBlockConsistency(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\tif cond {\n\t\tbody1()\n\t}\n}\n"
	code2 := "package main\n\nfunc f() {\n\tif cond {\n\t\tbody2()\n\t}\n}\n"
	code3 := "package main\n\nfunc f() {\n\tif cond {\n\t\tbody3()\n\t}\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")
	root3 := parseSnippet(t, code3, "test.go")

	ifStmt1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	body1 := findFirstNode(ifStmt1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	ifStmt2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	body2 := findFirstNode(ifStmt2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	ifStmt3 := findFirstNode(root3, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })

	m := NewMapping()
	m.Add(body1, body2)

	w := DefaultAffinityWeights
	scoreForeign := computeAffinity(ifStmt1, ifStmt3, m, w, false)
	if scoreForeign != -1.0 {
		t.Errorf("expected affinity to be -1.0 when candidate has foreign body block, got %f", scoreForeign)
	}

	scoreMatching := computeAffinity(ifStmt1, ifStmt2, m, w, false)
	if scoreMatching <= 0.0 {
		t.Errorf("expected positive affinity for candidate matching mapped body, got %f", scoreMatching)
	}
}

func TestRollupMatchedContainers_WrapperForeignDescendant(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\t_ = (a && (b))\n}\n"
	code2 := "package main\n\nfunc f() {\n\t_ = (a && b)\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	parens1 := findAllNodesByType(root1, "parenthesized_expression")
	outer1 := parens1[0]
	inner1 := parens1[1]

	dstParen := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "parenthesized_expression" })

	leafA1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	leafB1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "b" })
	leafA2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	leafB2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "b" })

	m := NewMapping()
	m.Add(leafA1, leafA2)
	m.Add(leafB1, leafB2)

	if !hasForeignMappedDescendants(inner1, dstParen, m) {
		t.Errorf("expected hasForeignMappedDescendants to be true for inner wrapper vs outer container")
	}

	if hasForeignMappedDescendants(outer1, dstParen, m) {
		t.Errorf("expected hasForeignMappedDescendants to be false for outer wrapper containing all mapped descendants")
	}
}

func TestComputeAffinity_MismatchedDeclarationNames(t *testing.T) {
	makeFunc := func(name string) (*treesitter.ASTNode, *treesitter.ASTNode, []*treesitter.ASTNode) {
		code := fmt.Sprintf("package main\n\nfunc %s() {\n\ts1()\n\ts2()\n\ts3()\n\ts4()\n}\n", name)
		root := parseSnippet(t, code, "test.go")
		decl := findFirstNode(root, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
		body := findFirstNode(decl, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
		stmts := findAllNodesByType(body, "expression_statement")
		return decl, body, stmts
	}

	t.Run("rejects function bodies across different declaration names when similarity < 60%", func(t *testing.T) {
		_, bodyA, stmtsA := makeFunc("funcA")
		_, bodyB, stmtsB := makeFunc("funcB")

		m := NewMapping()
		// Map only 1 of 4 statements (~22% similarity).
		addIsomorphicPairs(stmtsA[0], stmtsB[0], m)

		score := computeAffinity(bodyA, bodyB, m, DefaultAffinityWeights, false)
		if score != -1.0 {
			t.Errorf("expected -1.0 for low-similarity bodies under different function names, got %f", score)
		}
	})

	t.Run("allows function bodies across different declaration names when similarity >= 60%", func(t *testing.T) {
		_, bodyA, stmtsA := makeFunc("funcA")
		_, bodyB, stmtsB := makeFunc("funcB")

		m := NewMapping()
		// Map all 4 statements (>60% similarity).
		for i := range stmtsA {
			addIsomorphicPairs(stmtsA[i], stmtsB[i], m)
		}

		score := computeAffinity(bodyA, bodyB, m, DefaultAffinityWeights, false)
		if score <= 0.0 {
			t.Errorf("expected positive affinity for high-similarity bodies under different function names, got %f", score)
		}
	})

	t.Run("enforces 60% similarity threshold on declarations with different names", func(t *testing.T) {
		declA, _, stmtsA := makeFunc("funcA")
		declB, _, stmtsB := makeFunc("funcB")

		mLow := NewMapping()
		addIsomorphicPairs(stmtsA[0], stmtsB[0], mLow)
		if score := computeAffinity(declA, declB, mLow, DefaultAffinityWeights, false); score != -1.0 {
			t.Errorf("expected -1.0 for low-similarity declarations with different names, got %f", score)
		}

		mHigh := NewMapping()
		for i := range stmtsA {
			addIsomorphicPairs(stmtsA[i], stmtsB[i], mHigh)
		}
		if score := computeAffinity(declA, declB, mHigh, DefaultAffinityWeights, false); score <= 0.0 {
			t.Errorf("expected positive affinity for high-similarity declarations with different names, got %f", score)
		}
	})
}

func TestRollupMatchedContainers_MismatchedDeclarationNames(t *testing.T) {
	buildTree := func(name string) (*treesitter.ASTNode, *treesitter.ASTNode, *treesitter.ASTNode, []*treesitter.ASTNode) {
		code := fmt.Sprintf("package main\n\nfunc %s() {\n\ta()\n\tb()\n\tc()\n\td()\n\te()\n}\n", name)
		root := parseSnippet(t, code, "test.go")
		decl := findFirstNode(root, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
		body := findFirstNode(decl, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
		stmts := findAllNodesByType(body, "expression_statement")
		return root, decl, body, stmts
	}

	// 2 of 5 leaves + body mapped gives ~38% Dice (< 0.60, but >= 0.30).
	root1, declA, bodyA, stmtsA := buildTree("funcA")
	root2, _, bodyB, stmtsB := buildTree("funcB")

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(bodyA, bodyB)
	addIsomorphicPairs(stmtsA[0], stmtsB[0], m)
	addIsomorphicPairs(stmtsA[1], stmtsB[1], m)

	RollupMatchedContainers(root1, root2, m)
	if m.Get(declA) != nil {
		t.Errorf("expected RollupMatchedContainers to reject declA -> declB with mismatched names at <60%% Dice, got %v", m.Get(declA))
	}

	// Same overlap when declaration names match ("funcA" -> "funcA") should roll up (>= 30% Dice).
	rootSame2, declSameA, bodySameA, stmtsSameA := buildTree("funcA")
	mSame := NewMapping()
	mSame.Add(root1, rootSame2)
	mSame.Add(bodyA, bodySameA)
	addIsomorphicPairs(stmtsA[0], stmtsSameA[0], mSame)
	addIsomorphicPairs(stmtsA[1], stmtsSameA[1], mSame)

	RollupMatchedContainers(root1, rootSame2, mSame)
	if mSame.Get(declA) != declSameA {
		t.Errorf("expected RollupMatchedContainers to pair matching declaration names at >=30%% Dice, got %v", mSame.Get(declA))
	}
}

func TestRollupMatchedContainers_EquivalentTypeTargetVariableMismatch(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\toverlap += min(v1, v2)\n}\n"
	code2 := "package main\n\nfunc f() {\n\tmatched := min(v1, v2)\n\t_ = matched\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	assignStmt1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	shortDecl2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "short_var_declaration" })

	callMin1 := findFirstNode(assignStmt1, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	callMin2 := findFirstNode(shortDecl2, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })

	m := NewMapping()
	m.Add(callMin1, callMin2)
	args1 := findFirstNode(callMin1, func(n *treesitter.ASTNode) bool { return n.Type == "argument_list" })
	args2 := findFirstNode(callMin2, func(n *treesitter.ASTNode) bool { return n.Type == "argument_list" })
	if args1 != nil && args2 != nil {
		m.Add(args1, args2)
	}

	RollupMatchedContainers(root1, root2, m)

	if m.Get(assignStmt1) != nil {
		t.Errorf("expected assignStmt1 NOT to roll up to shortDecl2 with different target variable, got: %v", m.Get(assignStmt1))
	}
}

func TestRollupMatchedContainers_SameTypeTargetVariableMismatch(t *testing.T) {
	code1 := "package main\n\nfunc f() {\n\tdstNode = ms\n}\n"
	code2 := "package main\n\nfunc f() {\n\ttoDemote = dstNode\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	assignStmt1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })
	assignStmt2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "assignment_statement" })

	idDst1 := findFirstNode(assignStmt1, func(n *treesitter.ASTNode) bool { return n.Label == "dstNode" })
	idDst2 := findFirstNode(assignStmt2, func(n *treesitter.ASTNode) bool { return n.Label == "dstNode" })

	m := NewMapping()
	m.Add(idDst1, idDst2)

	RollupMatchedContainers(root1, root2, m)

	if m.Get(assignStmt1) != nil {
		t.Errorf("expected assignStmt1 NOT to roll up to assignStmt2 with disjoint target variables, got: %v", m.Get(assignStmt1))
	}
}

func TestHasCallCalleeOrArgMatch_UnextinguishedPeer(t *testing.T) {
	r := rules.Get("go")

	code1 := `package main

func f() {
	addSpan(spansByLine, lineIndex, fileBytes, a)
}
`
	code2 := `package main

func f() {
	_ = absorbSyntacticDelimiters(fileBytes, a)
	addSpan(spansByLine, lineIndex, fileBytes, sb)
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	call1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool {
		return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "addSpan"
	})
	callAbsorb2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool {
		return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "absorbSyntacticDelimiters"
	})
	callAddSpan2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool {
		return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "addSpan"
	})

	argA1 := findFirstNode(call1, func(n *treesitter.ASTNode) bool { return n.Label == "a" })
	argA2 := findFirstNode(callAbsorb2, func(n *treesitter.ASTNode) bool { return n.Label == "a" })

	m := NewMapping()
	m.Add(argA1, argA2)

	// callAbsorb2 shouldn't steal call1 while addSpan is still unmapped in the block.
	if hasCallCalleeOrArgMatch(call1, callAbsorb2, m, r) {
		t.Errorf("expected hasCallCalleeOrArgMatch to be false when unmapped peer with callee 'addSpan' exists in scope")
	}

	// But callAddSpan2 itself should still match call1.
	if !hasCallCalleeOrArgMatch(call1, callAddSpan2, m, r) {
		t.Errorf("expected hasCallCalleeOrArgMatch to be true for call with matching callee name")
	}
}

func TestReconcileBodyBlocks(t *testing.T) {
	t.Run("forward wrapping: re-aligns body block to outer container", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\tif a {\n\t\tx()\n\t}\n}\n"
		code2 := "package main\n\nfunc f() {\n\tif a {\n\t\tif b {\n\t\t\tx()\n\t\t}\n\t}\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		if1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
		body1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

		ifStmts2 := findAllNodesByType(root2, "if_statement")
		if2 := ifStmts2[0]
		innerIf2 := ifStmts2[1]
		body2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
		innerBody2 := findFirstNode(innerIf2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

		m := NewMapping()
		m.Add(if1, if2)
		m.Add(body1, innerBody2)

		reconcileBodyBlocks(root1, m)

		if m.Get(body1) != body2 {
			t.Errorf("expected body1 to be reconciled to outer body2, got %v", m.Get(body1))
		}
	})

	t.Run("reverse unwrapping: re-aligns outer body block when inner was mapped", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\tif a {\n\t\tif b {\n\t\t\tx()\n\t\t}\n\t}\n}\n"
		code2 := "package main\n\nfunc f() {\n\tif a {\n\t\tx()\n\t}\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		ifStmts1 := findAllNodesByType(root1, "if_statement")
		if1 := ifStmts1[0]
		innerIf1 := ifStmts1[1]
		body1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
		innerBody1 := findFirstNode(innerIf1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

		if2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
		body2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

		m := NewMapping()
		m.Add(if1, if2)
		m.Add(innerBody1, body2)

		reconcileBodyBlocks(root1, m)

		if m.Get(body1) != body2 {
			t.Errorf("expected outer body1 to be reconciled to body2, got %v", m.Get(body1))
		}
		if m.Get(innerBody1) != nil {
			t.Errorf("expected innerBody1 to be evicted, got %v", m.Get(innerBody1))
		}
	})
}

func TestHasHeaderMatch(t *testing.T) {
	r := rules.Get("go")

	code1 := "package main\n\nfunc f() {\n\tif a > 0 {\n\t\tx()\n\t}\n}\n"
	code2 := "package main\n\nfunc f() {\n\tif a > 0 {\n\t\tx()\n\t}\n}\n"
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	if1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	cond1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	s1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "expression_statement" })

	if2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	cond2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	s2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "expression_statement" })

	m := NewMapping()
	if hasHeaderMatch(if1, if2, m, r) {
		t.Errorf("expected hasHeaderMatch to be false with empty mapping")
	}

	m.Add(s1, s2)
	if hasHeaderMatch(if1, if2, m, r) {
		t.Errorf("expected hasHeaderMatch to be false when only body nodes match")
	}

	m.Add(cond1, cond2)
	if !hasHeaderMatch(if1, if2, m, r) {
		t.Errorf("expected hasHeaderMatch to be true when header condition matches")
	}
}

func TestHasForeignHeaderOwner(t *testing.T) {
	r := rules.Get("go")

	code1 := `package main

func f() {
	if nilR.IsCaseClause("expression_case") {
		t.Errorf()
	}
	if !IsCaseClause("case_clause") {
		t.Errorf()
	}
}
`
	code2 := `package main

func f() {
	if nilR.IsCaseClause("expression_case") {
		t.Errorf()
	}
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	ifStmts1 := findAllNodesByType(root1, "if_statement")
	if1, if2 := ifStmts1[0], ifStmts1[1]
	cond1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	body1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	body2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	newIf := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	newCond := findFirstNode(newIf, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	newBody := findFirstNode(newIf, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	m := NewMapping()
	m.Add(cond1, newCond)

	if !hasForeignHeaderOwner(if2, newIf, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be true for if2 against newIf whose header is mapped to if1")
	}
	if !hasForeignHeaderOwner(body2, newBody, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be true for body2 against newBody whose parent header is mapped to if1")
	}

	if hasForeignHeaderOwner(if1, newIf, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be false for if1 against newIf")
	}
	if hasForeignHeaderOwner(body1, newBody, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be false for body1 against newBody")
	}
}

func TestHasForeignHeaderOwner_MinorityStraySubexpression(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func f(src, dst, r *int) {
	if src == nil || dst == nil || r == nil {
		foo("old")
	}
	if bar(src, dst) {
		baz(1)
	}
}
`
		dst := `package main

func f(src, dst, r *int) {
	if src == nil || dst == nil {
		foo("new")
	}
	if bar(src, dst) && r == nil {
		baz(2)
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func f(src, dst, r *int)",
			"func f(src, dst, r *int)",
		)

		// 2. Both if statements map in place across minority stray subexpressions
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if src == nil || dst == nil || r == nil {\n\t\tfoo(\"old\")\n\t}",
			"if src == nil || dst == nil {\n\t\tfoo(\"new\")\n\t}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if bar(src, dst) {\n\t\tbaz(1)\n\t}",
			"if bar(src, dst) && r == nil {\n\t\tbaz(2)\n\t}",
		)

		// 3. Whole-snippet verification: calls inside if blocks map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"foo(\"old\")",
			"foo(\"new\")",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"baz(1)",
			"baz(2)",
		)
	})
}

func TestRollupMatchedContainers_ScopePreservedGuard(t *testing.T) {
	code1 := `package main

func test() {
	if !r.IsCaseClause("foo") {
		t.Errorf()
	}
}
`
	code2 := `package main

func test() {
	for i := 0; i < 10; i++ {
		if got := r.IsCaseClause("foo"); got != tt.want {
			t.Errorf()
		}
	}
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	fn1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	fn2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "function_declaration" })
	fnBlock1 := findFirstNode(fn1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })
	fnBlock2 := findFirstNode(fn2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	if1 := findFirstNode(fnBlock1, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	if2 := findFirstNode(fnBlock2, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })

	call1 := findFirstNode(if1, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })
	call2 := findFirstNode(if2, func(n *treesitter.ASTNode) bool { return n.Type == "call_expression" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(fn1, fn2)
	m.Add(fnBlock1, fnBlock2)
	if call1 != nil && call2 != nil {
		m.Add(call1, call2)
	}

	RollupMatchedContainers(fn1, fn2, m)

	if m.Get(if1) == if2 {
		t.Errorf("expected if1 NOT to roll up to if2 across unmapped loop scope boundary")
	}
}

func TestBottomUp_BinaryChainLeadingOperandDeletion(t *testing.T) {
	code1 := `package main

func f(n *Node) bool {
	return isOp(n) || isPunct(n) || n.IsKeyword || isKw(n)
}
`
	code2 := `package main

func f(n *Node) bool {
	return isPunct(n) || n.IsKeyword || isKw(n)
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")

	ret1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "return_statement" })
	ret2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "return_statement" })

	bins1 := findAllNodesByType(ret1, "binary_expression")
	oldBin1 := bins1[0]
	oldBin2 := bins1[1]
	oldBin3 := bins1[2]

	bins2 := findAllNodesByType(ret2, "binary_expression")
	newBin1 := bins2[0]
	newBin2 := bins2[1]

	r := rules.Get("go")
	oldB := findFirstNode(ret1, func(n *treesitter.ASTNode) bool { return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isPunct" })
	newB := findFirstNode(ret2, func(n *treesitter.ASTNode) bool { return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isPunct" })

	oldC := findFirstNode(ret1, func(n *treesitter.ASTNode) bool { return n.Type == "selector_expression" })
	newC := findFirstNode(ret2, func(n *treesitter.ASTNode) bool { return n.Type == "selector_expression" })

	oldD := findFirstNode(ret1, func(n *treesitter.ASTNode) bool { return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isKw" })
	newD := findFirstNode(ret2, func(n *treesitter.ASTNode) bool { return r.IsCall(n.Type) && extractCalleeLabel(n, r) == "isKw" })

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(ret1, ret2)
	m.Add(oldB, newB)
	for i := range oldB.Children {
		m.Add(oldB.Children[i], newB.Children[i])
	}
	m.Add(oldD, newD)
	for i := range oldD.Children {
		m.Add(oldD.Children[i], newD.Children[i])
	}

	BottomUp(root1, root2, m, 0.30)

	if m.Has(oldBin3) {
		t.Errorf("expected inner oldBin3 (isOp || isPunct) to remain unmapped, but mapped to %v", m.Get(oldBin3))
	}
	if m.Get(oldBin2) != newBin2 {
		t.Errorf("expected oldBin2 to match newBin2, got %v", m.Get(oldBin2))
	}
	if m.Get(oldBin1) != newBin1 {
		t.Errorf("expected oldBin1 to match newBin1, got %v", m.Get(oldBin1))
	}
	if m.Get(oldC) != newC {
		t.Errorf("expected oldC to match newC, got %v", m.Get(oldC))
	}
}

func TestBottomUp_NestedCallFlattening_EnclosingConstructGuard(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("go")

		code1 := "package main\n\nfunc f() {\n\t_ = max(foo(a), max(0, diff))\n}\n"
		code2 := "package main\n\nfunc f() {\n\t_ = max(foo(a), 0, diff)\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		var outerMax, innerMax, firstArg *treesitter.ASTNode
		for _, n := range root1.PostOrder() {
			if r.IsCall(n.Type) {
				switch extractCalleeLabel(n, r) {
				case "foo":
					firstArg = n
				case "max":
					if innerMax == nil {
						innerMax = n
					} else {
						outerMax = n
					}
				}
			}
		}

		var newMax, newFirstArg *treesitter.ASTNode
		for _, n := range root2.PostOrder() {
			if r.IsCall(n.Type) {
				switch extractCalleeLabel(n, r) {
				case "foo":
					newFirstArg = n
				case "max":
					newMax = n
				}
			}
		}

		innerZero := findFirstNode(innerMax, func(n *treesitter.ASTNode) bool { return n.Label == "0" })
		innerDiff := findFirstNode(innerMax, func(n *treesitter.ASTNode) bool { return n.Label == "diff" })
		newZero := findFirstNode(newMax, func(n *treesitter.ASTNode) bool { return n.Label == "0" })
		newDiff := findFirstNode(newMax, func(n *treesitter.ASTNode) bool { return n.Label == "diff" })

		m := NewMapping()
		addIsomorphicPairs(firstArg, newFirstArg, m)
		m.Add(innerZero, newZero)
		m.Add(innerDiff, newDiff)

		if !hasEnclosingConstructWithMappedDescendant(innerMax, newMax, m, r) {
			t.Errorf("expected innerMax <-> newMax candidate to be rejected by enclosing construct guard")
		}

		if hasEnclosingConstructWithMappedDescendant(outerMax, newMax, m, r) {
			t.Errorf("expected outerMax <-> newMax candidate to be allowed")
		}

		revM := NewMapping()
		addIsomorphicPairs(newFirstArg, firstArg, revM)
		revM.Add(newZero, innerZero)
		revM.Add(newDiff, innerDiff)

		if !hasEnclosingConstructWithMappedDescendant(newMax, innerMax, revM, r) {
			t.Errorf("expected symmetric candidate newMax <-> innerMax to be rejected")
		}
	})

	t.Run("TopDownBottomUp", func(t *testing.T) {
		oldSrc := []byte(`package main
func f() {
	_ = max(foo(a), max(0, n-m))
}
`)
		newSrc := []byte(`package main
func f() {
	_ = max(foo(a), 0, n-m)
}
`)

		t1, err := treesitter.Parse(oldSrc, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		t2, err := treesitter.Parse(newSrc, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		m := NewMapping()

		TopDown(t1, t2, 2, m, part)
		BottomUp(t1, t2, m, 0.5)

		r1 := rulesFor(t1)
		r2 := rulesFor(t2)

		var oldOuterMax, oldInnerMax, newMax *treesitter.ASTNode
		for _, n := range t1.PostOrder() {
			if r1.IsCall(n.Type) && extractCalleeLabel(n, r1) == "max" {
				if oldInnerMax == nil {
					oldInnerMax = n
				} else {
					oldOuterMax = n
				}
			}
		}
		for _, n := range t2.PostOrder() {
			if r2.IsCall(n.Type) && extractCalleeLabel(n, r2) == "max" {
				newMax = n
			}
		}

		if oldOuterMax == nil || oldInnerMax == nil || newMax == nil {
			t.Fatalf("failed to locate call nodes: outer=%v inner=%v new=%v", oldOuterMax, oldInnerMax, newMax)
		}

		if m.Get(oldInnerMax) == newMax {
			t.Errorf("inner max(0, n-m) stole newMax container; expected inner max to remain unmapped")
		}
		if m.Get(oldOuterMax) != newMax {
			t.Errorf("expected outer max(foo(a), ...) to match newMax, got %v", m.Get(oldOuterMax))
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func f() {
	ts := setup()
	_ = http.Get(fmt.Sprintf("%s/test", ts.URL))
	ts.Close()
}
`
		dst := `package main

func f() {
	ts := setup()
	_ = http.Get(ts.URL + "/test")
	ts.Close()
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

		srcBytes := []byte(src)
		dstBytes := []byte(dst)
		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Target call expression & argument_list container verification
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`http.Get(fmt.Sprintf("%s/test", ts.URL))`,
			`http.Get(ts.URL + "/test")`,
		)
		testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
			`fmt.Sprintf("%s/test", ts.URL)`,
		)

		// 2. Whole-snippet verification: enclosing function declaration and sequential body statements map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func f()",
			"func f()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"ts := setup()",
			"ts := setup()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			`_ = http.Get(fmt.Sprintf("%s/test", ts.URL))`,
			`_ = http.Get(ts.URL + "/test")`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"ts.Close()",
			"ts.Close()",
		)

		// 3. Stationary Operands: ts.URL selector expression maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"ts.URL",
			"ts.URL",
		)
	})
}

func TestBottomUp_InvertedNestedGuardsMatching(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package test

func f() {
	initTask()
	if checkA() && checkB() {
		if condC {
			doAction()
		}
	}
	cleanupTask()
}
`
		newSrc := `package test

func f() {
	initTask()
	if condC {
		if checkA() && checkB() {
			doAction()
		}
	}
	cleanupTask()
}
`
		srcAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, nil)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Target guard expression verification: checkA() && checkB() maps across inverted guards
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"checkA() && checkB()",
			"checkA() && checkB()",
		)

		// 2. Enclosing function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func f()",
			"func f()",
		)

		// 3. Surrounding statements and body call map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"initTask()",
			"initTask()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"cleanupTask()",
			"cleanupTask()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"doAction()",
			"doAction()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"condC",
			"condC",
		)
	})
}

func TestBottomUp_SplitStatementDescendantsRejected(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		runTest := func(t *testing.T, oldSrc, newSrc, oldBin string) {
			srcBytes := []byte(oldSrc)
			dstBytes := []byte(newSrc)
			srcAST, err := treesitter.Parse(srcBytes, "test.go")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dstBytes, "test.go")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}

			part := NewLinePartition(srcBytes, dstBytes)
			mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Target binary expression rejection: compound expression MUST NOT map to dstBin
			testutil.AssertUnmappedText(t, srcAST, srcBytes, mr.Mappings.Src(),
				oldBin,
			)
			testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				oldBin,
				"condA() || !otherCheck()",
			)

			// Stationary / moved sub-expressions
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				"condA()",
				"condA()",
			)
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				"condB()",
				"condB()",
			)

			// 2. Enclosing function maps in place
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				"func f()",
				"func f()",
			)

			// 3. Surrounding sequence: setupContext and finalizeContext map in place
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				"setupContext()",
				"setupContext()",
			)
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				"finalizeContext()",
				"finalizeContext()",
			)
		}

		newSrc := `package test

func f() {
	setupContext()
	if condA() || !otherCheck() {
		return
	}
	if condB() {
		action2()
	}
	finalizeContext()
}
`

		t.Run("StandardOrder", func(t *testing.T) {
			oldSrc := `package test

func f() {
	setupContext()
	if !condA() && condB() {
		action1()
	}
	finalizeContext()
}
`
			runTest(t, oldSrc, newSrc, "!condA() && condB()")
		})

		t.Run("ReversedOrder", func(t *testing.T) {
			oldSrc := `package test

func f() {
	setupContext()
	if condB() && !condA() {
		action1()
	}
	finalizeContext()
}
`
			runTest(t, oldSrc, newSrc, "condB() && !condA()")
		})
	})
}

func TestHasForeignBodyOwner_InvertedNestedGuards(t *testing.T) {
	goRules := rules.Get("go")

	code1 := `package main

func f() {
	if checkA && checkB {
		if condC {
			body()
		}
	}
}
`
	code2 := `package main

func f() {
	if condC {
		if checkA && checkB {
			body()
		}
	}
}
`
	codeParen := `package main

func f() {
	if (checkA && checkB) {
		if condC {
			body()
		}
	}
}
`
	codeUnrelated := `package main

func other() {
	if unrelatedCond {
		unrelatedBody()
	}
}
`
	root1 := parseSnippet(t, code1, "test.go")
	root2 := parseSnippet(t, code2, "test.go")
	rootParen := parseSnippet(t, codeParen, "test.go")
	rootUnrelated := parseSnippet(t, codeUnrelated, "test.go")

	ifStmts1 := findAllNodesByType(root1, "if_statement")
	outerIf1, innerIf1 := ifStmts1[0], ifStmts1[1]
	cond1 := findFirstNode(outerIf1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	condC1 := findFirstNode(innerIf1, func(n *treesitter.ASTNode) bool { return n.Label == "condC" })
	body1 := findFirstNode(innerIf1, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	ifStmts2 := findAllNodesByType(root2, "if_statement")
	outerIf2, innerIf2 := ifStmts2[0], ifStmts2[1]
	condC2 := findFirstNode(outerIf2, func(n *treesitter.ASTNode) bool { return n.Label == "condC" })
	cond2 := findFirstNode(innerIf2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	body2 := findFirstNode(innerIf2, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	m := NewMapping()
	m.Add(body1, body2)
	m.Add(condC1, condC2)

	if hasForeignBodyOwner(cond1, cond2, m, goRules) {
		t.Errorf("expected inverted nested guards to permit matching cond1 vs cond2")
	}

	ifStmtsParen := findAllNodesByType(rootParen, "if_statement")
	outerIfParen, innerIfParen := ifStmtsParen[0], ifStmtsParen[1]
	rawCondParen := findFirstNode(outerIfParen, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
	condCParen := findFirstNode(innerIfParen, func(n *treesitter.ASTNode) bool { return n.Label == "condC" })
	bodyParen := findFirstNode(innerIfParen, func(n *treesitter.ASTNode) bool { return n.Type == "block" })

	mParen := NewMapping()
	mParen.Add(bodyParen, body2)
	mParen.Add(condCParen, condC2)

	if hasForeignBodyOwner(rawCondParen, cond2, mParen, goRules) {
		t.Errorf("expected parenthesized inverted guard to permit matching rawCondParen vs cond2")
	}

	unrelatedIf := findFirstNode(rootUnrelated, func(n *treesitter.ASTNode) bool { return n.Type == "if_statement" })
	unrelatedCond := findFirstNode(unrelatedIf, func(n *treesitter.ASTNode) bool { return n.Label == "unrelatedCond" })

	if !hasForeignBodyOwner(unrelatedCond, cond2, m, goRules) {
		t.Errorf("expected unrelated condition to be rejected by hasForeignBodyOwner")
	}
}

func TestHasSplitStatementDescendant(t *testing.T) {
	goRules := rules.Get("go")

	t.Run("LeftInsideRightOutside", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\t_ = condA && condB\n}\n"
		code2 := "package main\n\nfunc f() {\n\t_ = condA || other\n\t_ = condB\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		bin1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
		bin2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

		d1Left := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d1Right := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })

		d2Left := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d2Outside := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })

		m := NewMapping()
		m.Add(d1Left, d2Left)
		m.Add(d1Right, d2Outside)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject bin1 vs bin2 when operands split across statements")
		}
	})

	t.Run("RightInsideLeftOutside_ReversedOrder", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\t_ = condB && condA\n}\n"
		code2 := "package main\n\nfunc f() {\n\t_ = condA || other\n\t_ = condB\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		bin1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
		bin2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

		d1Left := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })
		d1Right := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })

		d2Inside := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d2Outside := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })

		m := NewMapping()
		m.Add(d1Right, d2Inside)
		m.Add(d1Left, d2Outside)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject bin1 vs bin2 even when outside descendant appears first in pre-order")
		}
	})

	t.Run("ReverseDirection", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\t_ = condA && unused\n\t_ = other\n}\n"
		code2 := "package main\n\nfunc f() {\n\t_ = condA || other\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		bin1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
		bin2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

		d1Left := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d1Outside := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Label == "other" })

		d2Left := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d2Right := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "other" })

		m := NewMapping()
		m.Add(d1Left, d2Left)
		m.Add(d1Outside, d2Right)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject when candidate c has operands split across statements in tree 1")
		}
	})

	t.Run("SameStatementPermitted", func(t *testing.T) {
		code1 := "package main\n\nfunc f() {\n\t_ = condA && condB\n}\n"
		code2 := "package main\n\nfunc f() {\n\t_ = condA || condB\n}\n"
		root1 := parseSnippet(t, code1, "test.go")
		root2 := parseSnippet(t, code2, "test.go")

		bin1 := findFirstNode(root1, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })
		bin2 := findFirstNode(root2, func(n *treesitter.ASTNode) bool { return n.Type == "binary_expression" })

		d1Left := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d1Right := findFirstNode(bin1, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })

		d2Left := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "condA" })
		d2Right := findFirstNode(bin2, func(n *treesitter.ASTNode) bool { return n.Label == "condB" })

		m := NewMapping()
		m.Add(d1Left, d2Left)
		m.Add(d1Right, d2Right)

		if hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to permit expressions whose operands stay in the same statement")
		}
	})
}

func TestBottomUp_SplitStatementDescendant_Bidirectional(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package test

func f() {
	setupContext()
	res := checkA() || checkB()
	finalizeContext(res)
}
`)

		newSrc := []byte(`package test

func f() {
	setupContext()
	if checkA() {
		return
	}
	other := checkB()
	res := other || defaultVal()
	finalizeContext(res)
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		m := mr.Mappings

		// 1. Enclosing Context: Function declaration maps in place.
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"func f()",
			"func f()",
		)

		// 2. Surrounding Sequence: setupContext and finalizeContext map in place.
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"setupContext()",
			"setupContext()",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"finalizeContext(res)",
			"finalizeContext(res)",
		)

		// 3. Target Construct: Split binary expression in old statement must NOT map to destination binary expression.
		testutil.AssertNotMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"checkA() || checkB()",
			"other || defaultVal()",
		)

		// 4. Target Statements: Statements map by variable slot (res :=), but without cross-statement operand moves.
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"res := checkA() || checkB()",
			"res := other || defaultVal()",
		)
	})
}

func TestContestContainers_LinePartition(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package main

func stationaryHelper(val int) int {
	if val > 10 {
		return val * 2
	}
	return val + 1
}

func process(x, y int) int {
	res := 0
	if x > 0 {
		res += x * 10
	}
	return res + y
}
`)
		newSrc := []byte(`package main

func stationaryHelper(val int) int {
	if val > 10 {
		return val * 2
	}
	return val + 1
}

func process(x, y int) int {
	res := 0
	if x > 0 {
		res += x * 20
	}
	return res + y
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "test.go")
		if err != nil {
			t.Fatalf("failed to parse old tree: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.go")
		if err != nil {
			t.Fatalf("failed to parse new tree: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		// 1. Enclosing Context: Both functions must map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"func stationaryHelper(val int) int",
			"func stationaryHelper(val int) int",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"func process(x, y int) int",
			"func process(x, y int) int",
		)

		// 2. Untouched Containers in stationaryHelper: if block and return statements must map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"if val > 10 {\n\t\treturn val * 2\n\t}",
			"if val > 10 {\n\t\treturn val * 2\n\t}",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"return val * 2",
			"return val * 2",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"return val + 1",
			"return val + 1",
		)

		// 3. Surrounding Sequence & Target Containers in process
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"res := 0",
			"res := 0",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"if x > 0 {\n\t\tres += x * 10\n\t}",
			"if x > 0 {\n\t\tres += x * 20\n\t}",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"return res + y",
			"return res + y",
		)
	})
}

func TestBlockWrappedCaseClauseMatching(t *testing.T) {
	t.Run("FullEngine_Rust", func(t *testing.T) {
		oldSrc := []byte(`fn foo() -> Vec<String> {
    let mut ansi_strings = Vec::new();
    match placeholder.placeholder {
        Some("nm") => ansi_strings.push(minus_number_style.paint(format_line_number(
            minus_number,
            alignment_spec,
            width,
            None,
            config,
        ))),
        Some("np") => ansi_strings.push(plus_number_style.paint(format_line_number(
            plus_number,
            alignment_spec,
            width,
            Some(plus_file),
            config,
        ))),
        None => {}
        Some(_) => unreachable!(),
    }
    ansi_strings
}
`)
		newSrc := []byte(`fn foo() -> Vec<String> {
    let mut ansi_strings = Vec::new();
    match placeholder.placeholder {
        Some(Placeholder::NumberMinus) => ansi_strings.push(styles[Minus].paint(
            format_line_number(line_numbers[Minus], alignment_spec, width, None, config),
        )),
        Some(Placeholder::NumberPlus) => {
            ansi_strings.push(styles[Plus].paint(format_line_number(
                line_numbers[Plus],
                alignment_spec,
                width,
                Some(plus_file),
                config,
            )))
        }
        None => {}
        _ => unreachable!("Invalid placeholder"),
    }
    ansi_strings
}
`)
		oldTree, err := treesitter.Parse(oldSrc, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse old tree: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse new tree: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		m := mr.Mappings

		// 1. Enclosing Context: Function declaration maps in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"fn foo() -> Vec<String>", "fn foo() -> Vec<String>")

		// 2. Surrounding Sequence: let binding maps in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"let mut ansi_strings = Vec::new();", "let mut ansi_strings = Vec::new();")

		// 3. Target & Containers:
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"None => {}", "None => {}")

		// 4. Stationary Operands inside Arm 1: alignment_spec, width, plus_file, config
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "alignment_spec", Occurrence: 2},
			testutil.TextQuery{Text: "alignment_spec", Occurrence: 2})
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "width", Occurrence: 2},
			testutil.TextQuery{Text: "width", Occurrence: 2})
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"plus_file", "plus_file")
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "config", Occurrence: 2},
			testutil.TextQuery{Text: "config", Occurrence: 2})
	})

	t.Run("FullEngine_Zig", func(t *testing.T) {
		oldSrc := []byte(`fn testZig(x: u32) void {
    switch (x) {
        1 => push(format(1)),
        2 => push(paint(format(num2, spec, width, opt, config))),
        else => unreachable,
    }
}
`)
		newSrc := []byte(`fn testZig(x: u32) void {
    switch (x) {
        1 => push(format(1)),
        2 => {
            push(paint(format(new_num2, spec, width, opt, config)));
        },
        else => unreachable,
    }
}
`)
		oldTree, err := treesitter.Parse(oldSrc, "test.zig")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.zig")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		m := mr.Mappings

		// Case 2 maps across block
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"1 => push(format(1)),", "1 => push(format(1)),")

		// Stationary operands
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"spec", "spec")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"width", "width")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"opt", "opt")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"config", "config")
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		oldSrc := []byte(`class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> push(paint(format(num2, spec, width, opt, config)));
            default -> unreachable();
        }
    }
}
`)
		newSrc := []byte(`class Test {
    void testJava(int x) {
        switch (x) {
            case 1 -> push(format(1));
            case 2 -> {
                push(paint(format(new_num2, spec, width, opt, config)));
            }
            default -> unreachable();
        }
    }
}
`)
		oldTree, err := treesitter.Parse(oldSrc, "test.java")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.java")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		m := mr.Mappings

		// Java Case 2 maps across block
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"case 1 -> push(format(1));", "case 1 -> push(format(1));")

		// Stationary operands
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"spec", "spec")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"width", "width")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"opt", "opt")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"config", "config")
	})

	t.Run("FullEngine_TypeScript", func(t *testing.T) {
		oldSrc := []byte(`function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2:
            return push(paint(format(num2, spec, width, opt, config)));
        default:
            return unreachable();
    }
}
`)
		newSrc := []byte(`function testTS(x: number) {
    switch (x) {
        case 1:
            return push(format(1));
        case 2: {
            return push(paint(format(new_num2, spec, width, opt, config)));
        }
        default:
            return unreachable();
    }
}
`)
		oldTree, err := treesitter.Parse(oldSrc, "test.ts")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.ts")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		m := mr.Mappings

		// TS Case 2 maps across block
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"return push(format(1));", "return push(format(1));")

		// Stationary operands
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"spec", "spec")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"width", "width")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"opt", "opt")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"config", "config")
	})
}

func TestWrapperEnclosingConstructTheft(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`mod tests {
    use regex::Captures;
    use crate::ansi::strip_ansi_codes;
    use crate::tests::integration_test_utils::integration_test_utils::{
        make_config_from_args, run_delta,
    };
    use super::*;

    fn test_helper() -> bool {
        let _ = make_config_from_args();
        let _ = run_delta();
        true
    }
}
`)
		newSrc := []byte(`mod tests {
    use regex::Captures;
    use crate::ansi::strip_ansi_codes;
    use crate::features::side_by_side::ansifill::ODD_PAD_CHAR;
    use crate::format::FormatStringData;
    use crate::tests::integration_test_utils::{make_config_from_args, run_delta};
    use super::*;

    fn test_helper() -> bool {
        let _ = make_config_from_args();
        let _ = run_delta();
        true
    }
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse old tree: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "test.rs")
		if err != nil {
			t.Fatalf("failed to parse new tree: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(oldTree, newTree)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil result")
		}

		m := mr.Mappings

		// 1. Enclosing Context: Both mod tests and fn test_helper must map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"mod tests", "mod tests")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"fn test_helper() -> bool", "fn test_helper() -> bool")

		// 2. Surrounding Sequence: Preceding and succeeding use declarations and statements must map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"use regex::Captures;", "use regex::Captures;")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"use crate::ansi::strip_ansi_codes;", "use crate::ansi::strip_ansi_codes;")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"use super::*;", "use super::*;")

		// Verify statements inside fn test_helper map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"let _ = make_config_from_args();", "let _ = make_config_from_args();")
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			"let _ = run_delta();", "let _ = run_delta();")

		// 3. Stationary Operands: Identifiers in use_list map in place
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "make_config_from_args", Occurrence: 1},
			testutil.TextQuery{Text: "make_config_from_args", Occurrence: 1})
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "run_delta", Occurrence: 1},
			testutil.TextQuery{Text: "run_delta", Occurrence: 1})
	})
}

// TestBottomUp_CallChain_HollowEnclosingGuard verifies that outer enclosing calls in a
// call chain do not steal matches from compatible unmapped inner calls.
func TestBottomUp_CallChain_HollowEnclosingGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust", func(t *testing.T) {
			oldSrc := []byte(`fn paint() {
    for (line_index, (syntax_sections, diff_sections)) in syntax_style_sections
        .iter()
        .zip_eq(diff_style_sections.iter())
        .enumerate()
    {
        foo();
    }
}
`)
			newSrc := []byte(`fn paint() {
    for (line_index, ((syntax_sections, diff_sections), state)) in syntax_style_sections
        .into_iter()
        .zip_eq(diff_style_sections.iter())
        .zip_eq(states.into_iter())
        .enumerate()
    {
        foo();
    }
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

			// Enclosing function declaration mapped in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"fn paint()", "fn paint()")

			// Surrounding for loop mapped in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"foo();", "foo();")

			// Target calls: old zip_eq should match inner zip_eq, leaving inserted outer unmapped.
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				testutil.TextQuery{Text: "zip_eq", Occurrence: 1},
				testutil.TextQuery{Text: "zip_eq", Occurrence: 1},
			)
			testutil.AssertUnmappedTextQuery(t, dstAST, newSrc, mr.Mappings.Dst(),
				testutil.TextQuery{Text: "zip_eq", Occurrence: 2},
			)

			// Arguments map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"diff_style_sections.iter()", "diff_style_sections.iter()")
		})

		t.Run("Go", func(t *testing.T) {
			oldSrc := []byte(`package main

func paint() {
	syntax_style_sections.
		Iter().
		ZipEq(diff_style_sections.Iter()).
		Enumerate()
}
`)
			newSrc := []byte(`package main

func paint() {
	syntax_style_sections.
		IntoIter().
		ZipEq(diff_style_sections.Iter()).
		ZipEq(states.IntoIter()).
		Enumerate()
}
`)

			srcAST, err := treesitter.Parse(oldSrc, "test.go")
			if err != nil {
				t.Fatalf("parse oldSrc: %v", err)
			}
			dstAST, err := treesitter.Parse(newSrc, "test.go")
			if err != nil {
				t.Fatalf("parse newSrc: %v", err)
			}
			if srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("oldSrc has parse errors: %d", srcAST.ParseErrorCount)
			}
			if dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("newSrc has parse errors: %d", dstAST.ParseErrorCount)
			}

			part := NewLinePartition(oldSrc, newSrc)
			mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil mappings")
			}

			// Enclosing function declaration mapped in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"func paint()", "func paint()")

			// Surrounding expression statement mapped in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"Enumerate()", "Enumerate()")

			// Target calls: old ZipEq should match inner ZipEq, leaving inserted outer unmapped.
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				testutil.TextQuery{Text: "ZipEq", Occurrence: 1},
				testutil.TextQuery{Text: "ZipEq", Occurrence: 1},
			)
			testutil.AssertUnmappedTextQuery(t, dstAST, newSrc, mr.Mappings.Dst(),
				testutil.TextQuery{Text: "ZipEq", Occurrence: 2},
			)

			// Arguments map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
				"diff_style_sections.Iter()", "diff_style_sections.Iter()")
		})
	})
}

func TestHasForeignScopeConflict_SameScopeExemption(t *testing.T) {
	src, err := treesitter.Parse([]byte(`fn try_pager() {
    own(); extracted_one(); extracted_two();
}
fn helper() {}
`), "test.rs")
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	dst, err := treesitter.Parse([]byte(`fn try_pager() {
    own();
}
fn helper() { extracted_one(); extracted_two(); }
`), "test.rs")
	if err != nil {
		t.Fatalf("parse destination: %v", err)
	}
	if src.HasError || dst.HasError {
		t.Fatalf("parse errors: source=%d, destination=%d", src.ParseErrorCount, dst.ParseErrorCount)
	}

	r := rulesFor(src)
	m := NewMapping()
	for _, name := range []string{"own", "extracted_one", "extracted_two"} {
		oldCall := findFirstNode(src.Root(), func(n *treesitter.ASTNode) bool {
			return r.IsCall(n.Type) && extractCalleeLabel(n, r) == name
		})
		newCall := findFirstNode(dst.Root(), func(n *treesitter.ASTNode) bool {
			return r.IsCall(n.Type) && extractCalleeLabel(n, r) == name
		})
		if oldCall == nil || newCall == nil {
			t.Fatalf("find %s calls: source=%v, destination=%v", name, oldCall, newCall)
		}
		m.Add(oldCall.Children[0], newCall.Children[0])
	}

	oldBlock := findFirstNode(src.Root(), func(n *treesitter.ASTNode) bool {
		return r.IsBlock(n.Type)
	})
	newBlock := findFirstNode(dst.Root(), func(n *treesitter.ASTNode) bool {
		return r.IsBlock(n.Type)
	})
	m.Add(GetEnclosingDeclaration(oldBlock), GetEnclosingDeclaration(newBlock))
	if hasForeignScopeConflict(oldBlock, newBlock, m) {
		t.Error("extracting code from the same function must not be a foreign-scope conflict")
	}

	t.Run("SameNameDifferentNamespace", func(t *testing.T) {
		src, err := treesitter.Parse([]byte(`mod alpha {
    fn process() {
        own(); extracted_one(); extracted_two();
    }
}
mod beta {
    fn process() {
        other();
    }
}
`), "test.rs")
		if err != nil {
			t.Fatalf("parse source: %v", err)
		}
		dst, err := treesitter.Parse([]byte(`mod alpha {
    fn process() {
        own();
    }
}
mod beta {
    fn process() {
        extracted_one(); extracted_two(); other();
    }
}
`), "test.rs")
		if err != nil {
			t.Fatalf("parse destination: %v", err)
		}
		if src.HasError || dst.HasError {
			t.Fatalf("parse errors: source=%d, destination=%d", src.ParseErrorCount, dst.ParseErrorCount)
		}

		m := NewMapping()
		for _, name := range []string{"own", "extracted_one", "extracted_two"} {
			oldCall := findFirstNode(src.Root(), func(n *treesitter.ASTNode) bool {
				return r.IsCall(n.Type) && extractCalleeLabel(n, r) == name
			})
			newCall := findFirstNode(dst.Root(), func(n *treesitter.ASTNode) bool {
				return r.IsCall(n.Type) && extractCalleeLabel(n, r) == name
			})
			if oldCall == nil || newCall == nil {
				t.Fatalf("find %s calls: source=%v, destination=%v", name, oldCall, newCall)
			}
			m.Add(oldCall.Children[0], newCall.Children[0])
		}
		oldAlphaBlock := findFirstNode(src.Root(), func(n *treesitter.ASTNode) bool {
			return r.IsBlock(n.Type)
		})
		newAlphaBlock := findFirstNode(dst.Root(), func(n *treesitter.ASTNode) bool {
			return r.IsBlock(n.Type)
		})
		if !hasForeignScopeConflict(oldAlphaBlock, newAlphaBlock, m) {
			t.Error("same-named functions in different modules are separate scopes")
		}
	})
}

func TestBottomUp_CrossScopeMatchingGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `fn try_pager(config: &Config) -> Result<()> {
    let mut replace_args = false;
    match pagerflags.split_first() {
        Some((name, args)) => {
            let mut process = if is_less {
                let mut p = Command::new(&name);
                p.args(args);
                p.env("UTF8");
                Ok(p.spawn())
            } else {
                None
            };
        }
        None => Ok(())
    }
}
`
		dst := `fn try_pager(config: &Config) -> Result<()> {
    let mut replace_args = false;
    Ok(match pagerflags.split_first() {
        Some((name, args)) => {
            let process = if is_less {
                _make_process(name, args)
            } else {
                None
            };
        }
        None => ()
    })
}

fn _make_process(name: String, args: Vec<String>) -> Option<Command> {
    if let Ok(path) = resolve_binary(name) {
        let mut p = Command::new(&path);
        p.args(args);
        p.env("UTF8");
        Some(p)
    } else {
        None
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: fn try_pager
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn try_pager(config: &Config) -> Result<()>", "fn try_pager(config: &Config) -> Result<()>")

		// 2. Surrounding sequence: let mut replace_args
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"let mut replace_args = false;", "let mut replace_args = false;")

		// 3. Target: outer expression statement in src should NOT map to
		// outer statement of _make_process in dst.
		testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"match pagerflags.split_first()", "if let Ok(path) = resolve_binary(name)")

		// 4. Extracted statement: let mut p = Command::new(&name) should map to let mut p = Command::new(&path)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"let mut p = Command::new(&name);", "let mut p = Command::new(&path);")
	})
}

func TestRollupMatchedContainers_HeaderMatchUnmappedScope(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `fn outer_helper() -> usize {
    42
}

fn get_right_fill_style_for_left_panel(
    line_is_empty: bool,
    line_index: Option<usize>,
    diff_style_sections: &[Vec<(usize, &str)>],
    config: &Config,
) -> usize {
    match (line_is_empty, line_index) {
        (true, _) => config.null_style,
        (false, None) => config.null_style,
        (false, Some(index)) => {
            let (should_fill, fill_style) =
                compute_fill(
                    &diff_style_sections[index],
                    config,
                );
            if should_fill {
                fill_style
            } else {
                config.null_style
            }
        }
    }
}
`
		dst := `fn outer_helper() -> usize {
    42
}

fn get_right_fill_style_for_panel<'a>(
    line_is_empty: bool,
    line_index: Option<usize>,
    diff_style_sections: &[Vec<(usize, &str)>],
    config: &Config,
) -> (Option<usize>, usize) {
    let none_or_override = None;
    match (line_is_empty, line_index) {
        (true, _) => (none_or_override, config.null_style),
        (false, None) => (none_or_override, config.null_style),
        (false, Some(index)) => {
            let (bg_fill_mode, fill_style) =
                compute_fill(
                    &diff_style_sections[index],
                    config,
                );

            match bg_fill_mode {
                None => (none_or_override, config.null_style),
                Some(1) => (Some(1), fill_style),
                _ => (bg_fill_mode, fill_style),
            }
        }
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// Enclosing context: outer_helper shouldn't be affected by changes inside the match.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn outer_helper() -> usize", "fn outer_helper() -> usize")

		// Surrounding sequence: arm 0 and arm 1 are untouched, so they stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"(true, _) => config.null_style,", "(true, _) => (none_or_override, config.null_style),")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"(false, None) => config.null_style,", "(false, None) => (none_or_override, config.null_style),")

		// Target construct: arm 2 (false, Some(index)) should map in place despite the nested match rewrite.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"(false, Some(index)) =>", "(false, Some(index)) =>")

		// Stationary operands: compute_fill call inside arm 2 maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"compute_fill(", "compute_fill(")
	})
}

func TestBottomUpShortenedCompoundIdentifierMatching(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `#[cfg(test)]
mod tests {
    use crate::ansi::{self, strip_ansi_codes};
    use crate::cli::InspectRawLines;
    use crate::delta::State;
    use crate::style;
    use crate::tests::ansi_test_utils::ansi_test_utils;
    use crate::tests::integration_test_utils::integration_test_utils;
    use crate::tests::test_utils::test_utils;

    #[test]
    fn test_added_file() {
        let config = integration_test_utils::make_config_from_args(&[]);
        let output = integration_test_utils::run_delta(ADDED_FILE_INPUT, &config);
        let output = strip_ansi_codes(&output);
        assert!(output.contains("\nadded: a.py\n"));
    }
}
`
		dst := `#[cfg(test)]
mod tests {
    use crate::ansi::{self, strip_ansi_codes};
    use crate::cli::InspectRawLines;
    use crate::delta::State;
    use crate::style;
    use crate::tests::ansi_test_utils::ansi_test_utils;
    use crate::tests::integration_test_utils;
    use crate::tests::test_utils;

    #[test]
    fn test_added_file() {
        let config = integration_test_utils::make_config_from_args(&[]);
        let output = integration_test_utils::run_delta(ADDED_FILE_INPUT, &config);
        let output = strip_ansi_codes(&output);
        assert!(output.contains("\nadded: a.py\n"));
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: mod tests and its declaration_list body map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"mod tests", "mod tests")

		// 2. Surrounding sequence: preceding unchanged imports map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::ansi::{self, strip_ansi_codes};", "use crate::ansi::{self, strip_ansi_codes};")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::cli::InspectRawLines;", "use crate::cli::InspectRawLines;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::delta::State;", "use crate::delta::State;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::style;", "use crate::style;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::tests::ansi_test_utils::ansi_test_utils;", "use crate::tests::ansi_test_utils::ansi_test_utils;")

		// Succeeding function test_added_file maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn test_added_file()", "fn test_added_file()")

		// 3. Target & Containers: both shortened use_declaration nodes must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::tests::integration_test_utils::integration_test_utils;", "use crate::tests::integration_test_utils;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"use crate::tests::test_utils::test_utils;", "use crate::tests::test_utils;")

		// 4. Stationary operands: shared prefix scoped_identifier must map to destination root scoped_identifier
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"crate::tests::integration_test_utils", "crate::tests::integration_test_utils")
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		src := `package com.example;

import com.example.foo.bar.baz;
import com.example.alpha.beta.gamma;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`
		dst := `package com.example;

import com.example.foo.bar;
import com.example.alpha.beta;

public class Main {
    public static void main(String[] args) {
        System.out.println("hello");
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "Main.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "Main.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: class Main maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"public class Main", "public class Main")

		// 2. Targets: shortened import_declaration nodes must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"import com.example.foo.bar.baz;", "import com.example.foo.bar;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"import com.example.alpha.beta.gamma;", "import com.example.alpha.beta;")
	})

	t.Run("FullEngine_Lua", func(t *testing.T) {
		src := `function process()
    foo.bar.baz()
    alpha.beta.gamma()
    local x = 1
end
`
		dst := `function process()
    foo.bar()
    alpha.beta()
    local x = 1
end
`
		srcAST, err := treesitter.Parse([]byte(src), "test.lua")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.lua")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function process maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"function process()", "function process()")

		// 2. Surrounding sequence: local x = 1 maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"local x = 1", "local x = 1")

		// 3. Targets: function_call nodes map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"foo.bar.baz()", "foo.bar()")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"alpha.beta.gamma()", "alpha.beta()")
	})

	t.Run("FullEngine_Zig", func(t *testing.T) {
		src := `pub fn setup() void {
    const a = @import("foo").bar.baz;
    const b = @import("alpha").beta.gamma;
    const c = 1;
}
`
		dst := `pub fn setup() void {
    const a = @import("foo").bar;
    const b = @import("alpha").beta;
    const c = 1;
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.zig")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.zig")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: fn setup maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"pub fn setup() void", "pub fn setup() void")

		// 2. Surrounding sequence: const c = 1 maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"const c = 1;", "const c = 1;")

		// 3. Targets: variable_declaration nodes map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"const a = @import(\"foo\").bar.baz;", "const a = @import(\"foo\").bar;")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"const b = @import(\"alpha\").beta.gamma;", "const b = @import(\"alpha\").beta;")
	})
}

func TestBottomUp_HollowEnclosingCall_WrappedMacroEvolution(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`pub fn parse_color(s: &str) {
    let die = || {
        eprintln!("Invalid color or style attribute: {}", s);
        process::exit(1);
    };
    println!("color: {}", s);
}
`)
		dst := []byte(`pub fn parse_color(s: &str) {
    let die = || {
        fatal(format!("Invalid color or style attribute: {}", s));
    };
    println!("color: {}", s);
}
`)
		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: parse_color function and die closure map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"pub fn parse_color(s: &str)", "pub fn parse_color(s: &str)")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let die = ||", "let die = ||")

		// 2. Surrounding sequence: succeeding println! statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"println!(\"color: {}\", s);", "println!(\"color: {}\", s);")

		// 3. Target: eprintln! must map to format!, NOT the outer call fatal
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"eprintln!(\"Invalid color or style attribute: {}\", s)", "format!(\"Invalid color or style attribute: {}\", s)")

		// 4. Stationary Operands: token_tree maps in place across eprintln! -> format!
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"(\"Invalid color or style attribute: {}\", s)", "(\"Invalid color or style attribute: {}\", s)")
	})
}

func TestBottomUp_HollowEnclosingCall_WrappedCallEvolution(t *testing.T) {
	t.Run("FullEngine/Go", func(t *testing.T) {
		src := []byte(`package main

func process(s string) {
	render("Invalid attribute: " + s)
	println(s)
}
`)
		dst := []byte(`package main

func process(s string) {
	die(format("Invalid attribute: " + s))
	println(s)
}
`)
		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: process function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"func process(s string)", "func process(s string)")

		// 2. Surrounding sequence: succeeding println statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"println(s)", "println(s)")

		// 3. Target: render call must map to format, not outer wrapper die
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"render(\"Invalid attribute: \" + s)", "format(\"Invalid attribute: \" + s)")

		// 4. Stationary operands: binary expression argument maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"\"Invalid attribute: \" + s", "\"Invalid attribute: \" + s")
	})

	t.Run("FullEngine/Java", func(t *testing.T) {
		src := []byte(`class Test {
    void process(String s) {
        render("Invalid attribute: " + s);
        println(s);
    }
}
`)
		dst := []byte(`class Test {
    void process(String s) {
        die(format("Invalid attribute: " + s));
        println(s);
    }
}
`)
		srcAST, err := treesitter.Parse(src, "test.java")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.java")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: class and method process map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"class Test", "class Test")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"void process(String s)", "void process(String s)")

		// 2. Surrounding sequence: succeeding println statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"println(s);", "println(s);")

		// 3. Target: render call maps to inner format, not outer die
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"render(\"Invalid attribute: \" + s)", "format(\"Invalid attribute: \" + s)")

		// 4. Stationary operands: binary_expression maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"\"Invalid attribute: \" + s", "\"Invalid attribute: \" + s")
	})

	t.Run("FullEngine/Zig", func(t *testing.T) {
		src := []byte(`fn process(s: []const u8) void {
    render("Invalid attribute: {s}", .{s});
    println(s);
}
`)
		dst := []byte(`fn process(s: []const u8) void {
    die(format("Invalid attribute: {s}", .{s}));
    println(s);
}
`)
		srcAST, err := treesitter.Parse(src, "test.zig")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.zig")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: process function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn process(s: []const u8) void", "fn process(s: []const u8) void")

		// 2. Surrounding sequence: succeeding println call maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"println(s);", "println(s);")

		// 3. Target: render call maps to inner format, not outer die
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"render(\"Invalid attribute: {s}\", .{s})", "format(\"Invalid attribute: {s}\", .{s})")

		// 4. Stationary operands: string literal maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"\"Invalid attribute: {s}\"", "\"Invalid attribute: {s}\"")
	})
}

func TestBottomUp_HollowSelectorOuterInnerTheft(t *testing.T) {
	t.Run("FullEngine/Rust", func(t *testing.T) {
		src := []byte(`fn format_panel(config: &Config) {
    let header = config.header.title.clone();
    let panel_width = config.side_by_side_data.left_panel.width;
    println!("width: {}", panel_width);
}
`)
		dst := []byte(`fn format_panel(config: &Config) {
    let header = config.header.title.clone();
    let panel_width = config.side_by_side_data[panel_side].width;
    println!("width: {}", panel_width);
}
`)
		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: format_panel function and preceding header assignment map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn format_panel(config: &Config)",
			"fn format_panel(config: &Config)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let header = config.header.title.clone();",
			"let header = config.header.title.clone();",
		)

		// 2. Surrounding Sequence: succeeding println! statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`println!("width: {}", panel_width);`,
			`println!("width: {}", panel_width);`,
		)

		// 3. Target & Containers: outer field_expression (.width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.side_by_side_data.left_panel.width",
			"config.side_by_side_data[panel_side].width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"left_panel",
		)

		// 4. Stationary Operands: base field_expression config.side_by_side_data maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.side_by_side_data",
			"config.side_by_side_data",
		)
	})

	t.Run("FullEngine/Go", func(t *testing.T) {
		src := []byte(`package main

func formatPanel(config *Config) {
	header := config.header.title
	panelWidth := config.side_by_side_data.left_panel.width
	println("width:", panelWidth)
}
`)
		dst := []byte(`package main

func formatPanel(config *Config) {
	header := config.header.title
	panelWidth := config.side_by_side_data[panel_side].width
	println("width:", panelWidth)
}
`)
		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: formatPanel function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"func formatPanel(config *Config)",
			"func formatPanel(config *Config)",
		)

		// 2. Surrounding Sequence: succeeding println call maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`println("width:", panelWidth)`,
			`println("width:", panelWidth)`,
		)

		// 3. Target & Containers: outer selector_expression (.width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.side_by_side_data.left_panel.width",
			"config.side_by_side_data[panel_side].width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"left_panel",
		)
	})

	t.Run("FullEngine/TypeScript", func(t *testing.T) {
		src := []byte(`function formatPanel(config: Config) {
    const header = config.header.title;
    const panelWidth = config.sideBySideData.leftPanel.width;
    console.log("width:", panelWidth);
}
`)
		dst := []byte(`function formatPanel(config: Config) {
    const header = config.header.title;
    const panelWidth = config.sideBySideData[panelSide].width;
    console.log("width:", panelWidth);
}
`)
		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: formatPanel function and preceding header assignment map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"function formatPanel(config: Config)",
			"function formatPanel(config: Config)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"const header = config.header.title;",
			"const header = config.header.title;",
		)

		// 2. Surrounding Sequence: succeeding console.log call maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`console.log("width:", panelWidth);`,
			`console.log("width:", panelWidth);`,
		)

		// 3. Target & Containers: outer member_expression (.width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.sideBySideData.leftPanel.width",
			"config.sideBySideData[panelSide].width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"leftPanel",
		)
	})

	t.Run("FullEngine/Python", func(t *testing.T) {
		src := []byte(`def format_panel(config):
    header = config.header.title
    panel_width = config.side_by_side_data.left_panel.width
    print("width:", panel_width)
`)
		dst := []byte(`def format_panel(config):
    header = config.header.title
    panel_width = config.side_by_side_data[panel_side].width
    print("width:", panel_width)
`)
		srcAST, err := treesitter.Parse(src, "test.py")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.py")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: format_panel function and preceding header assignment map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"def format_panel(config):",
			"def format_panel(config):",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"header = config.header.title",
			"header = config.header.title",
		)

		// 2. Surrounding Sequence: succeeding print statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`print("width:", panel_width)`,
			`print("width:", panel_width)`,
		)

		// 3. Target & Containers: outer attribute (.width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.side_by_side_data.left_panel.width",
			"config.side_by_side_data[panel_side].width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"left_panel",
		)
	})

	t.Run("FullEngine/Java", func(t *testing.T) {
		src := []byte(`class PanelFormatter {
    void formatPanel(Config config) {
        String header = config.header.title;
        int panelWidth = config.side_by_side_data.left_panel.width;
        System.out.println("width: " + panelWidth);
    }
}
`)
		dst := []byte(`class PanelFormatter {
    void formatPanel(Config config) {
        String header = config.header.title;
        int panelWidth = config.side_by_side_data[panel_side].width;
        System.out.println("width: " + panelWidth);
    }
}
`)
		srcAST, err := treesitter.Parse(src, "Test.java")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "Test.java")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: method formatPanel and preceding header declaration map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"void formatPanel(Config config)",
			"void formatPanel(Config config)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"String header = config.header.title;",
			"String header = config.header.title;",
		)

		// 2. Surrounding Sequence: succeeding println statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`System.out.println("width: " + panelWidth);`,
			`System.out.println("width: " + panelWidth);`,
		)

		// 3. Target & Containers: outer field_access (.width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config.side_by_side_data.left_panel.width",
			"config.side_by_side_data[panel_side].width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"left_panel",
		)
	})

	t.Run("FullEngine/CArrow", func(t *testing.T) {
		src := []byte(`void format_panel(Config *config) {
    char *header = config->header->title;
    int panel_width = config->side_by_side_data->left_panel->width;
    printf("width: %d\n", panel_width);
}
`)
		dst := []byte(`void format_panel(Config *config) {
    char *header = config->header->title;
    int panel_width = config->side_by_side_data[panel_side]->width;
    printf("width: %d\n", panel_width);
}
`)
		srcAST, err := treesitter.Parse(src, "test.c")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.c")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: function format_panel and preceding header declaration map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"void format_panel(Config *config)",
			"void format_panel(Config *config)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"char *header = config->header->title;",
			"char *header = config->header->title;",
		)

		// 2. Surrounding Sequence: succeeding printf statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`printf("width: %d\n", panel_width);`,
			`printf("width: %d\n", panel_width);`,
		)

		// 3. Target & Containers: outer field_expression (->width) must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"config->side_by_side_data->left_panel->width",
			"config->side_by_side_data[panel_side]->width",
		)
		testutil.AssertUnmappedText(t, srcAST, src, mr.Mappings.Src(),
			"left_panel",
		)
	})
}

func TestRollupMatchedContainers_ElseClause_FullEngine(t *testing.T) {
	tests := []struct {
		name            string
		lang            string
		filename        string
		src             string
		dst             string
		targetFunc      string
		fnType          string
		totalType       string
		elseType        string
		elseBodyType    string
		elseBodyLine    int
		dstElseLine     int
		dstElseBodyLine int
		srcMetricLine   int
		dstBatchLine    int
		dstMetricLine   int
	}{
		{
			name:            "Rust",
			lang:            "rust",
			filename:        "test.rs",
			fnType:          "function_item",
			totalType:       "let_declaration",
			elseType:        "else_clause",
			elseBodyType:    "block",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    30,
			dstMetricLine:   32,
			src: `fn process_records(flag: bool, count: i32) {
    let mut total = 0;
    if flag {
        total += 1;
    } else {
        finish_records(count);
        handle_batch(count);
    }
    record_metric(total);
}
`,
			dst: `fn process_records(flag: bool, count: i32) {
    let mut total = 0;
    validate_flags(flag);
    check_active_context();
    audit_preconditions();
    init_buffer_cache();
    prepare_worker_pool();
    reset_staged_entries();
    verify_system_clock();
    ensure_capacity(count);
    sync_storage_metrics();
    prepare_runtime();
    allocate_channel();
    if flag {
        total += 1;
    } else {
        match as_singleton_list(count) {
            Some(items) => {
                let mut split = split_records(&items, count);
                if split.len() > 1 {
                    process_all(&mut split);
                } else {
                    record_fallback(count);
                }
            }
            None => {
                record_fallback(count);
            }
        }
        handle_batch(count);
    }
    record_metric(total);
}
`,
			targetFunc: "process_records",
		},
		{
			name:            "C",
			lang:            "c",
			filename:        "test.c",
			fnType:          "function_definition",
			totalType:       "declaration",
			elseType:        "else_clause",
			elseBodyType:    "compound_statement",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    33,
			dstMetricLine:   35,
			src: `void process_records(int flag, int count) {
    int total = 0;
    if (flag) {
        total += 1;
    } else {
        finish_records(count);
        handle_batch(count);
    }
    record_metric(total);
}
`,
			dst: `void process_records(int flag, int count) {
    int total = 0;
    validate_flags(flag);
    check_active_context();
    audit_preconditions();
    init_buffer_cache();
    prepare_worker_pool();
    reset_staged_entries();
    verify_system_clock();
    ensure_capacity(count);
    sync_storage_metrics();
    prepare_runtime();
    allocate_channel();
    if (flag) {
        total += 1;
    } else {
        if (check_extra_threshold(count)) {
            split_and_process(count);
            log_debug_batch(count);
            record_entry(1);
            record_entry(2);
            record_entry(3);
            record_entry(4);
        } else if (check_secondary_threshold(count)) {
            record_fallback(count);
            apply_single_entry(count);
            record_entry(5);
            record_entry(6);
        } else {
            notify_worker_pool(count);
            flush_intermediate_state(count);
        }
        handle_batch(count);
    }
    record_metric(total);
}
`,
			targetFunc: "process_records",
		},
		{
			name:            "Cpp",
			lang:            "cpp",
			filename:        "test.cpp",
			fnType:          "function_definition",
			totalType:       "declaration",
			elseType:        "else_clause",
			elseBodyType:    "compound_statement",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    33,
			dstMetricLine:   35,
			src: `void process_records(bool flag, int count) {
    int total = 0;
    if (flag) {
        total += 1;
    } else {
        finish_records(count);
        handle_batch(count);
    }
    record_metric(total);
}
`,
			dst: `void process_records(bool flag, int count) {
    int total = 0;
    validate_flags(flag);
    check_active_context();
    audit_preconditions();
    init_buffer_cache();
    prepare_worker_pool();
    reset_staged_entries();
    verify_system_clock();
    ensure_capacity(count);
    sync_storage_metrics();
    prepare_runtime();
    allocate_channel();
    if (flag) {
        total += 1;
    } else {
        if (check_extra_threshold(count)) {
            split_and_process(count);
            log_debug_batch(count);
            record_entry(1);
            record_entry(2);
            record_entry(3);
            record_entry(4);
        } else if (check_secondary_threshold(count)) {
            record_fallback(count);
            apply_single_entry(count);
            record_entry(5);
            record_entry(6);
        } else {
            notify_worker_pool(count);
            flush_intermediate_state(count);
        }
        handle_batch(count);
    }
    record_metric(total);
}
`,
			targetFunc: "process_records",
		},
		{
			name:            "JavaScript",
			lang:            "javascript",
			filename:        "test.js",
			fnType:          "function_declaration",
			totalType:       "lexical_declaration",
			elseType:        "else_clause",
			elseBodyType:    "statement_block",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    33,
			dstMetricLine:   35,
			src: `function processRecords(flag, count) {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        finishRecords(count);
        handleBatch(count);
    }
    recordMetric(total);
}
`,
			dst: `function processRecords(flag, count) {
    let total = 0;
    validateFlags(flag);
    checkActiveContext();
    auditPreconditions();
    initBufferCache();
    prepareWorkerPool();
    resetStagedEntries();
    verifySystemClock();
    ensureCapacity(count);
    syncStorageMetrics();
    prepareRuntime();
    allocateChannel();
    if (flag) {
        total += 1;
    } else {
        if (checkExtraThreshold(count)) {
            splitAndProcess(count);
            logDebugBatch(count);
            recordEntry(1);
            recordEntry(2);
            recordEntry(3);
            recordEntry(4);
        } else if (checkSecondaryThreshold(count)) {
            recordFallback(count);
            applySingleEntry(count);
            recordEntry(5);
            recordEntry(6);
        } else {
            notifyWorkerPool(count);
            flushIntermediateState(count);
        }
        handleBatch(count);
    }
    recordMetric(total);
}
`,
			targetFunc: "processRecords",
		},
		{
			name:            "TypeScript",
			lang:            "typescript",
			filename:        "test.ts",
			fnType:          "function_declaration",
			totalType:       "lexical_declaration",
			elseType:        "else_clause",
			elseBodyType:    "statement_block",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    33,
			dstMetricLine:   35,
			src: `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    if (flag) {
        total += 1;
    } else {
        finishRecords(count);
        handleBatch(count);
    }
    recordMetric(total);
}
`,
			dst: `function processRecords(flag: boolean, count: number): void {
    let total = 0;
    validateFlags(flag);
    checkActiveContext();
    auditPreconditions();
    initBufferCache();
    prepareWorkerPool();
    resetStagedEntries();
    verifySystemClock();
    ensureCapacity(count);
    syncStorageMetrics();
    prepareRuntime();
    allocateChannel();
    if (flag) {
        total += 1;
    } else {
        if (checkExtraThreshold(count)) {
            splitAndProcess(count);
            logDebugBatch(count);
            recordEntry(1);
            recordEntry(2);
            recordEntry(3);
            recordEntry(4);
        } else if (checkSecondaryThreshold(count)) {
            recordFallback(count);
            applySingleEntry(count);
            recordEntry(5);
            recordEntry(6);
        } else {
            notifyWorkerPool(count);
            flushIntermediateState(count);
        }
        handleBatch(count);
    }
    recordMetric(total);
}
`,
			targetFunc: "processRecords",
		},
		{
			name:            "Python",
			lang:            "python",
			filename:        "test.py",
			fnType:          "function_definition",
			totalType:       "expression_statement",
			elseType:        "else_clause",
			elseBodyType:    "block",
			elseBodyLine:    6,
			dstElseLine:     16,
			dstElseBodyLine: 17,
			srcMetricLine:   8,
			dstBatchLine:    32,
			dstMetricLine:   33,
			src: `def process_records(flag: bool, count: int) -> None:
    total = 0
    if flag:
        total += 1
    else:
        finish_records(count)
        handle_batch(count)
    record_metric(total)
`,
			dst: `def process_records(flag: bool, count: int) -> None:
    total = 0
    validate_flags(flag)
    check_active_context()
    audit_preconditions()
    init_buffer_cache()
    prepare_worker_pool()
    reset_staged_entries()
    verify_system_clock()
    ensure_capacity(count)
    sync_storage_metrics()
    prepare_runtime()
    allocate_channel()
    if flag:
        total += 1
    else:
        if check_extra_threshold(count):
            split_and_process(count)
            log_debug_batch(count)
            record_entry(1)
            record_entry(2)
            record_entry(3)
            record_entry(4)
        elif check_secondary_threshold(count):
            record_fallback(count)
            apply_single_entry(count)
            record_entry(5)
            record_entry(6)
        else:
            notify_worker_pool(count)
            flush_intermediate_state(count)
        handle_batch(count)
    record_metric(total)
`,
			targetFunc: "process_records",
		},
		{
			name:            "Zig",
			lang:            "zig",
			filename:        "test.zig",
			fnType:          "function_declaration",
			totalType:       "variable_declaration",
			elseType:        "else_clause",
			elseBodyType:    "block",
			elseBodyLine:    5,
			dstElseLine:     16,
			dstElseBodyLine: 16,
			srcMetricLine:   9,
			dstBatchLine:    30,
			dstMetricLine:   32,
			src: `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    if (flag) {
        total += 1;
    } else {
        finish_records(count);
        handle_batch(count);
    }
    record_metric(total);
}
`,
			dst: `fn process_records(flag: bool, count: i32) void {
    var total: i32 = 0;
    validate_flags(flag);
    check_active_context();
    audit_preconditions();
    init_buffer_cache();
    prepare_worker_pool();
    reset_staged_entries();
    verify_system_clock();
    ensure_capacity(count);
    sync_storage_metrics();
    prepare_runtime();
    allocate_channel();
    if (flag) {
        total += 1;
    } else {
        switch (as_singleton_list(count)) {
            1 => {
                var split = split_records(count);
                if (split > 1) {
                    process_all(&split);
                } else {
                    record_fallback(count);
                }
            },
            else => {
                record_fallback(count);
            },
        }
        handle_batch(count);
    }
    record_metric(total);
}
`,
			targetFunc: "process_records",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srcBytes := []byte(tt.src)
			dstBytes := []byte(tt.dst)

			srcAST, err := treesitter.Parse(srcBytes, tt.filename)
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dstBytes, tt.filename)
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
			}

			part := NewLinePartition(srcBytes, dstBytes)
			mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			batchCall := "handle_batch(count)"
			metricCall := "record_metric(total)"
			totalDecl := "total = 0"
			if tt.name == "Zig" {
				totalDecl = "var total: i32 = 0;"
			} else if tt.targetFunc == "processRecords" {
				batchCall = "handleBatch(count)"
				metricCall = "recordMetric(total)"
			}

			// 1. Enclosing context: function declaration maps in place
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				tt.targetFunc,
				tt.targetFunc,
			)

			// 2. Surrounding sequence: preceding total = 0 and succeeding record_metric map in place
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				totalDecl,
				totalDecl,
			)
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				metricCall,
				metricCall,
			)

			// 3. Target clause: outer else clause must map in place to dst else clause
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				testutil.TextQuery{Text: "else", Occurrence: 1},
				testutil.TextQuery{Text: "else", Occurrence: 1},
			)

			// 4. Stationary inner statement handle_batch must map in place
			testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
				batchCall,
				batchCall,
			)
		})
	}
}

func TestChainedCallWrapperUpdateTheft(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		tests := []struct {
			name        string
			file        string
			oldSrc      string
			newSrc      string
			targetFunc  string
			preceding   string
			succeeding  string
			oldMethod   string
			innerMethod string
			outerMethod string
			flagMethod  string
			oldCall     string
			innerCall   string
			outerCall   string
		}{
			{
				name: "Rust_ChainedMethodCall_WrapperUpdateTheft",
				file: "app.rs",
				oldSrc: `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.contains_id("diff") && self.matches.get_flag("diff") {
            _ => None,
        },
    };
    self.flush_buffer();
}`,
				newSrc: `fn render_lines() {
    let style = compute_style();
    let _ = Config {
        visible_lines: match self.matches.try_contains_id("diff").unwrap_or_default()
            && self.matches.get_flag("diff")
        {
            _ => None,
        },
    };
    self.flush_buffer();
}`,
				targetFunc:  "render_lines",
				preceding:   "compute_style",
				succeeding:  "flush_buffer",
				oldMethod:   "contains_id",
				innerMethod: "try_contains_id",
				outerMethod: "unwrap_or_default",
				oldCall:     `self.matches.contains_id("diff")`,
				innerCall:   `self.matches.try_contains_id("diff")`,
				outerCall:   `self.matches.try_contains_id("diff").unwrap_or_default()`,
			},
			{
				name: "Go_ChainedMethodCall_WrapperUpdateTheft",
				file: "app.go",
				oldSrc: `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.ContainsID("diff") && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}`,
				newSrc: `package main

func renderLines() {
	style := computeStyle()
	_ = Config{
		VisibleLines: self.Matches.TryContainsID("diff").UnwrapOrDefault() && self.Matches.GetFlag("diff"),
	}
	self.flushBuffer()
}`,
				targetFunc:  "renderLines",
				preceding:   "computeStyle",
				succeeding:  "flushBuffer",
				oldMethod:   "ContainsID",
				innerMethod: "TryContainsID",
				outerMethod: "UnwrapOrDefault",
				flagMethod:  "GetFlag",
				oldCall:     `self.Matches.ContainsID("diff")`,
				innerCall:   `self.Matches.TryContainsID("diff")`,
				outerCall:   `self.Matches.TryContainsID("diff").UnwrapOrDefault()`,
			},
			{
				name: "TypeScript_ChainedMethodCall_WrapperUpdateTheft",
				file: "app.ts",
				oldSrc: `function renderLines() {
    const style = computeStyle();
    const config = {
        visibleLines: self.matches.containsId("diff") && self.matches.getFlag("diff"),
    };
    self.flushBuffer();
}`,
				newSrc: `function renderLines() {
    const style = computeStyle();
    const config = {
        visibleLines: self.matches.tryContainsId("diff").unwrapOrDefault() && self.matches.getFlag("diff"),
    };
    self.flushBuffer();
}`,
				targetFunc:  "renderLines",
				preceding:   "computeStyle",
				succeeding:  "flushBuffer",
				oldMethod:   "containsId",
				innerMethod: "tryContainsId",
				outerMethod: "unwrapOrDefault",
				flagMethod:  "getFlag",
				oldCall:     `self.matches.containsId("diff")`,
				innerCall:   `self.matches.tryContainsId("diff")`,
				outerCall:   `self.matches.tryContainsId("diff").unwrapOrDefault()`,
			},
			{
				name: "Python_ChainedMethodCall_WrapperUpdateTheft",
				file: "app.py",
				oldSrc: `def render_lines():
    style = compute_style()
    config = {
        "visible_lines": self.matches.contains_id("diff") and self.matches.get_flag("diff"),
    }
    self.flush_buffer()
`,
				newSrc: `def render_lines():
    style = compute_style()
    config = {
        "visible_lines": self.matches.try_contains_id("diff").unwrap_or_default() and self.matches.get_flag("diff"),
    }
    self.flush_buffer()
`,
				targetFunc:  "render_lines",
				preceding:   "compute_style",
				succeeding:  "flush_buffer",
				oldMethod:   "contains_id",
				innerMethod: "try_contains_id",
				outerMethod: "unwrap_or_default",
				oldCall:     `self.matches.contains_id("diff")`,
				innerCall:   `self.matches.try_contains_id("diff")`,
				outerCall:   `self.matches.try_contains_id("diff").unwrap_or_default()`,
			},
			{
				name: "Lua_ChainedMethodCall_WrapperUpdateTheft",
				file: "app.lua",
				oldSrc: `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:contains_id("diff") and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`,
				newSrc: `local function render_lines()
    local style = compute_style()
    local config = {
        visible_lines = self.matches:try_contains_id("diff"):unwrap_or_default() and self.matches:get_flag("diff"),
    }
    self:flush_buffer()
end
`,
				targetFunc:  "render_lines",
				preceding:   "compute_style",
				succeeding:  "flush_buffer",
				oldMethod:   "contains_id",
				innerMethod: "try_contains_id",
				outerMethod: "unwrap_or_default",
				oldCall:     `self.matches:contains_id("diff")`,
				innerCall:   `self.matches:try_contains_id("diff")`,
				outerCall:   `self.matches:try_contains_id("diff"):unwrap_or_default()`,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				srcBytes := []byte(tt.oldSrc)
				dstBytes := []byte(tt.newSrc)
				srcAST, err := treesitter.Parse(srcBytes, tt.file)
				if err != nil || srcAST == nil {
					t.Fatalf("failed to parse src: %v", err)
				}
				dstAST, err := treesitter.Parse(dstBytes, tt.file)
				if err != nil || dstAST == nil {
					t.Fatalf("failed to parse dst: %v", err)
				}
				if srcAST.HasError || srcAST.ParseErrorCount > 0 {
					t.Fatalf("src AST has %d parse errors", srcAST.ParseErrorCount)
				}
				if dstAST.HasError || dstAST.ParseErrorCount > 0 {
					t.Fatalf("dst AST has %d parse errors", dstAST.ParseErrorCount)
				}

				r := rulesFor(srcAST)
				if r == nil {
					t.Fatalf("missing rules for %s", tt.file)
				}

				part := NewLinePartition(srcBytes, dstBytes)
				mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
				if mr == nil || mr.Mappings == nil {
					t.Fatal("Match returned nil")
				}

				// 1. Enclosing context: function declaration maps in place
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.targetFunc, tt.targetFunc)

				// 2. Surrounding sequence: preceding and succeeding statements map in place
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.preceding, tt.preceding)
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.succeeding, tt.succeeding)

				// 3. Stationary sub-expression: sibling flag call maps in place
				flagName := tt.flagMethod
				if flagName == "" {
					flagName = "get_flag"
				}
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), flagName, flagName)

				// 4. Target call: old method call must map to inner call, NOT to outer wrapper call
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.oldCall, tt.innerCall)
				testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.oldCall, tt.outerCall)

				// 5. Method identifier: oldMethod must map to innerMethod, NOT outerMethod
				testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.oldMethod, tt.innerMethod)
				testutil.AssertNotMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(), tt.oldMethod, tt.outerMethod)
			})
		}
	})
}

func TestBottomUp_BinaryOperatorOperandSlots_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`def compute_scores(a, b, c, d):
    init_context()
    total = (a + b) + c
    finalize_context(total)
`)
		newSrc := []byte(`def compute_scores(a, b, c, d):
    init_context()
    total = (a + d) + c
    finalize_context(total)
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

		// 1. Enclosing function declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"def compute_scores(a, b, c, d):",
			"def compute_scores(a, b, c, d):",
		)

		// 2. Surrounding sequence (init_context, finalize_context) maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"init_context()",
			"init_context()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"finalize_context(total)",
			"finalize_context(total)",
		)

		// 3. Verification: Left operand a maps to dst a, right operand c maps to dst c.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "a", Occurrence: 2},
			testutil.TextQuery{Text: "a", Occurrence: 2},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "c", Occurrence: 2},
			testutil.TextQuery{Text: "c", Occurrence: 2},
		)

		// 4. Operand b in (a + b) maps to d in (a + d), and MUST NOT steal outer operand c.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "b", Occurrence: 2},
			testutil.TextQuery{Text: "d", Occurrence: 3},
		)
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "b", Occurrence: 2},
			testutil.TextQuery{Text: "c", Occurrence: 2},
		)
		testutil.AssertNotMappedTextQuery(t, dstAST, srcAST, newSrc, oldSrc, mr.Mappings.Dst(),
			testutil.TextQuery{Text: "d", Occurrence: 3},
			testutil.TextQuery{Text: "c", Occurrence: 2},
		)
	})
}

func TestBottomUp_AffinityFieldMutualExclusion_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`int evaluate(int x) {
    setup();
    if (x > 0)
        printf("positive\n");
    teardown();
    return 0;
}
`)
		newSrc := []byte(`int evaluate(int x) {
    setup();
    if (x < 0)
        printf("positive\n");
    teardown();
    return 0;
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.c")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.c")
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

		// 1. Enclosing function declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"int evaluate(int x)",
			"int evaluate(int x)",
		)

		// 2. Surrounding sequence (setup, teardown, return 0) maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setup();",
			"setup();",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardown();",
			"teardown();",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"return 0;",
			"return 0;",
		)

		// 3. The if statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"if (x > 0)\n        printf(\"positive\\n\");",
			"if (x < 0)\n        printf(\"positive\\n\");",
		)

		// 4. Consequence body maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"printf(\"positive\\n\");",
			"printf(\"positive\\n\");",
		)

		// 5. The if condition shouldn't match the consequence body.
		testutil.AssertNotMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"x > 0",
			"printf(\"positive\\n\");",
		)
	})
}

func TestBottomUp_NestedBinaryExpressionOperatorInversion(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `package test

func isCaseClauseBody(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || n.Parent == nil {
		return false
	}
	isPunct := (r != nil && (r.IsPunctuation(n.Type) || r.IsOperatorLiteral(n.Type) || r.IsKeyword(n.Type, n.Type))) ||
		(r == nil && (rules.IsPunctuation(n.Type) || rules.IsOperatorLiteral(n.Type) || rules.IsKeyword(n.Type, n.Type)))
	if isPunct {
		return false
	}
	return true
}
`
		newSrc := `package test

func isCaseClauseBody(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || n.Parent == nil {
		return false
	}
	isPunct := n.IsKeyword ||
		(r != nil && (r.IsPunctuation(n.Type) || r.IsOperatorLiteral(n.Type) || r.IsKeyword(n.Type, n.Label))) ||
		(r == nil && (rules.IsPunctuation(n.Type) || rules.IsOperatorLiteral(n.Type) || rules.IsKeyword(n.Type, n.Label)))
	if isPunct {
		return false
	}
	return true
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil || srcAST == nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil || dstAST == nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has %d parse errors", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has %d parse errors", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"func isCaseClauseBody(n *treesitter.ASTNode, r *rules.Rules) bool",
			"func isCaseClauseBody(n *treesitter.ASTNode, r *rules.Rules) bool",
		)

		// 2. Surrounding sequence: preceding nil guard and succeeding statements map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if n == nil || n.Parent == nil {\n\t\treturn false\n\t}",
			"if n == nil || n.Parent == nil {\n\t\treturn false\n\t}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if isPunct {\n\t\treturn false\n\t}",
			"if isPunct {\n\t\treturn false\n\t}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return true",
			"return true",
		)

		// 3. Target construct: short var declaration isPunct := ...
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"isPunct :=",
			"isPunct :=",
		)

		// 4. Expression mappings: inner binary expressions map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"r != nil",
			"r != nil",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"r == nil",
			"r == nil",
		)
	})
}

func TestBottomUp_NestedBinaryExpressionOperatorInversion_TypeScript(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := `function check(n: any, r: any): boolean {
	if (!n) {
		return false;
	}
	const isPunct = (r !== null && (r.isPunct(n.type) || r.isOp(n.type) || r.isKeyword(n.type, n.type))) ||
		(r === null && (isPunct(n.type) || isOp(n.type) || isKeyword(n.type, n.type)));
	if (isPunct) {
		return false;
	}
	return true;
}
`
		newSrc := `function check(n: any, r: any): boolean {
	if (!n) {
		return false;
	}
	const isPunct = n.isKeyword ||
		(r !== null && (r.isPunct(n.type) || r.isOp(n.type) || r.isKeyword(n.type, n.label))) ||
		(r === null && (isPunct(n.type) || isOp(n.type) || isKeyword(n.type, n.label)));
	if (isPunct) {
		return false;
	}
	return true;
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)
		srcAST, err := treesitter.Parse(srcBytes, "test.ts")
		if err != nil || srcAST == nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse src: %v (errors: %d)", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.ts")
		if err != nil || dstAST == nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("failed to parse dst: %v (errors: %d)", err, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: function declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"function check(n: any, r: any): boolean",
			"function check(n: any, r: any): boolean",
		)

		// 2. Surrounding sequence: preceding guard and succeeding statements map in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if (!n) {\n\t\treturn false;\n\t}",
			"if (!n) {\n\t\treturn false;\n\t}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"if (isPunct) {\n\t\treturn false;\n\t}",
			"if (isPunct) {\n\t\treturn false;\n\t}",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"return true;",
			"return true;",
		)

		// 3. Target construct: lexical declaration const isPunct = ...
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"const isPunct =",
			"const isPunct =",
		)

		// 4. Expression mappings: outerAnd (&&) expression maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"r !== null",
			"r !== null",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"r === null",
			"r === null",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, srcBytes, dstBytes, mr.Mappings.Src(),
			"r !== null && (r.isPunct(n.type) || r.isOp(n.type) || r.isKeyword(n.type, n.type))",
			"r !== null && (r.isPunct(n.type) || r.isOp(n.type) || r.isKeyword(n.type, n.label))",
		)
	})
}
