package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestRootCmdFlags(t *testing.T) {
	flags := []struct {
		name      string
		shorthand string
		defValue  string
	}{
		{name: "format", shorthand: "f", defValue: ""},
		{name: "theme", shorthand: "t", defValue: ""},
		{name: "ignore-comments", shorthand: "C", defValue: "false"},
		{name: "parse-error-limit", shorthand: "e", defValue: "0"},
		{name: "ui", shorthand: "", defValue: "false"},
		{name: "full", shorthand: "", defValue: "false"},
		{name: "cached", shorthand: "", defValue: "false"},
	}

	for _, tt := range flags {
		f := rootCmd.Flags().Lookup(tt.name)
		if f == nil {
			t.Fatalf("flag %q not registered on rootCmd", tt.name)
		}
		if tt.shorthand != "" && f.Shorthand != tt.shorthand {
			t.Errorf("flag %q shorthand = %q, want %q", tt.name, f.Shorthand, tt.shorthand)
		}
		if f.DefValue != tt.defValue {
			t.Errorf("flag %q default = %q, want %q", tt.name, f.DefValue, tt.defValue)
		}
	}
}

func TestComputeDiffWithDevNull(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.go")
	if err := os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	devNull := os.DevNull
	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(devNull, tmpFile, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(/dev/null, sample.go) failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}

	res2, err := pipeline.RunFiles(tmpFile, devNull, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(sample.go, /dev/null) failed: %v", err)
	}
	if res2 == nil || res2.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope for deleted file")
	}
}

func TestComputeDiffUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.unknown")
	fileB := filepath.Join(dir, "b.unknown")

	_ = os.WriteFile(fileA, []byte("line 1\nline 2\nline 3\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("line 1\nline 2 modified\nline 3\nline 4\n"), 0o644)

	opts := serialize.EnvelopeOptions{IncludeActions: true}
	res, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles failed for unsupported files: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil result and envelope for unsupported file diff")
	}
	if len(res.Envelope.Actions) == 0 {
		t.Error("expected fallback line diff actions for unsupported files, got 0")
	}
}

func TestComputeDiffWithParseErrorLimit(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	fileB := filepath.Join(dir, "b.go")

	// File with syntax error (missing := RHS)
	_ = os.WriteFile(fileA, []byte("package main\n\nfunc foo() {\n\tx :=\n}\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc foo() {\n\tx := 10\n}\n"), 0o644)

	opts := serialize.EnvelopeOptions{IncludeActions: true}
	// Zero error limit should fall back to line diff
	resDefault, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 0, EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(0) failed: %v", err)
	}
	if resDefault.MatchResult != nil {
		t.Error("expected line diff fallback (nil MatchResult) when limit is 0")
	}

	// Tolerated errors should keep AST matching enabled
	resAllowed, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 5, EnvelopeOpts: opts})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(5) failed: %v", err)
	}
	if resAllowed.MatchResult == nil {
		t.Error("expected structural AST match result when limit is 5")
	}
}

func TestIsFileOrDevNull(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "sample.go")
	_ = os.WriteFile(tmpFile, []byte("package main\n"), 0o644)

	if !isFileOrDevNull(tmpFile) {
		t.Errorf("isFileOrDevNull(%q) = false, want true", tmpFile)
	}
	if !isFileOrDevNull(os.DevNull) {
		t.Errorf("isFileOrDevNull(%q) = false, want true", os.DevNull)
	}
	if isFileOrDevNull("/path/does/not/exist/surely.go") {
		t.Errorf("isFileOrDevNull(nonexistent) = true, want false")
	}
}
