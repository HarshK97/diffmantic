package cmd

import (
	"bytes"
	"cmp"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"

	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/spf13/cobra"
)

// runDirectoryDiff compares two directories recursively, rendering a diff for every changed file through a single shared pager session.
func runDirectoryDiff(cmd *cobra.Command, dirA, dirB string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) {
	// Build maps of relative path -> full path for both directories
	filesA, err := listDirectoryFiles(dirA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory %s: %v\n", dirA, err)
		os.Exit(1)
	}
	filesB, err := listDirectoryFiles(dirB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory %s: %v\n", dirB, err)
		os.Exit(1)
	}

	allPaths := make(map[string]struct{}, len(filesA)+len(filesB))
	for relPath := range filesA {
		allPaths[relPath] = struct{}{}
	}
	for relPath := range filesB {
		allPaths[relPath] = struct{}{}
	}

	// Convert to sorted slice for deterministic output
	relPaths := slices.Sorted(maps.Keys(allPaths))

	format = cmp.Or(format, "side-by-side")

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

	hadErrors := false

	// First pass: collect changed files
	type changedFile struct {
		relPath  string
		srcBytes []byte
		dstBytes []byte
	}
	var changedFiles []changedFile

	for _, relPath := range relPaths {
		pathA, existsInA := filesA[relPath]
		pathB, existsInB := filesB[relPath]

		var srcBytes, dstBytes []byte

		// Determine file status and read bytes
		switch {
		case !existsInA:
			// File added: only exists in B
			srcBytes = []byte{}
			var err error
			dstBytes, err = os.ReadFile(pathB)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathB, err)
				hadErrors = true
				continue
			}
		case !existsInB:
			// File deleted: only exists in A
			var err error
			srcBytes, err = os.ReadFile(pathA)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathA, err)
				hadErrors = true
				continue
			}
			dstBytes = []byte{}
		default:
			// Both exist: check if modified
			var errA, errB error
			srcBytes, errA = os.ReadFile(pathA)
			dstBytes, errB = os.ReadFile(pathB)
			
			if errA != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathA, errA)
				hadErrors = true
				continue
			}
			if errB != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathB, errB)
				hadErrors = true
				continue
			}

			// Skip unchanged files
			if bytes.Equal(srcBytes, dstBytes) {
				continue
			}
		}

		changedFiles = append(changedFiles, changedFile{
			relPath:  relPath,
			srcBytes: srcBytes,
			dstBytes: dstBytes,
		})
	}

	// No changed files - check if it's due to errors or truly no changes
	if len(changedFiles) == 0 {
		if hadErrors {
			os.Exit(1)  // Had errors and nothing to show
		}
		return  // No changes, exit with success
	}

	// Initialize pager now that we know we have changes to display
	var p *pager.Pager
	var writer io.Writer = os.Stdout
	if format == "inline" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
		defer p.Close()
	}

	// Second pass: render all changed files with correct banner setting
	showBanner := len(changedFiles) > 1

	for _, cf := range changedFiles {
		if p != nil && !p.IsActive() {
			break
		}

		dr, err := pipeline.Run(cf.srcBytes, cf.dstBytes, cf.relPath, cf.relPath, pipeline.DiffOptions{
			ParseErrorLimit:  parseErrorLimit,
			IgnoreComments:   ignoreComments,
			DisableSizeLimit: sizeLimitKB <= 0,
			MaxASTFileSize:   sizeLimitKB * 1024,
			DisableLineLimit: lineLimitLines <= 0,
			MaxASTFileLines:  lineLimitLines,
			EnvelopeOpts:     opts,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: diffing %s: %v\n", cf.relPath, err)
			hadErrors = true
			continue
		}

		// Use the shared rendering function
		if err := renderDiffResult(cf.relPath, cf.relPath, cf.srcBytes, cf.dstBytes, dr, renderConfig{
			format:     format,
			showBanner: showBanner,
			writer:     writer,
			inlineOpts: inlineOpts,
			sbsOpts:    sbsOpts,
		}); err != nil {
			if pager.IsBrokenPipe(err) {
				return
			}
			fmt.Fprintf(os.Stderr, "Error: rendering %s: %v\n", cf.relPath, err)
			hadErrors = true
		}
	}

	if hadErrors {
		os.Exit(1)
	}
}

func listDirectoryFiles(root string) (map[string]string, error) {
	files := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" && path != root {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relPath)] = path
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}
