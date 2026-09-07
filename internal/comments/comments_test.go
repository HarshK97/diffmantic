package comments

import (
	"os"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/treesitter"
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
	if act.Value != "// New Comment" {
		t.Errorf("unexpected update value: %q", act.Value)
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
	if act.Node.Label != " * Line 2 Old" || act.Value != " * Line 2 New" {
		t.Errorf("expected line update on line 2, got %q -> %q", act.Node.Label, act.Value)
	}
	if act.DestNode == nil {
		t.Errorf("expected DestNode to be set on line update action")
	}
}

func TestExtractCommentsWithTreeSitter(t *testing.T) {
	src := []byte("package main\n\n// Line comment 1\nfunc main() {\n\t// Line comment 2\n}\n")

	_, flatNodes, symbols, err := treesitter.ParseForPipeline(src, "go")
	if err != nil {
		t.Fatal(err)
	}

	comments := ExtractComments(flatNodes, symbols, src, "go")
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
	if res.Actions[0].Type != actions.Update {
		t.Errorf("expected Update action, got %v", res.Actions[0].Type)
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

func TestDiffCommentsGuzzlePhp(t *testing.T) {
	src, err := os.ReadFile("../../tests/testdata/php_guzzle_handler_curl_multi/old.php")
	if err != nil {
		t.Fatal(err)
	}
	dst, err := os.ReadFile("../../tests/testdata/php_guzzle_handler_curl_multi/new.php")
	if err != nil {
		t.Fatal(err)
	}

	_, flatNodesA, symbolsA, err := treesitter.ParseForPipeline(src, "php")
	if err != nil {
		t.Fatal(err)
	}
	_, flatNodesB, symbolsB, err := treesitter.ParseForPipeline(dst, "php")
	if err != nil {
		t.Fatal(err)
	}

	srcComments := ExtractComments(flatNodesA, symbolsA, src, "php")
	dstComments := ExtractComments(flatNodesB, symbolsB, dst, "php")

	res := DiffComments(srcComments, dstComments, nil)
	moveCount := 0
	for _, a := range res.Actions {
		if a.Type == actions.Move {
			moveCount++
		}
	}
	if moveCount != 0 {
		t.Errorf("expected 0 Move actions for comment trivia in Guzzle PHP diff, got %d", moveCount)
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

func TestSyntheticCommentLineNodeInvariants(t *testing.T) {
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

	node := createCommentLineNode(&cb, " * line 2 */", 20, 32, 3, cb.Language)
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

	_, flatNodes, symbols, err := treesitter.ParseForPipeline(src, "java")
	if err != nil {
		t.Fatal(err)
	}

	comments := ExtractComments(flatNodes, symbols, src, "java")
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
