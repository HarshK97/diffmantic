package engine

import (
	"cmp"
	"slices"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

type scoredPair struct {
	pair                    [2]*treesitter.ASTNode
	dice                    float64
	stmtAffinity            float64
	parentLeafSim           float64
	ancSim, lineageSim      int
	drift                   int
	relDrift                int
	hasRelDrift             bool
	slotMatch               bool
	isFallback              bool
	nameMatched, mismatched bool
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
						effectiveHeight := Height(t1)
						r := rulesFor(t1)
						if r != nil && r.IsWrapper(t1.Type) && len(t1.Children) == 1 {
							effectiveHeight = Height(t1.Children[0])
						}
						// Subtrees without identifiers (like boilerplate `{ return false }`) shouldn't match across different scopes.
						if effectiveHeight <= 2 || isDeclarationHeader(t1, r) || !hasIdentifier(t1, r) {
							s1 := getScopeName(t1)
							s2 := getScopeName(t2)
							if s1 != "" && s2 != "" && s1 != s2 {
								continue
							}
						}

						if isCrossStatementDredging(t1, t2, m, part, r) {
							continue
						}

						if t1.Parent != nil && t2.Parent != nil {
							isCall1 := (r != nil && r.IsCall(t1.Parent.Type)) || (r == nil && rules.IsCall(t1.Parent.Type))
							isCall2 := (r != nil && r.IsCall(t2.Parent.Type)) || (r == nil && rules.IsCall(t2.Parent.Type))
							if isCall1 && isCall2 && len(t1.Parent.Children) > 0 && len(t2.Parent.Children) > 0 {
								if !calleeLeavesOverlap(t1.Parent, t2.Parent) && !isSubstantiveDelimitedContainer(t1, r) {
									continue
								}
							}
						}

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
							if hasDelimitedContainerConflict(t1, t2, m, r) {
								continue
							}
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

		name1 := getScopeName(t1)
		name2 := getScopeName(t2)

		nameMatched := name1 != "" && name2 != "" && name1 == name2
		mismatched := name1 != "" && name2 != "" && name1 != name2

		di := Dice(t1.Parent, t2.Parent, m.Src())
		isFallback := false
		r1 := rulesFor(t1)
		r2 := rulesFor(t2)
		c1 := getEnclosingConstruct(t1, r1)
		c2 := getEnclosingConstruct(t2, r2)
		if di == 0 && nameMatched {
			if c1 != nil && c2 != nil && (c1 != t1.Parent || c2 != t2.Parent) {
				if f := Dice(c1, c2, m.Src()) * 0.5; f > 0 {
					di = f
					isFallback = true
				}
			}
		}

		s1 := FindImmediateStatement(t1, r1)
		s2 := FindImmediateStatement(t2, r2)
		stmtSim := LeafSimilarity(s1, s2)
		constructSim := stmtSim
		if s1 != c1 || s2 != c2 {
			constructSim = LeafSimilarity(c1, c2)
		}
		stmtAffinity := 0.5*stmtSim + 0.5*constructSim

		si := AncestorNameSimilarity(t1, t2)
		li := parentLineageSimilarity(t1, t2)
		pls := LeafSimilarity(t1.Parent, t2.Parent)
		drift := m.AdjustedLineDistance(t1, t2)

		anchor1 := resolveDriftAnchor(t1, c1)
		anchor2 := resolveDriftAnchor(t2, c2)

		var (
			hasRelDrift bool
			relDrift    int
			slotMatch   bool
		)

		if nameMatched && anchor1 != nil && anchor2 != nil {
			isCompatible := anchor1.Type == anchor2.Type &&
				(LeafSimilarity(anchor1, anchor2) >= 0.30 || Dice(anchor1, anchor2, m.Src()) >= 0.30)

			if !isCompatible {
				d1 := GetEnclosingDeclaration(t1)
				d2 := GetEnclosingDeclaration(t2)
				if d1 != nil && d2 != nil && d1.Type == d2.Type {
					anchor1 = d1
					anchor2 = d2
					isCompatible = true
				}
			}

			if isCompatible {
				hasRelDrift = true
				relSrc := int(t1.StartRow) - int(anchor1.StartRow)
				relDst := int(t2.StartRow) - int(anchor2.StartRow)
				d := relSrc - relDst
				if d < 0 {
					d = -d
				}
				relDrift = d

				if anchor1 == c1 && anchor2 == c2 {
					slot1 := branchSlotIndex(t1, c1)
					slot2 := branchSlotIndex(t2, c2)
					slotMatch = (slot1 >= 0 && slot2 >= 0 && slot1 == slot2)
				}
			}
		}

		scored = append(scored, scoredPair{
			pair:          pair,
			dice:          di,
			stmtAffinity:  stmtAffinity,
			parentLeafSim: pls,
			isFallback:    isFallback,
			ancSim:        si,
			lineageSim:    li,
			drift:         drift,
			relDrift:      relDrift,
			hasRelDrift:   hasRelDrift,
			slotMatch:     slotMatch,
			nameMatched:   nameMatched,
			mismatched:    mismatched,
		})
	}

	slices.SortStableFunc(scored, func(a, b scoredPair) int {
		if a.nameMatched != b.nameMatched {
			if a.nameMatched {
				return -1
			}
			return 1
		}
		if a.mismatched != b.mismatched {
			if !a.mismatched {
				return -1
			}
			return 1
		}
		// Direct parent matches always beat outer construct fallbacks.
		if a.isFallback != b.isFallback {
			if !a.isFallback && a.dice > 0 {
				return -1
			}
			if !b.isFallback && b.dice > 0 {
				return 1
			}
		}

		if c := cmp.Compare(b.dice, a.dice); c != 0 {
			return c
		}
		if c := cmp.Compare(b.ancSim, a.ancSim); c != 0 {
			return c
		}
		if c := cmp.Compare(b.lineageSim, a.lineageSim); c != 0 {
			return c
		}
		if c := cmp.Compare(b.parentLeafSim, a.parentLeafSim); c != 0 {
			return c
		}
		if c := cmp.Compare(b.stmtAffinity, a.stmtAffinity); c != 0 {
			return c
		}

		// Prefer in-branch candidates over cross-branch candidates when competing within the same construct.
		if a.nameMatched && b.nameMatched && a.slotMatch != b.slotMatch {
			if a.slotMatch {
				return -1
			}
			return 1
		}

		// Compare drift relative to the construct or function so upstream deletions don't skew line distance.
		if a.nameMatched && b.nameMatched && a.hasRelDrift && b.hasRelDrift {
			if c := cmp.Compare(a.relDrift, b.relDrift); c != 0 {
				return c
			}
		}

		// Only compare line drift when both candidates belong to the same named declaration.
		// Across different functions or top-level code, physical row distance would penalize
		// moved functions that shifted far down the file.
		if a.nameMatched && b.nameMatched {
			if c := cmp.Compare(a.drift, b.drift); c != 0 {
				return c
			}
		}

		// If still tied, keep source document order so statements in the same function don't swap places.
		if a.nameMatched && b.nameMatched {
			if c := cmp.Compare(a.pair[0].StartByte, b.pair[0].StartByte); c != 0 {
				return c
			}
			return cmp.Compare(a.pair[1].StartByte, b.pair[1].StartByte)
		}

		return 0
	})

	for len(scored) > 0 {
		sp := scored[0]
		pair := sp.pair
		scored = scored[1:]
		t1, t2 := pair[0], pair[1]

		if m.Has(t1) || m.HasDst(t2) {
			continue
		}
		if sp.mismatched && (Height(t1) <= 2 || isDeclarationHeader(t1, rulesFor(t1)) || !hasIdentifier(t1, rulesFor(t1))) {
			continue
		}
		if hasDelimitedContainerConflict(t1, t2, m, rulesFor(t1)) {
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
	// Sort by document position so ties between equal-height candidates
	// resolve in preorder instead of queue traversal discovery order.
	slices.SortFunc(nodes, func(a, b *treesitter.ASTNode) int {
		return cmp.Or(
			cmp.Compare(a.StartByte, b.StartByte),
			cmp.Compare(a.StartRow, b.StartRow),
			cmp.Compare(a.StartCol, b.StartCol),
		)
	})
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

// parentLineageSimilarity reports whether the immediate parent and grandparent
// node types match to help break ties when identical subtrees appear in different
// parts of the file.
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

// getScopeName returns the identifier of n, its enclosing declaration, or
// the enclosing call expression for anonymous callbacks/closures.
func getScopeName(n *treesitter.ASTNode) string {
	if n == nil {
		return ""
	}
	if name := getDeclarationName(n); name != "" {
		return name
	}
	r := rulesFor(n)
	if hasClosureChild(n, r) {
		if name := getClosureDeclarationName(n, r); name != "" {
			return name
		}
	}
	enc := GetEnclosingDeclaration(n)
	if enc != nil {
		if name := getDeclarationName(enc); name != "" {
			return name
		}
		if hasClosureChild(enc, r) {
			if name := getClosureDeclarationName(enc, r); name != "" {
				return name
			}
		}
		// Anonymous callbacks lack a declaration name, so resolve scope
		// from the caller (e.g. vim.wait, describe, or t.Run).
		r = rulesFor(enc)
		for curr := enc.Parent; curr != nil; curr = curr.Parent {
			if isCallNode(curr, r) {
				if len(curr.Children) > 0 {
					c := curr.Children[0]
					if c.Label != "" {
						return c.Label
					}
					if len(c.Children) > 0 {
						last := c.Children[len(c.Children)-1]
						if last.Label != "" {
							return last.Label
						}
					}
				}
			}
			if r != nil && r.IsDeclaration(curr.Type) {
				break
			}
		}
	}
	return ""
}

// isCrossStatementDredging checks if an isomorphic subtree candidate reaches across
// distinct statements, violating line partition locks, statement affinity, or drift limits.
func isCrossStatementDredging(t1, t2 *treesitter.ASTNode, m *Mapping, part *LinePartition, r *rules.Rules) bool {
	s1 := FindImmediateStatement(t1, r)
	s2 := FindImmediateStatement(t2, r)
	if s1 == nil || s2 == nil {
		return false
	}
	// If both nodes are the statements themselves, let them match normally.
	if s1 == t1 && s2 == t2 {
		return false
	}
	// If the enclosing statements already map to each other, this is an intra-statement match.
	if m != nil && m.Src()[s1] == s2 {
		return false
	}

	r = cmp.Or(r, rulesFor(t1))
	scope1 := getScopeName(s1.Parent)
	scope2 := getScopeName(s2.Parent)
	sameScope := (scope1 == "" && scope2 == "") || (scope1 != "" && scope2 != "" && scope1 == scope2)

	// Matching declarations with the same name belong together; don't reject them as dredging.
	name1 := cmp.Or(getDeclarationName(s1), getClosureDeclarationName(s1, r))
	name2 := cmp.Or(getDeclarationName(s2), getClosureDeclarationName(s2, r))
	if sameScope && name1 != "" && name2 != "" && name1 == name2 && TypesMatch(s1.Type, s2.Type, r) {
		return false
	}

	// Gate 1: Phase 0 LinePartition Untouched Line Lock
	if part != nil {
		isG1A, _ := part.IsGroup1A(s1)
		isG1B, _ := part.IsGroup1B(s2)
		if (isG1A || isG1B) && !part.CanMatch(s1, s2) {
			return true // Don't dredge out of or into an untouched stationary statement.
		}
	}

	// Gate 2: Statement Boundary Affinity (Case 57 Tier 1)
	if !TypesMatch(s1.Type, s2.Type, r) {
		isExtractTarget := r != nil && (r.IsAssignment(s2.Type) || r.IsLocalVarDeclaration(s2.Type) || r.IsJumpStatement(s2.Type) ||
			r.IsAssignment(s1.Type) || r.IsLocalVarDeclaration(s1.Type))
		if !hasIdentifier(t1, r) || !isExtractTarget {
			return true // Incompatible statement types, and this isn't an extraction.
		}
	}
	hasMappedOverlap := m != nil && m.DiceSrc(s1, s2) > 0
	if !hasMappedOverlap && !hasIdentifier(t1, r) && LeafSimilarity(s1, s2) < 0.30 {
		return true // Not enough statement-level lexical overlap.
	}

	// Gate 3: Relative Drift Boundedness (Case 79)
	anchor1 := resolveDriftAnchor(s1, getEnclosingConstruct(s1, r))
	anchor2 := resolveDriftAnchor(s2, getEnclosingConstruct(s2, r))
	row1, row2 := int(s1.StartRow), int(s2.StartRow)
	if anchor1 != nil {
		row1 -= int(anchor1.StartRow)
	}
	if anchor2 != nil {
		row2 -= int(anchor2.StartRow)
	}
	drift := row1 - row2
	return drift < -10 || drift > 10
}

// isCallNode reports whether n is a function or method invocation node.
func isCallNode(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r == nil {
		r = rulesFor(n)
	}
	if r != nil {
		return r.IsCall(n.Type)
	}
	return rules.IsCall(n.Type)
}

// hasIdentifier reports whether n or any descendant contains an identifier.
func hasIdentifier(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r == nil {
		r = rulesFor(n)
	}
	isIdent := func(t string) bool {
		return (r != nil && r.IsIdentifier(t)) || (r == nil && rules.IsIdentifier(t))
	}
	if n.Index != nil && n.PreSize > 0 {
		for _, d := range n.Index.Nodes[n.ID : n.ID+n.PreSize] {
			if isIdent(d.Type) {
				return true
			}
		}
		return false
	}
	for _, d := range n.PreOrder() {
		if isIdent(d.Type) {
			return true
		}
	}
	return false
}

// getEnclosingConstruct finds the top-level statement directly under the enclosing
// declaration's body block, stopping before it escapes into the declaration itself.
func getEnclosingConstruct(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r = cmp.Or(r, rulesFor(n))
	curr := n
	for curr.Parent != nil {
		p := curr.Parent
		if p.Parent == nil {
			return curr
		}
		if (r != nil && r.IsDeclaration(p.Type)) || rules.IsDeclaration(p.Type) {
			return curr
		}
		if (r != nil && r.IsDeclaration(p.Parent.Type)) || rules.IsDeclaration(p.Parent.Type) {
			return curr
		}
		curr = p
	}
	return curr
}

// resolveDriftAnchor returns the baseline node used to measure relative line drift.
// If t is inside a construct (if, for, switch, etc.), we measure from that construct.
// If t is already at the function level, we fall back to the function declaration itself.
func resolveDriftAnchor(t, c *treesitter.ASTNode) *treesitter.ASTNode {
	if c != nil && c != t {
		return c
	}
	return GetEnclosingDeclaration(t)
}

// branchSlotIndex returns the child index of the direct child container under c that encloses t.
// Returns -1 if t is nil, c is nil, or t is c.
func branchSlotIndex(t, c *treesitter.ASTNode) int {
	if t == nil || c == nil || t == c {
		return -1
	}
	curr := t
	for curr.Parent != nil && curr.Parent != c {
		curr = curr.Parent
	}
	if curr.Parent != c {
		return -1
	}
	return curr.ChildIndex()
}

// calleeLeavesOverlap reports whether two call expressions share any callee leaf tokens.
func calleeLeavesOverlap(call1, call2 *treesitter.ASTNode) bool {
	if call1 == nil || call2 == nil || len(call1.Children) == 0 || len(call2.Children) == 0 {
		return true
	}
	fn1 := call1.Children[0]
	fn2 := call2.Children[0]
	if fn1 == nil || fn2 == nil {
		return true
	}
	leaves1 := fn1.Leaves()
	for _, l2 := range fn2.Leaves() {
		if l2.Label != "" && slices.ContainsFunc(leaves1, func(l1 *treesitter.ASTNode) bool {
			return l1.Label == l2.Label
		}) {
			return true
		}
	}
	return false
}

// isSubstantiveDelimitedContainer reports whether n is a delimited container
// (like an argument_list or token_tree) carrying real content (string literals,
// multi-line blocks, or closures) that can match across differing callee names
// without risking cross-call argument theft.
func isSubstantiveDelimitedContainer(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	isDelimited := (r != nil && r.IsDelimitedContainer(n.Type)) || (r == nil && rules.IsDelimitedContainer(n.Type))
	if !isDelimited {
		return false
	}
	if n.EndRow > n.StartRow && n.PreSize >= 4 {
		return true
	}
	for label := range n.LeafLabels() {
		if isQuotedLiteral(label) && len(label) >= 4 {
			return true
		}
	}
	return hasClosureChild(n, r)
}
