package cmd

import (
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

	l := diffCmd.Flags().Lookup("lang")
	if l == nil {
		t.Fatal("lang flag not registered")
	}
}
