package treesitter

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// IngestFlatAST converts the flat node buffer into a full ASTNode tree,
// applying language rules, labels, and subtree hashes.
func IngestFlatAST(nodes []FlatNode, symbols []string, src []byte, langName string) *ASTNode {
	if len(nodes) == 0 {
		return nil
	}

	r := rules.Get(langName)
	srcLen := uint32(len(src))

	var errCount int
	for i := range nodes {
		fn := &nodes[i]
		var rawType string
		if (fn.Flags & FlatNodeError) != 0 {
			rawType = "ERROR"
		} else if int(fn.TypeID) < len(symbols) {
			rawType = symbols[fn.TypeID]
		}
		if rawType == "ERROR" || (fn.Flags&FlatNodeError != 0) || (fn.Flags&FlatNodeMissing != 0) {
			errCount++
		}
	}

	flatToAST := make([]*ASTNode, len(nodes))
	root := buildFromIndex(0, nodes, symbols, src, nil, r, srcLen, flatToAST)
	if root != nil {
		root.Language = langName
		root.ParseErrorCount = errCount
		root.HasError = errCount > 0
		root.ComputeHashes()
		EnsureIndex(root)
		attachTrivia(root, nodes, symbols, src, langName, r, flatToAST)
	}
	return root
}

func buildFromIndex(idx uint32, nodes []FlatNode, symbols []string, src []byte, parent *ASTNode, r *rules.Rules, srcLen uint32, flatToAST []*ASTNode) *ASTNode {
	if int(idx) >= len(nodes) {
		return nil
	}
	fn := &nodes[idx]
	var rawType string
	if (fn.Flags & FlatNodeError) != 0 {
		rawType = "ERROR"
	} else if int(fn.TypeID) < len(symbols) {
		rawType = symbols[fn.TypeID]
	}
	if (fn.Flags & FlatNodeMissing) != 0 {
		rawType = "MISSING " + rawType
	}

	isLeaf := fn.ChildCount == 0 || (r != nil && r.IsFlattened(rawType))
	var label string
	if isLeaf {
		start, end := min(fn.StartByte, srcLen), min(fn.EndByte, srcLen)
		if start > end {
			start = end
		}
		label = strings.TrimSpace(string(src[start:end]))
	}

	if r != nil && r.IsIgnored(rawType, label) {
		return nil
	}

	node := &ASTNode{
		Type:      rawType,
		Parent:    parent,
		StartByte: fn.StartByte,
		EndByte:   fn.EndByte,
		StartRow:  fn.StartRow,
		StartCol:  fn.StartCol,
		EndRow:    fn.EndRow,
		EndCol:    fn.EndCol,
	}
	flatToAST[idx] = node

	if isLeaf {
		node.Label = label
	}

	if r != nil {
		if alias, ok := r.Alias(node.Type, label); ok {
			node.Type = alias
		}
		if r.IsLabelIgnored(node.Type) {
			node.Label = ""
		}
		if isLeaf && r.IsKeyword(node.Type, label) {
			node.IsKeyword = true
		}
		if r.IsUnordered(node.Type) {
			node.IsUnordered = true
		}
	}

	childIdx := fn.FirstChildIdx
	for childIdx != FlatNodeNone && int(childIdx) < len(nodes) {
		if child := buildFromIndex(childIdx, nodes, symbols, src, node, r, srcLen, flatToAST); child != nil {
			node.Children = append(node.Children, child)
		}
		childIdx = nodes[childIdx].NextSiblingIdx
	}

	if r != nil && r.IsFlattened(rawType) {
		var flattenedChildren []*ASTNode
		for _, child := range node.Children {
			flattenedChildren = append(flattenedChildren, child.Children...)
			for _, grandchild := range child.Children {
				grandchild.Parent = node
			}
		}
		node.Children = flattenedChildren
	}

	return node
}

func attachTrivia(root *ASTNode, nodes []FlatNode, symbols []string, src []byte, langName string, r *rules.Rules, flatToAST []*ASTNode) {
	if root == nil || len(nodes) == 0 {
		return
	}
	srcLen := uint32(len(src))

	isComment := func(nodeType string) bool {
		if r != nil {
			return r.IsComment(nodeType)
		}
		return rules.IsComment(nodeType)
	}

	for idx := range nodes {
		fn := &nodes[idx]
		var rawType string
		if int(fn.TypeID) < len(symbols) {
			rawType = symbols[fn.TypeID]
		}
		if !isComment(rawType) {
			continue
		}

		// Skip child comments whose ancestor is already a comment (e.g. Rust doc_comment wrapping line_comment)
		isChildComment := false
		for pIdx := fn.ParentIdx; pIdx != FlatNodeNone && int(pIdx) < len(nodes); pIdx = nodes[pIdx].ParentIdx {
			pType := ""
			if int(nodes[pIdx].TypeID) < len(symbols) {
				pType = symbols[nodes[pIdx].TypeID]
			}
			if isComment(pType) {
				isChildComment = true
				break
			}
		}
		if isChildComment {
			continue
		}

		start := min(fn.StartByte, srcLen)
		end := min(fn.EndByte, srcLen)
		if start > end {
			start = end
		}
		cb := &CommentBlock{
			Type:      rawType,
			Text:      string(src[start:end]),
			StartByte: start,
			EndByte:   end,
			StartRow:  int(fn.StartRow),
			StartCol:  int(fn.StartCol),
			EndRow:    int(fn.EndRow),
			EndCol:    int(fn.EndCol),
			Language:  langName,
		}

		// 1. Find the CST parent node and its nearest ASTNode
		pIdx := fn.ParentIdx
		for pIdx != FlatNodeNone && int(pIdx) < len(nodes) && flatToAST[pIdx] == nil {
			pIdx = nodes[pIdx].ParentIdx
		}
		parentAST := root
		if pIdx != FlatNodeNone && int(pIdx) < len(nodes) && flatToAST[pIdx] != nil {
			parentAST = flatToAST[pIdx]
		}

		// Trailing trivia: attach to the preceding code node on the same line.
		lastPrevIdx := FlatNodeNone
		if fn.ParentIdx != FlatNodeNone && int(fn.ParentIdx) < len(nodes) {
			sIdx := nodes[fn.ParentIdx].FirstChildIdx
			for sIdx != FlatNodeNone && sIdx != uint32(idx) && int(sIdx) < len(nodes) {
				sType := ""
				if int(nodes[sIdx].TypeID) < len(symbols) {
					sType = symbols[nodes[sIdx].TypeID]
				}
				if !isComment(sType) && sType != "\n" {
					lastPrevIdx = sIdx
				}
				sIdx = nodes[sIdx].NextSiblingIdx
			}
		}

		if lastPrevIdx != FlatNodeNone {
			prev := &nodes[lastPrevIdx]
			if prev.EndRow == fn.StartRow && prev.EndByte <= fn.StartByte {
				target := findLastASTNode(lastPrevIdx, nodes, flatToAST)
				if target != nil {
					if target.Parent != nil {
						cb.ParentType = target.Parent.Type
						cb.ParentStart = target.Parent.StartByte
						cb.ParentEnd = target.Parent.EndByte
						cb.ParentRow = int(target.Parent.StartRow)
						cb.ParentEndRow = int(target.Parent.EndRow)
					}
					if enc := findEnclosingDecl(target, r); enc != nil {
						cb.EnclosingDecl = enc
						cb.DeclType = enc.Type
						cb.DeclStart = enc.StartByte
						cb.DeclEnd = enc.EndByte
						cb.RelativePath = "body"
					} else {
						cb.EnclosingDecl = target
					}
					cb.AnchorNode = target
					target.TrailingTrivia = append(target.TrailingTrivia, cb)
					continue
				}
			}
		}

		// Leading trivia: attach to the next non-comment node in the same CST parent.
		nextCodeIdx := FlatNodeNone
		nextIdx := fn.NextSiblingIdx
		for nextIdx != FlatNodeNone && int(nextIdx) < len(nodes) {
			nType := ""
			if int(nodes[nextIdx].TypeID) < len(symbols) {
				nType = symbols[nodes[nextIdx].TypeID]
			}
			if !isComment(nType) && nType != "\n" {
				if r == nil || (!r.IsDelimiter(nType, nType) && !r.IsPunctuation(nType)) {
					nextCodeIdx = nextIdx
					break
				}
			}
			nextIdx = nodes[nextIdx].NextSiblingIdx
		}

		if nextCodeIdx != FlatNodeNone {
			target := findASTNode(nextCodeIdx, nodes, flatToAST)
			if target != nil {
				if target.Parent != nil {
					cb.ParentType = target.Parent.Type
					cb.ParentStart = target.Parent.StartByte
					cb.ParentEnd = target.Parent.EndByte
					cb.ParentRow = int(target.Parent.StartRow)
					cb.ParentEndRow = int(target.Parent.EndRow)
				}
				isTargetDecl := rules.IsDeclaration(target.Type)
				if r != nil {
					isTargetDecl = r.IsDeclaration(target.Type) && !r.IsLocalVarDeclaration(target.Type)
				}
				if isTargetDecl {
					cb.EnclosingDecl = target
					cb.DeclType = target.Type
					cb.DeclStart = target.StartByte
					cb.DeclEnd = target.EndByte
					cb.RelativePath = "doc"
				} else if enc := findEnclosingDecl(target.Parent, r); enc != nil {
					cb.EnclosingDecl = enc
					cb.DeclType = enc.Type
					cb.DeclStart = enc.StartByte
					cb.DeclEnd = enc.EndByte
					cb.RelativePath = "body"
				} else {
					cb.EnclosingDecl = target
				}
				cb.AnchorNode = target
				target.LeadingTrivia = append(target.LeadingTrivia, cb)
				continue
			}
		}

		// Dangling trivia: falls back to the enclosing container if no sibling claimed it.
		cb.ParentType = parentAST.Type
		cb.ParentStart = parentAST.StartByte
		cb.ParentEnd = parentAST.EndByte
		cb.ParentRow = int(parentAST.StartRow)
		cb.ParentEndRow = int(parentAST.EndRow)
		if enc := findEnclosingDecl(parentAST, r); enc != nil {
			cb.EnclosingDecl = enc
			cb.DeclType = enc.Type
			cb.DeclStart = enc.StartByte
			cb.DeclEnd = enc.EndByte
			cb.RelativePath = "body"
		} else {
			cb.EnclosingDecl = parentAST
		}
		cb.AnchorNode = parentAST
		parentAST.DanglingTrivia = append(parentAST.DanglingTrivia, cb)
	}
}

func findEnclosingDecl(n *ASTNode, r *rules.Rules) *ASTNode {
	for curr := n; curr != nil; curr = curr.Parent {
		isDecl := rules.IsDeclaration(curr.Type)
		if r != nil {
			isDecl = r.IsDeclaration(curr.Type) && !r.IsLocalVarDeclaration(curr.Type)
		}
		if isDecl {
			return curr
		}
	}
	return nil
}

func findASTNode(idx uint32, nodes []FlatNode, flatToAST []*ASTNode) *ASTNode {
	if idx == FlatNodeNone || int(idx) >= len(nodes) {
		return nil
	}
	if n := flatToAST[idx]; n != nil {
		return n
	}
	child := nodes[idx].FirstChildIdx
	for child != FlatNodeNone && int(child) < len(nodes) {
		if n := findASTNode(child, nodes, flatToAST); n != nil {
			return n
		}
		child = nodes[child].NextSiblingIdx
	}
	return nil
}

func findLastASTNode(idx uint32, nodes []FlatNode, flatToAST []*ASTNode) *ASTNode {
	if idx == FlatNodeNone || int(idx) >= len(nodes) {
		return nil
	}
	if n := flatToAST[idx]; n != nil {
		return n
	}
	var last *ASTNode
	child := nodes[idx].FirstChildIdx
	for child != FlatNodeNone && int(child) < len(nodes) {
		if n := findLastASTNode(child, nodes, flatToAST); n != nil {
			last = n
		}
		child = nodes[child].NextSiblingIdx
	}
	return last
}
