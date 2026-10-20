package cmd

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

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

// diffJob represents an in-flight file diff operation processed by a worker.
type diffJob struct {
	cf       changedFile
	srcBytes []byte
	dstBytes []byte
	dr       *pipeline.DiffResult
	err      error
	done     chan struct{}
}

// runDirectoryDiff recursively diffs two directories, streaming all changed files through a single pager session.
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

	// Stop workers, close the pager to unblock readers, then wait for goroutines to exit.
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	defer func() {
		cancel()
		if p != nil {
			p.Close()
		}
		wg.Wait()
	}()

	// Cancel background workers if the user exits the pager early.
	if p != nil {
		wg.Go(func() {
			select {
			case <-ctx.Done():
			case <-p.Done():
				cancel()
			}
		})
	}

	// Diff files concurrently across workers while streaming output in order.
	showBanner := len(changedFiles) > 1

	jobs := make([]diffJob, len(changedFiles))
	for i := range jobs {
		jobs[i].cf = changedFiles[i]
		jobs[i].done = make(chan struct{})
	}

	numWorkers := min(len(changedFiles), max(1, runtime.GOMAXPROCS(0)))
	maxInFlight := min(len(changedFiles), max(numWorkers*4, 64))

	workQueue := make(chan int, maxInFlight)
	sem := make(chan struct{}, maxInFlight)
	for range maxInFlight {
		sem <- struct{}{}
	}

	// Feed jobs to workers only as semaphore permits allow, keeping in-flight memory bounded.
	wg.Go(func() {
		for idx := range jobs {
			select {
			case <-ctx.Done():
				return
			case <-sem:
				select {
				case <-ctx.Done():
					return
				case workQueue <- idx:
				}
			}
		}
	})

	for range numWorkers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case idx := <-workQueue:
					processDirectoryJob(ctx, &jobs[idx], format, parseErrorLimit, sizeLimitKB, lineLimitLines, ignoreComments, opts)
				}
			}
		})
	}

	for i := range jobs {
		select {
		case <-ctx.Done():
			return hadErrors
		case <-jobs[i].done:
		}

		if ctx.Err() != nil {
			return hadErrors
		}

		if p != nil && !p.IsActive() {
			break
		}

		j := &jobs[i]
		if j.err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", j.err)
			hadErrors = true
		} else if j.dr != nil {
			displayA := filepath.Join(dirA, j.cf.relPath)
			displayB := filepath.Join(dirB, j.cf.relPath)
			if err := renderDiffResult(displayA, displayB, j.srcBytes, j.dstBytes, j.dr, renderConfig{
				format:      format,
				showBanner:  showBanner,
				writer:      writer,
				unifiedOpts: unifiedOpts,
				sbsOpts:     sbsOpts,
			}); err != nil {
				if pager.IsBrokenPipe(err) {
					return hadErrors
				}
				fmt.Fprintf(os.Stderr, "Error: rendering %s: %v\n", j.cf.relPath, err)
				hadErrors = true
			}
		}

		// Clear heavy buffers so the GC can reclaim them while remaining files finish.
		j.dr = nil
		j.srcBytes = nil
		j.dstBytes = nil

		select {
		case sem <- struct{}{}:
		default:
		}
	}

	return hadErrors
}

// processDirectoryJob reads the files for a job and executes the diff pipeline.
func processDirectoryJob(ctx context.Context, j *diffJob, format string, parseErrorLimit, sizeLimitKB, lineLimitLines int, ignoreComments bool, opts serialize.EnvelopeOptions) {
	defer close(j.done)

	if err := ctx.Err(); err != nil {
		j.err = err
		return
	}

	var err error
	if j.cf.pathA != "" {
		j.srcBytes, err = os.ReadFile(j.cf.pathA)
		if err != nil {
			j.err = fmt.Errorf("reading %s: %w", j.cf.pathA, err)
			return
		}
	}
	if j.cf.pathB != "" {
		j.dstBytes, err = os.ReadFile(j.cf.pathB)
		if err != nil {
			j.err = fmt.Errorf("reading %s: %w", j.cf.pathB, err)
			return
		}
	}

	if err := ctx.Err(); err != nil {
		j.err = err
		return
	}

	j.dr, err = pipeline.Run(j.srcBytes, j.dstBytes, j.cf.relPath, j.cf.relPath, pipeline.DiffOptions{
		ParseErrorLimit:  parseErrorLimit,
		IgnoreComments:   ignoreComments,
		DisableSizeLimit: sizeLimitKB <= 0,
		MaxASTFileSize:   sizeLimitKB * 1024,
		DisableLineLimit: lineLimitLines <= 0,
		MaxASTFileLines:  lineLimitLines,
		EnvelopeOpts:     opts,
	})
	if err != nil {
		j.err = fmt.Errorf("diffing %s: %w", j.cf.relPath, err)
		return
	}

	// The renderer only needs actions, alignment, and the envelope (unless printing raw actions), so drop the syntax trees and match tables to keep memory low.
	if j.dr != nil {
		j.dr.SrcAST = nil
		j.dr.DstAST = nil
		if format != "actions" {
			j.dr.MatchResult = nil
		}
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
