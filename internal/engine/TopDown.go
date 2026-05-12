package engine

import (
	"math"
	"sort"

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

func NewMapping() *Mapping {
	return &Mapping{
		src: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
		dst: make(map[*treesitter.ASTNode]*treesitter.ASTNode),
	}
}

// PriorityList is a height-indexed max-priority list.
// Internally we store buckets keyed by height so PeekMax / Pop are O(1)
// and Push is O(log n) via a sorted key list.
type PriorityList struct {
	buckets map[int][]*treesitter.ASTNode
	heights []int
}

func NewPriorityList() *PriorityList {
	return &PriorityList{buckets: make(map[int][]*treesitter.ASTNode)}
}

// Height returns the height of a subtree rooted at n.
// A leaf has height 1.
func Height(n *treesitter.ASTNode) int {
	if n == nil {
		return 0
	}
	if len(n.Children) == 0 {
		return 1
	}
	max := 0
	for _, c := range n.Children {
		if h := Height(c); h > max {
			max = h
		}
	}
	return max + 1
}

// Push inserts node n (keyed by its height) into the list.
func Push(n *treesitter.ASTNode, l *PriorityList) {
	h := Height(n)
	if _, exists := l.buckets[h]; !exists {
		// insert height into sorted slice
		idx := sort.SearchInts(l.heights, h)
		l.heights = append(l.heights, 0)
		copy(l.heights[idx+1:], l.heights[idx:])
		l.heights[idx] = h
	}
	l.buckets[h] = append(l.buckets[h], n)
}

// PeekMax returns the maximum height currently in the list.
func PeekMax(l *PriorityList) int {
	if len(l.heights) == 0 {
		return math.MinInt32
	}
	return l.heights[len(l.heights)-1]
}

// Pop removes and returns all nodes whose height equals PeekMax.
func Pop(l *PriorityList) []*treesitter.ASTNode {
	if len(l.heights) == 0 {
		return nil
	}
	maxH := l.heights[len(l.heights)-1]
	l.heights = l.heights[:len(l.heights)-1]
	nodes := l.buckets[maxH]
	delete(l.buckets, maxH)
	return nodes
}

// Open expansd node t into its childrens and pushes each into list.
func Open(t *treesitter.ASTNode, l *PriorityList) {
	for _, c := range t.Children {
		Push(c, l)
	}
}

// Descendants returns all nodes in the subtree rooted at n,
// excluding n itself.
func Descendants(n *treesitter.ASTNode) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range n.Children {
		out = append(out, c)
		out = append(out, Descendants(c)...)
	}
	return out
}

// s(t) - the set of descendants of t used by the dice formula.
func descendantSet(n *treesitter.ASTNode) map[*treesitter.ASTNode]struct{} {
	s := make(map[*treesitter.ASTNode]struct{})
	for _, d := range Descendants(n) {
		s[d] = struct{}{}
	}
	return s
}

// Dice computes the Dice similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// dice(t1, t2, m) = 2 × |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / (|s(t1)| + |s(t2)|)
func Dice(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	s1 := Descendants(t1)
	s2 := descendantSet(t2)

	denom := float64(len(s1) + len(Descendants(t2)))
	if denom == 0 {
		return 0
	}

	common := 0
	for _, d := range s1 {
		if mapped, ok := m[d]; ok {
			if _, inS2 := s2[mapped]; inS2 {
				common++
			}
		}
	}
	return 2.0 * float64(common) / denom
}

// TODO: replace the O(n) algorithm with O(1) hash comparison from
// M. Chilowicz, E. Duris, and G. Roussel. Syntax tree
// fingerprinting for source code similarity detection.
//
// Isomorphic returns true when two subtrees are structurally
// and label-wise identical.
func Isomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	if a.Type != b.Type || a.Label != b.Label || len(a.Children) != len(b.Children) {
		return false
	}

	for i := range a.Children {
		if !Isomorphic(a.Children[i], b.Children[i]) {
			return false
		}
	}

	return true
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

// TopDown imploments the Gumtree Top-Down matching phase.
//
// It takes:
//
//	t1Root - root of source tree T1
//	t2Root - root of destination tree T2
//	minHeight - minimum subtree height (Will be removed after adding the Simple recovery function)
//
// Returns the set of mappings m and the candidate list A
func TopDown(
	t1Root, t2Root *treesitter.ASTNode,
	minHeight int,
) *Mapping {
	l1 := NewPriorityList()
	l2 := NewPriorityList()
	var A [][2]*treesitter.ASTNode // canditdate mappings
	m := NewMapping()              // final mapping

	Push(t1Root, l1)
	Push(t2Root, l2)

	for min(PeekMax(l1), PeekMax(l2)) >= minHeight {
		if PeekMax(l1) != PeekMax(l2) {
			if PeekMax(l1) > PeekMax(l2) {
				for _, t := range Pop(l1) {
					Open(t, l1)
				}
			} else {
				for _, t := range Pop(l2) {
					Open(t, l2)
				}
			}
		} else {
			H1 := Pop(l1)
			H2 := Pop(l2)

			for _, t1 := range H1 {
				for _, t2 := range H2 {
					if Isomorphic(t1, t2) {
						ambiguous := false

						for _, ta := range H2 {
							if ta != t2 && Isomorphic(t1, ta) {
								ambiguous = true
								break
							}
						}
						if !ambiguous {
							for _, ta := range H1 {
								if ta != t1 && Isomorphic(ta, t2) {
									ambiguous = true
									break
								}
							}
						}

						if ambiguous {
							A = append(A, [2]*treesitter.ASTNode{t1, t2})
						} else {
							addIsomorphicPairs(t1, t2, m)
						}
					}
				}
			}

			for _, t1 := range H1 {
				matched := false
				for _, t2 := range H2 {
					if Isomorphic(t1, t2) && m.Has(t1) {
						matched = true
						break
					}
				}

				if !matched {
					for _, pair := range A {
						if pair[0] == t1 {
							matched = true
							break
						}
					}
				}
				if !matched {
					Open(t1, l1)
				}
			}
			for _, t2 := range H2 {
				matched := false
				for _, t1 := range H1 {
					if Isomorphic(t1, t2) && m.HasDst(t2) {
						matched = true
						break
					}
				}
				if !matched {
					for _, pair := range A {
						if pair[1] == t2 {
							matched = true
							break
						}
					}
				}
				if !matched {
					Open(t2, l2)
				}
			}
		}
	}

	sort.SliceStable(A, func(i, j int) bool {
		di := Dice(A[i][0].Parent, A[i][1].Parent, m.Src())
		dj := Dice(A[j][0].Parent, A[j][1].Parent, m.Src())
		return di > dj
	})

	for len(A) > 0 {
		pair := A[0]
		A = A[1:]
		t1, t2 := pair[0], pair[1]

		addIsomorphicPairs(t1, t2, m)

		filtered := A[:0]
		for _, p := range A {
			if p[0] != t1 && p[1] != t2 {
				filtered = append(filtered, p)
			}
		}
		A = filtered
	}

	return m
}
