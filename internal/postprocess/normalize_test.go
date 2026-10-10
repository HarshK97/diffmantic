package postprocess

import (
	"fmt"
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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

func TestIsTerminatingStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("return statement is terminating", func(t *testing.T) {
		stmt := mkNode("return_statement", "")
		stmt.Language = "go"
		if !isTerminatingStatement(stmt, r) {
			t.Error("expected return_statement to be terminating")
		}
	})

	t.Run("os.Exit call is terminating", func(t *testing.T) {
		operand := mkNode("identifier", "os")
		operand.Language = "go"
		field := mkNode("field_identifier", "Exit")
		field.Language = "go"
		selector := mkNode("selector_expression", "")
		selector.Language = "go"
		selector.Children = []*treesitter.ASTNode{operand, field}
		operand.Parent = selector
		field.Parent = selector

		arg := mkNode("interpreted_string_literal", "1")
		arg.Language = "go"
		args := mkNode("argument_list", "")
		args.Language = "go"
		args.Children = []*treesitter.ASTNode{arg}
		arg.Parent = args

		call := mkNode("call_expression", "")
		call.Language = "go"
		call.Children = []*treesitter.ASTNode{selector, args}
		selector.Parent = call
		args.Parent = call

		exprStmt := mkNode("expression_statement", "")
		exprStmt.Language = "go"
		exprStmt.Children = []*treesitter.ASTNode{call}
		call.Parent = exprStmt

		if !isTerminatingStatement(exprStmt, r) {
			t.Error("expected os.Exit call to be terminating")
		}
	})

	t.Run("regular expression is not terminating", func(t *testing.T) {
		stmt := mkNode("expression_statement", "")
		stmt.Language = "go"
		call := mkNode("call_expression", "")
		call.Language = "go"
		operand := mkNode("identifier", "fmt")
		operand.Language = "go"
		field := mkNode("field_identifier", "Println")
		field.Language = "go"
		selector := mkNode("selector_expression", "")
		selector.Language = "go"
		selector.Children = []*treesitter.ASTNode{operand, field}
		operand.Parent = selector
		field.Parent = selector
		call.Children = []*treesitter.ASTNode{selector}
		selector.Parent = call
		stmt.Children = []*treesitter.ASTNode{call}
		call.Parent = stmt

		if isTerminatingStatement(stmt, r) {
			t.Error("expected fmt.Println call to NOT be terminating")
		}
	})
}

func TestIsTrivialJumpBody(t *testing.T) {
	r := rules.Get("go")

	t.Run("body of only jump statements is trivial", func(t *testing.T) {
		openBrace := mkNode("{", "{")
		retStmt := mkNode("return_statement", "")
		closeBrace := mkNode("}", "}")
		body := mkNode("block", "", openBrace, retStmt, closeBrace)
		setLanguageRecursive(body, "go")

		if !isTrivialJumpBody(body, r) {
			t.Error("expected body of only a return statement to be trivial")
		}
	})

	// An error print followed by an exit is still boilerplate - the print
	// shouldn't keep it from being scored as a trivial exit block.
	t.Run("body pairing an error print with a terminating call is trivial", func(t *testing.T) {
		openBrace := mkNode("{", "{")
		fprintfSel := mkNode("selector_expression", "",
			mkNode("identifier", "fmt"),
			mkNode("field_identifier", "Fprintf"),
		)
		printCall := mkNode("call_expression", "", fprintfSel, mkNode("argument_list", ""))
		printStmt := mkNode("expression_statement", "", printCall)

		exitSel := mkNode("selector_expression", "",
			mkNode("identifier", "os"),
			mkNode("field_identifier", "Exit"),
		)
		exitCall := mkNode("call_expression", "", exitSel, mkNode("argument_list", ""))
		exitStmt := mkNode("expression_statement", "", exitCall)
		closeBrace := mkNode("}", "}")

		body := mkNode("block", "", openBrace, printStmt, exitStmt, closeBrace)
		setLanguageRecursive(body, "go")

		if !isTrivialJumpBody(body, r) {
			t.Error("expected print+os.Exit body to be trivial boilerplate")
		}
	})
}

func setLanguageRecursive(n *treesitter.ASTNode, lang string) {
	if n == nil {
		return
	}
	n.Language = lang
	for _, c := range n.Children {
		setLanguageRecursive(c, lang)
	}
}

func TestMoveStructuralScore(t *testing.T) {
	r := rules.Get("go")
	ms := engine.NewMapping()

	t.Run("bare token clamped to 1", func(t *testing.T) {
		op := mkNode("arithmetic_operator_literal", "+")
		op.Language = "go"
		score := moveStructuralScore(op, r, ms)
		if score != 1 {
			t.Errorf("expected token score 1, got %d", score)
		}
	})

	t.Run("small if block with trivial body", func(t *testing.T) {
		retStmt := mkNode("return_statement", "")
		retStmt.Language = "go"
		body := mkNode("block", "", retStmt)
		body.Language = "go"
		retStmt.Parent = body
		ifStmt := mkNode("if_statement", "")
		ifStmt.Language = "go"
		cond := mkNode("identifier", "err")
		cond.Language = "go"
		ifStmt.Children = []*treesitter.ASTNode{cond, body}
		cond.Parent = ifStmt
		body.Parent = ifStmt

		score := moveStructuralScore(ifStmt, r, ms)
		// Size ~4, height ~2, lines 0, boilerplate -20 → clamp to 1
		if score < 1 {
			t.Errorf("expected score >= 1, got %d", score)
		}
	})

	t.Run("declaration gets +40 bonus", func(t *testing.T) {
		decl := mkNode("function_declaration", "foo")
		decl.Language = "go"
		decl.Children = []*treesitter.ASTNode{mkNode("block", "")}
		decl.Children[0].Parent = decl

		score := moveStructuralScore(decl, r, ms)
		if score < 40 {
			t.Errorf("expected declaration score >= 40, got %d", score)
		}
	})

	t.Run("container with deleted contents discounts size by surviving mass", func(t *testing.T) {
		id1 := mkNode("identifier", "a")
		id2 := mkNode("identifier", "b")
		id3 := mkNode("identifier", "c")
		stmt1 := mkNode("expression_statement", "", id1)
		stmt2 := mkNode("expression_statement", "", id2)
		stmt3 := mkNode("expression_statement", "", id3)
		block := mkNode("block", "", stmt1, stmt2, stmt3)
		setLanguageRecursive(block, "go")

		emptyMs := engine.NewMapping()
		scoreEmpty := moveStructuralScore(block, r, emptyMs)

		partialMs := engine.NewMapping()
		partner := mkNode("identifier", "a")
		partner.Language = "go"
		partialMs.Add(id1, partner)
		scorePartial := moveStructuralScore(block, r, partialMs)

		if scorePartial <= scoreEmpty {
			t.Errorf("expected score with surviving child (%d) > empty score (%d)", scorePartial, scoreEmpty)
		}

		fullMs := engine.NewMapping()
		fullMs.Add(id1, partner)
		fullMs.Add(id2, partner)
		fullMs.Add(id3, partner)
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
		parent := mkNode("argument_list", "")
		parent.Language = "go"
		src := mkNode("identifier", "a")
		src.Language = "go"
		src.Parent = parent
		dst := mkNode("identifier", "b")
		dst.Language = "go"
		dst.Parent = parent
		parent.Children = []*treesitter.ASTNode{src, dst}

		ms.Add(parent, parent)

		threshold := requiredMoveThreshold(src, dst, ms, r)
		if threshold != 1 {
			t.Errorf("expected threshold 1 for intra-container reorder, got %d", threshold)
		}
	})

	t.Run("top-level declaration returns 20", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.StartRow = 10
		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.StartRow = 500

		threshold := requiredMoveThreshold(srcFunc, dstFunc, ms, r)
		if threshold != 20 {
			t.Errorf("expected threshold 20 for top-level declaration, got %d", threshold)
		}
	})

	// Don't let a bare literal move between different struct fields just
	// because they sit on the same line (e.g. `defValue: ""` vs `shorthand: ""`).
	t.Run("cross-key bare literal at same line requires threshold >= 5", func(t *testing.T) {
		srcVal := mkNode("string_literal", "\"\"")
		srcVal.Language = "go"
		srcPair := mkNode("keyed_element", "", mkNode("field_identifier", "defValue"), srcVal)
		srcPair.Language = "go"
		srcVal.Parent = srcPair

		dstVal := mkNode("string_literal", "\"\"")
		dstVal.Language = "go"
		dstPair := mkNode("keyed_element", "", mkNode("field_identifier", "shorthand"), dstVal)
		dstPair.Language = "go"
		dstVal.Parent = dstPair

		// Same line (ΔL = 0); srcPair is intentionally left unmapped to dstPair.
		srcVal.StartRow, srcVal.EndRow = 5, 5
		dstVal.StartRow, dstVal.EndRow = 5, 5

		threshold := requiredMoveThreshold(srcVal, dstVal, ms, r)
		if threshold < 5 {
			t.Errorf("expected threshold >= 5 for cross-key bare literal, got %d (a clamped S=1 token would survive as a Move)", threshold)
		}
	})

	t.Run("same-line inline shift returns 1", func(t *testing.T) {
		ms := engine.NewMapping()
		srcStmt := mkNode("expression_statement", "foo()")
		srcStmt.Language = "go"
		srcStmt.StartRow, srcStmt.EndRow = 10, 10
		dstStmt := mkNode("expression_statement", "foo()")
		dstStmt.Language = "go"
		dstStmt.StartRow, dstStmt.EndRow = 10, 10

		threshold := requiredMoveThreshold(srcStmt, dstStmt, ms, r)
		if threshold != 1 {
			t.Errorf("requiredMoveThreshold() = %d, want 1", threshold)
		}
	})

	t.Run("distant statement across scopes does not get same-line threshold of 1", func(t *testing.T) {
		// A surviving outer scope shouldn't cancel out line distance between distant functions.
		rootSrc := mkNode("source_file", "")
		rootSrc.Language = "go"
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.Parent = rootSrc
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcIf := mkNode("if_statement", "")
		srcIf.Language = "go"
		srcIf.StartRow = 10
		srcIf.Parent = srcBody

		rootDst := mkNode("source_file", "")
		rootDst.Language = "go"
		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.Parent = rootDst
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstIf := mkNode("if_statement", "")
		dstIf.Language = "go"
		dstIf.StartRow = 210
		dstIf.Parent = dstBody

		msDrift := engine.NewMapping()
		msDrift.Add(rootSrc, rootDst)

		threshold := requiredMoveThreshold(srcIf, dstIf, msDrift, r)
		if threshold <= 1 {
			t.Errorf("expected threshold > 1 for distant statement 200 lines away, got %d", threshold)
		}
		// 200 lines apart gets the cross-scope penalty: 50 + 200/10 = 70.
		if threshold < 50 {
			t.Errorf("expected threshold >= 50 for 200-line distant statement across scopes, got %d", threshold)
		}
	})
}

func TestNormalizeMovesByStructure(t *testing.T) {
	ms := engine.NewMapping()

	t.Run("demotes boilerplate if block across distant scope", func(t *testing.T) {
		retStmt := mkNode("return_statement", "")
		retStmt.Language = "go"
		srcBody := mkNode("block", "", retStmt)
		srcBody.Language = "go"
		retStmt.Parent = srcBody
		srcIf := mkNode("if_statement", "")
		srcIf.Language = "go"
		srcCond := mkNode("identifier", "err")
		srcCond.Language = "go"
		srcIf.Children = []*treesitter.ASTNode{srcCond, srcBody}
		srcCond.Parent = srcIf
		srcBody.Parent = srcIf
		srcIf.StartRow = 10

		retStmt2 := mkNode("return_statement", "")
		retStmt2.Language = "go"
		dstBody := mkNode("block", "", retStmt2)
		dstBody.Language = "go"
		retStmt2.Parent = dstBody
		dstIf := mkNode("if_statement", "")
		dstIf.Language = "go"
		dstCond := mkNode("identifier", "err")
		dstCond.Language = "go"
		dstIf.Children = []*treesitter.ASTNode{dstCond, dstBody}
		dstCond.Parent = dstIf
		dstBody.Parent = dstIf
		dstIf.StartRow = 500

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcIf, DestNode: dstIf})

		result := normalizeMovesByStructure(es, ms)
		// Boilerplate if-block across 490 lines should be demoted to delete+insert
		if result.Size() != 2 {
			t.Fatalf("expected 2 actions (delete+insert) for demoted boilerplate, got %d", result.Size())
		}
		if result.Actions()[0].Type != actions.Delete || result.Actions()[1].Type != actions.Insert {
			t.Errorf("expected delete then insert, got %v and %v", result.Actions()[0].Type, result.Actions()[1].Type)
		}
	})

	t.Run("preserves declaration move", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.StartRow = 10
		srcFunc.Children = []*treesitter.ASTNode{mkNode("block", "")}
		srcFunc.Children[0].Parent = srcFunc

		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.StartRow = 500
		dstFunc.Children = []*treesitter.ASTNode{mkNode("block", "")}
		dstFunc.Children[0].Parent = dstFunc

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcFunc, DestNode: dstFunc})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected declaration move to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("suppresses orphaned update when paired move is demoted", func(t *testing.T) {
		// Chawathe emits Update + Move on the same node when labels differ.
		// When the Move is demoted, the Update must also be suppressed.
		srcVal := mkNode("string", "main")
		srcVal.Language = "javascript"
		srcVal.StartRow = 5

		dstVal := mkNode("string", "2.0.0")
		dstVal.Language = "javascript"
		dstVal.StartRow = 500

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Update, Node: srcVal, DestNode: dstVal})
		es.Add(actions.Action{Type: actions.Move, Node: srcVal, DestNode: dstVal})

		result := normalizeMovesByStructure(es, ms)
		for _, a := range result.Actions() {
			if a.Type == actions.Update {
				t.Error("expected orphaned Update to be suppressed when paired Move is demoted")
			}
		}
		// Should have Delete + Insert from the demoted Move, no Update.
		if result.Size() != 2 {
			t.Errorf("expected 2 actions (delete+insert), got %d", result.Size())
		}
	})

	t.Run("preserves sibling relocation within same parent", func(t *testing.T) {
		parent := mkNode("argument_list", "")
		parent.Language = "go"
		srcArg := mkNode("identifier", "a")
		srcArg.Language = "go"
		srcArg.Parent = parent
		dstArg := mkNode("identifier", "b")
		dstArg.Language = "go"
		dstArg.Parent = parent
		parent.Children = []*treesitter.ASTNode{srcArg, dstArg}

		ms2 := engine.NewMapping()
		ms2.Add(parent, parent)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcArg, DestNode: dstArg})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected sibling relocation to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves one-hop container-preserving reparent", func(t *testing.T) {
		// Value wrapped in a brand-new pair node, but the matched container is intact.
		srcContainer := mkNode("object", "")
		srcContainer.Language = "javascript"
		srcVal := mkNode("string", "hello")
		srcVal.Language = "javascript"
		srcVal.Parent = srcContainer
		srcContainer.Children = []*treesitter.ASTNode{srcVal}

		dstContainer := mkNode("object", "")
		dstContainer.Language = "javascript"
		dstPair := mkNode("pair", "")
		dstPair.Language = "javascript"
		dstPair.Parent = dstContainer
		dstVal := mkNode("string", "hello")
		dstVal.Language = "javascript"
		dstVal.Parent = dstPair
		dstPair.Children = []*treesitter.ASTNode{dstVal}
		dstContainer.Children = []*treesitter.ASTNode{dstPair}

		ms2 := engine.NewMapping()
		ms2.Add(srcContainer, dstContainer)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcVal, DestNode: dstVal})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected one-hop reparent to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("preserves intra-scope move with moderate drift", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcStmt := mkNode("expression_statement", "")
		srcStmt.Language = "go"
		srcStmt.Parent = srcFunc
		srcStmt.StartRow = 10
		srcFunc.Children = []*treesitter.ASTNode{srcStmt}

		dstFunc := mkNode("function_declaration", "foo")
		dstFunc.Language = "go"
		dstStmt := mkNode("expression_statement", "")
		dstStmt.Language = "go"
		dstStmt.Parent = dstFunc
		dstStmt.StartRow = 30
		dstFunc.Children = []*treesitter.ASTNode{dstStmt}

		ms2 := engine.NewMapping()
		ms2.Add(srcFunc, dstFunc)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Errorf("expected intra-scope move with drift=20 to be preserved, got %d actions", result.Size())
		}
	})

	t.Run("demotes cross-scope move with large drift and no mapped src decl", func(t *testing.T) {
		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcStmt := mkNode("expression_statement", "")
		srcStmt.Language = "go"
		srcStmt.Parent = srcBlock
		srcStmt.StartRow = 10
		srcBlock.Children = []*treesitter.ASTNode{srcStmt}

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstStmt := mkNode("expression_statement", "")
		dstStmt.Language = "go"
		dstStmt.Parent = dstBlock
		dstStmt.StartRow = 200
		dstBlock.Children = []*treesitter.ASTNode{dstStmt}

		// No parent mapping: cross-scope with no mapped enclosing decl.
		ms2 := engine.NewMapping()

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcStmt, DestNode: dstStmt})

		result := normalizeMovesByStructure(es, ms2)
		if result.Size() == 1 && result.Actions()[0].Type == actions.Move {
			t.Error("expected cross-scope move with large drift and no mapped decl to be demoted")
		}
	})

	t.Run("demotes small inlined expression across different statements", func(t *testing.T) {
		srcFn := mkNode("function_declaration", "init")
		srcFn.Language = "go"
		dstFn := mkNode("function_declaration", "init")
		dstFn.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = srcFn
		srcFn.Children = []*treesitter.ASTNode{srcBlock}

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = dstFn
		dstFn.Children = []*treesitter.ASTNode{dstBlock}

		srcCall := mkNode("call_expression", "", mkNode("identifier", "len"), mkNode("argument_list", "", mkNode("identifier", "pairs")))
		srcCall.Language = "go"
		srcCall.StartRow = 20
		srcCall.EndRow = 20
		srcStmt := mkNode("assignment_statement", "", mkNode("identifier", "pairCount"), srcCall)
		srcStmt.Language = "go"
		srcStmt.Parent = srcBlock
		srcCall.Parent = srcStmt
		srcBlock.Children = []*treesitter.ASTNode{srcStmt}

		dstCall := mkNode("call_expression", "", mkNode("identifier", "len"), mkNode("argument_list", "", mkNode("identifier", "pairs")))
		dstCall.Language = "go"
		dstCall.StartRow = 25
		dstCall.EndRow = 25
		dstStmt := mkNode("assignment_statement", "", mkNode("identifier", "s"), dstCall)
		dstStmt.Language = "go"
		dstStmt.Parent = dstBlock
		dstCall.Parent = dstStmt
		dstBlock.Children = []*treesitter.ASTNode{dstStmt}

		ms := engine.NewMapping()
		ms.Add(srcFn, dstFn)
		ms.Add(srcCall, dstCall)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcCall, DestNode: dstCall})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 2 {
			t.Fatalf("expected 2 actions (delete+insert) for small inlined expression, got %d", result.Size())
		}
		if result.Actions()[0].Type != actions.Delete || result.Actions()[1].Type != actions.Insert {
			t.Errorf("expected delete then insert, got %v and %v", result.Actions()[0].Type, result.Actions()[1].Type)
		}
	})

	t.Run("preserves substantial inlined call across different statements", func(t *testing.T) {
		srcFn := mkNode("function_declaration", "init")
		srcFn.Language = "go"
		dstFn := mkNode("function_declaration", "init")
		dstFn.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = srcFn
		srcFn.Children = []*treesitter.ASTNode{srcBlock}

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = dstFn
		dstFn.Children = []*treesitter.ASTNode{dstBlock}

		srcArgs := make([]*treesitter.ASTNode, 0, 10)
		dstArgs := make([]*treesitter.ASTNode, 0, 10)
		for range 10 {
			a1 := mkNode("identifier", "arg")
			a1.Language = "go"
			srcArgs = append(srcArgs, a1)
			a2 := mkNode("identifier", "arg")
			a2.Language = "go"
			dstArgs = append(dstArgs, a2)
		}
		srcArgList := mkNode("argument_list", "", srcArgs...)
		srcArgList.Language = "go"
		dstArgList := mkNode("argument_list", "", dstArgs...)
		dstArgList.Language = "go"

		srcCall := mkNode("call_expression", "", mkNode("identifier", "format"), srcArgList)
		srcCall.Language = "go"
		srcCall.StartRow = 20
		srcCall.EndRow = 22

		dstCall := mkNode("call_expression", "", mkNode("identifier", "format"), dstArgList)
		dstCall.Language = "go"
		dstCall.StartRow = 25
		dstCall.EndRow = 27

		srcStmt := mkNode("assignment_statement", "", mkNode("identifier", "v"), srcCall)
		srcStmt.Language = "go"
		srcStmt.Parent = srcBlock
		srcCall.Parent = srcStmt

		dstStmt := mkNode("assignment_statement", "", mkNode("identifier", "res"), dstCall)
		dstStmt.Language = "go"
		dstStmt.Parent = dstBlock
		dstCall.Parent = dstStmt

		ms := engine.NewMapping()
		ms.Add(srcFn, dstFn)
		ms.Add(srcCall, dstCall)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: srcCall, DestNode: dstCall})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 1 || result.Actions()[0].Type != actions.Move {
			t.Fatalf("expected 1 Move action for substantial inlined call, got %d actions", result.Size())
		}
	})

	t.Run("non-move actions pass through unchanged", func(t *testing.T) {
		node := mkNode("identifier", "x")
		node.Language = "go"

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Insert, Node: node})
		es.Add(actions.Action{Type: actions.Delete, Node: node})

		result := normalizeMovesByStructure(es, ms)
		if result.Size() != 2 {
			t.Errorf("expected non-move actions to pass through, got %d", result.Size())
		}
	})

	t.Run("nested move inside demoted ancestor move is suppressed", func(t *testing.T) {
		childSrc := mkNode("identifier", "val")
		childSrc.Language = "go"
		parentSrc := mkNode("expression_statement", "", childSrc)
		parentSrc.Language = "go"
		parentSrc.StartRow = 10
		parentSrc.EndRow = 10

		childDst := mkNode("identifier", "val")
		childDst.Language = "go"
		parentDst := mkNode("expression_statement", "", childDst)
		parentDst.Language = "go"
		parentDst.StartRow = 500
		parentDst.EndRow = 500

		msNested := engine.NewMapping()
		msNested.Add(parentSrc, parentDst)
		msNested.Add(childSrc, childDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})
		es.Add(actions.Action{Type: actions.Move, Node: childSrc, DestNode: childDst})

		result := normalizeMovesByStructure(es, msNested)
		if result.Size() != 2 {
			t.Fatalf("expected exactly 2 actions (Delete+Insert) after ancestor demotion, got %d", result.Size())
		}
		if idx := slices.IndexFunc(result.Actions(), func(a actions.Action) bool {
			return a.Type == actions.Move
		}); idx != -1 {
			a := result.Actions()[idx]
			t.Fatalf("expected no move actions to survive demotion of ancestor move, got %s on %s", a.Type, a.Node.Type)
		}
	})

	t.Run("nested move inside demoted ancestor wrapper is not suppressed and demotes to delete and insert", func(t *testing.T) {
		childSrc := mkNode("call_expression", "r.IsDeclaration(n1.Type)")
		childSrc.Language = "go"
		childSrc.StartRow = 10
		childSrc.EndRow = 10
		binSrc := mkNode("binary_expression", "", childSrc)
		binSrc.Language = "go"
		binSrc.StartRow = 10
		binSrc.EndRow = 10
		childSrc.Parent = binSrc
		parentSrc := mkNode("parenthesized_expression", "", binSrc)
		parentSrc.Language = "go"
		parentSrc.StartRow = 10
		parentSrc.EndRow = 10
		binSrc.Parent = parentSrc

		childDst := mkNode("call_expression", "r.IsDeclaration(n1.Type)")
		childDst.Language = "go"
		childDst.StartRow = 500
		childDst.EndRow = 500
		binDst := mkNode("binary_expression", "", childDst)
		binDst.Language = "go"
		binDst.StartRow = 500
		binDst.EndRow = 500
		childDst.Parent = binDst
		parentDst := mkNode("parenthesized_expression", "", binDst)
		parentDst.Language = "go"
		parentDst.StartRow = 500
		parentDst.EndRow = 500
		binDst.Parent = parentDst

		msNested := engine.NewMapping()
		msNested.Add(parentSrc, parentDst)
		msNested.Add(childSrc, childDst)

		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})
		es.Add(actions.Action{Type: actions.Move, Node: childSrc, DestNode: childDst})

		result := normalizeMovesByStructure(es, msNested)
		foundChildDelete := false
		foundChildInsert := false
		for _, a := range result.Actions() {
			if a.Node == childSrc && a.Type == actions.Delete {
				foundChildDelete = true
			}
			if a.Node == childDst && a.Type == actions.Insert {
				foundChildInsert = true
			}
		}
		if !foundChildDelete || !foundChildInsert {
			t.Errorf("expected childSrc to be demoted to delete and insert, got actions: %+v", result.Actions())
		}
	})

	t.Run("descendants without own move are evicted and demoted to full subtree delete and insert", func(t *testing.T) {
		childSrc := mkNode("call_expression", "min(v1, v2)")
		childSrc.Language = "go"
		childSrc.StartRow = 110
		childSrc.EndRow = 110
		parentSrc := mkNode("expression_list", "", childSrc)
		parentSrc.Language = "go"
		parentSrc.StartRow = 110
		parentSrc.EndRow = 110
		childSrc.Parent = parentSrc

		childDst := mkNode("call_expression", "min(v1, v2)")
		childDst.Language = "go"
		childDst.StartRow = 109
		childDst.EndRow = 109
		parentDst := mkNode("expression_list", "", childDst)
		parentDst.Language = "go"
		parentDst.StartRow = 109
		parentDst.EndRow = 109
		childDst.Parent = parentDst

		// Enclosing statements on different lines so cross-statement guard demotes move.
		stmtSrc := mkNode("assignment_statement", "", parentSrc)
		stmtSrc.Language = "go"
		stmtSrc.StartRow = 110
		stmtSrc.EndRow = 110
		parentSrc.Parent = stmtSrc

		stmtDst := mkNode("short_var_declaration", "", parentDst)
		stmtDst.Language = "go"
		stmtDst.StartRow = 109
		stmtDst.EndRow = 109
		parentDst.Parent = stmtDst

		declSrc := mkNode("function_declaration", "", stmtSrc)
		declSrc.Language = "go"
		declSrc.StartRow = 100
		declSrc.EndRow = 150
		stmtSrc.Parent = declSrc

		declDst := mkNode("function_declaration", "", stmtDst)
		declDst.Language = "go"
		declDst.StartRow = 100
		declDst.EndRow = 150
		stmtDst.Parent = declDst

		ms := engine.NewMapping()
		ms.Add(declSrc, declDst)
		ms.Add(parentSrc, parentDst)
		ms.Add(childSrc, childDst)

		// Only parentSrc has a Move action in es (childSrc moves implicitly).
		es := actions.NewEditScript()
		es.Add(actions.Action{Type: actions.Move, Node: parentSrc, DestNode: parentDst})

		result := normalizeMovesByStructure(es, ms)

		if ms.Has(childSrc) {
			t.Errorf("expected childSrc to be evicted from mapping, but still present")
		}
		if ms.HasDst(childDst) {
			t.Errorf("expected childDst to be evicted from mapping destination, but still present")
		}

		foundDeleteSubtree := false
		foundInsertSubtree := false
		for _, a := range result.Actions() {
			if a.Node == parentSrc && a.Type == actions.Delete && a.Subtree {
				foundDeleteSubtree = true
			}
			if a.Node == parentDst && a.Type == actions.Insert && a.Subtree {
				foundInsertSubtree = true
			}
			if a.Type == actions.Move {
				t.Errorf("unexpected move action survived: %+v", a)
			}
		}
		if !foundDeleteSubtree || !foundInsertSubtree {
			t.Errorf("expected parentSrc to demote with Subtree: true, got actions: %+v", result.Actions())
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
}

func TestIsFillerCallStatement(t *testing.T) {
	r := rules.Get("go")

	t.Run("bare call is filler", func(t *testing.T) {
		sel := mkNode("selector_expression", "")
		sel.Language = "go"
		sel.Children = []*treesitter.ASTNode{mkNode("identifier", "fmt"), mkNode("field_identifier", "Println")}
		sel.Children[0].Parent = sel
		sel.Children[1].Parent = sel
		call := mkNode("call_expression", "", sel)
		call.Language = "go"
		sel.Parent = call
		if !isFillerCallStatement(call, r) {
			t.Error("expected bare call to be filler")
		}
	})

	t.Run("single-child wrapper around call is filler", func(t *testing.T) {
		sel := mkNode("selector_expression", "")
		sel.Language = "go"
		sel.Children = []*treesitter.ASTNode{mkNode("identifier", "fmt"), mkNode("field_identifier", "Println")}
		sel.Children[0].Parent = sel
		sel.Children[1].Parent = sel
		call := mkNode("call_expression", "", sel)
		call.Language = "go"
		sel.Parent = call
		exprStmt := mkNode("expression_statement", "", call)
		exprStmt.Language = "go"
		call.Parent = exprStmt
		if !isFillerCallStatement(exprStmt, r) {
			t.Error("expected single-child wrapper around call to be filler")
		}
	})

	t.Run("multi-child statement is not filler", func(t *testing.T) {
		ifStmt := mkNode("if_statement", "")
		ifStmt.Language = "go"
		ifStmt.Children = []*treesitter.ASTNode{mkNode("identifier", "err"), mkNode("block", "")}
		ifStmt.Children[0].Parent = ifStmt
		ifStmt.Children[1].Parent = ifStmt
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

func TestShouldDemoteMove(t *testing.T) {
	r := rules.Get("go")

	t.Run("returns false for sibling relocation", func(t *testing.T) {
		parent := mkNode("argument_list", "")
		parent.Language = "go"
		src := mkNode("identifier", "a")
		src.Language = "go"
		src.Parent = parent
		dst := mkNode("identifier", "b")
		dst.Language = "go"
		dst.Parent = parent
		parent.Children = []*treesitter.ASTNode{src, dst}

		ms := engine.NewMapping()
		ms.Add(parent, parent)

		if shouldDemoteMove(src, dst, ms, r) {
			t.Error("expected sibling relocation to not be demoted")
		}
	})

	t.Run("demotes sibling relocation across different enclosing scopes", func(t *testing.T) {
		fn1 := mkNode("function_declaration", "funcA")
		fn1.Language = "go"
		parent1 := mkNode("binary_expression", "")
		parent1.Language = "go"
		parent1.Parent = fn1
		src := mkNode("identifier", "a")
		src.Language = "go"
		src.Parent = parent1
		parent1.Children = []*treesitter.ASTNode{src}

		fn2 := mkNode("function_declaration", "funcB")
		fn2.Language = "go"
		parent2 := mkNode("binary_expression", "")
		parent2.Language = "go"
		parent2.Parent = fn2
		parent2.StartRow = 500
		parent2.EndRow = 500
		dst := mkNode("identifier", "b")
		dst.Language = "go"
		dst.Parent = parent2
		dst.StartRow = 500
		dst.EndRow = 500
		parent2.Children = []*treesitter.ASTNode{dst}

		ms := engine.NewMapping()
		ms.Add(parent1, parent2)

		if !shouldDemoteMove(src, dst, ms, r) {
			t.Error("expected sibling relocation across different enclosing functions to be demoted")
		}
	})

	t.Run("returns false for declaration move", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "foo")
		srcFunc.Language = "go"
		srcFunc.Children = []*treesitter.ASTNode{mkNode("block", "")}
		srcFunc.Children[0].Parent = srcFunc

		dstFunc := mkNode("function_declaration", "bar")
		dstFunc.Language = "go"
		dstFunc.Children = []*treesitter.ASTNode{mkNode("block", "")}
		dstFunc.Children[0].Parent = dstFunc

		ms := engine.NewMapping()

		if shouldDemoteMove(srcFunc, dstFunc, ms, r) {
			t.Error("expected declaration move to not be demoted")
		}
	})

	t.Run("returns true for boilerplate if block across distant scope", func(t *testing.T) {
		retStmt := mkNode("return_statement", "")
		retStmt.Language = "go"
		srcBody := mkNode("block", "", retStmt)
		srcBody.Language = "go"
		retStmt.Parent = srcBody
		srcIf := mkNode("if_statement", "")
		srcIf.Language = "go"
		srcCond := mkNode("identifier", "err")
		srcCond.Language = "go"
		srcIf.Children = []*treesitter.ASTNode{srcCond, srcBody}
		srcCond.Parent = srcIf
		srcBody.Parent = srcIf
		srcIf.StartRow = 10

		retStmt2 := mkNode("return_statement", "")
		retStmt2.Language = "go"
		dstBody := mkNode("block", "", retStmt2)
		dstBody.Language = "go"
		retStmt2.Parent = dstBody
		dstIf := mkNode("if_statement", "")
		dstIf.Language = "go"
		dstCond := mkNode("identifier", "err")
		dstCond.Language = "go"
		dstIf.Children = []*treesitter.ASTNode{dstCond, dstBody}
		dstCond.Parent = dstIf
		dstBody.Parent = dstIf
		dstIf.StartRow = 500

		ms := engine.NewMapping()

		if !shouldDemoteMove(srcIf, dstIf, ms, r) {
			t.Error("expected boilerplate if block across distant scope to be demoted")
		}
	})

	t.Run("returns false for nil arguments", func(t *testing.T) {
		ms := engine.NewMapping()
		node := mkNode("identifier", "x")
		if shouldDemoteMove(nil, node, ms, r) {
			t.Error("expected false for nil src")
		}
		if shouldDemoteMove(node, nil, ms, r) {
			t.Error("expected false for nil dst")
		}
		if shouldDemoteMove(node, node, nil, r) {
			t.Error("expected false for nil mapping")
		}
		if shouldDemoteMove(node, node, ms, nil) {
			t.Error("expected false for nil rules")
		}
	})

	t.Run("returns true for cross-scope move with asymmetric destination mass", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "funcA")
		srcFunc.Language = "go"
		dstFunc := mkNode("function_declaration", "funcB")
		dstFunc.Language = "go"

		srcCall := mkNode("call_expression", "")
		srcCall.Language = "go"
		srcCall.Parent = srcFunc
		srcCall.StartRow = 10
		srcCall.EndRow = 25
		for range 40 {
			c := mkNode("identifier", "arg")
			c.Language = "go"
			c.Parent = srcCall
			srcCall.Children = append(srcCall.Children, c)
		}

		dstCall := mkNode("call_expression", "")
		dstCall.Language = "go"
		dstCall.Parent = dstFunc
		dstCall.StartRow = 350
		dstCall.EndRow = 353
		for range 5 {
			c := mkNode("identifier", "arg")
			c.Language = "go"
			c.Parent = dstCall
			dstCall.Children = append(dstCall.Children, c)
		}

		ms := engine.NewMapping()
		// Even if the deleted call was huge, the destination snippet is too small
		// to justify a cross-scope move.
		if !shouldDemoteMove(srcCall, dstCall, ms, r) {
			t.Error("expected cross-scope move with tiny destination to be demoted via bidirectional scoring")
		}
	})

	t.Run("returns false for cross-scope move with substantial mass on both ends", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "funcA")
		srcFunc.Language = "go"
		dstFunc := mkNode("function_declaration", "funcB")
		dstFunc.Language = "go"

		srcCall := mkNode("call_expression", "")
		srcCall.Language = "go"
		srcCall.Parent = srcFunc
		srcCall.StartRow = 10
		srcCall.EndRow = 40
		for range 80 {
			c := mkNode("identifier", "arg")
			c.Language = "go"
			c.Parent = srcCall
			srcCall.Children = append(srcCall.Children, c)
		}

		dstCall := mkNode("call_expression", "")
		dstCall.Language = "go"
		dstCall.Parent = dstFunc
		dstCall.StartRow = 350
		dstCall.EndRow = 380
		for range 80 {
			c := mkNode("identifier", "arg")
			c.Language = "go"
			c.Parent = dstCall
			dstCall.Children = append(dstCall.Children, c)
		}

		ms := engine.NewMapping()
		// Substantial mass on both sides clears the cross-scope drift penalty.
		if shouldDemoteMove(srcCall, dstCall, ms, r) {
			t.Error("expected cross-scope move with substantial mass on both ends to be preserved")
		}
	})

	t.Run("demotes wrapper/block moves when no child moves with them into destination", func(t *testing.T) {
		// (r == nil && (...)) moving to (r == nil && rules.IsDeclaration(...))
		// where no leaf tokens inside src move to dst.
		srcParen := mkNode("parenthesized_expression", "")
		srcParen.Language = "go"
		srcBin := mkNode("binary_expression", "")
		srcBin.Language = "go"
		srcBin.Parent = srcParen
		srcParen.Children = append(srcParen.Children, srcBin)

		srcOp := mkNode("logical_operator_literal", "&&")
		srcOp.Language = "go"
		srcOp.Parent = srcBin
		srcBin.Children = append(srcBin.Children, srcOp)

		dstParen := mkNode("parenthesized_expression", "")
		dstParen.Language = "go"
		dstBin := mkNode("binary_expression", "")
		dstBin.Language = "go"
		dstBin.Parent = dstParen
		dstParen.Children = append(dstParen.Children, dstBin)

		dstOp := mkNode("logical_operator_literal", "&&")
		dstOp.Language = "go"
		dstOp.Parent = dstBin
		dstBin.Children = append(dstBin.Children, dstOp)

		ms := engine.NewMapping()
		ms.Add(srcParen, dstParen)
		ms.Add(srcBin, dstBin)
		ms.Add(srcOp, dstOp)

		// Only the operator and internal binary_expression match, no leaf expressions/identifiers.
		if !shouldDemoteMove(srcParen, dstParen, ms, r) {
			t.Error("expected empty wrapper move with no substantive child to be demoted")
		}
	})

	t.Run("preserves wrapper move when substantive leaf moves with it into destination", func(t *testing.T) {
		srcParen := mkNode("parenthesized_expression", "")
		srcParen.Language = "go"
		srcIdent := mkNode("identifier", "foo")
		srcIdent.Language = "go"
		srcIdent.Parent = srcParen
		srcParen.Children = append(srcParen.Children, srcIdent)

		dstParen := mkNode("parenthesized_expression", "")
		dstParen.Language = "go"
		dstIdent := mkNode("identifier", "foo")
		dstIdent.Language = "go"
		dstIdent.Parent = dstParen
		dstParen.Children = append(dstParen.Children, dstIdent)

		// Create sibling relocation context (same parent) so it isn't demoted by sibling/threshold
		parent1 := mkNode("call_expression", "")
		parent1.Language = "go"
		parent2 := mkNode("call_expression", "")
		parent2.Language = "go"
		srcParen.Parent = parent1
		dstParen.Parent = parent2

		ms := engine.NewMapping()
		ms.Add(parent1, parent2)
		ms.Add(srcParen, dstParen)
		ms.Add(srcIdent, dstIdent)

		if shouldDemoteMove(srcParen, dstParen, ms, r) {
			t.Error("expected wrapper move with surviving substantive child to be preserved")
		}
	})

	t.Run("demotes cross-scope container move with low retention", func(t *testing.T) {
		fn1 := mkNode("function_declaration", "f1")
		fn1.Language = "go"
		fn2 := mkNode("function_declaration", "f2")
		fn2.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = fn1
		fn1.Children = append(fn1.Children, srcBlock)

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = fn2
		fn2.Children = append(fn2.Children, dstBlock)

		for i := range 20 {
			c := mkNode("identifier", fmt.Sprintf("src%d", i))
			c.Language = "go"
			c.Parent = srcBlock
			srcBlock.Children = append(srcBlock.Children, c)
		}
		for i := range 20 {
			c := mkNode("identifier", fmt.Sprintf("dst%d", i))
			c.Language = "go"
			c.Parent = dstBlock
			dstBlock.Children = append(dstBlock.Children, c)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		// Map only 2 out of 20 leaves (10% retention < 25%).
		ms.Add(srcBlock.Children[0], dstBlock.Children[0])
		ms.Add(srcBlock.Children[1], dstBlock.Children[1])

		if !shouldDemoteMove(srcBlock, dstBlock, ms, r) {
			t.Error("expected cross-scope container move with low retention (10%) to be demoted")
		}
	})

	t.Run("preserves cross-scope container move with high retention", func(t *testing.T) {
		fn1 := mkNode("function_declaration", "f1")
		fn1.Language = "go"
		fn2 := mkNode("function_declaration", "f2")
		fn2.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = fn1
		srcBlock.StartRow = 10
		srcBlock.EndRow = 30
		fn1.Children = append(fn1.Children, srcBlock)

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = fn2
		dstBlock.StartRow = 35
		dstBlock.EndRow = 55
		fn2.Children = append(fn2.Children, dstBlock)

		for i := range 20 {
			c := mkNode("identifier", fmt.Sprintf("var%d", i))
			c.Language = "go"
			c.Parent = srcBlock
			srcBlock.Children = append(srcBlock.Children, c)
		}
		for i := range 20 {
			c := mkNode("identifier", fmt.Sprintf("var%d", i))
			c.Language = "go"
			c.Parent = dstBlock
			dstBlock.Children = append(dstBlock.Children, c)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		// Map 18 out of 20 leaves (90% retention >= 25%).
		for i := range 18 {
			ms.Add(srcBlock.Children[i], dstBlock.Children[i])
		}

		if shouldDemoteMove(srcBlock, dstBlock, ms, r) {
			t.Error("expected cross-scope container move with high retention (90%) to be preserved")
		}
	})

	t.Run("anchor protection preserves high-value node despite cascade eviction", func(t *testing.T) {
		fn1 := mkNode("function_declaration", "handlerA")
		fn1.Language = "go"
		fn2 := mkNode("function_declaration", "handlerB")
		fn2.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = fn1
		srcBlock.StartRow = 10
		srcBlock.EndRow = 20
		fn1.Children = append(fn1.Children, srcBlock)

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = fn2
		dstBlock.StartRow = 210
		dstBlock.EndRow = 220
		fn2.Children = append(fn2.Children, dstBlock)

		for i := range 20 {
			c := mkNode("identifier", fmt.Sprintf("x%d", i))
			c.Language = "go"
			c.Parent = srcBlock
			srcBlock.Children = append(srcBlock.Children, c)

			d := mkNode("identifier", fmt.Sprintf("x%d", i))
			d.Language = "go"
			d.Parent = dstBlock
			dstBlock.Children = append(dstBlock.Children, d)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		for i := range 20 {
			ms.Add(srcBlock.Children[i], dstBlock.Children[i])
		}

		// Evict 8 of 20 leaves: retention drops from 100% to 60%.
		// Score after churn penalty drops from 62 to 37, which is below threshold (70).
		// Anchor protection (score >= 30, intrinsic retention 100%) prevents demotion.
		evicted := make(map[*treesitter.ASTNode]struct{})
		for i := range 8 {
			evicted[srcBlock.Children[i]] = struct{}{}
			evicted[dstBlock.Children[i]] = struct{}{}
		}

		if shouldDemoteMove(srcBlock, dstBlock, ms, r, evicted) {
			t.Error("expected anchor move (score >= 30 with intrinsic retention 100%) to resist cascade demotion")
		}
	})

	t.Run("anchor protection does not exempt node when cascade caused no retention loss", func(t *testing.T) {
		fn1 := mkNode("function_declaration", "handlerA")
		fn1.Language = "go"
		fn2 := mkNode("function_declaration", "handlerB")
		fn2.Language = "go"

		srcBlock := mkNode("block", "")
		srcBlock.Language = "go"
		srcBlock.Parent = fn1
		srcBlock.StartRow = 10
		srcBlock.EndRow = 20
		fn1.Children = append(fn1.Children, srcBlock)

		dstBlock := mkNode("block", "")
		dstBlock.Language = "go"
		dstBlock.Parent = fn2
		dstBlock.StartRow = 210
		dstBlock.EndRow = 220
		fn2.Children = append(fn2.Children, dstBlock)

		for i := range 8 {
			c := mkNode("identifier", fmt.Sprintf("x%d", i))
			c.Language = "go"
			c.Parent = srcBlock
			srcBlock.Children = append(srcBlock.Children, c)

			d := mkNode("identifier", fmt.Sprintf("x%d", i))
			d.Language = "go"
			d.Parent = dstBlock
			dstBlock.Children = append(dstBlock.Children, d)
		}

		ms := engine.NewMapping()
		ms.Add(srcBlock, dstBlock)
		for i := range 8 {
			ms.Add(srcBlock.Children[i], dstBlock.Children[i])
		}

		unrelatedNode := mkNode("identifier", "unrelated")
		evicted := map[*treesitter.ASTNode]struct{}{unrelatedNode: {}}

		// Node had 0 cascade degradation (retention == preEvictionRetention == 1.0),
		// so anchor protection must not exempt it from score < threshold demotion.
		if !shouldDemoteMove(srcBlock, dstBlock, ms, r, evicted) {
			t.Error("expected move below threshold without cascade degradation to be demoted")
		}
	})
}

func TestNormalizeMovesByStructure_ScopedEviction(t *testing.T) {
	// Container t1 moves across statements to dst1, but contains a substantial child call c1 mapped to c2 elsewhere.
	var srcArgs []*treesitter.ASTNode
	var dstArgs []*treesitter.ASTNode
	for range 10 {
		a1 := mkNode("identifier", "arg")
		a1.Language = "go"
		srcArgs = append(srcArgs, a1)
		a2 := mkNode("identifier", "arg")
		a2.Language = "go"
		dstArgs = append(dstArgs, a2)
	}
	srcArgList := mkNode("argument_list", "", srcArgs...)
	srcArgList.Language = "go"
	for _, a := range srcArgs {
		a.Parent = srcArgList
	}
	dstArgList := mkNode("argument_list", "", dstArgs...)
	dstArgList.Language = "go"
	for _, a := range dstArgs {
		a.Parent = dstArgList
	}

	c1 := mkNode("call_expression", "", mkNode("identifier", "format"), srcArgList)
	c1.Language = "go"
	c1.StartRow = 10
	c1.EndRow = 12
	srcArgList.Parent = c1
	c1.Children[0].Parent = c1

	t1 := mkNode("parenthesized_expression", "", c1)
	t1.Language = "go"
	c1.Parent = t1
	t1.StartRow = 10
	t1.EndRow = 12

	dst1 := mkNode("parenthesized_expression", "")
	dst1.Language = "go"
	dst1.StartRow = 50
	dst1.EndRow = 50

	c2 := mkNode("call_expression", "", mkNode("identifier", "format"), dstArgList)
	c2.Language = "go"
	c2.StartRow = 100
	c2.EndRow = 102
	dstArgList.Parent = c2
	c2.Children[0].Parent = c2

	ms := engine.NewMapping()
	ms.Add(t1, dst1)
	ms.Add(c1, c2)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: t1, DestNode: dst1})
	es.Add(actions.Action{Type: actions.Move, Node: c1, DestNode: c2})

	result := normalizeMovesByStructure(es, ms)

	if ms.Src()[c1] != c2 {
		t.Errorf("expected mapping of survivor child c1 to be preserved, got %v", ms.Src()[c1])
	}

	var c1Moved bool
	for _, a := range result.Actions() {
		if a.Node == c1 && a.Type == actions.Move {
			c1Moved = true
		}
		if a.Node == t1 && a.Type == actions.Delete && a.Subtree {
			t.Errorf("expected demoted container delete to have Subtree: false, got true")
		}
	}
	if !c1Moved {
		t.Errorf("expected survivor child Move action to be preserved")
	}
}

func TestNormalizeMovesByStructure_EmitsDeleteForEvictedDescendantsWhenNonSubtree(t *testing.T) {
	// Moving c1 out of t1 (and c0 into dst1) keeps the demoted container's Delete
	// and Insert non-subtree. Evicted children like opSrc/opDst that stayed inside
	// the container still need their own Delete and Insert actions.
	opSrc := mkNode("logical_operator_literal", "&&")
	opSrc.Language = "go"
	c1 := mkNode("identifier", "survivor")
	c1.Language = "go"
	t1 := mkNode("binary_expression", "", c1, opSrc)
	t1.Language = "go"
	t1.StartRow = 10
	t1.EndRow = 10

	opDst := mkNode("logical_operator_literal", "&&")
	opDst.Language = "go"
	c3 := mkNode("identifier", "survivor2")
	c3.Language = "go"
	dst1 := mkNode("binary_expression", "", c3, opDst)
	dst1.Language = "go"
	dst1.StartRow = 50
	dst1.EndRow = 50

	c2 := mkNode("identifier", "survivor")
	c2.Language = "go"
	c2.StartRow = 80
	c2.EndRow = 80

	c0 := mkNode("identifier", "survivor2")
	c0.Language = "go"
	c0.StartRow = 2
	c0.EndRow = 2

	ms := engine.NewMapping()
	ms.Add(t1, dst1)
	ms.Add(opSrc, opDst)
	ms.Add(c1, c2)
	ms.Add(c0, c3)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: t1, DestNode: dst1})
	es.Add(actions.Action{Type: actions.Move, Node: c1, DestNode: c2})
	es.Add(actions.Action{Type: actions.Move, Node: c0, DestNode: c3})

	result := normalizeMovesByStructure(es, ms)

	opDeleted := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == opSrc && a.Type == actions.Delete
	})
	opInserted := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == opDst && a.Type == actions.Insert
	})

	if !opDeleted {
		t.Errorf("normalizeMovesByStructure() missing Delete for opSrc; got actions = %+v", result.Actions())
	}
	if !opInserted {
		t.Errorf("normalizeMovesByStructure() missing Insert for opDst; got actions = %+v", result.Actions())
	}
	if ms.Has(opSrc) {
		t.Error("normalizeMovesByStructure() kept opSrc in mapping, want evicted")
	}
}

func TestNormalizeMovesByStructure_TrivialJumpBlockDemoted(t *testing.T) {
	// A breakaway trivial jump block ({ return }) in another scope should not move on its own.
	retStmt1 := mkNode("return_statement", "", mkNode("return", "return"))
	retStmt1.Language = "go"
	retStmt1.Children[0].Parent = retStmt1
	block1 := mkNode("block", "", retStmt1)
	block1.Language = "go"
	retStmt1.Parent = block1
	block1.StartRow = 10
	block1.EndRow = 12

	retStmt2 := mkNode("return_statement", "", mkNode("return", "return"))
	retStmt2.Language = "go"
	retStmt2.Children[0].Parent = retStmt2
	block2 := mkNode("block", "", retStmt2)
	block2.Language = "go"
	retStmt2.Parent = block2
	block2.StartRow = 100
	block2.EndRow = 102

	ms := engine.NewMapping()
	ms.Add(block1, block2)
	ms.Add(retStmt1, retStmt2)

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: block1, DestNode: block2})

	// Boilerplate penalty knocks the score to 1 (below the threshold of 20), forcing delete+insert.
	result := normalizeMovesByStructure(es, ms)

	hasMove := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block1 && a.Type == actions.Move
	})
	hasDelete := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block1 && a.Type == actions.Delete
	})
	hasInsert := slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == block2 && a.Type == actions.Insert
	})

	if hasMove {
		t.Errorf("expected breakaway trivial jump block move to be demoted, but Move action survived")
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected trivial jump block to be demoted to Delete+Insert, got delete=%v, insert=%v", hasDelete, hasInsert)
	}
}

func TestNormalizeMovesByStructure_StationaryExpressionMoveDropped(t *testing.T) {
	// A call expression reorganized within the same line and statement shouldn't stay a move.
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
	stmt2.StartRow = 10

	call2 := mkNode("call_expression", "")
	call2.Language = "go"
	call2.Parent = stmt2
	call2.StartRow = 10

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

	if slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == call1 && a.Type == actions.Move
	}) {
		t.Errorf("expected stationary expression Move to be dropped from edit script")
	}
}

func TestNormalizeMovesByStructure_HollowLoopDemotedAcrossScopes(t *testing.T) {
	// Even if the loop header matches across functions, drop the move if the body was completely wiped out.
	rangeClause1 := mkNode("range_clause", "", mkNode("identifier", "a"), mkNode("identifier", "Actions"))
	rangeClause1.Language = "go"
	for _, c := range rangeClause1.Children {
		c.Parent = rangeClause1
	}

	bodyStmt1 := mkNode("expression_statement", "", mkNode("identifier", "deadLogic"))
	bodyStmt1.Language = "go"
	bodyStmt1.Children[0].Parent = bodyStmt1
	bodyBlock1 := mkNode("block", "", bodyStmt1)
	bodyBlock1.Language = "go"
	bodyStmt1.Parent = bodyBlock1

	forStmt1 := mkNode("for_statement", "", rangeClause1, bodyBlock1)
	forStmt1.Language = "go"
	forStmt1.StartRow = 160
	forStmt1.EndRow = 211
	rangeClause1.Parent = forStmt1
	bodyBlock1.Parent = forStmt1

	rangeClause2 := mkNode("range_clause", "", mkNode("identifier", "a"), mkNode("identifier", "Actions"))
	rangeClause2.Language = "go"
	for _, c := range rangeClause2.Children {
		c.Parent = rangeClause2
	}

	bodyStmt2 := mkNode("expression_statement", "", mkNode("identifier", "brandNewLogic"))
	bodyStmt2.Language = "go"
	bodyStmt2.Children[0].Parent = bodyStmt2
	bodyBlock2 := mkNode("block", "", bodyStmt2)
	bodyBlock2.Language = "go"
	bodyStmt2.Parent = bodyBlock2

	forStmt2 := mkNode("for_statement", "", rangeClause2, bodyBlock2)
	forStmt2.Language = "go"
	forStmt2.StartRow = 476
	forStmt2.EndRow = 525
	rangeClause2.Parent = forStmt2
	bodyBlock2.Parent = forStmt2

	ms := engine.NewMapping()
	ms.Add(forStmt1, forStmt2)
	ms.Add(rangeClause1, rangeClause2)
	for i := range rangeClause1.Children {
		ms.Add(rangeClause1.Children[i], rangeClause2.Children[i])
	}
	// Only the loop header is mapped, body has 0% retention.

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: forStmt1, DestNode: forStmt2})

	result := normalizeMovesByStructure(es, ms)

	if slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == forStmt1 && a.Type == actions.Move
	}) {
		t.Errorf("expected hollow loop move with 0%% body retention to be demoted, but Move survived")
	}
}

func TestNormalizeMovesByStructure_LoopExtractionPreserved(t *testing.T) {
	// Moving a loop out to a helper function should stay a Move when its body logic survives.
	rangeClause1 := mkNode("range_clause", "", mkNode("identifier", "item"))
	rangeClause1.Language = "go"
	rangeClause1.Children[0].Parent = rangeClause1

	stmt1 := mkNode("expression_statement", "", mkNode("identifier", "process"))
	stmt1.Language = "go"
	stmt1.Children[0].Parent = stmt1
	bodyBlock1 := mkNode("block", "", stmt1)
	bodyBlock1.Language = "go"
	stmt1.Parent = bodyBlock1

	forStmt1 := mkNode("for_statement", "", rangeClause1, bodyBlock1)
	forStmt1.Language = "go"
	forStmt1.StartRow = 20
	forStmt1.EndRow = 30
	rangeClause1.Parent = forStmt1
	bodyBlock1.Parent = forStmt1

	rangeClause2 := mkNode("range_clause", "", mkNode("identifier", "item"))
	rangeClause2.Language = "go"
	rangeClause2.Children[0].Parent = rangeClause2

	stmt2 := mkNode("expression_statement", "", mkNode("identifier", "process"))
	stmt2.Language = "go"
	stmt2.Children[0].Parent = stmt2
	bodyBlock2 := mkNode("block", "", stmt2)
	bodyBlock2.Language = "go"
	stmt2.Parent = bodyBlock2

	forStmt2 := mkNode("for_statement", "", rangeClause2, bodyBlock2)
	forStmt2.Language = "go"
	forStmt2.StartRow = 150
	forStmt2.EndRow = 160
	rangeClause2.Parent = forStmt2
	bodyBlock2.Parent = forStmt2

	ms := engine.NewMapping()
	ms.Add(forStmt1, forStmt2)
	ms.Add(rangeClause1, rangeClause2)
	ms.Add(rangeClause1.Children[0], rangeClause2.Children[0])
	ms.Add(bodyBlock1, bodyBlock2)
	ms.Add(stmt1, stmt2)
	ms.Add(stmt1.Children[0], stmt2.Children[0])

	es := actions.NewEditScript()
	es.Add(actions.Action{Type: actions.Move, Node: forStmt1, DestNode: forStmt2})

	result := normalizeMovesByStructure(es, ms)

	if !slices.ContainsFunc(result.Actions(), func(a actions.Action) bool {
		return a.Node == forStmt1 && a.Type == actions.Move
	}) {
		t.Errorf("expected extracted loop with 100%% surviving body to be preserved as Move")
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

func TestIsPayloadLeaf(t *testing.T) {
	r := rules.Get("go")

	t.Run("nil safety and non-leaf", func(t *testing.T) {
		if isPayloadLeaf(nil, r) {
			t.Errorf("expected nil node to not be payload leaf")
		}
		parent := mkNode("call_expression", "", mkNode("identifier", "foo"))
		if isPayloadLeaf(parent, r) {
			t.Errorf("expected node with children to not be payload leaf")
		}
	})

	t.Run("punctuation and operators rejected", func(t *testing.T) {
		punct := mkNode(";", ";")
		if isPayloadLeaf(punct, r) {
			t.Errorf("expected semicolon to not be payload leaf")
		}
		op := mkNode("arithmetic_operator_literal", "+")
		if isPayloadLeaf(op, r) {
			t.Errorf("expected operator to not be payload leaf")
		}
	})

	t.Run("keywords filtering and jump statement exception", func(t *testing.T) {
		funcKw := mkNode("func", "func")
		funcKw.IsKeyword = true
		if isPayloadLeaf(funcKw, r) {
			t.Errorf("expected 'func' keyword to not be payload leaf")
		}

		retKwBare := mkNode("return", "return")
		retKwBare.IsKeyword = true
		if isPayloadLeaf(retKwBare, r) {
			t.Errorf("expected bare 'return' without jump parent to not be payload leaf")
		}

		jumpStmt := mkNode("return_statement", "")
		retKwUnderJump := mkNode("return", "return")
		retKwUnderJump.IsKeyword = true
		retKwUnderJump.Parent = jumpStmt
		jumpStmt.Children = append(jumpStmt.Children, retKwUnderJump)

		if !isPayloadLeaf(retKwUnderJump, r) {
			t.Errorf("expected 'return' keyword inside return_statement to be payload leaf")
		}
	})

	t.Run("content leaves accepted", func(t *testing.T) {
		id := mkNode("identifier", "myVar")
		if !isPayloadLeaf(id, r) {
			t.Errorf("expected identifier to be payload leaf")
		}
		lit := mkNode("interpreted_string_literal", `"hello"`)
		if !isPayloadLeaf(lit, r) {
			t.Errorf("expected literal to be payload leaf")
		}
	})
}

func TestSurvivingAndSummarizeMappedLeaves(t *testing.T) {
	r := rules.Get("go")

	srcLeaf1 := mkNode("identifier", "foo")
	srcLeaf2 := mkNode("identifier", "bar")
	srcPunct := mkNode(",", ",")
	srcBlock := mkNode("block", "", srcLeaf1, srcLeaf2, srcPunct)
	srcLeaf1.Parent = srcBlock
	srcLeaf2.Parent = srcBlock
	srcPunct.Parent = srcBlock

	dstLeaf1 := mkNode("identifier", "foo") // exact
	dstLeaf2 := mkNode("identifier", "baz") // updated label
	dstBlock := mkNode("block", "", dstLeaf1, dstLeaf2)
	dstLeaf1.Parent = dstBlock
	dstLeaf2.Parent = dstBlock

	ms := engine.NewMapping()
	ms.Add(srcLeaf1, dstLeaf1)
	ms.Add(srcLeaf2, dstLeaf2)

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
			t.Errorf("expected total=2 (ignoring comma), got %d", s.total)
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

	blockNode := mkNode("block", "")
	if !isDelimitedOrBlockContainer(blockNode, r) {
		t.Errorf("expected block to return true")
	}

	argListNode := mkNode("argument_list", "")
	if !isDelimitedOrBlockContainer(argListNode, r) {
		t.Errorf("expected argument_list to return true")
	}

	identNode := mkNode("identifier", "x")
	if isDelimitedOrBlockContainer(identNode, r) {
		t.Errorf("expected identifier to return false")
	}

	// r == nil fallback
	if !isDelimitedOrBlockContainer(blockNode, nil) {
		t.Errorf("expected block with nil rules to return true")
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

func TestBuildCohortProtected(t *testing.T) {
	t.Run("cohort with anchor is protected", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		anchorSrc := mkNode("for_statement", "")
		anchorSrc.Language = "go"
		anchorSrc.StartRow, anchorSrc.EndRow = 10, 25
		anchorSrc.Parent = srcBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorSrc
			anchorSrc.Children = append(anchorSrc.Children, child)
		}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody

		stmt2Src := mkNode("short_var_declaration", "y := 2")
		stmt2Src.Language = "go"
		stmt2Src.Parent = srcBody

		srcBody.Children = []*treesitter.ASTNode{anchorSrc, stmt1Src, stmt2Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		anchorDst := mkNode("for_statement", "")
		anchorDst.Language = "go"
		anchorDst.StartRow, anchorDst.EndRow = 10, 25
		anchorDst.Parent = dstBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorDst
			anchorDst.Children = append(anchorDst.Children, child)
		}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody

		stmt2Dst := mkNode("short_var_declaration", "y := 2")
		stmt2Dst.Language = "go"
		stmt2Dst.Parent = dstBody

		dstBody.Children = []*treesitter.ASTNode{anchorDst, stmt1Dst, stmt2Dst}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		for i := range anchorSrc.Children {
			ms.Add(anchorSrc.Children[i], anchorDst.Children[i])
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
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody

		stmt2Src := mkNode("short_var_declaration", "y := 2")
		stmt2Src.Language = "go"
		stmt2Src.Parent = srcBody

		stmt3Src := mkNode("short_var_declaration", "z := 3")
		stmt3Src.Language = "go"
		stmt3Src.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{stmt1Src, stmt2Src, stmt3Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody

		stmt2Dst := mkNode("short_var_declaration", "y := 2")
		stmt2Dst.Language = "go"
		stmt2Dst.Parent = dstBody

		stmt3Dst := mkNode("short_var_declaration", "z := 3")
		stmt3Dst.Language = "go"
		stmt3Dst.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{stmt1Dst, stmt2Dst, stmt3Dst}

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
				t.Errorf("expected small move without anchor to be demoted, but found Move for %v", a.Node.Label)
			}
		}
	})

	t.Run("cohort with fewer than 3 moves is not protected", func(t *testing.T) {
		srcFunc := mkNode("function_declaration", "fnOld")
		srcFunc.Language = "go"
		srcBody := mkNode("block", "")
		srcBody.Language = "go"
		srcBody.Parent = srcFunc
		srcFunc.Children = []*treesitter.ASTNode{srcBody}

		anchorSrc := mkNode("for_statement", "")
		anchorSrc.Language = "go"
		anchorSrc.Parent = srcBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorSrc
			anchorSrc.Children = append(anchorSrc.Children, child)
		}

		stmt1Src := mkNode("short_var_declaration", "x := 1")
		stmt1Src.Language = "go"
		stmt1Src.Parent = srcBody
		srcBody.Children = []*treesitter.ASTNode{anchorSrc, stmt1Src}

		dstFunc := mkNode("function_declaration", "fnNew")
		dstFunc.Language = "go"
		dstBody := mkNode("block", "")
		dstBody.Language = "go"
		dstBody.Parent = dstFunc
		dstFunc.Children = []*treesitter.ASTNode{dstBody}

		anchorDst := mkNode("for_statement", "")
		anchorDst.Language = "go"
		anchorDst.Parent = dstBody
		for i := range 20 {
			child := mkNode("expression_statement", fmt.Sprintf("stmt%d", i))
			child.Language = "go"
			child.Parent = anchorDst
			anchorDst.Children = append(anchorDst.Children, child)
		}

		stmt1Dst := mkNode("short_var_declaration", "x := 1")
		stmt1Dst.Language = "go"
		stmt1Dst.Parent = dstBody
		dstBody.Children = []*treesitter.ASTNode{anchorDst, stmt1Dst}

		ms := engine.NewMapping()
		ms.Add(anchorSrc, anchorDst)
		for i := range anchorSrc.Children {
			ms.Add(anchorSrc.Children[i], anchorDst.Children[i])
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

func TestNormalizeMovesByStructure_DemotesHollowExpressionMoveWhenDescendantsEvicted(t *testing.T) {
	src := []byte(`package test
func isDeclarationHeader(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	enc := GetEnclosingDeclaration(n)
	if enc == nil {
		return false
	}
	body := findBodyBlock(enc, r)
	if body != nil && (body == n || body.Contains(n)) {
		return false
	}
	return true
}
`)

	dst := []byte(`package test
func isDeclarationHeader(n *treesitter.ASTNode, r *rules.Rules) bool {
	if n == nil {
		return false
	}
	if r == nil {
		r = rulesFor(n)
	}
	for curr := n.Parent; curr != nil; curr = curr.Parent {
		isContainer := (r != nil && r.IsContainerDeclaration(curr.Type)) ||
			(r == nil && rules.IsContainerDeclaration(curr.Type))
		if isContainer {
			body := findBodyBlock(curr, r)
			if body == nil {
				return false
			}
			return body != n && !body.Contains(n)
		}
	}
	return false
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

	hasHollowMove := slices.ContainsFunc(normalized.Actions(), func(a actions.Action) bool {
		return a.Type == actions.Move && a.Node.Type == "binary_expression"
	})
	if hasHollowMove {
		t.Errorf("expected binary_expression move to be demoted after descendants were evicted, but found an active Move")
	}
}
