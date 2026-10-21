package postprocess

import (
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
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

	t.Run("FullEngine", func(t *testing.T) {
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

		// 3. Whole-snippet verification: cleanup closure and stop() statement map in place.
		var srcStop, dstStop *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(src[n.StartByte:n.EndByte]), "stop") {
				srcStop = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if r.IsCall(n.Type) && strings.Contains(string(dst[n.StartByte:n.EndByte]), "stop") {
				dstStop = n
				break
			}
		}
		if srcStop == nil || dstStop == nil {
			t.Fatal("expected to find stop() call expressions in src and dst")
		}
		if matchResult.Mappings.Get(srcStop) != dstStop {
			t.Errorf("expected stationary stop() to map in place, got %v", matchResult.Mappings.Get(srcStop))
		}

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
	t.Run("Unit", func(t *testing.T) {
		// Moving into a container whose mapped source never held this node can't be
		// stationary, even when row/column coordinates match.
		oldStmt := mkNode("var_declaration", "")
		oldStmt.Language = "go"
		oldContA := mkNode("literal_value", "")
		oldContA.Language = "go"
		oldElem := mkNode("keyed_element", "RightLine: 1")
		oldElem.Language = "go"
		oldElem.StartRow, oldElem.EndRow = 5, 5
		oldElem.StartCol, oldElem.EndCol = 2, 14
		oldElem.EndByte = 12

		oldStmt.Children = []*treesitter.ASTNode{oldContA}
		oldContA.Parent = oldStmt
		oldContA.Children = []*treesitter.ASTNode{oldElem}
		oldElem.Parent = oldContA

		oldContB := mkNode("literal_value", "")
		oldContB.Language = "go"

		newStmt := mkNode("var_declaration", "")
		newStmt.Language = "go"
		newContB := mkNode("literal_value", "")
		newContB.Language = "go"
		newElem := mkNode("keyed_element", "RightLine: 1")
		newElem.Language = "go"
		newElem.StartRow, newElem.EndRow = 5, 5
		newElem.StartCol, newElem.EndCol = 2, 14
		newElem.EndByte = 12

		newStmt.Children = []*treesitter.ASTNode{newContB}
		newContB.Parent = newStmt
		newContB.Children = []*treesitter.ASTNode{newElem}
		newElem.Parent = newContB

		ms := engine.NewMapping()
		ms.Add(oldStmt, newStmt)
		ms.Add(oldContB, newContB)
		ms.Add(oldElem, newElem)

		r := rules.Get("go")
		// 1. isStationaryMove returns false even when coordinates collide.
		if isStationaryMove(oldElem, newElem, ms, r) {
			t.Errorf("expected isStationaryMove to return false for cross-container move into foreign container")
		}

		// 2. requiredMoveThreshold returns 20 when moved across containers within the statement.
		oldElem.StartRow, oldElem.EndRow = 5, 5
		newElem.StartRow, newElem.EndRow = 3, 3
		threshold := requiredMoveThreshold(oldElem, newElem, ms, r)
		if threshold != 20 {
			t.Errorf("expected requiredMoveThreshold 20 for cross-container move within statement, got %d", threshold)
		}
	})

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

		// 4. Whole-snippet verification: untouched second element {LeftLine: 4, RightLine: -1} maps in place
		var srcUntouchedElem, dstUntouchedElem *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "literal_value" && string(src[n.StartByte:n.EndByte]) == "{LeftLine: 4, RightLine: -1}" {
				srcUntouchedElem = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "literal_value" && string(dst[n.StartByte:n.EndByte]) == "{LeftLine: 4, RightLine: -1}" {
				dstUntouchedElem = n
				break
			}
		}
		if srcUntouchedElem == nil || dstUntouchedElem == nil {
			t.Fatalf("missing untouched literal_value: src=%v, dst=%v", srcUntouchedElem, dstUntouchedElem)
		}
		if matchResult.Mappings.Src()[srcUntouchedElem] != dstUntouchedElem {
			t.Errorf("expected untouched element {LeftLine: 4, RightLine: -1} to map in place, got %v", matchResult.Mappings.Src()[srcUntouchedElem])
		}
	})
}

func TestNormalizeMoves_PreservesSubExpressionMoveWhenParentIsNotContainer(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		// Non-container parents (like binary expressions or calls) shouldn't trigger
		// the T=20 container penalty when an expression is unnested or re-parented.
		oldBlock := mkNode("block", "")
		oldBlock.Language = "go"
		oldStmt := mkNode("expression_statement", "")
		oldStmt.Language = "go"
		oldStmt.StartRow, oldStmt.EndRow = 3, 3
		oldOp := mkNode("binary_expression", "")
		oldOp.Language = "go"
		oldExpr := mkNode("call_expression", "make(...)")
		oldExpr.Language = "go"
		oldExpr.StartRow, oldExpr.EndRow = 3, 3

		oldBlock.Children = []*treesitter.ASTNode{oldStmt}
		oldStmt.Parent = oldBlock
		oldStmt.Children = []*treesitter.ASTNode{oldOp}
		oldOp.Parent = oldStmt
		oldOp.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldOp

		foreignOp := mkNode("binary_expression", "")
		foreignOp.Language = "go"

		newBlock := mkNode("block", "")
		newBlock.Language = "go"
		newStmt := mkNode("short_var_declaration", "")
		newStmt.Language = "go"
		newStmt.StartRow, newStmt.EndRow = 5, 5
		newOp := mkNode("binary_expression", "")
		newOp.Language = "go"
		newExpr := mkNode("call_expression", "make(...)")
		newExpr.Language = "go"
		newExpr.StartRow, newExpr.EndRow = 5, 5

		newBlock.Children = []*treesitter.ASTNode{newStmt}
		newStmt.Parent = newBlock
		newStmt.Children = []*treesitter.ASTNode{newOp}
		newOp.Parent = newStmt
		newOp.Children = []*treesitter.ASTNode{newExpr}
		newExpr.Parent = newOp

		ms := engine.NewMapping()
		ms.Add(oldStmt, newStmt)
		// Map newOp to a foreign parent that doesn't contain oldExpr.
		ms.Add(foreignOp, newOp)
		ms.Add(oldExpr, newExpr)

		r := rules.Get("go")
		threshold := requiredMoveThreshold(oldExpr, newExpr, ms, r)
		if threshold != 4 {
			t.Errorf("expected requiredMoveThreshold to be 4 for expression move, got %d", threshold)
		}

		// Real containers (like literal_value) should still hit the T=20 penalty.
		oldCont := mkNode("literal_value", "")
		oldCont.Language = "go"
		newCont := mkNode("literal_value", "")
		newCont.Language = "go"
		foreignCont := mkNode("literal_value", "")
		foreignCont.Language = "go"

		oldStmt.Children = []*treesitter.ASTNode{oldCont}
		oldCont.Parent = oldStmt
		oldCont.Children = []*treesitter.ASTNode{oldExpr}
		oldExpr.Parent = oldCont

		newStmt.Children = []*treesitter.ASTNode{newCont}
		newCont.Parent = newStmt
		newCont.Children = []*treesitter.ASTNode{newExpr}
		newExpr.Parent = newCont

		msCont := engine.NewMapping()
		msCont.Add(oldStmt, newStmt)
		msCont.Add(foreignCont, newCont)
		msCont.Add(oldExpr, newExpr)

		contThreshold := requiredMoveThreshold(oldExpr, newExpr, msCont, r)
		if contThreshold != 20 {
			t.Errorf("expected requiredMoveThreshold to return 20 for container parent, got %d", contThreshold)
		}
	})

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
		var srcFn, dstFn *treesitter.ASTNode
		for _, n := range srcAST.Descendants() {
			if n.Type == "function_declaration" {
				srcFn = n
				break
			}
		}
		for _, n := range dstAST.Descendants() {
			if n.Type == "function_declaration" {
				dstFn = n
				break
			}
		}
		if srcFn == nil || dstFn == nil {
			t.Fatalf("missing function_declaration: src=%v, dst=%v", srcFn, dstFn)
		}
		if matchResult.Mappings.Src()[srcFn] != dstFn {
			t.Errorf("expected function_declaration to map in place, got %v", matchResult.Mappings.Src()[srcFn])
		}
	})
}
