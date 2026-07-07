package cmd

import (
	"testing"
)

func TestDiffCmdRegistered(t *testing.T) {
	// Verify the diff subcommand exists on rootCmd.
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
	if f.DefValue != "json" {
		t.Errorf("format default = %q, want %q", f.DefValue, "json")
	}

	l := diffCmd.Flags().Lookup("lang")
	if l == nil {
		t.Fatal("lang flag not registered")
	}
}
