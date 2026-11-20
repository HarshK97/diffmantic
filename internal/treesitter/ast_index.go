package treesitter

import "slices"

// SyntaxTriviaBlock holds CST punctuation byte spans (brackets, braces, separators)
// so diffing can absorb delimiters without adding synthetic nodes to the AST.
// All byte offsets are 0-indexed in the source file buffer.
type SyntaxTriviaBlock struct {
	OpeningStart  uint32
	OpeningEnd    uint32
	ClosingStart  uint32
	ClosingEnd    uint32
	TrailingStart uint32
	TrailingEnd   uint32
}

// ASTIndex holds pre-order and post-order arrays for fast subtree lookups.
type ASTIndex struct {
	Nodes        []*ASTNode
	PostOrder    []*ASTNode
	LabelIndex   map[string][]int32
	SyntaxTrivia []SyntaxTriviaBlock // Indexed by ASTNode.ID; kept out of ASTNode to protect its 240-byte cache footprint.
	ChildFields  []uint16            // Indexed by ASTNode.ID; zero means an unknown or unfielded parent edge.
	Fields       []string            // Read-only language field names indexed by Tree-sitter field ID; Fields[0] is empty.
}

// EnsureIndex indexes the tree if it hasn't been indexed yet.
func EnsureIndex(root *ASTNode) *ASTIndex {
	if root == nil {
		return nil
	}
	if root.Index != nil && root.PreSize > 0 {
		if root.Index.SyntaxTrivia == nil {
			root.Index.SyntaxTrivia = make([]SyntaxTriviaBlock, len(root.Index.Nodes))
		}
		if root.Index.ChildFields == nil {
			root.Index.ChildFields = make([]uint16, len(root.Index.Nodes))
		}
		return root.Index
	}

	idx := &ASTIndex{
		LabelIndex: make(map[string][]int32),
	}

	// 1. Assign PreOrder IDs
	var preOrder []*ASTNode
	var walkPre func(*ASTNode) int32
	walkPre = func(n *ASTNode) int32 {
		n.ID = int32(len(preOrder))
		n.Index = idx
		preOrder = append(preOrder, n)
		size := int32(1)
		for _, child := range n.Children {
			size += walkPre(child)
		}
		n.PreSize = size
		return size
	}
	walkPre(root)
	idx.Nodes = preOrder
	idx.SyntaxTrivia = make([]SyntaxTriviaBlock, len(preOrder))
	idx.ChildFields = make([]uint16, len(preOrder))

	// 2. Assign PostOrder
	var postOrder []*ASTNode
	var walkPost func(*ASTNode)
	walkPost = func(n *ASTNode) {
		n.PostStart = int32(len(postOrder))
		for _, child := range n.Children {
			walkPost(child)
		}
		postOrder = append(postOrder, n)
	}
	walkPost(root)
	idx.PostOrder = postOrder

	// 3. Build LabelIndex
	for pos, node := range postOrder {
		if len(node.Children) == 0 && node.Label != "" {
			idx.LabelIndex[node.Label] = append(idx.LabelIndex[node.Label], int32(pos))
		}
	}

	return idx
}

// FieldName returns the Tree-sitter field name on n's parent edge, or "" if
// the node is nil, synthetic, unindexed, or has no known field.
func (n *ASTNode) FieldName() string {
	if n == nil || n.Index == nil || n.ID < 0 || int(n.ID) >= len(n.Index.ChildFields) {
		return ""
	}
	id := n.Index.ChildFields[n.ID]
	if id == 0 || int(id) >= len(n.Index.Fields) {
		return ""
	}
	return n.Index.Fields[id]
}

// IsMutuallyExclusiveField reports whether two edge fields represent conflicting
// branches (like consequence vs alternative).
func IsMutuallyExclusiveField(f1, f2 string) bool {
	if f1 == "" || f2 == "" || f1 == f2 {
		return false
	}
	if f1 > f2 {
		f1, f2 = f2, f1
	}
	switch f1 {
	case "alternative":
		return f2 == "body" || f2 == "condition" || f2 == "consequence"
	case "argument", "arguments":
		return f2 == "function"
	case "body":
		return f2 == "condition" || f2 == "consequence" || f2 == "name" || f2 == "parameters"
	case "condition":
		return f2 == "consequence"
	case "key":
		return f2 == "value"
	case "left":
		return f2 == "right"
	case "name":
		return f2 == "parameters"
	}
	return false
}

// Contains returns true if candidate is a descendant of n.
func (n *ASTNode) Contains(candidate *ASTNode) bool {
	if n == nil || candidate == nil || n == candidate {
		return false
	}
	if n.Index != nil && candidate.Index == n.Index && n.PreSize > 0 {
		return candidate.ID > n.ID && candidate.ID < n.ID+n.PreSize
	}
	for curr := candidate.Parent; curr != nil; curr = curr.Parent {
		if curr == n {
			return true
		}
	}
	return false
}

// FrequencyInSubtree returns how many times a leaf label appears in n's subtree.
func (n *ASTNode) FrequencyInSubtree(label string) int {
	if n == nil || label == "" {
		return 0
	}
	if n.Index != nil && n.PreSize > 0 {
		positions := n.Index.LabelIndex[label]
		if len(positions) == 0 {
			return 0
		}
		start := n.PostStart
		end := start + n.PreSize
		left, _ := slices.BinarySearch(positions, start)
		right, _ := slices.BinarySearch(positions, end)
		return right - left
	}
	if len(n.Children) == 0 {
		if n.Label == label {
			return 1
		}
		return 0
	}
	count := 0
	for _, child := range n.Children {
		count += child.FrequencyInSubtree(label)
	}
	return count
}
