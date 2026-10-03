package treesitter_test

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestAll11NativeGrammarsLoaded(t *testing.T) {
	langs := []string{
		"c", "cpp", "go", "rust", "python", "javascript", "typescript", "tsx",
		"java", "lua", "zig",
	}

	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			ptr, err := treesitter.GetNativeLanguage(lang)
			if err != nil {
				t.Fatalf("failed to get native language for %s: %v", lang, err)
			}
			if ptr == nil {
				t.Fatalf("nil language pointer for %s", lang)
			}
			symbols := treesitter.NativeLanguageSymbols(ptr)
			if len(symbols) == 0 {
				t.Fatalf("empty symbols list for %s", lang)
			}
		})
	}
}

func TestNativeFlatBufferParsing_AllLanguages(t *testing.T) {
	testCases := []struct {
		lang     string
		filename string
		src      string
	}{
		{lang: "c", filename: "main.c", src: "int main(void) { return 0; }"},
		{lang: "cpp", filename: "main.cpp", src: "int main() { return 0; }"},
		{lang: "go", filename: "main.go", src: "package main\nfunc main() {}"},
		{lang: "rust", filename: "main.rs", src: "fn main() {}"},
		{lang: "python", filename: "main.py", src: "def main():\n    pass\n"},
		{lang: "javascript", filename: "main.js", src: "function main() {}"},
		{lang: "typescript", filename: "main.ts", src: "function main(): void {}"},
		{lang: "tsx", filename: "main.tsx", src: "const App = () => <div>Hello</div>;"},
		{lang: "java", filename: "Main.java", src: "class Main { public static void main(String[] args) {} }"},
		{lang: "lua", filename: "main.lua", src: "function main() print('hello') end"},
		{lang: "zig", filename: "main.zig", src: "pub fn main() void {}"},
	}

	for _, tc := range testCases {
		t.Run(tc.lang, func(t *testing.T) {
			ptr, err := treesitter.GetNativeLanguage(tc.lang)
			if err != nil {
				t.Fatalf("failed to get native language for %s: %v", tc.lang, err)
			}

			symbols := treesitter.NativeLanguageSymbols(ptr)
			ast, err := treesitter.ParseWithNativeFlatBuffer([]byte(tc.src), ptr, tc.lang, symbols)
			if err != nil {
				t.Fatalf("native parse error for %s: %v", tc.lang, err)
			}
			if ast == nil {
				t.Fatalf("nil AST for %s", tc.lang)
			}
			if ast.Size() == 0 {
				t.Fatalf("empty AST for %s", tc.lang)
			}
			if ast.HasError {
				t.Errorf("AST has error for %s", tc.lang)
			}
		})
	}
}

func TestIngestFlatAST_Synthetic(t *testing.T) {
	symbols := []string{"source_file", "function_declaration", "identifier", "block", "{", "}"}
	src := []byte("func foo() {}")

	nodes := []treesitter.FlatNode{
		{TypeID: 0, Flags: treesitter.FlatNodeNamed, StartByte: 0, EndByte: 13, FirstChildIdx: 1, NextSiblingIdx: 0xFFFFFFFF, ChildCount: 1},
		{TypeID: 1, Flags: treesitter.FlatNodeNamed, StartByte: 0, EndByte: 13, FirstChildIdx: 2, NextSiblingIdx: 0xFFFFFFFF, ParentIdx: 0, ChildCount: 2},
		{TypeID: 2, Flags: treesitter.FlatNodeNamed, StartByte: 5, EndByte: 8, FirstChildIdx: 0xFFFFFFFF, NextSiblingIdx: 3, ParentIdx: 1, ChildCount: 0},
		{TypeID: 3, Flags: treesitter.FlatNodeNamed, StartByte: 11, EndByte: 13, FirstChildIdx: 0xFFFFFFFF, NextSiblingIdx: 0xFFFFFFFF, ParentIdx: 1, ChildCount: 0},
	}

	root := treesitter.IngestFlatAST(nodes, symbols, src, "go")
	if root == nil {
		t.Fatalf("root is nil")
	}
	if root.Type != "source_file" {
		t.Errorf("expected Type='source_file', got %q", root.Type)
	}
	if root.Language != "go" {
		t.Errorf("expected Language='go', got %q", root.Language)
	}
	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child, got %d", len(root.Children))
	}
	if root.Children[0].Type != "function_declaration" {
		t.Errorf("expected function_declaration, got %q", root.Children[0].Type)
	}
	if len(root.Children[0].Children) != 2 {
		t.Fatalf("expected 2 children, got %d", len(root.Children[0].Children))
	}
	if root.Children[0].Children[0].Label != "foo" {
		t.Errorf("expected label 'foo', got %q", root.Children[0].Children[0].Label)
	}
	if root.Children[0].Children[0].StartByte != 5 || root.Children[0].Children[0].EndByte != 8 {
		t.Errorf("expected byte range [5, 8], got [%d, %d]", root.Children[0].Children[0].StartByte, root.Children[0].Children[0].EndByte)
	}
	if root.HasError {
		t.Errorf("expected HasError=false")
	}
	if root.ParseErrorCount != 0 {
		t.Errorf("expected ParseErrorCount=0, got %d", root.ParseErrorCount)
	}
}

func TestNativeFlatBufferParsing_TriviaAttachment(t *testing.T) {
	src := []byte(`package main

// Doc for foo
func foo() {
	x := 1 // inline comment
	// done inside
}
// trailing file comment
`)

	root, err := treesitter.ParseWithLanguage(src, "go")
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	if root == nil {
		t.Fatal("expected non-nil root")
	}

	// 1. Check LeadingTrivia on function_declaration (child 1, after package_clause)
	var fn *treesitter.ASTNode
	for _, child := range root.Children {
		if child.Type == "function_declaration" {
			fn = child
			break
		}
	}
	if fn == nil {
		t.Fatal("expected function_declaration in root children")
	}
	if len(fn.LeadingTrivia) != 1 {
		t.Fatalf("expected 1 LeadingTrivia on func, got %d", len(fn.LeadingTrivia))
	}
	if fn.LeadingTrivia[0].Text != "// Doc for foo" {
		t.Errorf("unexpected LeadingTrivia text: %q", fn.LeadingTrivia[0].Text)
	}

	// 2. Check TrailingTrivia on x := 1 (inside block)
	var block *treesitter.ASTNode
	for _, child := range fn.Children {
		if child.Type == "block" {
			block = child
			break
		}
	}
	if block == nil {
		t.Fatal("expected block in function_declaration children")
	}
	var stmt *treesitter.ASTNode
	for _, child := range block.Children {
		if child.Type == "short_var_declaration" {
			stmt = child
			break
		}
	}
	if stmt == nil {
		t.Fatal("expected short_var_declaration in block children")
	}
	if len(stmt.TrailingTrivia) != 1 {
		t.Fatalf("expected 1 TrailingTrivia on short_var_declaration, got %d", len(stmt.TrailingTrivia))
	}
	if stmt.TrailingTrivia[0].Text != "// inline comment" {
		t.Errorf("unexpected TrailingTrivia text: %q", stmt.TrailingTrivia[0].Text)
	}

	// 3. Check DanglingTrivia on block (for "// done inside")
	if len(block.DanglingTrivia) != 1 {
		t.Fatalf("expected 1 DanglingTrivia on block, got %d", len(block.DanglingTrivia))
	}
	if block.DanglingTrivia[0].Text != "// done inside" {
		t.Errorf("unexpected DanglingTrivia on block: %q", block.DanglingTrivia[0].Text)
	}

	// 4. Check DanglingTrivia on root (for "// trailing file comment")
	if len(root.DanglingTrivia) != 1 {
		t.Fatalf("expected 1 DanglingTrivia on root, got %d", len(root.DanglingTrivia))
	}
	if root.DanglingTrivia[0].Text != "// trailing file comment" {
		t.Errorf("unexpected DanglingTrivia on root: %q", root.DanglingTrivia[0].Text)
	}
}
