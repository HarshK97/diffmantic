package postprocess

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// normalizeStationaryWrapperMoves drops false-positive Move actions when a wrapper
// (like friend_declaration or template_declaration) is added or removed around code that stayed in place.
func normalizeStationaryWrapperMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil && a.Node.Parent != nil && dstNode.Parent != nil {
				hasPos := (a.Node.EndByte > 0 || a.Node.StartRow > 0 || a.Node.EndRow > 0) &&
					(dstNode.EndByte > 0 || dstNode.StartRow > 0 || dstNode.EndRow > 0)
				if hasPos && a.Node.StartRow == dstNode.StartRow && a.Node.EndRow == dstNode.EndRow && a.Node.StartCol == dstNode.StartCol && a.Node.EndCol == dstNode.EndCol {
					continue
				}
				srcParent := a.Node.Parent
				dstParent := dstNode.Parent

				// If direct parents match, the node moved within the same container (like swapped arguments).
				if ms.Src()[srcParent] == dstParent {
					result.Add(a)
					continue
				}

				sameLine := a.Node.StartRow == dstNode.StartRow

				// Walk up past transparent wrappers to find the enclosing container.
				srcBase := srcParent
				srcChild := a.Node
				dstBase := dstParent
				dstChild := dstNode

				canUnwrap := func(base, child *treesitter.ASTNode, isDst bool) bool {
					if base == nil || base.Parent == nil {
						return false
					}
					r := rules.Get(base.GetLanguage())
					isCall := (r != nil && r.IsCall(base.Type)) || (r == nil && rules.IsCall(base.Type))
					if isCall && child.ChildIndex() != 0 && !sameLine {
						return false
					}
					if base.Parent != nil {
						parentIsCall := (r != nil && r.IsCall(base.Parent.Type)) || (r == nil && rules.IsCall(base.Parent.Type))
						if parentIsCall && base.ChildIndex() != 0 && !sameLine {
							return false
						}
					}

					if base.IsWrapper() {
						return true
					}
					hasMapping := ms.Has(base)
					if isDst {
						hasMapping = ms.HasDst(base)
					}
					if !hasMapping && canUnwrapSubExpression(base) {
						if isDst {
							return base.ChildIndex() == 0 || ms.Dst()[base.Parent] == srcBase || ms.Dst()[base.Parent] == srcParent
						}
						return base.ChildIndex() == 0 || ms.Src()[base.Parent] == dstBase || ms.Src()[base.Parent] == dstParent
					}
					return false
				}

				for canUnwrap(srcBase, srcChild, false) && srcBase.Parent != nil {
					srcChild = srcBase
					srcBase = srcBase.Parent
				}

				for canUnwrap(dstBase, dstChild, true) && dstBase.Parent != nil {
					dstChild = dstBase
					dstBase = dstBase.Parent
				}

				if (srcBase != srcParent || dstBase != dstParent) && ms.Src()[srcBase] == dstBase && srcChild.ChildIndex() == dstChild.ChildIndex() {
					continue
				}
			}
		}
		result.Add(a)
	}
	return result
}

// canUnwrapSubExpression checks if n is a lightweight wrapper or sub-expression
// that code can stay stationary across, rather than a distinct semantic container.
func canUnwrapSubExpression(n *treesitter.ASTNode) bool {
	if n == nil || n.Parent == nil {
		return false
	}
	r := rules.Get(n.GetLanguage())
	if r != nil {
		if r.IsPair(n.Type) || r.IsUnordered(n.Type) || r.IsBlock(n.Type) {
			return false
		}
		if r.IsDeclaration(n.Type) && !r.IsWrapper(n.Type) {
			return false
		}
	}
	return true
}

// normalizeCrossScopeNonStructuralMoves demotes moves of non-structural, single-line nodes
// across different semantic scopes into separate Delete and Insert actions.
func normalizeCrossScopeNonStructuralMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				lineDist := int(a.Node.StartRow) - int(dstNode.StartRow)
				if lineDist < 0 {
					lineDist = -lineDist
				}

				isType := (r != nil && r.IsType(a.Node.Type)) || (r == nil && rules.IsType(a.Node.Type))
				if isType && !engine.IsScopePreserved(ms, a.Node, dstNode, r, r) && lineDist >= 10 {
					demoteMoveToDelIns(result, ms, a.Node, dstNode)
					continue
				}
			}
		}
		result.Add(a)
	}
	return result
}

// normalizeOrphanedOperatorMoves demotes Move actions on operator literals when
// their enclosing parent containers did not move together.
func normalizeOrphanedOperatorMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				isOperator := (r != nil && (r.IsOperatorLiteral(a.Node.Type) || r.IsPunctuation(a.Node.Type))) ||
					(r == nil && (rules.IsOperatorLiteral(a.Node.Type) || rules.IsPunctuation(a.Node.Type)))
				if isOperator {
					parentMatched := a.Node.Parent != nil && dstNode.Parent != nil && ms.Src()[a.Node.Parent] == dstNode.Parent
					if !parentMatched {
						demoteMoveToDelIns(result, ms, a.Node, dstNode)
						continue
					}
				}
			}
		}
		result.Add(a)
	}
	return result
}

// findBodyBlock returns the primary consequence/body block of a control-flow statement.
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
	return nil
}

// normalizeControlFlowMoves demotes Move actions on control-flow statements when their
// consequence bodies share zero matched statements across hunks (>= 10 lines).
func normalizeControlFlowMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				isDecl := (r != nil && r.IsDeclaration(a.Node.Type)) || (r == nil && rules.IsDeclaration(a.Node.Type))
				// When a compound statement or declaration body block has no matching statements across distant hunks, demote the move.
				srcBody := findBodyBlock(a.Node, r)
				dstBody := findBodyBlock(dstNode, r)
				if srcBody != nil && dstBody != nil {
					lineDist := int(a.Node.StartRow) - int(dstNode.StartRow)
					if lineDist < 0 {
						lineDist = -lineDist
					}
					if lineDist >= 10 {
						bodyMatched := ms.Src()[srcBody] == dstBody || ms.DiceSrc(srcBody, dstBody) > 0
						if isDecl && ms.DiceSrc(srcBody, dstBody) == 0 {
							bodyMatched = false
						}
						srcCond := findCondition(a.Node, r)
						dstCond := findCondition(dstNode, r)
						if isTrivialJumpBody(srcBody, r) && srcCond != nil && dstCond != nil && !hasMatchedCondition(a.Node, dstNode, ms, r) {
							bodyMatched = false
						}
						if !bodyMatched {
							demoteMoveToDelIns(result, ms, a.Node, dstNode)
							continue
						}
					}
				}

				// Demote when an outer control-flow clause or header moved independently of its enclosing statement.
				srcCF, srcBody := findEnclosingControlFlow(a.Node, r)
				dstCF, _ := findEnclosingControlFlow(dstNode, r)
				if srcCF != nil && dstCF != nil {
					isHeader := !srcBody.Contains(a.Node) && a.Node != srcBody
					if isHeader {
						lineDist := int(a.Node.StartRow) - int(dstNode.StartRow)
						if lineDist < 0 {
							lineDist = -lineDist
						}
						cfMatched := ms.Src()[srcCF] == dstCF ||
							(ms.Src()[srcCF] != nil && ms.Src()[srcCF].Contains(dstCF)) ||
							(ms.Dst()[dstCF] != nil && ms.Dst()[dstCF].Contains(srcCF))
						if lineDist >= 10 && !cfMatched {
							demoteMoveToDelIns(result, ms, a.Node, dstNode)
							continue
						}
					}
				}
			}
		}
		result.Add(a)
	}
	return result
}

// findEnclosingControlFlow returns the nearest ancestor statement that has a body block,
// stopping if a declaration or block boundary is reached.
func findEnclosingControlFlow(n *treesitter.ASTNode, r *rules.Rules) (*treesitter.ASTNode, *treesitter.ASTNode) {
	if n == nil {
		return nil, nil
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		body := findBodyBlock(curr, r)
		if body != nil {
			return curr, body
		}
		isBoundary := (r != nil && (r.IsDeclaration(curr.Type) || r.IsBlock(curr.Type))) ||
			(r == nil && (rules.IsDeclaration(curr.Type) || rules.IsBlock(curr.Type)))
		if isBoundary {
			break
		}
	}
	return nil, nil
}

// findCondition returns the primary conditional expression node of a control-flow statement.
func findCondition(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		isBlock := (r != nil && r.IsBlock(c.Type)) || (r == nil && rules.IsBlock(c.Type))
		if isBlock {
			break
		}
		isKeyword := (r != nil && r.IsKeyword(c.Type, c.Label)) || (r == nil && rules.IsKeyword(c.Type, c.Label))
		if !isKeyword && c.Type != "(" && c.Type != ")" {
			return c
		}
	}
	return nil
}

// hasMatchedCondition returns true if any expression or identifier inside srcIf's condition
// is mapped to a descendant of dstIf's condition.
func hasMatchedCondition(srcIf, dstIf *treesitter.ASTNode, ms *engine.Mapping, r *rules.Rules) bool {
	srcCond := findCondition(srcIf, r)
	dstCond := findCondition(dstIf, r)
	if srcCond == nil || dstCond == nil {
		return false
	}
	if ms.Src()[srcCond] == dstCond {
		return true
	}
	for _, d := range srcCond.Descendants() {
		if dst, ok := ms.Src()[d]; ok && (dst == dstCond || dstCond.Contains(dst)) {
			return true
		}
	}
	return false
}

func isTrivialJumpStatement(s *treesitter.ASTNode) bool {
	if s == nil {
		return false
	}
	t := s.Type
	return strings.HasPrefix(t, "return") || strings.HasPrefix(t, "break") ||
		strings.HasPrefix(t, "continue") || strings.HasPrefix(t, "goto")
}

// isTrivialJumpBody returns true if body consists entirely of trivial control jump statements
// (return, break, continue, goto).
func isTrivialJumpBody(body *treesitter.ASTNode, r *rules.Rules) bool {
	if body == nil {
		return true
	}
	stmtCount := 0
	nonJumpCount := 0
	for _, c := range body.Children {
		if c.Type == "statement_list" {
			for _, s := range c.Children {
				stmtCount++
				if !isTrivialJumpStatement(s) {
					nonJumpCount++
				}
			}
		} else {
			isPunct := (r != nil && r.IsPunctuation(c.Type)) || (r == nil && rules.IsPunctuation(c.Type))
			if !isPunct && c.Type != "{" && c.Type != "}" {
				stmtCount++
				if !isTrivialJumpStatement(c) {
					nonJumpCount++
				}
			}
		}
	}
	return stmtCount > 0 && nonJumpCount == 0
}

// findEnclosingDeclaration returns the nearest ancestor declaration node of n,
// stopping if a block boundary is reached.
func findEnclosingDeclaration(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isDecl := (r != nil && r.IsDeclaration(curr.Type)) || (r == nil && rules.IsDeclaration(curr.Type))
		if isDecl {
			return curr
		}
		isBlock := (r != nil && r.IsBlock(curr.Type)) || (r == nil && rules.IsBlock(curr.Type))
		if isBlock {
			break
		}
	}
	return nil
}

// normalizeOrphanedDeclarationParameterMoves demotes Move actions on function/method declaration
// parameters when their enclosing declarations were not matched or moved together across hunks (>= 10 lines).
func normalizeOrphanedDeclarationParameterMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				srcDecl := findEnclosingDeclaration(a.Node, r)
				dstDecl := findEnclosingDeclaration(dstNode, r)
				if srcDecl != nil || dstDecl != nil {
					lineDist := int(a.Node.StartRow) - int(dstNode.StartRow)
					if lineDist < 0 {
						lineDist = -lineDist
					}
					if lineDist >= 10 {
						declMatched := srcDecl != nil && dstDecl != nil && ms.Src()[srcDecl] == dstDecl
						if !declMatched {
							demoteMoveToDelIns(result, ms, a.Node, dstNode)
							continue
						}
					}
				}
			}
		}
		result.Add(a)
	}
	return result
}

// findEnclosingCall returns the nearest ancestor call of n,
// stopping if a declaration or block boundary is reached.
func findEnclosingCall(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isCall := (r != nil && r.IsCall(curr.Type)) || (r == nil && rules.IsCall(curr.Type))
		if isCall {
			return curr
		}
		isBoundary := (r != nil && (r.IsDeclaration(curr.Type) || r.IsBlock(curr.Type))) ||
			(r == nil && (rules.IsDeclaration(curr.Type) || rules.IsBlock(curr.Type)))
		if isBoundary {
			break
		}
	}
	return nil
}

// normalizeOrphanedCallArgumentMoves demotes Move actions on function call arguments
// when their enclosing function calls were not matched or moved together across hunks (>= 10 lines).
func normalizeOrphanedCallArgumentMoves(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		if a.Type == actions.Move && a.Node != nil {
			dstNode := a.DestNode
			if dstNode == nil {
				dstNode = ms.Src()[a.Node]
			}
			if dstNode != nil {
				r := rules.Get(a.Node.GetLanguage())
				srcCall := findEnclosingCall(a.Node, r)
				dstCall := findEnclosingCall(dstNode, r)
				if srcCall != nil || dstCall != nil {
					lineDist := int(a.Node.StartRow) - int(dstNode.StartRow)
					if lineDist < 0 {
						lineDist = -lineDist
					}
					callMatched := (srcCall != nil && dstCall != nil && ms.Src()[srcCall] == dstCall) ||
						(srcCall != nil && ms.Src()[srcCall] != nil && ms.Src()[srcCall].Contains(dstNode)) ||
						(dstCall != nil && ms.Dst()[dstCall] != nil && ms.Dst()[dstCall].Contains(a.Node))
					if (srcCall != nil && dstCall != nil && !callMatched) || (lineDist >= 10 && !callMatched) {
						demoteMoveToDelIns(result, ms, a.Node, dstNode)
						continue
					}
				}
			}
		}
		result.Add(a)
	}
	return result
}

// normalizeWrapperDelimiterChanges emits non-subtree Insert or Delete actions when a mapped
// wrapper container (such as argument_list) gains or loses outer delimiter punctuation
// (such as optional parentheses in Ruby) around surviving mapped children.
func normalizeWrapperDelimiterChanges(es *actions.EditScript, ms *engine.Mapping) *actions.EditScript {
	if es == nil || ms == nil {
		return es
	}

	hasAction := make(map[*treesitter.ASTNode]bool)
	for _, a := range es.Actions() {
		if a.Node != nil {
			hasAction[a.Node] = true
		}
	}

	result := actions.NewEditScript()
	for _, a := range es.Actions() {
		result.Add(a)
	}

	for _, p := range ms.Pairs {
		srcNode, dstNode := p.Src, p.Dst
		if srcNode == nil || dstNode == nil {
			continue
		}
		if hasAction[srcNode] || hasAction[dstNode] {
			continue
		}
		r := rules.Get(srcNode.GetLanguage())
		if r == nil || !r.IsWrapper(srcNode.Type) || !r.IsWrapper(dstNode.Type) {
			continue
		}
		if len(srcNode.Children) == 0 || len(dstNode.Children) == 0 {
			continue
		}

		srcHasDelims := srcNode.StartByte < srcNode.Children[0].StartByte || srcNode.EndByte > srcNode.Children[len(srcNode.Children)-1].EndByte
		dstHasDelims := dstNode.StartByte < dstNode.Children[0].StartByte || dstNode.EndByte > dstNode.Children[len(dstNode.Children)-1].EndByte

		if !srcHasDelims && dstHasDelims {
			result.Add(actions.Action{
				Type:     actions.Insert,
				Node:     dstNode,
				Parent:   dstNode.Parent,
				Position: dstNode.ChildIndex(),
				Subtree:  false,
			})
		} else if srcHasDelims && !dstHasDelims {
			result.Add(actions.Action{
				Type:    actions.Delete,
				Node:    srcNode,
				Parent:  srcNode.Parent,
				Subtree: false,
			})
		}
	}

	return result
}

// demoteMoveToDelIns demotes a Move action into separate Delete and Insert actions
// and clears descendant mappings from the mapping store.
func demoteMoveToDelIns(result *actions.EditScript, ms *engine.Mapping, srcNode, dstNode *treesitter.ASTNode) {
	result.Add(actions.Action{
		Type:    actions.Delete,
		Node:    srcNode,
		Parent:  srcNode.Parent,
		Subtree: len(srcNode.Children) > 0,
	})
	result.Add(actions.Action{
		Type:     actions.Insert,
		Node:     dstNode,
		Parent:   dstNode.Parent,
		Position: dstNode.ChildIndex(),
		Subtree:  len(dstNode.Children) > 0,
	})
	for _, d := range srcNode.Descendants() {
		ms.Remove(d)
	}
	ms.Remove(srcNode)
}
