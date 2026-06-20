/*
Copyright © 2026 Harsh Kapse <harshkapse.dev@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/tui"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// diffCmd represents the diff command
var diffCmd = &cobra.Command{
	Use:   "diff [file-a] [file-b]",
	Short: "Compute semantic diff between two files",
	Long: `Compute a structural, AST-aware diff between two source files.

diffmantic parses both files with Tree-sitter and runs a multi-phase matching
algorithm to detect inserts, deletes, updates, moves, and renames at the syntax
node level, not just changed lines.

Examples:
  diffmantic diff before.go after.go                Interactive side-by-side viewer
  diffmantic diff before.go after.go -f json        JSON output for editor plugins
  diffmantic diff before.go after.go -f actions     Print structural actions list
  diffmantic diff before.go after.go --lang go      Override language detection`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fileA, fileB := args[0], args[1]
		// lang, _ := cmd.Flags().GetString("lang")
		format, _ := cmd.Flags().GetString("format")
		if format != "tui" && format != "json" && format != "actions" {
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: tui, json, actions\n", format)
			os.Exit(1)
		}

		infoA, err := os.Stat(fileA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error stating %s: %v\n", fileA, err)
			os.Exit(1)
		}
		infoB, err := os.Stat(fileB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error stating %s: %v\n", fileB, err)
			os.Exit(1)
		}
		isDirA := infoA.IsDir()
		isDirB := infoB.IsDir()
		if isDirA != isDirB {
			fmt.Fprintln(os.Stderr, "Error: Both arguments must be files or both directories")
			os.Exit(1)
		}
		if isDirA && isDirB {
			if format != "tui" {
				fmt.Fprintln(os.Stderr, "Error: Directory diffing is only supported in TUI mode currently")
				os.Exit(1)
			}
			diffFiles, err := diffDirectories(fileA, fileB)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error diffing directories: %v\n", err)
				os.Exit(1)
			}
			if err := tui.Run(diffFiles); err != nil {
				fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
				os.Exit(1)
			}
			return
		}
		var (
			srcA []byte
			srcB []byte
			astA *treesitter.ASTNode
			astB *treesitter.ASTNode
		)

		g := new(errgroup.Group)

		g.Go(func() error {
			data, err := os.ReadFile(fileA)
			if err != nil {
				return fmt.Errorf("error reading %s: %w", fileA, err)
			}
			srcA = data
			return nil
		})

		g.Go(func() error {
			data, err := os.ReadFile(fileB)
			if err != nil {
				return fmt.Errorf("error reading %s: %w", fileB, err)
			}
			srcB = data
			return nil
		})

		if err := g.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		if format == "tui" {
			file := tui.NewDiffFile(fileA, fileB, srcA, srcB, nil)
			file.NeedsCompute = true
			if err := tui.Run([]tui.DiffFile{file}); err != nil {
				fmt.Fprintf(os.Stderr, "error running TUI: %v\n", err)
				os.Exit(1)
			}
			return
		}
		g = new(errgroup.Group)

		g.Go(func() error {
			parsed, err := treesitter.Parse(srcA, fileA)
			if err != nil {
				return fmt.Errorf("error parsing %s: %w", fileA, err)
			}
			astA = parsed
			return nil
		})

		g.Go(func() error {
			parsed, err := treesitter.Parse(srcB, fileB)
			if err != nil {
				return fmt.Errorf("error parsing %s: %w", fileB, err)
			}
			astB = parsed
			return nil
		})

		if err := g.Wait(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}

		// fmt.Println("=== AST A ===")
		// treesitter.PrintAST(astA, 0)
		// fmt.Println("=== AST B ===")
		// treesitter.PrintAST(astB, 0)

		result := engine.Match(astA, astB)
		es := actions.GenerateEditScript(astA, astB, result.Mappings)
		es = actions.Simplify(es)

		switch format {
		case "json":
			if err := actions.WriteJSON(os.Stdout, es, result.Mappings); err != nil {
				fmt.Fprintf(os.Stderr, "error writing JSON: %v\n", err)
				os.Exit(1)
			}
		case "actions":
			fmt.Printf("Diffing  %s  →  %s\n\n", fileA, fileB)
			engine.PrintMappings(result)
			actions.PrintActions(es)
		default:
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: tui, json, actions\n", format)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringP("format", "f", "tui", "Output format: tui, json, actions")
	diffCmd.Flags().StringP("lang", "l", "", "Override language detection (e.g., go, python, c)")
}

func diffDirectories(dirA, dirB string) ([]tui.DiffFile, error) {
	filesA := make(map[string]string)
	filesB := make(map[string]string)

	err := filepath.WalkDir(dirA, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirA, path)
		if err != nil {
			return err
		}
		filesA[rel] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = filepath.WalkDir(dirB, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(dirB, path)
		if err != nil {
			return err
		}
		filesB[rel] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	var allRels []string
	seen := make(map[string]bool)
	for rel := range filesA {
		if !seen[rel] {
			seen[rel] = true
			allRels = append(allRels, rel)
		}
	}
	for rel := range filesB {
		if !seen[rel] {
			seen[rel] = true
			allRels = append(allRels, rel)
		}
	}
	sort.Strings(allRels)
	var diffFiles []tui.DiffFile
	for _, rel := range allRels {
		pathA := filesA[rel]
		pathB := filesB[rel]
		var srcA, srcB []byte
		if pathA != "" {
			data, err := os.ReadFile(pathA)
			if err != nil {
				return nil, fmt.Errorf("error reading %s: %w", pathA, err)
			}
			srcA = data
		}
		if pathB != "" {
			data, err := os.ReadFile(pathB)
			if err != nil {
				return nil, fmt.Errorf("error reading %s: %w", pathB, err)
			}
			srcB = data
		}
		file := tui.NewDiffFile(pathA, pathB, srcA, srcB, nil)
		file.RelPath = rel
		file.NeedsCompute = true
		diffFiles = append(diffFiles, file)
	}
	return diffFiles, nil
}
