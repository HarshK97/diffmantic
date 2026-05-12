package engine

import (
	"math"
	"sort"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// PriorityList is a height-indexed max-priority list.
// Internally we store buckets keyed by height so PeekMax / Pop are O(1)
// and Push is O(log n) via a sorted key list.
type PriorityList struct {
	buckets map[int][]*treesitter.ASTNode
	heights []int
}

func NewPriorityList() *PriorityList {
	return &PriorityList{buckets: make(map[int][]*treesitter.ASTNode)}
}

// Push inserts node n (keyed by its height) into the list.
func Push(n *treesitter.ASTNode, l *PriorityList) {
	h := Height(n)
	if _, exists := l.buckets[h]; !exists {
		// insert height into sorted slice
		idx := sort.SearchInts(l.heights, h)
		l.heights = append(l.heights, 0)
		copy(l.heights[idx+1:], l.heights[idx:])
		l.heights[idx] = h
	}
	l.buckets[h] = append(l.buckets[h], n)
}

// PeekMax returns the maximum height currently in the list.
func PeekMax(l *PriorityList) int {
	if len(l.heights) == 0 {
		return math.MinInt32
	}
	return l.heights[len(l.heights)-1]
}

// Pop removes and returns all nodes whose height equals PeekMax.
func Pop(l *PriorityList) []*treesitter.ASTNode {
	if len(l.heights) == 0 {
		return nil
	}
	maxH := l.heights[len(l.heights)-1]
	l.heights = l.heights[:len(l.heights)-1]
	nodes := l.buckets[maxH]
	delete(l.buckets, maxH)
	return nodes
}

// Open expansd node t into its childrens and pushes each into list.
func Open(t *treesitter.ASTNode, l *PriorityList) {
	for _, c := range t.Children {
		Push(c, l)
	}
}
