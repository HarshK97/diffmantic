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
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/config"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/inline"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var rootCmd = &cobra.Command{
	Use:     "diffm [refA] [refB]",
	Version: "0.7.0",
	Short:   "Semantic diff engine powered by Tree-sitter",
	Long: `diffmantic is a structural source code diff engine.

It parses source files into ASTs using Tree-sitter and computes semantic
differences. It detects not just what lines changed, but what code structures
were inserted, deleted, updated, moved, or renamed.

Works as a standalone file diff tool, a git difftool, or a backend engine for
editor plugins (Neovim, VS Code) via JSON output.

Examples:
  diffm before.go after.go                 Inline diff with pager (default in TTY)
  diffm before.go after.go -f inline       Print AST-aware inline diff with pager
  diffm before.go after.go -f json         JSON output for editor plugins
  diffm before.go after.go -f actions      Print structural actions list
  diffm                                    Git mode on unstaged changes
  diffm -f inline                          Git mode inline diff with pager
  diffm --cached -f inline                 Git staged changes inline diff
  diffm HEAD~1 HEAD -f inline              Git revision comparison in inline diff`,
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

		noPager, _ := cmd.Flags().GetBool("no-pager")
		patchMode, _ := cmd.Flags().GetBool("patch")
		if patchMode && !cmd.Flags().Changed("no-pager") {
			noPager = true
		}

		format, _ := cmd.Flags().GetString("format")
		if patchMode && !cmd.Flags().Changed("format") {
			format = "inline"
		} else if !cmd.Flags().Changed("format") && cfg.Format != "" {
			format = cfg.Format
		}
		if format != "" && !slices.Contains([]string{"json", "actions", "inline"}, format) {
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: json, actions, inline\n", format)
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

		parseTree, _ := cmd.Flags().GetBool("parse-tree")
		isCST, _ := cmd.Flags().GetBool("cst")
		if parseTree || isCST {
			runParseTree(cmd, args, noPager, isCST)
			return
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
				runFileDiff(cmd, argA, argB, format, ignoreComments, parseErrorLimit, noPager)
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
					runGitMode(cmd, []string{argA, argB}, format, ignoreComments, parseErrorLimit, noPager)
					return
				}

				// Case 3: One revision and one tracked/existing file path (e.g. diffm main internal/config.go)
				if isRevA && isTrackedOrFileB {
					runGitMode(cmd, []string{argA, argB}, format, ignoreComments, parseErrorLimit, noPager)
					return
				}
				if isRevB && isTrackedOrFileA {
					runGitMode(cmd, []string{argB, argA}, format, ignoreComments, parseErrorLimit, noPager)
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
			runGitMode(cmd, args, format, ignoreComments, parseErrorLimit, noPager)
			return
		}

		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: Git mode requires a valid Git repository\n")
			os.Exit(1)
		}

		_ = cmd.Help()
	},
}

func runGitMode(cmd *cobra.Command, args []string, format string, ignoreComments bool, parseErrorLimit int, noPager bool) {
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

	if format == "" {
		format = "inline"
	}

	// Git modes: inline, json, actions
	files, err := git.GetChangedFiles(".", refA, refB, pathFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: retrieving git status: %v\n", err)
		os.Exit(1)
	}

	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	includeUI := format == "inline" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "inline" || !uiMode || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	renderOpts := resolveRenderOptions(cmd)

	type diffTarget struct {
		srcFile  string
		dstFile  string
		srcBytes []byte
		dstBytes []byte
	}

	var targets []diffTarget

	if refA == "" {
		for _, f := range files {
			if stagedOnly && !f.Staged {
				continue
			}

			srcFile := f.Path
			if f.OldPath != "" {
				srcFile = f.OldPath
			}
			dstFile := f.Path

			if f.Staged {
				srcBytes, _ := git.GetContent(".", srcFile, "HEAD")
				dstBytes, _ := git.GetContent(".", dstFile, ":")
				if !bytes.Equal(srcBytes, dstBytes) {
					targets = append(targets, diffTarget{
						srcFile:  srcFile,
						dstFile:  dstFile,
						srcBytes: srcBytes,
						dstBytes: dstBytes,
					})
				}
			}

			if f.Unstaged && !stagedOnly {
				revA := ":"
				if !f.Staged {
					revA = "HEAD"
				}
				srcBytes, _ := git.GetContent(".", srcFile, revA)
				dstBytes, _ := git.GetContent(".", dstFile, "")
				if !bytes.Equal(srcBytes, dstBytes) {
					targets = append(targets, diffTarget{
						srcFile:  srcFile,
						dstFile:  dstFile,
						srcBytes: srcBytes,
						dstBytes: dstBytes,
					})
				}
			}
		}
	} else {
		for _, f := range files {
			srcFile := f.Path
			if f.OldPath != "" {
				srcFile = f.OldPath
			}
			dstFile := f.Path

			srcBytes, _ := git.GetContent(".", srcFile, refA)
			dstBytes, _ := git.GetContent(".", dstFile, refB)

			if !bytes.Equal(srcBytes, dstBytes) {
				targets = append(targets, diffTarget{
					srcFile:  srcFile,
					dstFile:  dstFile,
					srcBytes: srcBytes,
					dstBytes: dstBytes,
				})
			}
		}
	}

	if len(targets) == 0 && pathFilter != "" && (isFileOrDevNull(pathFilter) || git.IsTrackedFile(".", pathFilter)) {
		var srcBytes, dstBytes []byte
		if refA == "" {
			if stagedOnly {
				srcBytes, _ = git.GetContent(".", pathFilter, "HEAD")
				dstBytes, _ = git.GetContent(".", pathFilter, ":")
			} else {
				srcBytes, _ = git.GetContent(".", pathFilter, ":")
				if len(srcBytes) == 0 {
					srcBytes, _ = git.GetContent(".", pathFilter, "HEAD")
				}
				dstBytes, _ = git.GetContent(".", pathFilter, "")
			}
		} else {
			srcBytes, _ = git.GetContent(".", pathFilter, refA)
			dstBytes, _ = git.GetContent(".", pathFilter, refB)
		}

		if format == "json" || !bytes.Equal(srcBytes, dstBytes) {
			targets = append(targets, diffTarget{
				srcFile:  pathFilter,
				dstFile:  pathFilter,
				srcBytes: srcBytes,
				dstBytes: dstBytes,
			})
		}
	}

	if len(targets) == 0 {
		return
	}

	var p *pager.Pager
	var writer io.Writer = os.Stdout

	if format == "inline" || format == "actions" {
		p, writer = pager.Start(noPager)
		defer p.Close()
	}

	for _, t := range targets {
		dr, err := pipeline.Run(t.srcBytes, t.dstBytes, t.srcFile, t.dstFile, pipeline.DiffOptions{
			ParseErrorLimit: parseErrorLimit,
			IgnoreComments:  ignoreComments,
			EnvelopeOpts:    opts,
		})
		if err != nil {
			continue
		}

		switch format {
		case "inline":
			output := inline.Render(t.srcFile, t.dstFile, t.srcBytes, t.dstBytes, dr.Envelope, renderOpts)
			if output != "" {
				if _, err := io.WriteString(writer, output); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: writing diff output: %v\n", err)
					return
				}
				if !strings.HasSuffix(output, "\n") {
					if _, err := io.WriteString(writer, "\n"); err != nil {
						if pager.IsBrokenPipe(err) {
							return
						}
						fmt.Fprintf(os.Stderr, "Error: writing newline: %v\n", err)
						return
					}
				}
			}
		case "json":
			jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
			if err == nil {
				if _, err := writer.Write(jsonData); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
				}
				if _, err := writer.Write([]byte("\n")); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
				}
			}
		case "actions":
			if _, err := fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", t.srcFile, t.dstFile); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
			}
			_ = engine.FprintMappings(writer, dr.MatchResult)
			_ = actions.FprintActions(writer, dr.EditScript)
		}
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

func resolveRenderOptions(cmd *cobra.Command) inline.RenderOptions {
	patchMode, _ := cmd.Flags().GetBool("patch")

	colorFlag, _ := cmd.Flags().GetString("color")
	if patchMode && !cmd.Flags().Changed("color") {
		colorFlag = "never"
	}

	contextLines, _ := cmd.Flags().GetInt("context")
	if contextLines < 0 {
		contextLines = 3
	}

	lineNumbers, _ := cmd.Flags().GetBool("line-numbers")
	if patchMode && !cmd.Flags().Changed("line-numbers") {
		lineNumbers = false
	}

	annotations, _ := cmd.Flags().GetBool("annotations")
	if patchMode && !cmd.Flags().Changed("annotations") {
		annotations = false
	}

	var useColor bool
	switch colorFlag {
	case "always":
		useColor = true
	case "never":
		useColor = false
	default:
		useColor = isTerminal(os.Stdout)
	}

	wrapChanged := cmd.Flags().Changed("wrap")
	wrapFlag, _ := cmd.Flags().GetBool("wrap")
	wrapWidth, _ := cmd.Flags().GetInt("wrap-width")

	if patchMode {
		wrapFlag = false
	} else if !wrapChanged && wrapWidth > 0 && cmd.Flags().Changed("wrap-width") {
		wrapFlag = true
	}
	if !wrapChanged && !cmd.Flags().Changed("wrap-width") && !isTerminal(os.Stdout) {
		wrapFlag = false
	}

	termWidth := wrapWidth
	if wrapFlag && termWidth <= 0 {
		if wFd, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && wFd > 0 {
			termWidth = wFd
		} else if colStr := os.Getenv("COLUMNS"); colStr != "" {
			if cols, err := strconv.Atoi(colStr); err == nil && cols > 0 {
				termWidth = cols
			}
		}
		if termWidth <= 0 && isTerminal(os.Stdout) {
			termWidth = 80
		}
	}

	if termWidth <= 0 {
		wrapFlag = false
	}

	cfg, _ := config.Load()
	tabWidth, _ := cmd.Flags().GetInt("tab-width")
	if !cmd.Flags().Changed("tab-width") && cfg != nil && cfg.TabWidth > 0 {
		tabWidth = cfg.TabWidth
	}
	if tabWidth <= 0 {
		tabWidth = 4
	}

	return inline.RenderOptions{
		Color:              useColor,
		ContextLines:       contextLines,
		LineNumbers:        lineNumbers,
		DisableAnnotations: !annotations,
		Wrap:               wrapFlag,
		TerminalWidth:      termWidth,
		TabWidth:           tabWidth,
	}
}

func isFileOrDevNull(path string) bool {
	if path == os.DevNull {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func runFileDiff(cmd *cobra.Command, fileA, fileB string, format string, ignoreComments bool, parseErrorLimit int, noPager bool) {
	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	if format == "" {
		format = "inline"
	}

	includeUI := format == "inline" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "inline" || !uiMode || fullMode,
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
	case "inline":
		p, writer := pager.Start(noPager)
		defer p.Close()

		renderOpts := resolveRenderOptions(cmd)
		output := inline.Render(fileA, fileB, dr.SrcBytes, dr.DstBytes, dr.Envelope, renderOpts)
		if output != "" {
			if _, err := io.WriteString(writer, output); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error: writing diff output: %v\n", err)
				os.Exit(1)
			}
			if !strings.HasSuffix(output, "\n") {
				if _, err := io.WriteString(writer, "\n"); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: writing newline: %v\n", err)
					os.Exit(1)
				}
			}
		}
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serializing JSON: %v\n", err)
			os.Exit(1)
		}
		_, _ = os.Stdout.Write(jsonData)
		_, _ = os.Stdout.Write([]byte("\n"))
	case "actions":
		p, writer := pager.Start(noPager)
		defer p.Close()
		_, _ = fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", fileA, fileB)
		_ = engine.FprintMappings(writer, dr.MatchResult)
		_ = actions.FprintActions(writer, dr.EditScript)
	}
}

func runParseTree(cmd *cobra.Command, args []string, noPager bool, isCST bool) {
	if len(args) == 0 {
		flagName := "--parse-tree"
		if isCST {
			flagName = "--cst"
		}
		fmt.Fprintf(os.Stderr, "Error: %s requires at least one file argument\n", flagName)
		os.Exit(1)
	}

	p, writer := pager.Start(noPager)
	if p != nil {
		defer p.Close()
	}

	bw := bufio.NewWriter(writer)
	defer func() {
		_ = bw.Flush()
	}()

	if err := dumpFiles(bw, args, isCST); err != nil {
		_ = bw.Flush()
		if p != nil {
			p.Close()
		}
		if !pager.IsBrokenPipe(err) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}
}

func dumpFiles(w io.Writer, paths []string, isCST bool) error {
	multiple := len(paths) > 1

	for i, path := range paths {
		srcBytes, err := os.ReadFile(path)
		if err != nil {
			if git.IsGitRepository(".") && git.IsTrackedFile(".", path) {
				srcBytes, err = git.GetContent(".", path, "")
			}
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
		}

		if multiple {
			if i > 0 {
				if _, err := fmt.Fprintln(w); err != nil {
					return err
				}
			}
			if _, err := fmt.Fprintf(w, "=== %s ===\n", path); err != nil {
				return err
			}
		}

		lang, err := treesitter.DetectLanguage(path)
		if err != nil {
			return fmt.Errorf("detecting language for %s: %w", path, err)
		}

		if isCST {
			_, flatNodes, symbols, err := treesitter.ParseForPipeline(srcBytes, lang.Name)
			if err != nil {
				return fmt.Errorf("parsing CST for %s: %w", path, err)
			}
			if err := treesitter.DumpCST(w, flatNodes, symbols, srcBytes); err != nil {
				return err
			}
		} else {
			ast, err := treesitter.ParseWithLanguage(srcBytes, lang.Name)
			if err != nil {
				return fmt.Errorf("parsing AST for %s: %w", path, err)
			}
			if err := treesitter.DumpAST(w, ast); err != nil {
				return err
			}
		}
	}

	return nil
}

// Execute runs the CLI and exits on error.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("format", "f", "", "Output format: json, actions, inline (default: inline)")
	rootCmd.Flags().BoolP("ignore-comments", "C", false, "Ignore all comments when diffing")
	rootCmd.Flags().IntP("parse-error-limit", "e", 0, "Maximum parse errors allowed before falling back to line diffing")
	rootCmd.Flags().Bool("ui", false, "Include line alignment and highlight spans in JSON output")
	rootCmd.Flags().Bool("full", false, "Include actions, line alignment, and highlight spans in JSON output")
	rootCmd.Flags().Bool("cached", false, "Show only staged changes in Git mode")
	rootCmd.Flags().String("color", "auto", "Color output: always, never, auto")
	rootCmd.Flags().IntP("context", "U", 3, "Lines of context to show for inline diff")
	rootCmd.Flags().BoolP("line-numbers", "n", true, "Show line numbers in inline diff output")
	rootCmd.Flags().Bool("annotations", true, "Include AST move annotations in inline diff")
	rootCmd.Flags().BoolP("patch", "p", false, "Generate a standard patch suitable for git apply / patch tools")
	rootCmd.Flags().Bool("no-pager", false, "Do not pipe output into a pager")
	rootCmd.Flags().Bool("wrap", false, "Wrap long lines to terminal width in inline diff")
	rootCmd.Flags().Int("wrap-width", 0, "Explicit column width for line wrapping (0 to auto-detect terminal width)")
	rootCmd.Flags().Int("tab-width", 4, "Number of spaces per tab stop in inline diff")
	rootCmd.Flags().Bool("parse-tree", false, "Parse files and dump the syntax tree for debugging")
	rootCmd.Flags().Bool("cst", false, "Dump the raw Tree-sitter concrete syntax tree instead of the Diffmantic AST")
}
