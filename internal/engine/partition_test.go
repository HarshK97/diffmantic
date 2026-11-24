package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestLinePartition(t *testing.T) {
	t.Run("Identical lines classification and CanMatch", func(t *testing.T) {
		srcA := []byte("func main() {\n\tx := 1\n}\n")
		srcB := []byte("func main() {\n\tx := 1\n}\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(srcB, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		n1 := rootA.Children[0]
		n2 := rootB.Children[0]

		if !part.CanMatch(n1, n2) {
			t.Errorf("expected identical subtrees on matching lines to be allowed")
		}
	})

	t.Run("Forbids cross-matching between Group 1 (non-edited) and Group 2 (edited)", func(t *testing.T) {
		srcA := []byte("func main() {\n\tuntouched := 1\n\tdeletedLine()\n}\n")
		srcB := []byte("func main() {\n\tuntouched := 1\n\tinsertedLine()\n}\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(srcB, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		n1, err := testutil.FindNodeByText(rootA, srcA, "untouched := 1", "short_var_declaration")
		if err != nil {
			t.Fatal(err)
		}
		n2, err := testutil.FindNodeByText(rootB, srcB, "insertedLine()", "expression_statement")
		if err != nil {
			t.Fatal(err)
		}

		if part.CanMatch(n1, n2) {
			t.Errorf("expected CanMatch to return false between Group 1 (non-edited) and Group 2 (edited) lines")
		}
	})

	t.Run("Allows matching between Group 2 (edited) and Group 2 (edited) nodes", func(t *testing.T) {
		srcA := []byte("func oldFunc() {\n\tdeletedCode()\n}\n")
		srcB := []byte("func newFunc() {\n\tinsertedCode()\n}\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(srcB, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		n1, err := testutil.FindNodeByText(rootA, srcA, "deletedCode()", "expression_statement")
		if err != nil {
			t.Fatal(err)
		}
		n2, err := testutil.FindNodeByText(rootB, srcB, "insertedCode()", "expression_statement")
		if err != nil {
			t.Fatal(err)
		}

		if !part.CanMatch(n1, n2) {
			t.Errorf("expected CanMatch to return true between two Group 2 (edited) nodes")
		}
	})

	t.Run("Forbids Group 1 matching on non-corresponding line offsets", func(t *testing.T) {
		srcA := []byte("package main\nvar a = 0\nvar b = 1\n")
		srcB := []byte("// header\npackage main\nvar a = 0\nvar b = 1\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(srcB, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		n1, err := testutil.FindNodeByText(rootA, srcA, "package main", "package_clause")
		if err != nil {
			t.Fatal(err)
		}
		n2Correct, err := testutil.FindNodeByText(rootB, srcB, "package main", "package_clause")
		if err != nil {
			t.Fatal(err)
		}
		n2Wrong, err := testutil.FindNodeByText(rootB, srcB, "var a = 0", "var_declaration")
		if err != nil {
			t.Fatal(err)
		}

		if !part.CanMatch(n1, n2Correct) {
			t.Errorf("expected CanMatch to return true for mapped line offsets")
		}
		if part.CanMatch(n1, n2Wrong) {
			t.Errorf("expected CanMatch to return false for non-corresponding Group 1 lines")
		}
	})

	t.Run("Memoization cache and nil handling", func(t *testing.T) {
		srcA := []byte("package main\nvar a = 1\n")
		srcB := []byte("package main\nvar a = 1\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)
		n := rootA.Children[0]

		g1First, startFirst := part.IsGroup1A(n)
		g1Second, startSecond := part.IsGroup1A(n)

		if g1First != g1Second || startFirst != startSecond {
			t.Errorf("cached result mismatch: first=(%v, %d), second=(%v, %d)", g1First, startFirst, g1Second, startSecond)
		}

		var nilPart *LinePartition
		g1Nil, startNil := nilPart.IsGroup1A(n)
		if g1Nil != false || startNil != -1 {
			t.Errorf("expected nil LinePartition to return (false, -1), got (%v, %d)", g1Nil, startNil)
		}

		g1NilNode, startNilNode := part.IsGroup1A(nil)
		if g1NilNode != false || startNilNode != -1 {
			t.Errorf("expected nil ASTNode to return (false, -1), got (%v, %d)", g1NilNode, startNilNode)
		}
	})

	t.Run("Single substantive line adjacent only to bare brace stays in Group 2", func(t *testing.T) {
		srcA := []byte("func f() {\n\tif oldCond {\n\t\toldCall()\n\t}\n\tm.Add(p1, p2)\n\tafterOld()\n}\n")
		srcB := []byte("func f() {\n\tif newCond {\n\t\tnewCall()\n\t}\n\tm.Add(p1, p2)\n\tafterNew()\n}\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		// Line 5 (`m.Add(p1, p2)`) only has line 4 (`\t}`) as a contiguous matched neighbor;
		// because `\t}` is trivial punctuation, line 5 alone should not be locked as Group 1.
		n1, err := testutil.FindNodeByText(rootA, srcA, "m.Add(p1, p2)", "expression_statement")
		if err != nil {
			t.Fatal(err)
		}
		if isG1, _ := part.IsGroup1A(n1); isG1 {
			t.Errorf("expected single substantive line with only a brace neighbor to remain Group 2, got isG1=%v", isG1)
		}
	})

	t.Run("Cross-scope matched lines are excluded from Group 1 via AttachRoots", func(t *testing.T) {
		srcA := []byte("func oldHelper() {\n\tdoWork(a, b)\n\tdoWork(c, d)\n}\nfunc receiver() {\n}\n")
		srcB := []byte("func oldHelper() {\n}\nfunc receiver() {\n\tdoWork(a, b)\n\tdoWork(c, d)\n}\n")

		rootA, err := treesitter.Parse(srcA, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(srcB, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(srcA, srcB)

		// Before attaching roots, line diff matches lines 1..2 in A with lines 3..4 in B as identical runs.
		nodeA, err := testutil.FindNodeByText(rootA, srcA, "doWork(a, b)", "expression_statement")
		if err != nil {
			t.Fatal(err)
		}
		if isG1, _ := part.IsGroup1A(nodeA); !isG1 {
			t.Errorf("expected lines to match as Group 1 before declaration scopes are attached")
		}

		// After attaching roots, the declaration scope mismatch (oldHelper vs receiver) must demote to Group 2.
		part.AttachRoots(rootA, rootB)
		if isG1, _ := part.IsGroup1A(nodeA); isG1 {
			t.Errorf("expected cross-scope lines to be excluded from Group 1 after AttachRoots, got isG1=true")
		}
	})
}
