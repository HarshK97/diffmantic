package treesitter

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// CommentBlock represents an extracted comment and its source position.
type CommentBlock struct {
	Type         string
	Text         string
	StartByte    uint32
	EndByte      uint32
	StartRow     int // 0-indexed
	StartCol     int
	EndRow       int // 0-indexed
	EndCol       int
	ScopeKey     string
	ParentType   string
	ParentStart  uint32
	ParentEnd    uint32
	ParentRow    int
	ParentEndRow int
	Language     string

	// Structural Declaration & Scope Context
	DeclStart     uint32
	DeclEnd       uint32
	DeclType      string
	RelativePath  string
	EnclosingDecl *ASTNode
	AnchorNode    *ASTNode
}

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

	LeadingTrivia  []*CommentBlock
	TrailingTrivia []*CommentBlock
	DanglingTrivia []*CommentBlock

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
	for i := range len(s) {
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

// IsAnonymous reports whether n is an unnamed syntax token like punctuation or a delimiter.
func (n *ASTNode) IsAnonymous() bool {
	if n == nil || len(n.Type) == 0 {
		return false
	}
	return !IsWordChar(n.Type[0])
}

// IsWordChar checks if c is an ASCII letter, digit, or underscore.
func IsWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_'
}

// IsScaffolding reports whether the node is a variable-arity container.
func (n *ASTNode) IsScaffolding() bool {
	if n == nil {
		return false
	}
	return rules.Get(n.GetLanguage()).IsScaffolding(n.Type)
}

// IsWrapper reports whether the node is a syntactic wrapper (e.g. parens, argument list).
func (n *ASTNode) IsWrapper() bool {
	if n == nil {
		return false
	}
	return rules.Get(n.GetLanguage()).IsWrapper(n.Type)
}

// IsBlock reports whether the node is a code block.
func (n *ASTNode) IsBlock() bool {
	if n == nil {
		return false
	}
	return rules.Get(n.GetLanguage()).IsBlock(n.Type)
}

// IsCaseClause reports whether the node is a switch/match/select case clause.
func (n *ASTNode) IsCaseClause() bool {
	if n == nil {
		return false
	}
	return rules.Get(n.GetLanguage()).IsCaseClause(n.Type)
}

// IsStatement reports whether the node represents a statement.
func (n *ASTNode) IsStatement() bool {
	if n == nil {
		return false
	}
	r := rules.Get(n.GetLanguage())
	if r != nil {
		return r.IsStatement(n.Type)
	}
	return rules.IsStatement(n.Type)
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
		for _, d := range n.Index.Nodes[n.ID : n.ID+n.PreSize] {
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
	if len(n.Children) == 0 {
		return true
	}
	if lang := n.GetLanguage(); lang != "" {
		return rules.Get(lang).IsFlattened(n.Type)
	}
	return rules.IsFlattened(n.Type)
}

// Leaves returns all leaf nodes (and atomic string literals) in pre-order under n.
func (n *ASTNode) Leaves() []*ASTNode {
	if n == nil {
		return nil
	}
	var leaves []*ASTNode
	if n.Index != nil && n.PreSize > 0 {
		for _, node := range n.Index.Nodes[n.ID : n.ID+n.PreSize] {
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

// CalleePath extracts the canonical dotted identifier path from a call node's function receiver.
// For selector expressions (os.Exit), it walks leaf tokens and joins with dots.
// For scoped identifiers (std::process::exit), it normalizes :: to dots.
// For bare identifiers (exit, panic), it returns the identifier directly.
func CalleePath(callNode *ASTNode) string {
	if callNode == nil || len(callNode.Children) == 0 {
		return ""
	}
	callee := callNode.Children[0]
	if len(callee.Children) == 0 {
		return callee.Label
	}
	// Fast path for the common 2-leaf case (e.g. os.Exit, fmt.Println).
	if len(callee.Children) == 2 && len(callee.Children[0].Children) == 0 && len(callee.Children[1].Children) == 0 {
		l0, l1 := callee.Children[0].Label, callee.Children[1].Label
		if l0 != "" && l1 != "" && l0 != "." && l0 != "::" && l0 != "->" && l1 != "." && l1 != "::" && l1 != "->" {
			return l0 + "." + l1
		}
	}
	var parts []string
	for _, d := range callee.Descendants() {
		if len(d.Children) == 0 && d.Label != "" && d.Label != "." && d.Label != "::" && d.Label != "->" {
			parts = append(parts, d.Label)
		}
	}
	return strings.Join(parts, ".")
}

// EnclosingContainerDeclaration walks up from n to find the nearest container declaration
// (such as a function, method, class, or struct).
func (n *ASTNode) EnclosingContainerDeclaration(r *rules.Rules) *ASTNode {
	if n == nil {
		return nil
	}
	if r == nil {
		r = rules.Get(n.GetLanguage())
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isDecl := (r != nil && r.IsContainerDeclaration(curr.Type)) ||
			(r == nil && rules.IsContainerDeclaration(curr.Type))
		if isDecl {
			return curr
		}
	}
	return nil
}

// SyntaxTrivia pulls this node's punctuation offsets from the index, returning nil if the node has no captured delimiters or the index isn't built.
func (n *ASTNode) SyntaxTrivia() *SyntaxTriviaBlock {
	if n == nil || n.Index == nil || n.ID < 0 || int(n.ID) >= len(n.Index.SyntaxTrivia) {
		return nil
	}
	b := &n.Index.SyntaxTrivia[n.ID]
	if *b == (SyntaxTriviaBlock{}) {
		return nil
	}
	return b
}
