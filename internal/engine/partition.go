package engine

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// LinePartition partitions AST nodes into untouched lines (Group 1) and edited
// lines (Group 2) using line-level LCS matching ("Beyond GumTree"). This stops
// untouched lines from matching against edited regions across the file.
type groupResult struct {
	isG1       bool
	startOther int
}

type LinePartition struct {
	matchedA     map[int]int // rowA -> rowB
	matchedB     map[int]int // rowB -> rowA
	substantiveA []bool
	substantiveB []bool
	scopeA       []string // rowA -> declaration scope name
	scopeB       []string // rowB -> declaration scope name
	cacheA       map[*treesitter.ASTNode]groupResult
	cacheB       map[*treesitter.ASTNode]groupResult
}

func NewLinePartition(srcA, srcB []byte) *LinePartition {
	linesA := strings.Split(string(srcA), "\n")
	linesB := strings.Split(string(srcB), "\n")
	mA := LineDiff(linesA, linesB)
	mB := make(map[int]int, len(mA))
	for rA, rB := range mA {
		mB[rB] = rA
	}
	subA := make([]bool, len(linesA))
	for i, s := range linesA {
		subA[i] = lineWeight(s) > trivialLineWeight
	}
	subB := make([]bool, len(linesB))
	for j, s := range linesB {
		subB[j] = lineWeight(s) > trivialLineWeight
	}
	return &LinePartition{
		matchedA:     mA,
		matchedB:     mB,
		substantiveA: subA,
		substantiveB: subB,
		cacheA:       make(map[*treesitter.ASTNode]groupResult),
		cacheB:       make(map[*treesitter.ASTNode]groupResult),
	}
}

// AttachRoots records declaration scopes so lines moved across functions
// aren't treated as untouched stationary code (Group 1).
func (p *LinePartition) AttachRoots(rootA, rootB *treesitter.ASTNode) {
	if p == nil {
		return
	}
	buildScopes := func(root *treesitter.ASTNode, lineCount int) []string {
		if root == nil || lineCount <= 0 {
			return nil
		}
		scopes := make([]string, lineCount)
		r := rulesFor(root)
		for _, n := range root.PreOrder() {
			var name string
			isContainer := (r != nil && r.IsContainerDeclaration(n.Type)) || (r == nil && rules.IsContainerDeclaration(n.Type))
			isClosureAssign := ((r != nil && (r.IsLocalVarDeclaration(n.Type) || r.IsAssignment(n.Type))) ||
				(r == nil && (rules.IsLocalVarDeclaration(n.Type) || rules.IsAssignment(n.Type)))) &&
				hasClosureChild(n, r)
			if isContainer {
				name = getDeclarationName(n)
			} else if isClosureAssign {
				name = getClosureDeclarationName(n, r)
			}
			if name != "" {
				start := max(0, int(n.StartRow))
				end := min(lineCount-1, int(n.EndRow))
				for row := start; row <= end; row++ {
					scopes[row] = name
				}
			}
		}
		return scopes
	}
	p.scopeA = buildScopes(rootA, len(p.substantiveA))
	p.scopeB = buildScopes(rootB, len(p.substantiveB))
	p.cacheA = make(map[*treesitter.ASTNode]groupResult)
	p.cacheB = make(map[*treesitter.ASTNode]groupResult)
}

// IsGroup1A returns true if all lines spanned by n1 in file A are non-edited lines.
func (p *LinePartition) IsGroup1A(n1 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1Cached(n1, p.cacheA, p.matchedA, p.substantiveA, true)
}

// IsGroup1B returns true if all lines spanned by n2 in file B are non-edited lines.
func (p *LinePartition) IsGroup1B(n2 *treesitter.ASTNode) (bool, int) {
	if p == nil {
		return false, -1
	}
	return p.isGroup1Cached(n2, p.cacheB, p.matchedB, p.substantiveB, false)
}

func (p *LinePartition) isGroup1Cached(n *treesitter.ASTNode, cache map[*treesitter.ASTNode]groupResult, matched map[int]int, substantive []bool, isA bool) (bool, int) {
	if p == nil || n == nil {
		return false, -1
	}
	if res, ok := cache[n]; ok {
		return res.isG1, res.startOther
	}
	isG1, startOther := p.isGroup1(matched, substantive, n, isA)
	cache[n] = groupResult{isG1: isG1, startOther: startOther}
	return isG1, startOther
}

func (p *LinePartition) isGroup1(matched map[int]int, substantive []bool, n *treesitter.ASTNode, isA bool) (bool, int) {
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
	if len(p.scopeA) > 0 && len(p.scopeB) > 0 {
		var sOwn, sOther string
		if isA {
			if startRow < len(p.scopeA) {
				sOwn = p.scopeA[startRow]
			}
			if startOther < len(p.scopeB) {
				sOther = p.scopeB[startOther]
			}
		} else {
			if startRow < len(p.scopeB) {
				sOwn = p.scopeB[startRow]
			}
			if startOther < len(p.scopeA) {
				sOther = p.scopeA[startOther]
			}
		}
		if (sOwn != "" || sOther != "") && sOwn != sOther {
			return false, -1
		}
	}
	// Single-line matches with only trivial neighbors (like braces) are usually accidental
	// collisions in an edited region, so keep them in Group 2.
	if startRow == endRow && len(matched) > 1 {
		if !hasSubstantiveNeighborInRun(matched, substantive, startRow, startOther) {
			return false, -1
		}
	}
	return true, startOther
}

// hasSubstantiveNeighborInRun reports whether the contiguous matched run containing
// startRow includes at least one substantive line.
func hasSubstantiveNeighborInRun(matched map[int]int, substantive []bool, startRow, startOther int) bool {
	isSub := func(r int) bool {
		if r < 0 || r >= len(substantive) {
			return false
		}
		return substantive[r]
	}
	for r := startRow - 1; r >= 0; r-- {
		m, ok := matched[r]
		if !ok || m != startOther-(startRow-r) {
			break
		}
		if isSub(r) {
			return true
		}
	}
	for r := startRow + 1; r < len(substantive); r++ {
		m, ok := matched[r]
		if !ok || m != startOther+(r-startRow) {
			break
		}
		if isSub(r) {
			return true
		}
	}
	return false
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
