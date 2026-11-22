package testutil

import (
	"bytes"
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

// TextQuery specifies criteria for locating an AST node enclosing a text fragment.
// Occurrence is 1-based index among all occurrences of Text in src (0 means "must be unique").
type TextQuery struct {
	Text       string
	Type       string
	Occurrence int
}

// FindNodeByTextQuery locates an AST node enclosing query.Text in src within tree.
func FindNodeByTextQuery(tree *treesitter.ASTNode, src []byte, query TextQuery) (*treesitter.ASTNode, error) {
	if tree == nil {
		return nil, fmt.Errorf("tree is nil")
	}
	if len(src) == 0 {
		return nil, fmt.Errorf("src is empty")
	}
	if query.Text == "" {
		return nil, fmt.Errorf("query text must not be empty")
	}
	if query.Occurrence < 0 {
		return nil, fmt.Errorf("invalid occurrence %d: must be >= 0", query.Occurrence)
	}

	target := []byte(query.Text)
	var matchIndices []int
	offset := 0
	for {
		idx := bytes.Index(src[offset:], target)
		if idx < 0 {
			break
		}
		matchIndices = append(matchIndices, offset+idx)
		offset += idx + 1
	}

	if len(matchIndices) == 0 {
		return nil, fmt.Errorf("target text %q not found in source", query.Text)
	}

	var targetIdx int
	if query.Occurrence == 0 {
		if len(matchIndices) > 1 {
			return nil, fmt.Errorf("ambiguous query: target text %q appears %d times in source (specify Occurrence to disambiguate)", query.Text, len(matchIndices))
		}
		targetIdx = matchIndices[0]
	} else {
		if query.Occurrence > len(matchIndices) {
			return nil, fmt.Errorf("text query occurrence out of range: requested %d-th occurrence of %q, but only %d found", query.Occurrence, query.Text, len(matchIndices))
		}
		targetIdx = matchIndices[query.Occurrence-1]
	}

	startByte := uint32(targetIdx)
	endByte := uint32(targetIdx + len(query.Text))

	var best *treesitter.ASTNode
	bestSpan := ^uint32(0)

	for _, n := range tree.PreOrder() {
		if query.Type != "" && n.Type != query.Type {
			continue
		}
		if n.StartByte <= startByte && n.EndByte >= endByte {
			span := n.EndByte - n.StartByte
			if span <= bestSpan {
				best = n
				bestSpan = span
			}
		}
	}

	if best == nil {
		if query.Type != "" {
			return nil, fmt.Errorf("no node of type %q encloses target text %q", query.Type, query.Text)
		}
		return nil, fmt.Errorf("no AST node encloses target text %q", query.Text)
	}

	return best, nil
}

// FindNodeByText locates the tightest AST node enclosing targetText in src.
// If optional nodeType is provided, only nodes of that type are considered.
func FindNodeByText(tree *treesitter.ASTNode, src []byte, targetText string, nodeType ...string) (*treesitter.ASTNode, error) {
	var nt string
	if len(nodeType) > 0 {
		nt = nodeType[0]
	}
	return FindNodeByTextQuery(tree, src, TextQuery{Text: targetText, Type: nt})
}

// AssertMappedText asserts that the AST node enclosing srcText in srcTree maps to
// the AST node enclosing dstText in dstTree in mappings.
func AssertMappedText(t testing.TB, srcTree, dstTree *treesitter.ASTNode, src, dst []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcText, dstText string, nodeType ...string) {
	t.Helper()
	srcNode, err := FindNodeByText(srcTree, src, srcText, nodeType...)
	if err != nil {
		t.Fatalf("AssertMappedText: failed to find src node for %q: %v", srcText, err)
		return
	}
	dstNode, err := FindNodeByText(dstTree, dst, dstText, nodeType...)
	if err != nil {
		t.Fatalf("AssertMappedText: failed to find dst node for %q: %v", dstText, err)
		return
	}
	if got := mappings[srcNode]; got != dstNode {
		t.Errorf("AssertMappedText: expected %s %q to map to %s %q, got %v",
			srcNode.Type, srcText, dstNode.Type, dstText, got)
	}
}

// AssertNotMappedText asserts that the AST node enclosing srcText in srcTree does not
// map to the AST node enclosing dstText in dstTree in mappings.
func AssertNotMappedText(t testing.TB, srcTree, dstTree *treesitter.ASTNode, src, dst []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcText, dstText string, nodeType ...string) {
	t.Helper()
	srcNode, err := FindNodeByText(srcTree, src, srcText, nodeType...)
	if err != nil {
		t.Fatalf("AssertNotMappedText: failed to find src node for %q: %v", srcText, err)
		return
	}
	dstNode, err := FindNodeByText(dstTree, dst, dstText, nodeType...)
	if err != nil {
		t.Fatalf("AssertNotMappedText: failed to find dst node for %q: %v", dstText, err)
		return
	}
	if mappings[srcNode] == dstNode {
		t.Errorf("AssertNotMappedText: expected %s %q not to map to %s %q, but it did",
			srcNode.Type, srcText, dstNode.Type, dstText)
	}
}

// AssertUnmappedText asserts that the AST node enclosing text in tree is unmapped or maps to nil.
func AssertUnmappedText(t testing.TB, tree *treesitter.ASTNode, src []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, text string, nodeType ...string) {
	t.Helper()
	node, err := FindNodeByText(tree, src, text, nodeType...)
	if err != nil {
		t.Fatalf("AssertUnmappedText: failed to find node for %q: %v", text, err)
		return
	}
	if partner, ok := mappings[node]; ok && partner != nil {
		t.Errorf("AssertUnmappedText: expected node %s %q to be unmapped, but mapped to %v",
			node.Type, text, partner)
	}
}

// AssertMappedTextQuery asserts that the AST node matching srcQuery in srcTree maps to
// the AST node matching dstQuery in dstTree in mappings.
func AssertMappedTextQuery(t testing.TB, srcTree, dstTree *treesitter.ASTNode, src, dst []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcQuery, dstQuery TextQuery) {
	t.Helper()
	srcNode, err := FindNodeByTextQuery(srcTree, src, srcQuery)
	if err != nil {
		t.Fatalf("AssertMappedTextQuery: failed to find src node for %+v: %v", srcQuery, err)
		return
	}
	dstNode, err := FindNodeByTextQuery(dstTree, dst, dstQuery)
	if err != nil {
		t.Fatalf("AssertMappedTextQuery: failed to find dst node for %+v: %v", dstQuery, err)
		return
	}
	if got := mappings[srcNode]; got != dstNode {
		t.Errorf("AssertMappedTextQuery: expected %s %q to map to %s %q, got %v",
			srcNode.Type, srcQuery.Text, dstNode.Type, dstQuery.Text, got)
	}
}

// AssertNotMappedTextQuery asserts that the AST node matching srcQuery in srcTree does not
// map to the AST node matching dstQuery in dstTree in mappings.
func AssertNotMappedTextQuery(t testing.TB, srcTree, dstTree *treesitter.ASTNode, src, dst []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, srcQuery, dstQuery TextQuery) {
	t.Helper()
	srcNode, err := FindNodeByTextQuery(srcTree, src, srcQuery)
	if err != nil {
		t.Fatalf("AssertNotMappedTextQuery: failed to find src node for %+v: %v", srcQuery, err)
		return
	}
	dstNode, err := FindNodeByTextQuery(dstTree, dst, dstQuery)
	if err != nil {
		t.Fatalf("AssertNotMappedTextQuery: failed to find dst node for %+v: %v", dstQuery, err)
		return
	}
	if mappings[srcNode] == dstNode {
		t.Errorf("AssertNotMappedTextQuery: expected %s %q not to map to %s %q, but it did",
			srcNode.Type, srcQuery.Text, dstNode.Type, dstQuery.Text)
	}
}

// AssertUnmappedTextQuery asserts that the AST node matching query in tree is unmapped or maps to nil.
func AssertUnmappedTextQuery(t testing.TB, tree *treesitter.ASTNode, src []byte, mappings map[*treesitter.ASTNode]*treesitter.ASTNode, query TextQuery) {
	t.Helper()
	node, err := FindNodeByTextQuery(tree, src, query)
	if err != nil {
		t.Fatalf("AssertUnmappedTextQuery: failed to find node for %+v: %v", query, err)
		return
	}
	if partner, ok := mappings[node]; ok && partner != nil {
		t.Errorf("AssertUnmappedTextQuery: expected node %s %q to be unmapped, but mapped to %v",
			node.Type, query.Text, partner)
	}
}
