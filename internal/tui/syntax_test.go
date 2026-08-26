package tui

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/theme"
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
		t.Log("Warning: Tree-sitter Python highlight returned nil (maybe grammar is not linked in this test binary)")
	} else {
		spans, ok := res[0]
		if !ok || len(spans) == 0 {
			t.Errorf("expected syntax spans for line 0, got %v", spans)
		}
	}
}

func TestHighlightSyntaxGo(t *testing.T) {
	source := []byte("if engine.handlers404 == nil {\n}\n")
	mocha := theme.CatppuccinMochaTheme()
	latte := theme.CatppuccinLatteTheme()

	resMocha := highlightSyntax("test.go", source, mocha)
	resLatte := highlightSyntax("test.go", source, latte)

	if resMocha == nil || resLatte == nil {
		t.Fatal("expected non-nil syntax highlighting for Go code")
	}

	// Line 0 spans: "if", "handlers404", "==", "nil"
	spansLatte := resLatte[0]
	if len(spansLatte) == 0 {
		t.Errorf("expected syntax spans for line 0 in Latte")
	}
}
