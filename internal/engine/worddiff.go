package engine

import (
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Word diff limits and FNV-1a hash constants.
const (
	// MaxTokensPerLine is the maximum number of tokens allowed per line before
	// the circuit breaker trips and falls back to whole-line diffing.
	MaxTokensPerLine = 120

	// maxDPCells is the maximum size of the LCS DP table: (120+1)*(120+1) = 14641 <= 16384.
	maxDPCells = 16384

	fnvOffset64 = 14695981039346656037
	fnvPrime64  = 1099511628211
)

// WordToken represents a single token in a line with byte boundaries and 64-bit FNV-1a hash.
type WordToken struct {
	Hash  uint64
	Start int
	End   int
}

// WordOp represents a word-level diff operation within a line.
// Action is one of "update", "delete", or "insert".
type WordOp struct {
	Action   string // "update", "delete", "insert"
	SrcStart int
	SrcEnd   int
	DstStart int
	DstEnd   int
}

// WordDiffScratch holds reusable pooled buffers for zero-allocation word diffing.
type WordDiffScratch struct {
	tokensSrc  []WordToken
	tokensDst  []WordToken
	dp         []uint16
	allMatches [][2]int
	Ops        []WordOp
}

var wordDiffScratchPool = sync.Pool{
	New: func() any {
		return &WordDiffScratch{
			tokensSrc:  make([]WordToken, 0, 128),
			tokensDst:  make([]WordToken, 0, 128),
			dp:         make([]uint16, maxDPCells),
			allMatches: make([][2]int, 0, 128),
			Ops:        make([]WordOp, 0, 128),
		}
	},
}

// AcquireWordDiffScratch retrieves a pre-allocated WordDiffScratch from the pool.
func AcquireWordDiffScratch() *WordDiffScratch {
	return wordDiffScratchPool.Get().(*WordDiffScratch)
}

// ReleaseWordDiffScratch returns a WordDiffScratch to the pool after resetting slice lengths.
func ReleaseWordDiffScratch(s *WordDiffScratch) {
	if s == nil {
		return
	}
	s.tokensSrc = s.tokensSrc[:0]
	s.tokensDst = s.tokensDst[:0]
	s.allMatches = s.allMatches[:0]
	s.Ops = s.Ops[:0]
	wordDiffScratchPool.Put(s)
}

func isWordRune(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || (r >= utf8.RuneSelf && (unicode.IsLetter(r) || unicode.IsDigit(r)))
}

func isWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t'
}

func fnvHash(s string, start, end int) uint64 {
	var h uint64 = fnvOffset64
	for i := start; i < end; i++ {
		h ^= uint64(s[i])
		h *= fnvPrime64
	}
	return h
}

// tokenize breaks s into words, punctuation, and whitespace tokens up to MaxTokensPerLine.
// Returns false if the line exceeds the token budget.
func tokenize(s string, buf []WordToken) ([]WordToken, bool) {
	tokens := buf[:0]
	n := len(s)
	i := 0
	for i < n {
		if len(tokens) >= MaxTokensPerLine {
			return tokens, false
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		if isWordRune(r) {
			start := i
			i += size
			for i < n {
				r2, size2 := utf8.DecodeRuneInString(s[i:])
				if !isWordRune(r2) {
					break
				}
				i += size2
			}
			tokens = append(tokens, WordToken{
				Hash:  fnvHash(s, start, i),
				Start: start,
				End:   i,
			})
		} else if isWhitespaceRune(r) {
			start := i
			i += size
			for i < n {
				r2, size2 := utf8.DecodeRuneInString(s[i:])
				if !isWhitespaceRune(r2) {
					break
				}
				i += size2
			}
			tokens = append(tokens, WordToken{
				Hash:  fnvHash(s, start, i),
				Start: start,
				End:   i,
			})
		} else {
			start := i
			i += size
			tokens = append(tokens, WordToken{
				Hash:  fnvHash(s, start, i),
				Start: start,
				End:   i,
			})
		}
	}
	return tokens, true
}

func tokensEqual(srcStr, dstStr string, t1, t2 WordToken) bool {
	return t1.Hash == t2.Hash && srcStr[t1.Start:t1.End] == dstStr[t2.Start:t2.End]
}

// Diff computes word-level diff operations between src and dst using the scratch buffers.
// The returned operations slice is stored in s.Ops and remains valid until the next call
// to Diff or until s is released.
func (s *WordDiffScratch) Diff(src, dst string) []WordOp {
	src = strings.TrimRight(src, "\r\n")
	dst = strings.TrimRight(dst, "\r\n")

	s.Ops = s.Ops[:0]

	tokensSrc, okSrc := tokenize(src, s.tokensSrc)
	s.tokensSrc = tokensSrc

	tokensDst, okDst := tokenize(dst, s.tokensDst)
	s.tokensDst = tokensDst

	// Circuit breaker: If either line exceeds MaxTokensPerLine, fall back to whole-line diff.
	if !okSrc || !okDst {
		if len(src) > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "delete",
				SrcStart: 0,
				SrcEnd:   len(src),
			})
		}
		if len(dst) > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "insert",
				DstStart: 0,
				DstEnd:   len(dst),
			})
		}
		return s.Ops
	}

	N := len(tokensSrc)
	M := len(tokensDst)

	if N == 0 && M == 0 {
		return s.Ops
	}
	if N == 0 {
		s.Ops = append(s.Ops, WordOp{
			Action:   "insert",
			DstStart: 0,
			DstEnd:   len(dst),
		})
		return s.Ops
	}
	if M == 0 {
		s.Ops = append(s.Ops, WordOp{
			Action:   "delete",
			SrcStart: 0,
			SrcEnd:   len(src),
		})
		return s.Ops
	}

	// Trim common prefix tokens
	prefix := 0
	for prefix < min(N, M) && tokensEqual(src, dst, tokensSrc[prefix], tokensDst[prefix]) {
		prefix++
	}

	// Trim common suffix tokens
	suffixSrc := N - 1
	suffixDst := M - 1
	for suffixSrc >= prefix && suffixDst >= prefix && tokensEqual(src, dst, tokensSrc[suffixSrc], tokensDst[suffixDst]) {
		suffixSrc--
		suffixDst--
	}

	// Fast paths when one or both middle ranges are empty
	if prefix > suffixSrc && prefix > suffixDst {
		return s.Ops
	}

	if prefix > suffixSrc && prefix <= suffixDst {
		// Source has no unmatched tokens; dest inserted tokens between prefix and suffix
		s.Ops = append(s.Ops, WordOp{
			Action:   "insert",
			DstStart: tokensDst[prefix].Start,
			DstEnd:   tokensDst[suffixDst].End,
		})
		return s.Ops
	}

	if prefix <= suffixSrc && prefix > suffixDst {
		// Dest has no unmatched tokens; source deleted tokens between prefix and suffix
		s.Ops = append(s.Ops, WordOp{
			Action:   "delete",
			SrcStart: tokensSrc[prefix].Start,
			SrcEnd:   tokensSrc[suffixSrc].End,
		})
		return s.Ops
	}

	// Dynamic programming LCS on middle sub-slices
	subSrc := tokensSrc[prefix : suffixSrc+1]
	subDst := tokensDst[prefix : suffixDst+1]
	subN := len(subSrc)
	subM := len(subDst)

	stride := subM + 1
	cells := (subN + 1) * stride
	dp := s.dp[:cells]
	clear(dp)

	for i := 1; i <= subN; i++ {
		for j := 1; j <= subM; j++ {
			if tokensEqual(src, dst, subSrc[i-1], subDst[j-1]) {
				dp[i*stride+j] = dp[(i-1)*stride+(j-1)] + 1
			} else {
				dp[i*stride+j] = max(dp[(i-1)*stride+j], dp[i*stride+(j-1)])
			}
		}
	}

	// Backtrack to find matched pairs in subSrc / subDst
	s.allMatches = s.allMatches[:0]
	for p := range prefix {
		s.allMatches = append(s.allMatches, [2]int{p, p})
	}
	midStart := len(s.allMatches)
	i, j := subN, subM
	for i > 0 && j > 0 {
		if tokensEqual(src, dst, subSrc[i-1], subDst[j-1]) && dp[i*stride+j] == dp[(i-1)*stride+(j-1)]+1 {
			s.allMatches = append(s.allMatches, [2]int{prefix + (i - 1), prefix + (j - 1)})
			i--
			j--
		} else if dp[(i-1)*stride+j] >= dp[i*stride+(j-1)] {
			i--
		} else {
			j--
		}
	}
	slices.Reverse(s.allMatches[midStart:])

	suffixCount := N - 1 - suffixSrc
	for k := range suffixCount {
		s.allMatches = append(s.allMatches, [2]int{suffixSrc + 1 + k, suffixDst + 1 + k})
	}

	// When two lines share zero matching tokens (K == 0), there's no common context anchor.
	// Treat the line as a complete rewrite (delete + insert in red/green) instead of
	// painting it as a misleading update.
	K := len(s.allMatches)
	if K == 0 {
		if N > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "delete",
				SrcStart: tokensSrc[0].Start,
				SrcEnd:   tokensSrc[N-1].End,
			})
		}
		if M > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "insert",
				DstStart: tokensDst[0].Start,
				DstEnd:   tokensDst[M-1].End,
			})
		}
		return s.Ops
	}

	// Partition unmatched tokens into slots between consecutive matches.
	// Slots with tokens on both sides become updates; one-sided slots become pure deletes or inserts.
	for slot := range K + 1 {
		var srcStartTok, srcEndTok int
		var dstStartTok, dstEndTok int

		if slot == 0 {
			srcStartTok = 0
			dstStartTok = 0
			srcEndTok = s.allMatches[0][0]
			dstEndTok = s.allMatches[0][1]
		} else if slot < K {
			srcStartTok = s.allMatches[slot-1][0] + 1
			srcEndTok = s.allMatches[slot][0]
			dstStartTok = s.allMatches[slot-1][1] + 1
			dstEndTok = s.allMatches[slot][1]
		} else { // slot == K
			srcStartTok = s.allMatches[K-1][0] + 1
			srcEndTok = N
			dstStartTok = s.allMatches[K-1][1] + 1
			dstEndTok = M
		}

		numSrc := srcEndTok - srcStartTok
		numDst := dstEndTok - dstStartTok

		if numSrc > 0 && numDst > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "update",
				SrcStart: tokensSrc[srcStartTok].Start,
				SrcEnd:   tokensSrc[srcEndTok-1].End,
				DstStart: tokensDst[dstStartTok].Start,
				DstEnd:   tokensDst[dstEndTok-1].End,
			})
		} else if numSrc > 0 && numDst == 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "delete",
				SrcStart: tokensSrc[srcStartTok].Start,
				SrcEnd:   tokensSrc[srcEndTok-1].End,
			})
		} else if numSrc == 0 && numDst > 0 {
			s.Ops = append(s.Ops, WordOp{
				Action:   "insert",
				DstStart: tokensDst[dstStartTok].Start,
				DstEnd:   tokensDst[dstEndTok-1].End,
			})
		}
	}

	return s.Ops
}

// DiffWords computes word-level diff operations between src and dst using pooled scratch buffers.
// For hot diff loops across multiple lines, prefer AcquireWordDiffScratch / ReleaseWordDiffScratch.
func DiffWords(src, dst string) []WordOp {
	scratch := AcquireWordDiffScratch()
	defer ReleaseWordDiffScratch(scratch)
	ops := scratch.Diff(src, dst)
	if len(ops) == 0 {
		return nil
	}
	return slices.Clone(ops)
}
