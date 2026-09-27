package engine

import (
	"cmp"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// RecoverGlueAnchors pairs unmapped parent containers when their operands or body
// blocks already match, keeping shared operators and recovering any remaining children.
func RecoverGlueAnchors(m *Mapping) {
	if m == nil {
		return
	}
	pairs := slices.Clone(m.Pairs)
	for _, p := range pairs {
		c1, c2 := p.Src, p.Dst
		if c1 == nil || c2 == nil {
			continue
		}
		r := rulesFor(c1)
		p1 := getEffectiveParent(c1, r)
		p2 := getEffectiveParent(c2, r)
		if p1 == nil || p2 == nil || m.Has(p1) || m.HasDst(p2) {
			continue
		}
		if !TypesMatch(p1.Type, p2.Type, r) {
			continue
		}
		if !isSameScope(p1, p2, m, r) {
			continue
		}
		lineDist := int(p1.StartRow) - int(p2.StartRow)
		if lineDist < 0 {
			lineDist = -lineDist
		}

		body1 := findBodyBlock(p1, r)
		body2 := findBodyBlock(p2, r)
		if body1 != nil && body2 != nil {
			isDecl := (r != nil && r.IsDeclaration(p1.Type)) || (r == nil && rules.IsDeclaration(p1.Type))

			// Matched body block in a control-flow statement or declaration.
			if m.Src()[body1] == body2 {
				if lineDist > 10 || (isDecl && m.DiceSrc(p1, p2) < 0.30) {
					continue
				}
				m.Add(p1, p2)
				pairKeywordGlue(p1, p2, m, r)
				if isDecl {
					reconcileDeclarationSignatures(p1, p2, m, r)
				}
				Recover(p1, p2, m)
				continue
			}

			// Matched condition or header when the body is still unmapped.
			if !m.Has(body1) && !m.HasDst(body2) {
				if isDecl {
					continue
				}
				if c1 != body1 && c2 != body2 && !isGlueToken(c1, r) {
					if lineDist > 2 || isBoilerplateCondition(c1, r) || isBoilerplateCondition(c2, r) {
						continue
					}
					m.Add(p1, p2)
					pairKeywordGlue(p1, p2, m, r)
					Recover(p1, p2, m)
					continue
				}
			}
		}

		// Binary or assignment expression with a matching operator.
		if isGlueToken(c1, r) {
			continue
		}
		op1 := findOperatorGlue(p1, r)
		op2 := findOperatorGlue(p2, r)
		if op1 != nil && op2 != nil && op1.Label == op2.Label && lineDist <= 10 {
			if (c1.StartByte < op1.StartByte) != (c2.StartByte < op2.StartByte) {
				continue
			}
			if !m.Has(op1) && !m.HasDst(op2) {
				m.Add(op1, op2)
			}
			m.Add(p1, p2)
			Recover(p1, p2, m)
			continue
		}
	}
	sortMappingsByPreOrder(m)
}

// RevokeOrphanedGlue unmaps glue tokens (operators, punctuation, keywords)
// that matched across different containers, and drops moving containers that
// are held solely by glue. Glue can help anchor stationary constructs, but it
// shouldn't move on its own or drag an unmapped container into a new scope.
func RevokeOrphanedGlue(m *Mapping) {
	if m == nil {
		return
	}

	for _, p := range slices.Clone(m.Pairs) {
		src, dst := p.Src, p.Dst
		if src == nil || dst == nil || src.Parent == nil {
			continue
		}
		if !m.Has(src) || m.Src()[src] != dst {
			continue
		}
		r := rulesFor(src)
		if !isGlueToken(src, r) {
			continue
		}
		if dst.Parent != nil && m.Src()[src.Parent] == dst.Parent {
			srcParent := src.Parent
			dstParent := dst.Parent
			isMoving := isMovingScope(srcParent, dstParent, m)
			if isMoving && !hasMatchedDirectNonGlueChild(srcParent, dstParent, m, r) {
				revokeSubtreeMatchUnder(srcParent, dstParent, m)
				continue
			}
			continue
		}
		m.Remove(src)
	}
	sortMappingsByPreOrder(m)
}

func isMovingScope(src, dst *treesitter.ASTNode, m *Mapping) bool {
	if src == nil || dst == nil || m == nil {
		return false
	}
	r := rulesFor(src)
	srcStmt := FindEnclosingStatement(src, r)
	dstStmt := FindEnclosingStatement(dst, r)
	// Changes staying inside the same matched statement haven't moved scope.
	if srcStmt != nil && dstStmt != nil && src != srcStmt && dst != dstStmt && m.Src()[srcStmt] == dstStmt {
		return false
	}
	if src.Parent != nil && (dst.Parent == nil || m.Src()[src.Parent] != dst.Parent) {
		return true
	}
	srcDecl := getEnclosingScopeDeclaration(src, r)
	dstDecl := getEnclosingScopeDeclaration(dst, r)
	if srcDecl != nil && (dstDecl == nil || m.Src()[srcDecl] != dstDecl) {
		return true
	}
	return false
}

func getEnclosingScopeDeclaration(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r = cmp.Or(r, rulesFor(n))
	isContainer := func(t string) bool {
		if r != nil {
			return r.IsContainerDeclaration(t) || r.IsClosure(t)
		}
		return rules.IsContainerDeclaration(t) || rules.IsClosure(t)
	}
	isDecl := func(t string) bool {
		if r != nil {
			return !r.IsLocalVarDeclaration(t) && r.IsDeclaration(t)
		}
		return !rules.IsLocalVarDeclaration(t) && rules.IsDeclaration(t)
	}

	for curr := n.Parent; curr != nil; curr = curr.Parent {
		if isContainer(curr.Type) {
			return curr
		}
	}
	// Fall back to general declarations if we're not inside a function or container.
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		if isDecl(curr.Type) {
			return curr
		}
	}
	return nil
}

// hasMatchedDirectNonGlueChild checks if the containers share at least one
// non-glue child with enough structural mass (effective height >= 2) to justify
// treating the container as moved.
func hasMatchedDirectNonGlueChild(srcParent, dstParent *treesitter.ASTNode, m *Mapping, r *rules.Rules) bool {
	if srcParent == nil || dstParent == nil || m == nil {
		return false
	}
	for _, c := range srcParent.Children {
		if isGlueToken(c, r) {
			continue
		}
		if dstC, ok := m.Src()[c]; ok && dstC.Parent == dstParent {
			if effectiveMatchedHeight(c, dstC, m, r) >= 2 {
				return true
			}
		}
	}
	return false
}

func effectiveMatchedHeight(c, dstC *treesitter.ASTNode, m *Mapping, r *rules.Rules) int {
	if c == nil || dstC == nil || m == nil {
		return 0
	}
	isWrap := (r != nil && r.IsWrapper(c.Type)) || (r == nil && rules.IsWrapper(c.Type))
	if isWrap && len(c.Children) == 1 {
		inner := c.Children[0]
		if dstInner, ok := m.Src()[inner]; ok && dstInner.Parent == dstC {
			return effectiveMatchedHeight(inner, dstInner, m, r)
		}
		return 0
	}
	return Height(c)
}

func revokeSubtreeMatchUnder(srcNode, dstNode *treesitter.ASTNode, m *Mapping) {
	if srcNode == nil || dstNode == nil || m == nil {
		return
	}
	m.Remove(srcNode)
	for _, c := range srcNode.Children {
		if dstC, ok := m.Src()[c]; ok && dstC.Parent == dstNode {
			revokeSubtreeMatchUnder(c, dstC, m)
		}
	}
}

func isGlueToken(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || len(n.Children) > 0 {
		return false
	}
	if (r != nil && r.IsType(n.Type)) || (r == nil && rules.IsType(n.Type)) {
		return false
	}
	if isOperatorGlue(n, r) {
		return true
	}
	if r != nil {
		return r.IsPunctuation(n.Type) || n.IsKeyword || r.IsKeyword(n.Type, n.Label)
	}
	return rules.IsPunctuation(n.Type) || n.IsKeyword || rules.IsKeyword(n.Type, n.Label)
}

func getEffectiveParent(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil || n.Parent == nil {
		return nil
	}
	p := n.Parent
	isWrap := (r != nil && r.IsWrapper(p.Type)) || (r == nil && rules.IsWrapper(p.Type))
	if isWrap && len(p.Children) == 1 && p.Parent != nil {
		return p.Parent
	}
	return p
}

func isSameScope(src, dst *treesitter.ASTNode, m *Mapping, r *rules.Rules) bool {
	if src == nil || dst == nil || m == nil {
		return false
	}
	srcStmt := FindEnclosingStatement(src, r)
	dstStmt := FindEnclosingStatement(dst, r)
	if srcStmt != nil && dstStmt != nil && m.Src()[srcStmt] == dstStmt {
		return true
	}
	srcDecl := getEnclosingScopeDeclaration(src, r)
	dstDecl := getEnclosingScopeDeclaration(dst, r)
	if srcDecl != nil && dstDecl != nil {
		return m.Src()[srcDecl] == dstDecl
	}
	return srcDecl == nil && dstDecl == nil
}

func isOperatorSymbol(s string) bool {
	if s == "" || s == ":" {
		return false
	}
	for _, ch := range s {
		switch ch {
		case '=', '+', '-', '*', '/', '%', '&', '|', '^', '<', '>', '!', '~', '?', ':', '@':
		default:
			return false
		}
	}
	return true
}

func isOperatorGlue(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil || len(n.Children) > 0 {
		return false
	}
	if (r != nil && r.IsType(n.Type)) || (r == nil && rules.IsType(n.Type)) {
		return false
	}
	if rules.IsDelimiter(n.Type, n.Label) {
		return false
	}
	if rules.IsOperatorLiteral(n.Type) {
		return true
	}
	return isOperatorSymbol(n.Label) || isOperatorSymbol(n.Type)
}

func findOperatorGlue(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if isOperatorGlue(c, r) {
			return c
		}
	}
	return nil
}

func pairKeywordGlue(p1, p2 *treesitter.ASTNode, m *Mapping, r *rules.Rules) {
	kw1 := findKeywordGlue(p1, r)
	kw2 := findKeywordGlue(p2, r)
	if kw1 != nil && kw2 != nil && kw1.Label == kw2.Label && !m.Has(kw1) && !m.HasDst(kw2) {
		m.Add(kw1, kw2)
	}
}

func findKeywordGlue(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if len(c.Children) == 0 && (c.IsKeyword || (r != nil && r.IsKeyword(c.Type, c.Label)) || (r == nil && rules.IsKeyword(c.Type, c.Label))) {
			return c
		}
	}
	return nil
}

func isUserVariable(leaf *treesitter.ASTNode, r *rules.Rules) bool {
	if leaf == nil || leaf.IsKeyword {
		return false
	}
	isID := (r != nil && r.IsIdentifier(leaf.Type)) || (r == nil && rules.IsIdentifier(leaf.Type))
	if !isID {
		return false
	}
	isSentinel := (r != nil && r.IsSentinel(leaf.Label)) || (r == nil && rules.IsSentinel(leaf.Label))
	return !isSentinel
}

func isBoilerplateCondition(c *treesitter.ASTNode, r *rules.Rules) bool {
	if c == nil || c.Size() <= 1 {
		return true
	}
	leaves := c.Leaves()
	if len(leaves) == 0 {
		return true
	}
	userVars := make(map[string]struct{})
	semanticTokens := 0
	for _, leaf := range leaves {
		isPunct := (r != nil && r.IsPunctuation(leaf.Type)) || (r == nil && rules.IsPunctuation(leaf.Type))
		if isPunct || rules.IsDelimiter(leaf.Type, leaf.Label) {
			continue
		}
		semanticTokens++
		if isUserVariable(leaf, r) {
			userVars[leaf.Label] = struct{}{}
		}
	}
	return semanticTokens <= 3 && len(userVars) <= 1
}
