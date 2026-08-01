package treesitter

import (
	"slices"
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

	// Subtree hash (type, label, children) for exact matching.
	Hash uint64
	// Shape hash (type and children) ignoring leaf labels.
	StructureHash uint64
}

const (
	fnvOffset uint64 = 14695981039346656037
	fnvPrime  uint64 = 1099511628211
)

func hashStr(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h = (h ^ uint64(s[i])) * fnvPrime
	}
	return h
}

// ComputeHashes runs a post-order walk to fill Hash and StructureHash for the subtree.
func (n *ASTNode) ComputeHashes() {
	sh := hashStr(fnvOffset, n.Type)
	h := hashStr(sh, n.Label)

	for _, child := range n.Children {
		child.ComputeHashes()
		h = (h ^ child.Hash) * fnvPrime
		sh = (sh ^ child.StructureHash) * fnvPrime
	}

	n.Hash = h
	n.StructureHash = sh
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	node := buildASTWithRules(n, src, lang, parent, GetRules(lang.Name))
	if node != nil && parent == nil {
		node.Language = lang.Name
		node.ComputeHashes()
	}
	return node
}

var stringLiteralTypes = []string{
	"string",
	"string_literal",
	"interpreted_string_literal",
	"raw_string_literal",
	"template_string",
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
	if n.ChildCount() == 0 || slices.Contains(stringLiteralTypes, nodeType) {
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

	// Only set label for leaf nodes or string literals.
	if n.ChildCount() == 0 || slices.Contains(stringLiteralTypes, nodeType) {
		node.Label = label
	}

	if rules != nil {
		if alias, ok := rules.Aliased[nodeType]; ok {
			node.Type = alias
		}
		if alias, ok := rules.Aliased[label]; ok {
			node.Type = alias
		}
		if slices.Contains(rules.LabelIgnored, node.Type) {
			node.Label = ""
		}
	}

	for i := 0; i < n.ChildCount(); i++ {
		child := buildASTWithRules(n.Child(i), src, lang, node, rules)
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	if rules != nil && slices.Contains(rules.Flattened, nodeType) {
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
	return slices.Contains(ignored, nodeType) || slices.Contains(ignored, label)
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

// GetLanguage walks up to the root to retrieve the AST's language.
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
	return slices.Contains(rules.Scaffolding, n.Type)
}
