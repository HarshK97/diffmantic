package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/HarshK97/diffmantic/cmd"
)

func main() {
	outDir := "man"
	if len(os.Args) > 1 {
		outDir = os.Args[1]
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory %s: %v\n", outDir, err)
		os.Exit(1)
	}

	content := cmd.GenerateManPage(cmd.RootCmd(), "2026-09-19")

	targets := []string{"diffm.1", "diffmantic.1"}
	for _, target := range targets {
		dest := filepath.Join(outDir, target)
		if err := os.WriteFile(dest, []byte(content), 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", dest, err)
			os.Exit(1)
		}
		fmt.Printf("Generated %s\n", dest)
	}
}
