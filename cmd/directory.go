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

	// First pass: identify changed files without buffering content
	type changedFile struct {
		relPath string
		pathA   string
		pathB   string
	}
	var changedFiles []changedFile

	for _, relPath := range relPaths {
		pathA, existsInA := filesA[relPath]
		pathB, existsInB := filesB[relPath]

		// Determine if file changed
		var changed bool
		switch {
		case !existsInA:
			// File added: only exists in B
			changed = true
		case !existsInB:
			// File deleted: only exists in A
			changed = true
		default:
			// Both exist: check if modified using size first, then content
			infoA, errA := os.Stat(pathA)
			infoB, errB := os.Stat(pathB)

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

			// If sizes differ, files are different
			if infoA.Size() != infoB.Size() {
				changed = true
			} else {
				// Same size - read and compare content
				srcBytes, errA := os.ReadFile(pathA)
				dstBytes, errB := os.ReadFile(pathB)

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

				changed = !bytes.Equal(srcBytes, dstBytes)
			}
		}

		if changed {
			changedFiles = append(changedFiles, changedFile{
				relPath: relPath,
				pathA:   pathA,
				pathB:   pathB,
			})
		}
	}

	// No changed files - check if it's due to errors or truly no changes
	if len(changedFiles) == 0 {
		if hadErrors {
			os.Exit(1) // Had errors and nothing to show
		}
		return // No changes, exit with success
	}

	// Initialize pager now that we know we have changes to display
	var p *pager.Pager
	var writer io.Writer = os.Stdout
	if format == "inline" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
		defer p.Close()
	}

	// Second pass: read and render each changed file on demand
	showBanner := len(changedFiles) > 1

	for _, cf := range changedFiles {
		if p != nil && !p.IsActive() {
			break
		}

		// Read file bytes on demand
		var srcBytes, dstBytes []byte
		var err error

		if cf.pathA == "" {
			// File added: only exists in B
			srcBytes = []byte{}
			dstBytes, err = os.ReadFile(cf.pathB)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", cf.pathB, err)
				hadErrors = true
				continue
			}
		} else if cf.pathB == "" {
			// File deleted: only exists in A
			srcBytes, err = os.ReadFile(cf.pathA)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", cf.pathA, err)
				hadErrors = true
				continue
			}
			dstBytes = []byte{}
		} else {
			// Both exist
			var errA, errB error
			srcBytes, errA = os.ReadFile(cf.pathA)
			dstBytes, errB = os.ReadFile(cf.pathB)

			if errA != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", cf.pathA, errA)
				hadErrors = true
				continue
			}
			if errB != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", cf.pathB, errB)
				hadErrors = true
				continue
			}
		}

		dr, err := pipeline.Run(srcBytes, dstBytes, cf.relPath, cf.relPath, pipeline.DiffOptions{
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

		displayA := filepath.Join(dirA, cf.relPath)
		displayB := filepath.Join(dirB, cf.relPath)
		if err := renderDiffResult(displayA, displayB, srcBytes, dstBytes, dr, renderConfig{
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
		if p != nil {
			p.Close()
		}
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
