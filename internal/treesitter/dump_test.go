package treesitter

import (
	"bytes"
	"strings"
	"testing"
)

func TestDumpAST(t *testing.T) {
	src := []byte(`package main

func main() {
	println("hello")
}
`)

	ast, err := Parse(src, "main.go")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if ast == nil {
		t.Fatal("AST is nil")
	}

	var buf bytes.Buffer
	if err := DumpAST(&buf, ast); err != nil {
		t.Fatalf("DumpAST error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "source_file") {
		t.Errorf("expected source_file in output, got:\n%s", output)
	}
	if !strings.Contains(output, "function_declaration") {
		t.Errorf("expected function_declaration in output, got:\n%s", output)
	}
	if !strings.Contains(output, "identifier: \"main\"") {
		t.Errorf("expected identifier: \"main\" in output, got:\n%s", output)
	}
	if !strings.Contains(output, "├── ") && !strings.Contains(output, "└── ") {
		t.Errorf("expected tree branches in output, got:\n%s", output)
	}

	// Verify coordinates are 1-indexed (e.g. line 1:1) and contain byte offsets
	if !strings.Contains(output, "[1:1 - ") {
		t.Errorf("expected 1-indexed line [1:1 - ...] in output, got:\n%s", output)
	}
	if !strings.Contains(output, "(bytes ") {
		t.Errorf("expected byte coordinates (bytes ...) in output, got:\n%s", output)
	}
}

func TestDumpCST(t *testing.T) {
	src := []byte(`package main

func main() {}
`)

	lang, err := DetectLanguage("main.go")
	if err != nil {
		t.Fatalf("DetectLanguage failed: %v", err)
	}

	_, flatNodes, symbols, err := ParseForPipeline(src, lang.Name)
	if err != nil {
		t.Fatalf("ParseForPipeline failed: %v", err)
	}

	var buf bytes.Buffer
	if err := DumpCST(&buf, flatNodes, symbols, src); err != nil {
		t.Fatalf("DumpCST error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "source_file") {
		t.Errorf("expected source_file in CST output, got:\n%s", output)
	}
	if !strings.Contains(output, "package") {
		t.Errorf("expected package keyword in CST output, got:\n%s", output)
	}
	if !strings.Contains(output, "package_identifier") {
		t.Errorf("expected package_identifier in CST output, got:\n%s", output)
	}
	if !strings.Contains(output, "(bytes ") {
		t.Errorf("expected byte coordinates (bytes ...) in CST output, got:\n%s", output)
	}
}

func TestDumpNilAST(t *testing.T) {
	var buf bytes.Buffer
	if err := DumpAST(&buf, nil); err != nil {
		t.Fatalf("unexpected error for nil AST: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil AST, got %q", buf.String())
	}

	if err := DumpCST(&buf, nil, nil, nil); err != nil {
		t.Fatalf("unexpected error for nil CST: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output for nil CST, got %q", buf.String())
	}

	// Verify nil child within Children slice does not panic
	rootWithNilChild := &ASTNode{
		Type:      "root",
		StartByte: 0,
		EndByte:   10,
		Children: []*ASTNode{
			nil,
			{
				Type:      "child",
				StartByte: 0,
				EndByte:   5,
			},
		},
	}
	buf.Reset()
	if err := DumpAST(&buf, rootWithNilChild); err != nil {
		t.Fatalf("unexpected error dumping AST with nil child: %v", err)
	}
	if !strings.Contains(buf.String(), "child") {
		t.Errorf("expected child node to be formatted, got:\n%s", buf.String())
	}
}

func TestDumpRecursionDepthLimit(t *testing.T) {
	// 1. AST deep recursion
	curr := &ASTNode{Type: "node"}
	astRoot := curr
	for range 1030 {
		child := &ASTNode{Type: "node"}
		curr.Children = []*ASTNode{child}
		curr = child
	}

	var buf bytes.Buffer
	err := DumpAST(&buf, astRoot)
	if err == nil || !strings.Contains(err.Error(), "maximum tree depth exceeded") {
		t.Errorf("expected max tree depth exceeded error for AST, got: %v", err)
	}

	// 2. CST deep recursion
	const chainLen = 1030
	cstNodes := make([]FlatNode, chainLen)
	for i := range chainLen - 1 {
		cstNodes[i] = FlatNode{
			TypeID:         0,
			FirstChildIdx:  uint32(i + 1),
			NextSiblingIdx: FlatNodeNone,
			ChildCount:     1,
		}
	}
	cstNodes[chainLen-1] = FlatNode{
		TypeID:         0,
		FirstChildIdx:  FlatNodeNone,
		NextSiblingIdx: FlatNodeNone,
		ChildCount:     0,
	}

	buf.Reset()
	err = DumpCST(&buf, cstNodes, []string{"node"}, nil)
	if err == nil || !strings.Contains(err.Error(), "maximum tree depth exceeded") {
		t.Errorf("expected max tree depth exceeded error for CST, got: %v", err)
	}
}

func TestDumpCSTSnippetTruncation(t *testing.T) {
	// Leaf node with snippet longer than 64 runes
	longText := strings.Repeat("abcdefghij", 8) // 80 runes
	src := []byte(longText)
	nodes := []FlatNode{
		{
			TypeID:         0,
			StartByte:      0,
			EndByte:        uint32(len(src)),
			FirstChildIdx:  FlatNodeNone,
			NextSiblingIdx: FlatNodeNone,
			ChildCount:     0,
		},
	}
	symbols := []string{"string_literal"}

	var buf bytes.Buffer
	if err := DumpCST(&buf, nodes, symbols, src); err != nil {
		t.Fatalf("DumpCST failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "...") {
		t.Errorf("expected snippet truncation with '...', got:\n%s", out)
	}
	if strings.Contains(out, longText) {
		t.Errorf("expected long text to be truncated, but found full string in:\n%s", out)
	}

	// Leaf node with newlines that should be sanitized
	multilineText := "line1\r\nline2\nline3"
	srcMulti := []byte(multilineText)
	nodesMulti := []FlatNode{
		{
			TypeID:         0,
			StartByte:      0,
			EndByte:        uint32(len(srcMulti)),
			FirstChildIdx:  FlatNodeNone,
			NextSiblingIdx: FlatNodeNone,
			ChildCount:     0,
		},
	}
	buf.Reset()
	if err := DumpCST(&buf, nodesMulti, symbols, srcMulti); err != nil {
		t.Fatalf("DumpCST failed: %v", err)
	}
	outMulti := buf.String()
	if strings.Contains(outMulti, "line1\r\nline2") || strings.Contains(outMulti, "line2\nline3") {
		t.Errorf("expected newlines to be sanitized in snippet, got:\n%s", outMulti)
	}
	if !strings.Contains(outMulti, "line1 line2 line3") {
		t.Errorf("expected sanitized space-separated snippet, got:\n%s", outMulti)
	}
}
