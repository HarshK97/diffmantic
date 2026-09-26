package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestManPageUpToDate(t *testing.T) {
	expected := GenerateManPage(rootCmd, "2026-09-26")

	targets := []string{
		filepath.Join("..", "man", "diffm.1"),
		filepath.Join("..", "man", "diffmantic.1"),
	}

	for _, target := range targets {
		actual, err := os.ReadFile(target)
		if err != nil {
			t.Fatalf("reading %s failed: %v. Run 'make man' to generate manual pages.", target, err)
		}

		actualStr := strings.ReplaceAll(string(actual), "\r\n", "\n")
		if actualStr != expected {
			t.Errorf("%s is out of date with current CLI flags. Run 'make man' to regenerate.", target)
		}
	}
}

func TestManPageLint(t *testing.T) {
	mandocPath, err := exec.LookPath("mandoc")
	if err != nil {
		t.Skip("mandoc not found in PATH; skipping troff syntax lint")
	}

	targets := []string{
		filepath.Join("..", "man", "diffm.1"),
		filepath.Join("..", "man", "diffmantic.1"),
	}

	for _, target := range targets {
		cmd := exec.Command(mandocPath, "-Tlint", target)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Errorf("mandoc -Tlint %s failed: %v\nOutput:\n%s", target, err, string(out))
		}
	}
}
