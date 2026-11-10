package engine

import (
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
