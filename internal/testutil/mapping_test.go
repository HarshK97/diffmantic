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

func TestFindNode(t *testing.T) {
	src := `package main

func example() {
	x := 1 + 2
	return
}
`
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	t.Run("0 matches - line out of range", func(t *testing.T) {
		_, err := FindNode(tree, NodeQuery{Type: "return_statement", Line: 10})
		if err == nil {
			t.Fatal("expected error for line out of range, got nil")
		}
		if !strings.Contains(err.Error(), "no node of type") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("0 matches - wrong type on line", func(t *testing.T) {
		_, err := FindNode(tree, NodeQuery{Type: "for_statement", Line: 4})
		if err == nil {
			t.Fatal("expected error for non-existent node type on line, got nil")
		}
		if !strings.Contains(err.Error(), "no node of type") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ambiguous matches without Nth", func(t *testing.T) {
		// Line 4 has two int_literal nodes: 1 and 2
		_, err := FindNode(tree, NodeQuery{Type: "int_literal", Line: 4})
		if err == nil {
			t.Fatal("expected ambiguity error when Nth is 0 and multiple nodes exist, got nil")
		}
		if !strings.Contains(err.Error(), "ambiguous query") {
			t.Errorf("expected ambiguous query error, got: %v", err)
		}
	})

	t.Run("disambiguation with Nth", func(t *testing.T) {
		first, err := FindNode(tree, NodeQuery{Type: "int_literal", Line: 4, Nth: 1})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if first.Label != "1" {
			t.Errorf("expected label 1, got %q", first.Label)
		}

		second, err := FindNode(tree, NodeQuery{Type: "int_literal", Line: 4, Nth: 2})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if second.Label != "2" {
			t.Errorf("expected label 2, got %q", second.Label)
		}
	})

	t.Run("Nth out of range", func(t *testing.T) {
		_, err := FindNode(tree, NodeQuery{Type: "int_literal", Line: 4, Nth: 3})
		if err == nil {
			t.Fatal("expected index out of range error, got nil")
		}
		if !strings.Contains(err.Error(), "index out of range") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("unique match defaults to Nth=1 when Nth is 0", func(t *testing.T) {
		node, err := FindNode(tree, NodeQuery{Type: "return_statement", Line: 5})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if node.Type != "return_statement" {
			t.Errorf("expected return_statement, got %s", node.Type)
		}
	})

	t.Run("invalid arguments", func(t *testing.T) {
		if _, err := FindNode(nil, NodeQuery{Type: "return_statement", Line: 1}); err == nil {
			t.Error("expected error for nil tree")
		}
		if _, err := FindNode(tree, NodeQuery{Type: "", Line: 1}); err == nil {
			t.Error("expected error for empty Type")
		}
		if _, err := FindNode(tree, NodeQuery{Type: "return_statement", Line: 0}); err == nil {
			t.Error("expected error for Line < 1")
		}
		if _, err := FindNode(tree, NodeQuery{Type: "return_statement", Line: 1, Nth: -1}); err == nil {
			t.Error("expected error for Nth < 0")
		}
	})
}

func TestAssertMapped(t *testing.T) {
	src := `package main

func foo() {
	return
}
`
	dst := `package main

func foo() {
	return
}
`
	srcTree, err := treesitter.Parse([]byte(src), "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcRet, err := FindNode(srcTree, NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find srcRet: %v", err)
	}
	dstRet, err := FindNode(dstTree, NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find dstRet: %v", err)
	}

	t.Run("successful mapping", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcRet: dstRet,
		}
		AssertMapped(spy, srcTree, dstTree, mappings,
			NodeQuery{Type: "return_statement", Line: 4},
			NodeQuery{Type: "return_statement", Line: 4},
		)
		if spy.failed || spy.fataled {
			t.Errorf("AssertMapped reported failure on valid mapping: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("wrong mapping", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcRet: nil,
		}
		AssertMapped(spy, srcTree, dstTree, mappings,
			NodeQuery{Type: "return_statement", Line: 4},
			NodeQuery{Type: "return_statement", Line: 4},
		)
		if !spy.failed {
			t.Error("expected AssertMapped to report error for wrong mapping, but it did not")
		}
	})

	t.Run("node not found causes fatal", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{}
		AssertMapped(spy, srcTree, dstTree, mappings,
			NodeQuery{Type: "nonexistent", Line: 4},
			NodeQuery{Type: "return_statement", Line: 4},
		)
		if !spy.fataled {
			t.Error("expected AssertMapped to fatal when src node is not found")
		}
	})
}

func TestAssertUnmapped(t *testing.T) {
	src := `package main

func foo() {
	return
}
`
	tree, err := treesitter.Parse([]byte(src), "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}

	retNode, err := FindNode(tree, NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find retNode: %v", err)
	}

	t.Run("unmapped when absent from map", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{}
		AssertUnmapped(spy, tree, mappings, NodeQuery{Type: "return_statement", Line: 4})
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure for unmapped node: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("unmapped when mapped to nil", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			retNode: nil,
		}
		AssertUnmapped(spy, tree, mappings, NodeQuery{Type: "return_statement", Line: 4})
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure for node mapped to nil: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("fails when mapped to non-nil", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			retNode: retNode,
		}
		AssertUnmapped(spy, tree, mappings, NodeQuery{Type: "return_statement", Line: 4})
		if !spy.failed {
			t.Error("expected AssertUnmapped to fail when node is mapped to non-nil")
		}
	})
}

func TestAssertNotMapped(t *testing.T) {
	src := `package main

func foo() {
	return
}
`
	dst := `package main

func foo() {
	return
}
`
	srcTree, err := treesitter.Parse([]byte(src), "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse([]byte(dst), "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcRet, err := FindNode(srcTree, NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find srcRet: %v", err)
	}
	dstRet, err := FindNode(dstTree, NodeQuery{Type: "return_statement", Line: 4})
	if err != nil {
		t.Fatalf("find dstRet: %v", err)
	}

	t.Run("succeeds when not mapped to dst", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcRet: nil,
		}
		AssertNotMapped(spy, srcTree, dstTree, mappings,
			NodeQuery{Type: "return_statement", Line: 4},
			NodeQuery{Type: "return_statement", Line: 4},
		)
		if spy.failed || spy.fataled {
			t.Errorf("unexpected failure when nodes are not mapped: %v %v", spy.errors, spy.fatals)
		}
	})

	t.Run("fails when mapped to dst", func(t *testing.T) {
		spy := &spyTB{TB: t}
		mappings := map[*treesitter.ASTNode]*treesitter.ASTNode{
			srcRet: dstRet,
		}
		AssertNotMapped(spy, srcTree, dstTree, mappings,
			NodeQuery{Type: "return_statement", Line: 4},
			NodeQuery{Type: "return_statement", Line: 4},
		)
		if !spy.failed {
			t.Error("expected AssertNotMapped to fail when nodes are mapped to each other")
		}
	})
}
