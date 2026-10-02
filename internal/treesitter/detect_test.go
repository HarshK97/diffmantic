package treesitter_test

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestDetectLanguageName(t *testing.T) {
	tests := []struct {
		filename string
		want     string
		wantErr  bool
	}{
		{"main.go", "go", false},
		{"app.js", "javascript", false},
		{"index.ts", "typescript", false},
		{"component.tsx", "tsx", false},
		{"main.rs", "rust", false},
		{"script.py", "python", false},
		{"test.cpp", "cpp", false},
		{"header.h", "c", false},
		{"main.zig", "zig", false},
		{"main.lua", "lua", false},
		{"Main.java", "java", false},
		// Compound compression wrappers
		{"archive.go.zst", "go", false},
		{"script.py.gz", "python", false},
		// Unsupported
		{"data.json", "", true},
		{"config.yaml", "", true},
		{"Cargo.toml", "", true},
		{"Rakefile", "", true},
		{"file.unknown", "", true},
		{"archive.tar.gz", "", true},
		{"no_ext", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got, err := treesitter.DetectLanguageName(tt.filename)
			if (err != nil) != tt.wantErr {
				t.Fatalf("DetectLanguageName(%q) error = %v, wantErr = %v", tt.filename, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("DetectLanguageName(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}
