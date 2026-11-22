package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func parseTestNodes(t *testing.T) map[string]*treesitter.ASTNode {
	t.Helper()
	tree, err := treesitter.Parse([]byte("package p\nfunc f() { _ = a1 + b1 + a2 + b2 + a3 + b3 + a4 + b4 + x + y + z + x1 + x2 + unmapped }\n"), "test.go")
	if err != nil {
		t.Fatalf("parseTestNodes: %v", err)
	}
	nodes := make(map[string]*treesitter.ASTNode)
	var walk func(n *treesitter.ASTNode)
	walk = func(n *treesitter.ASTNode) {
		if n.Type == "identifier" {
			nodes[n.Label] = n
		}
		for _, c := range n.Children {
			walk(c)
		}
	}
	walk(tree)
	return nodes
}

func TestMappingAddAndLookup(t *testing.T) {
	nodes := parseTestNodes(t)
	m := NewMapping()
	a := nodes["x"]
	b := nodes["y"]

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
	nodes := parseTestNodes(t)
	m := NewMapping()
	a1 := nodes["a1"]
	b1 := nodes["b1"]
	a2 := nodes["a2"]
	b2 := nodes["b2"]

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
	nodes := parseTestNodes(t)
	m := NewMapping()
	a := nodes["x"]
	b1 := nodes["y"]
	b2 := nodes["z"]

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
	nodes := parseTestNodes(t)
	m := NewMapping()
	a1 := nodes["x1"]
	a2 := nodes["x2"]
	b := nodes["y"]

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
	nodes := parseTestNodes(t)
	m := NewMapping()
	a := nodes["x"]
	b := nodes["y"]

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
	nodes := parseTestNodes(t)
	m := NewMapping()
	node := nodes["x"]

	// Removing from an empty mapping should not panic or add state.
	m.Remove(node)
	if m.Has(node) || len(m.Pairs) != 0 || len(m.Src()) != 0 || len(m.Dst()) != 0 {
		t.Error("expected mapping to remain empty after removing non-existent key")
	}

	// Removing an unmapped node from a populated mapping must leave existing pairs intact.
	existingSrc := nodes["a1"]
	existingDst := nodes["b1"]
	m.Add(existingSrc, existingDst)

	m.Remove(node)
	if !m.Has(existingSrc) || m.Get(existingSrc) != existingDst {
		t.Error("existing mapping corrupted after removing non-existent key")
	}
	if len(m.Pairs) != 1 {
		t.Errorf("expected 1 pair remaining, got %d", len(m.Pairs))
	}
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

	nodes := parseTestNodes(t)
	a1 := nodes["a1"]
	b1 := nodes["b1"]
	a2 := nodes["a2"]
	b2 := nodes["b2"]

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
	a3 := nodes["a3"]
	b3 := nodes["b3"]
	cp.Add(a3, b3)
	cp.Remove(a1)

	if len(m.Pairs) != 2 || !m.Has(a1) || m.Has(a3) {
		t.Error("mutating clone modified original mapping")
	}
	if len(cp.Pairs) != 2 || cp.Has(a1) || !cp.Has(a3) {
		t.Error("clone state unexpected after mutation")
	}
}

func TestMappingRemoveSet(t *testing.T) {
	nodes := parseTestNodes(t)
	var nilMapping *Mapping
	nilMapping.RemoveSet(map[*treesitter.ASTNode]struct{}{nodes["x"]: {}})

	m := NewMapping()
	m.RemoveSet(nil)
	m.RemoveSet(map[*treesitter.ASTNode]struct{}{})

	a1 := nodes["a1"]
	b1 := nodes["b1"]
	a2 := nodes["a2"]
	b2 := nodes["b2"]
	a3 := nodes["a3"]
	b3 := nodes["b3"]
	a4 := nodes["a4"]
	b4 := nodes["b4"]

	m.Add(a1, b1)
	m.Add(a2, b2)
	m.Add(a3, b3)
	m.Add(a4, b4)

	unmapped := nodes["unmapped"]
	toRemove := map[*treesitter.ASTNode]struct{}{
		a1:       {},
		a3:       {},
		unmapped: {},
	}

	m.RemoveSet(toRemove)

	if len(m.Pairs) != 2 {
		t.Fatalf("got len(m.Pairs) = %d, want 2", len(m.Pairs))
	}
	if m.Pairs[0].Src != a2 || m.Pairs[0].Dst != b2 {
		t.Errorf("got pair 0 (%v, %v), want (a2, b2)", m.Pairs[0].Src, m.Pairs[0].Dst)
	}
	if m.Pairs[1].Src != a4 || m.Pairs[1].Dst != b4 {
		t.Errorf("got pair 1 (%v, %v), want (a4, b4)", m.Pairs[1].Src, m.Pairs[1].Dst)
	}

	if m.Has(a1) || m.HasDst(b1) {
		t.Error("a1/b1 should be removed")
	}
	if m.Has(a3) || m.HasDst(b3) {
		t.Error("a3/b3 should be removed")
	}
	if !m.Has(a2) || !m.HasDst(b2) {
		t.Error("a2/b2 should be retained")
	}
	if !m.Has(a4) || !m.HasDst(b4) {
		t.Error("a4/b4 should be retained")
	}
}

func TestMappingDiceSrc(t *testing.T) {
	srcA := []byte("package p\nfunc f() { x() }\n")
	srcB := []byte("package p\nfunc f() { x() }\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	callA, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	callB, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	for i, cA := range callA.Children {
		m.Add(cA, callB.Children[i])
	}

	d := m.DiceSrc(callA, callB)
	if d != 1.0 {
		t.Errorf("DiceSrc = %f, want 1.0", d)
	}
}

func TestAddIsomorphicPairs(t *testing.T) {
	srcA := []byte("package p\nfunc f() { x() }\n")
	srcB := []byte("package p\nfunc f() { x() }\n")

	rootA, err := treesitter.Parse(srcA, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(srcB, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	callA, err := testutil.FindNode(rootA, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}
	callB, err := testutil.FindNode(rootB, testutil.NodeQuery{Type: "call_expression", Line: 2})
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	addIsomorphicPairs(callA, callB, m)

	if !m.Has(callA) {
		t.Error("addIsomorphicPairs should map root call")
	}
	for i, cA := range callA.Children {
		cB := callB.Children[i]
		if !m.Has(cA) || m.Src()[cA] != cB {
			t.Errorf("child %d should be mapped to corresponding child", i)
		}
	}
}
