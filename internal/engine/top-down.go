package engine

import (
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

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
