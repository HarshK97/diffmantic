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

	root := buildFromIndex(0, nodes, symbols, src, nil, r, srcLen)
	if root != nil {
		root.Language = langName
		root.ParseErrorCount = errCount
		root.HasError = errCount > 0
		root.ComputeHashes()
		EnsureIndex(root)
	}
	return root
}

func buildFromIndex(idx uint32, nodes []FlatNode, symbols []string, src []byte, parent *ASTNode, r *rules.Rules, srcLen uint32) *ASTNode {
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
	for childIdx != 0xFFFFFFFF && int(childIdx) < len(nodes) {
		if child := buildFromIndex(childIdx, nodes, symbols, src, node, r, srcLen); child != nil {
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
