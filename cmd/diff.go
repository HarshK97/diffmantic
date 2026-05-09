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

	"github.com/spf13/cobra"
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
  diffmantic diff before.go after.go -f unified     Unified diff for scripts/CI
  diffmantic diff before.go after.go --lang go      Override language detection`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fileA, fileB := args[0], args[1]
		format, _ := cmd.Flags().GetString("format")
		lang, _ := cmd.Flags().GetString("lang")

		fmt.Printf("diffing %s vs %s (format=%s, lang=%s)\n", fileA, fileB, format, lang)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
	diffCmd.Flags().StringP("format", "f", "tui", "Output format: tui, json, unified")
	diffCmd.Flags().StringP("lang", "l", "", "Override language detection (e.g., go, python, c)")
}
