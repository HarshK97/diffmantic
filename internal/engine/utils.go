package engine

import (
	"slices"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// Height returns the height of a subtree rooted at n.
// A leaf has height 1.
func Height(n *treesitter.ASTNode) int {
	if n == nil {
		return 0
	}
	if len(n.Children) == 0 {
		return 1
	}
	maxH := 0
	for _, c := range n.Children {
		maxH = max(maxH, Height(c))
	}
	return maxH + 1
}

// IsTrivialLeaf reports whether a leaf node contains only punctuation (no alphanumeric chars or underscores).
func IsTrivialLeaf(n *treesitter.ASTNode) bool {
	if n == nil || len(n.Children) > 0 || n.Label == "" {
		return false
	}
	for _, c := range n.Label {
		if treesitter.IsWordChar(byte(c)) {
			return false
		}
	}
	return true
}

// TypesMatch checks if t1 and t2 are the same type, falling back to the
// language's equivalent_types rules when they're not.
func TypesMatch(t1, t2 string, rules *treesitter.Rules) bool {
	if t1 == t2 {
		return true
	}
	if rules != nil {
		return rules.AreTypesEquivalent(t1, t2)
	}
	return false
}

func rulesFor(root *treesitter.ASTNode) *treesitter.Rules {
	if root == nil {
		return nil
	}
	return treesitter.GetRules(root.GetLanguage())
}

func areAncestorsMatched(anc1, anc2 *treesitter.ASTNode, m *Mapping) bool {
	return (anc1 == nil && anc2 == nil) || (anc1 != nil && anc2 != nil && m.Src()[anc1] == anc2)
}

func descendantSet(n *treesitter.ASTNode) map[*treesitter.ASTNode]struct{} {
	desc := n.Descendants()
	s := make(map[*treesitter.ASTNode]struct{}, len(desc))
	for _, d := range desc {
		s[d] = struct{}{}
	}
	return s
}

func getKeyLabel(n *treesitter.ASTNode) string {
	if n == nil || len(n.Children) == 0 {
		return ""
	}
	k := n.Children[0]
	if k.Label != "" {
		return k.Label
	}
	for _, desc := range k.Descendants() {
		if desc.Label != "" {
			return desc.Label
		}
	}
	return ""
}

func commonMappedDescendants(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) (common, lenS1, lenS2 int) {
	s1 := t1.Descendants()
	s2 := descendantSet(t2)
	for _, d := range s1 {
		if mapped, ok := m[d]; ok {
			if _, inS2 := s2[mapped]; inS2 {
				common++
			}
		}
	}
	return common, len(s1), len(s2)
}

// Dice computes the Dice similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// dice(t1, t2, m) = 2 × |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / (|s(t1)| + |s(t2)|)
func Dice(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	common, lenS1, lenS2 := commonMappedDescendants(t1, t2, m)
	denom := float64(lenS1 + lenS2)
	if denom == 0 {
		return 0
	}
	return 2.0 * float64(common) / denom
}

// ChawatheSimilarity computes the Chawathe similarity coefficient between two subtrees
// given the current mapping set m (maps T1 nodes -> T2 nodes).
//
// chawathe(t1, t2, m) = |{t ∈ s(t1) | (t, t2') ∈ m for some t2'}| / max(|s(t1)|, |s(t2)|)
func ChawatheSimilarity(t1, t2 *treesitter.ASTNode, m map[*treesitter.ASTNode]*treesitter.ASTNode) float64 {
	common, lenS1, lenS2 := commonMappedDescendants(t1, t2, m)
	maxDesc := max(lenS1, lenS2)
	if maxDesc == 0 {
		return 0
	}
	return float64(common) / float64(maxDesc)
}

// Isomorphic returns true if both subtrees match in type, label, and structure.
func Isomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Hash == 0 {
		a.ComputeHashes()
	}
	if b.Hash == 0 {
		b.ComputeHashes()
	}
	return a.Hash == b.Hash
}

// StructureIsomorphic returns true if both subtrees match in shape and node types, ignoring leaf labels.
func StructureIsomorphic(a, b *treesitter.ASTNode) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Type == "comment" || b.Type == "comment" {
		if a.Type != b.Type {
			return false
		}
		return CommentSimilarity(a.Label, b.Label) >= 0.4
	}
	if a.StructureHash == 0 {
		a.ComputeHashes()
	}
	if b.StructureHash == 0 {
		b.ComputeHashes()
	}
	return a.StructureHash == b.StructureHash
}

// NearestMatchedAncestor finds the closest ancestor of n that is present in the mapping.
// If isDst is true, it checks m.HasDst; otherwise it checks m.Has.
func NearestMatchedAncestor(n *treesitter.ASTNode, m *Mapping, isDst bool) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	curr := n.Parent
	for curr != nil {
		if isDst {
			if m.HasDst(curr) {
				return curr
			}
		} else {
			if m.Has(curr) {
				return curr
			}
		}
		curr = curr.Parent
	}
	return nil
}

// SharedMatchedAncestor finds the nearest pair of ancestors (a1, a2) of t1 and t2
// such that a1 in T1 is mapped to a2 in T2.
func SharedMatchedAncestor(t1, t2 *treesitter.ASTNode, m *Mapping) (*treesitter.ASTNode, *treesitter.ASTNode) {
	if t1 == nil || t2 == nil || m == nil {
		return nil, nil
	}
	dstToSrc := make(map[*treesitter.ASTNode]*treesitter.ASTNode)
	for curr1 := t1.Parent; curr1 != nil; curr1 = curr1.Parent {
		if target, ok := m.Src()[curr1]; ok {
			dstToSrc[target] = curr1
		}
	}
	if len(dstToSrc) == 0 {
		return nil, nil
	}

	for curr2 := t2.Parent; curr2 != nil; curr2 = curr2.Parent {
		if a1, ok := dstToSrc[curr2]; ok {
			return a1, curr2
		}
	}
	return nil, nil
}

// GetEnclosingDeclaration traverses up from n to find the nearest ancestor registered in rules.Declarations.
func GetEnclosingDeclaration(n *treesitter.ASTNode) *treesitter.ASTNode {
	if n == nil {
		return nil
	}
	lang := n.GetLanguage()
	r := treesitter.GetRules(lang)
	if r == nil || len(r.Declarations) == 0 {
		return nil
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		if slices.Contains(r.Declarations, curr.Type) {
			return curr
		}
	}
	return nil
}

// AncestorNameSimilarity calculates the number of matching identifier labels
// among the ancestors of t1 and t2. This helps break ties in top-down matching
// by preferring pairs located in similarly named functions or classes.
func AncestorNameSimilarity(t1, t2 *treesitter.ASTNode) int {
	if t1 == nil || t2 == nil {
		return 0
	}
	r1 := rulesFor(t1)
	r2 := rulesFor(t2)

	isID := func(r *treesitter.Rules, typ string) bool {
		if r != nil && len(r.Identifiers) > 0 {
			return slices.Contains(r.Identifiers, typ)
		}
		return typ == "identifier" || typ == "field_identifier" || typ == "type_identifier" || typ == "name"
	}

	labels1 := make(map[string]bool)
	curr := t1.Parent
	for curr != nil {
		if name := getDeclarationName(curr); name != "" {
			labels1[name] = true
		}
		for _, child := range curr.Children {
			if child.Label != "" && isID(r1, child.Type) {
				labels1[child.Label] = true
			}
			if child.IsScaffolding() {
				for _, sub := range child.Children {
					if sub.Label != "" && isID(r1, sub.Type) {
						labels1[sub.Label] = true
					}
				}
			}
		}
		curr = curr.Parent
	}

	labels2 := make(map[string]bool)
	curr = t2.Parent
	for curr != nil {
		if name := getDeclarationName(curr); name != "" {
			labels2[name] = true
		}
		for _, child := range curr.Children {
			if child.Label != "" && isID(r2, child.Type) {
				labels2[child.Label] = true
			}
			if child.IsScaffolding() {
				for _, sub := range child.Children {
					if sub.Label != "" && isID(r2, sub.Type) {
						labels2[sub.Label] = true
					}
				}
			}
		}
		curr = curr.Parent
	}

	overlap := 0
	for l := range labels2 {
		if labels1[l] {
			overlap++
		}
	}
	return overlap
}

// CommentSimilarity returns the bigram Dice coefficient for two cleaned comment strings.
func CommentSimilarity(s1, s2 string) float64 {
	s1 = cleanComment(s1)
	s2 = cleanComment(s2)
	if s1 == "" && s2 == "" {
		return 1.0
	}
	if s1 == "" || s2 == "" {
		return 0.0
	}
	if s1 == s2 {
		return 1.0
	}

	b1 := makeBigrams(s1)
	b2 := makeBigrams(s2)
	if len(b1) == 0 || len(b2) == 0 {
		return 0.0
	}

	intersection := 0
	counts := make(map[string]int, len(b1))
	for _, bg := range b1 {
		counts[bg]++
	}
	for _, bg := range b2 {
		if counts[bg] > 0 {
			intersection++
			counts[bg]--
		}
	}

	return 2.0 * float64(intersection) / float64(len(b1)+len(b2))
}

func cleanComment(label string) string {
	return strings.Trim(strings.ToLower(label), "/#* \t\r\n")
}

func makeBigrams(s string) []string {
	var runes []rune
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			runes = append(runes, r)
		}
	}
	if len(runes) < 2 {
		return nil
	}
	bigrams := make([]string, len(runes)-1)
	for i := range len(runes) - 1 {
		bigrams[i] = string(runes[i : i+2])
	}
	return bigrams
}

func hasWordChar(s string) bool {
	for i := 0; i < len(s); i++ {
		if treesitter.IsWordChar(s[i]) {
			return true
		}
	}
	return false
}

// LeafSimilarity returns the Dice coefficient of non-trivial leaf labels between subtrees a and b.
func LeafSimilarity(a, b *treesitter.ASTNode) float64 {
	if a == nil || b == nil {
		return 0.0
	}
	l1, l2 := a.LeafLabels(), b.LeafLabels()
	intersection, total1, total2 := 0, 0, 0
	for label, count1 := range l1 {
		if !hasWordChar(label) {
			continue
		}
		total1 += count1
		if count2, ok := l2[label]; ok {
			intersection += min(count1, count2)
		}
	}
	for label, count2 := range l2 {
		if hasWordChar(label) {
			total2 += count2
		}
	}
	if total1+total2 == 0 {
		return 0.0
	}
	return 2.0 * float64(intersection) / float64(total1+total2)
}

// HasLongLeafToken reports whether a subtree contains any non-trivial leaf label with length > 3.
func HasLongLeafToken(n *treesitter.ASTNode) bool {
	if n == nil {
		return false
	}
	for label := range n.LeafLabels() {
		if len(label) > 3 && hasWordChar(label) {
			return true
		}
	}
	return false
}
