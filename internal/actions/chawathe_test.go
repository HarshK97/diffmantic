package actions

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

func TestUnorderedNodeMatching(t *testing.T) {
	// Create two container nodes with reordered child nodes
	childA1 := &treesitter.ASTNode{Type: "pair", Label: "a"}
	childB1 := &treesitter.ASTNode{Type: "pair", Label: "b"}
	src := &treesitter.ASTNode{
		Type:        "object",
		IsUnordered: true,
		Children:    []*treesitter.ASTNode{childA1, childB1},
	}
	childA1.Parent = src
	childB1.Parent = src

	childB2 := &treesitter.ASTNode{Type: "pair", Label: "b"}
	childA2 := &treesitter.ASTNode{Type: "pair", Label: "a"}
	dst := &treesitter.ASTNode{
		Type:        "object",
		IsUnordered: true,
		Children:    []*treesitter.ASTNode{childB2, childA2},
	}
	childB2.Parent = dst
	childA2.Parent = dst

	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(childA1, childA2)
	ms.Add(childB1, childB2)

	script := GenerateEditScript(src, dst, ms)
	for _, action := range script.Actions() {
		if action.Type == Move {
			t.Errorf("expected 0 Move actions for unordered container, got Move action on node %s", action.Node.Label)
		}
	}
}

func TestHTMLSelfClosingTagMatching(t *testing.T) {
	// Verify that <meta charset="utf-8"> vs <meta charset="utf-8"/> produces 0 actions
	src := []byte(`<meta charset="utf-8">`)
	dst := []byte(`<meta charset="utf-8"/>`)

	srcAST, err := treesitter.Parse(src, "index.html")
	if err != nil {
		t.Fatalf("failed to parse src: %v", err)
	}
	dstAST, err := treesitter.Parse(dst, "index.html")
	if err != nil {
		t.Fatalf("failed to parse dst: %v", err)
	}

	matchResult := engine.Match(srcAST, dstAST, src, dst, nil)
	script := GenerateEditScript(srcAST, dstAST, matchResult.Mappings)

	for _, a := range script.Actions() {
		if a.Type == Move {
			t.Errorf("expected 0 Move actions for void self-closing tag conversion, got: %s on node %s", a.Type, a.Node.Type)
		}
	}

	if script.Size() != 0 {
		t.Errorf("expected 0 actions for semantic void tag conversion, got %d actions", script.Size())
		for _, a := range script.Actions() {
			t.Logf("unexpected action: %s on node %s (%s)", a.Type, a.Node.Type, a.Node.Label)
		}
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
		deleteChild(nil, child)
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
		node := &treesitter.ASTNode{Type: "id", Parent: &treesitter.ASTNode{Type: "block"}}
		if pos := s.findPos(node, nil); pos != 0 {
			t.Errorf("expected findPos with nil target = 0, got %d", pos)
		}
	})

	t.Run("orphan node", func(t *testing.T) {
		orphan := &treesitter.ASTNode{Type: "id"}
		if pos := s.findPos(orphan, targetContainer); pos != 0 {
			t.Errorf("expected findPos(orphan) = 0, got %d", pos)
		}
	})

	t.Run("detached node", func(t *testing.T) {
		parent := &treesitter.ASTNode{Type: "block"}
		detached := &treesitter.ASTNode{Type: "id", Parent: parent}
		if pos := s.findPos(detached, targetContainer); pos != 0 {
			t.Errorf("expected findPos(detached) = 0, got %d", pos)
		}
	})

	t.Run("unmapped partner sibling", func(t *testing.T) {
		parent := &treesitter.ASTNode{Type: "block"}
		sibling := &treesitter.ASTNode{Type: "id", Parent: parent}
		target := &treesitter.ASTNode{Type: "id", Parent: parent}
		parent.Children = []*treesitter.ASTNode{sibling, target}
		s.dstInOrder[sibling] = true
		if pos := s.findPos(target, targetContainer); pos != 0 {
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

		parent := &treesitter.ASTNode{Type: "block"}
		sibling := &treesitter.ASTNode{Type: "id", Parent: parent}
		target := &treesitter.ASTNode{Type: "id", Parent: parent}
		parent.Children = []*treesitter.ASTNode{sibling, target}

		s.dstInOrder[sibling] = true
		s.srcInOrder[foreignChild] = true
		s.cpyDstToSrc[sibling] = foreignChild

		pos := s.findPos(target, targetContainer)
		if pos != 0 {
			t.Errorf("expected pos 0 due to foreign parent check, got %d (positional bleed occurred)", pos)
		}
	})

	t.Run("valid partner in target container", func(t *testing.T) {
		partnerChild := &cnode{nodeType: "id", parent: targetContainer}
		targetContainer.children = []*cnode{partnerChild}

		parent := &treesitter.ASTNode{Type: "block"}
		sibling := &treesitter.ASTNode{Type: "id", Parent: parent}
		target := &treesitter.ASTNode{Type: "id", Parent: parent}
		parent.Children = []*treesitter.ASTNode{sibling, target}

		s.dstInOrder[sibling] = true
		s.srcInOrder[partnerChild] = true
		s.cpyDstToSrc[sibling] = partnerChild

		pos := s.findPos(target, targetContainer)
		if pos != 1 {
			t.Errorf("expected pos 1, got %d", pos)
		}
	})
}

func TestNonRootProvenanceFallback(t *testing.T) {
	dstRoot := &treesitter.ASTNode{Type: "module", Label: "root"}
	dstContainer := &treesitter.ASTNode{Type: "function", Label: "fn", Parent: dstRoot}
	dstChild := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: dstContainer}
	dstContainer.Children = []*treesitter.ASTNode{dstChild}
	dstRoot.Children = []*treesitter.ASTNode{dstContainer}

	script := GenerateEditScript(nil, dstRoot, nil)
	if script == nil {
		t.Fatalf("expected non-nil script")
	}

	actions := script.Actions()
	for _, a := range actions {
		if a.Node == dstRoot {
			if a.Parent != nil {
				t.Errorf("expected root Insert to have Parent nil (source realm purity), got %v", a.Parent)
			}
		}
		if a.Node == dstContainer {
			if a.Parent != dstRoot {
				t.Errorf("expected dstContainer Insert to have Parent dstRoot, got %v", a.Parent)
			}
		}
		if a.Node == dstChild {
			if a.Parent != dstContainer {
				t.Errorf("expected dstChild Insert to have Parent dstContainer, got %v", a.Parent)
			}
		}
	}

	// Verify unmapped synthetic parent evaluates strictly to nil (pointer domain purity)
	ghostParent := &treesitter.ASTNode{Type: "ghost"}
	orphanChild := &treesitter.ASTNode{Type: "identifier", Label: "orphan", Parent: ghostParent}
	treeWithOrphan := &treesitter.ASTNode{
		Type:     "module",
		Label:    "root2",
		Children: []*treesitter.ASTNode{orphanChild},
	}
	scriptOrphan := GenerateEditScript(nil, treeWithOrphan, nil)
	for _, a := range scriptOrphan.Actions() {
		if a.Node == orphanChild {
			if a.Parent != nil {
				t.Errorf("expected orphan child with unmapped ghost parent to have nil Parent, got %v", a.Parent)
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
	src := &treesitter.ASTNode{Type: "module", Label: "root"}
	dstChild := &treesitter.ASTNode{Type: "function_def", Label: "my_func"}
	dst := &treesitter.ASTNode{
		Type:     "module",
		Label:    "root",
		Children: []*treesitter.ASTNode{dstChild},
	}
	dstChild.Parent = dst

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
	if w.nodeType != "function_def" {
		t.Errorf("expected working node type 'function_def', got %q", w.nodeType)
	}
	if w.label != "my_func" {
		t.Errorf("expected working node label 'my_func', got %q", w.label)
	}
	if w.orig != dstChild {
		t.Errorf("expected working node orig to point to dstChild, got %v", w.orig)
	}
}

func TestLCSTieBreakingScore(t *testing.T) {
	s := &chawatheState{
		cpyDstToSrc: make(map[*treesitter.ASTNode]*cnode),
	}

	parentX := &cnode{nodeType: "block"}
	parentY := &treesitter.ASTNode{Type: "block"}

	x0 := &cnode{nodeType: "item", parent: parentX}
	x1 := &cnode{nodeType: "item", parent: parentX}
	parentX.children = []*cnode{x0, x1}

	y0 := &treesitter.ASTNode{Type: "item", Parent: parentY}
	parentY.Children = []*treesitter.ASTNode{y0}

	s.cpyDstToSrc[y0] = x0

	pairs := s.lcs([]*cnode{x0, x1}, []*treesitter.ASTNode{y0})
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
		dst := &treesitter.ASTNode{Type: "module", Label: "dst"}
		dstChild := &treesitter.ASTNode{Type: "identifier", Label: "x", Parent: dst}
		dst.Children = []*treesitter.ASTNode{dstChild}

		esInsert := GenerateEditScript(nil, dst, nil)
		if esInsert == nil || esInsert.Size() != 2 {
			t.Fatalf("expected 2 inserts for nil src, got %d actions", esInsert.Size())
		}
		for _, a := range esInsert.Actions() {
			if a.Type != Insert {
				t.Errorf("expected Insert action, got %v", a.Type)
			}
		}
	})

	t.Run("src non-nil dst nil", func(t *testing.T) {
		src := &treesitter.ASTNode{Type: "module", Label: "src"}
		srcChild := &treesitter.ASTNode{Type: "identifier", Label: "y", Parent: src}
		src.Children = []*treesitter.ASTNode{srcChild}

		esDelete := GenerateEditScript(src, nil, nil)
		if esDelete == nil || esDelete.Size() != 2 {
			t.Fatalf("expected 2 deletes for nil dst, got %d actions", esDelete.Size())
		}
		for _, a := range esDelete.Actions() {
			if a.Type != Delete {
				t.Errorf("expected Delete action, got %v", a.Type)
			}
		}
	})

	t.Run("detached malformed dst node", func(t *testing.T) {
		src := &treesitter.ASTNode{Type: "module", Label: "src"}
		detachedDst := &treesitter.ASTNode{Type: "module"}
		unlinkedChild := &treesitter.ASTNode{Type: "error_node", Parent: &treesitter.ASTNode{Type: "ghost_parent"}}
		detachedDst.Children = []*treesitter.ASTNode{unlinkedChild}

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
