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

	ContestContainers(root1, root2, m)

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
	t.Run("Unit", func(t *testing.T) {
		r := rules.Get("go")

		// When if1 drops "|| r == nil" and newIf2 adds it, TopDown can cross-match
		// that small nil guard across the two if-statements. The primary conditions
		// still match in place, so hasForeignHeaderOwner shouldn't veto either one.
		mainCond1 := testutil.Node("binary_expression", "",
			testutil.Node("binary_expression", "", testutil.Leaf("identifier", "src"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil")),
			testutil.Leaf("logical_operator_literal", "||"),
			testutil.Node("binary_expression", "", testutil.Leaf("identifier", "dst"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil")),
		)
		strayRNil1 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "r"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil"))
		compoundCond1 := testutil.Node("binary_expression", "", mainCond1, testutil.Leaf("logical_operator_literal", "||"), strayRNil1)
		compBody1 := testutil.Node("block", "", testutil.Leaf("return", "return"))
		compIf1 := testutil.Node("if_statement", "", compoundCond1, compBody1)

		newMainCond1 := testutil.Node("binary_expression", "",
			testutil.Node("binary_expression", "", testutil.Leaf("identifier", "src"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil")),
			testutil.Leaf("logical_operator_literal", "||"),
			testutil.Node("binary_expression", "", testutil.Leaf("identifier", "dst"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil")),
		)
		newCompBody1 := testutil.Node("block", "", testutil.Leaf("return", "return"))
		newCompIf1 := testutil.Node("if_statement", "", newMainCond1, newCompBody1)

		mainCond2 := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "sameScopeDeclaration"),
			testutil.Node("argument_list", "", testutil.Leaf("identifier", "src"), testutil.Leaf("identifier", "dst"), testutil.Leaf("identifier", "ms"), testutil.Leaf("identifier", "r")),
		)
		compBody2 := testutil.Node("block", "", testutil.Leaf("identifier", "score"))
		compIf2 := testutil.Node("if_statement", "", mainCond2, compBody2)

		newMainCond2 := testutil.Node("call_expression", "",
			testutil.Leaf("identifier", "sameScopeDeclaration"),
			testutil.Node("argument_list", "", testutil.Leaf("identifier", "src"), testutil.Leaf("identifier", "dst"), testutil.Leaf("identifier", "ms"), testutil.Leaf("identifier", "r")),
		)
		strayRNil2 := testutil.Node("binary_expression", "", testutil.Leaf("identifier", "r"), testutil.Leaf("comparison_operator_literal", "=="), testutil.Leaf("nil", "nil"))
		newCompoundCond2 := testutil.Node("binary_expression", "", newMainCond2, testutil.Leaf("logical_operator_literal", "&&"), strayRNil2)
		newCompBody2 := testutil.Node("block", "", testutil.Leaf("identifier", "score"))
		newCompIf2 := testutil.Node("if_statement", "", newCompoundCond2, newCompBody2)

		root1 := testutil.Node("block", "", compIf1, compIf2)
		root2 := testutil.Node("block", "", newCompIf1, newCompIf2)
		treesitter.EnsureIndex(root1)
		treesitter.EnsureIndex(root2)

		m2 := NewMapping()
		addIsomorphicPairs(mainCond1, newMainCond1, m2)
		addIsomorphicPairs(mainCond2, newMainCond2, m2)
		addIsomorphicPairs(strayRNil1, strayRNil2, m2)
		addIsomorphicPairs(compBody1, newCompBody1, m2)
		addIsomorphicPairs(compBody2, newCompBody2, m2)

		if hasForeignHeaderOwner(compIf1, newCompIf1, m2, r) {
			t.Errorf("expected minority foreign header match (r == nil) not to veto compIf1 <-> newCompIf1")
		}
		if hasForeignHeaderOwner(compIf2, newCompIf2, m2, r) {
			t.Errorf("expected minority foreign header match (r == nil) not to veto compIf2 <-> newCompIf2")
		}

		BottomUp(root1, root2, m2, 0.50)

		if m2.Get(compIf1) != newCompIf1 {
			t.Errorf("expected BottomUp to pair compIf1 with newCompIf1, got %v", m2.Get(compIf1))
		}
		if m2.Get(compIf2) != newCompIf2 {
			t.Errorf("expected BottomUp to pair compIf2 with newCompIf2, got %v", m2.Get(compIf2))
		}
	})

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

		var srcIfs []*treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "if_statement" {
				srcIfs = append(srcIfs, n)
			}
		}
		var dstIfs []*treesitter.ASTNode
		for _, n := range dstAST.Descendants() {
			if n.Type == "if_statement" {
				dstIfs = append(dstIfs, n)
			}
		}

		if len(srcIfs) != 2 || len(dstIfs) != 2 {
			t.Fatalf("expected 2 if_statements in src and dst, got src=%d dst=%d", len(srcIfs), len(dstIfs))
		}

		if matched := mr.Mappings.Src()[srcIfs[0]]; matched != dstIfs[0] {
			t.Errorf("expected src if #1 to match dst if #1 in place, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcIfs[1]]; matched != dstIfs[1] {
			t.Errorf("expected src if #2 to match dst if #2 in place, got %v", matched)
		}

		// Whole-snippet verification: ensure all other statements and declarations remain mapped
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "function_declaration" {
				srcFunc = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "function_declaration" {
				dstFunc = n
				break
			}
		}
		if srcFunc == nil || dstFunc == nil {
			t.Fatalf("missing function_declaration: src=%v, dst=%v", srcFunc, dstFunc)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected func f to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}

		// Body statements inside both if blocks must remain mapped
		var srcCalls, dstCalls []*treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "expression_statement" {
				srcCalls = append(srcCalls, n)
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "expression_statement" {
				dstCalls = append(dstCalls, n)
			}
		}
		if len(srcCalls) != 2 || len(dstCalls) != 2 {
			t.Fatalf("expected 2 expression_statements in src and dst, got src=%d, dst=%d", len(srcCalls), len(dstCalls))
		}
		if mr.Mappings.Src()[srcCalls[0]] != dstCalls[0] {
			t.Errorf("expected foo call to map in place, got %v", mr.Mappings.Src()[srcCalls[0]])
		}
		if mr.Mappings.Src()[srcCalls[1]] != dstCalls[1] {
			t.Errorf("expected baz call to map in place, got %v", mr.Mappings.Src()[srcCalls[1]])
		}
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

		r := rulesFor(srcAST)

		// 1. Target call expression & argument_list container verification
		var srcHttpGet, srcSprintf, dstHttpGet *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) {
				callee := extractCalleeLabel(n, r)
				switch callee {
				case "http":
					srcHttpGet = n
				case "fmt":
					srcSprintf = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && extractCalleeLabel(n, r) == "http" {
				dstHttpGet = n
			}
		}

		if srcHttpGet == nil || srcSprintf == nil || dstHttpGet == nil {
			t.Fatalf("failed to locate call nodes: srcHttpGet=%v srcSprintf=%v dstHttpGet=%v",
				srcHttpGet, srcSprintf, dstHttpGet)
		}

		if matched := mr.Mappings.Src()[srcHttpGet]; matched != dstHttpGet {
			t.Errorf("expected outer http.Get to match dstHttpGet, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcSprintf]; matched != nil {
			t.Errorf("inner fmt.Sprintf must remain unmapped, got %v", matched)
		}

		// Verify outer argument_list matches dstHttpGet argument_list, and inner argument_list is deleted
		var srcOuterArgs, srcInnerArgs, dstArgs *treesitter.ASTNode
		for _, c := range srcHttpGet.Children {
			if r.IsDelimitedContainer(c.Type) {
				srcOuterArgs = c
			}
		}
		for _, c := range srcSprintf.Children {
			if r.IsDelimitedContainer(c.Type) {
				srcInnerArgs = c
			}
		}
		for _, c := range dstHttpGet.Children {
			if r.IsDelimitedContainer(c.Type) {
				dstArgs = c
			}
		}

		if srcOuterArgs == nil || srcInnerArgs == nil || dstArgs == nil {
			t.Fatalf("failed to locate argument_list containers: srcOuterArgs=%v srcInnerArgs=%v dstArgs=%v",
				srcOuterArgs, srcInnerArgs, dstArgs)
		}
		// In call flattening, the inner argument_list holding surviving arguments (ts.URL)
		// maps to dstArgs, while the outer wrapper argument_list is deleted.
		if matched := mr.Mappings.Src()[srcInnerArgs]; matched != dstArgs {
			t.Errorf("expected inner fmt.Sprintf argument_list to match dstArgs, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcOuterArgs]; matched != nil {
			t.Errorf("outer http.Get argument_list must remain unmapped, got %v", matched)
		}

		// 2. Whole-snippet verification: enclosing function declaration maps in place
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				srcFunc = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				dstFunc = n
				break
			}
		}
		if srcFunc == nil || dstFunc == nil {
			t.Fatalf("missing function_declaration: src=%v, dst=%v", srcFunc, dstFunc)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected func f to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}

		// Verify all 3 sequential statements in func body map in place
		var srcStmts, dstStmts []*treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && n.Parent != nil && r.IsBlock(n.Parent.Type) {
				srcStmts = append(srcStmts, n)
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && n.Parent != nil && r.IsBlock(n.Parent.Type) {
				dstStmts = append(dstStmts, n)
			}
		}
		if len(srcStmts) != 3 || len(dstStmts) != 3 {
			t.Fatalf("expected 3 body statements in src and dst, got src=%d, dst=%d", len(srcStmts), len(dstStmts))
		}
		for i := range srcStmts {
			if matched := mr.Mappings.Src()[srcStmts[i]]; matched != dstStmts[i] {
				t.Errorf("expected body statement #%d to map in place, got %v", i, matched)
			}
		}

		// 3. Stationary Operands: ts.URL selector expression maps in place
		var srcTsURL, dstTsURL *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "selector_expression" && len(n.Children) >= 2 && n.Children[1].Label == "URL" {
				srcTsURL = n
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "selector_expression" && len(n.Children) >= 2 && n.Children[1].Label == "URL" {
				dstTsURL = n
			}
		}
		if srcTsURL == nil || dstTsURL == nil {
			t.Fatalf("missing ts.URL selector_expression: src=%v dst=%v", srcTsURL, dstTsURL)
		}
		if matched := mr.Mappings.Src()[srcTsURL]; matched != dstTsURL {
			t.Errorf("expected ts.URL to map in place, got %v", matched)
		}
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

		r := rulesFor(srcAST)
		if r == nil {
			t.Fatal("rulesFor returned nil")
		}

		// 1. Target guard expression verification: checkA() && checkB() maps across inverted guards
		var oldBin, newBin *treesitter.ASTNode
		var srcDoAction, dstDoAction *treesitter.ASTNode
		var srcInit, dstInit, srcCleanup, dstCleanup *treesitter.ASTNode

		for _, n := range srcAST.Descendants() {
			if r.IsExpression(n.Type) && n.Type == "binary_expression" {
				oldBin = n
			}
			if r.IsCall(n.Type) {
				switch extractCalleeLabel(n, r) {
				case "doAction":
					srcDoAction = n
				case "initTask":
					srcInit = n
				case "cleanupTask":
					srcCleanup = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsExpression(n.Type) && n.Type == "binary_expression" {
				newBin = n
			}
			if r.IsCall(n.Type) {
				switch extractCalleeLabel(n, r) {
				case "doAction":
					dstDoAction = n
				case "initTask":
					dstInit = n
				case "cleanupTask":
					dstCleanup = n
				}
			}
		}

		if oldBin == nil || newBin == nil {
			t.Fatalf("failed to locate binary expression nodes: old=%v new=%v", oldBin, newBin)
		}
		if mr.Mappings.Src()[oldBin] != newBin {
			t.Errorf("expected checkA() && checkB() to match across inverted guards, got %v", mr.Mappings.Src()[oldBin])
		}

		// 2. Enclosing function declaration maps in place
		var srcFunc, dstFunc *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				srcFunc = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) {
				dstFunc = n
				break
			}
		}
		if srcFunc == nil || dstFunc == nil {
			t.Fatalf("missing function declaration: src=%v dst=%v", srcFunc, dstFunc)
		}
		if mr.Mappings.Src()[srcFunc] != dstFunc {
			t.Errorf("expected func f to map in place, got %v", mr.Mappings.Src()[srcFunc])
		}

		// 3. Surrounding statements and body call map in place
		if matched := mr.Mappings.Src()[srcInit]; matched != dstInit {
			t.Errorf("expected initTask to map in place, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcCleanup]; matched != dstCleanup {
			t.Errorf("expected cleanupTask to map in place, got %v", matched)
		}
		if matched := mr.Mappings.Src()[srcDoAction]; matched != dstDoAction {
			t.Errorf("expected doAction to map in place, got %v", matched)
		}
	})
}

func TestBottomUp_SplitStatementDescendantsRejected(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		runTest := func(t *testing.T, oldSrc, newSrc string) {
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

			r := rulesFor(srcAST)
			if r == nil {
				t.Fatal("rulesFor returned nil")
			}

			// 1. Target binary expression rejection: compound expression MUST NOT map to dstBin
			var srcBin, dstBin *treesitter.ASTNode
			var srcCondA, srcCondB, dstCondA, dstCondB *treesitter.ASTNode
			var srcSetup, srcFinalize, dstSetup, dstFinalize *treesitter.ASTNode

			for _, n := range srcAST.Descendants() {
				if r.IsExpression(n.Type) && n.Type == "binary_expression" {
					srcBin = n
				}
				if r.IsCall(n.Type) {
					switch extractCalleeLabel(n, r) {
					case "condA":
						srcCondA = n
					case "condB":
						srcCondB = n
					case "setupContext":
						srcSetup = n
					case "finalizeContext":
						srcFinalize = n
					}
				}
			}

			for _, n := range dstAST.Descendants() {
				if r.IsExpression(n.Type) && n.Type == "binary_expression" {
					dstBin = n
				}
				if r.IsCall(n.Type) {
					switch extractCalleeLabel(n, r) {
					case "condA":
						dstCondA = n
					case "condB":
						dstCondB = n
					case "setupContext":
						dstSetup = n
					case "finalizeContext":
						dstFinalize = n
					}
				}
			}

			if srcBin == nil || dstBin == nil {
				t.Fatalf("missing binary expressions: srcBin=%v dstBin=%v", srcBin, dstBin)
			}
			if srcCondA == nil || srcCondB == nil || dstCondA == nil || dstCondB == nil {
				t.Fatalf("missing cond calls: srcA=%v srcB=%v dstA=%v dstB=%v", srcCondA, srcCondB, dstCondA, dstCondB)
			}
			if srcSetup == nil || srcFinalize == nil || dstSetup == nil || dstFinalize == nil {
				t.Fatalf("missing surrounding calls: srcSetup=%v dstSetup=%v", srcSetup, dstSetup)
			}

			// Split compound expression must remain unmapped
			if mapped := mr.Mappings.Src()[srcBin]; mapped != nil {
				t.Errorf("expected split compound expression srcBin NOT to map to dstBin, got %v", mapped)
			}

			// Stationary / moved sub-expressions
			if matched := mr.Mappings.Src()[srcCondA]; matched != dstCondA {
				t.Errorf("expected condA to match dstCondA, got %v", matched)
			}
			if matched := mr.Mappings.Src()[srcCondB]; matched != dstCondB {
				t.Errorf("expected condB to match dstCondB, got %v", matched)
			}

			// 2. Enclosing function maps in place
			var srcFunc, dstFunc *treesitter.ASTNode
			for _, n := range srcAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					srcFunc = n
					break
				}
			}
			for _, n := range dstAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					dstFunc = n
					break
				}
			}
			if srcFunc == nil || dstFunc == nil {
				t.Fatalf("missing function declaration: src=%v dst=%v", srcFunc, dstFunc)
			}
			if mr.Mappings.Src()[srcFunc] != dstFunc {
				t.Errorf("expected func f to map in place, got %v", mr.Mappings.Src()[srcFunc])
			}

			// 3. Surrounding sequence: setupContext and finalizeContext map in place
			if matched := mr.Mappings.Src()[srcSetup]; matched != dstSetup {
				t.Errorf("expected setupContext to map in place, got %v", matched)
			}
			if matched := mr.Mappings.Src()[srcFinalize]; matched != dstFinalize {
				t.Errorf("expected finalizeContext to map in place, got %v", matched)
			}
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
			runTest(t, oldSrc, newSrc)
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
			runTest(t, oldSrc, newSrc)
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
