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
		{
			lang: "javascript",
			operators: []string{
				"+", "-", "*", "/", "%", "**",
				"==", "!=", "===", "!==", "<", "<=", ">", ">=",
				"&&", "||", "??",
				"=", "+=", "-=", "*=", "/=", "%=", "**=",
				"&&=", "||=", "??=",
				"&", "|", "^", "<<", ">>", ">>>",
				"!", "~", "++", "--",
				"=>",
			},
		},
		{
			lang: "typescript",
			operators: []string{
				"+", "-", "*", "/", "%", "**",
				"==", "!=", "===", "!==", "<", "<=", ">", ">=",
				"&&", "||", "??",
				"=", "+=", "-=", "*=", "/=", "%=", "**=",
				"&&=", "||=", "??=",
				"&", "|", "^", "<<", ">>", ">>>",
				"!", "~", "++", "--",
				"=>",
				"type", "interface", "namespace", "enum", "abstract", "readonly",
			},
		},
		{
			lang: "tsx",
			operators: []string{
				"+", "-", "*", "/", "%", "**",
				"==", "!=", "===", "!==", "<", "<=", ">", ">=",
				"&&", "||", "??",
				"=", "+=", "-=", "*=", "/=", "%=", "**=",
				"&&=", "||=", "??=",
				"&", "|", "^", "<<", ">>", ">>>",
				"!", "~", "++", "--",
				"=>",
				"type", "interface", "namespace", "enum", "abstract", "readonly",
				"/>", "</",
			},
		},
		{
			lang: "rust",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||", "!",
				"=", "+=", "-=", "*=", "/=", "%=", "^=", "&=", "|=", "<<=", ">>=",
				"&", "|", "^", "<<", ">>",
				"..", "..=", "->", "=>", "::",
				"fn", "let", "mut", "struct", "enum", "trait", "impl", "use", "pub",
			},
		},
		{
			lang: "c",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||", "!",
				"=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=",
				"&", "|", "^", "~", "<<", ">>",
				"++", "--", "->", ".",
				"if", "else", "switch", "case", "default", "for", "while", "do",
				"return", "break", "continue", "goto", "struct", "union", "enum", "typedef",
			},
		},
		{
			lang: "cpp",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||", "!",
				"=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=",
				"&", "|", "^", "~", "<<", ">>",
				"++", "--", "->", ".", "::",
				"if", "else", "switch", "case", "default", "for", "while", "do",
				"return", "break", "continue", "goto", "struct", "union", "enum", "typedef",
				"class", "namespace", "template", "typename", "public", "private", "protected",
				"virtual", "override", "final", "constexpr", "try", "catch", "throw", "using",
			},
		},
		{
			lang: "java",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||", "!",
				"=", "+=", "-=", "*=", "/=", "%=", "&=", "|=", "^=", "<<=", ">>=", ">>>=",
				"&", "|", "^", "~", "<<", ">>", ">>>",
				"++", "--", "->", "::", ".",
				"if", "else", "switch", "case", "default", "for", "while", "do",
				"return", "break", "continue", "yield", "class", "interface", "enum", "record",
				"public", "private", "protected", "static", "final", "abstract", "synchronized",
				"volatile", "transient", "extends", "implements", "new", "this", "super",
				"try", "catch", "finally", "throw", "throws", "assert", "instanceof", "var",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			rules := GetRules(tt.lang)
			if rules == nil {
				t.Skipf("rules for %s not loaded", tt.lang)
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

			if len(rules.Scaffolding) == 0 {
				t.Errorf("expected non-empty scaffolding list for %s", tt.lang)
			}
		})
	}
}
