package serialize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/comments"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
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
		t.Fatal("expected left highlights for deleted line")
	}
	if len(env.RightHighlights) == 0 {
		t.Fatal("expected right highlights for inserted line")
	}

	// Verify sub-line word-level highlights isolating "old" vs "new"
	hLeft := env.LeftHighlights[0]
	if hLeft.Line != 1 || hLeft.Action != "update" {
		t.Errorf("expected left highlight on line 1 with action 'update', got %+v", hLeft)
	}
	srcLine1 := "\tprintln(\"old\")"
	if srcLine1[hLeft.StartCol:hLeft.EndCol] != "old" {
		t.Errorf("expected left highlight to isolate 'old', got %q", srcLine1[hLeft.StartCol:hLeft.EndCol])
	}

	hRight := env.RightHighlights[0]
	if hRight.Line != 1 || hRight.Action != "update" {
		t.Errorf("expected right highlight on line 1 with action 'update', got %+v", hRight)
	}
	dstLine1 := "\tprintln(\"new\")"
	if dstLine1[hRight.StartCol:hRight.EndCol] != "new" {
		t.Errorf("expected right highlight to isolate 'new', got %q", dstLine1[hRight.StartCol:hRight.EndCol])
	}
}

func TestBuildLineDiffEnvelopeWithOptions_WordDiff_AsymmetricAndSpacers(t *testing.T) {
	// Line 0: identical
	// Line 1: modified (asymmetrical insert: foo(a) -> foo(a, b))
	// Line 2: spacer deleted on left (no counterpart on right)
	srcBytes := []byte("header\nfoo(a)\ndeleted_line\nfooter\n")
	dstBytes := []byte("header\nfoo(a, b)\nfooter\ninserted_line\n")

	opts := EnvelopeOptions{
		IncludeActions:    true,
		IncludeAlignment:  true,
		IncludeHighlights: true,
	}
	env := BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, opts)
	if env == nil {
		t.Fatal("expected non-nil envelope")
	}

	// Line 1 (foo(a) -> foo(a, b)):
	// Left side should have NO highlights on line 1 because 'foo', '(', 'a', ')' all matched.
	// Right side should have an 'insert' highlight on line 1 isolating ', b'.
	for _, h := range env.LeftHighlights {
		if h.Line == 1 {
			t.Errorf("unexpected left highlight on line 1 for pure insertion: %+v", h)
		}
	}
	var rightLine1Spans []HighlightSpan
	for _, h := range env.RightHighlights {
		if h.Line == 1 {
			rightLine1Spans = append(rightLine1Spans, h)
		}
	}
	if len(rightLine1Spans) != 1 {
		t.Fatalf("expected 1 right highlight on line 1, got %d: %+v", len(rightLine1Spans), rightLine1Spans)
	}
	if rightLine1Spans[0].Action != "insert" {
		t.Errorf("expected action 'insert', got %q", rightLine1Spans[0].Action)
	}
	dstLine1 := "foo(a, b)"
	if dstLine1[rightLine1Spans[0].StartCol:rightLine1Spans[0].EndCol] != ", b" {
		t.Errorf("expected right highlight to isolate ', b', got %q", dstLine1[rightLine1Spans[0].StartCol:rightLine1Spans[0].EndCol])
	}

	// Line 2: spacer line "deleted_line" on left should emit whole-line delete highlight
	var leftLine2Spans []HighlightSpan
	for _, h := range env.LeftHighlights {
		if h.Line == 2 {
			leftLine2Spans = append(leftLine2Spans, h)
		}
	}
	if len(leftLine2Spans) != 1 {
		t.Fatalf("expected 1 left highlight on line 2, got %d: %+v", len(leftLine2Spans), leftLine2Spans)
	}
	if leftLine2Spans[0].Action != "delete" || leftLine2Spans[0].StartCol != 0 || leftLine2Spans[0].EndCol != len("deleted_line") {
		t.Errorf("expected whole-line delete on line 2, got %+v", leftLine2Spans[0])
	}

	// Line 3 on right: spacer line "inserted_line" should emit whole-line insert highlight
	var rightLine3Spans []HighlightSpan
	for _, h := range env.RightHighlights {
		if h.Line == 3 {
			rightLine3Spans = append(rightLine3Spans, h)
		}
	}
	if len(rightLine3Spans) != 1 {
		t.Fatalf("expected 1 right highlight on line 3, got %d: %+v", len(rightLine3Spans), rightLine3Spans)
	}
	if rightLine3Spans[0].Action != "insert" || rightLine3Spans[0].StartCol != 0 || rightLine3Spans[0].EndCol != len("inserted_line") {
		t.Errorf("expected whole-line insert on line 3, got %+v", rightLine3Spans[0])
	}
}

func TestBuildLineDiffEnvelopeWithOptions_ShowStyleChanges(t *testing.T) {
	srcBytes := []byte("func main() {\n\tval  :=  1\n}\n")
	dstBytes := []byte("func main() {\n\tval := 1\n}\n")

	// When ShowStyleChanges is false (default), cosmetic whitespace difference should NOT emit delete/insert actions
	optsSuppressed := EnvelopeOptions{
		IncludeActions:    true,
		IncludeAlignment:  true,
		IncludeHighlights: true,
		ShowStyleChanges:  false,
	}
	envSuppressed := BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, optsSuppressed)
	if len(envSuppressed.Actions) != 0 {
		t.Errorf("expected 0 actions when ShowStyleChanges=false, got %d: %+v", len(envSuppressed.Actions), envSuppressed.Actions)
	}

	// When ShowStyleChanges is true, whitespace difference should emit synthetic delete + insert
	optsShown := EnvelopeOptions{
		IncludeActions:    true,
		IncludeAlignment:  true,
		IncludeHighlights: true,
		ShowStyleChanges:  true,
	}
	envShown := BuildLineDiffEnvelopeWithOptions(srcBytes, dstBytes, optsShown)
	if len(envShown.Actions) != 2 {
		t.Fatalf("expected 2 actions when ShowStyleChanges=true, got %d: %+v", len(envShown.Actions), envShown.Actions)
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

func TestAdjustRangeForContainerTupleStructPatternWrapper(t *testing.T) {
	code := []byte("fn f() { match x { State::HunkHeader(_, _) => {} } }")
	ast, err := treesitter.Parse(code, "test.rs")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var pat *treesitter.ASTNode
	for _, n := range ast.Descendants() {
		if n.Type == "tuple_struct_pattern" {
			pat = n
			break
		}
	}
	if pat == nil {
		t.Fatal("expected tuple_struct_pattern node")
	}

	start := pat.StartByte
	end := pat.EndByte

	hasFooter, fStart, fEnd := adjustRangeForContainer(pat, &start, &end, code)
	if !hasFooter {
		t.Fatal("expected tuple_struct_pattern to report footer")
	}
	if string(code[start:end]) != "(" {
		t.Errorf("expected opening delimiter '(', got %q [%d, %d]", string(code[start:end]), start, end)
	}
	if string(code[fStart:fEnd]) != ")" {
		t.Errorf("expected closing delimiter ')', got %q [%d, %d]", string(code[fStart:fEnd]), fStart, fEnd)
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

	t.Run("Container header and enclosed body co-mover share move color without interference", func(t *testing.T) {
		srcBytes := []byte("line0\nif cond {\n\tbody()\n}\n")
		dstBytes := []byte("other0\nif cond {\n\tbody()\n}\n")
		srcOffsets := BuildLineIndex(srcBytes)
		dstOffsets := BuildLineIndex(dstBytes)

		srcBody := &treesitter.ASTNode{Type: "expression_statement", StartByte: 16, EndByte: 23, Language: "go"}
		srcBlock := &treesitter.ASTNode{Type: "block", StartByte: 14, EndByte: 25, Children: []*treesitter.ASTNode{srcBody}, Language: "go"}
		srcBody.Parent = srcBlock
		srcIf := &treesitter.ASTNode{Type: "if_statement", StartByte: 6, EndByte: 25, Children: []*treesitter.ASTNode{srcBlock}, Language: "go"}
		srcBlock.Parent = srcIf

		dstBody := &treesitter.ASTNode{Type: "expression_statement", StartByte: 17, EndByte: 24, Language: "go"}
		dstBlock := &treesitter.ASTNode{Type: "block", StartByte: 15, EndByte: 26, Children: []*treesitter.ASTNode{dstBody}, Language: "go"}
		dstBody.Parent = dstBlock
		dstIf := &treesitter.ASTNode{Type: "if_statement", StartByte: 7, EndByte: 26, Children: []*treesitter.ASTNode{dstBlock}, Language: "go"}
		dstBlock.Parent = dstIf

		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 5},
				DestStartByte: ptr(uint32(0)),
				DestEndByte:   ptr(uint32(6)),
			},
			{
				Action:        "move",
				GroupID:       "group-1",
				ASTNode:       srcBody,
				DestASTNode:   dstBody,
				Node:          &NodeRef{Type: "expression_statement", StartByte: 16, EndByte: 23},
				DestStartByte: ptr(uint32(17)),
				DestEndByte:   ptr(uint32(24)),
			},
			{
				Action:        "move",
				ASTNode:       srcIf,
				DestASTNode:   dstIf,
				Node:          &NodeRef{Type: "if_statement", StartByte: 6, EndByte: 15},
				DestStartByte: ptr(uint32(7)),
				DestEndByte:   ptr(uint32(16)),
			},
		}

		AssignMoveColors(actions, srcOffsets, dstOffsets)

		if actions[1].MoveColorIndex != 0 {
			t.Fatalf("expected body move in group-1 to get color 0, got %d", actions[1].MoveColorIndex)
		}
		if actions[2].MoveColorIndex != 0 {
			t.Errorf("expected container header to inherit co-mover body color 0 instead of taking conflicting color 1, got %d", actions[2].MoveColorIndex)
		}
	})

	t.Run("parallel co-movers share color slot 0", func(t *testing.T) {
		// Move 0: src line 0 (bytes 0-10) -> dst line 2 (bytes 40-50)
		// Move 1: src line 1 (bytes 20-30) -> dst line 3 (bytes 60-70)
		// Both start adjacent in source, land adjacent in destination, and preserve relative order.
		actions := []Action{
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 0, EndByte: 10},
				DestStartByte: ptr(uint32(40)),
				DestEndByte:   ptr(uint32(50)),
			},
			{
				Action:        "move",
				Node:          &NodeRef{StartByte: 20, EndByte: 30},
				DestStartByte: ptr(uint32(60)),
				DestEndByte:   ptr(uint32(70)),
			},
		}
		AssignMoveColors(actions, srcOffsets, dstOffsets)
		if actions[0].MoveColorIndex != 0 || actions[1].MoveColorIndex != 0 {
			t.Errorf("expected parallel co-movers to share MoveColorIndex 0, got %d and %d",
				actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})

	t.Run("diverging child move escaping enclosing move receives contrasting color", func(t *testing.T) {
		actions := []Action{
			{
				Action: "move",
				Node: &NodeRef{
					Tree:      "before",
					Type:      "match_expression",
					StartByte: 0,
					EndByte:   100,
				},
				DestNode: &NodeRef{
					Tree:      "after",
					Type:      "match_expression",
					StartByte: 0,
					EndByte:   50,
				},
				OrigEndByte:     100,
				DestOrigEndByte: 50,
				GroupID:         "group-outer",
				OldParent: &NodeRef{
					Tree:      "before",
					Type:      "block",
					StartByte: 0,
					EndByte:   200,
				},
			},
			{
				Action: "move",
				Node: &NodeRef{
					Tree:      "before",
					Type:      "let_declaration",
					StartByte: 20,
					EndByte:   40,
				},
				DestNode: &NodeRef{
					Tree:      "after",
					Type:      "let_declaration",
					StartByte: 500,
					EndByte:   550,
				},
				OrigEndByte:     40,
				DestOrigEndByte: 550,
				GroupID:         "group-inner",
				OldParent: &NodeRef{
					Tree:      "before",
					Type:      "block",
					StartByte: 10,
					EndByte:   90,
				},
			},
		}

		srcOffsets := []int{0, 10, 20, 50, 80, 100}
		dstOffsets := make([]int, 60)
		for i := range dstOffsets {
			dstOffsets[i] = i * 10
		}

		AssignMoveColors(actions, srcOffsets, dstOffsets)

		if actions[0].MoveColorIndex == actions[1].MoveColorIndex {
			t.Errorf("expected outer move (slot %d) and escaped child move (slot %d) to have contrasting colors",
				actions[0].MoveColorIndex, actions[1].MoveColorIndex)
		}
	})
}

func TestAssignMoveColors_FullEngine(t *testing.T) {
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
			IncludeActions:    true,
			IncludeHighlights: true,
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

		// Whole-snippet context line purity
		assertContextLinesPure(t, env.LeftHighlights, []int{4, 5})
		assertContextLinesPure(t, env.RightHighlights, []int{5, 6, 7, 8})
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

		// Whole-snippet context line purity
		assertContextLinesPure(t, env.LeftHighlights, []int{3, 4})
		assertContextLinesPure(t, env.RightHighlights, []int{4, 5, 6, 10, 11, 12})
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

		// Whole-snippet context line purity
		assertContextLinesPure(t, env.LeftHighlights, []int{3, 4, 5, 10, 11, 12})
		assertContextLinesPure(t, env.RightHighlights, []int{7, 8, 9, 10})
	})

	t.Run("dominant cohort gets slot 0 and reordered outlier gets slot 1", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func run() {
	if err != nil {
		return
	}
	step1()
	step2()
	step3()
	step4()
	step5()
	step6()
	step7()
	step8()
	step9()
	step10()
	step11()
	step12()
	step13()
	step14()
	step15()
}
`
		newSrc := `package main

func run() {
	step5()
	step6()
	step7()
	step8()
	step9()
	step10()
	step11()
	step12()
	step13()
	step14()
	step15()
	step1()
	step2()
	step3()
	step4()
	if err != nil {
		return
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) != 5 {
			t.Fatalf("expected 5 moves (4 cohort + 1 sunk guard), got %d: %+v", len(moves), moves)
		}

		// 1. Dominant cohort (step1..step4) stays in order and shares group-1 in slot 0 (Cyan)
		statementColors := make(map[string]int)
		statementGroups := make(map[string]string)
		for _, m := range moves {
			if m.ASTNode != nil {
				text := strings.TrimSpace(string([]byte(oldSrc)[m.ASTNode.StartByte:m.ASTNode.EndByte]))
				statementColors[text] = m.MoveColorIndex
				statementGroups[text] = m.GroupID
			}
		}

		cohortGroup := statementGroups["step1()"]
		if cohortGroup == "" {
			t.Fatalf("expected cohort move to have a non-empty GroupID, got empty")
		}
		for _, stmt := range []string{"step1()", "step2()", "step3()", "step4()"} {
			if gid, ok := statementGroups[stmt]; !ok || gid != cohortGroup {
				t.Errorf("expected cohort statement %s to share GroupID %q, got %q", stmt, cohortGroup, gid)
			}
			if color, ok := statementColors[stmt]; !ok || color != 0 {
				t.Errorf("expected cohort statement %s to get MoveColorIndex 0 (Cyan), got %d", stmt, color)
			}
		}

		// 2. Out-of-order outlier (sunk if err != nil guard) stays ungrouped in slot 1 (Magenta)
		guardText := "if err != nil {\n\t\treturn\n\t}"
		if gid, ok := statementGroups[guardText]; !ok || gid != "" {
			t.Errorf("expected sunk guard to have empty GroupID, got %q", gid)
		}
		if color, ok := statementColors[guardText]; !ok || color != 1 {
			t.Errorf("expected sunk guard to get MoveColorIndex 1 (Magenta), got %d", color)
		}

		// 3. Highlight spans reflect their assigned color slots
		for _, h := range env.RightHighlights {
			if h.Action != "move" {
				continue
			}
			// Lines 14..17 are step1..step4 (cohort -> slot 0 / Cyan)
			if h.Line >= 14 && h.Line <= 17 {
				if h.ColorIndex != 0 {
					t.Errorf("expected cohort highlight on line %d to have ColorIndex 0, got %d", h.Line+1, h.ColorIndex)
				}
			}
			// Lines 18..20 are the sunk guard (outlier -> slot 1 / Magenta)
			if h.Line >= 18 && h.Line <= 20 {
				if h.ColorIndex != 1 {
					t.Errorf("expected outlier guard highlight on line %d to have ColorIndex 1, got %d", h.Line+1, h.ColorIndex)
				}
			}
		}

		// 4. Whole-snippet verification: enclosing package and func declarations have no edit actions
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 5. Whole-snippet context line purity: stationary statements step5..step15 remain unhighlighted
		assertContextLinesPure(t, env.LeftHighlights, []int{3, 4, 5, 6, 7, 8, 9})
		assertContextLinesPure(t, env.RightHighlights, []int{14, 15, 16, 17, 18, 19, 20})
	})

	t.Run("container header and body co-movers share move color without fracture", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func run() {
	extra()
	if cond {
		body1()
		body2()
	}
	workMovedIn()
	step1()
	step2()
	step3()
}
`
		newSrc := `package main

func run() {
	step1()
	step2()
	step3()
	extra()
	if cond {
		workMovedIn()
		body1()
		body2()
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)

		// 1. Whole-snippet verification: enclosing package and func declarations have no edit actions.
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Whole-snippet context line purity: stationary statements step1..step3 remain unhighlighted.
		assertContextLinesPure(t, env.LeftHighlights, []int{3, 4, 5, 6, 7, 8})
		assertContextLinesPure(t, env.RightHighlights, []int{6, 7, 8, 9, 10, 11})

		// 3. Container header, body statements, and closing delimiter all share move ColorIndex 0 without fracture.
		for _, h := range env.LeftHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected left move span on line %d [%d..%d] to have ColorIndex 0, got %d",
					h.Line, h.StartCol, h.EndCol, h.ColorIndex)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected right move span on line %d [%d..%d] to have ColorIndex 0, got %d",
					h.Line, h.StartCol, h.EndCol, h.ColorIndex)
			}
		}

		// 4. Specifically verify opening '{' and closing '}' delimiters on right side share ColorIndex 0.
		var foundOpeningBrace, foundClosingBrace bool
		for _, h := range env.RightHighlights {
			if h.Line == 7 && h.StartCol <= 9 && h.EndCol >= 10 && h.Action == "move" && h.ColorIndex == 0 {
				foundOpeningBrace = true
			}
			if h.Line == 11 && h.StartCol == 1 && h.EndCol == 2 && h.Action == "move" && h.ColorIndex == 0 {
				foundClosingBrace = true
			}
		}
		if !foundOpeningBrace {
			t.Errorf("expected right opening brace highlight span on line 7 with ColorIndex 0, got %+v", env.RightHighlights)
		}
		if !foundClosingBrace {
			t.Errorf("expected right closing brace highlight span on line 11 with ColorIndex 0, got %+v", env.RightHighlights)
		}
	})

	t.Run("parallel unnesting moves share move color slot 0", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if strings.HasPrefix(cleanLine, "rename from ") {
		if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
			current.oldPath = p
		}
	} else if strings.HasPrefix(cleanLine, "rename to ") {
		if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
			current.newPath = p
		}
	} else if strings.HasPrefix(cleanLine, "--- ") {
		if p := parseHeaderPath(cleanLine, "--- "); p != "" {
			current.oldPath = p
		}
	} else if strings.HasPrefix(cleanLine, "+++ ") {
		if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
			current.newPath = p
		}
	}
}
`
		newSrc := `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
		current.oldPath = p
	} else if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
		current.newPath = p
	} else if p := parseHeaderPath(cleanLine, "--- "); p != "" {
		current.oldPath = p
	} else if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
		current.newPath = p
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) != 7 {
			t.Fatalf("expected 7 moves across the 4 unnested branches, got %d: %+v", len(moves), moves)
		}

		// Find the first unnested branch (p := ... and current.oldPath = p).
		// Both should share MoveColorIndex 0 (Cyan) instead of current.oldPath = p taking Magenta (slot 1).
		var firstVarMove, firstAssignMove *Action
		seenBranches := make(map[string]bool, 4)
		for i := range moves {
			m := &moves[i]
			if m.Node == nil {
				continue
			}
			srcText := oldSrc[m.Node.StartByte:m.Node.EndByte]
			for _, marker := range []string{"rename from ", "rename to ", "--- ", "+++ "} {
				if strings.Contains(srcText, marker) {
					seenBranches[marker] = true
				}
			}
			if r.IsLocalVarDeclaration(m.Node.Type) && strings.Contains(srcText, "rename from") {
				firstVarMove = m
			}
			if r.IsAssignment(m.Node.Type) && strings.Contains(srcText, "current.oldPath = p") {
				if firstAssignMove == nil {
					firstAssignMove = m
				}
			}
		}
		if firstVarMove == nil || firstAssignMove == nil || len(seenBranches) != 4 {
			t.Fatalf("expected moves across all 4 branches (got var=%v, assign=%v, branches=%v)",
				firstVarMove, firstAssignMove, seenBranches)
		}

		for _, m := range moves {
			if m.MoveColorIndex != 0 {
				nodeType := "<nil>"
				var startByte, endByte uint32
				if m.Node != nil {
					nodeType = m.Node.Type
					startByte = m.Node.StartByte
					endByte = m.Node.EndByte
				}
				t.Errorf("expected all parallel unnested branch moves to share MoveColorIndex 0, got %d for %s (bytes %d..%d)",
					m.MoveColorIndex, nodeType, startByte, endByte)
			}
		}

		for _, h := range env.LeftHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected left move span on line %d [%d..%d] to have ColorIndex 0, got %d",
					h.Line, h.StartCol, h.EndCol, h.ColorIndex)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected right move span on line %d [%d..%d] to have ColorIndex 0, got %d",
					h.Line, h.StartCol, h.EndCol, h.ColorIndex)
			}
		}

		// Whole-snippet verification: enclosing package and func declarations have no edit actions.
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// Surrounding func declaration and index branch stay untouched as context.
		for _, h := range env.LeftHighlights {
			if h.Line <= 4 || h.Line >= 22 {
				t.Errorf("left line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line, h.Action, h.StartCol, h.EndCol)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Line <= 4 || h.Line >= 14 {
				t.Errorf("right line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line, h.Action, h.StartCol, h.EndCol)
			}
		}
	})

	t.Run("extracted condition does not taint body statement move", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func check() (bool, float64) {
	if (r != nil && r.IsDelimited(u1.Type)) || (r == nil && rules.IsDelimited(u1.Type)) {
		if (len(u1.Children) == 0) != (len(u2.Children) == 0) {
			return false, 0.0
		}
	}

	cScore := 1.0
	leafSim := substantiveLeafSimilarity(u1, u2, r)
	compositeScore := 0.50*cScore + 0.50*leafSim
	if compositeScore < 0.50 {
		return false, 0.0
	}
	return true, compositeScore
}
`
		newSrc := `package main

func check() (bool, float64) {
	isDelim1 := (r != nil && r.IsDelimited(u1.Type)) || (r == nil && rules.IsDelimited(u1.Type))
	isDelim2 := (r != nil && r.IsDelimited(u2.Type)) || (r == nil && rules.IsDelimited(u2.Type))
	if isDelim1 != isDelim2 {
		return false, 0.0
	}
	if isDelim1 && isDelim2 {
		if (len(u1.Children) == 0) != (len(u2.Children) == 0) {
			return false, 0.0
		}
		if len(u1.Children) == 0 && len(u2.Children) == 0 {
			return true, 1.0
		}
		leafSim := substantiveLeafSimilarity(u1, u2, r)
		if leafSim >= 0.40 {
			return true, leafSim
		}
		return false, 0.0
	}

	cScore := 1.0
	leafSim := substantiveLeafSimilarity(u1, u2, r)
	compositeScore := 0.50*cScore + 0.50*leafSim
	if compositeScore < 0.50 {
		return false, 0.0
	}
	return true, compositeScore
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) < 2 {
			t.Fatalf("expected at least 2 moves (condition + body), got %d: %+v", len(moves), moves)
		}

		// Find the body statement move (if (len(u1.Children) == 0)...).
		var bodyMove *Action
		for i := range moves {
			m := &moves[i]
			if m.ASTNode != nil && m.ASTNode.Type == "if_statement" {
				bodyMove = m
				break
			}
		}
		if bodyMove == nil {
			t.Fatalf("expected body if_statement move, but none found in moves: %+v", moves)
		}
		if bodyMove.MoveColorIndex != 0 {
			t.Errorf("expected body if_statement to have MoveColorIndex 0 (Cyan), got %d (tainted by extracted condition)",
				bodyMove.MoveColorIndex)
		}

		// Surrounding func declaration and postCheck stay untouched as context.
		for _, h := range env.LeftHighlights {
			if h.Line <= 2 || (h.Line >= 8 && h.Line <= 16) {
				t.Errorf("left line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line+1, h.Action, h.StartCol, h.EndCol)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Line <= 2 || (h.Line >= 21 && h.Line <= 29) {
				t.Errorf("right line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line+1, h.Action, h.StartCol, h.EndCol)
			}
		}
	})

	t.Run("downward lockstep cohort moves share move color slot 0", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if strings.HasPrefix(cleanLine, "rename from ") {
		if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
			current.oldPath = p
		}
	} else if strings.HasPrefix(cleanLine, "rename to ") {
		if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
			current.newPath = p
		}
	}
	finish()
}
`
		newSrc := `package main

func process(cleanLine string) {
	a := prepareA(cleanLine)
	b := prepareB(a)
	c := prepareC(b)
	d := prepareD(c)
	use(d)
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if p := parseHeaderPath(cleanLine, "rename from "); p != "" {
		current.oldPath = p
	} else if p := parseHeaderPath(cleanLine, "rename to "); p != "" {
		current.newPath = p
	}
	finish()
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) != 3 {
			t.Fatalf("expected 3 downward lockstep moves across both branches, got %d: %+v", len(moves), moves)
		}
		seenBranches := make(map[string]bool, 2)
		for _, m := range moves {
			if m.Node == nil {
				continue
			}
			srcText := oldSrc[m.Node.StartByte:m.Node.EndByte]
			for _, marker := range []string{"rename from ", "rename to "} {
				if strings.Contains(srcText, marker) {
					seenBranches[marker] = true
				}
			}
		}
		if len(seenBranches) != 2 {
			t.Fatalf("expected moves across both downward branches, got %v", seenBranches)
		}

		for _, m := range moves {
			if m.MoveColorIndex != 0 {
				t.Errorf("expected downward lockstep cohort move to share MoveColorIndex 0, got %d (%+v)", m.MoveColorIndex, m)
			}
		}
		for _, h := range env.LeftHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected left move span on line %d to have ColorIndex 0, got %d", h.Line, h.ColorIndex)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Action == "move" && h.ColorIndex != 0 {
				t.Errorf("expected right move span on line %d to have ColorIndex 0, got %d", h.Line, h.ColorIndex)
			}
		}

		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}
		assertContextLinesPure(t, env.LeftHighlights, []int{5, 6, 7, 8, 9, 10, 11, 12, 13})
		assertContextLinesPure(t, env.RightHighlights, []int{3, 4, 5, 6, 7, 10, 11, 12, 13, 14})
	})

	t.Run("converging moves exceeding lockstep delta difference keep distinct colors", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func run() {
	if skip1() {
		logStep("1")
		logStep("2")
		logStep("3")
		logStep("4")
		logStep("5")
	}
	if mode == "a" {
		first()
	}
	if skip2() {
		logStep("6")
		logStep("7")
		logStep("8")
		logStep("9")
		logStep("10")
	}
	if mode == "b" {
		second()
	}
	tail()
}
`
		newSrc := `package main

func run() {
	first()
	second()
	tail()
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) != 2 {
			t.Fatalf("expected 2 moves (first and second), got %d: %+v", len(moves), moves)
		}

		firstColor, secondColor := -1, -1
		for _, m := range moves {
			if m.Node == nil {
				continue
			}
			txt := strings.TrimSpace(oldSrc[m.Node.StartByte:m.Node.EndByte])
			if strings.Contains(txt, "first()") {
				firstColor = m.MoveColorIndex
			}
			if strings.Contains(txt, "second()") {
				secondColor = m.MoveColorIndex
			}
		}
		if firstColor == -1 || secondColor == -1 {
			t.Fatalf("expected moves for first() and second(), got %+v", moves)
		}
		if firstColor == secondColor {
			t.Errorf("expected non-cohort converging moves exceeding delta/gap thresholds to contrast, both got %d", firstColor)
		}

		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}
		for _, h := range env.LeftHighlights {
			if h.Line <= 2 || h.Line >= 23 {
				t.Errorf("left line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line, h.Action, h.StartCol, h.EndCol)
			}
		}
		for _, h := range env.RightHighlights {
			if h.Line <= 2 || h.Line >= 5 {
				t.Errorf("right line %d expected to be pure context (white), but found %s highlight span [%d..%d]",
					h.Line, h.Action, h.StartCol, h.EndCol)
			}
		}
	})

	t.Run("reordered statement inside moved container takes contrasting slot 1", func(t *testing.T) {
		r := rules.Get("go")
		if r == nil {
			t.Fatal("rules for go not found")
		}

		oldSrc := `package main

func process() {
	extraHeader1()
	extraHeader2()
	extraHeader3()
	for i := range n1 {
		for j := range n2 {
			calc()
			canPair[i][j] = ok
			sim[i][j] = score
		}
	}
	extraFooter1()
	extraFooter2()
	extraFooter3()
}
`
		newSrc := `package main

func process() {
	extraHeader1()
	extraHeader2()
	extraHeader3()
	extraFooter1()
	extraFooter2()
	extraFooter3()
	for i := range n1 {
		for j := range n2 {
			calc()
			if ok {
				sim[i][j] = score
			} else {
				sim[i][j] = -1.0
			}
		}
	}
}
`
		env := runPipeline(t, oldSrc, newSrc)

		// 1. Whole-snippet verification: enclosing package and func declarations have no edit actions.
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// 2. Whole-snippet context line purity: stationary statements extraHeader1..3 and extraFooter1..3 remain unhighlighted.
		assertContextLinesPure(t, env.LeftHighlights, []int{6, 7, 8, 9, 10, 11, 12})
		assertContextLinesPure(t, env.RightHighlights, []int{9, 10, 11, 12, 13, 14, 15, 16, 17, 18})

		// 3. Move color verification: in-order statements share container color 0 (Cyan),
		// while the reordered statement (canPair -> sim in else branch) receives contrasting color 1 (Magenta).
		var foundReorderedLeft, foundReorderedRight bool
		for _, h := range env.LeftHighlights {
			// Old line 10: canPair[i][j] = ok
			if h.Line == 9 && h.Action == "move" {
				if h.ColorIndex != 1 {
					t.Errorf("expected left move span on line 10 to have contrasting ColorIndex 1 (Magenta), got %d", h.ColorIndex)
				} else {
					foundReorderedLeft = true
				}
			}
		}
		for _, h := range env.RightHighlights {
			// New line 16: sim[i][j] = -1.0
			if h.Line == 15 && h.Action == "move" {
				if h.ColorIndex != 1 {
					t.Errorf("expected right move span on line 16 to have contrasting ColorIndex 1 (Magenta), got %d", h.ColorIndex)
				} else {
					foundReorderedRight = true
				}
			}
		}

		if !foundReorderedLeft {
			t.Errorf("expected to find left move span on line 10 with ColorIndex 1, got %+v", env.LeftHighlights)
		}
		if !foundReorderedRight {
			t.Errorf("expected to find right move span on line 16 with ColorIndex 1, got %+v", env.RightHighlights)
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

func TestAdjustRangeForContainer_FullEngine(t *testing.T) {
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

	// 3. Whole-snippet context line purity: fn f() void { and closing } must have zero highlights
	assertContextLinesPure(t, env.LeftHighlights, []int{1, 2, 3})
	assertContextLinesPure(t, env.RightHighlights, []int{1, 2, 3, 4, 5, 6, 7})
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

func TestCollectMappedTrailingDelimiters_FullEngine(t *testing.T) {
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

		// Whole-snippet context line purity: line 4 is the only edited line
		assertContextLinesPure(t, env.LeftHighlights, nil)
		assertContextLinesPure(t, env.RightHighlights, []int{4})
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

		// Whole-snippet context line purity: only line 3 has highlights
		assertContextLinesPure(t, env.LeftHighlights, []int{3})
		assertContextLinesPure(t, env.RightHighlights, []int{3})
	})
}

func TestCommentTriviaMoveHighlighting_FullEngine(t *testing.T) {
	t.Run("MovedFunctionWithDocComment", func(t *testing.T) {
		oldSrc := `package main

// CalculateTax computes order tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`
		newSrc := `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

// CalculateTax computes order tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		srcComments := comments.ExtractComments(oldAST)
		dstComments := comments.ExtractComments(newAST)

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		var commentLineMappings map[int]int
		if len(srcComments) > 0 || len(dstComments) > 0 {
			cRes := comments.DiffComments(srcComments, dstComments, mr.Mappings)
			if cRes != nil {
				for _, act := range cRes.Actions {
					es.Add(act)
				}
				commentLineMappings = cRes.LineMappings
			}
		}

		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:      true,
			IncludeHighlights:   true,
			IncludeAlignment:    true,
			CommentLineMappings: commentLineMappings,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Only the moved function belongs in env.Actions, with no synthetic comment actions.
		moveCount := 0
		for _, a := range env.Actions {
			if a.Action == "move" {
				moveCount++
				if a.Node != nil && a.Node.Type != "function_declaration" {
					t.Errorf("unexpected move action on non-function node: %s", a.Node.Type)
				}
			}
		}
		if moveCount != 1 {
			t.Fatalf("expected exactly 1 move action in env.Actions, got %d (actions: %+v)", moveCount, env.Actions)
		}
		if len(env.Actions) != 1 {
			t.Errorf("expected 1:1 AST action purity (len == 1), got %d: %+v", len(env.Actions), env.Actions)
		}

		// The doc comment on line 2 must move along with the function on line 3 using the same color.
		var leftCommentSpans, leftFuncSpans []HighlightSpan
		for _, h := range env.LeftHighlights {
			switch h.Line {
			case 2:
				leftCommentSpans = append(leftCommentSpans, h)
			case 3:
				leftFuncSpans = append(leftFuncSpans, h)
			}
		}
		if len(leftCommentSpans) == 0 {
			t.Fatalf("expected move highlight on left doc comment line 2, got none (highlights: %+v)", env.LeftHighlights)
		}
		if leftCommentSpans[0].Action != "move" {
			t.Errorf("expected left comment action to be 'move', got %q", leftCommentSpans[0].Action)
		}
		if len(leftFuncSpans) == 0 {
			t.Fatalf("expected move highlight on left function line 3, got none")
		}
		if leftCommentSpans[0].ColorIndex != leftFuncSpans[0].ColorIndex {
			t.Errorf("expected left comment ColorIndex (%d) to match function ColorIndex (%d)",
				leftCommentSpans[0].ColorIndex, leftFuncSpans[0].ColorIndex)
		}

		// On the right side, the doc comment on line 14 should also take the move highlight.
		var rightCommentSpans, rightFuncSpans []HighlightSpan
		for _, h := range env.RightHighlights {
			switch h.Line {
			case 14:
				rightCommentSpans = append(rightCommentSpans, h)
			case 15:
				rightFuncSpans = append(rightFuncSpans, h)
			}
		}
		if len(rightCommentSpans) == 0 {
			t.Fatalf("expected move highlight on right doc comment line 14, got none (highlights: %+v)", env.RightHighlights)
		}
		if rightCommentSpans[0].Action != "move" {
			t.Errorf("expected right comment action to be 'move', got %q", rightCommentSpans[0].Action)
		}
		if len(rightFuncSpans) == 0 {
			t.Fatalf("expected move highlight on right function line 15, got none")
		}
		if rightCommentSpans[0].ColorIndex != rightFuncSpans[0].ColorIndex {
			t.Errorf("expected right comment ColorIndex (%d) to match function ColorIndex (%d)",
				rightCommentSpans[0].ColorIndex, rightFuncSpans[0].ColorIndex)
		}

		// Surrounding stationary functions shouldn't have any highlights.
		assertContextLinesPure(t, env.LeftHighlights, []int{2, 3, 4, 5})
		assertContextLinesPure(t, env.RightHighlights, []int{14, 15, 16, 17})
	})

	t.Run("MovedFunctionWithInsertedDocCommentWord", func(t *testing.T) {
		oldSrc := `package main

// CalculateTax computes standard tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`
		newSrc := `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

// CalculateTax computes standard regional tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		srcComments := comments.ExtractComments(oldAST)
		dstComments := comments.ExtractComments(newAST)

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		var commentLineMappings map[int]int
		if len(srcComments) > 0 || len(dstComments) > 0 {
			cRes := comments.DiffComments(srcComments, dstComments, mr.Mappings)
			if cRes != nil {
				for _, act := range cRes.Actions {
					es.Add(act)
				}
				commentLineMappings = cRes.LineMappings
			}
		}

		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:      true,
			IncludeHighlights:   true,
			IncludeAlignment:    true,
			CommentLineMappings: commentLineMappings,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// The unchanged source comment stays a move; the inserted destination word is a move_update.
		hasMoveLeft := false
		for _, h := range env.LeftHighlights {
			if h.Line == 2 && h.Action == "move" {
				hasMoveLeft = true
			}
		}
		if !hasMoveLeft {
			t.Errorf("expected left comment line 2 to contain a move span, got: %+v", env.LeftHighlights)
		}

		hasMoveUpdateRight := false
		for _, h := range env.RightHighlights {
			if h.Line == 14 && h.Action == "move_update" {
				hasMoveUpdateRight = true
			}
		}
		if !hasMoveUpdateRight {
			t.Errorf("expected right comment line 14 to contain 'move_update' span, got: %+v", env.RightHighlights)
		}

		// Untouched functions stay unhighlighted.
		assertContextLinesPure(t, env.LeftHighlights, []int{2, 3, 4, 5})
		assertContextLinesPure(t, env.RightHighlights, []int{14, 15, 16, 17})
	})

	t.Run("DeletedCommentOnMovedFunction", func(t *testing.T) {
		oldSrc := `package main

// CalculateTax computes order tax.
func CalculateTax(subtotal int) int {
	return subtotal * 2
}

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`
		newSrc := `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

func CalculateTax(subtotal int) int {
	return subtotal * 2
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		srcComments := comments.ExtractComments(oldAST)
		dstComments := comments.ExtractComments(newAST)

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		var commentLineMappings map[int]int
		if len(srcComments) > 0 || len(dstComments) > 0 {
			cRes := comments.DiffComments(srcComments, dstComments, mr.Mappings)
			if cRes != nil {
				for _, act := range cRes.Actions {
					es.Add(act)
				}
				commentLineMappings = cRes.LineMappings
			}
		}

		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:      true,
			IncludeHighlights:   true,
			IncludeAlignment:    true,
			CommentLineMappings: commentLineMappings,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// The deleted comment on line 2 should take delete, not move.
		var leftCommentSpans []HighlightSpan
		for _, h := range env.LeftHighlights {
			if h.Line == 2 {
				leftCommentSpans = append(leftCommentSpans, h)
			}
		}
		if len(leftCommentSpans) == 0 {
			t.Fatalf("expected delete highlight on left doc comment line 2, got none")
		}
		for _, s := range leftCommentSpans {
			if s.Action == "move" {
				t.Errorf("deleted comment must NOT take 'move' highlight, got %+v", s)
			}
			if s.Action != "delete" {
				t.Errorf("expected deleted comment to take 'delete' highlight, got %q", s.Action)
			}
		}

		// Make sure line 13 on the right stays pure context since no comment was added there.
		for _, h := range env.RightHighlights {
			if h.Line == 13 {
				t.Errorf("expected line 13 on right to be pure context, got highlight %+v", h)
			}
		}
	})

	t.Run("LineAlignmentCommentInversionShielding", func(t *testing.T) {
		// Moving a comment shouldn't create inverted Tier 1 line anchors.
		oldSrc := `package main

func first() int {
	return 1
}

// Comment on second
func second() int {
	return 2
}

func stationary() int {
	return 3
}
`
		newSrc := `package main

func first() int {
	return 1
}

func stationary() int {
	return 3
}

// Comment on second
func second() int {
	return 2
}
`
		srcBytes := []byte(oldSrc)
		dstBytes := []byte(newSrc)

		oldAST, err := treesitter.Parse(srcBytes, "old.go")
		if err != nil {
			t.Fatalf("parse oldSrc: %v", err)
		}
		newAST, err := treesitter.Parse(dstBytes, "new.go")
		if err != nil {
			t.Fatalf("parse newSrc: %v", err)
		}

		srcComments := comments.ExtractComments(oldAST)
		dstComments := comments.ExtractComments(newAST)

		part := engine.NewLinePartition(srcBytes, dstBytes)
		mr := engine.Match(oldAST, newAST, srcBytes, dstBytes, part)
		es := actions.GenerateEditScript(oldAST, newAST, mr.Mappings)

		var commentLineMappings map[int]int
		if len(srcComments) > 0 || len(dstComments) > 0 {
			cRes := comments.DiffComments(srcComments, dstComments, mr.Mappings)
			if cRes != nil {
				for _, act := range cRes.Actions {
					es.Add(act)
				}
				commentLineMappings = cRes.LineMappings
			}
		}

		es = postprocess.Run(es, mr.Mappings, oldAST, newAST)

		env, err := BuildEnvelopeWithOptions(es, mr.Mappings, oldAST, newAST, srcBytes, dstBytes, EnvelopeOptions{
			IncludeActions:      true,
			IncludeHighlights:   true,
			IncludeAlignment:    true,
			CommentLineMappings: commentLineMappings,
		})
		if err != nil {
			t.Fatalf("BuildEnvelopeWithOptions failed: %v", err)
		}

		// Alignment pairs must be strictly monotonic without inversions.
		lastLeft := -1
		lastRight := -1
		for _, pair := range env.LineAlignment {
			if pair.LeftLine != -1 && pair.RightLine != -1 {
				if pair.LeftLine < lastLeft || pair.RightLine < lastRight {
					t.Errorf("detected alignment inversion: pair (%d, %d) preceded by (%d, %d)",
						pair.LeftLine, pair.RightLine, lastLeft, lastRight)
				}
				lastLeft = pair.LeftLine
				lastRight = pair.RightLine
			}
		}
	})
}

func TestFormatFallbackAnnotation(t *testing.T) {
	t.Run("SizeLimit", func(t *testing.T) {
		tests := []struct {
			bytes int
			want  string
		}{
			{500, "[500 B exceeded DIFFM_SIZE_LIMIT, falling back to line diff]"},
			{1024, "[1.0 KB exceeded DIFFM_SIZE_LIMIT, falling back to line diff]"},
			{500 * 1024, "[500.0 KB exceeded DIFFM_SIZE_LIMIT, falling back to line diff]"},
			{1024 * 1024, "[1.0 MB exceeded DIFFM_SIZE_LIMIT, falling back to line diff]"},
			{1468006, "[1.4 MB exceeded DIFFM_SIZE_LIMIT, falling back to line diff]"},
		}
		for _, tt := range tests {
			if got := FormatFallbackAnnotationSizeLimit(tt.bytes); got != tt.want {
				t.Errorf("FormatFallbackAnnotationSizeLimit(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		}
	})

	t.Run("LineLimit", func(t *testing.T) {
		tests := []struct {
			lines int
			want  string
		}{
			{10, "[10 lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]"},
			{999, "[999 lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]"},
			{1000, "[1,000 lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]"},
			{10500, "[10,500 lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]"},
			{1234567, "[1,234,567 lines exceeded DIFFM_LINE_LIMIT, falling back to line diff]"},
		}
		for _, tt := range tests {
			if got := FormatFallbackAnnotationLineLimit(tt.lines); got != tt.want {
				t.Errorf("FormatFallbackAnnotationLineLimit(%d) = %q, want %q", tt.lines, got, tt.want)
			}
		}
	})
}
