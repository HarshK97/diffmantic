package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

func TestProcessDirectoryJob(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	fileA := filepath.Join(dirA, "calc.go")
	fileB := filepath.Join(dirB, "calc.go")

	if err := os.WriteFile(fileA, []byte("package calc\n\nfunc Add(a, b int) int { return a }\n"), 0o644); err != nil {
		t.Fatalf("writing fileA: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("package calc\n\nfunc Add(a, b int) int { return a + b }\n"), 0o644); err != nil {
		t.Fatalf("writing fileB: %v", err)
	}

	job := diffJob{
		cf: changedFile{
			relPath: "calc.go",
			pathA:   fileA,
			pathB:   fileB,
		},
		done: make(chan struct{}),
	}

	ctx := t.Context()
	opts := serialize.EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true}
	processDirectoryJob(ctx, &job, "side-by-side" /* parseErrorLimit */, 0 /* sizeLimitKB */, 1024 /* lineLimitLines */, 10000 /* ignoreComments */, false, opts)

	<-job.done

	if job.err != nil {
		t.Fatalf("unexpected job error: %v", job.err)
	}
	if job.dr == nil {
		t.Fatal("expected non-nil DiffResult")
	}
	if job.dr.SrcAST != nil || job.dr.DstAST != nil {
		t.Error("expected SrcAST and DstAST to be pruned to nil")
	}
	if job.dr.MatchResult != nil {
		t.Error("expected MatchResult to be pruned to nil for non-actions format")
	}
	if len(job.srcBytes) == 0 || len(job.dstBytes) == 0 {
		t.Error("expected non-empty srcBytes and dstBytes")
	}
}

func TestProcessDirectoryJob_CanceledContext(t *testing.T) {
	dir := t.TempDir()
	fileA := filepath.Join(dir, "a.go")
	if err := os.WriteFile(fileA, []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("writing file: %v", err)
	}

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	job := diffJob{
		cf: changedFile{
			relPath: "a.go",
			pathA:   fileA,
			pathB:   fileA,
		},
		done: make(chan struct{}),
	}

	opts := serialize.EnvelopeOptions{IncludeActions: true}
	processDirectoryJob(ctx, &job, "unified" /* parseErrorLimit */, 0 /* sizeLimitKB */, 1024 /* lineLimitLines */, 10000 /* ignoreComments */, false, opts)

	<-job.done

	if job.dr != nil {
		t.Error("expected DiffResult to be nil when context is canceled")
	}
}

func TestRunDirectoryDiff_ConcurrentOrdering(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	const numFiles = 15
	for i := 1; i <= numFiles; i++ {
		name := fmt.Sprintf("file_%02d.go", i)
		srcContent := fmt.Sprintf("package pkg\n\nfunc F%02d() int {\n\treturn %d\n}\n", i, i)
		dstContent := fmt.Sprintf("package pkg\n\nfunc F%02d() int {\n\treturn %d\n}\n", i, i*10)

		if err := os.WriteFile(filepath.Join(dirA, name), []byte(srcContent), 0o644); err != nil {
			t.Fatalf("writing file to dirA: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dirB, name), []byte(dstContent), 0o644); err != nil {
			t.Fatalf("writing file to dirB: %v", err)
		}
	}

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = buf.ReadFrom(r)
	}()

	runDirectoryDiff(rootCmd, dirA, dirB, "unified" /* ignoreComments */, false /* parseErrorLimit */, 0 /* sizeLimitKB */, 1024 /* lineLimitLines */, 10000 /* noPager */, true)

	_ = w.Close()
	<-readDone
	os.Stdout = oldStdout

	output := buf.String()

	lastPos := -1
	for i := 1; i <= numFiles; i++ {
		name := fmt.Sprintf("file_%02d.go", i)
		pos := strings.Index(output, name)
		if pos == -1 {
			t.Fatalf("expected output to contain %q, but it was missing", name)
		}
		if pos < lastPos {
			t.Fatalf("file %q appeared out of order: pos %d < lastPos %d", name, pos, lastPos)
		}
		lastPos = pos
	}
}

// TestRunDirectoryDiff_HeterogeneousWorkload tests that a slow, heavy file doesn't stall workers or scramble output ordering.
func TestRunDirectoryDiff_HeterogeneousWorkload(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()

	const numFiles = 20

	// Heavy file with 60 functions to keep one worker busy.
	var heavySrc, heavyDst strings.Builder
	heavySrc.WriteString("package pkg\n\n")
	heavyDst.WriteString("package pkg\n\n")
	for f := 1; f <= 60; f++ {
		fmt.Fprintf(&heavySrc, "func ComputeHeavy%02d(x, y int) int {\n\tif x > y {\n\t\treturn x - y\n\t}\n\treturn y - x\n}\n\n", f)
		fmt.Fprintf(&heavyDst, "func ComputeHeavy%02d(x, y int) int {\n\tif x >= y {\n\t\treturn x * y\n\t}\n\treturn y + x\n}\n\n", f)
	}
	if err := os.WriteFile(filepath.Join(dirA, "file_01.go"), []byte(heavySrc.String()), 0o644); err != nil {
		t.Fatalf("writing heavy file to dirA: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "file_01.go"), []byte(heavyDst.String()), 0o644); err != nil {
		t.Fatalf("writing heavy file to dirB: %v", err)
	}

	for i := 2; i <= numFiles; i++ {
		name := fmt.Sprintf("file_%02d.go", i)
		src := fmt.Sprintf("package pkg\nfunc F%02d() int { return %d }\n", i, i)
		dst := fmt.Sprintf("package pkg\nfunc F%02d() int { return %d }\n", i, i+50)

		if err := os.WriteFile(filepath.Join(dirA, name), []byte(src), 0o644); err != nil {
			t.Fatalf("writing file to dirA: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dirB, name), []byte(dst), 0o644); err != nil {
			t.Fatalf("writing file to dirB: %v", err)
		}
	}

	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	os.Stdout = w

	var buf bytes.Buffer
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = buf.ReadFrom(r)
	}()

	hadErrors := runDirectoryDiff(rootCmd, dirA, dirB, "unified" /* ignoreComments */, false /* parseErrorLimit */, 0 /* sizeLimitKB */, 1024 /* lineLimitLines */, 10000 /* noPager */, true)

	_ = w.Close()
	<-readDone
	os.Stdout = oldStdout

	if hadErrors {
		t.Errorf("expected runDirectoryDiff to succeed without errors")
	}

	output := buf.String()

	lastPos := -1
	for i := 1; i <= numFiles; i++ {
		name := fmt.Sprintf("file_%02d.go", i)
		pos := strings.Index(output, name)
		if pos == -1 {
			t.Fatalf("expected output to contain %q, but was missing", name)
		}
		if pos < lastPos {
			t.Fatalf("file %q appeared out of order: pos %d < lastPos %d", name, pos, lastPos)
		}
		lastPos = pos
	}
}

// Non-existent directories should return an error status rather than calling os.Exit.
func TestRunDirectoryDiff_ErrorPropagation(t *testing.T) {
	hadErrors := runDirectoryDiff(rootCmd, "/non/existent/path/a", "/non/existent/path/b", "unified" /* ignoreComments */, false /* parseErrorLimit */, 0 /* sizeLimitKB */, 1024 /* lineLimitLines */, 10000 /* noPager */, true)
	if !hadErrors {
		t.Error("expected runDirectoryDiff to return true (hadErrors) for non-existent directories")
	}
}
