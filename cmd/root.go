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

	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "diffm [refA] [refB]",
	Version: "0.3.0",
	Short:   "Semantic diff engine powered by Tree-sitter",
	Long: `diffmantic is a structural source code diff engine.

It parses source files into ASTs using Tree-sitter and computes semantic
differences. It detects not just what lines changed, but what code structures
were inserted, deleted, updated, moved, or renamed.

Works as a standalone CLI, a git difftool, or a backend engine for editor
plugins (Neovim, VS Code) via JSON output.

Supported languages: Go, JavaScript, TypeScript, Python.`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if git.IsGitRepository(".") {
			stagedOnly, _ := cmd.Flags().GetBool("cached")

			var refs, paths []string
			for _, arg := range args {
				if git.IsValidRevision(".", arg) {
					refs = append(refs, arg)
				} else {
					paths = append(paths, arg)
				}
			}

			var refA, refB, pathFilter string
			if len(refs) > 0 {
				refA = refs[0]
			}
			if len(refs) > 1 {
				refB = refs[1]
			}
			if len(paths) > 0 {
				pathFilter = paths[0]
			}

			if err := tui.RunGit(".", refA, refB, pathFilter, stagedOnly); err != nil {
				fmt.Fprintf(os.Stderr, "error running Git interactive diff: %v\n", err)
				os.Exit(1)
			}
		} else {
			if len(args) > 0 {
				fmt.Fprintf(os.Stderr, "Error: Git mode requires a valid Git repository\n")
				os.Exit(1)
			}
			_ = cmd.Help()
		}
	},
}

// Execute runs the CLI and exits on error.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().Bool("cached", false, "Show only staged changes in Git mode")
}
