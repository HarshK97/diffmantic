// Package cmd implements the CLI commands for diffm.
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/inline"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	defaultTabWidth       = 4
	defaultSizeLimitKB    = 1024
	defaultLineLimitLines = 10000
)

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && (fi.Mode()&os.ModeCharDevice) != 0
}

var rootCmd = &cobra.Command{
	Use:     "diffm [refA] [refB]",
	Version: "0.9.0",
	Short:   "Semantic diff engine powered by Tree-sitter",
	Long: `diffmantic is a structural source code diff engine.

It parses source files into ASTs using Tree-sitter and computes semantic
differences. It detects not just what lines changed, but what code structures
were inserted, deleted, updated, moved, or renamed.

Works as a standalone file diff tool, a git difftool, or a backend engine for
editor plugins (Neovim, VS Code) via JSON output.`,
	Example: `  diffm before.go after.go                     Side-by-side diff with pager (default in TTY)
  diffm before.go after.go -f inline           Print AST-aware inline diff with pager
  diffm before.go after.go -f json             JSON output for editor plugins
  diffm before.go after.go -f actions          Print structural actions list
  diffm                                        Git mode on unstaged changes
  diffm -f inline                              Git mode inline diff with pager
  diffm --cached                               Git staged changes diff
  diffm HEAD~1 HEAD                            Git revision comparison`,
	Args: cobra.ArbitraryArgs,
	ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if !git.IsGitRepository(".") {
			return nil, cobra.ShellCompDirectiveDefault
		}
		refs, err := git.ListRefs(".", toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveDefault
		}
		return refs, cobra.ShellCompDirectiveDefault
	},
	Run: func(cmd *cobra.Command, args []string) {
		if err := validateNoMisplacedFlags(args); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		noPager, _ := cmd.Flags().GetBool("no-pager")
		patchMode, _ := cmd.Flags().GetBool("patch")
		if patchMode && !cmd.Flags().Changed("no-pager") {
			noPager = true
		}

		format, _ := cmd.Flags().GetString("format")
		if patchMode && !cmd.Flags().Changed("format") {
			format = "inline"
		} else if !cmd.Flags().Changed("format") {
			if envFmt := getEnvString("DIFFM_FORMAT", ""); envFmt != "" {
				format = envFmt
			}
		}
		if format == "" {
			format = "side-by-side"
		}

		normFormat := normalizeFormat(format)
		if format != "" && !slices.Contains([]string{"json", "actions", "inline", "side-by-side"}, normFormat) {
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: side-by-side, inline, json, actions\n", format)
			os.Exit(1)
		}

		ignoreComments, _ := cmd.Flags().GetBool("ignore-comments")
		if !cmd.Flags().Changed("ignore-comments") {
			ignoreComments = getEnvBool("DIFFM_IGNORE_COMMENTS", false)
		}

		parseErrorLimit, _ := cmd.Flags().GetInt("parse-error-limit")
		if !cmd.Flags().Changed("parse-error-limit") {
			parseErrorLimit = getEnvInt("DIFFM_PARSE_ERROR_LIMIT", 0)
		}

		sizeLimitKB, _ := cmd.Flags().GetInt("size-limit")
		if !cmd.Flags().Changed("size-limit") {
			sizeLimitKB = getEnvInt("DIFFM_SIZE_LIMIT", defaultSizeLimitKB)
		}

		lineLimitLines, _ := cmd.Flags().GetInt("line-limit")
		if !cmd.Flags().Changed("line-limit") {
			lineLimitLines = getEnvInt("DIFFM_LINE_LIMIT", defaultLineLimitLines)
		}

		parseTree, _ := cmd.Flags().GetBool("parse-tree")
		isCST, _ := cmd.Flags().GetBool("cst")
		if parseTree || isCST {
			runParseTree(cmd, args, noPager, isCST)
			return
		}

		// Seven args: Git diff.external driver protocol signature
		// Usage: diffm <path> <old-file> <old-hex> <old-mode> <new-file> <new-hex> <new-mode>
		if len(args) == 7 {
			path := args[0]
			oldFile := args[1]
			newFile := args[4]
			if !isFileOrDevNull(oldFile) {
				fmt.Fprintf(os.Stderr, "Error: invalid old-file for external diff driver: %s\n", oldFile)
				os.Exit(1)
			}
			if !isFileOrDevNull(newFile) {
				fmt.Fprintf(os.Stderr, "Error: invalid new-file for external diff driver: %s\n", newFile)
				os.Exit(1)
			}
			if !cmd.Flags().Changed("no-pager") {
				noPager = true
			}
			if normFormat == "" {
				normFormat = "side-by-side"
			}
			runFileDiff(cmd, oldFile, newFile, path, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
			return
		}

		// Two args: diff two files directly, or compare revisions/paths if in a git repo.
		if len(args) == 2 {
			argA, argB := args[0], args[1]
			infoA, errA := os.Stat(argA)
			infoB, errB := os.Stat(argB)

			if (errA == nil && infoA.IsDir()) || (errB == nil && infoB.IsDir()) {
				// Handle directory diffing
				if errA != nil || !infoA.IsDir() || errB != nil || !infoB.IsDir() {
					fmt.Fprintf(os.Stderr, "Error: when comparing directories, both arguments must be directories\n")
					os.Exit(1)
				}
				runDirectoryDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
				return
			}

			// Case 1: Both exist on disk as files or /dev/null
			if isFileOrDevNull(argA) && isFileOrDevNull(argB) {
				runFileDiff(cmd, argA, argB, "", normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
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
					runGitMode(cmd, []string{argA, argB}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
					return
				}

				// Case 3: One revision and one tracked/existing file path (e.g. diffm main internal/git/git.go)
				if isRevA && isTrackedOrFileB {
					runGitMode(cmd, []string{argA, argB}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
					return
				}
				if isRevB && isTrackedOrFileA {
					runGitMode(cmd, []string{argB, argA}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
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

		// In a git repo, launch git mode (optionally filtered by ref or path)
		if git.IsGitRepository(".") {
			runGitMode(cmd, args, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager)
			return
		}

		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: Git mode requires a valid Git repository\n")
			os.Exit(1)
		}

		_ = cmd.Help()
	},
}

func normalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "sbs", "sidebyside", "side-by-side", "tui":
		return "side-by-side"
	case "inline":
		return "inline"
	case "json":
		return "json"
	case "actions":
		return "actions"
	default:
		return f
	}
}

func countLineStats(srcBytes, dstBytes []byte, env *serialize.Envelope) (int, int, int) {
	if env == nil || len(env.LineAlignment) == 0 {
		return 0, 0, 0
	}

	srcLines := strings.Split(string(srcBytes), "\n")
	dstLines := strings.Split(string(dstBytes), "\n")

	srcEndsWithNL := len(srcBytes) > 0 && srcBytes[len(srcBytes)-1] == '\n'
	dstEndsWithNL := len(dstBytes) > 0 && dstBytes[len(dstBytes)-1] == '\n'

	lastSrcLineIdx := len(srcLines) - 1
	if srcEndsWithNL && lastSrcLineIdx >= 0 && srcLines[lastSrcLineIdx] == "" {
		lastSrcLineIdx--
	}
	lastDstLineIdx := len(dstLines) - 1
	if dstEndsWithNL && lastDstLineIdx >= 0 && dstLines[lastDstLineIdx] == "" {
		lastDstLineIdx--
	}

	srcEOFLine := -1
	if srcEndsWithNL && len(srcLines) > 0 && srcLines[len(srcLines)-1] == "" {
		srcEOFLine = len(srcLines) - 1
	}
	dstEOFLine := -1
	if dstEndsWithNL && len(dstLines) > 0 && dstLines[len(dstLines)-1] == "" {
		dstEOFLine = len(dstLines) - 1
	}

	ins, del, upd := 0, 0, 0

	for _, pair := range env.LineAlignment {
		left := pair.LeftLine
		right := pair.RightLine

		if srcEOFLine != -1 && left == srcEOFLine {
			if right == dstEOFLine || right == -1 {
				continue
			}
			left = -1
		}
		if dstEOFLine != -1 && right == dstEOFLine {
			if left == srcEOFLine || left == -1 {
				continue
			}
			right = -1
		}

		if left == -1 && right >= 0 {
			ins++
		} else if left >= 0 && right == -1 {
			del++
		} else if left >= 0 && right >= 0 {
			sText := ""
			if left < len(srcLines) {
				sText = srcLines[left]
			}
			dText := ""
			if right < len(dstLines) {
				dText = dstLines[right]
			}
			isEOFLine := (left == lastSrcLineIdx && right == lastDstLineIdx)
			if sText != dText || (isEOFLine && srcEndsWithNL != dstEndsWithNL) {
				upd++
			}
		}
	}

	return ins, del, upd
}

// renderConfig groups all configuration needed to render a diff result.
type renderConfig struct {
	format     string
	showBanner bool
	writer     io.Writer
	inlineOpts inline.RenderOptions
	sbsOpts    sidebyside.RenderOptions
}

// renderDiffResult renders a single diff result in the specified format.
// It handles all output formats (side-by-side, inline, json, actions) consistently.
func renderDiffResult(srcFile, dstFile string, srcBytes, dstBytes []byte, dr *pipeline.DiffResult, rc renderConfig) error {
	if dr.IsBinary {
		renderBinaryDiff(srcFile, dstFile, rc)
		return nil
	}

	switch rc.format {
	case "side-by-side":
		if rc.showBanner {
			numIns, numDel, numUpd := countLineStats(srcBytes, dstBytes, dr.Envelope)
			if err := sidebyside.RenderFileBanner(dstFile, numIns, numDel, numUpd, rc.sbsOpts.Color, rc.writer); err != nil {
				return err
			}
		}
		err := sidebyside.Render(srcFile, dstFile, srcBytes, dstBytes, dr.Envelope, rc.sbsOpts, rc.writer)
		if err != nil {
			return err
		}
	case "inline":
		output := inline.Render(srcFile, dstFile, srcBytes, dstBytes, dr.Envelope, rc.inlineOpts)
		if output != "" {
			if _, err := io.WriteString(rc.writer, output); err != nil {
				return err
			}
			if !strings.HasSuffix(output, "\n") {
				if _, err := io.WriteString(rc.writer, "\n"); err != nil {
					return err
				}
			}
		}
	case "json":
		var jsonData []byte
		var err error
		if rc.showBanner {
			jsonData, err = json.Marshal(dr.Envelope)
		} else {
			jsonData, err = json.MarshalIndent(dr.Envelope, "", "  ")
		}
		if err == nil {
			if _, err := rc.writer.Write(jsonData); err != nil {
				return err
			}
			if _, err := rc.writer.Write([]byte("\n")); err != nil {
				return err
			}
		}
	case "actions":
		if _, err := fmt.Fprintf(rc.writer, "Diffing  %s  →  %s\n\n", srcFile, dstFile); err != nil {
			return err
		}
		if err := engine.FprintMappings(rc.writer, dr.MatchResult); err != nil {
			return err
		}
		if err := actions.FprintActions(rc.writer, dr.EditScript); err != nil {
			return err
		}
	}

	return nil
}

// renderBinaryDiff outputs a message indicating that files are binary and differ.
func renderBinaryDiff(srcFile, dstFile string, rc renderConfig) {
	switch rc.format {
	case "side-by-side":
		if rc.showBanner {
			_ = sidebyside.RenderFileBanner(dstFile, 0, 0, 0, rc.sbsOpts.Color, rc.writer)
		}
		_, _ = fmt.Fprintf(rc.writer, "Binary files %s and %s differ\n", srcFile, dstFile)
	case "inline":
		_, _ = fmt.Fprintf(rc.writer, "Binary files %s and %s differ\n", srcFile, dstFile)
	case "json":
		env := &serialize.Envelope{
			Version:  serialize.SchemaVersion,
			IsBinary: true,
		}
		var jsonData []byte
		var err error
		if rc.showBanner {
			jsonData, err = json.Marshal(env)
		} else {
			jsonData, err = json.MarshalIndent(env, "", "  ")
		}
		if err == nil {
			_, _ = rc.writer.Write(jsonData)
			_, _ = rc.writer.Write([]byte("\n"))
		}
	case "actions":
		_, _ = fmt.Fprintf(rc.writer, "Binary files %s and %s differ\n", srcFile, dstFile)
	}
}

func runGitMode(cmd *cobra.Command, args []string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) {
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
		format = "side-by-side"
	}

	files, err := git.GetChangedFiles(".", refA, refB, pathFilter)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: retrieving git status: %v\n", err)
		os.Exit(1)
	}

	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	includeUI := format == "inline" || format == "side-by-side" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "inline" || format == "side-by-side" || (!uiMode) || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	inlineOpts := resolveRenderOptions(cmd)
	sbsOpts := resolveSideBySideOptions(cmd)

	var p *pager.Pager
	var writer io.Writer = os.Stdout

	if format == "inline" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
		defer p.Close()
	}

	showBanner := len(files) > 1
	var filesRendered int

	textconv, _ := cmd.Flags().GetBool("textconv")

	processFile := func(srcFile, dstFile string, srcBytes, dstBytes []byte) error {
		if bytes.Equal(srcBytes, dstBytes) && format != "json" {
			return nil
		}

		dr, err := pipeline.Run(srcBytes, dstBytes, srcFile, dstFile, pipeline.DiffOptions{
			ParseErrorLimit:  parseErrorLimit,
			IgnoreComments:   ignoreComments,
			DisableSizeLimit: sizeLimitKB <= 0,
			MaxASTFileSize:   sizeLimitKB * 1024,
			DisableLineLimit: lineLimitLines <= 0,
			MaxASTFileLines:  lineLimitLines,
			EnvelopeOpts:     opts,
		})
		if err != nil {
			return fmt.Errorf("diffing %s: %w", dstFile, err)
		}

		filesRendered++
		return renderDiffResult(srcFile, dstFile, srcBytes, dstBytes, dr, renderConfig{
			format:     format,
			showBanner: showBanner,
			writer:     writer,
			inlineOpts: inlineOpts,
			sbsOpts:    sbsOpts,
		})
	}

	if refA == "" {
		for _, f := range files {
			if p != nil && !p.IsActive() {
				return
			}

			if stagedOnly && !f.Staged {
				continue
			}

			srcFile := f.Path
			if f.OldPath != "" {
				srcFile = f.OldPath
			}
			dstFile := f.Path

			if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
				continue
			}
			if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
				continue
			}

			if f.IsBinary && !textconv {
				filesRendered++
				renderBinaryDiff(srcFile, dstFile, renderConfig{
					format:     format,
					showBanner: showBanner,
					writer:     writer,
					inlineOpts: inlineOpts,
					sbsOpts:    sbsOpts,
				})
				continue
			}

			if f.Staged {
				srcBytes, err := git.GetContent(".", srcFile, "HEAD", textconv)
				if err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", srcFile, err)
					}
					return
				}
				dstBytes, err := git.GetContent(".", dstFile, ":", textconv)
				if err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", dstFile, err)
					}
					return
				}
				if err := processFile(srcFile, dstFile, srcBytes, dstBytes); err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					}
					return
				}
			}

			if f.Unstaged && !stagedOnly {
				revA := ":"
				if !f.Staged {
					revA = "HEAD"
				}
				srcBytes, err := git.GetContent(".", srcFile, revA, textconv)
				if err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", srcFile, err)
					}
					return
				}
				dstBytes, err := git.GetContent(".", dstFile, "", textconv)
				if err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", dstFile, err)
					}
					return
				}
				if err := processFile(srcFile, dstFile, srcBytes, dstBytes); err != nil {
					if !pager.IsBrokenPipe(err) {
						fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					}
					return
				}
			}
		}
	} else {
		for _, f := range files {
			if p != nil && !p.IsActive() {
				return
			}

			srcFile := f.Path
			if f.OldPath != "" {
				srcFile = f.OldPath
			}
			dstFile := f.Path

			if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
				continue
			}
			if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
				continue
			}

			if f.IsBinary && !textconv {
				filesRendered++
				renderBinaryDiff(srcFile, dstFile, renderConfig{
					format:     format,
					showBanner: showBanner,
					writer:     writer,
					inlineOpts: inlineOpts,
					sbsOpts:    sbsOpts,
				})
				continue
			}

			srcBytes, err := git.GetContent(".", srcFile, refA, textconv)
			if err != nil {
				if !pager.IsBrokenPipe(err) {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", srcFile, err)
				}
				return
			}
			dstBytes, err := git.GetContent(".", dstFile, refB, textconv)
			if err != nil {
				if !pager.IsBrokenPipe(err) {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", dstFile, err)
				}
				return
			}

			if err := processFile(srcFile, dstFile, srcBytes, dstBytes); err != nil {
				if !pager.IsBrokenPipe(err) {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				}
				return
			}
		}
	}

	if filesRendered == 0 && pathFilter != "" && (isFileOrDevNull(pathFilter) || git.IsTrackedFile(".", pathFilter)) {
		var srcBytes, dstBytes []byte
		var err error
		if refA == "" {
			if stagedOnly {
				srcBytes, err = git.GetContent(".", pathFilter, "HEAD", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return
				}
				dstBytes, err = git.GetContent(".", pathFilter, ":", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return
				}
			} else {
				srcBytes, err = git.GetContent(".", pathFilter, ":", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return
				}
				if len(srcBytes) == 0 {
					srcBytes, err = git.GetContent(".", pathFilter, "HEAD", textconv)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
						return
					}
				}
				dstBytes, err = git.GetContent(".", pathFilter, "", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return
				}
			}
		} else {
			srcBytes, err = git.GetContent(".", pathFilter, refA, textconv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
				return
			}
			dstBytes, err = git.GetContent(".", pathFilter, refB, textconv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
				return
			}
		}

		if err := processFile(pathFilter, pathFilter, srcBytes, dstBytes); err != nil {
			if !pager.IsBrokenPipe(err) {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			}
			return
		}
	}
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

	tabWidth := resolveTabWidth(cmd)

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

func resolveTabWidth(cmd *cobra.Command) int {
	tabWidth, _ := cmd.Flags().GetInt("tab-width")
	if !cmd.Flags().Changed("tab-width") {
		tabWidth = getEnvInt("DIFFM_TAB_WIDTH", defaultTabWidth)
	}
	if tabWidth <= 0 {
		tabWidth = defaultTabWidth
	}
	return tabWidth
}

func resolveSideBySideOptions(cmd *cobra.Command) sidebyside.RenderOptions {
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

	forceSBSFlag, _ := cmd.Flags().GetBool("force-sbs")

	tabWidth := resolveTabWidth(cmd)

	return sidebyside.RenderOptions{
		Color:              useColor,
		ContextLines:       contextLines,
		LineNumbers:        lineNumbers,
		DisableAnnotations: !annotations,
		TabWidth:           tabWidth,
		ForceSideBySide:    forceSBSFlag,
	}
}

func isFileOrDevNull(path string) bool {
	if path == os.DevNull || path == "/dev/null" {
		return true
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// Cobra stops parsing flags at '--' and treats everything after as a positional
// arg. Catch flags accidentally put after '--', unless they're real files starting with '-'.
func validateNoMisplacedFlags(args []string) error {
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && arg != "-" && !isFileOrDevNull(arg) && !git.IsTrackedFile(".", arg) {
			return fmt.Errorf("flag %q cannot be placed after '--'\n\n"+
				"In CLI syntax, '--' marks the end of options; all subsequent arguments are treated as paths.\n"+
				"Place flags before '--' or omit '--':\n"+
				"  diffm [flags] [refs...] [--] [paths...]\n"+
				"  diffm [refs...] [paths...] [flags]", arg)
		}
	}
	return nil
}

func runFileDiff(cmd *cobra.Command, fileA, fileB, displayPath string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) {
	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	displayA, displayB := fileA, fileB
	if displayPath != "" {
		displayA, displayB = displayPath, displayPath
	}

	if format == "" {
		format = "side-by-side"
	}

	includeUI := format == "side-by-side" || format == "inline" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "inline" || format == "side-by-side" || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	var srcBytes, dstBytes []byte
	var err error

	if fileA == os.DevNull || fileA == "/dev/null" {
		srcBytes = []byte{}
	} else {
		srcBytes, err = os.ReadFile(fileA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", fileA, err)
			os.Exit(1)
		}
	}

	if fileB == os.DevNull || fileB == "/dev/null" {
		dstBytes = []byte{}
	} else {
		dstBytes, err = os.ReadFile(fileB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", fileB, err)
			os.Exit(1)
		}
	}

	dr, err := pipeline.Run(srcBytes, dstBytes, displayA, displayB, pipeline.DiffOptions{
		ParseErrorLimit:  parseErrorLimit,
		IgnoreComments:   ignoreComments,
		DisableSizeLimit: sizeLimitKB <= 0,
		MaxASTFileSize:   sizeLimitKB * 1024,
		DisableLineLimit: lineLimitLines <= 0,
		MaxASTFileLines:  lineLimitLines,
		EnvelopeOpts:     opts,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	switch format {
	case "side-by-side":
		p, writer := pager.Start(noPager)
		defer p.Close()

		renderOpts := resolveSideBySideOptions(cmd)
		err := sidebyside.Render(displayA, displayB, dr.SrcBytes, dr.DstBytes, dr.Envelope, renderOpts, writer)
		if err != nil && pager.IsBrokenPipe(err) {
			return
		}
	case "inline":
		p, writer := pager.Start(noPager)
		defer p.Close()

		renderOpts := resolveRenderOptions(cmd)
		output := inline.Render(displayA, displayB, dr.SrcBytes, dr.DstBytes, dr.Envelope, renderOpts)
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
		if dr.IsBinary {
			_, _ = fmt.Fprintf(writer, "Binary files %s and %s differ\n", displayA, displayB)
			return
		}
		_, _ = fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", displayA, displayB)
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

	if err := dumpFiles(writer, args, isCST); err != nil {
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
				srcBytes, err = git.GetContent(".", path, "", false)
			}
			if err != nil {
				return fmt.Errorf("reading %s: %w", path, err)
			}
		}

		if multiple {
			if i > 0 {
				if _, werr := fmt.Fprintln(w); werr != nil {
					return werr
				}
			}
			if _, werr := fmt.Fprintf(w, "=== %s ===\n", path); werr != nil {
				return werr
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

// RootCmd returns the root Cobra command for documentation generation and inspection.
func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.Flags().StringP("format", "f", "", "Output format: side-by-side, inline, json, actions (default: side-by-side)")
	rootCmd.Flags().BoolP("ignore-comments", "C", false, "Ignore all comments when diffing")
	rootCmd.Flags().IntP("parse-error-limit", "e", 0, "Maximum parse errors allowed before falling back to line diffing")
	rootCmd.Flags().Int("size-limit", 1024, "Maximum file size in KB for AST parsing before falling back to line diff (0 to disable limit)")
	rootCmd.Flags().Int("line-limit", 10000, "Maximum line count for AST parsing before falling back to line diff (0 to disable limit)")
	rootCmd.Flags().Bool("ui", false, "Include line alignment and highlight spans in JSON output")
	rootCmd.Flags().Bool("full", false, "Include actions, line alignment, and highlight spans in JSON output")
	rootCmd.Flags().Bool("cached", false, "Show only staged changes in Git mode")
	rootCmd.Flags().String("color", "auto", "Color output: always, never, auto")
	rootCmd.Flags().IntP("context", "U", 3, "Lines of context to show for diff")
	rootCmd.Flags().BoolP("line-numbers", "n", true, "Show line numbers in diff output")
	rootCmd.Flags().Bool("annotations", true, "Include AST move annotations in diff")
	rootCmd.Flags().BoolP("patch", "p", false, "Generate a standard patch suitable for git apply / patch tools")
	rootCmd.Flags().Bool("force-sbs", false, "Force strict 50/50 side-by-side rendering without adaptive hybrid inline switching")
	rootCmd.Flags().Bool("no-pager", false, "Do not pipe output into a pager")
	rootCmd.Flags().Bool("wrap", false, "Wrap long lines to terminal width in inline diff")
	rootCmd.Flags().Int("wrap-width", 0, "Explicit column width for line wrapping (0 to auto-detect terminal width)")
	rootCmd.Flags().Int("tab-width", 4, "Number of spaces per tab stop in diff output")
	rootCmd.Flags().Bool("parse-tree", false, "Parse files and dump the syntax tree for debugging")
	rootCmd.Flags().Bool("cst", false, "Dump the raw Tree-sitter concrete syntax tree instead of the Diffmantic AST")
	rootCmd.Flags().Bool("textconv", false, "Allow external text conversion filters to be run when comparing binary files")

	_ = rootCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"side-by-side", "inline", "json", "actions"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("color", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"always", "never", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})
}
