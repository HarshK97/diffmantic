package postprocess

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// mkNode builds a node with children and sets Parent pointers.
func mkNode(typ, label string, children ...*treesitter.ASTNode) *treesitter.ASTNode {
	n := &treesitter.ASTNode{Type: typ, Label: label}
	for _, c := range children {
		c.Parent = n
		n.Children = append(n.Children, c)
	}
	return n
}

func TestNormalizeStationaryWrapperMoves(t *testing.T) {
	t.Run("drops move when wrapper is removed", func(t *testing.T) {
		oldClass := mkNode("class_specifier", "")
		oldClass.Language = "cpp"
		oldWrapper := mkNode("friend_declaration", "")
		oldWrapper.Language = "cpp"
		oldFn := mkNode("function_definition", "foo")
		oldFn.Language = "cpp"
		oldClass.Children = append(oldClass.Children, oldWrapper)
		oldWrapper.Parent = oldClass
		oldWrapper.Children = append(oldWrapper.Children, oldFn)
		oldFn.Parent = oldWrapper

		newClass := mkNode("class_specifier", "")
		newClass.Language = "cpp"
		newFn := mkNode("function_definition", "foo")
		newFn.Language = "cpp"
		newClass.Children = append(newClass.Children, newFn)
		newFn.Parent = newClass

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
		oldClass := mkNode("class_specifier", "")
		oldClass.Language = "cpp"
		oldFn := mkNode("function_definition", "foo")
		oldFn.Language = "cpp"
		oldClass.Children = append(oldClass.Children, oldFn)
		oldFn.Parent = oldClass

		newClass := mkNode("class_specifier", "")
		newClass.Language = "cpp"
		newWrapper := mkNode("friend_declaration", "")
		newWrapper.Language = "cpp"
		newFn := mkNode("function_definition", "foo")
		newFn.Language = "cpp"
		newClass.Children = append(newClass.Children, newWrapper)
		newWrapper.Parent = newClass
		newWrapper.Children = append(newWrapper.Children, newFn)
		newFn.Parent = newWrapper

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
		oldClassA := mkNode("class_specifier", "A")
		oldClassA.Language = "cpp"
		oldFn := mkNode("function_definition", "foo")
		oldFn.Language = "cpp"
		oldClassA.Children = append(oldClassA.Children, oldFn)
		oldFn.Parent = oldClassA

		newClassB := mkNode("class_specifier", "B")
		newClassB.Language = "cpp"
		newFn := mkNode("function_definition", "foo")
		newFn.Language = "cpp"
		newClassB.Children = append(newClassB.Children, newFn)
		newFn.Parent = newClassB

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
		oldStmtList := mkNode("statement_list", "")
		oldStmtList.Language = "go"
		oldVarDecl := mkNode("var_declaration", "")
		oldVarDecl.Language = "go"
		oldVarKeyword := mkNode("var", "var")
		oldVarKeyword.Language = "go"
		oldVarSpec := mkNode("var_spec", "")
		oldVarSpec.Language = "go"
		oldID := mkNode("identifier", "fc")
		oldID.Language = "go"
		oldEq := mkNode("assignment_operator_literal", "=")
		oldEq.Language = "go"
		oldFn := mkNode("func_literal", "")
		oldFn.Language = "go"

		oldStmtList.Children = []*treesitter.ASTNode{oldVarDecl}
		oldVarDecl.Parent = oldStmtList
		oldVarDecl.Children = []*treesitter.ASTNode{oldVarKeyword, oldVarSpec}
		oldVarKeyword.Parent = oldVarDecl
		oldVarSpec.Parent = oldVarDecl
		oldVarSpec.Children = []*treesitter.ASTNode{oldID, oldEq, oldFn}
		oldID.Parent = oldVarSpec
		oldEq.Parent = oldVarSpec
		oldFn.Parent = oldVarSpec

		newStmtList := mkNode("statement_list", "")
		newStmtList.Language = "go"
		newShortVar := mkNode("short_var_declaration", "")
		newShortVar.Language = "go"
		newExprList := mkNode("expression_list", "")
		newExprList.Language = "go"
		newID := mkNode("identifier", "fc")
		newID.Language = "go"
		newWalrus := mkNode("assignment_operator_literal", ":=")
		newWalrus.Language = "go"
		newFn := mkNode("func_literal", "")
		newFn.Language = "go"

		newStmtList.Children = []*treesitter.ASTNode{newShortVar}
		newShortVar.Parent = newStmtList
		newExprList.Children = []*treesitter.ASTNode{newID}
		newID.Parent = newExprList
		newShortVar.Children = []*treesitter.ASTNode{newExprList, newWalrus, newFn}
		newExprList.Parent = newShortVar
		newWalrus.Parent = newShortVar
		newFn.Parent = newShortVar

		ms := engine.NewMapping()
		ms.Add(oldStmtList, newStmtList)
		ms.Add(oldVarSpec, newShortVar)
		ms.Add(oldID, newID)
		ms.Add(oldEq, newWalrus)
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
		oldArgs := mkNode("argument_list", "")
		oldArgs.Language = "go"
		oldArg1 := mkNode("call_expression", "info.Mode().Perm()")
		oldArg1.Language = "go"
		oldArg2 := mkNode("identifier", "mode")
		oldArg2.Language = "go"
		oldArgs.Children = []*treesitter.ASTNode{oldArg1, oldArg2}
		oldArg1.Parent = oldArgs
		oldArg2.Parent = oldArgs

		newArgs := mkNode("argument_list", "")
		newArgs.Language = "go"
		newArg1 := mkNode("identifier", "mode")
		newArg1.Language = "go"
		newArg2 := mkNode("call_expression", "info.Mode().Perm()")
		newArg2.Language = "go"
		newArgs.Children = []*treesitter.ASTNode{newArg1, newArg2}
		newArg1.Parent = newArgs
		newArg2.Parent = newArgs

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
		oldProg := mkNode("program", "")
		oldProg.Language = "typescript"
		oldTypeDecl := mkNode("type_alias_declaration", "type Foo = number")
		oldTypeDecl.Language = "typescript"
		oldProg.Children = []*treesitter.ASTNode{oldTypeDecl}
		oldTypeDecl.Parent = oldProg

		newProg := mkNode("program", "")
		newProg.Language = "typescript"
		newExportStmt := mkNode("export_statement", "export type Foo = number")
		newExportStmt.Language = "typescript"
		newExportKw := mkNode("export", "export")
		newExportKw.Language = "typescript"
		newTypeDecl := mkNode("type_alias_declaration", "type Foo = number")
		newTypeDecl.Language = "typescript"

		newProg.Children = []*treesitter.ASTNode{newExportStmt}
		newExportStmt.Parent = newProg
		newExportStmt.Children = []*treesitter.ASTNode{newExportKw, newTypeDecl}
		newExportKw.Parent = newExportStmt
		newTypeDecl.Parent = newExportStmt

		ms := engine.NewMapping()
		ms.Add(oldProg, newProg)
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

	t.Run("preserves Move when expression is passed into a new function argument list", func(t *testing.T) {
		oldCall := mkNode("call_expression", "")
		oldCall.Language = "php"
		oldArgs := mkNode("arguments", "")
		oldArgs.Language = "php"
		oldArg := mkNode("argument", "")
		oldArg.Language = "php"
		oldExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		oldExpr.Language = "php"

		oldCall.Children = []*treesitter.ASTNode{oldArgs}
		oldArgs.Parent = oldCall
		oldArgs.Children = []*treesitter.ASTNode{oldArg}
		oldArg.Parent = oldArgs
		oldArg.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldArg

		newOuterCall := mkNode("call_expression", "")
		newOuterCall.Language = "php"
		newOuterArgs := mkNode("arguments", "")
		newOuterArgs.Language = "php"
		newNestedCall := mkNode("call_expression", "")
		newNestedCall.Language = "php"
		newNestedArgs := mkNode("arguments", "")
		newNestedArgs.Language = "php"
		newArg := mkNode("argument", "")
		newArg.Language = "php"
		newExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		newExpr.Language = "php"

		newOuterCall.Children = []*treesitter.ASTNode{newOuterArgs}
		newOuterArgs.Parent = newOuterCall
		newOuterArgs.Children = []*treesitter.ASTNode{newNestedCall}
		newNestedCall.Parent = newOuterArgs
		newNestedCall.Children = []*treesitter.ASTNode{newNestedArgs}
		newNestedArgs.Parent = newNestedCall
		newNestedArgs.Children = []*treesitter.ASTNode{newArg}
		newArg.Parent = newNestedArgs
		newArg.Children = []*treesitter.ASTNode{newExpr}
		newExpr.Parent = newArg

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
		oldCall := mkNode("function_call_expression", "")
		oldCall.Language = "php"
		oldArgs := mkNode("arguments", "")
		oldArgs.Language = "php"
		oldArg := mkNode("argument", "")
		oldArg.Language = "php"
		oldExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		oldExpr.Language = "php"
		oldExpr.StartRow, oldExpr.EndRow = 98, 98

		oldName := mkNode("name", "pack")
		oldName.Language = "php"
		oldCall.Children = []*treesitter.ASTNode{oldName, oldArgs}
		oldName.Parent = oldCall
		oldArgs.Parent = oldCall
		oldArgs.Children = []*treesitter.ASTNode{oldArg}
		oldArg.Parent = oldArgs
		oldArg.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldArg

		newOuterCall := mkNode("function_call_expression", "")
		newOuterCall.Language = "php"
		newOuterName := mkNode("name", "hex2bin")
		newOuterName.Language = "php"
		newOuterArgs := mkNode("arguments", "")
		newOuterArgs.Language = "php"
		newOuterCall.Children = []*treesitter.ASTNode{newOuterName, newOuterArgs}
		newOuterName.Parent = newOuterCall
		newOuterArgs.Parent = newOuterCall

		newNestedCall := mkNode("member_call_expression", "")
		newNestedCall.Language = "php"
		newReceiver := mkNode("variable_name", "$this")
		newReceiver.Language = "php"
		newNestedArgs := mkNode("arguments", "")
		newNestedArgs.Language = "php"
		newNestedCall.Children = []*treesitter.ASTNode{newReceiver, newNestedArgs}
		newReceiver.Parent = newNestedCall
		newNestedArgs.Parent = newNestedCall

		newOuterArgs.Children = []*treesitter.ASTNode{newNestedCall}
		newNestedCall.Parent = newOuterArgs

		newArg := mkNode("argument", "")
		newArg.Language = "php"
		newExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		newExpr.Language = "php"
		newExpr.StartRow, newExpr.EndRow = 99, 99

		newNestedArgs.Children = []*treesitter.ASTNode{newArg}
		newArg.Parent = newNestedArgs
		newArg.Children = []*treesitter.ASTNode{newExpr}
		newExpr.Parent = newArg

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
		oldImport := mkNode("import_statement", "")
		oldImport.Language = "typescript"
		oldClause := mkNode("import_clause", "")
		oldClause.Language = "typescript"
		oldNamed := mkNode("named_imports", "")
		oldNamed.Language = "typescript"
		oldSpec := mkNode("import_specifier", "")
		oldSpec.Language = "typescript"
		oldID := mkNode("identifier", "util")
		oldID.Language = "typescript"

		oldImport.Children = []*treesitter.ASTNode{oldClause}
		oldClause.Parent = oldImport
		oldClause.Children = []*treesitter.ASTNode{oldNamed}
		oldNamed.Parent = oldClause
		oldNamed.Children = []*treesitter.ASTNode{oldSpec}
		oldSpec.Parent = oldNamed
		oldSpec.Children = []*treesitter.ASTNode{oldID}
		oldID.Parent = oldSpec

		newImport := mkNode("import_statement", "")
		newImport.Language = "typescript"
		newClause := mkNode("import_clause", "")
		newClause.Language = "typescript"
		newNamespace := mkNode("namespace_import", "")
		newNamespace.Language = "typescript"
		newID := mkNode("identifier", "util")
		newID.Language = "typescript"

		newImport.Children = []*treesitter.ASTNode{newClause}
		newClause.Parent = newImport
		newClause.Children = []*treesitter.ASTNode{newNamespace}
		newNamespace.Parent = newClause
		newNamespace.Children = []*treesitter.ASTNode{newID}
		newID.Parent = newNamespace

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
		oldBin := mkNode("binary_expression", "!=")
		oldBin.Language = "zig"
		oldLHS := mkNode("field_expression", "stream.positional")
		oldLHS.Language = "zig"
		oldRHS := mkNode("field_expression", "arg.param")
		oldRHS.Language = "zig"
		oldBin.Children = []*treesitter.ASTNode{oldLHS, oldRHS}
		oldLHS.Parent = oldBin
		oldRHS.Parent = oldBin

		newBin := mkNode("binary_expression", "!=")
		newBin.Language = "zig"
		newCall := mkNode("call_expression", "")
		newCall.Language = "zig"
		newLHS := mkNode("field_expression", "arg.param")
		newLHS.Language = "zig"
		newCall.Children = []*treesitter.ASTNode{newLHS}
		newLHS.Parent = newCall
		newRHS := mkNode("enum_literal", ".positional")
		newRHS.Language = "zig"
		newBin.Children = []*treesitter.ASTNode{newCall, newRHS}
		newCall.Parent = newBin
		newRHS.Parent = newBin

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
		oldIf := mkNode("if_statement", "")
		oldIf.Language = "c"
		oldParen := mkNode("parenthesized_expression", "")
		oldParen.Language = "c"
		oldField := mkNode("field_expression", "server.allow_access_expired")
		oldField.Language = "c"
		oldField.StartRow = 10
		oldField.EndRow = 10
		oldField.StartCol = 8
		oldField.EndCol = 35
		oldIf.Children = []*treesitter.ASTNode{oldParen}
		oldParen.Parent = oldIf
		oldParen.Children = []*treesitter.ASTNode{oldField}
		oldField.Parent = oldParen

		newIf := mkNode("if_statement", "")
		newIf.Language = "c"
		newParen := mkNode("parenthesized_expression", "")
		newParen.Language = "c"
		newBin := mkNode("binary_expression", "&&")
		newBin.Language = "c"
		newLHS := mkNode("identifier", "subtractExpiredFields")
		newLHS.Language = "c"
		newField := mkNode("field_expression", "server.allow_access_expired")
		newField.Language = "c"
		newField.StartRow = 12
		newField.EndRow = 12
		newField.StartCol = 33
		newField.EndCol = 60
		newIf.Children = []*treesitter.ASTNode{newParen}
		newParen.Parent = newIf
		newParen.Children = []*treesitter.ASTNode{newBin}
		newBin.Parent = newParen
		newBin.Children = []*treesitter.ASTNode{newLHS, newField}
		newLHS.Parent = newBin
		newField.Parent = newBin

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
		oldIf := mkNode("if_statement", "")
		oldIf.Language = "go"
		oldIfKw := mkNode("if", "if")
		oldIfKw.Language = "go"
		oldOuterBin := mkNode("binary_expression", "||")
		oldOuterBin.Language = "go"
		oldInnerBin := mkNode("binary_expression", "||")
		oldInnerBin.Language = "go"
		oldCall := mkNode("call_expression", "r.IsCall(n.Type)")
		oldCall.Language = "go"
		oldBlock := mkNode("block", "")
		oldBlock.Language = "go"

		oldIf.Children = []*treesitter.ASTNode{oldIfKw, oldOuterBin, oldBlock}
		oldIfKw.Parent = oldIf
		oldOuterBin.Parent = oldIf
		oldBlock.Parent = oldIf
		oldOuterBin.Children = []*treesitter.ASTNode{oldInnerBin, oldCall}
		oldInnerBin.Parent = oldOuterBin
		oldCall.Parent = oldOuterBin

		newIf := mkNode("if_statement", "")
		newIf.Language = "go"
		newIfKw := mkNode("if", "if")
		newIfKw.Language = "go"
		newCondition := mkNode("binary_expression", "||")
		newCondition.Language = "go"
		newBlock := mkNode("block", "")
		newBlock.Language = "go"

		newIf.Children = []*treesitter.ASTNode{newIfKw, newCondition, newBlock}
		newIfKw.Parent = newIf
		newCondition.Parent = newIf
		newBlock.Parent = newIf

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
}

func TestNormalizeCrossScopeNonStructuralMoves(t *testing.T) {
	t.Run("demotes cross-scope distant type move to delete and insert", func(t *testing.T) {
		oldFn := mkNode("function_declaration", "resolveColHighlights")
		oldFn.Language = "go"
		oldSlice := mkNode("slice_type", "")
		oldSlice.Language = "go"
		oldSlice.StartRow = 506
		oldSlice.EndRow = 506
		oldFn.Children = append(oldFn.Children, oldSlice)
		oldSlice.Parent = oldFn

		newStruct := mkNode("type_declaration", "inlineScratch")
		newStruct.Language = "go"
		newField := mkNode("field_declaration", "colHighlight")
		newField.Language = "go"
		newSlice := mkNode("slice_type", "")
		newSlice.Language = "go"
		newSlice.StartRow = 45
		newSlice.EndRow = 45
		newField.Children = append(newField.Children, newSlice)
		newSlice.Parent = newField
		newStruct.Children = append(newStruct.Children, newField)
		newField.Parent = newStruct

		ms := engine.NewMapping()
		ms.Add(oldSlice, newSlice)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldSlice, DestNode: newSlice})

		result := normalizeCrossScopeNonStructuralMoves(es, ms)

		hasDelete := false
		hasInsert := false
		hasMove := false
		for _, a := range result.Actions() {
			if a.Type == actions.Delete && a.Node == oldSlice {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newSlice {
				hasInsert = true
			}
			if a.Type == actions.Move {
				hasMove = true
			}
		}
		if hasMove {
			t.Error("expected cross-scope distant type move to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v hasInsert=%v", hasDelete, hasInsert)
		}
		if ms.Has(oldSlice) || ms.HasDst(newSlice) {
			t.Error("expected oldSlice and newSlice to be unmapped from ms")
		}
	})

	t.Run("preserves local type move within same scope", func(t *testing.T) {
		fn := mkNode("function_declaration", "foo")
		fn.Language = "go"
		oldSlice := mkNode("slice_type", "")
		oldSlice.Language = "go"
		oldSlice.StartRow = 10
		oldSlice.EndRow = 10
		fn.Children = append(fn.Children, oldSlice)
		oldSlice.Parent = fn

		newFn := mkNode("function_declaration", "foo")
		newFn.Language = "go"
		newSlice := mkNode("slice_type", "")
		newSlice.Language = "go"
		newSlice.StartRow = 12
		newSlice.EndRow = 12
		newFn.Children = append(newFn.Children, newSlice)
		newSlice.Parent = newFn

		ms := engine.NewMapping()
		ms.Add(fn, newFn)
		ms.Add(oldSlice, newSlice)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldSlice, DestNode: newSlice})

		result := normalizeCrossScopeNonStructuralMoves(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected local intra-scope type move to be preserved, got %d actions", result.Size())
		}
	})
}

func TestNormalizeOrphanedOperatorMoves(t *testing.T) {
	t.Run("demotes operator move when parents are unmapped or different", func(t *testing.T) {
		oldStmt := mkNode("short_var_declaration", "")
		oldStmt.Language = "go"
		oldOp := mkNode("assignment_operator_literal", ":=")
		oldOp.Language = "go"
		oldStmt.Children = append(oldStmt.Children, oldOp)
		oldOp.Parent = oldStmt

		newStmt := mkNode("assignment_statement", "")
		newStmt.Language = "go"
		newOp := mkNode("assignment_operator_literal", "=")
		newOp.Language = "go"
		newStmt.Children = append(newStmt.Children, newOp)
		newOp.Parent = newStmt

		ms := engine.NewMapping()
		ms.Add(oldOp, newOp)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldOp, DestNode: newOp})

		result := normalizeOrphanedOperatorMoves(es, ms)
		hasMove := false
		hasDelete := false
		hasInsert := false
		for _, a := range result.Actions() {
			if a.Type == actions.Move {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldOp {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newOp {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected orphaned operator move to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})

	t.Run("preserves operator move when parent container is matched", func(t *testing.T) {
		oldParent := mkNode("binary_expression", "")
		oldParent.Language = "go"
		oldOp := mkNode("comparison_operator_literal", "==")
		oldOp.Language = "go"
		oldParent.Children = append(oldParent.Children, oldOp)
		oldOp.Parent = oldParent

		newParent := mkNode("binary_expression", "")
		newParent.Language = "go"
		newOp := mkNode("comparison_operator_literal", "==")
		newOp.Language = "go"
		newParent.Children = append(newParent.Children, newOp)
		newOp.Parent = newParent

		ms := engine.NewMapping()
		ms.Add(oldParent, newParent)
		ms.Add(oldOp, newOp)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldOp, DestNode: newOp})

		result := normalizeOrphanedOperatorMoves(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected operator move within matched parent to be preserved, got %d actions", result.Size())
		}
	})
}

func TestNormalizeControlFlowMoves(t *testing.T) {
	t.Run("demotes cross-hunk if move when bodies share zero statements", func(t *testing.T) {
		oldIf := mkNode("if_statement", "")
		oldIf.StartRow = 10
		oldIf.Language = "go"
		oldCond := mkNode("selector_expression", "opts.Color")
		oldCond.Language = "go"
		oldBody := mkNode("block", "")
		oldBody.Language = "go"
		oldStmt := mkNode("assignment_statement", "headerStyle = ...")
		oldStmt.Language = "go"
		oldBody.Children = append(oldBody.Children, oldStmt)
		oldStmt.Parent = oldBody
		oldIf.Children = append(oldIf.Children, oldCond, oldBody)
		oldCond.Parent = oldIf
		oldBody.Parent = oldIf

		newIf := mkNode("if_statement", "")
		newIf.StartRow = 200
		newIf.Language = "go"
		newCond := mkNode("selector_expression", "opts.Color")
		newCond.Language = "go"
		newBody := mkNode("block", "")
		newBody.Language = "go"
		newStmt := mkNode("expression_statement", "out.WriteString(...)")
		newStmt.Language = "go"
		newBody.Children = append(newBody.Children, newStmt)
		newStmt.Parent = newBody
		newIf.Children = append(newIf.Children, newCond, newBody)
		newCond.Parent = newIf
		newBody.Parent = newIf

		ms := engine.NewMapping()
		ms.Add(oldIf, newIf)
		ms.Add(oldCond, newCond)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldIf, DestNode: newIf})

		result := normalizeControlFlowMoves(es, ms)

		var hasMove, hasDelete, hasInsert bool
		for _, a := range result.Actions() {
			if a.Type == actions.Move && a.Node == oldIf {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldIf {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newIf {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected cross-hunk if move with unmatched bodies to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})

	t.Run("preserves if move when body is matched", func(t *testing.T) {
		oldIf := mkNode("if_statement", "")
		oldIf.StartRow = 10
		oldIf.Language = "go"
		oldBody := mkNode("block", "")
		oldBody.Language = "go"
		oldStmt := mkNode("return_statement", "return true")
		oldStmt.Language = "go"
		oldBody.Children = append(oldBody.Children, oldStmt)
		oldStmt.Parent = oldBody
		oldIf.Children = append(oldIf.Children, oldBody)
		oldBody.Parent = oldIf

		newIf := mkNode("if_statement", "")
		newIf.StartRow = 50
		newIf.Language = "go"
		newBody := mkNode("block", "")
		newBody.Language = "go"
		newStmt := mkNode("return_statement", "return true")
		newStmt.Language = "go"
		newBody.Children = append(newBody.Children, newStmt)
		newStmt.Parent = newBody
		newIf.Children = append(newIf.Children, newBody)
		newBody.Parent = newIf

		ms := engine.NewMapping()
		ms.Add(oldIf, newIf)
		ms.Add(oldBody, newBody)
		ms.Add(oldStmt, newStmt)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldIf, DestNode: newIf})

		result := normalizeControlFlowMoves(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected if move with matched body to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("demotes function_declaration move when consequence body is completely unmatched across hunks", func(t *testing.T) {
		oldFunc := mkNode("function_declaration", "")
		oldFunc.StartRow = 10
		oldFunc.Language = "go"
		oldName := mkNode("identifier", "Foo")
		oldName.Language = "go"
		oldParams := mkNode("parameter_list", "(t *testing.T)")
		oldParams.Language = "go"
		oldBody := mkNode("block", "{ stmt1 }")
		oldBody.Language = "go"
		oldFunc.Children = append(oldFunc.Children, oldName, oldParams, oldBody)
		oldName.Parent = oldFunc
		oldParams.Parent = oldFunc
		oldBody.Parent = oldFunc

		newFunc := mkNode("function_declaration", "")
		newFunc.StartRow = 50
		newFunc.Language = "go"
		newName := mkNode("identifier", "Bar")
		newName.Language = "go"
		newParams := mkNode("parameter_list", "(t *testing.T)")
		newParams.Language = "go"
		newBody := mkNode("block", "{ stmt2 }")
		newBody.Language = "go"
		newFunc.Children = append(newFunc.Children, newName, newParams, newBody)
		newName.Parent = newFunc
		newParams.Parent = newFunc
		newBody.Parent = newFunc

		ms := engine.NewMapping()
		ms.Add(oldFunc, newFunc)
		ms.Add(oldParams, newParams)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFunc, DestNode: newFunc})

		result := normalizeControlFlowMoves(es, ms)

		var hasMove, hasDelete, hasInsert bool
		for _, a := range result.Actions() {
			if a.Type == actions.Move && a.Node == oldFunc {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldFunc {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newFunc {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected function_declaration move with unmatched body to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})

	t.Run("demotes for_clause move when parent for_statement did not move", func(t *testing.T) {
		oldFor := mkNode("for_statement", "")
		oldFor.StartRow = 10
		oldFor.Language = "go"
		oldClause := mkNode("for_clause", "i := 0; i < n; i++")
		oldClause.StartRow = 10
		oldClause.Language = "go"
		oldBody := mkNode("block", "")
		oldBody.Language = "go"
		oldFor.Children = append(oldFor.Children, oldClause, oldBody)
		oldClause.Parent = oldFor
		oldBody.Parent = oldFor

		newFor := mkNode("for_statement", "")
		newFor.StartRow = 50
		newFor.Language = "go"
		newClause := mkNode("for_clause", "i := 0; i < n; i++")
		newClause.StartRow = 50
		newClause.Language = "go"
		newBody := mkNode("block", "")
		newBody.Language = "go"
		newFor.Children = append(newFor.Children, newClause, newBody)
		newClause.Parent = newFor
		newBody.Parent = newFor

		ms := engine.NewMapping()
		ms.Add(oldClause, newClause)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldClause, DestNode: newClause})

		result := normalizeControlFlowMoves(es, ms)

		var hasMove, hasDelete, hasInsert bool
		for _, a := range result.Actions() {
			if a.Type == actions.Move && a.Node == oldClause {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldClause {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newClause {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected for_clause move with unmatched parent to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})
}

func TestNormalizeOrphanedCallArgumentMoves(t *testing.T) {
	t.Run("demotes argument move across distant hunks when calls do not match", func(t *testing.T) {
		oldCall := mkNode("call_expression", "")
		oldCall.Language = "go"
		oldCall.StartRow = 20
		oldArgs := mkNode("argument_list", "")
		oldArgs.Language = "go"
		oldArg := mkNode("selector_expression", "l.text")
		oldArg.Language = "go"
		oldArg.StartRow = 21
		oldArgs.Children = append(oldArgs.Children, oldArg)
		oldArg.Parent = oldArgs
		oldCall.Children = append(oldCall.Children, oldArgs)
		oldArgs.Parent = oldCall

		newCall := mkNode("call_expression", "")
		newCall.Language = "go"
		newCall.StartRow = 100
		newArgs := mkNode("argument_list", "")
		newArgs.Language = "go"
		newArg := mkNode("selector_expression", "l.text")
		newArg.Language = "go"
		newArg.StartRow = 101
		newArgs.Children = append(newArgs.Children, newArg)
		newArg.Parent = newArgs
		newCall.Children = append(newCall.Children, newArgs)
		newArgs.Parent = newCall

		ms := engine.NewMapping()
		ms.Add(oldArg, newArg)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldArg, DestNode: newArg})

		result := normalizeOrphanedCallArgumentMoves(es, ms)

		var hasMove, hasDelete, hasInsert bool
		for _, a := range result.Actions() {
			if a.Type == actions.Move && a.Node == oldArg {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldArg {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newArg {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected orphaned call argument move across distant hunks to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})

	t.Run("preserves swapped arguments within the same matched call", func(t *testing.T) {
		oldCall := mkNode("call_expression", "")
		oldCall.Language = "go"
		oldCall.StartRow = 20
		oldArgs := mkNode("argument_list", "")
		oldArgs.Language = "go"
		oldArg := mkNode("identifier", "a")
		oldArg.Language = "go"
		oldArg.StartRow = 20
		oldArgs.Children = append(oldArgs.Children, oldArg)
		oldArg.Parent = oldArgs
		oldCall.Children = append(oldCall.Children, oldArgs)
		oldArgs.Parent = oldCall

		newCall := mkNode("call_expression", "")
		newCall.Language = "go"
		newCall.StartRow = 20
		newArgs := mkNode("argument_list", "")
		newArgs.Language = "go"
		newArg := mkNode("identifier", "a")
		newArg.Language = "go"
		newArg.StartRow = 20
		newArgs.Children = append(newArgs.Children, newArg)
		newArg.Parent = newArgs
		newCall.Children = append(newCall.Children, newArgs)
		newArgs.Parent = newCall

		ms := engine.NewMapping()
		ms.Add(oldCall, newCall)
		ms.Add(oldArg, newArg)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldArg, DestNode: newArg})

		result := normalizeOrphanedCallArgumentMoves(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected argument move within matched call to be preserved, got %d actions", result.Size())
		}
	})
}

func TestNormalizeOrphanedDeclarationParameterMoves(t *testing.T) {
	t.Run("demotes parameter move across distant hunks when declarations do not match", func(t *testing.T) {
		oldDecl := mkNode("function_declaration", "")
		oldDecl.Language = "go"
		oldDecl.StartRow = 20
		oldParams := mkNode("parameter_list", "")
		oldParams.Language = "go"
		oldParam := mkNode("parameter_declaration", "color bool")
		oldParam.Language = "go"
		oldParam.StartRow = 21
		oldParams.Children = append(oldParams.Children, oldParam)
		oldParam.Parent = oldParams
		oldDecl.Children = append(oldDecl.Children, oldParams)
		oldParams.Parent = oldDecl

		newDecl := mkNode("function_declaration", "")
		newDecl.Language = "go"
		newDecl.StartRow = 100
		newParams := mkNode("parameter_list", "")
		newParams.Language = "go"
		newParam := mkNode("parameter_declaration", "color bool")
		newParam.Language = "go"
		newParam.StartRow = 101
		newParams.Children = append(newParams.Children, newParam)
		newParam.Parent = newParams
		newDecl.Children = append(newDecl.Children, newParams)
		newParams.Parent = newDecl

		ms := engine.NewMapping()
		ms.Add(oldParam, newParam)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldParam, DestNode: newParam})

		result := normalizeOrphanedDeclarationParameterMoves(es, ms)

		var hasMove, hasDelete, hasInsert bool
		for _, a := range result.Actions() {
			if a.Type == actions.Move && a.Node == oldParam {
				hasMove = true
			}
			if a.Type == actions.Delete && a.Node == oldParam {
				hasDelete = true
			}
			if a.Type == actions.Insert && a.Node == newParam {
				hasInsert = true
			}
		}

		if hasMove {
			t.Error("expected orphaned declaration parameter move to be demoted, but Move survived")
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected separate Delete and Insert actions, got hasDelete=%v, hasInsert=%v", hasDelete, hasInsert)
		}
	})

	t.Run("preserves parameter move when enclosing declaration matches", func(t *testing.T) {
		oldDecl := mkNode("function_declaration", "")
		oldDecl.Language = "go"
		oldDecl.StartRow = 20
		oldParams := mkNode("parameter_list", "")
		oldParams.Language = "go"
		oldParam := mkNode("parameter_declaration", "color bool")
		oldParam.Language = "go"
		oldParam.StartRow = 20
		oldParams.Children = append(oldParams.Children, oldParam)
		oldParam.Parent = oldParams
		oldDecl.Children = append(oldDecl.Children, oldParams)
		oldParams.Parent = oldDecl

		newDecl := mkNode("function_declaration", "")
		newDecl.Language = "go"
		newDecl.StartRow = 20
		newParams := mkNode("parameter_list", "")
		newParams.Language = "go"
		newParam := mkNode("parameter_declaration", "color bool")
		newParam.Language = "go"
		newParam.StartRow = 20
		newParams.Children = append(newParams.Children, newParam)
		newParam.Parent = newParams
		newDecl.Children = append(newDecl.Children, newParams)
		newParams.Parent = newDecl

		ms := engine.NewMapping()
		ms.Add(oldDecl, newDecl)
		ms.Add(oldParam, newParam)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldParam, DestNode: newParam})

		result := normalizeOrphanedDeclarationParameterMoves(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected parameter move within matched declaration to be preserved, got %d actions", result.Size())
		}
	})
}
