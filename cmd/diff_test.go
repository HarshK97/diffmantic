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
