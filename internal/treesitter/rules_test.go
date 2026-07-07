package treesitter

import (
	"testing"
)

func TestRulesAliased(t *testing.T) {
	tests := []struct {
		lang      string
		operators []string
	}{
		{
			lang: "go",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||",
				"=", ":=", "+=", "-=", "*=", "/=", "%=",
				"&", "|", "^", "<<", ">>", "&^",
				"!", "<-", "++", "--",
			},
		},
		{
			lang: "python",
			operators: []string{
				"==", "<=", ">=", "!=", "<", ">", "<>",
				"and", "or",
				"=", "-=", "+=", "*=", "/=", "//=", "%=", "**=",
				"is", "is not",
				"+", "-", "*", "/", "//", "%", "**",
				"&", "|", "^", "<<", ">>",
				"not",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			rules := GetRules(tt.lang)
			if rules == nil {
				t.Fatalf("expected rules for %s to be loaded", tt.lang)
			}

			for _, op := range tt.operators {
				alias, ok := rules.Aliased[op]
				if !ok {
					t.Errorf("expected operator %q to be aliased, but it was not", op)
				}
				if alias == "" {
					t.Errorf("expected non-empty alias for operator %q", op)
				}
			}
		})
	}
}
