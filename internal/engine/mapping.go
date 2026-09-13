package engine

import (
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// MappingPair is a single src→dst node mapping, kept for deterministic ordering.
type MappingPair struct {
	Src *treesitter.ASTNode
	Dst *treesitter.ASTNode
}

// Mapping is a 1-1 node mapping between T1 nodes and T2 nodes.
// Pairs preserves insertion order for deterministic iteration.
type Mapping struct {
	src   map[*treesitter.ASTNode]*treesitter.ASTNode
	dst   map[*treesitter.ASTNode]*treesitter.ASTNode
	Pairs []MappingPair
}

// NewMapping allocates an empty 1:1 AST node mapping.
func NewMapping() *Mapping {
	return &Mapping{
		src: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
		dst: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
	}
}

// Add registers the t1→t2 pair, enforcing strict 1:1 bijection. If t2 was already
// claimed by a different t1, that old mapping is evicted. If t1 was already mapped to a
// different t2, that old destination is replaced in-place to preserve insertion order.
func (m *Mapping) Add(t1, t2 *treesitter.ASTNode) {
	if t1 == nil || t2 == nil {
		return
	}
	if curT2, ok := m.src[t1]; ok && curT2 == t2 {
		return
	}

	// Evict any prior mapping claiming t2 to preserve strict 1:1 bijection.
	if oldT1, ok := m.dst[t2]; ok && oldT1 != t1 {
		delete(m.src, oldT1)
		m.Pairs = slices.DeleteFunc(m.Pairs, func(p MappingPair) bool {
			return p.Src == oldT1
		})
	}

	// If t1 was already paired, replace its destination in-place to preserve order.
	if oldT2, ok := m.src[t1]; ok {
		delete(m.dst, oldT2)
		for i := range m.Pairs {
			if m.Pairs[i].Src == t1 {
				m.Pairs[i].Dst = t2
				m.src[t1] = t2
				m.dst[t2] = t1
				return
			}
		}
	}

	m.Pairs = append(m.Pairs, MappingPair{Src: t1, Dst: t2})
	m.src[t1] = t2
	m.dst[t2] = t1
}

// Has reports whether t1 has been mapped to a destination node.
func (m *Mapping) Has(t1 *treesitter.ASTNode) bool {
	_, ok := m.src[t1]
	return ok
}

// HasDst reports whether t2 has been claimed as a destination node.
func (m *Mapping) HasDst(t2 *treesitter.ASTNode) bool {
	_, ok := m.dst[t2]
	return ok
}

// Remove clears the mapping for t1 and any destination node it was paired with.
func (m *Mapping) Remove(t1 *treesitter.ASTNode) {
	if t2, ok := m.src[t1]; ok {
		delete(m.dst, t2)
	}
	delete(m.src, t1)
	m.Pairs = slices.DeleteFunc(m.Pairs, func(p MappingPair) bool {
		return p.Src == t1
	})
}

// Src exposes the T1->T2 map for use in Dice calculations.
func (m *Mapping) Src() map[*treesitter.ASTNode]*treesitter.ASTNode { return m.src }

// Dst exposes the T2->T1 map.
func (m *Mapping) Dst() map[*treesitter.ASTNode]*treesitter.ASTNode { return m.dst }

// DiceSrc computes dice(t1, t2) using the internal src mapping.
func (m *Mapping) DiceSrc(t1, t2 *treesitter.ASTNode) float64 {
	return Dice(t1, t2, m.src)
}

// addIsomorphicPairs adds all pairs of isomorphic descendants of t1/t2 to m recursively.
// Guards against child-count mismatches if a hash collision pairs different subtrees.
func addIsomorphicPairs(t1, t2 *treesitter.ASTNode, m *Mapping) {
	if len(t1.Children) != len(t2.Children) {
		return
	}
	m.Add(t1, t2)
	for i, c1 := range t1.Children {
		addIsomorphicPairs(c1, t2.Children[i], m)
	}
}
