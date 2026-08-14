package engine

// LineDiff matches line indices between linesA and linesB using trimmed LCS.
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

	subA := linesA[start : endA+1]
	subB := linesB[start : endB+1]
	lenA, lenB := len(subA), len(subB)

	// Switch to Patience diff above 1M matrix cells so we don't blow up memory with a huge DP table.
	if int64(lenA)*int64(lenB) > 1000000 {
		lineDiffPatience(subA, subB, start, start, matchedA)
		return matchedA
	}

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
			bestJ := j
			if i-j > 20 || j-i > 20 {
				for k := j - 1; k >= 1; k-- {
					if subA[i-1] == subB[k-1] && dp[(i-1)*stride+(k-1)]+1 == dp[i*stride+j] {
						if i-k <= 20 && k-i <= 20 {
							bestJ = k
							break
						}
					}
				}
			}
			matchedA[start+i-1] = start + bestJ - 1
			i--
			j = bestJ - 1
		} else if dp[(i-1)*stride+j] >= dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}

	return matchedA
}

type linePair struct {
	i, j int
}

func computeLIS(candidates []linePair) []linePair {
	if len(candidates) == 0 {
		return nil
	}
	tails := make([]int, 0, len(candidates))
	parent := make([]int, len(candidates))
	for i := range parent {
		parent[i] = -1
	}

	for idx, p := range candidates {
		x := p.j
		low, high := 0, len(tails)
		for low < high {
			mid := (low + high) / 2
			if candidates[tails[mid]].j < x {
				low = mid + 1
			} else {
				high = mid
			}
		}
		if low > 0 {
			parent[idx] = tails[low-1]
		}
		if low == len(tails) {
			tails = append(tails, idx)
		} else {
			tails[low] = idx
		}
	}

	lisLen := len(tails)
	lis := make([]linePair, lisLen)
	curr := tails[lisLen-1]
	for k := lisLen - 1; k >= 0; k-- {
		lis[k] = candidates[curr]
		curr = parent[curr]
	}
	return lis
}

func lineDiffPatience(subA, subB []string, startA, startB int, matchedA map[int]int) {
	n := len(subA)
	m := len(subB)
	if n == 0 || m == 0 {
		return
	}

	freqA := make(map[string]int, n)
	for _, s := range subA {
		freqA[s]++
	}
	freqB := make(map[string]int, m)
	for _, s := range subB {
		freqB[s]++
	}

	posB := make(map[string]int, m)
	for j, s := range subB {
		if freqB[s] == 1 {
			posB[s] = j
		}
	}

	var candidates []linePair
	for i, s := range subA {
		if freqA[s] == 1 {
			if j, ok := posB[s]; ok {
				candidates = append(candidates, linePair{i, j})
			}
		}
	}

	lis := computeLIS(candidates)

	lastI, lastJ := 0, 0

	alignWindow := func(winA, winB []string, wStartA, wStartB int) {
		wN := len(winA)
		wM := len(winB)
		if wN == 0 || wM == 0 || int64(wN)*int64(wM) > 1000000 {
			return
		}

		stride := wM + 1
		dp := make([]int, (wN+1)*stride)
		for i := 1; i <= wN; i++ {
			for j := 1; j <= wM; j++ {
				if winA[i-1] == winB[j-1] {
					dp[i*stride+j] = dp[(i-1)*stride+(j-1)] + 1
				} else {
					dp[i*stride+j] = max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
				}
			}
		}
		i, j := wN, wM
		for i > 0 && j > 0 {
			if winA[i-1] == winB[j-1] {
				matchedA[wStartA+i-1] = wStartB + j - 1
				i--
				j--
			} else if dp[(i-1)*stride+j] >= dp[i*stride+(j-1)] {
				i--
			} else {
				j--
			}
		}
	}

	for _, anchor := range lis {
		winA := subA[lastI:anchor.i]
		winB := subB[lastJ:anchor.j]
		alignWindow(winA, winB, startA+lastI, startB+lastJ)

		matchedA[startA+anchor.i] = startB + anchor.j

		lastI = anchor.i + 1
		lastJ = anchor.j + 1
	}

	winA := subA[lastI:]
	winB := subB[lastJ:]
	alignWindow(winA, winB, startA+lastI, startB+lastJ)
}
