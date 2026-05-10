package treesitter

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

type ASTNode struct {
	Type     string
	Label    string
	Children []*ASTNode
	Parent   *ASTNode
	SrcNode  *gotreesitter.Node
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	if !n.IsNamed() {
		return nil
	}

	node := &ASTNode{
		Type:    n.Type(lang),
		Parent:  parent,
		SrcNode: n,
	}

	// label only for leave nodes
	if n.ChildCount() == 0 {
		node.Label = strings.TrimSpace(string(src[n.StartByte():n.EndByte()]))
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		child := BuildAST(n.Child(i), src, lang, node)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node
}
