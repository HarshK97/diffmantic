package treesitter

import (
	"fmt"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

type ASTNode struct {
	Type      string
	Label     string
	Children  []*ASTNode
	Parent    *ASTNode
	StartByte uint32
	EndByte   uint32
	StartRow  uint32 // 0-indexed
	StartCol  uint32
	EndRow    uint32
	EndCol    uint32
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	return buildASTWithRules(n, src, lang, parent, GetRules(lang.Name))
}

func buildASTWithRules(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode, rules *Rules) *ASTNode {
	if !n.IsNamed() {
		return nil
	}

	nodeType := n.Type(lang)

	var label string
	if n.NamedChildCount() == 0 {
		label = strings.TrimSpace(string(src[n.StartByte():n.EndByte()]))
	}

	if rules != nil {
		if isIgnored(nodeType, label, rules.Ignored) {
			return nil
		}
	}

	node := &ASTNode{
		Type:      nodeType,
		Parent:    parent,
		StartByte: n.StartByte(),
		EndByte:   n.EndByte(),
		StartRow:  n.StartPoint().Row,
		StartCol:  n.StartPoint().Column,
		EndRow:    n.EndPoint().Row,
		EndCol:    n.EndPoint().Column,
	}

	// label only for leave nodes
	if n.NamedChildCount() == 0 {
		node.Label = label
	}

	if rules != nil {
		if alias, ok := rules.Aliased[nodeType]; ok {
			node.Type = alias
		}
		if alias, ok := rules.Aliased[label]; ok {
			node.Type = alias
		}
		for _, ignoredLabel := range rules.LabelIgnored {
			if node.Type == ignoredLabel {
				node.Label = ""
				break
			}
		}
	}

	for i := 0; i < int(n.ChildCount()); i++ {
		child := buildASTWithRules(n.Child(i), src, lang, node, rules)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	if rules != nil && isFlattened(nodeType, rules.Flattened) {
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

func isIgnored(nodeType, label string, ignored []string) bool {
	for _, ign := range ignored {
		if nodeType == ign || label == ign {
			return true
		}
	}
	return false
}

func isFlattened(nodeType string, flattened []string) bool {
	for _, flat := range flattened {
		if nodeType == flat {
			return true
		}
	}
	return false
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
// Size returns the total number of nodes in the subtree rooted at n.
func (n *ASTNode) Size() int {
	if n == nil {
		return 0
	}
	size := 1
	for _, child := range n.Children {
		size += child.Size()
	}
	return size
}
