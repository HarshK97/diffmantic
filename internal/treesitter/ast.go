package treesitter

import (
	"fmt"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

type ASTNode struct {
	Type     string
	Label    string
	Children []*ASTNode
	Parent   *ASTNode
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	if !n.IsNamed() {
		return nil
	}

	node := &ASTNode{
		Type:   n.Type(lang),
		Parent: parent,
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

func PrintAST(n *ASTNode, depth int) {
	if n == nil {
		return
	}
	indent := strings.Repeat("  ", depth)
	if n.Label != "" {
		fmt.Printf("%s(%s) %q\n", indent, n.Type, n.Label)
	} else {
		fmt.Printf("%s(%s)\n", indent, n.Type)
	}
	for _, child := range n.Children {
		PrintAST(child, depth+1)
	}
}
