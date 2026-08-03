package cmd

import (
	"os"
	"testing"
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
	res, err := computeDiff(devNull, tmpFile)
	if err != nil {
		t.Fatalf("computeDiff(/dev/null, sample.go) failed: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil diff result and envelope")
	}

	res2, err := computeDiff(tmpFile, devNull)
	if err != nil {
		t.Fatalf("computeDiff(sample.go, /dev/null) failed: %v", err)
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

	res, err := computeDiff(fileA, fileB)
	if err != nil {
		t.Fatalf("computeDiff failed for unsupported files: %v", err)
	}
	if res == nil || res.Envelope == nil {
		t.Fatal("expected non-nil result and envelope for unsupported file diff")
	}
	if len(res.Envelope.Actions) == 0 {
		t.Error("expected fallback line diff actions for unsupported files, got 0")
	}
}
