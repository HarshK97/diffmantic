package cmd

import (
	"os"
	"testing"

	"github.com/HarshK97/diffmantic/internal/pipeline"
)

func TestDiffCmdRegistered(t *testing.T) {
	// Make sure the diff subcommand is registered on the root command.
	found := false
	for _, c := range rootCmd.Commands() {
		if c.Name() == "diff" {
			found = true
			break
		}
	}
	if !found {
		t.Error("diff command not registered on rootCmd")
	}
}

func TestDiffCmdFlags(t *testing.T) {
	f := diffCmd.Flags().Lookup("format")
	if f == nil {
		t.Fatal("format flag not registered")
	}
	if f.DefValue != "" {
		t.Errorf("format default = %q, want %q", f.DefValue, "")
	}
}

func TestComputeDiffWithDevNull(t *testing.T) {
	tmpFile := t.TempDir() + "/sample.go"
	if err := os.WriteFile(tmpFile, []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	devNull := os.DevNull
	res, err := pipeline.RunFiles(devNull, tmpFile, pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(/dev/null, sample.go) failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}

	res2, err := pipeline.RunFiles(tmpFile, devNull, pipeline.DiffOptions{})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(sample.go, /dev/null) failed: %v", err)
	}
	if res2 == nil || res2.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope for deleted file")
	}
}

func TestComputeDiffUnsupportedLanguage(t *testing.T) {
	dir := t.TempDir()
	fileA := dir + "/a.unknown"
	fileB := dir + "/b.unknown"

	_ = os.WriteFile(fileA, []byte("line 1\nline 2\nline 3\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("line 1\nline 2 modified\nline 3\nline 4\n"), 0o644)

	res, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{})
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

func TestParseErrorLimitFlag(t *testing.T) {
	f := diffCmd.Flags().Lookup("parse-error-limit")
	if f == nil {
		t.Fatal("parse-error-limit flag not registered")
	}
	if f.Shorthand != "e" {
		t.Errorf("shorthand = %q, want %q", f.Shorthand, "e")
	}
	if f.DefValue != "0" {
		t.Errorf("default = %q, want %q", f.DefValue, "0")
	}
}

func TestComputeDiffWithParseErrorLimit(t *testing.T) {
	dir := t.TempDir()
	fileA := dir + "/a.go"
	fileB := dir + "/b.go"

	// File with a syntax error (missing expression after :=)
	_ = os.WriteFile(fileA, []byte("package main\n\nfunc foo() {\n\tx :=\n}\n"), 0o644)
	_ = os.WriteFile(fileB, []byte("package main\n\nfunc foo() {\n\tx := 10\n}\n"), 0o644)

	// Default (limit = 0): should fall back to line diff
	resDefault, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 0})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(0) failed: %v", err)
	}
	if resDefault.MatchResult != nil {
		t.Error("expected line diff fallback (nil MatchResult) when limit is 0")
	}

	// Allowed limit (limit = 5): should perform AST structural matching
	resAllowed, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{ParseErrorLimit: 5})
	if err != nil {
		t.Fatalf("pipeline.RunFiles(5) failed: %v", err)
	}
	if resAllowed.MatchResult == nil {
		t.Error("expected structural AST match result when limit is 5")
	}
}
