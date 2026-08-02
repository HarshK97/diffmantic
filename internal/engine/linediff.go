package engine

// LineDiff matches line indices from linesA to linesB using trimmed LCS.
// Returns a map from A's line index to B's line index.
func LineDiff(linesA, linesB []string) map[int]int {
	matchedA := make(map[int]int)
	m, n := len(linesA), len(linesB)

	// Trim common prefix
	start := 0
	for start < m && start < n && linesA[start] == linesB[start] {
		matchedA[start] = start
		start++
	}

	// Trim common suffix
	endA, endB := m-1, n-1
	for endA >= start && endB >= start && linesA[endA] == linesB[endB] {
		matchedA[endA] = endB
		endA--
		endB--
	}

	if start > endA || start > endB {
		return matchedA
	}

	// 1D DP table for the remaining middle window
	subA := linesA[start : endA+1]
	subB := linesB[start : endB+1]
	lenA, lenB := len(subA), len(subB)
	stride := lenB + 1

	dp := make([]int, (lenA+1)*stride)
	for i := 1; i <= lenA; i++ {
		for j := 1; j <= lenB; j++ {
			if subA[i-1] == subB[j-1] {
				dp[i*stride+j] = dp[(i-1)*stride+(j-1)] + 1
			} else {
				dp[i*stride+j] = max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
			}
		}
	}

	// Backtrack to find matching lines
	i, j := lenA, lenB
	for i > 0 && j > 0 {
		if subA[i-1] == subB[j-1] {
			matchedA[start+i-1] = start + j - 1
			i--
			j--
		} else if dp[(i-1)*stride+j] >= dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}

	return matchedA
}
