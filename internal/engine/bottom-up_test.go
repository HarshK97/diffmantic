package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestComputeAffinity_HardConstraints(t *testing.T) {
	leaf1 := testutil.Leaf("id", "x")
	leaf2 := testutil.Leaf("id", "x")
	t1 := testutil.Node("block", "", leaf1)
	c := testutil.Node("block", "", leaf2)

	m := NewMapping()

	// 1. Without common descendants, affinity should be -1.0
	scoreNoCommon := computeAffinity(t1, c, m, DefaultAffinityWeights, false)
	if scoreNoCommon != -1.0 {
		t.Errorf("expected -1.0 for no common descendants, got %f", scoreNoCommon)
	}

	m.Add(leaf1, leaf2)

	// 2. Already mapped destination candidate should return -1.0
	cMapped := testutil.Node("block", "")
	m.Add(t1, cMapped)
	scoreAlreadyMapped := computeAffinity(t1, cMapped, m, DefaultAffinityWeights, false)
	if scoreAlreadyMapped != -1.0 {
		t.Errorf("expected -1.0 for already mapped candidate, got %f", scoreAlreadyMapped)
	}

	// 3. Incompatible types should return -1.0
	m2 := NewMapping()
	m2.Add(leaf1, leaf2)
	cDiffType := testutil.Node("expression_statement", "", leaf2)
	scoreDiffType := computeAffinity(t1, cDiffType, m2, DefaultAffinityWeights, false)
	if scoreDiffType != -1.0 {
		t.Errorf("expected -1.0 for incompatible types, got %f", scoreDiffType)
	}
}

func TestComputeAffinity_ScopeProximity(t *testing.T) {
	l1 := testutil.Leaf("id", "val")
	l2 := testutil.Leaf("id", "val")
	b1 := testutil.Node("block", "", l1)
	b2 := testutil.Node("block", "", l2)

	scopeA1 := testutil.Node("function_declaration", "", b1)
	scopeA2 := testutil.Node("function_declaration", "", b2)

	m := NewMapping()
	m.Add(l1, l2)
	m.Add(scopeA1, scopeA2)

	scoreInScope := computeAffinity(b1, b2, m, DefaultAffinityWeights, false)

	l3 := testutil.Leaf("id", "val")
	b3 := testutil.Node("block", "", l3)
	scopeB2 := testutil.Node("function_declaration", "", b3)
	scopeB1 := testutil.Node("function_declaration", "")
	_ = testutil.Node("root", "", scopeA1, scopeB1)
	_ = testutil.Node("root", "", scopeA2, scopeB2)

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
	l1 := testutil.Leaf("id", "a")
	l2 := testutil.Leaf("id", "a")
	item1 := testutil.Node("item", "", l1)
	item2 := testutil.Node("item", "", l2)
	parent1 := testutil.Node("container", "", item1, testutil.Node("item", ""))
	parent2 := testutil.Node("container", "", item2, testutil.Node("item", ""))

	m := NewMapping()
	m.Add(l1, l2)

	scoreSamePos := computeAffinity(item1, item2, m, DefaultAffinityWeights, false)

	// Shift item2 to index 1
	item2Shifted := testutil.Node("item", "", l2)
	_ = testutil.Node("container", "", testutil.Node("item", ""), item2Shifted)
	mShifted := NewMapping()
	mShifted.Add(l1, l2)

	scoreShifted := computeAffinity(item1, item2Shifted, mShifted, DefaultAffinityWeights, false)
	if scoreSamePos < scoreShifted {
		t.Errorf("expected same positional score (%f) >= shifted score (%f)", scoreSamePos, scoreShifted)
	}

	// Unordered parent container ignores positional differences
	parent1.IsUnordered = true
	parent2.IsUnordered = true
	scoreUnordered := computeAffinity(item1, item2, m, DefaultAffinityWeights, false)
	if scoreUnordered < scoreSamePos {
		t.Errorf("expected unordered score (%f) to be neutral/maximal", scoreUnordered)
	}
}

func TestComputeAffinity_KeyBonus(t *testing.T) {
	k1 := testutil.Leaf("string", "\"name\"")
	v1 := testutil.Leaf("string", "\"alice\"")
	pair1 := testutil.Node("pair", "", k1, v1)

	k2 := testutil.Leaf("string", "\"name\"")
	v2 := testutil.Leaf("string", "\"alice\"")
	pair2 := testutil.Node("pair", "", k2, v2)

	k3 := testutil.Leaf("string", "\"other\"")
	v3 := testutil.Leaf("string", "\"alice\"")
	pair3 := testutil.Node("pair", "", k3, v3)

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
	l1 := testutil.Leaf("id", "x")
	l2 := testutil.Leaf("id", "x")
	sub1 := testutil.Node("binary_expression", "", l1)
	sub2 := testutil.Node("binary_expression", "", l2)

	anc1 := testutil.Node("func", "", sub1)
	_ = testutil.Node("func", "", sub2)
	otherAnc := testutil.Node("func", "")

	m := NewMapping()
	m.Add(l1, l2)
	m.Add(anc1, otherAnc)

	score := computeAffinity(sub1, sub2, m, DefaultAffinityWeights, false)
	if score != -1.0 {
		t.Errorf("expected -1.0 for small subtree crossing scope boundary, got %f", score)
	}
}

// TestComputeAffinity_DepthPenaltyFires verifies that when both nodes have a mapped
// ancestor in scope and their relative depths differ, the depth penalty fires and reduces
// the cross-depth score below the same-depth score.
func TestComputeAffinity_DepthPenaltyFires(t *testing.T) {
	sharedLeaf1 := testutil.Leaf("id", "shared")
	sharedLeaf2 := testutil.Leaf("id", "shared")

	// Same-depth: both blocks are direct children of their parents (depth 1).
	block1 := testutil.Node("block", "", sharedLeaf1, testutil.Leaf("id", "x"))
	block2 := testutil.Node("block", "", sharedLeaf2, testutil.Leaf("id", "x"))
	parent1 := testutil.Node("func", "", block1)
	parent2 := testutil.Node("func", "", block2)

	// Cross-depth: block1Deep is nested under an if-node inside parent1Deep (depth 2).
	// block2Shallow is a direct child of parent2Shallow (depth 1).
	sharedLeaf1d := testutil.Leaf("id", "shared")
	sharedLeaf2d := testutil.Leaf("id", "shared")
	block1Deep := testutil.Node("block", "", sharedLeaf1d, testutil.Leaf("id", "x"))
	ifNode := testutil.Node("if", "", testutil.Leaf("id", "cond"), block1Deep) // block1Deep.Parent = ifNode
	parent1Deep := testutil.Node("func", "", ifNode)                           // block1Deep depth = 2

	block2Shallow := testutil.Node("block", "", sharedLeaf2d, testutil.Leaf("id", "x"))
	parent2Shallow := testutil.Node("func", "", block2Shallow) // block2Shallow depth = 1

	mSame := NewMapping()
	mSame.Add(sharedLeaf1, sharedLeaf2)
	mSame.Add(parent1, parent2)
	scoreSameDepth := computeAffinity(block1, block2, mSame, DefaultAffinityWeights, false)

	// depthPenalty = min(0.30, DepthCoeff × |2−1|) = 0.20 → cross-depth score is lower.
	mDeep := NewMapping()
	mDeep.Add(sharedLeaf1d, sharedLeaf2d)
	mDeep.Add(parent1Deep, parent2Shallow)
	scoreCrossDepth := computeAffinity(block1Deep, block2Shallow, mDeep, DefaultAffinityWeights, false)

	if scoreSameDepth <= scoreCrossDepth {
		t.Errorf("depth penalty did not fire: same-depth %f should exceed cross-depth %f", scoreSameDepth, scoreCrossDepth)
	}
}

func TestContestContainers_SiblingArbitration(t *testing.T) {
	// Root array containers
	// T1 array: [t1Early, t1Stationary]
	// T2 array: [t2]

	leafURL1 := testutil.Leaf("string", "\"url_match\"")
	t1Early := testutil.Node("object", "",
		testutil.Node("pair", "", testutil.Leaf("string", "\"name\""), testutil.Leaf("string", "\"early\"")),
		testutil.Node("pair", "", testutil.Leaf("string", "\"url\""), leafURL1),
	)

	leafDesc1 := testutil.Leaf("string", "\"desc_match\"")
	leafMatch1 := testutil.Leaf("string", "\"file_match\"")
	t1Stationary := testutil.Node("object", "",
		testutil.Node("pair", "", testutil.Leaf("string", "\"name\""), testutil.Leaf("string", "\"stationary\"")),
		testutil.Node("pair", "", testutil.Leaf("string", "\"desc\""), leafDesc1),
		testutil.Node("pair", "", testutil.Leaf("string", "\"fileMatch\""), leafMatch1),
	)

	root1 := testutil.Node("array", "", t1Early, t1Stationary)

	leafURL2 := testutil.Leaf("string", "\"url_match\"")
	leafDesc2 := testutil.Leaf("string", "\"desc_match\"")
	leafMatch2 := testutil.Leaf("string", "\"file_match\"")
	t2 := testutil.Node("object", "",
		testutil.Node("pair", "", testutil.Leaf("string", "\"name\""), testutil.Leaf("string", "\"stationary_updated\"")),
		testutil.Node("pair", "", testutil.Leaf("string", "\"desc\""), leafDesc2),
		testutil.Node("pair", "", testutil.Leaf("string", "\"fileMatch\""), leafMatch2),
		testutil.Node("pair", "", testutil.Leaf("string", "\"url\""), leafURL2),
	)

	root2 := testutil.Node("array", "", t2)

	m := NewMapping()
	m.Add(root1, root2)
	// TopDown matches
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
	inner1 := testutil.Node("block", "", testutil.Leaf("id", "throw"))
	outer1 := testutil.Node("block", "", testutil.Node("if", "", inner1))
	root1 := testutil.Node("func", "", outer1)

	inner2 := testutil.Leaf("id", "throw")
	outer2 := testutil.Node("block", "", inner2)
	root2 := testutil.Node("func", "", outer2)

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(inner1.Children[0], inner2)
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
	p1 := testutil.Node("parameter_list", "", testutil.Leaf("identifier", "t"))
	p1.Language = "go"
	b1 := testutil.Node("block", "", testutil.Leaf("identifier", "statementInOld"))
	b1.Language = "go"
	f1 := testutil.Node("function_declaration", "", testutil.Leaf("identifier", "Foo"), p1, b1)
	f1.Language = "go"

	p2 := testutil.Node("parameter_list", "", testutil.Leaf("identifier", "t"))
	p2.Language = "go"
	b2 := testutil.Node("block", "", testutil.Leaf("identifier", "statementInNew"))
	b2.Language = "go"
	f2 := testutil.Node("function_declaration", "", testutil.Leaf("identifier", "Bar"), p2, b2)
	f2.Language = "go"

	m := NewMapping()
	m.Add(p1, p2)
	m.Add(p1.Children[0], p2.Children[0])

	RollupMatchedContainers(f1, f2, m)

	if m.Get(f1) != nil {
		t.Errorf("expected function_declaration with unmatched body not to roll up, but got mapped to %v", m.Get(f1))
	}
}

func TestRollupMatchedContainers_MassWeighting(t *testing.T) {
	// t1 (short_var_declaration) has:
	// Child 1: operator leaf ":=" (size 1)
	// Child 2: compound RHS expression (size 3)
	leafOp1 := testutil.Leaf("assignment_operator_literal", ":=")
	rhsCompound1 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "a"),
		testutil.Leaf("arithmetic_operator_literal", "+"),
		testutil.Leaf("identifier", "b"),
	)
	t1 := testutil.Node("short_var_declaration", "", leafOp1, rhsCompound1)
	root1 := testutil.Node("source_file", "", t1)

	// In T2:
	// c2Wrong has a child matched to leafOp1 (1 vote)
	leafOp2 := testutil.Leaf("assignment_operator_literal", ":=")
	c2Wrong := testutil.Node("short_var_declaration", "", leafOp2, testutil.Leaf("identifier", "other"))

	// c2Right has a child matched to rhsCompound1 (3 votes from subtree mass)
	rhsCompound2 := testutil.Node("binary_expression", "",
		testutil.Leaf("identifier", "a"),
		testutil.Leaf("arithmetic_operator_literal", "+"),
		testutil.Leaf("identifier", "b"),
	)
	c2Right := testutil.Node("short_var_declaration", "", testutil.Leaf("assignment_operator_literal", ":="), rhsCompound2)
	root2 := testutil.Node("source_file", "", c2Wrong, c2Right)

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(leafOp1, leafOp2)
	m.Add(rhsCompound1, rhsCompound2)
	m.Add(rhsCompound1.Children[0], rhsCompound2.Children[0])
	m.Add(rhsCompound1.Children[1], rhsCompound2.Children[1])
	m.Add(rhsCompound1.Children[2], rhsCompound2.Children[2])

	RollupMatchedContainers(root1, root2, m)

	if m.Get(t1) != c2Right {
		t.Fatalf("expected t1 to roll up to c2Right by structural mass, got %v", m.Get(t1))
	}
}

func TestRollupMatchedContainers_TieBreaking_ScopeAffinity(t *testing.T) {
	// t1 is inside func1_src
	t1Child := testutil.Leaf("identifier", "x")
	t1 := testutil.Node("assignment_statement", "", t1Child)
	b1 := testutil.Node("block", "", t1)
	func1Src := testutil.Node("function_declaration", "", b1)
	root1 := testutil.Node("source_file", "", func1Src)

	// T2: func1_dst (mapped to func1_src) and func2_dst (unrelated function)
	candInScopeChild := testutil.Leaf("identifier", "x")
	candInScope := testutil.Node("assignment_statement", "", candInScopeChild)
	b2 := testutil.Node("block", "", candInScope)
	func1Dst := testutil.Node("function_declaration", "", b2)

	candForeignChild := testutil.Leaf("identifier", "x")
	candForeign := testutil.Node("assignment_statement", "", candForeignChild)
	b3 := testutil.Node("block", "", candForeign)
	func2Dst := testutil.Node("function_declaration", "", b3)

	root2 := testutil.Node("source_file", "", func1Dst, func2Dst)

	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(func1Src, func1Dst)

	// Both candidate parents get equal votes
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
	t1Child := testutil.Leaf("identifier", "x")
	t1 := testutil.Node("assignment_statement", "", t1Child)
	t1.StartRow = 10

	candCloseChild := testutil.Leaf("identifier", "x")
	candClose := testutil.Node("assignment_statement", "", candCloseChild)
	candClose.StartRow = 12 // distance 2

	candFarChild := testutil.Leaf("identifier", "x")
	candFar := testutil.Node("assignment_statement", "", candFarChild)
	candFar.StartRow = 35 // distance 25

	root1 := testutil.Node("source_file", "", t1)
	root2 := testutil.Node("source_file", "", candClose, candFar)
	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

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
	t1Child := testutil.Leaf("identifier", "x")
	t1 := testutil.Node("assignment_statement", "", t1Child)
	t1.StartRow = 10

	candFirstChild := testutil.Leaf("identifier", "x")
	candFirst := testutil.Node("assignment_statement", "", candFirstChild)
	candFirst.StartRow = 10

	candSecondChild := testutil.Leaf("identifier", "x")
	candSecond := testutil.Node("assignment_statement", "", candSecondChild)
	candSecond.StartRow = 10

	root1 := testutil.Node("source_file", "", t1)
	root2 := testutil.Node("source_file", "", candFirst, candSecond)
	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	// Since candFirst comes before candSecond in root2, candFirst.ID < candSecond.ID
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
	// Source tree:
	// block1 contains:
	//   if_stmt -> block -> currentT1 (assignment_statement "pairCount = len(pairs)")
	//   candidate (assignment_statement "s.cpySrcToDst = make(...)")
	lhsOld := testutil.Node("expression_list", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "s"),
			testutil.Leaf("field_identifier", "cpySrcToDst"),
		),
	)
	opOld := testutil.Leaf("assignment_operator_literal", "=")
	rhsOld := testutil.Node("expression_list", "",
		testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "make"),
			testutil.Node("argument_list", "",
				testutil.Node("map_type", "",
					testutil.Leaf("map", "map"),
					testutil.Leaf("type_identifier", "key"),
					testutil.Leaf("type_identifier", "val"),
				),
				testutil.Leaf("identifier", "pairCount"),
			),
		),
	)
	candidate := testutil.Node("assignment_statement", "", lhsOld, opOld, rhsOld)

	lhsDecomp := testutil.Node("expression_list", "", testutil.Leaf("identifier", "pairCount"))
	opDecomp := testutil.Leaf("assignment_operator_literal", "=")
	rhsDecomp := testutil.Node("expression_list", "",
		testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "len"),
			testutil.Node("argument_list", "", testutil.Leaf("identifier", "pairs")),
		),
	)
	currentT1 := testutil.Node("assignment_statement", "", lhsDecomp, opDecomp, rhsDecomp)
	ifBlock := testutil.Node("block", "", currentT1)
	ifStmt := testutil.Node("if_statement", "", ifBlock)

	block1 := testutil.Node("block", "", ifStmt, candidate)
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	// Destination tree:
	// block2 contains:
	//   t2 (assignment_statement "s.cpySrcToDst = make(map[...], len(pairs))")
	lhsNew := testutil.Node("expression_list", "",
		testutil.Node("selector_expression", "",
			testutil.Leaf("identifier", "s"),
			testutil.Leaf("field_identifier", "cpySrcToDst"),
		),
	)
	opNew := testutil.Leaf("assignment_operator_literal", "=")
	rhsNew := testutil.Node("expression_list", "",
		testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "make"),
			testutil.Node("argument_list", "",
				testutil.Node("map_type", "",
					testutil.Leaf("map", "map"),
					testutil.Leaf("type_identifier", "key"),
					testutil.Leaf("type_identifier", "val"),
				),
				testutil.Node("call_expression", "",
					testutil.Leaf("identifier", "len"),
					testutil.Node("argument_list", "", testutil.Leaf("identifier", "pairs")),
				),
			),
		),
	)
	t2 := testutil.Node("assignment_statement", "", lhsNew, opNew, rhsNew)
	block2 := testutil.Node("block", "", t2)
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(block1, block2)

	// TopDown leaf mappings
	m.Add(lhsOld, lhsNew)
	for i := range lhsOld.Children {
		m.Add(lhsOld.Children[i], lhsNew.Children[i])
	}
	// map_type leaves matched
	mapTypeOld := rhsOld.Children[0].Children[1].Children[0]
	mapTypeNew := rhsNew.Children[0].Children[1].Children[0]
	m.Add(mapTypeOld, mapTypeNew)
	for i := range mapTypeOld.Children {
		m.Add(mapTypeOld.Children[i], mapTypeNew.Children[i])
	}

	// Simulate BottomUp greedily matching currentT1 to t2 before contest:
	m.Add(currentT1, t2)
	m.Add(opDecomp, opNew)
	m.Add(rhsDecomp, rhsNew)

	ContestContainers(root1, root2, m, nil)

	if got := m.Get(currentT1); got != nil {
		t.Errorf("m.Get(currentT1) = %v, want nil", got)
	}
	if got := m.Get(candidate); got != t2 {
		t.Fatalf("m.Get(candidate) = %v, want %v", got, t2)
	}
	if got := m.Get(opOld); got != opNew {
		t.Errorf("m.Get(opOld) = %v, want %v", got, opNew)
	}
}

func TestRollupMatchedContainers_CrossStatementBoundaryGuard(t *testing.T) {
	childA1 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "a"))
	childB1 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "b"))
	cond1 := testutil.Node("binary_expression", "", childA1, childB1)
	body1 := testutil.Node("block", "")
	ifStmt1 := testutil.Node("if_statement", "", cond1, body1)
	block1 := testutil.Node("block", "", ifStmt1)
	root1 := testutil.Node("source_file", "", block1)
	root1.Language = "go"

	childA2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "a"))
	childC2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "c"))
	cond2 := testutil.Node("binary_expression", "", childA2, childC2)
	body2 := testutil.Node("block", "")
	ifStmt2 := testutil.Node("if_statement", "", cond2, body2)

	childB2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "b"))
	childD2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "d"))
	cond3 := testutil.Node("binary_expression", "", childB2, childD2)
	body3 := testutil.Node("block", "")
	ifStmt3 := testutil.Node("if_statement", "", cond3, body3)

	block2 := testutil.Node("block", "", ifStmt2, ifStmt3)
	root2 := testutil.Node("source_file", "", block2)
	root2.Language = "go"

	otherBodyA := testutil.Node("block", "")
	otherStmtA := testutil.Node("if_statement", "", otherBodyA)
	otherBodyB := testutil.Node("block", "")
	otherStmtB := testutil.Node("if_statement", "", otherBodyB)

	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

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

	// Verify reverse direction: t1's enclosing body is mapped to a construct that does not contain c.
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
	init1 := testutil.Node("short_var_declaration", "")
	cond1 := testutil.Node("binary_expression", "")
	body1 := testutil.Node("block", "")
	ifStmt1 := testutil.Node("if_statement", "", init1, cond1, body1)
	ifStmt1.Language = "go"

	assign2 := testutil.Node("assignment_statement", "")
	cond2 := testutil.Node("binary_expression", "")
	body2 := testutil.Node("block", "")
	ifStmt2 := testutil.Node("if_statement", "", cond2, body2)
	ifStmt2.Language = "go"

	m := NewMapping()
	m.Add(body1, body2)

	// Moving an init statement outside an if statement shouldn't be blocked by foreign body checks.
	if hasForeignBodyOwner(init1, assign2, m, nil) {
		t.Errorf("expected hasForeignBodyOwner to permit initializer statement init1 vs assign2")
	}

	// But condition expressions tested against an unrelated construct must still be rejected.
	otherBody := testutil.Node("block", "")
	otherCond := testutil.Node("binary_expression", "")
	testutil.Node("if_statement", "", otherCond, otherBody)
	if !hasForeignBodyOwner(cond1, otherCond, m, nil) {
		t.Errorf("expected hasForeignBodyOwner to reject cond1 vs otherCond across foreign construct")
	}
}

func TestComputeAffinity_BodyBlockConsistency(t *testing.T) {
	body1 := testutil.Node("block", "")
	cond1 := testutil.Node("identifier", "cond")
	ifStmt1 := testutil.Node("if_statement", "", cond1, body1)
	ifStmt1.Language = "go"

	body2 := testutil.Node("block", "")
	cond2 := testutil.Node("identifier", "cond")
	ifStmt2 := testutil.Node("if_statement", "", cond2, body2)
	ifStmt2.Language = "go"

	body3 := testutil.Node("block", "")
	cond3 := testutil.Node("identifier", "cond")
	ifStmt3 := testutil.Node("if_statement", "", cond3, body3)
	ifStmt3.Language = "go"

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
	// Going from (a && (b)) to (a && b), the inner (b) wrapper shouldn't roll up
	// to the outer (a && b) container since 'a' came from outside (b).
	leafA1 := testutil.Leaf("id", "a")
	leafB1 := testutil.Leaf("id", "b")
	inner1 := testutil.Node("parenthesized_expression", "", leafB1)
	inner1.Language = "go"
	outer1 := testutil.Node("parenthesized_expression", "", leafA1, inner1)
	outer1.Language = "go"

	leafA2 := testutil.Leaf("id", "a")
	leafB2 := testutil.Leaf("id", "b")
	dstParen := testutil.Node("parenthesized_expression", "", leafA2, leafB2)
	dstParen.Language = "go"

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
		stmts := []*treesitter.ASTNode{
			testutil.Node("expression_statement", "", testutil.Leaf("identifier", "s1")),
			testutil.Node("expression_statement", "", testutil.Leaf("identifier", "s2")),
			testutil.Node("expression_statement", "", testutil.Leaf("identifier", "s3")),
			testutil.Node("expression_statement", "", testutil.Leaf("identifier", "s4")),
		}
		body := testutil.Node("block", "", stmts...)
		decl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", name),
			body,
		)
		root := testutil.Node("source_file", "", decl)
		root.Language = "go"
		for _, n := range root.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root)
		return decl, body, stmts
	}

	t.Run("rejects function bodies across different declaration names when similarity < 60%", func(t *testing.T) {
		_, bodyA, stmtsA := makeFunc("funcA")
		_, bodyB, stmtsB := makeFunc("funcB")

		m := NewMapping()
		// Map only 1 of 4 statements (~22% similarity).
		m.Add(stmtsA[0], stmtsB[0])
		m.Add(stmtsA[0].Children[0], stmtsB[0].Children[0])

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
			m.Add(stmtsA[i], stmtsB[i])
			m.Add(stmtsA[i].Children[0], stmtsB[i].Children[0])
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
		mLow.Add(stmtsA[0], stmtsB[0])
		mLow.Add(stmtsA[0].Children[0], stmtsB[0].Children[0])
		if score := computeAffinity(declA, declB, mLow, DefaultAffinityWeights, false); score != -1.0 {
			t.Errorf("expected -1.0 for low-similarity declarations with different names, got %f", score)
		}

		mHigh := NewMapping()
		for i := range stmtsA {
			mHigh.Add(stmtsA[i], stmtsB[i])
			mHigh.Add(stmtsA[i].Children[0], stmtsB[i].Children[0])
		}
		if score := computeAffinity(declA, declB, mHigh, DefaultAffinityWeights, false); score <= 0.0 {
			t.Errorf("expected positive affinity for high-similarity declarations with different names, got %f", score)
		}
	})
}

func TestRollupMatchedContainers_MismatchedDeclarationNames(t *testing.T) {
	buildTree := func(name string) (*treesitter.ASTNode, *treesitter.ASTNode, *treesitter.ASTNode, []*treesitter.ASTNode) {
		leaves := []*treesitter.ASTNode{
			testutil.Leaf("identifier", "a"),
			testutil.Leaf("identifier", "b"),
			testutil.Leaf("identifier", "c"),
			testutil.Leaf("identifier", "d"),
			testutil.Leaf("identifier", "e"),
		}
		body := testutil.Node("block", "", leaves...)
		decl := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", name),
			body,
		)
		root := testutil.Node("source_file", "", decl)
		for _, n := range root.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root)
		return root, decl, body, leaves
	}

	// 2 of 5 leaves + body mapped gives ~38% Dice (< 0.60, but >= 0.30).
	root1, declA, bodyA, leavesA := buildTree("funcA")
	root2, _, bodyB, leavesB := buildTree("funcB")

	m := NewMapping()
	m.Add(root1, root2)
	m.Add(bodyA, bodyB)
	m.Add(leavesA[0], leavesB[0])
	m.Add(leavesA[1], leavesB[1])

	RollupMatchedContainers(root1, root2, m)
	if m.Get(declA) != nil {
		t.Errorf("expected RollupMatchedContainers to reject declA -> declB with mismatched names at <60%% Dice, got %v", m.Get(declA))
	}

	// Same overlap when declaration names match ("funcA" -> "funcA") should roll up (>= 30% Dice).
	rootSame2, declSameA, bodySameA, leavesSameA := buildTree("funcA")
	mSame := NewMapping()
	mSame.Add(root1, rootSame2)
	mSame.Add(bodyA, bodySameA)
	mSame.Add(leavesA[0], leavesSameA[0])
	mSame.Add(leavesA[1], leavesSameA[1])

	RollupMatchedContainers(root1, rootSame2, mSame)
	if mSame.Get(declA) != declSameA {
		t.Errorf("expected RollupMatchedContainers to pair matching declaration names at >=30%% Dice, got %v", mSame.Get(declA))
	}
}

func TestRollupMatchedContainers_EquivalentTypeTargetVariableMismatch(t *testing.T) {
	// Extracting min(v1, v2) from `overlap += min(v1, v2)` into `matched := min(v1, v2)`
	// shouldn't roll the old `overlap +=` container up into `matched :=`.
	idOverlap1 := testutil.Node("identifier", "overlap")
	lhsList1 := testutil.Node("expression_list", "", idOverlap1)
	opPlus1 := testutil.Node("assignment_operator_literal", "+=")
	callMin1 := testutil.Node("call_expression", "min")
	rhsList1 := testutil.Node("expression_list", "", callMin1)
	assignStmt1 := testutil.Node("assignment_statement", "", lhsList1, opPlus1, rhsList1)
	assignStmt1.Language = "go"

	idMatched2 := testutil.Node("identifier", "matched")
	lhsList2 := testutil.Node("expression_list", "", idMatched2)
	opDecl2 := testutil.Node("assignment_operator_literal", ":=")
	callMin2 := testutil.Node("call_expression", "min")
	rhsList2 := testutil.Node("expression_list", "", callMin2)
	shortDecl2 := testutil.Node("short_var_declaration", "", lhsList2, opDecl2, rhsList2)
	shortDecl2.Language = "go"

	root1 := testutil.Node("block", "", assignStmt1)
	root2 := testutil.Node("block", "", shortDecl2)
	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	m := NewMapping()
	m.Add(callMin1, callMin2)
	m.Add(rhsList1, rhsList2)

	RollupMatchedContainers(root1, root2, m)

	if m.Get(assignStmt1) != nil {
		t.Errorf("expected assignStmt1 NOT to roll up to shortDecl2 with different target variable, got: %v", m.Get(assignStmt1))
	}
}

func TestRollupMatchedContainers_SameTypeTargetVariableMismatch(t *testing.T) {
	// Old: dstNode = ms.Src()[a.Node]
	// New: toDemote[a.Node] = dstNode
	// Even though both are assignments and share dstNode across sides (LHS in old, RHS in new),
	// they shouldn't roll up since they assign to different target variables.
	idDst1 := testutil.Node("identifier", "dstNode")
	lhsList1 := testutil.Node("expression_list", "", idDst1)
	opAssign1 := testutil.Node("assignment_operator_literal", "=")
	idMs1 := testutil.Node("identifier", "ms")
	rhsList1 := testutil.Node("expression_list", "", idMs1)
	assignStmt1 := testutil.Node("assignment_statement", "", lhsList1, opAssign1, rhsList1)
	assignStmt1.Language = "go"

	idMap2 := testutil.Node("identifier", "toDemote")
	lhsList2 := testutil.Node("expression_list", "", idMap2)
	opAssign2 := testutil.Node("assignment_operator_literal", "=")
	idDst2 := testutil.Node("identifier", "dstNode")
	rhsList2 := testutil.Node("expression_list", "", idDst2)
	assignStmt2 := testutil.Node("assignment_statement", "", lhsList2, opAssign2, rhsList2)
	assignStmt2.Language = "go"

	root1 := testutil.Node("block", "", assignStmt1)
	root2 := testutil.Node("block", "", assignStmt2)
	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	m := NewMapping()
	m.Add(idDst1, idDst2)
	m.Add(lhsList1, rhsList2)

	RollupMatchedContainers(root1, root2, m)

	if m.Get(assignStmt1) != nil {
		t.Errorf("expected assignStmt1 NOT to roll up to assignStmt2 with disjoint target variables, got: %v", m.Get(assignStmt1))
	}
}

func TestHasCallCalleeOrArgMatch_UnextinguishedPeer(t *testing.T) {
	r := rules.Get("go")

	// Old: addSpan(spansByLine, lineIndex, fileBytes, a.Node.StartByte)
	idAddSpan1 := testutil.Node("identifier", "addSpan")
	argA1 := testutil.Node("identifier", "a")
	args1 := testutil.Node("argument_list", "", argA1)
	call1 := testutil.Node("call_expression", "", idAddSpan1, args1)

	// New helper: absorbSyntacticDelimiters(fileBytes, a.Node.StartByte)
	idAbsorb2 := testutil.Node("identifier", "absorbSyntacticDelimiters")
	argA2 := testutil.Node("identifier", "a")
	argsAbsorb2 := testutil.Node("argument_list", "", argA2)
	callAbsorb2 := testutil.Node("call_expression", "", idAbsorb2, argsAbsorb2)
	shortDecl2 := testutil.Node("short_var_declaration", "", callAbsorb2)

	// New surviving: addSpan(spansByLine, lineIndex, fileBytes, sb)
	idAddSpan2 := testutil.Node("identifier", "addSpan")
	argSb2 := testutil.Node("identifier", "sb")
	argsAddSpan2 := testutil.Node("argument_list", "", argSb2)
	callAddSpan2 := testutil.Node("call_expression", "", idAddSpan2, argsAddSpan2)
	exprStmt2 := testutil.Node("expression_statement", "", callAddSpan2)

	block2 := testutil.Node("block", "", shortDecl2, exprStmt2)
	treesitter.EnsureIndex(call1)
	treesitter.EnsureIndex(block2)

	m := NewMapping()
	// Mapped descendant: argA1 -> argA2
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
		s1 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
		body1 := testutil.Node("block", "", s1)
		cond1 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
		if1 := testutil.Node("if_statement", "", cond1, body1)
		root1 := testutil.Node("source_file", "", if1)
		root1.Language = "go"
		for _, n := range root1.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root1)

		innerS2 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
		innerBody2 := testutil.Node("block", "", innerS2)
		innerCond2 := testutil.Node("binary_expression", "", testutil.Node("identifier", "b"))
		innerIf2 := testutil.Node("if_statement", "", innerCond2, innerBody2)
		body2 := testutil.Node("block", "", innerIf2)
		cond2 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
		if2 := testutil.Node("if_statement", "", cond2, body2)
		root2 := testutil.Node("source_file", "", if2)
		root2.Language = "go"
		for _, n := range root2.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root2)

		m := NewMapping()
		m.Add(if1, if2)
		m.Add(body1, innerBody2)

		reconcileBodyBlocks(root1, m)

		if m.Get(body1) != body2 {
			t.Errorf("expected body1 to be reconciled to outer body2, got %v", m.Get(body1))
		}
	})

	t.Run("reverse unwrapping: re-aligns outer body block when inner was mapped", func(t *testing.T) {
		innerS1 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
		innerBody1 := testutil.Node("block", "", innerS1)
		innerCond1 := testutil.Node("binary_expression", "", testutil.Node("identifier", "b"))
		innerIf1 := testutil.Node("if_statement", "", innerCond1, innerBody1)
		body1 := testutil.Node("block", "", innerIf1)
		cond1 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
		if1 := testutil.Node("if_statement", "", cond1, body1)
		root1 := testutil.Node("source_file", "", if1)
		root1.Language = "go"
		for _, n := range root1.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root1)

		s2 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
		body2 := testutil.Node("block", "", s2)
		cond2 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
		if2 := testutil.Node("if_statement", "", cond2, body2)
		root2 := testutil.Node("source_file", "", if2)
		root2.Language = "go"
		for _, n := range root2.PreOrder() {
			n.Language = "go"
		}
		treesitter.EnsureIndex(root2)

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

	cond1 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
	s1 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
	body1 := testutil.Node("block", "", s1)
	if1 := testutil.Node("if_statement", "", cond1, body1)
	root1 := testutil.Node("source_file", "", if1)
	root1.Language = "go"
	for _, n := range root1.PreOrder() {
		n.Language = "go"
	}
	treesitter.EnsureIndex(root1)

	cond2 := testutil.Node("binary_expression", "", testutil.Node("identifier", "a"))
	s2 := testutil.Node("expression_statement", "", testutil.Node("identifier", "x"))
	body2 := testutil.Node("block", "", s2)
	if2 := testutil.Node("if_statement", "", cond2, body2)
	root2 := testutil.Node("source_file", "", if2)
	root2.Language = "go"
	for _, n := range root2.PreOrder() {
		n.Language = "go"
	}
	treesitter.EnsureIndex(root2)

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

	// Construct 1 in Old: if nilR.IsCaseClause("expression_case") { t.Errorf(...) }
	cond1 := testutil.Node("call_expression", "nilR.IsCaseClause")
	body1 := testutil.Node("block", "", testutil.Leaf("identifier", "t"))
	if1 := testutil.Node("if_statement", "", cond1, body1)

	// Construct 2 in Old: if !IsCaseClause("case_clause") { t.Errorf(...) }
	cond2 := testutil.Node("call_expression", "IsCaseClause")
	body2 := testutil.Node("block", "", testutil.Leaf("identifier", "t"))
	if2 := testutil.Node("if_statement", "", cond2, body2)

	// Construct in New: if nilR.IsCaseClause("expression_case") { t.Errorf(...) }
	newCond := testutil.Node("call_expression", "nilR.IsCaseClause")
	newBody := testutil.Node("block", "", testutil.Leaf("identifier", "t"))
	newIf := testutil.Node("if_statement", "", newCond, newBody)

	treesitter.EnsureIndex(if1)
	treesitter.EnsureIndex(if2)
	treesitter.EnsureIndex(newIf)

	m := NewMapping()
	// Map newIf's header to if1.
	m.Add(cond1, newCond)

	// if2 and its body shouldn't match against newIf because newIf's header belongs to if1.
	if !hasForeignHeaderOwner(if2, newIf, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be true for if2 against newIf whose header is mapped to if1")
	}
	if !hasForeignHeaderOwner(body2, newBody, m, r) {
		t.Errorf("expected hasForeignHeaderOwner to be true for body2 against newBody whose parent header is mapped to if1")
	}

	// if1 is the rightful owner, so it shouldn't be flagged as foreign.
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 7},
			testutil.NodeQuery{Type: "if_statement", Line: 7},
		)

		// Whole-snippet verification: ensure all other statements and declarations remain mapped
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 5},
			testutil.NodeQuery{Type: "expression_statement", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 8},
			testutil.NodeQuery{Type: "expression_statement", Line: 8},
		)
	})
}

func TestRollupMatchedContainers_ScopePreservedGuard(t *testing.T) {
	// Function block in Old with flat if statement:
	// func test() {
	//     if !r.IsCaseClause("foo") { t.Errorf() }
	// }
	idT1 := testutil.Leaf("identifier", "t")
	body1 := testutil.Node("block", "", idT1)
	cond1 := testutil.Node("unary_expression", "", testutil.Leaf("identifier", "r"))
	if1 := testutil.Node("if_statement", "", cond1, body1)
	fnBlock1 := testutil.Node("block", "", if1)
	fn1 := testutil.Node("function_declaration", "", fnBlock1)

	// Function block in New with a for loop containing an if statement:
	// func test() {
	//     for ... {
	//         if got := r.IsCaseClause(...); got != tt.want { t.Errorf() }
	//     }
	// }
	idT2 := testutil.Leaf("identifier", "t")
	body2 := testutil.Node("block", "", idT2)
	cond2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "got"))
	if2 := testutil.Node("if_statement", "", cond2, body2)
	forBlock := testutil.Node("block", "", if2)
	forStmt := testutil.Node("for_statement", "", forBlock)
	fnBlock2 := testutil.Node("block", "", forStmt)
	fn2 := testutil.Node("function_declaration", "", fnBlock2)

	treesitter.EnsureIndex(fn1)
	treesitter.EnsureIndex(fn2)

	m := NewMapping()
	m.Add(fn1, fn2)
	m.Add(fnBlock1, fnBlock2)
	m.Add(idT1, idT2)

	RollupMatchedContainers(fn1, fn2, m)

	// if1 shouldn't roll up into if2 across an unmapped loop boundary when Dice is low.
	if m.Get(if1) == if2 {
		t.Errorf("expected if1 NOT to roll up to if2 across unmapped loop scope boundary")
	}
}

func TestBottomUp_BinaryChainLeadingOperandDeletion(t *testing.T) {
	// Old: return A || B || C || D
	// New: return B || C || D
	// When deleting "A ||" from the front of a binary chain, inner bin3 (A || B)
	// shouldn't steal bin2 (B || C) across the operator. bin2 ((A || B) || C) should
	// match bin2 (B || C), and bin1 (((A || B) || C) || D) should match bin1 ((B || C) || D).
	mkLeaf := func(typ, label string, startByte, endByte uint32) *treesitter.ASTNode {
		n := testutil.Leaf(typ, label)
		n.Language = "go"
		n.StartByte, n.EndByte = startByte, endByte
		return n
	}
	mkContainer := func(typ string, startByte, endByte uint32, children ...*treesitter.ASTNode) *treesitter.ASTNode {
		n := testutil.Node(typ, "", children...)
		n.Language = "go"
		n.StartByte, n.EndByte = startByte, endByte
		return n
	}

	oldA := mkContainer("call_expression", 10, 25, mkLeaf("identifier", "isOp", 10, 14), mkLeaf("identifier", "n", 15, 16))
	oldOp1 := mkLeaf("logical_operator_literal", "||", 26, 28)
	oldB := mkContainer("call_expression", 29, 45, mkLeaf("identifier", "isPunct", 29, 36), mkLeaf("identifier", "n", 37, 38))
	oldBin3 := mkContainer("binary_expression", 10, 45, oldA, oldOp1, oldB)

	oldOp2 := mkLeaf("logical_operator_literal", "||", 46, 48)
	oldC := mkContainer("selector_expression", 49, 60, mkLeaf("identifier", "n", 49, 50), mkLeaf("field_identifier", "IsKeyword", 51, 60))
	oldBin2 := mkContainer("binary_expression", 10, 60, oldBin3, oldOp2, oldC)

	oldOp3 := mkLeaf("logical_operator_literal", "||", 61, 63)
	oldD := mkContainer("call_expression", 64, 80, mkLeaf("identifier", "isKw", 64, 68), mkLeaf("identifier", "n", 69, 70))
	oldBin1 := mkContainer("binary_expression", 10, 80, oldBin2, oldOp3, oldD)
	oldRet := mkContainer("return_statement", 3, 80, mkLeaf("return", "return", 3, 9), oldBin1)

	newB := mkContainer("call_expression", 10, 26, mkLeaf("identifier", "isPunct", 10, 17), mkLeaf("identifier", "n", 18, 19))
	newOp2 := mkLeaf("logical_operator_literal", "||", 27, 29)
	newC := mkContainer("selector_expression", 30, 41, mkLeaf("identifier", "n", 30, 31), mkLeaf("field_identifier", "IsKeyword", 32, 41))
	newBin2 := mkContainer("binary_expression", 10, 41, newB, newOp2, newC)

	newOp3 := mkLeaf("logical_operator_literal", "||", 42, 44)
	newD := mkContainer("call_expression", 45, 61, mkLeaf("identifier", "isKw", 45, 49), mkLeaf("identifier", "n", 50, 51))
	newBin1 := mkContainer("binary_expression", 10, 61, newBin2, newOp3, newD)
	newRet := mkContainer("return_statement", 3, 61, mkLeaf("return", "return", 3, 9), newBin1)

	treesitter.EnsureIndex(oldRet)
	treesitter.EnsureIndex(newRet)

	m := NewMapping()
	// TopDown matches calls B and D. BottomUp recovers selector C.
	m.Add(oldB, newB)
	m.Add(oldB.Children[0], newB.Children[0])
	m.Add(oldB.Children[1], newB.Children[1])
	m.Add(oldD, newD)
	m.Add(oldD.Children[0], newD.Children[0])
	m.Add(oldD.Children[1], newD.Children[1])

	BottomUp(oldRet, newRet, m, 0.30)

	if m.Has(oldBin3) {
		t.Errorf("expected inner oldBin3 (A || B) to remain unmapped, but mapped to %v", m.Get(oldBin3))
	}
	if m.Get(oldBin2) != newBin2 {
		t.Errorf("expected oldBin2 ((A || B) || C) to match newBin2 (B || C), got %v", m.Get(oldBin2))
	}
	if m.Get(oldBin1) != newBin1 {
		t.Errorf("expected oldBin1 to match newBin1, got %v", m.Get(oldBin1))
	}
	if m.Get(oldC) != newC {
		t.Errorf("expected oldC (n.IsKeyword) to match newC, got %v", m.Get(oldC))
	}
}

func TestBottomUp_NestedCallFlattening_EnclosingConstructGuard(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("go")

		// Source tree:
		// outerMax(foo(a), innerMax(0, diff))
		firstArg := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("argument_list", "", testutil.Leaf("identifier", "a")),
		)
		innerZero := testutil.Leaf("int_literal", "0")
		innerDiff := testutil.Leaf("identifier", "diff")
		innerMax := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "max"),
			testutil.Node("argument_list", "", innerZero, innerDiff),
		)
		outerMax := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "max"),
			testutil.Node("argument_list", "", firstArg, innerMax),
		)

		// Destination tree (flattened):
		// newMax(foo(a), 0, diff)
		newFirstArg := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("argument_list", "", testutil.Leaf("identifier", "a")),
		)
		newZero := testutil.Leaf("int_literal", "0")
		newDiff := testutil.Leaf("identifier", "diff")
		newMax := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "max"),
			testutil.Node("argument_list", "", newFirstArg, newZero, newDiff),
		)

		treesitter.EnsureIndex(outerMax)
		treesitter.EnsureIndex(newMax)

		m := NewMapping()
		// Map foo(a) <-> foo(a), which sits in outerMax outside innerMax.
		addIsomorphicPairs(firstArg, newFirstArg, m)
		// Map 0 <-> 0 and diff <-> diff inside innerMax.
		m.Add(innerZero, newZero)
		m.Add(innerDiff, newDiff)

		// innerMax should NOT be allowed to steal newMax because newMax contains newFirstArg,
		// whose source partner firstArg belongs to outerMax (an unmapped ancestor of innerMax).
		if !hasEnclosingConstructWithMappedDescendant(innerMax, newMax, m, r) {
			t.Errorf("expected innerMax <-> newMax candidate to be rejected by enclosing construct guard")
		}

		// outerMax, however, IS the enclosing construct, so it should not be blocked from newMax.
		if hasEnclosingConstructWithMappedDescendant(outerMax, newMax, m, r) {
			t.Errorf("expected outerMax <-> newMax candidate to be allowed")
		}

		// Symmetrically in reverse: reject if source container has descendants whose partners
		// belong to an unmapped outer ancestor of the candidate.
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

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Target call expression & argument_list container verification
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 1},
			testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 1},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 2},
		)

		// In call flattening, the inner argument_list holding surviving arguments (ts.URL)
		// maps to dstArgs, while the outer wrapper argument_list is deleted.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "argument_list", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "argument_list", Line: 5, Nth: 1},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "argument_list", Line: 5, Nth: 1},
		)

		// 2. Whole-snippet verification: enclosing function declaration and sequential body statements map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "assignment_statement", Line: 5},
			testutil.NodeQuery{Type: "assignment_statement", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 6},
			testutil.NodeQuery{Type: "expression_statement", Line: 6},
		)

		// 3. Stationary Operands: ts.URL selector expression maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "selector_expression", Line: 5, Nth: 3},
			testutil.NodeQuery{Type: "selector_expression", Line: 5, Nth: 2},
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

		mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), nil)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Target guard expression verification: checkA() && checkB() maps across inverted guards
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5},
			testutil.NodeQuery{Type: "binary_expression", Line: 6},
		)

		// 2. Enclosing function declaration maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// 3. Surrounding statements and body call map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 10},
			testutil.NodeQuery{Type: "expression_statement", Line: 10},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 7},
			testutil.NodeQuery{Type: "expression_statement", Line: 7},
		)
	})
}

func TestBottomUp_SplitStatementDescendantsRejected(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		runTest := func(t *testing.T, oldSrc, newSrc string, condAOldNth, condBOldNth int) {
			srcAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse([]byte(newSrc), "test.go")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}

			part := NewLinePartition([]byte(oldSrc), []byte(newSrc))
			mr := Match(srcAST, dstAST, []byte(oldSrc), []byte(newSrc), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Target binary expression rejection: compound expression MUST NOT map to dstBin
			testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "binary_expression", Line: 5},
			)
			testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "binary_expression", Line: 5},
				testutil.NodeQuery{Type: "binary_expression", Line: 5},
			)

			// Stationary / moved sub-expressions
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: condAOldNth},
				testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 1},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: condBOldNth},
				testutil.NodeQuery{Type: "call_expression", Line: 8, Nth: 1},
			)

			// 2. Enclosing function maps in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
			)

			// 3. Surrounding sequence: setupContext and finalizeContext map in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "expression_statement", Line: 4},
				testutil.NodeQuery{Type: "expression_statement", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "expression_statement", Line: 8},
				testutil.NodeQuery{Type: "expression_statement", Line: 11},
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
			runTest(t, oldSrc, newSrc, 1, 2)
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
			runTest(t, oldSrc, newSrc, 2, 1)
		})
	})
}

func TestHasForeignBodyOwner_InvertedNestedGuards(t *testing.T) {
	goRules := rules.Get("go")

	body1 := testutil.Node("block", "")
	condC1 := testutil.Node("identifier", "condC")
	innerIf1 := testutil.Node("if_statement", "", condC1, body1)
	block1 := testutil.Node("block", "", innerIf1)
	cond1 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "checkA"), testutil.Leaf("identifier", "checkB"))
	outerIf1 := testutil.Node("if_statement", "", cond1, block1)
	root1 := testutil.Node("source_file", "", outerIf1)
	root1.Language = "go"

	body2 := testutil.Node("block", "")
	cond2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "checkA"), testutil.Leaf("identifier", "checkB"))
	innerIf2 := testutil.Node("if_statement", "", cond2, body2)
	block2 := testutil.Node("block", "", innerIf2)
	condC2 := testutil.Node("identifier", "condC")
	outerIf2 := testutil.Node("if_statement", "", condC2, block2)
	root2 := testutil.Node("source_file", "", outerIf2)
	root2.Language = "go"

	treesitter.EnsureIndex(root1)
	treesitter.EnsureIndex(root2)

	m := NewMapping()
	m.Add(body1, body2)
	m.Add(condC1, condC2)

	if hasForeignBodyOwner(cond1, cond2, m, goRules) {
		t.Errorf("expected inverted nested guards to permit matching cond1 vs cond2")
	}

	// Parenthesized guard variant: expression wrappers shouldn't trap the construct lookup.
	bodyParen := testutil.Node("block", "")
	condCParen := testutil.Node("identifier", "condC")
	innerIfParen := testutil.Node("if_statement", "", condCParen, bodyParen)
	blockParen := testutil.Node("block", "", innerIfParen)
	rawCondParen := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "checkA"), testutil.Leaf("identifier", "checkB"))
	parenExpr := testutil.Node("parenthesized_expression", "", rawCondParen)
	outerIfParen := testutil.Node("if_statement", "", parenExpr, blockParen)
	rootParen := testutil.Node("source_file", "", outerIfParen)
	rootParen.Language = "go"
	treesitter.EnsureIndex(rootParen)

	mParen := NewMapping()
	mParen.Add(bodyParen, body2)
	mParen.Add(condCParen, condC2)

	if hasForeignBodyOwner(rawCondParen, cond2, mParen, goRules) {
		t.Errorf("expected parenthesized inverted guard to permit matching rawCondParen vs cond2")
	}

	// Unrelated construct condition should still be rejected as foreign.
	unrelatedBody := testutil.Node("block", "")
	unrelatedCond := testutil.Node("binary_expression", "")
	unrelatedIf := testutil.Node("if_statement", "", unrelatedCond, unrelatedBody)
	rootUnrelated := testutil.Node("source_file", "", unrelatedIf)
	rootUnrelated.Language = "go"
	treesitter.EnsureIndex(rootUnrelated)

	if !hasForeignBodyOwner(unrelatedCond, cond2, m, goRules) {
		t.Errorf("expected unrelated condition to be rejected by hasForeignBodyOwner")
	}
}

func TestHasSplitStatementDescendant(t *testing.T) {
	goRules := rules.Get("go")

	t.Run("LeftInsideRightOutside", func(t *testing.T) {
		d1Left := testutil.Node("identifier", "condA")
		op1 := testutil.Leaf("&&", "&&")
		d1Right := testutil.Node("identifier", "condB")
		bin1 := testutil.Node("binary_expression", "", d1Left, op1, d1Right)
		stmt1 := testutil.Node("expression_statement", "", bin1)
		root1 := testutil.Node("source_file", "", stmt1)
		root1.Language = "go"
		treesitter.EnsureIndex(root1)

		d2Left := testutil.Node("identifier", "condA")
		op2 := testutil.Leaf("||", "||")
		d2Other := testutil.Node("identifier", "other")
		bin2 := testutil.Node("binary_expression", "", d2Left, op2, d2Other)
		stmt2 := testutil.Node("expression_statement", "", bin2)

		d2Outside := testutil.Node("identifier", "condB")
		stmtOutside := testutil.Node("expression_statement", "", d2Outside)

		block2 := testutil.Node("block", "", stmt2, stmtOutside)
		root2 := testutil.Node("source_file", "", block2)
		root2.Language = "go"
		treesitter.EnsureIndex(root2)

		m := NewMapping()
		m.Add(d1Left, d2Left)
		m.Add(d1Right, d2Outside)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject bin1 vs bin2 when operands split across statements")
		}
	})

	t.Run("RightInsideLeftOutside_ReversedOrder", func(t *testing.T) {
		// When outside descendant is encountered before inside descendant in pre-order,
		// the matcher must not break prematurely.
		d1Left := testutil.Node("identifier", "condB") // outside
		op1 := testutil.Leaf("&&", "&&")
		d1Right := testutil.Node("identifier", "condA") // inside
		bin1 := testutil.Node("binary_expression", "", d1Left, op1, d1Right)
		stmt1 := testutil.Node("expression_statement", "", bin1)
		root1 := testutil.Node("source_file", "", stmt1)
		root1.Language = "go"
		treesitter.EnsureIndex(root1)

		d2Inside := testutil.Node("identifier", "condA")
		op2 := testutil.Leaf("||", "||")
		d2Other := testutil.Node("identifier", "other")
		bin2 := testutil.Node("binary_expression", "", d2Inside, op2, d2Other)
		stmt2 := testutil.Node("expression_statement", "", bin2)

		d2Outside := testutil.Node("identifier", "condB")
		stmtOutside := testutil.Node("expression_statement", "", d2Outside)

		block2 := testutil.Node("block", "", stmt2, stmtOutside)
		root2 := testutil.Node("source_file", "", block2)
		root2.Language = "go"
		treesitter.EnsureIndex(root2)

		m := NewMapping()
		m.Add(d1Right, d2Inside)
		m.Add(d1Left, d2Outside)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject bin1 vs bin2 even when outside descendant appears first in pre-order")
		}
	})

	t.Run("ReverseDirection", func(t *testing.T) {
		// Candidate c in tree 2 has operands split across multiple statements in tree 1.
		d1Left := testutil.Node("identifier", "condA")
		op1 := testutil.Leaf("&&", "&&")
		d1Unused := testutil.Node("identifier", "unused")
		bin1 := testutil.Node("binary_expression", "", d1Left, op1, d1Unused)
		stmt1 := testutil.Node("expression_statement", "", bin1)

		d1Outside := testutil.Node("identifier", "other")
		stmtOutside1 := testutil.Node("expression_statement", "", d1Outside)

		block1 := testutil.Node("block", "", stmt1, stmtOutside1)
		root1 := testutil.Node("source_file", "", block1)
		root1.Language = "go"
		treesitter.EnsureIndex(root1)

		d2Left := testutil.Node("identifier", "condA")
		op2 := testutil.Leaf("||", "||")
		d2Right := testutil.Node("identifier", "other")
		bin2 := testutil.Node("binary_expression", "", d2Left, op2, d2Right)
		stmt2 := testutil.Node("expression_statement", "", bin2)
		root2 := testutil.Node("source_file", "", stmt2)
		root2.Language = "go"
		treesitter.EnsureIndex(root2)

		m := NewMapping()
		m.Add(d1Left, d2Left)
		m.Add(d1Outside, d2Right)

		if !hasSplitStatementDescendant(bin1, bin2, m, goRules) {
			t.Errorf("expected hasSplitStatementDescendant to reject when candidate c has operands split across statements in tree 1")
		}
	})

	t.Run("SameStatementPermitted", func(t *testing.T) {
		d1Left := testutil.Node("identifier", "condA")
		op1 := testutil.Leaf("&&", "&&")
		d1Right := testutil.Node("identifier", "condB")
		bin1 := testutil.Node("binary_expression", "", d1Left, op1, d1Right)
		stmt1 := testutil.Node("expression_statement", "", bin1)
		root1 := testutil.Node("source_file", "", stmt1)
		root1.Language = "go"
		treesitter.EnsureIndex(root1)

		d2Left := testutil.Node("identifier", "condA")
		op2 := testutil.Leaf("||", "||")
		d2Right := testutil.Node("identifier", "condB")
		bin2 := testutil.Node("binary_expression", "", d2Left, op2, d2Right)
		stmt2 := testutil.Node("expression_statement", "", bin2)
		root2 := testutil.Node("source_file", "", stmt2)
		root2.Language = "go"
		treesitter.EnsureIndex(root2)

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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// 2. Surrounding Sequence: setupContext and finalizeContext map in place.
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 10},
		)

		// 3. Target Construct: Split binary expression in old statement must NOT map to destination binary expression.
		testutil.AssertNotMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 5},
			testutil.NodeQuery{Type: "binary_expression", Line: 9},
		)

		// 4. Target Statements: Statements map by variable slot (res :=), but without cross-statement operand moves.
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 5},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 9},
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
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3})
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 10},
			testutil.NodeQuery{Type: "function_declaration", Line: 10})

		// 2. Untouched Containers in stationaryHelper: if block and return statements must map in place
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4})
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 5},
			testutil.NodeQuery{Type: "return_statement", Line: 5})
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 7},
			testutil.NodeQuery{Type: "return_statement", Line: 7})

		// 3. Surrounding Sequence & Target Containers in process
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 11},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 11})
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 12},
			testutil.NodeQuery{Type: "if_statement", Line: 12})
		testutil.AssertMapped(t, oldTree, newTree, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 15},
			testutil.NodeQuery{Type: "return_statement", Line: 15})
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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding Sequence: let binding maps in place
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)

		// 3. Target & Containers:
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 4},
			testutil.NodeQuery{Type: "match_arm", Line: 4},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 11},
			testutil.NodeQuery{Type: "match_arm", Line: 7},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 18},
			testutil.NodeQuery{Type: "match_arm", Line: 16},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 19},
			testutil.NodeQuery{Type: "match_arm", Line: 17},
		)

		// 4. Stationary Operands inside Arm 1: alignment_spec, width, plus_file, config
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 13},
			testutil.NodeQuery{Type: "identifier", Line: 10},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 14},
			testutil.NodeQuery{Type: "identifier", Line: 11},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 15, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 2},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 16},
			testutil.NodeQuery{Type: "identifier", Line: 13},
		)
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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "switch_case", Line: 4},
			testutil.NodeQuery{Type: "switch_case", Line: 4},
		)

		// Stationary operands
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 5},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 6},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 6},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 7},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 7},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 8},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 8},
		)
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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "switch_rule", Line: 5},
			testutil.NodeQuery{Type: "switch_rule", Line: 5},
		)

		// Stationary operands
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 5},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 6},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 6},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 7},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 7},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 8},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 8},
		)
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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "switch_case", Line: 5},
			testutil.NodeQuery{Type: "switch_case", Line: 5},
		)

		// Stationary operands
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 5},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 6},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 6},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 7},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 7},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 8},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 8},
		)
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
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "mod_item", Line: 1},
			testutil.NodeQuery{Type: "mod_item", Line: 1},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 9},
			testutil.NodeQuery{Type: "function_item", Line: 9},
		)

		// 2. Surrounding Sequence: Preceding and succeeding use declarations and statements must map in place
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 7},
			testutil.NodeQuery{Type: "use_declaration", Line: 7},
		)

		// Verify statements inside fn test_helper map in place
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 10},
			testutil.NodeQuery{Type: "call_expression", Line: 10},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 11},
			testutil.NodeQuery{Type: "call_expression", Line: 11},
		)

		// 3. Target Construct: Target use_declaration and scoped_use_list must map in place
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 4},
			testutil.NodeQuery{Type: "use_declaration", Line: 6},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "scoped_use_list", Line: 4},
			testutil.NodeQuery{Type: "scoped_use_list", Line: 6},
		)

		// 4. Stationary Operands: Identifiers in use_list map in place
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 1},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 3},
		)
		testutil.AssertMapped(t, oldTree, newTree, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 4},
		)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 1},
				testutil.NodeQuery{Type: "function_item", Line: 1},
			)

			// Surrounding for loop mapped in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "for_expression", Line: 2},
				testutil.NodeQuery{Type: "for_expression", Line: 2},
			)

			// Target calls: old zip_eq should match inner zip_eq, leaving inserted outer unmapped.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 2},
				testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 3},
			)
			testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
				testutil.NodeQuery{Type: "call_expression", Line: 2, Nth: 2},
			)

			// Arguments map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "arguments", Line: 4, Nth: 1},
				testutil.NodeQuery{Type: "arguments", Line: 4, Nth: 1},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
			)

			// Surrounding expression statement mapped in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "expression_statement", Line: 4},
				testutil.NodeQuery{Type: "expression_statement", Line: 4},
			)

			// Target calls: old ZipEq should match inner ZipEq, leaving inserted outer unmapped.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 4, Nth: 2},
				testutil.NodeQuery{Type: "call_expression", Line: 4, Nth: 3},
			)
			testutil.AssertUnmapped(t, dstAST, mr.Mappings.Dst(),
				testutil.NodeQuery{Type: "call_expression", Line: 4, Nth: 2},
			)

			// Arguments map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "argument_list", Line: 6, Nth: 1},
				testutil.NodeQuery{Type: "argument_list", Line: 6, Nth: 1},
			)
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: let mut replace_args
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)

		// 3. Target: outer expression statement in src should NOT map to
		// outer statement of _make_process in dst.
		testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 3},
			testutil.NodeQuery{Type: "expression_statement", Line: 16},
		)

		// 4. Extracted statement: let mut p = Command::new(&name) should map to let mut p = Command::new(&path)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 6},
			testutil.NodeQuery{Type: "let_declaration", Line: 17},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// Surrounding sequence: arm 0 and arm 1 are untouched, so they stay mapped in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 12},
			testutil.NodeQuery{Type: "match_arm", Line: 13},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 13},
			testutil.NodeQuery{Type: "match_arm", Line: 14},
		)

		// Target construct: arm 2 (false, Some(index)) should map in place despite the nested match rewrite.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 14},
			testutil.NodeQuery{Type: "match_arm", Line: 15},
		)

		// Stationary operands: compute_fill call inside arm 2 maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 16},
			testutil.NodeQuery{Type: "call_expression", Line: 17},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "mod_item", Line: 2},
			testutil.NodeQuery{Type: "mod_item", Line: 2},
		)

		// 2. Surrounding sequence: preceding unchanged imports (lines 3 to 7) map in place
		for line := 3; line <= 7; line++ {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "use_declaration", Line: line},
				testutil.NodeQuery{Type: "use_declaration", Line: line},
			)
		}

		// Succeeding function test_added_file maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 12},
			testutil.NodeQuery{Type: "function_item", Line: 12},
		)

		// 3. Target & Containers: both shortened use_declaration nodes at lines 8 and 9 must map in place
		for _, line := range []int{8, 9} {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "use_declaration", Line: line},
				testutil.NodeQuery{Type: "use_declaration", Line: line},
			)
		}

		// 4. Stationary operands: shared prefix scoped_identifier must map to destination root scoped_identifier
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "scoped_identifier", Line: 8, Nth: 2},
			testutil.NodeQuery{Type: "scoped_identifier", Line: 8, Nth: 1},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "class_declaration", Line: 6},
			testutil.NodeQuery{Type: "class_declaration", Line: 6},
		)

		// 2. Targets: shortened import_declaration nodes at lines 3 and 4 must map in place.
		for _, line := range []int{3, 4} {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "import_declaration", Line: line},
				testutil.NodeQuery{Type: "import_declaration", Line: line},
			)
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Surrounding sequence: local x = 1 maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "variable_declaration", Line: 4},
			testutil.NodeQuery{Type: "variable_declaration", Line: 4},
		)

		// 3. Targets: function_call nodes at lines 2 and 3 map in place.
		for _, line := range []int{2, 3} {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_call", Line: line},
				testutil.NodeQuery{Type: "function_call", Line: line},
			)
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Surrounding sequence: const c = 1 maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "variable_declaration", Line: 4},
			testutil.NodeQuery{Type: "variable_declaration", Line: 4},
		)

		// 3. Targets: variable_declaration nodes at lines 2 and 3 map in place.
		for _, line := range []int{2, 3} {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: line},
				testutil.NodeQuery{Type: "variable_declaration", Line: line},
			)
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "closure_expression", Line: 2},
			testutil.NodeQuery{Type: "closure_expression", Line: 2},
		)

		// 2. Surrounding sequence: succeeding println! statement maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "macro_invocation", Line: 6},
			testutil.NodeQuery{Type: "macro_invocation", Line: 5},
		)

		// 3. Target: eprintln! must map to format!, NOT the outer call fatal
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "macro_invocation", Line: 3},
			testutil.NodeQuery{Type: "macro_invocation", Line: 3},
		)

		// 4. Stationary Operands: token_tree maps in place across eprintln! -> format!
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "token_tree", Line: 3},
			testutil.NodeQuery{Type: "token_tree", Line: 3},
		)
	})
}

func TestBottomUp_HollowEnclosingCall_WrappedCallEvolution_Go(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
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

		r := rulesFor(srcAST)
		part := NewLinePartition(src, dst)
		mr := Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: package clause and process function
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "process" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "process" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("process function mapping got %v, want %v", mr.Mappings.Src()[srcFn], dstFn)
		}

		// 2. Surrounding sequence: succeeding println statement
		var srcPrint, dstPrint *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "println" {
				srcPrint = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "println" {
				dstPrint = n
				break
			}
		}
		if srcPrint == nil || dstPrint == nil || mr.Mappings.Src()[srcPrint] != dstPrint {
			t.Errorf("println mapping got %v, want %v", mr.Mappings.Src()[srcPrint], dstPrint)
		}

		// 3. Target: render call must map to format, not outer wrapper die
		var srcRender, dstDie, dstFormat *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "render" {
				srcRender = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsCall(n.Type) {
				switch extractCalleeLabel(n, r) {
				case "die":
					dstDie = n
				case "format":
					dstFormat = n
				}
			}
		}
		if srcRender == nil || dstDie == nil || dstFormat == nil {
			t.Fatalf("failed to locate target calls: srcRender=%v, dstDie=%v, dstFormat=%v", srcRender, dstDie, dstFormat)
		}

		if mapped := mr.Mappings.Src()[srcRender]; mapped != dstFormat {
			t.Fatalf("render mapping got %v, want %v (die=%v)", mapped, dstFormat, dstDie)
		}

		// 4. Stationary operands: binary expression argument
		var srcArg, dstArg *treesitter.ASTNode
		for _, d := range srcRender.Descendants() {
			if d.Type == "binary_expression" {
				srcArg = d
				break
			}
		}
		for _, d := range dstFormat.Descendants() {
			if d.Type == "binary_expression" {
				dstArg = d
				break
			}
		}
		if srcArg == nil || dstArg == nil || mr.Mappings.Src()[srcArg] != dstArg {
			t.Errorf("binary_expression argument mapping got %v, want %v", mr.Mappings.Src()[srcArg], dstArg)
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)

		// 2. Surrounding Sequence: succeeding println! statement maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "macro_invocation", Line: 4},
			testutil.NodeQuery{Type: "macro_invocation", Line: 4},
		)

		// 3. Target & Containers: outer field_expression (.width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 3},
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 2},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 2},
		)

		// 4. Stationary Operands: base field_expression config.side_by_side_data maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 3},
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 2},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)

		// 2. Surrounding Sequence: succeeding println call maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 6},
		)

		// 3. Target & Containers: outer selector_expression (.width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "selector_expression", Line: 5, Nth: 1},
			testutil.NodeQuery{Type: "selector_expression", Line: 5, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 5, Nth: 3},
			testutil.NodeQuery{Type: "field_identifier", Line: 5, Nth: 2},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 5, Nth: 2},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
		)

		// 2. Surrounding Sequence: succeeding console.log call maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
		)

		// 3. Target & Containers: outer member_expression (.width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "member_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "member_expression", Line: 3, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "property_identifier", Line: 3, Nth: 3},
			testutil.NodeQuery{Type: "property_identifier", Line: 3, Nth: 2},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "property_identifier", Line: 3, Nth: 2},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_definition", Line: 1},
			testutil.NodeQuery{Type: "function_definition", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 2},
			testutil.NodeQuery{Type: "expression_statement", Line: 2},
		)

		// 2. Surrounding Sequence: succeeding print statement maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
		)

		// 3. Target & Containers: outer attribute (.width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "attribute", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "attribute", Line: 3, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 5},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 4},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
		)

		// 2. Surrounding Sequence: succeeding println statement maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 5},
			testutil.NodeQuery{Type: "expression_statement", Line: 5},
		)

		// 3. Target & Containers: outer field_access (.width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_access", Line: 4, Nth: 1},
			testutil.NodeQuery{Type: "field_access", Line: 4, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 5},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 4},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_definition", Line: 1},
			testutil.NodeQuery{Type: "function_definition", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "declaration", Line: 2},
			testutil.NodeQuery{Type: "declaration", Line: 2},
		)

		// 2. Surrounding Sequence: succeeding printf statement maps in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
			testutil.NodeQuery{Type: "expression_statement", Line: 4},
		)

		// 3. Target & Containers: outer field_expression (->width) must map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "field_expression", Line: 3, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 3},
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 2},
		)
		testutil.AssertUnmapped(t, srcAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 3, Nth: 2},
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

			// 1. Enclosing context: function declaration maps in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: tt.fnType, Line: 1},
				testutil.NodeQuery{Type: tt.fnType, Line: 1},
			)

			// 2. Surrounding sequence: preceding total = 0 and succeeding record_metric map in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: tt.totalType, Line: 2},
				testutil.NodeQuery{Type: tt.totalType, Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "expression_statement", Line: tt.srcMetricLine},
				testutil.NodeQuery{Type: "expression_statement", Line: tt.dstMetricLine},
			)

			// 3. Target clause: else_clause must map in place to dst else_clause
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: tt.elseType, Line: 5},
				testutil.NodeQuery{Type: tt.elseType, Line: tt.dstElseLine},
			)

			// 4. Alternative body block must map in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: tt.elseBodyType, Line: tt.elseBodyLine},
				testutil.NodeQuery{Type: tt.elseBodyType, Line: tt.dstElseBodyLine},
			)

			// 5. Stationary inner statement handle_batch must map in place
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "expression_statement", Line: 7},
				testutil.NodeQuery{Type: "expression_statement", Line: tt.dstBatchLine},
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
				var srcFn, dstFn *treesitter.ASTNode
				for _, n := range srcAST.Descendants() {
					if r.IsDeclaration(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte(tt.targetFunc)) {
						srcFn = n
						break
					}
				}
				for _, n := range dstAST.Descendants() {
					if r.IsDeclaration(n.Type) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(tt.targetFunc)) {
						dstFn = n
						break
					}
				}
				if srcFn == nil || dstFn == nil {
					t.Fatalf("missing target function: src=%v dst=%v", srcFn, dstFn)
				}
				if mr.Mappings.Src()[srcFn] != dstFn {
					t.Errorf("expected function declaration to map in place, got %v", mr.Mappings.Src()[srcFn])
				}

				// 2. Surrounding sequence: preceding and succeeding statements map in place
				var srcPre, dstPre *treesitter.ASTNode
				for _, n := range srcAST.Descendants() {
					if bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte(tt.preceding)) && (r.IsStatement(n.Type) || r.IsLocalVarDeclaration(n.Type)) {
						srcPre = n
						break
					}
				}
				for _, n := range dstAST.Descendants() {
					if bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(tt.preceding)) && (r.IsStatement(n.Type) || r.IsLocalVarDeclaration(n.Type)) {
						dstPre = n
						break
					}
				}
				if srcPre == nil || dstPre == nil || mr.Mappings.Src()[srcPre] != dstPre {
					t.Errorf("expected preceding statement %q to map in place, got %v", tt.preceding, mr.Mappings.Src()[srcPre])
				}

				var srcPost, dstPost *treesitter.ASTNode
				for _, n := range srcAST.Descendants() {
					if bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte(tt.succeeding)) && r.IsStatement(n.Type) {
						srcPost = n
						break
					}
				}
				for _, n := range dstAST.Descendants() {
					if bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(tt.succeeding)) && r.IsStatement(n.Type) {
						dstPost = n
						break
					}
				}
				if srcPost == nil || dstPost == nil || mr.Mappings.Src()[srcPost] != dstPost {
					t.Errorf("expected succeeding statement %q to map in place, got %v", tt.succeeding, mr.Mappings.Src()[srcPost])
				}

				// 3. Stationary sub-expression: sibling flag call maps in place
				flagName := tt.flagMethod
				if flagName == "" {
					flagName = "get_flag"
				}
				var srcFlag, dstFlag *treesitter.ASTNode
				for _, n := range srcAST.Descendants() {
					if bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte(flagName)) {
						if r.IsCall(n.Type) {
							srcFlag = n
							break
						}
					}
				}
				for _, n := range dstAST.Descendants() {
					if bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(flagName)) {
						if r.IsCall(n.Type) {
							dstFlag = n
							break
						}
					}
				}
				if srcFlag == nil || dstFlag == nil || mr.Mappings.Src()[srcFlag] != dstFlag {
					t.Errorf("expected stationary flag call %q to map in place, got %v", flagName, mr.Mappings.Src()[srcFlag])
				}

				// 4. Target call & selector: old method call must map to inner call, NOT to outer wrapper call
				var srcCall, dstInnerCall, dstOuterCall *treesitter.ASTNode
				for _, n := range srcAST.Descendants() {
					if r.IsCall(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte(tt.oldMethod)) {
						srcCall = n
						break
					}
				}
				for _, n := range dstAST.Descendants() {
					if r.IsCall(n.Type) {
						if bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(tt.outerMethod)) {
							dstOuterCall = n
						} else if bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte(tt.innerMethod)) {
							dstInnerCall = n
						}
					}
				}
				if srcCall == nil || dstInnerCall == nil || dstOuterCall == nil {
					t.Fatalf("missing target call nodes: srcCall=%v dstInnerCall=%v dstOuterCall=%v", srcCall, dstInnerCall, dstOuterCall)
				}

				mappedCall := mr.Mappings.Src()[srcCall]
				if mappedCall == dstOuterCall {
					t.Errorf("update theft detected: old call %q mapped to outer wrapper %q instead of inner call %q",
						string(srcBytes[srcCall.StartByte:srcCall.EndByte]),
						string(dstBytes[dstOuterCall.StartByte:dstOuterCall.EndByte]),
						string(dstBytes[dstInnerCall.StartByte:dstInnerCall.EndByte]))
				}
				if mappedCall != dstInnerCall {
					t.Errorf("expected old call %q to map to inner call %q, got %v",
						string(srcBytes[srcCall.StartByte:srcCall.EndByte]),
						string(dstBytes[dstInnerCall.StartByte:dstInnerCall.EndByte]),
						mappedCall)
				}

				// 5. Method identifier: oldMethod must map to innerMethod, NOT outerMethod
				var srcMethodId, dstInnerMethodId, dstOuterMethodId *treesitter.ASTNode
				for _, n := range srcCall.Descendants() {
					if (r.IsIdentifier(n.Type) || n.Label != "") && string(srcBytes[n.StartByte:n.EndByte]) == tt.oldMethod {
						srcMethodId = n
						break
					}
				}
				for _, n := range dstInnerCall.Descendants() {
					if (r.IsIdentifier(n.Type) || n.Label != "") && string(dstBytes[n.StartByte:n.EndByte]) == tt.innerMethod {
						dstInnerMethodId = n
						break
					}
				}
				for _, n := range dstOuterCall.Descendants() {
					if (r.IsIdentifier(n.Type) || n.Label != "") && string(dstBytes[n.StartByte:n.EndByte]) == tt.outerMethod {
						dstOuterMethodId = n
						break
					}
				}
				if srcMethodId == nil || dstInnerMethodId == nil || dstOuterMethodId == nil {
					t.Fatalf("missing method identifiers: srcMethodId=%v dstInnerMethodId=%v dstOuterMethodId=%v", srcMethodId, dstInnerMethodId, dstOuterMethodId)
				}

				mappedMethodId := mr.Mappings.Src()[srcMethodId]
				if mappedMethodId == dstOuterMethodId {
					t.Errorf("update theft detected: method identifier %q mapped to outer method %q", tt.oldMethod, tt.outerMethod)
				}
				if mappedMethodId != dstInnerMethodId {
					t.Errorf("expected method identifier %q to map to inner method %q, got %v", tt.oldMethod, tt.innerMethod, mappedMethodId)
				}
			})
		}
	})
}

func TestBottomUp_BinaryOperatorOperandSlots_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("python")

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
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "compute_scores" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "compute_scores" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected compute_scores to map in place, got src=%v dst=%v", srcFn, dstFn)
		}

		// 2. Surrounding sequence (init_context, finalize_context) maps in place.
		var srcInit, dstInit, srcFinalize, dstFinalize *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsCall(n.Type) {
				if extractCalleeLabel(n, r) == "init_context" {
					srcInit = n
				} else if extractCalleeLabel(n, r) == "finalize_context" {
					srcFinalize = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsCall(n.Type) {
				if extractCalleeLabel(n, r) == "init_context" {
					dstInit = n
				} else if extractCalleeLabel(n, r) == "finalize_context" {
					dstFinalize = n
				}
			}
		}
		if srcInit == nil || dstInit == nil || mr.Mappings.Src()[srcInit] != dstInit {
			t.Errorf("expected init_context to map in place")
		}
		if srcFinalize == nil || dstFinalize == nil || mr.Mappings.Src()[srcFinalize] != dstFinalize {
			t.Errorf("expected finalize_context to map in place")
		}

		// 3. Locate operands: a, b in src, a, d in dst, and c in both.
		var srcA, srcB, srcC, dstA, dstD, dstC *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsIdentifier(n.Type) && n.StartRow == 2 {
				switch n.Label {
				case "a":
					srcA = n
				case "b":
					srcB = n
				case "c":
					srcC = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsIdentifier(n.Type) && n.StartRow == 2 {
				switch n.Label {
				case "a":
					dstA = n
				case "d":
					dstD = n
				case "c":
					dstC = n
				}
			}
		}

		if srcA == nil || srcC == nil || dstA == nil || dstC == nil {
			t.Fatalf("failed to locate operands: srcA=%v srcC=%v dstA=%v dstC=%v", srcA, srcC, dstA, dstC)
		}

		// 4. Verification: Left operand a maps to dstA, right operand c maps to dstC.
		if matched := mr.Mappings.Src()[srcA]; matched != dstA {
			t.Errorf("expected left operand 'a' to map in place, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcC]; matched != dstC {
			t.Errorf("expected right operand 'c' to map in place, got %v", matched)
		}
		if srcB != nil && mr.Mappings.Src()[srcB] == dstC {
			t.Errorf("deleted operand 'b' must not steal outer operand 'c'")
		}
		if dstD != nil && mr.Mappings.Dst()[dstD] == srcC {
			t.Errorf("inserted operand 'd' must not claim outer operand 'c'")
		}
	})
}

func TestBottomUp_AffinityFieldMutualExclusion_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("c")

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
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "evaluate" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "evaluate" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected evaluate function to map in place, got src=%v dst=%v", srcFn, dstFn)
		}

		// 2. Surrounding sequence (setup, teardown) maps in place.
		var srcSetup, dstSetup, srcTeardown, dstTeardown *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsCall(n.Type) {
				if extractCalleeLabel(n, r) == "setup" {
					srcSetup = n
				} else if extractCalleeLabel(n, r) == "teardown" {
					srcTeardown = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsCall(n.Type) {
				if extractCalleeLabel(n, r) == "setup" {
					dstSetup = n
				} else if extractCalleeLabel(n, r) == "teardown" {
					dstTeardown = n
				}
			}
		}
		if srcSetup == nil || dstSetup == nil || mr.Mappings.Src()[srcSetup] != dstSetup {
			t.Errorf("expected setup call to map in place")
		}
		if srcTeardown == nil || dstTeardown == nil || mr.Mappings.Src()[srcTeardown] != dstTeardown {
			t.Errorf("expected teardown call to map in place")
		}

		// 3. The if statement maps in place.
		var srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsStatement(n.Type) && n.ChildByFieldName("condition") != nil {
				srcIf = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsStatement(n.Type) && n.ChildByFieldName("condition") != nil {
				dstIf = n
				break
			}
		}
		if srcIf == nil || dstIf == nil || mr.Mappings.Src()[srcIf] != dstIf {
			t.Fatalf("expected if statement to map in place, got src=%v dst=%v", srcIf, dstIf)
		}

		// 4. The if condition shouldn't match the consequence body.
		srcCond := srcIf.ChildByFieldName("condition")
		dstBody := dstIf.ChildByFieldName("consequence")
		if dstBody == nil {
			dstBody = dstIf.ChildByFieldName("body")
		}
		if srcCond != nil && dstBody != nil {
			if mr.Mappings.Src()[srcCond] == dstBody {
				t.Errorf("condition node mapped to consequence body across mutually exclusive fields")
			}
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3})

		// 2. Surrounding sequence: preceding nil guard and succeeding statements map in place
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 9},
			testutil.NodeQuery{Type: "if_statement", Line: 10})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 12},
			testutil.NodeQuery{Type: "return_statement", Line: 13})

		// 3. Target construct: short var declaration isPunct := ...
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 7},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 7})

		// 4. Expression mappings: outerAnd (&&) and innerOr (||)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 7, Nth: 2},
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "binary_expression", Line: 7, Nth: 5},
			testutil.NodeQuery{Type: "binary_expression", Line: 8, Nth: 4})

		// 5. Operator glue: outerAnd && and innerOr ||
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "logical_operator_literal", Line: 7, Nth: 1},
			testutil.NodeQuery{Type: "logical_operator_literal", Line: 8, Nth: 1})
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "logical_operator_literal", Line: 7, Nth: 2},
			testutil.NodeQuery{Type: "logical_operator_literal", Line: 8, Nth: 2})
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

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("missing rules for TypeScript")
		}

		part := NewLinePartition(srcBytes, dstBytes)
		mr := Match(srcAST, dstAST, srcBytes, dstBytes, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: function declaration maps in place
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("check")) {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("check")) {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil {
			t.Fatalf("missing target function: src=%v dst=%v", srcFn, dstFn)
		}
		if mr.Mappings.Src()[srcFn] != dstFn {
			t.Errorf("expected function declaration to map in place, got %v", mr.Mappings.Src()[srcFn])
		}

		// 2. Surrounding sequence: preceding guard and succeeding statements map in place
		var srcPre, dstPre *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("!n")) {
				srcPre = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("!n")) {
				dstPre = n
				break
			}
		}
		if srcPre == nil || dstPre == nil || mr.Mappings.Src()[srcPre] != dstPre {
			t.Errorf("expected preceding guard statement to map in place, got %v", mr.Mappings.Src()[srcPre])
		}

		var srcPost1, dstPost1 *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("if (isPunct)")) {
				srcPost1 = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("if (isPunct)")) {
				dstPost1 = n
				break
			}
		}
		if srcPost1 == nil || dstPost1 == nil || mr.Mappings.Src()[srcPost1] != dstPost1 {
			t.Errorf("expected succeeding if (isPunct) statement to map in place, got %v", mr.Mappings.Src()[srcPost1])
		}

		// 3. Target construct: lexical declaration const isPunct = ...
		var srcStmt, dstStmt *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if (r.IsStatement(n.Type) || r.IsLocalVarDeclaration(n.Type)) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("isPunct =")) {
				srcStmt = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if (r.IsStatement(n.Type) || r.IsLocalVarDeclaration(n.Type)) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("isPunct =")) {
				dstStmt = n
				break
			}
		}
		if srcStmt == nil || dstStmt == nil {
			t.Fatalf("missing isPunct declaration: srcStmt=%v dstStmt=%v", srcStmt, dstStmt)
		}
		if mr.Mappings.Src()[srcStmt] != dstStmt {
			t.Errorf("expected isPunct declaration to map in place, got %v", mr.Mappings.Src()[srcStmt])
		}

		// 4. Expression mappings: outerAnd (&&) and innerOr (||)
		var srcOuterAnd, dstOuterAnd *treesitter.ASTNode
		for _, n := range srcStmt.Descendants() {
			if r.IsExpression(n.Type) && bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("r !== null")) &&
				bytes.Contains(srcBytes[n.StartByte:n.EndByte], []byte("isPunct")) {
				if op := findOperatorGlue(n, r); op != nil && op.Label == "&&" {
					srcOuterAnd = n
					break
				}
			}
		}
		for _, n := range dstStmt.Descendants() {
			if r.IsExpression(n.Type) && bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("r !== null")) &&
				bytes.Contains(dstBytes[n.StartByte:n.EndByte], []byte("isPunct")) {
				if op := findOperatorGlue(n, r); op != nil && op.Label == "&&" {
					dstOuterAnd = n
					break
				}
			}
		}
		if srcOuterAnd == nil || dstOuterAnd == nil {
			t.Fatalf("missing outerAnd (&&) expression: srcOuterAnd=%v dstOuterAnd=%v", srcOuterAnd, dstOuterAnd)
		}

		mappedOuterAnd := mr.Mappings.Src()[srcOuterAnd]
		if mappedOuterAnd != dstOuterAnd {
			t.Errorf("expected outerAnd (&&) to map to dstOuterAnd (&&), got %v", mappedOuterAnd)
		}
	})
}
