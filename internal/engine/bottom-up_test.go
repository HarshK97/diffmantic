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
	ifNode := testutil.Node("if", "", block1Deep)    // block1Deep.Parent = ifNode
	parent1Deep := testutil.Node("func", "", ifNode) // block1Deep depth = 2

	block2Shallow := testutil.Node("block", "", sharedLeaf2d, testutil.Leaf("id", "x"))
	parent2Shallow := testutil.Node("func", "", block2Shallow) // block2Shallow depth = 1

	mSame := NewMapping()
	mSame.Add(sharedLeaf1, sharedLeaf2)
	mSame.Add(parent1, parent2)
	scoreSameDepth := computeAffinity(block1, block2, mSame, DefaultAffinityWeights, false)

	// depthPenalty = DepthCoeff × (2−1)² = 0.20 × 1 = 0.20 → cross-depth score is lower.
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

	ContestContainers(root1, root2, m)

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

	ContestContainers(root1, root2, m)

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
