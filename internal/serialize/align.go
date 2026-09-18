package serialize

import (
	"cmp"
	"maps"
	"math"
	"slices"
	"strings"
	"sync"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

const (
	gapOpen   = -15
	gapExtend = -2
	negInf    = -1000000
)

type anchor struct {
	src int
	dst int
}

type moveRange struct {
	sStart, sEnd int
	dStart, dEnd int
	hasDst       bool
}

type alignScratch struct {
	dp      []int
	bigrams [65536]uint8
	touched []uint16
}

var alignScratchPool = sync.Pool{
	New: func() any {
		return &alignScratch{
			dp:      make([]int, 0, 16384),
			touched: make([]uint16, 0, 256),
		}
	},
}

// AlignLines computes the side-by-side alignment grid using exact line diff
// matches and semantic AST node mappings as alignment anchors.
func AlignLines(srcBytes, dstBytes []byte, ms *engine.Mapping, es *actions.EditScript, commentLineMappings ...map[int]int) []LineAlignmentPair {
	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	// If one file is empty, align the other with empty space (fillers).
	if len(srcLines) == 1 && srcLines[0] == "" {
		res := make([]LineAlignmentPair, len(dstLines))
		for j := range dstLines {
			res[j] = LineAlignmentPair{LeftLine: -1, RightLine: j}
		}
		return res
	}
	if len(dstLines) == 1 && dstLines[0] == "" {
		res := make([]LineAlignmentPair, len(srcLines))
		for i := range srcLines {
			res[i] = LineAlignmentPair{LeftLine: i, RightLine: -1}
		}
		return res
	}

	matched := engine.LineDiff(srcLines, dstLines)

	// Mark all moved nodes from EditScript.
	movedNodes := collectMovedNodes(es, ms)

	// Pre-index mapped statements for stationary statements (Priority 1 anchor validation).
	stationaryStatements := collectMappedStatements(srcLines, dstLines, ms, func(n1, n2 *treesitter.ASTNode) bool {
		return !movedNodes[n1] && !movedNodes[n2]
	})

	moves, inPlaceNodes := collectMoveRanges(es, ms, srcLines, matched, stationaryStatements)

	// Declarations and comments form invariant boundary anchors.
	primary := collectDeclarationAndCommentAnchors(srcLines, dstLines, ms, movedNodes, moves, commentLineMappings)

	// Unmoved non-punctuation text matches form pivot anchors.
	text := collectTextAnchors(srcLines, dstLines, moves, stationaryStatements, matched)
	anchors := mergeAnchors(primary, text)

	// In-place monotonic statements receive mapping prior bonus in Gotoh.
	inPlaceStatements := collectMappedStatements(srcLines, dstLines, ms, func(n1, n2 *treesitter.ASTNode) bool {
		return inPlaceNodes[n1] && inPlaceNodes[n2]
	})

	gotohMappedStatements := make(map[int]int, len(stationaryStatements)+len(inPlaceStatements))
	maps.Copy(gotohMappedStatements, stationaryStatements)
	for k, v := range inPlaceStatements {
		if _, exists := gotohMappedStatements[k]; !exists {
			gotohMappedStatements[k] = v
		}
	}

	scratch := alignScratchPool.Get().(*alignScratch)
	defer alignScratchPool.Put(scratch)
	return emitGrid(srcLines, dstLines, anchors, moves, gotohMappedStatements, scratch)
}

func collectMovedNodes(es *actions.EditScript, ms *engine.Mapping) map[*treesitter.ASTNode]bool {
	movedNodes := make(map[*treesitter.ASTNode]bool)
	if es == nil {
		return movedNodes
	}
	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil {
			continue
		}
		markMoved(a.Node, movedNodes, a.Subtree)
		if a.DestNode != nil {
			markMoved(a.DestNode, movedNodes, a.Subtree)
		} else if ms != nil {
			if dst := ms.Get(a.Node); dst != nil {
				markMoved(dst, movedNodes, a.Subtree)
			}
		}
	}
	return movedNodes
}

func collectMoveRanges(es *actions.EditScript, ms *engine.Mapping, srcLines []string, matched map[int]int, stationaryStatements map[int]int) ([]moveRange, map[*treesitter.ASTNode]bool) {
	inPlaceNodes := make(map[*treesitter.ASTNode]bool)
	if es == nil {
		return nil, inPlaceNodes
	}

	var moves []moveRange
	for _, a := range es.Actions() {
		if a.Type != actions.Move || a.Node == nil {
			continue
		}

		var dstNode *treesitter.ASTNode
		if a.DestNode != nil {
			dstNode = a.DestNode
		} else if ms != nil {
			dstNode = ms.Get(a.Node)
		}

		sStart := int(a.Node.StartRow)
		sEnd := int(a.Node.EndRow)

		var dStart, dEnd int
		hasDst := false
		if dstNode != nil {
			dStart = int(dstNode.StartRow)
			dEnd = int(dstNode.EndRow)
			hasDst = true
		}

		r := rules.Get(a.Node.GetLanguage())
		drift := sStart - dStart
		if drift < 0 {
			drift = -drift
		}
		if ms != nil && dstNode != nil {
			drift = ms.AdjustedLineDistance(a.Node, dstNode)
		}
		isCrossScope := dstNode != nil && !isScopePreserved(ms, a.Node, dstNode, r, r)

		inverts := false
		if hasDst {
			for sLine, dLine := range matched {
				if sLine >= 0 && sLine < len(srcLines) {
					sTrim := strings.TrimSpace(srcLines[sLine])
					if sTrim == "" || rules.IsPunctuation(sTrim) {
						continue
					}
				}
				if (sStart < sLine && dStart > dLine) || (sStart > sLine && dStart < dLine) ||
					(sEnd < sLine && dEnd > dLine) || (sEnd > sLine && dEnd < dLine) {
					inverts = true
					break
				}
			}
			if !inverts {
				for sLine, dLine := range stationaryStatements {
					if (sStart < sLine && dStart > dLine) || (sStart > sLine && dStart < dLine) ||
						(sEnd < sLine && dEnd > dLine) || (sEnd > sLine && dEnd < dLine) {
						inverts = true
						break
					}
				}
			}
		}

		isInPlaceMonotonic := hasDst && !isCrossScope && drift <= 5 && !inverts
		if isInPlaceMonotonic {
			markMoved(a.Node, inPlaceNodes, a.Subtree)
			if dstNode != nil {
				markMoved(dstNode, inPlaceNodes, a.Subtree)
			}
		}

		if !isInPlaceMonotonic && (!hasDst || sStart != dStart || sEnd-sStart != dEnd-dStart) {
			isStructural := (r != nil && (r.IsDeclaration(a.Node.Type) || r.IsBlock(a.Node.Type))) || (a.Node.EndRow-a.Node.StartRow >= 2)
			if isCrossScope || isStructural {
				moves = append(moves, moveRange{
					sStart: sStart,
					sEnd:   sEnd,
					dStart: dStart,
					dEnd:   dEnd,
					hasDst: hasDst,
				})
			}
		}
	}
	return moves, inPlaceNodes
}

func collectDeclarationAndCommentAnchors(srcLines, dstLines []string, ms *engine.Mapping, movedNodes map[*treesitter.ASTNode]bool, moves []moveRange, commentLineMappings []map[int]int) []anchor {
	var cands []anchor
	seenSrc := make(map[int]bool)
	seenDst := make(map[int]bool)

	// Declarations from GumTree mapping
	if ms != nil {
		for _, p := range ms.Pairs {
			n1, n2 := p.Src, p.Dst
			if n1 == nil || n2 == nil {
				continue
			}
			if movedNodes[n1] || movedNodes[n2] {
				continue
			}
			var r *rules.Rules
			if lang := n1.GetLanguage(); lang != "" {
				r = rules.Get(lang)
			}
			isDecl := (r != nil && r.IsDeclaration(n1.Type)) || (r == nil && rules.IsDeclaration(n1.Type))
			isDataKind := r != nil && (r.GetKind() == rules.KindData || r.GetKind() == rules.KindMarkup)
			if !isDecl && !isDataKind {
				continue
			}
			if n1.Type != n2.Type && (r == nil || !r.AreTypesEquivalent(n1.Type, n2.Type)) {
				continue
			}
			if !isScopePreserved(ms, n1, n2, r, r) {
				continue
			}

			sRow := int(n1.StartRow)
			dRow := int(n2.StartRow)

			for _, c1 := range n1.Children {
				if c1 == nil {
					continue
				}
				isC1Block := (r != nil && r.IsBlock(c1.Type)) || (r == nil && rules.IsBlock(c1.Type))
				if isC1Block {
					continue
				}
				c2 := ms.Get(c1)
				if c2 != nil && c2.Parent == n2 {
					isC2Block := (r != nil && r.IsBlock(c2.Type)) || (r == nil && rules.IsBlock(c2.Type))
					if !isC2Block {
						sRow = int(c1.StartRow)
						dRow = int(c2.StartRow)
						break
					}
				}
			}

			if sRow < 0 || sRow >= len(srcLines) || dRow < 0 || dRow >= len(dstLines) {
				continue
			}
			if seenSrc[sRow] || seenDst[dRow] {
				continue
			}

			sTrim := strings.TrimSpace(srcLines[sRow])
			dTrim := strings.TrimSpace(dstLines[dRow])
			isPunct := (r != nil && (r.IsPunctuation(sTrim) || r.IsPunctuation(dTrim))) || (r == nil && (rules.IsPunctuation(sTrim) || rules.IsPunctuation(dTrim)))
			if sTrim == "" || dTrim == "" || isPunct {
				continue
			}
			if isDataKind && sTrim != dTrim {
				continue
			}

			seenSrc[sRow] = true
			seenDst[dRow] = true
			cands = append(cands, anchor{src: sRow, dst: dRow})
		}
	}

	// Unmoved comments
	if len(commentLineMappings) > 0 && commentLineMappings[0] != nil {
		for s, d := range commentLineMappings[0] {
			if s < 0 || s >= len(srcLines) || d < 0 || d >= len(dstLines) {
				continue
			}
			if seenSrc[s] || seenDst[d] {
				continue
			}
			if isInsideMove(s, d, moves) {
				continue
			}
			seenSrc[s] = true
			seenDst[d] = true
			cands = append(cands, anchor{src: s, dst: d})
		}
	}

	return filterMonotonic(cands)
}

func collectMappedStatements(srcLines, dstLines []string, ms *engine.Mapping, allowNode func(n1, n2 *treesitter.ASTNode) bool) map[int]int {
	if ms == nil {
		return nil
	}

	mapped := make(map[int]int)
	seenSrc := make(map[int]bool)
	seenDst := make(map[int]bool)

	for _, p := range ms.Pairs {
		n1, n2 := p.Src, p.Dst
		if n1 == nil || n2 == nil {
			continue
		}
		if allowNode != nil && !allowNode(n1, n2) {
			continue
		}
		var r *rules.Rules
		if lang := n1.GetLanguage(); lang != "" {
			r = rules.Get(lang)
		}
		if n1.Type != n2.Type && (r == nil || !r.AreTypesEquivalent(n1.Type, n2.Type)) {
			continue
		}
		if !isScopePreserved(ms, n1, n2, r, r) {
			continue
		}
		var isIgnored bool
		if r != nil {
			isIgnored = r.IsKeyword(n1.Type, n1.Label) || r.IsDelimiter(n1.Type, n1.Label) ||
				r.IsOperatorLiteral(n1.Type) || r.IsIdentifier(n1.Type) ||
				r.IsWrapper(n1.Type) || r.IsBlock(n1.Type) || r.IsFlattened(n1.Type) ||
				r.IsPunctuation(n1.Type) || r.IsPunctuation(n1.Label)
		} else {
			isIgnored = rules.IsKeyword(n1.Type, n1.Label) || rules.IsDelimiter(n1.Type, n1.Label) ||
				rules.IsOperatorLiteral(n1.Type) || rules.IsIdentifier(n1.Type) ||
				rules.IsWrapper(n1.Type) || rules.IsBlock(n1.Type) || rules.IsFlattened(n1.Type) ||
				rules.IsPunctuation(n1.Type) || rules.IsPunctuation(n1.Label)
		}
		if isIgnored {
			continue
		}
		if n1.Parent != nil {
			isParentBlock := (r != nil && r.IsBlock(n1.Parent.Type)) || (r == nil && rules.IsBlock(n1.Parent.Type))
			if !isParentBlock && n1.Parent.StartRow == n1.StartRow && n1.Parent.EndRow == n1.EndRow {
				continue
			}
		}

		sRow := int(n1.StartRow)
		dRow := int(n2.StartRow)

		if sRow >= 0 && sRow < len(srcLines) && dRow >= 0 && dRow < len(dstLines) && !seenSrc[sRow] && !seenDst[dRow] {
			sTrim := strings.TrimSpace(srcLines[sRow])
			dTrim := strings.TrimSpace(dstLines[dRow])
			isPunct := (r != nil && (r.IsPunctuation(sTrim) || r.IsPunctuation(dTrim))) || (r == nil && (rules.IsPunctuation(sTrim) || rules.IsPunctuation(dTrim)))
			if sTrim != "" && dTrim != "" && !isPunct {
				seenSrc[sRow] = true
				seenDst[dRow] = true
				mapped[sRow] = dRow
			}
		}

		if n1.EndRow > n1.StartRow && n2.EndRow > n2.StartRow {
			sEnd := int(n1.EndRow)
			dEnd := int(n2.EndRow)
			if sEnd >= 0 && sEnd < len(srcLines) && dEnd >= 0 && dEnd < len(dstLines) && !seenSrc[sEnd] && !seenDst[dEnd] {
				sEndTrim := strings.TrimSpace(srcLines[sEnd])
				dEndTrim := strings.TrimSpace(dstLines[dEnd])
				if sEndTrim != "" && dEndTrim != "" && sEndTrim == dEndTrim {
					seenSrc[sEnd] = true
					seenDst[dEnd] = true
					mapped[sEnd] = dEnd
				}
			}
		}
	}

	return mapped
}

func collectTextAnchors(srcLines, dstLines []string, moves []moveRange, mappedStatements map[int]int, matched map[int]int) []anchor {
	if matched == nil {
		matched = engine.LineDiff(srcLines, dstLines)
	}
	if len(matched) == 0 {
		return nil
	}

	var cands []anchor
	lastDst := -1
	for i := 0; i < len(srcLines); i++ {
		j, ok := matched[i]
		if !ok || j <= lastDst {
			continue
		}
		if mappedStatements != nil {
			if targetDst, hasMap := mappedStatements[i]; hasMap && targetDst != j {
				continue
			}
			crosses := false
			for sStmt, dStmt := range mappedStatements {
				if (i < sStmt && j > dStmt) || (i > sStmt && j < dStmt) {
					crosses = true
					break
				}
			}
			if crosses {
				continue
			}
		}
		sTrim := strings.TrimSpace(srcLines[i])
		if sTrim == "" || rules.IsPunctuation(sTrim) {
			continue
		}
		if isInsideMove(i, j, moves) {
			continue
		}
		cands = append(cands, anchor{src: i, dst: j})
		lastDst = j
	}
	return cands
}

func mergeAnchors(primary, secondary []anchor) []anchor {
	if len(primary) == 0 {
		return secondary
	}
	if len(secondary) == 0 {
		return primary
	}

	var result []anchor
	secIdx := 0
	lastSrc := -1
	lastDst := -1

	for _, p := range primary {
		for secIdx < len(secondary) && secondary[secIdx].src < p.src {
			s := secondary[secIdx]
			secIdx++
			if s.src > lastSrc && s.dst > lastDst && s.dst < p.dst {
				result = append(result, s)
				lastSrc = s.src
				lastDst = s.dst
			}
		}
		if p.src > lastSrc && p.dst > lastDst {
			result = append(result, p)
			lastSrc = p.src
			lastDst = p.dst
		}
	}

	for secIdx < len(secondary) {
		s := secondary[secIdx]
		secIdx++
		if s.src > lastSrc && s.dst > lastDst {
			result = append(result, s)
			lastSrc = s.src
			lastDst = s.dst
		}
	}

	return result
}

func filterMonotonic(cands []anchor) []anchor {
	if len(cands) == 0 {
		return nil
	}
	// Sort primarily by src ascending, secondarily by dst ascending.
	slices.SortFunc(cands, func(a, b anchor) int {
		if a.src != b.src {
			return cmp.Compare(a.src, b.src)
		}
		return cmp.Compare(a.dst, b.dst)
	})

	// Deduplicate identical (src, dst) pairs.
	dedup := make([]anchor, 0, len(cands))
	for i, c := range cands {
		if i == 0 || c != cands[i-1] {
			dedup = append(dedup, c)
		}
	}
	cands = dedup
	if len(cands) <= 1 {
		return cands
	}

	// Dynamic programming to compute the Longest Increasing Subsequence (LIS)
	// where both src and dst are strictly increasing.
	n := len(cands)
	dp := make([]int, n)
	parent := make([]int, n)
	for i := range dp {
		dp[i] = 1
		parent[i] = -1
	}

	maxLen := 1
	bestEnd := 0

	for i := 0; i < n; i++ {
		for j := 0; j < i; j++ {
			if cands[j].src < cands[i].src && cands[j].dst < cands[i].dst {
				if dp[j]+1 > dp[i] {
					dp[i] = dp[j] + 1
					parent[i] = j
					if dp[i] > maxLen {
						maxLen = dp[i]
						bestEnd = i
					}
				}
			}
		}
	}

	result := make([]anchor, 0, maxLen)
	for curr := bestEnd; curr != -1; curr = parent[curr] {
		result = append(result, cands[curr])
	}
	slices.Reverse(result)
	return result
}

func emitGrid(
	srcLines, dstLines []string,
	anchors []anchor,
	moves []moveRange,
	mappedStatements map[int]int,
	scratch *alignScratch,
) []LineAlignmentPair {
	var grid []LineAlignmentPair
	currSrc := 0
	currDst := 0

	for _, a := range anchors {
		grid = alignGapGotoh(grid, srcLines, dstLines, currSrc, a.src, currDst, a.dst, moves, mappedStatements, scratch)
		grid = append(grid, LineAlignmentPair{LeftLine: a.src, RightLine: a.dst})
		currSrc = a.src + 1
		currDst = a.dst + 1
	}

	return alignGapGotoh(grid, srcLines, dstLines, currSrc, len(srcLines), currDst, len(dstLines), moves, mappedStatements, scratch)
}

const maxGotohMatrixCells = 1000000

func alignGapGotoh(
	grid []LineAlignmentPair,
	srcLines, dstLines []string,
	sStart, sEnd, dStart, dEnd int,
	moves []moveRange,
	mappedStatements map[int]int,
	scratch *alignScratch,
) []LineAlignmentPair {
	srcCount := sEnd - sStart
	dstCount := dEnd - dStart
	if srcCount <= 0 && dstCount <= 0 {
		return grid
	}
	if srcCount <= 0 {
		for k := 0; k < dstCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: -1, RightLine: dStart + k})
		}
		return grid
	}
	if dstCount <= 0 {
		for k := 0; k < srcCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: sStart + k, RightLine: -1})
		}
		return grid
	}

	// For massive gaps (> 1M cells), avoid quadratic DP matrix allocation.
	// Recursively divide the gap using line diff matches as sub-anchors.
	if int64(srcCount)*int64(dstCount) > maxGotohMatrixCells {
		subSrc := srcLines[sStart:sEnd]
		subDst := dstLines[dStart:dEnd]
		matched := engine.LineDiff(subSrc, subDst)
		if len(matched) > 0 {
			var subAnchors []anchor
			lastDst := -1
			for i := 0; i < len(subSrc); i++ {
				j, ok := matched[i]
				if ok && j > lastDst {
					subAnchors = append(subAnchors, anchor{src: sStart + i, dst: dStart + j})
					lastDst = j
				}
			}
			if len(subAnchors) > 0 {
				currSrc := sStart
				currDst := dStart
				for _, sa := range subAnchors {
					grid = alignGapGotoh(grid, srcLines, dstLines, currSrc, sa.src, currDst, sa.dst, moves, mappedStatements, scratch)
					grid = append(grid, LineAlignmentPair{LeftLine: sa.src, RightLine: sa.dst})
					currSrc = sa.src + 1
					currDst = sa.dst + 1
				}
				return alignGapGotoh(grid, srcLines, dstLines, currSrc, sEnd, currDst, dEnd, moves, mappedStatements, scratch)
			}
		}

		// If no sub-anchors exist in this massive gap, emit deletions followed by insertions.
		for k := 0; k < srcCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: sStart + k, RightLine: -1})
		}
		for k := 0; k < dstCount; k++ {
			grid = append(grid, LineAlignmentPair{LeftLine: -1, RightLine: dStart + k})
		}
		return grid
	}

	if srcCount <= 40 || dstCount <= 40 || srcCount*dstCount <= 3600 {
		return alignGapGotohDense(grid, srcLines, dstLines, sStart, sEnd, dStart, dEnd, moves, mappedStatements, scratch)
	}
	return alignGapGotohBanded(grid, srcLines, dstLines, sStart, sEnd, dStart, dEnd, moves, mappedStatements, scratch)
}

const maxPooledDPSize = 262144

func getDPBuffer(scratch *alignScratch, reqSize int) []int {
	if scratch == nil {
		return make([]int, reqSize)
	}
	if reqSize <= maxPooledDPSize {
		if cap(scratch.dp) < reqSize {
			scratch.dp = make([]int, reqSize)
		} else {
			scratch.dp = scratch.dp[:reqSize]
		}
		return scratch.dp
	}
	return make([]int, reqSize)
}

func alignGapGotohDense(
	grid []LineAlignmentPair,
	srcLines, dstLines []string,
	sStart, sEnd, dStart, dEnd int,
	moves []moveRange,
	mappedStatements map[int]int,
	scratch *alignScratch,
) []LineAlignmentPair {
	m := sEnd - sStart
	n := dEnd - dStart
	stride := n + 1
	layerSize := (m + 1) * (n + 1)
	reqSize := 3 * layerSize

	dp := getDPBuffer(scratch, reqSize)

	offM := 0
	offIx := layerSize
	offIy := 2 * layerSize

	dp[offM+0] = 0
	dp[offIx+0] = negInf
	dp[offIy+0] = negInf

	for i := 1; i <= m; i++ {
		idx := i*stride + 0
		dp[offM+idx] = negInf
		dp[offIx+idx] = gapOpen + (i-1)*gapExtend
		dp[offIy+idx] = negInf
	}

	for j := 1; j <= n; j++ {
		idx := 0*stride + j
		dp[offM+idx] = negInf
		dp[offIx+idx] = negInf
		dp[offIy+idx] = gapOpen + (j-1)*gapExtend
	}

	for i := 1; i <= m; i++ {
		sIdx := sStart + i - 1
		sLine := srcLines[sIdx]
		rowIdx := i * stride
		prevRowIdx := (i - 1) * stride

		for j := 1; j <= n; j++ {
			dIdx := dStart + j - 1
			dLine := dstLines[dIdx]
			score := lineMatchScore(sLine, dLine, sIdx, dIdx, moves, mappedStatements, scratch)

			prevM := dp[offM+prevRowIdx+j-1]
			prevIx := dp[offIx+prevRowIdx+j-1]
			prevIy := dp[offIy+prevRowIdx+j-1]
			maxDiag := max(prevM, prevIx, prevIy)
			dp[offM+rowIdx+j] = maxDiag + score

			fromM_Ix := dp[offM+prevRowIdx+j] + gapOpen
			fromIx_Ix := dp[offIx+prevRowIdx+j] + gapExtend
			fromIy_Ix := dp[offIy+prevRowIdx+j] + gapOpen
			dp[offIx+rowIdx+j] = max(fromM_Ix, fromIx_Ix, fromIy_Ix)

			fromM_Iy := dp[offM+rowIdx+j-1] + gapOpen
			fromIx_Iy := dp[offIx+rowIdx+j-1] + gapOpen
			fromIy_Iy := dp[offIy+rowIdx+j-1] + gapExtend
			dp[offIy+rowIdx+j] = max(fromM_Iy, fromIx_Iy, fromIy_Iy)
		}
	}

	lastIdx := m*stride + n
	valM := dp[offM+lastIdx]
	valIx := dp[offIx+lastIdx]
	valIy := dp[offIy+lastIdx]
	bestVal := max(valM, valIx, valIy)

	var currState int
	if valIy == bestVal && n > 0 {
		currState = 2
	} else if valIx == bestVal && m > 0 {
		currState = 1
	} else if valM == bestVal && m > 0 && n > 0 {
		currState = 0
	} else {
		currState = selectState(valM, valIx, valIy, m, n)
	}

	temp := make([]LineAlignmentPair, 0, m+n)
	i := m
	j := n

	for i > 0 || j > 0 {
		switch currState {
		case 0:
			temp = append(temp, LineAlignmentPair{LeftLine: sStart + i - 1, RightLine: dStart + j - 1})
			prevIdx := (i-1)*stride + (j - 1)
			pM := dp[offM+prevIdx]
			pIx := dp[offIx+prevIdx]
			pIy := dp[offIy+prevIdx]
			currState = selectState(pM, pIx, pIy, i-1, j-1)
			i--
			j--
		case 1:
			temp = append(temp, LineAlignmentPair{LeftLine: sStart + i - 1, RightLine: -1})
			if j == 0 {
				i--
				continue
			}
			prevIdx := (i-1)*stride + j
			val := dp[offIx+i*stride+j]
			switch val {
			case dp[offIx+prevIdx] + gapExtend:
				currState = 1
			case dp[offM+prevIdx] + gapOpen:
				currState = 0
			default:
				currState = 2
			}
			i--
		case 2:
			temp = append(temp, LineAlignmentPair{LeftLine: -1, RightLine: dStart + j - 1})
			if i == 0 {
				j--
				continue
			}
			prevIdx := i*stride + (j - 1)
			val := dp[offIy+i*stride+j]
			switch val {
			case dp[offIy+prevIdx] + gapExtend:
				currState = 2
			case dp[offIx+prevIdx] + gapOpen:
				currState = 1
			default:
				currState = 0
			}
			j--
		}
	}

	slices.Reverse(temp)
	return append(grid, temp...)
}

func alignGapGotohBanded(
	grid []LineAlignmentPair,
	srcLines, dstLines []string,
	sStart, sEnd, dStart, dEnd int,
	moves []moveRange,
	mappedStatements map[int]int,
	scratch *alignScratch,
) []LineAlignmentPair {
	m := sEnd - sStart
	n := dEnd - dStart
	stride := n + 1
	layerSize := (m + 1) * (n + 1)
	reqSize := 3 * layerSize

	dp := getDPBuffer(scratch, reqSize)

	offM := 0
	offIx := layerSize
	offIy := 2 * layerSize
	for k := 0; k < reqSize; k++ {
		dp[k] = negInf
	}
	dp[offM+0] = 0
	for i := 1; i <= m; i++ {
		dp[offIx+i*stride] = gapOpen + (i-1)*gapExtend
	}
	for j := 1; j <= n; j++ {
		dp[offIy+j] = gapOpen + (j-1)*gapExtend
	}

	diff := m - n
	if diff < 0 {
		diff = -diff
	}
	k := max(64, diff/2+32)
	alpha := float64(n) / float64(m)

	for i := 1; i <= m; i++ {
		sIdx := sStart + i - 1
		sLine := srcLines[sIdx]
		rowIdx := i * stride

		jMinCurr, jMaxCurr := bandBounds(i, n, k, alpha)

		for j := jMinCurr; j <= jMaxCurr; j++ {
			dIdx := dStart + j - 1
			dLine := dstLines[dIdx]
			score := lineMatchScore(sLine, dLine, sIdx, dIdx, moves, mappedStatements, scratch)

			// Diagonal match: sIdx vs dIdx
			diagIdx := (i-1)*stride + (j - 1)
			prevM := dp[offM+diagIdx]
			prevIx := dp[offIx+diagIdx]
			prevIy := dp[offIy+diagIdx]
			maxDiag := max(prevM, prevIx, prevIy)
			dp[offM+rowIdx+j] = maxDiag + score

			// Vertical transition: gap in destination
			upIdx := (i-1)*stride + j
			upM := dp[offM+upIdx]
			upIx := dp[offIx+upIdx]
			upIy := dp[offIy+upIdx]
			fromM_Ix := upM + gapOpen
			fromIx_Ix := upIx + gapExtend
			fromIy_Ix := upIy + gapOpen
			dp[offIx+rowIdx+j] = max(fromM_Ix, fromIx_Ix, fromIy_Ix)

			// Horizontal transition: gap in source
			leftIdx := rowIdx + (j - 1)
			leftM := dp[offM+leftIdx]
			leftIx := dp[offIx+leftIdx]
			leftIy := dp[offIy+leftIdx]
			fromM_Iy := leftM + gapOpen
			fromIx_Iy := leftIx + gapOpen
			fromIy_Iy := leftIy + gapExtend
			dp[offIy+rowIdx+j] = max(fromM_Iy, fromIx_Iy, fromIy_Iy)
		}
	}

	lastIdx := m*stride + n
	valM := dp[offM+lastIdx]
	valIx := dp[offIx+lastIdx]
	valIy := dp[offIy+lastIdx]
	bestVal := max(valM, valIx, valIy)

	var currState int
	if valIy == bestVal && n > 0 {
		currState = 2
	} else if valIx == bestVal && m > 0 {
		currState = 1
	} else if valM == bestVal && m > 0 && n > 0 {
		currState = 0
	} else {
		currState = selectState(valM, valIx, valIy, m, n)
	}

	temp := make([]LineAlignmentPair, 0, m+n)
	i := m
	j := n

	for i > 0 || j > 0 {
		// If the optimal path hugs the artificial band boundary on an internal cell, fall back to dense DP.
		if i > 0 && i < m && j > 0 && j < n {
			center := float64(i) * alpha
			rawMin := int(math.Floor(center)) - k
			rawMax := int(math.Ceil(center)) + k
			if (rawMin > 1 && j == rawMin) || (rawMax < n && j == rawMax) {
				return alignGapGotohDense(grid, srcLines, dstLines, sStart, sEnd, dStart, dEnd, moves, mappedStatements, scratch)
			}
		}

		switch currState {
		case 0:
			temp = append(temp, LineAlignmentPair{LeftLine: sStart + i - 1, RightLine: dStart + j - 1})
			prevIdx := (i-1)*stride + (j - 1)
			pM := dp[offM+prevIdx]
			pIx := dp[offIx+prevIdx]
			pIy := dp[offIy+prevIdx]
			currState = selectState(pM, pIx, pIy, i-1, j-1)
			i--
			j--
		case 1:
			temp = append(temp, LineAlignmentPair{LeftLine: sStart + i - 1, RightLine: -1})
			if j == 0 {
				i--
				continue
			}
			prevIdx := (i-1)*stride + j
			val := dp[offIx+i*stride+j]
			switch val {
			case dp[offIx+prevIdx] + gapExtend:
				currState = 1
			case dp[offM+prevIdx] + gapOpen:
				currState = 0
			default:
				currState = 2
			}
			i--
		case 2:
			temp = append(temp, LineAlignmentPair{LeftLine: -1, RightLine: dStart + j - 1})
			if i == 0 {
				j--
				continue
			}
			prevIdx := i*stride + (j - 1)
			val := dp[offIy+i*stride+j]
			switch val {
			case dp[offIy+prevIdx] + gapExtend:
				currState = 2
			case dp[offIx+prevIdx] + gapOpen:
				currState = 1
			default:
				currState = 0
			}
			j--
		}
	}

	slices.Reverse(temp)
	return append(grid, temp...)
}

func bandBounds(i, n, k int, alpha float64) (int, int) {
	if i <= 1 {
		return 1, min(n, int(math.Ceil(alpha))+k)
	}
	center := float64(i) * alpha
	minCol := max(1, int(math.Floor(center))-k)
	maxCol := min(n, int(math.Ceil(center))+k)
	return minCol, maxCol
}

func selectState(mVal, ixVal, iyVal, i, j int) int {
	best := max(mVal, ixVal, iyVal)
	if i == j {
		if mVal == best && i > 0 && j > 0 {
			return 0
		}
		if iyVal == best && j > 0 {
			return 2
		}
		if ixVal == best && i > 0 {
			return 1
		}
		return 0
	}
	if i > j {
		if ixVal == best && iyVal != best && i > 0 {
			return 1
		}
		if iyVal == best && j > 0 {
			return 2
		}
		if mVal == best && i > 0 && j > 0 {
			return 0
		}
		return 1
	}
	// j > i
	if iyVal == best && j > 0 {
		return 2
	}
	if mVal == best && i > 0 && j > 0 {
		return 0
	}
	if ixVal == best && i > 0 {
		return 1
	}
	return 2
}

func lineMatchScore(
	sLine, dLine string,
	sIdx, dIdx int,
	moves []moveRange,
	mappedStatements map[int]int,
	scratch *alignScratch,
) int {
	// Suppress diagonal match for lines moved across scopes
	if isInsideMove(sIdx, dIdx, moves) {
		return -1000
	}

	bonus := 0
	if targetDst, ok := mappedStatements[sIdx]; ok {
		if targetDst == dIdx {
			bonus = 35
		} else {
			bonus = -20
		}
	}

	sTrim := strings.TrimSpace(sLine)
	dTrim := strings.TrimSpace(dLine)
	if sTrim == "" && dTrim == "" {
		return 10
	}
	if sTrim == "" || dTrim == "" {
		return -20
	}

	// Exact Text Match + Indentation
	if sLine == dLine {
		if len(sTrim) <= 5 && rules.IsPunctuation(sTrim) {
			return 2 + bonus
		}
		return 30 + bonus
	}

	// Exact Text Match (Ignoring Indentation)
	if sTrim == dTrim {
		if len(sTrim) <= 5 && rules.IsPunctuation(sTrim) {
			return 2 + bonus
		}
		return 25 + bonus
	}

	// AST Statement / Expression Mapping Prior (when text differed slightly)
	if bonus > 0 {
		return bonus
	}

	// Continuous Edit Similarity (No rigid cliff-edges)
	sim := calculateSimilarity(sTrim, dTrim, scratch)
	return int(math.Round(20.0*sim)) - 10 + bonus
}

func calculateSimilarity(a, b string, scratch *alignScratch) float64 {
	if a == b {
		return 1.0
	}
	la := len(a)
	lb := len(b)
	if la < 2 || lb < 2 {
		return 0.0
	}

	// Skip bigram comparison early if line lengths differ by more than 3x.
	if la > 3*lb || lb > 3*la {
		return 0.0
	}

	nA := la - 1
	nB := lb - 1
	scratch.touched = scratch.touched[:0]

	// Accumulate bigrams from string A
	for i := 0; i < nA; i++ {
		bg := uint16(a[i])<<8 | uint16(a[i+1])
		if scratch.bigrams[bg] < 255 {
			scratch.bigrams[bg]++
		}
		scratch.touched = append(scratch.touched, bg)
	}

	// Intersect with string B
	intersection := 0
	for j := 0; j < nB; j++ {
		bg := uint16(b[j])<<8 | uint16(b[j+1])
		if scratch.bigrams[bg] > 0 {
			scratch.bigrams[bg]--
			intersection++
		}
	}

	// Fast rollback (zero memset overhead)
	for _, bg := range scratch.touched {
		scratch.bigrams[bg] = 0
	}

	return 2.0 * float64(intersection) / float64(nA+nB)
}

func isInsideMove(s, d int, moves []moveRange) bool {
	for _, m := range moves {
		if s >= m.sStart && s <= m.sEnd {
			return true
		}
		if m.hasDst && d >= m.dStart && d <= m.dEnd {
			return true
		}
	}
	return false
}

func isScopePreserved(ms *engine.Mapping, n1, n2 *treesitter.ASTNode, r1, r2 *rules.Rules) bool {
	if ms == nil || n1 == nil || n2 == nil {
		return false
	}
	s1 := findEnclosingScope(n1, r1)
	s2 := findEnclosingScope(n2, r2)
	if s1 == nil && s2 == nil {
		return true
	}
	if s1 != nil && s2 != nil && ms.Get(s1) == s2 {
		return true
	}
	lineDist := int(n1.StartRow) - int(n2.StartRow)
	if lineDist < 0 {
		lineDist = -lineDist
	}
	if lineDist <= 5 && isSameEnclosingDeclaration(ms, n1, n2, r1) {
		return true
	}
	return false
}

func isSameEnclosingDeclaration(ms *engine.Mapping, n1, n2 *treesitter.ASTNode, r *rules.Rules) bool {
	if ms == nil || n1 == nil || n2 == nil {
		return false
	}
	d1 := n1.EnclosingContainerDeclaration(r)
	d2 := n2.EnclosingContainerDeclaration(r)
	if d1 == nil && d2 == nil {
		return true
	}
	if d1 != nil && d2 != nil {
		return ms.Get(d1) == d2
	}
	return false
}

func findEnclosingScope(n *treesitter.ASTNode, r *rules.Rules) *treesitter.ASTNode {
	if r == nil && n != nil {
		r = rules.Get(n.GetLanguage())
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isScope := (r != nil && (r.IsBlock(curr.Type) || r.IsDeclaration(curr.Type) || r.IsWrapper(curr.Type))) ||
			(r == nil && (rules.IsBlock(curr.Type) || rules.IsDeclaration(curr.Type) || rules.IsWrapper(curr.Type))) ||
			curr.Parent == nil
		if isScope {
			return curr
		}
	}
	return nil
}

func markMoved(n *treesitter.ASTNode, m map[*treesitter.ASTNode]bool, subtree bool) {
	if n == nil {
		return
	}
	m[n] = true
	if subtree {
		for _, c := range n.Children {
			markMoved(c, m, true)
		}
	}
}
