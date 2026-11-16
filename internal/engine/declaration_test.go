package engine

import (
	"bytes"
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestGetDeclarationName(t *testing.T) {
	t.Run("function declaration", func(t *testing.T) {
		n := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
		)
		if got := getDeclarationName(n); got != "foo" {
			t.Errorf("getDeclarationName = %q, want %q", got, "foo")
		}
	})

	t.Run("method declaration", func(t *testing.T) {
		n := testutil.Node("method_declaration", "",
			testutil.Leaf("field_identifier", "Bar"),
		)
		if got := getDeclarationName(n); got != "Bar" {
			t.Errorf("getDeclarationName = %q, want %q", got, "Bar")
		}
	})

	t.Run("nil node", func(t *testing.T) {
		if got := getDeclarationName(nil); got != "" {
			t.Errorf("getDeclarationName(nil) = %q, want empty", got)
		}
	})

	t.Run("no identifier child", func(t *testing.T) {
		n := testutil.Node("function_declaration", "",
			testutil.Leaf("block", "{}"),
		)
		if got := getDeclarationName(n); got != "" {
			t.Errorf("getDeclarationName = %q, want empty", got)
		}
	})
}

func TestMatchDeclarations(t *testing.T) {
	t.Run("matches functions with same name", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "a()"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "b()"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		dstFn := dst.Children[0]
		if !m.Has(srcFn) {
			t.Error("src function_declaration should be mapped by name")
		}
		if m.Src()[srcFn] != dstFn {
			t.Error("functions with same name should map to each other")
		}
	})

	t.Run("skips ambiguous declarations", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "a()"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "b()"),
			),
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
				testutil.Leaf("call", "c()"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		if m.Has(srcFn) {
			t.Error("should not map when multiple dst candidates exist")
		}
	})

	t.Run("does not match different names", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "foo"),
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "bar"),
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		if m.Has(srcFn) {
			t.Error("foo and bar should not be matched")
		}
	})

	t.Run("matches class declarations by name", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("class_declaration", "",
				testutil.Leaf("identifier", "User"),
				testutil.Leaf("body", "{...}"),
			),
		)
		src.Language = "java"
		dst := testutil.Node("program", "",
			testutil.Node("class_declaration", "",
				testutil.Leaf("identifier", "User"),
				testutil.Leaf("body", "{...different...}"),
			),
		)
		dst.Language = "java"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcCls := src.Children[0]
		if !m.Has(srcCls) {
			t.Error("class declarations with same name should be matched")
		}
	})

	t.Run("matches methods with same receiver type", func(t *testing.T) {
		srcParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Node("pointer_type", "",
					testutil.Leaf("type_identifier", "Type"),
				),
			),
		)
		dstParam := testutil.Node("parameter_list", "",
			testutil.Node("parameter_declaration", "",
				testutil.Node("pointer_type", "",
					testutil.Leaf("type_identifier", "Type"),
				),
			),
		)

		src := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				srcParam,
			),
		)
		src.Language = "go"
		dst := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				dstParam,
			),
		)
		dst.Language = "go"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcMeth := src.Children[0]
		if !m.Has(srcMeth) {
			t.Error("methods with same name and receiver should be matched")
		}
	})

	t.Run("matches remaining unmatched declarations when duplicate is already mapped", func(t *testing.T) {
		fn1Src := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Leaf("call", "a()"),
		)
		fn2Src := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Leaf("call", "b()"),
		)
		src := testutil.Node("program", "", fn1Src, fn2Src)
		src.Language = "go"

		fn1Dst := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Leaf("call", "a()"),
		)
		fn2Dst := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Leaf("call", "c()"),
		)
		dst := testutil.Node("program", "", fn1Dst, fn2Dst)
		dst.Language = "go"

		m := NewMapping()
		// Simulate TopDown having already matched fn1Src -> fn1Dst
		m.Add(fn1Src, fn1Dst)

		matchDeclarations(src, dst, m)

		if !m.Has(fn2Src) {
			t.Fatal("fn2Src should be matched to fn2Dst")
		}
		if got := m.Src()[fn2Src]; got != fn2Dst {
			t.Errorf("m.Src()[fn2Src] = %v, want %v", got, fn2Dst)
		}
	})
}

func TestMatchDeclarationIntegration(t *testing.T) {
	// Functions with same name but completely different bodies
	// should still match via the declaration pre-pass.
	src := testutil.Node("program", "",
		testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("block", "",
				testutil.Leaf("return_statement", "return 1"),
			),
		),
	)
	dst := testutil.Node("program", "",
		testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "foo"),
			testutil.Node("block", "",
				testutil.Leaf("call", "other()"),
				testutil.Leaf("return_statement", "return 2"),
			),
		),
	)

	r := Match(src, dst, nil, nil, nil)
	srcFn := src.Children[0]
	dstFn := dst.Children[0]
	if !r.Mappings.Has(srcFn) {
		t.Error("Match should map declarations with same name")
	}
	if r.Mappings.Src()[srcFn] != dstFn {
		t.Error("declaration pre-match should map foo -> foo")
	}
}

func TestBottomUpDeclarationNameAffinity(t *testing.T) {
	// BottomUp should skip c1 because its declaration name doesn't match t1.
	leafA1 := testutil.Leaf("id", "x")
	leafA2 := testutil.Leaf("id", "x")
	leafB1 := testutil.Leaf("id", "y")
	leafB2 := testutil.Leaf("id", "y")

	t1 := testutil.Node("function_declaration", "",
		testutil.Leaf("identifier", "foo"),
		leafA1,
		leafB1,
	)
	t1.Language = "go"

	c1 := testutil.Node("function_declaration", "",
		testutil.Leaf("identifier", "bar"),
		leafA2,
	)
	c1.Language = "go"

	c2 := testutil.Node("function_declaration", "",
		testutil.Leaf("identifier", "foo"),
		leafB2,
	)
	c2.Language = "go"

	m := NewMapping()
	m.Add(leafA1, leafA2)
	m.Add(leafB1, leafB2)

	picked := candidate(t1, []*treesitter.ASTNode{c1, c2}, m)
	if picked != c2 {
		t.Errorf("candidate() selected %v, want c2 (%v)", picked, c2)
	}
}

func TestMatchDeclarationsEquivalentTypes(t *testing.T) {
	t.Run("matches function_declaration with variable_declaration in lua", func(t *testing.T) {
		src := testutil.Node("chunk", "",
			testutil.Node("function_declaration", "",
				testutil.Leaf("identifier", "sync_once_impl"),
				testutil.Node("block", "",
					testutil.Leaf("return_statement", "return"),
				),
			),
		)
		src.Language = "lua"

		dst := testutil.Node("chunk", "",
			testutil.Node("variable_declaration", "",
				testutil.Leaf("identifier", "sync_once_impl"),
				testutil.Node("block", "",
					testutil.Leaf("return_statement", "return"),
				),
			),
		)
		dst.Language = "lua"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		srcFn := src.Children[0]
		dstFn := dst.Children[0]
		if !m.Has(srcFn) {
			t.Fatal("src function_declaration should map to dst variable_declaration under equivalent types")
		}
		if m.Src()[srcFn] != dstFn {
			t.Errorf("expected %v to map to %v, got %v", srcFn, dstFn, m.Src()[srcFn])
		}

		srcBlock := srcFn.Children[1]
		dstBlock := dstFn.Children[1]
		if m.Src()[srcBlock] != dstBlock {
			t.Errorf("expected body blocks to be matched and recovered")
		}
	})

	t.Run("disambiguates forward declaration vs full definition by size", func(t *testing.T) {
		srcForward := testutil.Node("variable_declaration", "",
			testutil.Leaf("identifier", "sync_once_impl"),
		)
		srcForward.Language = "lua"

		srcFull := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "sync_once_impl"),
			testutil.Node("parameters", ""),
			testutil.Node("block", "",
				testutil.Leaf("statement", "a"),
				testutil.Leaf("statement", "b"),
				testutil.Leaf("statement", "c"),
			),
		)
		srcFull.Language = "lua"

		src := testutil.Node("chunk", "", srcForward, srcFull)
		src.Language = "lua"

		dstFull := testutil.Node("function_declaration", "",
			testutil.Leaf("identifier", "sync_once_impl"),
			testutil.Node("parameters", ""),
			testutil.Node("block", "",
				testutil.Leaf("statement", "a"),
				testutil.Leaf("statement", "b"),
			),
		)
		dstFull.Language = "lua"
		dst := testutil.Node("chunk", "", dstFull)
		dst.Language = "lua"

		m := NewMapping()
		matchDeclarations(src, dst, m)

		if m.Has(srcForward) {
			t.Errorf("forward declaration should not steal destination function")
		}
		if !m.Has(srcFull) {
			t.Fatalf("full definition should map to destination function")
		}
		if m.Src()[srcFull] != dstFull {
			t.Errorf("expected srcFull to map to dstFull")
		}
	})
}

func TestMethodReceiverDisambiguation(t *testing.T) {
	t.Run("same name different receivers only changes one", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "TypeA"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "x = 1"),
				),
			),
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "TypeB"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "y = 2"),
				),
			),
		)
		src.Language = "go"

		dst := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "TypeA"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "x = 999"),
				),
			),
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "TypeB"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "y = 2"),
				),
			),
		)
		dst.Language = "go"

		r := Match(src, dst, nil, nil, nil)

		srcA := src.Children[0]
		srcB := src.Children[1]
		dstA := dst.Children[0]
		dstB := dst.Children[1]

		if r.Mappings.Src()[srcA] != dstA {
			t.Error("TypeA.Foo should map to TypeA.Foo")
		}
		if r.Mappings.Src()[srcB] != dstB {
			t.Error("TypeB.Foo should map to TypeB.Foo")
		}
	})

	t.Run("same name same receiver matches", func(t *testing.T) {
		src := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "Type"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "x = 1"),
				),
			),
		)
		src.Language = "go"

		dst := testutil.Node("program", "",
			testutil.Node("method_declaration", "",
				testutil.Leaf("field_identifier", "Foo"),
				testutil.Node("parameter_list", "",
					testutil.Node("parameter_declaration", "",
						testutil.Node("pointer_type", "",
							testutil.Leaf("type_identifier", "Type"),
						),
					),
				),
				testutil.Node("block", "",
					testutil.Leaf("assignment", "x = 999"),
				),
			),
		)
		dst.Language = "go"

		r := Match(src, dst, nil, nil, nil)

		srcMeth := src.Children[0]
		dstMeth := dst.Children[0]
		if r.Mappings.Src()[srcMeth] != dstMeth {
			t.Error("same name same receiver should match")
		}
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
			)

			// 2. Surrounding Sequence: const intermediate and return format
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "lexical_declaration", Line: 3},
				testutil.NodeQuery{Type: "lexical_declaration", Line: 3},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "return_statement", Line: 4},
				testutil.NodeQuery{Type: "return_statement", Line: 5},
			)

			// 3. Target: first const addUndefined must map in place to const addUndefinedForParameter
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
				testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
			)

			// The new const at line 4 must NOT have stolen the identity of src line 2
			testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
				testutil.NodeQuery{Type: "lexical_declaration", Line: 4},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
			)

			// 2. Surrounding Sequence: var intermediate and return format
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "var_declaration", Line: 5},
				testutil.NodeQuery{Type: "var_declaration", Line: 5},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "return_statement", Line: 6},
				testutil.NodeQuery{Type: "return_statement", Line: 7},
			)

			// 3. Target: first var addUndefined must map in place to var addUndefinedForParameter
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "var_declaration", Line: 4},
				testutil.NodeQuery{Type: "var_declaration", Line: 4},
			)

			// The new var at line 6 must NOT have stolen the identity of src line 4
			testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "var_declaration", Line: 4},
				testutil.NodeQuery{Type: "var_declaration", Line: 6},
			)

			// 4. Stationary Operands: parameter declaration identifier
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
				testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
			)

			// 2. Surrounding Sequence: local intermediate and return format
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 3},
				testutil.NodeQuery{Type: "variable_declaration", Line: 3},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "return_statement", Line: 4},
				testutil.NodeQuery{Type: "return_statement", Line: 5},
			)

			// 3. Target: first local add_undefined must map in place to local add_undefined_for_parameter
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
			)

			// The new local var at line 4 must NOT have stolen the identity of src line 2
			testutil.AssertNotMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
				testutil.NodeQuery{Type: "variable_declaration", Line: 4},
			)

			// 4. Stationary Operands: parameter declaration identifier
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 1, Nth: 2},
				testutil.NodeQuery{Type: "identifier", Line: 1, Nth: 2},
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

func findFirstNode(root *treesitter.ASTNode, pred func(*treesitter.ASTNode) bool) *treesitter.ASTNode {
	for _, n := range root.PreOrder() {
		if pred(n) {
			return n
		}
	}
	return nil
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

			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "type_spec", Line: 4},
				testutil.NodeQuery{Type: "type_spec", Line: 3},
			)
			testutil.AssertUnmapped(t, srcAST, m.Src(),
				testutil.NodeQuery{Type: "type_declaration", Line: 3},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "impl_item", Line: 1},
				testutil.NodeQuery{Type: "impl_item", Line: 1},
			)

			// 2 & 3. Surrounding Sequence + Target methods and body block
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 2},
				testutil.NodeQuery{Type: "function_item", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 13},
				testutil.NodeQuery{Type: "function_item", Line: 24},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "block", Line: 2},
				testutil.NodeQuery{Type: "block", Line: 5},
			)

			// 4. Stationary Operands: unchanged assignment in initialize_hunk and LineNumbersData return type
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "assignment_expression", Line: 14},
				testutil.NodeQuery{Type: "assignment_expression", Line: 25},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "type_identifier", Line: 2},
				testutil.NodeQuery{Type: "type_identifier", Line: 5},
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 1},
				testutil.NodeQuery{Type: "function_definition", Line: 1},
			)

			// 2 & 3. Targets: both decorated functions and their outer decorated_definition wrappers
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "decorated_definition", Line: 4},
				testutil.NodeQuery{Type: "decorated_definition", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 5},
				testutil.NodeQuery{Type: "function_definition", Line: 5},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "decorated_definition", Line: 9},
				testutil.NodeQuery{Type: "decorated_definition", Line: 9},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 10},
				testutil.NodeQuery{Type: "function_definition", Line: 10},
			)

			// 4. Stationary Operands: route call identifier inside decorators
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 4},
				testutil.NodeQuery{Type: "identifier", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 9},
				testutil.NodeQuery{Type: "identifier", Line: 9},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 1},
				testutil.NodeQuery{Type: "variable_declaration", Line: 1},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 3},
				testutil.NodeQuery{Type: "variable_declaration", Line: 3},
			)

			// 2 & 3. Surrounding Sequence + Target functions
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 5},
				testutil.NodeQuery{Type: "function_declaration", Line: 5},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 13},
				testutil.NodeQuery{Type: "function_declaration", Line: 13},
			)

			// 4. Stationary Operands: local s = path and vim.fs.normalize call
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 6},
				testutil.NodeQuery{Type: "variable_declaration", Line: 6},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_call", Line: 14},
				testutil.NodeQuery{Type: "function_call", Line: 14},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 2},
				testutil.NodeQuery{Type: "function_definition", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 7},
				testutil.NodeQuery{Type: "function_definition", Line: 7},
			)

			// 3 & 4. Stationary Operands inside format_items: return out
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "return_statement", Line: 12},
				testutil.NodeQuery{Type: "return_statement", Line: 12},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "class_declaration", Line: 3},
				testutil.NodeQuery{Type: "class_declaration", Line: 3},
			)

			// 2. Surrounding Sequence: fields (must resolve to entries/svc, not List/UserService)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "field_declaration", Line: 4},
				testutil.NodeQuery{Type: "field_declaration", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "field_declaration", Line: 5},
				testutil.NodeQuery{Type: "field_declaration", Line: 5},
			)

			// 3. Target: getUsers must map to getUsers (not collide with createTeam on UserResponse)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "method_declaration", Line: 7},
				testutil.NodeQuery{Type: "method_declaration", Line: 7},
			)

			// 4. Stationary Operands: rows variable declaration
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "local_variable_declaration", Line: 9},
				testutil.NodeQuery{Type: "local_variable_declaration", Line: 9},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 9},
				testutil.NodeQuery{Type: "function_definition", Line: 9},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_definition", Line: 3},
				testutil.NodeQuery{Type: "function_definition", Line: 3},
			)

			// 3 & 4. Stationary Operands inside build_node: return n
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "return_statement", Line: 6},
				testutil.NodeQuery{Type: "return_statement", Line: 6},
			)
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

			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "type_spec", Line: 4},
				testutil.NodeQuery{Type: "type_spec", Line: 3},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "type_spec", Line: 7},
				testutil.NodeQuery{Type: "type_spec", Line: 8},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "package_identifier", Line: 1},
				testutil.NodeQuery{Type: "package_identifier", Line: 1},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "field_identifier", Line: 5},
				testutil.NodeQuery{Type: "field_identifier", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "field_identifier", Line: 8},
				testutil.NodeQuery{Type: "field_identifier", Line: 9},
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 8},
				testutil.NodeQuery{Type: "function_item", Line: 8},
			)

			// 2. Target function: SideBySideData::new must map to new_sbs in place
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 4},
				testutil.NodeQuery{Type: "function_item", Line: 4},
			)

			// 3. SideBySideData::new was not stolen by UseFullPanelWidth::new across containers
			testutil.AssertNotMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 4},
				testutil.NodeQuery{Type: "function_item", Line: 16},
			)

			// 4. UseFullPanelWidth::new is unmapped
			testutil.AssertUnmapped(t, dstAST, m.Dst(),
				testutil.NodeQuery{Type: "function_item", Line: 16},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 6},
				testutil.NodeQuery{Type: "function_declaration", Line: 6},
			)

			// 2. Target function: Alpha.make must map to make_renamed in place
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 2},
				testutil.NodeQuery{Type: "function_declaration", Line: 2},
			)

			// 3. Alpha.make was not stolen by Beta.make across anonymous struct containers
			testutil.AssertNotMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 2},
				testutil.NodeQuery{Type: "function_declaration", Line: 10},
			)

			// 4. Beta.make is an unmapped insertion
			testutil.AssertUnmapped(t, dstAST, m.Dst(),
				testutil.NodeQuery{Type: "function_declaration", Line: 10},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "method_declaration", Line: 11},
				testutil.NodeQuery{Type: "method_declaration", Line: 11},
			)

			// 2. Target function: SideBySideData.New maps to NewSBS in place
			testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "method_declaration", Line: 5},
				testutil.NodeQuery{Type: "method_declaration", Line: 5},
			)

			// 3. SideBySideData.New was not stolen by UseFullPanelWidth.New across receivers
			testutil.AssertNotMapped(t, srcAST, dstAST, m.Src(),
				testutil.NodeQuery{Type: "method_declaration", Line: 5},
				testutil.NodeQuery{Type: "method_declaration", Line: 17},
			)

			// 4. UseFullPanelWidth.New is unmapped
			testutil.AssertUnmapped(t, dstAST, m.Dst(),
				testutil.NodeQuery{Type: "method_declaration", Line: 17},
			)
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
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 2},
			testutil.NodeQuery{Type: "function_item", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 55},
			testutil.NodeQuery{Type: "function_item", Line: 40},
		)

		// 2. Inserted helper compute_panel_style must be a clean insertion
		testutil.AssertUnmapped(t, dstAST, m.Dst(),
			testutil.NodeQuery{Type: "function_item", Line: 6},
		)

		// 3. Deleted sibling function right_fill_right_panel_line must be unmapped
		testutil.AssertUnmapped(t, srcAST, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 29},
		)

		// 4. Target function: right_pad_left_panel_line must map to pad_panel_line_to_width
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 6},
			testutil.NodeQuery{Type: "function_item", Line: 18},
		)

		// 5. Delimited parameters container and individual parameters map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "parameters", Line: 6},
			testutil.NodeQuery{Type: "parameters", Line: 18},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "parameter", Line: 7},
			testutil.NodeQuery{Type: "parameter", Line: 19},
		)

		// 6. Stationary body statements: function body block maps in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "block", Line: 14},
			testutil.NodeQuery{Type: "block", Line: 27},
		)
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
		mr := Match(srcAST, dstAST, oldSrc, newSrc, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		m := mr.Mappings

		// 1. Enclosing Context: Succeeding function execute_app must map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 15},
			testutil.NodeQuery{Type: "function_item", Line: 14},
		)

		// 2. Surrounding Sequence: Preceding unchanged imports map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 1},
			testutil.NodeQuery{Type: "use_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
			testutil.NodeQuery{Type: "use_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
			testutil.NodeQuery{Type: "use_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 6},
			testutil.NodeQuery{Type: "use_declaration", Line: 5},
		)

		// Deleted import std::str::FromStr must be unmapped
		testutil.AssertUnmapped(t, srcAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 4},
		)

		// Succeeding unchanged imports clap and console must map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 12},
			testutil.NodeQuery{Type: "use_declaration", Line: 11},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 13},
			testutil.NodeQuery{Type: "use_declaration", Line: 12},
		)

		// 3. Target Construct: Target use_declaration and scoped_use_list must map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "use_declaration", Line: 8},
			testutil.NodeQuery{Type: "use_declaration", Line: 7},
		)

		// Outer scoped_use_list must map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "scoped_use_list", Line: 8},
			testutil.NodeQuery{Type: "scoped_use_list", Line: 7},
		)

		// 4. Stationary Operands: Unchanged imported identifiers must map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 9},
			testutil.NodeQuery{Type: "identifier", Line: 8},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 2},
		)

		// Modified item get_args_from_env_var should map to its renamed counterpart get_args_from_env_vars
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 3},
			testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 4},
		)
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
		r := rules.Get("lua")

		// 1. Enclosing Context: check function declaration must map in place
		var fnSrc, fnDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "check" {
				fnSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "check" {
				fnDst = n
				break
			}
		}
		if fnSrc == nil || fnDst == nil || m.Src()[fnSrc] != fnDst {
			t.Fatalf("expected function check to map in place, got src=%v dst=%v", fnSrc, fnDst)
		}

		// 2. Surrounding Sequence: local start and local end_val must map in place
		needleStart := []byte("start")
		needleEnd := []byte("end_val")
		var startSrc, startDst, endSrc, endDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needleStart) {
					startSrc = n
				} else if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needleEnd) {
					endSrc = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				if bytes.Contains(newSrc[n.StartByte:n.EndByte], needleStart) {
					startDst = n
				} else if bytes.Contains(newSrc[n.StartByte:n.EndByte], needleEnd) {
					endDst = n
				}
			}
		}
		if startSrc == nil || startDst == nil || m.Src()[startSrc] != startDst {
			t.Errorf("expected preceding start declaration to map in place")
		}
		if endSrc == nil || endDst == nil || m.Src()[endSrc] != endDst {
			t.Errorf("expected succeeding end_val declaration to map in place")
		}

		// 3. Target: else_statement clause must map in place when body changes
		// On unpatched main, missing else_statement from r.Clauses caused reconcileClauseWrappers
		// to fail and drop the else clause into keyword deletion/insertion (<del>else</del>/<ins>else</ins>).
		var elseSrc, elseDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsClause(n.Type) {
				elseSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsClause(n.Type) {
				elseDst = n
				break
			}
		}
		if elseSrc == nil || elseDst == nil {
			t.Fatalf("else_statement clause missing: src=%v dst=%v", elseSrc, elseDst)
		}
		if m.Src()[elseSrc] != elseDst {
			t.Errorf("expected else_statement clause to map in place when body changes, got %v", m.Src()[elseSrc])
		}
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
		var fnSrc, fnDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "test" {
				fnSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && getDeclarationName(n) == "test" {
				fnDst = n
				break
			}
		}
		if fnSrc == nil || fnDst == nil || m.Src()[fnSrc] != fnDst {
			t.Fatalf("expected function test to map in place, got src=%v dst=%v", fnSrc, fnDst)
		}

		// 2. Surrounding Sequence: local start and local end_val must map in place
		needleStart := []byte("start")
		needleEnd := []byte("end_val")
		var startSrc, startDst, endSrc, endDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needleStart) {
					startSrc = n
				} else if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needleEnd) {
					endSrc = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) {
				if bytes.Contains(newSrc[n.StartByte:n.EndByte], needleStart) {
					startDst = n
				} else if bytes.Contains(newSrc[n.StartByte:n.EndByte], needleEnd) {
					endDst = n
				}
			}
		}
		if startSrc == nil || startDst == nil || m.Src()[startSrc] != startDst {
			t.Errorf("expected preceding start declaration to map in place")
		}
		if endSrc == nil || endDst == nil || m.Src()[endSrc] != endDst {
			t.Errorf("expected succeeding end_val declaration to map in place")
		}

		// 3. Target Statement Classification:
		// Standalone Lua function calls must be recognized as statements (r.IsStatement).
		needlePoke := []byte("poke_eventloop")
		needleEq := []byte("eq(true")
		var pokeSrc, oldEqSrc, newEqDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsStatement(n.Type) {
				if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needlePoke) {
					pokeSrc = n
				} else if bytes.Contains(oldSrc[n.StartByte:n.EndByte], needleEq) {
					oldEqSrc = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsStatement(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], needleEq) {
				newEqDst = n
				break
			}
		}
		if pokeSrc == nil {
			t.Fatalf("poke_eventloop function_call statement missing from src")
		}
		if oldEqSrc == nil {
			t.Fatalf("old eq function_call statement missing from src")
		}
		if newEqDst == nil {
			t.Fatalf("new eq function_call statement missing from dst")
		}
		if !r.IsStatement(pokeSrc.Type) || !r.IsStatement(oldEqSrc.Type) || !r.IsStatement(newEqDst.Type) {
			t.Errorf("expected function_call to be classified as a statement")
		}
		if m.Src()[oldEqSrc] == newEqDst {
			t.Errorf("expected statement boundary to prevent cross-statement callee dredging of eq")
		}
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
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "macro_definition", Line: 1},
			testutil.NodeQuery{Type: "macro_definition", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "macro_definition", Line: 9},
			testutil.NodeQuery{Type: "macro_definition", Line: 9},
		)

		// 2. The renamed macro_definition maps in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "macro_definition", Line: 5},
			testutil.NodeQuery{Type: "macro_definition", Line: 5},
		)

		// 3. The inner macro_rule maps in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "macro_rule", Line: 6},
			testutil.NodeQuery{Type: "macro_rule", Line: 6},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "trait_item", Line: 1},
			testutil.NodeQuery{Type: "trait_item", Line: 1},
		)

		// 2. Enclosing FnObject type alias and delimited type_arguments map in place
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "type_item", Line: 4},
			testutil.NodeQuery{Type: "type_item", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "type_arguments", Line: 4},
			testutil.NodeQuery{Type: "type_arguments", Line: 4},
		)

		// 3. Sibling invariant: the identical second reference_type MUST map to the second reference_type
		// without crossing over or stealing the first slot.
		testutil.AssertMapped(t, srcAST, dstAST, m.Src(),
			testutil.NodeQuery{Type: "reference_type", Line: 4, Nth: 2},
			testutil.NodeQuery{Type: "reference_type", Line: 4, Nth: 2},
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
		r := rules.Get("go")

		// Surrounding declarations must map in place.
		var pairSrc, pairDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDeclaration(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("IntPair")) {
				pairSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDeclaration(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("IntPair")) {
				pairDst = n
				break
			}
		}
		if pairSrc == nil || pairDst == nil {
			t.Fatalf("IntPair declaration missing: pairSrc=%v pairDst=%v", pairSrc, pairDst)
		}
		if m.Src()[pairSrc] != pairDst {
			t.Errorf("expected IntPair declaration to map in place, got %v", m.Src()[pairSrc])
		}

		// Delimited type_arguments container must map in place.
		var typeArgsSrc, typeArgsDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("int, int")) {
				typeArgsSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("string, int")) {
				typeArgsDst = n
				break
			}
		}
		if typeArgsSrc == nil || typeArgsDst == nil {
			t.Fatalf("type_arguments container missing: typeArgsSrc=%v typeArgsDst=%v", typeArgsSrc, typeArgsDst)
		}
		if m.Src()[typeArgsSrc] != typeArgsDst {
			t.Errorf("expected type_arguments to map in place, got %v", m.Src()[typeArgsSrc])
		}

		// Locate the two int types in oldSrc.
		var oldInt0, oldInt1, newInt1 *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(oldSrc[n.StartByte:n.EndByte], []byte("int")) {
				if oldInt0 == nil {
					oldInt0 = n
				} else if oldInt1 == nil {
					oldInt1 = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(newSrc[n.StartByte:n.EndByte], []byte("int")) {
				newInt1 = n
				break
			}
		}
		if oldInt0 == nil || oldInt1 == nil || newInt1 == nil {
			t.Fatalf("int types missing: oldInt0=%v oldInt1=%v newInt1=%v", oldInt0, oldInt1, newInt1)
		}

		if m.Src()[oldInt0] == newInt1 {
			t.Errorf("first int type crossed over to second slot")
		}
		if m.Src()[oldInt1] != newInt1 {
			t.Errorf("expected second int to map in place, got %v", m.Src()[oldInt1])
		}
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
		r := rules.Get("java")

		// Surrounding class declaration must map in place.
		var classSrc, classDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsContainerDeclaration(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("Container")) {
				classSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsContainerDeclaration(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("Container")) {
				classDst = n
				break
			}
		}
		if classSrc == nil || classDst == nil {
			t.Fatalf("Container class missing: classSrc=%v classDst=%v", classSrc, classDst)
		}
		if m.Src()[classSrc] != classDst {
			t.Errorf("expected Container class to map in place, got %v", m.Src()[classSrc])
		}

		// Delimited type_arguments container must map in place.
		var typeArgsSrc, typeArgsDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("String, String")) {
				typeArgsSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("Integer, String")) {
				typeArgsDst = n
				break
			}
		}
		if typeArgsSrc == nil || typeArgsDst == nil {
			t.Fatalf("type_arguments container missing: typeArgsSrc=%v typeArgsDst=%v", typeArgsSrc, typeArgsDst)
		}
		if m.Src()[typeArgsSrc] != typeArgsDst {
			t.Errorf("expected type_arguments to map in place, got %v", m.Src()[typeArgsSrc])
		}

		// Locate the two String type identifiers in oldSrc.
		var oldStr0, oldStr1, newStr1 *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(oldSrc[n.StartByte:n.EndByte], []byte("String")) {
				if oldStr0 == nil {
					oldStr0 = n
				} else if oldStr1 == nil {
					oldStr1 = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(newSrc[n.StartByte:n.EndByte], []byte("String")) {
				newStr1 = n
				break
			}
		}
		if oldStr0 == nil || oldStr1 == nil || newStr1 == nil {
			t.Fatalf("String types missing: oldStr0=%v oldStr1=%v newStr1=%v", oldStr0, oldStr1, newStr1)
		}

		if m.Src()[oldStr0] == newStr1 {
			t.Errorf("first String type crossed over to second slot")
		}
		if m.Src()[oldStr1] != newStr1 {
			t.Errorf("expected second String to map in place, got %v", m.Src()[oldStr1])
		}
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
		r := rules.Get("typescript")

		// Surrounding type alias must map in place.
		var aliasSrc, aliasDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsContainerDeclaration(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("PairMap")) {
				aliasSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsContainerDeclaration(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("PairMap")) {
				aliasDst = n
				break
			}
		}
		if aliasSrc == nil || aliasDst == nil {
			t.Fatalf("PairMap alias missing: aliasSrc=%v aliasDst=%v", aliasSrc, aliasDst)
		}
		if m.Src()[aliasSrc] != aliasDst {
			t.Errorf("expected PairMap alias to map in place, got %v", m.Src()[aliasSrc])
		}

		// Delimited type_arguments container must map in place.
		var typeArgsSrc, typeArgsDst *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(oldSrc[n.StartByte:n.EndByte], []byte("string, string")) {
				typeArgsSrc = n
				break
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsDelimitedContainer(n.Type) && bytes.Contains(newSrc[n.StartByte:n.EndByte], []byte("number, string")) {
				typeArgsDst = n
				break
			}
		}
		if typeArgsSrc == nil || typeArgsDst == nil {
			t.Fatalf("type_arguments container missing: typeArgsSrc=%v typeArgsDst=%v", typeArgsSrc, typeArgsDst)
		}
		if m.Src()[typeArgsSrc] != typeArgsDst {
			t.Errorf("expected type_arguments to map in place, got %v", m.Src()[typeArgsSrc])
		}

		// Locate the two string types in oldSrc.
		var oldStr0, oldStr1, newStr1 *treesitter.ASTNode
		for _, n := range srcAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(oldSrc[n.StartByte:n.EndByte], []byte("string")) {
				if oldStr0 == nil {
					oldStr0 = n
				} else if oldStr1 == nil {
					oldStr1 = n
				}
			}
		}
		for _, n := range dstAST.PreOrder() {
			if r.IsType(n.Type) && bytes.Equal(newSrc[n.StartByte:n.EndByte], []byte("string")) {
				newStr1 = n
				break
			}
		}
		if oldStr0 == nil || oldStr1 == nil || newStr1 == nil {
			t.Fatalf("string types missing: oldStr0=%v oldStr1=%v newStr1=%v", oldStr0, oldStr1, newStr1)
		}

		if m.Src()[oldStr0] == newStr1 {
			t.Errorf("first string type crossed over to second slot")
		}
		if m.Src()[oldStr1] != newStr1 {
			t.Errorf("expected second string to map in place, got %v", m.Src()[oldStr1])
		}
	})
}
