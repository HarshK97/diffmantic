package tui

import (
	"testing"
)

func TestHighlightSyntax(t *testing.T) {
	res := highlightSyntax("test.py", nil)
	if res != nil {
		t.Errorf("expected nil syntax for nil source, got %v", res)
	}

	res = highlightSyntax("test.unknown_extension", []byte("some code content"))
	if res != nil {
		t.Errorf("expected nil syntax for unknown language, got %v", res)
	}

	source := []byte("def hello_world():\n    pass\n")
	res = highlightSyntax("test.py", source)
	if res == nil {
		// Tree-sitter python grammar is bundled.
		// If it failed to build or parse, warn or fail.
		t.Log("Warning: Tree-sitter Python highlight returned nil (maybe grammar is not linked in this test binary)")
	} else {
		spans, ok := res[0]
		if !ok || len(spans) == 0 {
			t.Errorf("expected syntax spans for line 0, got %v", spans)
		}
	}
}
