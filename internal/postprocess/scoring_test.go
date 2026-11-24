package postprocess

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestIsTerminatingStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("return statement is terminating", func(t *testing.T) {
		src := []byte("package main\nfunc f() { return }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		stmt, err := testutil.FindNodeByText(tree, src, "return", "return_statement")
		if err != nil {
			t.Fatalf("find return_statement: %v", err)
		}
		if !isTerminatingStatement(stmt, r) {
			t.Error("expected return_statement to be terminating")
		}
	})

	t.Run("os.Exit call is terminating", func(t *testing.T) {
		src := []byte("package main\nimport \"os\"\nfunc f() { os.Exit(1) }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		exprStmt, err := testutil.FindNodeByText(tree, src, "os.Exit(1)", "expression_statement")
		if err != nil {
			t.Fatalf("find os.Exit: %v", err)
		}
		if !isTerminatingStatement(exprStmt, r) {
			t.Error("expected os.Exit call to be terminating")
		}
	})

	t.Run("regular expression is not terminating", func(t *testing.T) {
		src := []byte("package main\nimport \"fmt\"\nfunc f() { fmt.Println() }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		stmt, err := testutil.FindNodeByText(tree, src, "fmt.Println()", "expression_statement")
		if err != nil {
			t.Fatalf("find fmt.Println: %v", err)
		}
		if isTerminatingStatement(stmt, r) {
			t.Error("expected fmt.Println call to NOT be terminating")
		}
	})
}

func TestIsTrivialJumpBody(t *testing.T) {
	r := rules.Get("go")

	t.Run("body of only jump statements is trivial", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\treturn\n}\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		body, err := testutil.FindNodeByText(tree, src, "{\n\treturn\n}", "block")
		if err != nil {
			t.Fatalf("find block: %v", err)
		}
		if !isTrivialJumpBody(body, r) {
			t.Error("expected body of only a return statement to be trivial")
		}
	})

	// An error print followed by an exit is still boilerplate - the print
	// shouldn't keep it from being scored as a trivial exit block.
	t.Run("body pairing an error print with a terminating call is trivial", func(t *testing.T) {
		src := []byte("package main\nimport (\n\t\"fmt\"\n\t\"os\"\n)\nfunc f() {\n\tfmt.Fprintf(os.Stderr, \"err\")\n\tos.Exit(1)\n}\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		body, err := testutil.FindNodeByText(tree, src, "{\n\tfmt.Fprintf(os.Stderr, \"err\")\n\tos.Exit(1)\n}", "block")
		if err != nil {
			t.Fatalf("find block: %v", err)
		}
		if !isTrivialJumpBody(body, r) {
			t.Error("expected print+os.Exit body to be trivial boilerplate")
		}
	})
}

func TestMoveStructuralScore(t *testing.T) {
	r := rules.Get("go")
	ms := engine.NewMapping()

	t.Run("bare token clamped to 1", func(t *testing.T) {
		src := []byte("package main\nfunc f() { _ = 1 + 2 }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		op, err := testutil.FindNodeByText(tree, src, "+")
		if err != nil {
			t.Fatalf("find operator: %v", err)
		}
		score := moveStructuralScore(op, r, ms)
		if score != 1 {
			t.Errorf("expected token score 1, got %d", score)
		}
	})

	t.Run("small if block with trivial body", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif err {\n\t\treturn\n\t}\n}\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		ifStmt, err := testutil.FindNodeByText(tree, src, "if err {\n\t\treturn\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find if_statement: %v", err)
		}

		score := moveStructuralScore(ifStmt, r, ms)
		// Size ~4, height ~2, lines 0, boilerplate -20 → clamp to 1
		if score < 1 {
			t.Errorf("expected score >= 1, got %d", score)
		}
	})

	t.Run("declaration gets +40 bonus", func(t *testing.T) {
		src := []byte("package main\nfunc foo() {}\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		decl, err := testutil.FindNodeByText(tree, src, "func foo() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find declaration: %v", err)
		}

		score := moveStructuralScore(decl, r, ms)
		if score < 40 {
			t.Errorf("expected declaration score >= 40, got %d", score)
		}
	})

	t.Run("container with deleted contents discounts size by surviving mass", func(t *testing.T) {
		srcBytes := []byte("package main\nfunc f() {\n\talpha()\n\tbeta()\n\tgamma()\n}\n")
		dstBytes := []byte("package main\nfunc f() {\n\talpha()\n\tbeta()\n\tgamma()\n}\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		block, err := testutil.FindNodeByText(srcTree, srcBytes, "{\n\talpha()\n\tbeta()\n\tgamma()\n}", "block")
		if err != nil {
			t.Fatalf("find block: %v", err)
		}
		id1Src, err := testutil.FindNodeByText(srcTree, srcBytes, "alpha", "identifier")
		if err != nil {
			t.Fatalf("find alpha src: %v", err)
		}
		id2Src, err := testutil.FindNodeByText(srcTree, srcBytes, "beta", "identifier")
		if err != nil {
			t.Fatalf("find beta src: %v", err)
		}
		id3Src, err := testutil.FindNodeByText(srcTree, srcBytes, "gamma", "identifier")
		if err != nil {
			t.Fatalf("find gamma src: %v", err)
		}
		id1Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "alpha", "identifier")
		if err != nil {
			t.Fatalf("find alpha dst: %v", err)
		}
		id2Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "beta", "identifier")
		if err != nil {
			t.Fatalf("find beta dst: %v", err)
		}
		id3Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "gamma", "identifier")
		if err != nil {
			t.Fatalf("find gamma dst: %v", err)
		}

		emptyMs := engine.NewMapping()
		scoreEmpty := moveStructuralScore(block, r, emptyMs)

		partialMs := engine.NewMapping()
		partialMs.Add(id1Src, id1Dst)
		scorePartial := moveStructuralScore(block, r, partialMs)

		if scorePartial <= scoreEmpty {
			t.Errorf("expected score with surviving child (%d) > empty score (%d)", scorePartial, scoreEmpty)
		}

		fullMs := engine.NewMapping()
		fullMs.Add(id1Src, id1Dst)
		fullMs.Add(id2Src, id2Dst)
		fullMs.Add(id3Src, id3Dst)
		scoreFull := moveStructuralScore(block, r, fullMs)
		expectedScore := block.Size() + 2*subtreeHeight(block) + 10
		if scoreFull > expectedScore {
			t.Errorf("expected scoreFull (%d) not to exceed undiscounted score (%d)", scoreFull, expectedScore)
		}
	})
}

func TestRequiredMoveThreshold(t *testing.T) {
	r := rules.Get("go")
	ms := engine.NewMapping()

	t.Run("intra-container reorder returns 1", func(t *testing.T) {
		srcBytes := []byte("package main\nfunc f() { call(arg1, arg2) }\n")
		dstBytes := []byte("package main\nfunc f() { call(arg2, arg1) }\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, srcBytes, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find src func: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dstBytes, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find dst func: %v", err)
		}
		parentSrc, err := testutil.FindNodeByText(srcTree, srcBytes, "(arg1, arg2)", "argument_list")
		if err != nil {
			t.Fatalf("find src argument_list: %v", err)
		}
		parentDst, err := testutil.FindNodeByText(dstTree, dstBytes, "(arg2, arg1)", "argument_list")
		if err != nil {
			t.Fatalf("find dst argument_list: %v", err)
		}
		srcA, err := testutil.FindNodeByText(srcTree, srcBytes, "arg1", "identifier")
		if err != nil {
			t.Fatalf("find src arg1: %v", err)
		}
		dstB, err := testutil.FindNodeByText(dstTree, dstBytes, "arg2", "identifier")
		if err != nil {
			t.Fatalf("find dst arg2: %v", err)
		}

		ms.Add(srcFunc, dstFunc)
		ms.Add(parentSrc, parentDst)

		threshold := requiredMoveThreshold(srcA, dstB, ms, r)
		if threshold != 1 {
			t.Errorf("expected threshold 1 for intra-container reorder, got %d", threshold)
		}
	})

	t.Run("top-level declaration returns 20", func(t *testing.T) {
		srcBytes := []byte("package main\n\nfunc foo() {}\n")
		dstBytes := []byte("package main\n" + strings.Repeat("\n", 500) + "func bar() {}\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, srcBytes, "func foo() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find src func: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dstBytes, "func bar() {}", "function_declaration")
		if err != nil {
			t.Fatalf("find dst func: %v", err)
		}

		threshold := requiredMoveThreshold(srcFunc, dstFunc, ms, r)
		if threshold != 20 {
			t.Errorf("expected threshold 20 for top-level declaration, got %d", threshold)
		}
	})

	// Don't let a bare literal move between different struct fields just
	// because they sit on the same line (e.g. `defValue: ""` vs `shorthand: ""`).
	t.Run("cross-key bare literal at same line requires threshold >= 5", func(t *testing.T) {
		srcBytes := []byte("package main\nvar x = T{defValue: \"\"}\n")
		dstBytes := []byte("package main\nvar x = T{shorthand: \"\"}\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcVal, err := testutil.FindNodeByText(srcTree, srcBytes, `""`, "interpreted_string_literal")
		if err != nil {
			t.Fatalf("find srcVal: %v", err)
		}
		dstVal, err := testutil.FindNodeByText(dstTree, dstBytes, `""`, "interpreted_string_literal")
		if err != nil {
			t.Fatalf("find dstVal: %v", err)
		}

		threshold := requiredMoveThreshold(srcVal, dstVal, ms, r)
		if threshold < 5 {
			t.Errorf("expected threshold >= 5 for cross-key bare literal, got %d (a clamped S=1 token would survive as a Move)", threshold)
		}
	})

	t.Run("same-line inline shift returns 1", func(t *testing.T) {
		ms := engine.NewMapping()
		srcBytes := []byte("package main\nfunc f() { foo() }\n")
		dstBytes := []byte("package main\nfunc f() { foo() }\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, srcBytes, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find src func: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dstBytes, "func f()", "function_declaration")
		if err != nil {
			t.Fatalf("find dst func: %v", err)
		}
		ms.Add(srcFunc, dstFunc)

		srcStmt, err := testutil.FindNodeByText(srcTree, srcBytes, "foo()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dstBytes, "foo()", "expression_statement")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		threshold := requiredMoveThreshold(srcStmt, dstStmt, ms, r)
		if threshold != 1 {
			t.Errorf("requiredMoveThreshold() = %d, want 1", threshold)
		}
	})

	t.Run("distant statement across scopes does not get same-line threshold of 1", func(t *testing.T) {
		// A surviving outer scope shouldn't cancel out line distance between distant functions.
		srcBytes := []byte("package main\nfunc foo() {\n\tif true {\n\t}\n}\n")
		dstBytes := []byte("package main\n" + strings.Repeat("\n", 200) + "func bar() {\n\tif true {\n\t}\n}\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcIf, err := testutil.FindNodeByText(srcTree, srcBytes, "if true {\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find src if: %v", err)
		}
		dstIf, err := testutil.FindNodeByText(dstTree, dstBytes, "if true {\n\t}", "if_statement")
		if err != nil {
			t.Fatalf("find dst if: %v", err)
		}

		msDrift := engine.NewMapping()
		msDrift.Add(srcTree, dstTree)

		threshold := requiredMoveThreshold(srcIf, dstIf, msDrift, r)
		if threshold <= 1 {
			t.Errorf("expected threshold > 1 for distant statement 200 lines away, got %d", threshold)
		}
		// 200 lines apart gets the cross-scope penalty: 50 + 200/10 = 70.
		if threshold < 50 {
			t.Errorf("expected threshold >= 50 for 200-line distant statement across scopes, got %d", threshold)
		}
	})

	t.Run("initializer statement moving out of if gets statement threshold", func(t *testing.T) {
		srcBytes := []byte("package main\nfunc foo() {\n\tif x := 1; true {}\n}\n")
		dstBytes := []byte("package main\nfunc foo() {\n\n\n\n\n\n\tx = 1\n}\n")
		srcTree, err := treesitter.Parse(srcBytes, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstBytes, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, srcBytes, "func foo()", "function_declaration")
		if err != nil {
			t.Fatalf("find src func: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dstBytes, "func foo()", "function_declaration")
		if err != nil {
			t.Fatalf("find dst func: %v", err)
		}
		srcBody, err := testutil.FindNodeByText(srcTree, srcBytes, "{\n\tif x := 1; true {}\n}", "block")
		if err != nil {
			t.Fatalf("find src block: %v", err)
		}
		dstBody, err := testutil.FindNodeByText(dstTree, dstBytes, "{\n\n\n\n\n\n\tx = 1\n}", "block")
		if err != nil {
			t.Fatalf("find dst block: %v", err)
		}
		srcInit, err := testutil.FindNodeByText(srcTree, srcBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find srcInit: %v", err)
		}
		dstAssign, err := testutil.FindNodeByText(dstTree, dstBytes, "x = 1", "assignment_statement")
		if err != nil {
			t.Fatalf("find dstAssign: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)
		ms.Add(srcBody, dstBody)

		threshold := requiredMoveThreshold(srcInit, dstAssign, ms, r)
		// Distance is 5 lines (row 2 to row 7). With 4 + lineDist/5, threshold should be 5, not 20.
		if threshold >= 20 {
			t.Errorf("expected threshold < 20 for moving initializer statement, got %d", threshold)
		}
		if threshold != 5 {
			t.Errorf("expected threshold = 5 for moving initializer statement (4 + 5/5), got %d", threshold)
		}
	})
}

func TestIsFillerCallStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("bare call is filler", func(t *testing.T) {
		src := []byte("package main\nimport \"fmt\"\nfunc f() { fmt.Println() }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		call, err := testutil.FindNodeByText(tree, src, "fmt.Println()", "call_expression")
		if err != nil {
			t.Fatalf("find call: %v", err)
		}
		if !isFillerCallStatement(call, r) {
			t.Error("expected bare call to be filler")
		}
	})

	t.Run("single-child wrapper around call is filler", func(t *testing.T) {
		src := []byte("package main\nimport \"fmt\"\nfunc f() { fmt.Println() }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		exprStmt, err := testutil.FindNodeByText(tree, src, "fmt.Println()", "expression_statement")
		if err != nil {
			t.Fatalf("find exprStmt: %v", err)
		}
		if !isFillerCallStatement(exprStmt, r) {
			t.Error("expected single-child wrapper around call to be filler")
		}
	})

	t.Run("multi-child statement is not filler", func(t *testing.T) {
		src := []byte("package main\nfunc f() { if err {} }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		ifStmt, err := testutil.FindNodeByText(tree, src, "if err {}", "if_statement")
		if err != nil {
			t.Fatalf("find if: %v", err)
		}
		if isFillerCallStatement(ifStmt, r) {
			t.Error("expected multi-child statement to not be filler")
		}
	})

	t.Run("nil node is not filler", func(t *testing.T) {
		if isFillerCallStatement(nil, r) {
			t.Error("expected nil to not be filler")
		}
	})
}

func TestIsPayloadLeaf(t *testing.T) {
	r := rules.Get("go")

	t.Run("nil safety and non-leaf", func(t *testing.T) {
		if isPayloadLeaf(nil, r) {
			t.Errorf("expected nil node to not be payload leaf")
		}
		src := []byte("package main\nfunc f() { foo() }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		parent, err := testutil.FindNodeByText(tree, src, "foo()", "call_expression")
		if err != nil {
			t.Fatalf("find call: %v", err)
		}
		if isPayloadLeaf(parent, r) {
			t.Errorf("expected node with children to not be payload leaf")
		}
	})

	t.Run("punctuation and operators rejected", func(t *testing.T) {
		src := []byte("package main\nfunc f() { x := 1 + 2; }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		punct, err := testutil.FindNodeByText(tree, src, ";")
		if err != nil {
			t.Fatalf("find semicolon: %v", err)
		}
		if isPayloadLeaf(punct, r) {
			t.Errorf("expected semicolon to not be payload leaf")
		}
		op, err := testutil.FindNodeByText(tree, src, "+")
		if err != nil {
			t.Fatalf("find operator: %v", err)
		}
		if isPayloadLeaf(op, r) {
			t.Errorf("expected operator to not be payload leaf")
		}
	})

	t.Run("keywords filtering and jump statement exception", func(t *testing.T) {
		src := []byte("package main\nfunc f() { return }\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		funcKw, err := testutil.FindNodeByText(tree, src, "func")
		if err != nil {
			t.Fatalf("find func: %v", err)
		}
		if isPayloadLeaf(funcKw, r) {
			t.Errorf("expected 'func' keyword to not be payload leaf")
		}

		retKwUnderJump, err := testutil.FindNodeByText(tree, src, "return")
		if err != nil {
			t.Fatalf("find return: %v", err)
		}
		if !isPayloadLeaf(retKwUnderJump, r) {
			t.Errorf("expected 'return' keyword inside return_statement to be payload leaf")
		}

		// A bare keyword without jump parent (e.g. var keyword under var_declaration)
		srcVar := []byte("package main\nvar x = 1\n")
		treeVar, err := treesitter.Parse(srcVar, "main.go")
		if err != nil {
			t.Fatalf("parse var: %v", err)
		}
		varKw, err := testutil.FindNodeByText(treeVar, srcVar, "var")
		if err != nil {
			t.Fatalf("find var: %v", err)
		}
		if isPayloadLeaf(varKw, r) {
			t.Errorf("expected 'var' keyword to not be payload leaf")
		}
	})

	t.Run("content leaves accepted", func(t *testing.T) {
		src := []byte("package main\nvar myVar = \"hello\"\n")
		tree, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		id, err := testutil.FindNodeByText(tree, src, "myVar", "identifier")
		if err != nil {
			t.Fatalf("find myVar: %v", err)
		}
		if !isPayloadLeaf(id, r) {
			t.Errorf("expected identifier to be payload leaf")
		}
		lit, err := testutil.FindNodeByText(tree, src, `"hello"`, "interpreted_string_literal")
		if err != nil {
			t.Fatalf("find literal: %v", err)
		}
		if !isPayloadLeaf(lit, r) {
			t.Errorf("expected literal to be payload leaf")
		}
	})
}

func TestSurvivingAndSummarizeMappedLeaves(t *testing.T) {
	r := rules.Get("go")

	srcBytes := []byte("package main\nvar _ = []string{foo, bar}\n")
	dstBytes := []byte("package main\nvar _ = []string{foo, baz}\n")
	srcTree, err := treesitter.Parse(srcBytes, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dstBytes, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcBlock, err := testutil.FindNodeByText(srcTree, srcBytes, "{foo, bar}", "literal_value")
	if err != nil {
		t.Fatalf("find src literal_value: %v", err)
	}
	dstBlock, err := testutil.FindNodeByText(dstTree, dstBytes, "{foo, baz}", "literal_value")
	if err != nil {
		t.Fatalf("find dst literal_value: %v", err)
	}
	srcLeaf1, err := testutil.FindNodeByText(srcTree, srcBytes, "foo", "identifier")
	if err != nil {
		t.Fatalf("find srcLeaf1: %v", err)
	}
	srcLeaf2, err := testutil.FindNodeByText(srcTree, srcBytes, "bar", "identifier")
	if err != nil {
		t.Fatalf("find srcLeaf2: %v", err)
	}
	dstLeaf1, err := testutil.FindNodeByText(dstTree, dstBytes, "foo", "identifier")
	if err != nil {
		t.Fatalf("find dstLeaf1: %v", err)
	}
	dstLeaf2, err := testutil.FindNodeByText(dstTree, dstBytes, "baz", "identifier")
	if err != nil {
		t.Fatalf("find dstLeaf2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcLeaf1, dstLeaf1)
	ms.Add(srcLeaf2, dstLeaf2)

	// Declarative verification of mapping setup
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, ms.Src(), "foo", "foo", "identifier")
	testutil.AssertMappedText(t, srcTree, dstTree, srcBytes, dstBytes, ms.Src(), "bar", "baz", "identifier")

	t.Run("hasSurvivingMappedLeaves detects mapped payload", func(t *testing.T) {
		if !hasSurvivingMappedLeaves(srcBlock, dstBlock, ms.Src(), nil, r) {
			t.Fatalf("expected surviving mapped leaves")
		}
		// Bidirectional symmetry check
		if !hasSurvivingMappedLeaves(dstBlock, srcBlock, ms.Dst(), nil, r) {
			t.Fatalf("expected surviving mapped leaves in reverse mapping")
		}
	})

	t.Run("hasSurvivingMappedLeaves respects eviction", func(t *testing.T) {
		evicted := map[*treesitter.ASTNode]struct{}{
			srcLeaf1: {},
			srcLeaf2: {},
		}
		if hasSurvivingMappedLeaves(srcBlock, dstBlock, ms.Src(), evicted, r) {
			t.Errorf("expected no surviving leaves when all payload leaves are evicted")
		}
		// In reverse direction, partner (source node) eviction is respected
		if hasSurvivingMappedLeaves(dstBlock, srcBlock, ms.Dst(), evicted, r) {
			t.Errorf("expected no surviving leaves in reverse when partners are evicted")
		}
	})

	t.Run("summarizeMappedLeaves counts exact and updated leaves", func(t *testing.T) {
		s := summarizeMappedLeaves(srcBlock, dstBlock, ms, r, nil)
		if s.total != 2 {
			t.Errorf("expected total=2 (ignoring punctuation), got %d", s.total)
		}
		if s.exact != 1 {
			t.Errorf("expected exact=1 ('foo'), got %d", s.exact)
		}
		if s.updated != 1 {
			t.Errorf("expected updated=1 ('bar' -> 'baz'), got %d", s.updated)
		}

		// Evicting one leaf adjusts exact count
		sEvicted := summarizeMappedLeaves(srcBlock, dstBlock, ms, r, map[*treesitter.ASTNode]struct{}{srcLeaf1: {}})
		if sEvicted.total != 2 {
			t.Errorf("expected total=2, got %d", sEvicted.total)
		}
		if sEvicted.exact != 0 {
			t.Errorf("expected exact=0 after evicting 'foo', got %d", sEvicted.exact)
		}
		if sEvicted.updated != 1 {
			t.Errorf("expected updated=1 for 'bar' -> 'baz', got %d", sEvicted.updated)
		}
	})
}

func TestIsDelimitedOrBlockContainer(t *testing.T) {
	r := rules.Get("go")

	if isDelimitedOrBlockContainer(nil, r) {
		t.Errorf("expected nil to return false")
	}

	src := []byte("package main\nfunc f(x int) { foo(x) }\n")
	tree, err := treesitter.Parse(src, "main.go")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	blockNode, err := testutil.FindNodeByText(tree, src, "{ foo(x) }", "block")
	if err != nil {
		t.Fatalf("find block: %v", err)
	}
	if !isDelimitedOrBlockContainer(blockNode, r) {
		t.Errorf("expected block to return true")
	}

	argListNode, err := testutil.FindNodeByText(tree, src, "(x)", "argument_list")
	if err != nil {
		t.Fatalf("find argument_list: %v", err)
	}
	if !isDelimitedOrBlockContainer(argListNode, r) {
		t.Errorf("expected argument_list to return true")
	}

	identNode, err := testutil.FindNodeByText(tree, src, "foo", "identifier")
	if err != nil {
		t.Fatalf("find identifier: %v", err)
	}
	if isDelimitedOrBlockContainer(identNode, r) {
		t.Errorf("expected identifier to return false")
	}

	// r == nil fallback
	if !isDelimitedOrBlockContainer(blockNode, nil) {
		t.Errorf("expected block with nil rules to return true")
	}
}

func parseCohortSnippet(t *testing.T, fnName string, loopCount int, extraStmts ...string) (*treesitter.ASTNode, []byte) {
	t.Helper()
	var lines []string
	lines = append(lines, "package main", fmt.Sprintf("func %s() {", fnName))
	if loopCount > 0 {
		lines = append(lines, "\tfor i := 0; i < 20; i++ {")
		for i := 0; i < loopCount; i++ {
			lines = append(lines, fmt.Sprintf("\t\tprintln(%d)", i))
		}
		lines = append(lines, "\t}")
	}
	for _, stmt := range extraStmts {
		lines = append(lines, "\t"+stmt)
	}
	lines = append(lines, "}")
	src := []byte(strings.Join(lines, "\n") + "\n")
	tree, err := treesitter.Parse(src, "main.go")
	if err != nil {
		t.Fatalf("failed to parse cohort func %s: %v", fnName, err)
	}
	return tree, src
}

func TestBuildCohortProtected(t *testing.T) {
	t.Run("cohort with anchor is protected", func(t *testing.T) {
		srcTree, srcBytes := parseCohortSnippet(t, "fnOld", 20, "x := 1", "y := 2")
		dstTree, dstBytes := parseCohortSnippet(t, "fnNew", 20, "x := 1", "y := 2")

		anchorSrc, err := testutil.FindNodeByText(srcTree, srcBytes, "for i := 0; i < 20; i++", "for_statement")
		if err != nil {
			t.Fatalf("find anchorSrc: %v", err)
		}
		anchorDst, err := testutil.FindNodeByText(dstTree, dstBytes, "for i := 0; i < 20; i++", "for_statement")
		if err != nil {
			t.Fatalf("find anchorDst: %v", err)
		}

		stmt1Src, err := testutil.FindNodeByText(srcTree, srcBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Src: %v", err)
		}
		stmt1Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Dst: %v", err)
		}

		stmt2Src, err := testutil.FindNodeByText(srcTree, srcBytes, "y := 2", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt2Src: %v", err)
		}
		stmt2Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "y := 2", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt2Dst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		srcChildren := anchorSrc.PostOrder()
		dstChildren := anchorDst.PostOrder()
		for i := range min(len(srcChildren), len(dstChildren)) {
			ms.Add(srcChildren[i], dstChildren[i])
		}
		ms.Add(stmt1Src, stmt1Dst)
		ms.Add(stmt2Src, stmt2Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: anchorSrc, DestNode: anchorDst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt2Src, DestNode: stmt2Dst})

		protected := buildCohortProtected(es, ms)
		if !protected[anchorSrc] || !protected[stmt1Src] || !protected[stmt2Src] {
			t.Errorf("expected all 3 cohort moves to be protected, got: %+v", protected)
		}

		result := normalizeMovesByStructure(es, ms)
		moveCount := 0
		for _, a := range result.Actions() {
			if a.Type == actions.Move {
				moveCount++
			}
		}
		if moveCount != 3 {
			t.Errorf("expected 3 moves to survive in protected cohort, got %d", moveCount)
		}
	})

	t.Run("cohort without qualifying anchor is not protected", func(t *testing.T) {
		srcTree, srcBytes := parseCohortSnippet(t, "fnOld", 0, "x := 1", "y := 2", "z := 3")
		dstTree, dstBytes := parseCohortSnippet(t, "fnNew", 0, "x := 1", "y := 2", "z := 3")

		stmt1Src, err := testutil.FindNodeByText(srcTree, srcBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Src: %v", err)
		}
		stmt1Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Dst: %v", err)
		}

		stmt2Src, err := testutil.FindNodeByText(srcTree, srcBytes, "y := 2", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt2Src: %v", err)
		}
		stmt2Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "y := 2", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt2Dst: %v", err)
		}

		stmt3Src, err := testutil.FindNodeByText(srcTree, srcBytes, "z := 3", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt3Src: %v", err)
		}
		stmt3Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "z := 3", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt3Dst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(stmt1Src, stmt1Dst)
		ms.Add(stmt2Src, stmt2Dst)
		ms.Add(stmt3Src, stmt3Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt2Src, DestNode: stmt2Dst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt3Src, DestNode: stmt3Dst})

		protected := buildCohortProtected(es, ms)
		if len(protected) != 0 {
			t.Errorf("expected 0 protected nodes when no anchor qualifies, got %d", len(protected))
		}

		result := normalizeMovesByStructure(es, ms)
		for _, a := range result.Actions() {
			if a.Type == actions.Move {
				t.Errorf("expected small move without anchor to be demoted, but found Move for node %s (id=%d)", a.Node.Type, a.Node.ID)
			}
		}
	})

	t.Run("cohort with fewer than 3 moves is not protected", func(t *testing.T) {
		srcTree, srcBytes := parseCohortSnippet(t, "fnOld", 20, "x := 1")
		dstTree, dstBytes := parseCohortSnippet(t, "fnNew", 20, "x := 1")

		anchorSrc, err := testutil.FindNodeByText(srcTree, srcBytes, "for i := 0; i < 20; i++", "for_statement")
		if err != nil {
			t.Fatalf("find anchorSrc: %v", err)
		}
		anchorDst, err := testutil.FindNodeByText(dstTree, dstBytes, "for i := 0; i < 20; i++", "for_statement")
		if err != nil {
			t.Fatalf("find anchorDst: %v", err)
		}

		stmt1Src, err := testutil.FindNodeByText(srcTree, srcBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Src: %v", err)
		}
		stmt1Dst, err := testutil.FindNodeByText(dstTree, dstBytes, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find stmt1Dst: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		srcChildren := anchorSrc.PostOrder()
		dstChildren := anchorDst.PostOrder()
		for i := range min(len(srcChildren), len(dstChildren)) {
			ms.Add(srcChildren[i], dstChildren[i])
		}
		ms.Add(stmt1Src, stmt1Dst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: anchorSrc, DestNode: anchorDst})
		es.Add(actions.Action{Type: actions.Move, Node: stmt1Src, DestNode: stmt1Dst})

		protected := buildCohortProtected(es, ms)
		if len(protected) != 0 {
			t.Errorf("expected 0 protected nodes for cohort size < 3, got %d", len(protected))
		}
	})
}

func TestBuildCohortProtected_ContiguousMonotonicRun(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `package main

func OldHandler() {
	config := loadConfig("production", true)
	logger := initLogger(config.LogLevel, "stderr")
	client := connectDatabase(config.DBHost, config.DBPort, config.Timeout)
	session := client.CreateSession("admin", true)
	metrics := registerMetrics("service_v1", session.ID)
	worker := newWorkerPool(config.Concurrency, metrics)
	worker.Start(session.Context())
	cache := initCache(config.CacheSize, "lru")
	server := startServer(config.HTTPPort, router, cache)
	notifyStartup(logger, server.Address())
}

func Helper1() { println("h1") }
func Helper2() { println("h2") }
func Helper3() { println("h3") }
func Helper4() { println("h4") }
func Helper5() { println("h5") }
func Helper6() { println("h6") }
func Helper7() { println("h7") }
func Helper8() { println("h8") }
func Helper9() { println("h9") }
func Helper10() { println("h10") }
`
		dst := `package main

func OldHandler() {
	println("placeholder")
}

func Helper1() { println("h1") }
func Helper2() { println("h2") }
func Helper3() { println("h3") }
func Helper4() { println("h4") }
func Helper5() { println("h5") }
func Helper6() { println("h6") }
func Helper7() { println("h7") }
func Helper8() { println("h8") }
func Helper9() { println("h9") }
func Helper10() { println("h10") }

func NewExtractedHandler() {
	config := loadConfig("production", true)
	logger := initLogger(config.LogLevel, "stderr")
	client := connectDatabase(config.DBHost, config.DBPort, config.Timeout)
	session := client.CreateSession("admin", true)
	metrics := registerMetrics("service_v1", session.ID)
	worker := newWorkerPool(config.Concurrency, metrics)
	worker.Start(session.Context())
	cache := initCache(config.CacheSize, "lru")
	server := startServer(config.HTTPPort, router, cache)
	notifyStartup(logger, server.Address())
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

		part := engine.NewLinePartition([]byte(src), []byte(dst))
		mr := engine.Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		es := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		es = Run(es, mr.Mappings, srcAST, dstAST)

		// Whole-Snippet Verification:
		// 1. Enclosing context and surrounding helper functions map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
			"func OldHandler()", "func OldHandler()", "function_declaration",
		)
		for i := 1; i <= 10; i++ {
			fnName := fmt.Sprintf("func Helper%d()", i)
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
				fnName, fnName, "function_declaration",
			)
		}

		// 2. Extracted procedural statements map to destination in NewExtractedHandler.
		statements := []string{
			`config := loadConfig("production", true)`,
			`logger := initLogger(config.LogLevel, "stderr")`,
			`client := connectDatabase(config.DBHost, config.DBPort, config.Timeout)`,
			`session := client.CreateSession("admin", true)`,
			`metrics := registerMetrics("service_v1", session.ID)`,
			`worker := newWorkerPool(config.Concurrency, metrics)`,
			`worker.Start(session.Context())`,
			`cache := initCache(config.CacheSize, "lru")`,
			`server := startServer(config.HTTPPort, router, cache)`,
			`notifyStartup(logger, server.Address())`,
		}

		for _, stmtText := range statements {
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), stmtText, stmtText)
		}

		// 3. All contiguous procedural statements must survive postprocessing as Moves.
		r := rules.Get("go")
		for _, stmtText := range statements {
			node, err := testutil.FindNodeByText(srcAST, []byte(src), stmtText)
			if err != nil {
				t.Fatalf("failed to locate node for %q: %v", stmtText, err)
			}
			stmtNode := engine.FindEnclosingStatement(node, r)
			if stmtNode == nil {
				stmtNode = node
			}
			hasMove := slices.ContainsFunc(es.Actions(), func(a actions.Action) bool {
				return a.Type == actions.Move && (a.Node == stmtNode || a.Node == node)
			})
			if !hasMove {
				t.Errorf("expected Move action for extracted statement %q, but none found", stmtText)
			}
		}
	})
}
