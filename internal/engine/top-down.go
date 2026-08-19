package engine

import (
	"slices"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type scoredPair struct {
	pair        [2]*treesitter.ASTNode
	dice        float64
	ancSim      int
	lineageSim  int
	nameMatched bool
	mismatched  bool
}

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

	h2ByHash := make(map[uint64][]*treesitter.ASTNode)

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

			clear(h2ByHash)
			for _, t2 := range H2 {
				if t2.Hash == 0 {
					t2.ComputeHashes()
				}
				h2ByHash[t2.Hash] = append(h2ByHash[t2.Hash], t2)
			}

			for _, t1 := range H1 {
				if t1.Hash == 0 {
					t1.ComputeHashes()
				}

				candidates := h2ByHash[t1.Hash]
				for _, t2 := range candidates {
					if part != nil && !part.CanMatch(t1, t2) {
						continue
					}
					if Isomorphic(t1, t2) {
						ambiguous := false

						for _, ta := range candidates {
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

			openUnmatched(H1, m.Has, 0, A, l1)
			openUnmatched(H2, m.HasDst, 1, A, l2)
		}
	}

	scored := make([]scoredPair, 0, len(A))
	for _, pair := range A {
		t1, t2 := pair[0], pair[1]

		name1 := getDeclarationName(t1)
		name2 := getDeclarationName(t2)
		if name1 == "" && name2 == "" {
			enc1 := GetEnclosingDeclaration(t1)
			enc2 := GetEnclosingDeclaration(t2)
			name1 = getDeclarationName(enc1)
			name2 = getDeclarationName(enc2)
		}

		nameMatched := name1 != "" && name2 != "" && name1 == name2
		mismatched := name1 != "" && name2 != "" && name1 != name2

		di := Dice(t1.Parent, t2.Parent, m.Src())
		si := AncestorNameSimilarity(t1, t2)
		li := parentLineageSimilarity(t1, t2)
		scored = append(scored, scoredPair{
			pair:        pair,
			dice:        di,
			ancSim:      si,
			lineageSim:  li,
			nameMatched: nameMatched,
			mismatched:  mismatched,
		})
	}

	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].nameMatched != scored[j].nameMatched {
			return scored[i].nameMatched
		}
		if scored[i].mismatched != scored[j].mismatched {
			return !scored[i].mismatched
		}
		if scored[i].ancSim != scored[j].ancSim {
			return scored[i].ancSim > scored[j].ancSim
		}
		if scored[i].dice != scored[j].dice {
			return scored[i].dice > scored[j].dice
		}
		return scored[i].lineageSim > scored[j].lineageSim
	})

	for len(scored) > 0 {
		sp := scored[0]
		pair := sp.pair
		scored = scored[1:]
		t1, t2 := pair[0], pair[1]

		if m.Has(t1) || m.HasDst(t2) {
			continue
		}
		if sp.mismatched && Height(t1) <= 2 {
			continue
		}

		addIsomorphicPairs(t1, t2, m)

		scored = slices.DeleteFunc(scored, func(s scoredPair) bool {
			return s.pair[0] == t1 || s.pair[1] == t2
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
	nodes []*treesitter.ASTNode,
	hasFn func(*treesitter.ASTNode) bool,
	pairIdx int,
	candidates [][2]*treesitter.ASTNode,
	targetList *priorityList,
) {
	for _, n := range nodes {
		if !hasFn(n) && !slices.ContainsFunc(candidates, func(p [2]*treesitter.ASTNode) bool { return p[pairIdx] == n }) {
			targetList.Open(n)
		}
	}
}

// Checks if the immediate parent and grandparent node types match to help break
// ties when identical subtrees appear in different parts of the file.
func parentLineageSimilarity(t1, t2 *treesitter.ASTNode) int {
	score := 0
	p1, p2 := t1.Parent, t2.Parent
	if p1 != nil && p2 != nil && p1.Type == p2.Type {
		score += 2
		gp1, gp2 := p1.Parent, p2.Parent
		if gp1 != nil && gp2 != nil && gp1.Type == gp2.Type {
			score += 1
		}
	}
	return score
}
