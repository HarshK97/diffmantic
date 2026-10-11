package engine

import (
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// LineDiff matches corresponding line indices between linesA and linesB using trimmed LCS,
// sliding change boundaries to blank lines and braces via indent heuristics.
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
		return compactLineDiff(linesA, linesB, matchedA)
	}

	subA := linesA[start : endA+1]
	subB := linesB[start : endB+1]
	lenA, lenB := len(subA), len(subB)

	if int64(lenA)*int64(lenB) > 1000000 {
		lineDiffPatience(subA, subB, start, start, matchedA)
		return compactLineDiff(linesA, linesB, matchedA)
	}

	alignLineWindow(subA, subB, start, start, matchedA)
	return compactLineDiff(linesA, linesB, matchedA)
}

const (
	exactSubstantiveWeight      = 10
	reindentedSubstantiveWeight = 8
	trivialLineWeight           = 1
)

// lineWeight returns a matching weight for a line. Substantive code lines
// are given higher weight than trivial punctuation or blank lines so that
// meaningful statements act as anchors when statement order shifts.
func lineWeight(s string) int {
	trimmed := strings.TrimSpace(s)
	if len(trimmed) == 0 {
		return trivialLineWeight
	}
	if len(trimmed) <= 5 && rules.IsPunctuation(trimmed) {
		return trivialLineWeight
	}
	return exactSubstantiveWeight
}

// alignLineWindow aligns winA and winB using weighted dynamic programming and records matches in matchedA.
func alignLineWindow(winA, winB []string, wStartA, wStartB int, matchedA map[int]int) {
	wN := len(winA)
	wM := len(winB)
	if wN == 0 || wM == 0 || int64(wN)*int64(wM) > 1000000 {
		return
	}

	trimA := make([]string, wN)
	weightA := make([]int, wN)
	for idx, s := range winA {
		trimA[idx] = strings.TrimSpace(s)
		weightA[idx] = lineWeight(s)
	}
	trimB := make([]string, wM)
	for idx, s := range winB {
		trimB[idx] = strings.TrimSpace(s)
	}

	matchWeight := func(idxA, idxB int) int {
		if winA[idxA] == winB[idxB] {
			return weightA[idxA]
		}
		if weightA[idxA] > trivialLineWeight && trimA[idxA] == trimB[idxB] {
			return reindentedSubstantiveWeight
		}
		return 0
	}

	stride := wM + 1
	dp := make([]int, (wN+1)*stride)
	for i := 1; i <= wN; i++ {
		for j := 1; j <= wM; j++ {
			best := max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
			if mw := matchWeight(i-1, j-1); mw > 0 {
				best = max(best, dp[(i-1)*stride+(j-1)]+mw)
			}
			dp[i*stride+j] = best
		}
	}

	i, j := wN, wM
	for i > 0 && j > 0 {
		mw := matchWeight(i-1, j-1)
		if mw > 0 && dp[i*stride+j] == dp[(i-1)*stride+(j-1)]+mw {
			bestJ := j
			bestDist := max(i-j, j-i)
			for k := j - 1; k >= 1; k-- {
				kw := matchWeight(i-1, k-1)
				if kw > 0 && dp[(i-1)*stride+(k-1)]+kw == dp[i*stride+j] {
					dist := max(i-k, k-i)
					if dist < bestDist {
						bestDist = dist
						bestJ = k
					}
				}
			}
			matchedA[wStartA+i-1] = wStartB + bestJ - 1
			i--
			j = bestJ - 1
		} else if dp[(i-1)*stride+j] >= dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}
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

	for _, anchor := range lis {
		winA := subA[lastI:anchor.i]
		winB := subB[lastJ:anchor.j]
		alignLineWindow(winA, winB, startA+lastI, startB+lastJ, matchedA)

		matchedA[startA+anchor.i] = startB + anchor.j

		lastI = anchor.i + 1
		lastJ = anchor.j + 1
	}

	winA := subA[lastI:]
	winB := subB[lastJ:]
	alignLineWindow(winA, winB, startA+lastI, startB+lastJ, matchedA)
}

func compactLineDiff(linesA, linesB []string, matchedA map[int]int) map[int]int {
	m, n := len(linesA), len(linesB)
	if m == 0 || n == 0 || len(matchedA) == 0 {
		return matchedA
	}

	changedA := make([]bool, m)
	changedB := make([]bool, n)
	for i := range changedA {
		changedA[i] = true
	}
	for j := range changedB {
		changedB[j] = true
	}

	for i, j := range matchedA {
		if i >= 0 && i < m && j >= 0 && j < n {
			changedA[i] = false
			changedB[j] = false
		}
	}

	compactChanges(linesA, changedA)
	compactChanges(linesB, changedB)

	result := make(map[int]int, len(matchedA))
	matchedLinesA := make([]int, 0, len(matchedA))
	matchedLinesB := make([]int, 0, len(matchedA))
	for i, c := range changedA {
		if !c {
			matchedLinesA = append(matchedLinesA, i)
		}
	}
	for j, c := range changedB {
		if !c {
			matchedLinesB = append(matchedLinesB, j)
		}
	}

	minLen := min(len(matchedLinesA), len(matchedLinesB))
	for k := range minLen {
		result[matchedLinesA[k]] = matchedLinesB[k]
	}

	return result
}

func compactChanges(lines []string, changed []bool) {
	n := len(lines)
	if n == 0 {
		return
	}

	for i := 0; i < n; {
		if !changed[i] {
			i++
			continue
		}

		start := i
		for i < n && changed[i] {
			i++
		}
		end := i
		groupLen := end - start

		// Slide backward to find the earliest valid start.
		earliest := start
		for earliest > 0 && lines[earliest-1] == lines[earliest+groupLen-1] {
			if changed[earliest-1] {
				break
			}
			earliest--
		}

		// Slide forward to find the latest valid start.
		latest := start
		for latest+groupLen < n && lines[latest] == lines[latest+groupLen] {
			if changed[latest+groupLen] {
				break
			}
			latest++
		}

		if earliest == latest {
			continue
		}

		bestPos := start
		bestScore := scoreChangeGroup(lines, start, end)

		for pos := earliest; pos <= latest; pos++ {
			s := scoreChangeGroup(lines, pos, pos+groupLen)
			if s > bestScore {
				bestScore = s
				bestPos = pos
			}
		}

		if bestPos != start {
			for k := start; k < end; k++ {
				changed[k] = false
			}
			for k := bestPos; k < bestPos+groupLen; k++ {
				changed[k] = true
			}
			if bestPos > start {
				i = bestPos + groupLen
			}
		}
	}
}

func scoreChangeGroup(lines []string, start, end int) int {
	return scoreBoundary(lines, start) + scoreBoundary(lines, end)
}

func scoreBoundary(lines []string, pos int) int {
	n := len(lines)
	if pos == 0 || pos == n {
		return 50
	}

	prev := lines[pos-1]
	curr := lines[pos]

	trimmedPrev := strings.TrimSpace(prev)
	trimmedCurr := strings.TrimSpace(curr)

	prevBlank := trimmedPrev == ""
	currBlank := trimmedCurr == ""

	score := 0

	if prevBlank && currBlank {
		score += 80
	} else if prevBlank || currBlank {
		score += 100
	} else {
		pIndent := lineIndent(prev)
		cIndent := lineIndent(curr)
		if pIndent >= 0 && cIndent >= 0 {
			minIndent := min(pIndent, cIndent)
			score += max(0, 40-minIndent*2)
			if cIndent < pIndent {
				score += 30
			} else if cIndent > pIndent {
				score += 20
			}
		}
	}

	if trimmedPrev == "}" || trimmedPrev == "};" || trimmedPrev == "end" || trimmedPrev == ");" || trimmedPrev == "})" {
		score += 40
	}
	if trimmedCurr == "}" || trimmedCurr == "};" {
		score -= 30
	}

	return score
}

func lineIndent(s string) int {
	indent := 0
	for _, ch := range s {
		switch ch {
		case ' ':
			indent++
		case '\t':
			indent += 4
		default:
			return indent
		}
	}
	return -1
}
