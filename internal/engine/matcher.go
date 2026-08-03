package engine

import (
	"cmp"
	"fmt"
	"io"
	"slices"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type MatchResult struct {
	Mappings *Mapping
}

func Match(t1, t2 *treesitter.ASTNode, srcA, srcB []byte) *MatchResult {
	mappings := NewMapping()

	part := NewLinePartition(srcA, srcB)

	minHeight := 2
	minDice := 0.5

	// Match AST nodes top-down, by declaration, and bottom-up using line partitioning.
	TopDown(t1, t2, minHeight, mappings, part)
	matchDeclarations(t1, t2, mappings)
	BottomUp(t1, t2, mappings, minDice)

	MatchUnmatchedLeaves(t1, t2, mappings, part)

	if !mappings.Has(t1) && !mappings.HasDst(t2) {
		mappings.Add(t1, t2)
	}

	sortMappingsByPreOrder(t1, mappings)

	return &MatchResult{Mappings: mappings}
}

// MatchUnmatchedLeaves pairs unmatched leaf nodes of the same type and label using
// parent Dice similarity and positional scores to break ties. Leaves under unmatched
// parents are skipped since they belong to deleted or inserted blocks.
func MatchUnmatchedLeaves(t1Root, t2Root *treesitter.ASTNode, m *Mapping, part *LinePartition) {
	t2Nodes := PostOrder(t2Root)
	for _, t1 := range PostOrder(t1Root) {
		if m.Has(t1) || len(t1.Children) > 0 || t1.Label == "" {
			continue
		}

		// No matched parent → leaf is in a deleted/inserted subtree.
		if t1.Parent != nil && !m.Has(t1.Parent) {
			continue
		}

		var bestT2 *treesitter.ASTNode
		bestDice := 0.0
		bestPosScore := -1

		t1Idx := -1
		if t1.Parent != nil {
			t1Idx = childIndexWithin(t1, t1.Parent)
		}

		for _, t2 := range t2Nodes {
			if m.HasDst(t2) || t2.Type != t1.Type || t2.Label != t1.Label || len(t2.Children) > 0 {
				continue
			}
			if part != nil && !part.CanMatch(t1, t2) {
				continue
			}

			// No matched parent on destination leaf -> it belongs to an inserted subtree.
			if t2.Parent != nil && !m.HasDst(t2.Parent) {
				continue
			}

			d := 0.0
			if t1.Parent != nil && t2.Parent != nil {
				d = Dice(t1.Parent, t2.Parent, m.Src())
			}

			anc1 := NearestMatchedAncestor(t1, m, false)
			anc2 := NearestMatchedAncestor(t2, m, true)
			cMatches := (anc1 == nil && anc2 == nil) || (anc1 != nil && anc2 != nil && m.Src()[anc1] == anc2)

			parentMatched := t1.Parent != nil && t2.Parent != nil && m.Src()[t1.Parent] == t2.Parent

			samePositional := false
			if t1.Parent != nil && t2.Parent != nil {
				samePositional = t1Idx == childIndexWithin(t2, t2.Parent)
			}

			// Check if both parents sit at the same index within their
			// nearest matched ancestors (for fixed-position keywords like
			// "if" where samePositional can't tell them apart).
			parentPositional := false
			if cMatches && anc1 != nil && anc2 != nil &&
				t1.Parent != nil && t2.Parent != nil &&
				anc1 != t1.Parent && anc2 != t2.Parent {
				p1Idx := childIndexWithin(t1.Parent, anc1)
				p2Idx := childIndexWithin(t2.Parent, anc2)
				if p1Idx >= 0 && p2Idx >= 0 {
					parentPositional = p1Idx == p2Idx
				}
			}

			siblingScore := 0
			if t1.Parent != nil && t2.Parent != nil && t1Idx >= 0 {
				if t2Idx := childIndexWithin(t2, t2.Parent); t2Idx >= 0 {
					for _, offset := range []int{-1, 1} {
						i1, i2 := t1Idx+offset, t2Idx+offset
						if i1 >= 0 && i1 < len(t1.Parent.Children) && i2 >= 0 && i2 < len(t2.Parent.Children) {
							if m.Src()[t1.Parent.Children[i1]] == t2.Parent.Children[i2] {
								siblingScore += 500
							}
						}
					}
				}
			}

			posScore := 0
			if parentMatched {
				posScore += 1000
			}
			if parentPositional {
				posScore += 100
			}
			if samePositional {
				posScore += 10
			}
			if cMatches {
				posScore += 1
			}
			posScore += siblingScore

			isBetter := posScore > bestPosScore || (posScore == bestPosScore && d > bestDice)

			if isBetter {
				bestDice = d
				bestT2 = t2
				bestPosScore = posScore
			}
		}

		if bestT2 != nil {
			m.Add(t1, bestT2)
		}
	}
}

// sortMappingsByPreOrder sorts mapped pairs by T1 pre-order index.
func sortMappingsByPreOrder(t1Root *treesitter.ASTNode, m *Mapping) {
	nodes := PreOrder(t1Root)
	index := make(map[*treesitter.ASTNode]int, len(nodes))
	for i, n := range nodes {
		index[n] = i
	}
	slices.SortStableFunc(m.Pairs, func(a, b MappingPair) int {
		return cmp.Compare(index[a.Src], index[b.Src])
	})
}

func FprintMappings(w io.Writer, r *MatchResult) error {
	if r == nil || r.Mappings == nil {
		_, err := fmt.Fprintln(w, "(no mappings)")
		return err
	}

	pairs := r.Mappings.Pairs
	if len(pairs) == 0 {
		_, err := fmt.Fprintln(w, "(no mappings found)")
		return err
	}
	if _, err := fmt.Fprintf(w, "%-4s  %-30s %-20s  →  %-30s %-20s\n",
		"#", "T1 Type", "T1 Label", "T2 Type", "T2 Label"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "─────────────────────────────────────────────────────────────────────────────────────"); err != nil {
		return err
	}
	for i, p := range pairs {
		t1Label := p.Src.Label
		if t1Label == "" {
			t1Label = "-"
		}
		t2Label := p.Dst.Label
		if t2Label == "" {
			t2Label = "-"
		}
		if _, err := fmt.Fprintf(w, "%-4d  %-30s %-20s  →  %-30s %-20s\n",
			i+1, p.Src.Type, t1Label, p.Dst.Type, t2Label); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintf(w, "\nTotal mappings: %d\n", len(pairs))
	return err
}

func matchDeclarations(t1Root, t2Root *treesitter.ASTNode, m *Mapping) {
	t1Decs := findDeclarations(t1Root)
	t2Decs := findDeclarations(t2Root)

	for _, d1 := range t1Decs {
		if m.Has(d1) {
			continue
		}
		name1 := getDeclarationName(d1)
		if name1 == "" {
			continue
		}
		rec1 := getReceiverTypeName(d1)

		var bestMatch *treesitter.ASTNode
		matchCount := 0

		for _, d2 := range t2Decs {
			if m.HasDst(d2) {
				continue
			}
			if d2.Type == d1.Type && getDeclarationName(d2) == name1 && getReceiverTypeName(d2) == rec1 {
				bestMatch = d2
				matchCount++
			}
		}

		if matchCount == 1 {
			m.Add(d1, bestMatch)
		}
	}
}

var declarationTypes = map[string]bool{
	"function_declaration": true,
	"method_declaration":   true,
	"function_definition":  true,
	"class_definition":     true,
	"class_declaration":    true,
	"method_definition":    true,
}

func findDeclarations(root *treesitter.ASTNode) []*treesitter.ASTNode {
	var decs []*treesitter.ASTNode
	var traverse func(*treesitter.ASTNode)
	traverse = func(n *treesitter.ASTNode) {
		if n == nil {
			return
		}
		if declarationTypes[n.Type] {
			decs = append(decs, n)
		}
		for _, child := range n.Children {
			traverse(child)
		}
	}
	traverse(root)
	return decs
}

func getDeclarationName(n *treesitter.ASTNode) string {
	if n == nil {
		return ""
	}
	for _, child := range n.Children {
		if child.Type == "identifier" || child.Type == "field_identifier" {
			return child.Label
		}
	}
	return ""
}

func getReceiverTypeName(n *treesitter.ASTNode) string {
	if n == nil || n.Type != "method_declaration" {
		return ""
	}
	for _, child := range n.Children {
		if child.Type == "parameter_list" {
			for _, p := range child.Children {
				if p.Type == "parameter_declaration" {
					for _, t := range p.Children {
						if t.Type == "type_identifier" {
							return t.Label
						}
						if t.Type == "pointer_type" {
							for _, pt := range t.Children {
								if pt.Type == "type_identifier" {
									return pt.Label
								}
							}
						}
					}
				}
			}
		}
	}
	return ""
}
