package treesitter_test

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestAll18NativeGrammarsLoaded(t *testing.T) {
	langs := []string{
		"c", "cpp", "go", "rust", "python", "javascript", "typescript", "tsx",
		"java", "php", "ruby", "lua", "zig", "css", "html", "json", "toml", "yaml",
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
		{lang: "php", filename: "main.php", src: "<?php echo 'hello'; ?>"},
		{lang: "ruby", filename: "main.rb", src: "def main; puts 'hello'; end"},
		{lang: "lua", filename: "main.lua", src: "function main() print('hello') end"},
		{lang: "zig", filename: "main.zig", src: "pub fn main() void {}"},
		{lang: "css", filename: "style.css", src: "body { color: red; }"},
		{lang: "html", filename: "index.html", src: "<html><body>Hello</body></html>"},
		{lang: "json", filename: "data.json", src: "{\"key\": \"value\", \"count\": 42}"},
		{lang: "toml", filename: "config.toml", src: "[server]\nport = 8080\n"},
		{lang: "yaml", filename: "config.yaml", src: "name: diffmantic\nversion: 1.0\n"},
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
