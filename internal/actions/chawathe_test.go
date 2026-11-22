package actions

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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

	for _, action := range script.Actions() {
		if action.Type == Move {
			t.Errorf("expected 0 Move actions for unordered container, got Move action on node %s", action.Node.Type)
		}
	}
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

	for _, a := range script.Actions() {
		if a.Type == Move {
			t.Errorf("expected 0 Move actions for unordered JSX attribute reordering, got: %s on node %s", a.Type, a.Node.Type)
		}
	}
	if script.Size() != 0 {
		t.Errorf("expected 0 actions for unordered JSX attribute reordering, got %d actions", script.Size())
	}
}

func TestInsertChildDefensive(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		insertChild(nil, nil, 0)
	})

	t.Run("child nil", func(t *testing.T) {
		parent := &cnode{nodeType: "block"}
		insertChild(parent, nil, 0)
		if len(parent.children) != 0 {
			t.Fatalf("expected 0 children, got %d", len(parent.children))
		}
	})

	t.Run("parent nil", func(t *testing.T) {
		child := &cnode{nodeType: "stmt"}
		insertChild(nil, child, 0)
		if child.parent != nil {
			t.Errorf("expected nil parent, got %v", child.parent)
		}
	})

	t.Run("negative index clamped", func(t *testing.T) {
		parent := &cnode{nodeType: "block"}
		child := &cnode{nodeType: "stmt"}
		insertChild(parent, child, -5)
		if len(parent.children) != 1 || parent.children[0] != child {
			t.Fatalf("expected child at index 0")
		}
	})

	t.Run("excess index clamped", func(t *testing.T) {
		parent := &cnode{nodeType: "block"}
		child := &cnode{nodeType: "stmt"}
		insertChild(parent, child, 0)
		child2 := &cnode{nodeType: "stmt2"}
		insertChild(parent, child2, 100)
		if len(parent.children) != 2 || parent.children[1] != child2 {
			t.Fatalf("expected child2 at end of children list")
		}
	})

	t.Run("atomic reparenting", func(t *testing.T) {
		oldParent := &cnode{nodeType: "block1"}
		newParent := &cnode{nodeType: "block2"}
		child := &cnode{nodeType: "stmt"}
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

		parent := &cnode{nodeType: "block"}
		child := &cnode{nodeType: "stmt"}

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
		parent := &cnode{nodeType: "block"}
		child := &cnode{nodeType: "stmt"}
		unlinked := &cnode{nodeType: "orphan"}
		insertChild(parent, child, 0)
		deleteChild(parent, unlinked)
		if len(parent.children) != 1 {
			t.Fatalf("expected 1 child remaining, got %d", len(parent.children))
		}
	})

	t.Run("valid deletion", func(t *testing.T) {
		parent := &cnode{nodeType: "block"}
		child := &cnode{nodeType: "stmt"}
		insertChild(parent, child, 0)
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
	code := []byte("package main\n\nfunc fn() {\n\ta()\n\tb()\n}\n")
	ast, err := treesitter.Parse(code, "main.go")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	var fnBlock *treesitter.ASTNode
	for _, n := range ast.Children {
		if n.Type == "function_declaration" {
			for _, child := range n.Children {
				if child.Type == "block" {
					fnBlock = child
				}
			}
		}
	}
	if fnBlock == nil || len(fnBlock.Children) < 2 {
		t.Fatalf("failed to find function block with statements")
	}
	stmtA := fnBlock.Children[0]
	stmtB := fnBlock.Children[1]

	s := &chawatheState{
		dstInOrder:  make(map[*treesitter.ASTNode]bool),
		srcInOrder:  make(map[*cnode]bool),
		cpyDstToSrc: make(map[*treesitter.ASTNode]*cnode),
	}
	targetContainer := &cnode{nodeType: "target_block"}

	t.Run("nil node", func(t *testing.T) {
		if pos := s.findPos(nil, targetContainer); pos != 0 {
			t.Errorf("expected findPos(nil) = 0, got %d", pos)
		}
	})

	t.Run("nil target container", func(t *testing.T) {
		if pos := s.findPos(stmtB, nil); pos != 0 {
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
		if pos := s.findPos(stmtB, targetContainer); pos != 0 {
			t.Errorf("expected findPos with unmapped partner = 0, got %d", pos)
		}
	})

	t.Run("foreign parent sibling prevention", func(t *testing.T) {
		foreignContainer := &cnode{nodeType: "foreign_block"}
		foreignChild := &cnode{nodeType: "id", parent: foreignContainer}
		foreignContainer.children = []*cnode{
			{nodeType: "first"},
			{nodeType: "second"},
			foreignChild,
		}

		s.dstInOrder[stmtA] = true
		s.srcInOrder[foreignChild] = true
		s.cpyDstToSrc[stmtA] = foreignChild

		pos := s.findPos(stmtB, targetContainer)
		if pos != 0 {
			t.Errorf("expected pos 0 due to foreign parent check, got %d (positional bleed occurred)", pos)
		}
	})

	t.Run("valid partner in target container", func(t *testing.T) {
		partnerChild := &cnode{nodeType: "id", parent: targetContainer}
		targetContainer.children = []*cnode{partnerChild}

		s.dstInOrder[stmtA] = true
		s.srcInOrder[partnerChild] = true
		s.cpyDstToSrc[stmtA] = partnerChild

		pos := s.findPos(stmtB, targetContainer)
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
	var fnNode *treesitter.ASTNode
	for _, n := range dstRoot.PostOrder() {
		if n.Type == "function_declaration" {
			fnNode = n
			break
		}
	}
	if fnNode == nil {
		t.Fatal("failed to find function_declaration in dstRoot")
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
	parent := &cnode{nodeType: "block"}
	c0 := &cnode{nodeType: "a"}
	c1 := &cnode{nodeType: "b"}
	c2 := &cnode{nodeType: "c"}

	insertChild(parent, c0, 0)
	insertChild(parent, c1, 1)
	insertChild(parent, c2, 2)

	if len(parent.children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(parent.children))
	}

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

	var dstChild *treesitter.ASTNode
	for _, n := range dst.Children {
		if n.Type == "function_definition" {
			dstChild = n
			break
		}
	}
	if dstChild == nil {
		t.Fatal("failed to find function_definition in dst")
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
	if w.nodeType != "function_definition" {
		t.Errorf("expected working node type 'function_definition', got %q", w.nodeType)
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
		root := &cnode{nodeType: "root"}
		c1 := &cnode{nodeType: "c1", parent: root}
		c2 := &cnode{nodeType: "c2", parent: root}
		c1Sub := &cnode{nodeType: "c1_sub", parent: c1}
		c1.children = []*cnode{c1Sub}
		root.children = []*cnode{c1, c2}

		nodes := root.PostOrder()
		if len(nodes) != 4 {
			t.Fatalf("expected 4 nodes, got %d", len(nodes))
		}
		if nodes[0] != c1Sub || nodes[1] != c1 || nodes[2] != c2 || nodes[3] != root {
			t.Errorf("unexpected postorder sequence: got [%s, %s, %s, %s]",
				nodes[0].nodeType, nodes[1].nodeType, nodes[2].nodeType, nodes[3].nodeType)
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

	var moveCount int
	var movedNode *treesitter.ASTNode
	for _, a := range script.Actions() {
		if a.Type == Move {
			moveCount++
			movedNode = a.Node
		}
	}

	if moveCount != 1 {
		t.Fatalf("expected 1 Move action, got %d", moveCount)
	}
	if movedNode == nil || (movedNode.Type != "assignment" && movedNode.Type != "expression_statement") {
		t.Errorf("expected Move action on assignment or expression_statement (resp.history), got %v", movedNode)
	}
}
