package actions

import (
	"slices"
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestUnorderedNodeMatching(t *testing.T) {
	srcCode := []byte("const x = { a: 1, b: 2 };\n")
	dstCode := []byte("const x = { b: 2, a: 1 };\n")

	srcAST, err := treesitter.Parse(srcCode, "test.js")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dstCode, "test.js")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, srcCode, dstCode, nil)
	script := GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

	assertNoActions(t, script, Move)
}

func TestTSXUnorderedAttributesMatching(t *testing.T) {
	src := []byte(`<Component a="1" b="2" />`)
	dst := []byte(`<Component b="2" a="1" />`)

	srcAST, err := treesitter.Parse(src, "Component.tsx")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "Component.tsx")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
	script := GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

	assertNoActions(t, script, Move)
	if script.Size() != 0 {
		t.Errorf("expected 0 actions for unordered JSX attribute reordering, got %d actions", script.Size())
	}
}

func TestInsertChildDefensive(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		insertChild(nil, nil, 0)
	})

	t.Run("child nil", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		parent.children = nil

		insertChild(parent, nil, 0)
		if len(parent.children) != 0 {
			t.Fatalf("expected 0 children, got %d", len(parent.children))
		}
	})

	t.Run("parent nil", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		child := cs.cpySrc.children[0]
		child.parent = nil

		insertChild(nil, child, 0)
		if child.parent != nil {
			t.Errorf("expected nil parent, got %v", child.parent)
		}
	})

	t.Run("negative index clamped", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		child := parent.children[0]
		parent.children = nil
		child.parent = nil

		insertChild(parent, child, -5)
		if len(parent.children) != 1 || parent.children[0] != child {
			t.Fatalf("expected child at index 0")
		}
	})

	t.Run("excess index clamped", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\nb = 2\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		child1 := parent.children[0]
		child2 := parent.children[1]
		parent.children = nil
		child1.parent = nil
		child2.parent = nil

		insertChild(parent, child1, 0)
		insertChild(parent, child2, 100)
		if len(parent.children) != 2 || parent.children[1] != child2 {
			t.Fatalf("expected child2 at end of children list")
		}
	})

	t.Run("atomic reparenting", func(t *testing.T) {
		code := []byte("def f():\n    pass\ndef g():\n    pass\nx = 1\n")
		ast, err := treesitter.Parse(code, "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		oldParent := cs.cpySrc.children[0]
		newParent := cs.cpySrc.children[1]
		child := cs.cpySrc.children[2]

		child.parent = nil
		oldParent.children = nil
		newParent.children = nil

		insertChild(oldParent, child, 0)
		if len(oldParent.children) != 1 || child.parent != oldParent {
			t.Fatalf("failed initial insert into oldParent")
		}
		insertChild(newParent, child, 0)
		if len(oldParent.children) != 0 {
			t.Errorf("expected child detached from oldParent, got %d children", len(oldParent.children))
		}
		if len(newParent.children) != 1 || child.parent != newParent {
			t.Errorf("expected child attached to newParent")
		}
	})
}

func TestDeleteChildDefensive(t *testing.T) {
	t.Run("nil safety", func(t *testing.T) {
		deleteChild(nil, nil)

		ast, err := treesitter.Parse([]byte("a = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		child := parent.children[0]
		parent.children = nil
		child.parent = nil

		deleteChild(parent, nil)
		if len(parent.children) != 0 {
			t.Errorf("expected 0 children after deleteChild(parent, nil), got %d", len(parent.children))
		}

		deleteChild(nil, child)
		if child.parent != nil {
			t.Errorf("expected child.parent to remain nil after deleteChild(nil, child), got %v", child.parent)
		}
	})

	t.Run("unlinked child", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\nb = 2\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		child := parent.children[0]
		unlinked := parent.children[1]

		parent.children = []*cnode{child}
		child.parent = parent
		unlinked.parent = nil

		deleteChild(parent, unlinked)
		if len(parent.children) != 1 {
			t.Fatalf("expected 1 child remaining, got %d", len(parent.children))
		}
	})

	t.Run("valid deletion", func(t *testing.T) {
		ast, err := treesitter.Parse([]byte("a = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}
		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		parent := cs.cpySrc
		child := parent.children[0]

		deleteChild(parent, child)
		if len(parent.children) != 0 {
			t.Fatalf("expected 0 children after delete, got %d", len(parent.children))
		}
		if child.parent != nil {
			t.Errorf("expected child.parent to be nil after deletion")
		}
	})
}

func TestFindPosDefensive(t *testing.T) {
	code := []byte("package main\n\nfunc f1() {\n\ta()\n\tc()\n}\n\nfunc f2() {\n\tb()\n}\n")
	ast, err := treesitter.Parse(code, "main.go")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	callA, err := testutil.FindNodeByText(ast, code, "a()")
	if err != nil {
		t.Fatalf("failed to find callA: %v", err)
	}
	stmtA := callA.Parent
	callC, err := testutil.FindNodeByText(ast, code, "c()")
	if err != nil {
		t.Fatalf("failed to find callC: %v", err)
	}
	stmtC := callC.Parent
	callB, err := testutil.FindNodeByText(ast, code, "b()")
	if err != nil {
		t.Fatalf("failed to find callB: %v", err)
	}
	stmtB := callB.Parent

	cs := &chawatheState{}
	cs.init(ast, ast, engine.NewMapping())

	targetChildA := cs.origToCopy[stmtA]
	targetContainer := targetChildA.parent

	foreignChildB := cs.origToCopy[stmtB]
	if foreignChildB.parent == targetContainer {
		t.Fatal("expected foreign container to differ from target container")
	}

	s := &chawatheState{
		dstInOrder:  make(map[*treesitter.ASTNode]bool),
		srcInOrder:  make(map[*cnode]bool),
		cpyDstToSrc: make(map[*treesitter.ASTNode]*cnode),
	}

	t.Run("nil node", func(t *testing.T) {
		if pos := s.findPos(nil, targetContainer); pos != 0 {
			t.Errorf("expected findPos(nil) = 0, got %d", pos)
		}
	})

	t.Run("nil target container", func(t *testing.T) {
		if pos := s.findPos(stmtC, nil); pos != 0 {
			t.Errorf("expected findPos with nil target = 0, got %d", pos)
		}
	})

	t.Run("orphan node", func(t *testing.T) {
		// Root node ast has nil parent
		if pos := s.findPos(ast, targetContainer); pos != 0 {
			t.Errorf("expected findPos(orphan) = 0, got %d", pos)
		}
	})

	t.Run("detached node", func(t *testing.T) {
		if pos := s.findPos(stmtA, targetContainer); pos != 0 {
			t.Errorf("expected findPos(detached) = 0, got %d", pos)
		}
	})

	t.Run("unmapped partner sibling", func(t *testing.T) {
		s.dstInOrder[stmtA] = true
		if pos := s.findPos(stmtC, targetContainer); pos != 0 {
			t.Errorf("expected findPos with unmapped partner = 0, got %d", pos)
		}
	})

	t.Run("foreign parent sibling prevention", func(t *testing.T) {
		s.dstInOrder[stmtA] = true
		s.srcInOrder[foreignChildB] = true
		s.cpyDstToSrc[stmtA] = foreignChildB // foreignChildB.parent == foreignContainer != targetContainer

		pos := s.findPos(stmtC, targetContainer)
		if pos != 0 {
			t.Errorf("expected pos 0 due to foreign parent check, got %d (positional bleed occurred)", pos)
		}
	})

	t.Run("valid partner in target container", func(t *testing.T) {
		s.dstInOrder[stmtA] = true
		s.srcInOrder[targetChildA] = true
		s.cpyDstToSrc[stmtA] = targetChildA // targetChildA.parent == targetContainer

		pos := s.findPos(stmtC, targetContainer)
		if pos != 1 {
			t.Errorf("expected pos 1, got %d", pos)
		}
	})
}

func TestNonRootProvenanceFallback(t *testing.T) {
	code := []byte("package main\n\nfunc fn() {\n\tx := 1\n}\n")
	dstRoot, err := treesitter.Parse(code, "main.go")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	script := GenerateEditScript(nil, dstRoot, nil)
	if script == nil {
		t.Fatalf("expected non-nil script")
	}

	for _, a := range script.Actions() {
		if a.Node == dstRoot {
			if a.Parent != nil {
				t.Errorf("expected root Insert to have Parent nil (source realm purity), got %v", a.Parent)
			}
		} else if a.Node.Parent != nil {
			if a.Parent != a.Node.Parent {
				t.Errorf("expected child %v Insert to have Parent %v, got %v", a.Node.Type, a.Node.Parent.Type, a.Parent)
			}
		}
	}

	// Verify unmapped synthetic parent evaluates strictly to nil (pointer domain purity)
	ident, err := testutil.FindNodeByText(dstRoot, code, "fn")
	if err != nil {
		t.Fatalf("failed to find fn identifier: %v", err)
	}
	goRules := rules.Get("go")
	var fnNode *treesitter.ASTNode
	for p := ident.Parent; p != nil; p = p.Parent {
		if goRules.IsDeclaration(p.Type) {
			fnNode = p
			break
		}
	}
	if fnNode == nil {
		t.Fatal("failed to find function declaration in dstRoot")
	}
	// Passing a subtree directly without mapping its enclosing root
	scriptSub := GenerateEditScript(nil, fnNode, nil)
	for _, a := range scriptSub.Actions() {
		if a.Node == fnNode {
			if a.Parent != nil {
				t.Errorf("expected root of subtree insert to have nil Parent, got %v", a.Parent)
			}
		}
	}
}

func TestInsertChildSameParentNoDuplication(t *testing.T) {
	code := []byte("a = 1\nb = 2\nc = 3\n")
	ast, err := treesitter.Parse(code, "test.py")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	cs := &chawatheState{}
	cs.init(ast, ast, engine.NewMapping())
	parent := cs.cpySrc
	if len(parent.children) < 3 {
		t.Fatalf("expected at least 3 children, got %d", len(parent.children))
	}
	c0 := parent.children[0]
	c1 := parent.children[1]
	c2 := parent.children[2]

	// Move c0 from index 0 to index 2 within the same parent
	insertChild(parent, c0, 2)

	if len(parent.children) != 3 {
		t.Fatalf("expected 3 children after intra-container move, got %d (child was duplicated)", len(parent.children))
	}
	if parent.children[0] != c1 || parent.children[1] != c2 || parent.children[2] != c0 {
		t.Fatalf("unexpected child ordering after reorder: got [%v, %v, %v]",
			parent.children[0].nodeType, parent.children[1].nodeType, parent.children[2].nodeType)
	}

	// Move c0 back to index 0 within the same parent
	insertChild(parent, c0, 0)
	if len(parent.children) != 3 {
		t.Fatalf("expected 3 children after moving back to index 0, got %d", len(parent.children))
	}
	if parent.children[0] != c0 || parent.children[1] != c1 || parent.children[2] != c2 {
		t.Fatalf("unexpected child ordering after moving to index 0: got [%v, %v, %v]",
			parent.children[0].nodeType, parent.children[1].nodeType, parent.children[2].nodeType)
	}
}

func TestInsertNodeTypePreserved(t *testing.T) {
	srcCode := []byte("# empty\n")
	dstCode := []byte("def my_func():\n    pass\n")
	src, err := treesitter.Parse(srcCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	pyRules := rules.Get("python")
	fnIdent, err := testutil.FindNodeByText(dst, dstCode, "my_func")
	if err != nil {
		t.Fatalf("failed to find identifier my_func: %v", err)
	}
	var dstChild *treesitter.ASTNode
	for p := fnIdent.Parent; p != nil; p = p.Parent {
		if pyRules.IsDeclaration(p.Type) {
			dstChild = p
			break
		}
	}
	if dstChild == nil {
		t.Fatal("failed to find function declaration in dst")
	}

	ms := engine.NewMapping()
	ms.Add(src, dst)

	s := &chawatheState{}
	s.init(src, dst, ms)
	script := s.generate()
	if script == nil {
		t.Fatalf("expected non-nil script")
	}

	w, ok := s.cpyDstToSrc[dstChild]
	if !ok || w == nil {
		t.Fatalf("expected inserted node to be registered in cpyDstToSrc")
	}
	if !pyRules.IsDeclaration(w.nodeType) {
		t.Errorf("expected working node type to be declaration, got %q", w.nodeType)
	}
	if w.orig != dstChild {
		t.Errorf("expected working node orig to point to dstChild, got %v", w.orig)
	}
}

func TestLCSTieBreakingScore(t *testing.T) {
	srcCode := []byte("a = 1\nb = 2\n")
	dstCode := []byte("a = 1\n")
	src, err := treesitter.Parse(srcCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "test.py")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	cs := &chawatheState{}
	cs.init(src, dst, engine.NewMapping())

	if len(cs.cpySrc.children) < 2 {
		t.Fatalf("expected at least 2 children in cpySrc, got %d", len(cs.cpySrc.children))
	}
	if len(dst.Children) < 1 {
		t.Fatalf("expected at least 1 child in dst, got %d", len(dst.Children))
	}

	x0 := cs.cpySrc.children[0]
	x1 := cs.cpySrc.children[1]
	y0 := dst.Children[0]

	cs.cpyDstToSrc[y0] = x0

	pairs := cs.lcs([]*cnode{x0, x1}, []*treesitter.ASTNode{y0})
	if len(pairs) != 1 || pairs[0].src != x0 || pairs[0].dst != y0 {
		t.Fatalf("expected pair (x0, y0), got %+v", pairs)
	}
}

func TestGenerateEditScriptNilAndPartialASTs(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		es := GenerateEditScript(nil, nil, nil)
		if es == nil || es.Size() != 0 {
			t.Fatalf("expected empty edit script for nil trees, got %v", es)
		}
	})

	t.Run("src nil dst non-nil", func(t *testing.T) {
		dst, err := treesitter.Parse([]byte("x = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}

		esInsert := GenerateEditScript(nil, dst, nil)
		if esInsert == nil || esInsert.Size() != dst.Size() {
			t.Fatalf("expected %d inserts for nil src, got %d actions", dst.Size(), esInsert.Size())
		}
		for _, a := range esInsert.Actions() {
			if a.Type != Insert {
				t.Errorf("expected Insert action, got %v", a.Type)
			}
		}
	})

	t.Run("src non-nil dst nil", func(t *testing.T) {
		src, err := treesitter.Parse([]byte("y = 2\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}

		esDelete := GenerateEditScript(src, nil, nil)
		if esDelete == nil || esDelete.Size() != src.Size() {
			t.Fatalf("expected %d deletes for nil dst, got %d actions", src.Size(), esDelete.Size())
		}
		for _, a := range esDelete.Actions() {
			if a.Type != Delete {
				t.Errorf("expected Delete action, got %v", a.Type)
			}
		}
	})

	t.Run("detached malformed dst node", func(t *testing.T) {
		src, err := treesitter.Parse([]byte("y = 2\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse src: %v", err)
		}
		detachedDst, err := treesitter.Parse([]byte("x = 1\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse dst: %v", err)
		}
		ghostParent, err := treesitter.Parse([]byte("z = 3\n"), "test.py")
		if err != nil {
			t.Fatalf("failed to parse ghost parent: %v", err)
		}
		if len(detachedDst.Children) > 0 {
			detachedDst.Children[0].Parent = ghostParent
		}

		esDetached := GenerateEditScript(src, detachedDst, engine.NewMapping())
		if esDetached == nil {
			t.Fatalf("expected non-nil script for malformed tree")
		}
	})
}

func TestCnodePostOrderDefensive(t *testing.T) {
	t.Run("nil receiver", func(t *testing.T) {
		var n *cnode
		nodes := n.PostOrder()
		if nodes != nil {
			t.Errorf("expected nil result for nil receiver, got %v", nodes)
		}
	})

	t.Run("valid hierarchy", func(t *testing.T) {
		code := []byte("def f():\n    x = 1\n")
		ast, err := treesitter.Parse(code, "test.py")
		if err != nil {
			t.Fatalf("failed to parse: %v", err)
		}

		cs := &chawatheState{}
		cs.init(ast, ast, engine.NewMapping())
		root := cs.cpySrc

		nodes := root.PostOrder()
		if len(nodes) != ast.Size() {
			t.Fatalf("expected %d nodes, got %d", ast.Size(), len(nodes))
		}
		if nodes[len(nodes)-1] != root {
			t.Fatalf("postorder root should be last, got %v", nodes[len(nodes)-1])
		}
		for i, n := range nodes {
			for _, child := range n.children {
				childIdx := slices.Index(nodes, child)
				if childIdx == -1 || childIdx >= i {
					t.Errorf("parent %s at %d has child %s at %d", n.nodeType, i, child.nodeType, childIdx)
				}
			}
		}
	})
}

func TestLCSStationarySiblingOrder(t *testing.T) {
	src := []byte(`while url:
    prepared_request = req.copy()
    hist.append(resp)
    resp.history = hist[1:]
    try:
        pass
    except Exception:
        pass
`)
	dst := []byte(`while url:
    prepared_request = req.copy()
    resp.history = hist[:]
    hist.append(resp)
    try:
        pass
    except Exception:
        pass
`)

	srcAST, err := treesitter.Parse(src, "test.py")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "test.py")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	part := engine.NewLinePartition(src, dst)
	matchResult := engine.Match(srcAST, dstAST, src, dst, part)
	script := GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

	moves := assertActionCount(t, script, Move, 1)
	movedNode := moves[0].Node

	if movedNode == nil || !movedNode.IsStatement() {
		t.Errorf("expected Move action on statement (resp.history), got %v", movedNode)
	}
}
