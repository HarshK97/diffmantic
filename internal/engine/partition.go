package engine

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// LinePartition partitions AST nodes into untouched lines (Group 1) and edited
// lines (Group 2) using line-level LCS matching ("Beyond GumTree"). This stops
// untouched lines from matching against edited regions across the file.
type LinePartition struct {
	matchedA map[int]int // rowA -> rowB
	matchedB map[int]int // rowB -> rowA
}

func NewLinePartition(srcA, srcB []byte) *LinePartition {
	linesA := strings.Split(string(srcA), "\n")
	linesB := strings.Split(string(srcB), "\n")
	mA := LineDiff(linesA, linesB)
	mB := make(map[int]int, len(mA))
	for rA, rB := range mA {
		mB[rB] = rA
	}
	return &LinePartition{matchedA: mA, matchedB: mB}
}

// IsGroup1A returns true if all lines spanned by n1 in file A are non-edited lines.
func (p *LinePartition) IsGroup1A(n1 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1(p.matchedA, n1)
}

// IsGroup1B returns true if all lines spanned by n2 in file B are non-edited lines.
func (p *LinePartition) IsGroup1B(n2 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1(p.matchedB, n2)
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
