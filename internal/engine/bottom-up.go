package engine

import (
	"math"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// BottomUp pairs unmatched container nodes in post-order when they share matched
// children, running recovery on newly matched blocks.
func BottomUp(
	t1Root, t2Root *treesitter.ASTNode,
	m *Mapping,
	minDice float64,
) {
	for _, t1 := range t1Root.PostOrder() {
		if t1 == t1Root {
			if !m.Has(t1) && !m.HasDst(t2Root) {
				m.Add(t1, t2Root)
			}
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChild(t1, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(t1, t2, m)
			}
			break
		}

		if m.Has(t1) {
			t2 := m.Src()[t1]
			if t2 != nil && hasUnmappedChild(t1, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(t1, t2, m)
			}
			continue
		}

		if len(t1.Children) == 0 {
			continue
		}

		if !hasMatchedChild(t1, m) {
			continue
		}

		candidates := findCandidatesWithCommonDescendants(t1, m)
		t2 := candidate(t1, candidates, m)
		if t2 == nil {
			continue
		}

		sim := ChawatheSimilarity(t1, t2, m.Src())
		threshold := 1.0 / (1.0 + math.Log(float64((t1.Size()-1)+(t2.Size()-1))))
		if m.DiceSrc(t1, t2) >= minDice || sim >= threshold {
			m.Add(t1, t2)
			Recover(t1, t2, m)
		}
	}
}

func findCandidatesWithCommonDescendants(t1 *treesitter.ASTNode, m *Mapping) []*treesitter.ASTNode {
	candMap := make(map[*treesitter.ASTNode]bool)
	var cands []*treesitter.ASTNode

	var r *rules.Rules
	if t1 != nil {
		r = rules.Get(t1.GetLanguage())
	}

	for _, d1 := range t1.Descendants() {
		if d2, ok := m.Src()[d1]; ok {
			for anc := d2.Parent; anc != nil; anc = anc.Parent {
				if TypesMatch(t1.Type, anc.Type, r) && !m.HasDst(anc) {
					if !candMap[anc] {
						candMap[anc] = true
						cands = append(cands, anc)
					}
				}
			}
		}
	}
	return cands
}

func hasUnmappedChild(t *treesitter.ASTNode, hasFn func(*treesitter.ASTNode) bool) bool {
	for _, c := range t.Children {
		if !hasFn(c) {
			return true
		}
	}
	return false
}

func hasMatchedChild(t1 *treesitter.ASTNode, m *Mapping) bool {
	return slices.ContainsFunc(t1.Children, func(c *treesitter.ASTNode) bool {
		return m.Has(c)
	})
}

// AffinityWeights holds the scoring weights used to rank bottom-up candidates.
type AffinityWeights struct {
	Sim        float64 // Chawathe leaf similarity weight
	Dice       float64 // Subtree descendant Dice weight
	Scope      float64 // Scope proximity weight
	Pos        float64 // Sibling index alignment weight
	Label      float64 // Leaf label similarity weight
	KeyBonus   float64 // Exact key match bonus for pair nodes
	DepthCoeff float64 // Quadratic penalty for relative depth divergence
}

// DefaultAffinityWeights is the default set of scoring weights used by candidate selection.
// Exported so callers can use it as a baseline for language-specific tuning.
var DefaultAffinityWeights = AffinityWeights{
	Sim:        0.30,
	Dice:       0.25,
	Scope:      0.25,
	Pos:        0.10,
	Label:      0.10,
	KeyBonus:   0.35,
	DepthCoeff: 0.20,
}

// computeAffinity scores how well t1 matches candidate c. Returns -1.0 if hard constraints fail.
func computeAffinity(t1, c *treesitter.ASTNode, m *Mapping, w AffinityWeights) float64 {
	if m.HasDst(c) {
		return -1.0
	}
	if !hasCommonDescendant(t1, c, m) {
		return -1.0
	}
	if !CompatiblePairRoles(t1, c) {
		return -1.0
	}

	r := rulesFor(t1)
	if !TypesMatch(t1.Type, c.Type, r) {
		return -1.0
	}

	anc1 := NearestMatchedAncestor(t1, m, false)
	anc2 := NearestMatchedAncestor(c, m, true)
	cMatches := areAncestorsMatched(anc1, anc2, m)

	// Keep small subtrees and syntactic wrappers inside their mapped scope.
	if !cMatches && (Height(t1) <= 2 || t1.IsWrapper() || (r != nil && r.IsWrapper(t1.Type))) {
		return -1.0
	}

	// Don't let an inner block claim an outer container when an outer ancestor matches the construct.
	if hasEnclosingConstructAncestor(t1, c, r) {
		return -1.0
	}

	sim := ChawatheSimilarity(t1, c, m.Src())
	dice := Dice(t1, c, m.Src())

	scopeScore := 0.5
	if anc1 == nil && anc2 == nil {
		scopeScore = 1.0
	} else if anc1 != nil && anc2 != nil {
		if m.Src()[anc1] == anc2 {
			scopeScore = 1.0
		} else {
			scopeScore = 0.0
		}
	}

	posScore := 1.0
	isUnordered := (t1.Parent != nil && t1.Parent.IsUnordered) || (c.Parent != nil && c.Parent.IsUnordered)
	if !isUnordered && t1.Parent != nil && c.Parent != nil {
		idx1 := t1.ChildIndex()
		idx2 := c.ChildIndex()
		maxLen := max(len(t1.Parent.Children), len(c.Parent.Children))
		if maxLen > 0 && idx1 >= 0 && idx2 >= 0 {
			absDiff := idx1 - idx2
			if absDiff < 0 {
				absDiff = -absDiff
			}
			posScore = 1.0 - float64(absDiff)/float64(maxLen)
		}
	}

	lblScore := LeafSimilarity(t1, c)

	keyBonus := 0.0
	if t1Key, cKey := getKeyLabel(t1), getKeyLabel(c); t1Key != "" && t1Key == cKey {
		keyBonus = w.KeyBonus
	}

	depthPenalty := 0.0
	if (anc1 == nil) == (anc2 == nil) {
		d1 := t1.DepthTo(anc1)
		d2 := c.DepthTo(anc2)
		diff := float64(d1 - d2)
		depthPenalty = w.DepthCoeff * (diff * diff)
	}

	return (w.Sim * sim) + (w.Dice * dice) + (w.Scope * scopeScore) + (w.Pos * posScore) + (w.Label * lblScore) + keyBonus - depthPenalty
}

// candidate finds the best unmatched node in T2 to pair with t1 using unified affinity scoring.
func candidate(
	t1 *treesitter.ASTNode,
	candidates []*treesitter.ASTNode,
	m *Mapping,
) *treesitter.ASTNode {
	var best *treesitter.ASTNode
	bestScore := -1.0

	for _, c := range candidates {
		score := computeAffinity(t1, c, m, DefaultAffinityWeights)
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

func hasCommonDescendant(
	t1 *treesitter.ASTNode,
	c *treesitter.ASTNode,
	m *Mapping,
) bool {
	for _, d := range c.Descendants() {
		if t1Partner, ok := m.Dst()[d]; ok && t1.Contains(t1Partner) {
			return true
		}
	}
	return false
}

// RollupMatchedContainers pairs unmatched container nodes in post-order when their
// mapped children predominantly belong to the same unmatched parent container in T2.
func RollupMatchedContainers(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	rules := rulesFor(t1Root)
	for _, t1 := range t1Root.PostOrder() {
		if m.Has(t1) || len(t1.Children) == 0 {
			continue
		}

		parentCounts := make(map[*treesitter.ASTNode]int)
		var bestParent *treesitter.ASTNode
		bestCount := 0
		for _, c := range t1.Children {
			if c2, ok := m.Src()[c]; ok && c2.Parent != nil && !m.HasDst(c2.Parent) && TypesMatch(t1.Type, c2.Parent.Type, rules) {
				cnt := parentCounts[c2.Parent] + 1
				parentCounts[c2.Parent] = cnt
				if cnt > bestCount {
					bestCount = cnt
					bestParent = c2.Parent
				}
			}
		}

		if bestParent != nil {
			m.Add(t1, bestParent)
			if hasUnmappedChild(t1, m.Has) && hasUnmappedChild(bestParent, m.HasDst) {
				Recover(t1, bestParent, m)
			}
		}
	}
}

// ContestContainers reassigns T2 containers that got greedily claimed by an inner T1 node
// during bottom-up matching back to their proper outer T1 parent.
//
// Example: deleting an if-block can trick BottomUp into matching the if's inner block
// to the outer function body (if they share something like a throw). That leaves the
// real function body unmapped and generates a mess of spurious Move actions.
func ContestContainers(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	dstMap := m.Dst()
	rules := rulesFor(t1Root)

	for _, t2 := range t2Root.PostOrder() {
		if !m.HasDst(t2) || len(t2.Children) == 0 || t2.Parent == nil {
			continue
		}

		currentT1 := dstMap[t2]

		t1MappedParent := dstMap[t2.Parent]
		if t1MappedParent == nil {
			continue
		}

		// Already at the expected depth under the mapped parent — nothing to fix.
		if currentT1.Parent == t1MappedParent {
			continue
		}

		// T1 sits deeper than expected. Look for an unmapped sibling at the expected depth.
		for _, candidate := range t1MappedParent.Children {
			if !TypesMatch(candidate.Type, t2.Type, rules) || m.Has(candidate) {
				continue
			}

			canReclaim := false
			if candidate.Contains(currentT1) {
				if m.DiceSrc(candidate, t2) > m.DiceSrc(currentT1, t2) {
					canReclaim = true
				}
			} else {
				if directKeyMatch(candidate, t2) && !directKeyMatch(currentT1, t2) {
					canReclaim = true
				}
			}

			if !canReclaim {
				continue
			}

			// Clear any leaf mappings from currentT1 so candidate can claim them.
			for _, t2Child := range t2.Children {
				if len(t2Child.Children) == 0 {
					if srcLeaf := dstMap[t2Child]; srcLeaf != nil && currentT1.Contains(srcLeaf) {
						m.Remove(srcLeaf)
					}
				}
			}
			m.Remove(currentT1)
			m.Add(candidate, t2)

			if hasUnmappedChild(candidate, m.Has) && hasUnmappedChild(t2, m.HasDst) {
				Recover(candidate, t2, m)
			}
			break
		}
	}
}

func directKeyMatch(n1, n2 *treesitter.ASTNode) bool {
	if n1 == nil || n2 == nil || len(n1.Children) == 0 || len(n2.Children) == 0 {
		return false
	}
	c1 := n1.Children[0]
	c2 := n2.Children[0]
	return c1.Type == c2.Type && c1.Label != "" && c1.Label == c2.Label
}

// hasEnclosingConstructAncestor checks if an outer ancestor matches the peer's parent construct,
// preventing inner blocks from stealing outer containers during post-order traversal.
func hasEnclosingConstructAncestor(t1, c *treesitter.ASTNode, r *rules.Rules) bool {
	if r == nil || (!r.IsBlock(t1.Type) && !r.IsDeclaration(t1.Type)) {
		return false
	}
	if t1.Parent != nil && c.Parent != nil && !TypesMatch(t1.Parent.Type, c.Parent.Type, r) {
		for anc := t1.Parent; anc != nil; anc = anc.Parent {
			if TypesMatch(anc.Type, c.Type, r) && anc.Parent != nil && TypesMatch(anc.Parent.Type, c.Parent.Type, r) {
				return true
			}
		}
		for anc := c.Parent; anc != nil; anc = anc.Parent {
			if TypesMatch(t1.Type, anc.Type, r) && anc.Parent != nil && TypesMatch(t1.Parent.Type, anc.Parent.Type, r) {
				return true
			}
		}
	}
	return false
}
