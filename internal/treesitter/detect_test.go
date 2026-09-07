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
		{"data.json", "json", false},
		{"config.yaml", "yaml", false},
		{"config.yml", "yaml", false},
		{"Cargo.toml", "toml", false},
		{"main.rs", "rust", false},
		{"script.py", "python", false},
		{"test.cpp", "cpp", false},
		{"header.h", "c", false},
		{"Rakefile", "ruby", false},
		{"Gemfile", "ruby", false},
		// Compound compression wrappers
		{"expected_ui.json.gz", "json", false},
		{"archive.go.zst", "go", false},
		{"manifest.yaml.xz", "yaml", false},
		{"Gemfile.gz", "ruby", false},
		// Unsupported
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
