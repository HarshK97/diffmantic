package engine

import (
	"math"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// Recover aligns unmatched nodes inside a matched container pair.
// Small subtrees (< 20 nodes) run Zhang-Shasha tree edit distance unless they
// are structured constructs (delimited containers, wrappers, blocks, calls, or assignments),
// which route to SimpleRecovery instead.
func Recover(t1, t2 *treesitter.ASTNode, m *Mapping) {
	reclaimInParentIsomorphicChildren(t1, t2, m)
	r := rulesFor(t1)
	if !shouldBypassZS(t1, t2, r) && t1.Size() < 20 && t2.Size() < 20 {
		RunZSRecovery(t1, t2, m)
	} else {
		SimpleRecovery(t1, t2, m)
	}
}

// shouldBypassZS reports whether container pair (t1, t2) should skip Zhang-Shasha
// tree edit distance and route directly to SimpleRecovery.
func shouldBypassZS(t1, t2 *treesitter.ASTNode, r *rules.Rules) bool {
	if (r != nil && (r.IsDelimitedContainer(t1.Type) || r.IsDelimitedContainer(t2.Type))) ||
		(r == nil && (rules.IsDelimitedContainer(t1.Type) || rules.IsDelimitedContainer(t2.Type))) {
		return true
	}
	if (r != nil && (r.IsWrapper(t1.Type) || r.IsWrapper(t2.Type))) ||
		(r == nil && (rules.IsWrapper(t1.Type) || rules.IsWrapper(t2.Type))) {
		return true
	}
	if findBodyBlock(t1, r) != nil || findBodyBlock(t2, r) != nil {
		return true
	}
	// Single-child nodes don't need tree edit distance; let recovery match the child directly.
	if len(t1.Children) == 1 && len(t2.Children) == 1 {
		return true
	}
	// Keep argument alignment sequential for calls to the same function.
	isCall1 := (r != nil && r.IsCall(t1.Type)) || (r == nil && rules.IsCall(t1.Type))
	isCall2 := (r != nil && r.IsCall(t2.Type)) || (r == nil && rules.IsCall(t2.Type))
	if isCall1 && isCall2 {
		c1 := extractCalleeLabel(t1, r)
		c2 := extractCalleeLabel(t2, r)
		if c1 != "" && c1 == c2 {
			return true
		}
	}
	// Assignments with matching simple identifier LHS should align LHS and RHS separately.
	isAssign1 := (r != nil && r.IsAssignment(t1.Type)) || (r == nil && rules.IsAssignment(t1.Type))
	isAssign2 := (r != nil && r.IsAssignment(t2.Type)) || (r == nil && rules.IsAssignment(t2.Type))
	if isAssign1 && isAssign2 {
		lhs1 := assignmentLHSIdent(t1)
		lhs2 := assignmentLHSIdent(t2)
		if lhs1 != "" && lhs1 == lhs2 {
			return true
		}
	}
	return false
}

func assignmentLHSIdent(n *treesitter.ASTNode) string {
	if n == nil || len(n.Children) == 0 {
		return ""
	}
	first := n.Children[0]
	if first.Label != "" {
		return first.Label
	}
	if len(first.Children) == 1 && first.Children[0].Label != "" {
		return first.Children[0].Label
	}
	return ""
}

// reclaimInParentIsomorphicChildren steals back children of t2 that TopDown
// paired with a duplicate from another block, keeping them in-place under t1.
func reclaimInParentIsomorphicChildren(t1, t2 *treesitter.ASTNode, m *Mapping) {
	if t1 == nil || t2 == nil || m == nil || m.Src()[t1] != t2 {
		return
	}
	r := rulesFor(t1)
	for _, c1 := range t1.Children {
		if m.Has(c1) || c1.IsKeyword || isGlueToken(c1, r) {
			continue
		}
		for _, c2 := range t2.Children {
			if !Isomorphic(c1, c2) {
				continue
			}
			other1 := m.Dst()[c2]
			if other1 == nil || other1.Parent == t1 {
				continue
			}
			revokeSubtreeMatchUnder(other1, c2, m)
			addIsomorphicPairs(c1, c2, m)
			break
		}
	}
}

// SimpleRecovery maps unmatched children inside container pair (t1, t2) using
// positional anchors, label/structural LCS, and unique-type matching.
func SimpleRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	matchDirectKeywords(t1, t2, m)
	reconcileDeclarationSignatures(t1, t2, m, rulesFor(t1))

	r := rulesFor(t1)
	// Pair up unmatched nodes sandwiched between matched neighbors at the same index, or trailing at the end.
	for idx, c1 := range t1.Children {
		if m.Has(c1) || idx == 0 || idx >= len(t2.Children) {
			continue
		}
		c2 := t2.Children[idx]
		if m.HasDst(c2) || !TypesMatch(c1.Type, c2.Type, r) {
			continue
		}
		left1, left2 := t1.Children[idx-1], t2.Children[idx-1]
		leftMatched := m.Src()[left1] == left2

		var rightMatched bool
		if idx+1 < min(len(t1.Children), len(t2.Children)) {
			right1, right2 := t1.Children[idx+1], t2.Children[idx+1]
			rightMatched = m.Src()[right1] == right2
		} else if idx == len(t1.Children)-1 && idx == len(t2.Children)-1 {
			rightMatched = true
		}

		if leftMatched && rightMatched {
			if c1.Label != "" && Isomorphic(c1, c2) {
				addIsomorphicPairs(c1, c2, m)
			} else if (r != nil && r.IsBlock(c1.Type)) || (r == nil && rules.IsBlock(c1.Type)) {
				m.Add(c1, c2)
				Recover(c1, c2, m)
			}
		}
	}

	matchIntroducingKeywords(t1, t2, m)

	uc1 := unmatchedChildren(t1, m.Has)
	uc2 := unmatchedChildren(t2, m.HasDst)

	for _, pair := range LCSLabel(uc1, uc2) {
		addIsomorphicPairs(pair[0], pair[1], m)
	}

	isDelimited := (r != nil && r.IsDelimitedContainer(t1.Type)) || (r == nil && rules.IsDelimitedContainer(t1.Type))
	isBlock := (r != nil && r.IsBlock(t1.Type)) || (r == nil && rules.IsBlock(t1.Type))
	if isDelimited && !isBlock {
		recoverDelimitedSiblingSequences(t1, t2, m, r)
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	if !isDelimited || isBlock {
		for _, pair := range LCSStructure(uc1, uc2) {
			addIsomorphicPairs(pair[0], pair[1], m)
		}
	}

	uc1 = unmatchedChildren(t1, m.Has)
	uc2 = unmatchedChildren(t2, m.HasDst)

	for _, pair := range uniqueTypePairs(uc1, uc2, m) {
		m.Add(pair[0], pair[1])
		Recover(pair[0], pair[1], m)
	}
}

// recoverDelimitedSiblingSequences aligns unmatched children inside non-block
// delimited containers using 2D sequence dynamic programming.
func recoverDelimitedSiblingSequences(t1, t2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) {
	uc1 := unmatchedSubstantiveChildren(t1, m.Has, r)
	uc2 := unmatchedSubstantiveChildren(t2, m.HasDst, r)
	n1, n2 := len(uc1), len(uc2)
	if n1 == 0 || n2 == 0 {
		return
	}

	dp := make([][]float64, n1+1)
	for i := range dp {
		dp[i] = make([]float64, n2+1)
	}

	sim := make([][]float64, n1)
	for i := range n1 {
		sim[i] = make([]float64, n2)
		for j := range n2 {
			if ok, score := canPairDelimitedSiblings(uc1[i], uc2[j], m, r); ok {
				sim[i][j] = score
			} else {
				sim[i][j] = -1.0
			}
		}
	}

	const gapPenalty = 0.20

	dp[0][0] = 0.0
	for i := 1; i <= n1; i++ {
		dp[i][0] = dp[i-1][0] - gapPenalty
	}
	for j := 1; j <= n2; j++ {
		dp[0][j] = dp[0][j-1] - gapPenalty
	}

	for i := 1; i <= n1; i++ {
		for j := 1; j <= n2; j++ {
			best := max(dp[i-1][j]-gapPenalty, dp[i][j-1]-gapPenalty)
			if s := sim[i-1][j-1]; s >= 0 {
				if diag := dp[i-1][j-1] + s; diag > best {
					best = diag
				}
			}
			dp[i][j] = best
		}
	}

	var matchedPairs [][2]*treesitter.ASTNode
	i, j := n1, n2
	for i > 0 && j > 0 {
		if s := sim[i-1][j-1]; s >= 0 && math.Abs(dp[i][j]-(dp[i-1][j-1]+s)) < 1e-6 {
			matchedPairs = append(matchedPairs, [2]*treesitter.ASTNode{uc1[i-1], uc2[j-1]})
			i--
			j--
		} else if math.Abs(dp[i][j]-(dp[i-1][j]-gapPenalty)) < 1e-6 {
			i--
		} else {
			j--
		}
	}
	slices.Reverse(matchedPairs)

	for _, pair := range matchedPairs {
		m.Add(pair[0], pair[1])
		Recover(pair[0], pair[1], m)
	}
}

// canPairDelimitedSiblings checks whether two unmatched sibling elements can pair,
// evaluating container isolation, callee compatibility, and leaf similarity.
func canPairDelimitedSiblings(u1, u2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) (bool, float64) {
	if u1 == nil || u2 == nil {
		return false, 0.0
	}
	if !TypesMatch(u1.Type, u2.Type, r) || !CompatiblePairRoles(u1, u2) {
		return false, 0.0
	}
	p1, p2 := u1.Parent, u2.Parent
	if m != nil && p1 != nil {
		hasConflict := slices.ContainsFunc(u2.Descendants(), func(d2 *treesitter.ASTNode) bool {
			src := m.Dst()[d2]
			return src != nil && p1.Contains(src) && !u1.Contains(src)
		})
		if hasConflict {
			return false, 0.0
		}
	}
	if m != nil && p2 != nil {
		hasConflict := slices.ContainsFunc(u1.Descendants(), func(d1 *treesitter.ASTNode) bool {
			dst := m.Src()[d1]
			return dst != nil && p2.Contains(dst) && !u2.Contains(dst)
		})
		if hasConflict {
			return false, 0.0
		}
	}

	isCall1 := (r != nil && r.IsCall(u1.Type)) || (r == nil && rules.IsCall(u1.Type))
	isCall2 := (r != nil && r.IsCall(u2.Type)) || (r == nil && rules.IsCall(u2.Type))
	if isCall1 != isCall2 {
		return false, 0.0
	}
	cScore := 1.0
	if isCall1 && isCall2 {
		callee1, rec1 := extractCalleeAndReceiver(u1, r)
		callee2, rec2 := extractCalleeAndReceiver(u2, r)

		if callee1 != "" && callee2 != "" && !calleesCompatible(callee1, callee2) {
			if hasUnmappedPeerWithCallee(u2, callee1, m, r) {
				return false, 0.0
			}
		}

		if callee1 == callee2 || calleesCompatible(callee1, callee2) {
			cScore = 1.0
		} else if rec1 != "" && rec1 == rec2 {
			cScore = 0.80
		} else {
			return false, 0.0
		}
	}

	leafSim := substantiveLeafSimilarity(u1, u2, r)
	compositeScore := 0.50*cScore + 0.50*leafSim
	if compositeScore < 0.50 {
		return false, 0.0
	}
	return true, compositeScore
}

// unmatchedSubstantiveChildren collects non-keyword, non-glue, non-punctuation children of t.
func unmatchedSubstantiveChildren(t *treesitter.ASTNode, hasFn func(*treesitter.ASTNode) bool, r *rules.Rules) []*treesitter.ASTNode {
	var out []*treesitter.ASTNode
	isPunct := func(s string) bool {
		return (r != nil && r.IsPunctuation(s)) || (r == nil && rules.IsPunctuation(s))
	}
	for _, c := range t.Children {
		if hasFn(c) || c.IsKeyword || isGlueToken(c, r) || isPunct(c.Type) || isPunct(c.Label) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func extractCalleeAndReceiver(n *treesitter.ASTNode, r *rules.Rules) (callee string, receiver string) {
	if n == nil || len(n.Children) == 0 {
		return "", ""
	}
	argIdx := -1
	for i, c := range n.Children {
		if (r != nil && r.IsDelimitedContainer(c.Type)) || (r == nil && rules.IsDelimitedContainer(c.Type)) {
			argIdx = i
			break
		}
	}
	if argIdx > 0 {
		prev := n.Children[argIdx-1]
		if (r != nil && r.IsIdentifier(prev.Type)) || (r == nil && rules.IsIdentifier(prev.Type)) {
			callee = prev.Label
			if argIdx > 1 {
				for _, c := range n.Children[0].Descendants() {
					if c.Label != "" {
						receiver = c.Label
						break
					}
				}
				if receiver == "" {
					receiver = n.Children[0].Label
				}
			}
			return callee, receiver
		}
	}
	target := n.Children[0]
	if len(target.Children) > 0 {
		for _, c := range target.Children {
			if c.Label != "" {
				if receiver == "" {
					receiver = c.Label
				}
				callee = c.Label
			}
		}
		if receiver == callee {
			receiver = ""
		}
		return callee, receiver
	}
	return target.Label, ""
}

func substantiveLeafSimilarity(a, b *treesitter.ASTNode, r *rules.Rules) float64 {
	if a == nil || b == nil {
		return 0.0
	}
	l1, l2 := a.LeafLabels(), b.LeafLabels()
	intersection, total1, total2 := 0, 0, 0
	for label, count1 := range l1 {
		if !isSubstantiveLeaf(label, r) {
			continue
		}
		total1 += count1
		if count2, ok := l2[label]; ok {
			intersection += min(count1, count2)
		}
	}
	for label, count2 := range l2 {
		if isSubstantiveLeaf(label, r) {
			total2 += count2
		}
	}
	if total1+total2 == 0 {
		return 0.0
	}
	return 2.0 * float64(intersection) / float64(total1+total2)
}

func isSubstantiveLeaf(label string, r *rules.Rules) bool {
	if !hasWordChar(label) && !isQuotedLiteral(label) {
		return false
	}
	unquoted := Unquote(label)
	if unquoted == "" {
		return false
	}
	isStop := func(s string) bool {
		if r != nil {
			return r.IsKeyword(s, s) || r.IsSentinel(s) || r.IsPunctuation(s) || r.IsOperatorLiteral(s)
		}
		return rules.IsKeyword(s, s) || rules.IsSentinel(s) || rules.IsPunctuation(s) || rules.IsOperatorLiteral(s)
	}
	if isStop(unquoted) {
		return false
	}
	switch unquoted {
	case "0", "1", "self", "this":
		return false
	}
	return true
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
			p1, p2 := n1.Parent, n2.Parent
			r := rulesFor(n1)
			if (r != nil && r.IsExpression(n1.Type)) || (r == nil && rules.IsExpression(n1.Type)) {
				if cand1 := findUnwrappedExpressionCandidate(n1, n2, m, r); cand1 != nil {
					n1 = cand1
				} else if cand2 := findUnwrappedDstExpressionCandidate(n1, n2, m, r); cand2 != nil {
					n2 = cand2
				}
			}

			hasSiblingConflict := m != nil && p1 != nil && !isBlockLikeContainer(p1) &&
				slices.ContainsFunc(n2.Descendants(), func(d2 *treesitter.ASTNode) bool {
					src := m.Dst()[d2]
					return src != nil && p1.Contains(src) && !nodes1[0].Contains(src) && src != nodes1[0]
				})
			if !hasSiblingConflict && m != nil && p2 != nil && !isBlockLikeContainer(p2) {
				hasSiblingConflict = slices.ContainsFunc(n1.Descendants(), func(d1 *treesitter.ASTNode) bool {
					dst := m.Src()[d1]
					return dst != nil && p2.Contains(dst) && !nodes2[0].Contains(dst) && dst != nodes2[0]
				})
			}
			if hasSiblingConflict {
				continue
			}

			isDecl := (r != nil && r.IsDeclaration(n1.Type)) || (r == nil && rules.IsDeclaration(n1.Type))
			isStmt := ((r != nil && r.IsStatement(n1.Type)) || (r == nil && rules.IsStatement(n1.Type))) && !isDecl
			diceSrc := 0.0
			if m != nil {
				diceSrc = m.DiceSrc(n1, n2)
			}
			// Skip body-bearing statements (like if/for) when headers don't match and no children are mapped yet.
			hasBodyBlock := findBodyBlock(n1, r) != nil && findBodyBlock(n2, r) != nil
			if isStmt && hasBodyBlock && !isomorphicHeader(n1, n2, r) && !labelIsomorphic(n1, n2) && diceSrc == 0.0 {
				continue
			}
			isCall := (r != nil && r.IsCall(n1.Type)) || (r == nil && rules.IsCall(n1.Type))
			isDelimitedContainer := p1 != nil && ((r != nil && r.IsDelimitedContainer(p1.Type)) || (r == nil && rules.IsDelimitedContainer(p1.Type)))
			if isDelimitedContainer && isCall {
				callee1, rec1 := extractCalleeAndReceiver(n1, r)
				callee2, rec2 := extractCalleeAndReceiver(n2, r)
				if !calleesCompatible(callee1, callee2) && (rec1 == "" || rec1 != rec2) {
					continue
				}
			}
			if len(n1.Children) > 0 && len(n2.Children) > 0 {
				labels1 := n1.LeafLabels()
				labels2 := n2.LeafLabels()
				isDeclaration := isDecl
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
					isContainer := isDeclaration || hasBlockDescendant(n1, r) || hasBlockDescendant(n2, r)

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
					tag1 := getTagName(n1)
					tag2 := getTagName(n2)
					if tag1 != "" && tag2 != "" && tag1 != tag2 {
						continue
					}
					if semanticOverlap == 0 && (isDeclaration || tag1 != "" || tag2 != "") {
						continue
					}
				}
			}
			pairs = append(pairs, [2]*treesitter.ASTNode{n1, n2})
		}
	}
	return pairs
}

// hasBlockDescendant reports whether n or any descendant in its subtree is a block node.
func hasBlockDescendant(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if isBlockNode(n, r) {
		return true
	}
	if n.Index != nil && n.PreSize > 0 {
		for _, d := range n.Index.Nodes[n.ID+1 : n.ID+n.PreSize] {
			if isBlockNode(d, r) {
				return true
			}
		}
		return false
	}
	for _, c := range n.Children {
		if hasBlockDescendant(c, r) {
			return true
		}
	}
	return false
}

// isExpressionUnwrapBoundary reports whether c is a block, closure, statement, or
// declaration boundary that expression unwrapping must not descend into.
func isExpressionUnwrapBoundary(c *treesitter.ASTNode, r *rules.Rules) bool {
	if c == nil {
		return false
	}
	if r == nil {
		r = rulesFor(c)
	}
	if r != nil {
		return r.IsBlock(c.Type) || r.IsClosure(c.Type) || r.IsStatement(c.Type) || r.IsDeclaration(c.Type)
	}
	return rules.IsBlock(c.Type) || rules.IsClosure(c.Type) || rules.IsStatement(c.Type) || rules.IsDeclaration(c.Type)
}

// isBlockLikeContainer reports whether p is a block, declaration, or file root
// where multiple sibling constructs naturally coexist without sibling absorption conflicts.
func isBlockLikeContainer(p *treesitter.ASTNode) bool {
	if p == nil {
		return false
	}
	pr := rulesFor(p)
	if pr != nil {
		return pr.IsBlock(p.Type) || pr.IsDeclaration(p.Type) || p.Type == pr.DefaultRootType()
	}
	return rules.IsBlock(p.Type) || rules.IsDeclaration(p.Type)
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
	mismatchFn := func(d *treesitter.ASTNode) bool { return hasCrossSideOperatorMismatch(d, target, m, r) }
	return findBestExpressionMatch(compound, target, r, hasFn, typesMatchFn, commonFn, mismatchFn)
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
	mismatchFn := func(d *treesitter.ASTNode) bool { return hasCrossSideOperatorMismatch(target, d, m, r) }
	return findBestExpressionMatch(compound, target, r, hasFn, typesMatchFn, commonFn, mismatchFn)
}

// findBestExpressionMatch picks the descendant in compound that best matches
// target by leaf overlap. It skips mapped nodes, rejects structural mismatches,
// and boosts candidates that already share mapped nodes with target.
func findBestExpressionMatch(
	compound, target *treesitter.ASTNode,
	r *rules.Rules,
	hasFn func(*treesitter.ASTNode) bool,
	typesMatchFn func(*treesitter.ASTNode) bool,
	commonFn func(*treesitter.ASTNode) bool,
	mismatchFn func(*treesitter.ASTNode) bool,
) *treesitter.ASTNode {
	if compound == nil || target == nil {
		return nil
	}
	targetOp := getOperatorNode(target, r)
	compoundOp := getOperatorNode(compound, r)

	// Only unwrap compound expressions that actually have an operator.
	if targetOp == nil || compoundOp == nil {
		return nil
	}

	// If compound shares target's operator and isn't deeper,
	// compound itself is already the closest possible match.
	if targetOp.Label == compoundOp.Label && Height(compound) <= Height(target) {
		return nil
	}

	targetLabels := target.LeafLabels()
	var bestCandidate *treesitter.ASTNode
	bestScore := 0.0

	var search func(curr *treesitter.ASTNode)
	search = func(curr *treesitter.ASTNode) {
		for _, d := range curr.Children {
			if isExpressionUnwrapBoundary(d, r) {
				continue
			}
			if !hasFn(d) && typesMatchFn(d) {
				if mismatchFn != nil && mismatchFn(d) {
					continue
				}
				dOp := getOperatorNode(d, r)
				if targetOp == nil || (dOp != nil && dOp.Label == targetOp.Label) {
					dLabels := d.LeafLabels()
					overlap := 0
					totalD := 0
					for k, vD := range dLabels {
						totalD += vD
						if vT, ok := targetLabels[k]; ok {
							overlap += min(vD, vT)
						}
					}
					if overlap > 0 {
						score := float64(overlap) / float64(max(totalD, len(targetLabels)))
						if commonFn(d) {
							score += 10.0
						}
						if score > bestScore {
							bestScore = score
							bestCandidate = d
						}
					}
				}
			}
			search(d)
		}
	}
	search(compound)

	return bestCandidate
}

// matchIntroducingKeywords pairs keywords like "else" or "catch" that sit
// immediately before an already-matched sibling block, even when re-nesting
// changed one side's parent container.
func matchIntroducingKeywords(t1, t2 *treesitter.ASTNode, m *Mapping) {
	for idx2, c2 := range t2.Children {
		if idx2 == 0 {
			continue
		}
		c1 := m.Dst()[c2]
		if c1 == nil || c1.Parent == nil {
			continue
		}
		idx1 := c1.ChildIndex()
		if idx1 == 0 {
			continue
		}
		prev1 := c1.Parent.Children[idx1-1]
		prev2 := t2.Children[idx2-1]
		if !m.Has(prev1) && !m.HasDst(prev2) && prev1.Label == prev2.Label && prev1.Type == prev2.Type && (prev1.IsKeyword || prev2.IsKeyword) {
			m.Add(prev1, prev2)
		}
	}
	for idx1, c1 := range t1.Children {
		if idx1 == 0 {
			continue
		}
		c2 := m.Src()[c1]
		if c2 == nil || c2.Parent == nil {
			continue
		}
		idx2 := c2.ChildIndex()
		if idx2 == 0 {
			continue
		}
		prev1 := t1.Children[idx1-1]
		prev2 := c2.Parent.Children[idx2-1]
		if !m.Has(prev1) && !m.HasDst(prev2) && prev1.Label == prev2.Label && prev1.Type == prev2.Type && (prev1.IsKeyword || prev2.IsKeyword) {
			m.Add(prev1, prev2)
		}
	}
}

// isomorphicHeader checks if two statements share matching headers or conditions,
// ignoring their executable body blocks.
func isomorphicHeader(n1, n2 *treesitter.ASTNode, rOpt ...*rules.Rules) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	var r *rules.Rules
	if len(rOpt) > 0 {
		r = rOpt[0]
	}
	if r == nil {
		r = rulesFor(n1)
	}
	body1 := findBodyBlock(n1, r)
	body2 := findBodyBlock(n2, r)
	if body1 == nil || body2 == nil {
		return false
	}
	var h1, h2 []*treesitter.ASTNode
	for _, c := range n1.Children {
		if c != body1 && !isBlockNode(c, r) {
			h1 = append(h1, c)
		}
	}
	for _, c := range n2.Children {
		if c != body2 && !isBlockNode(c, r) {
			h2 = append(h2, c)
		}
	}
	if len(h1) == 0 || len(h1) != len(h2) {
		return false
	}
	hasSemantic := false
	for i := range h1 {
		if !Isomorphic(h1[i], h2[i]) {
			return false
		}
		isPunct := (r != nil && (r.IsPunctuation(h1[i].Type) || r.IsOperatorLiteral(h1[i].Type))) ||
			(r == nil && (rules.IsPunctuation(h1[i].Type) || rules.IsOperatorLiteral(h1[i].Type)))
		if !h1[i].IsKeyword && !isPunct {
			hasSemantic = true
		}
	}
	return hasSemantic
}

// labelIsomorphic checks if two nodes share the same label or identical AST structure.
func labelIsomorphic(n1, n2 *treesitter.ASTNode) bool {
	if n1 == nil || n2 == nil {
		return false
	}
	if n1.Label != "" && n2.Label != "" {
		return n1.Label == n2.Label
	}
	return Isomorphic(n1, n2)
}
