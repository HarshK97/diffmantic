package comments

import (
	"math"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// DiffResult holds actions produced from diffing comments.
type DiffResult struct {
	Actions []actions.Action
}

// DiffComments matches and diffs comments between source and destination files.
func DiffComments(srcComments, dstComments []CommentBlock) *DiffResult {
	res := &DiffResult{}
	if len(srcComments) == 0 && len(dstComments) == 0 {
		return res
	}

	srcMatched := make([]bool, len(srcComments))
	dstMatched := make([]bool, len(dstComments))

	// 1. Exact text match in the same scope.
	for i := range srcComments {
		if srcMatched[i] {
			continue
		}
		sc := &srcComments[i]
		bestJ := -1
		bestDist := math.MaxInt

		for j := range dstComments {
			if dstMatched[j] {
				continue
			}
			dc := &dstComments[j]
			if sc.Text == dc.Text && sc.ScopeKey == dc.ScopeKey {
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
		}
	}

	// 2. Exact text match across different scopes (detected as a move).
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

			if sc.ScopeKey != dc.ScopeKey {
				srcNode := createCommentNode(sc)
				dstNode := createCommentNode(dc)
				res.Actions = append(res.Actions, actions.Action{
					Type:     actions.Move,
					Node:     srcNode,
					DestNode: dstNode,
					Parent:   dstNode.Parent,
				})
			}
		}
	}

	// 3. Fuzzy match modified comments in the same scope.
	for i := range srcComments {
		if srcMatched[i] {
			continue
		}
		sc := &srcComments[i]
		bestJ := -1
		bestScore := 0.0

		for j := range dstComments {
			if dstMatched[j] {
				continue
			}
			dc := &dstComments[j]

			if sc.ScopeKey != dc.ScopeKey {
				continue
			}

			srcIsMulti := strings.Contains(sc.Text, "\n")
			dstIsMulti := strings.Contains(dc.Text, "\n")

			sim := stringSimilarity(sc.Text, dc.Text)
			minThreshold := 0.40
			if srcIsMulti && dstIsMulti {
				minThreshold = 0.20
			}

			if sim >= minThreshold && sim > bestScore {
				dist := max(sc.StartRow, dc.StartRow) - min(sc.StartRow, dc.StartRow)
				if dist <= 60 {
					bestScore = sim
					bestJ = j
				}
			}
		}

		if bestJ >= 0 {
			srcMatched[i] = true
			dstMatched[bestJ] = true
			dc := &dstComments[bestJ]

			diffCommentBlock(sc, dc, res)
		}
	}

	// 4. Remaining unmatched comments become deletes and inserts.
	for i := range srcComments {
		if !srcMatched[i] {
			sc := &srcComments[i]
			node := createCommentNode(sc)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Delete,
				Node: node,
			})
		}
	}

	for j := range dstComments {
		if !dstMatched[j] {
			dc := &dstComments[j]
			node := createCommentNode(dc)
			res.Actions = append(res.Actions, actions.Action{
				Type: actions.Insert,
				Node: node,
			})
		}
	}

	return res
}

func diffCommentBlock(sc, dc *CommentBlock, res *DiffResult) {
	srcIsMulti := strings.Contains(sc.Text, "\n")
	dstIsMulti := strings.Contains(dc.Text, "\n")

	if !srcIsMulti && !dstIsMulti {
		srcNode := createCommentNode(sc)
		dstNode := createCommentNode(dc)
		res.Actions = append(res.Actions, actions.Action{
			Type:     actions.Update,
			Node:     srcNode,
			DestNode: dstNode,
			Value:    dc.Text,
		})
		return
	}

	// Diff line by line for multiline comments.
	srcLines := strings.Split(sc.Text, "\n")
	dstLines := strings.Split(dc.Text, "\n")

	matchedA := engine.LineDiff(srcLines, dstLines)
	matchedB := make(map[int]int)
	for a, b := range matchedA {
		matchedB[b] = a
	}

	// Fuzzy match lines with minor edits.
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

	srcOffsets := computeLineOffsets(sc.StartByte, srcLines)
	dstOffsets := computeLineOffsets(dc.StartByte, dstLines)

	for i := range srcLines {
		if j, ok := matchedA[i]; ok {
			if srcLines[i] != dstLines[j] {
				startByte, endByte := srcOffsets[i][0], srcOffsets[i][1]
				dstStartByte, dstEndByte := dstOffsets[j][0], dstOffsets[j][1]
				row := uint32(sc.StartRow + i)
				dstRow := uint32(dc.StartRow + j)

				srcNode := createCommentLineNode(sc, srcLines[i], startByte, endByte, row)
				dstNode := createCommentLineNode(dc, dstLines[j], dstStartByte, dstEndByte, dstRow)
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
			lineNode := createCommentLineNode(sc, srcLines[i], startByte, endByte, row)
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
			lineNode := createCommentLineNode(dc, dstLines[j], startByte, endByte, row)
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

func createCommentNode(c *CommentBlock) *treesitter.ASTNode {
	node := createSyntheticNode(c.Type, c.Text, c.StartByte, c.EndByte, uint32(c.StartRow), uint32(c.StartCol), uint32(c.EndRow), uint32(c.EndCol))
	if c.ParentType != "" {
		node.Parent = createSyntheticNode(c.ParentType, "", c.ParentStart, c.ParentEnd, uint32(c.ParentRow), 0, uint32(c.ParentEndRow), 0)
	}
	return node
}

func createCommentLineNode(c *CommentBlock, label string, startByte, endByte, row uint32) *treesitter.ASTNode {
	node := createSyntheticNode(c.Type, label, startByte, endByte, row, 0, row, uint32(len(label)))
	if c.ParentType != "" {
		node.Parent = createSyntheticNode(c.ParentType, "", c.ParentStart, c.ParentEnd, uint32(c.ParentRow), 0, uint32(c.ParentEndRow), 0)
	}
	return node
}

func createSyntheticNode(nodeType, label string, startByte, endByte, startRow, startCol, endRow, endCol uint32) *treesitter.ASTNode {
	return &treesitter.ASTNode{
		Type:      nodeType,
		Label:     label,
		StartByte: startByte,
		EndByte:   endByte,
		StartRow:  startRow,
		StartCol:  startCol,
		EndRow:    endRow,
		EndCol:    endCol,
	}
}
