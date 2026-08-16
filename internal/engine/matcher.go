package engine

import (
	"cmp"
	"fmt"
	"io"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode, srcA, srcB []byte, part *LinePartition) *MatchResult {
	mappings := NewMapping()

	if part == nil {
		part = NewLinePartition(srcA, srcB)
	}

	minHeight := 2
	minDice := 0.5

	// Match AST nodes top-down, by declaration, and bottom-up using line partitioning.
	TopDown(t1, t2, minHeight, mappings, part)
	matchDeclarations(t1, t2, mappings)
	matchPairValues(t1, t2, mappings)
	BottomUp(t1, t2, mappings, minDice)
	ContestContainers(t1, t2, mappings)

	MatchUnmatchedLeaves(t1, t2, mappings, part)
	RollupMatchedContainers(t1, t2, mappings)
	MatchContainerKeywords(t1, t2, mappings)

	if !mappings.Has(t1) && !mappings.HasDst(t2) {
		mappings.Add(t1, t2)
	}

	sortMappingsByPreOrder(mappings)

	return &MatchResult{Mappings: mappings}
}

// MatchUnmatchedLeaves pairs unmatched leaf nodes of the same type and label using
// parent Dice similarity and positional scores to break ties. Leaves under unmatched
// parents are allowed if they share a matched ancestor.
func MatchUnmatchedLeaves(t1Root, t2Root *treesitter.ASTNode, m *Mapping, part *LinePartition) {
	type leafKey struct{ Type, Label string }
	t2Leaves := make(map[leafKey][]*treesitter.ASTNode)
	for _, t2 := range t2Root.PostOrder() {
		if len(t2.Children) == 0 && t2.Label != "" && !t2.IsKeyword {
			t2Leaves[leafKey{t2.Type, t2.Label}] = append(t2Leaves[leafKey{t2.Type, t2.Label}], t2)
		}
	}

	type parentPair struct{ p1, p2 *treesitter.ASTNode }
	diceCache := make(map[parentPair]float64)
	for _, t1 := range t1Root.PostOrder() {
		if m.Has(t1) || len(t1.Children) > 0 || t1.Label == "" || t1.IsKeyword {
			continue
		}

		anc1 := NearestMatchedAncestor(t1, m, false)
		if anc1 == nil {
			continue
		}

		candidates := t2Leaves[leafKey{t1.Type, t1.Label}]
		if len(candidates) == 0 {
			continue
		}

		var bestT2 *treesitter.ASTNode
		bestDice := 0.0
		bestPosScore := -1

		t1Idx := t1.ChildIndex()

		for _, t2 := range candidates {
			if m.HasDst(t2) {
				continue
			}
			if part != nil && !part.CanMatch(t1, t2) {
				continue
			}
			anc1, anc2 := SharedMatchedAncestor(t1, t2, m)
			if anc1 == nil || anc2 == nil {
				continue
			}

			parentMatched := t1.Parent != nil && t2.Parent != nil && m.Src()[t1.Parent] == t2.Parent

			samePositional := false
			if t1.Parent != nil && t2.Parent != nil {
				samePositional = t1Idx == t2.ChildIndex()
			}

			parentPositional := false
			if anc1 != nil && anc2 != nil &&
				t1.Parent != nil && t2.Parent != nil &&
				anc1 != t1.Parent && anc2 != t2.Parent {
				p1Idx := t1.Parent.ChildIndex()
				p2Idx := t2.Parent.ChildIndex()
				if p1Idx >= 0 && p2Idx >= 0 {
					parentPositional = p1Idx == p2Idx
				}
			}

			siblingScore := 0
			if t1.Parent != nil && t2.Parent != nil && t1Idx >= 0 {
				if t2Idx := t2.ChildIndex(); t2Idx >= 0 {
					for _, offset := range []int{-1, 1} {
						i1, i2 := t1Idx+offset, t2Idx+offset
						if i1 >= 0 && i1 < len(t1.Parent.Children) && i2 >= 0 && i2 < len(t2.Parent.Children) {
							if m.Src()[t1.Parent.Children[i1]] == t2.Parent.Children[i2] {
								siblingScore += 500
							}
						}
					}
				}
			}

			posScore := 1
			if parentMatched {
				posScore += 1000
			}
			if parentPositional {
				posScore += 100
			}
			if samePositional {
				posScore += 10
			}
			posScore += siblingScore

			if posScore >= bestPosScore {
				d := 0.0
				if t1.Parent != nil && t2.Parent != nil {
					pair := parentPair{p1: t1.Parent, p2: t2.Parent}
					if cached, ok := diceCache[pair]; ok {
						d = cached
					} else {
						d = Dice(t1.Parent, t2.Parent, m.Src())
						diceCache[pair] = d
					}
				}

				depth1 := t1.DepthTo(anc1)
				depth2 := t2.DepthTo(anc2)
				if !parentMatched && siblingScore == 0 && d < 0.25 && (depth1 > 2 || depth2 > 2) {
					continue
				}

				if posScore > bestPosScore || d > bestDice {
					bestDice = d
					bestT2 = t2
					bestPosScore = posScore
				}
			}
		}

		if bestT2 != nil {
			m.Add(t1, bestT2)
		}
	}
}

// MatchContainerKeywords maps unmapped keyword children between mapped parent containers.
func MatchContainerKeywords(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	for _, p1 := range t1Root.PostOrder() {
		p2 := m.Src()[p1]
		if p2 == nil {
			continue
		}
		for _, c1 := range p1.Children {
			if !m.Has(c1) {
				if c1.IsKeyword {
					k2 := findMatchingKeywordChild(p1, p2, c1, m)
					if k2 != nil {
						m.Add(c1, k2)
					}
				} else if c1.IsScaffolding() && len(c1.Children) == 1 && c1.Children[0].IsKeyword {
					k2 := findMatchingScaffoldKeywordChild(p1, p2, c1, m)
					if k2 != nil {
						m.Add(c1, k2)
						m.Add(c1.Children[0], k2.Children[0])
					}
				}
			}
		}
	}
}

func findMatchingKeywordChild(p1, p2 *treesitter.ASTNode, k1 *treesitter.ASTNode, m *Mapping) *treesitter.ASTNode {
	if p1 == nil || p2 == nil || k1 == nil {
		return nil
	}
	k1Idx := k1.ChildIndex()
	var fallback *treesitter.ASTNode
	for _, c2 := range p2.Children {
		if c2.IsKeyword && c2.Type == k1.Type && !m.HasDst(c2) {
			if !isKeywordClauseConsistent(p1, p2, k1, c2, m) {
				continue
			}
			if c2.ChildIndex() == k1Idx {
				return c2
			}
			if fallback == nil {
				fallback = c2
			}
		}
	}
	return fallback
}

func findMatchingScaffoldKeywordChild(p1, p2 *treesitter.ASTNode, s1 *treesitter.ASTNode, m *Mapping) *treesitter.ASTNode {
	if p1 == nil || p2 == nil || s1 == nil || len(s1.Children) != 1 {
		return nil
	}
	k1 := s1.Children[0]
	s1Idx := s1.ChildIndex()
	var fallback *treesitter.ASTNode
	for _, c2 := range p2.Children {
		if c2.Type == s1.Type && len(c2.Children) == 1 && c2.Children[0].IsKeyword && c2.Children[0].Type == k1.Type && !m.HasDst(c2) && !m.HasDst(c2.Children[0]) {
			if !isKeywordClauseConsistent(p1, p2, s1, c2, m) {
				continue
			}
			if c2.ChildIndex() == s1Idx {
				return c2
			}
			if fallback == nil {
				fallback = c2
			}
		}
	}
	return fallback
}

func isKeywordClauseConsistent(p1, p2, k1, c2 *treesitter.ASTNode, m *Mapping) bool {
	if p1 == nil || p2 == nil || k1 == nil || c2 == nil {
		return false
	}
	k1Idx := k1.ChildIndex()
	c2Idx := c2.ChildIndex()

	var prev1, next1 *treesitter.ASTNode
	for i := k1Idx - 1; i >= 0; i-- {
		if !p1.Children[i].IsKeyword && !p1.Children[i].IsBracketOrParen() {
			prev1 = p1.Children[i]
			break
		}
	}
	for i := k1Idx + 1; i < len(p1.Children); i++ {
		if !p1.Children[i].IsKeyword && !p1.Children[i].IsBracketOrParen() {
			next1 = p1.Children[i]
			break
		}
	}

	var prev2, next2 *treesitter.ASTNode
	for i := c2Idx - 1; i >= 0; i-- {
		if !p2.Children[i].IsKeyword && !p2.Children[i].IsBracketOrParen() {
			prev2 = p2.Children[i]
			break
		}
	}
	for i := c2Idx + 1; i < len(p2.Children); i++ {
		if !p2.Children[i].IsKeyword && !p2.Children[i].IsBracketOrParen() {
			next2 = p2.Children[i]
			break
		}
	}

	// If k1 is preceded by an expression in p1 and c2 is preceded by an expression in p2,
	// ensure they share mapping overlap.
	if prev1 != nil && prev2 != nil {
		if hasMappedDescendant(prev1, m) && hasMappedDescendant(prev2, m) {
			if !hasSubtreeMappingOverlap(prev1, prev2, m) {
				return false
			}
		}
	}

	// If k1 is followed by an expression/statement in p1 and c2 is followed by an expression/statement in p2,
	// ensure they share mapping overlap.
	if next1 != nil && next2 != nil {
		if hasMappedDescendant(next1, m) && hasMappedDescendant(next2, m) {
			if !hasSubtreeMappingOverlap(next1, next2, m) {
				return false
			}
		}
	}

	return true
}

func hasMappedDescendant(n *treesitter.ASTNode, m *Mapping) bool {
	if n == nil || m == nil {
		return false
	}
	if m.Has(n) || m.HasDst(n) {
		return true
	}
	for _, l := range n.Leaves() {
		if m.Has(l) || m.HasDst(l) {
			return true
		}
	}
	return false
}

func hasSubtreeMappingOverlap(n1, n2 *treesitter.ASTNode, m *Mapping) bool {
	if n1 == nil || n2 == nil || m == nil {
		return false
	}
	if m.Src()[n1] == n2 || m.Dst()[n2] == n1 {
		return true
	}
	for _, l1 := range n1.Leaves() {
		if dst := m.Src()[l1]; dst != nil {
			for curr := dst; curr != nil; curr = curr.Parent {
				if curr == n2 {
					return true
				}
			}
		}
	}
	return false
}

// sortMappingsByPreOrder sorts mapped pairs by T1 pre-order index.
func sortMappingsByPreOrder(m *Mapping) {
	slices.SortStableFunc(m.Pairs, func(a, b MappingPair) int {
		return cmp.Compare(a.Src.ID, b.Src.ID)
	})
}

func FprintMappings(w io.Writer, r *MatchResult) error {
	if r == nil || r.Mappings == nil {
		_, err := fmt.Fprintln(w, "(no mappings)")
		return err
	}

	pairs := r.Mappings.Pairs
	if len(pairs) == 0 {
		_, err := fmt.Fprintln(w, "(no mappings found)")
		return err
	}
	if _, err := fmt.Fprintf(w, "%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────────────────────────"); err != nil {
		return err
	}
	for i, p := range pairs {
		t1Label := p.Src.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := p.Dst.Label
		if t2Label == "" {
			t2Label = "-"
		}
		if _, err := fmt.Fprintf(w, "%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i+1, p.Src.Type, t1Label, p.Dst.Type, t2Label); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nTotal mappings: %d\n", len(pairs))
	return err
}

func matchDeclarations(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	t1Decs := findDeclarations(t1Root)
	t2Decs := findDeclarations(t2Root)

	for _, d1 := range t1Decs {
		if m.Has(d1) {
			continue
		}
		name1 := getDeclarationName(d1)
		if name1 == "" {
			continue
		}
		rec1 := getReceiverTypeName(d1)

		var bestMatch *treesitter.ASTNode
		matchCount := 0

		for _, d2 := range t2Decs {
			if m.HasDst(d2) {
				continue
			}
			if d2.Type == d1.Type && getDeclarationName(d2) == name1 && getReceiverTypeName(d2) == rec1 {
				bestMatch = d2
				matchCount++
			}
		}

		if matchCount == 1 {
			m.Add(d1, bestMatch)
		}
	}
}

func findDeclarations(root *treesitter.ASTNode) []*treesitter.ASTNode {
	if root == nil {
		return nil
	}
	lang := root.GetLanguage()
	if lang == "" {
		return nil
	}
	r := treesitter.GetRules(lang)
	if r == nil || len(r.Declarations) == 0 {
		return nil
	}
	decMap := make(map[string]bool, len(r.Declarations))
	for _, d := range r.Declarations {
		decMap[d] = true
	}

	var decs []*treesitter.ASTNode
	var traverse func(*treesitter.ASTNode)
	traverse = func(n *treesitter.ASTNode) {
		if n == nil {
			return
		}
		if decMap[n.Type] {
			decs = append(decs, n)
		}
		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(root)
	return decs
}

func getDeclarationName(n *treesitter.ASTNode) string {
	if n == nil {
		return ""
	}
	var idTypes []string
	lang := n.GetLanguage()
	if lang != "" {
		if r := treesitter.GetRules(lang); r != nil {
			if len(r.Declarations) > 0 && !slices.Contains(r.Declarations, n.Type) {
				return ""
			}
			if len(r.Identifiers) > 0 {
				idTypes = r.Identifiers
			}
		}
	}
	if len(idTypes) == 0 {
		idTypes = []string{"identifier", "field_identifier", "type_identifier", "name", "tag_name"}
	}

	for _, child := range n.Children {
		if slices.Contains(idTypes, child.Type) {
			if child.Label != "" {
				return child.Label
			}
		}
	}
	return ""
}

func getReceiverTypeName(n *treesitter.ASTNode) string {
	if n == nil || n.Type != "method_declaration" {
		return ""
	}
	for _, child := range n.Children {
		if child.Type == "parameter_list" {
			for _, p := range child.Children {
				if p.Type == "parameter_declaration" {
					for _, t := range p.Children {
						if t.Type == "type_identifier" {
							return t.Label
						}
						if t.Type == "pointer_type" {
							for _, pt := range t.Children {
								if pt.Type == "type_identifier" {
									return pt.Label
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}

func getParentPairKey(n *treesitter.ASTNode) string {
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		if label := getKeyLabel(curr.Parent); label != "" {
			return label
		}
	}
	return ""
}

func matchPairValues(t1, t2 *treesitter.ASTNode, m *Mapping) {
	if t1 == nil || t1.Language == "" {
		return
	}
	r := treesitter.GetRules(t1.Language)
	if r == nil || len(r.Pairs) == 0 {
		return
	}

	for _, n1 := range t1.PostOrder() {
		if !slices.Contains(r.Pairs, n1.Type) {
			continue
		}

		var n2 *treesitter.ASTNode
		if m.Has(n1) {
			n2 = m.Src()[n1]
			if n2 == nil || !slices.Contains(r.Pairs, n2.Type) {
				continue
			}
		} else {
			key1 := getKeyLabel(n1)
			if key1 == "" {
				continue
			}
			anc1 := NearestMatchedAncestor(n1, m, false)
			pKey1 := getParentPairKey(n1)

			for _, cand := range t2.PostOrder() {
				if !slices.Contains(r.Pairs, cand.Type) || m.HasDst(cand) {
					continue
				}
				if getKeyLabel(cand) != key1 {
					continue
				}
				anc2 := NearestMatchedAncestor(cand, m, true)
				if !areAncestorsMatched(anc1, anc2, m) {
					continue
				}

				pKey2 := getParentPairKey(cand)
				if pKey1 != pKey2 {
					continue
				}

				n2 = cand
				m.Add(n1, n2)
				m.Add(n1.Children[0], n2.Children[0])
				break
			}
		}

		if n2 != nil && len(n1.Children) >= 2 && len(n2.Children) >= 2 {
			val1 := n1.Children[len(n1.Children)-1]
			val2 := n2.Children[len(n2.Children)-1]
			if !m.Has(val1) && !m.HasDst(val2) && len(val1.Children) > 0 && len(val2.Children) > 0 && val1.Type == val2.Type {
				m.Add(val1, val2)
			}
		}
	}
}
