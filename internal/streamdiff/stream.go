// Package streamdiff intercepts streaming Git patches on stdin (git diff, git log -p,
// git show), fetches the blobs, and renders AST diffs in place of standard patch text.
package streamdiff

import (
	"bufio"
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/pager"
	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/unified"
)

// StreamConfig configures the streaming diff processor and renderers.
type StreamConfig struct {
	Format         string
	DiffOpts       pipeline.DiffOptions
	UnifiedOpts    unified.RenderOptions
	SideBySideOpts sidebyside.RenderOptions
	// IsActive reports whether the output receiver is still alive and accepting data.
	// If nil, processing continues until EOF.
	IsActive func() bool
	// Pager monitors the terminal pager subprocess, if active.
	Pager *pager.Pager
	// Context cancels streaming when the caller is done.
	Context context.Context
}

type streamItemKind int

const (
	itemRawText streamItemKind = iota
	itemDiffJob
)

type streamItem struct {
	kind    streamItemKind
	rawText []byte
	job     *diffJob
}

// diffJob is a single file diff sent to background workers.
type diffJob struct {
	hunk     *fileHunk
	srcBytes []byte // pre-fetched by reader so workers don't need git access
	dstBytes []byte

	out  []byte
	err  error
	done chan struct{}
}

// fileHunk buffers raw lines and tracks object IDs for a single file in the stream.
type fileHunk struct {
	oldPath      string
	newPath      string
	oldOID       string
	newOID       string
	lines        []string
	streamingRaw bool
}

const (
	defaultMaxWorkers = 4
	streamQueueCap    = 256
	diffJobSemCap     = 16
	rawChunkLimit     = 8 * 1024 // batch non-diff output up to 8 KB before sending to the queue
)

// rawChunkBuffer batches non-diff text so we don't flood the queue with single-line channel items.
type rawChunkBuffer struct {
	buf   bytes.Buffer
	queue chan<- streamItem
}

func (b *rawChunkBuffer) writeString(ctx context.Context, s string) error {
	b.buf.WriteString(s)
	if b.buf.Len() >= rawChunkLimit {
		return b.flush(ctx)
	}
	return nil
}

func (b *rawChunkBuffer) flush(ctx context.Context) error {
	if b.buf.Len() == 0 {
		return nil
	}
	chunk := bytes.Clone(b.buf.Bytes())
	b.buf.Reset()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.queue <- streamItem{kind: itemRawText, rawText: chunk}:
		return nil
	}
}

// processStreamJob diffs a single file and renders the output into job.out.
// It drops the AST and buffers before returning so memory doesn't stick around.
func processStreamJob(ctx context.Context, job *diffJob, cfg StreamConfig) {
	defer close(job.done)
	defer func() {
		job.srcBytes = nil
		job.dstBytes = nil
	}()

	if err := ctx.Err(); err != nil {
		job.err = err
		return
	}

	if bytes.Equal(job.srcBytes, job.dstBytes) && cfg.Format != "json" {
		return
	}

	var buf bytes.Buffer

	dr, err := pipeline.Run(job.srcBytes, job.dstBytes, job.hunk.oldPath, job.hunk.newPath, cfg.DiffOpts)
	if err != nil || dr == nil {
		_ = dumpLines(&buf, job.hunk.lines)
		job.out = buf.Bytes()
		return
	}

	if renderErr := renderDiff(&buf, dr, job.hunk, cfg); renderErr != nil {
		job.err = renderErr
		return
	}

	// Drop large trees right away to keep heap usage low.
	dr.SrcAST = nil
	dr.DstAST = nil
	dr.MatchResult = nil
	dr.SrcBytes = nil
	dr.DstBytes = nil

	job.out = buf.Bytes()
}

// Process parses a Git patch stream from in, runs 2-way diffs through the AST pipeline
// concurrently across a bounded worker pool, and passes everything else straight to out in order.
func Process(in io.Reader, out io.Writer, cwd string, cfg StreamConfig) error {
	ctx, cancel := context.WithCancel(cmp.Or(cfg.Context, context.Background()))

	numWorkers := min(max(1, runtime.GOMAXPROCS(0)), defaultMaxWorkers)
	workQueue := make(chan *diffJob, diffJobSemCap)
	streamQueue := make(chan streamItem, streamQueueCap)
	sem := make(chan struct{}, diffJobSemCap)

	var wgWorkers sync.WaitGroup

	var (
		flusherErr     error
		firstWorkerErr error
		readerErr      atomic.Pointer[error]
	)
	failReader := func(err error) {
		readerErr.CompareAndSwap(nil, &err)
		cancel()
	}

	// Clean up in order: cancel context, close pager handle, then wait for workers.
	// BatchSession is reader-confined and closed exclusively by the reader goroutine.
	defer func() {
		cancel()
		if cfg.Pager != nil {
			cfg.Pager.Close()
		}
		wgWorkers.Wait()
	}()

	if deadliner, ok := in.(interface{ SetReadDeadline(time.Time) error }); ok {
		stopDeadliner := context.AfterFunc(ctx, func() {
			_ = deadliner.SetReadDeadline(time.Now())
		})
		defer stopDeadliner()
	}

	// Stop processing if the pager quits (e.g. user pressed 'q').
	if cfg.Pager != nil {
		wgWorkers.Go(func() {
			select {
			case <-ctx.Done():
			case <-cfg.Pager.Done():
				cancel()
			}
		})
	}

	for range numWorkers {
		wgWorkers.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case job, ok := <-workQueue:
					if !ok {
						return
					}
					processStreamJob(ctx, job, cfg)
				}
			}
		})
	}

	// Flusher writes chunks and completed diffs to out in their original order.
	flusherDone := make(chan struct{})
	wgWorkers.Go(func() {
		defer close(flusherDone)

		for {
			select {
			case <-ctx.Done():
				return
			case item, ok := <-streamQueue:
				if !ok {
					return
				}
				switch item.kind {
				case itemRawText:
					if len(item.rawText) > 0 {
						if _, err := out.Write(item.rawText); err != nil {
							if pager.IsBrokenPipe(err) {
								cancel()
								return
							}
							flusherErr = err
							cancel()
							return
						}
					}
				case itemDiffJob:
					select {
					case <-ctx.Done():
						return
					case <-item.job.done:
					}

					if item.job.err != nil {
						if pager.IsBrokenPipe(item.job.err) || errors.Is(item.job.err, context.Canceled) {
							cancel()
							return
						}
						if firstWorkerErr == nil {
							firstWorkerErr = item.job.err
						}
					}

					if len(item.job.out) > 0 {
						if _, err := out.Write(item.job.out); err != nil {
							if pager.IsBrokenPipe(err) {
								cancel()
								return
							}
							flusherErr = err
							cancel()
							return
						}
					}

					item.job.out = nil

					// Free up a slot so the reader can queue another diff.
					select {
					case <-sem:
					default:
					}
				}
			}
		}
	})

	// The reader runs separately from wgWorkers so a blocked stdin read won't hold up teardown.
	go func() {
		var session *git.BatchSession
		gitAvailable := true

		defer func() {
			if session != nil {
				_ = session.Close()
			}
			close(workQueue)
			close(streamQueue)
		}()

		reader := bufio.NewReader(in)
		rawBuf := &rawChunkBuffer{
			queue: streamQueue,
		}

		var current *fileHunk

		dumpToRawBuf := func(lines []string) error {
			for _, l := range lines {
				if err := rawBuf.writeString(ctx, l); err != nil {
					return err
				}
			}
			return nil
		}

		flushCurrent := func() error {
			if current == nil {
				return nil
			}
			h := current
			current = nil

			if h.streamingRaw {
				return nil
			}

			if h.oldPath == "" || h.newPath == "" || h.oldOID == "" || h.newOID == "" {
				return dumpToRawBuf(h.lines)
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			oldBytes, newBytes, err := fetchBuffers(cwd, h, session, gitAvailable)
			if err != nil {
				return dumpToRawBuf(h.lines)
			}

			if err := rawBuf.flush(ctx); err != nil {
				return err
			}

			job := &diffJob{
				hunk:     h,
				srcBytes: oldBytes,
				dstBytes: newBytes,
				done:     make(chan struct{}),
			}

			permitAcquired := false
			defer func() {
				if permitAcquired {
					select {
					case <-sem:
					default:
					}
				}
			}()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case sem <- struct{}{}:
				permitAcquired = true
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case workQueue <- job:
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case streamQueue <- streamItem{kind: itemDiffJob, job: job}:
				permitAcquired = false // ownership safely transferred to flusher
			}

			return nil
		}

		for {
			if ctx.Err() != nil {
				return
			}

			if cfg.IsActive != nil && !cfg.IsActive() {
				break
			}

			line, err := reader.ReadString('\n')
			if err != nil && len(line) == 0 {
				if errors.Is(err, io.EOF) {
					break
				}
				failReader(err)
				return
			}

			cleanLine := cleanStreamLine(line)

			if strings.HasPrefix(cleanLine, "commit ") || strings.HasPrefix(cleanLine, "From ") {
				if flushErr := flushCurrent(); flushErr != nil {
					failReader(flushErr)
					return
				}
				if err := rawBuf.writeString(ctx, line); err != nil {
					failReader(err)
					return
				}
				continue
			}

			if strings.HasPrefix(cleanLine, "diff --git ") {
				if flushErr := flushCurrent(); flushErr != nil {
					failReader(flushErr)
					return
				}

				if session == nil && gitAvailable {
					if s, err := git.StartBatchSession(cwd); err == nil {
						session = s
					} else {
						gitAvailable = false
					}
				}

				oldPath, newPath := parseDiffGitPaths(cleanLine)
				current = &fileHunk{
					oldPath: oldPath,
					newPath: newPath,
					lines:   []string{line},
				}
				continue
			}

			// Combined diff detection (pass straight through)
			if strings.HasPrefix(cleanLine, "diff --combined ") || strings.HasPrefix(cleanLine, "diff --cc ") {
				if flushErr := flushCurrent(); flushErr != nil {
					failReader(flushErr)
					return
				}
				if err := rawBuf.writeString(ctx, line); err != nil {
					failReader(err)
					return
				}
				continue
			}

			if current != nil {
				if current.streamingRaw {
					if err := rawBuf.writeString(ctx, line); err != nil {
						failReader(err)
						return
					}
					continue
				}

				current.lines = append(current.lines, line)
				if len(current.lines) > 50000 {
					if err := dumpToRawBuf(current.lines); err != nil {
						failReader(err)
						return
					}
					current.lines = nil
					current.streamingRaw = true
					continue
				}

				if strings.HasPrefix(cleanLine, "index ") && current.oldOID == "" {
					oldOID, newOID := parseIndexOIDs(cleanLine)
					current.oldOID = oldOID
					current.newOID = newOID
				} else if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
					current.oldPath = p
				} else if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
					current.newPath = p
				} else if p := parseHeaderPath(cleanLine, "--- "); p != "" {
					current.oldPath = p
				} else if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
					current.newPath = p
				}
				continue
			}

			if err := rawBuf.writeString(ctx, line); err != nil {
				failReader(err)
				return
			}

			if errors.Is(err, io.EOF) {
				break
			}
		}

		if flushErr := flushCurrent(); flushErr != nil {
			failReader(flushErr)
			return
		}
		_ = rawBuf.flush(ctx)
	}()

	<-flusherDone

	if flusherErr != nil && !pager.IsBrokenPipe(flusherErr) {
		return flusherErr
	}
	var rErr error
	if p := readerErr.Load(); p != nil {
		rErr = *p
	}
	if rErr != nil && !pager.IsBrokenPipe(rErr) && !errors.Is(rErr, context.Canceled) && !errors.Is(rErr, os.ErrDeadlineExceeded) {
		return rErr
	}
	if ctx.Err() == nil && firstWorkerErr != nil {
		return fmt.Errorf("streamdiff worker: %w", firstWorkerErr)
	}
	return nil
}

// parseDiffGitPaths extracts the old and new file paths from "diff --git a/foo b/bar".
// It handles standard prefixes, unquoted, quoted, and asymmetrically quoted paths.
func parseDiffGitPaths(line string) (string, string) {
	rest, ok := strings.CutPrefix(line, "diff --git ")
	if !ok {
		return "", ""
	}
	rest = strings.TrimSpace(rest)

	// Symmetric unquoted path with spaces: "a/path with spaces.go b/path with spaces.go"
	if strings.HasPrefix(rest, "a/") {
		if rem := len(rest) - 5; rem > 0 && rem%2 == 0 {
			xLen := rem / 2
			first := rest[2 : 2+xLen]
			sep := rest[2+xLen : 2+xLen+3]
			second := rest[2+xLen+3:]
			if sep == " b/" && first == second {
				return first, second
			}
		}
	} else if rem := len(rest) - 1; rem > 0 && rem%2 == 0 {
		xLen := rem / 2
		if rest[xLen] == ' ' && rest[:xLen] == rest[xLen+1:] {
			return rest[:xLen], rest[xLen+1:]
		}
	}

	pA, rest := parseGitPathToken(rest)
	pB, _ := parseGitPathToken(rest)
	if pA != "" && pB != "" {
		return pA, pB
	}
	return "", ""
}

// parseGitPathToken extracts a single path token (quoted or unquoted) from s.
// It trims Git's "a/" or "b/" prefix and unquotes C-style escape sequences when present.
func parseGitPathToken(s string) (string, string) {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return "", ""
	}
	if s[0] == '"' {
		// Quoted path: find unescaped closing quote
		for i := 1; i < len(s); i++ {
			if s[i] == '\\' {
				i++
				continue
			}
			if s[i] == '"' {
				raw := s[:i+1]
				unquoted, err := strconv.Unquote(raw)
				if err != nil {
					unquoted = s[1:i]
				}
				rem := strings.TrimSpace(s[i+1:])
				return cleanGitPathPrefix(unquoted), rem
			}
		}
		// Fallback for unclosed quote
		return cleanGitPathPrefix(strings.Trim(s, "\"")), ""
	}

	if before, after, ok := strings.Cut(s, " "); ok {
		return cleanGitPathPrefix(before), strings.TrimSpace(after)
	}
	return cleanGitPathPrefix(s), ""
}

// parseHeaderPath extracts the target path from hunk headers like "--- a/path",
// "+++ b/path", "rename from path", and "rename to path".
func parseHeaderPath(line, prefix string) string {
	after, ok := strings.CutPrefix(line, prefix)
	if !ok {
		return ""
	}
	after = strings.TrimSpace(after)
	pathPart, _, _ := strings.Cut(after, "\t")
	pathPart = strings.TrimSpace(pathPart)
	if pathPart == "" || pathPart == os.DevNull || pathPart == "/dev/null" {
		return ""
	}
	if strings.HasPrefix(pathPart, "\"") {
		p, _ := parseGitPathToken(pathPart)
		return p
	}
	return cleanGitPathPrefix(pathPart)
}

func cleanGitPathPrefix(p string) string {
	if after, ok := strings.CutPrefix(p, "a/"); ok {
		return after
	}
	if after, ok := strings.CutPrefix(p, "b/"); ok {
		return after
	}
	return p
}

// parseIndexOIDs extracts old and new object IDs from "index 1234567..89abcdef 100644".
func parseIndexOIDs(line string) (string, string) {
	rest, ok := strings.CutPrefix(line, "index ")
	if !ok {
		return "", ""
	}
	rest = strings.TrimSpace(rest)

	hashes, _, _ := strings.Cut(rest, " ")
	oldOID, newOID, found := strings.Cut(hashes, "..")
	if !found || strings.Contains(oldOID, ",") || strings.Contains(newOID, ",") {
		return "", ""
	}
	return oldOID, newOID
}

func isAllZeros(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] != '0' {
			return false
		}
	}
	return true
}

// fetchBlob loads object content from the batch coprocess, falling back
// to git cat-file or working-tree disk files on missing hashes.
func fetchBlob(cwd, oid, path string, session *git.BatchSession, gitAvailable bool) ([]byte, bool, error) {
	if isAllZeros(oid) {
		return nil, true, nil
	}
	var data []byte
	var err error
	if session != nil && !session.IsPoisoned() {
		data, err = session.Fetch(oid)
		if err != nil {
			return nil, false, fmt.Errorf("reading blob %s: %w", oid, err)
		}
	} else if gitAvailable {
		data, err = git.RunGit(cwd, "cat-file", "-p", oid)
		if err != nil {
			data = nil
		}
	}

	if data != nil {
		return data, true, nil
	}

	diskPath := filepath.Join(cwd, path)
	diskData, diskErr := os.ReadFile(diskPath)
	if diskErr != nil {
		return nil, false, fmt.Errorf("reading file %s: %w", path, diskErr)
	}
	return diskData, false, nil
}

// fetchBuffers retrieves before and after file contents from Git objects or disk.
func fetchBuffers(cwd string, h *fileHunk, session *git.BatchSession, gitAvailable bool) ([]byte, []byte, error) {
	oldBytes, oldFromGit, err := fetchBlob(cwd, h.oldOID, h.oldPath, session, gitAvailable)
	if err != nil {
		return nil, nil, err
	}
	newBytes, newFromGit, err := fetchBlob(cwd, h.newOID, h.newPath, session, gitAvailable)
	if err != nil {
		return nil, nil, err
	}

	// If neither blob existed in Git and both read the exact same working-tree file,
	// the commit objects are missing from the local repo (e.g. a shallow clone).
	if !oldFromGit && !newFromGit && !isAllZeros(h.oldOID) && !isAllZeros(h.newOID) && h.oldOID != h.newOID && h.oldPath == h.newPath {
		return nil, nil, fmt.Errorf("git blobs %s and %s missing from repository", h.oldOID, h.newOID)
	}

	return oldBytes, newBytes, nil
}

func formatBannerPath(h *fileHunk) string {
	if h.oldPath != "" && h.newPath != "" && h.oldPath != h.newPath {
		if isAllZeros(h.oldOID) || h.oldPath == os.DevNull || h.oldPath == "/dev/null" {
			return h.newPath
		}
		if isAllZeros(h.newOID) || h.newPath == os.DevNull || h.newPath == "/dev/null" {
			return h.oldPath
		}
		return fmt.Sprintf("%s → %s", h.oldPath, h.newPath)
	}
	if h.newPath != "" && h.newPath != os.DevNull && h.newPath != "/dev/null" {
		return h.newPath
	}
	return h.oldPath
}

// renderDiff writes the AST diff in the requested format.
func renderDiff(out io.Writer, dr *pipeline.DiffResult, h *fileHunk, cfg StreamConfig) error {
	path := formatBannerPath(h)

	if dr.IsBinary {
		if cfg.Format == "side-by-side" || cfg.Format == "" {
			if err := sidebyside.RenderFileBanner(path, 0, 0, 0, "", cfg.SideBySideOpts.Color, out); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(out, "Binary files %s and %s differ\n", h.oldPath, h.newPath)
		return err
	}

	switch cfg.Format {
	case "unified":
		output := unified.Render(h.oldPath, h.newPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, cfg.UnifiedOpts)
		if output != "" {
			if _, err := io.WriteString(out, output); err != nil {
				return err
			}
			if !strings.HasSuffix(output, "\n") {
				if _, err := io.WriteString(out, "\n"); err != nil {
					return err
				}
			}
		}
		return nil
	case "json":
		jsonData, err := json.MarshalIndent(dr.Envelope, "", "  ")
		if err != nil {
			return err
		}
		if _, err := out.Write(jsonData); err != nil {
			return err
		}
		_, err = out.Write([]byte("\n"))
		return err
	case "actions":
		if _, err := fmt.Fprintf(out, "Diffing  %s  →  %s\n\n", h.oldPath, h.newPath); err != nil {
			return err
		}
		if err := engine.FprintMappings(out, dr.MatchResult); err != nil {
			return err
		}
		return actions.FprintActions(out, dr.EditScript)
	default:
		significantWhitespace := treesitter.DetectSignificantWhitespace(h.oldPath, h.newPath)
		ins, del, upd := sidebyside.CountLineStats(dr.SrcBytes, dr.DstBytes, dr.Envelope, cfg.SideBySideOpts.ShowStyleChanges, significantWhitespace)
		annotation := ""
		if !cfg.SideBySideOpts.DisableAnnotations && dr.Envelope != nil {
			annotation = dr.Envelope.FallbackAnnotation
		}
		if err := sidebyside.RenderFileBanner(path, ins, del, upd, annotation, cfg.SideBySideOpts.Color, out); err != nil {
			return err
		}
		sbsOpts := cfg.SideBySideOpts
		sbsOpts.SuppressHunkFallbackAnnotation = true
		return sidebyside.Render(h.oldPath, h.newPath, dr.SrcBytes, dr.DstBytes, dr.Envelope, sbsOpts, out)
	}
}

// dumpLines replays buffered raw Git lines to out.
func dumpLines(out io.Writer, lines []string) error {
	for _, l := range lines {
		if _, err := io.WriteString(out, l); err != nil {
			return err
		}
	}
	return nil
}

var ansiRegex = regexp.MustCompile(`\x1b(?:\[[0-9;?]*[a-zA-Z]|\([a-zA-Z])`)

// cleanStreamLine strips ANSI escape codes and trailing newlines for header matching.
// Skips regex stripping when no escape character is present to avoid extra allocations.
func cleanStreamLine(line string) string {
	cleaned := line
	if strings.Contains(line, "\x1b") {
		cleaned = ansiRegex.ReplaceAllString(line, "")
	}
	return strings.TrimRight(cleaned, "\r\n")
}
