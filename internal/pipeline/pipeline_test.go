package pipeline

import (
	"strings"
	"testing"
)

func TestPipeline_LineLimitFallback(t *testing.T) {
	// Build a small Go file with 12 lines
	var lines []string
	for i := 0; i < 12; i++ {
		lines = append(lines, "package main")
	}
	content := []byte(strings.Join(lines, "\n"))

	// With maxLines=10 and 12-line file, it should fall back to line diff (no AST).
	res, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SrcAST != nil || res.DstAST != nil {
		t.Errorf("expected AST nil on line limit fallback, got non-nil")
	}

	// With DisableLineLimit=true, AST should parse regardless.
	resUnlimited, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines:  10,
		DisableLineLimit: true,
	})
	if err != nil {
		t.Fatalf("unexpected error with disabled limit: %v", err)
	}
	if resUnlimited.SrcAST == nil {
		t.Errorf("expected AST parsed when line limit disabled")
	}

	// With higher limit of 20, file has fewer lines - should parse as AST.
	resHigher, err := Run(content, content, "main.go", "main.go", DiffOptions{
		MaxASTFileLines: 20,
	})
	if err != nil {
		t.Fatalf("unexpected error with higher limit: %v", err)
	}
	if resHigher.SrcAST == nil {
		t.Errorf("expected AST parsed when file is under line limit")
	}
}

func TestPipeline_UnchangedCommentsRetainedOnLineShift(t *testing.T) {
	src := []byte(`package main

// Foo does foo things.
func Foo() {
	// Step 1: initialize
	x := 1
	_ = x
}

// Bar does bar things.
func Bar() {
	// Step 2: finalize
	y := 2
	_ = y
}
`)

	// dst inserts a 30-line function between Foo and Bar to trigger line distance > 25
	var filler []string
	filler = append(filler, "// NewFunction does extra work.", "func NewFunction() {")
	for i := 0; i < 28; i++ {
		filler = append(filler, "\tz := 0\n\t_ = z")
	}
	filler = append(filler, "}")

	dst := []byte(`package main

// Foo does foo things.
func Foo() {
	// Step 1: initialize
	x := 1
	_ = x
}

` + strings.Join(filler, "\n") + `

// Bar does bar things.
func Bar() {
	// Step 2: finalize
	y := 2
	_ = y
}
`)

	res, err := Run(src, dst, "a.go", "b.go", DiffOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify that comments in Foo and Bar are NOT deleted or inserted.
	for _, act := range res.Envelope.Actions {
		if act.Node != nil && act.Node.Type == "comment" {
			if strings.Contains(act.Node.Label, "Foo does foo") ||
				strings.Contains(act.Node.Label, "Step 1") ||
				strings.Contains(act.Node.Label, "Bar does bar") ||
				strings.Contains(act.Node.Label, "Step 2") {
				t.Errorf("spurious comment action %s on %q", act.Action, act.Node.Label)
			}
		}
	}
}
