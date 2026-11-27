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
	fieldScore              int
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
						if effectiveHeight <= 2 || declarationHeader(t1, r) != nil || !hasIdentifier(t1, r) {
							s1 := getScopeName(t1)
							s2 := getScopeName(t2)
							if s1 != "" && s2 != "" && s1 != s2 {
								continue
							}
						}

						if isCrossStatementDredging(t1, t2, m, part, r) {
							continue
						}

						p1 := t1.Parent
						if p1 != nil && !isCallNode(p1, r) && isDelimitedContainerNode(p1, r) && isCallNode(p1.Parent, r) && !hasIdentifier(t1, r) {
							p1 = p1.Parent
						}
						p2 := t2.Parent
						if p2 != nil && !isCallNode(p2, r) && isDelimitedContainerNode(p2, r) && isCallNode(p2.Parent, r) && !hasIdentifier(t2, r) {
							p2 = p2.Parent
						}
						if p1 != nil && p2 != nil && isCallNode(p1, r) && isCallNode(p2, r) {
							if !calleeLeavesOverlap(p1, p2, r) && !isSubstantiveDelimitedContainer(t1, r) {
								continue
							}
						}

						ambiguous := false

						for _, ta := range candidates {
							if ta != t2 && (part == nil || part.CanMatch(t1, ta)) && Isomorphic(t1, ta) {
								ambiguous = true
								break
							}
						}
						if !ambiguous {
							for _, ta := range H1 {
								if ta != t1 && (part == nil || part.CanMatch(ta, t2)) && Isomorphic(ta, t2) {
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
				d1 := getEnclosingDriftScope(t1, r1)
				d2 := getEnclosingDriftScope(t2, r2)
				if d1 == nil || d2 == nil || d1.Type != d2.Type {
					d1 = GetEnclosingDeclaration(t1)
					d2 = GetEnclosingDeclaration(t2)
				}
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

		f1 := t1.FieldName()
		f2 := t2.FieldName()
		fieldScore := 0
		if f1 != "" && f2 != "" {
			if f1 == f2 {
				fieldScore = 2
			} else if r1 != nil && r2 != nil && (r1.FieldRole(f1)&r2.FieldRole(f2)) != rules.RoleNone {
				fieldScore = 1
			} else if treesitter.IsMutuallyExclusiveField(f1, f2) {
				fieldScore = -2
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
			fieldScore:    fieldScore,
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

		// Penalize conflicting slots (like key vs value) before checking line drift so same-line tokens don't falsely match.
		if (a.fieldScore < 0 || b.fieldScore < 0) && a.fieldScore != b.fieldScore {
			return cmp.Compare(b.fieldScore, a.fieldScore)
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
			// When row drift is tied, compare relative column offset within the parent
			// so same-line siblings stay in their slot even if the line was re-indented or renamed.
			colDist := func(p [2]*treesitter.ASTNode) int {
				srcCol, dstCol := int(p[0].StartCol), int(p[1].StartCol)
				if p[0].Parent != nil && p[1].Parent != nil &&
					p[0].Parent.StartRow == p[0].StartRow &&
					p[1].Parent.StartRow == p[1].StartRow {
					srcCol -= int(p[0].Parent.StartCol)
					dstCol -= int(p[1].Parent.StartCol)
				}
				diff := srcCol - dstCol
				return max(diff, -diff)
			}
			if c := cmp.Compare(colDist(a.pair), colDist(b.pair)); c != 0 {
				return c
			}
		}

		// When drift is tied, use field and role concordance before falling back to document order.
		if c := cmp.Compare(b.fieldScore, a.fieldScore); c != 0 {
			return c
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
		if m.Has(scored[0].pair[0]) || m.HasDst(scored[0].pair[1]) {
			scored = scored[1:]
			continue
		}
		sp := selectMonotonicPair(scored, m)
		pair := sp.pair
		t1, t2 := pair[0], pair[1]

		if (sp.mismatched && (Height(t1) <= 2 || declarationHeader(t1, rulesFor(t1)) != nil || !hasIdentifier(t1, rulesFor(t1)))) ||
			hasDelimitedContainerConflict(t1, t2, m, rulesFor(t1)) {
			if scored[0].pair[0] == t1 && scored[0].pair[1] == t2 {
				scored = scored[1:]
			} else {
				scored = slices.DeleteFunc(scored, func(s scoredPair) bool {
					return s.pair[0] == t1 && s.pair[1] == t2
				})
			}
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

// getScopeName returns the enclosing declaration name for nodes inside a
// declaration signature or header, the identifier of n, its enclosing declaration,
// or the enclosing call expression for anonymous callbacks/closures.
func getScopeName(n *treesitter.ASTNode) string {
	if n == nil {
		return ""
	}
	r := rulesFor(n)
	if enc := declarationHeader(n, r); enc != nil {
		if name := getDeclarationName(enc); name != "" {
			return name
		}
	}
	if name := getDeclarationName(n); name != "" {
		return name
	}
	if hasClosureChild(n, r) {
		if name := getClosureDeclarationName(n, r); name != "" {
			return name
		}
	}
	enc := GetEnclosingDeclaration(n)
	// Skip imports so symbols inside them resolve their scope to the enclosing module or file.
	for enc != nil && ((r != nil && r.IsImport(enc.Type)) || (r == nil && rules.IsImport(enc.Type))) {
		enc = GetEnclosingDeclaration(enc)
	}
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
	sameScope := scope1 == scope2

	// Matching declarations with the same name belong together; don't reject them as dredging.
	name1 := cmp.Or(getDeclarationName(s1), getClosureDeclarationName(s1, r))
	name2 := cmp.Or(getDeclarationName(s2), getClosureDeclarationName(s2, r))
	if sameScope && name1 != "" && name2 != "" && name1 == name2 && TypesMatch(s1.Type, s2.Type, r) {
		return false
	}

	// Similar statements in the same scope are evolving in place, not dredging across scopes.
	if sameScope && s1.Type == s2.Type && LeafSimilarity(s1, s2) >= 0.50 {
		return false
	}
	// A short var declaration often turns into an assignment (:= to =).
	// If the right-hand sides still match, treat this as in-place evolution, not dredging.
	if sameScope && r != nil && s1.Type != s2.Type && TypesMatch(s1.Type, s2.Type, r) &&
		(r.IsAssignment(s1.Type) || r.IsLocalVarDeclaration(s1.Type)) &&
		(r.IsAssignment(s2.Type) || r.IsLocalVarDeclaration(s2.Type)) {
		_, rhs1 := getAssignmentSides(s1, r)
		_, rhs2 := getAssignmentSides(s2, r)
		for _, r1 := range rhs1 {
			for _, r2 := range rhs2 {
				if LeafSimilarity(r1, r2) >= 0.50 {
					return false
				}
			}
		}
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
	if f1, f2 := t1.FieldName(), t2.FieldName(); f1 != "" && f2 != "" && treesitter.IsMutuallyExclusiveField(f1, f2) {
		return true // Conflicting parent fields (e.g. key vs value, condition vs consequence) shouldn't match across statements.
	}
	if !TypesMatch(s1.Type, s2.Type, r) {
		isExtractTarget := r != nil && (r.IsAssignment(s2.Type) || r.IsLocalVarDeclaration(s2.Type) || r.IsJumpStatement(s2.Type) ||
			r.IsAssignment(s1.Type) || r.IsLocalVarDeclaration(s1.Type))
		if !hasIdentifier(t1, r) || !isExtractTarget {
			return true // Incompatible statement types, and this isn't an extraction.
		}
	}

	// Don't pair LHS target variables across different assignment types (like := vs =)
	// unless their values actually match.
	if s1.Type != s2.Type {
		lhs1, rhs1 := getAssignmentSides(s1, r)
		if isNodeInLHS(t1, lhs1) {
			lhs2, rhs2 := getAssignmentSides(s2, r)
			if isNodeInLHS(t2, lhs2) && len(rhs1) > 0 && len(rhs2) > 0 {
				hasOverlap := slices.ContainsFunc(rhs1, func(r1 *treesitter.ASTNode) bool {
					return slices.ContainsFunc(rhs2, func(r2 *treesitter.ASTNode) bool {
						return (m != nil && m.DiceSrc(r1, r2) > 0) || LeafSimilarity(r1, r2) >= 0.30
					})
				})
				if !hasOverlap {
					return true // Target variables pair across assignments with disjoint values
				}
			}
		}
	}

	hasMappedOverlap := m != nil && m.DiceSrc(s1, s2) > 0
	if !hasMappedOverlap && LeafSimilarity(s1, s2) < 0.30 {
		return true // Not enough statement-level lexical overlap.
	}

	// Gate 3: Relative Drift Boundedness (Case 79)
	c1 := getEnclosingConstruct(s1, r)
	c2 := getEnclosingConstruct(s2, r)

	// If s1 and s2 sit in different constructs (like separate if/for blocks), make sure the constructs themselves match.
	if c1 != nil && c2 != nil && c1 != s1 && c2 != s2 {
		// If either construct is already mapped to something else, don't dredge across them.
		if m != nil {
			if mappedC1 := m.Src()[c1]; mappedC1 != nil && mappedC1 != c2 && !mappedC1.Contains(s2) {
				return true // s1's enclosing construct is already mapped elsewhere
			}
			if mappedC2 := m.Dst()[c2]; mappedC2 != nil && mappedC2 != c1 && !mappedC2.Contains(s1) {
				return true // s2's enclosing construct is already mapped elsewhere
			}
		}
		if m == nil || m.Src()[c1] != c2 {
			isCompatConstruct := TypesMatch(c1.Type, c2.Type, r) && LeafSimilarity(c1, c2) >= 0.30
			if !isCompatConstruct {
				return true // Dredging across incompatible enclosing control constructs
			}
		}
	}

	// Calibrate drift against Phase 0 LineDiff anchors so upstream deletions don't inflate distance.
	var calibratedDrift int
	if part != nil {
		calibratedDrift = part.CalibratedDrift(int(s1.StartRow), int(s2.StartRow))
	} else {
		// Without line partitions, fall back to relative distance from the enclosing construct.
		anchor1 := resolveDriftAnchor(s1, c1)
		anchor2 := resolveDriftAnchor(s2, c2)
		if anchor1 != nil && anchor2 != nil && (m == nil || m.Src()[anchor1] != anchor2) {
			calibratedDrift = max(int(s1.StartRow)-int(s2.StartRow), int(s2.StartRow)-int(s1.StartRow))
		} else {
			row1, row2 := int(s1.StartRow), int(s2.StartRow)
			if anchor1 != nil {
				row1 -= int(anchor1.StartRow)
			}
			if anchor2 != nil {
				row2 -= int(anchor2.StartRow)
			}
			calibratedDrift = max(row1-row2, row2-row1)
		}
	}

	if calibratedDrift > 10 {
		return true // Relative drift exceeds the 10-line limit.
	}

	return false
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

// isDelimitedContainerNode checks if a node is an argument list or collection delimiter.
func isDelimitedContainerNode(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r == nil {
		r = rulesFor(n)
	}
	if r != nil {
		return r.IsDelimitedContainer(n.Type)
	}
	return rules.IsDelimitedContainer(n.Type)
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

func isDeclarationOrClosure(nodeType string, r *rules.Rules) bool {
	if r != nil {
		return r.IsDeclaration(nodeType) || r.IsClosure(nodeType)
	}
	return rules.IsDeclaration(nodeType) || rules.IsClosure(nodeType)
}

// getEnclosingDriftScope returns the nearest enclosing declaration or closure so we
// measure line drift inside the closure instead of the outer function.
func getEnclosingDriftScope(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r = cmp.Or(r, rulesFor(n))
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isLocalVar := (r != nil && r.IsLocalVarDeclaration(curr.Type)) || (r == nil && rules.IsLocalVarDeclaration(curr.Type))
		if isDeclarationOrClosure(curr.Type, r) || isLocalVar {
			return curr
		}
	}
	return nil
}

// getEnclosingConstruct finds the top-level statement directly under the enclosing
// declaration or closure body block, stopping before it escapes into the enclosing scope.
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
		if isDeclarationOrClosure(p.Type, r) || isDeclarationOrClosure(p.Parent.Type, r) {
			return curr
		}
		curr = p
	}
	return curr
}

// getAssignmentSides splits an assignment or declaration statement into its LHS (targets)
// and RHS (values) child nodes.
func getAssignmentSides(n *treesitter.ASTNode, r *rules.Rules) (lhs, rhs []*treesitter.ASTNode) {
	if n == nil {
		return nil, nil
	}
	r = cmp.Or(r, rulesFor(n))
	isAssign := (r != nil && r.IsAssignment(n.Type)) || (r == nil && rules.IsAssignment(n.Type))
	isDecl := (r != nil && r.IsLocalVarDeclaration(n.Type)) || (r == nil && rules.IsLocalVarDeclaration(n.Type))
	if !isAssign && !isDecl {
		return nil, nil
	}
	opIdx := slices.IndexFunc(n.Children, func(child *treesitter.ASTNode) bool {
		return isOperatorGlue(child, r)
	})
	if opIdx < 0 {
		return nil, nil
	}
	return n.Children[:opIdx], n.Children[opIdx+1:]
}

// isNodeInLHS reports whether node t sits within the LHS target list of an assignment or declaration.
func isNodeInLHS(t *treesitter.ASTNode, lhs []*treesitter.ASTNode) bool {
	return slices.ContainsFunc(lhs, func(l *treesitter.ASTNode) bool {
		return l == t || l.Contains(t)
	})
}

// resolveDriftAnchor returns the baseline node used to measure relative line drift.
// If t is inside a construct (if, for, switch, call, etc.), we measure from that construct.
// If t sits directly in a function or closure, fall back to that scope.
func resolveDriftAnchor(t, c *treesitter.ASTNode) *treesitter.ASTNode {
	if c != nil && c != t {
		return c
	}
	return getEnclosingDriftScope(t, rulesFor(t))
}

func sameStructuralTier(a, b scoredPair) bool {
	return a.nameMatched == b.nameMatched &&
		a.mismatched == b.mismatched &&
		a.isFallback == b.isFallback &&
		a.dice == b.dice &&
		a.ancSim == b.ancSim &&
		a.lineageSim == b.lineageSim &&
		a.parentLeafSim == b.parentLeafSim &&
		a.stmtAffinity == b.stmtAffinity &&
		(!a.nameMatched || a.slotMatch == b.slotMatch) &&
		a.fieldScore == b.fieldScore
}

func findPairInTier(tier []scoredPair, t1, t2 *treesitter.ASTNode, m *Mapping) (scoredPair, bool) {
	for _, s := range tier {
		if s.pair[0] == t1 && s.pair[1] == t2 && !m.Has(t1) && !m.HasDst(t2) {
			return s, true
		}
	}
	return scoredPair{}, false
}

// enclosingSiblingContainer returns the deepest shared block or container for a and b,
// plus the direct children that hold them.
func enclosingSiblingContainer(a, b *treesitter.ASTNode) (container, childA, childB *treesitter.ASTNode) {
	if a == nil || b == nil || a == b {
		return nil, nil, nil
	}
	isSiblingBox := func(p *treesitter.ASTNode) bool {
		r := rulesFor(p)
		if r != nil {
			return r.IsBlock(p.Type) || r.IsDelimitedContainer(p.Type) || r.IsDeclaration(p.Type) || r.IsClosure(p.Type)
		}
		return rules.IsBlock(p.Type) || rules.IsDelimitedContainer(p.Type) || rules.IsDeclaration(p.Type) || rules.IsClosure(p.Type)
	}
	if a.Parent != nil && a.Parent == b.Parent {
		if isSiblingBox(a.Parent) {
			return a.Parent, a, b
		}
		return nil, nil, nil
	}
	ancA := make(map[*treesitter.ASTNode]*treesitter.ASTNode, 8)
	for curr := a; curr.Parent != nil; curr = curr.Parent {
		ancA[curr.Parent] = curr
	}
	for curr := b; curr.Parent != nil; curr = curr.Parent {
		if cA, ok := ancA[curr.Parent]; ok && cA != curr {
			if isSiblingBox(curr.Parent) {
				return curr.Parent, cA, curr
			}
			return nil, nil, nil
		}
	}
	return nil, nil, nil
}

func hasMappedInterveningSibling(container, childA, childB *treesitter.ASTNode, m *Mapping, isDst bool) bool {
	if container == nil || childA == nil || childB == nil || m == nil {
		return false
	}
	idxA, idxB := childA.ChildIndex(), childB.ChildIndex()
	if idxA > idxB {
		idxA, idxB = idxB, idxA
	}
	for i := idxA + 1; i < idxB; i++ {
		c := container.Children[i]
		if (!isDst && m.Has(c)) || (isDst && m.HasDst(c)) {
			return true
		}
	}
	return false
}

// monotonicImprovesDrift checks whether uncrossing (sp, other) into (p12, p21)
// lowers the worst-case line drift without increasing total drift.
func monotonicImprovesDrift(sp, other, p12, p21 scoredPair) bool {
	improvesMetric := func(c1, c2, m1, m2 int) (better, equal bool) {
		cMax, mMax := max(c1, c2), max(m1, m2)
		cSum, mSum := c1+c2, m1+m2
		if mMax != cMax {
			return mMax < cMax && mSum <= cSum, false
		}
		if mSum != cSum {
			return mSum < cSum, false
		}
		return false, true
	}

	if sp.hasRelDrift && other.hasRelDrift && p12.hasRelDrift && p21.hasRelDrift {
		if better, eq := improvesMetric(sp.relDrift, other.relDrift, p12.relDrift, p21.relDrift); !eq {
			return better
		}
	}
	better, _ := improvesMetric(sp.drift, other.drift, p12.drift, p21.drift)
	return better
}

// selectMonotonicPair prevents greedy minimum-drift selection from inverting identical
// siblings in the same container when an earlier deletion or insertion shifts lines.
func selectMonotonicPair(scored []scoredPair, m *Mapping) scoredPair {
	sp := scored[0]
	if !sp.nameMatched {
		return sp
	}
	tierEnd := 1
	for tierEnd < len(scored) && sameStructuralTier(sp, scored[tierEnd]) {
		tierEnd++
	}
	if tierEnd < 2 {
		return sp
	}
	tier := scored[:tierEnd]
	var tierMap map[[2]*treesitter.ASTNode]scoredPair
	if len(tier) > 8 {
		tierMap = make(map[[2]*treesitter.ASTNode]scoredPair, len(tier))
		for _, s := range tier {
			if !m.Has(s.pair[0]) && !m.HasDst(s.pair[1]) {
				tierMap[s.pair] = s
			}
		}
	}
	lookupInTier := func(t1, t2 *treesitter.ASTNode) (scoredPair, bool) {
		if tierMap != nil {
			s, ok := tierMap[[2]*treesitter.ASTNode{t1, t2}]
			return s, ok
		}
		return findPairInTier(tier, t1, t2, m)
	}
	for {
		t1, t2 := sp.pair[0], sp.pair[1]
		replaced := false
		for i := range tier {
			other := tier[i]
			t1p, t2p := other.pair[0], other.pair[1]
			if t1p == t1 || t2p == t2 || m.Has(t1p) || m.HasDst(t2p) {
				continue
			}
			// Check whether (t1, t2) and (t1p, t2p) cross in document order.
			crosses := (t1.StartByte < t1p.StartByte && t2.StartByte > t2p.StartByte) ||
				(t1.StartByte > t1p.StartByte && t2.StartByte < t2p.StartByte)
			if !crosses {
				continue
			}
			p12, ok1 := lookupInTier(t1, t2p)
			p21, ok2 := lookupInTier(t1p, t2)
			if !ok1 || !ok2 {
				continue
			}
			cSrc, chSrc1, chSrc2 := enclosingSiblingContainer(t1, t1p)
			cDst, chDst1, chDst2 := enclosingSiblingContainer(t2, t2p)
			if cSrc == nil || cDst == nil || cSrc.Type != cDst.Type {
				continue
			}
			if hasMappedInterveningSibling(cSrc, chSrc1, chSrc2, m, false) ||
				hasMappedInterveningSibling(cDst, chDst1, chDst2, m, true) {
				continue
			}
			if monotonicImprovesDrift(sp, other, p12, p21) {
				if t1.StartByte < t1p.StartByte {
					sp = p12
				} else {
					sp = p21
				}
				replaced = true
				break
			}
		}
		if !replaced {
			return sp
		}
	}
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

// extractCalleeNode pulls out the callee identifier (e.g. 'bar' in 'foo.bar()') so receiver
// chains don't trigger false matches in method chaining.
func extractCalleeNode(call *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if call == nil || len(call.Children) == 0 {
		return nil
	}
	if r == nil {
		r = rulesFor(call)
	}

	// For Java method_invocation: [receiver, ., identifier, argument_list].
	// Scanning backwards locates argument_list first even if type_arguments is present.
	for i := len(call.Children) - 1; i >= 1; i-- {
		c := call.Children[i]
		if (r != nil && r.IsDelimitedContainer(c.Type)) || (r == nil && rules.IsDelimitedContainer(c.Type)) {
			prev := call.Children[i-1]
			if (r != nil && r.IsIdentifier(prev.Type)) || (r == nil && rules.IsIdentifier(prev.Type)) {
				return prev
			}
		}
	}

	fn := call.Children[0]
	if len(fn.Children) > 0 {
		// In member access / selector / field / attribute expressions, the callee method is the last child.
		return fn.Children[len(fn.Children)-1]
	}
	return fn
}

// calleeLeavesOverlap checks if two calls invoke the same callee or compatible method names.
func calleeLeavesOverlap(call1, call2 *treesitter.ASTNode, r *rules.Rules) bool {
	if call1 == nil || call2 == nil || len(call1.Children) == 0 || len(call2.Children) == 0 {
		return true
	}
	fn1 := extractCalleeNode(call1, r)
	fn2 := extractCalleeNode(call2, r)
	if fn1 == nil || fn2 == nil {
		return true
	}
	if fn1.Label != "" && fn2.Label != "" {
		return fn1.Label == fn2.Label || calleesCompatible(fn1.Label, fn2.Label)
	}
	leaves1 := fn1.Leaves()
	return slices.ContainsFunc(fn2.Leaves(), func(l2 *treesitter.ASTNode) bool {
		return l2.Label != "" && slices.ContainsFunc(leaves1, func(l1 *treesitter.ASTNode) bool {
			return l1.Label == l2.Label || calleesCompatible(l1.Label, l2.Label)
		})
	})
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
