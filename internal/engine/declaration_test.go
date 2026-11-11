package engine

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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

func TestDeclarationSignaturePreservation(t *testing.T) {
	t.Run("renamed outer function preserves parameter list instead of siphoning to inner helper", func(t *testing.T) {
		// src:
		// fn is_hidden(dent: &DirEntry) -> bool { ... body ... }
		srcParams := testutil.Node("parameters", "",
			testutil.Node("parameter", "",
				testutil.Leaf("identifier", "dent"),
				testutil.Leaf("type_identifier", "DirEntry"),
			),
		)
		srcBody := testutil.Node("block", "",
			testutil.Leaf("expr", "check_hidden()"),
			testutil.Leaf("expr", "more_code()"),
			testutil.Leaf("return", "true"),
		)
		srcFunc := testutil.Node("function_item", "",
			testutil.Leaf("identifier", "is_hidden"),
			srcParams,
			srcBody,
		)
		srcFunc.Language = "rust"
		src := testutil.Node("source_file", "", srcFunc)
		src.Language = "rust"

		// dst:
		// fn is_hidden_entry(dent: &DirEntry) -> bool {
		//     fn imp(dent: &DirEntry) -> bool { ... body ... }
		//     imp(dent)
		// }
		dstOuterParams := testutil.Node("parameters", "",
			testutil.Node("parameter", "",
				testutil.Leaf("identifier", "dent"),
				testutil.Leaf("type_identifier", "DirEntry"),
			),
		)
		dstInnerParams := testutil.Node("parameters", "",
			testutil.Node("parameter", "",
				testutil.Leaf("identifier", "dent"),
				testutil.Leaf("type_identifier", "DirEntry"),
			),
		)
		dstInnerBody := testutil.Node("block", "",
			testutil.Leaf("expr", "check_hidden()"),
			testutil.Leaf("expr", "more_code()"),
			testutil.Leaf("return", "true"),
		)
		dstInnerFunc := testutil.Node("function_item", "",
			testutil.Leaf("identifier", "imp"),
			dstInnerParams,
			dstInnerBody,
		)
		dstOuterBody := testutil.Node("block", "",
			dstInnerFunc,
			testutil.Leaf("call", "imp(dent)"),
		)
		dstOuterFunc := testutil.Node("function_item", "",
			testutil.Leaf("identifier", "is_hidden_entry"),
			dstOuterParams,
			dstOuterBody,
		)
		dstOuterFunc.Language = "rust"
		dst := testutil.Node("source_file", "", dstOuterFunc)
		dst.Language = "rust"

		r := Match(src, dst, nil, nil, nil)
		if r == nil || r.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		if r.Mappings.Src()[srcFunc] != dstOuterFunc {
			t.Errorf("outer function should match outer function: got %v, want %v", r.Mappings.Src()[srcFunc], dstOuterFunc)
		}
		if r.Mappings.Src()[srcParams] != dstOuterParams {
			t.Errorf("outer parameters should match outer parameters: got %v, want %v", r.Mappings.Src()[srcParams], dstOuterParams)
		}
		if r.Mappings.Src()[srcParams] == dstInnerParams {
			t.Errorf("outer parameters must NOT be siphoned into inner helper function")
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Enclosing Context: buildNode function
			var srcFunc, dstFunc *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildNode" {
					srcFunc = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildNode" {
					dstFunc = n
					break
				}
			}
			if srcFunc == nil || dstFunc == nil || mr.Mappings.Src()[srcFunc] != dstFunc {
				t.Fatalf("expected function buildNode to map in place, got src=%v dst=%v", srcFunc, dstFunc)
			}

			// 2. Surrounding Sequence: const intermediate and return format
			var srcInter, dstInter *treesitter.ASTNode
			var srcReturn, dstReturn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 2 {
					srcInter = n
				}
				if r.IsJumpStatement(n.Type) {
					srcReturn = n
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 2 {
					dstInter = n
				}
				if r.IsJumpStatement(n.Type) {
					dstReturn = n
				}
			}
			if srcInter == nil || dstInter == nil || mr.Mappings.Src()[srcInter] != dstInter {
				t.Errorf("expected surrounding const intermediate statement to map in place")
			}
			if srcReturn == nil || dstReturn == nil || mr.Mappings.Src()[srcReturn] != dstReturn {
				t.Errorf("expected succeeding return statement to map in place")
			}

			// 3. Target: first const addUndefined must map in place to const addUndefinedForParameter
			var srcFirstConst, dstFirstConst, dstSecondConst *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 1 {
					srcFirstConst = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 1 {
					dstFirstConst = n
				}
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 3 {
					dstSecondConst = n
				}
			}
			if srcFirstConst == nil || dstFirstConst == nil {
				t.Fatalf("missing target local var declarations: src=%v dst1=%v", srcFirstConst, dstFirstConst)
			}
			if mr.Mappings.Src()[srcFirstConst] != dstFirstConst {
				t.Errorf("expected renamed const to map in place to dst row 1, got %v (dstSecondConst=%v)",
					mr.Mappings.Src()[srcFirstConst], dstSecondConst)
			}

			// The new const at row 3 must NOT have stolen the identity of src row 1
			if mr.Mappings.Dst()[dstSecondConst] == srcFirstConst {
				t.Errorf("newly introduced const at row 3 must not steal src row 1 declaration")
			}
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Enclosing Context: buildNode function
			var srcFunc, dstFunc *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildNode" {
					srcFunc = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "buildNode" {
					dstFunc = n
					break
				}
			}
			if srcFunc == nil || dstFunc == nil || mr.Mappings.Src()[srcFunc] != dstFunc {
				t.Fatalf("expected function buildNode to map in place, got src=%v dst=%v", srcFunc, dstFunc)
			}

			// 2. Surrounding Sequence: var intermediate and return format
			var srcInter, dstInter *treesitter.ASTNode
			var srcReturn, dstReturn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 4 {
					srcInter = n
				}
				if r.IsJumpStatement(n.Type) {
					srcReturn = n
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 4 {
					dstInter = n
				}
				if r.IsJumpStatement(n.Type) {
					dstReturn = n
				}
			}
			if srcInter == nil || dstInter == nil || mr.Mappings.Src()[srcInter] != dstInter {
				t.Errorf("expected surrounding var intermediate statement to map in place")
			}
			if srcReturn == nil || dstReturn == nil || mr.Mappings.Src()[srcReturn] != dstReturn {
				t.Errorf("expected succeeding return statement to map in place")
			}

			// 3. Target: first var addUndefined must map in place to var addUndefinedForParameter
			var srcFirstVar, dstFirstVar, dstSecondVar *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 3 {
					srcFirstVar = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if dstFirstVar == nil && r.IsLocalVarDeclaration(n.Type) && n.StartRow == 3 {
					dstFirstVar = n
				}
				if dstSecondVar == nil && r.IsLocalVarDeclaration(n.Type) && n.StartRow == 5 {
					dstSecondVar = n
				}
			}
			if srcFirstVar == nil || dstFirstVar == nil {
				t.Fatalf("missing target local var declarations: src=%v dst1=%v", srcFirstVar, dstFirstVar)
			}
			if mr.Mappings.Src()[srcFirstVar] != dstFirstVar {
				t.Errorf("expected renamed var to map in place to dst row 3, got %v (dstSecondVar=%v)",
					mr.Mappings.Src()[srcFirstVar], dstSecondVar)
			}

			// The new var at row 5 must NOT have stolen the identity of src row 3
			if mr.Mappings.Dst()[dstSecondVar] == srcFirstVar {
				t.Errorf("newly introduced var at row 5 must not steal src row 3 declaration")
			}

			// 4. Stationary Operands: parameter declaration identifier
			var srcParam, dstParam *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsIdentifier(n.Type) && identifierText(n) == "declaration" && n.StartRow == 2 {
					srcParam = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsIdentifier(n.Type) && identifierText(n) == "declaration" && n.StartRow == 2 {
					dstParam = n
					break
				}
			}
			if srcParam == nil || dstParam == nil || mr.Mappings.Src()[srcParam] != dstParam {
				t.Errorf("expected parameter declaration identifier to map in place")
			}
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Enclosing Context: build_node function
			var srcFunc, dstFunc *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "build_node" {
					srcFunc = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && getDeclarationName(n) == "build_node" {
					dstFunc = n
					break
				}
			}
			if srcFunc == nil || dstFunc == nil || mr.Mappings.Src()[srcFunc] != dstFunc {
				t.Fatalf("expected function build_node to map in place, got src=%v dst=%v", srcFunc, dstFunc)
			}

			// 2. Surrounding Sequence: local intermediate and return format
			var srcInter, dstInter *treesitter.ASTNode
			var srcReturn, dstReturn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 2 {
					srcInter = n
				}
				if r.IsJumpStatement(n.Type) {
					srcReturn = n
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 2 {
					dstInter = n
				}
				if r.IsJumpStatement(n.Type) {
					dstReturn = n
				}
			}
			if srcInter == nil || dstInter == nil || mr.Mappings.Src()[srcInter] != dstInter {
				t.Errorf("expected surrounding local intermediate statement to map in place")
			}
			if srcReturn == nil || dstReturn == nil || mr.Mappings.Src()[srcReturn] != dstReturn {
				t.Errorf("expected succeeding return statement to map in place")
			}

			// 3. Target: first local add_undefined must map in place to local add_undefined_for_parameter
			var srcFirstVar, dstFirstVar, dstSecondVar *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 1 {
					srcFirstVar = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 1 {
					dstFirstVar = n
				}
				if r.IsLocalVarDeclaration(n.Type) && n.StartRow == 3 {
					dstSecondVar = n
				}
			}
			if srcFirstVar == nil || dstFirstVar == nil {
				t.Fatalf("missing target local var declarations: src=%v dst1=%v", srcFirstVar, dstFirstVar)
			}
			if mr.Mappings.Src()[srcFirstVar] != dstFirstVar {
				t.Errorf("expected renamed local var to map in place to dst row 1, got %v (dstSecondVar=%v)",
					mr.Mappings.Src()[srcFirstVar], dstSecondVar)
			}

			// The new local var at row 3 must NOT have stolen the identity of src row 1
			if mr.Mappings.Dst()[dstSecondVar] == srcFirstVar {
				t.Errorf("newly introduced local var at row 3 must not steal src row 1 declaration")
			}

			// 4. Stationary Operands: parameter declaration identifier
			var srcParam, dstParam *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsIdentifier(n.Type) && identifierText(n) == "declaration" && n.StartRow == 0 {
					srcParam = n
					break
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsIdentifier(n.Type) && identifierText(n) == "declaration" && n.StartRow == 0 {
					dstParam = n
					break
				}
			}
			if srcParam == nil || dstParam == nil || mr.Mappings.Src()[srcParam] != dstParam {
				t.Errorf("expected parameter declaration identifier to map in place")
			}
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

func assertMapped(t *testing.T, label string, m *Mapping, srcAST, dstAST *treesitter.ASTNode, srcPred, dstPred func(*treesitter.ASTNode) bool) (*treesitter.ASTNode, *treesitter.ASTNode) {
	t.Helper()
	s := findFirstNode(srcAST, srcPred)
	d := findFirstNode(dstAST, dstPred)
	if s == nil || d == nil {
		t.Fatalf("missing %s: src=%v dst=%v", label, s, d)
	}
	if m.Src()[s] != d {
		t.Errorf("expected %s to map in place, got %v", label, m.Src()[s])
	}
	return s, d
}

func assertMappedPair(t *testing.T, label string, m *Mapping, srcAST, dstAST *treesitter.ASTNode, pred func(*treesitter.ASTNode) bool) (*treesitter.ASTNode, *treesitter.ASTNode) {
	t.Helper()
	return assertMapped(t, label, m, srcAST, dstAST, pred, pred)
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
				want: []string{"get_items", "format_items", "ptr", "foo", "x"},
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
			r := rulesFor(srcAST)

			m := NewMapping()
			matchDeclarations(srcAST, dstAST, m)

			isInnerTypeSpec := func(n *treesitter.ASTNode, name string) bool {
				return r.IsDeclaration(n.Type) && getDeclarationName(n) == name &&
					!slices.ContainsFunc(n.Children, func(c *treesitter.ASTNode) bool { return r.IsDeclaration(c.Type) })
			}
			assertMappedPair(t, "Foo type_spec", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return isInnerTypeSpec(n, "Foo")
			})

			srcGrouped := findFirstNode(srcAST, func(n *treesitter.ASTNode) bool {
				return r.IsContainerDeclaration(n.Type) && len(n.Children) > 2
			})
			if srcGrouped == nil {
				t.Fatal("missing grouped type_declaration in src")
			}
			if mapped := m.Src()[srcGrouped]; mapped != nil {
				t.Errorf("expected multi-spec grouped type_declaration to stay unmapped in matchDeclarations, got %v", mapped)
			}
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
			r := rulesFor(srcAST)

			// 1. Enclosing Context: impl block
			assertMappedPair(t, "LineNumbersData impl", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsContainerDeclaration(n.Type) && !r.IsBlock(n.Type) && getDeclarationName(n) == "LineNumbersData"
			})

			// 2 & 3. Surrounding Sequence + Target methods and body block
			var srcTarget, dstTarget *treesitter.ASTNode
			for _, name := range []string{"initialize_hunk", "from_format_strings"} {
				s, d := assertMappedPair(t, name, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsDeclaration(n.Type) && getDeclarationName(n) == name
				})
				if name == "from_format_strings" {
					srcTarget, dstTarget = s, d
				}
			}

			srcBody := findFirstNode(srcTarget, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
			dstBody := findFirstNode(dstTarget, func(n *treesitter.ASTNode) bool { return r.IsBlock(n.Type) })
			if srcBody == nil || dstBody == nil || m.Src()[srcBody] != dstBody {
				t.Fatalf("expected from_format_strings body block to map in place, got %v", m.Src()[srcBody])
			}

			// 4. Stationary Operands: unchanged assignment in initialize_hunk and LineNumbersData return type
			assertMappedPair(t, "self.plus_file = plus_file", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsAssignment(n.Type)
			})
			assertMapped(t, "LineNumbersData return type identifier", m, srcAST, dstAST,
				func(n *treesitter.ASTNode) bool {
					return r.IsIdentifier(n.Type) && n.Label == "LineNumbersData" && n.StartRow == 1
				},
				func(n *treesitter.ASTNode) bool {
					return r.IsIdentifier(n.Type) && n.Label == "LineNumbersData" && n.StartRow == 4
				},
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
			r := rulesFor(srcAST)

			// 1. Surrounding Sequence: unchanged health_check function
			assertMappedPair(t, "health_check", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == "health_check"
			})

			// 2 & 3. Targets: both decorated functions and their outer decorated_definition wrappers
			for _, name := range []string{"get_users", "get_posts"} {
				assertMappedPair(t, name+" wrapper", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsDeclaration(n.Type) && !r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == name
				})
				assertMappedPair(t, name+" function", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == name
				})
			}

			// 4. Stationary Operands: route call identifier inside decorators
			assertMappedPair(t, "route decorator identifier", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsIdentifier(n.Type) && n.Label == "route"
			})
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
			r := rulesFor(srcAST)

			// 1. Module-level variable declarations
			for _, varName := range []string{"uv", "M", "iswin"} {
				assertMappedPair(t, varName, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsLocalVarDeclaration(n.Type) && getDeclarationName(n) == varName
				})
			}

			// 2 & 3. Surrounding Sequence + Target functions
			for _, target := range []struct {
				row  uint32
				name string
			}{
				{4, "M.slug"},
				{12, "M.normalize"},
			} {
				assertMappedPair(t, target.name, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsContainerDeclaration(n.Type) && n.StartRow == target.row
				})
			}

			// 4. Stationary Operands: local s = path and normalize identifier inside vim.fs.normalize(path)
			assertMappedPair(t, "local s = path", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsLocalVarDeclaration(n.Type) && getDeclarationName(n) == "s"
			})
			assertMapped(t, "vim.fs.normalize call", m, srcAST, dstAST,
				func(n *treesitter.ASTNode) bool {
					return r.IsCall(n.Type) && strings.Contains(src[n.StartByte:n.EndByte], "vim.fs.normalize")
				},
				func(n *treesitter.ASTNode) bool {
					return r.IsCall(n.Type) && strings.Contains(dst[n.StartByte:n.EndByte], "vim.fs.normalize")
				},
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
			r := rulesFor(srcAST)

			// 1 & 2. Surrounding context + Target functions must map to themselves (no stdstring collision)
			for _, name := range []string{"get_items", "format_items"} {
				assertMappedPair(t, name, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsContainerDeclaration(n.Type) && !r.IsWrapper(n.Type) && getDeclarationName(n) == name
				})
			}

			// 3 & 4. Stationary Operands inside format_items: return out
			assertMapped(t, "return out", m, srcAST, dstAST,
				func(n *treesitter.ASTNode) bool {
					return r.IsJumpStatement(n.Type) && strings.Contains(src[n.StartByte:n.EndByte], "return out")
				},
				func(n *treesitter.ASTNode) bool {
					return r.IsJumpStatement(n.Type) && strings.Contains(dst[n.StartByte:n.EndByte], "return out")
				},
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
			r := rulesFor(srcAST)

			// 1. Enclosing Context: UserController
			assertMappedPair(t, "UserController", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == "UserController"
			})

			// 2. Surrounding Sequence: fields (must resolve to entries/svc, not List/UserService)
			for _, fieldName := range []string{"entries", "svc"} {
				assertMappedPair(t, fieldName, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsDeclaration(n.Type) && !r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == fieldName
				})
			}

			// 3. Target: getUsers must map to getUsers (not collide with createTeam on UserResponse)
			assertMappedPair(t, "getUsers", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == "getUsers"
			})

			// 4. Stationary Operands: rows variable declaration
			assertMappedPair(t, "rows declaration", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsLocalVarDeclaration(n.Type) && getDeclarationName(n) == "rows"
			})
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
			r := rulesFor(srcAST)

			// 1 & 2. Surrounding Sequence + Target functions
			for _, name := range []string{"init_all", "build_node"} {
				assertMappedPair(t, name, m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return r.IsContainerDeclaration(n.Type) && getDeclarationName(n) == name
				})
			}

			// 3 & 4. Stationary Operands inside build_node: return n
			assertMapped(t, "return n", m, srcAST, dstAST,
				func(n *treesitter.ASTNode) bool {
					return r.IsJumpStatement(n.Type) && strings.Contains(src[n.StartByte:n.EndByte], "return n")
				},
				func(n *treesitter.ASTNode) bool {
					return r.IsJumpStatement(n.Type) && strings.Contains(dst[n.StartByte:n.EndByte], "return n")
				},
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
			r := rulesFor(srcAST)

			isInnerTypeSpec := func(n *treesitter.ASTNode, name string) bool {
				return r.IsDeclaration(n.Type) && getDeclarationName(n) == name &&
					!slices.ContainsFunc(n.Children, func(c *treesitter.ASTNode) bool { return r.IsDeclaration(c.Type) })
			}
			for _, name := range []string{"Foo", "Bar"} {
				assertMappedPair(t, name+" type_spec", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
					return isInnerTypeSpec(n, name)
				})
			}
			assertMappedPair(t, "package config identifier", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsIdentifier(n.Type) && n.Label == "config"
			})
			assertMappedPair(t, "Foo field A", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsIdentifier(n.Type) && n.Label == "A"
			})
			assertMappedPair(t, "Bar field B", m, srcAST, dstAST, func(n *treesitter.ASTNode) bool {
				return r.IsIdentifier(n.Type) && n.Label == "B"
			})
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			var srcNewFn, dstNewSbsFn, dstUseFullNewFn *treesitter.ASTNode
			var srcHelperFn, dstHelperFn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "new":
						srcNewFn = n
					case "other_helper":
						srcHelperFn = n
					}
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "new_sbs":
						dstNewSbsFn = n
					case "new":
						dstUseFullNewFn = n
					case "other_helper":
						dstHelperFn = n
					}
				}
			}

			if srcNewFn == nil || dstNewSbsFn == nil || dstUseFullNewFn == nil {
				t.Fatalf("missing target functions: srcNew=%v dstNewSbs=%v dstUseFullNew=%v", srcNewFn, dstNewSbsFn, dstUseFullNewFn)
			}
			if srcHelperFn == nil || dstHelperFn == nil {
				t.Fatalf("missing other_helper: src=%v dst=%v", srcHelperFn, dstHelperFn)
			}

			// other_helper is unchanged, so it should map in place.
			if m.Src()[srcHelperFn] != dstHelperFn {
				t.Errorf("expected other_helper to map in place, got %v", m.Src()[srcHelperFn])
			}

			if m.Src()[srcNewFn] == dstUseFullNewFn {
				t.Errorf("SideBySideData::new was stolen by UseFullPanelWidth::new across containers")
			}

			if m.Src()[srcNewFn] != dstNewSbsFn {
				t.Errorf("expected SideBySideData::new to map to new_sbs in place, got %v", m.Src()[srcNewFn])
			}

			if m.Dst()[dstUseFullNewFn] != nil {
				t.Errorf("expected UseFullPanelWidth::new to be unmapped, got %v", m.Dst()[dstUseFullNewFn])
			}
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			var srcMakeFn, dstMakeRenamedFn, dstBetaMakeFn *treesitter.ASTNode
			var srcHelperFn, dstHelperFn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "make":
						srcMakeFn = n
					case "helper":
						srcHelperFn = n
					}
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "make_renamed":
						dstMakeRenamedFn = n
					case "make":
						if getEnclosingContainerName(n, r) == "Beta" {
							dstBetaMakeFn = n
						}
					case "helper":
						dstHelperFn = n
					}
				}
			}

			if srcMakeFn == nil || dstMakeRenamedFn == nil || dstBetaMakeFn == nil {
				t.Fatalf("missing target functions: srcMake=%v dstMakeRenamed=%v dstBetaMake=%v", srcMakeFn, dstMakeRenamedFn, dstBetaMakeFn)
			}
			if srcHelperFn == nil || dstHelperFn == nil {
				t.Fatalf("missing helper: src=%v dst=%v", srcHelperFn, dstHelperFn)
			}

			// helper is unchanged, so it should map in place.
			if m.Src()[srcHelperFn] != dstHelperFn {
				t.Errorf("expected helper to map in place, got %v", m.Src()[srcHelperFn])
			}

			if m.Src()[srcMakeFn] == dstBetaMakeFn {
				t.Errorf("Alpha.make was stolen by Beta.make across anonymous struct containers")
			}

			if m.Src()[srcMakeFn] != dstMakeRenamedFn {
				t.Errorf("expected Alpha.make to map to make_renamed in place, got %v", m.Src()[srcMakeFn])
			}

			if m.Dst()[dstBetaMakeFn] != nil {
				t.Errorf("expected Beta.make to be an unmapped insertion, got %v", m.Dst()[dstBetaMakeFn])
			}
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

			r := rulesFor(srcAST)
			part := NewLinePartition([]byte(src), []byte(dst))
			mr := Match(srcAST, dstAST, []byte(src), []byte(dst), part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}
			m := mr.Mappings

			var srcNewFn, dstNewSbsFn, dstUseFullNewFn *treesitter.ASTNode
			var srcHelperFn, dstHelperFn *treesitter.ASTNode
			for _, n := range srcAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "New":
						srcNewFn = n
					case "OtherHelper":
						srcHelperFn = n
					}
				}
			}
			for _, n := range dstAST.PreOrder() {
				if r.IsDeclaration(n.Type) && r.IsContainerDeclaration(n.Type) {
					switch getDeclarationName(n) {
					case "NewSBS":
						dstNewSbsFn = n
					case "New":
						dstUseFullNewFn = n
					case "OtherHelper":
						dstHelperFn = n
					}
				}
			}

			if srcNewFn == nil || dstNewSbsFn == nil || dstUseFullNewFn == nil {
				t.Fatalf("missing target functions: srcNew=%v dstNewSbs=%v dstUseFullNew=%v", srcNewFn, dstNewSbsFn, dstUseFullNewFn)
			}
			if srcHelperFn == nil || dstHelperFn == nil {
				t.Fatalf("missing OtherHelper: src=%v dst=%v", srcHelperFn, dstHelperFn)
			}

			// OtherHelper is unchanged, so it should map in place.
			if m.Src()[srcHelperFn] != dstHelperFn {
				t.Errorf("expected OtherHelper to map in place, got %v", m.Src()[srcHelperFn])
			}

			if m.Src()[srcNewFn] == dstUseFullNewFn {
				t.Errorf("SideBySideData.New was stolen by UseFullPanelWidth.New across receivers")
			}

			if m.Src()[srcNewFn] != dstNewSbsFn {
				t.Errorf("expected SideBySideData.New to map to NewSBS in place, got %v", m.Src()[srcNewFn])
			}

			if m.Dst()[dstUseFullNewFn] != nil {
				t.Errorf("expected UseFullPanelWidth.New to be unmapped, got %v", m.Dst()[dstUseFullNewFn])
			}
		})
	})
}
