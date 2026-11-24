package testutil

import (
	"fmt"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type spyTB struct {
	testing.TB
	failed  bool
	fataled bool
	errors  []string
	fatals  []string
}

func (s *spyTB) Helper() {}

func (s *spyTB) Errorf(format string, args ...any) {
	s.failed = true
	s.errors = append(s.errors, fmt.Sprintf(format, args...))
}

func (s *spyTB) Fatalf(format string, args ...any) {
	s.fataled = true
	s.fatals = append(s.fatals, fmt.Sprintf(format, args...))
}

func TestFindNodeByText(t *testing.T) {
	src := []byte(`package main

func example() {
	x := 1 + 2
	y := 1 + 3
	return
}
`)
	tree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	t.Run("find tightest node by unique text", func(t *testing.T) {
		node, err := FindNodeByText(tree, src, "x := 1 + 2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if node.Type != "short_var_declaration" {
			t.Errorf("expected short_var_declaration, got %s", node.Type)
		}
	})

	t.Run("filter by node type", func(t *testing.T) {
		node, err := FindNodeByText(tree, src, "1 + 2", "binary_expression")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if node.Type != "binary_expression" {
			t.Errorf("expected binary_expression, got %s", node.Type)
		}
	})

	t.Run("ambiguous query fails when occurrence is 0", func(t *testing.T) {
		// "1" appears twice (in 1 + 2 and 1 + 3)
		_, err := FindNodeByText(tree, src, "1")
		if err == nil {
			t.Fatal("expected ambiguity error, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous query") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("disambiguate with occurrence", func(t *testing.T) {
		first, err := FindNodeByTextQuery(tree, src, TextQuery{Text: "1", Occurrence: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := FindNodeByTextQuery(tree, src, TextQuery{Text: "1", Occurrence: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if first == second {
			t.Errorf("expected different nodes for occurrences 1 and 2")
		}
	})

	t.Run("text not found returns error", func(t *testing.T) {
		_, err := FindNodeByText(tree, src, "nonexistent")
		if err == nil {
			t.Fatal("expected error for non-existent text, got nil")
		}
	})
}

func TestAssertMappedText(t *testing.T) {
	src := []byte(`package main
func run() {
	compute()
}
`)
	dst := []byte(`package main
func run() {
	computeFast()
}
`)
	srcTree, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcCall, err := FindNodeByText(srcTree, src, "compute()")
	if err != nil {
		t.Fatalf("find srcCall: %v", err)
	}
	dstCall, err := FindNodeByText(dstTree, dst, "computeFast()")
	if err != nil {
		t.Fatalf("find dstCall: %v", err)
	}

	t.Run("succeeds when mapped", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcCall: dstCall,
		}
		AssertMappedText(spy, srcTree, dstTree, src, dst, mappings, "compute()", "computeFast()")
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("fails when not mapped", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcCall: nil,
		}
		AssertMappedText(spy, srcTree, dstTree, src, dst, mappings, "compute()", "computeFast()")
		if !spy.failed {
			t.Error("expected failure when nodes are not mapped")
		}
	})

	t.Run("AssertNotMappedText succeeds when different", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcCall: nil,
		}
		AssertNotMappedText(spy, srcTree, dstTree, src, dst, mappings, "compute()", "computeFast()")
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("AssertUnmappedText succeeds when unmapped", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{}
		AssertUnmappedText(spy, srcTree, src, mappings, "compute()")
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure: %v %v", spy.errors, spy.fatals)
		}
	})
}
