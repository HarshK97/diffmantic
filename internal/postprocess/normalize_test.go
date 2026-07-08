package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// mkNode builds a node with children and sets Parent pointers.
func mkNode(typ, label string, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	n := &treesitter.ASTNode{Type: typ, Label: label}
	for _, c := range children {
		c.Parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

func mkLeaf(typ, label string) *treesitter.ASTNode {
	return &treesitter.ASTNode{Type: typ, Label: label}
}

func TestCommentTextSimilarityIdentical(t *testing.T) {
	if sim := commentTextSimilarity("hello", "hello"); sim != 1.0 {
		t.Errorf("identical strings: got %f, want 1.0", sim)
	}
}

func TestCommentTextSimilarityEmpty(t *testing.T) {
	if sim := commentTextSimilarity("", "hello"); sim != 0.0 {
		t.Errorf("empty vs non-empty: got %f, want 0.0", sim)
	}
	if sim := commentTextSimilarity("hello", ""); sim != 0.0 {
		t.Errorf("non-empty vs empty: got %f, want 0.0", sim)
	}
}

func TestCommentTextSimilaritySimilar(t *testing.T) {
	sim := commentTextSimilarity("// fix the bug", "// fix a bug")
	if sim < 0.7 {
		t.Errorf("similar comments should have sim >= 0.7, got %f", sim)
	}
}

func TestCommentTextSimilarityDifferent(t *testing.T) {
	sim := commentTextSimilarity("// initialize database", "// render template output")
	if sim >= 0.7 {
		t.Errorf("different comments should have sim < 0.7, got %f", sim)
	}
}

func TestIsSpuriousMoveCandidate(t *testing.T) {
	tests := []struct {
		typ  string
		want bool
	}{
		{"identifier", true},
		{"type_identifier", true},
		{"true", true},
		{"false", true},
		{"nil", true},
		{"function_declaration", false},
		{"if_statement", false},
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			n := &treesitter.ASTNode{Type: tt.typ}
			if got := isSpuriousMoveCandidate(n); got != tt.want {
				t.Errorf("isSpuriousMoveCandidate(%q) = %v, want %v", tt.typ, got, tt.want)
			}
		})
	}
}

func TestNormalizeBareLiteralMovesPassthrough(t *testing.T) {
	// Non-move actions should pass through unchanged.
	es := actions.NewEditScript()
	node := mkLeaf("comment", "// hello")
	es.Add(actions.Action{Type: actions.Update, Node: node, Value: "// world"})

	ms := engine.NewMapping()
	result := normalizeBareLiteralMoves(es, ms)
	if result.Size() != 1 {
		t.Errorf("passthrough: want 1 action, got %d", result.Size())
	}
	if result.Actions()[0].Type != actions.Update {
		t.Error("non-move action should be preserved")
	}
}

func TestNormalizeBareLiteralMovesConverts(t *testing.T) {
	// Move of an identifier with unrelated parent → should become delete+insert.
	srcParent := mkNode("block_a", "")
	srcNode := mkLeaf("identifier", "x")
	srcParent.Children = append(srcParent.Children, srcNode)
	srcNode.Parent = srcParent

	dstParent := mkNode("block_b", "")
	dstNode := mkLeaf("identifier", "x")
	dstParent.Children = append(dstParent.Children, dstNode)
	dstNode.Parent = dstParent

	ms := engine.NewMapping()
	ms.Add(srcNode, dstNode)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:   actions.Move,
		Node:   srcNode,
		Parent: dstParent,
	})

	result := normalizeBareLiteralMoves(es, ms)
	if result.Size() != 2 {
		t.Fatalf("spurious move should become 2 actions, got %d", result.Size())
	}
	if result.Actions()[0].Type != actions.Delete {
		t.Errorf("first action should be Delete, got %s", result.Actions()[0].Type)
	}
	if result.Actions()[1].Type != actions.Insert {
		t.Errorf("second action should be Insert, got %s", result.Actions()[1].Type)
	}
}

func TestNormalizeBareLiteralMovesKeepsCoherent(t *testing.T) {
	// Move where parent also moved coherently → should keep the Move.
	srcParent := mkNode("block", "")
	srcNode := mkLeaf("identifier", "x")
	srcParent.Children = append(srcParent.Children, srcNode)
	srcNode.Parent = srcParent

	dstParent := mkNode("block", "")
	dstNode := mkLeaf("identifier", "x")
	dstParent.Children = append(dstParent.Children, dstNode)
	dstNode.Parent = dstParent

	ms := engine.NewMapping()
	ms.Add(srcNode, dstNode)
	ms.Add(srcParent, dstParent)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:   actions.Move,
		Node:   srcNode,
		Parent: dstParent,
	})

	result := normalizeBareLiteralMoves(es, ms)
	if result.Size() != 1 {
		t.Fatalf("coherent move should stay as 1 action, got %d", result.Size())
	}
	if result.Actions()[0].Type != actions.Move {
		t.Error("coherent move should be preserved")
	}
}

func TestNormalizeBareLiteralMovesNilMapping(t *testing.T) {
	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Insert, Node: mkLeaf("id", "x")})

	result := normalizeBareLiteralMoves(es, nil)
	if result.Size() != 1 {
		t.Error("nil mapping should passthrough")
	}
}

func TestNormalizeCommentMovesConverts(t *testing.T) {
	// Dissimilar comment move → should become delete+insert.
	srcComment := mkLeaf("comment", "// initialize database connection pool")
	dstComment := mkLeaf("comment", "// render HTML template output now")
	parent := mkNode("block", "", dstComment)
	_ = parent

	ms := engine.NewMapping()
	ms.Add(srcComment, dstComment)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:   actions.Move,
		Node:   srcComment,
		Parent: mkNode("block", ""),
	})

	result := normalizeCommentMoves(es, ms)

	hasDelete := false
	hasInsert := false
	for _, a := range result.Actions() {
		if a.Type == actions.Delete {
			hasDelete = true
		}
		if a.Type == actions.Insert {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Error("dissimilar comment move should be converted to delete+insert")
	}
}

func TestNormalizeCommentMovesKeepsSimilar(t *testing.T) {
	// Similar comment move → should keep the Move.
	srcComment := mkLeaf("comment", "// fix the bug here")
	dstComment := mkLeaf("comment", "// fix a bug here")

	ms := engine.NewMapping()
	ms.Add(srcComment, dstComment)

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:   actions.Move,
		Node:   srcComment,
		Parent: mkNode("block", ""),
	})

	result := normalizeCommentMoves(es, ms)
	if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
		t.Error("similar comment move should be preserved")
	}
}

func TestRemoveSubtreeMappings(t *testing.T) {
	child := mkLeaf("id", "x")
	root := mkNode("call", "", child)

	ms := engine.NewMapping()
	ms.Add(root, mkNode("call", ""))
	ms.Add(child, mkLeaf("id", ""))

	removeSubtreeMappings(root, ms)
	if ms.Has(root) || ms.Has(child) {
		t.Error("removeSubtreeMappings should clear all mappings in subtree")
	}
}

func TestRemoveSubtreeMappingsNil(t *testing.T) {
	ms := engine.NewMapping()
	removeSubtreeMappings(nil, ms) // should not panic
}
