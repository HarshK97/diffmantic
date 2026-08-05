package treesitter

import "sort"

// ASTIndex holds pre-order and post-order arrays for fast subtree lookups.
type ASTIndex struct {
	Nodes      []*ASTNode
	PostOrder  []*ASTNode
	LabelIndex map[string][]int32
}

// EnsureIndex indexes the tree if it hasn't been indexed yet.
func EnsureIndex(root *ASTNode) *ASTIndex {
	if root == nil {
		return nil
	}
	if root.Index != nil && root.PreSize > 0 {
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
		left := sort.Search(len(positions), func(i int) bool { return positions[i] >= start })
		right := sort.Search(len(positions), func(i int) bool { return positions[i] >= end })
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
