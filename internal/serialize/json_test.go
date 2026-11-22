package serialize

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/comments"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/testutil"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestRoundTrip(t *testing.T) {
	srcCode := []byte(`def foo():
    x = 1
    y = 2

def bar():
    pass
`)
	dstCode := []byte(`def foo():
    pass

def bar():
    z = 3
    x = 1
`)
	src, err := treesitter.Parse(srcCode, "src.py")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "dst.py")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	var srcFoo, srcBar, srcX, srcY, srcBarIdent *treesitter.ASTNode
	for _, n := range src.Children {
		if n.Type == "function_definition" {
			for _, c := range n.Children {
				if c.Type == "identifier" {
					switch c.Label {
					case "foo":
						srcFoo = n
					case "bar":
						srcBar = n
						srcBarIdent = c
					}
				}
			}
		}
	}
	for _, n := range srcFoo.Descendants() {
		if n.Type == "identifier" {
			if n.Label == "x" && srcX == nil {
				srcX = n
			}
			if n.Label == "y" && srcY == nil {
				srcY = n
			}
		}
	}

	var dstFoo, dstBar, dstZ, dstMovedX *treesitter.ASTNode
	for _, n := range dst.Children {
		if n.Type == "function_definition" {
			for _, c := range n.Children {
				if c.Type == "identifier" {
					switch c.Label {
					case "foo":
						dstFoo = n
					case "bar":
						dstBar = n
					}
				}
			}
		}
	}
	for _, n := range dstBar.Descendants() {
		if n.Type == "identifier" {
			if n.Label == "z" && dstZ == nil {
				dstZ = n
			}
			if n.Label == "x" && dstMovedX == nil {
				dstMovedX = n
			}
		}
	}

	if srcFoo == nil || srcBar == nil || srcX == nil || srcY == nil || srcBarIdent == nil ||
		dstFoo == nil || dstBar == nil || dstZ == nil || dstMovedX == nil {
		t.Fatalf("nodes not found in roundtrip snippets")
	}

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

	// 3. Update action (update srcBarIdent from "bar" to "new_bar")
	es.Add(actions.Action{
		Type:  actions.Update,
		Node:  srcBarIdent,
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
	jsonData, err := MarshalWithOptions(es, ms, src, dst, srcCode, dstCode, opts)
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
	if int(act4["old_position"].(float64)) != srcX.ChildIndex() {
		t.Errorf("move old_position should be %d, got %v", srcX.ChildIndex(), act4["old_position"])
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
	srcCode := []byte("def foo():\n    x = 1\n")
	dstCode := []byte("def foo():\n    pass\n")

	src, err := treesitter.Parse(srcCode, "src.py")
	if err != nil {
		t.Fatalf("parse src failed: %v", err)
	}
	dst, err := treesitter.Parse(dstCode, "dst.py")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	var srcFoo, srcX *treesitter.ASTNode
	for _, n := range src.Descendants() {
		if n.Type == "function_definition" && srcFoo == nil {
			srcFoo = n
		}
		if n.Type == "identifier" && n.Label == "x" && srcX == nil {
			srcX = n
		}
	}
	if srcFoo == nil || srcX == nil {
		t.Fatalf("nodes not found: srcFoo=%v, srcX=%v", srcFoo, srcX)
	}

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
	_, err = MarshalWithOptions(es, ms, src, dst, srcCode, dstCode, opts)
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

	// intentionally no postprocess.Run: isolates line diff highlight synthesis from AST matching
	testutil.AssertTagged(t, string(srcBytes), toTestSpans(env.LeftHighlights), `func main() {
	println("<upd>old</upd>")
}
`)
	testutil.AssertTagged(t, string(dstBytes), toTestSpans(env.RightHighlights), `func main() {
	println("<upd>new</upd>")
}
`)
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

	// intentionally no postprocess.Run: isolates line diff word-diff highlight synthesis
	testutil.AssertTagged(t, string(srcBytes), toTestSpans(env.LeftHighlights), `header
foo(a)
<del>deleted_line</del>
footer
`)
	testutil.AssertTagged(t, string(dstBytes), toTestSpans(env.RightHighlights), `header
foo(a<ins>, b</ins>)
footer
<ins>inserted_line</ins>
`)
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
	code := []byte("class A {\n    friend void foo() {\n    }\n};\n")
	ast, err := treesitter.Parse(code, "test.cpp")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var container *treesitter.ASTNode
	for _, n := range ast.Descendants() {
		if n.Type == "friend_declaration" {
			container = n
			break
		}
	}
	if container == nil {
		t.Fatal("expected friend_declaration node")
	}

	start := container.StartByte
	end := container.EndByte

	adjustRangeForContainer(container, &start, &end, nil)

	var fnChild *treesitter.ASTNode
	for _, c := range container.Children {
		if c.Type == "function_definition" {
			fnChild = c
			break
		}
	}
	if fnChild == nil {
		t.Fatal("expected function_definition child")
	}

	if start != container.StartByte || end != fnChild.StartByte {
		t.Errorf("expected container range sliced to [%d, %d], got [%d, %d]", container.StartByte, fnChild.StartByte, start, end)
	}
}

func TestAdjustRangeForContainerParenWrapper(t *testing.T) {
	code := []byte("int x = (12345678);\n")
	ast, err := treesitter.Parse(code, "test.c")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var paren *treesitter.ASTNode
	for _, n := range ast.Descendants() {
		if n.Type == "parenthesized_expression" {
			paren = n
			break
		}
	}
	if paren == nil {
		t.Fatal("expected parenthesized_expression node")
	}

	start := paren.StartByte
	end := paren.EndByte

	hasFooter, fStart, fEnd := adjustRangeForContainer(paren, &start, &end, code)
	if !hasFooter {
		t.Fatal("expected paren wrapper to report footer")
	}
	if string(code[start:end]) != "(" {
		t.Errorf("expected opening delimiter '(', got %q [%d, %d]", string(code[start:end]), start, end)
	}
	if string(code[fStart:fEnd]) != ")" {
		t.Errorf("expected closing delimiter ')', got %q [%d, %d]", string(code[fStart:fEnd]), fStart, fEnd)
	}
}

func TestAdjustRangeForContainerImplicitLengthArrayType(t *testing.T) {
	code := []byte("package main\n\nvar x = [...]string{}\n")
	ast, err := treesitter.Parse(code, "test.go")
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	var arr *treesitter.ASTNode
	for _, n := range ast.Descendants() {
		if n.Type == "implicit_length_array_type" {
			arr = n
			break
		}
	}
	if arr == nil {
		t.Fatal("expected implicit_length_array_type node")
	}

	start := arr.StartByte
	end := arr.EndByte

	hasFooter, fStart, fEnd := adjustRangeForContainer(arr, &start, &end, code)
	if !hasFooter {
		t.Errorf("expected closing delimiter footer for implicit length array")
	}
	if string(code[start:end]) != "[" {
		t.Errorf("expected container header range '[' for implicit length array, got %q [%d, %d]", string(code[start:end]), start, end)
	}
	if string(code[fStart:fEnd]) != "]" {
		t.Errorf("expected closing delimiter ']', got %q [%d, %d]", string(code[fStart:fEnd]), fStart, fEnd)
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
	// func f() {
	//     x := 1
	// }
	srcBytes := []byte("func f() {\n    x := 1\n}\n")
	dstBytes := []byte("func f() {\n    x := 1\n}\n")

	dstAST, err := treesitter.Parse(dstBytes, "test.go")
	if err != nil {
		t.Fatalf("parse dst failed: %v", err)
	}

	var blockNode *treesitter.ASTNode
	for _, n := range dstAST.Descendants() {
		if n.Type == "block" {
			blockNode = n
			break
		}
	}
	if blockNode == nil {
		t.Fatal("expected block node in dst")
	}

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
	env, err := BuildEnvelopeWithOptions(es, nil, nil, dstAST, srcBytes, dstBytes, opts)
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
	if env.Actions[0].Node.StartByte != blockNode.StartByte {
		t.Errorf("expected header start %d, got %d", blockNode.StartByte, env.Actions[0].Node.StartByte)
	}

	// intentionally no postprocess.Run: isolates delimiter highlight synthesis from move normalization
	testutil.AssertTagged(t, string(srcBytes), toTestSpans(env.LeftHighlights), `func f() {
    x := 1
}
`)
	testutil.AssertTagged(t, string(dstBytes), toTestSpans(env.RightHighlights), `func f() <ins>{</ins>
    x := 1
<ins>}</ins>
`)
}

func TestIndentationAndCommentFooterExclusion(t *testing.T) {
	// Python block has no closing delimiter (indentation based)
	// def foo():
	//     pass
	// \n
	pyCode := []byte("def foo():\n    pass\n\n")
	pyAST, err := treesitter.Parse(pyCode, "test.py")
	if err != nil {
		t.Fatalf("parse py failed: %v", err)
	}

	var funcDef *treesitter.ASTNode
	for _, n := range pyAST.Descendants() {
		if n.Type == "function_definition" {
			funcDef = n
			break
		}
	}
	if funcDef == nil {
		t.Fatal("expected function_definition in pyAST")
	}

	start := funcDef.StartByte
	end := funcDef.EndByte
	hasFooter, _, _ := adjustRangeForContainer(funcDef, &start, &end, pyCode)
	if hasFooter {
		t.Errorf("expected hasFooter=false for Python function_definition indentation block")
	}

	// Trailing comments should not be treated as closing delimiter
	pyCommentCode := []byte("def foo():\n    x = 1\n    # trailing comment\n")
	pyCommentAST, err := treesitter.Parse(pyCommentCode, "test.py")
	if err != nil {
		t.Fatalf("parse pyComment failed: %v", err)
	}

	var pyBlock *treesitter.ASTNode
	for _, n := range pyCommentAST.Descendants() {
		if n.Type == "block" {
			pyBlock = n
			break
		}
	}
	if pyBlock == nil {
		t.Fatal("expected block in pyCommentAST")
	}

	bStart := pyBlock.StartByte
	bEnd := pyBlock.EndByte
	hasCommentFooter, _, _ := adjustRangeForContainer(pyBlock, &bStart, &bEnd, pyCommentCode)
	if hasCommentFooter {
		t.Errorf("expected hasCommentFooter=false for sliced region containing comments")
	}

	// Python subscripts use bracket delimiters ([ ]), so they need closing footers.
	pySubscriptCode := []byte("cookie_dict[cookie.name]\n")
	pySubAST, err := treesitter.Parse(pySubscriptCode, "test.py")
	if err != nil {
		t.Fatalf("parse pySubscript failed: %v", err)
	}

	var subscript *treesitter.ASTNode
	for _, n := range pySubAST.Descendants() {
		if n.Type == "subscript" {
			subscript = n
			break
		}
	}
	if subscript == nil {
		t.Fatal("expected subscript in pySubAST")
	}

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
		srcAST, err := treesitter.Parse([]byte("package main\n\nfunc f() {\n\ta = 1\n}\n"), "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse([]byte("package main\n\nfunc f() {\n\ta := 1\n}\n"), "test.go")
		if err != nil {
			t.Fatal(err)
		}

		var parentSrc, childSrc, parentDst, childDst *treesitter.ASTNode
		for _, d := range srcAST.Descendants() {
			if d.Type == "assignment_statement" {
				parentSrc = d
				for _, c := range d.Children {
					if c.Type == "expression_list" {
						childSrc = c
						break
					}
				}
				break
			}
		}
		for _, d := range dstAST.Descendants() {
			if d.Type == "short_var_declaration" {
				parentDst = d
				for _, c := range d.Children {
					if c.Type == "expression_list" {
						childDst = c
						break
					}
				}
				break
			}
		}
		if parentSrc == nil || childSrc == nil || parentDst == nil || childDst == nil {
			t.Fatal("failed to find assignment and expression_list nodes in parsed snippets")
		}

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
				Node:          &NodeRef{Type: parentSrc.Type, StartByte: parentSrc.StartByte, EndByte: parentSrc.EndByte},
				DestStartByte: ptr(uint32(80)),
				DestEndByte:   ptr(uint32(95)),
			},
			{
				Action:        "move",
				ASTNode:       childSrc,
				DestASTNode:   childDst,
				Node:          &NodeRef{Type: childSrc.Type, StartByte: childSrc.StartByte, EndByte: childSrc.EndByte},
				OldParent:     &NodeRef{Type: parentSrc.Type, StartByte: parentSrc.StartByte, EndByte: parentSrc.EndByte},
				Parent:        &NodeRef{Type: parentDst.Type, StartByte: 80, EndByte: 95},
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
		srcBytes := []byte("package main\n\nfunc f() {\n\tline0()\n\tif cond {\n\t\tbody()\n\t}\n}\n")
		dstBytes := []byte("package main\n\nfunc f() {\n\tother0()\n\tif cond {\n\t\tbody()\n\t}\n}\n")
		srcOffsets := BuildLineIndex(srcBytes)
		dstOffsets := BuildLineIndex(dstBytes)

		srcAST, err := treesitter.Parse(srcBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}
		dstAST, err := treesitter.Parse(dstBytes, "test.go")
		if err != nil {
			t.Fatal(err)
		}

		var srcIf, srcBody, dstIf, dstBody *treesitter.ASTNode
		for _, d := range srcAST.Descendants() {
			if d.Type == "if_statement" {
				srcIf = d
			}
			if d.Type == "expression_statement" && len(d.Children) > 0 && d.Children[0].Type == "call_expression" {
				for _, c := range d.Children[0].Children {
					if c.Type == "identifier" && c.Label == "body" {
						srcBody = d
					}
				}
			}
		}
		for _, d := range dstAST.Descendants() {
			if d.Type == "if_statement" {
				dstIf = d
			}
			if d.Type == "expression_statement" && len(d.Children) > 0 && d.Children[0].Type == "call_expression" {
				for _, c := range d.Children[0].Children {
					if c.Type == "identifier" && c.Label == "body" {
						dstBody = d
					}
				}
			}
		}
		if srcIf == nil || srcBody == nil || dstIf == nil || dstBody == nil {
			t.Fatal("failed to find if_statement and body expression_statement in parsed snippets")
		}

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
				Node:          &NodeRef{Type: srcBody.Type, StartByte: srcBody.StartByte, EndByte: srcBody.EndByte},
				DestStartByte: ptr(dstBody.StartByte),
				DestEndByte:   ptr(dstBody.EndByte),
			},
			{
				Action:        "move",
				ASTNode:       srcIf,
				DestASTNode:   dstIf,
				Node:          &NodeRef{Type: srcIf.Type, StartByte: srcIf.StartByte, EndByte: srcIf.EndByte},
				DestStartByte: ptr(dstIf.StartByte),
				DestEndByte:   ptr(dstIf.EndByte),
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

		// Whole-snippet context line purity and co-mover move highlights
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() {
	keep1()
	<mov>moveA()</mov>
	<mov>moveB()</mov>
	keep2()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() {
	keep1()
	keep2()
	<ins>if true {</ins>
		<mov>moveA()</mov>
		<mov>moveB()</mov>
	<ins>}</ins>
}
`)
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

		// Whole-snippet context line purity and contrasting move color slots
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() {
	<mov>moveA()</mov>
	<mov1>moveB()</mov1>
	keep1()
	keep2()
	keep3()
	keep4()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() {
	keep1()
	<ins>if cond1 {</ins>
		<mov>moveA()</mov>
	<ins>}</ins>
	keep2()
	keep3()
	keep4()
	<ins>if cond2 {</ins>
		<mov1>moveB()</mov1>
	<ins>}</ins>
}
`)
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

		// Whole-snippet context line purity and converging moves
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() {
	<mov>if </mov><mUpd>cond1</mUpd><mov> {</mov>
<mov>		moveA()</mov>
<mov>	}</mov>
	keep1()
	keep2()
	keep3()
	keep4()
	<del>if cond2 {</del>
		<mov1>moveB()</mov1>
	<del>}</del>
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() {
	keep1()
	keep2()
	keep3()
	keep4()
	<mov>if </mov><mUpd>condTarget</mUpd><mov> {</mov>
<mov>		moveA()</mov>
<mov>	}</mov>
	<mov1>moveB()</mov1>
}
`)
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

		// 3. Whole-snippet tagged assertions: cohort in slot 0 (<mov>), sunk guard in slot 1 (<mov1>),
		// and stationary statements step5..step15 remain unhighlighted context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func run() {
	<mov1>if err != nil {</mov1>
<mov1>		return</mov1>
<mov1>	}</mov1>
	<mov>step1()</mov>
	<mov>step2()</mov>
	<mov>step3()</mov>
	<mov>step4()</mov>
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
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

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
	<mov>step1()</mov>
	<mov>step2()</mov>
	<mov>step3()</mov>
	<mov>step4()</mov>
	<mov1>if err != nil {</mov1>
<mov1>		return</mov1>
<mov1>	}</mov1>
}
`)

		// 4. Whole-snippet verification: enclosing package and func declarations have no edit actions
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}
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

		// 2. Whole-snippet tagged assertions: container header, body statements, and closing delimiter
		// all share move ColorIndex 0 (<mov>) without fracture, and stationary statements step1..step3 stay context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func run() {
	<mov>extra()</mov>
	<mov>if cond {</mov>
		<mov>body1()</mov>
		<mov>body2()</mov>
	<mov>}</mov>
	<mov>workMovedIn()</mov>
	step1()
	step2()
	step3()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func run() {
	step1()
	step2()
	step3()
	<mov>extra()</mov>
	<mov>if cond {</mov>
		<mov>workMovedIn()</mov>
		<mov>body1()</mov>
		<mov>body2()</mov>
	<mov>}</mov>
}
`)
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
		if len(moves) != 6 {
			t.Fatalf("expected 6 moves across the first 3 unnested branches, got %d: %+v", len(moves), moves)
		}

		// Find the first unnested branch (p := ... and current.oldPath = p).
		// Both should share MoveColorIndex 0 (Cyan) instead of current.oldPath = p taking Magenta (slot 1).
		var firstVarMove, firstAssignMove *Action
		seenBranches := make(map[string]bool, 3)
		for i := range moves {
			m := &moves[i]
			if m.Node == nil {
				continue
			}
			srcText := oldSrc[m.Node.StartByte:m.Node.EndByte]
			for _, marker := range []string{"rename from ", "rename to ", "--- "} {
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
		if firstVarMove == nil || firstAssignMove == nil || len(seenBranches) != 3 {
			t.Fatalf("expected moves across the first 3 branches (got var=%v, assign=%v, branches=%v)",
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

		// Whole-snippet verification: enclosing package and func declarations have no edit actions.
		for _, a := range env.Actions {
			if a.Node != nil && r.IsDeclaration(a.Node.Type) {
				t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
			}
		}

		// Whole-snippet tagged assertions: parallel unnested branches share MoveColorIndex 0 (<mov>),
		// and enclosing func declaration and index branch stay untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if <del>strings.HasPrefix(cleanLine, "rename from ")</del> {
		<del>if</del> <mov>p := parseHeaderPath(cleanLine, "rename from ")</mov><del>; p != "" {</del>
			<mov>current.oldPath = p</mov>
		<del>}</del>
	} else if <del>strings.HasPrefix(cleanLine, "rename to ")</del> {
		<del>if</del> <mov>p := parseHeaderPath(cleanLine, "rename to ")</mov><del>; p != "" {</del>
			<mov>current.newPath = p</mov>
		<del>}</del>
	} else if <del>strings.HasPrefix(cleanLine, "--- ")</del> {
		<del>if</del> <mov>p := parseHeaderPath(cleanLine, "--- ")</mov><del>; p != "" {</del>
			<mov>current.oldPath = p</mov>
		<del>}</del>
	} else <del>if strings.HasPrefix(cleanLine, "+++ ") {</del>
		if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
			current.newPath = p
		}
	<del>}</del>
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if <mov>p := parseHeaderPath(cleanLine, "rename from ")</mov><ins>; p != ""</ins> {
		<mov>current.oldPath = p</mov>
	} else if <mov>p := parseHeaderPath(cleanLine, "rename to ")</mov><ins>; p != ""</ins> {
		<mov>current.newPath = p</mov>
	} else if <mov>p := parseHeaderPath(cleanLine, "--- ")</mov><ins>; p != ""</ins> {
		<mov>current.oldPath = p</mov>
	} else if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
		current.newPath = p
	}
}
`)
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

		// Whole-snippet tagged assertions: extracted condition and body statement both share MoveColorIndex 0 (<mov>),
		// and post-check statements stay untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func check() (bool, float64) {
	<del>if</del> <mov>(r != nil && r.IsDelimited(u1.Type)) || (r == nil && rules.IsDelimited(u1.Type))</mov> <del>{</del>
		<mov>if (len(u1.Children) == 0) != (len(u2.Children) == 0) {</mov>
<mov>			return false, 0.0</mov>
<mov>		}</mov>
	<del>}</del>

	cScore := 1.0
	leafSim := substantiveLeafSimilarity(u1, u2, r)
	compositeScore := 0.50*cScore + 0.50*leafSim
	if compositeScore < 0.50 {
		return false, 0.0
	}
	return true, compositeScore
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func check() (bool, float64) {
	<ins>isDelim1 :=</ins> <mov>(r != nil && r.IsDelimited(u1.Type)) || (r == nil && rules.IsDelimited(u1.Type))</mov>
	<ins>isDelim2 := (r != nil && r.IsDelimited(u2.Type)) || (r == nil && rules.IsDelimited(u2.Type))</ins>
	<ins>if isDelim1 != isDelim2 {</ins>
<ins>		return false, 0.0</ins>
<ins>	}</ins>
	<ins>if isDelim1 && isDelim2 {</ins>
		<mov>if (len(u1.Children) == 0) != (len(u2.Children) == 0) {</mov>
<mov>			return false, 0.0</mov>
<mov>		}</mov>
		<ins>if len(u1.Children) == 0 && len(u2.Children) == 0 {</ins>
<ins>			return true, 1.0</ins>
<ins>		}</ins>
		<ins>leafSim := substantiveLeafSimilarity(u1, u2, r)</ins>
		<ins>if leafSim >= 0.40 {</ins>
<ins>			return true, leafSim</ins>
<ins>		}</ins>
		<ins>return false, 0.0</ins>
	<ins>}</ins>

	cScore := 1.0
	leafSim := substantiveLeafSimilarity(u1, u2, r)
	compositeScore := 0.50*cScore + 0.50*leafSim
	if compositeScore < 0.50 {
		return false, 0.0
	}
	return true, compositeScore
}
`)
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
	} else if strings.HasPrefix(cleanLine, "+++ ") {
		if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
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
	} else if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
		current.newPath = p
	}
	finish()
}
`
		env := runPipeline(t, oldSrc, newSrc)

		moves := collectMoves(env)
		if len(moves) != 4 {
			t.Fatalf("expected 4 downward lockstep moves across the first 2 unnested branches, got %d: %+v", len(moves), moves)
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
			t.Fatalf("expected moves across both downward lockstep branches, got %v", seenBranches)
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

		// Whole-snippet tagged assertions: downward lockstep cohort moves share MoveColorIndex 0 (<mov>),
		// and preamble/trailing statements stay untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process(cleanLine string) {
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if <del>strings.HasPrefix(cleanLine, "rename from ")</del> {
		<del>if</del> <mov>p := parseHeaderPath(cleanLine, "rename from ")</mov><del>; p != "" {</del>
			<mov>current.oldPath = p</mov>
		<del>}</del>
	} else if <del>strings.HasPrefix(cleanLine, "rename to ")</del> {
		<del>if</del> <mov>p := parseHeaderPath(cleanLine, "rename to ")</mov><del>; p != "" {</del>
			<mov>current.newPath = p</mov>
		<del>}</del>
	} else <del>if strings.HasPrefix(cleanLine, "+++ ") {</del>
		if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
			current.newPath = p
		}
	<del>}</del>
	finish()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process(cleanLine string) {
	<ins>a := prepareA(cleanLine)</ins>
	<ins>b := prepareB(a)</ins>
	<ins>c := prepareC(b)</ins>
	<ins>d := prepareD(c)</ins>
	<ins>use(d)</ins>
	if strings.HasPrefix(cleanLine, "index ") {
		current.oldOID = 1
	} else if <mov>p := parseHeaderPath(cleanLine, "rename from ")</mov><ins>; p != ""</ins> {
		<mov>current.oldPath = p</mov>
	} else if <mov>p := parseHeaderPath(cleanLine, "rename to ")</mov><ins>; p != ""</ins> {
		<mov>current.newPath = p</mov>
	} else if p := parseHeaderPath(cleanLine, "+++ "); p != "" {
		current.newPath = p
	}
	finish()
}
`)
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

		// Whole-snippet tagged assertions: converging moves keep distinct color slots (<mov> and <mov1>),
		// and enclosing func declaration and tail() stay untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func run() {
	<del>if skip1() {</del>
<del>		logStep("1")</del>
<del>		logStep("2")</del>
<del>		logStep("3")</del>
<del>		logStep("4")</del>
<del>		logStep("5")</del>
<del>	}</del>
	<del>if mode == "a" {</del>
		<mov>first()</mov>
	<del>}</del>
	<del>if skip2() {</del>
<del>		logStep("6")</del>
<del>		logStep("7")</del>
<del>		logStep("8")</del>
<del>		logStep("9")</del>
<del>		logStep("10")</del>
<del>	}</del>
	<del>if mode == "b" {</del>
		<mov1>second()</mov1>
	<del>}</del>
	tail()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func run() {
	<mov>first()</mov>
	<mov1>second()</mov1>
	tail()
}
`)
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

		// 2. Whole-snippet tagged assertions: in-order container and statements share slot 0 (<mov>),
		// while the reordered statement takes contrasting slot 1 (<mov1>/<mUpd1>),
		// and extraHeader/extraFooter statements stay untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

func process() {
	extraHeader1()
	extraHeader2()
	extraHeader3()
	<mov>for i := range n1 {</mov>
		<mov>for j := range n2 {</mov>
			<mov>calc()</mov>
			<mUpd1>canPair</mUpd1><mov1>[i][j] = </mov1><del>ok</del>
			<mov>sim[i][j] = score</mov>
		<mov>}</mov>
	<mov>}</mov>
	extraFooter1()
	extraFooter2()
	extraFooter3()
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func process() {
	extraHeader1()
	extraHeader2()
	extraHeader3()
	extraFooter1()
	extraFooter2()
	extraFooter3()
	<mov>for i := range n1 {</mov>
		<mov>for j := range n2 {</mov>
			<mov>calc()</mov>
			<ins>if ok {</ins>
				<mov>sim[i][j] = score</mov>
			<ins>} else {</ins>
				<mUpd1>sim</mUpd1><mov1>[i][j] = </mov1><ins>-1.0</ins>
			<ins>}</ins>
		<mov>}</mov>
	<mov>}</mov>
}
`)
	})
}

func TestAdjustRangeForContainer_LeadingTrivia(t *testing.T) {
	fileBytes := []byte("class Test {\n    void foo() {\n        // comment\n        if (x) {}\n    }\n}")
	ast, err := treesitter.ParseWithLanguage(fileBytes, "java")
	if err != nil {
		t.Fatal(err)
	}

	var block, stmt *treesitter.ASTNode
	for _, d := range ast.Descendants() {
		if d.Type == "block" && d.Parent != nil && d.Parent.Type == "method_declaration" {
			block = d
		}
		if d.Type == "if_statement" {
			stmt = d
		}
	}
	if block == nil || stmt == nil {
		t.Fatal("failed to find method block and if_statement in parsed java snippet")
	}
	if len(stmt.LeadingTrivia) == 0 {
		t.Fatal("expected if_statement to have attached LeadingTrivia")
	}

	start := block.StartByte
	end := block.EndByte
	hasFooter, fStart, fEnd := adjustRangeForContainer(block, &start, &end, fileBytes)

	expectedEnd := stmt.LeadingTrivia[0].StartByte
	if end != expectedEnd {
		t.Errorf("expected container prologue end to be clamped to %d (start of comment), got %d", expectedEnd, end)
	}
	if !hasFooter {
		t.Errorf("expected container to have footer for closing brace")
	}
	if fStart >= fEnd || string(fileBytes[fStart:fEnd]) != "}" {
		t.Errorf("expected closing brace footer, got [%d, %d] %q", fStart, fEnd, string(fileBytes[fStart:fEnd]))
	}
}

func TestAdjustRangeForContainer_MultilineWrapperWithBlock(t *testing.T) {
	fileBytes := []byte("fn main() {\n    model(|| {\n        let a = 1;\n    });\n}")
	ast, err := treesitter.ParseWithLanguage(fileBytes, "rust")
	if err != nil {
		t.Fatal(err)
	}

	var exprStmt, callExpr, block *treesitter.ASTNode
	for _, d := range ast.Descendants() {
		if d.Type == "expression_statement" {
			exprStmt = d
		}
		if d.Type == "call_expression" {
			callExpr = d
		}
		if d.Type == "block" && d.Parent != nil && d.Parent.Type == "closure_expression" {
			block = d
		}
	}
	if exprStmt == nil || callExpr == nil || block == nil {
		t.Fatal("failed to find expression_statement, call_expression, and closure block in parsed rust snippet")
	}

	t.Run("expression_statement", func(t *testing.T) {
		start := exprStmt.StartByte
		end := exprStmt.EndByte
		hasFooter, fStart, fEnd := adjustRangeForContainer(exprStmt, &start, &end, fileBytes)
		if end != block.StartByte {
			t.Errorf("expected exprStmt end to be clamped to %d (start of block), got %d", block.StartByte, end)
		}
		if !hasFooter {
			t.Errorf("expected exprStmt to have closing footer")
		}
		if string(fileBytes[fStart:fEnd]) != "}" {
			t.Errorf("expected exprStmt closing footer '}', got [%d, %d] %q", fStart, fEnd, string(fileBytes[fStart:fEnd]))
		}
	})

	t.Run("call_expression", func(t *testing.T) {
		start := callExpr.StartByte
		end := callExpr.EndByte
		hasFooter, fStart, fEnd := adjustRangeForContainer(callExpr, &start, &end, fileBytes)
		if end != block.StartByte {
			t.Errorf("expected callExpr end to be clamped to %d (start of block), got %d", block.StartByte, end)
		}
		if !hasFooter {
			t.Errorf("expected callExpr to have closing footer")
		}
		if string(fileBytes[fStart:fEnd]) != "}" {
			t.Errorf("expected callExpr closing footer '}', got [%d, %d] %q", fStart, fEnd, string(fileBytes[fStart:fEnd]))
		}
	})
}

func TestAdjustRangeForContainer_TrailingBlockChildAndMultiArgList(t *testing.T) {
	fileBytes := []byte("package main\n\nfunc f() {\n\topDeleted := slices.ContainsFunc(result.Actions(), func() {\n\t\treturn\n\t})\n}\n")
	ast, err := treesitter.Parse(fileBytes, "test.go")
	if err != nil {
		t.Fatal(err)
	}

	var shortVar, argList, rhsList *treesitter.ASTNode
	for _, d := range ast.Descendants() {
		if d.Type == "short_var_declaration" {
			shortVar = d
		}
		if d.Type == "argument_list" && len(d.Children) >= 2 {
			argList = d
		}
		if d.Type == "expression_list" && d.Parent != nil && d.Parent.Type == "short_var_declaration" && d != d.Parent.Children[0] {
			rhsList = d
		}
	}
	if shortVar == nil || argList == nil || rhsList == nil {
		t.Fatal("failed to find short_var_declaration, rhs expression_list, and argument_list in parsed go snippet")
	}

	// Make sure the var declaration header clamps before the multiline RHS and doesn't claim a closing delimiter.
	start := shortVar.StartByte
	end := shortVar.EndByte
	hasFooter, _, _ := adjustRangeForContainer(shortVar, &start, &end, fileBytes)
	if end != rhsList.StartByte {
		t.Errorf("expected short_var_declaration prologue end to clamp to %d, got %d", rhsList.StartByte, end)
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
	if argFStart != argList.EndByte-1 || argFEnd != argList.EndByte {
		t.Errorf("expected argument_list footer [%d, %d], got [%d, %d]", argList.EndByte-1, argList.EndByte, argFStart, argFEnd)
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

	// 2. Whole-snippet tagged assertions: else_clause prologue and new branches are inserted (<ins>),
	// moved lines inside else block have no phantom insert spans (<mov>), and enclosing function stays context.
	testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn f() void {
    <mov>a();</mov>
    <mov>b();</mov>
    <mov>c();</mov>
}
`)
	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn f() void {
    <ins>if (cond) {</ins>
<ins>        x();</ins>
<ins>    } else {</ins>
        <mov>a();</mov>
        <mov>b();</mov>
        <mov>c();</mov>
    <ins>}</ins>
}
`)
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

		// 2. Whole-snippet tagged assertions: line 4 is the only edited line with inserted comma.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package p

var x = []int{
	1,
	2
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package p

var x = []int{
	1,
	2<ins>,</ins>
}
`)
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

		// Whole-snippet tagged assertions: inserted comma and call wrapper around stationary args a.Type and b.Type.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package p

func f() bool {
	return a.Type <del>==</del> b.Type
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package p

func f() bool {
	return <ins>TypesMatch(</ins>a.Type<ins>,</ins> b.Type<ins>, r)</ins>
}
`)
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

		// Whole-snippet tagged assertions: doc comment moves along with function (<mov>),
		// and surrounding stationary functions remain untouched context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

<mov>// CalculateTax computes order tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

<mov>// CalculateTax computes order tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>
`)
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

		// Whole-snippet tagged assertions: unchanged comment words move (<mov>), inserted word is <ins>,
		// and untouched functions stay unhighlighted context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

<mov>// CalculateTax computes standard tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

<mov>// CalculateTax computes standard </mov><ins>regional</ins><mov> tax.</mov>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>
`)
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

		// Whole-snippet tagged assertions: deleted doc comment takes <del> (not <mov>),
		// moved function takes <mov>, and line 13 on right stays pure unadorned context.
		testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `package main

<del>// CalculateTax computes order tax.</del>
<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}
`)
		testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `package main

func stationaryOne() int {
	return 1
}

func stationaryTwo() int {
	return 2
}

func stationaryThree() int {
	return 3
}

<mov>func CalculateTax(subtotal int) int {</mov>
<mov>	return subtotal * 2</mov>
<mov>}</mov>
`)
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

func TestAssignMoveColors_ExtractedBlockColorContrast(t *testing.T) {
	r := rules.Get("rust")
	if r == nil {
		t.Fatal("rules for rust not found")
	}

	oldSrc := `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    match pagerflags.split_first() {
        Some((_, _)) => {
            let mut process = Command::new("less");
            process.arg("-R");
            process.arg("-F");
            process.arg("-X");
            process.arg("-K");
            process.arg("-i");
            process.env("LESSCHARSET", "UTF-8");
            process.env("LESSHISTFILE", "/dev/null");
            Ok(process)
        }
        None => Ok(Command::new("cat")),
    };
    process.wait()
}
`
	newSrc := `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    Ok(match pagerflags.split_first() {
        Some((_, _)) => {
            _make_process_from_less_path()
        }
        None => Command::new("cat"),
    });
    process.wait()
}

fn _make_process_from_less_path() -> Command {
    let mut process = Command::new("less");
    process.arg("-R");
    process.arg("-F");
    process.arg("-X");
    process.arg("-K");
    process.arg("-i");
    process.env("LESSCHARSET", "UTF-8");
    process.env("LESSHISTFILE", "/dev/null");
    process
}
`

	oldAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
	if err != nil {
		t.Fatalf("parse oldSrc: %v", err)
	}
	if oldAST.HasError || oldAST.ParseErrorCount > 0 {
		t.Fatalf("expected zero parse errors in oldSrc, got %d", oldAST.ParseErrorCount)
	}

	newAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
	if err != nil {
		t.Fatalf("parse newSrc: %v", err)
	}
	if newAST.HasError || newAST.ParseErrorCount > 0 {
		t.Fatalf("expected zero parse errors in newSrc, got %d", newAST.ParseErrorCount)
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

	// The outer try_pager function shouldn't have any edit actions.
	for _, a := range env.Actions {
		if a.Node != nil && r.IsDeclaration(a.Node.Type) && a.Node.StartByte == 0 {
			t.Errorf("expected context declaration %s to have no edit actions, got %v", a.Node.Type, a)
		}
	}

	// Extracted block moved to a new function, so it shouldn't share match_arm's group ID.
	var matchArmGid string
	var extractedBlockGid string
	var foundExtractedBlock bool
	for _, a := range env.Actions {
		if a.Action != "move" && a.Action != "move_update" {
			continue
		}
		if a.Node != nil && a.Node.Type == "match_arm" {
			matchArmGid = a.GroupID
		}
		if a.Node != nil && r.IsBlock(a.Node.Type) && a.DestASTNode != nil && a.DestASTNode.StartRow >= 11 {
			foundExtractedBlock = true
			extractedBlockGid = a.GroupID
		}
	}
	if !foundExtractedBlock {
		t.Fatalf("expected to find moved extracted block in actions, but none found: %+v", env.Actions)
	}
	if matchArmGid != "" && extractedBlockGid == matchArmGid {
		t.Errorf("extracted block leaked parent match_arm GroupID %q", extractedBlockGid)
	}

	// 3. Whole-snippet tagged assertions: match expression moves with color slot 0 (<mov>),
	// extracted block statements in _make_process_from_less_path have contrasting color slot 1 (<mov1>),
	// delimiter symmetry on opening and closing braces is maintained, and try_pager preamble/tail stay context.
	testutil.AssertTagged(t, oldSrc, toTestSpans(env.LeftHighlights), `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    <mov>match pagerflags.split_first() {</mov>
        <mov>Some((_, _)) =></mov> <mov1>{</mov1>
<mov1>            let mut process = Command::new("less");</mov1>
<mov1>            process.arg("-R");</mov1>
<mov1>            process.arg("-F");</mov1>
<mov1>            process.arg("-X");</mov1>
<mov1>            process.arg("-K");</mov1>
<mov1>            process.arg("-i");</mov1>
<mov1>            process.env("LESSCHARSET", "UTF-8");</mov1>
<mov1>            process.env("LESSHISTFILE", "/dev/null");</mov1>
<mov1>            </mov1><del>Ok(process)</del>
<mov1>        }</mov1>
        <mov>None => </mov><del>Ok(</del><mov>Command::new("cat")</mov><del>)</del><mov>,</mov>
    <mov>}</mov><del>;</del>
    process.wait()
}
`)
	testutil.AssertTagged(t, newSrc, toTestSpans(env.RightHighlights), `fn try_pager(pager: Option<String>) -> Result<()> {
    let pagerflags = ["--RAW-CONTROL-CHARS"];
    <ins>Ok(</ins><mov>match pagerflags.split_first() {</mov>
        <mov>Some((_, _)) =></mov> <ins>{</ins>
<ins>            _make_process_from_less_path()</ins>
<ins>        }</ins>
        <mov>None => Command::new("cat"),</mov>
    <mov>}</mov><ins>)</ins>;
    process.wait()
}

<ins>fn _make_process_from_less_path() -> Command</ins> <mov1>{</mov1>
<mov1>    let mut process = Command::new("less");</mov1>
<mov1>    process.arg("-R");</mov1>
<mov1>    process.arg("-F");</mov1>
<mov1>    process.arg("-X");</mov1>
<mov1>    process.arg("-K");</mov1>
<mov1>    process.arg("-i");</mov1>
<mov1>    process.env("LESSCHARSET", "UTF-8");</mov1>
<mov1>    process.env("LESSHISTFILE", "/dev/null");</mov1>
    <ins>process</ins>
<mov1>}</mov1>
`)
}
