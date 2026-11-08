package comments

import (
	"os"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestDiffCommentsIdentical(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// Hello World", StartRow: 5, StartByte: 10, EndByte: 24},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// Hello World", StartRow: 8, StartByte: 15, EndByte: 29},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 0 {
		t.Errorf("expected 0 actions for identical comment, got %d actions", len(res.Actions))
	}
}

func TestDiffCommentsSingleLineUpdate(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// Old Comment", StartRow: 5, StartByte: 10, EndByte: 24},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// New Comment", StartRow: 5, StartByte: 10, EndByte: 24},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	act := res.Actions[0]
	if act.Type != actions.Update {
		t.Fatalf("expected Update action, got %v", act.Type)
	}
	if act.Node.Label != "Old" || act.Value != "New" {
		t.Errorf("expected word update 'Old' -> 'New', got %q -> %q", act.Node.Label, act.Value)
	}
}

func TestDiffCommentsMultiLineLineDiff(t *testing.T) {
	oldJavadoc := "/**\n * Line 1\n * Line 2 Old\n * Line 3\n */"
	newJavadoc := "/**\n * Line 1\n * Line 2 New\n * Line 3\n */"

	srcComments := []CommentBlock{
		{Type: "block_comment", Text: oldJavadoc, StartRow: 10, StartByte: 0, EndByte: uint32(len(oldJavadoc))},
	}
	dstComments := []CommentBlock{
		{Type: "block_comment", Text: newJavadoc, StartRow: 10, StartByte: 0, EndByte: uint32(len(newJavadoc))},
	}

	res := DiffComments(srcComments, dstComments, nil)

	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 line-level update action in multiline comment, got %d", len(res.Actions))
	}
	act := res.Actions[0]
	if act.Type != actions.Update {
		t.Fatalf("expected Update action, got %v", act.Type)
	}
	if act.Node.Label != "Old" || act.Value != "New" {
		t.Errorf("expected line update on line 2, got %q -> %q", act.Node.Label, act.Value)
	}
	if act.DestNode == nil {
		t.Errorf("expected DestNode to be set on line update action")
	}
}

func TestExtractCommentsWithTreeSitter(t *testing.T) {
	src := []byte("package main\n\n// Line comment 1\nfunc main() {\n\t// Line comment 2\n}\n")

	ast, err := treesitter.ParseWithLanguage(src, "go")
	if err != nil {
		t.Fatal(err)
	}

	comments := ExtractComments(ast)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments extracted, got %d", len(comments))
	}
	if comments[0].Text != "// Line comment 1" {
		t.Errorf("expected '// Line comment 1', got %q", comments[0].Text)
	}
	if comments[0].Language != "go" {
		t.Errorf("expected comments[0].Language == 'go', got %q", comments[0].Language)
	}
	if comments[1].Text != "// Line comment 2" {
		t.Errorf("expected '// Line comment 2', got %q", comments[1].Text)
	}
	if comments[1].Language != "go" {
		t.Errorf("expected comments[1].Language == 'go', got %q", comments[1].Language)
	}
}

func TestDiffCommentsScopeLocking(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// Method A comment", ScopeKey: "method:funcA", StartRow: 10},
		{Type: "comment", Text: "// Method B comment", ScopeKey: "method:funcB", StartRow: 20},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// Method A new comment", ScopeKey: "method:funcA", StartRow: 10},
		{Type: "comment", Text: "// Method B comment", ScopeKey: "method:funcB", StartRow: 20},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(res.Actions))
	}
	// Word diffing classifies inserting "new " as an insert operation.
	if res.Actions[0].Type != actions.Insert {
		t.Errorf("expected Insert action, got %v", res.Actions[0].Type)
	}
}

func TestDiffCommentsControlBranchScopeLocking(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "-- TODO: track whitespace for level", ScopeKey: "function:foo/if_statement/else_clause/elseif_clause", StartRow: 50},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "-- Track leading whitespace for level", ScopeKey: "function:foo/if_statement", StartRow: 45},
	}

	res := DiffComments(srcComments, dstComments, nil)
	// Moving between branches should delete and insert instead of update in place.
	if len(res.Actions) != 2 {
		t.Fatalf("expected 2 actions (1 delete, 1 insert), got %d", len(res.Actions))
	}
	hasDelete := false
	hasInsert := false
	for _, act := range res.Actions {
		if act.Type == actions.Delete {
			hasDelete = true
		}
		if act.Type == actions.Insert {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected 1 Delete and 1 Insert action across different branches, got: %+v", res.Actions)
	}
}

func TestDiffCommentsMovedScope(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// Optional params comment", ScopeKey: "method_declaration:getParamsToSign", StartRow: 162, EndRow: 162, Language: "go"},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// Optional params comment", ScopeKey: "method_declaration:getOauthParams", StartRow: 159, EndRow: 159, Language: "go"},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 2 {
		t.Fatalf("expected 2 actions (1 Delete, 1 Insert) for comment moved across scopes, got %d actions", len(res.Actions))
	}
	hasDelete := false
	hasInsert := false
	for _, act := range res.Actions {
		if act.Type == actions.Move {
			t.Errorf("expected no Move action for comment trivia, got actions.Move")
		}
		if act.Type == actions.Delete {
			hasDelete = true
		}
		if act.Type == actions.Insert {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected 1 Delete and 1 Insert action, got: %+v", res.Actions)
	}
	if _, ok := res.LineMappings[162]; ok {
		t.Errorf("expected cross-scope comment NOT to populate LineMappings, got %v", res.LineMappings)
	}
}

func TestDiffCommentsGinGo(t *testing.T) {
	src, err := os.ReadFile("../../tests/testdata/go_gin_fix_1784/old.go")
	if err != nil {
		t.Fatal(err)
	}
	dst, err := os.ReadFile("../../tests/testdata/go_gin_fix_1784/new.go")
	if err != nil {
		t.Fatal(err)
	}

	astA, err := treesitter.ParseWithLanguage(src, "go")
	if err != nil {
		t.Fatal(err)
	}
	astB, err := treesitter.ParseWithLanguage(dst, "go")
	if err != nil {
		t.Fatal(err)
	}

	srcComments := ExtractComments(astA)
	dstComments := ExtractComments(astB)

	res := DiffComments(srcComments, dstComments, nil)
	moveCount := 0
	for _, a := range res.Actions {
		if a.Type == actions.Move {
			moveCount++
		}
	}
	if moveCount != 0 {
		t.Errorf("expected 0 Move actions for comment trivia in Gin Go diff, got %d", moveCount)
	}
}

func TestSyntheticCommentNodeInvariants(t *testing.T) {
	cb := CommentBlock{
		Type:         "comment",
		Text:         "// Hello World",
		StartByte:    10,
		EndByte:      24,
		StartRow:     2,
		StartCol:     0,
		EndRow:       2,
		EndCol:       14,
		ParentType:   "function_declaration",
		ParentStart:  0,
		ParentEnd:    100,
		ParentRow:    1,
		ParentEndRow: 10,
		Language:     "go",
	}

	node := createCommentNode(&cb, cb.Language)
	if node == nil {
		t.Fatal("expected non-nil ASTNode")
	}
	if node.Parent == nil {
		t.Fatal("expected non-nil Parent")
	}
	if len(node.Parent.Children) != 1 || node.Parent.Children[0] != node {
		t.Fatalf("expected Parent.Children to contain node, got %v", node.Parent.Children)
	}
	if idx := node.ChildIndex(); idx != 0 {
		t.Errorf("expected ChildIndex() == 0, got %d", idx)
	}
	if lang := node.GetLanguage(); lang != "go" {
		t.Errorf("expected GetLanguage() == %q, got %q", "go", lang)
	}

	// Without parent
	cbNoParent := CommentBlock{
		Type:      "comment",
		Text:      "// Top-level comment",
		StartByte: 0,
		EndByte:   20,
		Language:  "go",
	}
	nodeNoParent := createCommentNode(&cbNoParent, cbNoParent.Language)
	if nodeNoParent.Parent != nil {
		t.Errorf("expected nil parent for top-level comment")
	}
	if idx := nodeNoParent.ChildIndex(); idx != -1 {
		t.Errorf("expected ChildIndex() == -1 for parentless node, got %d", idx)
	}
}

func TestSyntheticCommentWordNodeInvariants(t *testing.T) {
	cb := CommentBlock{
		Type:         "block_comment",
		Text:         "/* line 1\n * line 2 */",
		StartByte:    10,
		EndByte:      40,
		StartRow:     2,
		StartCol:     0,
		EndRow:       3,
		EndCol:       11,
		ParentType:   "class_declaration",
		ParentStart:  0,
		ParentEnd:    200,
		ParentRow:    1,
		ParentEndRow: 20,
		Language:     "python",
	}

	node := createCommentWordNode(&cb, " * line 2 */", 20, 32, 3, 0, uint32(len(" * line 2 */")), cb.Language)
	if node == nil {
		t.Fatal("expected non-nil ASTNode")
	}
	if node.Parent == nil {
		t.Fatal("expected non-nil Parent")
	}
	if len(node.Parent.Children) != 1 || node.Parent.Children[0] != node {
		t.Fatalf("expected Parent.Children to contain line node, got %v", node.Parent.Children)
	}
	if idx := node.ChildIndex(); idx != 0 {
		t.Errorf("expected ChildIndex() == 0, got %d", idx)
	}
	if lang := node.GetLanguage(); lang != "python" {
		t.Errorf("expected GetLanguage() == %q, got %q", "python", lang)
	}
}

func TestDiffCommentsScopedLCSNoCrossover(t *testing.T) {
	// Repeated comments in the same function should pair up in order rather than crossing over.
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 10, EndRow: 10},
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 20, EndRow: 20},
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 30, EndRow: 30},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 12, EndRow: 12},
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 22, EndRow: 22},
		{Type: "comment", Text: "// step", ScopeKey: "func:doWork", StartRow: 32, EndRow: 32},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions for matched identical comments, got %d", len(res.Actions))
	}
	if res.LineMappings[10] != 12 || res.LineMappings[20] != 22 || res.LineMappings[30] != 32 {
		t.Errorf("expected mappings 10->12, 20->22, 30->32; got %v", res.LineMappings)
	}
}

func TestDiffCommentsRenamedFunction(t *testing.T) {
	srcDecl := &treesitter.ASTNode{ID: 10, Type: "function_declaration", StartByte: 0, EndByte: 200}
	dstDecl := &treesitter.ASTNode{ID: 25, Type: "function_declaration", StartByte: 0, EndByte: 200}

	mappings := engine.NewMapping()
	mappings.Add(srcDecl, dstDecl)

	srcComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// step 1: initialize",
			StartRow:      5,
			EndRow:        5,
			EnclosingDecl: srcDecl,
			RelativePath:  "body",
		},
		{
			Type:          "comment",
			Text:          "// step 2: execute",
			StartRow:      10,
			EndRow:        10,
			EnclosingDecl: srcDecl,
			RelativePath:  "body",
		},
	}

	dstComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// step 1: initialize",
			StartRow:      7,
			EndRow:        7,
			EnclosingDecl: dstDecl,
			RelativePath:  "body",
		},
		{
			Type:          "comment",
			Text:          "// step 2: execute",
			StartRow:      12,
			EndRow:        12,
			EnclosingDecl: dstDecl,
			RelativePath:  "body",
		},
	}

	res := DiffComments(srcComments, dstComments, mappings)
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions for comments inside renamed function, got %d actions: %+v", len(res.Actions), res.Actions)
	}
	if res.LineMappings[5] != 7 || res.LineMappings[10] != 12 {
		t.Errorf("expected line mappings 5->7, 10->12; got %v", res.LineMappings)
	}
}

func TestDiffCommentsLeadingDocstringRenamed(t *testing.T) {
	srcDecl := &treesitter.ASTNode{ID: 10, Type: "function_declaration"}
	dstDecl := &treesitter.ASTNode{ID: 25, Type: "function_declaration"}

	mappings := engine.NewMapping()
	mappings.Add(srcDecl, dstDecl)

	srcComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// CalculateTotal computes total price",
			StartRow:      4,
			EndRow:        4,
			EnclosingDecl: srcDecl,
			RelativePath:  "doc",
		},
	}
	dstComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// CalculateTotal computes total price",
			StartRow:      8,
			EndRow:        8,
			EnclosingDecl: dstDecl,
			RelativePath:  "doc",
		},
	}

	res := DiffComments(srcComments, dstComments, mappings)
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions for docstring attached to renamed function, got %d actions", len(res.Actions))
	}
	if res.LineMappings[4] != 8 {
		t.Errorf("expected line mapping 4->8, got %v", res.LineMappings)
	}
}

func TestDiffCommentsBranchIsolation(t *testing.T) {
	srcDecl := &treesitter.ASTNode{ID: 10, Type: "function_declaration"}
	dstDecl := &treesitter.ASTNode{ID: 10, Type: "function_declaration"}

	mappings := engine.NewMapping()
	mappings.Add(srcDecl, dstDecl)

	srcComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// handle error",
			StartRow:      10,
			EndRow:        10,
			EnclosingDecl: srcDecl,
			RelativePath:  "body/if_statement/consequence",
		},
	}
	dstComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// handle error",
			StartRow:      20,
			EndRow:        20,
			EnclosingDecl: dstDecl,
			RelativePath:  "body/if_statement/alternative",
		},
	}

	res := DiffComments(srcComments, dstComments, mappings)
	// Because relative paths differ ("consequence" vs "alternative"), it should not match in Pass 1 LCS
	// Pass 2 handles cross-scope as Delete + Insert (since dist is 10 <= 25)
	if len(res.Actions) != 2 {
		t.Fatalf("expected 2 actions (1 Delete, 1 Insert) across different branches, got %d", len(res.Actions))
	}
}

func TestDiffCommentsUnmappedStrictIsolation(t *testing.T) {
	srcDecl := &treesitter.ASTNode{ID: 10, Type: "function_declaration"}
	dstDecl := &treesitter.ASTNode{ID: 99, Type: "function_declaration"}

	// Unmapped declarations (mappings does NOT map srcDecl to dstDecl)
	mappings := engine.NewMapping()

	srcComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// validate inputs",
			StartRow:      10,
			EndRow:        10,
			EnclosingDecl: srcDecl,
			RelativePath:  "body",
		},
	}
	dstComments := []CommentBlock{
		{
			Type:          "comment",
			Text:          "// validate inputs",
			StartRow:      100, // distant row > 25
			EndRow:        100,
			EnclosingDecl: dstDecl,
			RelativePath:  "body",
		},
	}

	res := DiffComments(srcComments, dstComments, mappings)
	if len(res.Actions) != 2 {
		t.Fatalf("expected 2 actions (1 Delete, 1 Insert) for unmapped distant declarations, got %d", len(res.Actions))
	}
	hasDelete := false
	hasInsert := false
	for _, act := range res.Actions {
		if act.Type == actions.Delete {
			hasDelete = true
		}
		if act.Type == actions.Insert {
			hasInsert = true
		}
	}
	if !hasDelete || !hasInsert {
		t.Errorf("expected 1 Delete and 1 Insert action, got: %+v", res.Actions)
	}
}

func TestExtractCommentsInsideFunctionNotDocComment(t *testing.T) {
	src := []byte(`
public class TestClass {
    public void releaseByteBuffer(int ix, byte[] buffer) {
        // 13-Jan-2024, tatu: [core#1186] Replace only if beneficial:
        byte[] oldBuffer = _byteBuffers.get(ix);
    }
}
`)

	ast, err := treesitter.ParseWithLanguage(src, "java")
	if err != nil {
		t.Fatal(err)
	}

	comments := ExtractComments(ast)
	if len(comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(comments))
	}
	c := comments[0]
	if c.DeclType != "method_declaration" {
		t.Errorf("expected DeclType to be 'method_declaration', got %q", c.DeclType)
	}
	if c.RelativePath == "doc" {
		t.Errorf("expected RelativePath NOT to be 'doc' for comment inside method body, got %q", c.RelativePath)
	}
}

func TestDiffCommentsCrossBlockFuzzyMatchRejection(t *testing.T) {
	declSrc := &treesitter.ASTNode{Type: "function_declaration", Language: "lua"}
	declDst := &treesitter.ASTNode{Type: "function_declaration", Language: "lua"}

	// Branch 1 in src and dst (mapped to each other)
	block1Src := &treesitter.ASTNode{Type: "block", Parent: declSrc, Language: "lua"}
	block1Dst := &treesitter.ASTNode{Type: "block", Parent: declDst, Language: "lua"}

	// Branch 2 in src and dst (mapped to each other)
	block2Src := &treesitter.ASTNode{Type: "block", Parent: declSrc, Language: "lua"}
	block2Dst := &treesitter.ASTNode{Type: "block", Parent: declDst, Language: "lua"}

	mappings := engine.NewMapping()
	mappings.Add(declSrc, declDst)
	mappings.Add(block1Src, block1Dst)
	mappings.Add(block2Src, block2Dst)

	// Comment in src block 2:
	stmt2Src := &treesitter.ASTNode{Type: "assignment_statement", Parent: block2Src, Language: "lua"}
	sc := CommentBlock{
		Text:          "-- TODO: Need to track exact whitespace length for each level.",
		StartRow:      10,
		EnclosingDecl: declSrc,
		AnchorNode:    stmt2Src,
	}

	// Comment in dst block 1 (similar wording, but in a completely different branch!):
	stmt1Dst := &treesitter.ASTNode{Type: "assignment_statement", Parent: block1Dst, Language: "lua"}
	dc := CommentBlock{
		Text:          "-- Track the leading whitespace for each indent level so that we can dedent",
		StartRow:      12,
		EnclosingDecl: declDst,
		AnchorNode:    stmt1Dst,
	}

	res := DiffComments([]CommentBlock{sc}, []CommentBlock{dc}, mappings)
	// Comments in separate control flow branches must not fuzzy-match into an update.
	if len(res.Actions) != 2 {
		t.Fatalf("expected 2 actions (1 Delete, 1 Insert) across different mapped blocks, got %d", len(res.Actions))
	}
	for _, act := range res.Actions {
		if act.Type == actions.Update {
			t.Errorf("expected cross-block fuzzy match to be rejected, but got an Update action")
		}
	}
}

func TestDiffCommentsGlobalBestScorePreventsGreedyTheft(t *testing.T) {
	// sc1 only shares a few generic words with dc (~45% similarity).
	sc1 := CommentBlock{
		Text:     "// means the parent container is intact and the child was simply reordered within it.",
		StartRow: 10,
		ScopeKey: "func_main",
	}

	// sc2 is clearly the right match (~75% similarity).
	sc2 := CommentBlock{
		Text:     "// Delimiter and structural containers (such as parentheses () and blocks {})\n// must never move if no surviving child moves with them into the destination container.",
		StartRow: 25,
		ScopeKey: "func_main",
	}

	dc := CommentBlock{
		Text:     "// Delimiter and structural containers must never move if no surviving child moves with them.",
		StartRow: 30,
		ScopeKey: "func_main",
	}

	res := DiffComments([]CommentBlock{sc1, sc2}, []CommentBlock{dc}, nil)

	// sc2 should take dc as an Update; sc1 gets left behind as a Delete.
	sc1Deleted := false
	sc2Updated := false
	for _, act := range res.Actions {
		if act.Type == actions.Update && act.Node != nil && act.Node.StartRow == uint32(sc2.StartRow) {
			sc2Updated = true
		}
		if act.Type == actions.Delete && act.Node != nil && act.Node.StartRow == uint32(sc1.StartRow) {
			sc1Deleted = true
		}
	}
	if !sc2Updated {
		t.Errorf("expected Update action between sc2 and dc, got actions: %+v", res.Actions)
	}
	if !sc1Deleted {
		t.Errorf("expected Delete action for sc1, got actions: %+v", res.Actions)
	}
}

func TestDiffCommentsAnchorFirstPrecedenceOverUnanchoredExactMatch(t *testing.T) {
	// Make sure an edited comment on a mapped statement isn't stolen by an
	// unmapped deleted statement that happens to share identical text.
	stmtSrc1 := &treesitter.ASTNode{Type: "assignment_statement", StartRow: 5, Language: "go"}
	stmtDst1 := &treesitter.ASTNode{Type: "assignment_statement", StartRow: 5, Language: "go"}
	stmtSrcDeleted := &treesitter.ASTNode{Type: "assignment_statement", StartRow: 10, Language: "go"}

	mappings := engine.NewMapping()
	mappings.Add(stmtSrc1, stmtDst1)

	sc1 := CommentBlock{
		Text:       "//   doB()",
		StartRow:   5,
		ScopeKey:   "func_main",
		AnchorNode: stmtSrc1,
	}
	scDeleted := CommentBlock{
		Text:       "// doB()",
		StartRow:   10,
		ScopeKey:   "func_main",
		AnchorNode: stmtSrcDeleted,
	}
	dc1 := CommentBlock{
		Text:       "// doB()",
		StartRow:   5,
		ScopeKey:   "func_main",
		AnchorNode: stmtDst1,
	}

	res := DiffComments([]CommentBlock{sc1, scDeleted}, []CommentBlock{dc1}, mappings)

	var sc1Updated, scDeletedDeleted bool
	for _, act := range res.Actions {
		if act.Type == actions.Update && act.Node != nil && act.Node.StartRow == uint32(sc1.StartRow) {
			sc1Updated = true
		}
		if act.Type == actions.Delete && act.Node != nil && act.Node.StartRow == uint32(scDeleted.StartRow) {
			scDeletedDeleted = true
		}
	}

	if !sc1Updated {
		t.Errorf("expected sc1 to update to dc1 on mapped anchor, got actions: %+v", res.Actions)
	}
	if !scDeletedDeleted {
		t.Errorf("expected scDeleted to be deleted, got actions: %+v", res.Actions)
	}
}

func TestDiffCommentsAnchorFirstGlobalBestScorePreventsGreedyTheft(t *testing.T) {
	// When multiple comments share the same mapped anchor, the pair with
	// higher similarity should match first, even if the lower similarity
	// comment appears earlier in document order.
	stmtSrc := &treesitter.ASTNode{Type: "function_declaration", StartRow: 10, Language: "go"}
	stmtDst := &treesitter.ASTNode{Type: "function_declaration", StartRow: 10, Language: "go"}

	mappings := engine.NewMapping()
	mappings.Add(stmtSrc, stmtDst)

	// sc1 appears first but only has ~45% similarity to dc.
	sc1 := CommentBlock{
		Text:       "// Process incoming request buffer and return formatted output.",
		StartRow:   8,
		ScopeKey:   "func_main",
		AnchorNode: stmtSrc,
	}
	// sc2 appears second but has ~90% similarity to dc.
	sc2 := CommentBlock{
		Text:       "// Handle incoming user request and return formatted output string.",
		StartRow:   9,
		ScopeKey:   "func_main",
		AnchorNode: stmtSrc,
	}

	dc := CommentBlock{
		Text:       "// Handle incoming user request and return formatted output text.",
		StartRow:   9,
		ScopeKey:   "func_main",
		AnchorNode: stmtDst,
	}

	res := DiffComments([]CommentBlock{sc1, sc2}, []CommentBlock{dc}, mappings)

	var sc2Updated, sc1Deleted bool
	for _, act := range res.Actions {
		if act.Type == actions.Update && act.Node != nil && act.Node.Label == "string" && act.Value == "text" {
			sc2Updated = true
		}
		if act.Type == actions.Delete && act.Node != nil && act.Node.Label == sc1.Text {
			sc1Deleted = true
		}
	}

	if !sc2Updated {
		t.Errorf("expected sc2 to update to dc due to higher similarity, got actions: %+v", res.Actions)
	}
	if !sc1Deleted {
		t.Errorf("expected sc1 to be deleted, got actions: %+v", res.Actions)
	}
}

func TestDiffCommentsAnchorFirst_FullEngine(t *testing.T) {
	t.Run("TrailingDelimiterAttachment", func(t *testing.T) {
		oldSrc := `package main

func main() {
	_ = []int{
		1, //   step
	}
	doDeleted() // step
}
`
		newSrc := `package main

func main() {
	_ = []int{
		1, // step
	}
}
`
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

		srcComments := ExtractComments(oldAST)
		dstComments := ExtractComments(newAST)

		// Verify trailing comma was skipped during trivia attachment so "//   step" has an AnchorNode.
		var foundTrailingWithAnchor bool
		for _, c := range srcComments {
			if strings.Contains(c.Text, "step") && c.AnchorNode != nil {
				foundTrailingWithAnchor = true
				break
			}
		}
		if !foundTrailingWithAnchor {
			t.Fatalf("expected trailing comment on line ending with comma to attach to an AnchorNode, got %+v", srcComments)
		}

		res := DiffComments(srcComments, dstComments, mr.Mappings)

		var stepUpdated, doDeletedDeleted bool
		for _, act := range res.Actions {
			if act.Type == actions.Update && act.Node != nil && act.Node.StartRow == 4 {
				stepUpdated = true
			}
			if act.Type == actions.Delete && act.Node != nil && strings.Contains(act.Node.Label, "step") && act.Node.StartRow == 6 {
				doDeletedDeleted = true
			}
		}

		if !stepUpdated {
			t.Errorf("expected '//   step' to update to '// step' on mapped anchor, got actions: %+v", res.Actions)
		}
		if !doDeletedDeleted {
			t.Errorf("expected doDeleted's '// step' to be deleted, got actions: %+v", res.Actions)
		}
	})

	t.Run("StatementAnchorPrecedenceOverUnanchoredExactMatch", func(t *testing.T) {
		oldSrc := `package main

func main() {
	x := 1      //   step
	doDeleted() // step
}
`
		newSrc := `package main

func main() {
	x := 1 // step
}
`
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

		srcComments := ExtractComments(oldAST)
		dstComments := ExtractComments(newAST)

		res := DiffComments(srcComments, dstComments, mr.Mappings)

		var stepUpdated, doDeletedDeleted bool
		for _, act := range res.Actions {
			if act.Type == actions.Update && act.Node != nil && act.Node.StartRow == 3 {
				stepUpdated = true
			}
			if act.Type == actions.Delete && act.Node != nil && strings.Contains(act.Node.Label, "step") && act.Node.StartRow == 4 {
				doDeletedDeleted = true
			}
		}

		if !stepUpdated {
			t.Errorf("expected 'x := 1' comment to update in-place on mapped anchor, got actions: %+v", res.Actions)
		}
		if !doDeletedDeleted {
			t.Errorf("expected doDeleted's comment to be deleted, got actions: %+v", res.Actions)
		}
	})
}

// TestCleanCommentContent verifies delimiter stripping across all 10 supported languages.
func TestCleanCommentContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "double slash line comment", input: "// Hello world", expected: "Hello world"},
		{name: "block comment", input: "/* hello world */", expected: "hello world"},
		{name: "docstring star prefix", input: " * param x", expected: "param x"},
		{name: "python hash comment", input: "# log error", expected: "log error"},
		{name: "lua dash comment", input: "-- lua comment", expected: "lua comment"},
		{name: "triple slash doc comment", input: "/// triple slash", expected: "triple slash"},
		{name: "leading and trailing whitespace", input: "   //  trimmed text   ", expected: "trimmed text"},
		{name: "empty comment", input: "//", expected: ""},
		{name: "empty block comment", input: "/* */", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanCommentContent(tt.input)
			if got != tt.expected {
				t.Errorf("cleanCommentContent(%q) = %q, expected %q", tt.input, got, tt.expected)
			}
		})
	}
}

// TestStringSimilarity_SubstantiveLengthGuard verifies that short comments require exact matching.
func TestStringSimilarity_SubstantiveLengthGuard(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		if sim := stringSimilarity("// ok", "// on"); sim != 0.0 {
			t.Errorf("expected 0.0 for '// ok' vs '// on', got %f", sim)
		}
		if sim := stringSimilarity("// a", "// b"); sim != 0.0 {
			t.Errorf("expected 0.0 for '// a' vs '// b', got %f", sim)
		}
		if sim := stringSimilarity("// ok", "// ok"); sim != 1.0 {
			t.Errorf("expected 1.0 for '// ok' vs '// ok', got %f", sim)
		}
		if sim := stringSimilarity("//", "//"); sim != 0.0 {
			t.Errorf("expected 0.0 for empty comments, got %f", sim)
		}

		sim := stringSimilarity("// TODO: fix", "// TODO: bug")
		if sim < 0.50 {
			t.Errorf("expected similarity >= 0.50 for '// TODO: fix' vs '// TODO: bug', got %f", sim)
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		oldSrc := []byte(`package main

func check() {
	before()
	// ok
	target("unchanged")
	after()
}
`)
		newSrc := []byte(`package main

func check() {
	before()
	// on
	target("unchanged")
	after()
}
`)

		oldAST, err := treesitter.Parse(oldSrc, "old.go")
		if err != nil {
			t.Fatalf("parse old source: %v", err)
		}
		newAST, err := treesitter.Parse(newSrc, "new.go")
		if err != nil {
			t.Fatalf("parse new source: %v", err)
		}
		if oldAST.ParseErrorCount != 0 || newAST.ParseErrorCount != 0 {
			t.Fatalf("expected parseable snippets, got old=%d new=%d errors", oldAST.ParseErrorCount, newAST.ParseErrorCount)
		}

		r := rules.Get("go")
		if r == nil {
			t.Fatal("missing Go rules")
		}
		findAtRow := func(root *treesitter.ASTNode, row uint32, matches func(string) bool) *treesitter.ASTNode {
			var found *treesitter.ASTNode
			var visit func(*treesitter.ASTNode)
			visit = func(node *treesitter.ASTNode) {
				if node == nil || found != nil {
					return
				}
				if node.StartRow == row && matches(node.Type) {
					found = node
					return
				}
				for _, child := range node.Children {
					visit(child)
				}
			}
			visit(root)
			return found
		}

		part := engine.NewLinePartition(oldSrc, newSrc)
		match := engine.Match(oldAST, newAST, oldSrc, newSrc, part)
		assertMapped := func(name string, src, dst *treesitter.ASTNode) {
			t.Helper()
			if src == nil || dst == nil {
				t.Fatalf("missing %s node: src=%v dst=%v", name, src, dst)
			}
			if got := match.Mappings.Src()[src]; got != dst {
				t.Errorf("%s did not map in place: got %v, want %v", name, got, dst)
			}
		}

		assertMapped("function declaration", findAtRow(oldAST, 2, r.IsDeclaration), findAtRow(newAST, 2, r.IsDeclaration))
		assertMapped("function block", findAtRow(oldAST, 2, r.IsBlock), findAtRow(newAST, 2, r.IsBlock))
		assertMapped("preceding statement", findAtRow(oldAST, 3, r.IsStatement), findAtRow(newAST, 3, r.IsStatement))
		assertMapped("target statement", findAtRow(oldAST, 5, r.IsStatement), findAtRow(newAST, 5, r.IsStatement))
		assertMapped("following statement", findAtRow(oldAST, 6, r.IsStatement), findAtRow(newAST, 6, r.IsStatement))
		assertMapped("target call", findAtRow(oldAST, 5, r.IsCall), findAtRow(newAST, 5, r.IsCall))
		assertMapped("target arguments", findAtRow(oldAST, 5, r.IsDelimitedContainer), findAtRow(newAST, 5, r.IsDelimitedContainer))

		srcComments := ExtractComments(oldAST)
		dstComments := ExtractComments(newAST)
		if len(srcComments) != 1 || len(dstComments) != 1 {
			t.Fatalf("expected one comment on each side, got src=%d dst=%d", len(srcComments), len(dstComments))
		}
		if srcComments[0].AnchorNode == nil || dstComments[0].AnchorNode == nil {
			t.Fatalf("expected comments to remain anchored, got src=%+v dst=%+v", srcComments[0], dstComments[0])
		}
		assertMapped("comment anchor", srcComments[0].AnchorNode, dstComments[0].AnchorNode)

		result := DiffComments(srcComments, dstComments, match.Mappings)
		if len(result.Actions) != 2 {
			t.Fatalf("expected delete and insert for short rewritten comments, got %+v", result.Actions)
		}
		var hasDelete, hasInsert bool
		for _, action := range result.Actions {
			hasDelete = hasDelete || action.Type == actions.Delete && action.Node.Label == "// ok"
			hasInsert = hasInsert || action.Type == actions.Insert && action.Node.Label == "// on"
		}
		if !hasDelete || !hasInsert {
			t.Errorf("expected '// ok' delete and '// on' insert, got %+v", result.Actions)
		}
	})
}

// TestDiffComments_WordLevelSingleLine verifies that single-line comment edits
// isolate the modified words and leave surrounding comment syntax unhighlighted.
func TestDiffComments_WordLevelSingleLine(t *testing.T) {
	srcText := "// timeout is 30ms"
	dstText := "// timeout is 60ms"

	srcComments := []CommentBlock{
		{Type: "comment", Text: srcText, StartRow: 5, StartCol: 4, StartByte: 50, EndByte: 50 + uint32(len(srcText)), Language: "go"},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: dstText, StartRow: 5, StartCol: 4, StartByte: 50, EndByte: 50 + uint32(len(dstText)), Language: "go"},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 1 {
		t.Fatalf("expected exactly 1 word update action, got %d", len(res.Actions))
	}
	act := res.Actions[0]
	if act.Type != actions.Update {
		t.Fatalf("expected Update action, got %v", act.Type)
	}
	if act.Node.Label != "30ms" {
		t.Errorf("expected update src label '30ms', got %q", act.Node.Label)
	}
	if act.Value != "60ms" {
		t.Errorf("expected update dest value '60ms', got %q", act.Value)
	}
	if act.DestNode == nil || act.DestNode.Label != "60ms" {
		t.Errorf("expected DestNode label '60ms', got %+v", act.DestNode)
	}

	// Verify byte offsets: "// timeout is " is 14 bytes, so "30ms" starts at 50 + 14 = 64
	expectedStart := uint32(50 + 14)
	expectedEnd := expectedStart + 4
	if act.Node.StartByte != expectedStart || act.Node.EndByte != expectedEnd {
		t.Errorf("expected src byte range [%d, %d], got [%d, %d]", expectedStart, expectedEnd, act.Node.StartByte, act.Node.EndByte)
	}
	if act.DestNode.StartByte != expectedStart || act.DestNode.EndByte != expectedEnd {
		t.Errorf("expected dst byte range [%d, %d], got [%d, %d]", expectedStart, expectedEnd, act.DestNode.StartByte, act.DestNode.EndByte)
	}

	// Verify row and column mapping
	if act.Node.StartRow != 5 || act.DestNode.StartRow != 5 {
		t.Errorf("expected row 5 on both nodes, got src %d, dst %d", act.Node.StartRow, act.DestNode.StartRow)
	}
	expectedCol := uint32(4 + 14)
	if act.Node.StartCol != expectedCol || act.DestNode.StartCol != expectedCol {
		t.Errorf("expected col %d on both nodes, got src %d, dst %d", expectedCol, act.Node.StartCol, act.DestNode.StartCol)
	}
	if res.LineMappings[5] != 5 {
		t.Errorf("expected line mapping 5 -> 5, got %v", res.LineMappings)
	}
}

// TestDiffComments_WordLevelMultiLine verifies that modified lines in multiline block
// comments produce fine-grained word updates rather than whole-line updates.
func TestDiffComments_WordLevelMultiLine(t *testing.T) {
	oldComment := "/*\n * Line 1\n * timeout is 30ms\n * Line 3\n */"
	newComment := "/*\n * Line 1\n * timeout is 60ms\n * Line 3\n */"

	srcComments := []CommentBlock{
		{Type: "block_comment", Text: oldComment, StartRow: 10, StartByte: 100, EndByte: 100 + uint32(len(oldComment)), Language: "c"},
	}
	dstComments := []CommentBlock{
		{Type: "block_comment", Text: newComment, StartRow: 10, StartByte: 100, EndByte: 100 + uint32(len(newComment)), Language: "c"},
	}

	res := DiffComments(srcComments, dstComments, nil)
	if len(res.Actions) != 1 {
		t.Fatalf("expected exactly 1 word update action in multiline block, got %d (actions: %+v)", len(res.Actions), res.Actions)
	}
	act := res.Actions[0]
	if act.Type != actions.Update {
		t.Fatalf("expected Update action, got %v", act.Type)
	}
	if act.Node.Label != "30ms" {
		t.Errorf("expected src node label '30ms', got %q", act.Node.Label)
	}
	if act.Value != "60ms" {
		t.Errorf("expected dest value '60ms', got %q", act.Value)
	}
	if act.Node.StartRow != 12 || act.DestNode.StartRow != 12 {
		t.Errorf("expected row 12 for the 3rd line, got src %d, dst %d", act.Node.StartRow, act.DestNode.StartRow)
	}
}

// TestCommentsAcrossLanguages checks doc comment scoping across all languages
// for plain and decorated fields, methods, and functions.
func TestCommentsAcrossLanguages(t *testing.T) {
	langs := []struct {
		lang string
		file string
		src  string
	}{
		{
			lang: "java",
			file: "Test.java",
			src: `class Foo {
    /** Doc comment on field */
    @Deprecated
    private int bar;

    /** Doc comment on method */
    @Override
    public void baz() {}
}`,
		},
		{
			lang: "python",
			file: "test.py",
			src: `# Doc comment on fn 1
def foo():
    pass

# Doc comment on fn 2
@staticmethod
def bar():
    pass
`,
		},
		{
			lang: "typescript",
			file: "test.ts",
			src: `class Foo {
    /** Doc comment on property */
    @Input()
    bar: string;

    /** Doc comment on method */
    @HostListener("click")
    baz() {}
}`,
		},
		{
			lang: "c",
			file: "test.c",
			src: `struct Foo {
    /** Doc comment on field */
    int bar;
};

/** Doc comment on fn */
void baz(void) {}
`,
		},
		{
			lang: "cpp",
			file: "test.cpp",
			src: `struct Foo {
    /** Doc comment on field */
    int bar;
};

/** Doc comment on fn */
int baz() { return 0; }
`,
		},
		{
			lang: "go",
			file: "test.go",
			src: `package main

type Foo struct {
    // Doc comment on field
    Bar int
}

// Doc comment on fn
func Baz() {}
`,
		},
		{
			lang: "zig",
			file: "test.zig",
			src: `const Foo = struct {
    /// Doc comment on field
    bar: i32,

    /// Doc comment on fn
    pub fn baz() void {}
};
`,
		},
		{
			lang: "lua",
			file: "test.lua",
			src: `local obj = {
    -- Doc comment on field
    field = 1,
}

-- Doc comment on fn
function baz()
end
`,
		},
	}

	for _, tc := range langs {
		t.Run(tc.lang, func(t *testing.T) {
			ast, err := treesitter.Parse([]byte(tc.src), tc.file)
			if err != nil {
				t.Fatalf("parse %s: %v", tc.file, err)
			}
			comments := ExtractComments(ast)
			if len(comments) != 2 {
				t.Fatalf("expected 2 comments for %s, got %d", tc.lang, len(comments))
			}
			for i, c := range comments {
				if c.EnclosingDecl == nil {
					t.Errorf("[%s] comment %d: EnclosingDecl should not be nil", tc.lang, i)
				}
				if c.AnchorNode == nil {
					t.Errorf("[%s] comment %d: AnchorNode should not be nil", tc.lang, i)
				}
				if tc.lang != "lua" || i == 1 {
					if c.RelativePath != "doc" {
						t.Errorf("[%s] comment %d: expected RelativePath 'doc', got %q", tc.lang, i, c.RelativePath)
					}
				}
			}
		})
	}
}

func TestAttributedDeclarationCommentScoping(t *testing.T) {
	t.Run("Unit", func(t *testing.T) {
		src := `pub struct Opt {
    /// The width of decorations.
    #[structopt(short = "w", long = "width")]
    pub width: Option<String>,

    /// Whether to use a pager when displaying output.
    /// default pager is less: this can be altered by setting the environment variables
    #[structopt(long = "paging", default_value = "auto")]
    pub paging_mode: String,
}
`
		ast, err := treesitter.Parse([]byte(src), "test.rs")
		if err != nil {
			t.Fatalf("parse test.rs: %v", err)
		}

		comments := ExtractComments(ast)
		if len(comments) != 3 {
			t.Fatalf("expected 3 comments, got %d", len(comments))
		}

		// Comment 0: "/// The width of decorations."
		c0 := comments[0]
		if c0.EnclosingDecl == nil || c0.EnclosingDecl.Type != "field_declaration" {
			t.Errorf("expected comment 0 EnclosingDecl to be field_declaration, got %v", c0.EnclosingDecl)
		}
		if c0.RelativePath != "doc" {
			t.Errorf("expected comment 0 RelativePath to be 'doc', got %q", c0.RelativePath)
		}
		if c0.AnchorNode == nil || c0.AnchorNode.Type != "field_declaration" {
			t.Errorf("expected comment 0 AnchorNode to be field_declaration, got %v", c0.AnchorNode)
		}

		// Comments 1 & 2: on paging_mode
		for i, ci := range comments[1:3] {
			idx := i + 1
			if ci.EnclosingDecl == nil || ci.EnclosingDecl.Type != "field_declaration" {
				t.Errorf("expected comment %d EnclosingDecl to be field_declaration, got %v", idx, ci.EnclosingDecl)
			}
			if ci.RelativePath != "doc" {
				t.Errorf("expected comment %d RelativePath to be 'doc', got %q", idx, ci.RelativePath)
			}
			if ci.AnchorNode == nil || ci.AnchorNode.Type != "field_declaration" {
				t.Errorf("expected comment %d AnchorNode to be field_declaration, got %v", idx, ci.AnchorNode)
			}
			// Must have a different declaration ID than comment 0 (different fields)
			if ci.EnclosingDecl.ID == c0.EnclosingDecl.ID {
				t.Errorf("comment %d and comment 0 should belong to distinct field declarations, both have ID %d", idx, c0.EnclosingDecl.ID)
			}
		}
	})

	t.Run("FullEngine", func(t *testing.T) {
		t.Run("Rust", func(t *testing.T) {
			oldSrc := `pub struct Opt {
    /// The width of underline/overline decorations. Use --width=variable to extend
    #[structopt(short = "w", long = "width")]
    pub width: Option<String>,

    /// Whether to use a pager when displaying output. Options are: auto, always, and never. The
    /// default pager is less: this can be altered by setting the environment variables
    #[structopt(long = "paging", default_value = "auto")]
    pub paging_mode: String,
}
`
			newSrc := `pub struct Opt {
    /// The width of underline/overline decorations. Examples: 72
    // (equivalent to 72 but may be useful if the caller has a variable holding the value 74).
    #[structopt(short = "w", long = "width")]
    pub width: Option<String>,

    /// Whether to use a pager when displaying output. Options are: auto, always, and never.
    #[structopt(long = "paging", default_value = "auto")]
    pub paging_mode: String,
}
`
			oldAST, err := treesitter.Parse([]byte(oldSrc), "test.rs")
			if err != nil {
				t.Fatalf("parse oldSrc: %v", err)
			}
			newAST, err := treesitter.Parse([]byte(newSrc), "test.rs")
			if err != nil {
				t.Fatalf("parse newSrc: %v", err)
			}

			part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
			mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)

			// 1. Enclosing Context: struct Opt must map in place
			if mr.Mappings.Src()[oldAST] != newAST {
				t.Errorf("expected struct_item to map in place")
			}

			// 2. Surrounding Sequence & Target Declarations: all declarations must map in place
			r := rules.Get("rust")
			for _, n := range oldAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					if mr.Mappings.Src()[n] == nil {
						t.Errorf("expected declaration %s at row %d to map in place", n.Type, n.StartRow)
					}
				}
			}
			srcComments := ExtractComments(oldAST)
			dstComments := ExtractComments(newAST)
			res := DiffComments(srcComments, dstComments, mr.Mappings)

			var foundCrossFieldUpdate bool
			var foundDeletedPagerLine bool
			var foundInsertedWidthLine bool

			for _, act := range res.Actions {
				if act.Type == actions.Update {
					// Make sure the pager comment doesn't pair with the width comment across fields.
					if (act.Node != nil && act.DestNode != nil && act.Node.StartRow == 6 && act.DestNode.StartRow == 2) ||
						((strings.Contains(act.Node.Label, "default") || strings.Contains(act.Node.Label, "pager")) &&
							(strings.Contains(act.DestNode.Label, "equivalent") || strings.Contains(act.DestNode.Label, "holding"))) {
						foundCrossFieldUpdate = true
					}
				}
				if act.Type == actions.Delete && strings.Contains(act.Node.Label, "default pager") {
					foundDeletedPagerLine = true
				}
				if act.Type == actions.Insert && strings.Contains(act.Node.Label, "equivalent to 72") {
					foundInsertedWidthLine = true
				}
			}

			if foundCrossFieldUpdate {
				t.Errorf("FATAL: false cross-field update detected between pager comment and width comment! Actions: %+v", res.Actions)
			}
			if !foundDeletedPagerLine {
				t.Errorf("expected pager comment line to be emitted as Delete action, got actions: %+v", res.Actions)
			}
			if !foundInsertedWidthLine {
				t.Errorf("expected width comment line to be emitted as Insert action, got actions: %+v", res.Actions)
			}
		})

		t.Run("TypeScript", func(t *testing.T) {
			oldSrc := `class Component {
    /** The width of decorations. */
    @HostListener("click")
    onClick() {
        console.log("clicked");
    }

    /** Options are: auto, always, and never. */
    /** The default pager is less: this can be altered by setting the environment variables */
    @HostListener("keydown")
    onKeydown() {
        console.log("keydown");
    }
}
`
			newSrc := `class Component {
    /** The width of decorations. */
    /** (equivalent to 72 but may be useful if the caller has a variable holding the value 74). */
    @HostListener("click")
    onClick() {
        console.log("clicked");
    }

    /** Options are: auto, always, and never. */
    @HostListener("keydown")
    onKeydown() {
        console.log("keydown");
    }
}
`
			oldAST, err := treesitter.Parse([]byte(oldSrc), "test.ts")
			if err != nil {
				t.Fatalf("parse oldSrc: %v", err)
			}
			newAST, err := treesitter.Parse([]byte(newSrc), "test.ts")
			if err != nil {
				t.Fatalf("parse newSrc: %v", err)
			}

			part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
			mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)

			// 1. Enclosing Context: class Component must map in place
			if mr.Mappings.Src()[oldAST] != newAST {
				t.Errorf("expected class_declaration to map in place")
			}

			// 2. Surrounding Sequence & Target Declarations: all declarations must map in place
			r := rules.Get("typescript")
			for _, n := range oldAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					if mr.Mappings.Src()[n] == nil {
						t.Errorf("expected declaration %s at row %d to map in place", n.Type, n.StartRow)
					}
				}
			}
			srcComments := ExtractComments(oldAST)
			dstComments := ExtractComments(newAST)
			res := DiffComments(srcComments, dstComments, mr.Mappings)

			var foundCrossMethodUpdate bool
			var foundDeletedPagerLine bool
			var foundInsertedWidthLine bool

			for _, act := range res.Actions {
				if act.Type == actions.Update {
					if (strings.Contains(act.Node.Label, "default") || strings.Contains(act.Node.Label, "pager")) &&
						(strings.Contains(act.DestNode.Label, "equivalent") || strings.Contains(act.DestNode.Label, "holding")) {
						foundCrossMethodUpdate = true
					}
				}
				if act.Type == actions.Delete && strings.Contains(act.Node.Label, "default pager") {
					foundDeletedPagerLine = true
				}
				if act.Type == actions.Insert && strings.Contains(act.Node.Label, "equivalent to 72") {
					foundInsertedWidthLine = true
				}
			}

			if foundCrossMethodUpdate {
				t.Errorf("FATAL: false cross-method update detected in TypeScript! Actions: %+v", res.Actions)
			}
			if !foundDeletedPagerLine {
				t.Errorf("expected pager comment line to be emitted as Delete action, got actions: %+v", res.Actions)
			}
			if !foundInsertedWidthLine {
				t.Errorf("expected width comment line to be emitted as Insert action, got actions: %+v", res.Actions)
			}
		})

		t.Run("Python", func(t *testing.T) {
			oldSrc := `class Controller:
    # The width of decorations.
    @app.route("/first")
    def handle_first(self):
        return "first"

    # Options are: auto, always, and never.
    # The default pager is less: this can be altered by setting the environment variables
    @app.route("/second")
    def handle_second(self):
        return "second"
`
			newSrc := `class Controller:
    # The width of decorations.
    # (equivalent to 72 but may be useful if the caller has a variable holding the value 74).
    @app.route("/first")
    def handle_first(self):
        return "first"

    # Options are: auto, always, and never.
    @app.route("/second")
    def handle_second(self):
        return "second"
`
			oldAST, err := treesitter.Parse([]byte(oldSrc), "test.py")
			if err != nil {
				t.Fatalf("parse oldSrc: %v", err)
			}
			newAST, err := treesitter.Parse([]byte(newSrc), "test.py")
			if err != nil {
				t.Fatalf("parse newSrc: %v", err)
			}

			part := engine.NewLinePartition([]byte(oldSrc), []byte(newSrc))
			mr := engine.Match(oldAST, newAST, []byte(oldSrc), []byte(newSrc), part)

			// 1. Enclosing Context: class Controller must map in place
			if mr.Mappings.Src()[oldAST] != newAST {
				t.Errorf("expected class_definition to map in place")
			}

			// 2. Surrounding Sequence & Target Declarations: all declarations must map in place
			r := rules.Get("python")
			for _, n := range oldAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					if mr.Mappings.Src()[n] == nil {
						t.Errorf("expected declaration %s at row %d to map in place", n.Type, n.StartRow)
					}
				}
			}

			srcComments := ExtractComments(oldAST)
			dstComments := ExtractComments(newAST)
			res := DiffComments(srcComments, dstComments, mr.Mappings)

			var foundCrossMethodUpdate bool
			var foundDeletedPagerLine bool
			var foundInsertedWidthLine bool

			for _, act := range res.Actions {
				if act.Type == actions.Update {
					if (strings.Contains(act.Node.Label, "default") || strings.Contains(act.Node.Label, "pager")) &&
						(strings.Contains(act.DestNode.Label, "equivalent") || strings.Contains(act.DestNode.Label, "holding")) {
						foundCrossMethodUpdate = true
					}
				}
				if act.Type == actions.Delete && strings.Contains(act.Node.Label, "default pager") {
					foundDeletedPagerLine = true
				}
				if act.Type == actions.Insert && strings.Contains(act.Node.Label, "equivalent to 72") {
					foundInsertedWidthLine = true
				}
			}

			if foundCrossMethodUpdate {
				t.Errorf("FATAL: false cross-method update detected in Python! Actions: %+v", res.Actions)
			}
			if !foundDeletedPagerLine {
				t.Errorf("expected pager comment line to be emitted as Delete action, got actions: %+v", res.Actions)
			}
			if !foundInsertedWidthLine {
				t.Errorf("expected width comment line to be emitted as Insert action, got actions: %+v", res.Actions)
			}
		})

		t.Run("Zig", func(t *testing.T) {
			oldSrc := `const Config = struct {
    /// The width of decorations.
    width: u32,

    /// Options are: auto, always, and never.
    /// The default pager is less: this can be altered by setting the environment variables
    paging: []const u8,
};
`
			newSrc := `const Config = struct {
    /// The width of decorations.
    /// (equivalent to 72 but may be useful if the caller has a variable holding the value 74).
    width: u32,

    /// Options are: auto, always, and never.
    paging: []const u8,
};
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

			// 1. Enclosing Context: struct Config must map in place
			if mr.Mappings.Src()[oldAST] != newAST {
				t.Errorf("expected struct to map in place")
			}

			// 2. Surrounding Sequence & Target Declarations: all declarations must map in place
			r := rules.Get("zig")
			for _, n := range oldAST.Descendants() {
				if r.IsDeclaration(n.Type) {
					if mr.Mappings.Src()[n] == nil {
						t.Errorf("expected declaration %s at row %d to map in place", n.Type, n.StartRow)
					}
				}
			}

			srcComments := ExtractComments(oldAST)
			dstComments := ExtractComments(newAST)
			res := DiffComments(srcComments, dstComments, mr.Mappings)

			var foundCrossFieldUpdate bool
			var foundDeletedPagerLine bool
			var foundInsertedWidthLine bool

			for _, act := range res.Actions {
				if act.Type == actions.Update {
					if (strings.Contains(act.Node.Label, "default") || strings.Contains(act.Node.Label, "pager")) &&
						(strings.Contains(act.DestNode.Label, "equivalent") || strings.Contains(act.DestNode.Label, "holding")) {
						foundCrossFieldUpdate = true
					}
				}
				if act.Type == actions.Delete && strings.Contains(act.Node.Label, "default pager") {
					foundDeletedPagerLine = true
				}
				if act.Type == actions.Insert && strings.Contains(act.Node.Label, "equivalent to 72") {
					foundInsertedWidthLine = true
				}
			}

			if foundCrossFieldUpdate {
				t.Errorf("FATAL: false cross-field update detected in Zig! Actions: %+v", res.Actions)
			}
			if !foundDeletedPagerLine {
				t.Errorf("expected pager comment line to be emitted as Delete action, got actions: %+v", res.Actions)
			}
			if !foundInsertedWidthLine {
				t.Errorf("expected width comment line to be emitted as Insert action, got actions: %+v", res.Actions)
			}
		})
	})
}
