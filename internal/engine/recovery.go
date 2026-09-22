package engine

import (
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// Recover aligns unmatched nodes inside a matched container pair.
// Uses Zhang-Shasha (1989) tree edit distance as a last-chance fallback on tiny
// subtrees (< 20 nodes), and uses SimpleRecovery (Sibling LCS) on larger subtrees.
func Recover(t1, t2 *treesitter.ASTNode, m *Mapping) {
	if t1.Size() < 20 && t2.Size() < 20 {
		RunZSRecovery(t1, t2, m)
	} else {
		SimpleRecovery(t1, t2, m)
	}
}

// SimpleRecovery maps unmatched children inside container pair (t1, t2) using
// positional anchors, label/structural LCS, and unique-type matching.
func SimpleRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	// If an unmatched node is sandwiched between already-matched neighbors at the exact same index, pair it up.
	for idx, c1 := range t1.Children {
		if m.Has(c1) {
			continue
		}
		if idx > 0 && idx+1 < len(t1.Children) && idx+1 < len(t2.Children) {
			c2 := t2.Children[idx]
			if !m.HasDst(c2) && c1.Label != "" && Isomorphic(c1, c2) {
				left1, left2 := t1.Children[idx-1], t2.Children[idx-1]
				right1, right2 := t1.Children[idx+1], t2.Children[idx+1]
				if m.Src()[left1] == left2 && m.Src()[right1] == right2 {
					addIsomorphicPairs(c1, c2, m)
				}
			}
		}
	}

	uc1 := unmatchedChildren(t1, m.Has)
	uc2 := unmatchedChildren(t2, m.HasDst)

	for _, pair := range LCSLabel(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	for _, pair := range LCSStructure(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	for _, pair := range uniqueTypePairs(uc1, uc2, m) {
		m.Add(pair[0], pair[1])
		Recover(pair[0], pair[1], m)
	}
}

func unmatchedChildren(t *treesitter.ASTNode, hasFn func(*treesitter.ASTNode) bool) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	for _, c := range t.Children {
		if !hasFn(c) {
			out = append(out, c)
		}
	}
	return out
}

func uniqueTypePairs(
	uc1, uc2 []*treesitter.ASTNode,
	m *Mapping,
) [][2]*treesitter.ASTNode {
	count1 := make(map[string][]*treesitter.ASTNode)
	count2 := make(map[string][]*treesitter.ASTNode)
	for _, c := range uc1 {
		count1[c.Type] = append(count1[c.Type], c)
	}
	for _, c := range uc2 {
		count2[c.Type] = append(count2[c.Type], c)
	}

	seen := make(map[string]bool)
	var pairs [][2]*treesitter.ASTNode
	for _, c := range uc1 {
		typ := c.Type
		if seen[typ] {
			continue
		}
		seen[typ] = true
		nodes1 := count1[typ]
		nodes2 := count2[typ]
		if len(nodes1) == 1 && len(nodes2) == 1 && CompatiblePairRoles(nodes1[0], nodes2[0]) {
			n1, n2 := nodes1[0], nodes2[0]
			r := rulesFor(n1)
			if (r != nil && r.IsExpression(n1.Type)) || (r == nil && rules.IsExpression(n1.Type)) {
				if cand1 := findUnwrappedExpressionCandidate(n1, n2, m, r); cand1 != nil {
					n1 = cand1
				} else if cand2 := findUnwrappedDstExpressionCandidate(n1, n2, m, r); cand2 != nil {
					n2 = cand2
				}
			}
			if len(n1.Children) > 0 && len(n2.Children) > 0 {
				labels1 := n1.LeafLabels()
				labels2 := n2.LeafLabels()
				isDeclaration := (r != nil && r.IsDeclaration(n1.Type)) || (r == nil && rules.IsDeclaration(n1.Type))
				isWrapper := ((r != nil && r.IsWrapper(n1.Type)) || (r == nil && rules.IsWrapper(n1.Type))) && !isDeclaration
				if !isWrapper {
					overlap := 0
					semanticOverlap := 0
					total1 := 0
					for k, v1 := range labels1 {
						total1 += v1
						if v2, ok := labels2[k]; ok {
							matched := min(v1, v2)
							overlap += matched
							isKw := (r != nil && r.IsKeyword(k, k)) || (r == nil && rules.IsKeyword(k, k))
							isPunct := (r != nil && (r.IsPunctuation(k) || r.IsOperatorLiteral(k))) ||
								(r == nil && (rules.IsPunctuation(k) || rules.IsOperatorLiteral(k)))
							if !isKw && !isPunct && k != "=" && k != ":=" && k != "*" && k != "&" {
								semanticOverlap += matched
							}
						}
					}
					total2 := 0
					for _, v2 := range labels2 {
						total2 += v2
					}
					isContainer := isDeclaration || (r != nil && r.IsBlock(n1.Type)) || (r == nil && rules.IsBlock(n1.Type))
					if !isContainer {
						for _, child := range n1.Children {
							if (r != nil && r.IsBlock(child.Type)) || (r == nil && rules.IsBlock(child.Type)) {
								isContainer = true
								break
							}
						}
					}

					minRatio := 0.25
					var ratio float64
					if isContainer {
						denom := total1 + total2
						if denom == 0 {
							continue
						}
						minRatio = 0.50
						ratio = float64(2*overlap) / float64(denom)
					} else {
						denom := min(total1, total2)
						if denom == 0 {
							continue
						}
						ratio = float64(overlap) / float64(denom)
					}
					if ratio < minRatio {
						continue
					}
					if isDeclaration && semanticOverlap == 0 {
						continue
					}
				}
			}
			pairs = append(pairs, [2]*treesitter.ASTNode{n1, n2})
		}
	}
	return pairs
}

// findUnwrappedExpressionCandidate searches compound for an inner expression
// that matches target better than compound's root. Without this, uniqueTypePairs
// binds to the outer || or && instead of the specific arm being checked.
func findUnwrappedExpressionCandidate(
	compound, target *treesitter.ASTNode,
	m *Mapping,
	r *rules.Rules,
) *treesitter.ASTNode {
	hasFn := func(d *treesitter.ASTNode) bool { return m != nil && m.Has(d) }
	typesMatchFn := func(d *treesitter.ASTNode) bool {
		return TypesMatch(d.Type, target.Type, r) && CompatiblePairRoles(d, target)
	}
	commonFn := func(d *treesitter.ASTNode) bool { return m != nil && hasCommonDescendant(d, target, m) }
	return findBestExpressionMatch(compound, target, r, hasFn, typesMatchFn, commonFn)
}

// findUnwrappedDstExpressionCandidate mirrors findUnwrappedExpressionCandidate
// when the compound expression is on dst instead of src.
func findUnwrappedDstExpressionCandidate(
	target, compound *treesitter.ASTNode,
	m *Mapping,
	r *rules.Rules,
) *treesitter.ASTNode {
	hasFn := func(d *treesitter.ASTNode) bool { return m != nil && m.HasDst(d) }
	typesMatchFn := func(d *treesitter.ASTNode) bool {
		return TypesMatch(target.Type, d.Type, r) && CompatiblePairRoles(target, d)
	}
	commonFn := func(d *treesitter.ASTNode) bool { return m != nil && hasCommonDescendant(target, d, m) }
	return findBestExpressionMatch(compound, target, r, hasFn, typesMatchFn, commonFn)
}

// findBestExpressionMatch picks the descendant in compound that best matches
// target by leaf overlap. It skips mapped nodes, checks type/role compatibility,
// and gives a heavy score boost if an inner node is already mapped to target.
func findBestExpressionMatch(
	compound, target *treesitter.ASTNode,
	r *rules.Rules,
	hasFn func(*treesitter.ASTNode) bool,
	typesMatchFn func(*treesitter.ASTNode) bool,
	commonFn func(*treesitter.ASTNode) bool,
) *treesitter.ASTNode {
	if compound == nil || target == nil {
		return nil
	}
	targetOp := getOperatorNode(target, r)
	compoundOp := getOperatorNode(compound, r)

	// If compound shares target's operator and isn't deeper,
	// compound itself is already the closest possible match.
	if targetOp != nil && compoundOp != nil && targetOp.Label == compoundOp.Label && Height(compound) <= Height(target) {
		return nil
	}

	targetLabels := target.LeafLabels()
	var bestCandidate *treesitter.ASTNode
	bestScore := 0.0

	for _, d := range compound.Descendants() {
		if d == compound || hasFn(d) {
			continue
		}
		if !typesMatchFn(d) {
			continue
		}
		dOp := getOperatorNode(d, r)
		if targetOp != nil {
			if dOp == nil || dOp.Label != targetOp.Label {
				continue
			}
		}

		dLabels := d.LeafLabels()
		overlap := 0
		totalD := 0
		for k, vD := range dLabels {
			totalD += vD
			if vT, ok := targetLabels[k]; ok {
				overlap += min(vD, vT)
			}
		}
		if overlap == 0 {
			continue
		}

		score := float64(overlap) / float64(max(totalD, len(targetLabels)))
		if commonFn(d) {
			score += 10.0
		}

		if score > bestScore {
			bestScore = score
			bestCandidate = d
		}
	}

	return bestCandidate
}
