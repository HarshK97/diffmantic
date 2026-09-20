package engine

import (
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// RevokeOrphanedGlue unmaps glue tokens (operators, punctuation, keywords)
// that matched across different containers, and drops moving containers that
// are held solely by glue. Glue can help anchor stationary constructs, but it
// shouldn't move on its own or drag an unmapped container into a new scope.
func RevokeOrphanedGlue(m *Mapping) {
	if m == nil {
		return
	}
	for _, p := range append([]MappingPair(nil), m.Pairs...) {
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
	if src.Parent != nil && (dst.Parent == nil || m.Src()[src.Parent] != dst.Parent) {
		return true
	}
	srcDecl := GetEnclosingDeclaration(src)
	dstDecl := GetEnclosingDeclaration(dst)
	if srcDecl != nil && (dstDecl == nil || m.Src()[srcDecl] != dstDecl) {
		return true
	}
	return false
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
	if r != nil {
		return r.IsOperatorLiteral(n.Type) || r.IsPunctuation(n.Type) || n.IsKeyword || r.IsKeyword(n.Type, n.Label)
	}
	return rules.IsOperatorLiteral(n.Type) || rules.IsPunctuation(n.Type) || n.IsKeyword || rules.IsKeyword(n.Type, n.Label)
}
