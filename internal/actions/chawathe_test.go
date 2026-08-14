package actions

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestUnorderedNodeMatching(t *testing.T) {
	// Create two container nodes with reordered child nodes
	childA1 := &treesitter.ASTNode{Type: "pair", Label: "a"}
	childB1 := &treesitter.ASTNode{Type: "pair", Label: "b"}
	src := &treesitter.ASTNode{
		Type:        "object",
		IsUnordered: true,
		Children:    []*treesitter.ASTNode{childA1, childB1},
	}
	childA1.Parent = src
	childB1.Parent = src

	childB2 := &treesitter.ASTNode{Type: "pair", Label: "b"}
	childA2 := &treesitter.ASTNode{Type: "pair", Label: "a"}
	dst := &treesitter.ASTNode{
		Type:        "object",
		IsUnordered: true,
		Children:    []*treesitter.ASTNode{childB2, childA2},
	}
	childB2.Parent = dst
	childA2.Parent = dst

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(childA1, childA2)
	ms.Add(childB1, childB2)

	script := GenerateEditScript(src, dst, ms)
	for _, action := range script.Actions() {
		if action.Type == Move {
			t.Errorf("expected 0 Move actions for unordered container, got Move action on node %s", action.Node.Label)
		}
	}
}

func TestHTMLSelfClosingTagMatching(t *testing.T) {
	// Verify that <meta charset="utf-8"> vs <meta charset="utf-8"/> produces 0 actions
	src := []byte(`<meta charset="utf-8">`)
	dst := []byte(`<meta charset="utf-8"/>`)

	srcAST, err := treesitter.Parse(src, "index.html")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "index.html")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, src, dst)
	script := GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

	for _, a := range script.Actions() {
		if a.Type == Move {
			t.Errorf("expected 0 Move actions for void self-closing tag conversion, got: %s on node %s", a.Type, a.Node.Type)
		}
	}

	if script.Size() != 0 {
		t.Errorf("expected 0 actions for semantic void tag conversion, got %d actions", script.Size())
		for _, a := range script.Actions() {
			t.Logf("unexpected action: %s on node %s (%s)", a.Type, a.Node.Type, a.Node.Label)
		}
	}
}
