package serialize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// setParentAndRange links a node to its parent and sets its byte range.
func setParentAndRange(node, parent *treesitter.ASTNode, startByte, endByte uint32) {
	node.Parent = parent
	node.StartByte = startByte
	node.EndByte = endByte
	if parent != nil {
		parent.Children = append(parent.Children, node)
	}
}

func TestRoundTrip(t *testing.T) {
	// Let's build a source (before) tree:
	// module
	//  ├── function (foo)
	//  │    ├── identifier (x)
	//  │    └── identifier (y)
	//  └── function (bar)
	src := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(src, nil, 0, 100)

	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(srcFoo, src, 10, 50)

	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(srcX, srcFoo, 15, 20)

	srcY := &treesitter.ASTNode{Type: "identifier", Label: "y"}
	setParentAndRange(srcY, srcFoo, 25, 30)

	srcBar := &treesitter.ASTNode{Type: "function", Label: "bar"}
	setParentAndRange(srcBar, src, 60, 90)

	// Let's build a destination (after) tree:
	// module
	//  ├── function (foo)
	//  │    └── identifier (x)  <-- y deleted
	//  └── function (bar)
	//       ├── identifier (z)  <-- z inserted
	//       └── identifier (x)  <-- x moved here from foo
	dst := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(dst, nil, 0, 100)

	dstFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(dstFoo, dst, 10, 40)

	dstX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(dstX, dstFoo, 15, 20)

	dstBar := &treesitter.ASTNode{Type: "function", Label: "bar"}
	setParentAndRange(dstBar, dst, 50, 95)

	dstZ := &treesitter.ASTNode{Type: "identifier", Label: "z"}
	setParentAndRange(dstZ, dstBar, 55, 60)

	dstMovedX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(dstMovedX, dstBar, 65, 70)

	// Create Mappings:
	ms := engine.NewMapping()
	ms.Add(src, dst)
	ms.Add(srcFoo, dstFoo)
	ms.Add(srcBar, dstBar)
	ms.Add(srcX, dstMovedX) // Move mapping: srcX -> dstMovedX

	// Generate some simulated actions
	es := actions.NewEditScript()

	// 1. Insert action (insert dstZ under dstBar at position 0)
	es.Add(actions.Action{
		Type:     actions.Insert,
		Node:     dstZ,
		Parent:   srcBar, // parent in src tree
		Position: 0,
		Subtree:  false,
	})

	// 2. Delete action (delete srcY)
	es.Add(actions.Action{
		Type:    actions.Delete,
		Node:    srcY,
		Subtree: true, // test subtree delete
	})

	// 3. Update action (update srcBar from "bar" to "new_bar")
	es.Add(actions.Action{
		Type:  actions.Update,
		Node:  srcBar,
		Value: "new_bar",
	})

	// 4. Move action (move srcX to dstBar at position 1)
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcX,
		Parent:   srcBar, // new parent in src tree
		Position: 1,
		Subtree:  false,
	})

	opts := EnvelopeOptions{IncludeActions: true, IncludeAlignment: true, IncludeHighlights: true}
	jsonData, err := MarshalWithOptions(es, ms, src, dst, make([]byte, src.EndByte), make([]byte, dst.EndByte), opts)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	t.Logf("Generated JSON:\n%s", string(jsonData))

	// Verify JSON structure and options:
	var parsed map[string]any
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("Unmarshal to map failed: %v", err)
	}

	if parsed["version"] != SchemaVersion {
		t.Errorf("expected version %q, got %v", SchemaVersion, parsed["version"])
	}

	acts, ok := parsed["actions"].([]any)
	if !ok {
		t.Fatalf("actions field missing or not an array")
	}

	if len(acts) != 4 {
		t.Fatalf("expected 4 actions, got %d", len(acts))
	}

	// 1. Check Insert action fields and tree tags
	act1 := acts[0].(map[string]any)
	if act1["action"] != "insert" {
		t.Errorf("expected first action to be insert, got %v", act1["action"])
	}
	node1 := act1["node"].(map[string]any)
	if node1["tree"] != "after" {
		t.Errorf("insert node should be 'after', got %v", node1["tree"])
	}
	parent1 := act1["parent"].(map[string]any)
	if parent1["tree"] != "after" {
		t.Errorf("insert parent should be 'after', got %v", parent1["tree"])
	}
	if act1["subtree"] != nil {
		t.Errorf("insert subtree=false should be omitted, got %v", act1["subtree"])
	}

	// 2. Check Delete action fields, tree tags and subtree
	act2 := acts[1].(map[string]any)
	if act2["action"] != "delete" {
		t.Errorf("expected second action to be delete, got %v", act2["action"])
	}
	node2 := act2["node"].(map[string]any)
	if node2["tree"] != "before" {
		t.Errorf("delete node should be 'before', got %v", node2["tree"])
	}
	if act2["subtree"] != true {
		t.Errorf("delete subtree should be true, got %v", act2["subtree"])
	}

	// 3. Check Update action fields and subtree absence
	act3 := acts[2].(map[string]any)
	if act3["action"] != "update" {
		t.Errorf("expected third action to be update, got %v", act3["action"])
	}
	if act3["subtree"] != nil {
		t.Errorf("update should never contain subtree field, got %v", act3["subtree"])
	}
	if act3["old_value"] != "bar" || act3["new_value"] != "new_bar" {
		t.Errorf("update values mismatch: old=%v new=%v", act3["old_value"], act3["new_value"])
	}

	// 4. Check Move action fields, tree tags
	act4 := acts[3].(map[string]any)
	if act4["action"] != "move" {
		t.Errorf("expected fourth action to be move, got %v", act4["action"])
	}
	node4 := act4["node"].(map[string]any)
	if node4["tree"] != "before" {
		t.Errorf("move node should be 'before', got %v", node4["tree"])
	}
	parent4 := act4["parent"].(map[string]any)
	if parent4["tree"] != "after" {
		t.Errorf("move parent should be 'after', got %v", parent4["tree"])
	}
	oldParent4 := act4["old_parent"].(map[string]any)
	if oldParent4["tree"] != "before" {
		t.Errorf("move old_parent should be 'before', got %v", oldParent4["tree"])
	}
	if int(act4["old_position"].(float64)) != 0 {
		t.Errorf("move old_position should be 0, got %v", act4["old_position"])
	}
	if int(act4["position"].(float64)) != 1 {
		t.Errorf("move position should be 1, got %v", act4["position"])
	}
}

func TestNilEditScript(t *testing.T) {
	opts := EnvelopeOptions{IncludeActions: true}
	_, err := MarshalWithOptions(nil, nil, nil, nil, nil, nil, opts)
	if err == nil || !strings.Contains(err.Error(), "edit script is nil") {
		t.Errorf("expected error for nil edit script, got %v", err)
	}
}

func TestMarshalErrorUnmappedMoveParent(t *testing.T) {
	src := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(src, nil, 0, 100)

	srcFoo := &treesitter.ASTNode{Type: "function", Label: "foo"}
	setParentAndRange(srcFoo, src, 10, 50)

	srcX := &treesitter.ASTNode{Type: "identifier", Label: "x"}
	setParentAndRange(srcX, srcFoo, 15, 20)

	dst := &treesitter.ASTNode{Type: "module"}
	setParentAndRange(dst, nil, 0, 100)

	// Create an empty mapping
	ms := engine.NewMapping()

	es := actions.NewEditScript()
	// Move action where the new parent srcFoo is NOT mapped in ms.
	es.Add(actions.Action{
		Type:     actions.Move,
		Node:     srcX,
		Parent:   srcFoo,
		Position: 0,
	})

	opts := EnvelopeOptions{IncludeActions: true}
	_, err := MarshalWithOptions(es, ms, src, dst, make([]byte, src.EndByte), make([]byte, dst.EndByte), opts)
	if err == nil {
		t.Fatal("expected Marshal to fail due to unmapped move parent, but it succeeded")
	}
	if !strings.Contains(err.Error(), "failed to resolve move parent") {
		t.Errorf("expected error message to mention 'failed to resolve move parent', got: %v", err)
	}
}

func TestBuildLineDiffEnvelopeWithOptions_ModifiedLines(t *testing.T) {
	// Fallback line diff should emit a delete on the left and insert on the right for edited lines.
	srcBytes := []byte("func main() {\n\tprintln(\"old\")\n}\n")
	dstBytes := []byte("func main() {\n\tprintln(\"new\")\n}\n")

	opts := EnvelopeOptions{
		IncludeActions:    true,
		IncludeAlignment:  true,
		IncludeHighlights: true,
	}
	env := BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, opts)
	if env == nil {
		t.Fatal("expected non-nil envelope")
	}

	if len(env.Actions) != 2 {
		t.Fatalf("expected 2 actions (delete + insert), got %d: %+v", len(env.Actions), env.Actions)
	}

	if env.Actions[0].Action != "delete" || env.Actions[0].Node.Tree != "before" {
		t.Errorf("expected delete action on before tree, got %+v", env.Actions[0])
	}
	if env.Actions[1].Action != "insert" || env.Actions[1].Node.Tree != "after" {
		t.Errorf("expected insert action on after tree, got %+v", env.Actions[1])
	}

	if len(env.LeftHighlights) == 0 {
		t.Error("expected left highlights for deleted line")
	}
	if len(env.RightHighlights) == 0 {
		t.Error("expected right highlights for inserted line")
	}
}

func TestAdjustRangeForContainerDeclarationChild(t *testing.T) {
	kw := &treesitter.ASTNode{Type: "friend", StartByte: 10, EndByte: 16, StartRow: 1, EndRow: 1}
	fn := &treesitter.ASTNode{Type: "function_definition", StartByte: 17, EndByte: 80, StartRow: 1, EndRow: 4}
	container := &treesitter.ASTNode{
		Type: "friend_declaration", StartByte: 10, EndByte: 80, StartRow: 1, EndRow: 4,
		Children: []*treesitter.ASTNode{kw, fn},
	}
	container.Language = "cpp"
	kw.Parent = container
	fn.Parent = container

	start := container.StartByte
	end := container.EndByte

	adjustRangeForContainer(container, &start, &end, nil)

	if start != 10 || end != 17 {
		t.Errorf("expected container range sliced to [10, 17], got [%d, %d]", start, end)
	}
}

func TestAdjustRangeForContainerParenWrapper(t *testing.T) {
	inner := &treesitter.ASTNode{Type: "identifier", StartByte: 1, EndByte: 9, StartRow: 1, EndRow: 1}
	paren := &treesitter.ASTNode{
		Type:      "parenthesized_expression",
		StartByte: 0,
		EndByte:   10,
		StartRow:  1,
		EndRow:    1,
		Children:  []*treesitter.ASTNode{inner},
	}
	paren.Language = "cpp"
	inner.Parent = paren

	start := paren.StartByte
	end := paren.EndByte

	code := []byte("(12345678)")
	hasFooter, fStart, fEnd := adjustRangeForContainer(paren, &start, &end, code)
	if !hasFooter {
		t.Fatal("expected paren wrapper to report footer")
	}
	if start != 0 || end != 1 {
		t.Errorf("expected header range [0, 1], got [%d, %d]", start, end)
	}
	if fStart != 9 || fEnd != 10 {
		t.Errorf("expected footer range [9, 10], got [%d, %d]", fStart, fEnd)
	}
}

func TestAdjustRangeForContainerImplicitLengthArrayType(t *testing.T) {
	dots := &treesitter.ASTNode{Type: "...", StartByte: 1, EndByte: 4, StartRow: 1, EndRow: 1}
	typeIdent := &treesitter.ASTNode{Type: "type_identifier", StartByte: 4, EndByte: 10, StartRow: 1, EndRow: 1}
	arr := &treesitter.ASTNode{
		Type:      "implicit_length_array_type",
		StartByte: 0,
		EndByte:   10,
		StartRow:  1,
		EndRow:    1,
		Children:  []*treesitter.ASTNode{dots, typeIdent},
	}
	arr.Language = "go"
	dots.Parent = arr
	typeIdent.Parent = arr

	start := arr.StartByte
	end := arr.EndByte

	hasFooter, fStart, fEnd := adjustRangeForContainer(arr, &start, &end, []byte("[...]string"))
	if hasFooter {
		t.Errorf("expected no footer delimiter for implicit length array, got [%d, %d]", fStart, fEnd)
	}
	if start != 0 || end != 1 {
		t.Errorf("expected container header range [0, 1] for '[', got [%d, %d]", start, end)
	}
}

func TestASTActionPurityAndVisualDelimiterHighlights(t *testing.T) {
	// A block container with an inner statement and closing brace:
	// line 0: {
	// line 1:     x = 1
	// line 2: }
	srcBytes := []byte("{\n    x = 1\n}\n")
	dstBytes := []byte("{\n    x = 1\n}\n")

	root := &treesitter.ASTNode{Type: "source_file"}
	setParentAndRange(root, nil, 0, uint32(len(dstBytes)))

	blockNode := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 2}
	setParentAndRange(blockNode, root, 0, uint32(len(dstBytes)-1))

	innerStmt := &treesitter.ASTNode{Type: "assignment", StartRow: 1, EndRow: 1}
	setParentAndRange(innerStmt, blockNode, 2, 11) // "x = 1\n"

	es := actions.NewEditScript()
	es.Add(actions.Action{
		Type:    actions.Insert,
		Node:    blockNode,
		Subtree: false, // non-subtree container insertion
	})

	opts := EnvelopeOptions{
		IncludeActions:    true,
		IncludeHighlights: true,
	}
	env, err := BuildEnvelopeWithOptions(es, nil, nil, root, srcBytes, dstBytes, opts)
	if err != nil {
		t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
	}

	// Verify we only get the original AST action, not a duplicate footer action.
	if len(env.Actions) != 1 {
		t.Fatalf("expected strictly 1 action in env.Actions, got %d: %+v", len(env.Actions), env.Actions)
	}
	if env.Actions[0].Action != "insert" || env.Actions[0].Node.Type != "block" {
		t.Errorf("expected 1 insert action on block, got %+v", env.Actions[0])
	}
	if env.Actions[0].Node.StartByte != 0 || env.Actions[0].Node.EndByte != 2 {
		t.Errorf("expected header range 0..2, got %d..%d", env.Actions[0].Node.StartByte, env.Actions[0].Node.EndByte)
	}

	// Check that the closing brace '}' still gets highlighted on line 2.
	var foundClosingBrace bool
	for _, hl := range env.RightHighlights {
		if hl.Line == 2 && hl.Action == "insert" {
			foundClosingBrace = true
		}
	}
	if !foundClosingBrace {
		t.Errorf("expected closing brace highlight span on line 2 in RightHighlights, got %+v", env.RightHighlights)
	}
}

func TestIndentationAndCommentFooterExclusion(t *testing.T) {
	// Python block has no closing delimiter (indentation based)
	// def foo():
	//     pass
	// \n
	pyCode := []byte("def foo():\n    pass\n\n")
	root := &treesitter.ASTNode{Type: "module", Language: "python"}
	setParentAndRange(root, nil, 0, uint32(len(pyCode)))

	funcDef := &treesitter.ASTNode{Type: "function_definition", StartRow: 0, EndRow: 1}
	setParentAndRange(funcDef, root, 0, uint32(len(pyCode)))

	body := &treesitter.ASTNode{Type: "block", StartRow: 1, EndRow: 1}
	setParentAndRange(body, funcDef, 11, 19)

	start := funcDef.StartByte
	end := funcDef.EndByte
	hasFooter, _, _ := adjustRangeForContainer(funcDef, &start, &end, pyCode)
	if hasFooter {
		t.Errorf("expected hasFooter=false for Python function_definition indentation block")
	}

	// Trailing comments should not be treated as closing delimiter
	codeWithComment := []byte("{\n    x = 1\n    // trailing comment\n")
	cRoot := &treesitter.ASTNode{Type: "source_file", Language: "go"}
	setParentAndRange(cRoot, nil, 0, uint32(len(codeWithComment)))

	bNode := &treesitter.ASTNode{Type: "block", StartRow: 0, EndRow: 2}
	setParentAndRange(bNode, cRoot, 0, uint32(len(codeWithComment)))

	stmt := &treesitter.ASTNode{Type: "assignment", StartRow: 1, EndRow: 1}
	setParentAndRange(stmt, bNode, 2, 11)

	bStart := bNode.StartByte
	bEnd := bNode.EndByte
	hasCommentFooter, _, _ := adjustRangeForContainer(bNode, &bStart, &bEnd, codeWithComment)
	if hasCommentFooter {
		t.Errorf("expected hasCommentFooter=false for sliced region containing comments")
	}

	// Python subscripts use bracket delimiters ([ ]), so they need closing footers.
	pySubscriptCode := []byte("cookie_dict[cookie.name]")
	pySubRoot := &treesitter.ASTNode{Type: "module", Language: "python"}
	setParentAndRange(pySubRoot, nil, 0, uint32(len(pySubscriptCode)))

	subscript := &treesitter.ASTNode{Type: "subscript", StartRow: 0, EndRow: 0}
	setParentAndRange(subscript, pySubRoot, 0, uint32(len(pySubscriptCode)))

	receiver := &treesitter.ASTNode{Type: "identifier", StartRow: 0, EndRow: 0}
	setParentAndRange(receiver, subscript, 0, 11)

	index := &treesitter.ASTNode{Type: "attribute", StartRow: 0, EndRow: 0}
	setParentAndRange(index, subscript, 12, 23)

	subStart := subscript.StartByte
	subEnd := subscript.EndByte
	hasSubFooter, fStart, fEnd := adjustRangeForContainer(subscript, &subStart, &subEnd, pySubscriptCode)
	if !hasSubFooter {
		t.Errorf("expected hasSubFooter=true for Python subscript with closing bracket")
	}
	if fStart != 23 || fEnd != 24 || string(pySubscriptCode[fStart:fEnd]) != "]" {
		t.Errorf("expected subscript closing delimiter footer at [23:24] (']'), got [%d:%d] ('%s')", fStart, fEnd, string(pySubscriptCode[fStart:fEnd]))
	}
	if subStart != 11 || subEnd != 12 || string(pySubscriptCode[subStart:subEnd]) != "[" {
		t.Errorf("expected subscript opening delimiter at [11:12] ('['), got [%d:%d] ('%s')", subStart, subEnd, string(pySubscriptCode[subStart:subEnd]))
	}
}

func TestAssignMoveColors(t *testing.T) {
	srcOffsets := []int{0, 20, 40, 60, 80, 100, 120}
	dstOffsets := []int{0, 20, 40, 60, 80, 100, 120}

	t.Run("single move", func(t *testing.T) {
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 5, EndByte: 15},
				DestStartByte: ptr(uint32(25)),
				DestEndByte:   ptr(uint32(35)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 {
			t.Errorf("expected single move to have MoveColorIndex 0, got %d", actions[0].MoveColorIndex)
		}
	})

	t.Run("intertwined moves", func(t *testing.T) {
		// Move 0: outer statement lines 0-3 (bytes 0-60) -> lines 0-3 (bytes 0-60)
		// Move 1: old branch 1 lines 1-2 (bytes 20-40) -> branch 2 lines 2-3 (bytes 40-60)
		// Move 2: old branch 2 lines 2-3 (bytes 40-60) -> branch 1 lines 1-2 (bytes 20-40)
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 60},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(60)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 20, EndByte: 40},
				DestStartByte: ptr(uint32(40)),
				DestEndByte:   ptr(uint32(60)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 40, EndByte: 60},
				DestStartByte: ptr(uint32(20)),
				DestEndByte:   ptr(uint32(40)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 {
			t.Errorf("expected move 0 to have MoveColorIndex 0, got %d", actions[0].MoveColorIndex)
		}
		if actions[1].MoveColorIndex != 1 {
			t.Errorf("expected move 1 to have MoveColorIndex 1 (Magenta), got %d", actions[1].MoveColorIndex)
		}
		if actions[2].MoveColorIndex != 0 {
			t.Errorf("expected move 2 to wrap to MoveColorIndex 0 (modulo 2), got %d", actions[2].MoveColorIndex)
		}
	})

	t.Run("pairwise swap", func(t *testing.T) {
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(15)),
				DestEndByte:   ptr(uint32(25)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 15, EndByte: 25},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(10)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 {
			t.Errorf("expected swap move 0 to have MoveColorIndex 0 (Cyan), got %d", actions[0].MoveColorIndex)
		}
		if actions[1].MoveColorIndex != 1 {
			t.Errorf("expected swap move 1 to have MoveColorIndex 1 (Magenta), got %d", actions[1].MoveColorIndex)
		}
	})

	t.Run("distant non-overlapping moves", func(t *testing.T) {
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(10)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 100, EndByte: 110},
				DestStartByte: ptr(uint32(100)),
				DestEndByte:   ptr(uint32(110)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 0 {
			t.Errorf("expected distant moves to both have MoveColorIndex 0, got %d and %d", actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("hierarchical co-movers", func(t *testing.T) {
		actions := []Action{
			{
				Action:   "move",
				Node:     &NodeRef{StartByte: 0, EndByte: 15, Type: "if_statement"},
				DestNode: &NodeRef{StartByte: 50, EndByte: 65, Type: "if_statement"},
			},
			{
				Action:    "move",
				Node:      &NodeRef{StartByte: 16, EndByte: 40, Type: "block"},
				OldParent: &NodeRef{StartByte: 0, Type: "if_statement"},
				Parent:    &NodeRef{StartByte: 50, Type: "if_statement"},
				DestNode:  &NodeRef{StartByte: 66, EndByte: 90, Type: "block"},
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 0 {
			t.Errorf("expected hierarchical co-movers to share MoveColorIndex 0, got %d and %d", actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("diverging moves from nearby source to distant destinations", func(t *testing.T) {
		// Move 0: src line 0 (bytes 0-10) -> dst line 0 (bytes 0-10)
		// Move 1: src line 1 (bytes 20-30) -> dst line 5 (bytes 100-110)
		// Starts within 2 lines in the source, but lands far apart in the destination.
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(10)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 20, EndByte: 30},
				DestStartByte: ptr(uint32(100)),
				DestEndByte:   ptr(uint32(110)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 1 {
			t.Errorf("expected diverging moves to get distinct colors 0 and 1, got %d and %d", actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("converging moves from distant sources to nearby destination", func(t *testing.T) {
		// Move 0: src line 0 (bytes 0-10) -> dst line 3 (bytes 60-70)
		// Move 1: src line 5 (bytes 100-110) -> dst line 5 (bytes 100-110)
		// Starts far apart in the source, but lands within 2 lines in the destination.
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(60)),
				DestEndByte:   ptr(uint32(70)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 100, EndByte: 110},
				DestStartByte: ptr(uint32(100)),
				DestEndByte:   ptr(uint32(110)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 1 {
			t.Errorf("expected converging moves landing within 2 lines to get distinct colors 0 and 1, got %d and %d", actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("grouped co-movers share color despite proximity", func(t *testing.T) {
		// Grouped moves should share a color even if they land next to each other.
		actions := []Action{
			{
				Action:        "move",
				GroupID:       "group-1",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(60)),
				DestEndByte:   ptr(uint32(70)),
			},
			{
				Action:        "move",
				GroupID:       "group-1",
				Node:          &NodeRef{StartByte: 20, EndByte: 30},
				DestStartByte: ptr(uint32(80)),
				DestEndByte:   ptr(uint32(90)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 0 {
			t.Errorf("expected grouped moves to share MoveColorIndex 0, got %d and %d", actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("child co-mover under equivalent parent type inherits parent color", func(t *testing.T) {
		parentSrc := &treesitter.ASTNode{Type: "assignment_statement", StartByte: 20, EndByte: 35, Language: "go"}
		childSrc := &treesitter.ASTNode{Type: "expression_list", StartByte: 20, EndByte: 29, Parent: parentSrc, Language: "go"}
		parentSrc.Children = []*treesitter.ASTNode{childSrc}

		parentDst := &treesitter.ASTNode{Type: "short_var_declaration", StartByte: 80, EndByte: 95, Language: "go"}
		childDst := &treesitter.ASTNode{Type: "expression_list", StartByte: 80, EndByte: 89, Parent: parentDst, Language: "go"}
		parentDst.Children = []*treesitter.ASTNode{childDst}

		// Move 0 and Move 1 conflict on adjacent lines, so Move 1 gets color 1.
		// Move 2 is a child of Move 1 (with only DestStartByte set, relying on EquivalentTypes matching),
		// so it should inherit Move 1's color.
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(10)),
			},
			{
				Action:        "move",
				ASTNode:       parentSrc,
				Node:          &NodeRef{Type: "assignment_statement", StartByte: 20, EndByte: 35},
				DestStartByte: ptr(uint32(80)),
				DestEndByte:   ptr(uint32(95)),
			},
			{
				Action:        "move",
				ASTNode:       childSrc,
				DestASTNode:   childDst,
				Node:          &NodeRef{Type: "expression_list", StartByte: 20, EndByte: 29},
				OldParent:     &NodeRef{Type: "assignment_statement", StartByte: 20, EndByte: 35},
				Parent:        &NodeRef{Type: "short_var_declaration", StartByte: 80, EndByte: 95},
				DestStartByte: ptr(uint32(80)),
				DestEndByte:   ptr(uint32(89)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[1].MoveColorIndex != 1 {
			t.Fatalf("expected parent move to get color 1, got %d", actions[1].MoveColorIndex)
		}
		if actions[2].MoveColorIndex != actions[1].MoveColorIndex {
			t.Errorf("expected child co-mover across EquivalentTypes to inherit parent color %d, got %d", actions[1].MoveColorIndex, actions[2].MoveColorIndex)
		}
	})
}

func TestAssignMoveColors_EndToEnd(t *testing.T) {
	runPipeline := func(t *testing.T, oldSrc, newSrc string) *Envelope {
		t.Helper()
		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}
		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeActions: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}
		return env
	}

	collectMoves := func(env *Envelope) []Action {
		var moves []Action
		for _, a := range env.Actions {
			if a.Action == "move" || a.Action == "move_update" {
				moves = append(moves, a)
			}
		}
		return moves
	}

	t.Run("grouped co-movers share color despite proximity", func(t *testing.T) {
		oldSrc := `package main

func process() {
	keep1()
	moveA()
	moveB()
	keep2()
}
`
		newSrc := `package main

func process() {
	keep1()
	keep2()
	if true {
		moveA()
		moveB()
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)
		moves := collectMoves(env)
		if len(moves) != 2 {
			t.Fatalf("expected 2 moves, got %d: %+v", len(moves), moves)
		}
		if moves[0].GroupID == "" || moves[0].GroupID != moves[1].GroupID {
			t.Errorf("expected moves to share a non-empty GroupID, got %q and %q", moves[0].GroupID, moves[1].GroupID)
		}
		if moves[0].MoveColorIndex != 0 || moves[1].MoveColorIndex != 0 {
			t.Errorf("expected grouped co-movers to share MoveColorIndex 0, got %d and %d", moves[0].MoveColorIndex, moves[1].MoveColorIndex)
		}
	})

	t.Run("diverging moves from nearby source get distinct colors", func(t *testing.T) {
		oldSrc := `package main

func process() {
	moveA()
	moveB()
	keep1()
	keep2()
	keep3()
	keep4()
}
`
		newSrc := `package main

func process() {
	keep1()
	if cond1 {
		moveA()
	}
	keep2()
	keep3()
	keep4()
	if cond2 {
		moveB()
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)
		moves := collectMoves(env)
		if len(moves) != 2 {
			t.Fatalf("expected 2 moves, got %d: %+v", len(moves), moves)
		}
		if moves[0].MoveColorIndex != 0 || moves[1].MoveColorIndex != 1 {
			t.Errorf("expected diverging moves to get distinct colors 0 and 1, got %d and %d", moves[0].MoveColorIndex, moves[1].MoveColorIndex)
		}
	})

	t.Run("converging moves to nearby destination get distinct colors", func(t *testing.T) {
		oldSrc := `package main

func process() {
	if cond1 {
		moveA()
	}
	keep1()
	keep2()
	keep3()
	keep4()
	if cond2 {
		moveB()
	}
}
`
		newSrc := `package main

func process() {
	keep1()
	keep2()
	keep3()
	keep4()
	if condTarget {
		moveA()
	}
	moveB()
}
`
		env := runPipeline(t, oldSrc, newSrc)
		moves := collectMoves(env)
		if len(moves) != 2 {
			t.Fatalf("expected 2 moves, got %d: %+v", len(moves), moves)
		}
		if moves[0].MoveColorIndex != 0 || moves[1].MoveColorIndex != 1 {
			t.Errorf("expected converging moves to get distinct colors 0 and 1, got %d and %d", moves[0].MoveColorIndex, moves[1].MoveColorIndex)
		}
	})
}

func TestAdjustRangeForContainer_LeadingTrivia(t *testing.T) {
	fileBytes := []byte("{\n    // comment\n    if (x) {}\n}")
	block := &treesitter.ASTNode{
		Type:      "block",
		Language:  "java",
		StartByte: 0,
		EndByte:   32,
		StartRow:  0,
		EndRow:    3,
	}
	stmt := &treesitter.ASTNode{
		Type:      "if_statement",
		StartByte: 17,
		EndByte:   30,
		StartRow:  2,
		EndRow:    2,
		Parent:    block,
		LeadingTrivia: []*treesitter.CommentBlock{
			{
				StartByte: 6,
				EndByte:   16,
				StartRow:  1,
				EndRow:    1,
			},
		},
	}
	block.Children = []*treesitter.ASTNode{stmt}

	start := uint32(0)
	end := uint32(32)
	hasFooter, fStart, fEnd := adjustRangeForContainer(block, &start, &end, fileBytes)

	if end != 6 {
		t.Errorf("expected container prologue end to be clamped to 6 (start of comment), got %d", end)
	}
	if !hasFooter {
		t.Errorf("expected container to have footer for closing brace")
	}
	if fStart != 30 || fEnd != 32 {
		t.Errorf("expected footer [30, 32], got [%d, %d]", fStart, fEnd)
	}
}

func TestAdjustRangeForContainer_MultilineWrapperWithBlock(t *testing.T) {
	fileBytes := []byte("model(|| {\n    let a = 1;\n});")
	exprStmt := &treesitter.ASTNode{
		Type:      "expression_statement",
		Language:  "rust",
		StartByte: 0,
		EndByte:   uint32(len(fileBytes)),
		StartRow:  0,
		EndRow:    2,
	}
	callExpr := &treesitter.ASTNode{
		Type:      "call_expression",
		Language:  "rust",
		StartByte: 0,
		EndByte:   29,
		StartRow:  0,
		EndRow:    2,
		Parent:    exprStmt,
	}
	exprStmt.Children = []*treesitter.ASTNode{callExpr}

	args := &treesitter.ASTNode{
		Type:      "arguments",
		Language:  "rust",
		StartByte: 5,
		EndByte:   29,
		StartRow:  0,
		EndRow:    2,
		Parent:    callExpr,
	}
	callExpr.Children = []*treesitter.ASTNode{
		{Type: "identifier", StartByte: 0, EndByte: 5, StartRow: 0, EndRow: 0, Parent: callExpr},
		args,
	}

	closure := &treesitter.ASTNode{
		Type:      "closure_expression",
		Language:  "rust",
		StartByte: 6,
		EndByte:   27,
		StartRow:  0,
		EndRow:    2,
		Parent:    args,
	}
	args.Children = []*treesitter.ASTNode{closure}

	block := &treesitter.ASTNode{
		Type:      "block",
		Language:  "rust",
		StartByte: 9,
		EndByte:   27,
		StartRow:  0,
		EndRow:    2,
		Parent:    closure,
	}
	closure.Children = []*treesitter.ASTNode{
		{Type: "closure_parameters", StartByte: 6, EndByte: 8, StartRow: 0, EndRow: 0, Parent: closure},
		block,
	}

	stmt := &treesitter.ASTNode{
		Type:      "let_declaration",
		Language:  "rust",
		StartByte: 15,
		EndByte:   25,
		StartRow:  1,
		EndRow:    1,
		Parent:    block,
	}
	block.Children = []*treesitter.ASTNode{stmt}

	t.Run("expression_statement", func(t *testing.T) {
		start := uint32(0)
		end := uint32(len(fileBytes))
		hasFooter, fStart, fEnd := adjustRangeForContainer(exprStmt, &start, &end, fileBytes)
		if end != 9 {
			t.Errorf("expected exprStmt end to be clamped to 9 (start of block), got %d", end)
		}
		if !hasFooter {
			t.Errorf("expected exprStmt to have closing footer")
		}
		if fStart != 27 || fEnd != 29 {
			t.Errorf("expected exprStmt footer [27, 29], got [%d, %d]", fStart, fEnd)
		}
	})

	t.Run("call_expression", func(t *testing.T) {
		start := uint32(0)
		end := uint32(28)
		hasFooter, fStart, fEnd := adjustRangeForContainer(callExpr, &start, &end, fileBytes)
		if end != 9 {
			t.Errorf("expected callExpr end to be clamped to 9 (start of block), got %d", end)
		}
		if !hasFooter {
			t.Errorf("expected callExpr to have closing footer")
		}
		if fStart != 27 || fEnd != 28 {
			t.Errorf("expected callExpr footer [27, 28], got [%d, %d]", fStart, fEnd)
		}
	})
}

func TestAdjustRangeForContainer_TrailingBlockChildAndMultiArgList(t *testing.T) {
	fileBytes := []byte("opDeleted := slices.ContainsFunc(result.Actions(), func() {\n\treturn\n})")
	shortVar := &treesitter.ASTNode{
		Type:      "short_var_declaration",
		Language:  "go",
		StartByte: 0,
		EndByte:   uint32(len(fileBytes)),
		StartRow:  0,
		EndRow:    2,
	}
	lhsList := &treesitter.ASTNode{
		Type:      "expression_list",
		Language:  "go",
		StartByte: 0,
		EndByte:   9,
		StartRow:  0,
		EndRow:    0,
		Parent:    shortVar,
	}
	colonEq := &treesitter.ASTNode{
		Type:      ":=",
		Language:  "go",
		StartByte: 10,
		EndByte:   12,
		StartRow:  0,
		EndRow:    0,
		Parent:    shortVar,
	}
	rhsList := &treesitter.ASTNode{
		Type:      "expression_list",
		Language:  "go",
		StartByte: 13,
		EndByte:   uint32(len(fileBytes)),
		StartRow:  0,
		EndRow:    2,
		Parent:    shortVar,
	}
	shortVar.Children = []*treesitter.ASTNode{lhsList, colonEq, rhsList}

	callExpr := &treesitter.ASTNode{
		Type:      "call_expression",
		Language:  "go",
		StartByte: 13,
		EndByte:   uint32(len(fileBytes)),
		StartRow:  0,
		EndRow:    2,
		Parent:    rhsList,
	}
	rhsList.Children = []*treesitter.ASTNode{callExpr}

	argList := &treesitter.ASTNode{
		Type:      "argument_list",
		Language:  "go",
		StartByte: 32,
		EndByte:   uint32(len(fileBytes)),
		StartRow:  0,
		EndRow:    2,
		Parent:    callExpr,
	}
	callExpr.Children = []*treesitter.ASTNode{
		{Type: "selector_expression", Language: "go", StartByte: 13, EndByte: 32, StartRow: 0, EndRow: 0, Parent: callExpr},
		argList,
	}

	firstArgCall := &treesitter.ASTNode{
		Type:      "call_expression",
		Language:  "go",
		StartByte: 33,
		EndByte:   49,
		StartRow:  0,
		EndRow:    0,
		Parent:    argList,
	}
	funcLit := &treesitter.ASTNode{
		Type:      "func_literal",
		Language:  "go",
		StartByte: 51,
		EndByte:   uint32(len(fileBytes) - 1),
		StartRow:  0,
		EndRow:    2,
		Parent:    argList,
	}
	argList.Children = []*treesitter.ASTNode{firstArgCall, funcLit}

	block := &treesitter.ASTNode{
		Type:      "block",
		Language:  "go",
		StartByte: 58,
		EndByte:   uint32(len(fileBytes) - 1),
		StartRow:  0,
		EndRow:    2,
		Parent:    funcLit,
	}
	funcLit.Children = []*treesitter.ASTNode{block}
	retStmt := &treesitter.ASTNode{
		Type:      "return_statement",
		Language:  "go",
		StartByte: 61,
		EndByte:   67,
		StartRow:  1,
		EndRow:    1,
		Parent:    block,
	}
	block.Children = []*treesitter.ASTNode{retStmt}

	// Make sure the var declaration header clamps before the multiline RHS (byte 13) and doesn't claim a closing delimiter.
	start := uint32(0)
	end := uint32(len(fileBytes))
	hasFooter, _, _ := adjustRangeForContainer(shortVar, &start, &end, fileBytes)
	if end != 13 {
		t.Errorf("expected short_var_declaration prologue end to clamp to 13, got %d", end)
	}
	if hasFooter {
		t.Errorf("expected short_var_declaration not to claim a closing footer")
	}

	// The argument list still needs its closing ')' even though its first child is a call expression.
	argStart := argList.StartByte
	argEnd := argList.EndByte
	argHasFooter, argFStart, argFEnd := adjustRangeForContainer(argList, &argStart, &argEnd, fileBytes)
	if !argHasFooter {
		t.Fatalf("expected 2-arg argument_list to preserve its closing ')' footer")
	}
	if argFStart != uint32(len(fileBytes)-1) || argFEnd != uint32(len(fileBytes)) {
		t.Errorf("expected argument_list footer [%d, %d], got [%d, %d]", len(fileBytes)-1, len(fileBytes), argFStart, argFEnd)
	}
}

func TestAdjustRangeForContainer_EndToEnd(t *testing.T) {
	oldSrc := `fn f() void {
    a();
    b();
    c();
}
`
	newSrc := `fn f() void {
    if (cond) {
        x();
    } else {
        a();
        b();
        c();
    }
}
`
	oldAST, err := treesitter.Parse([]byte(oldSrc), "test.zig")
	if err != nil {
		t.Fatalf("parse oldSrc: %v", err)
	}
	newAST, err := treesitter.Parse([]byte(newSrc), "test.zig")
	if err != nil {
		t.Fatalf("parse newSrc: %v", err)
	}

	part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
	mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
	es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
	es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

	env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
		IncludeActions:    true,
		IncludeHighlights: true,
	})
	if err != nil {
		t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
	}

	// 1. Verify else_clause clamps to the prologue keyword rather than spanning the entire block.
	var elseAct *Action
	for i := range env.Actions {
		if env.Actions[i].Node != nil && env.Actions[i].Node.Type == "else_clause" {
			elseAct = &env.Actions[i]
			break
		}
	}
	if elseAct == nil {
		t.Fatalf("expected insert action for else_clause, found none")
	}
	if elseAct.Node.EndByte != 54 {
		t.Errorf("expected else_clause prologue to clamp to byte 54, got %d", elseAct.Node.EndByte)
	}

	// 2. Verify right highlights on moved lines inside else block have no phantom insert spans.
	for _, h := range env.RightHighlights {
		if h.Line >= 4 && h.Line <= 6 && h.Action == "insert" && h.StartCol == 0 {
			t.Errorf("found spurious insert highlight on moved line %d: %+v", h.Line, h)
		}
	}
}

func TestCollectMappedTrailingDelimiters(t *testing.T) {
	t.Run("binary expression refactored to call highlights new comma between stationary args", func(t *testing.T) {
		srcBytes := []byte("package p\nfunc f() bool {\n\treturn a.Type == b.Type\n}\n")
		dstBytes := []byte("package p\nfunc f() bool {\n\treturn TypesMatch(a.Type, b.Type, r)\n}\n")

		srcRoot, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstRoot, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		var srcA, dstA *treesitter.ASTNode
		for _, n := range srcRoot.PreOrder() {
			if n.Type == "selector_expression" && string(srcBytes[n.StartByte:n.EndByte]) == "a.Type" {
				srcA = n
				break
			}
		}
		for _, n := range dstRoot.PreOrder() {
			if n.Type == "selector_expression" && string(dstBytes[n.StartByte:n.EndByte]) == "a.Type" {
				dstA = n
				break
			}
		}
		if srcA == nil || dstA == nil {
			t.Fatalf("failed to locate a.Type selector_expression nodes")
		}

		ms := engine.NewMapping()
		ms.Add(srcA, dstA)

		left, right := collectMappedTrailingDelimiters(ms, srcRoot, srcBytes, dstBytes)
		if len(left) != 0 {
			t.Errorf("expected 0 left delimiter spans, got %+v", left)
		}
		if len(right) != 1 {
			t.Fatalf("expected 1 right delimiter span for inserted comma after a.Type, got %+v", right)
		}
		got := string(dstBytes[right[0].StartByte:right[0].EndByte])
		if right[0].Action != "insert" || got != ", " {
			t.Errorf("expected right delimiter span action='insert' text=', ', got action=%q text=%q", right[0].Action, got)
		}

		// Reverse direction: call refactored back to binary expression highlights deleted comma.
		msRev := engine.NewMapping()
		msRev.Add(dstA, srcA)
		leftRev, rightRev := collectMappedTrailingDelimiters(msRev, dstRoot, dstBytes, srcBytes)
		if len(rightRev) != 0 {
			t.Errorf("expected 0 right delimiter spans in reverse, got %+v", rightRev)
		}
		if len(leftRev) != 1 {
			t.Fatalf("expected 1 left delimiter span in reverse, got %+v", leftRev)
		}
		// In the reversed call, dstBytes acts as the source (left) side.
		gotRev := string(dstBytes[leftRev[0].StartByte:leftRev[0].EndByte])
		if leftRev[0].Action != "delete" || gotRev != ", " {
			t.Errorf("expected left delimiter span action='delete' text=', ', got action=%q text=%q", leftRev[0].Action, gotRev)
		}
	})

	t.Run("unwrapping expression inside mapped argument_list preserves existing comma", func(t *testing.T) {
		srcBytes := []byte("package p\nfunc f() {\n\tfoo(a + b, c)\n}\n")
		dstBytes := []byte("package p\nfunc f() {\n\tfoo(a, c)\n}\n")

		srcRoot, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatalf("parse src: %v", err)
		}
		dstRoot, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil {
			t.Fatalf("parse dst: %v", err)
		}

		var srcArgs, dstArgs, srcA, dstA *treesitter.ASTNode
		for _, n := range srcRoot.PreOrder() {
			if n.Type == "argument_list" {
				srcArgs = n
			} else if n.Type == "identifier" && n.Label == "a" {
				srcA = n
			}
		}
		for _, n := range dstRoot.PreOrder() {
			if n.Type == "argument_list" {
				dstArgs = n
			} else if n.Type == "identifier" && n.Label == "a" {
				dstA = n
			}
		}
		if srcArgs == nil || dstArgs == nil || srcA == nil || dstA == nil {
			t.Fatalf("failed to find expected AST nodes")
		}

		ms := engine.NewMapping()
		ms.Add(srcArgs, dstArgs)
		ms.Add(srcA, dstA)

		left, right := collectMappedTrailingDelimiters(ms, srcRoot, srcBytes, dstBytes)
		if len(left) != 0 || len(right) != 0 {
			t.Errorf("expected 0 delimiter spans when comma was inherited from enclosing slot, got left=%+v right=%+v", left, right)
		}
	})
}

func TestCollectMappedTrailingDelimiters_EndToEnd(t *testing.T) {
	t.Run("multiline item gaining trailing comma emits insert highlight without action churn", func(t *testing.T) {
		oldSrc := "package p\n\nvar x = []int{\n\t1,\n\t2\n}\n"
		newSrc := "package p\n\nvar x = []int{\n\t1,\n\t2,\n}\n"

		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// 1. Verify 1:1 action purity: no synthetic actions emitted for the trailing comma.
		if len(env.Actions) != 0 {
			t.Errorf("expected 0 AST actions for stationary elements, got %d", len(env.Actions))
		}

		// 2. Verify right highlights contain the inserted comma on line 4 (0-indexed).
		var commaSpan *HighlightSpan
		for i := range env.RightHighlights {
			h := &env.RightHighlights[i]
			if h.Line == 4 && h.Action == "insert" {
				commaSpan = h
				break
			}
		}
		if commaSpan == nil {
			t.Fatalf("expected insert highlight span for added trailing comma on line 4, got %+v", env.RightHighlights)
		}
		if commaSpan.StartCol != 2 || commaSpan.EndCol != 3 {
			t.Errorf("expected comma highlight at cols 2..3, got %d..%d", commaSpan.StartCol, commaSpan.EndCol)
		}
	})

	t.Run("binary expression refactored to call highlights inserted comma between stationary args", func(t *testing.T) {
		oldSrc := "package p\n\nfunc f() bool {\n\treturn a.Type == b.Type\n}\n"
		newSrc := "package p\n\nfunc f() bool {\n\treturn TypesMatch(a.Type, b.Type, r)\n}\n"

		oldAST, err := treesitter.Parse([]byte(oldSrc), "test.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse([]byte(newSrc), "test.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
		mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)
		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, []byte(oldSrc), []byte(newSrc), EnvelopeOptions{
			IncludeActions:    true,
			IncludeHighlights: true,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Verify line 3 has an insert highlight covering the comma after a.Type (cols 25..26).
		var commaSpan *HighlightSpan
		for i := range env.RightHighlights {
			h := &env.RightHighlights[i]
			if h.Line == 3 && h.Action == "insert" && h.StartCol == 25 && h.EndCol == 26 {
				commaSpan = h
				break
			}
		}
		if commaSpan == nil {
			t.Fatalf("expected insert highlight span for comma after a.Type at cols 25..26 on line 3, got %+v", env.RightHighlights)
		}
	})
}
