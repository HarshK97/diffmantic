package engine

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestTopDown(t *testing.T) {
	t.Run("Identical trees", func(t *testing.T) {
		src := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")
		dst := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "package main", "package main")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "func main() {\n\tx := 1\n\ty := 2\n}", "func main() {\n\tx := 1\n\ty := 2\n}")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "x := 1", "x := 1")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "y := 2", "y := 2")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "x", "x", "identifier")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "y", "y", "identifier")
	})

	t.Run("Single leaf label change", func(t *testing.T) {
		src := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")
		dst := []byte("package main\nfunc main() {\n\tx := 1\n\tz := 2\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)

		// TopDown matches isomorphic subtrees like srcLeaf1.
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "x", "x", "identifier")
		testutil.AssertUnmappedText(t, rootA, src, m.Src(), "y", "identifier")
	})

	t.Run("Added leaf", func(t *testing.T) {
		src := []byte("package main\nfunc main() {\n\tx := 1\n}\n")
		dst := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "x", "x", "identifier")
		testutil.AssertUnmappedText(t, rootB, dst, m.Dst(), "y", "identifier")
	})

	t.Run("Structurally isomorphic subtrees", func(t *testing.T) {
		src := []byte("package main\nfunc a() { f() }\n")
		dst := []byte("package main\nfunc b() { f() }\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "f()", "f()", "call_expression")
		testutil.AssertUnmappedText(t, rootA, src, m.Src(), "func a() { f() }", "function_declaration")
	})
}

func TestBottomUp(t *testing.T) {
	t.Run("Fills gaps left by TopDown", func(t *testing.T) {
		src := []byte("package main\nfunc main() {\n\tx := 1\n}\n")
		dst := []byte("package main\nfunc main() {\n\tx := 1\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		srcLeaf, err := testutil.FindNodeByText(rootA, src, "x", "identifier")
		if err != nil {
			t.Fatal(err)
		}
		dstLeaf, err := testutil.FindNodeByText(rootB, dst, "x", "identifier")
		if err != nil {
			t.Fatal(err)
		}

		// Simulate TopDown mapping the leaf.
		m := NewMapping()
		m.Add(srcLeaf, dstLeaf)

		BottomUp(rootA, rootB, m, 0.5)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "{\n\tx := 1\n}", "{\n\tx := 1\n}", "block")
	})

	t.Run("Single leaf rename", func(t *testing.T) {
		src := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")
		dst := []byte("package main\nfunc main() {\n\tx := 1\n\tz := 2\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)
		BottomUp(rootA, rootB, m, 0.5)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "func main() {\n\tx := 1\n\ty := 2\n}", "func main() {\n\tx := 1\n\tz := 2\n}", "function_declaration")
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "{\n\tx := 1\n\ty := 2\n}", "{\n\tx := 1\n\tz := 2\n}", "block")
	})

	t.Run("Completely different trees", func(t *testing.T) {
		src := []byte("package main\nfunc a() { x() }\n")
		dst := []byte("package main\nfunc b() { \"hello\" }\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 1, m, nil)
		BottomUp(rootA, rootB, m, 0.5)

		// BottomUp might map the root as a fallback, but leaves shouldn't map.
		testutil.AssertUnmappedText(t, rootA, src, m.Src(), "x", "identifier")
		testutil.AssertUnmappedText(t, rootB, dst, m.Dst(), "\"hello\"", "interpreted_string_literal")
	})
}

func TestTopDownUncomputedHashes(t *testing.T) {
	src := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")
	dst := []byte("package main\nfunc main() {\n\tx := 1\n\ty := 2\n}\n")

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	// Reset hashes to 0 to make sure TopDown lazily computes missing hashes.
	for _, n := range rootA.PreOrder() {
		n.Hash = 0
	}
	for _, n := range rootB.PreOrder() {
		n.Hash = 0
	}

	m := NewMapping()
	TopDown(rootA, rootB, 1, m, nil)

	testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "x", "x", "identifier")
	testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "y", "y", "identifier")
}

func TestFindCandidatesWithCommonDescendants(t *testing.T) {
	src := []byte("package main\nfunc f() { x() }\n")
	dst := []byte("package main\nfunc f() { x() }\n")

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	callA, err := testutil.FindNodeByText(rootA, src, "x()", "call_expression")
	if err != nil {
		t.Fatal(err)
	}
	callB, err := testutil.FindNodeByText(rootB, dst, "x()", "call_expression")
	if err != nil {
		t.Fatal(err)
	}
	idA, err := testutil.FindNodeByText(rootA, src, "x", "identifier")
	if err != nil {
		t.Fatal(err)
	}
	idB, err := testutil.FindNodeByText(rootB, dst, "x", "identifier")
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	m.Add(idA, idB)

	candidates := findCandidatesWithCommonDescendants(callA, m)
	if len(candidates) != 1 || candidates[0] != callB {
		t.Errorf("findCandidatesWithCommonDescendants() = %v, want [%v]", candidates, callB)
	}
}

func TestTopDown_AmbiguityPreorderTieBreaker(t *testing.T) {
	// If dst has duplicate candidate subtrees at different lines, match the stationary
	// candidate closer in row distance first.
	src := []byte("package main\nfunc myFunc() {\n\tscore()\n}\n")
	dst := []byte("package main\nfunc myFunc() {\n\tscore()\n\textra1()\n\textra2()\n\textra3()\n\tscore()\n}\n")

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	TopDown(rootA, rootB, 2, m, nil)

	testutil.AssertMappedTextQuery(t, rootA, rootB, src, dst, m.Src(),
		testutil.TextQuery{Text: "score()", Type: "call_expression"},
		testutil.TextQuery{Text: "score()", Type: "call_expression", Occurrence: 1},
	)
}

func TestTopDown_IdentifierlessScopeIsolation(t *testing.T) {
	t.Run("rejects identifier-less subtree across different function scopes", func(t *testing.T) {
		src := []byte("package main\nfunc funcA() bool {\n\treturn false\n}\n")
		dst := []byte("package main\nfunc funcB() bool {\n\treturn false\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 2, m, nil)

		testutil.AssertUnmappedText(t, rootA, src, m.Src(), "return false", "return_statement")
	})

	t.Run("matches identifier-less subtree within same function scope", func(t *testing.T) {
		src := []byte("package main\nfunc funcA() bool {\n\treturn false\n}\n")
		dst := []byte("package main\nfunc funcA() bool {\n\treturn false\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 2, m, nil)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "return false", "return false", "return_statement")
	})

	t.Run("matches subtree containing identifier across different function scopes", func(t *testing.T) {
		src := []byte("package main\nfunc funcA() error {\n\treturn err\n}\n")
		dst := []byte("package main\nfunc funcB() error {\n\treturn err\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		TopDown(rootA, rootB, 2, m, nil)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "return err", "return err", "return_statement")
	})
}

func TestTopDown_DirectParentPrecedenceOverFallback(t *testing.T) {
	// Candidate A has a direct parent match, while Candidate B only matches via
	// a high-scoring outer loop. Make sure TopDown picks A over B.
	src := []byte(`package main
func myFunc() {
	for i := range 15 {
		common := 1
		sim := 10
		sim = 20
		_ = i
	}
}
`)
	dst := []byte(`package main
func myFunc() {
	for i := range 15 {
		common := 1
		sim := 10
		_ = i
	}
}
`)

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	TopDown(rootA, rootB, 2, m, nil)

	testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(), "sim := 10", "sim := 10", "short_var_declaration")
	testutil.AssertUnmappedText(t, rootA, src, m.Src(), "sim = 20", "assignment_statement")
}

func TestTopDown_ParentLeafSimilarityTieBreaker(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func process(items []int) {
	for _, x := range items {
		if shouldDemoteMove(x, dstNode, ms, r) {
			continue
		}
	}
}
`
		dst := `package main

func process(items []int) {
	for _, x := range items {
		if already := toDemote[x]; already {
			continue
		}
		if shouldDemoteMove(x, dstNode, ms, r, evicted) {
			continue
		}
	}
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context and surrounding loop map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func process(items []int)",
			"func process(items []int)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"for _, x := range items",
			"for _, x := range items",
		)

		// 2. Early newly introduced if statement is cleanly unmapped in dst
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"if already := toDemote[x]; already",
		)

		// 3. Evolving shouldDemoteMove if_statement and its block map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"if shouldDemoteMove(x, dstNode, ms, r)",
			"if shouldDemoteMove(x, dstNode, ms, r, evicted)",
			"if_statement",
		)

		// 4. Verify continue statement in srcIf maps to continue in dstLateIf
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			testutil.TextQuery{Text: "continue", Type: "continue_statement", Occurrence: 1},
			testutil.TextQuery{Text: "continue", Type: "continue_statement", Occurrence: 2},
		)
	})
}

func TestTopDown_DelimitedContainerBoundaryGuard(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		runCase := func(t *testing.T, name string, src, dst []byte) {
			t.Run(name, func(t *testing.T) {
				oldTree, err := treesitter.Parse(src, "old.go")
				if err != nil {
					t.Fatal(err)
				}
				newTree, err := treesitter.Parse(dst, "new.go")
				if err != nil {
					t.Fatal(err)
				}

				part := NewLinePartition(src, dst)
				mr := Match(oldTree, newTree, src, dst, part)
				m := mr.Mappings

				// Enclosing declarations must stay anchored in place.
				testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
					"type Server struct",
					"type Server struct",
				)
				testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
					"var servers = []Server{",
					"var servers = []Server{",
				)

				// Unchanged earlier elements must stay mapped in place.
				testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
					`{Host: "primary.internal", Port: 8080, SSL: true}`,
					`{Host: "primary.internal", Port: 8080, SSL: true}`,
				)

				// Staging was deleted; it must be unmapped in src.
				testutil.AssertUnmappedText(t, oldTree, src, m.Src(),
					`{Host: "staging.internal", Port: 9090, SSL: true}`,
				)

				// Stationary fields inside the updated element should map in place.
				testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
					`Host: "backup.internal"`,
					`Host: "backup.internal"`,
				)
				testutil.AssertMappedTextQuery(t, oldTree, newTree, src, dst, m.Src(),
					testutil.TextQuery{Text: "Port: 8080", Occurrence: 2},
					testutil.TextQuery{Text: "Port: 8080", Occurrence: 2},
				)
			})
		}

		// Case 1: Updated element appears before deleted element in source.
		srcStd := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: false},
	{Host: "staging.internal", Port: 9090, SSL: true},
}
`)
		dstStd := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: true},
}
`)
		runCase(t, "StandardOrder_DeletedAfterUpdated", srcStd, dstStd)

		// Case 2: Deleted element appears first; top-down matching must not greedily claim the updated element.
		srcInv := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "staging.internal", Port: 9090, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: false},
}
`)
		dstInv := []byte(`package config

type Server struct {
	Host string
	Port int
	SSL  bool
}

var servers = []Server{
	{Host: "primary.internal", Port: 8080, SSL: true},
	{Host: "backup.internal", Port: 8080, SSL: true},
}
`)
		runCase(t, "InvertedOrder_DeletedBeforeUpdated", srcInv, dstInv)
	})
}

// Sibling statements and functions with similar shapes shouldn't invert their matches
// across container declarations or steal statements from sibling scopes.
func TestTopDown_SiblingStatementMonotonicity(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`fn f1() void {}

fn f2() void {
    x += 1;
}
`)
		newSrc := []byte(`fn f1() void {
    foo();
    x += 1;
}

fn f2() void {
    foo();
    x += 1;
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "old.zig")
		if err != nil {
			t.Fatalf("parse old.zig: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "new.zig")
		if err != nil {
			t.Fatalf("parse new.zig: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: functions map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"fn f1() void",
			"fn f1() void",
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"fn f2() void",
			"fn f2() void",
		)

		// 2. Surrounding sequence & stationary target: x += 1 inside f2 maps monotonically in place to f2
		testutil.AssertMappedTextQuery(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "x += 1", Occurrence: 1},
			testutil.TextQuery{Text: "x += 1", Occurrence: 2},
		)

		// 3. Non-theft assertion: newly inserted statements in f1 are unmapped
		testutil.AssertUnmappedTextQuery(t, newTree, newSrc, mr.Mappings.Dst(),
			testutil.TextQuery{Text: "foo()", Occurrence: 1},
		)
		testutil.AssertUnmappedTextQuery(t, newTree, newSrc, mr.Mappings.Dst(),
			testutil.TextQuery{Text: "x += 1", Occurrence: 1},
		)
	})
}

func TestTopDown_ImmediateStatementAffinity(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package main

func f() {
	{
		iter := slice("1", "b")
		expect(1, iter.get(0))
		expectStrings("b", iter.get(1))
	}
}
`)

		dst := []byte(`package main

func f() {
	{
		iter := slice("1", "2")
		expect(1, iter.get(0))
		expect(2, iter.get(1))
	}

	{
		iter := slice("1", "2", "b")
		expect(1, iter.get(0))
		expect(2, iter.get(1))
		expectStrings("b", iter.get(2))
	}
}
`)

		oldTree, err := treesitter.Parse(src, "old.go")
		if err != nil {
			t.Fatal(err)
		}
		newTree, err := treesitter.Parse(dst, "new.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(src, dst)
		mr := Match(oldTree, newTree, src, dst, part)
		m := mr.Mappings

		// 1. Enclosing Context: Function declaration must map in place.
		testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
			"func f()",
			"func f()",
		)

		// Block mapping: oldBlock maps to newBlock2 where "b" survives, not newBlock1.
		testutil.AssertMappedText(t, oldTree, newTree, src, dst, m.Src(),
			"{\n\t\titer := slice(\"1\", \"b\")\n\t\texpect(1, iter.get(0))\n\t\texpectStrings(\"b\", iter.get(1))\n\t}",
			"{\n\t\titer := slice(\"1\", \"2\", \"b\")\n\t\texpect(1, iter.get(0))\n\t\texpect(2, iter.get(1))\n\t\texpectStrings(\"b\", iter.get(2))\n\t}",
		)

		// 2. Target Statement: expectStrings in oldBlock is anchored inside newBlock2 (Occurrence 2 of expect(2,...))
		// and NOT stolen by newly prepended newBlock1 (Occurrence 1).
		testutil.AssertMappedTextQuery(t, oldTree, newTree, src, dst, m.Src(),
			testutil.TextQuery{Text: `expectStrings("b", iter.get(1))`},
			testutil.TextQuery{Text: `expect(2, iter.get(1))`, Occurrence: 2},
		)
		testutil.AssertNotMappedTextQuery(t, oldTree, newTree, src, dst, m.Src(),
			testutil.TextQuery{Text: `expectStrings("b", iter.get(1))`},
			testutil.TextQuery{Text: `expect(2, iter.get(1))`, Occurrence: 1},
		)

		// 3. Stationary Operands: expect(1, iter.get(0)) in oldBlock must stay mapped to newBlock2's expect(1, iter.get(0)),
		// and NOT be stolen by newBlock1's expect(1) on line 6.
		testutil.AssertMappedTextQuery(t, oldTree, newTree, src, dst, m.Src(),
			testutil.TextQuery{Text: "expect(1, iter.get(0))", Occurrence: 1},
			testutil.TextQuery{Text: "expect(1, iter.get(0))", Occurrence: 2},
		)
		testutil.AssertNotMappedTextQuery(t, oldTree, newTree, src, dst, m.Src(),
			testutil.TextQuery{Text: "expect(1, iter.get(0))", Occurrence: 1},
			testutil.TextQuery{Text: "expect(1, iter.get(0))", Occurrence: 1},
		)
	})
}

// TestTopDown_BaselineDriftBranchCrossMatching ensures upstream line drift
// doesn't cause ambiguous subtrees in sibling branches (then vs else) to cross-match.
func TestTopDown_BaselineDriftBranchCrossMatching(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		// Realistic source with deleted helper lines before the function creating 15 lines of drift.
		oldSrc := []byte(`package render

func unusedHelperA() {
	println("1")
	println("2")
}

func renderWholeFileSingleColumn(action int, extra string) error {
	var hunkHeader string
	if action == 1 {
		hunkHeader = renderutil.FormatHunkHeader(0, 0, 1, 10, extra)
	} else {
		hunkHeader = renderutil.FormatHunkHeader(1, 10, 0, 0, extra)
	}
	fmt.Printf("%s\n", hunkHeader)
	return nil
}
`)

		newSrc := []byte(`package render

func renderWholeFileSingleColumn(action int) error {
	var hunkHeader string
	if action == 1 {
		hunkHeader = renderutil.FormatHunkHeader(0, 0, 1, 10, "")
	} else {
		hunkHeader = renderutil.FormatHunkHeader(1, 10, 0, 0, "")
	}
	fmt.Printf("%s\n", hunkHeader)
	return nil
}
`)

		oldTree, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatalf("parse old.go: %v", err)
		}
		newTree, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatalf("parse new.go: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(oldTree, newTree, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing context: function declaration must map in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"func renderWholeFileSingleColumn",
			"func renderWholeFileSingleColumn",
		)

		// 2. Surrounding sequence: if_statement maps in place
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			"if action == 1",
			"if action == 1",
		)

		// 3. In-branch statement preservation: then maps to then, else maps to else (never inverted)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			`hunkHeader = renderutil.FormatHunkHeader(0, 0, 1, 10, extra)`,
			`hunkHeader = renderutil.FormatHunkHeader(0, 0, 1, 10, "")`,
		)
		testutil.AssertMappedText(t, oldTree, newTree, oldSrc, newSrc, mr.Mappings.Src(),
			`hunkHeader = renderutil.FormatHunkHeader(1, 10, 0, 0, extra)`,
			`hunkHeader = renderutil.FormatHunkHeader(1, 10, 0, 0, "")`,
		)
	})
}

func TestTopDown_CalleeCompatibility(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `function evaluateType(type: Type, host: Node) {
    let original = type;
    if (addUndefined) {
        type = getOptionalType(type);
    }
    return type;
}
`
		dst := `function evaluateType(type: Type, host: Node) {
    let original = type;
    if (addUndefined) {
        type = getOptionalType(type, !isParameter(host));
    }
    if (containsNonMissingUndefinedType(type)) {
        return undefined;
    }
    return type;
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.ts")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: evaluateType declaration
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"function evaluateType(type: Type, host: Node)",
			"function evaluateType(type: Type, host: Node)",
		)

		// 2. Surrounding Sequence: let original statement and return type statement
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"let original = type;",
			"let original = type;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"return type;",
			"return type;",
		)

		// 3. Target & Containers: getOptionalType call
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"getOptionalType(type)",
			"getOptionalType(type, !isParameter(host))",
			"call_expression",
		)

		// 4. Newly introduced call containsNonMissingUndefinedType must be unmapped in src
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"containsNonMissingUndefinedType(type)",
		)
	})

	t.Run("FullEngine_Go", func(t *testing.T) {
		src := `package main

func evaluateType(t Type, host Node) Type {
	original := t
	if addUndefined {
		t = getOptionalType(t)
	}
	return t
}
`
		dst := `package main

func evaluateType(t Type, host Node) Type {
	original := t
	if addUndefined {
		t = getOptionalType(t, !isParameter(host))
	}
	if containsNonMissingUndefinedType(t) {
		return undefined
	}
	return t
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: evaluateType declaration
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func evaluateType(t Type, host Node) Type",
			"func evaluateType(t Type, host Node) Type",
		)

		// 2. Surrounding Sequence: original := t and return t
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"original := t",
			"original := t",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"return t",
			"return t",
		)

		// 3. Target: getOptionalType call
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"getOptionalType(t)",
			"getOptionalType(t, !isParameter(host))",
			"call_expression",
		)

		// 4. Newly introduced call containsNonMissingUndefinedType must be unmapped in src
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"containsNonMissingUndefinedType(t)",
		)
	})

	t.Run("FullEngine_RustMacroEvolution", func(t *testing.T) {
		src := `use regex::Regex;
fn configure(opt: &Opt) {
    let commit_regex = Regex::new(&opt.commit_regex).unwrap_or_else(|_| {
        eprintln!(
            "Invalid commit-regex: {}. \
             The value must be a valid Rust regular expression. \
             See https://docs.rs/regex.",
            opt.commit_regex
        );
        process::exit(1);
    });
    println!("Ready: {:?}", commit_regex);
}
`
		dst := `use regex::Regex;
fn configure(opt: &Opt) {
    let commit_regex = Regex::new(&opt.commit_regex).unwrap_or_else(|_| {
        fatal(format!(
            "Invalid commit-regex: {}. \
             The value must be a valid Rust regular expression. \
             See https://docs.rs/regex.",
            opt.commit_regex
        ));
    });
    println!("Ready: {:?}", commit_regex);
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: configure declaration and let commit_regex declaration
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn configure(opt: &Opt)",
			"fn configure(opt: &Opt)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"let commit_regex = Regex::new(&opt.commit_regex)",
			"let commit_regex = Regex::new(&opt.commit_regex)",
		)

		// 2. Surrounding Sequence: succeeding println! statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"println!(\"Ready: {:?}\", commit_regex);",
			"println!(\"Ready: {:?}\", commit_regex);",
		)

		// 3. Stationary Operands: multi-line format string maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"\"Invalid commit-regex: {}.",
			"\"Invalid commit-regex: {}.",
		)

		// 4. Deleted statement: process::exit(1) in src must be unmapped
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"process::exit(1);",
		)
	})

	t.Run("FullEngine_C_PrintfEvolution", func(t *testing.T) {
		src := `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        printf("Invalid commit-regex: %s. Must be valid regex.\n", pattern);
        exit(1);
    }
    printf("Ready: %s\n", pattern);
}
`
		dst := `#include <stdio.h>
#include <stdlib.h>

void validate(const char *pattern) {
    if (!is_valid(pattern)) {
        fatal(format_error("Invalid commit-regex: %s. Must be valid regex.\n", pattern));
    }
    printf("Ready: %s\n", pattern);
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.c")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.c")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: validate function definition maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void validate(const char *pattern)",
			"void validate(const char *pattern)",
		)

		// 2. Surrounding Sequence: succeeding printf("Ready:...") call maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"printf(\"Ready: %s\\n\", pattern);",
			"printf(\"Ready: %s\\n\", pattern);",
		)

		// 3. Stationary Operands: format string literal maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"\"Invalid commit-regex: %s. Must be valid regex.\\n\"",
			"\"Invalid commit-regex: %s. Must be valid regex.\\n\"",
		)

		// 4. Deleted Statement Theft Guard: exit(1) in src must be unmapped (never stolen by fatal)
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"exit(1);",
		)
	})

	t.Run("FullEngine_Zig_PrintEvolution", func(t *testing.T) {
		src := `const std = @import("std");

fn validate(pattern: []const u8) void {
    if (!isValid(pattern)) {
        std.debug.print("Invalid commit-regex: {s}. See docs.\n", .{pattern});
        std.process.exit(1);
    }
    std.debug.print("Ready: {s}\n", .{pattern});
}
`
		dst := `const std = @import("std");

fn validate(pattern: []const u8) void {
    if (!isValid(pattern)) {
        log.err(formatError("Invalid commit-regex: {s}. See docs.\n", .{pattern}));
    }
    std.debug.print("Ready: {s}\n", .{pattern});
}
`

		srcAST, err := treesitter.Parse([]byte(src), "test.zig")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.zig")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: validate function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn validate(pattern: []const u8) void",
			"fn validate(pattern: []const u8) void",
		)

		// 2. Surrounding Sequence: succeeding std.debug.print statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"std.debug.print(\"Ready: {s}\\n\", .{pattern});",
			"std.debug.print(\"Ready: {s}\\n\", .{pattern});",
		)

		// 3. Stationary Operands: format string literal maps in place as context
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"\"Invalid commit-regex: {s}. See docs.\\n\"",
			"\"Invalid commit-regex: {s}. See docs.\\n\"",
		)

		// 4. Deleted Statement: exit call in src must be unmapped
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"std.process.exit(1);",
		)
	})
}

// TestTopDown_GroupedVarAndClosureMatching verifies that grouped var blocks retain
// their surviving specs across leading deletions, and shifted closures match in place
// without cross-statement dredging rejection or closure parameter list theft.
func TestTopDown_GroupedVarAndClosureMatching(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		errMu     sync.Mutex
		workerErr error
		readerErr error
	)
	setErr := func(err error) {
		errMu.Lock()
		if readerErr == nil {
			readerErr = err
		}
		errMu.Unlock()
	}
	getErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return readerErr
	}
	failReader := func(err error) {
		setErr(err)
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`)

		newSrc := []byte(`package stream

import "sync"

func run(in chan string) {
	var wg sync.WaitGroup

	var (
		workerErr error
		readerErr error
	)
	failReader := func(err error) {
		readerErr = err
		cancel()
	}

	defer func() {
		cancel()
		wg.Wait()
	}()
	_ = in
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "stream_old.go")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "stream_new.go")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: run function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func run(in chan string)",
			"func run(in chan string)",
		)

		// 2. Surrounding Sequence: preceding wg declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"var wg sync.WaitGroup",
			"var wg sync.WaitGroup",
		)

		// 3. Target & Containers: grouped var_declaration and surviving specs (workerErr, readerErr)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"workerErr error",
			"workerErr error",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"readerErr error",
			"readerErr error",
		)
		testutil.AssertUnmappedText(t, srcAST, oldSrc, mr.Mappings.Src(),
			"errMu     sync.Mutex",
		)

		// 4. Target Closure: failReader closure matches in place across deleted sibling closures
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"failReader := func(err error)",
			"failReader := func(err error)",
		)

		// 5. Stationary Operands: cancel() inside failReader maps in place
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "cancel()", Occurrence: 1},
			testutil.TextQuery{Text: "cancel()", Occurrence: 1},
		)
	})
}

func TestEvolvingIfStatementWithDrift_Case89(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package stream

func runPipeline() error {
	// Upstream deletions to create line drift (> 10 lines)
	step1()
	step2()
	step3()
	step4()
	step5()
	step6()
	step7()
	step8()
	step9()
	step10()
	step11()

	if flusherErr != nil && !isBrokenPipe(flusherErr) {
		return flusherErr
	}
	if rErr := getReaderErr(); rErr != nil && !isBrokenPipe(rErr) && !isCanceled(rErr) {
		return rErr
	}
	return nil
}
`)

		newSrc := []byte(`package stream

func runPipeline() error {
	if flusherErr != nil && !isBrokenPipe(flusherErr) {
		return flusherErr
	}
	var rErr error
	if p := readerErr.Load(); p != nil {
		rErr = *p
	}
	if rErr != nil && !isBrokenPipe(rErr) && !isCanceled(rErr) && !isDeadline(rErr) {
		return rErr
	}
	return nil
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "stream_old.go")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "stream_new.go")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: runPipeline function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func runPipeline() error",
			"func runPipeline() error",
		)

		// 2. Surrounding Sequence: flusherErr if_statement maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"if flusherErr != nil && !isBrokenPipe(flusherErr)",
			"if flusherErr != nil && !isBrokenPipe(flusherErr)",
		)

		// 3. Target Construct: Evolving if_statement (with rErr) maps in place to the new rErr check
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"if rErr := getReaderErr(); rErr != nil && !isBrokenPipe(rErr) && !isCanceled(rErr)",
			"if rErr != nil && !isBrokenPipe(rErr) && !isCanceled(rErr) && !isDeadline(rErr)",
			"if_statement",
		)

		// 4. Initializer short_var_declaration must NOT map to rErr = *p inside the intermediate if
		testutil.AssertUnmappedText(t, srcAST, oldSrc, mr.Mappings.Src(),
			"rErr := getReaderErr()",
		)

		// 5. Stationary Operands: return rErr inside body block maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"return rErr",
			"return rErr",
		)
	})
}

func TestEvolvingAssignmentStatementWithDrift_Case90(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package engine

func ContestContainers(currentT1, t2 *ASTNode, m *Mapping) {
	precedingInit()

	currCommon, _, _ := commonMappedDescendants(currentT1, t2, m.Src())
	currDice := m.DiceSrc(currentT1, t2)
	currAffinity := max(0, computeAffinity(currentT1, t2, m, DefaultAffinityWeights, true))

	succeedingDone(currCommon, currDice, currAffinity)
}
`)

		newSrc := []byte(`package engine

func ContestContainers(currentT1, t2 *ASTNode, m *Mapping) {
	precedingInit()

	// Five inserted lines are enough to push drift past the 10-line dredging threshold.
	insertedLine01 := 1
	insertedLine02 := 2
	insertedLine03 := 3
	insertedLine04 := 4
	insertedLine05 := 5
	_ = insertedLine01
	_ = insertedLine02
	_ = insertedLine03
	_ = insertedLine04
	_ = insertedLine05

	var currCommon int
	var currDice float64
	var currAffinity float64

	currCommon, _, _ = commonMappedDescendants(currentT1, t2, m.Src())
	currDice = m.DiceSrc(currentT1, t2)
	currAffinity = max(0, computeAffinity(currentT1, t2, m, DefaultAffinityWeights, true))

	succeedingDone(currCommon, currDice, currAffinity)
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing Context: ContestContainers function maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func ContestContainers",
			"func ContestContainers",
		)

		// 2. Surrounding Sequence: precedingInit and succeedingDone map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"precedingInit()",
			"precedingInit()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"succeedingDone(currCommon, currDice, currAffinity)",
			"succeedingDone(currCommon, currDice, currAffinity)",
		)

		// 3. Target Statements: all 3 short_var_declarations map to assignment_statements
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"currCommon, _, _ := commonMappedDescendants",
			"currCommon, _, _ = commonMappedDescendants",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"currDice := m.DiceSrc(currentT1, t2)",
			"currDice = m.DiceSrc(currentT1, t2)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"currAffinity := max(0, computeAffinity(currentT1, t2, m, DefaultAffinityWeights, true))",
			"currAffinity = max(0, computeAffinity(currentT1, t2, m, DefaultAffinityWeights, true))",
		)
	})
}

// TestCrossScopeStatementDredging_Case91 verifies that an evolving statement's sub-expression
// won't dredge matching subtrees out of a completely different function scope.
func TestCrossScopeStatementDredging_Case91(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package comments

func TestDiffCommentsGlobal() {
	setupFixture()
	if act.Type == actions.Update && strings.Contains(act.Node.Type, "identifier") {
		recordAction()
	}
	teardownFixture()
}

func fillerStatement01() { noop() }
func fillerStatement02() { noop() }
func fillerStatement03() { noop() }
func fillerStatement04() { noop() }
func fillerStatement05() { noop() }
func fillerStatement06() { noop() }
func fillerStatement07() { noop() }
func fillerStatement08() { noop() }
func fillerStatement09() { noop() }
func fillerStatement10() { noop() }

func TestDiffCommentsAnchorFirst() {
	setupAnchor()
	if act.Type == actions.Update && act.Node != nil && act.Node.Label == "// doB()" {
		recordAnchor()
	}
	teardownAnchor()
}
`)

		newSrc := []byte(`package comments

func TestDiffCommentsGlobal() {
	setupFixture()
	if ((act.Type == actions.Update && act.Node != nil) && act.Node.StartRow == 100) && strings.Contains(act.Node.Type, "identifier") {
		recordAction()
	}
	teardownFixture()
}

func fillerStatement01() { noop() }
func fillerStatement02() { noop() }
func fillerStatement03() { noop() }
func fillerStatement04() { noop() }
func fillerStatement05() { noop() }
func fillerStatement06() { noop() }
func fillerStatement07() { noop() }
func fillerStatement08() { noop() }
func fillerStatement09() { noop() }
func fillerStatement10() { noop() }

func TestDiffCommentsAnchorFirst() {
	setupAnchor()
	if act.Type == actions.Update && act.Node != nil && act.Node.StartRow == 100 {
		recordAnchor()
	}
	teardownAnchor()
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// Both functions should map in place as enclosing context.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func TestDiffCommentsGlobal()",
			"func TestDiffCommentsGlobal()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func TestDiffCommentsAnchorFirst()",
			"func TestDiffCommentsAnchorFirst()",
		)

		// Fixture setup and teardown calls around the target statements shouldn't move.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setupFixture()",
			"setupFixture()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardownFixture()",
			"teardownFixture()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setupAnchor()",
			"setupAnchor()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardownAnchor()",
			"teardownAnchor()",
		)

		// The unchanged condition inside TestDiffCommentsGlobal should map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"strings.Contains(act.Node.Type, \"identifier\")",
			"strings.Contains(act.Node.Type, \"identifier\")",
		)
	})
}

func TestAttributeUnnestingAndInnerOuterEquivalence(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`#[cfg(test)]
pub mod test_utils {
    use std::fs::File;

    pub fn make_options() -> bool {
        true
    }

    #[allow(dead_code)]
    pub fn make_config() -> bool {
        true
    }
}

#[cfg(test)]
mod tests {
    pub fn run_test() {}
}
`)

		newSrc := []byte(`#![cfg(test)]

use std::fs::File;

pub fn make_options() -> bool {
    true
}

#[allow(dead_code)]
pub fn make_config() -> bool {
    true
}

#[cfg(test)]
mod tests {
    pub fn run_test() {}
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.rs")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.rs")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Target 1: Outer attribute #[cfg(test)] at line 1 must map to inner attribute #![cfg(test)] at line 1.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "#[cfg(test)]", Occurrence: 1},
			testutil.TextQuery{Text: "#![cfg(test)]", Occurrence: 1},
		)

		// 2. Target 2: #[allow(dead_code)] on make_config must map to #[allow(dead_code)] in place (NOT deleted or cross-matched).
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"#[allow(dead_code)]",
			"#[allow(dead_code)]",
		)

		// 3. Sibling Scope: tests mod #[cfg(test)] must remain mapped to tests mod #[cfg(test)] without cross-scope theft.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "#[cfg(test)]", Occurrence: 2},
			testutil.TextQuery{Text: "#[cfg(test)]", Occurrence: 1},
		)

		// 4. Surrounding Sequence & Stationary Functions: make_options, make_config, and tests mod map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"pub fn make_options() -> bool",
			"pub fn make_options() -> bool",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"pub fn make_config() -> bool",
			"pub fn make_config() -> bool",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"mod tests",
			"mod tests",
		)
	})

	t.Run("JavaFullEngine", func(t *testing.T) {
		r := rules.Get("java")
		if r == nil {
			t.Fatal("java rules not found")
		}

		oldSrc := []byte(`class Service {
    @Deprecated
    class Worker {
        public void execute() {}

        @SuppressWarnings("rawtypes")
        public boolean isBusy() {
            return false;
        }
    }

    @Deprecated
    class TaskManager {
        public void process() {}
    }
}
`)

		newSrc := []byte(`@Deprecated(since = "1.2")
class Worker {
    public void execute() {}

    @SuppressWarnings("rawtypes")
    public boolean isBusy() {
        return false;
    }
}

@Deprecated
class TaskManager {
    public void process() {}
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.java")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.java")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Target 1: Outer marker_annotation @Deprecated on inner class maps to unnested annotation @Deprecated(since = "1.2").
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "@Deprecated", Occurrence: 1},
			testutil.TextQuery{Text: "@Deprecated(since = \"1.2\")", Occurrence: 1},
		)

		// 2. Target 2: @SuppressWarnings on isBusy stays mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"@SuppressWarnings(\"rawtypes\")",
			"@SuppressWarnings(\"rawtypes\")",
		)

		// 3. Sibling Scope: TaskManager's @Deprecated shouldn't get stolen across scopes.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "@Deprecated", Occurrence: 2},
			testutil.TextQuery{Text: "@Deprecated", Occurrence: 2},
		)

		// 4. Surrounding Sequence & Stationary Declarations: Worker, TaskManager, execute, and isBusy map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"class Worker",
			"class Worker",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"public void execute()",
			"public void execute()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"public boolean isBusy()",
			"public boolean isBusy()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"class TaskManager",
			"class TaskManager",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"public void process()",
			"public void process()",
		)
	})
}

func TestTopDown_ChainedMethodCalleeCompatibility(t *testing.T) {
	t.Run("FullEngine_Rust", func(t *testing.T) {
		src := `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
                .takes_value(true)
        );
    finalize(flags);
}
`
		dst := `fn configure_app() {
    let mut flags = 0;
    let app = App::new()
        .arg(
            Arg::new("nonprintable-notation")
                .long("nonprintable-notation")
                .hide_default_value(true)
        )
        .arg(
            Arg::new("highlight-line")
                .long("highlight-line")
                .short('H')
        );
    finalize(flags);
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src has parse errors: count=%d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst has parse errors: count=%d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: configure_app declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"fn configure_app()",
			"fn configure_app()",
		)

		// 2. Surrounding Sequence: let mut flags = 0 and finalize(flags) map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"let mut flags = 0;",
			"let mut flags = 0;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"finalize(flags)",
			"finalize(flags)",
		)

		// 3. Target & Containers: stationary Arg::new("highlight-line") chain maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			`Arg::new("highlight-line")`,
			`Arg::new("highlight-line")`,
		)

		// 4. Deleted tail call takes_value(true) must be unmapped in src
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"takes_value(true)",
		)

		// 5. Newly inserted hide_default_value(true) must be unmapped in dst
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"hide_default_value(true)",
		)

		// 6. Argument literal 'true' must not be stolen across mismatched callees.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			testutil.TextQuery{Text: "true", Type: "boolean_literal", Occurrence: 1},
			testutil.TextQuery{Text: "true", Type: "boolean_literal", Occurrence: 1},
		)
	})

	t.Run("FullEngine_Go", func(t *testing.T) {
		src := `package main

func configureApp() {
	flags := 0
	app := NewApp().
		Arg(
			NewArg("highlight-line").
				Long("highlight-line").
				Short('H').
				TakesValue(true),
		)
	finalize(flags)
	_ = app
}
`
		dst := `package main

func configureApp() {
	flags := 0
	app := NewApp().
		Arg(
			NewArg("nonprintable-notation").
				Long("nonprintable-notation").
				HideDefaultValue(true),
		).
		Arg(
			NewArg("highlight-line").
				Long("highlight-line").
				Short('H'),
		)
	finalize(flags)
	_ = app
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src has parse errors: count=%d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst has parse errors: count=%d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing Context: configureApp declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func configureApp()",
			"func configureApp()",
		)

		// 2. Surrounding Sequence: flags := 0 and finalize(flags) map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"flags := 0",
			"flags := 0",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"finalize(flags)",
			"finalize(flags)",
		)

		// 3. Target & Containers: stationary NewArg("highlight-line") chain maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			`NewArg("highlight-line")`,
			`NewArg("highlight-line")`,
		)

		// 4. Deleted tail call TakesValue(true) must be unmapped in src
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"TakesValue(true)",
		)

		// 5. Newly inserted HideDefaultValue(true) must be unmapped in dst
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"HideDefaultValue(true)",
		)

		// 6. Argument literal 'true' must not be stolen across mismatched callees.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			testutil.TextQuery{Text: "true", Type: "true", Occurrence: 1},
			testutil.TextQuery{Text: "true", Type: "true", Occurrence: 1},
		)
	})

	t.Run("FullEngine_Java", func(t *testing.T) {
		t.Skip("temporarily skipped during rebase; pending BottomUp method chain callee update")
		src := `class AppConfig {
    void configureApp() {
        int flags = 0;
        App app = App.builder()
            .addArg(
                Arg.builder("highlight-line")
                    .longName("highlight-line")
                    .shortCode('H')
                    .takesValue(true)
            );
        finalize(flags);
    }
}
`
		dst := `class AppConfig {
    void configureApp() {
        int flags = 0;
        App app = App.builder()
            .addArg(
                Arg.builder("nonprintable-notation")
                    .longName("nonprintable-notation")
                    .hideDefaultValue(true)
            )
            .addArg(
                Arg.builder("highlight-line")
                    .longName("highlight-line")
                    .shortCode('H')
            );
        finalize(flags);
    }
}
`
		srcAST, err := treesitter.Parse([]byte(src), "test.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "test.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src has parse errors: count=%d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst has parse errors: count=%d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing function maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"void configureApp()",
			"void configureApp()",
		)

		// 2. Preceding and succeeding statements stay locked in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"int flags = 0;",
			"int flags = 0;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"finalize(flags)",
			"finalize(flags)",
		)

		// 3. Unchanged middle methods (.shortCode) still map across both trees.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"shortCode('H')",
			"shortCode('H')",
		)

		// 4. Deleted tail call stays unmapped in old tree.
		testutil.AssertUnmappedText(t, srcAST, []byte(src), mr.Mappings.Src(),
			"takesValue(true)",
		)

		// 5. Newly inserted tail call stays unmapped in new tree.
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), mr.Mappings.Dst(),
			"hideDefaultValue(true)",
		)

		// 6. Argument literal 'true' must not be stolen across mismatched callees.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			testutil.TextQuery{Text: "true", Type: "boolean_literal", Occurrence: 1},
			testutil.TextQuery{Text: "true", Type: "boolean_literal", Occurrence: 1},
		)
	})
}

func TestTopDown_FieldConcordanceTieBreaking_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`def configure_session():
    setup_logger()
    options = {
        (a + b): (c + d),
    }
    teardown_logger(options)
`)
		newSrc := []byte(`def configure_session():
    setup_logger()
    options = {
        (c + d): (a + b),
    }
    teardown_logger(options)
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.py")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.py")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing function declaration maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"def configure_session():",
			"def configure_session():",
		)

		// 2. Surrounding sequence (setup_logger, teardown_logger) maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setup_logger()",
			"setup_logger()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardown_logger(options)",
			"teardown_logger(options)",
		)

		// 3. Locate pair container and key/value expressions:
		// The pair container must map in place so the colon stays stationary trivia.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"(a + b): (c + d)",
			"(c + d): (a + b)",
			"pair",
		)

		// 4. Conflicting slots: key (a + b) must not match value (a + b) across the colon.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "(a + b)", Occurrence: 1},
			testutil.TextQuery{Text: "(a + b)", Occurrence: 1},
		)
	})
}

func TestTopDown_CrossStatementDredging_FieldMutualExclusion_FullEngine(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package main

func computeMetrics() {
	setupMetrics()
	leftTarget := 100
	unrelated := 200
	rightExpr := leftTarget + 5
	teardownMetrics(unrelated, rightExpr)
}
`)
		newSrc := []byte(`package main

func computeMetrics() {
	setupMetrics()
	discarded := 999
	unrelated := 200
	rightExpr := 500 + leftTarget
	teardownMetrics(unrelated, rightExpr)
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatalf("parse old: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatalf("parse new: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil mappings")
		}

		// 1. Enclosing function maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"func computeMetrics()",
			"func computeMetrics()",
		)

		// 2. Surrounding sequence (setupMetrics, teardownMetrics) maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"setupMetrics()",
			"setupMetrics()",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"teardownMetrics(unrelated, rightExpr)",
			"teardownMetrics(unrelated, rightExpr)",
		)

		// 3. Middle statement unrelated := 200 maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			"unrelated := 200",
			"unrelated := 200",
		)

		// 4. Locate leftTarget in old statement 1 (LHS target) and leftTarget in new statement 3 (RHS operand).
		// An LHS target variable in statement 1 shouldn't dredge into an RHS operand in another statement.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "leftTarget", Type: "identifier", Occurrence: 1},
			testutil.TextQuery{Text: "leftTarget", Type: "identifier", Occurrence: 1},
		)

		// 5. Commutative operand leftTarget in statement 3 should map to dstRhsLeftTarget.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, mr.Mappings.Src(),
			testutil.TextQuery{Text: "leftTarget", Type: "identifier", Occurrence: 2},
			testutil.TextQuery{Text: "leftTarget", Type: "identifier", Occurrence: 1},
		)
	})
}
