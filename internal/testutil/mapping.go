package testutil

import (
	"fmt"
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// NodeQuery specifies criteria for locating an AST node within a tree.
// Line is 1-based (source code line numbers). StartRow on AST nodes is 0-based.
// Nth is 1-based index among all nodes of Type on that Line (defaults to 1 if 0).
type NodeQuery struct {
	Type string
	Line int
	Nth  int
}

// FindNode locates an AST node in tree matching query.
// It matches nodes where n.Type == query.Type and n.StartRow == uint32(query.Line-1).
// If multiple nodes match on the line:
//   - if query.Nth == 0: fails with an ambiguity error.
//   - if query.Nth > 0: returns the Nth match (1-based), or fails if out of range.
//
// If query.Nth == 0 and exactly one node matches, it defaults to Nth=1 and returns it.
func FindNode(tree *treesitter.ASTNode, query NodeQuery) (*treesitter.ASTNode, error) {
	if tree == nil {
		return nil, fmt.Errorf("tree is nil")
	}
	if query.Type == "" {
		return nil, fmt.Errorf("node type must not be empty")
	}
	if query.Line < 1 {
		return nil, fmt.Errorf("line must be >= 1, got %d", query.Line)
	}
	if query.Nth < 0 {
		return nil, fmt.Errorf("invalid Nth %d: must be >= 0", query.Nth)
	}

	targetRow := uint32(query.Line - 1)
	var matches []*treesitter.ASTNode
	for _, n := range tree.PreOrder() {
		if n.Type == query.Type && n.StartRow == targetRow {
			matches = append(matches, n)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no node of type %q found on line %d", query.Type, query.Line)
	}

	if query.Nth == 0 {
		if len(matches) > 1 {
			return nil, fmt.Errorf("ambiguous query: found %d nodes of type %q on line %d (specify Nth to disambiguate)", len(matches), query.Type, query.Line)
		}
		return matches[0], nil
	}

	if query.Nth > len(matches) {
		return nil, fmt.Errorf("node query index out of range: requested %d-th node of type %q on line %d, but only %d found", query.Nth, query.Type, query.Line, len(matches))
	}

	return matches[query.Nth-1], nil
}

// AssertMapped asserts that the node matching srcQuery in srcTree maps to
// the node matching dstQuery in dstTree in mappings.
func AssertMapped(t testing.TB, srcTree, dstTree *treesitter.ASTNode, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcQuery, dstQuery NodeQuery) {
	t.Helper()
	srcNode, err := FindNode(srcTree, srcQuery)
	if err != nil {
		t.Fatalf("AssertMapped: failed to find src node: %v", err)
		return
	}
	dstNode, err := FindNode(dstTree, dstQuery)
	if err != nil {
		t.Fatalf("AssertMapped: failed to find dst node: %v", err)
		return
	}
	if got := mappings[srcNode]; got != dstNode {
		t.Errorf("AssertMapped: expected %s (line %d) to map to %s (line %d), got %v",
			srcQuery.Type, srcQuery.Line, dstQuery.Type, dstQuery.Line, got)
	}
}

// AssertUnmapped asserts that the node matching query in tree is not present
// in mappings or maps to nil.
func AssertUnmapped(t testing.TB, tree *treesitter.ASTNode, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, query NodeQuery) {
	t.Helper()
	node, err := FindNode(tree, query)
	if err != nil {
		t.Fatalf("AssertUnmapped: failed to find node: %v", err)
		return
	}
	if partner, ok := mappings[node]; ok && partner != nil {
		t.Errorf("AssertUnmapped: expected node %s (line %d) to be unmapped, but mapped to %v",
			query.Type, query.Line, partner)
	}
}

// AssertNotMapped asserts that the node matching srcQuery in srcTree does not
// map to the node matching dstQuery in dstTree in mappings.
func AssertNotMapped(t testing.TB, srcTree, dstTree *treesitter.ASTNode, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcQuery, dstQuery NodeQuery) {
	t.Helper()
	srcNode, err := FindNode(srcTree, srcQuery)
	if err != nil {
		t.Fatalf("AssertNotMapped: failed to find src node: %v", err)
		return
	}
	dstNode, err := FindNode(dstTree, dstQuery)
	if err != nil {
		t.Fatalf("AssertNotMapped: failed to find dst node: %v", err)
		return
	}
	if mappings[srcNode] == dstNode {
		t.Errorf("AssertNotMapped: expected %s (line %d) not to map to %s (line %d), but it did",
			srcQuery.Type, srcQuery.Line, dstQuery.Type, dstQuery.Line)
	}
}
