package engine

import (
	"slices"
	"sync"
)

const (
	// DefaultSeqDPCapacity fits up to 128x128 matrices (16,384 ints).
	DefaultSeqDPCapacity = 16384
	// MaxPooledSeqDPCapacity caps pooled buffers at 2 MB (256K ints) so one giant diff doesn't bloat the pool forever.
	MaxPooledSeqDPCapacity = 262144
)

// SeqScratch holds reusable buffers for zero-alloc sequence alignment.
// It is not safe for concurrent use; callers should acquire per-goroutine instances.
type SeqScratch struct {
	dp      []int
	matches [][2]int
}

// NewSeqScratch creates a SeqScratch buffer with the requested initial DP capacity.
func NewSeqScratch(initialDPCap int) *SeqScratch {
	if initialDPCap < DefaultSeqDPCapacity {
		initialDPCap = DefaultSeqDPCapacity
	}
	return &SeqScratch{
		dp:      make([]int, initialDPCap),
		matches: make([][2]int, 0, 128),
	}
}

var seqScratchPool = sync.Pool{
	New: func() any {
		return NewSeqScratch(DefaultSeqDPCapacity)
	},
}

// AcquireSeqScratch borrows a SeqScratch from the pool.
func AcquireSeqScratch() *SeqScratch {
	return seqScratchPool.Get().(*SeqScratch)
}

// ReleaseSeqScratch returns s to the pool, shrinking it if it grew past MaxPooledSeqDPCapacity.
func ReleaseSeqScratch(s *SeqScratch) {
	if s == nil {
		return
	}
	s.Reset()
	if cap(s.dp) > MaxPooledSeqDPCapacity {
		s.dp = nil
	}
	seqScratchPool.Put(s)
}

// Reset clears matches without reallocating.
func (s *SeqScratch) Reset() {
	s.matches = s.matches[:0]
}

func (s *SeqScratch) ensureDP(cells int) []int {
	if cap(s.dp) < cells {
		alloc := max(cells, DefaultSeqDPCapacity)
		s.dp = make([]int, alloc)
		return s.dp[:cells]
	}
	dp := s.dp[:cells]
	clear(dp)
	return dp
}

// TrimPrefix returns the count of leading items shared by sequences of lengths n and m.
func TrimPrefix(n, m int, eq func(i, j int) bool) int {
	limit := min(n, m)
	p := 0
	for p < limit && eq(p, p) {
		p++
	}
	return p
}

// TrimSuffix returns the inclusive end indices (endA, endB) after stripping matching trailing items.
func TrimSuffix(n, m, prefix int, eq func(i, j int) bool) (endA, endB int) {
	if prefix < 0 {
		prefix = 0
	}
	endA = n - 1
	endB = m - 1
	for endA >= prefix && endB >= prefix && eq(endA, endB) {
		endA--
		endB--
	}
	return endA, endB
}

// TrimBounds trims matching heads and tails, returning the prefix length and remaining middle bounds.
func TrimBounds(n, m int, eq func(i, j int) bool) (prefix, endA, endB int) {
	prefix = TrimPrefix(n, m, eq)
	endA, endB = TrimSuffix(n, m, prefix, eq)
	return prefix, endA, endB
}

// TrimSliceBounds trims matching heads and tails across two comparable slices.
func TrimSliceBounds[T comparable](a, b []T) (prefix, endA, endB int) {
	return TrimBounds(len(a), len(b), func(i, j int) bool {
		return a[i] == b[j]
	})
}

// CombineMatches splices prefix anchors, offset middle matches, and suffix anchors into dst.
func CombineMatches(dst [][2]int, prefix int, middle [][2]int, endA, endB, lenA int) [][2]int {
	for p := range prefix {
		dst = append(dst, [2]int{p, p})
	}
	for _, m := range middle {
		dst = append(dst, [2]int{prefix + m[0], prefix + m[1]})
	}
	suffixCount := lenA - 1 - endA
	for k := range suffixCount {
		dst = append(dst, [2]int{endA + 1 + k, endB + 1 + k})
	}
	return dst
}

// AlignUniform finds the longest common subsequence where every equal pair scores 1.
// The returned slice aliases internal scratch memory and is only valid until the next Align call or ReleaseSeqScratch.
func (s *SeqScratch) AlignUniform(n, m int, eq func(i, j int) bool) [][2]int {
	return s.AlignWeighted(n, m, func(i, j int) int {
		if eq(i, j) {
			return 1
		}
		return 0
	}, false)
}

// AlignWeighted aligns sequences using a scoring function. Pairs returning 0 are skipped.
// When tieBreakDiagonal is true, ties select candidates closest to the diagonal |i - j|.
// The returned slice aliases internal scratch memory and is only valid until the next Align call or ReleaseSeqScratch.
func (s *SeqScratch) AlignWeighted(n, m int, weight func(i, j int) int, tieBreakDiagonal bool) [][2]int {
	s.matches = s.matches[:0]
	if n <= 0 || m <= 0 {
		return s.matches
	}

	stride := m + 1
	cells := (n + 1) * stride
	dp := s.ensureDP(cells)

	for i := 1; i <= n; i++ {
		rowCurr := i * stride
		rowPrev := (i - 1) * stride
		for j := 1; j <= m; j++ {
			best := max(dp[rowPrev+j], dp[rowCurr+j-1])
			if w := weight(i-1, j-1); w > 0 {
				best = max(best, dp[rowPrev+j-1]+w)
			}
			dp[rowCurr+j] = best
		}
	}

	i, j := n, m
	for i > 0 && j > 0 {
		rowCurr := i * stride
		rowPrev := (i - 1) * stride
		w := weight(i-1, j-1)
		if w > 0 && dp[rowCurr+j] == dp[rowPrev+j-1]+w {
			bestJ := j
			if tieBreakDiagonal {
				bestDist := max(i-j, j-i)
				for k := j - 1; k >= 1; k-- {
					kw := weight(i-1, k-1)
					if kw > 0 && dp[rowPrev+k-1]+kw == dp[rowCurr+j] {
						dist := max(i-k, k-i)
						if dist < bestDist {
							bestDist = dist
							bestJ = k
						}
					}
				}
			}
			s.matches = append(s.matches, [2]int{i - 1, bestJ - 1})
			i--
			j = bestJ - 1
		} else if dp[rowPrev+j] >= dp[rowCurr+j-1] {
			i--
		} else {
			j--
		}
	}

	slices.Reverse(s.matches)
	return s.matches
}
