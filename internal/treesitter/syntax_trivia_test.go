package treesitter

import (
	"testing"
	"unsafe"
)

func TestSyntaxTrivia_StructSizeInvariants(t *testing.T) {
	if sz := unsafe.Sizeof(ASTNode{}); sz != 240 {
		t.Errorf("expected sizeof(ASTNode) == 240 bytes, got %d", sz)
	}

	if sz := unsafe.Sizeof(SyntaxTriviaBlock{}); sz != 24 {
		t.Errorf("expected sizeof(SyntaxTriviaBlock) == 24 bytes, got %d", sz)
	}
}

func TestSyntaxTrivia_NilSafety(t *testing.T) {
	var nilNode *ASTNode
	if trivia := nilNode.SyntaxTrivia(); trivia != nil {
		t.Errorf("expected nil node SyntaxTrivia() == nil, got %+v", trivia)
	}

	syntheticNode := &ASTNode{Type: "comment", StartByte: 10, EndByte: 20}
	if trivia := syntheticNode.SyntaxTrivia(); trivia != nil {
		t.Errorf("expected synthetic node (Index==nil) SyntaxTrivia() == nil, got %+v", trivia)
	}

	outOfBoundsNode := &ASTNode{
		ID: 999,
		Index: &ASTIndex{
			SyntaxTrivia: make([]SyntaxTriviaBlock, 2),
		},
	}
	if trivia := outOfBoundsNode.SyntaxTrivia(); trivia != nil {
		t.Errorf("expected out-of-bounds node SyntaxTrivia() == nil, got %+v", trivia)
	}

	negIDNode := &ASTNode{
		ID: -1,
		Index: &ASTIndex{
			SyntaxTrivia: make([]SyntaxTriviaBlock, 2),
		},
	}
	if trivia := negIDNode.SyntaxTrivia(); trivia != nil {
		t.Errorf("expected negative ID node SyntaxTrivia() == nil, got %+v", trivia)
	}

	// Zeroed blocks represent uncaptured trivia; SyntaxTrivia() must return nil to prevent false 0..0 spans.
	zeroNode := &ASTNode{
		ID: 0,
		Index: &ASTIndex{
			SyntaxTrivia: make([]SyntaxTriviaBlock, 2),
		},
	}
	if trivia := zeroNode.SyntaxTrivia(); trivia != nil {
		t.Errorf("expected all-zeros block to return nil, got %+v", trivia)
	}
}

func TestSyntaxTrivia_ContainerAndTrailingPunctuationIngest(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tfoo(a, b)\n}\n")
	root, err := Parse(src, "main.go")
	if err != nil || root == nil {
		t.Fatal("failed to parse Go source")
	}

	var callNode *ASTNode
	for _, d := range root.Descendants() {
		if d.Type == "call_expression" {
			callNode = d
			break
		}
	}
	if callNode == nil {
		t.Fatal("expected to find call_expression node")
	}

	var argListNode *ASTNode
	for _, child := range callNode.Children {
		if child.Type == "argument_list" {
			argListNode = child
			break
		}
	}
	if argListNode == nil {
		t.Fatal("expected to find argument_list node")
	}

	trivia := argListNode.SyntaxTrivia()
	if trivia == nil {
		t.Fatal("expected argument_list to have syntax trivia")
	}

	if trivia.OpeningEnd <= trivia.OpeningStart {
		t.Errorf("expected valid opening delimiter, got %d..%d", trivia.OpeningStart, trivia.OpeningEnd)
	}
	if string(src[trivia.OpeningStart:trivia.OpeningEnd]) != "(" {
		t.Errorf("expected opening delimiter '(', got %q", string(src[trivia.OpeningStart:trivia.OpeningEnd]))
	}

	if trivia.ClosingEnd <= trivia.ClosingStart {
		t.Errorf("expected valid closing delimiter, got %d..%d", trivia.ClosingStart, trivia.ClosingEnd)
	}
	if string(src[trivia.ClosingStart:trivia.ClosingEnd]) != ")" {
		t.Errorf("expected closing delimiter ')', got %q", string(src[trivia.ClosingStart:trivia.ClosingEnd]))
	}

	if len(argListNode.Children) < 2 {
		t.Fatalf("expected at least 2 children in argument_list, got %d", len(argListNode.Children))
	}
	childA := argListNode.Children[0]
	aTrivia := childA.SyntaxTrivia()
	if aTrivia == nil {
		t.Fatal("expected child 'a' to have syntax trivia")
	}
	if aTrivia.TrailingEnd <= aTrivia.TrailingStart {
		t.Fatalf("expected child 'a' to have trailing comma, got %d..%d", aTrivia.TrailingStart, aTrivia.TrailingEnd)
	}
	if string(src[aTrivia.TrailingStart:aTrivia.TrailingEnd]) != "," {
		t.Errorf("expected trailing comma ',', got %q", string(src[aTrivia.TrailingStart:aTrivia.TrailingEnd]))
	}

	childB := argListNode.Children[1]
	bTrivia := childB.SyntaxTrivia()
	if bTrivia != nil && bTrivia.TrailingEnd > bTrivia.TrailingStart {
		t.Errorf("expected child 'b' to not have trailing punctuation, got %d..%d (%q)",
			bTrivia.TrailingStart, bTrivia.TrailingEnd, string(src[bTrivia.TrailingStart:bTrivia.TrailingEnd]))
	}
}

func TestSyntaxTrivia_InterveningCommentPreserved(t *testing.T) {
	src := []byte("package main\n\nfunc main() {\n\tfoo(a /* keep comment */, b)\n}\n")
	root, err := Parse(src, "main.go")
	if err != nil || root == nil {
		t.Fatal("failed to parse Go source")
	}

	var argListNode *ASTNode
	for _, d := range root.Descendants() {
		if d.Type == "argument_list" {
			argListNode = d
			break
		}
	}
	if argListNode == nil {
		t.Fatal("expected to find argument_list node")
	}

	childA := argListNode.Children[0]
	aTrivia := childA.SyntaxTrivia()
	if aTrivia == nil {
		t.Fatal("expected child 'a' to have syntax trivia")
	}
	// The trailing span must latch onto the comma itself without swallowing the intervening comment.
	if string(src[aTrivia.TrailingStart:aTrivia.TrailingEnd]) != "," {
		t.Errorf("expected trailing comma ',', got %q", string(src[aTrivia.TrailingStart:aTrivia.TrailingEnd]))
	}
	if aTrivia.TrailingStart <= childA.EndByte {
		t.Errorf("expected gap between child 'a' end (%d) and comma start (%d)", childA.EndByte, aTrivia.TrailingStart)
	}
}
