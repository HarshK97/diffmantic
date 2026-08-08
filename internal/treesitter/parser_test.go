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
		{"test.cs", false},
		{"test.java", false},
		{"test.json", false},
		{"test.yaml", false},
		{"test.yml", false},
		{"test.html", false},
		{"test.css", false},
		{"test.lua", false},
		{"test.zig", false},
		{"test.sh", false},
		{"test.bash", false},
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
	t.Run("valid go source", func(t *testing.T) {
		src := []byte(`package main; func main() {}`)
		ast, err := Parse(src, "main.go")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

	t.Run("valid python source", func(t *testing.T) {
		src := []byte(`def hello(): pass`)
		ast, err := Parse(src, "main.py")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

	t.Run("valid javascript source", func(t *testing.T) {
		src := []byte(`function hello() { return "world"; }`)
		ast, err := Parse(src, "main.js")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

	t.Run("valid typescript source", func(t *testing.T) {
		src := []byte(`interface User { id: number; name: string; } const u: User = { id: 1, name: "Alice" };`)
		ast, err := Parse(src, "main.ts")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

	t.Run("valid jsx source", func(t *testing.T) {
		src := []byte(`const App = () => <div className="app">Hello</div>;`)
		ast, err := Parse(src, "App.jsx")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

	t.Run("valid tsx source", func(t *testing.T) {
		src := []byte(`export const App = (): JSX.Element => <div><h1>Hello World</h1></div>;`)
		ast, err := Parse(src, "main.tsx")
		if err != nil {
			t.Fatalf("Parse(): unexpected error: %v", err)
		}
		if ast == nil {
			t.Error("Parse(): expected non-nil AST root")
		}
	})

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
			t.Error("Parse(): expected non-nil AST root for empty source")
		}
	})
}
