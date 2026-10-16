package treesitter

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename    string
		expectError bool
	}{
		{"test.go", false},
		{"test.py", false},
		{"test.js", false},
		{"test.ts", false},
		{"test.tsx", false},
		{"test.jsx", false},
		{"test.rs", false},
		{"test.c", false},
		{"test.cpp", false},
		{"test.hpp", false},
		{"test.cc", false},
		{"test.cxx", false},
		{"test.hh", false},
		{"test.h", false},
		{"test.cs", true},
		{"test.java", false},
		{"test.php", true},
		{"test.rb", true},
		{"test.json", true},
		{"test.toml", true},
		{"test.yaml", true},
		{"test.yml", true},
		{"test.html", true},
		{"test.css", true},
		{"test.lua", false},
		{"test.zig", false},
		{"test.sh", true},
		{"test.bash", true},
		{"test.xyz", true},
		{"test", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			lang, err := DetectLanguage(tt.filename)

			if tt.expectError {
				if err == nil {
					t.Errorf("DetectLanguage(%q): expected error, got nil", tt.filename)
				}
				if lang != nil {
					t.Errorf("DetectLanguage(%q): expected nil language, got %v", tt.filename, lang)
				}
			} else {
				if err != nil {
					t.Errorf("DetectLanguage(%q): unexpected error: %v", tt.filename, err)
				}
				if lang == nil {
					t.Errorf("DetectLanguage(%q): expected non-nil language pointer", tt.filename)
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	tests := []struct {
		name         string
		src          string
		filename     string
		wantRootType string
		minChildren  int
	}{
		{"valid go source", `package main; func main() {}`, "main.go", "source_file", 2},
		{"valid python source", "def hello(): pass", "main.py", "module", 1},
		{"valid javascript source", `function hello() { return "world"; }`, "main.js", "program", 1},
		{"valid zig source", `const std = @import("std"); pub fn main() void {}`, "main.zig", "source_file", 2},
		{"valid typescript source", `interface User { id: number; name: string; } const u: User = { id: 1, name: "Alice" };`, "main.ts", "program", 2},
		{"valid jsx source", `const App = () => <div className="app">Hello</div>;`, "App.jsx", "program", 1},
		{"valid tsx source", `export const App = (): JSX.Element => <div><h1>Hello World</h1></div>;`, "main.tsx", "program", 1},
		{"valid rust source", `fn main() { println!("Hello, world!"); }`, "main.rs", "source_file", 1},
		{"valid c source", "int main() { return 0; }", "main.c", "translation_unit", 1},
		{"valid cpp source", "#include <iostream>\nint main() { std::cout << \"Hello C++\\n\"; return 0; }", "main.cpp", "translation_unit", 2},
		{"valid java source", `package com.example; public class Main { public static void main(String[] args) { System.out.println("Hello Java"); } }`, "Main.java", "program", 2},
		{"valid lua source", `local function greet(name) print("Hello, " .. name) end`, "main.lua", "chunk", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, err := Parse([]byte(tt.src), tt.filename)
			if err != nil {
				t.Fatalf("Parse(): unexpected error: %v", err)
			}
			if ast == nil {
				t.Fatal("Parse(): expected non-nil AST root")
			}
			if ast.Type != tt.wantRootType {
				t.Errorf("root type = %q, want %q", ast.Type, tt.wantRootType)
			}
			if len(ast.Children) < tt.minChildren {
				t.Errorf("root children count = %d, want >= %d", len(ast.Children), tt.minChildren)
			}
			if ast.HasError {
				t.Errorf("expected zero parse errors, got %d ERROR nodes", ast.ParseErrorCount)
			}
		})
	}

	t.Run("unknown extension", func(t *testing.T) {
		src := []byte(`some text`)
		ast, err := Parse(src, "main.xyz")
		if err == nil {
			t.Error("Parse(): expected error for unknown extension, got nil")
		}
		if ast != nil {
			t.Errorf("Parse(): expected nil AST root for unknown extension, got %v", ast)
		}
	})

	t.Run("empty source", func(t *testing.T) {
		src := []byte(``)
		ast, err := Parse(src, "main.go")
		if err != nil {
			t.Fatalf("Parse(): unexpected error for empty source: %v", err)
		}
		if ast == nil {
			t.Fatal("Parse(): expected non-nil AST root for empty source")
		}
		if ast.Type != "source_file" {
			t.Errorf("empty source root type = %q, want 'source_file'", ast.Type)
		}
		if len(ast.Children) != 0 {
			t.Errorf("empty source children = %d, want 0", len(ast.Children))
		}
	})
}
