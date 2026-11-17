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

	t.Run("preserves Move when variable declaration is hoisted out of if_statement", func(t *testing.T) {
		oldBlock := mkNode("block", "")
		oldBlock.Language = "go"
		oldIf := mkNode("if_statement", "")
		oldIf.Language = "go"
		oldDecl := mkNode("short_var_declaration", "r := rulesFor(t1)")
		oldDecl.Language = "go"
		oldBlock.Children = []*treesitter.ASTNode{oldIf}
		oldIf.Parent = oldBlock
		oldIf.Children = []*treesitter.ASTNode{oldDecl}
		oldDecl.Parent = oldIf

		newBlock := mkNode("block", "")
		newBlock.Language = "go"
		newDecl := mkNode("short_var_declaration", "r := rulesFor(t1)")
		newDecl.Language = "go"
		newIf := mkNode("if_statement", "")
		newIf.Language = "go"
		newBlock.Children = []*treesitter.ASTNode{newDecl, newIf}
		newDecl.Parent = newBlock
		newIf.Parent = newBlock

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
		oldCall := mkNode("call_expression", "")
		oldCall.Language = "javascript"
		oldArgs := mkNode("arguments", "")
		oldArgs.Language = "javascript"
		oldArg := mkNode("argument", "")
		oldArg.Language = "javascript"
		oldExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		oldExpr.Language = "javascript"

		oldCall.Children = []*treesitter.ASTNode{oldArgs}
		oldArgs.Parent = oldCall
		oldArgs.Children = []*treesitter.ASTNode{oldArg}
		oldArg.Parent = oldArgs
		oldArg.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldArg

		newOuterCall := mkNode("call_expression", "")
		newOuterCall.Language = "javascript"
		newOuterArgs := mkNode("arguments", "")
		newOuterArgs.Language = "javascript"
		newNestedCall := mkNode("call_expression", "")
		newNestedCall.Language = "javascript"
		newNestedArgs := mkNode("arguments", "")
		newNestedArgs.Language = "javascript"
		newArg := mkNode("argument", "")
		newArg.Language = "javascript"
		newExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		newExpr.Language = "javascript"

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
		oldCall := mkNode("call_expression", "")
		oldCall.Language = "javascript"
		oldArgs := mkNode("arguments", "")
		oldArgs.Language = "javascript"
		oldArg := mkNode("argument", "")
		oldArg.Language = "javascript"
		oldExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		oldExpr.Language = "javascript"
		oldExpr.StartRow, oldExpr.EndRow = 98, 98

		oldName := mkNode("name", "pack")
		oldName.Language = "javascript"
		oldCall.Children = []*treesitter.ASTNode{oldName, oldArgs}
		oldName.Parent = oldCall
		oldArgs.Parent = oldCall
		oldArgs.Children = []*treesitter.ASTNode{oldArg}
		oldArg.Parent = oldArgs
		oldArg.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldArg

		newOuterCall := mkNode("call_expression", "")
		newOuterCall.Language = "javascript"
		newOuterName := mkNode("name", "hex2bin")
		newOuterName.Language = "javascript"
		newOuterArgs := mkNode("arguments", "")
		newOuterArgs.Language = "javascript"
		newOuterCall.Children = []*treesitter.ASTNode{newOuterName, newOuterArgs}
		newOuterName.Parent = newOuterCall
		newOuterArgs.Parent = newOuterCall

		newNestedCall := mkNode("call_expression", "")
		newNestedCall.Language = "javascript"
		newReceiver := mkNode("variable_name", "$this")
		newReceiver.Language = "javascript"
		newNestedArgs := mkNode("arguments", "")
		newNestedArgs.Language = "javascript"
		newNestedCall.Children = []*treesitter.ASTNode{newReceiver, newNestedArgs}
		newReceiver.Parent = newNestedCall
		newNestedArgs.Parent = newNestedCall

		newOuterArgs.Children = []*treesitter.ASTNode{newNestedCall}
		newNestedCall.Parent = newOuterArgs

		newArg := mkNode("argument", "")
		newArg.Language = "javascript"
		newExpr := mkNode("member_call_expression", "$localIdentifier->toString()")
		newExpr.Language = "javascript"
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

	t.Run("drops Move when boolean operator precedence or parens rebalance within if condition", func(t *testing.T) {
		// Old: if (A || B) || C { block }
		oldAId := mkNode("identifier", "A")
		oldBId := mkNode("identifier", "B")
		oldCId := mkNode("identifier", "C")
		oldA := mkNode("binary_expression", "==", oldAId)
		oldB := mkNode("binary_expression", "<=", oldBId)
		oldC := mkNode("call_expression", "", oldCId)
		oldLeftBin := mkNode("binary_expression", "||", oldA, oldB)
		oldRootBin := mkNode("binary_expression", "||", oldLeftBin, oldC)
		oldBlock := mkNode("block", "")
		oldIf := mkNode("if_statement", "", mkNode("if", "if"), oldRootBin, oldBlock)
		oldOuterBlock := mkNode("block", "", oldIf)
		setLanguageRecursive(oldOuterBlock, "go")

		// New: if A && (B || C) { block }
		newAId := mkNode("identifier", "A")
		newBId := mkNode("identifier", "B")
		newCId := mkNode("identifier", "C")
		newA := mkNode("binary_expression", "==", newAId)
		newB := mkNode("binary_expression", "<=", newBId)
		newC := mkNode("call_expression", "", newCId)
		newRightBin := mkNode("binary_expression", "||", newB, newC)
		newParen := mkNode("parenthesized_expression", "", newRightBin)
		newRootBin := mkNode("binary_expression", "&&", newA, newParen)
		newBlock := mkNode("block", "")
		newIf := mkNode("if_statement", "", mkNode("if", "if"), newRootBin, newBlock)
		newOuterBlock := mkNode("block", "", newIf)
		setLanguageRecursive(newOuterBlock, "go")

		oldA.StartByte, oldA.EndByte = 10, 20
		oldAId.StartByte, oldAId.EndByte = 10, 20
		oldB.StartByte, oldB.EndByte = 30, 40
		oldBId.StartByte, oldBId.EndByte = 30, 40
		oldC.StartByte, oldC.EndByte = 50, 60
		oldCId.StartByte, oldCId.EndByte = 50, 60
		oldLeftBin.StartByte, oldLeftBin.EndByte = 10, 40
		oldRootBin.StartByte, oldRootBin.EndByte = 10, 60

		newA.StartByte, newA.EndByte = 10, 20
		newAId.StartByte, newAId.EndByte = 10, 20
		newB.StartByte, newB.EndByte = 35, 45
		newBId.StartByte, newBId.EndByte = 35, 45
		newC.StartByte, newC.EndByte = 55, 65
		newCId.StartByte, newCId.EndByte = 55, 65
		newRightBin.StartByte, newRightBin.EndByte = 35, 65
		newParen.StartByte, newParen.EndByte = 34, 66
		newRootBin.StartByte, newRootBin.EndByte = 10, 66

		ms := engine.NewMapping()
		ms.Add(oldOuterBlock, newOuterBlock)
		ms.Add(oldIf, newIf)
		ms.Add(oldBlock, newBlock)
		ms.Add(oldA, newA)
		ms.Add(oldAId, newAId)
		ms.Add(oldB, newB)
		ms.Add(oldBId, newBId)
		ms.Add(oldC, newC)
		ms.Add(oldCId, newCId)
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
		oldAId := mkNode("identifier", "A")
		oldBId := mkNode("identifier", "B")
		oldA := mkNode("binary_expression", "==", oldAId)
		oldB := mkNode("binary_expression", "==", oldBId)
		oldRootBin := mkNode("binary_expression", "||", oldA, oldB)
		oldIf := mkNode("if_statement", "", oldRootBin)
		oldOuterBlock := mkNode("block", "", oldIf)
		setLanguageRecursive(oldOuterBlock, "go")

		oldA.StartByte, oldA.EndByte = 10, 20
		oldA.StartCol, oldA.EndCol = 10, 20
		oldAId.StartByte, oldAId.EndByte = 10, 20
		oldB.StartByte, oldB.EndByte = 30, 40
		oldBId.StartByte, oldBId.EndByte = 30, 40

		newAId := mkNode("identifier", "A")
		newBId := mkNode("identifier", "B")
		newA := mkNode("binary_expression", "==", newAId)
		newB := mkNode("binary_expression", "==", newBId)
		newParen := mkNode("parenthesized_expression", "", newB)
		newRootBin := mkNode("binary_expression", "||", newParen, newA)
		newIf := mkNode("if_statement", "", newRootBin)
		newOuterBlock := mkNode("block", "", newIf)
		setLanguageRecursive(newOuterBlock, "go")

		// B is now before A in document order.
		newParen.StartByte, newParen.EndByte = 10, 24
		newB.StartByte, newB.EndByte = 12, 22
		newBId.StartByte, newBId.EndByte = 12, 22
		newA.StartByte, newA.EndByte = 26, 36
		newA.StartCol, newA.EndCol = 26, 36
		newAId.StartByte, newAId.EndByte = 26, 36

		ms := engine.NewMapping()
		ms.Add(oldOuterBlock, newOuterBlock)
		ms.Add(oldIf, newIf)
		ms.Add(oldA, newA)
		ms.Add(oldAId, newAId)
		ms.Add(oldB, newB)
		ms.Add(oldBId, newBId)
		ms.Add(oldRootBin, newRootBin)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldA, DestNode: newA})

		result := normalizeStationaryWrapperMoves(es, ms)
		if result.Size() != 1 {
			t.Fatalf("expected swapped operand move to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("does not treat operator-only shell inside same statement as stationary expression move", func(t *testing.T) {
		oldOp := mkNode("logical_operator_literal", "&&")
		oldBin := mkNode("binary_expression", "&&", mkNode("identifier", "a"), oldOp, mkNode("identifier", "b"))
		oldParen := mkNode("parenthesized_expression", "", oldBin)
		oldRoot := mkNode("binary_expression", "||", oldParen, mkNode("identifier", "keep"))
		oldIf := mkNode("if_statement", "", oldRoot)
		oldOuter := mkNode("block", "", oldIf)
		setLanguageRecursive(oldOuter, "go")

		newOp := mkNode("logical_operator_literal", "&&")
		newBin := mkNode("binary_expression", "&&", mkNode("identifier", "c"), newOp, mkNode("identifier", "d"))
		newParen := mkNode("parenthesized_expression", "", newBin)
		newRoot := mkNode("binary_expression", "||", newParen, mkNode("identifier", "other"))
		newIf := mkNode("if_statement", "", newRoot)
		newOuter := mkNode("block", "", newIf)
		setLanguageRecursive(newOuter, "go")

		oldParen.StartCol, oldParen.EndCol = 3, 11
		newParen.StartCol, newParen.EndCol = 3, 15

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
		oldClass := mkNode("class_specifier", "")
		oldClass.Language = "cpp"
		w1 := mkNode("friend_declaration", "")
		w1.Language = "cpp"
		w2 := mkNode("friend_declaration", "")
		w2.Language = "cpp"
		w3 := mkNode("friend_declaration", "")
		w3.Language = "cpp"
		w4 := mkNode("friend_declaration", "")
		w4.Language = "cpp"
		oldFn := mkNode("function_definition", "foo")
		oldFn.Language = "cpp"
		oldClass.Children = []*treesitter.ASTNode{w1}
		w1.Parent = oldClass
		w1.Children = []*treesitter.ASTNode{w2}
		w2.Parent = w1
		w2.Children = []*treesitter.ASTNode{w3}
		w3.Parent = w2
		w3.Children = []*treesitter.ASTNode{w4}
		w4.Parent = w3
		w4.Children = []*treesitter.ASTNode{oldFn}
		oldFn.Parent = w4

		newClass := mkNode("class_specifier", "")
		newClass.Language = "cpp"
		newFn := mkNode("function_definition", "foo")
		newFn.Language = "cpp"
		newClass.Children = []*treesitter.ASTNode{newFn}
		newFn.Parent = newClass

		ms := engine.NewMapping()
		ms.Add(oldClass, newClass)
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFn, DestNode: newFn})

		result := normalizeStationaryWrapperMoves(es, ms)
		// With maxUnwrapDepth = 3, 4 hops cannot unwrap to oldClass, so move is preserved
		if result.Size() != 1 {
			t.Fatalf("expected move to be preserved across >3 wrapper hops, got %d actions", result.Size())
		}
	})

	t.Run("preserves move when wrapper unwrap is asymmetric", func(t *testing.T) {
		oldClass := mkNode("class_specifier", "")
		oldClass.Language = "cpp"
		w1 := mkNode("friend_declaration", "")
		w1.Language = "cpp"
		w2 := mkNode("friend_declaration", "")
		w2.Language = "cpp"
		w3 := mkNode("friend_declaration", "")
		w3.Language = "cpp"
		oldFn := mkNode("function_definition", "foo")
		oldFn.Language = "cpp"
		oldClass.Children = []*treesitter.ASTNode{w1}
		w1.Parent = oldClass
		w1.Children = []*treesitter.ASTNode{w2}
		w2.Parent = w1
		w2.Children = []*treesitter.ASTNode{w3}
		w3.Parent = w2
		w3.Children = []*treesitter.ASTNode{oldFn}
		oldFn.Parent = w3

		newClass := mkNode("class_specifier", "")
		newClass.Language = "cpp"
		nw1 := mkNode("friend_declaration", "")
		nw1.Language = "cpp"
		newFn := mkNode("function_definition", "foo")
		newFn.Language = "cpp"
		newClass.Children = []*treesitter.ASTNode{nw1}
		nw1.Parent = newClass
		nw1.Children = []*treesitter.ASTNode{newFn}
		newFn.Parent = nw1

		ms := engine.NewMapping()
		ms.Add(oldClass, newClass)
		ms.Add(oldFn, newFn)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: oldFn, DestNode: newFn})

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
		src := mkNode("identifier", "x")
		src.Language = "go"
		dst := mkNode("identifier", "y")
		dst.Language = "go"
		if sameScopeDeclaration(src, dst, nil, r) {
			t.Error("expected false for nil mapping")
		}
	})

	t.Run("both outside any container returns true", func(t *testing.T) {
		ms := engine.NewMapping()
		src := mkNode("identifier", "x")
		src.Language = "go"
		dst := mkNode("identifier", "y")
		dst.Language = "go"
		if !sameScopeDeclaration(src, dst, ms, r) {
			t.Error("expected true when both nodes are outside any container")
		}
	})

	t.Run("matched enclosing containers returns true", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcStmt := mkNode("expression_statement", "")
		srcStmt.Language = "go"
		srcStmt.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcStmt}

		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstStmt := mkNode("expression_statement", "")
		dstStmt.Language = "go"
		dstStmt.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstStmt}

		ms := engine.NewMapping()
		ms.Add(srcFunc, dstFunc)

		if !sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
			t.Error("expected true when enclosing containers are mapped")
		}
	})

	t.Run("unmatched enclosing containers returns false", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcStmt := mkNode("expression_statement", "")
		srcStmt.Language = "go"
		srcStmt.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcStmt}

		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstStmt := mkNode("expression_statement", "")
		dstStmt.Language = "go"
		dstStmt.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstStmt}

		ms := engine.NewMapping()
		// srcFunc and dstFunc are NOT mapped to each other.

		if sameScopeDeclaration(srcStmt, dstStmt, ms, r) {
			t.Error("expected false when enclosing containers are not mapped")
		}
	})

	t.Run("one inside container one outside returns false", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcStmt := mkNode("expression_statement", "")
		srcStmt.Language = "go"
		srcStmt.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcStmt}

		dst := mkNode("expression_statement", "")
		dst.Language = "go"

		ms := engine.NewMapping()
		if sameScopeDeclaration(srcStmt, dst, ms, r) {
			t.Error("expected false when only one node is inside a container")
		}
	})

	t.Run("Unit/distinct closures in same outer function returns false", func(t *testing.T) {
		outerFunc := mkNode("function_declaration", "outer")
		outerFunc.Language = "go"

		c1 := mkNode("func_literal", "")
		c1.Language = "go"
		c1.Parent = outerFunc

		c2 := mkNode("func_literal", "")
		c2.Language = "go"
		c2.Parent = outerFunc

		stmt1 := mkNode("expression_statement", "")
		stmt1.Language = "go"
		stmt1.Parent = c1
		c1.Children = []*treesitter.ASTNode{stmt1}

		stmt2 := mkNode("expression_statement", "")
		stmt2.Language = "go"
		stmt2.Parent = c2
		c2.Children = []*treesitter.ASTNode{stmt2}

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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "package_clause", Line: 1},
			testutil.NodeQuery{Type: "package_clause", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 12},
			testutil.NodeQuery{Type: "call_expression", Line: 5},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 16},
			testutil.NodeQuery{Type: "call_expression", Line: 9},
		)

		// 4. return nil in dst must be recorded as an Insert, not a Move.
		hasReturnNilInsert := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && r.IsJumpStatement(a.Node.Type) &&
				strings.Contains(string(dst[a.Node.StartByte:a.Node.EndByte]), "return nil")
		})
		if !hasReturnNilInsert {
			t.Error("expected return nil in cleanup to be recorded as an Insert")
		}

		// 5. Entire deleted validate closure must be recorded as a Delete.
		hasValidateDelete := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Delete && a.Node != nil && r.IsLocalVarDeclaration(a.Node.Type) &&
				strings.Contains(string(src[a.Node.StartByte:a.Node.EndByte]), "validate :=")
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

		srcFunc, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "function_declaration", Line: 3})
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "function_declaration", Line: 3})
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && r.IsCall(a.Node.Type) {
				callee := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(callee, "precedingInit") || strings.Contains(callee, "succeedingDone") {
					t.Errorf("expected stationary call to not have Move action, got %v", a)
				}
			}
		}

		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
			testutil.NodeQuery{Type: "parameter_list", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 8},
			testutil.NodeQuery{Type: "call_expression", Line: 25},
		)

		for _, item := range []struct {
			srcLine int
			dstLine int
		}{
			{srcLine: 5, dstLine: 20},
			{srcLine: 6, dstLine: 21},
			{srcLine: 7, dstLine: 22},
		} {
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "short_var_declaration", Line: item.srcLine},
				testutil.NodeQuery{Type: "assignment_statement", Line: item.dstLine},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: item.srcLine},
				testutil.NodeQuery{Type: "call_expression", Line: item.dstLine},
			)

			stmtNode, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "short_var_declaration", Line: item.srcLine})
			if err != nil {
				t.Fatalf("find short_var_declaration on line %d: %v", item.srcLine, err)
			}
			if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
				return a.Type == actions.Move && a.Node == stmtNode
			}) {
				t.Errorf("line %d has no Move action, want extraction into closure", item.srcLine)
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

		srcFunc, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "function_declaration", Line: 1})
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "function_declaration", Line: 1})
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && r.IsCall(a.Node.Type) {
				callee := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(callee, "precedingInit") {
					t.Errorf("expected stationary call to not have Move action, got %v", a)
				}
			}
		}

		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 2},
			testutil.NodeQuery{Type: "call_expression", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3},
			testutil.NodeQuery{Type: "call_expression", Line: 16},
		)

		srcReduce, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "call_expression", Line: 3})
		if err != nil {
			t.Fatalf("find srcReduce: %v", err)
		}
		dstReduce, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "call_expression", Line: 16})
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

		srcFunc, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "function_item", Line: 1})
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "function_item", Line: 1})
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil && r.IsCall(a.Node.Type) {
				callee := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(callee, "preceding_init") {
					t.Errorf("expected stationary call to not have Move action, got %v", a)
				}
			}
		}

		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 2},
			testutil.NodeQuery{Type: "call_expression", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "call_expression", Line: 16, Nth: 1},
		)

		srcSum, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1})
		if err != nil {
			t.Fatalf("find srcSum: %v", err)
		}
		dstSum, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "call_expression", Line: 16, Nth: 1})
		if err != nil {
			t.Fatalf("find dstSum: %v", err)
		}
		if !sameScopeDeclaration(srcSum, dstSum, matchResult.Mappings, r) {
			t.Errorf("expected sameScopeDeclaration to return true for closure_expression inside process_items")
		}
	})
}

func TestClosureScopeBoundaries(t *testing.T) {
	t.Run("FullEngine/java closure does not cross method and class boundaries", func(t *testing.T) {
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "class_declaration", Line: 3},
			testutil.NodeQuery{Type: "class_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_declaration", Line: 4},
			testutil.NodeQuery{Type: "method_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 5},
			testutil.NodeQuery{Type: "method_invocation", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 7},
			testutil.NodeQuery{Type: "method_invocation", Line: 6},
		)

		srcCompute, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "method_invocation", Line: 2})
		if err != nil {
			t.Fatalf("find srcCompute: %v", err)
		}
		dstCompute, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "method_invocation", Line: 5, Nth: 2})
		if err != nil {
			t.Fatalf("find dstCompute: %v", err)
		}
		srcField := srcCompute.Parent.Parent
		dstLambda := dstCompute.Parent
		if srcField == nil || !r.IsDeclaration(srcField.Type) || dstLambda == nil || !r.IsClosure(dstLambda.Type) {
			t.Fatalf("unexpected parsed scopes: field=%v lambda=%v", srcField, dstLambda)
		}
		mr.Mappings.Add(srcField, dstLambda)
		srcOuter, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "class_declaration", Line: 1})
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
		mr.Mappings.Add(srcCompute, dstCompute)
		if sameScopeDeclaration(srcCompute, dstCompute, mr.Mappings, r) {
			t.Error("sameScopeDeclaration() = true for mapped calls across Outer and Inner.run, want false")
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

		srcFunc, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "function_declaration", Line: 3})
		if err != nil {
			t.Fatalf("find srcFunc: %v", err)
		}
		dstFunc, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "function_declaration", Line: 3})
		if err != nil {
			t.Fatalf("find dstFunc: %v", err)
		}
		assertNoActionOn(t, normalized, srcFunc)
		assertNoActionOn(t, normalized, dstFunc)

		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 25},
			testutil.NodeQuery{Type: "call_expression", Line: 8},
		)

		for _, item := range []struct {
			srcLine int
			dstLine int
		}{
			{srcLine: 20, dstLine: 5},
			{srcLine: 21, dstLine: 6},
			{srcLine: 22, dstLine: 7},
		} {
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "assignment_statement", Line: item.srcLine},
				testutil.NodeQuery{Type: "short_var_declaration", Line: item.dstLine},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: item.srcLine},
				testutil.NodeQuery{Type: "call_expression", Line: item.dstLine},
			)

			srcStmt, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "assignment_statement", Line: item.srcLine})
			if err != nil {
				t.Fatalf("find srcStmt on line %d: %v", item.srcLine, err)
			}
			dstStmt, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "short_var_declaration", Line: item.dstLine})
			if err != nil {
				t.Fatalf("find dstStmt on line %d: %v", item.dstLine, err)
			}

			if !slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
				return a.Type == actions.Move && a.Node == srcStmt
			}) {
				t.Errorf("line %d has no Move action, want closure inlining", item.srcLine)
			}
			if !sameScopeDeclaration(srcStmt, dstStmt, mr.Mappings, r) {
				t.Errorf("sameScopeDeclaration() = false for inlined line %d, want true", item.srcLine)
			}
		}
	})
}

func TestNormalizeMovesByStructure_StationaryExpressionMoveDropped(t *testing.T) {
	// Setup: stationary expression restructuring on the same line within the same statement.
	outerBlock := mkNode("block", "")
	outerBlock.Language = "go"

	stmt1 := mkNode("expression_statement", "")
	stmt1.Language = "go"
	stmt1.Parent = outerBlock
	stmt1.StartRow = 10

	call1 := mkNode("call_expression", "")
	call1.Language = "go"
	call1.Parent = stmt1
	call1.StartRow = 10

	stmt2 := mkNode("expression_statement", "")
	stmt2.Language = "go"
	stmt2.Parent = outerBlock
	stmt2.StartRow = 50

	call2 := mkNode("call_expression", "")
	call2.Language = "go"
	call2.Parent = stmt2
	call2.StartRow = 50

	id1 := mkNode("identifier", "foo")
	id1.Language = "go"
	id1.Parent = call1

	id2 := mkNode("identifier", "foo")
	id2.Language = "go"
	id2.Parent = call2

	call1.Children = []*treesitter.ASTNode{id1}
	call2.Children = []*treesitter.ASTNode{id2}
	stmt1.Children = []*treesitter.ASTNode{call1}
	stmt2.Children = []*treesitter.ASTNode{call2}

	ms := engine.NewMapping()
	ms.Add(outerBlock, outerBlock)
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
	if !ms.Has(call1) {
		t.Errorf("expected stationary expression call1 to remain in mapping")
	}
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

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
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
		srcFunc := mkNode("function_declaration", "oldFn")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		srcStmt := mkNode("short_var_declaration", "x := 1")
		srcStmt.Language = "go"
		srcStmt.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{srcStmt}
		srcStmt.StartRow, srcStmt.EndRow = 10, 10
		srcStmt.StartCol, srcStmt.EndCol = 4, 10
		srcStmt.EndByte = 100

		dstFunc := mkNode("function_declaration", "newFn")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		dstStmt := mkNode("short_var_declaration", "x := 1")
		dstStmt.Language = "go"
		dstStmt.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{dstStmt}
		dstStmt.StartRow, dstStmt.EndRow = 10, 10
		dstStmt.StartCol, dstStmt.EndCol = 4, 10
		dstStmt.EndByte = 100

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
		srcFunc := mkNode("function_declaration", "fn")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		srcStmt := mkNode("short_var_declaration", "x := 1")
		srcStmt.Language = "go"
		srcStmt.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{srcStmt}
		srcStmt.StartRow, srcStmt.EndRow = 10, 10
		srcStmt.StartCol, srcStmt.EndCol = 4, 10
		srcStmt.EndByte = 100

		dstFunc := mkNode("function_declaration", "fn")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		dstStmt := mkNode("short_var_declaration", "x := 1")
		dstStmt.Language = "go"
		dstStmt.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{dstStmt}
		dstStmt.StartRow, dstStmt.EndRow = 10, 10
		dstStmt.StartCol, dstStmt.EndCol = 4, 10
		dstStmt.EndByte = 100

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

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
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
		base := mkNode("if_statement", "")
		base.Language = "go"
		parent := mkNode("if_statement", "")
		parent.Language = "go"
		if !areChainedStatements(base, parent, r) {
			t.Errorf("expected areChainedStatements to return true for matching if_statements")
		}
	})

	t.Run("returns false for non-statement nodes with identical types", func(t *testing.T) {
		base := mkNode("identifier", "x")
		base.Language = "go"
		parent := mkNode("identifier", "y")
		parent.Language = "go"
		if areChainedStatements(base, parent, r) {
			t.Errorf("expected areChainedStatements to return false for non-statement identifier nodes")
		}
	})

	t.Run("returns false when base is nil or parent is nil", func(t *testing.T) {
		base := mkNode("if_statement", "")
		base.Language = "go"
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

		matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
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
		hasElementDelete := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Delete && a.Node != nil && string(src[a.Node.StartByte:a.Node.EndByte]) == "{LeftLine: -1, RightLine: 1}"
		})
		if !hasElementDelete {
			t.Error("expected entire deleted collection element to be deleted")
		}

		// 4. Whole-snippet verification: package, var declaration, and untouched element map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "package_clause", Line: 1},
			testutil.NodeQuery{Type: "package_clause", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "var_declaration", Line: 3},
			testutil.NodeQuery{Type: "var_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "literal_value", Line: 5},
			testutil.NodeQuery{Type: "literal_value", Line: 5},
		)
		testutil.AssertUnmapped(t, srcAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "literal_value", Line: 6},
		)
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
		moveContent := strings.TrimSpace(string(src[move.Node.StartByte:move.Node.EndByte]))
		if !strings.Contains(moveContent, "longArg()") {
			t.Errorf("expected moved argument_list to contain longArg(), got %q", moveContent)
		}

		// 3. The other() call arguments should be recorded as an insert
		hasOtherInsert := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Insert && a.Node != nil && strings.Contains(string(dst[a.Node.StartByte:a.Node.EndByte]), "other()")
		})
		if !hasOtherInsert {
			t.Error("expected chained call argument_list with other() to be inserted")
		}

		// 4. Package and function headers still map in place
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "package_clause", Line: 1},
			testutil.NodeQuery{Type: "package_clause", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
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
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 4},
				testutil.NodeQuery{Type: "call_expression", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 6},
				testutil.NodeQuery{Type: "call_expression", Line: 6},
			)

			// 3. Target construct: EnsureIndex(root) must map in place, and its argument list must map in place.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 5},
				testutil.NodeQuery{Type: "call_expression", Line: 5},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "argument_list", Line: 5},
				testutil.NodeQuery{Type: "argument_list", Line: 5},
			)

			// 4. Stationary operand: root identifier inside argument list must map in place.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
				testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 3},
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
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 4},
				testutil.NodeQuery{Type: "call_expression", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 8},
				testutil.NodeQuery{Type: "call_expression", Line: 9},
			)

			// 3. Target construct: extracted slice must be mapped and preserved as Move.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "slice_expression", Line: 6},
				testutil.NodeQuery{Type: "slice_expression", Line: 5},
			)

			// 4. Stationary operands: identifiers inside slice must map in place without corruption.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 1},
				testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 2},
				testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 3},
			)
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 3},
				testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 4},
			)

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
			srcSlice, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "slice_expression", Line: 6})
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
			var hasSliceMove bool
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && r.IsIndexed(a.Node.Type) && string(src[a.Node.StartByte:a.Node.EndByte]) == "fileBytes[gapStart:gapEnd]" {
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
			var hasMove bool
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move && a.Node != nil && r.IsExpression(a.Node.Type) && string(src[a.Node.StartByte:a.Node.EndByte]) == "err != nil" {
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 8},
			testutil.NodeQuery{Type: "expression_statement", Line: 11},
		)

		// 3. Move action: res.Envelope == nil moves into the new guard if
		hasMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move && a.Node != nil && string(src[a.Node.StartByte:a.Node.EndByte]) == "res.Envelope == nil"
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		for _, a := range normalized.Actions() {
			if a.Node != nil && (r.IsDeclaration(a.Node.Type) || r.IsContainerDeclaration(a.Node.Type)) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Surrounding sequence: guard check and trailing log/return statements map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "expression_statement", Line: 12},
			testutil.NodeQuery{Type: "expression_statement", Line: 16},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "return_statement", Line: 13},
			testutil.NodeQuery{Type: "return_statement", Line: 17},
		)

		// 3. Target statement: the labeled assignment maps in place and drops its move.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 11},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 15},
		)

		srcTarget, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "short_var_declaration", Line: 11})
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "selector_expression", Line: 11, Nth: 1},
			testutil.NodeQuery{Type: "selector_expression", Line: 15, Nth: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "selector_expression", Line: 11, Nth: 2},
			testutil.NodeQuery{Type: "selector_expression", Line: 15, Nth: 2},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 6},
			testutil.NodeQuery{Type: "function_item", Line: 6},
		)

		// 2. Surrounding sequence: existing import symbols stay anchored.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3},
		)

		// 3. Target & wrappers: none of the existing import paths or symbols should be marked as moved.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				nodeText := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(nodeText, "side_by_side") {
					t.Errorf("unexpected Move action on side_by_side: %v", a)
				}
				if strings.Contains(nodeText, "GitConfigEntry") {
					t.Errorf("unexpected Move action on GitConfigEntry: %v", a)
				}
				if strings.Contains(nodeText, "git_config") {
					t.Errorf("unexpected Move action on git_config: %v", a)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 2, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 1},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: HunkPlus, CommitMeta, FileMeta, and default wildcard map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
		)

		// 3. Target & wrappers: State::HunkHeader stays matched without Move or Delete actions.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 2},
		)

		srcHunkHeader, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 2})
		if err != nil {
			t.Fatalf("find srcHunkHeader: %v", err)
		}
		dstHunkHeader, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "identifier", Line: 6, Nth: 2})
		if err != nil {
			t.Fatalf("find dstHunkHeader: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Node != nil && (a.Node == srcHunkHeader || a.Node == dstHunkHeader) {
				t.Errorf("unexpected action on HunkHeader: %v", a)
			}
		}

		// 4. Target & insertions: tuple_struct_pattern and SubmoduleLog are inserted.
		var foundTupleStructInsert, foundSubmoduleLogInsert bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Insert && a.Node != nil {
				if r.IsWrapper(a.Node.Type) && a.Node.Type == "tuple_struct_pattern" {
					foundTupleStructInsert = true
				}
				if a.Node.Type == "match_arm" && strings.Contains(string(dst[a.Node.StartByte:a.Node.EndByte]), "SubmoduleLog") {
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "field_identifier", Line: 6},
			testutil.NodeQuery{Type: "field_identifier", Line: 6},
		)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_definition", Line: 1},
			testutil.NodeQuery{Type: "function_definition", Line: 1},
		)

		// 2. Surrounding sequence: HUNK_PLUS, COMMIT_META, FILE_META map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 7, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 7, Nth: 2},
		)

		// 3. Target & wrappers: State.HUNK_HEADER identifier stays matched without Move or Delete actions.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 2},
		)

		srcHunkHeader, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 2})
		if err != nil {
			t.Fatalf("find srcHunkHeader: %v", err)
		}
		dstHunkHeader, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "identifier", Line: 9, Nth: 2})
		if err != nil {
			t.Fatalf("find dstHunkHeader: %v", err)
		}

		for _, a := range normalized.Actions() {
			if a.Node != nil && (a.Node == srcHunkHeader || a.Node == dstHunkHeader) {
				t.Errorf("unexpected action on HUNK_HEADER: %v", a)
			}
		}

		// 4. Target & insertions: class_pattern and SUBMODULE_LOG are inserted.
		var foundClassPatternInsert, foundSubmoduleLogInsert bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Insert && a.Node != nil {
				if pyRules.IsWrapper(a.Node.Type) && a.Node.Type == "class_pattern" {
					foundClassPatternInsert = true
				}
				if a.Node.Type == "case_clause" && strings.Contains(string(dst[a.Node.StartByte:a.Node.EndByte]), "SUBMODULE_LOG") {
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 2},
			testutil.NodeQuery{Type: "identifier", Line: 10, Nth: 2},
		)
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
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: tc.fnType, Line: tc.srcFnLine},
				testutil.NodeQuery{Type: tc.fnType, Line: tc.dstFnLine},
			)

			// 2. Surrounding sequence: calculate_width call maps in place.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: tc.srcCallLine},
				testutil.NodeQuery{Type: "call_expression", Line: tc.dstCallLine},
			)

			// 3. Target Statement: alignment_spec statement maps in place across versions.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: tc.stmtType, Line: tc.srcStmtLine},
				testutil.NodeQuery{Type: tc.stmtType, Line: tc.dstStmtLine},
			)

			// 4. Stationary Operands: placeholder and alignment_spec must NOT be deleted or inserted.
			for _, a := range normalized.Actions() {
				if a.Node == nil {
					continue
				}
				var text string
				switch a.Type {
				case actions.Delete:
					text = string(tc.src[a.Node.StartByte:a.Node.EndByte])
				case actions.Insert:
					text = string(tc.dst[a.Node.StartByte:a.Node.EndByte])
				default:
					continue
				}
				if text == "placeholder" || text == "alignment_spec" || text == "placeholder.alignment_spec" {
					t.Errorf("spurious %s on stationary payload %q (action: %v)", a.Type, text, a)
				}
			}

			// 5. Stationary Operands remain mapped in Mappings.
			testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
				testutil.NodeQuery{Type: tc.exprType, Line: tc.srcExprLine, Nth: 2},
				testutil.NodeQuery{Type: tc.exprType, Line: tc.dstExprLine, Nth: 3},
			)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: call arguments minus_line_index, line_numbers_data, and config map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "arguments", Line: 3, Nth: 2},
			testutil.NodeQuery{Type: "arguments", Line: 3, Nth: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4},
			testutil.NodeQuery{Type: "identifier", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 11},
			testutil.NodeQuery{Type: "identifier", Line: 11},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 18},
			testutil.NodeQuery{Type: "identifier", Line: 13},
		)

		// 3. Target: background_color_extends_to_terminal_width maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 17},
			testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 1},
		)

		srcTarget, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "identifier", Line: 17})
		if err != nil {
			t.Fatalf("find srcTarget: %v", err)
		}
		dstTarget, err := testutil.FindNode(dstAST, testutil.NodeQuery{Type: "identifier", Line: 12, Nth: 1})
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 1},
				testutil.NodeQuery{Type: "function_item", Line: 1},
			)

			// 2. Surrounding sequence: prefix declaration and output_buffer call map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "let_declaration", Line: 2},
				testutil.NodeQuery{Type: "let_declaration", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 8},
				testutil.NodeQuery{Type: "call_expression", Line: 10},
			)

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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
				testutil.NodeQuery{Type: "function_declaration", Line: 3},
			)

			// 2. Surrounding sequence: msg assignment and finalize call map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
				testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 10},
				testutil.NodeQuery{Type: "call_expression", Line: 12},
			)

			// 3. Target: surviving else if code == 3 branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Go: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 8},
				testutil.NodeQuery{Type: "call_expression", Line: 10},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "method_declaration", Line: 2},
				testutil.NodeQuery{Type: "method_declaration", Line: 2},
			)

			// 2. Surrounding sequence: msg local var declaration and finalize call map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
				testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "method_invocation", Line: 9},
				testutil.NodeQuery{Type: "method_invocation", Line: 11},
			)

			// 3. Target: surviving else if (code == 3) branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Java: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "method_invocation", Line: 7},
				testutil.NodeQuery{Type: "method_invocation", Line: 9},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
			)

			// 2. Surrounding sequence: msg variable declaration and finalize call map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
				testutil.NodeQuery{Type: "variable_declaration", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 8},
				testutil.NodeQuery{Type: "call_expression", Line: 10},
			)

			// 3. Target: surviving else if (code == 3) branch must NOT take Move action.
			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on surviving else branch in Zig: %v", a)
				}
			}

			// 4. Stationary Operands: processFinal call maps in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 6},
				testutil.NodeQuery{Type: "call_expression", Line: 8},
			)
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
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_item", Line: 1},
				testutil.NodeQuery{Type: "function_item", Line: 1},
			)

			// 2. Surrounding sequence: config variable and validate call map in place.
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "let_declaration", Line: 2},
				testutil.NodeQuery{Type: "let_declaration", Line: 2},
			)
			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "call_expression", Line: 12},
				testutil.NodeQuery{Type: "call_expression", Line: 10},
			)

			// 3. Edit actions: zero Move actions on the stationary receiver chain.
			script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
			normalized := Run(script, mr.Mappings, srcAST, dstAST)

			for _, a := range normalized.Actions() {
				if a.Type == actions.Move {
					t.Fatalf("unexpected Move action on fluent receiver chain: %v (Node=%s [%d:%d])",
						a, a.Node.Type, a.Node.StartRow, a.Node.EndRow)
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

			testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
				testutil.NodeQuery{Type: "function_declaration", Line: 1},
			)

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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: Some(1) arm and Some(2) arm map in place without disruption.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 4},
			testutil.NodeQuery{Type: "match_arm", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "match_arm", Line: 7},
			testutil.NodeQuery{Type: "match_arm", Line: 6},
		)

		// 3. Target: token_tree and operands within the macro invocation must NOT take Move actions.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				t.Fatalf("unexpected Move action on stationary token_tree: %v", a)
			}
		}

		// 4. Stationary Operands: string literal maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "string_literal", Line: 9},
			testutil.NodeQuery{Type: "string_literal", Line: 8},
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: prefix declaration and output_buffer push call map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5},
			testutil.NodeQuery{Type: "call_expression", Line: 6},
		)

		// 3. Target & Containers: calls to format_commit_line_captures and set_options map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3},
			testutil.NodeQuery{Type: "call_expression", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 5},
		)

		// 4. Stationary Operands: repo and git_config identifiers map in place and have NO Move or Delete actions.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3},
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 5},
			testutil.NodeQuery{Type: "identifier", Line: 5, Nth: 5},
		)
		srcRepo, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "identifier", Line: 3, Nth: 3})
		if err != nil {
			t.Fatalf("failed to locate srcRepo: %v", err)
		}
		srcGitConfig, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "identifier", Line: 4, Nth: 5})
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_expression", Line: 3},
			testutil.NodeQuery{Type: "if_expression", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 6},
			testutil.NodeQuery{Type: "let_declaration", Line: 6},
		)

		// 3. Target: Syntax type identifier stays stationary with no move or delete actions.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "type_identifier", Line: 1},
			testutil.NodeQuery{Type: "type_identifier", Line: 1},
		)
		srcSyntax, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "type_identifier", Line: 1})
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
			testutil.NodeQuery{Type: "function_declaration", Line: 1},
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
			testutil.NodeQuery{Type: "lexical_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 3},
			testutil.NodeQuery{Type: "if_statement", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "lexical_declaration", Line: 6},
			testutil.NodeQuery{Type: "lexical_declaration", Line: 6},
		)

		// 3. Target: Syntax type identifier maps in place and has NO Move or Delete action.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "type_identifier", Line: 1, Nth: 1},
			testutil.NodeQuery{Type: "type_identifier", Line: 1, Nth: 3},
		)
		srcSyntax, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "type_identifier", Line: 1, Nth: 1})
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
		)

		// 2. Surrounding sequence: initial variable, if condition, and end variable map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_statement", Line: 4},
			testutil.NodeQuery{Type: "if_statement", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 7},
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 7},
		)

		// 3. Target: Syntax type identifier maps in place and has NO Move or Delete action.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "type_identifier", Line: 2, Nth: 2},
			testutil.NodeQuery{Type: "type_identifier", Line: 2, Nth: 4},
		)
		srcSyntax, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "type_identifier", Line: 2, Nth: 2})
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: preceding and following statements map in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
			testutil.NodeQuery{Type: "let_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 8},
		)

		// 3. Target call compact_gaps maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
			testutil.NodeQuery{Type: "call_expression", Line: 3, Nth: 1},
		)

		// 4. Stationary operand: &matched_lines must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				nodeText := string(src[a.Node.StartByte:a.Node.EndByte])
				t.Errorf("expected no Move actions for stationary argument, got Move on %s: %q", a.Node.Type, nodeText)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
			testutil.NodeQuery{Type: "function_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
			testutil.NodeQuery{Type: "short_var_declaration", Line: 4},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 10},
		)

		// 2. Target call compactGaps maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 1},
			testutil.NodeQuery{Type: "call_expression", Line: 5, Nth: 1},
		)

		// 3. Stationary operand: &matched must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				nodeText := string(src[a.Node.StartByte:a.Node.EndByte])
				t.Errorf("unexpected Move action on stationary Go argument, got Move on %s: %q", a.Node.Type, nodeText)
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
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
			testutil.NodeQuery{Type: "class_declaration", Line: 1},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
			testutil.NodeQuery{Type: "method_declaration", Line: 2},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
			testutil.NodeQuery{Type: "local_variable_declaration", Line: 3},
		)
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 5},
			testutil.NodeQuery{Type: "method_invocation", Line: 9},
		)

		// 2. Target call compactGaps maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, matchResult.Mappings.Src(),
			testutil.NodeQuery{Type: "method_invocation", Line: 4, Nth: 1},
			testutil.NodeQuery{Type: "method_invocation", Line: 4, Nth: 1},
		)

		// 3. Stationary operand: data.matched must NOT be emitted as a Move.
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move {
				nodeText := string(src[a.Node.StartByte:a.Node.EndByte])
				t.Errorf("unexpected Move action on stationary Java field_access argument, got Move on %s: %q", a.Node.Type, nodeText)
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 5},
		)

		// 2. Surrounding sequence: set_parent and set_num_ancestors map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 2},
			testutil.NodeQuery{Type: "call_expression", Line: 6},
		)
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 3},
			testutil.NodeQuery{Type: "call_expression", Line: 7},
		)

		// 3. Target: set_unique_id call expression maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 4},
			testutil.NodeQuery{Type: "call_expression", Line: 8},
		)
		srcUniqueCall, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "call_expression", Line: 4})
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
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "function_item", Line: 1},
			testutil.NodeQuery{Type: "function_item", Line: 1},
		)

		// 2. Surrounding sequence: if statement maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_expression", Line: 2},
			testutil.NodeQuery{Type: "if_expression", Line: 2},
		)

		// 3. Target: deduped_lines.push call expression maps in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "call_expression", Line: 6},
			testutil.NodeQuery{Type: "call_expression", Line: 6},
		)
		srcPush, err := testutil.FindNode(srcAST, testutil.NodeQuery{Type: "call_expression", Line: 6})
		if err != nil {
			t.Fatalf("failed to locate srcPush: %v", err)
		}

		// 4. Stationary Operands: argument sub-expressions map in place.
		testutil.AssertMapped(t, srcAST, dstAST, mr.Mappings.Src(),
			testutil.NodeQuery{Type: "if_expression", Line: 7},
			testutil.NodeQuery{Type: "if_expression", Line: 7},
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
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "dedup_hunks") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "dedup_hunks") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected function dedup_hunks to map in place")
		}

		// 2. Surrounding sequence: if statement maps in place.
		var srcIf, dstIf *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "lhs_is_dupe && rhs_is_dupe") {
				srcIf = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsStatement(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "lhs_is_dupe && rhs_is_dupe") {
				dstIf = n
				break
			}
		}
		if srcIf == nil || dstIf == nil || mr.Mappings.Src()[srcIf] != dstIf {
			t.Fatalf("expected if statement to map in place")
		}

		// 3. Target: deduped_lines.push call expression maps in place.
		var srcPush, dstPush *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "deduped_lines.push") {
				srcPush = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "deduped_lines.push") {
				dstPush = n
				break
			}
		}
		if srcPush == nil || dstPush == nil {
			t.Fatalf("expected deduped_lines.push calls to exist")
		}
		if mr.Mappings.Src()[srcPush] != dstPush {
			t.Fatalf("expected deduped_lines.push to map in place, got %v", mr.Mappings.Src()[srcPush])
		}

		// 4. Stationary Operands: argument sub-expressions map in place.
		var srcLhsArg, dstLhsArg *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if strings.Contains(string(src[n.StartByte:n.EndByte]), "lhs_is_dupe { None }") && r.IsExpression(n.Type) {
				srcLhsArg = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if strings.Contains(string(dst[n.StartByte:n.EndByte]), "lhs_is_dupe { None }") && r.IsExpression(n.Type) {
				dstLhsArg = n
				break
			}
		}
		if srcLhsArg != nil && dstLhsArg != nil && mr.Mappings.Src()[srcLhsArg] != dstLhsArg {
			t.Fatalf("expected lhs argument if-expr to map in place")
		}

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
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "handle") {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsDeclaration(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "handle") {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil || mr.Mappings.Src()[srcFn] != dstFn {
			t.Fatalf("expected function handle to map in place")
		}

		// 2. Surrounding sequence: prepare() and finish() calls map in place.
		var srcPrep, dstPrep, srcFin, dstFin *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) {
				text := string(src[n.StartByte:n.EndByte])
				if strings.Contains(text, "prepare") {
					srcPrep = n
				} else if strings.Contains(text, "finish") {
					srcFin = n
				}
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) {
				text := string(dst[n.StartByte:n.EndByte])
				if strings.Contains(text, "prepare") {
					dstPrep = n
				} else if strings.Contains(text, "finish") {
					dstFin = n
				}
			}
		}
		if srcPrep == nil || dstPrep == nil || mr.Mappings.Src()[srcPrep] != dstPrep {
			t.Fatalf("expected prepare call to map in place")
		}
		if srcFin == nil || dstFin == nil || mr.Mappings.Src()[srcFin] != dstFin {
			t.Fatalf("expected finish call to map in place")
		}

		// 3. Edit actions: processTask statement moved across branches (consequence -> alternative),
		// so it MUST emit a Move action and NOT be treated as stationary.
		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		var foundMove bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				text := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(text, "processTask") {
					foundMove = true
					break
				}
			}
		}
		if !foundMove {
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

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		var foundMove bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				text := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(text, "process_task") {
					foundMove = true
					break
				}
			}
		}
		if !foundMove {
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

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		var foundMove bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				text := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(text, "processTask") {
					foundMove = true
					break
				}
			}
		}
		if !foundMove {
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

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		var foundMove bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				text := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(text, "process_task") {
					foundMove = true
					break
				}
			}
		}
		if !foundMove {
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

		script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
		normalized := Run(script, mr.Mappings, srcAST, dstAST)

		var foundMove bool
		for _, a := range normalized.Actions() {
			if a.Type == actions.Move && a.Node != nil {
				text := string(src[a.Node.StartByte:a.Node.EndByte])
				if strings.Contains(text, "processTask") {
					foundMove = true
					break
				}
			}
		}
		if !foundMove {
			t.Fatalf("expected Move action for processTask moving across branches in TypeScript, got actions: %v", normalized.Actions())
		}
	})
}
