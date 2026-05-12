package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

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

func NewMapping() *Mapping {
	return &Mapping{
		src: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
		dst: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
	}
}

func (m *Mapping) Add(t1, t2 *treesitter.ASTNode) {
	if _, exists := m.src[t1]; !exists {
		m.Pairs = append(m.Pairs, MappingPair{Src: t1, Dst: t2})
	}
	m.src[t1] = t2
	m.dst[t2] = t1
}

func (m *Mapping) Has(t1 *treesitter.ASTNode) bool {
	_, ok := m.src[t1]
	return ok
}

func (m *Mapping) HasDst(t2 *treesitter.ASTNode) bool {
	_, ok := m.dst[t2]
	return ok
}

func (m *Mapping) Remove(t1 *treesitter.ASTNode) {
	if t2, ok := m.src[t1]; ok {
		delete(m.dst, t2)
	}
	delete(m.src, t1)
}

// Src exposes the T1->T2 map for use in Dice calculations.
func (m *Mapping) Src() map[*treesitter.ASTNode]*treesitter.ASTNode { return m.src }

// addIsomorphicPairs add all the pairs of isomorphic descendants of t1/t2 to m.
func addIsomorphicPairs(t1, t2 *treesitter.ASTNode, m *Mapping) {
	d1 := append([]*treesitter.ASTNode{t1}, Descendants(t1)...)
	d2 := append([]*treesitter.ASTNode{t2}, Descendants(t2)...)
	for i, a := range d1 {
		b := d2[i] // subtrees are isomorphic so the structure is the same
		m.Add(a, b)
	}
}
