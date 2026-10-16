package postprocess

import (
	"fmt"
	"slices"
	"strings"
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

// setLanguageRecursive assigns the language tag to n and all its descendants.
func setLanguageRecursive(n *treesitter.ASTNode, lang string) {
	if n == nil {
		return
	}
	n.Language = lang
	for _, c := range n.Children {
		setLanguageRecursive(c, lang)
	}
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

func TestNormalizeMoves_DemotesHollowCallAndArgumentList(t *testing.T) {
	// When sort.SliceStable(...) falsely matches cmp.Or(...) and only their empty
	// argument lists get mapped, drop both moves so we don't leave an empty "()" move behind.
	srcArgList := mkNode("argument_list", "",
		mkNode("identifier", "scored"),
		mkNode("func_literal", "",
			mkNode("block", "",
				mkNode("return_statement", "", mkNode("identifier", "a")),
			),
		),
	)
	srcCall := mkNode("call_expression", "",
		mkNode("selector_expression", "",
			mkNode("identifier", "sort"),
			mkNode("field_identifier", "SliceStable"),
		),
		srcArgList,
	)
	srcStmt := mkNode("expression_statement", "", srcCall)
	srcRoot := mkNode("block", "", srcStmt)
	setLanguageRecursive(srcRoot, "go")

	dstArgList := mkNode("argument_list", "",
		mkNode("identifier", "x"),
		mkNode("identifier", "y"),
	)
	dstCall := mkNode("call_expression", "",
		mkNode("selector_expression", "",
			mkNode("identifier", "cmp"),
			mkNode("field_identifier", "Or"),
		),
		dstArgList,
	)
	dstRet := mkNode("return_statement", "", dstCall)
	dstRoot := mkNode("block", "", dstRet)
	setLanguageRecursive(dstRoot, "go")

	ms := engine.NewMapping()
	ms.Add(srcRoot, dstRoot)
	ms.Add(srcCall, dstCall)
	ms.Add(srcArgList, dstArgList)

	script := actions.GenerateEditScript(srcRoot, dstRoot, ms)
	normalized := Run(script, ms, srcRoot, dstRoot)

	for _, a := range normalized.Actions() {
		if a.Type == actions.Move && (a.Node == srcCall || a.Node == srcArgList) {
			t.Errorf("expected hollow %s move to be demoted, but Move survived", a.Node.Type)
		}
	}
	if ms.Has(srcCall) || ms.Has(srcArgList) {
		t.Errorf("expected hollow call_expression and argument_list to be evicted from mapping")
	}
}

func TestNormalizeMoves_ClosureCallAndArgumentList_EndToEnd(t *testing.T) {
	// Full pipeline: migrating from sortSliceStable to sortStableFunc returning cmpOr.
	// Postprocess must not emit a Move action on the outer call or argument list.
	src := `package main

func sortItems(scored any) {
	sortSliceStable(scored, func(i, j int) bool {
		if a != b {
			return a > b
		}
		return dice > ancSim
	})
}
`
	dst := `package main

func sortItems(scored any) {
	sortStableFunc(scored, func(a, b any) int {
		return cmpOr(
			compare(dice),
			compare(ancSim),
		)
	})
}
`
	srcBytes := []byte(src)
	dstBytes := []byte(dst)

	srcAST, err := treesitter.Parse(srcBytes, "test.go")
	if err != nil {
		t.Fatalf("parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst: %v", err)
	}

	part := engine.NewLinePartition(srcBytes, dstBytes)
	mr := engine.Match(srcAST, dstAST, srcBytes, dstBytes, part)
	if mr == nil || mr.Mappings == nil {
		t.Fatal("Match returned nil")
	}

	script := actions.GenerateEditScript(srcAST, dstAST, mr.Mappings)
	normalized := Run(script, mr.Mappings, srcAST, dstAST)

	for _, a := range normalized.Actions() {
		if a.Type == actions.Move {
			nodeText := string(srcBytes[a.Node.StartByte:a.Node.EndByte])
			if strings.HasPrefix(nodeText, "sortSliceStable") || a.Node.Type == "argument_list" {
				t.Errorf("expected no move action on %s, but found: %v", a.Node.Type, a)
			}
		}
	}
}
