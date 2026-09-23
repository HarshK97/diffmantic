package cmd

import (
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


type FileComparison struct {
	RelPath   string 
	PathA     string 
	PathB     string 
	Status    string 
	ExistsInA bool
	ExistsInB bool
}

// runDirectoryDiff compares two directories recursively, rendering a diff for every changed file through a single shared pager session 
func runDirectoryDiff(cmd *cobra.Command, dirA, dirB string, format string, ignoreComments bool, parseErrorLimit int, sizeLimitKB int, lineLimitLines int, noPager bool) {
	comparisons, err := compareDirectories(dirA, dirB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: comparing directories: %v\n", err)
		os.Exit(1)
	}

	var changedFiles []FileComparison
	for _, comp := range comparisons {
		if comp.Status != "unchanged" {
			changedFiles = append(changedFiles, comp)
		}
	}

	
	sort.Slice(changedFiles, func(i, j int) bool {
		return changedFiles[i].RelPath < changedFiles[j].RelPath
	})

	if len(changedFiles) == 0 {
		return
	}

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

	showBanner := len(changedFiles) > 1
	hadErrors := false

	for _, comp := range changedFiles {
		if p != nil && !p.IsActive() {
			break
		}

		pathA, pathB := comp.PathA, comp.PathB
		switch comp.Status {
		case "added":
			pathA = os.DevNull
		case "deleted":
			pathB = os.DevNull
		}

		srcBytes, err := readFileOrDevNull(pathA)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathA, err)
			hadErrors = true
			continue 
			
	// one unreadable file no longer aborts the whole batch
		}


		dstBytes, err := readFileOrDevNull(pathB)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: reading %s: %v\n", pathB, err)
			hadErrors = true
			continue
		}

		dr, err := pipeline.Run(srcBytes, dstBytes, comp.RelPath, comp.RelPath, pipeline.DiffOptions{
			ParseErrorLimit:  parseErrorLimit,
			IgnoreComments:   ignoreComments,
			DisableSizeLimit: sizeLimitKB <= 0,
			MaxASTFileSize:   sizeLimitKB * 1024,
			DisableLineLimit: lineLimitLines <= 0,
			MaxASTFileLines:  lineLimitLines,
			EnvelopeOpts:     opts,
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: diffing %s: %v\n", comp.RelPath, err)
			hadErrors = true
			continue
		}

		switch format {
		case "side-by-side":
			if showBanner {
				numIns, numDel, numUpd := countLineStats(srcBytes, dstBytes, dr.Envelope)
				if err := sidebyside.RenderFileBanner(comp.RelPath, numIns, numDel, numUpd, sbsOpts.Color, writer); err != nil {
					if pager.IsBrokenPipe(err) {
						return
					}
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					hadErrors = true
					continue
				}
			}
			if err := sidebyside.Render(comp.RelPath, comp.RelPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, sbsOpts, writer); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				hadErrors = true
			}
		case "inline":
			output := inline.Render(comp.RelPath, comp.RelPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, inlineOpts)
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
				fmt.Fprintf(os.Stderr, "Error: serializing JSON for %s: %v\n", comp.RelPath, jsonErr)
				hadErrors = true
				continue
			}
			if _, err := writer.Write(jsonData); err != nil {
				fmt.Fprintf(os.Stderr, "Error: writing JSON for %s: %v\n", comp.RelPath, err)
				hadErrors = true
				continue
			}
			_, _ = writer.Write([]byte("\n"))
		case "actions":
			if _, err := fmt.Fprintf(writer, "Diffing  %s  →  %s\n\n", comp.RelPath, comp.RelPath); err != nil {
				if pager.IsBrokenPipe(err) {
					return
				}
				hadErrors = true
				continue
			}
			_ = engine.FprintMappings(writer, dr.MatchResult)
			_ = actions.FprintActions(writer, dr.EditScrt)
		}
	}

	if hadErrors {
		os.Exit(1)
	}
}

func readFileOrDevNull(path string) ([]byte, error) {
	if path == "" || path == os.DevNull || path == "/dev/null" {
		return []byte{}, nil
	}
	return os.ReadFile(path)
}


func compareDirectories(dirA, dirB string) ([]FileComparison, error) {
	filesA, err := listDirectoryFiles(dirA)
	if err != nil {
		return nil, fmt.Errorf("walking directory A: %w", err)
	}
	filesB, err := listDirectoryFiles(dirB)
	if err != nil {
		return nil, fmt.Errorf("walking directory B: %w", err)
	}

	allPaths := make(map[string]bool, len(filesA)+len(filesB))
	for path := range filesA {
		allPaths[path] = true
	}
	for path := range filesB {
		allPaths[path] = true
	}

	comparisons := make([]FileComparison, 0, len(allPaths))
	for relPath := range allPaths {
		pathA, existsA := filesA[relPath]
		pathB, existsB := filesB[relPath]

		comp := FileComparison{
			RelPath:   relPath,
			PathA:     pathA,
			PathB:     pathB,
			ExistsInA: existsA,
			ExistsInB: existsB,
		}

		switch {
		case existsA && existsB:
			identical, err := areFilesIdentical(pathA, pathB)
			if err != nil {
 // Can't confirm equality (like for the permission error) — treat as modified rather than silently dropping the file.
				comp.Status = "modified"
			} else if identical {
				comp.Status = "unchanged"
			} else {
				comp.Status = "modified"
			}
		case existsB:
			comp.Status = "added"
		case existsA:
			comp.Status = "deleted"
		}

		comparisons = append(comparisons, comp)
	}

	return comparisons, nil
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


func areFilesIdentical(pathA, pathB string) (bool, error) {
	bytesA, err := os.ReadFile(pathA)
	if err != nil {
		return false, err
	}
	bytesB, err := os.ReadFile(pathB)
	if err != nil {
		return false, err
	}
	return string(bytesA) == string(bytesB), nil
}
