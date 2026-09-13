package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestComputeAffinity_HardConstraints(t *testing.T) {
	leaf1 := testutil.Leaf("id", "x")
	leaf2 := testutil.Leaf("id", "x")
	t1 := testutil.Node("block", "", leaf1)
	c := testutil.Node("block", "", leaf2)

	m := NewMapping()

	// 1. Without common descendants, affinity should be -1.0
	scoreNoCommon := computeAffinity(t1, c, m, DefaultAffinityWeights)
	if scoreNoCommon != -1.0 {
		t.Errorf("expected -1.0 for no common descendants, got %f", scoreNoCommon)
	}

	m.Add(leaf1, leaf2)

	// 2. Already mapped destination candidate should return -1.0
	cMapped := testutil.Node("block", "")
	m.Add(t1, cMapped)
	scoreAlreadyMapped := computeAffinity(t1, cMapped, m, DefaultAffinityWeights)
	if scoreAlreadyMapped != -1.0 {
		t.Errorf("expected -1.0 for already mapped candidate, got %f", scoreAlreadyMapped)
	}

	// 3. Incompatible types should return -1.0
	m2 := NewMapping()
	m2.Add(leaf1, leaf2)
	cDiffType := testutil.Node("expression_statement", "", leaf2)
	scoreDiffType := computeAffinity(t1, cDiffType, m2, DefaultAffinityWeights)
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

	scoreInScope := computeAffinity(b1, b2, m, DefaultAffinityWeights)

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

	scoreCrossScope := computeAffinity(b1, b3, mCross, DefaultAffinityWeights)

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

	scoreSamePos := computeAffinity(item1, item2, m, DefaultAffinityWeights)

	// Shift item2 to index 1
	item2Shifted := testutil.Node("item", "", l2)
	_ = testutil.Node("container", "", testutil.Node("item", ""), item2Shifted)
	mShifted := NewMapping()
	mShifted.Add(l1, l2)

	scoreShifted := computeAffinity(item1, item2Shifted, mShifted, DefaultAffinityWeights)
	if scoreSamePos < scoreShifted {
		t.Errorf("expected same positional score (%f) >= shifted score (%f)", scoreSamePos, scoreShifted)
	}

	// Unordered parent container ignores positional differences
	parent1.IsUnordered = true
	parent2.IsUnordered = true
	scoreUnordered := computeAffinity(item1, item2, m, DefaultAffinityWeights)
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

	scoreSameKey := computeAffinity(pair1, pair2, m, DefaultAffinityWeights)

	mDiff := NewMapping()
	mDiff.Add(v1, v3)
	scoreDiffKey := computeAffinity(pair1, pair3, mDiff, DefaultAffinityWeights)

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

	score := computeAffinity(sub1, sub2, m, DefaultAffinityWeights)
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
	scoreSameDepth := computeAffinity(block1, block2, mSame, DefaultAffinityWeights)

	// depthPenalty = DepthCoeff × (2−1)² = 0.20 × 1 = 0.20 → cross-depth score is lower.
	mDeep := NewMapping()
	mDeep.Add(sharedLeaf1d, sharedLeaf2d)
	mDeep.Add(parent1Deep, parent2Shallow)
	scoreCrossDepth := computeAffinity(block1Deep, block2Shallow, mDeep, DefaultAffinityWeights)

	if scoreSameDepth <= scoreCrossDepth {
		t.Errorf("depth penalty did not fire: same-depth %f should exceed cross-depth %f", scoreSameDepth, scoreCrossDepth)
	}
}
