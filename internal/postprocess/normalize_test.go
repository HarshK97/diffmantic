package postprocess

import (
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

	t.Run("bare token clamped to 1", func(t *testing.T) {
		op := mkNode("arithmetic_operator_literal", "+")
		op.Language = "go"
		score := moveStructuralScore(op, r)
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

		score := moveStructuralScore(ifStmt, r)
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

		score := moveStructuralScore(decl, r)
		if score < 40 {
			t.Errorf("expected declaration score >= 40, got %d", score)
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
		srcVal.Language = "toml"
		srcVal.StartRow = 5

		dstVal := mkNode("string", "2.0.0")
		dstVal.Language = "toml"
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
		srcContainer := mkNode("table", "")
		srcContainer.Language = "toml"
		srcVal := mkNode("string", "hello")
		srcVal.Language = "toml"
		srcVal.Parent = srcContainer
		srcContainer.Children = []*treesitter.ASTNode{srcVal}

		dstContainer := mkNode("table", "")
		dstContainer.Language = "toml"
		dstPair := mkNode("pair", "")
		dstPair.Language = "toml"
		dstPair.Parent = dstContainer
		dstVal := mkNode("string", "hello")
		dstVal.Language = "toml"
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
}
