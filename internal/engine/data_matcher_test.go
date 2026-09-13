package engine_test

import (
	"fmt"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestMatchDataContainersBasic(t *testing.T) {
	k1 := testutil.Leaf("string", "\"name\"")
	v1 := testutil.Leaf("string", "\"diffmantic\"")
	p1 := testutil.Node("pair", "", k1, v1)
	obj1 := testutil.Node("object", "", p1)
	srcRoot := testutil.Node("document", "", obj1)
	srcRoot.Language = "json"

	k2 := testutil.Leaf("string", "\"name\"")
	v2 := testutil.Leaf("string", "\"diffmantic\"")
	p2 := testutil.Node("pair", "", k2, v2)
	obj2 := testutil.Node("object", "", p2)
	dstRoot := testutil.Node("document", "", obj2)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(p1) || m.Src()[p1] != p2 {
		t.Errorf("pair p1 should be mapped to p2, got %v", m.Src()[p1])
	}
	if !m.Has(k1) || m.Src()[k1] != k2 {
		t.Errorf("key k1 should be mapped to k2, got %v", m.Src()[k1])
	}
	if !m.Has(v1) || m.Src()[v1] != v2 {
		t.Errorf("val v1 should be mapped to v2, got %v", m.Src()[v1])
	}
}

func TestMatchDataContainersDivergentValue(t *testing.T) {
	k1 := testutil.Leaf("string", "\"version\"")
	v1 := testutil.Leaf("string", "\"1.0.0\"")
	p1 := testutil.Node("pair", "", k1, v1)
	obj1 := testutil.Node("object", "", p1)
	srcRoot := testutil.Node("document", "", obj1)
	srcRoot.Language = "json"

	k2 := testutil.Leaf("string", "\"version\"")
	v2 := testutil.Leaf("string", "\"2.0.0\"")
	p2 := testutil.Node("pair", "", k2, v2)
	obj2 := testutil.Node("object", "", p2)
	dstRoot := testutil.Node("document", "", obj2)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(p1) || m.Src()[p1] != p2 {
		t.Errorf("pair p1 should be mapped to p2, got %v", m.Src()[p1])
	}
	if !m.Has(k1) || m.Src()[k1] != k2 {
		t.Errorf("key k1 should be mapped to k2, got %v", m.Src()[k1])
	}
	if !m.Has(v1) || m.Src()[v1] != v2 {
		t.Errorf("divergent value v1 should be mapped to v2 for update, got %v", m.Src()[v1])
	}
}

func TestMatchDataContainersKeyRenameNonTrivialValue(t *testing.T) {
	// Renaming "user_id" -> "account_id" with identical value "usr_abc123456789".
	k1 := testutil.Leaf("string", "\"user_id\"")
	v1 := testutil.Leaf("string", "\"usr_abc123456789\"")
	p1 := testutil.Node("pair", "", k1, v1)

	kOther1 := testutil.Leaf("string", "\"status\"")
	vOther1 := testutil.Leaf("string", "\"active\"")
	pOther1 := testutil.Node("pair", "", kOther1, vOther1)

	obj1 := testutil.Node("object", "", p1, pOther1)
	srcRoot := testutil.Node("document", "", obj1)
	srcRoot.Language = "json"

	k2 := testutil.Leaf("string", "\"account_id\"")
	v2 := testutil.Leaf("string", "\"usr_abc123456789\"")
	p2 := testutil.Node("pair", "", k2, v2)

	kOther2 := testutil.Leaf("string", "\"status\"")
	vOther2 := testutil.Leaf("string", "\"active\"")
	pOther2 := testutil.Node("pair", "", kOther2, vOther2)

	obj2 := testutil.Node("object", "", p2, pOther2)
	dstRoot := testutil.Node("document", "", obj2)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(p1) || m.Src()[p1] != p2 {
		t.Errorf("renamed pair p1 should be mapped to p2 via value inversion, got %v", m.Src()[p1])
	}
	if !m.Has(k1) || m.Src()[k1] != k2 {
		t.Errorf("renamed key k1 should be mapped to k2 for update, got %v", m.Src()[k1])
	}
	if !m.Has(v1) || m.Src()[v1] != v2 {
		t.Errorf("identical value v1 should be mapped to v2, got %v", m.Src()[v1])
	}
}

func TestMatchDataContainersTrivialValueFilter(t *testing.T) {
	// Two flags changing keys with trivial boolean values (true / false).
	// Common literals shouldn't trigger key-rename matching when values collide.
	k1 := testutil.Leaf("string", "\"flagA\"")
	v1 := testutil.Leaf("true", "true")
	p1 := testutil.Node("pair", "", k1, v1)

	k2 := testutil.Leaf("string", "\"flagB\"")
	v2 := testutil.Leaf("true", "true")
	p2 := testutil.Node("pair", "", k2, v2)

	obj1 := testutil.Node("object", "", p1, p2)
	srcRoot := testutil.Node("document", "", obj1)
	srcRoot.Language = "json"

	k3 := testutil.Leaf("string", "\"flagC\"")
	v3 := testutil.Leaf("true", "true")
	p3 := testutil.Node("pair", "", k3, v3)

	k4 := testutil.Leaf("string", "\"flagD\"")
	v4 := testutil.Leaf("true", "true")
	p4 := testutil.Node("pair", "", k4, v4)

	obj2 := testutil.Node("object", "", p3, p4)
	dstRoot := testutil.Node("document", "", obj2)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	// Both pairs share "true", so neither has a unique value in this container.
	engine.MatchDataContainers(srcRoot, dstRoot, m, r)
}

func TestMatchDataContainersNestedRecursion(t *testing.T) {
	innerK1 := testutil.Leaf("string", "\"timeout\"")
	innerV1 := testutil.Leaf("number", "30")
	innerP1 := testutil.Node("pair", "", innerK1, innerV1)
	innerObj1 := testutil.Node("object", "", innerP1)

	outerK1 := testutil.Leaf("string", "\"server\"")
	outerP1 := testutil.Node("pair", "", outerK1, innerObj1)
	srcRoot := testutil.Node("document", "", testutil.Node("object", "", outerP1))
	srcRoot.Language = "json"

	innerK2 := testutil.Leaf("string", "\"timeout\"")
	innerV2 := testutil.Leaf("number", "60")
	innerP2 := testutil.Node("pair", "", innerK2, innerV2)
	innerObj2 := testutil.Node("object", "", innerP2)

	outerK2 := testutil.Leaf("string", "\"server\"")
	outerP2 := testutil.Node("pair", "", outerK2, innerObj2)
	dstRoot := testutil.Node("document", "", testutil.Node("object", "", outerP2))
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(outerP1) || m.Src()[outerP1] != outerP2 {
		t.Errorf("outer pair should match")
	}
	if !m.Has(innerP1) || m.Src()[innerP1] != innerP2 {
		t.Errorf("inner pair should match recursively")
	}
	if !m.Has(innerV1) || m.Src()[innerV1] != innerV2 {
		t.Errorf("inner value should match for update")
	}
}

func TestMatchDataContainersLargeDictionary(t *testing.T) {
	const numEntries = 1000

	var srcPairs []*treesitter.ASTNode
	var dstPairs []*treesitter.ASTNode

	for i := range numEntries {
		k1 := testutil.Leaf("string", fmt.Sprintf("\"key_%d\"", i))
		v1 := testutil.Leaf("string", fmt.Sprintf("\"value_%d\"", i))
		srcPairs = append(srcPairs, testutil.Node("pair", "", k1, v1))

		k2 := testutil.Leaf("string", fmt.Sprintf("\"key_%d\"", i))
		// Tweak some values to test updates.
		valStr := fmt.Sprintf("\"value_%d\"", i)
		if i%10 == 0 {
			valStr = fmt.Sprintf("\"value_modified_%d\"", i)
		}
		v2 := testutil.Leaf("string", valStr)
		dstPairs = append(dstPairs, testutil.Node("pair", "", k2, v2))
	}

	srcObj := testutil.Node("object", "", srcPairs...)
	dstObj := testutil.Node("object", "", dstPairs...)
	srcRoot := testutil.Node("document", "", srcObj)
	dstRoot := testutil.Node("document", "", dstObj)
	srcRoot.Language = "json"
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	for i := range numEntries {
		p1 := srcPairs[i]
		p2 := dstPairs[i]
		if !m.Has(p1) || m.Src()[p1] != p2 {
			t.Fatalf("entry %d: expected pair match", i)
		}
	}
}

func TestMatchDataContainersPermutedArray(t *testing.T) {
	// Reordered array [{"id": "1"}, {"id": "2"}] -> [{"id": "2"}, {"id": "1"}]
	k1 := testutil.Leaf("string", "\"id\"")
	v1 := testutil.Leaf("string", "\"1\"")
	p1 := testutil.Node("pair", "", k1, v1)
	obj1 := testutil.Node("object", "", p1)

	k2 := testutil.Leaf("string", "\"id\"")
	v2 := testutil.Leaf("string", "\"2\"")
	p2 := testutil.Node("pair", "", k2, v2)
	obj2 := testutil.Node("object", "", p2)

	srcArray := testutil.Node("array", "", obj1, obj2)
	srcPair := testutil.Node("pair", "", testutil.Leaf("string", "\"items\""), srcArray)
	srcRoot := testutil.Node("document", "", testutil.Node("object", "", srcPair))
	srcRoot.Language = "json"

	k1Dst := testutil.Leaf("string", "\"id\"")
	v1Dst := testutil.Leaf("string", "\"1\"")
	p1Dst := testutil.Node("pair", "", k1Dst, v1Dst)
	obj1Dst := testutil.Node("object", "", p1Dst)

	k2Dst := testutil.Leaf("string", "\"id\"")
	v2Dst := testutil.Leaf("string", "\"2\"")
	p2Dst := testutil.Node("pair", "", k2Dst, v2Dst)
	obj2Dst := testutil.Node("object", "", p2Dst)

	// Note dstArray has [obj2Dst, obj1Dst]
	dstArray := testutil.Node("array", "", obj2Dst, obj1Dst)
	dstPair := testutil.Node("pair", "", testutil.Leaf("string", "\"items\""), dstArray)
	dstRoot := testutil.Node("document", "", testutil.Node("object", "", dstPair))
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	// Verify obj1 maps to obj1Dst (isomorphic counterpart) despite permutation
	if !m.Has(obj1) || m.Src()[obj1] != obj1Dst {
		t.Errorf("obj1 (id: 1) should be mapped to obj1Dst, got %v", m.Src()[obj1])
	}
	// Verify obj2 maps to obj2Dst
	if !m.Has(obj2) || m.Src()[obj2] != obj2Dst {
		t.Errorf("obj2 (id: 2) should be mapped to obj2Dst, got %v", m.Src()[obj2])
	}
}

func TestMatchDataContainersHashCollisionSafety(t *testing.T) {
	// Simulates a 64-bit hash collision: v1 (leaf) and v2 (object with children) are
	// structurally different but share a forced hash. When child counts differ,
	// addIsomorphicPairs must exit early rather than panicking on index out of range.
	k1 := testutil.Leaf("string", "\"old_key\"")
	v1 := testutil.Leaf("string", "\"collision_val\"")
	p1 := testutil.Node("pair", "", k1, v1)
	srcRoot := testutil.Node("document", "", testutil.Node("object", "", p1))
	srcRoot.Language = "json"

	k2 := testutil.Leaf("string", "\"new_key\"")
	// v2 has a different AST structure (an object with one child) — distinct child count.
	innerChild := testutil.Leaf("string", "\"inner\"")
	v2 := testutil.Node("object", "", innerChild)
	p2 := testutil.Node("pair", "", k2, v2)
	dstRoot := testutil.Node("document", "", testutil.Node("object", "", p2))
	dstRoot.Language = "json"

	// Force hash collision on structurally different values.
	const forcedHash = 0xdeadbeefcafebabe
	v1.Hash = forcedHash
	v2.Hash = forcedHash

	m := engine.NewMapping()
	r := rules.Get("json")

	// Must not panic on mismatched child counts.
	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	// Colliding hashes must not cause innerChild to be mapped spuriously.
	if m.HasDst(innerChild) {
		t.Errorf("innerChild (v2's child) must not be spuriously mapped via hash-collision addIsomorphicPairs")
	}
}

func TestMatchDataContainersDeepNestingCeiling(t *testing.T) {
	const depth = 200

	var buildDeepTree func(currentDepth int) *treesitter.ASTNode
	buildDeepTree = func(currentDepth int) *treesitter.ASTNode {
		if currentDepth >= depth {
			return testutil.Leaf("string", "\"deep_val\"")
		}
		k := testutil.Leaf("string", fmt.Sprintf("\"k_%d\"", currentDepth))
		v := buildDeepTree(currentDepth + 1)
		p := testutil.Node("pair", "", k, v)
		return testutil.Node("object", "", p)
	}

	srcRoot := testutil.Node("document", "", buildDeepTree(0))
	dstRoot := testutil.Node("document", "", buildDeepTree(0))
	srcRoot.Language = "json"
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	// Deep trees should stop at the depth ceiling instead of overflowing the stack.
	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(srcRoot) || m.Src()[srcRoot] != dstRoot {
		t.Errorf("expected root to be matched")
	}
}

func TestMatchDataContainersTopLevelArray(t *testing.T) {
	k1 := testutil.Leaf("string", "\"id\"")
	v1 := testutil.Leaf("string", "\"1\"")
	p1 := testutil.Node("pair", "", k1, v1)
	obj1 := testutil.Node("object", "", p1)

	k2 := testutil.Leaf("string", "\"id\"")
	v2 := testutil.Leaf("string", "\"2\"")
	p2 := testutil.Node("pair", "", k2, v2)
	obj2 := testutil.Node("object", "", p2)

	srcArray := testutil.Node("array", "", obj1, obj2)
	srcRoot := testutil.Node("document", "", srcArray)
	srcRoot.Language = "json"

	k1Dst := testutil.Leaf("string", "\"id\"")
	v1Dst := testutil.Leaf("string", "\"1\"")
	p1Dst := testutil.Node("pair", "", k1Dst, v1Dst)
	obj1Dst := testutil.Node("object", "", p1Dst)

	k2Dst := testutil.Leaf("string", "\"id\"")
	v2Dst := testutil.Leaf("string", "\"2\"")
	p2Dst := testutil.Node("pair", "", k2Dst, v2Dst)
	obj2Dst := testutil.Node("object", "", p2Dst)

	dstArray := testutil.Node("array", "", obj1Dst, obj2Dst)
	dstRoot := testutil.Node("document", "", dstArray)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(obj1) || m.Src()[obj1] != obj1Dst {
		t.Errorf("expected obj1 to match obj1Dst in top-level array, got %v", m.Src()[obj1])
	}
	if !m.Has(obj2) || m.Src()[obj2] != obj2Dst {
		t.Errorf("expected obj2 to match obj2Dst in top-level array, got %v", m.Src()[obj2])
	}
	if !m.Has(p1) || m.Src()[p1] != p1Dst {
		t.Errorf("expected p1 to match p1Dst in top-level array, got %v", m.Src()[p1])
	}
	if !m.Has(p2) || m.Src()[p2] != p2Dst {
		t.Errorf("expected p2 to match p2Dst in top-level array, got %v", m.Src()[p2])
	}
}

func TestMatchDataContainersDuplicateKeys(t *testing.T) {
	k1A := testutil.Leaf("string", "\"item\"")
	v1A := testutil.Leaf("string", "\"first\"")
	p1A := testutil.Node("pair", "", k1A, v1A)

	k1B := testutil.Leaf("string", "\"item\"")
	v1B := testutil.Leaf("string", "\"second\"")
	p1B := testutil.Node("pair", "", k1B, v1B)

	obj1 := testutil.Node("object", "", p1A, p1B)
	srcRoot := testutil.Node("document", "", obj1)
	srcRoot.Language = "json"

	k2A := testutil.Leaf("string", "\"item\"")
	v2A := testutil.Leaf("string", "\"first\"")
	p2A := testutil.Node("pair", "", k2A, v2A)

	k2B := testutil.Leaf("string", "\"item\"")
	v2B := testutil.Leaf("string", "\"second\"")
	p2B := testutil.Node("pair", "", k2B, v2B)

	obj2 := testutil.Node("object", "", p2A, p2B)
	dstRoot := testutil.Node("document", "", obj2)
	dstRoot.Language = "json"

	m := engine.NewMapping()
	r := rules.Get("json")

	engine.MatchDataContainers(srcRoot, dstRoot, m, r)

	if !m.Has(p1A) || m.Src()[p1A] != p2A {
		t.Errorf("expected p1A to match p2A, got %v", m.Src()[p1A])
	}
	if !m.Has(p1B) || m.Src()[p1B] != p2B {
		t.Errorf("expected p1B to match p2B, got %v", m.Src()[p1B])
	}
}
