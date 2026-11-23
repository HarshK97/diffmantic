package postprocess

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestNormalizeStationaryWrapperMoves(t *testing.T) {
	t.Run("drops move when wrapper is removed", func(t *testing.T) {
		src := []byte("class A { friend void foo() {} };\n")
		dst := []byte("class A { void foo() {} };\n")
		srcTree, err := treesitter.Parse(src, "src.cpp")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.cpp")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldClass, err := testutil.FindNodeByText(srcTree, src, "class A { friend void foo() {} }", "class_specifier")
		if err != nil {
			t.Fatalf("find oldClass: %v", err)
		}
		oldFn, err := testutil.FindNodeByText(srcTree, src, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find oldFn: %v", err)
		}
		newClass, err := testutil.FindNodeByText(dstTree, dst, "class A { void foo() {} }", "class_specifier")
		if err != nil {
			t.Fatalf("find newClass: %v", err)
		}
		newFn, err := testutil.FindNodeByText(dstTree, dst, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find newFn: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldClass, newClass)
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFn, DestNode: newFn})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected 0 actions after dropping spurious move, got %d", result.Size())
		}
	})

	t.Run("drops move when wrapper is added", func(t *testing.T) {
		src := []byte("class A { void foo() {} };\n")
		dst := []byte("class A { friend void foo() {} };\n")
		srcTree, err := treesitter.Parse(src, "src.cpp")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.cpp")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldClass, err := testutil.FindNodeByText(srcTree, src, "class A { void foo() {} }", "class_specifier")
		if err != nil {
			t.Fatalf("find oldClass: %v", err)
		}
		oldFn, err := testutil.FindNodeByText(srcTree, src, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find oldFn: %v", err)
		}
		newClass, err := testutil.FindNodeByText(dstTree, dst, "class A { friend void foo() {} }", "class_specifier")
		if err != nil {
			t.Fatalf("find newClass: %v", err)
		}
		newFn, err := testutil.FindNodeByText(dstTree, dst, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find newFn: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldClass, newClass)
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFn, DestNode: newFn})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected 0 actions after dropping spurious move, got %d", result.Size())
		}
	})

	t.Run("preserves genuine move across different containers", func(t *testing.T) {
		src := []byte("class A { void foo() {} };\n")
		dst := []byte("class B { void foo() {} };\n")
		srcTree, err := treesitter.Parse(src, "src.cpp")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.cpp")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldFn, err := testutil.FindNodeByText(srcTree, src, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find oldFn: %v", err)
		}
		newFn, err := testutil.FindNodeByText(dstTree, dst, "void foo() {}", "function_definition")
		if err != nil {
			t.Fatalf("find newFn: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFn, DestNode: newFn})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected 1 action preserved for genuine move, got %d", result.Size())
		}
	})

	t.Run("drops move when Go var_declaration is converted to short_var_declaration", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tvar fc = func() {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tfc := func() {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tvar fc = func() {}\n}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tfc := func() {}\n}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}
		oldVarSpec, err := testutil.FindNodeByText(srcTree, src, "fc = func() {}", "var_spec")
		if err != nil {
			t.Fatalf("find oldVarSpec: %v", err)
		}
		newShortVar, err := testutil.FindNodeByText(dstTree, dst, "fc := func() {}", "short_var_declaration")
		if err != nil {
			t.Fatalf("find newShortVar: %v", err)
		}
		oldID, err := testutil.FindNodeByText(srcTree, src, "fc", "identifier")
		if err != nil {
			t.Fatalf("find oldID: %v", err)
		}
		newID, err := testutil.FindNodeByText(dstTree, dst, "fc", "identifier")
		if err != nil {
			t.Fatalf("find newID: %v", err)
		}
		oldFn, err := testutil.FindNodeByText(srcTree, src, "func() {}", "func_literal")
		if err != nil {
			t.Fatalf("find oldFn: %v", err)
		}
		newFn, err := testutil.FindNodeByText(dstTree, dst, "func() {}", "func_literal")
		if err != nil {
			t.Fatalf("find newFn: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		ms.Add(oldVarSpec, newShortVar)
		ms.Add(oldID, newID)
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldVarSpec, DestNode: newShortVar, Subtree: true})
		es.Add(actions.Action{Type: actions.Move, Node: oldID, DestNode: newID})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected 0 actions after dropping spurious moves, got %d", result.Size())
		}
	})

	t.Run("preserves Move when arguments are swapped within mapped argument_list", func(t *testing.T) {
		src := []byte("package main\nfunc f() { call(info.Mode().Perm(), mode) }\n")
		dst := []byte("package main\nfunc f() { call(mode, info.Mode().Perm()) }\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldArgs, err := testutil.FindNodeByText(srcTree, src, "(info.Mode().Perm(), mode)", "argument_list")
		if err != nil {
			t.Fatalf("find oldArgs: %v", err)
		}
		newArgs, err := testutil.FindNodeByText(dstTree, dst, "(mode, info.Mode().Perm())", "argument_list")
		if err != nil {
			t.Fatalf("find newArgs: %v", err)
		}
		oldArg1, err := testutil.FindNodeByText(srcTree, src, "info.Mode().Perm()", "call_expression")
		if err != nil {
			t.Fatalf("find oldArg1: %v", err)
		}
		oldArg2, err := testutil.FindNodeByText(srcTree, src, "mode", "identifier")
		if err != nil {
			t.Fatalf("find oldArg2: %v", err)
		}
		newArg1, err := testutil.FindNodeByText(dstTree, dst, "mode", "identifier")
		if err != nil {
			t.Fatalf("find newArg1: %v", err)
		}
		newArg2, err := testutil.FindNodeByText(dstTree, dst, "info.Mode().Perm()", "call_expression")
		if err != nil {
			t.Fatalf("find newArg2: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldArgs, newArgs)
		ms.Add(oldArg1, newArg2)
		ms.Add(oldArg2, newArg1)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldArg1, DestNode: newArg2})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on swapped argument to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("drops move when TypeScript declaration is wrapped in export_statement", func(t *testing.T) {
		src := []byte("type Foo = number;\n")
		dst := []byte("export type Foo = number;\n")
		srcTree, err := treesitter.Parse(src, "src.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.ts")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldTypeDecl, err := testutil.FindNodeByText(srcTree, src, "type Foo = number;", "type_alias_declaration")
		if err != nil {
			t.Fatalf("find oldTypeDecl: %v", err)
		}
		newTypeDecl, err := testutil.FindNodeByText(dstTree, dst, "type Foo = number;", "type_alias_declaration")
		if err != nil {
			t.Fatalf("find newTypeDecl: %v", err)
		}
		newExportKw, err := testutil.FindNodeByText(dstTree, dst, "export", "export")
		if err != nil {
			t.Fatalf("find newExportKw: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcTree, dstTree)
		ms.Add(oldTypeDecl, newTypeDecl)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: newExportKw})
		es.Add(actions.Action{Type: actions.Move, Node: oldTypeDecl, DestNode: newTypeDecl, Subtree: true})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on type_alias_declaration to be dropped, leaving 1 action (insert), got %d actions", result.Size())
		}
		if result.Actions()[0].Type != actions.Insert || result.Actions()[0].Node != newExportKw {
			t.Fatalf("expected Insert(export) action to survive, got %+v", result.Actions()[0])
		}
	})

	t.Run("preserves Move when variable declaration is hoisted out of if_statement", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif r := rulesFor(t1); r != nil {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tr := rulesFor(t1)\n\tif r != nil {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tif r := rulesFor(t1); r != nil {}\n}", "block")
		if err != nil {
			t.Fatalf("find oldBlock: %v", err)
		}
		oldIf, err := testutil.FindNodeByText(srcTree, src, "if r := rulesFor(t1); r != nil {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		oldDecl, err := testutil.FindNodeByText(srcTree, src, "r := rulesFor(t1)", "short_var_declaration")
		if err != nil {
			t.Fatalf("find oldDecl: %v", err)
		}

		newBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tr := rulesFor(t1)\n\tif r != nil {}\n}", "block")
		if err != nil {
			t.Fatalf("find newBlock: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if r != nil {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		newDecl, err := testutil.FindNodeByText(dstTree, dst, "r := rulesFor(t1)", "short_var_declaration")
		if err != nil {
			t.Fatalf("find newDecl: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldBlock, newBlock)
		ms.Add(oldIf, newIf)
		ms.Add(oldDecl, newDecl)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldDecl, DestNode: newDecl, Subtree: true})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on hoisted variable declaration to be preserved, got %d actions", result.Size())
		}
		if result.Actions()[0].Type != actions.Move || result.Actions()[0].Node != oldDecl {
			t.Fatalf("expected Move(short_var_declaration) action to survive, got %+v", result.Actions()[0])
		}
	})

	t.Run("preserves Move when expression is passed into a new function argument list", func(t *testing.T) {
		src := []byte("call(localId.toString());\n")
		dst := []byte("outer(nested(localId.toString()));\n")
		srcTree, err := treesitter.Parse(src, "src.js")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.js")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldExpr, err := testutil.FindNodeByText(srcTree, src, "localId.toString()", "call_expression")
		if err != nil {
			t.Fatalf("find oldExpr: %v", err)
		}
		newExpr, err := testutil.FindNodeByText(dstTree, dst, "localId.toString()", "call_expression")
		if err != nil {
			t.Fatalf("find newExpr: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldExpr, newExpr)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldExpr, DestNode: newExpr})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on expression passed into new argument list to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves Move across lines when argument is wrapped in nested function calls", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tpack(\n\t\tlocalId.toString(),\n\t)\n}\n")
		dst := []byte("package main\nfunc f() {\n\thex2bin(\n\t\tthisCall(\n\t\t\tlocalId.toString(),\n\t\t),\n\t)\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldCall, err := testutil.FindNodeByText(srcTree, src, "pack(\n\t\tlocalId.toString(),\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find oldCall: %v", err)
		}
		newOuterCall, err := testutil.FindNodeByText(dstTree, dst, "hex2bin(\n\t\tthisCall(\n\t\t\tlocalId.toString(),\n\t\t),\n\t)", "call_expression")
		if err != nil {
			t.Fatalf("find newOuterCall: %v", err)
		}
		oldExpr, err := testutil.FindNodeByText(srcTree, src, "localId.toString()", "call_expression")
		if err != nil {
			t.Fatalf("find oldExpr: %v", err)
		}
		newExpr, err := testutil.FindNodeByText(dstTree, dst, "localId.toString()", "call_expression")
		if err != nil {
			t.Fatalf("find newExpr: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldCall, newOuterCall)
		ms.Add(oldExpr, newExpr)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldExpr, DestNode: newExpr})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on expression wrapped in nested call across lines to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves Move when identifier moves from named import to namespace import", func(t *testing.T) {
		src := []byte("import { util } from './module';\n")
		dst := []byte("import * as util from './module';\n")
		srcTree, err := treesitter.Parse(src, "src.ts")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.ts")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldImport, err := testutil.FindNodeByText(srcTree, src, "import { util } from './module';", "import_statement")
		if err != nil {
			t.Fatalf("find oldImport: %v", err)
		}
		newImport, err := testutil.FindNodeByText(dstTree, dst, "import * as util from './module';", "import_statement")
		if err != nil {
			t.Fatalf("find newImport: %v", err)
		}
		oldID, err := testutil.FindNodeByText(srcTree, src, "util", "identifier")
		if err != nil {
			t.Fatalf("find oldID: %v", err)
		}
		newID, err := testutil.FindNodeByText(dstTree, dst, "util", "identifier")
		if err != nil {
			t.Fatalf("find newID: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldImport, newImport)
		ms.Add(oldID, newID)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldID, DestNode: newID})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on identifier moving from named to namespace import to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves Move when expression moves from RHS to LHS operand slot in comparison", func(t *testing.T) {
		src := []byte("package main\nfunc f() { _ = streamPos != argParam }\n")
		dst := []byte("package main\nfunc f() { _ = call(argParam) != posVal }\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldBin, err := testutil.FindNodeByText(srcTree, src, "streamPos != argParam", "binary_expression")
		if err != nil {
			t.Fatalf("find oldBin: %v", err)
		}
		newBin, err := testutil.FindNodeByText(dstTree, dst, "call(argParam) != posVal", "binary_expression")
		if err != nil {
			t.Fatalf("find newBin: %v", err)
		}
		oldRHS, err := testutil.FindNodeByText(srcTree, src, "argParam", "identifier")
		if err != nil {
			t.Fatalf("find oldRHS: %v", err)
		}
		newLHS, err := testutil.FindNodeByText(dstTree, dst, "argParam", "identifier")
		if err != nil {
			t.Fatalf("find newLHS: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldBin, newBin)
		ms.Add(oldRHS, newLHS)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldRHS, DestNode: newLHS})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action on expression moving from RHS to LHS operand slot to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("drops Move when operand stays stationary within expanding conditional binary expression", func(t *testing.T) {
		src := []byte("void f() { if (server.allow_access_expired) {} }\n")
		dst := []byte("void f() { if (subtractExpiredFields && server.allow_access_expired) {} }\n")
		srcTree, err := treesitter.Parse(src, "src.c")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.c")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldIf, err := testutil.FindNodeByText(srcTree, src, "if (server.allow_access_expired) {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if (subtractExpiredFields && server.allow_access_expired) {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		oldParen, err := testutil.FindNodeByText(srcTree, src, "(server.allow_access_expired)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find oldParen: %v", err)
		}
		newParen, err := testutil.FindNodeByText(dstTree, dst, "(subtractExpiredFields && server.allow_access_expired)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find newParen: %v", err)
		}
		oldField, err := testutil.FindNodeByText(srcTree, src, "server.allow_access_expired", "field_expression")
		if err != nil {
			t.Fatalf("find oldField: %v", err)
		}
		newField, err := testutil.FindNodeByText(dstTree, dst, "server.allow_access_expired", "field_expression")
		if err != nil {
			t.Fatalf("find newField: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldIf, newIf)
		ms.Add(oldParen, newParen)
		ms.Add(oldField, newField)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldField, DestNode: newField})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected stationary operand move to be dropped, got %d actions", result.Size())
		}
	})

	t.Run("drops Move when binary condition expression is pruned in Go if statement", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif (a || b) || r.IsCall() {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tif a || b {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldIf, err := testutil.FindNodeByText(srcTree, src, "if (a || b) || r.IsCall() {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if a || b {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		oldInnerBin, err := testutil.FindNodeByText(srcTree, src, "a || b", "binary_expression")
		if err != nil {
			t.Fatalf("find oldInnerBin: %v", err)
		}
		newCondition, err := testutil.FindNodeByText(dstTree, dst, "a || b", "binary_expression")
		if err != nil {
			t.Fatalf("find newCondition: %v", err)
		}
		oldBlock, err := testutil.FindNodeByText(srcTree, src, "{}", "block")
		if err != nil {
			t.Fatalf("find oldBlock: %v", err)
		}
		newBlock, err := testutil.FindNodeByText(dstTree, dst, "{}", "block")
		if err != nil {
			t.Fatalf("find newBlock: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldIf, newIf)
		ms.Add(oldInnerBin, newCondition)
		ms.Add(oldBlock, newBlock)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldInnerBin, DestNode: newCondition, Subtree: true})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected pruned condition move to be dropped, got %d actions", result.Size())
		}
	})

	t.Run("drops Move when boolean operator precedence or parens rebalance within if condition", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif (a == 1 || b <= 2) || c() {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tif a == 1 && (b <= 2 || c()) {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldOuterBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tif (a == 1 || b <= 2) || c() {}\n}", "block")
		if err != nil {
			t.Fatalf("find oldOuterBlock: %v", err)
		}
		newOuterBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tif a == 1 && (b <= 2 || c()) {}\n}", "block")
		if err != nil {
			t.Fatalf("find newOuterBlock: %v", err)
		}
		oldIf, err := testutil.FindNodeByText(srcTree, src, "if (a == 1 || b <= 2) || c() {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if a == 1 && (b <= 2 || c()) {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		oldA, err := testutil.FindNodeByText(srcTree, src, "a == 1", "binary_expression")
		if err != nil {
			t.Fatalf("find oldA: %v", err)
		}
		newA, err := testutil.FindNodeByText(dstTree, dst, "a == 1", "binary_expression")
		if err != nil {
			t.Fatalf("find newA: %v", err)
		}
		oldB, err := testutil.FindNodeByText(srcTree, src, "b <= 2", "binary_expression")
		if err != nil {
			t.Fatalf("find oldB: %v", err)
		}
		newB, err := testutil.FindNodeByText(dstTree, dst, "b <= 2", "binary_expression")
		if err != nil {
			t.Fatalf("find newB: %v", err)
		}
		oldC, err := testutil.FindNodeByText(srcTree, src, "c()", "call_expression")
		if err != nil {
			t.Fatalf("find oldC: %v", err)
		}
		newC, err := testutil.FindNodeByText(dstTree, dst, "c()", "call_expression")
		if err != nil {
			t.Fatalf("find newC: %v", err)
		}
		oldLeftBin, err := testutil.FindNodeByText(srcTree, src, "a == 1 || b <= 2", "binary_expression")
		if err != nil {
			t.Fatalf("find oldLeftBin: %v", err)
		}
		newRightBin, err := testutil.FindNodeByText(dstTree, dst, "b <= 2 || c()", "binary_expression")
		if err != nil {
			t.Fatalf("find newRightBin: %v", err)
		}
		oldRootBin, err := testutil.FindNodeByText(srcTree, src, "(a == 1 || b <= 2) || c()", "binary_expression")
		if err != nil {
			t.Fatalf("find oldRootBin: %v", err)
		}
		newRootBin, err := testutil.FindNodeByText(dstTree, dst, "a == 1 && (b <= 2 || c())", "binary_expression")
		if err != nil {
			t.Fatalf("find newRootBin: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldOuterBlock, newOuterBlock)
		ms.Add(oldIf, newIf)
		ms.Add(oldA, newA)
		ms.Add(oldB, newB)
		ms.Add(oldC, newC)
		ms.Add(oldLeftBin, newRootBin)
		ms.Add(oldRootBin, newRightBin)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldLeftBin, DestNode: newRootBin, Subtree: true})
		es.Add(actions.Action{Type: actions.Move, Node: oldRootBin, DestNode: newRightBin, Subtree: true})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected stationary expression moves to be dropped, got %d actions", result.Size())
		}
	})

	t.Run("preserves Move when operands are swapped across intermediate expressions", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif a == 1 || b == 2 {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tif (b == 2) || a == 1 {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldOuterBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tif a == 1 || b == 2 {}\n}", "block")
		if err != nil {
			t.Fatalf("find oldOuterBlock: %v", err)
		}
		newOuterBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tif (b == 2) || a == 1 {}\n}", "block")
		if err != nil {
			t.Fatalf("find newOuterBlock: %v", err)
		}
		oldIf, err := testutil.FindNodeByText(srcTree, src, "if a == 1 || b == 2 {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if (b == 2) || a == 1 {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		oldA, err := testutil.FindNodeByText(srcTree, src, "a == 1", "binary_expression")
		if err != nil {
			t.Fatalf("find oldA: %v", err)
		}
		newA, err := testutil.FindNodeByText(dstTree, dst, "a == 1", "binary_expression")
		if err != nil {
			t.Fatalf("find newA: %v", err)
		}
		oldB, err := testutil.FindNodeByText(srcTree, src, "b == 2", "binary_expression")
		if err != nil {
			t.Fatalf("find oldB: %v", err)
		}
		newB, err := testutil.FindNodeByText(dstTree, dst, "b == 2", "binary_expression")
		if err != nil {
			t.Fatalf("find newB: %v", err)
		}
		oldRootBin, err := testutil.FindNodeByText(srcTree, src, "a == 1 || b == 2", "binary_expression")
		if err != nil {
			t.Fatalf("find oldRootBin: %v", err)
		}
		newRootBin, err := testutil.FindNodeByText(dstTree, dst, "(b == 2) || a == 1", "binary_expression")
		if err != nil {
			t.Fatalf("find newRootBin: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldOuterBlock, newOuterBlock)
		ms.Add(oldIf, newIf)
		ms.Add(oldA, newA)
		ms.Add(oldB, newB)
		ms.Add(oldRootBin, newRootBin)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldA, DestNode: newA})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected swapped operand move to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("does not treat operator-only shell inside same statement as stationary expression move", func(t *testing.T) {
		src := []byte("package main\nfunc f() {\n\tif (a && b) || keep {}\n}\n")
		dst := []byte("package main\nfunc f() {\n\tif (c && d) || other {}\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		oldOuter, err := testutil.FindNodeByText(srcTree, src, "{\n\tif (a && b) || keep {}\n}", "block")
		if err != nil {
			t.Fatalf("find oldOuter: %v", err)
		}
		newOuter, err := testutil.FindNodeByText(dstTree, dst, "{\n\tif (c && d) || other {}\n}", "block")
		if err != nil {
			t.Fatalf("find newOuter: %v", err)
		}
		oldIf, err := testutil.FindNodeByText(srcTree, src, "if (a && b) || keep {}", "if_statement")
		if err != nil {
			t.Fatalf("find oldIf: %v", err)
		}
		newIf, err := testutil.FindNodeByText(dstTree, dst, "if (c && d) || other {}", "if_statement")
		if err != nil {
			t.Fatalf("find newIf: %v", err)
		}
		oldParen, err := testutil.FindNodeByText(srcTree, src, "(a && b)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find oldParen: %v", err)
		}
		newParen, err := testutil.FindNodeByText(dstTree, dst, "(c && d)", "parenthesized_expression")
		if err != nil {
			t.Fatalf("find newParen: %v", err)
		}
		oldBin, err := testutil.FindNodeByText(srcTree, src, "a && b", "binary_expression")
		if err != nil {
			t.Fatalf("find oldBin: %v", err)
		}
		newBin, err := testutil.FindNodeByText(dstTree, dst, "c && d", "binary_expression")
		if err != nil {
			t.Fatalf("find newBin: %v", err)
		}
		oldOp, err := testutil.FindNodeByText(srcTree, src, "&&", "logical_operator_literal")
		if err != nil {
			t.Fatalf("find oldOp: %v", err)
		}
		newOp, err := testutil.FindNodeByText(dstTree, dst, "&&", "logical_operator_literal")
		if err != nil {
			t.Fatalf("find newOp: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(oldOuter, newOuter)
		ms.Add(oldIf, newIf)
		ms.Add(oldParen, newParen)
		ms.Add(oldBin, newBin)
		ms.Add(oldOp, newOp)

		r := rules.Get("go")
		if isStationaryExpressionMove(oldParen, newParen, ms, r, nil) {
			t.Fatal("expected operator-only expression shell to not be treated as stationary")
		}
		if !shouldDemoteMove(oldParen, newParen, ms, r) {
			t.Fatal("expected operator-only expression shell inside same statement to be demoted")
		}
	})

	t.Run("preserves move when wrapper unwrap depth exceeds limit", func(t *testing.T) {
		srcGo := []byte("package main\nfunc f() {\n\t_ = ((((foo))))\n}\n")
		dstGo := []byte("package main\nfunc f() {\n\t_ = foo\n}\n")
		srcTree, err := treesitter.Parse(srcGo, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstGo, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, srcGo, "{\n\t_ = ((((foo))))\n}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dstGo, "{\n\t_ = foo\n}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}
		oldID, err := testutil.FindNodeByText(srcTree, srcGo, "foo", "identifier")
		if err != nil {
			t.Fatalf("find oldID: %v", err)
		}
		newID, err := testutil.FindNodeByText(dstTree, dstGo, "foo", "identifier")
		if err != nil {
			t.Fatalf("find newID: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		ms.Add(oldID, newID)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldID, DestNode: newID})

		result := normalizeStationaryWrapperMoves(es, ms)
		// With maxUnwrapDepth = 3, 4 hops cannot unwrap to srcBlock, so move is preserved
		if result.Size() != 1 {
			t.Fatalf("expected move to be preserved across >3 wrapper hops, got %d actions", result.Size())
		}
	})

	t.Run("preserves move when wrapper unwrap is asymmetric", func(t *testing.T) {
		srcGo := []byte("package main\nfunc f() {\n\t_ = (((foo)))\n}\n")
		dstGo := []byte("package main\nfunc f() {\n\t_ = (foo)\n}\n")
		srcTree, err := treesitter.Parse(srcGo, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dstGo, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcBlock, err := testutil.FindNodeByText(srcTree, srcGo, "{\n\t_ = (((foo)))\n}", "block")
		if err != nil {
			t.Fatalf("find srcBlock: %v", err)
		}
		dstBlock, err := testutil.FindNodeByText(dstTree, dstGo, "{\n\t_ = (foo)\n}", "block")
		if err != nil {
			t.Fatalf("find dstBlock: %v", err)
		}
		oldID, err := testutil.FindNodeByText(srcTree, srcGo, "foo", "identifier")
		if err != nil {
			t.Fatalf("find oldID: %v", err)
		}
		newID, err := testutil.FindNodeByText(dstTree, dstGo, "foo", "identifier")
		if err != nil {
			t.Fatalf("find newID: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		ms.Add(oldID, newID)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldID, DestNode: newID})

		result := normalizeStationaryWrapperMoves(es, ms)
		// srcDepth = 3, dstDepth = 1, diff = 2 > 1 → asymmetric unwrap, move is preserved
		if result.Size() != 1 {
			t.Fatalf("expected move to be preserved across asymmetric unwrap depths, got %d actions", result.Size())
		}
	})
}

func assertNoActionOn(t *testing.T, es *actions.EditScript, node *treesitter.ASTNode) {
	t.Helper()
	if node == nil {
		t.Fatal("missing node for action assertion")
	}
	for _, a := range es.Actions() {
		if a.Node == node {
			t.Errorf("action on stationary node %s = %v, want none", node.Type, a)
		}
	}
}

func TestSameScopeDeclaration(t *testing.T) {
	r := rules.Get("go")

	t.Run("both nil nodes returns false", func(t *testing.T) {
		ms := engine.NewMapping()
		if sameScopeDeclaration(nil, nil, ms, r) {
			t.Error("expected false for nil nodes")
		}
	})

	t.Run("nil mapping returns false", func(t *testing.T) {
		src := []byte("package main\nvar x = 1\nvar y = 2\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		xNode, err := testutil.FindNodeByText(tree, src, "x", "identifier")
		if err != nil {
			t.Fatalf("find x: %v", err)
		}
		yNode, err := testutil.FindNodeByText(tree, src, "y", "identifier")
		if err != nil {
			t.Fatalf("find y: %v", err)
		}
		if sameScopeDeclaration(xNode, yNode, nil, r) {
			t.Error("expected false for nil mapping")
		}
	})

	t.Run("both outside any container returns true", func(t *testing.T) {
		ms := engine.NewMapping()
		src := []byte("package main\nvar x = 1\nvar y = 2\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		xNode, err := testutil.FindNodeByText(tree, src, "x", "identifier")
		if err != nil {
			t.Fatalf("find x: %v", err)
		}
		yNode, err := testutil.FindNodeByText(tree, src, "y", "identifier")
		if err != nil {
			t.Fatalf("find y: %v", err)
		}
		if !sameScopeDeclaration(xNode, yNode, ms, r) {
			t.Error("expected true when both nodes are outside any container")
		}
	})

	t.Run("matched enclosing containers returns true", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { stmtA() }\n")
		dst := []byte("package main\nfunc bar() { stmtB() }\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func foo()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func bar()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		srcStmt, err := testutil.FindNodeByText(srcTree, src, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "stmtB()", "expression_statement")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)

		if !sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
			t.Error("expected true when enclosing containers are mapped")
		}
	})

	t.Run("unmatched enclosing containers returns false", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { stmtA() }\n")
		dst := []byte("package main\nfunc bar() { stmtB() }\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		srcStmt, err := testutil.FindNodeByText(srcTree, src, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "stmtB()", "expression_statement")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms := engine.NewMapping()
		// srcFunc and dstFunc are NOT mapped to each other.

		if sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
			t.Error("expected false when enclosing containers are not mapped")
		}
	})

	t.Run("one inside container one outside returns false", func(t *testing.T) {
		src := []byte("package main\nfunc foo() { stmtA() }\n")
		dst := []byte("package main\nvar _ = stmtB()\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		srcStmt, err := testutil.FindNodeByText(srcTree, src, "stmtA()", "expression_statement")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "stmtB()", "call_expression")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms := engine.NewMapping()
		if sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
			t.Error("expected false when only one node is inside a container")
		}
	})

	t.Run("Unit/distinct closures in same outer function returns false", func(t *testing.T) {
		src := []byte("package main\nfunc outer() {\n\tc1 := func() { stmtOne() }\n\tc2 := func() { stmtTwo() }\n\t_, _ = c1, c2\n}\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		stmt1, err := testutil.FindNodeByText(tree, src, "stmtOne()", "expression_statement")
		if err != nil {
			t.Fatalf("find stmt1: %v", err)
		}
		stmt2, err := testutil.FindNodeByText(tree, src, "stmtTwo()", "expression_statement")
		if err != nil {
			t.Fatalf("find stmt2: %v", err)
		}

		ms := engine.NewMapping()
		// c1 and c2 are in the same outer function, but are distinct unmapped closures.
		if sameScopeDeclaration(stmt1, stmt2, ms, r) {
			t.Error("expected false for nodes inside distinct unmapped closures")
		}
	})

	t.Run("FullEngine/distinct sibling closures avoid cross-closure moves", func(t *testing.T) {
		// When a closure is deleted, statements inside it must not match as cross-scope
		// moves into an updated sibling closure inside the same outer function.
		src := []byte(`package main

func execute() {
	validate := func() error {
		if check() {
			return nil
		}
		return ErrInvalid
	}

	cleanup := func() {
		stop()
	}

	_ = validate
	cleanup()
}
`)
		dst := []byte(`package main

func execute() {
	cleanup := func() error {
		stop()
		return nil
	}

	cleanup()
}
`)

		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Context declarations: outer package and func execute() stay untouched without edit actions.
		for _, a := range normalized.Actions() {
			if a.Node != nil && (r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration")) {
				t.Errorf("expected context node %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Zero move actions across the entire snippet (no cross-closure token moves).
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Errorf("expected zero move actions across entire snippet, but found move on %s: %v", a.Node.Type, a)
			}
		}

		// 3. Whole-snippet verification: enclosing function, cleanup closure, and stop() map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "package main", "package main")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "func execute()", "func execute()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "stop()", "stop()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "cleanup()", "cleanup()")

		// 4. return nil in dst must be recorded as an Insert, not a Move.
		returnNil, err := testutil.FindNodeByText(dstAST, dst, "return nil")
		if err != nil {
			t.Fatalf("find return nil: %v", err)
		}
		hasReturnNilInsert := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && a.Node.ID == returnNil.ID
		})
		if !hasReturnNilInsert {
			t.Error("expected return nil in cleanup to be recorded as an Insert")
		}

		// 5. Entire deleted validate closure must be recorded as a Delete.
		validateClosure, err := testutil.FindNodeByText(srcAST, src, "validate :=", "short_var_declaration")
		if err != nil {
			t.Fatalf("find validate closure: %v", err)
		}
		hasValidateDelete := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Delete && a.Node != nil && a.Node.ID == validateClosure.ID
		})
		if !hasValidateDelete {
			t.Error("expected deleted validate closure to be recorded as a Delete")
		}
	})

	t.Run("FullEngine/closure extraction retains move actions", func(t *testing.T) {
		src := []byte(`package main

func processItems(items []int) {
	precedingInit()
	currCommon := common(items)
	currDice := dice(items)
	currAffinity := affinity(items)
	succeedingDone(currCommon, currDice, currAffinity)
}
`)
		dst := []byte(`package main

func processItems(items []int) {
	precedingInit()
	var currCommon int
	var currDice float64
	var currAffinity float64
	step01()
	step02()
	step03()
	step04()
	step05()
	step06()
	step07()
	step08()
	step09()
	step10()
	step11()
	ensureMetrics := func() {
		currCommon = common(items)
		currDice = dice(items)
		currAffinity = affinity(items)
	}
	ensureMetrics()
	succeedingDone(currCommon, currDice, currAffinity)
}
`)

		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		srcFunc, err := testutil.FindNodeByText(srcAST, src, "func processItems(items []int)")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstAST, dst, "func processItems(items []int)")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		srcInit, err := testutil.FindNodeByText(srcAST, src, "precedingInit()")
		if err != nil {
			t.Fatalf("find srcInit: %v", err)
		}
		srcDone, err := testutil.FindNodeByText(srcAST, src, "succeedingDone(currCommon, currDice, currAffinity)")
		if err != nil {
			t.Fatalf("find srcDone: %v", err)
		}
		assertNoActionOn(t, normalized, srcInit)
		assertNoActionOn(t, normalized, srcDone)

		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "func processItems(items []int)", "func processItems(items []int)")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "items []int", "items []int")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "precedingInit()", "precedingInit()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "succeedingDone(currCommon, currDice, currAffinity)", "succeedingDone(currCommon, currDice, currAffinity)")

		for _, item := range []struct {
			srcStmt string
			dstStmt string
			call    string
		}{
			{srcStmt: "currCommon := common(items)", dstStmt: "currCommon = common(items)", call: "common(items)"},
			{srcStmt: "currDice := dice(items)", dstStmt: "currDice = dice(items)", call: "dice(items)"},
			{srcStmt: "currAffinity := affinity(items)", dstStmt: "currAffinity = affinity(items)", call: "affinity(items)"},
		} {
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), item.srcStmt, item.dstStmt)
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), item.call, item.call)

			stmtNode, err := testutil.FindNodeByText(srcAST, src, item.srcStmt)
			if err != nil {
				t.Fatalf("find %q: %v", item.srcStmt, err)
			}
			if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
				return a.Type == actions.Move && a.Node == stmtNode
			}) {
				t.Errorf("stmt %q has no Move action, want extraction into closure", item.srcStmt)
			}
		}
	})

	t.Run("FullEngine/typescript arrow function extraction recognizes scope", func(t *testing.T) {
		r := rules.Get("typescript")

		src := []byte(`function processItems(items: number[]) {
    precedingInit();
    const currTotal = items.reduce((a, b) => a + b, 0);
    succeedingDone(currTotal);
}
`)
		dst := []byte(`function processItems(items: number[]) {
    precedingInit();

    step01();
    step02();
    step03();
    step04();
    step05();
    step06();
    step07();
    step08();
    step09();
    step10();
    step11();

    const getMetrics = () => items.reduce((a, b) => a + b, 0);
    succeedingDone(getMetrics());
}
`)

		srcAST, err := treesitter.Parse(src, "pipeline.ts")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "pipeline.ts")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		srcFunc, err := testutil.FindNodeByText(srcAST, src, "function processItems(items: number[])")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstAST, dst, "function processItems(items: number[])")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		srcInit, err := testutil.FindNodeByText(srcAST, src, "precedingInit()")
		if err != nil {
			t.Fatalf("find srcInit: %v", err)
		}
		assertNoActionOn(t, normalized, srcInit)

		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "function processItems(items: number[])", "function processItems(items: number[])")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "precedingInit()", "precedingInit()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "items.reduce((a, b) => a + b, 0)", "items.reduce((a, b) => a + b, 0)")

		srcReduce, err := testutil.FindNodeByText(srcAST, src, "items.reduce((a, b) => a + b, 0)")
		if err != nil {
			t.Fatalf("find srcReduce: %v", err)
		}
		dstReduce, err := testutil.FindNodeByText(dstAST, dst, "items.reduce((a, b) => a + b, 0)")
		if err != nil {
			t.Fatalf("find dstReduce: %v", err)
		}
		if !sameScopeDeclaration(srcReduce, dstReduce, matchResult.Mappings, r) {
			t.Errorf("expected sameScopeDeclaration to return true for arrow function inside processItems")
		}
	})

	t.Run("FullEngine/rust closure expression extraction recognizes scope", func(t *testing.T) {
		r := rules.Get("rust")

		src := []byte(`fn process_items(items: &[i32]) {
    preceding_init();
    let curr_total = items.iter().map(|x| x * 2).sum::<i32>();
    succeeding_done(curr_total);
}
`)
		dst := []byte(`fn process_items(items: &[i32]) {
    preceding_init();

    step01();
    step02();
    step03();
    step04();
    step05();
    step06();
    step07();
    step08();
    step09();
    step10();
    step11();

    let get_metrics = || items.iter().map(|x| x * 2).sum::<i32>();
    succeeding_done(get_metrics());
}
`)

		srcAST, err := treesitter.Parse(src, "pipeline.rs")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "pipeline.rs")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has syntax errors: %d", srcAST.ParseErrorCount)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has syntax errors: %d", dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		srcFunc, err := testutil.FindNodeByText(srcAST, src, "fn process_items(items: &[i32])")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstAST, dst, "fn process_items(items: &[i32])")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		srcInit, err := testutil.FindNodeByText(srcAST, src, "preceding_init()")
		if err != nil {
			t.Fatalf("find srcInit: %v", err)
		}
		assertNoActionOn(t, normalized, srcInit)

		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "fn process_items(items: &[i32])", "fn process_items(items: &[i32])")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "preceding_init()", "preceding_init()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "items.iter().map(|x| x * 2).sum::<i32>()", "items.iter().map(|x| x * 2).sum::<i32>()")

		srcSum, err := testutil.FindNodeByText(srcAST, src, "items.iter().map(|x| x * 2).sum::<i32>()")
		if err != nil {
			t.Fatalf("find srcSum: %v", err)
		}
		dstSum, err := testutil.FindNodeByText(dstAST, dst, "items.iter().map(|x| x * 2).sum::<i32>()")
		if err != nil {
			t.Fatalf("find dstSum: %v", err)
		}
		if !sameScopeDeclaration(srcSum, dstSum, matchResult.Mappings, r) {
			t.Errorf("expected sameScopeDeclaration to return true for closure_expression inside process_items")
		}
	})
}

func TestClosureScopeBoundaries(t *testing.T) {
	t.Run("FullEngine/java closure across method and class boundaries is unmapped and out of scope", func(t *testing.T) {
		r := rules.Get("java")
		src := []byte(`class Outer {
    int value = compute();
    static class Inner {
        void run() {
            precedingInit();
            use(value);
            succeedingDone();
        }
    }
}
`)
		dst := []byte(`class Outer {
    static class Inner {
        void run() {
            precedingInit();
            use(() -> compute());
            succeedingDone();
        }
    }
}
`)
		srcAST, err := treesitter.Parse(src, "Outer.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "Outer.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("snippets must parse cleanly: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "class Outer", "class Outer")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "static class Inner", "static class Inner")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "void run()", "void run()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "precedingInit()", "precedingInit()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "succeedingDone()", "succeedingDone()")

		// Whole-snippet unmapped assertions: cross-scope compute() and field declaration remain unmapped.
		testutil.AssertUnmappedText(t, srcAST.Root(), src, mr.Mappings.Src(), "compute()")
		testutil.AssertUnmappedText(t, dstAST.Root(), dst, mr.Mappings.Dst(), "compute()")
		testutil.AssertUnmappedText(t, srcAST.Root(), src, mr.Mappings.Src(), "int value = compute();")

		srcCompute, err := testutil.FindNodeByText(srcAST, src, "compute()")
		if err != nil {
			t.Fatalf("find srcCompute: %v", err)
		}
		dstCompute, err := testutil.FindNodeByText(dstAST, dst, "compute()")
		if err != nil {
			t.Fatalf("find dstCompute: %v", err)
		}
		srcField := srcCompute.Parent.Parent
		dstLambda := dstCompute.Parent
		if srcField == nil || !r.IsDeclaration(srcField.Type) || dstLambda == nil || !r.IsClosure(dstLambda.Type) {
			t.Fatalf("unexpected parsed scopes: field=%v lambda=%v", srcField, dstLambda)
		}
		srcOuter, err := testutil.FindNodeByText(srcAST, src, "class Outer")
		if err != nil {
			t.Fatalf("find srcOuter: %v", err)
		}
		if sameScopeDeclaration(srcField, dstCompute, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true across Outer field and Inner.run lambda, want false")
		}
		if sameScopeDeclaration(dstCompute, srcField, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true from Inner.run lambda to Outer field, want false")
		}
		if sameScopeDeclaration(srcOuter, dstCompute, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true across mapped Outer and Inner.run lambda, want false")
		}
		if sameScopeDeclaration(dstCompute, srcOuter, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true from Inner.run lambda to mapped Outer, want false")
		}
		if sameScopeDeclaration(srcCompute, dstCompute, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true for calls across Outer and Inner.run, want false")
		}
	})

	t.Run("FullEngine/java closure within method boundary maps in place", func(t *testing.T) {
		r := rules.Get("java")
		src := []byte(`class Outer {
    static class Inner {
        void run() {
            precedingInit();
            compute();
            succeedingDone();
        }
    }
}
`)
		dst := []byte(`class Outer {
    static class Inner {
        void run() {
            precedingInit();
            use(() -> compute());
            succeedingDone();
        }
    }
}
`)
		srcAST, err := treesitter.Parse(src, "Outer.java")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "Outer.java")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("snippets must parse cleanly: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "class Outer", "class Outer")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "static class Inner", "static class Inner")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "void run()", "void run()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "precedingInit()", "precedingInit()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "succeedingDone()", "succeedingDone()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "compute()", "compute()")

		srcCompute, err := testutil.FindNodeByText(srcAST, src, "compute()")
		if err != nil {
			t.Fatalf("find srcCompute: %v", err)
		}
		dstCompute, err := testutil.FindNodeByText(dstAST, dst, "compute()")
		if err != nil {
			t.Fatalf("find dstCompute: %v", err)
		}
		if !sameScopeDeclaration(srcCompute, dstCompute, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = false for call mapped into lambda within same method, want true")
		}
	})

	t.Run("FullEngine/go closure inlining retains all statement moves", func(t *testing.T) {
		r := rules.Get("go")
		dst := []byte(`package main

func processItems(items []int) {
	precedingInit()
	currCommon := common(items)
	currDice := dice(items)
	currAffinity := affinity(items)
	succeedingDone(currCommon, currDice, currAffinity)
}
`)
		src := []byte(`package main

func processItems(items []int) {
	precedingInit()
	var currCommon int
	var currDice float64
	var currAffinity float64
	step01()
	step02()
	step03()
	step04()
	step05()
	step06()
	step07()
	step08()
	step09()
	step10()
	step11()
	ensureMetrics := func() {
		currCommon = common(items)
		currDice = dice(items)
		currAffinity = affinity(items)
	}
	ensureMetrics()
	succeedingDone(currCommon, currDice, currAffinity)
}
`)
		srcAST, err := treesitter.Parse(src, "main.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "main.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("snippets must parse cleanly: src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}
		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		srcFunc, err := testutil.FindNodeByText(srcAST, src, "func processItems(items []int)")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstAST, dst, "func processItems(items []int)")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "func processItems(items []int)", "func processItems(items []int)")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "precedingInit()", "precedingInit()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "succeedingDone(currCommon, currDice, currAffinity)", "succeedingDone(currCommon, currDice, currAffinity)")

		for _, item := range []struct {
			srcStmt string
			dstStmt string
			call    string
		}{
			{srcStmt: "currCommon = common(items)", dstStmt: "currCommon := common(items)", call: "common(items)"},
			{srcStmt: "currDice = dice(items)", dstStmt: "currDice := dice(items)", call: "dice(items)"},
			{srcStmt: "currAffinity = affinity(items)", dstStmt: "currAffinity := affinity(items)", call: "affinity(items)"},
		} {
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), item.srcStmt, item.dstStmt)
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), item.call, item.call)

			srcStmt, err := testutil.FindNodeByText(srcAST, src, item.srcStmt)
			if err != nil {
				t.Fatalf("find srcStmt on %q: %v", item.srcStmt, err)
			}
			dstStmt, err := testutil.FindNodeByText(dstAST, dst, item.dstStmt)
			if err != nil {
				t.Fatalf("find dstStmt on %q: %v", item.dstStmt, err)
			}

			if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
				return a.Type == actions.Move && a.Node == srcStmt
			}) {
				t.Errorf("stmt %q has no Move action, want closure inlining", item.srcStmt)
			}
			if !sameScopeDeclaration(srcStmt, dstStmt, mr.Mappings, r) {
				t.Errorf("sameScopeDeclaration() = false for inlined stmt %q, want true", item.srcStmt)
			}
		}
	})
}

func TestNormalizeMovesByStructure_StationaryExpressionMoveDropped(t *testing.T) {
	// Setup: stationary expression restructuring on the same line within the same statement.
	src := []byte("package main\n" + strings.Repeat("\n", 9) + "func f() {\n\tcallFoo()\n}\n")
	dst := []byte("package main\n" + strings.Repeat("\n", 49) + "func f() {\n\tcallFoo()\n}\n")
	srcTree, err := treesitter.Parse(src, "src.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstTree, err := treesitter.Parse(dst, "dst.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	srcFunc, err := testutil.FindNodeByText(srcTree, src, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find srcFunc: %v", err)
	}
	dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func f()", "function_declaration")
	if err != nil {
		t.Fatalf("find dstFunc: %v", err)
	}
	srcBlock, err := testutil.FindNodeByText(srcTree, src, "{\n\tcallFoo()\n}", "block")
	if err != nil {
		t.Fatalf("find srcBlock: %v", err)
	}
	dstBlock, err := testutil.FindNodeByText(dstTree, dst, "{\n\tcallFoo()\n}", "block")
	if err != nil {
		t.Fatalf("find dstBlock: %v", err)
	}
	stmt1, err := testutil.FindNodeByText(srcTree, src, "callFoo()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmt1: %v", err)
	}
	stmt2, err := testutil.FindNodeByText(dstTree, dst, "callFoo()", "expression_statement")
	if err != nil {
		t.Fatalf("find stmt2: %v", err)
	}
	call1, err := testutil.FindNodeByText(srcTree, src, "callFoo()", "call_expression")
	if err != nil {
		t.Fatalf("find call1: %v", err)
	}
	call2, err := testutil.FindNodeByText(dstTree, dst, "callFoo()", "call_expression")
	if err != nil {
		t.Fatalf("find call2: %v", err)
	}
	id1, err := testutil.FindNodeByText(srcTree, src, "callFoo", "identifier")
	if err != nil {
		t.Fatalf("find id1: %v", err)
	}
	id2, err := testutil.FindNodeByText(dstTree, dst, "callFoo", "identifier")
	if err != nil {
		t.Fatalf("find id2: %v", err)
	}

	ms := engine.NewMapping()
	ms.Add(srcFunc, dstFunc)
	ms.Add(srcBlock, dstBlock)
	ms.Add(stmt1, stmt2)
	ms.Add(call1, call2)
	ms.Add(id1, id2)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: call1, DestNode: call2})

	result := normalizeMovesByStructure(es, ms)

	// Since call1 is stationary in stmt1 (even when stmt1 shifted from row 10 to row 50),
	// the Move action should be dropped completely and call1 should stay in the mapping.
	for _, a := range result.Actions() {
		if a.Node == call1 && (a.Type == actions.Move || a.Type == actions.Delete) {
			t.Errorf("expected stationary expression Move to be dropped from edit script, got %v", a.Type)
		}
	}
	testutil.AssertMappedText(t, srcTree, dstTree, src, dst, ms.Src(), "callFoo()", "callFoo()")
}

func TestTSXSelfClosingTagConversion(t *testing.T) {
	src := []byte(`<Component name="diffmantic" />`)
	dst := []byte(`<Component name="diffmantic"></Component>`)

	srcAST, err := treesitter.Parse(src, "Component.tsx")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "Component.tsx")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition(src, dst)
	matchResult := engine.Match(srcAST, dstAST, src, dst, part)
	script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

	for _, a := range normalized.Actions() {
		if a.Type == actions.Move {
			t.Errorf("expected 0 Move actions for TSX self-closing tag conversion after postprocessing, got: %s on node %s (%s)", a.Type, a.Node.Type, a.Node.Label)
		}
	}
}

func TestNormalizeStationaryMove_CrossScopeCoordinateCollision(t *testing.T) {
	// When code is replaced across different enclosing declarations, statements
	// that share identical line and column coordinates shouldn't be treated as
	// stationary moves and dropped as context.
	r := rules.Get("go")

	t.Run("preserves move across unmapped functions with identical coordinates", func(t *testing.T) {
		src := []byte("package main\nfunc oldFn() {\n\tx := 1\n}\n")
		dst := []byte("package main\nfunc newFn() {\n\tx := 1\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcStmt, err := testutil.FindNodeByText(srcTree, src, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcStmt, dstStmt)
		// srcFunc and dstFunc are intentionally unmapped.

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		// Identical coordinates across unmapped scopes shouldn't be treated as stationary.
		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected Move action preserved across unmapped scopes, got %d actions", result.Size())
		}

		threshold := requiredMoveThreshold(srcStmt, dstStmt, ms, r)
		if threshold < 50 {
			t.Errorf("expected cross-scope threshold >= 50, got %d", threshold)
		}
	})

	t.Run("drops move within mapped functions with identical coordinates", func(t *testing.T) {
		src := []byte("package main\nfunc fn() {\n\tx := 1\n}\n")
		dst := []byte("package main\nfunc fn() {\n\tx := 1\n}\n")
		srcTree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstTree, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		srcFunc, err := testutil.FindNodeByText(srcTree, src, "func fn()", "function_declaration")
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNodeByText(dstTree, dst, "func fn()", "function_declaration")
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		srcStmt, err := testutil.FindNodeByText(srcTree, src, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find srcStmt: %v", err)
		}
		dstStmt, err := testutil.FindNodeByText(dstTree, dst, "x := 1", "short_var_declaration")
		if err != nil {
			t.Fatalf("find dstStmt: %v", err)
		}

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)
		ms.Add(srcStmt, dstStmt)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 0 {
			t.Fatalf("expected stationary move dropped within mapped scopes, got %d actions", result.Size())
		}

		threshold := requiredMoveThreshold(srcStmt, dstStmt, ms, r)
		if threshold != 1 {
			t.Errorf("expected same-line shift threshold 1 within same scope, got %d", threshold)
		}
	})
}

func TestNormalizeStationaryMove_CrossContainerCoordinateCollisionPreservesMove(t *testing.T) {
	// Don't drop a move as stationary if coordinates match only because an item moved
	// into an adjacent container that landed on the item's old row and column.
	src := []byte(`package rules

var rules = &Rules{
	TagElements: []string{
		"jsx_element",
	},
	Tags: []string{
		"jsx_opening_element",
		"jsx_closing_element",
	},
}
`)
	dst := []byte(`package rules

var rules = &Rules{
	Tags: []string{
		"jsx_element",
		"jsx_opening_element",
		"jsx_closing_element",
	},
}
`)

	srcAST, err := treesitter.Parse(src, "rules.go")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "rules.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition(src, dst)
	matchResult := engine.Match(srcAST, dstAST, src, dst, part)
	script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

	hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
		return a.Type == actions.Move && a.Node != nil && (a.Node.Label == `"jsx_element"` || (len(a.Node.Children) > 0 && a.Node.Children[0].Label == `"jsx_element"`))
	})
	if !hasMove {
		t.Errorf("expected Move action on \"jsx_element\" to be preserved across containers despite coordinate collision, but none found")
	}
}

func TestAreChainedStatements(t *testing.T) {
	r := rules.Get("go")

	t.Run("returns true for matching chained if_statements", func(t *testing.T) {
		src := []byte("package main\nfunc f() { if a {} else if b {} }\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if1, err := testutil.FindNodeByText(tree, src, "if a {} else if b {}", "if_statement")
		if err != nil {
			t.Fatalf("find if1: %v", err)
		}
		if2, err := testutil.FindNodeByText(tree, src, "if b {}", "if_statement")
		if err != nil {
			t.Fatalf("find if2: %v", err)
		}
		origParent := if1.Parent
		if1.Parent = nil
		defer func() { if1.Parent = origParent }()
		if !areChainedStatements(if2, if1, r) {
			t.Errorf("expected areChainedStatements to return true for matching if_statements")
		}
	})

	t.Run("returns false for non-statement nodes with identical types", func(t *testing.T) {
		src := []byte("package main\nfunc f() { varAlpha := varBeta }\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		base, err := testutil.FindNodeByText(tree, src, "varAlpha", "identifier")
		if err != nil {
			t.Fatalf("find base: %v", err)
		}
		parent, err := testutil.FindNodeByText(tree, src, "varBeta", "identifier")
		if err != nil {
			t.Fatalf("find parent: %v", err)
		}
		if areChainedStatements(base, parent, r) {
			t.Errorf("expected areChainedStatements to return false for non-statement identifier nodes")
		}
	})

	t.Run("returns false when base is nil or parent is nil", func(t *testing.T) {
		src := []byte("package main\nfunc f() { if a {} }\n")
		tree, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		base, err := testutil.FindNodeByText(tree, src, "if a {}", "if_statement")
		if err != nil {
			t.Fatalf("find base: %v", err)
		}
		if areChainedStatements(base, nil, r) || areChainedStatements(nil, base, r) {
			t.Errorf("expected areChainedStatements to return false for nil nodes")
		}
	})
}

func TestNormalizeMoves_CrossCollectionElementMoveDemoted(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		// If a field like RightLine: 1 gets pulled from a deleted element into an updated
		// one, don't treat it as stationary or let it pass as a move. It should demote to
		// delete + insert.
		src := []byte(`package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: -1},
	{LeftLine: 4, RightLine: -1},
	{LeftLine: -1, RightLine: 1},
}
`)
		dst := []byte(`package test

var pairs = []LineAlignmentPair{
	{LeftLine: 2, RightLine: 1},
	{LeftLine: 4, RightLine: -1},
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Zero move actions anywhere in the snippet
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Errorf("expected zero move actions across entire snippet, but found move on %s: %v", a.Node.Type, a)
			}
			// 2. Package and var declaration headers must remain untouched context
			if a.Node != nil && (a.Node.Type == "package_clause" || a.Node.Type == "var_declaration") {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 3. Entire deleted collection element must be deleted
		deletedElem, err := testutil.FindNodeByText(srcAST, src, "{LeftLine: -1, RightLine: 1}", "literal_element")
		if err != nil {
			t.Fatalf("find deleted element: %v", err)
		}
		hasElementDelete := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Delete && a.Node != nil && a.Node.ID == deletedElem.ID
		})
		if !hasElementDelete {
			t.Error("expected entire deleted collection element to be deleted")
		}

		// 4. Whole-snippet verification: package, var declaration, and untouched element map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "package test", "package test")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "var pairs = []LineAlignmentPair{", "var pairs = []LineAlignmentPair{")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "{LeftLine: 4, RightLine: -1}", "{LeftLine: 4, RightLine: -1}")
		testutil.AssertUnmappedText(t, srcAST, src, matchResult.Mappings.Src(), "{LeftLine: -1, RightLine: 1}")
	})
}

func TestNormalizeMoves_PreservesSubExpressionMoveWhenParentIsNotContainer(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		// Arguments moving between chained method calls shouldn't get penalized
		// with T=20: it's a valid move, not a stolen container element.
		src := []byte(`package test

func f() {
	builder.Call(
		longArg(),
	)
}
`)
		dst := []byte(`package test

func f() {
	builder.Call(
		other(),
	).Call(
		longArg(),
	)
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Context declarations (package, func) should stay untouched
		for _, a := range normalized.Actions() {
			if a.Node != nil && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
				t.Errorf("expected context node %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Exactly one move for the argument_list
		var moveActions []actions.Action
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				moveActions = append(moveActions, a)
			}
		}
		if len(moveActions) != 1 {
			t.Fatalf("expected exactly 1 move action across entire snippet, got %d: %v", len(moveActions), moveActions)
		}
		move := moveActions[0]
		if move.Node == nil || move.Node.Type != "argument_list" {
			t.Errorf("expected moved node to be argument_list, got %v", move.Node)
		}
		longArgCall, err := testutil.FindNodeByText(srcAST, src, "longArg()", "call_expression")
		if err != nil {
			t.Fatalf("find longArg call: %v", err)
		}
		if !move.Node.Contains(longArgCall) {
			t.Errorf("expected moved argument_list to contain longArg()")
		}

		// 3. The other() call arguments should be recorded as an insert
		otherCall, err := testutil.FindNodeByText(dstAST, dst, "other()", "call_expression")
		if err != nil {
			t.Fatalf("find other call: %v", err)
		}
		hasOtherInsert := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && a.Node.Contains(otherCall)
		})
		if !hasOtherInsert {
			t.Error("expected chained call argument_list with other() to be inserted")
		}

		// 4. Package and function headers still map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "package test", "package test")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "func f()", "func f()")
	})
}

func TestStatementSlotEquivalent(t *testing.T) {
	r := rules.Get("go")

	t.Run("FullEngine", func(t *testing.T) {
		t.Run("ExpressionStatementToAssignment", func(t *testing.T) {
			src := []byte(`package main

func process(root *Node) {
	setup()
	EnsureIndex(root)
	cleanup()
}
`)
			dst := []byte(`package main

func process(root *Node) {
	setup()
	index := EnsureIndex(root)
	cleanup()
}
`)

			srcAST, err := treesitter.Parse(src, "main.go")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "main.go")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// 1. Enclosing context: outer package and func process() stay untouched without edit actions.
			for _, a := range normalized.Actions() {
				if a.Node != nil && r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
					t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
				}
			}

			// 2. Surrounding sequence: setup() and cleanup() must map in place without disruption.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "setup()", "setup()")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "cleanup()", "cleanup()")

			// 3. Target construct: EnsureIndex(root) must map in place, and its argument list must map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "EnsureIndex(root)", "EnsureIndex(root)")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "(root)", "(root)")

			// 4. Stationary operand: root identifier inside argument list must map in place.
			testutil.AssertMappedTextQuery(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
				testutil.TextQuery{Text: "root", Occurrence: 2},
				testutil.TextQuery{Text: "root", Occurrence: 2},
			)

			// 5. Zero Move actions anywhere in the snippet: EnsureIndex(root) stays stationary context.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Errorf("expected zero Move actions across snippet, found move on %s: %v", a.Node.Type, a)
				}
			}
		})

		t.Run("AssignmentToExpressionStatement", func(t *testing.T) {
			src := []byte(`package main

func process(root *Node) {
	setup()
	index := EnsureIndex(root)
	cleanup()
}
`)
			dst := []byte(`package main

func process(root *Node) {
	setup()
	EnsureIndex(root)
	cleanup()
}
`)

			srcAST, err := treesitter.Parse(src, "main.go")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "main.go")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// Zero Move actions: unassigning EnsureIndex(root) keeps it stationary in place.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Errorf("expected zero Move actions in reverse direction, found move on %s: %v", a.Node.Type, a)
				}
			}
		})
	})
}

func TestLocalVariableExtraction(t *testing.T) {
	r := rules.Get("go")
	if r == nil {
		t.Fatal("rules for go not found")
	}

	t.Run("FullEngine", func(t *testing.T) {
		t.Run("AdjacentExtractionPreserved", func(t *testing.T) {
			src := []byte(`package test

func process() {
	setup()
	onlyNonChars = isOnlyNonCharacters(
		fileBytes[gapStart:gapEnd],
	)
	cleanup()
}
`)
			dst := []byte(`package test

func process() {
	setup()
	gapBytes := fileBytes[gapStart:gapEnd]
	onlyNonChars = isOnlyNonCharacters(
		gapBytes,
	)
	cleanup()
}
`)

			srcAST, err := treesitter.Parse(src, "test.go")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "test.go")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// 1. Enclosing context: outer package and func process() stay untouched without edit actions.
			for _, a := range normalized.Actions() {
				if a.Node != nil && r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
					t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
				}
			}

			// 2. Surrounding sequence: setup() and cleanup() must map in place without disruption.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "setup()", "setup()")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "cleanup()", "cleanup()")

			// 3. Target construct: extracted slice must be mapped and preserved as Move.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "fileBytes[gapStart:gapEnd]", "fileBytes[gapStart:gapEnd]")

			// 4. Stationary operands: identifiers inside slice must map in place without corruption.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "fileBytes", "fileBytes")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "gapStart", "gapStart")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "gapEnd", "gapEnd")

			// 5. Action verification: exactly one Move action for the extracted slice expression.
			var moveActions []actions.Action
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					moveActions = append(moveActions, a)
				}
			}
			if len(moveActions) != 1 {
				t.Fatalf("expected exactly 1 move action for extracted slice, got %d: %v", len(moveActions), moveActions)
			}
			srcSlice, err := testutil.FindNodeByText(srcAST, src, "fileBytes[gapStart:gapEnd]")
			if err != nil {
				t.Fatalf("find srcSlice: %v", err)
			}
			if moveActions[0].Node != srcSlice {
				t.Errorf("expected move action on slice_expression, got node: %v", moveActions[0].Node)
			}
		})

		t.Run("MultilineExtractionPreserved", func(t *testing.T) {
			src := []byte(`package test

func process() {
	setup()
	onlyNonChars = isOnlyNonCharacters(fileBytes[gapStart:gapEnd])
	cleanup()
}
`)
			dst := []byte(`package test

func process() {
	setup()
	gapBytes := fileBytes[
		gapStart:gapEnd
	]
	onlyNonChars = isOnlyNonCharacters(gapBytes)
	cleanup()
}
`)

			srcAST, err := treesitter.Parse(src, "test.go")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "test.go")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// 1. Enclosing context stays untouched
			for _, a := range normalized.Actions() {
				if a.Node != nil && r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
					t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
				}
			}

			// 2. Extracted multiline slice must be preserved as Move
			sliceNode, err := testutil.FindNodeByText(srcAST, src, "fileBytes[gapStart:gapEnd]")
			if err != nil {
				t.Fatalf("find sliceNode: %v", err)
			}
			var hasSliceMove bool
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && a.Node.ID == sliceNode.ID {
					hasSliceMove = true
					break
				}
			}
			if !hasSliceMove {
				t.Error("expected multiline extracted slice expression to be preserved as Move, got demoted")
			}
		})

		t.Run("ConditionExtractedIntoVariable", func(t *testing.T) {
			// Extracting a condition like "err != nil" into a local variable
			// is recognized as a Move action from the if condition to the variable declaration.
			src := []byte(`package test

func process() {
	setup()
	if err != nil {
		cleanup()
		return
	}
}
`)
			dst := []byte(`package test

func process() {
	setup()
	isErr := err != nil
	if isErr {
		cleanup()
		return
	}
}
`)

			srcAST, err := treesitter.Parse(src, "test.go")
			if err != nil {
				t.Fatalf("failed to parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(dst, "test.go")
			if err != nil {
				t.Fatalf("failed to parse dst: %v", err)
			}

			part := engine.NewLinePartition(src, dst)
			matchResult := engine.Match(srcAST, dstAST, src, dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// 1. Enclosing context stays untouched
			for _, a := range normalized.Actions() {
				if a.Node != nil && r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
					t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
				}
			}

			// 2. Extracted condition err != nil is preserved as Move into the variable declaration
			condNode, err := testutil.FindNodeByText(srcAST, src, "err != nil")
			if err != nil {
				t.Fatalf("find condNode: %v", err)
			}
			var hasMove bool
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && a.Node.ID == condNode.ID {
					hasMove = true
					break
				}
			}
			if !hasMove {
				t.Error("expected extracted condition err != nil to be preserved as Move, got demoted")
			}
		})
	})
}

func TestNormalizeStationaryMove_CrossStatementCoordinateCollisionPreservesMove(t *testing.T) {
	r := rules.Get("go")

	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package test

func TestProcess(t *testing.T) {
	res := getResult()
	if res.Envelope == nil || res.FallbackReason != "" {
		t.Errorf("fallback error: %v", res.FallbackReason)
	}
	res.Cleanup()
}
`)
		dst := []byte(`package test

func TestProcess(t *testing.T) {
	res := getResult()
	if res.Envelope == nil {
		t.Fatal("expected non-nil envelope")
	}
	if res.FallbackReason != "" {
		t.Errorf("fallback error: %v", res.FallbackReason)
	}
	res.Cleanup()
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: package_clause and function TestProcess remain untouched
		for _, a := range normalized.Actions() {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) && (a.Node.Type == "package_clause" || a.Node.Type == "function_declaration") {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Surrounding sequence: res := getResult() and res.Cleanup() map in place
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "res := getResult()", "res := getResult()")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "res.Cleanup()", "res.Cleanup()")

		// 3. Move action: res.Envelope == nil moves into the new guard if
		guardNode, err := testutil.FindNodeByText(srcAST, src, "res.Envelope == nil")
		if err != nil {
			t.Fatalf("find guardNode: %v", err)
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node != nil && a.Node.ID == guardNode.ID
		})
		if !hasMove {
			t.Errorf("expected Move action on res.Envelope == nil to be preserved into new guard if statement, but none found: %v", normalized.Actions())
		}
	})
}

func TestNormalizeStationaryMove_LabeledStatementWrapper(t *testing.T) {
	r := rules.Get("go")

	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package engine

func Evaluate(node, dstNode *Node) bool {
	if node == nil || dstNode == nil {
		return false
	}
	if node.IsLeaf() && dstNode.IsLeaf() {
		return true
	}

	match := node.Kind == dstNode.Kind
	logMatch(match)
	return match
}
`)
		dst := []byte(`package engine

func Evaluate(node, dstNode *Node) bool {
	if node == nil || dstNode == nil {
		return false
	}
	if node.IsLeaf() && dstNode.IsLeaf() {
		if node.Value != dstNode.Value {
			goto fallback
		}
		return true
	}

fallback:
	match := node.Kind == dstNode.Kind
	logMatch(match)
	return match
}
`)

		srcAST, err := treesitter.Parse(src, "engine.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "engine.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: outer function declaration doesn't get touched.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "func Evaluate(node, dstNode *Node) bool", "func Evaluate(node, dstNode *Node) bool")
		for _, a := range normalized.Actions() {
			if a.Node != nil && (r.IsDeclaration(a.Node.Type) || r.IsContainerDeclaration(a.Node.Type)) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Surrounding sequence: guard check and trailing log/return statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "if node == nil || dstNode == nil", "if node == nil || dstNode == nil")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "logMatch(match)", "logMatch(match)")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "return match", "return match")

		// 3. Target statement: the labeled assignment maps in place and drops its move.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "match := node.Kind == dstNode.Kind", "match := node.Kind == dstNode.Kind")

		srcTarget, err := testutil.FindNodeByText(srcAST, src, "match := node.Kind == dstNode.Kind")
		if err != nil {
			t.Fatalf("find srcTarget: %v", err)
		}

		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node == srcTarget
		})
		if hasMove {
			t.Errorf("expected Move action on target statement to be dropped by stationary wrapper normalization, but found: %v", normalized.Actions())
		}

		hasLabelInsert := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && (a.Node.Label == "fallback" || (a.DestNode != nil && a.DestNode.Label == "fallback"))
		})
		if !hasLabelInsert {
			t.Errorf("expected label insert action for fallback label, but found actions: %v", normalized.Actions())
		}

		// 4. Stationary operands: inner selector expressions stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "node.Kind", "node.Kind")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "dstNode.Kind", "dstNode.Kind")
	})
}

func TestImportPathExtension_ScopedUseList(t *testing.T) {
	r := rules.Get("rust")
	if r == nil {
		t.Fatal("rules for rust not found")
	}

	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`use std::collections::HashMap;
use crate::features::side_by_side;
use crate::git_config::GitConfigEntry;
use crate::style::{self, Style};

pub fn run() {}
`)
		dst := []byte(`use std::collections::HashMap;
use crate::features::side_by_side::{self, ansifill, LeftRight};
use crate::git_config::{GitConfig, GitConfigEntry};
use crate::style::{self, Style};

pub fn run() {}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: make sure the enclosing function stays mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "pub fn run() {}", "pub fn run() {}")

		// 2. Surrounding sequence: existing import symbols stay anchored.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "GitConfigEntry", "GitConfigEntry")

		// 3. Target & wrappers: none of the existing import paths or symbols should be marked as moved.
		sideBySide, err := testutil.FindNodeByText(srcAST, src, "side_by_side")
		if err != nil {
			t.Fatalf("find side_by_side: %v", err)
		}
		gitConfigEntry, err := testutil.FindNodeByText(srcAST, src, "GitConfigEntry")
		if err != nil {
			t.Fatalf("find GitConfigEntry: %v", err)
		}
		gitConfigPath, err := testutil.FindNodeByText(srcAST, src, "git_config")
		if err != nil {
			t.Fatalf("find git_config: %v", err)
		}
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				for _, unmovable := range []*treesitter.ASTNode{sideBySide, gitConfigEntry, gitConfigPath} {
					if a.Node.ID == unmovable.ID || a.Node.Contains(unmovable) {
						t.Errorf("unexpected Move action on stationary import node %s: %v", unmovable.Type, a)
					}
				}
			}
		}

		// 4. Target & insertions: only the newly added item and its wrapping list get inserted.
		if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && a.Node.Label == "GitConfig"
		}) {
			t.Errorf("expected GitConfig to be inserted")
		}
		if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && a.Node.Type == "use_list"
		}) {
			t.Errorf("expected use_list to be inserted")
		}

		// 5. Stationary operands: existing path identifiers stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "side_by_side", "side_by_side")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "git_config", "git_config")
	})
}

func TestStationaryPatternWrapper(t *testing.T) {
	t.Run("Rust_FullEngine", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules for rust not found")
		}
		src := []byte(`fn get_style(state: State) -> &'static Style {
    match state {
        State::HunkPlus(_) => &self.plus_style,
        State::CommitMeta => &self.commit_style,
        State::FileMeta => &self.file_style,
        State::HunkHeader => &self.hunk_header_style,
        _ => unreachable!(),
    }
}
`)
		dst := []byte(`fn get_style(state: State) -> &'static Style {
    match state {
        State::HunkPlus(_) => &self.plus_style,
        State::CommitMeta => &self.commit_style,
        State::FileMeta => &self.file_style,
        State::HunkHeader(_, _) => &self.hunk_header_style,
        State::SubmoduleLog => &self.file_style,
        _ => unreachable!(),
    }
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: function get_style and parameter state map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "fn get_style(state: State) -> &'static Style", "fn get_style(state: State) -> &'static Style")

		// 2. Surrounding sequence: HunkPlus, CommitMeta, FileMeta, and default wildcard map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "HunkPlus", "HunkPlus")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "CommitMeta", "CommitMeta")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "FileMeta", "FileMeta")

		// 3. Target & wrappers: State::HunkHeader stays matched without Move or Delete actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "HunkHeader", "HunkHeader")

		srcHunkHeader, err := testutil.FindNodeByText(srcAST, src, "HunkHeader")
		if err != nil {
			t.Fatalf("find srcHunkHeader: %v", err)
		}
		dstHunkHeader, err := testutil.FindNodeByText(dstAST, dst, "HunkHeader")
		if err != nil {
			t.Fatalf("find dstHunkHeader: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Node != nil && (a.Node == srcHunkHeader || a.Node == dstHunkHeader) {
				t.Errorf("unexpected action on HunkHeader: %v", a)
			}
		}

		// 4. Target & insertions: tuple_struct_pattern and SubmoduleLog are inserted.
		submoduleArm, err := testutil.FindNodeByText(dstAST, dst, "SubmoduleLog", "match_arm")
		if err != nil {
			t.Fatalf("find SubmoduleLog match_arm: %v", err)
		}
		var foundTupleStructInsert, foundSubmoduleLogInsert bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Insert && a.Node != nil {
				if r.IsWrapper(a.Node.Type) && a.Node.Type == "tuple_struct_pattern" {
					foundTupleStructInsert = true
				}
				if a.Node.ID == submoduleArm.ID {
					foundSubmoduleLogInsert = true
				}
			}
		}
		if !foundTupleStructInsert {
			t.Errorf("expected tuple_struct_pattern to be inserted")
		}
		if !foundSubmoduleLogInsert {
			t.Errorf("expected SubmoduleLog to be inserted")
		}

		// 5. Stationary operands: hunk_header_style arm body maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "hunk_header_style", "hunk_header_style")
	})

	t.Run("Python_FullEngine", func(t *testing.T) {
		pyRules := rules.Get("python")
		if pyRules == nil {
			t.Fatal("rules for python not found")
		}

		src := []byte(`def get_style(state):
    match state:
        case State.HUNK_PLUS:
            return self.plus_style
        case State.COMMIT_META:
            return self.commit_style
        case State.FILE_META:
            return self.file_style
        case State.HUNK_HEADER:
            return self.hunk_header_style
        case _:
            pass
`)
		dst := []byte(`def get_style(state):
    match state:
        case State.HUNK_PLUS:
            return self.plus_style
        case State.COMMIT_META:
            return self.commit_style
        case State.FILE_META:
            return self.file_style
        case State.HUNK_HEADER(a, b):
            return self.hunk_header_style
        case State.SUBMODULE_LOG:
            return self.file_style
        case _:
            pass
`)

		srcAST, err := treesitter.Parse(src, "test.py")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.py")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatalf("Match returned nil or missing Mappings")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: function get_style maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "def get_style(state):", "def get_style(state):")

		// 2. Surrounding sequence: HUNK_PLUS, COMMIT_META, FILE_META map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "HUNK_PLUS", "HUNK_PLUS")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "COMMIT_META", "COMMIT_META")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "FILE_META", "FILE_META")

		// 3. Target & wrappers: State.HUNK_HEADER identifier stays matched without Move or Delete actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "HUNK_HEADER", "HUNK_HEADER")

		srcHunkHeader, err := testutil.FindNodeByText(srcAST, src, "HUNK_HEADER")
		if err != nil {
			t.Fatalf("find srcHunkHeader: %v", err)
		}
		dstHunkHeader, err := testutil.FindNodeByText(dstAST, dst, "HUNK_HEADER")
		if err != nil {
			t.Fatalf("find dstHunkHeader: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Node != nil && (a.Node == srcHunkHeader || a.Node == dstHunkHeader) {
				t.Errorf("unexpected action on HUNK_HEADER: %v", a)
			}
		}

		// 4. Target & insertions: class_pattern and SUBMODULE_LOG are inserted.
		submoduleCase, err := testutil.FindNodeByText(dstAST, dst, "SUBMODULE_LOG", "case_clause")
		if err != nil {
			t.Fatalf("find SUBMODULE_LOG case_clause: %v", err)
		}
		var foundClassPatternInsert, foundSubmoduleLogInsert bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Insert && a.Node != nil {
				if pyRules.IsWrapper(a.Node.Type) && a.Node.Type == "class_pattern" {
					foundClassPatternInsert = true
				}
				if a.Node.ID == submoduleCase.ID {
					foundSubmoduleLogInsert = true
				}
			}
		}
		if !foundClassPatternInsert {
			t.Errorf("expected class_pattern to be inserted")
		}
		if !foundSubmoduleLogInsert {
			t.Errorf("expected SUBMODULE_LOG to be inserted")
		}

		// 5. Stationary operands: hunk_header_style arm body maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "hunk_header_style", "hunk_header_style")
	})
}

func TestStationaryExpression_ReorderedStatement(t *testing.T) {
	cases := []struct {
		name        string
		filename    string
		lang        string
		fnType      string
		srcFnLine   int
		dstFnLine   int
		srcCallLine int
		dstCallLine int
		stmtType    string
		srcStmtLine int
		dstStmtLine int
		exprType    string
		srcExprLine int
		dstExprLine int
		src         []byte
		dst         []byte
	}{
		{
			name:        "Go",
			filename:    "test.go",
			lang:        "go",
			fnType:      "function_declaration",
			srcFnLine:   5,
			dstFnLine:   12,
			srcCallLine: 8,
			dstCallLine: 14,
			stmtType:    "short_var_declaration",
			srcStmtLine: 7,
			dstStmtLine: 20,
			exprType:    "selector_expression",
			srcExprLine: 7,
			dstExprLine: 20,
			src: []byte(`package main

func helper() {}

func foo() {
	for _, placeholder := range formatData {
		alignment_spec := placeholder.alignment_spec.unwrap_or("^")
		width := calculate_width(
			placeholder.width,
			min_field_width,
			extra_param_1,
			extra_param_2,
		)
		_ = width
		_ = alignment_spec
	}
}
`),
			dst: []byte(`package main

func helper1() {}
func helper2() {}
func helper3() {}
func helper4() {}
func helper5() {}
func helper6() {}
func helper7() {}
func helper8() {}

func foo() {
	for _, placeholder := range formatData {
		width := calculate_width(
			placeholder.width,
			min_field_width,
			extra_param_1,
			extra_param_2,
		)
		alignment_spec := placeholder.alignment_spec.as_ref().unwrap_or(AlignCenter)
		_ = width
		_ = alignment_spec
	}
}
`),
		},
		{
			name:        "Rust",
			filename:    "test.rs",
			lang:        "rust",
			fnType:      "function_item",
			srcFnLine:   3,
			dstFnLine:   10,
			srcCallLine: 6,
			dstCallLine: 12,
			stmtType:    "let_declaration",
			srcStmtLine: 5,
			dstStmtLine: 18,
			exprType:    "field_expression",
			srcExprLine: 5,
			dstExprLine: 18,
			src: []byte(`fn helper() {}

fn foo() {
    for placeholder in format_data {
        let alignment_spec = placeholder.alignment_spec.unwrap_or("^");
        let width = calculate_width(
            placeholder.width,
            min_field_width,
            extra_param_1,
            extra_param_2,
        );
    }
}
`),
			dst: []byte(`fn helper1() {}
fn helper2() {}
fn helper3() {}
fn helper4() {}
fn helper5() {}
fn helper6() {}
fn helper7() {}
fn helper8() {}

fn foo() {
    for placeholder in format_data {
        let width = calculate_width(
            placeholder.width,
            min_field_width,
            extra_param_1,
            extra_param_2,
        );
        let alignment_spec = placeholder.alignment_spec.as_ref().unwrap_or(AlignCenter);
    }
}
`),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srcAST, err := treesitter.Parse(tc.src, tc.filename)
			if err != nil {
				t.Fatalf("parse src: %v", err)
			}
			dstAST, err := treesitter.Parse(tc.dst, tc.filename)
			if err != nil {
				t.Fatalf("parse dst: %v", err)
			}

			part := engine.NewLinePartition(tc.src, tc.dst)
			matchResult := engine.Match(srcAST, dstAST, tc.src, tc.dst, part)
			if matchResult == nil || matchResult.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
			normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

			// 1. Enclosing context: function foo maps in place.
			var fnSig string
			if tc.lang == "rust" {
				fnSig = "fn foo()"
			} else {
				fnSig = "func foo()"
			}
			testutil.AssertMappedText(t, srcAST, dstAST, tc.src, tc.dst, matchResult.Mappings.Src(), fnSig, fnSig)

			// 2. Surrounding sequence: calculate_width call maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, tc.src, tc.dst, matchResult.Mappings.Src(), "calculate_width", "calculate_width")

			// 3. Target Statement: alignment_spec statement maps in place across versions.
			var stmtText string
			if tc.lang == "rust" {
				stmtText = "let alignment_spec = placeholder.alignment_spec"
			} else {
				stmtText = "alignment_spec := placeholder.alignment_spec"
			}
			testutil.AssertMappedText(t, srcAST, dstAST, tc.src, tc.dst, matchResult.Mappings.Src(), stmtText, stmtText)

			// 4. Stationary Operands: placeholder and alignment_spec must NOT be deleted or inserted.
			fieldExprSrc, err := testutil.FindNodeByText(srcAST, tc.src, "placeholder.alignment_spec")
			if err != nil {
				t.Fatalf("find src field_expression: %v", err)
			}
			fieldExprDst, err := testutil.FindNodeByText(dstAST, tc.dst, "placeholder.alignment_spec")
			if err != nil {
				t.Fatalf("find dst field_expression: %v", err)
			}

			for _, a := range normalized.Actions() {
				if a.Node == nil {
					continue
				}
				if a.Type == actions.Delete && (a.Node.ID == fieldExprSrc.ID || fieldExprSrc.Contains(a.Node)) {
					t.Errorf("spurious Delete on stationary payload %s (action: %v)", a.Node.Type, a)
				}
				if a.Type == actions.Insert && (a.Node.ID == fieldExprDst.ID || fieldExprDst.Contains(a.Node)) {
					t.Errorf("spurious Insert on stationary payload %s (action: %v)", a.Node.Type, a)
				}
			}

			// 5. Stationary Operands remain mapped in Mappings.
			testutil.AssertMappedText(t, srcAST, dstAST, tc.src, tc.dst, matchResult.Mappings.Src(), "placeholder.alignment_spec", "placeholder.alignment_spec")
		})
	}
}

func TestStationaryWrappedContainerChildWithDeletedSibling(t *testing.T) {
	r := rules.Get("rust")
	if r == nil {
		t.Fatal("rust rules not found")
	}

	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`fn paint_lines_side_by_side() {
    for (minus_line_index, plus_line_index) in line_alignment {
        output_buffer.push_str(&paint_left_panel_minus_line(
            minus_line_index,
            &minus_syntax_style_sections,
            &minus_diff_style_sections,
            match minus_line_index {
                Some(i) => minus_states[i],
                None => &State::HunkMinus(None),
            },
            line_numbers_data,
            if config.keep_plus_minus_markers {
                Some(config.minus_style.paint("-"))
            } else {
                None
            },
            background_color_extends_to_terminal_width,
            config,
        ));
    }
}
`)
		dst := []byte(`fn paint_lines_side_by_side() {
    for (minus_line_index, plus_line_index) in line_alignment {
        output_buffer.push_str(&paint_left_panel_minus_line(
            minus_line_index,
            &syntax_left_right[Left],
            &diff_left_right[Left],
            match minus_line_index {
                Some(i) => &states_left_right[Left][i],
                None => &State::HunkMinus(None),
            },
            line_numbers_data,
            background_color_extends_to_terminal_width[Left],
            config,
        ));
    }
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST has syntax errors")
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}
		if dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST has syntax errors")
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: enclosing function paint_lines_side_by_side maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "fn paint_lines_side_by_side()", "fn paint_lines_side_by_side()")

		// 2. Surrounding sequence: call arguments minus_line_index, line_numbers_data, and config map in place.
		testutil.AssertMappedTextQuery(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			testutil.TextQuery{Text: "minus_line_index", Occurrence: 2},
			testutil.TextQuery{Text: "minus_line_index", Occurrence: 2},
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "line_numbers_data", "line_numbers_data")
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "config,", "config,")

		// 3. Target: background_color_extends_to_terminal_width maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(), "background_color_extends_to_terminal_width", "background_color_extends_to_terminal_width")

		srcTarget, err := testutil.FindNodeByText(srcAST, src, "background_color_extends_to_terminal_width")
		if err != nil {
			t.Fatalf("find srcTarget: %v", err)
		}
		dstTarget, err := testutil.FindNodeByText(dstAST, dst, "background_color_extends_to_terminal_width")
		if err != nil {
			t.Fatalf("find dstTarget: %v", err)
		}

		// Target must NOT have any Move action in the normalized edit script.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && (a.Node == srcTarget || a.DestNode == dstTarget) {
				t.Errorf("unexpected Move action on stationary target: %v", a)
			}
		}

		// 4. Stationary Operands & Wrappers: index_expression is an Insert action.
		hasIndexInsert := false
		for _, a := range normalized.Actions() {
			if a.Type == actions.Insert && r.IsWrapper(a.Node.Type) && a.Node.Type == "index_expression" {
				hasIndexInsert = true
				break
			}
		}
		if !hasIndexInsert {
			t.Errorf("expected index_expression to be inserted")
		}
	})
}

func TestStationaryIfElseIfLadderMoves(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust", func(t *testing.T) {
			src := []byte(`fn paint_line() {
    let prefix = painted_prefix.clone();
    if should_right_fill_background_color {
        right_fill(&mut line, fill_style);
    } else if line_is_empty {
        mark_empty_line(&mut line);
    };
    output_buffer.push_str(&line);
}
`)
			dst := []byte(`fn paint_line() {
    let prefix = painted_prefix.clone();
    if let Some(BgFillMethod::TryAnsiSequence) = bg_fill_mode {
        right_fill(&mut line, fill_style);
    } else if let Some(BgFillMethod::Spaces) = bg_fill_mode {
        line.push_str(" ");
    } else if line_is_empty {
        mark_empty_line(&mut line);
    };
    output_buffer.push_str(&line);
}
`)
			srcAST, err := treesitter.Parse(src, "test.rs")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.rs")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			// 1. Enclosing context: enclosing function paint_line maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "fn paint_line()", "fn paint_line()")

			// 2. Surrounding sequence: prefix declaration and output_buffer call map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "let prefix = painted_prefix.clone();", "let prefix = painted_prefix.clone();")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "output_buffer.push_str(&line);", "output_buffer.push_str(&line);")

			// 3. Target: surviving else if line_is_empty branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch: %v", a)
				}
			}
		})

		t.Run("C", func(t *testing.T) {
			src := []byte(`void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
			dst := []byte(`void evaluate_option() {
    do_init();
    if (x == 1) {
        do_one();
    } else if (x == 2) {
        do_two();
    } else if (x == 3) {
        do_three();
    }
    do_done();
}
`)
			srcAST, err := treesitter.Parse(src, "test.c")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.c")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving C else_clause: %v", a)
				}
			}
		})

		t.Run("Python", func(t *testing.T) {
			src := []byte(`def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 3:
        do_three()
    do_done()
`)
			dst := []byte(`def evaluate_option():
    do_init()
    if x == 1:
        do_one()
    elif x == 2:
        do_two()
    elif x == 3:
        do_three()
    do_done()
`)
			srcAST, err := treesitter.Parse(src, "test.py")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.py")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving Python elif_clause: %v", a)
				}
			}
		})

		t.Run("TypeScript", func(t *testing.T) {
			src := []byte(`function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
			dst := []byte(`function evaluateOption() {
    doInit();
    if (x === 1) {
        doOne();
    } else if (x === 2) {
        doTwo();
    } else if (x === 3) {
        doThree();
    }
    doDone();
}
`)
			srcAST, err := treesitter.Parse(src, "test.ts")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.ts")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving TypeScript else_clause: %v", a)
				}
			}
		})

		t.Run("Go", func(t *testing.T) {
			src := []byte(`package main

func handleStatus(code int) {
	msg := "status"
	if code == 1 {
		processInitial()
	} else if code == 3 {
		processFinal()
	}
	finalize(msg)
}
`)
			dst := []byte(`package main

func handleStatus(code int) {
	msg := "status"
	if code == 1 {
		processInitial()
	} else if code == 2 {
		processPending()
	} else if code == 3 {
		processFinal()
	}
	finalize(msg)
}
`)
			srcAST, err := treesitter.Parse(src, "test.go")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.go")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			// 1. Enclosing context: enclosing function handleStatus maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "func handleStatus(code int)", "func handleStatus(code int)")

			// 2. Surrounding sequence: msg assignment and finalize call map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "msg := \"status\"", "msg := \"status\"")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "finalize(msg)", "finalize(msg)")

			// 3. Target: surviving else if code == 3 branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Go: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "processFinal()", "processFinal()")
		})

		t.Run("Java", func(t *testing.T) {
			src := []byte(`class StatusHandler {
    void handleStatus(int code) {
        String msg = "status";
        if (code == 1) {
            processInitial();
        } else if (code == 3) {
            processFinal();
        }
        finalize(msg);
    }
}
`)
			dst := []byte(`class StatusHandler {
    void handleStatus(int code) {
        String msg = "status";
        if (code == 1) {
            processInitial();
        } else if (code == 2) {
            processPending();
        } else if (code == 3) {
            processFinal();
        }
        finalize(msg);
    }
}
`)
			srcAST, err := treesitter.Parse(src, "test.java")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.java")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			// 1. Enclosing context: enclosing method handleStatus maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "void handleStatus(int code)", "void handleStatus(int code)")

			// 2. Surrounding sequence: msg local var declaration and finalize call map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "String msg = \"status\";", "String msg = \"status\";")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "finalize(msg);", "finalize(msg);")

			// 3. Target: surviving else if (code == 3) branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Java: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "processFinal();", "processFinal();")
		})

		t.Run("Zig", func(t *testing.T) {
			src := []byte(`pub fn handleStatus(code: i32) void {
    const msg = "status";
    if (code == 1) {
        processInitial();
    } else if (code == 3) {
        processFinal();
    }
    finalize(msg);
}
`)
			dst := []byte(`pub fn handleStatus(code: i32) void {
    const msg = "status";
    if (code == 1) {
        processInitial();
    } else if (code == 2) {
        processPending();
    } else if (code == 3) {
        processFinal();
    }
    finalize(msg);
}
`)
			srcAST, err := treesitter.Parse(src, "test.zig")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.zig")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			// 1. Enclosing context: enclosing function handleStatus maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "pub fn handleStatus(code: i32) void", "pub fn handleStatus(code: i32) void")

			// 2. Surrounding sequence: msg variable declaration and finalize call map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "const msg = \"status\";", "const msg = \"status\";")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "finalize(msg);", "finalize(msg);")

			// 3. Target: surviving else if (code == 3) branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Zig: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "processFinal();", "processFinal();")
		})
	})
}

// TestStationaryFluentMethodChains acts as a preventative non-regression suite
// verifying that intermediate method call additions and deletions on fluent builder
// chains remain stationary (no spurious Move actions). While statement-level
// stationarity protects single-statement assignments, dual-plane NCMA receiver
// unwrapping provides the architectural defense when unnesting chains.
func TestStationaryFluentMethodChains(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust/IntermediateMethodCallsDeleted", func(t *testing.T) {
			src := []byte(`fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                .use_value_delimiter(false)
                .takes_value(true)
                .overrides_with("style")
        );
    config.validate();
}
`)
			dst := []byte(`fn build_cli() {
    let mut config = Config::default();
    let app = Command::new("bat")
        .arg(
            Arg::new("style")
                .long("style")
                .value_name("components")
                .overrides_with("style")
        );
    config.validate();
}
`)
			srcAST, err := treesitter.Parse(src, "test.rs")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.rs")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			// 1. Enclosing context: enclosing function build_cli maps in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "fn build_cli()", "fn build_cli()")

			// 2. Surrounding sequence: config variable and validate call map in place.
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "let mut config = Config::default();", "let mut config = Config::default();")
			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "config.validate();", "config.validate();")

			// 3. Edit actions: zero Move actions on the stationary receiver chain.
			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on fluent receiver chain: %v (Node=%s)", a, a.Node.Type)
				}
			}
		})

		t.Run("TypeScript/IntermediateMethodCallsInserted", func(t *testing.T) {
			src := []byte(`function buildQuery() {
    const base = new QueryBuilder("users");
    const result = base
        .select("id")
        .where("active", true)
        .execute();
    logDone();
}
`)
			dst := []byte(`function buildQuery() {
    const base = new QueryBuilder("users");
    const result = base
        .select("id")
        .limit(10)
        .orderBy("created_at")
        .where("active", true)
        .execute();
    logDone();
}
`)
			srcAST, err := treesitter.Parse(src, "test.ts")
			if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
				t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
			}
			dstAST, err := treesitter.Parse(dst, "test.ts")
			if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
				t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
			}

			part := engine.NewLinePartition(src, dst)
			mr := engine.Match(srcAST, dstAST, src, dst, part)
			if mr == nil || mr.Mappings == nil {
				t.Fatal("Match returned nil")
			}

			testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(), "function buildQuery()", "function buildQuery()")

			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on TypeScript query builder: %v", a)
				}
			}
		})
	})
}

func TestStationaryWrappedMacroArguments(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`fn rewrite_options(element_name: &str) {
    let mode = get_mode();
    match mode {
        Some(1) => {
            let config = 10;
        }
        Some(2) => {
            eprintln!(
                "--{name}-color cannot be used with --{name}-style. \
                 Use --{name}-style=\"fg bg attr1 attr2 ...\" to set \
                 foreground color, background color, and style attributes. \
                 --{name}-color can only be used to set the background color. \
                 (It is still available for backwards-compatibility.)",
                name = element_name,
            );
            process::exit(1);
        }
        _ => {}
    }
}
`)
		// Trim two lines at the top of dst so line drift kicks in.
		dst := []byte(`fn rewrite_options(element_name: &str) {
    match get_mode() {
        Some(1) => {
            let config = 10;
        }
        Some(2) => {
            fatal(format!(
                "--{name}-color cannot be used with --{name}-style. \
                 Use --{name}-style=\"fg bg attr1 attr2 ...\" to set \
                 foreground color, background color, and style attributes. \
                 --{name}-color can only be used to set the background color. \
                 (It is still available for backwards-compatibility.)",
                name = element_name,
            ));
        }
        _ => {}
    }
}
`)
		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: enclosing function rewrite_options and match_expression map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn rewrite_options(element_name: &str)",
			"fn rewrite_options(element_name: &str)",
		)

		// 2. Surrounding sequence: Some(1) arm and Some(2) arm map in place without disruption.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"Some(1) => {",
			"Some(1) => {",
			"match_arm",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"Some(2) => {",
			"Some(2) => {",
			"match_arm",
		)

		// 3. Target: token_tree and operands within the macro invocation must NOT take Move actions.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Fatalf("unexpected Move action on stationary token_tree: %v", a)
			}
		}

		// 4. Stationary Operands: string literal maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`"--{name}-color cannot be used with --{name}-style.`,
			`"--{name}-color cannot be used with --{name}-style.`,
			"string_literal",
		)
	})
}

func TestStationaryReferenceExpressionWrapper(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`fn format_commit() {
    let prefix = "commit: ";
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, repo);
    options::set::set_options(&mut opt, git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`)
		dst := []byte(`fn format_commit() {
    let prefix = "commit: ";
    let shift = 42;
    format_commit_line_captures_with_osc8_commit_hyperlink(captures, &repo);
    options::set::set_options(&mut opt, &mut git_config, &arg_matches, assets);
    output_buffer.push_str(prefix);
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: enclosing function format_commit maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn format_commit()",
			"fn format_commit()",
		)

		// 2. Surrounding sequence: prefix declaration and output_buffer push call map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			`let prefix = "commit: ";`,
			`let prefix = "commit: ";`,
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"output_buffer.push_str(prefix)",
			"output_buffer.push_str(prefix)",
		)

		// 3. Target & Containers: calls to format_commit_line_captures and set_options map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"format_commit_line_captures_with_osc8_commit_hyperlink(captures, repo)",
			"format_commit_line_captures_with_osc8_commit_hyperlink(captures, &repo)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"options::set::set_options(&mut opt, git_config, &arg_matches, assets)",
			"options::set::set_options(&mut opt, &mut git_config, &arg_matches, assets)",
		)

		// 4. Stationary Operands: repo and git_config identifiers map in place and have NO Move or Delete actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"repo",
			"repo",
			"identifier",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"git_config",
			"git_config",
			"identifier",
		)
		srcRepo, err := testutil.FindNodeByText(srcAST, src, "repo", "identifier")
		if err != nil {
			t.Fatalf("failed to locate srcRepo: %v", err)
		}
		srcGitConfig, err := testutil.FindNodeByText(srcAST, src, "git_config", "identifier")
		if err != nil {
			t.Fatalf("failed to locate srcGitConfig: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && (a.Node == srcRepo || a.Node == srcGitConfig) {
				t.Fatalf("unexpected Move action on stationary operand: %v", a)
			}
			if a.Type == actions.Delete && (a.Node == srcRepo || a.Node == srcGitConfig) {
				t.Fatalf("unexpected Delete action on stationary operand: %v", a)
			}
		}
	})
}

func TestStationaryGenericTypeWrapperUnwrap(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`fn slide_to_prev_node(nodes: &[&Syntax], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`)
		dst := []byte(`fn slide_to_prev_node<'a>(nodes: &[&'a Syntax<'a>], start_idx: usize, end_idx: usize) {
    let initial = start_idx;
    if start_idx == 0 {
        return;
    }
    let end = end_idx;
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: slide_to_prev_node maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn slide_to_prev_node",
			"fn slide_to_prev_node",
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let initial = start_idx;",
			"let initial = start_idx;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if start_idx == 0",
			"if start_idx == 0",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let end = end_idx;",
			"let end = end_idx;",
		)

		// 3. Target: Syntax type identifier stays stationary with no move or delete actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"Syntax",
			"Syntax",
			"type_identifier",
		)
		srcSyntax, err := testutil.FindNodeByText(srcAST, src, "Syntax", "type_identifier")
		if err != nil {
			t.Fatalf("failed to locate srcSyntax: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcSyntax {
				t.Fatalf("unexpected Move action on stationary Syntax: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcSyntax {
				t.Fatalf("unexpected Delete action on stationary Syntax: %v", a)
			}
		}
	})
}

func TestStationaryGenericTypeWrapperUnwrap_TypeScript(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`function slideToPrevNode(nodes: Syntax[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`)
		dst := []byte(`function slideToPrevNode<T>(nodes: Promise<Syntax>[], startIdx: number): void {
    let initial = startIdx;
    if (startIdx === 0) {
        return;
    }
    let end = endIdx;
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: function slideToPrevNode stays mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"function slideToPrevNode",
			"function slideToPrevNode",
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let initial = startIdx;",
			"let initial = startIdx;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if (startIdx === 0)",
			"if (startIdx === 0)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"let end = endIdx;",
			"let end = endIdx;",
		)

		// 3. Target: Syntax type identifier maps in place and has NO Move or Delete action.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"Syntax",
			"Syntax",
			"type_identifier",
		)
		srcSyntax, err := testutil.FindNodeByText(srcAST, src, "Syntax", "type_identifier")
		if err != nil {
			t.Fatalf("failed to locate srcSyntax: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcSyntax {
				t.Fatalf("unexpected Move action on stationary Syntax: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcSyntax {
				t.Fatalf("unexpected Delete action on stationary Syntax: %v", a)
			}
		}
	})
}

func TestStationaryGenericTypeWrapperUnwrap_Java(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`class Parser {
    void slideToPrevNode(List<Syntax> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`)
		dst := []byte(`class Parser {
    <T> void slideToPrevNode(List<Optional<Syntax>> nodes, int startIdx) {
        int initial = startIdx;
        if (startIdx == 0) {
            return;
        }
        int end = endIdx;
    }
}
`)

		srcAST, err := treesitter.Parse(src, "Test.java")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "Test.java")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: class Parser and method slideToPrevNode stay mapped in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"class Parser",
			"class Parser",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"void slideToPrevNode",
			"void slideToPrevNode",
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"int initial = startIdx;",
			"int initial = startIdx;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if (startIdx == 0)",
			"if (startIdx == 0)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"int end = endIdx;",
			"int end = endIdx;",
		)

		// 3. Target: Syntax type identifier maps in place and has NO Move or Delete action.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"Syntax",
			"Syntax",
			"type_identifier",
		)
		srcSyntax, err := testutil.FindNodeByText(srcAST, src, "Syntax", "type_identifier")
		if err != nil {
			t.Fatalf("failed to locate srcSyntax: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcSyntax {
				t.Fatalf("unexpected Move action on stationary Syntax: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcSyntax {
				t.Fatalf("unexpected Delete action on stationary Syntax: %v", a)
			}
		}
	})
}

// TestNormalizeMoves_StationaryMultiLineWrappedArgument verifies that an unchanged
// argument wrapped in a new multi-line helper call is not emitted as a false move
// when its enclosing statement remains stationary.
func TestNormalizeMoves_StationaryMultiLineWrappedArgument(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(&matched_lines));
    finish_processing();
}
`)
		dst := []byte(`pub fn all_matched_lines_filled(lhs_lines: &[&str], rhs_lines: &[&str]) {
    let matched_lines = 1;
    compact_gaps(&ensure_contiguous(&match_preceding_blanks(
        &matched_lines,
        lhs_lines,
        rhs_lines,
    )));
    finish_processing();
}
`)

		srcAST, err := treesitter.Parse(src, "src.rs")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "dst.rs")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context: function_item maps in place with zero edit actions.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"pub fn all_matched_lines_filled",
			"pub fn all_matched_lines_filled",
		)

		// 2. Surrounding sequence: preceding and following statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"let matched_lines = 1;",
			"let matched_lines = 1;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"finish_processing()",
			"finish_processing()",
		)

		// 3. Target call compact_gaps maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"compact_gaps",
			"compact_gaps",
			"call_expression",
		)

		// 4. Stationary operand: &matched_lines must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Errorf("expected no Move actions for stationary argument, got Move on %s (id=%d)", a.Node.Type, a.Node.ID)
			}
		}
	})
}

func TestNormalizeMoves_StationaryMultiLineWrappedArgument_Go(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(&matched))
	finishProcessing()
}
`)
		dst := []byte(`package main

func process(lhs, rhs []string) {
	matched := 1
	compactGaps(ensureContiguous(matchPrecedingBlanks(
		&matched,
		lhs,
		rhs,
	)))
	finishProcessing()
}
`)

		srcAST, err := treesitter.Parse(src, "src.go")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "dst.go")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context and surrounding statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"func process",
			"func process",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"matched := 1",
			"matched := 1",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"finishProcessing()",
			"finishProcessing()",
		)

		// 2. Target call compactGaps maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"compactGaps",
			"compactGaps",
			"call_expression",
		)

		// 3. Stationary operand: &matched must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Errorf("unexpected Move action on stationary Go argument, got Move on %s (id=%d)", a.Node.Type, a.Node.ID)
			}
		}
	})
}

func TestNormalizeMoves_StationaryMultiLineWrappedArgument_Java(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		src := []byte(`class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(data.matched));
        finishProcessing();
    }
}
`)
		dst := []byte(`class Processor {
    void process(String[] lhs, String[] rhs) {
        int matched = 1;
        compactGaps(ensureContiguous(matchPrecedingBlanks(
            data.matched,
            lhs,
            rhs
        )));
        finishProcessing();
    }
}
`)

		srcAST, err := treesitter.Parse(src, "Processor.java")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "Processor.java")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		if srcAST.HasError || srcAST.ParseErrorCount > 0 || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("expected 0 parse errors, got src=%d dst=%d", srcAST.ParseErrorCount, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		matchResult := engine.Match(srcAST, dstAST, src, dst, part)
		if matchResult == nil || matchResult.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
		normalized := Run(script, matchResult.Mappings, srcAST, dstAST)

		// 1. Enclosing context and surrounding statements map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"class Processor",
			"class Processor",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"void process",
			"void process",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"int matched = 1;",
			"int matched = 1;",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"finishProcessing()",
			"finishProcessing()",
		)

		// 2. Target call compactGaps maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, matchResult.Mappings.Src(),
			"compactGaps",
			"compactGaps",
			"method_invocation",
		)

		// 3. Stationary operand: data.matched must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Errorf("unexpected Move action on stationary Java field_access argument, got Move on %s (id=%d)", a.Node.Type, a.Node.ID)
			}
		}
	})
}

func TestStationaryTailExpressionSemicolonTransition(t *testing.T) {
	t.Run("FullEngine/SingleLineWithDrift", func(t *testing.T) {
		src := []byte(`fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: &mut u32) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id)
}
`)
		dst := []byte(`fn added_upstream_helper() {
    let _ = 1;
}

fn init_info_single<'a>(roots: &[&'a Syntax<'a>], next_id: &mut NonZeroU32) {
    set_parent(roots, None);
    set_num_ancestors(roots, 0);
    set_unique_id(roots, next_id);
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: init_info_single maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn init_info_single",
			"fn init_info_single",
		)

		// 2. Surrounding sequence: set_parent and set_num_ancestors map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"set_parent(roots, None)",
			"set_parent(roots, None)",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"set_num_ancestors(roots, 0)",
			"set_num_ancestors(roots, 0)",
		)

		// 3. Target: set_unique_id call expression maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"set_unique_id(roots, next_id)",
			"set_unique_id(roots, next_id)",
		)
		srcUniqueCall, err := testutil.FindNodeByText(srcAST, src, "set_unique_id(roots, next_id)", "call_expression")
		if err != nil {
			t.Fatalf("failed to locate srcUniqueCall: %v", err)
		}

		// 4. In edit script: set_unique_id must NOT have Delete or Move actions.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcUniqueCall {
				t.Fatalf("unexpected Move action on stationary set_unique_id: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcUniqueCall {
				t.Fatalf("unexpected Delete action on stationary set_unique_id: %v", a)
			}
		}
	})

	t.Run("FullEngine/MultilineTailCall", func(t *testing.T) {
		r := rules.Get("rust")
		src := []byte(`fn dedup_hunks(lhs_is_dupe: bool, rhs_is_dupe: bool, lhs_line: usize, rhs_line: usize) {
    if lhs_is_dupe && rhs_is_dupe {
        return;
    }

    deduped_lines.push((
        if lhs_is_dupe { None } else { lhs_line },
        if rhs_is_dupe { None } else { rhs_line },
    ))
}
`)
		dst := []byte(`fn dedup_hunks(lhs_is_dupe: bool, rhs_is_dupe: bool, lhs_line: usize, rhs_line: usize) {
    if lhs_is_dupe && rhs_is_dupe {
        return;
    }

    deduped_lines.push((
        if lhs_is_dupe { None } else { lhs_line },
        if rhs_is_dupe { None } else { rhs_line },
    ));
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: dedup_hunks maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn dedup_hunks",
			"fn dedup_hunks",
		)

		// 2. Surrounding sequence: if statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if lhs_is_dupe && rhs_is_dupe",
			"if lhs_is_dupe && rhs_is_dupe",
		)

		// 3. Target: deduped_lines.push call expression maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"deduped_lines.push",
			"deduped_lines.push",
			"call_expression",
		)
		srcPush, err := testutil.FindNodeByText(srcAST, src, "deduped_lines.push", "call_expression")
		if err != nil {
			t.Fatalf("failed to locate srcPush: %v", err)
		}

		// 4. Stationary Operands: argument sub-expressions map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if lhs_is_dupe { None } else { lhs_line }",
			"if lhs_is_dupe { None } else { lhs_line }",
		)

		// 5. In edit script: NO Delete or Move on deduped_lines.push, and NO Insert on expression_statement.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcPush {
				t.Fatalf("unexpected Move action on stationary deduped_lines.push: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcPush {
				t.Fatalf("unexpected Delete action on stationary deduped_lines.push: %v", a)
			}
			if a.Type == actions.Insert && a.Node != nil && r.IsExpressionStatement(a.Node.Type) {
				t.Fatalf("unexpected Insert action on expression_statement wrapper: %v", a)
			}
		}
	})

	t.Run("FullEngine/MultilineTailCall_SemicolonRemoved", func(t *testing.T) {
		r := rules.Get("rust")
		src := []byte(`fn dedup_hunks(lhs_is_dupe: bool, rhs_is_dupe: bool, lhs_line: usize, rhs_line: usize) {
    if lhs_is_dupe && rhs_is_dupe {
        return;
    }

    deduped_lines.push((
        if lhs_is_dupe { None } else { lhs_line },
        if rhs_is_dupe { None } else { rhs_line },
    ));
}
`)
		dst := []byte(`fn dedup_hunks(lhs_is_dupe: bool, rhs_is_dupe: bool, lhs_line: usize, rhs_line: usize) {
    if lhs_is_dupe && rhs_is_dupe {
        return;
    }

    deduped_lines.push((
        if lhs_is_dupe { None } else { lhs_line },
        if rhs_is_dupe { None } else { rhs_line },
    ))
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v, errors=%d", err, srcAST.ParseErrorCount)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v, errors=%d", err, dstAST.ParseErrorCount)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		// 1. Enclosing context: dedup_hunks maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn dedup_hunks", "fn dedup_hunks", "function_item",
		)

		// 2. Surrounding sequence: if statement maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if lhs_is_dupe && rhs_is_dupe", "if lhs_is_dupe && rhs_is_dupe", "if_expression",
		)

		// 3. Target: deduped_lines.push call expression maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"deduped_lines.push", "deduped_lines.push", "call_expression",
		)
		srcPush, err := testutil.FindNodeByText(srcAST, src, "deduped_lines.push", "call_expression")
		if err != nil {
			t.Fatalf("failed to locate srcPush: %v", err)
		}

		// 4. Stationary Operands: argument sub-expressions map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"if lhs_is_dupe { None } else { lhs_line }", "if lhs_is_dupe { None } else { lhs_line }", "if_expression",
		)

		// 5. In edit script: NO Delete or Move on deduped_lines.push, and NO Delete on expression_statement.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node == srcPush {
				t.Fatalf("unexpected Move action on stationary deduped_lines.push: %v", a)
			}
			if a.Type == actions.Delete && a.Node == srcPush {
				t.Fatalf("unexpected Delete action on stationary deduped_lines.push: %v", a)
			}
			if a.Type == actions.Delete && a.Node != nil && r.IsExpressionStatement(a.Node.Type) {
				t.Fatalf("unexpected Delete action on expression_statement wrapper: %v", a)
			}
		}
	})
}

func TestCrossBranchMoveNotStationary(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		src := []byte(`package main

func handle(ready bool) {
	prepare()
	if ready {
		processTask()
	}
	finish()
}
`)

		dst := []byte(`package main

func handle(ready bool) {
	prepare()
	if ready {
		validate()
	} else {
		processTask()
	}
	finish()
}
`)

		srcAST, err := treesitter.Parse(src, "test.go")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.go")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context: enclosing function handle maps in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"func handle(ready bool)", "func handle(ready bool)", "function_declaration",
		)

		// 2. Surrounding sequence: prepare() and finish() calls map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"prepare()", "prepare()", "call_expression",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"finish()", "finish()", "call_expression",
		)

		// 3. Edit actions: processTask statement moved across branches (consequence -> alternative),
		// so it MUST emit a Move action and NOT be treated as stationary.
		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		processTaskNode, err := testutil.FindNodeByText(srcAST, src, "processTask()", "call_expression")
		if err != nil {
			t.Fatalf("find processTaskNode: %v", err)
		}
		stmtNode := engine.FindEnclosingStatement(processTaskNode, r)
		if stmtNode == nil {
			stmtNode = processTaskNode
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && (a.Node == stmtNode || a.Node == processTaskNode)
		})
		if !hasMove {
			t.Fatalf("expected Move action for processTask moving from consequence to alternative, got actions: %v", normalized.Actions())
		}
	})
}

func TestCrossBranchMoveNotStationary_Python(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("python")
		if r == nil {
			t.Fatal("rules for python not found")
		}

		src := []byte(`def handle(ready):
    prepare()
    if ready:
        process_task()
    finish()
`)

		dst := []byte(`def handle(ready):
    prepare()
    if ready:
        validate()
    else:
        process_task()
    finish()
`)

		srcAST, err := treesitter.Parse(src, "test.py")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.py")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context and surrounding sequence map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"def handle(ready):", "def handle(ready):", "function_definition",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"prepare()", "prepare()", "call",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"finish()", "finish()", "call",
		)

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		targetNode, err := testutil.FindNodeByText(srcAST, src, "process_task()", "call")
		if err != nil {
			t.Fatalf("find process_task: %v", err)
		}
		stmtNode := engine.FindEnclosingStatement(targetNode, r)
		if stmtNode == nil {
			stmtNode = targetNode
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && (a.Node == stmtNode || a.Node == targetNode)
		})
		if !hasMove {
			t.Fatalf("expected Move action for process_task moving across branches in Python, got actions: %v", normalized.Actions())
		}
	})
}

func TestCrossBranchMoveNotStationary_Java(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("java")
		if r == nil {
			t.Fatal("rules for java not found")
		}

		src := []byte(`class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) {
            processTask();
        }
        finish();
    }
}
`)

		dst := []byte(`class Handler {
    void handle(boolean ready) {
        prepare();
        if (ready) {
            validate();
        } else {
            processTask();
        }
        finish();
    }
}
`)

		srcAST, err := treesitter.Parse(src, "test.java")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.java")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context and surrounding sequence map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"class Handler", "class Handler", "class_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"void handle(boolean ready)", "void handle(boolean ready)", "method_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"prepare();", "prepare();", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"finish();", "finish();", "expression_statement",
		)

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		targetNode, err := testutil.FindNodeByText(srcAST, src, "processTask();", "expression_statement")
		if err != nil {
			t.Fatalf("find processTask: %v", err)
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node == targetNode
		})
		if !hasMove {
			t.Fatalf("expected Move action for processTask moving across branches in Java, got actions: %v", normalized.Actions())
		}
	})
}

func TestCrossBranchMoveNotStationary_Rust(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("rust")
		if r == nil {
			t.Fatal("rules for rust not found")
		}

		src := []byte(`fn handle(ready: bool) {
    prepare();
    if ready {
        process_task();
    }
    finish();
}
`)
		dst := []byte(`fn handle(ready: bool) {
    prepare();
    if ready {
        validate();
    } else {
        process_task();
    }
    finish();
}
`)

		srcAST, err := treesitter.Parse(src, "test.rs")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.rs")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context and surrounding sequence map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"fn handle(ready: bool)", "fn handle(ready: bool)", "function_item",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"prepare();", "prepare();", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"finish();", "finish();", "expression_statement",
		)

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		targetNode, err := testutil.FindNodeByText(srcAST, src, "process_task();", "expression_statement")
		if err != nil {
			t.Fatalf("find process_task: %v", err)
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node == targetNode
		})
		if !hasMove {
			t.Fatalf("expected Move action for process_task moving across branches in Rust, got actions: %v", normalized.Actions())
		}
	})
}

func TestCrossBranchMoveNotStationary_TypeScript(t *testing.T) {
	t.Run("FullEngine", func(t *testing.T) {
		r := rules.Get("typescript")
		if r == nil {
			t.Fatal("rules for typescript not found")
		}

		src := []byte(`function handle(ready: boolean) {
    prepare();
    if (ready) {
        processTask();
    }
    finish();
}
`)
		dst := []byte(`function handle(ready: boolean) {
    prepare();
    if (ready) {
        validate();
    } else {
        processTask();
    }
    finish();
}
`)

		srcAST, err := treesitter.Parse(src, "test.ts")
		if err != nil || srcAST.HasError || srcAST.ParseErrorCount > 0 {
			t.Fatalf("src AST parse error: %v", err)
		}
		dstAST, err := treesitter.Parse(dst, "test.ts")
		if err != nil || dstAST.HasError || dstAST.ParseErrorCount > 0 {
			t.Fatalf("dst AST parse error: %v", err)
		}

		part := engine.NewLinePartition(src, dst)
		mr := engine.Match(srcAST, dstAST, src, dst, part)
		if mr == nil || mr.Mappings == nil {
			t.Fatal("Match returned nil")
		}

		// 1. Enclosing context and surrounding sequence map in place.
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"function handle(ready: boolean)", "function handle(ready: boolean)", "function_declaration",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"prepare();", "prepare();", "expression_statement",
		)
		testutil.AssertMappedText(t, srcAST, dstAST, src, dst, mr.Mappings.Src(),
			"finish();", "finish();", "expression_statement",
		)

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		targetNode, err := testutil.FindNodeByText(srcAST, src, "processTask();", "expression_statement")
		if err != nil {
			t.Fatalf("find processTask: %v", err)
		}
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node == targetNode
		})
		if !hasMove {
			t.Fatalf("expected Move action for processTask moving across branches in TypeScript, got actions: %v", normalized.Actions())
		}
	})
}
