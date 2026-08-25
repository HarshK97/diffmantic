package comments

import (
	"os"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/odvcencio/gotreesitter"
)

func TestDiffCommentsIdentical(t *testing.T) {
	srcComments := []CommentBlock{
		{Type: "comment", Text: "// Hello World", StartRow: 5, StartByte: 10, EndByte: 24},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// Hello World", StartRow: 8, StartByte: 15, EndByte: 29},
	}

	res := DiffComments(srcComments, dstComments)
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

	res := DiffComments(srcComments, dstComments)
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

	res := DiffComments(srcComments, dstComments)

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
	lang, err := treesitter.DetectLanguage("test.go")
	if err != nil {
		t.Fatal(err)
	}
	rules := treesitter.GetRules("go")
	src := []byte("package main\n\n// Line comment 1\nfunc main() {\n\t// Line comment 2\n}\n")

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	comments := ExtractComments(tree.RootNode(), src, lang, rules)
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments extracted, got %d", len(comments))
	}
	if comments[0].Text != "// Line comment 1" {
		t.Errorf("expected '// Line comment 1', got %q", comments[0].Text)
	}
	if comments[1].Text != "// Line comment 2" {
		t.Errorf("expected '// Line comment 2', got %q", comments[1].Text)
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

	res := DiffComments(srcComments, dstComments)
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

	res := DiffComments(srcComments, dstComments)
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
		{Type: "comment", Text: "// Optional params comment", ScopeKey: "method_declaration:getParamsToSign", StartRow: 162, EndRow: 162},
	}
	dstComments := []CommentBlock{
		{Type: "comment", Text: "// Optional params comment", ScopeKey: "method_declaration:getOauthParams", StartRow: 159, EndRow: 159},
	}

	res := DiffComments(srcComments, dstComments)
	if len(res.Actions) != 1 {
		t.Fatalf("expected 1 Move action for comment moved across scopes, got %d actions", len(res.Actions))
	}
	if res.Actions[0].Type != actions.Move {
		t.Errorf("expected Move action, got %v", res.Actions[0].Type)
	}
}

func TestDiffCommentsGuzzlePhp(t *testing.T) {
	lang, err := treesitter.DetectLanguage("test.php")
	if err != nil {
		t.Fatal(err)
	}
	r := treesitter.GetRules("php")
	src, err := os.ReadFile("../../tests/testdata/php_guzzle_handler_curl_multi/old.php")
	if err != nil {
		t.Fatal(err)
	}
	dst, err := os.ReadFile("../../tests/testdata/php_guzzle_handler_curl_multi/new.php")
	if err != nil {
		t.Fatal(err)
	}

	parser := gotreesitter.NewParser(lang)
	treeA, err := parser.Parse(src)
	if err != nil {
		t.Fatal(err)
	}
	treeB, err := parser.Parse(dst)
	if err != nil {
		t.Fatal(err)
	}

	srcComments := ExtractComments(treeA.RootNode(), src, lang, r)
	dstComments := ExtractComments(treeB.RootNode(), dst, lang, r)

	res := DiffComments(srcComments, dstComments)
	moveCount := 0
	for _, a := range res.Actions {
		if a.Type == actions.Move && strings.Contains(a.Node.Label, "Optional parameters") {
			moveCount++
		}
	}
	if moveCount != 1 {
		t.Errorf("expected 1 Move action for Optional parameters comment, got %d", moveCount)
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

	res := DiffComments(srcComments, dstComments)
	if len(res.Actions) != 0 {
		t.Fatalf("expected 0 actions for matched identical comments, got %d", len(res.Actions))
	}
	if res.LineMappings[10] != 12 || res.LineMappings[20] != 22 || res.LineMappings[30] != 32 {
		t.Errorf("expected mappings 10->12, 20->22, 30->32; got %v", res.LineMappings)
	}
}
