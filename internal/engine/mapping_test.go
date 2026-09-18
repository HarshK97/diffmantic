package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
)

func TestMappingAddAndLookup(t *testing.T) {
	m := NewMapping()
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")

	m.Add(a, b)

	if !m.Has(a) {
		t.Error("Has(a) should be true after Add")
	}
	if !m.HasDst(b) {
		t.Error("HasDst(b) should be true after Add")
	}
	if m.Src()[a] != b {
		t.Error("Src()[a] should be b")
	}
	if m.Dst()[b] != a {
		t.Error("Dst()[b] should be a")
	}
}

func TestMappingPairsOrder(t *testing.T) {
	m := NewMapping()
	a1 := testutil.Leaf("id", "x")
	b1 := testutil.Leaf("id", "x")
	a2 := testutil.Leaf("id", "y")
	b2 := testutil.Leaf("id", "y")

	m.Add(a1, b1)
	m.Add(a2, b2)

	if len(m.Pairs) != 2 {
		t.Fatalf("want 2 pairs, got %d", len(m.Pairs))
	}
	if m.Pairs[0].Src != a1 || m.Pairs[1].Src != a2 {
		t.Error("pairs should preserve insertion order")
	}
}

func TestMappingDuplicateAdd(t *testing.T) {
	// Re-adding the same source doesn't duplicate the pair and updates m.Pairs and m.dst.
	m := NewMapping()
	a := testutil.Leaf("id", "x")
	b1 := testutil.Leaf("id", "y")
	b2 := testutil.Leaf("id", "z")

	m.Add(a, b1)
	m.Add(a, b2)

	if len(m.Pairs) != 1 {
		t.Errorf("duplicate Add should not create new pair, got %d pairs", len(m.Pairs))
	}
	if m.Src()[a] != b2 {
		t.Error("second Add should overwrite the mapping value in src")
	}
	if m.Pairs[0].Dst != b2 {
		t.Errorf("second Add should update Dst in m.Pairs, got %v", m.Pairs[0].Dst)
	}
	if m.HasDst(b1) {
		t.Error("b1 should no longer be in m.dst after remapping a to b2")
	}
	if m.Dst()[b2] != a {
		t.Error("b2 should map to a in m.dst")
	}
}

func TestMappingRemapDestination(t *testing.T) {
	// Adding a pair where destination was previously mapped to a different source.
	m := NewMapping()
	a1 := testutil.Leaf("id", "x1")
	a2 := testutil.Leaf("id", "x2")
	b := testutil.Leaf("id", "y")

	m.Add(a1, b)
	m.Add(a2, b)

	if len(m.Pairs) != 1 {
		t.Fatalf("expected 1 pair after remapping destination, got %d", len(m.Pairs))
	}
	if m.Pairs[0].Src != a2 || m.Pairs[0].Dst != b {
		t.Errorf("expected pair (a2, b), got (%v, %v)", m.Pairs[0].Src, m.Pairs[0].Dst)
	}
	if m.Has(a1) {
		t.Error("a1 should no longer be in m.src after b remapped to a2")
	}
	if m.Src()[a2] != b {
		t.Error("a2 should map to b in m.src")
	}
	if m.Dst()[b] != a2 {
		t.Error("b should map to a2 in m.dst")
	}
}

func TestMappingRemove(t *testing.T) {
	m := NewMapping()
	a := testutil.Leaf("id", "x")
	b := testutil.Leaf("id", "y")

	m.Add(a, b)
	m.Remove(a)

	if m.Has(a) {
		t.Error("Has(a) should be false after Remove")
	}
	if m.HasDst(b) {
		t.Error("HasDst(b) should be false after Remove")
	}
}

func TestMappingRemoveNonexistent(_ *testing.T) {
	// Removing missing keys won't panic.
	m := NewMapping()
	m.Remove(testutil.Leaf("id", "x"))
}

func TestMappingClone(t *testing.T) {
	var nilMapping *Mapping
	if nilMapping.Clone() != nil {
		t.Error("nil Mapping.Clone() should be nil")
	}

	m := NewMapping()
	cpEmpty := m.Clone()
	if cpEmpty == nil || len(cpEmpty.Pairs) != 0 || len(cpEmpty.src) != 0 || len(cpEmpty.dst) != 0 {
		t.Error("cloning empty mapping should yield valid empty mapping")
	}

	a1 := testutil.Leaf("id", "a1")
	b1 := testutil.Leaf("id", "b1")
	a2 := testutil.Leaf("id", "a2")
	b2 := testutil.Leaf("id", "b2")

	m.Add(a1, b1)
	m.Add(a2, b2)

	cp := m.Clone()
	if cp == m {
		t.Fatal("clone returned same pointer")
	}
	if len(cp.Pairs) != 2 {
		t.Fatalf("expected 2 pairs in clone, got %d", len(cp.Pairs))
	}
	if !cp.Has(a1) || !cp.HasDst(b1) || cp.Src()[a1] != b1 || cp.Dst()[b1] != a1 {
		t.Error("clone missing pair 1 mappings")
	}
	if !cp.Has(a2) || !cp.HasDst(b2) || cp.Src()[a2] != b2 || cp.Dst()[b2] != a2 {
		t.Error("clone missing pair 2 mappings")
	}

	// Mutating the clone should not mutate the original.
	a3 := testutil.Leaf("id", "a3")
	b3 := testutil.Leaf("id", "b3")
	cp.Add(a3, b3)
	cp.Remove(a1)

	if len(m.Pairs) != 2 || !m.Has(a1) || m.Has(a3) {
		t.Error("mutating clone modified original mapping")
	}
	if len(cp.Pairs) != 2 || cp.Has(a1) || !cp.Has(a3) {
		t.Error("clone state unexpected after mutation")
	}
}

func TestMappingDiceSrc(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	rootA := testutil.Node("call", "", a1)
	b1 := testutil.Leaf("id", "x")
	rootB := testutil.Node("call", "", b1)

	m := NewMapping()
	m.Add(a1, b1)

	d := m.DiceSrc(rootA, rootB)
	if d != 1.0 {
		t.Errorf("DiceSrc = %f, want 1.0", d)
	}
}

func TestAddIsomorphicPairs(t *testing.T) {
	a1 := testutil.Leaf("id", "x")
	rootA := testutil.Node("call", "", a1)
	b1 := testutil.Leaf("id", "x")
	rootB := testutil.Node("call", "", b1)

	m := NewMapping()
	addIsomorphicPairs(rootA, rootB, m)

	if !m.Has(rootA) || !m.Has(a1) {
		t.Error("addIsomorphicPairs should map all nodes")
	}
	if m.Src()[rootA] != rootB || m.Src()[a1] != b1 {
		t.Error("addIsomorphicPairs should map nodes by position")
	}
}
