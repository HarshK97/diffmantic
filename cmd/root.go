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
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/config"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/theme"
	"github.com/HarshK97/diffmantic/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "diffm [refA] [refB]",
	Version: "0.6.0",
	Short:   "Semantic diff engine powered by Tree-sitter",
	Long: `diffmantic is a structural source code diff engine.

It parses source files into ASTs using Tree-sitter and computes semantic
differences. It detects not just what lines changed, but what code structures
were inserted, deleted, updated, moved, or renamed.

Works as a standalone file diff tool, a git difftool, or a backend engine for
editor plugins (Neovim, VS Code) via JSON output.

Examples:
  diffm before.go after.go                 Interactive TUI (default)
  diffm before.go after.go -f json         JSON output for editor plugins
  diffm before.go after.go -f actions      Print structural actions list
  diffm                                    Interactive Git mode (unstaged diff)
  diffm --cached                           Interactive Git mode (staged diff)
  diffm HEAD~1 HEAD                        Interactive Git mode comparing revisions`,
	Args: cobra.ArbitraryArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
		}
		if cfg == nil {
			defaultCfg := config.DefaultConfig()
			cfg = &defaultCfg
		}

		format, _ := cmd.Flags().GetString("format")
		if !cmd.Flags().Changed("format") && cfg.Format != "" {
			format = cfg.Format
		}
		if format != "" && !slices.Contains([]string{"json", "actions", "tui"}, format) {
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: json, actions, tui\n", format)
			os.Exit(1)
		}

		ignoreComments, _ := cmd.Flags().GetBool("ignore-comments")
		if !cmd.Flags().Changed("ignore-comments") {
			ignoreComments = cfg.IgnoreComments
		}

		parseErrorLimit, _ := cmd.Flags().GetInt("parse-error-limit")
		if !cmd.Flags().Changed("parse-error-limit") {
			parseErrorLimit = cfg.ParseErrorLimit
		}

		themeName, _ := cmd.Flags().GetString("theme")
		if !cmd.Flags().Changed("theme") && cfg.Theme != "" {
			themeName = cfg.Theme
		}
		th, err := theme.ResolveThemeWithConfig(themeName, cfg.ThemeStyle, cfg.Themes)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Two args: diff two files directly, or compare revisions/paths if in a git repo.
		if len(args) == 2 {
			argA, argB := args[0], args[1]
			infoA, errA := os.Stat(argA)
			infoB, errB := os.Stat(argB)

			if (errA == nil && infoA.IsDir()) || (errB == nil && infoB.IsDir()) {
				fmt.Fprintln(os.Stderr, "Error: Directory diffing is not supported yet")
				os.Exit(1)
			}

			// Case 1: Both exist on disk as files or /dev/null
			if isFileOrDevNull(argA) && isFileOrDevNull(argB) {
				runFileDiff(cmd, argA, argB, format, ignoreComments, parseErrorLimit, th)
				return
			}

			// Git repository inspection
			if git.IsGitRepository(".") {
				isRevA := git.IsValidRevision(".", argA)
				isRevB := git.IsValidRevision(".", argB)
				isTrackedOrFileA := isFileOrDevNull(argA) || git.IsTrackedFile(".", argA)
				isTrackedOrFileB := isFileOrDevNull(argB) || git.IsTrackedFile(".", argB)

				// Case 2: Two Git revisions (e.g. diffm main feature-branch)
				if isRevA && isRevB {
					runGitMode(cmd, []string{argA, argB}, format, ignoreComments, parseErrorLimit, th)
					return
				}

				// Case 3: One revision and one tracked/existing file path (e.g. diffm main internal/config.go)
				if isRevA && isTrackedOrFileB {
					runGitMode(cmd, []string{argA, argB}, format, ignoreComments, parseErrorLimit, th)
					return
				}
				if isRevB && isTrackedOrFileA {
					runGitMode(cmd, []string{argB, argA}, format, ignoreComments, parseErrorLimit, th)
					return
				}

				// Case 4: Invalid argument detection (prevents silent typo routing)
				if !isRevA && !isTrackedOrFileA {
					fmt.Fprintf(os.Stderr, "Error: %q is neither a valid file nor a valid Git revision\n", argA)
					os.Exit(1)
				}
				if !isRevB && !isTrackedOrFileB {
					fmt.Fprintf(os.Stderr, "Error: %q is neither a valid file nor a valid Git revision\n", argB)
					os.Exit(1)
				}
			}

			// Neither arg is a git ref; report stat errors for missing files
			if errA != nil && argA != os.DevNull {
				fmt.Fprintf(os.Stderr, "Error: statting %s: %v\n", argA, errA)
				os.Exit(1)
			}
			if errB != nil && argB != os.DevNull {
				fmt.Fprintf(os.Stderr, "Error: statting %s: %v\n", argB, errB)
				os.Exit(1)
			}
		}

		// Single argument in Git repo: validate ref or tracked path
		if len(args) == 1 && git.IsGitRepository(".") {
			arg := args[0]
			isRev := git.IsValidRevision(".", arg)
			isTrackedOrFile := isFileOrDevNull(arg) || git.IsTrackedFile(".", arg)
			if !isRev && !isTrackedOrFile {
				fmt.Fprintf(os.Stderr, "Error: %q is neither a valid file nor a valid Git revision\n", arg)
				os.Exit(1)
			}
		}

		// In a git repo, launch interactive mode (optionally filtered by ref or path)
		if git.IsGitRepository(".") {
			runGitMode(cmd, args, format, ignoreComments, parseErrorLimit, th)
			return
		}

		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: Git mode requires a valid Git repository\n")
			os.Exit(1)
		}

		_ = cmd.Help()
	},
}

func runGitMode(cmd *cobra.Command, args []string, format string, ignoreComments bool, parseErrorLimit int, th *theme.Theme) {
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

	// Handle headless / non-interactive output formats in Git mode
	if format == "json" || format == "actions" {
		if pathFilter != "" {
			runGitFileDiff(cmd, refA, refB, pathFilter, stagedOnly, format, ignoreComments, parseErrorLimit, th)
			return
		}
		if !isatty.IsTerminal(os.Stdout.Fd()) && !isatty.IsTerminal(os.Stderr.Fd()) {
			refLabel := refA
			if refLabel == "" {
				refLabel = "HEAD"
			}
			fmt.Fprintf(os.Stderr, "Error: Git mode non-interactive output (%s) requires an explicit file path (e.g. diffm %s <file> -f %s)\n", format, refLabel, format)
			os.Exit(1)
		}
	}

	if err := tui.RunGit(".", refA, refB, pathFilter, stagedOnly, th); err != nil {
		fmt.Fprintf(os.Stderr, "Error: running Git interactive diff: %v\n", err)
		os.Exit(1)
	}
}

func runGitFileDiff(cmd *cobra.Command, refA, refB, pathFilter string, stagedOnly bool, format string, ignoreComments bool, parseErrorLimit int, th *theme.Theme) {
	var srcBytes, dstBytes []byte
	var err error

	if stagedOnly {
		srcBytes, err = git.GetContent(".", pathFilter, "HEAD")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading HEAD:%s: %v\n", pathFilter, err)
			os.Exit(1)
		}
		dstBytes, err = git.GetContent(".", pathFilter, ":")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading index:%s: %v\n", pathFilter, err)
			os.Exit(1)
		}
	} else if refA != "" && refB != "" {
		srcBytes, err = git.GetContent(".", pathFilter, refA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s:%s: %v\n", refA, pathFilter, err)
			os.Exit(1)
		}
		dstBytes, err = git.GetContent(".", pathFilter, refB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s:%s: %v\n", refB, pathFilter, err)
			os.Exit(1)
		}
	} else if refA != "" {
		srcBytes, err = git.GetContent(".", pathFilter, refA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s:%s: %v\n", refA, pathFilter, err)
			os.Exit(1)
		}
		dstBytes, err = git.GetContent(".", pathFilter, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading working tree %s: %v\n", pathFilter, err)
			os.Exit(1)
		}
	} else {
		// Working tree unstaged changes
		srcBytes, err = git.GetContent(".", pathFilter, ":")
		if err != nil || len(srcBytes) == 0 {
			srcBytes, _ = git.GetContent(".", pathFilter, "HEAD")
		}
		dstBytes, err = git.GetContent(".", pathFilter, "")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading working tree %s: %v\n", pathFilter, err)
			os.Exit(1)
		}
	}

	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	includeUI := format == "tui" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format != "tui" && !uiMode || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	dr, err := pipeline.Run(srcBytes, dstBytes, pathFilter, pathFilter, pipeline.DiffOptions{
		ParseErrorLimit: parseErrorLimit,
		IgnoreComments:  ignoreComments,
		EnvelopeOpts:    opts,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serializing JSON: %v\n", err)
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(jsonData)
		_, _ = os.Stdout.Write([]byte("\n"))
	case "actions":
		refSrc := refA
		if refSrc == "" {
			refSrc = "HEAD"
		}
		refDst := refB
		if refDst == "" {
			refDst = "working"
		}
		fmt.Printf("Diffing  %s:%s  →  %s:%s\n\n", refSrc, pathFilter, refDst, pathFilter)
		_ = engine.FprintMappings(os.Stdout, dr.MatchResult)
		_ = actions.FprintActions(os.Stdout, dr.EditScript)
	case "tui":
		if err := tui.Run(dr.SrcFile, dr.DstFile, dr.SrcBytes, dr.DstBytes, dr.Envelope, th); err != nil {
			fmt.Fprintf(os.Stderr, "Error: running TUI: %v\n", err)
			os.Exit(1)
		}
	}
}

func isFileOrDevNull(path string) bool {
	if path == os.DevNull {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func runFileDiff(cmd *cobra.Command, fileA, fileB string, format string, ignoreComments bool, parseErrorLimit int, th *theme.Theme) {
	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	if format == "" {
		if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsTerminal(os.Stderr.Fd()) {
			format = "tui"
		} else {
			format = "json"
		}
	}

	includeUI := format == "tui" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format != "tui" && !uiMode || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	dr, err := pipeline.RunFiles(fileA, fileB, pipeline.DiffOptions{
		ParseErrorLimit: parseErrorLimit,
		IgnoreComments:  ignoreComments,
		EnvelopeOpts:    opts,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serializing JSON: %v\n", err)
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(jsonData)
		_, _ = os.Stdout.Write([]byte("\n"))
	case "actions":
		fmt.Printf("Diffing  %s  →  %s\n\n", fileA, fileB)
		_ = engine.FprintMappings(os.Stdout, dr.MatchResult)
		_ = actions.FprintActions(os.Stdout, dr.EditScript)
	case "tui":
		if err := tui.Run(dr.SrcFile, dr.DstFile, dr.SrcBytes, dr.DstBytes, dr.Envelope, th); err != nil {
			fmt.Fprintf(os.Stderr, "Error: running TUI: %v\n", err)
			os.Exit(1)
		}
	}
}

// Execute runs the CLI and exits on error.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("format", "f", "", "Output format: json, actions, tui (default: tui if interactive, json otherwise)")
	rootCmd.Flags().StringP("theme", "t", "", "Color theme: mocha (dark), latte (light), or custom theme name")
	rootCmd.Flags().BoolP("ignore-comments", "C", false, "Ignore all comments when diffing")
	rootCmd.Flags().IntP("parse-error-limit", "e", 0, "Maximum parse errors allowed before falling back to line diffing")
	rootCmd.Flags().Bool("ui", false, "Include line alignment and highlight spans in JSON output")
	rootCmd.Flags().Bool("full", false, "Include actions, line alignment, and highlight spans in JSON output")
	rootCmd.Flags().Bool("cached", false, "Show only staged changes in Git mode")
}
