package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
)

type diffJob struct {
	displayA string
	displayB string
	relPathA string
	relPathB string
	srcBytes []byte
	dstBytes []byte
	isBinary bool
	load     func() (srcBytes, dstBytes []byte, err error)

	// Holds the rendered diff until it's this file's turn to print.
	out       []byte
	rendered  bool
	renderErr error
	err       error
	done      chan struct{}
}

// processDiffJob runs the diff and renders directly into j.out so we can drop
// the AST and match tables immediately, rather than holding them in memory
// until earlier files finish printing.
func processDiffJob(ctx context.Context, j *diffJob, diffOpts pipeline.DiffOptions, rc renderConfig) {
	defer close(j.done)
	defer func() {
		j.srcBytes = nil
		j.dstBytes = nil
	}()

	if err := ctx.Err(); err != nil {
		j.err = err
		return
	}

	if j.load != nil {
		var err error
		j.srcBytes, j.dstBytes, err = j.load()
		if err != nil {
			j.err = err
			return
		}
	}

	if err := ctx.Err(); err != nil {
		j.err = err
		return
	}

	var buf bytes.Buffer
	rc.writer = &buf

	if j.isBinary {
		renderBinaryDiff(j.displayA, j.displayB, rc)
		j.out, j.rendered = buf.Bytes(), true
		return
	}

	if j.srcBytes == nil {
		j.displayA = "/dev/null"
		j.relPathA = "/dev/null"
		j.srcBytes = []byte{}
	}
	if j.dstBytes == nil {
		j.displayB = "/dev/null"
		j.relPathB = "/dev/null"
		j.dstBytes = []byte{}
	}

	if bytes.Equal(j.srcBytes, j.dstBytes) && rc.format != "json" {
		return
	}

	dr, err := pipeline.Run(j.srcBytes, j.dstBytes, j.relPathA, j.relPathB, diffOpts)
	if err != nil {
		j.err = err
		return
	}

	if dr != nil {
		j.renderErr = renderDiffResult(j.displayA, j.displayB, j.srcBytes, j.dstBytes, dr, rc)
		dr.SrcAST = nil
		dr.DstAST = nil
		dr.MatchResult = nil
	}
	j.out, j.rendered = buf.Bytes(), true
}

// defaultMaxWorkers caps the pool. Each worker holds a full pipeline's working
// set, and past ~4 workers the speedup flattens while memory keeps growing linearly.
const defaultMaxWorkers = 4

func defaultWorkerCount() int {
	return min(max(1, runtime.GOMAXPROCS(0)), defaultMaxWorkers)
}

// runConcurrentDiffJobs diffs files in parallel across a worker pool, streaming
// output to rc.writer in the original job order.
func runConcurrentDiffJobs(
	jobs []diffJob,
	p *pager.Pager,
	rc renderConfig,
	diffOpts pipeline.DiffOptions,
) (hadErrors bool, filesRendered int) {
	if len(jobs) == 0 {
		return false, 0
	}

	numWorkers := min(len(jobs), defaultWorkerCount())
	maxInFlight := min(len(jobs), max(numWorkers*4, 64))

	workQueue := make(chan *diffJob, maxInFlight)
	inFlight := make(chan *diffJob, maxInFlight)
	sem := make(chan struct{}, maxInFlight)
	for range maxInFlight {
		sem <- struct{}{}
	}

	// Teardown order matters: cancel workers first so they stop generating output,
	// close the pager to unblock any writes, then wait for goroutines to drain.
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

	// Feed jobs to workers only as semaphore permits allow, keeping in-flight memory bounded.
	wg.Go(func() {
		for idx := range jobs {
			select {
			case <-ctx.Done():
				return
			case <-sem:
				j := &jobs[idx]
				j.done = make(chan struct{})
				select {
				case <-ctx.Done():
					return
				case workQueue <- j:
				}
				select {
				case <-ctx.Done():
					return
				case inFlight <- j:
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
				case j := <-workQueue:
					processDiffJob(ctx, j, diffOpts, rc)
				}
			}
		})
	}

	for range jobs {
		var j *diffJob
		select {
		case <-ctx.Done():
			return hadErrors, filesRendered
		case j = <-inFlight:
		}

		select {
		case <-ctx.Done():
			return hadErrors, filesRendered
		case <-j.done:
		}

		if ctx.Err() != nil {
			return hadErrors, filesRendered
		}

		if j.err != nil {
			if !pager.IsBrokenPipe(j.err) {
				fmt.Fprintf(os.Stderr, "Error: %v\n", j.err)
			}
			hadErrors = true
		} else if j.rendered {
			filesRendered++
			if _, err := rc.writer.Write(j.out); err != nil {
				if pager.IsBrokenPipe(err) {
					return hadErrors, filesRendered
				}
				fmt.Fprintf(os.Stderr, "Error: rendering %s: %v\n", j.displayB, err)
				hadErrors = true
			} else if j.renderErr != nil {
				fmt.Fprintf(os.Stderr, "Error: rendering %s: %v\n", j.displayB, j.renderErr)
				hadErrors = true
			}
		}

		// Clear rendered bytes and sync channel so memory stays flat while remaining files finish.
		j.out = nil
		j.done = nil

		sem <- struct{}{}
	}

	return hadErrors, filesRendered
}
