package cmd

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/HarshK97/diffmantic/internal/pipeline"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/sidebyside"
	"github.com/HarshK97/diffmantic/internal/unified"
)

func TestRunConcurrentDiffJobs_Empty(t *testing.T) {
	rc := renderConfig{format: "unified", writer: &bytes.Buffer{}}
	diffOpts := pipeline.DiffOptions{DisableSizeLimit: true, DisableLineLimit: true}
	hadErrors, filesRendered := runConcurrentDiffJobs(nil, nil, rc, diffOpts)
	if hadErrors {
		t.Error("expected hadErrors to be false for empty jobs")
	}
	if filesRendered != 0 {
		t.Errorf("expected filesRendered to be 0, got %d", filesRendered)
	}
}

func TestRunConcurrentDiffJobs_StrictOrdering(t *testing.T) {
	const numJobs = 20
	jobs := make([]diffJob, numJobs)
	for i := range jobs {
		name := fmt.Sprintf("file_%02d.go", i)
		jobs[i] = diffJob{
			displayA: name,
			displayB: name,
			relPathA: name,
			relPathB: name,
			srcBytes: fmt.Appendf(nil, "package pkg\n\nfunc F%02d() int {\n\treturn %d\n}\n", i, i),
			dstBytes: fmt.Appendf(nil, "package pkg\n\nfunc F%02d() int {\n\treturn %d\n}\n", i, (i+1)*10),
			done:     make(chan struct{}),
		}
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format:      "unified",
		showBanner:  true,
		writer:      &buf,
		unifiedOpts: unified.RenderOptions{},
		sbsOpts:     sidebyside.RenderOptions{},
	}
	diffOpts := pipeline.DiffOptions{
		DisableSizeLimit: true,
		DisableLineLimit: true,
		EnvelopeOpts:     serialize.EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true},
	}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if hadErrors {
		t.Fatal("expected hadErrors to be false")
	}
	if filesRendered != numJobs {
		t.Fatalf("expected filesRendered to be %d, got %d", numJobs, filesRendered)
	}

	out := buf.String()
	lastPos := -1
	for i := range numJobs {
		name := fmt.Sprintf("file_%02d.go", i)
		pos := strings.Index(out, name)
		if pos == -1 {
			t.Fatalf("expected output to contain %q, but it was missing", name)
		}
		if pos < lastPos {
			t.Fatalf("file %q rendered out of order: pos %d < lastPos %d", name, pos, lastPos)
		}
		lastPos = pos
	}
}

func TestRunConcurrentDiffJobs_MemoryFreed(t *testing.T) {
	jobs := []diffJob{
		{
			displayA: "test.go",
			displayB: "test.go",
			relPathA: "test.go",
			relPathB: "test.go",
			srcBytes: []byte("package main\nvar x = 1\n"),
			dstBytes: []byte("package main\nvar x = 2\n"),
			done:     make(chan struct{}),
		},
	}

	var buf bytes.Buffer
	rc := renderConfig{format: "unified", writer: &buf}
	diffOpts := pipeline.DiffOptions{
		DisableSizeLimit: true,
		DisableLineLimit: true,
		EnvelopeOpts:     serialize.EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true},
	}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if hadErrors {
		t.Fatal("expected hadErrors to be false")
	}
	if filesRendered != 1 {
		t.Fatalf("expected filesRendered to be 1, got %d", filesRendered)
	}

	if jobs[0].out != nil {
		t.Error("expected rendered output to be released after writing")
	}
	if jobs[0].srcBytes != nil || jobs[0].dstBytes != nil {
		t.Error("expected srcBytes and dstBytes to be nil after job rendering")
	}
}

func TestProcessDiffJob_SkipIdentical(t *testing.T) {
	content := []byte("package main\nfunc main() {}\n")
	job := diffJob{
		displayA: "same.go",
		displayB: "same.go",
		relPathA: "same.go",
		relPathB: "same.go",
		srcBytes: content,
		dstBytes: content,
		done:     make(chan struct{}),
	}

	diffOpts := pipeline.DiffOptions{DisableSizeLimit: true, DisableLineLimit: true}
	processDiffJob(t.Context(), &job, diffOpts, renderConfig{format: "unified"})
	<-job.done

	if job.rendered || job.out != nil {
		t.Error("expected identical file to be skipped in unified format")
	}
}

func TestDefaultWorkerCount(t *testing.T) {
	if n := defaultWorkerCount(); n < 1 {
		t.Fatalf("defaultWorkerCount() = %d, want >= 1", n)
	}
}

func TestRunConcurrentDiffJobs_HeterogeneousStarvation(t *testing.T) {
	const numJobs = 16
	jobs := make([]diffJob, numJobs)
	for i := range jobs {
		name := fmt.Sprintf("file_%02d.go", i)
		jobs[i] = diffJob{
			displayA: name,
			displayB: name,
			relPathA: name,
			relPathB: name,
			load: func() ([]byte, []byte, error) {
				if i == 0 {
					// Simulate heavy AST parse / slow job
					time.Sleep(50 * time.Millisecond)
				}
				src := fmt.Appendf(nil, "package pkg\nfunc F%02d() int { return %d }\n", i, i)
				dst := fmt.Appendf(nil, "package pkg\nfunc F%02d() int { return %d }\n", i, (i+1)*10)
				return src, dst, nil
			},
		}
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format:      "unified",
		showBanner:  true,
		writer:      &buf,
		unifiedOpts: unified.RenderOptions{},
		sbsOpts:     sidebyside.RenderOptions{},
	}
	diffOpts := pipeline.DiffOptions{
		DisableSizeLimit: true,
		DisableLineLimit: true,
		EnvelopeOpts:     serialize.EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true},
	}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if hadErrors {
		t.Fatal("expected hadErrors to be false")
	}
	if filesRendered != numJobs {
		t.Fatalf("expected filesRendered to be %d, got %d", numJobs, filesRendered)
	}

	out := buf.String()
	lastPos := -1
	for i := range numJobs {
		name := fmt.Sprintf("file_%02d.go", i)
		pos := strings.Index(out, name)
		if pos == -1 {
			t.Fatalf("expected output to contain %q, but missing", name)
		}
		if pos < lastPos {
			t.Fatalf("file %q rendered out of order: pos %d < lastPos %d", name, pos, lastPos)
		}
		lastPos = pos
	}
}

func TestRunConcurrentDiffJobs_ErrorPropagation(t *testing.T) {
	jobs := []diffJob{
		{
			displayA: "ok1.go",
			displayB: "ok1.go",
			relPathA: "ok1.go",
			relPathB: "ok1.go",
			srcBytes: []byte("package p\nvar a = 1\n"),
			dstBytes: []byte("package p\nvar a = 2\n"),
		},
		{
			displayA: "broken.go",
			displayB: "broken.go",
			relPathA: "broken.go",
			relPathB: "broken.go",
			load: func() ([]byte, []byte, error) {
				return nil, nil, fmt.Errorf("simulated read failure")
			},
		},
		{
			displayA: "ok2.go",
			displayB: "ok2.go",
			relPathA: "ok2.go",
			relPathB: "ok2.go",
			srcBytes: []byte("package p\nvar b = 1\n"),
			dstBytes: []byte("package p\nvar b = 2\n"),
		},
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format:      "unified",
		showBanner:  true,
		writer:      &buf,
		unifiedOpts: unified.RenderOptions{},
	}
	diffOpts := pipeline.DiffOptions{DisableSizeLimit: true, DisableLineLimit: true}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if !hadErrors {
		t.Error("expected hadErrors to be true when a job fails")
	}
	if filesRendered != 2 {
		t.Fatalf("expected filesRendered to be 2, got %d", filesRendered)
	}

	out := buf.String()
	if !strings.Contains(out, "ok1.go") || !strings.Contains(out, "ok2.go") {
		t.Errorf("expected surviving jobs to render, got:\n%s", out)
	}
}

func TestRunConcurrentDiffJobs_BinaryFilePreservesDisplayPaths(t *testing.T) {
	jobs := []diffJob{
		{
			displayA: "assets/logo.png",
			displayB: "assets/logo.png",
			isBinary: true,
		},
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format: "unified",
		writer: &buf,
	}
	diffOpts := pipeline.DiffOptions{DisableSizeLimit: true, DisableLineLimit: true}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if hadErrors {
		t.Error("expected hadErrors=false for clean binary diff")
	}
	if filesRendered != 1 {
		t.Fatalf("expected 1 file rendered, got %d", filesRendered)
	}

	out := buf.String()
	if !strings.Contains(out, "assets/logo.png") {
		t.Errorf("expected output to contain real file path 'assets/logo.png', got:\n%s", out)
	}
	if strings.Contains(out, "/dev/null") {
		t.Errorf("expected binary diff to NOT contain '/dev/null', got:\n%s", out)
	}
}

func TestRunConcurrentDiffJobs_DeletedFileBannerShowsSourcePath(t *testing.T) {
	jobs := []diffJob{
		{
			displayA: "pkg/old_file.go",
			displayB: "pkg/old_file.go",
			relPathA: "pkg/old_file.go",
			relPathB: "pkg/old_file.go",
			srcBytes: []byte("package pkg\nfunc Old() {}\n"),
			dstBytes: nil, // deleted file
		},
	}

	var buf bytes.Buffer
	rc := renderConfig{
		format:     "side-by-side",
		showBanner: true,
		writer:     &buf,
	}
	diffOpts := pipeline.DiffOptions{DisableSizeLimit: true, DisableLineLimit: true}

	hadErrors, filesRendered := runConcurrentDiffJobs(jobs, nil, rc, diffOpts)
	if hadErrors {
		t.Error("expected hadErrors=false for deleted file diff")
	}
	if filesRendered != 1 {
		t.Fatalf("expected 1 file rendered, got %d", filesRendered)
	}

	out := buf.String()
	if !strings.Contains(out, "pkg/old_file.go") {
		t.Errorf("expected banner to display deleted file path 'pkg/old_file.go', got:\n%s", out)
	}
	if strings.Contains(out, "/dev/null") {
		t.Errorf("expected side-by-side banner to NOT use '/dev/null' as title, got:\n%s", out)
	}
}
