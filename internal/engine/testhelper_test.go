package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// mkNode builds an ASTNode with children and sets Parent pointers.
func mkNode(typ, label string, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	n := &treesitter.ASTNode{Type: typ, Label: label}
	for _, c := range children {
		c.Parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

// mkLeaf builds a leaf ASTNode (no children).
func mkLeaf(typ, label string) *treesitter.ASTNode {
	return &treesitter.ASTNode{Type: typ, Label: label}
}
