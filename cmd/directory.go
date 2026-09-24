package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/inline"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/spf13/cobra"
)

// excludedDirNames lists directory names that are never walked when
// comparing two directory trees, regardless of depth.
var excludedDirNames = map[string]bool{
	".git": true,
}

// runDirectoryDiff compares two directories recursively, rendering a diff for every changed file through a single shared pager session.
func runDirectoryDiff(cmd *cobra.Command, dirA, dirB string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) {
	// Build maps of relative path -> full path for both directories
	filesA, err := listDirectoryFiles(dirA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory A: %v\n", err)
		os.Exit(1)
	}
	filesB, err := listDirectoryFiles(dirB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory B: %v\n", err)
		os.Exit(1)
	}

	// Collect all unique relative paths from both directories
	allPaths := make(map[string]bool, len(filesA)+len(filesB))
	for relPath := range filesA {
		allPaths[relPath] = true
	}
	for relPath := range filesB {
		allPaths[relPath] = true
	}

	// Convert to sorted slice for deterministic output
	relPaths := make([]string, 0, len(allPaths))
	for relPath := range allPaths {
		relPaths = append(relPaths, relPath)
	}
	sort.Strings(relPaths)

	if format == "" {
		format = "side-by-side"
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

	// One pager session for the whole batch, not one per file.
	var p *pager.Pager
	var writer io.Writer = os.Stdout
	if format == "inline" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
		defer p.Close()
	}

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
		if !existsInA {
			// File added: only exists in B
			srcBytes = []byte{}
			var err error
			dstBytes, err = os.ReadFile(pathB)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathB, err)
				hadErrors = true
				continue
			}
		} else if !existsInB {
			// File deleted: only exists in A
			var err error
			srcBytes, err = os.ReadFile(pathA)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathA, err)
				hadErrors = true
				continue
			}
			dstBytes = []byte{}
		} else {
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

		switch format {
		case "side-by-side":
			if showBanner {
				numIns, numDel, numUpd := countLineStats(cf.srcBytes, cf.dstBytes, dr.Envelope)
				if err := sidebyside.RenderFileBanner(cf.relPath, numIns, numDel, numUpd, sbsOpts.Color, writer); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					hadErrors = true
					continue
				}
			}
			if err := sidebyside.Render(cf.relPath, cf.relPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, sbsOpts, writer); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				hadErrors = true
			}
		case "inline":
			output := inline.Render(cf.relPath, cf.relPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, inlineOpts)
			if output != "" {
				if _, err := io.WriteString(writer, output); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: writing diff output: %v\n", err)
					hadErrors = true
					continue
				}
				if !strings.HasSuffix(output, "\n") {
					if _, err := io.WriteString(writer, "\n"); err != nil && pager.IsBrokenPipe(err) {
						return
					}
				}
			}
		case "json":
			var jsonData []byte
			var jsonErr error
			if showBanner {
				jsonData, jsonErr = json.Marshal(dr.Envelope)
			} else {
				jsonData, jsonErr = json.MarshalIndent(dr.Envelope, "", "  ")
			}
			if jsonErr != nil {
				fmt.Fprintf(os.Stderr, "Error: serializing JSON for %s: %v\n", cf.relPath, jsonErr)
				hadErrors = true
				continue
			}
			if _, err := writer.Write(jsonData); err != nil {
				fmt.Fprintf(os.Stderr, "Error: writing JSON for %s: %v\n", cf.relPath, err)
				hadErrors = true
				continue
			}
			_, _ = writer.Write([]byte("\n"))
		case "actions":
			if _, err := fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", cf.relPath, cf.relPath); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
				hadErrors = true
				continue
			}
			_ = engine.FprintMappings(writer, dr.MatchResult)
			_ = actions.FprintActions(writer, dr.EditScript)
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
		if d.IsDir() {
			if path != root && excludedDirNames[d.Name()] {
				return filepath.SkipDir
			}
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
