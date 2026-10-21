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

// changedFile tracks a detected file change between two directory trees.
type changedFile struct {
	relPath string
	pathA   string
	pathB   string
}

// runDirectoryDiff recursively diffs two directories, streaming all changed files
// through a single pager session. It returns hadErrors=true if any diff job failed.
func runDirectoryDiff(
	cmd *cobra.Command,
	dirA, dirB string,
	format string,
	ignoreComments bool,
	parseErrorLimit int,
	sizeLimitKB int,
	lineLimitLines int,
	noPager bool,
) (hadErrors bool) {
	filesA, err := listDirectoryFiles(dirA)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory %s: %v\n", dirA, err)
		return true
	}
	filesB, err := listDirectoryFiles(dirB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: walking directory %s: %v\n", dirB, err)
		return true
	}

	allPaths := make(map[string]struct{}, len(filesA)+len(filesB))
	for relPath := range filesA {
		allPaths[relPath] = struct{}{}
	}
	for relPath := range filesB {
		allPaths[relPath] = struct{}{}
	}

	// Sort paths so diff output order is deterministic across runs.
	relPaths := slices.Sorted(maps.Keys(allPaths))

	format = cmp.Or(format, "side-by-side")

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

	// Find which files changed before starting the pager.
	var changedFiles []changedFile

	for _, relPath := range relPaths {
		pathA, existsInA := filesA[relPath]
		pathB, existsInB := filesB[relPath]

		var changed bool
		switch {
		case !existsInA, !existsInB:
			changed = true
		default:
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

			if infoA.Size() != infoB.Size() {
				changed = true
			} else {
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

	if len(changedFiles) == 0 {
		return hadErrors
	}

	var p *pager.Pager
	var writer io.Writer = os.Stdout
	if format == "unified" || format == "side-by-side" || format == "actions" {
		p, writer = pager.Start(noPager)
	}

	showBanner := len(changedFiles) > 1

	jobs := make([]diffJob, len(changedFiles))
	for i, cf := range changedFiles {
		jobs[i] = diffJob{
			displayA: filepath.Join(dirA, cf.relPath),
			displayB: filepath.Join(dirB, cf.relPath),
			relPathA: cf.relPath,
			relPathB: cf.relPath,
			load: func() ([]byte, []byte, error) {
				var srcBytes, dstBytes []byte
				var err error
				if cf.pathA != "" {
					srcBytes, err = os.ReadFile(cf.pathA)
					if err != nil {
						return nil, nil, fmt.Errorf("reading %s: %w", cf.pathA, err)
					}
				}
				if cf.pathB != "" {
					dstBytes, err = os.ReadFile(cf.pathB)
					if err != nil {
						return nil, nil, fmt.Errorf("reading %s: %w", cf.pathB, err)
					}
				}
				return srcBytes, dstBytes, nil
			},
		}
	}

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

	diffErrors, _ := runConcurrentDiffJobs(jobs, p, rc, diffOpts)
	if diffErrors {
		hadErrors = true
	}

	return hadErrors
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
