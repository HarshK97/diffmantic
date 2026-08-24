package treesitter

import (
	"slices"
	"strings"

	"github.com/odvcencio/gotreesitter"
)

type ASTNode struct {
	Type            string
	Label           string
	Children        []*ASTNode
	Parent          *ASTNode
	StartByte       uint32
	EndByte         uint32
	StartRow        uint32 // 0-indexed
	StartCol        uint32
	EndRow          uint32
	EndCol          uint32
	Language        string // Set on root node only
	HasError        bool   // True if parse tree contained any ERROR nodes
	ParseErrorCount int    // Total count of ERROR nodes in parse tree
	IsKeyword       bool   // True if node is a keyword token
	IsUnordered     bool   // True if children of this container node are order-insensitive

	// Hash is the combined hash of node type, label, and children.
	Hash uint64
	// StructureHash is the shape hash, ignoring leaf labels.
	StructureHash uint64

	ID        int32
	PreSize   int32
	PostStart int32
	Index     *ASTIndex
}

const (
	fnvOffset = 14695981039346656037
	fnvPrime  = 1099511628211
)

func hashString(h uint64, s string) uint64 {
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return h
}

// ComputeHashes fills Hash and StructureHash for the node and its children.
func (n *ASTNode) ComputeHashes() {
	h := uint64(fnvOffset)
	h = hashString(h, n.Type)
	h = hashString(h, n.Label)

	sh := uint64(fnvOffset)
	sh = hashString(sh, n.Type)

	for _, child := range n.Children {
		child.ComputeHashes()
		h = (h ^ child.Hash) * fnvPrime
		sh = (sh ^ child.StructureHash) * fnvPrime
	}

	n.Hash = h
	n.StructureHash = sh
}

func BuildAST(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode) *ASTNode {
	if n == nil {
		return nil
	}
	rules := GetRules(lang.Name)
	if parent == nil && n.Type(lang) == "ERROR" {
		rootType := "translation_unit"
		if rules != nil && len(rules.Scaffolding) > 0 {
			rootType = rules.Scaffolding[0]
		}
		errCount := countErrorNodes(n, lang)
		root := &ASTNode{
			Type:            rootType,
			StartByte:       n.StartByte(),
			EndByte:         n.EndByte(),
			StartRow:        n.StartPoint().Row,
			StartCol:        n.StartPoint().Column,
			EndRow:          n.EndPoint().Row,
			EndCol:          n.EndPoint().Column,
			Language:        lang.Name,
			HasError:        errCount > 0,
			ParseErrorCount: errCount,
		}
		unwrapErrorNode(n, src, lang, root, rules)
		root.ComputeHashes()
		EnsureIndex(root)
		return root
	}

	node := buildASTWithRules(n, src, lang, parent, rules)
	if node != nil && parent == nil {
		errCount := countErrorNodes(n, lang)
		node.Language = lang.Name
		node.ParseErrorCount = errCount
		node.HasError = errCount > 0
		node.ComputeHashes()
		EnsureIndex(node)
	}
	return node
}

func countErrorNodes(n *gotreesitter.Node, lang *gotreesitter.Language) int {
	if n == nil || !n.HasError() {
		return 0
	}
	count := 0
	if n.Type(lang) == "ERROR" || n.IsError() || n.IsMissing() {
		count = 1
	}
	for i := 0; i < n.ChildCount(); i++ {
		count += countErrorNodes(n.Child(i), lang)
	}
	return count
}

var stringLiteralTypes = []string{
	"string",
	"string_literal",
	"interpreted_string_literal",
	"raw_string_literal",
	"template_string",
	"plain_scalar",
	"string_scalar",
	"double_quote_scalar",
	"single_quote_scalar",
	"block_scalar",
	"regex",
}

func buildASTWithRules(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode, rules *Rules) *ASTNode {
	nodeType := n.Type(lang)
	if nodeType == "ERROR" {
		if parent != nil {
			unwrapErrorNode(n, src, lang, parent, rules)
			return nil
		}
	}

	if rules == nil {
		return nil
	}

	isLeaf := n.ChildCount() == 0 || slices.Contains(stringLiteralTypes, nodeType)
	var label string
	if isLeaf {
		srcLen := uint32(len(src))
		start, end := min(n.StartByte(), srcLen), min(n.EndByte(), srcLen)
		if start > end {
			start = end
		}
		label = strings.TrimSpace(string(src[start:end]))
	}

	if rules.IsIgnored(nodeType, label) {
		return nil
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
	if isLeaf {
		node.Label = label
	}

	if alias, ok := rules.Alias(nodeType, label); ok {
		node.Type = alias
	}
	if rules.IsLabelIgnored(node.Type) {
		node.Label = ""
	}
	if isLeaf && rules.IsKeyword(nodeType, label) {
		node.IsKeyword = true
	}
	if rules.IsUnordered(node.Type) {
		node.IsUnordered = true
	}

	for i := 0; i < n.ChildCount(); i++ {
		if child := buildASTWithRules(n.Child(i), src, lang, node, rules); child != nil {
			node.Children = append(node.Children, child)
		}
	}

	if rules.IsFlattened(nodeType) {
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

func unwrapErrorNode(n *gotreesitter.Node, src []byte, lang *gotreesitter.Language, parent *ASTNode, rules *Rules) {
	for i := 0; i < n.ChildCount(); i++ {
		if child := buildASTWithRules(n.Child(i), src, lang, parent, rules); child != nil {
			parent.Children = append(parent.Children, child)
		}
	}
}

// Size returns the total number of nodes in the subtree rooted at n.
func (n *ASTNode) Size() int {
	if n == nil {
		return 0
	}
	if n.Index != nil && n.PreSize > 0 {
		return int(n.PreSize)
	}
	size := 1
	for _, child := range n.Children {
		size += child.Size()
	}
	return size
}

// Root walks up to the root node of the AST.
func (n *ASTNode) Root() *ASTNode {
	if n == nil {
		return nil
	}
	curr := n
	for curr.Parent != nil {
		curr = curr.Parent
	}
	return curr
}

// GetLanguage walks up to the root to retrieve the AST's language.
func (n *ASTNode) GetLanguage() string {
	root := n.Root()
	if root == nil {
		return ""
	}
	return root.Language
}

// IsBracketOrParen reports whether the node label is a bracket, brace, or parenthesis.
func (n *ASTNode) IsBracketOrParen() bool {
	if n == nil {
		return false
	}
	switch n.Label {
	case "{", "}", "(", ")", "[", "]":
		return true
	}
	return false
}

// IsWordChar checks if c is an ASCII letter, digit, or underscore.
func IsWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
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

// Descendants returns all child nodes under n in pre-order.
func (n *ASTNode) Descendants() []*ASTNode {
	size := n.Size()
	if size <= 1 {
		return []*ASTNode{}
	}
	if n.Index != nil && n.PreSize > 0 {
		return slices.Clone(n.Index.Nodes[n.ID+1 : n.ID+n.PreSize])
	}
	out := make([]*ASTNode, 0, size-1)
	var traverse func(*ASTNode)
	traverse = func(curr *ASTNode) {
		for _, c := range curr.Children {
			out = append(out, c)
			traverse(c)
		}
	}
	traverse(n)
	return out
}

// LeafLabels returns counts of each leaf label in the subtree.
func (n *ASTNode) LeafLabels() map[string]int {
	labels := make(map[string]int)
	if n.Index != nil && n.PreSize > 0 {
		start := int(n.ID)
		end := int(n.ID + n.PreSize)
		for i := start; i < end; i++ {
			d := n.Index.Nodes[i]
			if len(d.Children) == 0 && d.Label != "" {
				labels[d.Label]++
			}
		}
		return labels
	}
	var traverse func(*ASTNode)
	traverse = func(curr *ASTNode) {
		if len(curr.Children) == 0 && curr.Label != "" {
			labels[curr.Label]++
			return
		}
		for _, c := range curr.Children {
			traverse(c)
		}
	}
	traverse(n)
	return labels
}

// PostOrder returns all nodes in children-first order.
func (n *ASTNode) PostOrder() []*ASTNode {
	if n == nil {
		return nil
	}
	if n.Index != nil && n.PreSize > 0 {
		return slices.Clone(n.Index.PostOrder[n.PostStart : n.PostStart+n.PreSize])
	}
	out := make([]*ASTNode, 0, n.Size())
	var traverse func(*ASTNode)
	traverse = func(curr *ASTNode) {
		for _, c := range curr.Children {
			traverse(c)
		}
		out = append(out, curr)
	}
	traverse(n)
	return out
}

// PreOrder returns all nodes in parent-first order.
func (n *ASTNode) PreOrder() []*ASTNode {
	if n == nil {
		return nil
	}
	if n.Index != nil && n.PreSize > 0 {
		return slices.Clone(n.Index.Nodes[n.ID : n.ID+n.PreSize])
	}
	out := make([]*ASTNode, 0, n.Size())
	var traverse func(*ASTNode)
	traverse = func(curr *ASTNode) {
		out = append(out, curr)
		for _, c := range curr.Children {
			traverse(c)
		}
	}
	traverse(n)
	return out
}

// ChildIndex returns the node's position among its parent's children, or -1 if it has no parent.
func (n *ASTNode) ChildIndex() int {
	if n == nil || n.Parent == nil {
		return -1
	}
	return slices.Index(n.Parent.Children, n)
}

// IsLeafOrStringLiteral checks if n is a leaf node or string literal.
func (n *ASTNode) IsLeafOrStringLiteral() bool {
	if n == nil {
		return false
	}
	return len(n.Children) == 0 || slices.Contains(stringLiteralTypes, n.Type)
}

// Leaves returns all leaf nodes (and atomic string literals) in pre-order under n.
func (n *ASTNode) Leaves() []*ASTNode {
	if n == nil {
		return nil
	}
	var leaves []*ASTNode
	if n.Index != nil && n.PreSize > 0 {
		start := int(n.ID)
		end := int(n.ID + n.PreSize)
		for i := start; i < end; i++ {
			node := n.Index.Nodes[i]
			if node.IsLeafOrStringLiteral() {
				leaves = append(leaves, node)
			}
		}
		return leaves
	}
	var traverse func(*ASTNode)
	traverse = func(curr *ASTNode) {
		if curr.IsLeafOrStringLiteral() {
			leaves = append(leaves, curr)
			return
		}
		for _, c := range curr.Children {
			traverse(c)
		}
	}
	traverse(n)
	return leaves
}

// LevelOrder returns all nodes in the subtree level by level (breadth-first).
func (n *ASTNode) LevelOrder() []*ASTNode {
	if n == nil {
		return nil
	}
	size := n.Size()
	out := make([]*ASTNode, 0, size)
	queue := make([]*ASTNode, 0, size)
	queue = append(queue, n)

	head := 0
	for head < len(queue) {
		curr := queue[head]
		head++
		out = append(out, curr)
		queue = append(queue, curr.Children...)
	}
	return out
}

// DepthTo returns the number of parent edges between n and ancestor (or 0 if n is ancestor or ancestor is nil).
func (n *ASTNode) DepthTo(ancestor *ASTNode) int {
	depth := 0
	curr := n
	for curr != nil && curr != ancestor {
		depth++
		curr = curr.Parent
	}
	return depth
}
