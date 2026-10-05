package comments

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// DiffResult holds actions produced from diffing comments.
type DiffResult struct {
	Actions      []actions.Action
	LineMappings map[int]int
}

func canonicalScopeKey(c *CommentBlock, mappings *engine.Mapping, isSource bool) string {
	if c == nil {
		return "root"
	}
	if c.ScopeKey != "" {
		return c.ScopeKey
	}
	if c.EnclosingDecl == nil {
		if c.RelativePath != "" {
			return "root:" + c.RelativePath
		}
		return "root"
	}

	if isSource {
		if mappings != nil && mappings.Src() != nil {
			if dstDecl, ok := mappings.Src()[c.EnclosingDecl]; ok && dstDecl != nil {
				return fmt.Sprintf("decl_%d:%s", dstDecl.ID, c.RelativePath)
			}
		}
		// Keep unmapped source declarations isolated so their comments don't leak into new declarations.
		return fmt.Sprintf("unmapped_src_%d:%s", c.EnclosingDecl.ID, c.RelativePath)
	}

	return fmt.Sprintf("decl_%d:%s", c.EnclosingDecl.ID, c.RelativePath)
}

// commentCandidate scores an unmapped source/destination comment pair for fuzzy matching.
type commentCandidate struct {
	i           int
	j           int
	sim         float64
	dist        int
	exactAnchor bool
}

// DiffComments matches and diffs comments between source and destination files using AST mapping awareness.
func DiffComments(srcComments, dstComments []CommentBlock, mappings *engine.Mapping) *DiffResult {
	res := &DiffResult{
		LineMappings: make(map[int]int),
	}
	if len(srcComments) == 0 && len(dstComments) == 0 {
		return res
	}

	srcMatched := make([]bool, len(srcComments))
	dstMatched := make([]bool, len(dstComments))

	// Match identical comments within each canonical mapped scope using LCS so repeated comments don't cross over.
	scopeSrcMap := make(map[string][]int)
	scopeDstMap := make(map[string][]int)
	for i := range srcComments {
		key := canonicalScopeKey(&srcComments[i], mappings, true)
		scopeSrcMap[key] = append(scopeSrcMap[key], i)
	}
	for j := range dstComments {
		key := canonicalScopeKey(&dstComments[j], mappings, false)
		scopeDstMap[key] = append(scopeDstMap[key], j)
	}

	// PASS 1: Intra-Scope Monotonic LCS Dynamic Programming
	for scopeKey, srcIdxs := range scopeSrcMap {
		dstIdxs := scopeDstMap[scopeKey]
		if len(dstIdxs) == 0 {
			continue
		}

		n := len(srcIdxs)
		m := len(dstIdxs)
		dp := make([][]int, n+1)
		for i := range dp {
			dp[i] = make([]int, m+1)
		}

		for i := 1; i <= n; i++ {
			si := srcIdxs[i-1]
			for j := 1; j <= m; j++ {
				dj := dstIdxs[j-1]
				if srcComments[si].Text == dstComments[dj].Text {
					dp[i][j] = dp[i-1][j-1] + 1
				} else {
					dp[i][j] = max(dp[i-1][j], dp[i][j-1])
				}
			}
		}

		// Step backwards through the DP table to pair up matched comments.
		i, j := n, m
		for i > 0 && j > 0 {
			si := srcIdxs[i-1]
			dj := dstIdxs[j-1]
			if srcComments[si].Text == dstComments[dj].Text && dp[i][j] == dp[i-1][j-1]+1 {
				srcMatched[si] = true
				dstMatched[dj] = true
				sc := &srcComments[si]
				dc := &dstComments[dj]
				nLines := min(commentLineCount(sc), commentLineCount(dc))
				for k := 0; k < nLines; k++ {
					res.LineMappings[sc.StartRow+k] = dc.StartRow + k
				}
				i--
				j--
			} else if dp[i-1][j] >= dp[i][j-1] {
				i--
			} else {
				j--
			}
		}
	}

	// Pass 2: exact text across different scopes becomes delete + insert (we never emit actions.Move for comments).
	for i := range srcComments {
		if srcMatched[i] {
			continue
		}
		sc := &srcComments[i]
		bestJ := -1
		bestDist := 25 // keep the row window small so we don't jump across distant methods

		for j := range dstComments {
			if dstMatched[j] {
				continue
			}
			dc := &dstComments[j]
			if sc.Text == dc.Text {
				dist := max(sc.StartRow, dc.StartRow) - min(sc.StartRow, dc.StartRow)
				if dist < bestDist {
					bestDist = dist
					bestJ = j
				}
			}
		}

		if bestJ >= 0 {
			srcMatched[i] = true
			dstMatched[bestJ] = true
			dc := &dstComments[bestJ]

			scKey := canonicalScopeKey(sc, mappings, true)
			dcKey := canonicalScopeKey(dc, mappings, false)
			if scKey == dcKey {
				nLines := min(commentLineCount(sc), commentLineCount(dc))
				for k := 0; k < nLines; k++ {
					res.LineMappings[sc.StartRow+k] = dc.StartRow + k
				}
			} else {
				res.Actions = append(res.Actions,
					actions.Action{
						Type: actions.Delete,
						Node: createCommentNode(sc, sc.Language),
					},
					actions.Action{
						Type: actions.Insert,
						Node: createCommentNode(dc, dc.Language),
					},
				)
			}
		}
	}

	// PASS 3: Fuzzy match edited comments in the same scope.
	var candidates []commentCandidate
	for i := range srcComments {
		if srcMatched[i] {
			continue
		}
		sc := &srcComments[i]
		scKey := canonicalScopeKey(sc, mappings, true)

		for j := range dstComments {
			if dstMatched[j] {
				continue
			}
			dc := &dstComments[j]
			dcKey := canonicalScopeKey(dc, mappings, false)

			if scKey != dcKey {
				continue
			}

			exactAnchor := false
			if mappings != nil && mappings.Src() != nil && sc.AnchorNode != nil && dc.AnchorNode != nil {
				if mappings.Src()[sc.AnchorNode] == dc.AnchorNode {
					exactAnchor = true
				} else {
					srcBlock := findEnclosingBlock(sc.AnchorNode, sc.EnclosingDecl)
					dstBlock := findEnclosingBlock(dc.AnchorNode, dc.EnclosingDecl)
					if srcBlock != nil && dstBlock != nil {
						if mappedDstBlock, ok := mappings.Src()[srcBlock]; ok && mappedDstBlock != nil {
							if mappedDstBlock != dstBlock {
								continue
							}
						} else if mappings.Dst() != nil {
							if mappedSrcBlock, ok := mappings.Dst()[dstBlock]; ok && mappedSrcBlock != nil {
								if mappedSrcBlock != srcBlock {
									continue
								}
							}
						}
					}
				}
			}

			srcIsMulti := strings.Contains(strings.TrimRight(sc.Text, "\r\n"), "\n")
			dstIsMulti := strings.Contains(strings.TrimRight(dc.Text, "\r\n"), "\n")

			sim := stringSimilarity(sc.Text, dc.Text)
			minThreshold := 0.40
			if srcIsMulti && dstIsMulti {
				minThreshold = 0.20
			}

			if sim >= minThreshold {
				dist := max(sc.StartRow, dc.StartRow) - min(sc.StartRow, dc.StartRow)
				if dist <= 60 {
					candidates = append(candidates, commentCandidate{
						i:           i,
						j:           j,
						sim:         sim,
						dist:        dist,
						exactAnchor: exactAnchor,
					})
				}
			}
		}
	}

	slices.SortFunc(candidates, func(a, b commentCandidate) int {
		if a.exactAnchor != b.exactAnchor {
			if a.exactAnchor {
				return -1
			}
			return 1
		}
		return cmp.Or(
			cmp.Compare(b.sim, a.sim),   // higher similarity first
			cmp.Compare(a.dist, b.dist), // closer line distance first
			cmp.Compare(a.i, b.i),
			cmp.Compare(a.j, b.j),
		)
	})

	matchedDst := make(map[int]int, min(len(srcComments), len(dstComments)))
	for _, cand := range candidates {
		if srcMatched[cand.i] || dstMatched[cand.j] {
			continue
		}
		srcMatched[cand.i] = true
		dstMatched[cand.j] = true
		matchedDst[cand.i] = cand.j
	}

	for i := range srcComments {
		if j, ok := matchedDst[i]; ok {
			sc := &srcComments[i]
			dc := &dstComments[j]
			diffCommentBlock(sc, dc, res)
		}
	}

	// Pass 4: leftovers become deletes or inserts.
	for i := range srcComments {
		if !srcMatched[i] {
			sc := &srcComments[i]
			node := createCommentNode(sc, sc.Language)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Delete,
				Node: node,
			})
		}
	}

	for j := range dstComments {
		if !dstMatched[j] {
			dc := &dstComments[j]
			node := createCommentNode(dc, dc.Language)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Insert,
				Node: node,
			})
		}
	}

	return res
}

func diffCommentBlock(sc, dc *CommentBlock, res *DiffResult) {
	srcTrimmed := strings.TrimRight(sc.Text, "\r\n")
	dstTrimmed := strings.TrimRight(dc.Text, "\r\n")

	srcIsMulti := strings.Contains(srcTrimmed, "\n")
	dstIsMulti := strings.Contains(dstTrimmed, "\n")

	if !srcIsMulti && !dstIsMulti {
		res.LineMappings[sc.StartRow] = dc.StartRow
		srcNode := createCommentNode(sc, sc.Language)
		dstNode := createCommentNode(dc, dc.Language)
		res.Actions = append(res.Actions, actions.Action{
			Type:     actions.Update,
			Node:     srcNode,
			DestNode: dstNode,
			Value:    dc.Text,
		})
		return
	}

	// Line-by-line diff for multiline comments.
	srcLines := strings.Split(srcTrimmed, "\n")
	dstLines := strings.Split(dstTrimmed, "\n")

	matchedA := engine.LineDiff(srcLines, dstLines)
	matchedB := make(map[int]int)
	for a, b := range matchedA {
		matchedB[b] = a
	}

	// Fuzzy match lines with small edits.
	for i := range srcLines {
		if _, ok := matchedA[i]; ok {
			continue
		}
		bestJ := -1
		bestSim := 0.45

		for j := range dstLines {
			if _, ok := matchedB[j]; ok {
				continue
			}
			sim := stringSimilarity(srcLines[i], dstLines[j])
			if sim > bestSim {
				bestSim = sim
				bestJ = j
			}
		}

		if bestJ >= 0 {
			matchedA[i] = bestJ
			matchedB[bestJ] = i
		}
	}

	for i, j := range matchedA {
		res.LineMappings[sc.StartRow+i] = dc.StartRow + j
	}

	srcOffsets := computeLineOffsets(sc.StartByte, srcLines)
	dstOffsets := computeLineOffsets(dc.StartByte, dstLines)

	for i := range srcLines {
		if j, ok := matchedA[i]; ok {
			if srcLines[i] != dstLines[j] {
				startByte, endByte := srcOffsets[i][0], srcOffsets[i][1]
				dstStartByte, dstEndByte := dstOffsets[j][0], dstOffsets[j][1]
				row := uint32(sc.StartRow + i)
				dstRow := uint32(dc.StartRow + j)

				srcNode := createCommentLineNode(sc, srcLines[i], startByte, endByte, row, sc.Language)
				dstNode := createCommentLineNode(dc, dstLines[j], dstStartByte, dstEndByte, dstRow, dc.Language)
				res.Actions = append(res.Actions, actions.Action{
					Type:     actions.Update,
					Node:     srcNode,
					DestNode: dstNode,
					Value:    dstLines[j],
				})
			}
		}
	}

	for i := range srcLines {
		if _, ok := matchedA[i]; !ok {
			startByte, endByte := srcOffsets[i][0], srcOffsets[i][1]
			row := uint32(sc.StartRow + i)
			lineNode := createCommentLineNode(sc, srcLines[i], startByte, endByte, row, sc.Language)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Delete,
				Node: lineNode,
			})
		}
	}

	for j := range dstLines {
		if _, ok := matchedB[j]; !ok {
			startByte, endByte := dstOffsets[j][0], dstOffsets[j][1]
			row := uint32(dc.StartRow + j)
			lineNode := createCommentLineNode(dc, dstLines[j], startByte, endByte, row, dc.Language)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Insert,
				Node: lineNode,
			})
		}
	}
}

func stringSimilarity(a, b string) float64 {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == b {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	bigramsA := make(map[string]int)
	for i := 0; i < len(a)-1; i++ {
		bigramsA[a[i:i+2]]++
	}
	bigramsB := make(map[string]int)
	for i := 0; i < len(b)-1; i++ {
		bigramsB[b[i:i+2]]++
	}
	intersection := 0
	for bg, countA := range bigramsA {
		if countB, ok := bigramsB[bg]; ok {
			intersection += min(countA, countB)
		}
	}
	total := (len(a) - 1) + (len(b) - 1)
	if total <= 0 {
		return 0.0
	}
	return (2.0 * float64(intersection)) / float64(total)
}

func computeLineOffsets(baseOffset uint32, lines []string) [][2]uint32 {
	offsets := make([][2]uint32, len(lines))
	curr := baseOffset
	for i, l := range lines {
		start := curr
		end := curr + uint32(len(l))
		offsets[i] = [2]uint32{start, end}
		curr = end + 1 // +1 for '\n'
	}
	return offsets
}

func createCommentNode(c *CommentBlock, lang string) *treesitter.ASTNode {
	node := createSyntheticNode(c.Type, c.Text, c.StartByte, c.EndByte, uint32(c.StartRow), uint32(c.StartCol), uint32(c.EndRow), uint32(c.EndCol), lang)
	if c.ParentType != "" {
		parent := createSyntheticNode(c.ParentType, "", c.ParentStart, c.ParentEnd, uint32(c.ParentRow), 0, uint32(c.ParentEndRow), 0, lang)
		node.Parent = parent
		parent.Children = []*treesitter.ASTNode{node}
	}
	return node
}

func createCommentLineNode(c *CommentBlock, label string, startByte, endByte, row uint32, lang string) *treesitter.ASTNode {
	node := createSyntheticNode(c.Type, label, startByte, endByte, row, 0, row, uint32(len(label)), lang)
	if c.ParentType != "" {
		parent := createSyntheticNode(c.ParentType, "", c.ParentStart, c.ParentEnd, uint32(c.ParentRow), 0, uint32(c.ParentEndRow), 0, lang)
		node.Parent = parent
		parent.Children = []*treesitter.ASTNode{node}
	}
	return node
}

func createSyntheticNode(nodeType, label string, startByte, endByte, startRow, startCol, endRow, endCol uint32, lang string) *treesitter.ASTNode {
	return &treesitter.ASTNode{
		Type:      nodeType,
		Label:     label,
		StartByte: startByte,
		EndByte:   endByte,
		StartRow:  startRow,
		StartCol:  startCol,
		EndRow:    endRow,
		EndCol:    endCol,
		Language:  lang,
		Children:  make([]*treesitter.ASTNode, 0),
	}
}

func commentLineCount(c *CommentBlock) int {
	t := strings.TrimRight(c.Text, "\r\n")
	if t == "" {
		return 1
	}
	return strings.Count(t, "\n") + 1
}

func findEnclosingBlock(n *treesitter.ASTNode, decl *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	r := rules.Get(n.GetLanguage())
	if r == nil {
		return decl
	}
	for curr := n; curr != nil && curr != decl; curr = curr.Parent {
		if r.IsBlock(curr.Type) || r.IsCaseClause(curr.Type) {
			return curr
		}
	}
	return decl
}
