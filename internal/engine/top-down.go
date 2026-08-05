package engine

import (
	"slices"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// TopDown matches largest isomorphic subtrees top-down by height.
func TopDown(
	t1Root, t2Root *treesitter.ASTNode,
	minHeight int,
	m *Mapping,
	part *LinePartition,
) {
	l1 := newPriorityList()
	l2 := newPriorityList()
	var A [][2]*treesitter.ASTNode // canditdate mappings

	l1.Push(t1Root)
	l2.Push(t2Root)

	for min(l1.PeekMax(), l2.PeekMax()) >= minHeight {
		if l1.PeekMax() != l2.PeekMax() {
			if l1.PeekMax() > l2.PeekMax() {
				for _, t := range l1.Pop() {
					l1.Open(t)
				}
			} else {
				for _, t := range l2.Pop() {
					l2.Open(t)
				}
			}
		} else {
			H1 := l1.Pop()
			H2 := l2.Pop()

			for _, t1 := range H1 {
				for _, t2 := range H2 {
					if part != nil && !part.CanMatch(t1, t2) {
						continue
					}
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

			openUnmatched(H1, H2, m.Has, 0, A, l1)
			openUnmatched(H2, H1, m.HasDst, 1, A, l2)
		}
	}

	sort.SliceStable(A, func(i, j int) bool {
		di := Dice(A[i][0].Parent, A[i][1].Parent, m.Src())
		dj := Dice(A[j][0].Parent, A[j][1].Parent, m.Src())
		if di != dj {
			return di > dj
		}
		si := AncestorNameSimilarity(A[i][0], A[i][1])
		sj := AncestorNameSimilarity(A[j][0], A[j][1])
		return si > sj
	})

	for len(A) > 0 {
		pair := A[0]
		A = A[1:]
		t1, t2 := pair[0], pair[1]

		addIsomorphicPairs(t1, t2, m)

		A = slices.DeleteFunc(A, func(p [2]*treesitter.ASTNode) bool {
			return p[0] == t1 || p[1] == t2
		})
	}
}

type priorityList struct {
	buckets map[int][]*treesitter.ASTNode
	heights []int
}

func newPriorityList() *priorityList {
	return &priorityList{buckets: make(map[int][]*treesitter.ASTNode)}
}

func (l *priorityList) Push(n *treesitter.ASTNode) {
	h := Height(n)
	if _, exists := l.buckets[h]; !exists {
		idx, found := sort.Find(len(l.heights), func(i int) int {
			return h - l.heights[i]
		})
		if !found {
			l.heights = append(l.heights, 0)
			copy(l.heights[idx+1:], l.heights[idx:])
			l.heights[idx] = h
		}
	}
	l.buckets[h] = append(l.buckets[h], n)
}

func (l *priorityList) PeekMax() int {
	if len(l.heights) == 0 {
		return -1
	}
	return l.heights[len(l.heights)-1]
}

func (l *priorityList) Pop() []*treesitter.ASTNode {
	if len(l.heights) == 0 {
		return nil
	}
	maxH := l.heights[len(l.heights)-1]
	l.heights = l.heights[:len(l.heights)-1]
	nodes := l.buckets[maxH]
	delete(l.buckets, maxH)
	return nodes
}

func (l *priorityList) Open(t *treesitter.ASTNode) {
	for _, c := range t.Children {
		l.Push(c)
	}
}

func openUnmatched(
	nodes, partners []*treesitter.ASTNode,
	hasFn func(*treesitter.ASTNode) bool,
	pairIdx int,
	candidates [][2]*treesitter.ASTNode,
	targetList *priorityList,
) {
	for _, n := range nodes {
		matched := false
		for _, partner := range partners {
			if Isomorphic(n, partner) && hasFn(n) {
				matched = true
				break
			}
		}
		if !matched {
			matched = slices.ContainsFunc(candidates, func(p [2]*treesitter.ASTNode) bool {
				return p[pairIdx] == n
			})
		}
		if !matched {
			targetList.Open(n)
		}
	}
}
