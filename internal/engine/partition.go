package engine

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// LinePartition partitions AST nodes into untouched lines (Group 1) and edited
// lines (Group 2) using line-level LCS matching ("Beyond GumTree"). This stops
// untouched lines from matching against edited regions across the file.
type groupResult struct {
	isG1       bool
	startOther int
}

type LinePartition struct {
	matchedA map[int]int // rowA -> rowB
	matchedB map[int]int // rowB -> rowA
	cacheA   map[*treesitter.ASTNode]groupResult
	cacheB   map[*treesitter.ASTNode]groupResult
}

func NewLinePartition(srcA, srcB []byte) *LinePartition {
	linesA := strings.Split(string(srcA), "\n")
	linesB := strings.Split(string(srcB), "\n")
	mA := LineDiff(linesA, linesB)
	mB := make(map[int]int, len(mA))
	for rA, rB := range mA {
		mB[rB] = rA
	}
	return &LinePartition{
		matchedA: mA,
		matchedB: mB,
		cacheA:   make(map[*treesitter.ASTNode]groupResult),
		cacheB:   make(map[*treesitter.ASTNode]groupResult),
	}
}

// IsGroup1A returns true if all lines spanned by n1 in file A are non-edited lines.
func (p *LinePartition) IsGroup1A(n1 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1Cached(n1, p.cacheA, p.matchedA)
}

// IsGroup1B returns true if all lines spanned by n2 in file B are non-edited lines.
func (p *LinePartition) IsGroup1B(n2 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1Cached(n2, p.cacheB, p.matchedB)
}

func (p *LinePartition) isGroup1Cached(n *treesitter.ASTNode, cache map[*treesitter.ASTNode]groupResult, matched map[int]int) (bool, int) {
	if p == nil || n == nil {
		return false, -1
	}
	if res, ok := cache[n]; ok {
		return res.isG1, res.startOther
	}
	isG1, startOther := p.isGroup1(matched, n)
	cache[n] = groupResult{isG1: isG1, startOther: startOther}
	return isG1, startOther
}

func (p *LinePartition) isGroup1(matched map[int]int, n *treesitter.ASTNode) (bool, int) {
	if len(matched) == 0 || n == nil {
		return false, -1
	}
	startRow := int(n.StartRow)
	endRow := int(n.EndRow)
	if startRow < 0 {
		return false, -1
	}
	startOther, ok := matched[startRow]
	if !ok {
		return false, -1
	}
	for r := startRow; r <= endRow; r++ {
		m, ok := matched[r]
		if !ok || m != startOther+(r-startRow) {
			return false, -1
		}
	}
	return true, startOther
}

// CanMatch checks if two nodes are allowed to pair up:
// - Untouched nodes (Group 1) only match untouched nodes on corresponding lines.
// - Edited nodes (Group 2) only match other edited nodes.
// - Cross-matching between untouched and edited nodes is forbidden.
func (p *LinePartition) CanMatch(n1, n2 *treesitter.ASTNode) bool {
	if p == nil || n1 == nil || n2 == nil {
		return true
	}
	isG1A, startB := p.IsGroup1A(n1)
	isG1B, startA := p.IsGroup1B(n2)

	if isG1A && isG1B {
		return int(n2.StartRow) == startB && int(n1.StartRow) == startA
	}
	if isG1A != isG1B {
		return false
	}
	return true
}
