package serialize

import (
	"testing"
)

func TestEqualIgnoringStyle(t *testing.T) {
	tests := []struct {
		name                  string
		s, d                  string
		significantWhitespace bool
		want                  bool
	}{
		{
			name:                  "identical empty",
			s:                     "",
			d:                     "",
			significantWhitespace: true,
			want:                  true,
		},
		{
			name:                  "identical code",
			s:                     "func Foo() int { return 1 }",
			d:                     "func Foo() int { return 1 }",
			significantWhitespace: true,
			want:                  true,
		},
		{
			name:                  "internal whitespace difference",
			s:                     "\tAction string",
			d:                     "\tAction          string",
			significantWhitespace: true,
			want:                  true,
		},
		{
			name:                  "internal tabs vs spaces",
			s:                     "val\tx := 1",
			d:                     "val   x := 1",
			significantWhitespace: true,
			want:                  true,
		},
		{
			name:                  "significant whitespace indentation mismatch",
			s:                     "        return x",
			d:                     "    return x",
			significantWhitespace: true,
			want:                  false,
		},
		{
			name:                  "significant whitespace tab vs space indentation",
			s:                     "\tx = 1",
			d:                     "    x = 1",
			significantWhitespace: true,
			want:                  false,
		},
		{
			name:                  "insignificant whitespace indentation mismatch allowed",
			s:                     "        return x",
			d:                     "    return x",
			significantWhitespace: false,
			want:                  true,
		},
		{
			name:                  "insignificant whitespace tab vs spaces allowed",
			s:                     "\t\treturn x",
			d:                     "    return x",
			significantWhitespace: false,
			want:                  true,
		},
		{
			name:                  "different tokens same indentation",
			s:                     "    return x",
			d:                     "    return y",
			significantWhitespace: false,
			want:                  false,
		},
		{
			name:                  "different token count",
			s:                     "x = 1",
			d:                     "x = 1 + 2",
			significantWhitespace: false,
			want:                  false,
		},
		{
			name:                  "empty vs whitespace with significant whitespace",
			s:                     "",
			d:                     "   ",
			significantWhitespace: true,
			want:                  false,
		},
		{
			name:                  "empty vs whitespace with insignificant whitespace",
			s:                     "",
			d:                     "   ",
			significantWhitespace: false,
			want:                  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EqualIgnoringStyle(tt.s, tt.d, tt.significantWhitespace)
			if got != tt.want {
				t.Errorf("EqualIgnoringStyle(%q, %q, %v) = %v, want %v", tt.s, tt.d, tt.significantWhitespace, got, tt.want)
			}
		})
	}
}

func TestEqualIgnoringStyle_DualPlane(t *testing.T) {
	// When Tree-sitter matched the AST, pure re-indentation counts as matching.
	if !EqualIgnoringStyle("    x = 1", "  x = 1", false) {
		t.Errorf("AST plane: expected '    x = 1' and '  x = 1' to match with significantWhitespace=false")
	}

	// In line-diff fallback, indentation shifts must count as changes.
	if EqualIgnoringStyle("    x = 1", "  x = 1", true) {
		t.Errorf("Fallback plane: expected '    x = 1' and '  x = 1' to differ with significantWhitespace=true")
	}

	// Non-whitespace token differences return false on both planes.
	if EqualIgnoringStyle("    x = 1", "  y = 1", false) {
		t.Errorf("AST plane: expected token difference to return false")
	}
	if EqualIgnoringStyle("    x = 1", "  y = 1", true) {
		t.Errorf("Fallback plane: expected token difference to return false")
	}
}

func BenchmarkEqualIgnoringStyle(b *testing.B) {
	s := "    func Foo(a int,  b string) (int, error) {"
	d := "  func Foo(a int, b string) (int, error) {"
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = EqualIgnoringStyle(s, d, false)
	}
}
