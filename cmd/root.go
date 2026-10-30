// Package cmd implements the CLI commands for diffm.
package cmd

import (
	"bytes"
	"cmp"
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
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/HarshK97/diffmantic/internal/streamdiff"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/unified"
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

func isPipedInput(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode()&os.ModeNamedPipe) != 0 || fi.Mode().IsRegular()
}

var rootCmd = &cobra.Command{
	Use:     "diffm [refA] [refB]",
	Version: "0.14.0",
	Short:   "Semantic diff engine powered by Tree-sitter",
	Long: `diffmantic is a structural source code diff engine.

It parses source files into ASTs using Tree-sitter and computes semantic
differences. It detects not just what lines changed, but what code structures
were inserted, deleted, updated, moved, or renamed.

Works as a standalone file diff tool, a git difftool, or a backend engine for
editor plugins (Neovim, VS Code) via JSON output.`,
	Example: `  diffm before.go after.go                     Side-by-side diff with pager (default in TTY)
  diffm dir_a dir_b                            Recursive directory diff
  diffm -u before.go after.go                  AST-aware unified diff
  git diff | diffm                             Stream Git diff with AST highlighting
  git log -p | diffm                           Stream Git commit log with AST highlighting
  diffm before.go after.go -f json             JSON output for editor plugins
  diffm before.go after.go -f actions          Print structural actions list
  diffm                                        Git mode on unstaged changes
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

		if len(args) == 1 && args[0] == "help" && !isFileOrDevNull("help") {
			if helpFunc := cmd.HelpFunc(); helpFunc != nil {
				helpFunc(cmd, args)
			}
			return
		}

		noPager, _ := cmd.Flags().GetBool("no-pager")
		patchMode, _ := cmd.Flags().GetBool("patch")
		if patchMode && !cmd.Flags().Changed("no-pager") {
			noPager = true
		}

		normFormat := resolveFormat(cmd)
		if !slices.Contains(validFormats, normFormat) {
			rawFormat, _ := cmd.Flags().GetString("format")
			rawFormat = cmp.Or(rawFormat, getEnvString("DIFFM_FORMAT", ""))
			fmt.Fprintf(os.Stderr, "Error: Unsupported output format %q. Supported formats: side-by-side, unified, json, actions, auto\n", rawFormat)
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
			if runParseTree(cmd, args, noPager, isCST) {
				os.Exit(1)
			}
			return
		}

		// Git diff.external driver protocol signature (7, 8, or 9 arguments)
		// Usage: diffm <path> <old-file> <old-hex> <old-mode> <new-file> <new-hex> <new-mode> [other-path] [xfrm-msg]
		if len(args) >= 7 && len(args) <= 9 {
			pathA := args[0]
			oldFile := args[1]
			oldHex := args[2]
			newFile := args[4]
			newHex := args[5]

			pathB := pathA
			if len(args) >= 8 && args[7] != "" {
				pathB = args[7]
			}

			normFormat = cmp.Or(normFormat, "side-by-side")

			// Skip diffing when Git reports identical hashes (e.g. chmod without content changes),
			// unless JSON output was requested.
			if oldHex != "" && oldHex != "." && !git.IsNullOID(oldHex) && oldHex == newHex && normFormat != "json" {
				return
			}

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
			if runFileDiff(cmd, oldFile, newFile, pathA, pathB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
				os.Exit(1)
			}
			return
		}

		// Two args: diff two files directly, or compare revisions/paths if in a git repo.
		if len(args) == 2 {
			argA, argB := args[0], args[1]
			infoA, errA := os.Stat(argA)
			infoB, errB := os.Stat(argB)

			if (errA == nil && infoA.IsDir()) || (errB == nil && infoB.IsDir()) {
				if errA != nil || !infoA.IsDir() || errB != nil || !infoB.IsDir() {
					fmt.Fprintf(os.Stderr, "Error: when comparing directories, both arguments must be directories\n")
					os.Exit(1)
				}
				if runDirectoryDiff(cmd, argA, argB, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
					os.Exit(1)
				}
				return
			}

			// Case 1: Both exist on disk as files or /dev/null
			if isFileOrDevNull(argA) && isFileOrDevNull(argB) {
				if runFileDiff(cmd, argA, argB, "", "", normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
					os.Exit(1)
				}
				return
			}

			// Git repository inspection
			if git.IsGitRepository(".") {
				isRevA := git.IsValidRevision(".", argA)
				isRevB := git.IsValidRevision(".", argB)

				// Case 2: Two Git revisions (e.g. diffm main feature-branch)
				if isRevA && isRevB {
					if runGitMode(cmd, []string{argA, argB}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
						os.Exit(1)
					}
					return
				}
				isTrackedOrFileA := isFileOrDevNull(argA) || git.IsTrackedFile(".", argA)
				isTrackedOrFileB := isFileOrDevNull(argB) || git.IsTrackedFile(".", argB)

				// Case 3: One revision and one tracked/existing file path (e.g. diffm main internal/git/git.go)
				if isRevA && isTrackedOrFileB {
					if runGitMode(cmd, []string{argA, argB}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
						os.Exit(1)
					}
					return
				}
				if isRevB && isTrackedOrFileA {
					if runGitMode(cmd, []string{argB, argA}, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
						os.Exit(1)
					}
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

		// Stream mode kicks in when diffm is piped (git diff | diffm) or invoked as a Git pager.
		pagerFlag, _ := cmd.Flags().GetBool("pager")
		if len(args) == 0 && (isPipedInput(os.Stdin) || pagerFlag) {
			if runStreamMode(cmd, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
				os.Exit(1)
			}
			return
		}
		// In a git repo, launch git mode (optionally filtered by ref or path)
		if git.IsGitRepository(".") {
			if runGitMode(cmd, args, normFormat, ignoreComments, parseErrorLimit, sizeLimitKB, lineLimitLines, noPager) {
				os.Exit(1)
			}
			return
		}

		if len(args) > 0 {
			fmt.Fprintf(os.Stderr, "Error: Git mode requires a valid Git repository\n")
			os.Exit(1)
		}

		_ = cmd.Help()
	},
}

var (
	validFormats  = []string{"side-by-side", "unified", "json", "actions"}
	termFdWidthFn = defaultTerminalFdWidth
	isTerminalFn  = func() bool { return isTerminal(os.Stdout) }
)

func normalizeFormat(f string) string {
	switch strings.ToLower(strings.TrimSpace(f)) {
	case "sbs", "sidebyside", "side-by-side", "tui":
		return "side-by-side"
	case "unified", "uni":
		return "unified"
	case "auto":
		return "auto"
	case "json":
		return "json"
	case "actions":
		return "actions"
	default:
		return f
	}
}

func defaultTerminalFdWidth() int {
	if wFd, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil && wFd > 0 {
		return wFd
	}
	return 0
}

func terminalWidth() int {
	if w := termFdWidthFn(); w > 0 {
		return w
	}
	if colStr := os.Getenv("COLUMNS"); colStr != "" {
		if cols, err := strconv.Atoi(colStr); err == nil && cols > 0 {
			return cols
		}
	}
	return 0
}

func resolveFormat(cmd *cobra.Command) string {
	if cmd.Flags().Changed("format") {
		f, _ := cmd.Flags().GetString("format")
		if f != "" {
			norm := normalizeFormat(f)
			if norm != "auto" {
				return norm
			}
		}
	} else if patchMode, _ := cmd.Flags().GetBool("patch"); patchMode {
		return "unified"
	} else if unifiedFlag, _ := cmd.Flags().GetBool("unified"); unifiedFlag {
		return "unified"
	} else if envFmt := strings.TrimSpace(os.Getenv("DIFFM_FORMAT")); envFmt != "" {
		norm := normalizeFormat(envFmt)
		if norm != "auto" {
			return norm
		}
	}

	// Side-by-side layout needs roughly 60 columns per pane for line numbers, gutters,
	// and code before text wrapping degrades readability. Fall back to unified on narrow viewports.
	if w := terminalWidth(); w > 0 {
		if w < 120 {
			return "unified"
		}
		return "side-by-side"
	}

	if isTerminalFn() {
		return "unified"
	}

	return "side-by-side"
}

// renderConfig groups settings needed to render a diff.
type renderConfig struct {
	format      string
	showBanner  bool
	writer      io.Writer
	unifiedOpts unified.RenderOptions
	sbsOpts     sidebyside.RenderOptions
}

// renderDiffResult writes a single diff result to writer in the requested format.
func renderDiffResult(srcFile, dstFile string, srcBytes, dstBytes []byte, dr *pipeline.DiffResult, rc renderConfig) error {
	if dr.IsBinary {
		renderBinaryDiff(srcFile, dstFile, rc)
		return nil
	}

	switch rc.format {
	case "side-by-side":
		if rc.showBanner {
			numIns, numDel, numUpd := sidebyside.CountLineStats(srcBytes, dstBytes, dr.Envelope)
			bannerPath := dstFile
			if bannerPath == "/dev/null" || bannerPath == "" {
				bannerPath = srcFile
			}
			bannerAnnotation := ""
			if !rc.sbsOpts.DisableAnnotations && dr.Envelope != nil {
				bannerAnnotation = dr.Envelope.FallbackAnnotation
			}
			if err := sidebyside.RenderFileBanner(bannerPath, numIns, numDel, numUpd, bannerAnnotation, rc.sbsOpts.Color, rc.writer); err != nil {
				return err
			}
		}
		sbsOpts := rc.sbsOpts
		if rc.showBanner {
			sbsOpts.SuppressHunkFallbackAnnotation = true
		}
		err := sidebyside.Render(srcFile, dstFile, srcBytes, dstBytes, dr.Envelope, sbsOpts, rc.writer)
		if err != nil {
			return err
		}
	case "unified":
		output := unified.Render(srcFile, dstFile, srcBytes, dstBytes, dr.Envelope, rc.unifiedOpts)
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

// renderBinaryDiff prints the "Binary files ... differ" notice for the active format.
func renderBinaryDiff(srcFile, dstFile string, rc renderConfig) {
	switch rc.format {
	case "side-by-side":
		if rc.showBanner {
			_ = sidebyside.RenderFileBanner(dstFile, 0, 0, 0, "", rc.sbsOpts.Color, rc.writer)
		}
		_, _ = fmt.Fprintf(rc.writer, "Binary files %s and %s differ\n", srcFile, dstFile)
	case "unified":
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

// runGitMode diffs Git revisions or working tree changes, returning true if any file had errors.
func runGitMode(cmd *cobra.Command, args []string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) (hadErrors bool) {
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
		return true
	}

	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	includeUI := format == "unified" || format == "side-by-side" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "unified" || format == "side-by-side" || (!uiMode) || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	unifiedOpts := resolveUnifiedOptions(cmd)
	sbsOpts := resolveSideBySideOptions(cmd)

	var p *pager.Pager
	var writer io.Writer = os.Stdout

	if format == "unified" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
		// Pager.Close is idempotent; this catches early error exits before we reach runConcurrentDiffJobs.
		defer p.Close()
	}

	showBanner := len(files) > 1
	var filesRendered int

	textconv, _ := cmd.Flags().GetBool("textconv")

	rc := renderConfig{
		format:      format,
		showBanner:  showBanner,
		writer:      writer,
		unifiedOpts: unifiedOpts,
		sbsOpts:     sbsOpts,
	}

	diffOpts := pipeline.DiffOptions{
		ParseErrorLimit:  parseErrorLimit,
		IgnoreComments:   ignoreComments,
		DisableSizeLimit: sizeLimitKB <= 0,
		MaxASTFileSize:   sizeLimitKB * 1024,
		DisableLineLimit: lineLimitLines <= 0,
		MaxASTFileLines:  lineLimitLines,
		EnvelopeOpts:     opts,
	}

	var batchMap map[git.BlobRequest][]byte
	if !textconv {
		var reqs []git.BlobRequest
		if refA == "" {
			for _, f := range files {
				if stagedOnly && !f.Staged {
					continue
				}
				if f.IsBinary {
					continue
				}
				srcFile := cmp.Or(f.OldPath, f.Path)
				dstFile := f.Path

				if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
					continue
				}
				if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
					continue
				}

				if f.Staged {
					reqs = append(reqs, git.BlobRequest{Path: srcFile, Revision: "HEAD"})
					reqs = append(reqs, git.BlobRequest{Path: dstFile, Revision: ":"})
				}
				if f.Unstaged && !stagedOnly {
					revA := ":"
					if !f.Staged {
						revA = "HEAD"
					}
					reqs = append(reqs, git.BlobRequest{Path: srcFile, Revision: revA})
					reqs = append(reqs, git.BlobRequest{Path: dstFile, Revision: ""})
				}
			}
		} else {
			for _, f := range files {
				if f.IsBinary {
					continue
				}
				srcFile := cmp.Or(f.OldPath, f.Path)
				dstFile := f.Path

				if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
					continue
				}
				if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
					continue
				}

				reqs = append(reqs, git.BlobRequest{Path: srcFile, Revision: refA})
				reqs = append(reqs, git.BlobRequest{Path: dstFile, Revision: refB})
			}
		}
		if len(reqs) > 0 {
			// On batch failure, batchMap remains nil and getContent falls back to single-file queries.
			batchMap, _ = git.GetBatchContent(".", reqs)
		}
	}

	// Give each job its prefetched blobs directly so we can drop the batch map
	// before the worker pool starts. That lets us free blob buffers file-by-file
	// as diffs finish, instead of holding everything in memory for the whole run.
	// Anything not in the batch is loaded on demand by the worker.
	newContentJob := func(srcFile, dstFile, srcRev, dstRev string) diffJob {
		job := diffJob{
			displayA: srcFile,
			displayB: dstFile,
			relPathA: srcFile,
			relPathB: dstFile,
		}
		srcData, srcOK := batchMap[git.BlobRequest{Path: srcFile, Revision: srcRev}]
		dstData, dstOK := batchMap[git.BlobRequest{Path: dstFile, Revision: dstRev}]
		if srcOK && dstOK {
			job.srcBytes, job.dstBytes = srcData, dstData
			return job
		}
		job.load = func() ([]byte, []byte, error) {
			var err error
			if !srcOK {
				if srcData, err = git.GetContent(".", srcFile, srcRev, textconv); err != nil {
					return nil, nil, fmt.Errorf("reading %s: %w", srcFile, err)
				}
			}
			if !dstOK {
				if dstData, err = git.GetContent(".", dstFile, dstRev, textconv); err != nil {
					return nil, nil, fmt.Errorf("reading %s: %w", dstFile, err)
				}
			}
			return srcData, dstData, nil
		}
		return job
	}

	var jobs []diffJob
	if refA == "" {
		for _, f := range files {
			if stagedOnly && !f.Staged {
				continue
			}

			srcFile := cmp.Or(f.OldPath, f.Path)
			dstFile := f.Path

			if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
				continue
			}
			if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
				continue
			}

			if f.IsBinary && !textconv {
				jobs = append(jobs, diffJob{
					displayA: srcFile,
					displayB: dstFile,
					isBinary: true,
				})
				continue
			}

			if f.Staged {
				jobs = append(jobs, newContentJob(srcFile, dstFile, "HEAD", ":"))
			}

			if f.Unstaged && !stagedOnly {
				revA := ":"
				if !f.Staged {
					revA = "HEAD"
				}
				jobs = append(jobs, newContentJob(srcFile, dstFile, revA, ""))
			}
		}
	} else {
		for _, f := range files {
			srcFile := cmp.Or(f.OldPath, f.Path)
			dstFile := f.Path

			if strings.HasSuffix(dstFile, "/") || strings.HasSuffix(srcFile, "/") {
				continue
			}
			if fi, err := os.Stat(dstFile); err == nil && fi.IsDir() {
				continue
			}

			if f.IsBinary && !textconv {
				jobs = append(jobs, diffJob{
					displayA: srcFile,
					displayB: dstFile,
					isBinary: true,
				})
				continue
			}

			jobs = append(jobs, newContentJob(srcFile, dstFile, refA, refB))
		}
	}
	batchMap = nil

	diffErrors, count := runConcurrentDiffJobs(jobs, p, rc, diffOpts)
	if diffErrors {
		hadErrors = true
	}
	filesRendered += count

	if filesRendered == 0 && pathFilter != "" && (isFileOrDevNull(pathFilter) || git.IsTrackedFile(".", pathFilter)) {
		var srcBytes, dstBytes []byte
		var err error
		if refA == "" {
			if stagedOnly {
				srcBytes, err = git.GetContent(".", pathFilter, "HEAD", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return true
				}
				dstBytes, err = git.GetContent(".", pathFilter, ":", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return true
				}
			} else {
				srcBytes, err = git.GetContent(".", pathFilter, ":", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return true
				}
				if len(srcBytes) == 0 {
					srcBytes, err = git.GetContent(".", pathFilter, "HEAD", textconv)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
						return true
					}
				}
				dstBytes, err = git.GetContent(".", pathFilter, "", textconv)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
					return true
				}
			}
		} else {
			srcBytes, err = git.GetContent(".", pathFilter, refA, textconv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
				return true
			}
			dstBytes, err = git.GetContent(".", pathFilter, refB, textconv)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathFilter, err)
				return true
			}
		}

		fallbackJob := []diffJob{{
			displayA: pathFilter,
			displayB: pathFilter,
			relPathA: pathFilter,
			relPathB: pathFilter,
			srcBytes: srcBytes,
			dstBytes: dstBytes,
		}}
		fallbackErrors, _ := runConcurrentDiffJobs(fallbackJob, p, rc, diffOpts)
		if fallbackErrors {
			hadErrors = true
		}
	}

	return hadErrors
}

func resolveUnifiedOptions(cmd *cobra.Command) unified.RenderOptions {
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
		termWidth = terminalWidth()
		if termWidth <= 0 && isTerminal(os.Stdout) {
			termWidth = 80
		}
	}

	if termWidth <= 0 {
		wrapFlag = false
	}

	tabWidth := resolveTabWidth(cmd)

	return unified.RenderOptions{
		Color:              useColor,
		ContextLines:       contextLines,
		LineNumbers:        lineNumbers,
		DisableAnnotations: !annotations,
		Wrap:               wrapFlag,
		TerminalWidth:      termWidth,
		TabWidth:           tabWidth,
		GitDiff:            patchMode,
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
		TerminalWidth:      terminalWidth(),
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

// runFileDiff runs the diff pipeline for two files, supporting separate display paths for git external diff.
// Returns true if any errors occurred during processing.
func runFileDiff(cmd *cobra.Command, fileA, fileB, displayPathA, displayPathB string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) bool {
	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	displayA, displayB := fileA, fileB
	if fileA == os.DevNull || fileA == "/dev/null" {
		displayA = "/dev/null"
	} else if displayPathA != "" {
		displayA = displayPathA
	}

	if fileB == os.DevNull || fileB == "/dev/null" {
		displayB = "/dev/null"
	} else if displayPathB != "" {
		displayB = displayPathB
	}

	format = cmp.Or(format, "side-by-side")

	includeUI := format == "side-by-side" || format == "unified" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "unified" || format == "side-by-side" || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	var srcBytes, dstBytes []byte
	var err error

	if fileA == fileB && format != "json" {
		return false
	}

	if fileA == os.DevNull || fileA == "/dev/null" {
		srcBytes = []byte{}
	} else {
		srcBytes, err = os.ReadFile(fileA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", fileA, err)
			return true
		}
	}

	if fileB == os.DevNull || fileB == "/dev/null" {
		dstBytes = []byte{}
	} else {
		dstBytes, err = os.ReadFile(fileB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", fileB, err)
			return true
		}
	}

	if bytes.Equal(srcBytes, dstBytes) && format != "json" {
		return false
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
		return true
	}

	switch format {
	case "side-by-side":
		p, writer := pager.Start(noPager)
		defer p.Close()

		renderOpts := resolveSideBySideOptions(cmd)
		err := sidebyside.Render(displayA, displayB, dr.SrcBytes, dr.DstBytes, dr.Envelope, renderOpts, writer)
		if err != nil && pager.IsBrokenPipe(err) {
			return false
		}
	case "unified":
		p, writer := pager.Start(noPager)
		defer p.Close()

		renderOpts := resolveUnifiedOptions(cmd)
		output := unified.Render(displayA, displayB, dr.SrcBytes, dr.DstBytes, dr.Envelope, renderOpts)
		if output != "" {
			if _, err := io.WriteString(writer, output); err != nil {
				if pager.IsBrokenPipe(err) {
					return false
				}
				fmt.Fprintf(os.Stderr, "Error: writing diff output: %v\n", err)
				return true
			}
			if !strings.HasSuffix(output, "\n") {
				if _, err := io.WriteString(writer, "\n"); err != nil {
					if pager.IsBrokenPipe(err) {
						return false
					}
					fmt.Fprintf(os.Stderr, "Error: writing newline: %v\n", err)
					return true
				}
			}
		}
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: serializing JSON: %v\n", err)
			return true
		}
		_, _ = os.Stdout.Write(jsonData)
		_, _ = os.Stdout.Write([]byte("\n"))
	case "actions":
		p, writer := pager.Start(noPager)
		defer p.Close()
		if dr.IsBinary {
			_, _ = fmt.Fprintf(writer, "Binary files %s and %s differ\n", displayA, displayB)
			return false
		}
		_, _ = fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", displayA, displayB)
		_ = engine.FprintMappings(writer, dr.MatchResult)
		_ = actions.FprintActions(writer, dr.EditScript)
	}
	return false
}

func runStreamMode(
	cmd *cobra.Command,
	format string,
	ignoreComments bool,
	parseErrorLimit int,
	sizeLimitKB int,
	lineLimitLines int,
	noPager bool,
) bool {
	format = cmp.Or(format, "side-by-side")

	uiMode, _ := cmd.Flags().GetBool("ui")
	fullMode, _ := cmd.Flags().GetBool("full")

	includeUI := format == "unified" || format == "side-by-side" || uiMode || fullMode
	opts := serialize.EnvelopeOptions{
		IncludeActions:    format == "unified" || format == "side-by-side" || (!uiMode) || fullMode,
		IncludeAlignment:  includeUI,
		IncludeHighlights: includeUI,
	}

	diffOpts := pipeline.DiffOptions{
		ParseErrorLimit:  parseErrorLimit,
		IgnoreComments:   ignoreComments,
		DisableSizeLimit: sizeLimitKB <= 0,
		MaxASTFileSize:   sizeLimitKB * 1024,
		DisableLineLimit: lineLimitLines <= 0,
		MaxASTFileLines:  lineLimitLines,
		EnvelopeOpts:     opts,
	}

	unifiedOpts := resolveUnifiedOptions(cmd)
	sbsOpts := resolveSideBySideOptions(cmd)

	var p *pager.Pager
	var writer io.Writer = os.Stdout

	if (format == "unified" || format == "side-by-side") && isTerminal(os.Stdout) && !noPager {
		p, writer = pager.Start(noPager)
	}
	defer p.Close()

	cfg := streamdiff.StreamConfig{
		Format:         format,
		DiffOpts:       diffOpts,
		UnifiedOpts:    unifiedOpts,
		SideBySideOpts: sbsOpts,
		IsActive: func() bool {
			return p == nil || p.IsActive()
		},
	}

	repoRoot := cmp.Or(git.FindRepoRoot("."), ".")

	if err := streamdiff.Process(os.Stdin, writer, repoRoot, cfg); err != nil && !pager.IsBrokenPipe(err) {
		fmt.Fprintf(os.Stderr, "Error: processing diff stream: %v\n", err)
		return true
	}
	return false
}

func runParseTree(cmd *cobra.Command, args []string, noPager bool, isCST bool) bool {
	if len(args) == 0 {
		flagName := "--parse-tree"
		if isCST {
			flagName = "--cst"
		}
		fmt.Fprintf(os.Stderr, "Error: %s requires at least one file argument\n", flagName)
		return true
	}

	p, writer := pager.Start(noPager)
	if p != nil {
		defer p.Close()
	}

	if err := dumpFiles(writer, args, isCST); err != nil {
		if !pager.IsBrokenPipe(err) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return true
		}
	}
	return false
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
			flatNodes, symbols, err := treesitter.ParseCST(srcBytes, lang.Name)
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
	rootCmd.Flags().StringP("format", "f", "", "Output format: side-by-side, unified (or -u), json, actions, auto (default: auto, side-by-side if width >= 120)")
	rootCmd.Flags().BoolP("unified", "u", false, "Generate unified diff output (equivalent to -f unified)")
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
	rootCmd.Flags().Bool("force-sbs", false, "Force strict 50/50 side-by-side rendering without adaptive hybrid unified switching")
	rootCmd.Flags().Bool("no-pager", false, "Do not pipe output into a pager")
	rootCmd.Flags().Bool("wrap", false, "Wrap long lines to terminal width in unified diff")
	rootCmd.Flags().Int("wrap-width", 0, "Explicit column width for line wrapping (0 to auto-detect terminal width)")
	rootCmd.Flags().Int("tab-width", 4, "Number of spaces per tab stop in diff output")
	rootCmd.Flags().Bool("parse-tree", false, "Parse files and dump the syntax tree for debugging")
	rootCmd.Flags().Bool("cst", false, "Dump the raw Tree-sitter concrete syntax tree instead of the Diffmantic AST")
	rootCmd.Flags().Bool("textconv", false, "Allow external text conversion filters to be run when comparing binary files")
	rootCmd.Flags().Bool("pager", false, "Process incoming Git diff stream from standard input")

	rootCmd.SetHelpFunc(HelpHandler)
	rootCmd.SetUsageFunc(UsageHandler)
	rootCmd.Flags().BoolP("help", "h", false, "Print help (-h for summary, --help for full manual)")
	rootCmd.Flags().BoolP("version", "v", false, "Print version")

	_ = rootCmd.RegisterFlagCompletionFunc("format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"side-by-side", "unified", "json", "actions", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})
	_ = rootCmd.RegisterFlagCompletionFunc("color", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"always", "never", "auto"}, cobra.ShellCompDirectiveNoFileComp
	})
}
