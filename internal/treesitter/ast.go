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
	Language  string // Set on root node only
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	node := buildASTWithRules(n, src, lang, parent, GetRules(lang.Name))
	if node != nil && parent == nil {
		node.Language = lang.Name
	}
	return node
}

func isStringLiteralType(t string) bool {
	return t == "string" ||
		t == "string_literal" ||
		t == "interpreted_string_literal" ||
		t == "raw_string_literal" ||
		t == "template_string"
}

func buildASTWithRules(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode, rules *Rules) *ASTNode {
	nodeType := n.Type(lang)

	if !n.IsNamed() {
		isAliased := false
		if rules != nil {
			if _, ok := rules.Aliased[nodeType]; ok {
				isAliased = true
			}
		}
		if !isAliased {
			return nil
		}
	}

	var label string
	if n.ChildCount() == 0 || isStringLiteralType(nodeType) {
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

	// label only for leave nodes or string literals
	if n.ChildCount() == 0 || isStringLiteralType(nodeType) {
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

// GetLanguage walks up to the root to retrieve the language of the AST.
func (n *ASTNode) GetLanguage() string {
	curr := n
	for curr.Parent != nil {
		curr = curr.Parent
	}
	return curr.Language
}

// IsScaffolding checks rules.yml to see if this node type is a variable-arity container.
func (n *ASTNode) IsScaffolding() bool {
	lang := n.GetLanguage()
	rules := GetRules(lang)
	if rules == nil {
		return false
	}
	for _, t := range rules.Scaffolding {
		if n.Type == t {
			return true
		}
	}
	return false
}
