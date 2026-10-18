package postprocess

import (
	"cmp"
	"slices"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// isDelimitedOrBlockContainer reports whether n is a block, wrapper, or delimited container.
func isDelimitedOrBlockContainer(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r != nil {
		return r.IsWrapper(n.Type) || r.IsBlock(n.Type) || r.IsDelimitedContainer(n.Type)
	}
	return rules.IsWrapper(n.Type) || rules.IsBlock(n.Type) || rules.IsDelimitedContainer(n.Type)
}

// isPayloadLeaf reports whether d is a content leaf. Jump keywords (return,
// break, continue) count as payload so bare jumps contribute to their
// enclosing block's retention.
func isPayloadLeaf(d *treesitter.ASTNode, r *rules.Rules) bool {
	if d == nil || len(d.Children) > 0 {
		return false
	}
	if (r != nil && (r.IsPunctuation(d.Type) || r.IsOperatorLiteral(d.Type))) ||
		(r == nil && (rules.IsPunctuation(d.Type) || rules.IsOperatorLiteral(d.Type))) {
		return false
	}
	isJumpParent := d.Parent != nil && ((r != nil && r.IsJumpStatement(d.Parent.Type)) || (r == nil && rules.IsJumpStatement(d.Parent.Type)))
	isKw := d.IsKeyword || (r != nil && r.IsKeyword(d.Type, d.Label)) || (r == nil && rules.IsKeyword(d.Type, d.Label))
	if isJumpParent && isKw {
		return true
	}
	if isKw {
		return false
	}
	return true
}

type leafSummary struct {
	total   int
	exact   int
	updated int
}

// summarizeMappedLeaves counts payload leaves in src and measures how many survived in dst.
func summarizeMappedLeaves(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) leafSummary {
	var s leafSummary
	if src == nil || ms == nil {
		return s
	}
	for _, d := range src.Descendants() {
		if !isPayloadLeaf(d, r) {
			continue
		}
		s.total++
		if _, isEv := evicted[d]; isEv {
			continue
		}
		if dDst, ok := ms.Src()[d]; ok {
			if dst != nil && !dst.Contains(dDst) {
				continue
			}
			if d.Label == dDst.Label {
				s.exact++
			} else {
				s.updated++
			}
		}
	}
	return s
}

// hasSurvivingMappedLeaves reports whether from has any payload leaf mapped to
// an unevicted descendant of to, short-circuiting on the first match.
func hasSurvivingMappedLeaves(from, to *treesitter.ASTNode, mapping map[*treesitter.ASTNode]*treesitter.ASTNode, evicted map[*treesitter.ASTNode]struct{}, r *rules.Rules) bool {
	if from == nil || to == nil || mapping == nil {
		return false
	}
	nodes := from.Descendants()
	if from.Index != nil && from.PreSize > 0 {
		nodes = from.Index.Nodes[from.ID+1 : from.ID+from.PreSize]
	}
	for _, d := range nodes {
		if !isPayloadLeaf(d, r) {
			continue
		}
		if _, isEv := evicted[d]; isEv {
			continue
		}
		if partner, ok := mapping[d]; ok && to.Contains(partner) {
			if _, isEv := evicted[partner]; isEv {
				continue
			}
			return true
		}
	}
	return false
}

// hasPayloadLeaves reports whether n has at least one payload leaf in its subtree.
func hasPayloadLeaves(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	nodes := n.Descendants()
	if n.Index != nil && n.PreSize > 0 {
		nodes = n.Index.Nodes[n.ID+1 : n.ID+n.PreSize]
	}
	for _, d := range nodes {
		if isPayloadLeaf(d, r) {
			return true
		}
	}
	return false
}

// hasMappedDescendantIn reports whether any descendant of src is mapped inside dst.
func hasMappedDescendantIn(src, dst *treesitter.ASTNode, ms *engine.Mapping) bool {
	if src == nil || dst == nil || ms == nil {
		return false
	}
	mapping := ms.Src()
	nodes := src.Descendants()
	if src.Index != nil && src.PreSize > 0 {
		nodes = src.Index.Nodes[src.ID+1 : src.ID+src.PreSize]
	}
	for _, d := range nodes {
		if partner, ok := mapping[d]; ok && dst.Contains(partner) {
			return true
		}
	}
	return false
}

// computeMoveRetention returns the fraction of payload leaves preserved between
// src and dst in [0.0, 1.0], counting updated leaves at half weight.
func computeMoveRetention(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules, evicted map[*treesitter.ASTNode]struct{}) float64 {
	if src == nil || dst == nil || ms == nil {
		return 1.0
	}
	srcSummary := summarizeMappedLeaves(src, dst, ms, r, evicted)

	nDst := 0
	for _, d := range dst.Descendants() {
		if isPayloadLeaf(d, r) {
			nDst++
		}
	}
	nMax := max(srcSummary.total, nDst)
	if nMax == 0 {
		return 1.0
	}

	effectiveSurviving := float64(srcSummary.exact) + 0.5*float64(srcSummary.updated)
	return effectiveSurviving / float64(nMax)
}

// findBodyBlock returns the primary consequence/body block of a control-flow statement
// or an enclosed callback block inside a call expression.
func findBodyBlock(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		isBlock := (r != nil && r.IsBlock(c.Type)) || (r == nil && rules.IsBlock(c.Type))
		if isBlock {
			return c
		}
	}
	if r != nil && !r.IsDeclaration(n.Type) && !r.IsBlock(n.Type) && r.IsCall(n.Type) {
		for _, child := range n.Children {
			if r.IsClosure(child.Type) {
				for _, body := range child.Children {
					if r.IsBlock(body.Type) {
						return body
					}
				}
			}
			for _, sub := range child.Children {
				if r.IsClosure(sub.Type) {
					for _, body := range sub.Children {
						if r.IsBlock(body.Type) {
							return body
						}
					}
				}
			}
		}
	}
	return nil
}

// findCall returns the first call expression found within a statement subtree, or nil.
func findCall(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	if r != nil && r.IsCall(n.Type) {
		return n
	}
	for _, c := range n.Children {
		if found := findCall(c, r); found != nil {
			return found
		}
	}
	return nil
}

// isTerminatingStatement reports whether stmt is a control-flow jump (return, break, etc.)
// or an execution-terminating call (os.Exit, panic, sys.exit, etc.).
func isTerminatingStatement(stmt *treesitter.ASTNode, r *rules.Rules) bool {
	if stmt == nil || r == nil {
		return false
	}
	if r.IsJumpStatement(stmt.Type) {
		return true
	}
	call := findCall(stmt, r)
	if call == nil {
		return false
	}
	path := treesitter.CalleePath(call)
	return r.IsTerminalCallPath(path)
}

// isFillerCallStatement reports whether stmt is a standalone call without its
// own control flow. These can sit next to an exit call (like a log before os.Exit)
// without turning the block into real business logic.
func isFillerCallStatement(stmt *treesitter.ASTNode, r *rules.Rules) bool {
	if stmt == nil || r == nil {
		return false
	}
	if r.IsCall(stmt.Type) {
		return true
	}
	if len(stmt.Children) != 1 {
		return false
	}
	return r.IsCall(stmt.Children[0].Type)
}

// isTrivialJumpBody reports whether a block is just early-exit boilerplate
// (returns, breaks, or os.Exit/panic, plus any preceding print or log calls).
func isTrivialJumpBody(body *treesitter.ASTNode, r *rules.Rules) bool {
	if body == nil {
		return true
	}
	stmtCount := 0
	nonFillerCount := 0
	hasTerminator := false
	check := func(s *treesitter.ASTNode) {
		stmtCount++
		switch {
		case isTerminatingStatement(s, r):
			hasTerminator = true
		case !isFillerCallStatement(s, r):
			nonFillerCount++
		}
	}
	for _, c := range body.Children {
		isPunct := (r != nil && r.IsPunctuation(c.Type)) || (r == nil && rules.IsPunctuation(c.Type))
		if !isPunct {
			check(c)
		}
	}
	return stmtCount > 0 && nonFillerCount == 0 && hasTerminator
}

// subtreeHeight returns the maximum depth of a subtree.
func subtreeHeight(n *treesitter.ASTNode) int {
	if n == nil || len(n.Children) == 0 {
		return 0
	}
	maxChild := 0
	for _, c := range n.Children {
		maxChild = max(maxChild, subtreeHeight(c))
	}
	return maxChild + 1
}

// subtreeLines returns the line span (EndRow - StartRow) of a node.
func subtreeLines(n *treesitter.ASTNode) uint32 {
	if n == nil {
		return 0
	}
	if n.EndRow >= n.StartRow {
		return n.EndRow - n.StartRow
	}
	return 0
}

// isTokenNode reports whether n is a bare token (operator, punctuation, type, or identifier).
func isTokenNode(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || r == nil {
		return false
	}
	return r.IsOperatorLiteral(n.Type) || r.IsPunctuation(n.Type) ||
		r.IsType(n.Type) || r.IsIdentifier(n.Type)
}

// moveStructuralScore scores how structurally significant a node is.
//
//	S = BaseSize + 2*Height + 3*LineSpan + RoleBonus - BoilerplatePenalty
//
// Bare tokens are clamped to S=1. Declarations receive +40. Boilerplate bodies receive -20.
func moveStructuralScore(node *treesitter.ASTNode, r *rules.Rules, ms *engine.Mapping) int {
	if node == nil || r == nil {
		return 0
	}

	// Token clamp: bare tokens, operators, punctuation, types, identifiers.
	if isTokenNode(node, r) {
		return 1
	}

	size := node.Size()
	height := subtreeHeight(node)
	lines := subtreeLines(node)

	// If most of a container was deleted, score it by its surviving nodes so a
	// gutted block doesn't look like a real move.
	if ms != nil && (isDelimitedOrBlockContainer(node, r) || (!r.IsDeclaration(node.Type) && findBodyBlock(node, r) != nil)) {
		surviving := 0
		totalLeaves := 0
		for _, d := range node.Descendants() {
			if !isPayloadLeaf(d, r) {
				continue
			}
			totalLeaves++
			if ms.Has(d) || ms.HasDst(d) {
				surviving++
			}
		}
		if totalLeaves > 0 && surviving < totalLeaves {
			ratio := float64(surviving) / float64(totalLeaves)
			size = min(size, surviving*3)
			height = int(float64(height) * ratio)
			lines = uint32(float64(lines) * ratio)
		}
	}

	score := size + 2*height + 3*int(lines)

	// Role bonus.
	if r.IsDeclaration(node.Type) {
		score += 40
	} else if r.IsBlock(node.Type) {
		score += 10
	}

	// Boilerplate penalty: bodies consisting entirely of terminating statements.
	body := node
	if !r.IsBlock(node.Type) {
		body = findBodyBlock(node, r)
	}
	if body != nil && isTrivialJumpBody(body, r) {
		score -= 20
	}

	return max(score, 1)
}

// requiredMoveThreshold computes the dynamic threshold T for a Move action
// based on construct mobility, scope preservation, and line distance.
func requiredMoveThreshold(src, dst *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) int {
	if src == nil || dst == nil || ms == nil || r == nil {
		return 50
	}

	// Intra-container reorder: src.Parent mapped to dst.Parent within the same enclosing declaration.
	if src.Parent != nil && dst.Parent != nil && ms.Src()[src.Parent] == dst.Parent && sameScopeDeclaration(src, dst, ms, r) {
		return 1
	}

	// Cross-key guard: a value sitting under one key in a key-value pair
	// (keyed_element, pair) and a value under a *different*, unmapped key
	// should never be treated as a trivial same-line shift, even if they
	// happen to land on the same row. Otherwise a clamped bare token (S=1)
	// can "move" from one struct field to an unrelated one.
	crossKey := src.Parent != nil && dst.Parent != nil && r.IsPair(src.Parent.Type) && r.IsPair(dst.Parent.Type)

	lineDist := int(src.StartRow) - int(dst.StartRow)
	if lineDist < 0 {
		lineDist = -lineDist
	}

	// Same-line inline shifts within the same enclosing scope.
	if lineDist == 0 && sameScopeDeclaration(src, dst, ms, r) {
		if crossKey {
			return 5
		}
		return 1
	}

	// Only apply T=20 when both sides are declarations. If only src is a
	// declaration, dst could be any arbitrary node and steal stubs or forward
	// declarations across distant files.
	if r.IsDeclaration(src.Type) && r.IsDeclaration(dst.Type) {
		return 20
	}

	srcStmt := engine.FindEnclosingStatement(src, r)
	dstStmt := engine.FindEnclosingStatement(dst, r)

	// Treat control-flow init statements as independent statement boundaries so
	// moving an init statement out of an if/for/switch header doesn't get treated
	// as an expression extraction/inlining with a prohibitive T=20 threshold.
	isInitStmt := func(n *treesitter.ASTNode) bool {
		if n == nil || n.Parent == nil {
			return false
		}
		p := n.Parent
		body := findBodyBlock(p, r)
		if body == n || (body != nil && body.Contains(n)) {
			return false
		}
		return r.IsLocalVarDeclaration(n.Type) || r.IsAssignment(n.Type)
	}
	if isInitStmt(src) {
		srcStmt = src
	}
	if isInitStmt(dst) {
		dstStmt = dst
	}

	// Moves within the same function or declaration.
	if sameScopeDeclaration(src, dst, ms, r) {
		// Moving an expression across statements (like inlining a variable) or between
		// containers in the same statement (like pulling a field into another struct)
		// needs a higher threshold (T=20). That keeps small tokens from getting stolen
		// out of deleted elements.
		if src != srcStmt || dst != dstStmt {
			if ms.Get(srcStmt) != dstStmt {
				return 20
			}
			if src.Parent != nil && dst.Parent != nil {
				if mappedDstParent := ms.Dst()[dst.Parent]; mappedDstParent != nil && !mappedDstParent.Contains(src) {
					return 20
				}
				if mappedSrcParent := ms.Src()[src.Parent]; mappedSrcParent != nil && !mappedSrcParent.Contains(dst) {
					return 20
				}
			}
		}
		// If the statement itself didn't move, measure line drift relative to the
		// statement so earlier edits in the file don't inflate the move threshold.
		if src != srcStmt && dst != dstStmt && isStatementStationary(srcStmt, dstStmt, ms, r) {
			stmtLineDist := (int(src.StartRow) - int(srcStmt.StartRow)) - (int(dst.StartRow) - int(dstStmt.StartRow))
			if stmtLineDist < 0 {
				stmtLineDist = -stmtLineDist
			}
			return 4 + stmtLineDist/5
		}
		// Statements or reorders inside the same statement use mild distance scaling.
		// The floor of 4 lets small expressions like condition calls (S=4) still match.
		return 4 + lineDist/5
	}

	// Moves within 10 lines get mild scaling, but only if the source function
	// wasn't deleted.
	if lineDist < 10 {
		srcDecl := src.EnclosingContainerDeclaration(r)
		if srcDecl != nil && ms.Src()[srcDecl] != nil {
			return 4 + lineDist/5
		}
	}

	// Cross-scope statements: severe distance penalty.
	return 50 + lineDist/10
}

// cohortKey groups moves by source parent and destination container.
type cohortKey struct {
	srcParent    *treesitter.ASTNode
	dstContainer *treesitter.ASTNode
}

// buildCohortProtected marks moves that shouldn't be demoted because they're
// moving alongside an anchor. If several siblings move together to the same
// destination and at least one qualifies on its own, we keep the whole group.
func buildCohortProtected(es *actions.EditScript, ms *engine.Mapping) map[*treesitter.ASTNode]bool {
	const minCohortSize = 3

	groups := make(map[cohortKey][]*treesitter.ASTNode)
	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil || a.Node.Parent == nil {
			continue
		}
		dstNode := cmp.Or(a.DestNode, ms.Src()[a.Node])
		if dstNode == nil {
			continue
		}
		r := rules.Get(a.Node.GetLanguage())

		dstContainer := cmp.Or(dstNode.EnclosingContainerDeclaration(r), dstNode.Parent)
		if dstContainer == nil {
			continue
		}
		key := cohortKey{srcParent: a.Node.Parent, dstContainer: dstContainer}
		groups[key] = append(groups[key], a.Node)
	}

	protected := make(map[*treesitter.ASTNode]bool)
	for _, nodes := range groups {
		if len(nodes) < minCohortSize {
			continue
		}
		// At least one sibling must qualify on its own so unrelated boilerplate
		// doesn't drag the whole block along.
		hasAnchor := slices.ContainsFunc(nodes, func(n *treesitter.ASTNode) bool {
			dst := ms.Src()[n]
			r := rules.Get(n.GetLanguage())
			return moveStructuralScore(n, r, ms) >= requiredMoveThreshold(n, dst, ms, r)
		})
		if !hasAnchor {
			continue
		}
		for _, n := range nodes {
			protected[n] = true
		}
	}
	return protected
}
