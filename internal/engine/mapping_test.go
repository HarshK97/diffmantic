package engine

import "testing"

func TestMappingAddAndLookup(t *testing.T) {
	m := NewMapping()
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")

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
	a1 := mkLeaf("id", "x")
	b1 := mkLeaf("id", "x")
	a2 := mkLeaf("id", "y")
	b2 := mkLeaf("id", "y")

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
	// Re-adding same src should not create duplicate pair.
	m := NewMapping()
	a := mkLeaf("id", "x")
	b1 := mkLeaf("id", "y")
	b2 := mkLeaf("id", "z")

	m.Add(a, b1)
	m.Add(a, b2)

	if len(m.Pairs) != 1 {
		t.Errorf("duplicate Add should not create new pair, got %d pairs", len(m.Pairs))
	}
	if m.Src()[a] != b2 {
		t.Error("second Add should overwrite the mapping value")
	}
}

func TestMappingRemove(t *testing.T) {
	m := NewMapping()
	a := mkLeaf("id", "x")
	b := mkLeaf("id", "y")

	m.Add(a, b)
	m.Remove(a)

	if m.Has(a) {
		t.Error("Has(a) should be false after Remove")
	}
	if m.HasDst(b) {
		t.Error("HasDst(b) should be false after Remove")
	}
}

func TestMappingRemoveNonexistent(t *testing.T) {
	// Removing a key that doesn't exist should not panic.
	m := NewMapping()
	m.Remove(mkLeaf("id", "x"))
}

func TestMappingDiceSrc(t *testing.T) {
	a1 := mkLeaf("id", "x")
	rootA := mkNode("call", "", a1)
	b1 := mkLeaf("id", "x")
	rootB := mkNode("call", "", b1)

	m := NewMapping()
	m.Add(a1, b1)

	d := m.DiceSrc(rootA, rootB)
	if d != 1.0 {
		t.Errorf("DiceSrc = %f, want 1.0", d)
	}
}

func TestAddIsomorphicPairs(t *testing.T) {
	a1 := mkLeaf("id", "x")
	rootA := mkNode("call", "", a1)
	b1 := mkLeaf("id", "x")
	rootB := mkNode("call", "", b1)

	m := NewMapping()
	addIsomorphicPairs(rootA, rootB, m)

	if !m.Has(rootA) || !m.Has(a1) {
		t.Error("addIsomorphicPairs should map all nodes")
	}
	if m.Src()[rootA] != rootB || m.Src()[a1] != b1 {
		t.Error("addIsomorphicPairs should map nodes by position")
	}
}
