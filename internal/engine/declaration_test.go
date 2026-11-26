package engine

import (
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestGetDeclarationName(t *testing.T) {
	t.Run("function declaration", func(t *testing.T) {
		src := []byte("package main\nfunc foo() {}\n")
		tree, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		fn, err := testutil.FindNodeByText(tree, src, "func foo() {}", "function_declaration")
		if err != nil {
			t.Fatal(err)
		}
		if got := getDeclarationName(fn); got != "foo" {
			t.Errorf("getDeclarationName = %q, want %q", got, "foo")
		}
	})

	t.Run("method declaration", func(t *testing.T) {
		src := []byte("package main\ntype S struct{}\nfunc (s S) Bar() {}\n")
		tree, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		fn, err := testutil.FindNodeByText(tree, src, "func (s S) Bar() {}", "method_declaration")
		if err != nil {
			t.Fatal(err)
		}
		if got := getDeclarationName(fn); got != "Bar" {
			t.Errorf("getDeclarationName = %q, want %q", got, "Bar")
		}
	})

	t.Run("nil node", func(t *testing.T) {
		if got := getDeclarationName(nil); got != "" {
			t.Errorf("getDeclarationName(nil) = %q, want empty", got)
		}
	})

	t.Run("no identifier child", func(t *testing.T) {
		src := []byte("package main\nfunc f() { return }\n")
		tree, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		ret, err := testutil.FindNodeByText(tree, src, "return", "return_statement")
		if err != nil {
			t.Fatal(err)
		}
		if got := getDeclarationName(ret); got != "" {
			t.Errorf("getDeclarationName = %q, want empty", got)
		}
	})

	t.Run("rust destructuring let ignores rhs selector", func(t *testing.T) {
		src := []byte("fn demo(state: &State) {\n    let (x, y) = state.pos;\n}\n")
		tree, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		decl, err := testutil.FindNodeByText(tree, src, "let (x, y) = state.pos;", "let_declaration")
		if err != nil {
			t.Fatal(err)
		}
		if got := getDeclarationName(decl); got != "x" {
			t.Errorf("getDeclarationName = %q, want %q", got, "x")
		}
	})

	t.Run("rust reference trait impl skips lifetime", func(t *testing.T) {
		src := []byte("impl<'a> From<&'a str> for &'a MyType {\n    fn from(s: &'a str) -> Self { unimplemented!() }\n}\n")
		tree, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatal(err)
		}
		decl, err := testutil.FindNodeByText(tree, src, "impl<'a> From<&'a str> for &'a MyType", "impl_item")
		if err != nil {
			t.Fatal(err)
		}
		if got := getDeclarationName(decl); got != "From for MyType" {
			t.Errorf("getDeclarationName = %q, want %q", got, "From for MyType")
		}
		fn, err := testutil.FindNodeByText(tree, src, "fn from(s: &'a str) -> Self", "function_item")
		if err != nil {
			t.Fatal(err)
		}
		if got := getEnclosingContainerName(fn, rules.Get("rust")); got != "From for MyType" {
			t.Errorf("getEnclosingContainerName = %q, want %q", got, "From for MyType")
		}
	})
}

func TestMatchDeclarations(t *testing.T) {
	t.Run("matches functions with same name", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { a() }\n")
		dst := []byte("package main\nfunc foo() { b() }\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"func foo() { a() }",
			"func foo() { b() }",
			"function_declaration",
		)
	})

	t.Run("skips ambiguous declarations", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { a() }\n")
		dst := []byte("package main\nfunc foo() { b() }\nfunc foo() { c() }\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertUnmappedText(t, rootA, src, m.Src(),
			"func foo() { a() }",
			"function_declaration",
		)
	})

	t.Run("does not match different names", func(t *testing.T) {
		src := []byte("package main\nfunc foo() {}\n")
		dst := []byte("package main\nfunc bar() {}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertUnmappedText(t, rootA, src, m.Src(),
			"func foo() {}",
			"function_declaration",
		)
	})

	t.Run("matches class declarations by name", func(t *testing.T) {
		src := []byte("class User { void a() {} }\n")
		dst := []byte("class User { void b() {} }\n")

		rootA, err := treesitter.Parse(src, "test.java")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.java")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"class User { void a() {} }",
			"class User { void b() {} }",
			"class_declaration",
		)
	})

	t.Run("matches methods with same receiver type", func(t *testing.T) {
		src := []byte("package main\ntype Type struct{}\nfunc (t *Type) Foo() {}\n")
		dst := []byte("package main\ntype Type struct{}\nfunc (t *Type) Foo() {}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"func (t *Type) Foo() {}",
			"func (t *Type) Foo() {}",
			"method_declaration",
		)
	})

	t.Run("matches remaining unmatched declarations when duplicate is already mapped", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { a() }\nfunc foo() { b() }\n")
		dst := []byte("package main\nfunc foo() { a() }\nfunc foo() { c() }\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		fn1Src, err := testutil.FindNodeByText(rootA, src, "func foo() { a() }", "function_declaration")
		if err != nil {
			t.Fatal(err)
		}
		fn1Dst, err := testutil.FindNodeByText(rootB, dst, "func foo() { a() }", "function_declaration")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		m.Add(fn1Src, fn1Dst)

		matchDeclarations(rootA, rootB, m)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"func foo() { b() }",
			"func foo() { c() }",
			"function_declaration",
		)
	})
}

func TestMatchDeclarationIntegration(t *testing.T) {
	// Functions with same name but completely different bodies
	// should still match via the declaration pre-pass.
	src := []byte("package main\nfunc foo() {\n\treturn 1\n}\n")
	dst := []byte("package main\nfunc foo() {\n\tother()\n\treturn 2\n}\n")

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootB, err := treesitter.Parse(dst, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	part := NewLinePartition(src, dst)
	r := Match(rootA, rootB, src, dst, part)
	testutil.AssertMappedText(t, rootA, rootB, src, dst, r.Mappings.Src(),
		"func foo() {\n\treturn 1\n}",
		"func foo() {\n\tother()\n\treturn 2\n}",
		"function_declaration",
	)
}

func TestBottomUpDeclarationNameAffinity(t *testing.T) {
	// BottomUp should skip c1 because its declaration name doesn't match t1.
	src := []byte("package main\nfunc foo() {\n\tx := 1\n\ty := 2\n}\n")
	dst1 := []byte("package main\nfunc bar() {\n\tx := 1\n}\n")
	dst2 := []byte("package main\nfunc foo() {\n\ty := 2\n}\n")

	rootA, err := treesitter.Parse(src, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootD1, err := treesitter.Parse(dst1, "test.go")
	if err != nil {
		t.Fatal(err)
	}
	rootD2, err := treesitter.Parse(dst2, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	t1, err := testutil.FindNodeByText(rootA, src, "func foo() {\n\tx := 1\n\ty := 2\n}", "function_declaration")
	if err != nil {
		t.Fatal(err)
	}
	c1, err := testutil.FindNodeByText(rootD1, dst1, "func bar() {\n\tx := 1\n}", "function_declaration")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := testutil.FindNodeByText(rootD2, dst2, "func foo() {\n\ty := 2\n}", "function_declaration")
	if err != nil {
		t.Fatal(err)
	}

	xSrc, err := testutil.FindNodeByText(rootA, src, "x := 1", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}
	xDst, err := testutil.FindNodeByText(rootD1, dst1, "x := 1", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}
	ySrc, err := testutil.FindNodeByText(rootA, src, "y := 2", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}
	yDst, err := testutil.FindNodeByText(rootD2, dst2, "y := 2", "short_var_declaration")
	if err != nil {
		t.Fatal(err)
	}

	m := NewMapping()
	m.Add(xSrc, xDst)
	m.Add(ySrc, yDst)

	picked := candidate(t1, []*treesitter.ASTNode{c1, c2}, m)
	if picked != c2 {
		t.Errorf("candidate() selected %v, want c2 (%v)", picked, c2)
	}
}

func TestMatchDeclarationsEquivalentTypes(t *testing.T) {
	t.Run("matches function_declaration with variable_declaration in lua", func(t *testing.T) {
		src := []byte("function sync_once_impl()\n\treturn\nend\n")
		dst := []byte("local sync_once_impl = function()\n\treturn\nend\n")

		rootA, err := treesitter.Parse(src, "test.lua")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.lua")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"function sync_once_impl()\n\treturn\nend",
			"local sync_once_impl = function()\n\treturn\nend",
		)
	})

	t.Run("disambiguates forward declaration vs full definition by size", func(t *testing.T) {
		src := []byte("local sync_once_impl\nfunction sync_once_impl(a, b, c)\n\treturn a, b, c\nend\n")
		dst := []byte("function sync_once_impl(a, b)\n\treturn a, b\nend\n")

		rootA, err := treesitter.Parse(src, "test.lua")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.lua")
		if err != nil {
			t.Fatal(err)
		}

		m := NewMapping()
		matchDeclarations(rootA, rootB, m)

		testutil.AssertUnmappedText(t, rootA, src, m.Src(),
			"local sync_once_impl",
			"variable_declaration",
		)
		testutil.AssertMappedText(t, rootA, rootB, src, dst, m.Src(),
			"function sync_once_impl(a, b, c)\n\treturn a, b, c\nend",
			"function sync_once_impl(a, b)\n\treturn a, b\nend",
			"function_declaration",
		)
	})
}

func TestMethodReceiverDisambiguation(t *testing.T) {
	t.Run("same name different receivers only changes one", func(t *testing.T) {
		src := []byte("package main\ntype TypeA struct{}\ntype TypeB struct{}\nfunc (t *TypeA) Foo() {\n\tx = 1\n}\nfunc (t *TypeB) Foo() {\n\ty = 2\n}\n")
		dst := []byte("package main\ntype TypeA struct{}\ntype TypeB struct{}\nfunc (t *TypeA) Foo() {\n\tx = 999\n}\nfunc (t *TypeB) Foo() {\n\ty = 2\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(src, dst)
		r := Match(rootA, rootB, src, dst, part)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, r.Mappings.Src(),
			"func (t *TypeA) Foo() {\n\tx = 1\n}",
			"func (t *TypeA) Foo() {\n\tx = 999\n}",
			"method_declaration",
		)
		testutil.AssertMappedText(t, rootA, rootB, src, dst, r.Mappings.Src(),
			"func (t *TypeB) Foo() {\n\ty = 2\n}",
			"func (t *TypeB) Foo() {\n\ty = 2\n}",
			"method_declaration",
		)
	})

	t.Run("same name same receiver matches", func(t *testing.T) {
		src := []byte("package main\ntype Type struct{}\nfunc (t *Type) Foo() {\n\tx = 1\n}\n")
		dst := []byte("package main\ntype Type struct{}\nfunc (t *Type) Foo() {\n\tx = 999\n}\n")

		rootA, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		rootB, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		part := NewLinePartition(src, dst)
		r := Match(rootA, rootB, src, dst, part)

		testutil.AssertMappedText(t, rootA, rootB, src, dst, r.Mappings.Src(),
			"func (t *Type) Foo() {\n\tx = 1\n}",
			"func (t *Type) Foo() {\n\tx = 999\n}",
			"method_declaration",
		)
	})
}

func TestMatchDeclarations_ExcludesLocalVars(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		t.Run("TypeScript", func(t *testing.T) {
			src := `function buildNode(declaration: Declaration) {
    const addUndefined = declaration && isParameter(declaration);
    const intermediate = compute(declaration);
    return format(intermediate, addUndefined);
}
`
			dst := `function buildNode(declaration: Declaration) {
    const addUndefinedForParameter = declaration && isParameter(declaration);
    const intermediate = compute(declaration);
    const addUndefined = addUndefinedForParameter || checkOptional(declaration);
    return format(intermediate, addUndefined);
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

			// 1. Enclosing Context: buildNode function
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "function buildNode", "function buildNode")

			// 2. Surrounding Sequence: const intermediate and return format
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "const intermediate = compute(declaration);", "const intermediate = compute(declaration);")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "return format(intermediate, addUndefined);", "return format(intermediate, addUndefined);")

			// 3. Target: first const addUndefined must map in place to const addUndefinedForParameter
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "const addUndefined = declaration && isParameter(declaration);", "const addUndefinedForParameter = declaration && isParameter(declaration);")

			// The new const at line 4 must NOT have stolen the identity of src line 2
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "const addUndefined = declaration && isParameter(declaration);", "const addUndefined = addUndefinedForParameter || checkOptional(declaration);")
		})

		t.Run("Go", func(t *testing.T) {
			src := `package main

func buildNode(declaration Declaration) string {
	var addUndefined = declaration && isParameter(declaration)
	var intermediate = compute(declaration)
	return format(intermediate, addUndefined)
}
`
			dst := `package main

func buildNode(declaration Declaration) string {
	var addUndefinedForParameter = declaration && isParameter(declaration)
	var intermediate = compute(declaration)
	var addUndefined = addUndefinedForParameter || checkOptional(declaration)
	return format(intermediate, addUndefined)
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

			// 1. Enclosing Context: buildNode function
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "func buildNode", "func buildNode")

			// 2. Surrounding Sequence: var intermediate and return format
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "var intermediate = compute(declaration)", "var intermediate = compute(declaration)")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "return format(intermediate, addUndefined)", "return format(intermediate, addUndefined)")

			// 3. Target: first var addUndefined must map in place to var addUndefinedForParameter
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "var addUndefined = declaration && isParameter(declaration)", "var addUndefinedForParameter = declaration && isParameter(declaration)")

			// The new var at line 6 must NOT have stolen the identity of src line 4
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "var addUndefined = declaration && isParameter(declaration)", "var addUndefined = addUndefinedForParameter || checkOptional(declaration)")

			// 4. Stationary Operands: parameter declaration identifier
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
				testutil.TextQuery{Text: "declaration", Type: "identifier", Occurrence: 1},
				testutil.TextQuery{Text: "declaration", Type: "identifier", Occurrence: 1},
			)
		})

		t.Run("Lua", func(t *testing.T) {
			src := `function build_node(declaration)
    local add_undefined = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    return format(intermediate, add_undefined)
end
`
			dst := `function build_node(declaration)
    local add_undefined_for_parameter = declaration and is_parameter(declaration)
    local intermediate = compute(declaration)
    local add_undefined = add_undefined_for_parameter or check_optional(declaration)
    return format(intermediate, add_undefined)
end
`

			srcAST, err := treesitter.Parse([]byte(src), "test.lua")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse([]byte(dst), "test.lua")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}

			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Enclosing Context: build_node function
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "function build_node", "function build_node")

			// 2. Surrounding Sequence: local intermediate and return format
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "local intermediate = compute(declaration)", "local intermediate = compute(declaration)")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "return format(intermediate, add_undefined)", "return format(intermediate, add_undefined)")

			// 3. Target: first local add_undefined must map in place to local add_undefined_for_parameter
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "local add_undefined = declaration and is_parameter(declaration)", "local add_undefined_for_parameter = declaration and is_parameter(declaration)")

			// The new local var at line 4 must NOT have stolen the identity of src line 2
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(), "local add_undefined = declaration and is_parameter(declaration)", "local add_undefined = add_undefined_for_parameter or check_optional(declaration)")

			// 4. Stationary Operands: parameter declaration identifier
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), mr.Mappings.Src(),
				testutil.TextQuery{Text: "declaration", Type: "identifier", Occurrence: 1},
				testutil.TextQuery{Text: "declaration", Type: "identifier", Occurrence: 1},
			)
		})
	})
}

func parseSnippetForDeclTest(t *testing.T, src, filename string) *treesitter.ASTNode {
	t.Helper()
	ast, err := treesitter.Parse([]byte(src), filename)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	if ast.HasError || ast.ParseErrorCount > 0 {
		t.Fatalf("%s has %d syntax parse errors", filename, ast.ParseErrorCount)
	}
	return ast
}

func runMatchForDeclTest(t *testing.T, src, dst, filename string) (*treesitter.ASTNode, *treesitter.ASTNode, *Mapping) {
	t.Helper()
	srcAST := parseSnippetForDeclTest(t, src, filename)
	dstAST := parseSnippetForDeclTest(t, dst, filename)
	part := NewLinePartition([]byte(src), []byte(dst))
	mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}
	return srcAST, dstAST, mr.Mappings
}

func TestDeclarationNameTheft(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		tests := []struct {
			name     string
			filename string
			src      string
			want     []string
		}{
			{
				name:     "Lua_ModuleVariableDeclarationNames",
				filename: "vars.lua",
				src: `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")
`,
				want: []string{"uv", "M", "iswin"},
			},
			{
				name:     "Rust_GenericImplName",
				filename: "line_numbers.rs",
				src: `impl<'a> LineNumbersData<'a> {
    pub fn from_format_strings(s: &'a str) -> LineNumbersData<'a> {
        Self {}
    }
}
`,
				want: []string{"LineNumbersData", "from_format_strings", "s"},
			},
			{
				name:     "Python_DecoratedFunctionName",
				filename: "handlers.py",
				src: `@route("/users")
def get_users(req):
    return req
`,
				want: []string{"get_users"},
			},
			{
				name:     "Cpp_TemplateAndQualifiedReturnNames",
				filename: "render.cpp",
				src: `Vec<int> get_items() { return {}; }
std::string format_items() { return ""; }
Vec<int> *ptr;
template<typename T> void foo(T x);
`,
				want: []string{"get_items", "format_items", "ptr", "T", "foo", "x"},
			},
			{
				name:     "Java_ReturnAndFieldTypeNames",
				filename: "UserController.java",
				src: `class UserController {
    private final List<Entry<String, Integer>> entries;
    private UserService svc;
    @GetMapping("/users")
    public UserResponse getUsers(Request req) { return null; }
}
`,
				want: []string{"UserController", "entries", "svc", "getUsers", "req"},
			},
			{
				name:     "C_StructReturnFunctionName",
				filename: "nodes.c",
				src: `struct Node { int id; };
static struct Node *build_node(int id) { return 0; }
`,
				want: []string{"Node", "id", "build_node", "Node", "id"},
			},
			{
				name:     "Rust_TraitImplName",
				filename: "impls.rs",
				src: `impl<T: Clone> From<Vec<T>> for AlphaList<T> {
    fn from(items: Vec<T>) -> Self { Self {} }
}
impl<T: Clone> From<Vec<T>> for BetaList<T> {
    fn from(items: Vec<T>) -> Self { Self {} }
}
`,
				want: []string{"From for AlphaList", "from", "items", "From for BetaList", "from", "items"},
			},
			{
				name:     "Lua_ModuleMethodDeclarationNames",
				filename: "methods.lua",
				src: `local M = {}
function M.encode(tbl, opts)
    return tbl
end
function M:send(msg)
    return msg
end
`,
				want: []string{"M", "M.encode", "M:send"},
			},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				ast := parseSnippetForDeclTest(t, tc.src, tc.filename)
				decs := findDeclarations(ast)
				var names []string
				for _, d := range decs {
					names = append(names, getDeclarationName(d))
				}
				if !slices.Equal(names, tc.want) {
					t.Errorf("got declaration names %v, want %v", names, tc.want)
				}
			})
		}

		t.Run("Go_GroupedTypeSplitDoesNotPairMultiDecl", func(t *testing.T) {
			src := `package config

type (
	Foo struct {
		A int
	}
	Bar struct {
		B int
	}
)
`
			dst := `package config

type Foo struct {
	X string
}
`
			srcAST := parseSnippetForDeclTest(t, src, "config.go")
			dstAST := parseSnippetForDeclTest(t, dst, "config.go")

			m := NewMapping()
			matchDeclarations(srcAST, dstAST, m)

			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "Foo struct", "Foo struct", "type_spec")
			testutil.AssertUnmappedText(t, srcAST, []byte(src), m.Src(), "type (", "type_declaration")
		})
	})

	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust_ImplMethodMatch", func(t *testing.T) {
			src := `impl<'a> LineNumbersData<'a> {
    pub fn from_format_strings(left_format: &'a str, right_format: &'a str) -> LineNumbersData<'a> {
        Self {
            left_format_data: parse_line_number_format(left_format),
            right_format_data: parse_line_number_format(right_format),
            hunk_minus_line_number: 0,
            hunk_plus_line_number: 0,
            hunk_max_line_number_width: 0,
            plus_file: "".to_string(),
        }
    }

    pub fn initialize_hunk(&mut self, line_numbers: &[(usize, usize)], plus_file: String) {
        self.plus_file = plus_file;
    }
}
`
			dst := `impl<'a> LineNumbersData<'a> {
    pub fn from_format_strings(
        format: &'a MinusPlus<String>,
        use_full_width: ansifill::UseFullPanelWidth,
    ) -> LineNumbersData<'a> {
        let insert_center_space_on_odd_width = use_full_width.pad_width();
        Self {
            format_data: MinusPlus::new(
                format::parse_line_number_format(
                    &format[Left],
                    &*LINE_NUMBERS_PLACEHOLDER_REGEX,
                    false,
                ),
                format::parse_line_number_format(
                    &format[Right],
                    &*LINE_NUMBERS_PLACEHOLDER_REGEX,
                    insert_center_space_on_odd_width,
                ),
            ),
            ..Self::default()
        }
    }

    pub fn initialize_hunk(&mut self, line_numbers: &[(usize, usize)], plus_file: String) {
        self.plus_file = plus_file;
    }
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "line_numbers.rs")

			// 1. Enclosing Context: impl block
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "impl<'a> LineNumbersData<'a>", "impl<'a> LineNumbersData<'a>", "impl_item")

			// 2 & 3. Surrounding Sequence + Target methods and body block
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn from_format_strings", "pub fn from_format_strings", "function_item")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn initialize_hunk", "pub fn initialize_hunk", "function_item")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "Self {\n            left_format_data", "Self {\n            format_data", "block")

			// 4. Stationary Operands: unchanged assignment in initialize_hunk and LineNumbersData return type
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "self.plus_file = plus_file;", "self.plus_file = plus_file;")
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "LineNumbersData", Type: "type_identifier", Occurrence: 2},
				testutil.TextQuery{Text: "LineNumbersData", Type: "type_identifier", Occurrence: 2},
			)
		})

		t.Run("Python_DecoratedFunctionMatch", func(t *testing.T) {
			src := `def health_check():
    return {"status": "ok"}

@route("/users")
def get_users(req):
    for u in database.query("SELECT * FROM users WHERE active = 1"):
        yield u

@route("/posts")
def get_posts(req):
    for p in database.query("SELECT * FROM posts WHERE visible = 1"):
        yield p
`
			dst := `def health_check():
    return {"status": "ok"}

@route("/v2/users")
async def get_users(ctx: Context, db: AsyncSession) -> Response:
    res = await db.execute(select(UserModel).filter_by(status=Status.ACTIVE))
    return JSONResponse([row.to_dict() for row in res.scalars().all()])

@route("/v2/posts")
async def get_posts(ctx: Context, db: AsyncSession) -> Response:
    res = await db.execute(select(PostModel).filter_by(status=Status.PUBLISHED))
    return JSONResponse([row.to_dict() for row in res.scalars().all()])
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "handlers.py")

			// 1. Surrounding Sequence: unchanged health_check function
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "def health_check():", "def health_check():", "function_definition")

			// 2 & 3. Targets: both decorated functions and their outer decorated_definition wrappers
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "@route(\"/users\")\ndef get_users", "@route(\"/v2/users\")\nasync def get_users", "decorated_definition")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "def get_users(req):", "async def get_users(ctx: Context, db: AsyncSession) -> Response:", "function_definition")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "@route(\"/posts\")\ndef get_posts", "@route(\"/v2/posts\")\nasync def get_posts", "decorated_definition")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "def get_posts(req):", "async def get_posts(ctx: Context, db: AsyncSession) -> Response:", "function_definition")

			// 4. Stationary Operands: route call identifier inside decorators
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "route(\"/users\")", "route(\"/v2/users\")", "call")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "route(\"/posts\")", "route(\"/v2/posts\")", "call")
		})

		t.Run("Lua_ModuleVariableAndMethodMatch", func(t *testing.T) {
			src := `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    return vim.fs.normalize(path)
end
`
			dst := `local uv = vim.uv
local M = {}
local iswin = uv.os_uname().version:match("Windows")

function M.slug(path, opts)
    local s = path
    if iswin then
        s = s:gsub("/", "\\")
    end
    return s
end

function M.normalize(path)
    local p = vim.fs.normalize(path)
    return p
end
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "fs.lua")

			// 1. Module-level variable declarations
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "local uv = vim.uv", "local uv = vim.uv", "variable_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "local M = {}", "local M = {}", "variable_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "local iswin = uv.os_uname().version:match(\"Windows\")", "local iswin = uv.os_uname().version:match(\"Windows\")", "variable_declaration")

			// 2 & 3. Surrounding Sequence + Target functions
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "function M.slug", "function M.slug", "function_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "function M.normalize", "function M.normalize", "function_declaration")

			// 4. Stationary Operands: local s = path and vim.fs.normalize call
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "local s = path", "local s = path", "variable_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "vim.fs.normalize(path)", "vim.fs.normalize(path)", "function_call")
		})

		t.Run("Cpp_TemplateAndQualifiedReturnMatch", func(t *testing.T) {
			src := `template<typename T>
Vec<int> get_items(const Buffer<T>& buf) {
    return buf.items();
}

template<typename T>
std::string format_items(const Buffer<T>& buf) {
    std::string out = buf.header();
    for (const auto& item : buf) {
        out += item.label();
    }
    return out;
}
`
			dst := `template<typename T>
Vec<int> get_items(const Buffer<T>& buf) {
    return buf.items();
}

template<typename T>
std::string format_items(const Buffer<T>& buf, bool pretty) {
    std::string out = buf.title();
    if (pretty) {
        out += "\n";
    }
    return out;
}

template<typename T>
std::string render_report(const Summary<T>& sum) {
    return sum.text();
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "render.cpp")

			// 1 & 2. Surrounding context + Target functions must map to themselves (no stdstring collision)
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "Vec<int> get_items(const Buffer<T>& buf)", "Vec<int> get_items(const Buffer<T>& buf)", "function_definition")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "std::string format_items(const Buffer<T>& buf)", "std::string format_items(const Buffer<T>& buf, bool pretty)", "function_definition")

			// 3 & 4. Stationary Operands inside format_items: return out
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "return out;", "return out;", "return_statement")
		})

		t.Run("Java_ReturnAndFieldTypeMatch", func(t *testing.T) {
			src := `package api;

public class UserController {
    private final List<Entry<String, Integer>> entries;
    private UserService svc;

    @GetMapping("/users")
    public UserResponse getUsers(Request req) {
        List<User> rows = svc.listActive(req.orgId());
        return UserResponse.ok(rows);
    }
}
`
			dst := `package api;

public class UserController {
    private final List<Entry<String, Integer>> entries;
    private UserService svc;

    @GetMapping("/v2/users")
    public UserResponse getUsers(Context ctx, Filter filter) {
        List<User> rows = svc.fetchByFilter(ctx.tenant(), filter);
        return UserResponse.paged(rows, filter.cursor());
    }

    @PostMapping("/teams")
    public UserResponse createTeam(Context ctx) {
        return UserResponse.created(ctx.body());
    }
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "UserController.java")

			// 1. Enclosing Context: UserController
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "public class UserController", "public class UserController", "class_declaration")

			// 2. Surrounding Sequence: fields (must resolve to entries/svc, not List/UserService)
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "private final List<Entry<String, Integer>> entries;", "private final List<Entry<String, Integer>> entries;", "field_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "private UserService svc;", "private UserService svc;", "field_declaration")

			// 3. Target: getUsers must map to getUsers (not collide with createTeam on UserResponse)
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "public UserResponse getUsers(Request req)", "public UserResponse getUsers(Context ctx, Filter filter)", "method_declaration")

			// 4. Stationary Operands: rows variable declaration
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "List<User> rows =", "List<User> rows =", "local_variable_declaration")
		})

		t.Run("C_StructReturnFunctionMatch", func(t *testing.T) {
			src := `struct Node { int id; };

static struct Node *build_node(int id, const char *name) {
    struct Node *n = alloc_node(id);
    n->name = name;
    return n;
}

void init_all(void) {
    struct Node *root = build_node(1, "root");
    register_node(root);
}
`
			dst := `struct Node { int id; };

static struct Node *build_node(int id, const char *label, int flags) {
    struct Node *n = alloc_node_flagged(id, flags);
    n->label = label;
    return n;
}

void init_all(void) {
    struct Node *root = build_node(1, "root");
    register_node(root);
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "nodes.c")

			// 1 & 2. Surrounding Sequence + Target functions
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "void init_all(void)", "void init_all(void)", "function_definition")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "static struct Node *build_node(int id, const char *name)", "static struct Node *build_node(int id, const char *label, int flags)", "function_definition")

			// 3 & 4. Stationary Operands inside build_node: return n
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "return n;", "return n;", "return_statement")
		})

		t.Run("Go_GroupedTypeSplitMatch", func(t *testing.T) {
			src := `package config

type (
	Foo struct {
		A int
	}
	Bar struct {
		B int
		C string
	}
)
`
			dst := `package config

type Foo struct {
	A int
	X string
}

type Bar struct {
	B int
	C string
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "config.go")

			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "Foo struct", "Foo struct", "type_spec")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "Bar struct", "Bar struct", "type_spec")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "package config", "package config")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "A", "A", "field_identifier")
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "B", Type: "field_identifier", Occurrence: 2},
				testutil.TextQuery{Text: "B", Type: "field_identifier", Occurrence: 2},
			)
		})

		t.Run("Rust_TraitImplDistinctTargetMatch", func(t *testing.T) {
			src := `impl<T: Clone> From<Vec<T>> for AlphaList<T> {
    fn from(items: Vec<T>) -> Self {
        let len = items.len();
        Self { items, len }
    }
}

impl<T: Clone> From<Vec<T>> for BetaList<T> {
    fn from(items: Vec<T>) -> Self {
        let count = items.len();
        Self { items, count }
    }
}
`
			dst := `impl<T: Debug> Into<Vec<T>> for GammaList<T> {
    fn into(self) -> Vec<T> {
        self.raw.into_iter().filter(|x| !format!("{:?}", x).is_empty()).collect()
    }
}

impl<T: Clone> From<Vec<T>> for BetaList<T> {
    fn from(items: Vec<T>) -> Self {
        let count = items.len();
        Self { items, count }
    }
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "impls.rs")

			// 1. Surviving Impl: BetaList matches BetaList in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "impl<T: Clone> From<Vec<T>> for BetaList<T>", "impl<T: Clone> From<Vec<T>> for BetaList<T>", "impl_item")
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 2},
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 1},
			)

			// 2. Different traits and targets: AlphaList must not pair with GammaList on shared generic <T>
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "impl<T: Clone> From<Vec<T>> for AlphaList<T>", "impl<T: Debug> Into<Vec<T>> for GammaList<T>")
			testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 1},
				testutil.TextQuery{Text: "fn into(self) -> Vec<T>", Type: "function_item", Occurrence: 1},
			)

			// 3. Deleted AlphaList and Inserted GammaList must remain unmapped
			testutil.AssertUnmappedText(t, srcAST, []byte(src), m.Src(), "impl<T: Clone> From<Vec<T>> for AlphaList<T>", "impl_item")
			testutil.AssertUnmappedTextQuery(t, srcAST, []byte(src), m.Src(),
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 1},
			)
			testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "impl<T: Debug> Into<Vec<T>> for GammaList<T>", "impl_item")
			testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "fn into(self) -> Vec<T>", "function_item")
		})

		t.Run("Rust_GenericImplMethodRewriteMatch", func(t *testing.T) {
			src := `impl<T: Clone> From<Vec<T>> for AlphaList<T> {
    fn from(items: Vec<T>) -> Self {
        let mut buf = Vec::new();
        for x in items {
            buf.push(x);
        }
        Self(buf)
    }
}

impl<T: Clone> From<Vec<T>> for BetaList<T> {
    fn from(items: Vec<T>) -> Self {
        let mut out = Vec::new();
        for x in items {
            out.push(x);
        }
        Self(out)
    }
}
`
			dst := `impl<T: Debug> From<Vec<T>> for AlphaList<T> {
    fn from(raw: &[T]) -> Self {
        Self::build(raw, true)
    }
}

impl<T: Debug> From<Vec<T>> for BetaList<T> {
    fn from(raw: &[T]) -> Self {
        Self::create(raw, 0)
    }
}
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "impls.rs")

			// 1. Both generic impls match their own target rather than colliding on "T"
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "impl<T: Clone> From<Vec<T>> for AlphaList<T>", "impl<T: Debug> From<Vec<T>> for AlphaList<T>", "impl_item")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "impl<T: Clone> From<Vec<T>> for BetaList<T>", "impl<T: Debug> From<Vec<T>> for BetaList<T>", "impl_item")

			// 2. Both rewritten fn from methods match in place via their distinct enclosing impl container names
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 1},
				testutil.TextQuery{Text: "fn from(raw: &[T]) -> Self", Type: "function_item", Occurrence: 1},
			)
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "fn from(items: Vec<T>) -> Self", Type: "function_item", Occurrence: 2},
				testutil.TextQuery{Text: "fn from(raw: &[T]) -> Self", Type: "function_item", Occurrence: 2},
			)
		})

		t.Run("Lua_ModuleMethodRewriteMatch", func(t *testing.T) {
			src := `local M = {}

function M.encode(tbl, opts)
    local out = {}
    for k, v in pairs(tbl) do
        table.insert(out, k .. "=" .. tostring(v))
    end
    return table.concat(out, ",")
end

function M:send(msg)
    self.socket:write(msg)
    self.sent = self.sent + 1
end
`
			dst := `local M = {}

function M.encode(payload, config)
    assert(type(payload) == "table", "expected table")
    local serialized = json.stringify(payload, config.pretty)
    if config.compress then
        error("compression disabled")
    end
    io.write(serialized)
end

function M:send(packet, flags)
    assert(self.connected, "not connected")
    self.transport:flush(packet, flags)
end
`
			srcAST, dstAST, m := runMatchForDeclTest(t, src, dst, "api.lua")

			// 1. Enclosing Context: module table declaration
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "local M = {}", "local M = {}", "variable_declaration")

			// 2. Targets: M.encode and M:send match in place despite rewritten signatures and bodies
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "function M.encode", "function M.encode", "function_declaration")
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "function M:send", "function M:send", "function_declaration")

			// 3. Function declaration bodies match in place
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
				testutil.TextQuery{Text: "local out = {}", Type: "block", Occurrence: 1},
				testutil.TextQuery{Text: "assert(type(payload)", Type: "block", Occurrence: 1},
			)
		})
	})
}

func TestDeclarationContainerScoping(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust_ImplMethodScopingAndRenameInPlace", func(t *testing.T) {
			src := `struct SideBySideData;

impl SideBySideData {
    pub fn new(decorations_width: usize, available_terminal_width: usize) -> Self {
        let panel_width = decorations_width / 2;
        Self
    }
    pub fn other_helper(&self) -> bool {
        true
    }
}
`
			dst := `struct SideBySideData;

impl SideBySideData {
    pub fn new_sbs(decorations_width: usize, available_terminal_width: usize) -> Self {
        let panel_width = decorations_width / 2;
        Self
    }
    pub fn other_helper(&self) -> bool {
        true
    }
}

pub struct UseFullPanelWidth(pub bool);

impl UseFullPanelWidth {
    pub fn new(enabled: bool) -> Self {
        Self(enabled)
    }
}
`
			srcAST, err := treesitter.Parse([]byte(src), "sbs.rs")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse([]byte(dst), "sbs.rs")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}
			if srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
			}
			if dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
			}

			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			// 1. Enclosing context & Surrounding sequence: helper maps in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn other_helper", "pub fn other_helper", "function_item")

			// 2. Target function: SideBySideData::new must map to new_sbs in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn new(decorations_width", "pub fn new_sbs(decorations_width", "function_item")

			// 3. SideBySideData::new was not stolen by UseFullPanelWidth::new across containers
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn new(decorations_width", "pub fn new(enabled", "function_item")

			// 4. UseFullPanelWidth::new is unmapped
			testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "pub fn new(enabled", "function_item")
		})

		t.Run("Zig_AnonymousStructScopingAndRenameInPlace", func(t *testing.T) {
			src := `const Alpha = struct {
    fn make(x: u32) u32 {
        return x + 1;
    }

    fn helper() void {}
};
`
			dst := `const Alpha = struct {
    fn make_renamed(x: u32) u32 {
        return x + 1;
    }

    fn helper() void {}
};

const Beta = struct {
    fn make(x: u32) u32 {
        return x + 2;
    }
};
`
			srcAST, err := treesitter.Parse([]byte(src), "sbs.zig")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse([]byte(dst), "sbs.zig")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}
			if srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
			}
			if dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
			}

			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			// 1. Surrounding sequence: helper maps in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "fn helper() void {}", "fn helper() void {}", "function_declaration")

			// 2. Target function: Alpha.make must map to make_renamed in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "fn make(x: u32) u32 {\n        return x + 1;", "fn make_renamed(x: u32) u32 {\n        return x + 1;", "function_declaration")

			// 3. Alpha.make was not stolen by Beta.make across anonymous struct containers
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "fn make(x: u32) u32 {\n        return x + 1;", "fn make(x: u32) u32 {\n        return x + 2;", "function_declaration")

			// 4. Beta.make is an unmapped insertion
			testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "fn make(x: u32) u32 {\n        return x + 2;", "function_declaration")
		})

		t.Run("Go_MethodScopingAndRenameInPlace", func(t *testing.T) {
			src := `package main

type SideBySideData struct{}

func (s *SideBySideData) New(width int) *SideBySideData {
	panelWidth := width / 2
	_ = panelWidth
	return s
}

func (s *SideBySideData) OtherHelper() bool {
	return true
}
`
			dst := `package main

type SideBySideData struct{}

func (s *SideBySideData) NewSBS(width int) *SideBySideData {
	panelWidth := width / 2
	_ = panelWidth
	return s
}

func (s *SideBySideData) OtherHelper() bool {
	return true
}

type UseFullPanelWidth struct{}

func (u *UseFullPanelWidth) New(enabled bool) *UseFullPanelWidth {
	return u
}
`
			srcAST, err := treesitter.Parse([]byte(src), "sbs.go")
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse([]byte(dst), "sbs.go")
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}
			if srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
			}
			if dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
			}

			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			// 1. Surrounding sequence: OtherHelper maps in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "func (s *SideBySideData) OtherHelper", "func (s *SideBySideData) OtherHelper", "method_declaration")

			// 2. Target function: SideBySideData.New maps to NewSBS in place
			testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "func (s *SideBySideData) New", "func (s *SideBySideData) NewSBS", "method_declaration")

			// 3. SideBySideData.New was not stolen by UseFullPanelWidth.New across receivers
			testutil.AssertNotMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "func (s *SideBySideData) New", "func (u *UseFullPanelWidth) New", "method_declaration")

			// 4. UseFullPanelWidth.New is unmapped
			testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "func (u *UseFullPanelWidth) New", "method_declaration")
		})
	})
}

func TestDeclarationMatching_RenamedFunctionWithSiblingParameters(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := `// Top-level module
pub fn helper_alpha(x: usize) -> usize {
    x + 1
}

pub fn right_pad_left_panel_line(
    panel_line: &mut String,
    panel_line_is_empty: bool,
    line_index: Option<usize>,
    diff_style_sections: &[Vec<(Style, &str)>],
    state: &State,
    background_color_extends_to_terminal_width: Option<bool>,
    config: &Config,
) {
    if panel_line_is_empty && line_index.is_some() {
        match state {
            State::HunkMinus(_) => mark_empty_line(panel_line, " "),
            State::HunkZero => {}
            _ => unreachable!(),
        };
    };
    let text_width = measure_text_width(panel_line);
    let panel_width = config.side_by_side_data.left_panel.width;
    if text_width > panel_width {
        *panel_line = truncate_str(panel_line, panel_width);
    }
}

pub fn right_fill_right_panel_line(
    panel_line: &mut String,
    panel_line_is_empty: bool,
    line_index: Option<usize>,
    diff_style_sections: &[Vec<(Style, &str)>],
    state: &State,
    background_color_extends_to_terminal_width: Option<bool>,
    config: &Config,
) {
    *panel_line = truncate_str(panel_line, config.side_by_side_data.right_panel.width);
    let (should_right_fill_background_color, fill_style) = if let Some(index) = line_index {
        get_fill_style(diff_style_sections, state, background_color_extends_to_terminal_width, config)
    } else {
        (false, config.null_style)
    };
    if should_right_fill_background_color {
        right_fill_background_color(panel_line, fill_style);
    } else if panel_line_is_empty && line_index.is_some() {
        match state {
            State::HunkPlus(_) => mark_empty_line(panel_line, " "),
            State::HunkZero => {}
            _ => unreachable!(),
        };
    };
}

pub fn helper_omega(y: usize) -> usize {
    y * 2
}
`

		// dst introduces compute_panel_style helper above pad_panel_line_to_width so:
		// 1. lineDist from right_pad_left_panel_line (line 6) to pad_panel_line_to_width (line 27) is 21 > 10
		// 2. deleted sibling right_fill_right_panel_line (line 31) has smaller row distance (4) to pad_panel_line_to_width
		dst := `// Top-level module
pub fn helper_alpha(x: usize) -> usize {
    x + 1
}

pub fn compute_panel_style(state: &State, is_empty: bool) -> Option<&'static str> {
    if is_empty {
        match state {
            State::HunkMinus(_) => Some(" "),
            State::HunkPlus(_) => Some(" "),
            _ => None,
        }
    } else {
        None
    }
}

pub fn pad_panel_line_to_width<'a>(
    panel_line: &mut String,
    panel_line_is_empty: bool,
    line_index: Option<usize>,
    diff_style_sections: &[LineSegments<'a, Style>],
    state: &State,
    panel_side: PanelSide,
    background_color_extends_to_terminal_width: BgShouldFill,
    config: &Config,
) {
    if panel_line_is_empty && line_index.is_some() {
        if let Some(mark) = compute_panel_style(state, panel_line_is_empty) {
            mark_empty_line(panel_line, mark);
        }
    };
    let text_width = measure_text_width(panel_line);
    let panel_width = config.side_by_side_data[panel_side].width;
    if text_width > panel_width {
        *panel_line = truncate_str(panel_line, panel_width);
    }
}

pub fn helper_omega(y: usize) -> usize {
    y * 2
}
`

		srcAST, err := treesitter.Parse([]byte(src), "side_by_side.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse([]byte(dst), "side_by_side.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has %d syntax parse errors", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has %d syntax parse errors", dstAST.ParseErrorCount)
		}

		part := NewLinePartition([]byte(src), []byte(dst))
		mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		m := mr.Mappings

		// 1. Enclosing context & Surrounding sequence: helpers map in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn helper_alpha", "pub fn helper_alpha", "function_item")
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn helper_omega", "pub fn helper_omega", "function_item")

		// 2. Inserted helper compute_panel_style must be a clean insertion
		testutil.AssertUnmappedText(t, dstAST, []byte(dst), m.Dst(), "pub fn compute_panel_style", "function_item")

		// 3. Deleted sibling function right_fill_right_panel_line must be unmapped
		testutil.AssertUnmappedText(t, srcAST, []byte(src), m.Src(), "pub fn right_fill_right_panel_line", "function_item")

		// 4. Target function: right_pad_left_panel_line must map to pad_panel_line_to_width
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "pub fn right_pad_left_panel_line", "pub fn pad_panel_line_to_width", "function_item")

		// 5. Delimited parameters container and individual parameters map in place
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
			testutil.TextQuery{Text: "panel_line: &mut String", Type: "parameters", Occurrence: 1},
			testutil.TextQuery{Text: "panel_line: &mut String", Type: "parameters", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(),
			testutil.TextQuery{Text: "panel_line: &mut String", Type: "parameter", Occurrence: 1},
			testutil.TextQuery{Text: "panel_line: &mut String", Type: "parameter", Occurrence: 1},
		)

		// 6. Stationary body statements: function body block maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, []byte(src), []byte(dst), m.Src(), "{\n    if panel_line_is_empty && line_index.is_some() {\n        match state {", "{\n    if panel_line_is_empty && line_index.is_some() {\n        if let Some(mark)", "block")
	})
}

func TestRustScopedUseListModification_NoWholeBlockCollapse(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};
use std::str::FromStr;

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, get_args_from_env_var},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`)

		newSrc := []byte(`use std::collections::HashSet;
use std::env;
use std::path::{Path, PathBuf};

use atty::{self, Stream};

use crate::{
    clap_app,
    config::{get_args_from_config_file, get_args_from_env_opts_var, get_args_from_env_vars},
};
use clap::ArgMatches;
use console::Term;

pub fn execute_app() -> bool {
    clap_app();
    true
}
`)

		srcAST, err := treesitter.Parse(oldSrc, "app.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "app.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("srcAST has parse errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dstAST has parse errors: %d", dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		m := mr.Mappings

		// 1. Enclosing Context: Succeeding function execute_app must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "execute_app", "execute_app")

		// 2. Surrounding Sequence: Preceding unchanged imports map in place
		for _, name := range []string{"std::collections::HashSet", "std::env", "std::path::{Path, PathBuf}", "atty::{self, Stream}"} {
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), name, name)
		}

		// Deleted import std::str::FromStr must be unmapped
		testutil.AssertUnmappedText(t, srcAST, oldSrc, m.Src(), "FromStr", "use_declaration")

		// Succeeding unchanged imports clap and console must map in place
		for _, name := range []string{"ArgMatches", "Term"} {
			testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), name, name)
		}

		// 3. Target Construct: Target use_declaration and scoped_use_list must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "use crate", "use crate", "use_declaration")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "crate::{", "crate::{", "scoped_use_list")

		// 4. Stationary Operands: Unchanged imported identifiers must map in place
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "clap_app", Type: "identifier", Occurrence: 1},
			testutil.TextQuery{Text: "clap_app", Type: "identifier", Occurrence: 1},
		)
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "get_args_from_config_file", "get_args_from_config_file", "identifier")

		// Modified item get_args_from_env_var should map to its renamed counterpart get_args_from_env_vars
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "get_args_from_env_var", "get_args_from_env_vars", "identifier")
	})
}

func TestFullEngine_LanguageGrammarConstructs(t *testing.T) {
	t.Run("Lua_ElseStatement_ClauseRollup", func(t *testing.T) {
		oldSrc := []byte(`function check(x)
    local start = 1
    if x > 0 then
        print("positive")
    else
        do_old_thing_one()
        do_old_thing_two()
        do_old_thing_three()
    end
    local end_val = 2
end
`)
		newSrc := []byte(`function check(x)
    local start = 1
    if x > 0 then
        print("positive")
    else
        do_completely_different_new_action()
        another_unrelated_call()
    end
    local end_val = 2
end
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.lua")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.lua")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// 1. Enclosing Context: check function declaration must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "function check(x)", "function check(x)")

		// 2. Surrounding Sequence: local start and local end_val must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "local start = 1", "local start = 1")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "local end_val = 2", "local end_val = 2")

		// 3. Target: else_statement clause must map in place when body changes
		// On unpatched main, missing else_statement from r.Clauses caused reconcileClauseWrappers
		// to fail and drop the else clause into keyword deletion/insertion (<del>else</del>/<ins>else</ins>).
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "else\n        do_old_thing_one()", "else\n        do_completely_different_new_action()", "else_statement")
	})

	t.Run("Lua_FunctionCall_StatementBoundary", func(t *testing.T) {
		oldSrc := []byte(`function test()
    local start = 1
    poke_eventloop()
    eq(true, exec_capture('messages'):find('incorrect password', 1, true) ~= nil)
    local end_val = 2
end
`)
		newSrc := []byte(`function test()
    local start = 1
    eq(true, reported)
    local end_val = 2
end
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.lua")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.lua")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings
		r := rules.Get("lua")

		// 1. Enclosing Context: test function declaration must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "function test()", "function test()")

		// 2. Surrounding Sequence: local start and local end_val must map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "local start = 1", "local start = 1")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "local end_val = 2", "local end_val = 2")

		// 3. Target Statement Classification:
		// Standalone Lua function calls must be recognized as statements (r.IsStatement).
		pokeSrc, err := testutil.FindNodeByText(srcAST, oldSrc, "poke_eventloop()")
		if err != nil {
			t.Fatalf("poke_eventloop function_call statement missing from src: %v", err)
		}
		if !r.IsStatement(pokeSrc.Type) {
			t.Errorf("expected function_call to be classified as a statement, got %s", pokeSrc.Type)
		}
		testutil.AssertNotMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "eq(true, exec_capture('messages'):find('incorrect password', 1, true) ~= nil)", "eq(true, reported)")
	})

	t.Run("Rust_MacroDefinition_Renamed", func(t *testing.T) {
		oldSrc := []byte(`macro_rules! say_hello {
    () => { println!("hello"); };
}

macro_rules! empty [
    () => {};
];

macro_rules! say_bye {
    () => { println!("bye"); };
}
`)
		newSrc := []byte(`macro_rules! say_hello {
    () => { println!("hello"); };
}

macro_rules! empty_evolved [
    () => {};
];

macro_rules! say_bye {
    () => { println!("bye"); };
}
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// 1. Surrounding macros map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "macro_rules! say_hello", "macro_rules! say_hello")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "macro_rules! say_bye", "macro_rules! say_bye")

		// 2. The renamed macro_definition maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "macro_rules! empty", "macro_rules! empty_evolved")

		// 3. The inner macro_rule maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "() => {}", "() => {}", "macro_rule")
	})

	t.Run("Rust_TypeArguments_SameLineHorizontalMatching", func(t *testing.T) {
		oldSrc := []byte(`trait T: for<'a> AddAssign<&'a usize> {
}

type FnObject<'b> = dyn for<'a> FnLike<&'a isize, &'a isize> + 'b;
`)
		newSrc := []byte(`trait T: for<'a> AddAssign<&'a usize> {
}

type FnObject<'b> = dyn for<'a> FnLike<&'a isize_evolved, &'a isize> + 'b;
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.rs")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// 1. Surrounding trait declaration maps in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "trait T: for<'a> AddAssign<&'a usize>", "trait T: for<'a> AddAssign<&'a usize>")

		// 2. Enclosing FnObject type alias and delimited type_arguments map in place
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "type FnObject<'b>", "type FnObject<'b>")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "<&'a isize, &'a isize>", "<&'a isize_evolved, &'a isize>", "type_arguments")

		// 3. Sibling invariant: the identical second reference_type MUST map to the second reference_type
		// without crossing over or stealing the first slot.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "&'a isize", Type: "reference_type", Occurrence: 2},
			testutil.TextQuery{Text: "&'a isize", Type: "reference_type", Occurrence: 2},
		)
	})

	t.Run("Go_TypeArguments_SameLineHorizontalMatching", func(t *testing.T) {
		oldSrc := []byte(`package main

type Tuple[T, U any] struct{}

type IntPair = Tuple[int, int]
`)
		newSrc := []byte(`package main

type Tuple[T, U any] struct{}

type IntPair = Tuple[string, int]
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// Surrounding declarations must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "type Tuple[T, U any] struct{}", "type Tuple[T, U any] struct{}")
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "type IntPair = Tuple[int, int]", "type IntPair = Tuple[string, int]")

		// Delimited type_arguments container must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "[int, int]", "[string, int]", "type_arguments")

		// Sibling invariant: second int maps in place, first does not cross over.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "int", Type: "type_identifier", Occurrence: 1},
			testutil.TextQuery{Text: "int", Type: "type_identifier", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "int", Type: "type_identifier", Occurrence: 2},
			testutil.TextQuery{Text: "int", Type: "type_identifier", Occurrence: 1},
		)
	})

	t.Run("Java_TypeArguments_SameLineHorizontalMatching", func(t *testing.T) {
		oldSrc := []byte(`class Container {
    Map<String, String> cache;
}
`)
		newSrc := []byte(`class Container {
    Map<Integer, String> cache;
}
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.java")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.java")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// Surrounding class declaration must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "class Container", "class Container")

		// Delimited type_arguments container must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "<String, String>", "<Integer, String>", "type_arguments")

		// Sibling invariant: second String maps in place, first does not cross over.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "String", Type: "type_identifier", Occurrence: 1},
			testutil.TextQuery{Text: "String", Type: "type_identifier", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "String", Type: "type_identifier", Occurrence: 2},
			testutil.TextQuery{Text: "String", Type: "type_identifier", Occurrence: 1},
		)
	})

	t.Run("TypeScript_TypeArguments_SameLineHorizontalMatching", func(t *testing.T) {
		oldSrc := []byte(`type PairMap = Record<string, string>;
`)
		newSrc := []byte(`type PairMap = Record<number, string>;
`)
		srcAST, err := treesitter.Parse(oldSrc, "test.ts")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		dstAST, err := treesitter.Parse(newSrc, "test.ts")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		if srcAST.HasError || dstAST.HasError {
			t.Fatalf("parse errors: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := NewLinePartition(oldSrc, newSrc)
		part.AttachRoots(srcAST, dstAST)
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("nil match result")
		}
		m := mr.Mappings

		// Surrounding type alias must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "type PairMap", "type PairMap")

		// Delimited type_arguments container must map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, oldSrc, newSrc, m.Src(), "<string, string>", "<number, string>", "type_arguments")

		// Sibling invariant: second string maps in place, first does not cross over.
		testutil.AssertNotMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "string", Type: "predefined_type", Occurrence: 1},
			testutil.TextQuery{Text: "string", Type: "predefined_type", Occurrence: 1},
		)
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, oldSrc, newSrc, m.Src(),
			testutil.TextQuery{Text: "string", Type: "predefined_type", Occurrence: 2},
			testutil.TextQuery{Text: "string", Type: "predefined_type", Occurrence: 1},
		)
	})
}
