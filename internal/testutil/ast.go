// Package testutil provides shared test helpers for building AST nodes
// across all diffmantic test packages.
package testutil

import "github.com/HarshK97/diffmantic/internal/treesitter"

// Node builds an ASTNode and links its children.
func Node(typ, label string, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	n := &treesitter.ASTNode{Type: typ, Label: label}
	for _, c := range children {
		c.Parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

// Leaf creates a node with no children.
func Leaf(typ, label string) *treesitter.ASTNode {
	return &treesitter.ASTNode{Type: typ, Label: label}
}

// NodeAt creates a node with start and end bytes.
func NodeAt(typ, label string, startByte, endByte uint32) *treesitter.ASTNode {
	return &treesitter.ASTNode{
		Type:      typ,
		Label:     label,
		StartByte: startByte,
		EndByte:   endByte,
	}
}

// NodeAtRC creates a node with row and column info.
func NodeAtRC(typ, label string, row, col uint32) *treesitter.ASTNode {
	return &treesitter.ASTNode{
		Type:     typ,
		Label:    label,
		StartRow: row,
		StartCol: col,
		EndRow:   row,
		EndCol:   col + uint32(len(label)),
	}
}

// Tree links children to their parent and returns it.
func Tree(parent *treesitter.ASTNode, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	parent.Children = children
	for _, c := range children {
		c.Parent = parent
	}
	return parent
}
