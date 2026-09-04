package engine

import "github.com/HarshK97/diffmantic/internal/treesitter"

// RunZSRecovery runs Zhang-Shasha (1989) tree edit distance on small subtrees
// t1 and t2 and adds optimal node mappings to m.
func RunZSRecovery(t1, t2 *treesitter.ASTNode, m *Mapping) {
	nodes1 := t1.PostOrder()
	nodes2 := t2.PostOrder()
	size1 := len(nodes1)
	size2 := len(nodes2)
	if size1 == 0 || size2 == 0 {
		return
	}

	lld1 := computeLLD(nodes1)
	lld2 := computeLLD(nodes2)

	kr1 := computeKeyRoots(nodes1, lld1)
	kr2 := computeKeyRoots(nodes2, lld2)

	treedist := make([][]float64, size1+1)
	forestdist := make([][]float64, size1+1)
	for i := range treedist {
		treedist[i] = make([]float64, size2+1)
		forestdist[i] = make([]float64, size2+1)
	}

	for _, i := range kr1 {
		for _, j := range kr2 {
			zsForestDist(i, j, lld1, lld2, nodes1, nodes2, treedist, forestdist)
		}
	}

	mappings := zsBacktrack(size1, size2, lld1, lld2, nodes1, nodes2, treedist, forestdist)

	r := rulesFor(t1)
	for _, pair := range mappings {
		if pair[0] != 0 && pair[1] != 0 {
			srcg := nodes1[pair[0]-1]
			dstg := nodes2[pair[1]-1]
			if TypesMatch(srcg.Type, dstg.Type, r) && CompatiblePairRoles(srcg, dstg) && !m.Has(srcg) && !m.HasDst(dstg) {
				m.Add(srcg, dstg)
			}
		}
	}
}

func computeLLD(nodes []*treesitter.ASTNode) []int {
	idxMap := make(map[*treesitter.ASTNode]int, len(nodes))
	for i, n := range nodes {
		idxMap[n] = i
	}
	lld := make([]int, len(nodes))
	for i, n := range nodes {
		curr := n
		for len(curr.Children) > 0 {
			curr = curr.Children[0]
		}
		lld[i] = idxMap[curr]
	}
	return lld
}

func computeKeyRoots(nodes []*treesitter.ASTNode, lld []int) []int {
	visited := make([]bool, len(nodes))
	var kr []int
	for i := len(nodes) - 1; i >= 0; i-- {
		l := lld[i]
		if !visited[l] {
			visited[l] = true
			kr = append(kr, i+1) // 1-based index
		}
	}
	return kr
}

func zsMatchCost(n1, n2 *treesitter.ASTNode) float64 {
	r := rulesFor(n1)
	if !TypesMatch(n1.Type, n2.Type, r) || !CompatiblePairRoles(n1, n2) {
		return 1000.0
	}
	if n1.Label == n2.Label || Unquote(n1.Label) == Unquote(n2.Label) {
		return 0.0
	}
	if len(n1.Children) == 0 && len(n2.Children) == 0 {
		if n1.Parent != nil && n2.Parent != nil && TypesMatch(n1.Parent.Type, n2.Parent.Type, r) {
			return 1.0
		}
		return 2.0
	}
	return 1.0
}

func zsForestDist(
	i, j int,
	lld1, lld2 []int,
	nodes1, nodes2 []*treesitter.ASTNode,
	treedist, forestdist [][]float64,
) {
	firstI := lld1[i-1]
	firstJ := lld2[j-1]

	forestdist[firstI][firstJ] = 0.0

	for di := firstI + 1; di <= i; di++ {
		forestdist[di][firstJ] = forestdist[di-1][firstJ] + 1.0
		for dj := firstJ + 1; dj <= j; dj++ {
			forestdist[firstI][dj] = forestdist[firstI][dj-1] + 1.0

			if lld1[di-1] == firstI && lld2[dj-1] == firstJ {
				ren := zsMatchCost(nodes1[di-1], nodes2[dj-1])
				forestdist[di][dj] = min(
					forestdist[di-1][dj]+1.0,
					forestdist[di][dj-1]+1.0,
					forestdist[di-1][dj-1]+ren,
				)
				treedist[di][dj] = forestdist[di][dj]
			} else {
				forestdist[di][dj] = min(
					forestdist[di-1][dj]+1.0,
					forestdist[di][dj-1]+1.0,
					forestdist[lld1[di-1]][lld2[dj-1]]+treedist[di][dj],
				)
			}
		}
	}
}

func zsBacktrack(
	size1, size2 int,
	lld1, lld2 []int,
	nodes1, nodes2 []*treesitter.ASTNode,
	treedist, forestdist [][]float64,
) [][2]int {
	var editMapping [][2]int
	treePairs := [][2]int{{size1, size2}}
	rootPair := true

	for len(treePairs) > 0 {
		pair := treePairs[0]
		treePairs = treePairs[1:]
		lastRow := pair[0]
		lastCol := pair[1]

		if !rootPair {
			zsForestDist(lastRow, lastCol, lld1, lld2, nodes1, nodes2, treedist, forestdist)
		}
		rootPair = false

		firstRow := lld1[lastRow-1]
		firstCol := lld2[lastCol-1]
		row := lastRow
		col := lastCol

		for row > firstRow || col > firstCol {
			if row > firstRow && forestdist[row-1][col]+1.0 == forestdist[row][col] {
				editMapping = append([][2]int{{row, 0}}, editMapping...)
				row--
			} else if col > firstCol && forestdist[row][col-1]+1.0 == forestdist[row][col] {
				editMapping = append([][2]int{{0, col}}, editMapping...)
				col--
			} else if row > firstRow && col > firstCol {
				if lld1[row-1] == lld1[lastRow-1] && lld2[col-1] == lld2[lastCol-1] {
					editMapping = append([][2]int{{row, col}}, editMapping...)
					row--
					col--
				} else {
					treePairs = append([][2]int{{row, col}}, treePairs...)
					row = lld1[row-1]
					col = lld2[col-1]
				}
			} else {
				break
			}
		}
	}
	return editMapping
}
