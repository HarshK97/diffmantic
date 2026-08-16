package treesitter

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestASTIndexMatchesRecursiveTraversal(t *testing.T) {
	fixtures := []string{
		"c_redis_write_handler",
		"cpp_fmt_avoid_instantiating_formatter_const",
		"css_mdn_box_model_sizing_example",
		"go_gin_fix_lint",
		"html_mdn_stopwatch_controls",
		"java_commons_lang_reject_non_ascii",
		"js_express_call_callback",
		"json_schemastore_workflow_step_reorder",
		"jsx_express_feat_add",
		"lua_neovim_ui2_messages",
		"php_guzzle_client_pool_options",
		"py_requests_make_json",
		"ruby_sinatra_add_regression_test_for_conten_2",
		"rust_tokio_time_test_util",
		"toml_cargo_fix_comment_typo",
		"ts_zod_json_schema",
		"tsx_zod_docs",
		"yaml_microservices_kustomization_scalar",
		"zig_clap_compile_error_when",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			for _, side := range []string{"old", "new"} {
				path := fixtureFile(t, fixture, side)
				src, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("reading %s: %v", path, err)
				}
				root, err := Parse(src, path)
				if err != nil {
					t.Fatalf("parsing %s: %v", path, err)
				}
				assertIndexedTreeMatchesRecursive(t, root)
			}
		})
	}
}

func fixtureFile(t *testing.T, fixture, prefix string) string {
	t.Helper()
	dir := filepath.Join("..", "..", "tests", "testdata", fixture)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("fixture directory %s not present: %v", fixture, err)
		return ""
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), prefix+".") {
			return filepath.Join(dir, entry.Name())
		}
	}
	t.Fatalf("fixture %s missing %s.*", fixture, prefix)
	return ""
}

func assertIndexedTreeMatchesRecursive(t *testing.T, root *ASTNode) {
	t.Helper()
	idx := root.Index
	if idx == nil {
		t.Fatal("parsed tree has no ASTIndex")
	}

	refPre := recursivePreOrder(root)
	refPost := recursivePostOrder(root)
	if got := root.PreOrder(); !sameNodes(got, refPre) {
		t.Fatal("root PreOrder differs from recursive traversal")
	}
	if got := root.PostOrder(); !sameNodes(got, refPost) {
		t.Fatal("root PostOrder differs from recursive traversal")
	}

	for _, n := range refPre {
		if n.Index != idx {
			t.Fatalf("node %s has wrong index pointer", n.Type)
		}
		refDesc := recursiveDescendants(n)
		if got := n.Descendants(); !sameNodes(got, refDesc) {
			t.Fatalf("%s descendants differ", n.Type)
		}
		if got := n.Size() - 1; got != len(refDesc) {
			t.Fatalf("%s descendant count = %d, want %d", n.Type, got, len(refDesc))
		}
		if got := n.Size(); got != len(refDesc)+1 {
			t.Fatalf("%s Size = %d, want %d", n.Type, got, len(refDesc)+1)
		}
		if got := n.PreOrder(); !sameNodes(got, recursivePreOrder(n)) {
			t.Fatalf("%s PreOrder differs", n.Type)
		}
		if got := n.PostOrder(); !sameNodes(got, recursivePostOrder(n)) {
			t.Fatalf("%s PostOrder differs", n.Type)
		}

		descSet := make(map[*ASTNode]bool, len(refDesc))
		for _, d := range refDesc {
			descSet[d] = true
		}
		for _, candidate := range refPre {
			if got, want := n.Contains(candidate), descSet[candidate]; got != want {
				t.Fatalf("%s Contains(%s) = %v, want %v", n.Type, candidate.Type, got, want)
			}
		}

		refLabels := recursiveLeafLabels(n)
		for label, want := range refLabels {
			if got := n.FrequencyInSubtree(label); got != want {
				t.Fatalf("%s FrequencyInSubtree(%q) = %d, want %d", n.Type, label, got, want)
			}
		}
		for label := range idx.LabelIndex {
			if _, ok := refLabels[label]; !ok {
				if got := n.FrequencyInSubtree(label); got != 0 {
					t.Fatalf("%s FrequencyInSubtree(%q) = %d, want 0", n.Type, label, got)
				}
			}
		}
		if got := n.LeafLabels(); !reflect.DeepEqual(got, refLabels) {
			t.Fatalf("%s LeafLabels = %#v, want %#v", n.Type, got, refLabels)
		}
	}
}

func sameNodes(a, b []*ASTNode) bool {
	return slices.Equal(a, b)
}

func recursivePreOrder(n *ASTNode) []*ASTNode {
	var out []*ASTNode
	var walk func(*ASTNode)
	walk = func(curr *ASTNode) {
		out = append(out, curr)
		for _, child := range curr.Children {
			walk(child)
		}
	}
	walk(n)
	return out
}

func recursivePostOrder(n *ASTNode) []*ASTNode {
	var out []*ASTNode
	var walk func(*ASTNode)
	walk = func(curr *ASTNode) {
		for _, child := range curr.Children {
			walk(child)
		}
		out = append(out, curr)
	}
	walk(n)
	return out
}

func recursiveDescendants(n *ASTNode) []*ASTNode {
	out := recursivePreOrder(n)
	return slices.Delete(out, 0, 1)
}

func recursiveLeafLabels(n *ASTNode) map[string]int {
	labels := make(map[string]int)
	var walk func(*ASTNode)
	walk = func(curr *ASTNode) {
		if len(curr.Children) == 0 && curr.Label != "" {
			labels[curr.Label]++
			return
		}
		for _, child := range curr.Children {
			walk(child)
		}
	}
	walk(n)
	return labels
}
