package treesitter

import (
	"testing"
)

func TestGoRulesAliased(t *testing.T) {
	rules := GetRules("go")
	if rules == nil {
		t.Fatal("expected rules for go to be loaded")
	}

	expectedOperators := []string{
		"+", "-", "*", "/", "%",
		"==", "!=", "<", "<=", ">", ">=",
		"&&", "||",
		"=", ":=", "+=", "-=", "*=", "/=", "%=",
		"&", "|", "^", "<<", ">>", "&^",
		"!", "<-", "++", "--",
	}

	for _, op := range expectedOperators {
		alias, ok := rules.Aliased[op]
		if !ok {
			t.Errorf("expected operator %q to be aliased, but it was not", op)
		}
		if alias == "" {
			t.Errorf("expected non-empty alias for operator %q", op)
		}
	}
}
