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
			},
		},
		{
			lang: "zig",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"and", "or",
				"=", "+=", "-=", "*=", "/=", "%=",
				"&", "|", "^", "<<", ">>",
				"!", ".?", ".*",
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
				"..", "..=",
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
				"++", "--",
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
				"++", "--",
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
				"++", "--",
			},
		},
		{
			lang: "php",
			operators: []string{
				"+", "-", "*", "/", "%", "**",
				"==", "!=", "<>", "===", "!==", "<", "<=", ">", ">=", "<=>",
				"&&", "||", "!", "and", "or", "xor",
				"=", "+=", "-=", "*=", "/=", "%=", ".=", "**=", "<<=", ">>=", "&=", "^=", "|=", "??=",
				"++", "--", "??",
			},
		},
		{
			lang: "ruby",
			operators: []string{
				"==", "!=", "===", "<=>", "<", "<=", ">", ">=", "=~", "!~",
				"&&", "||", "!",
				"=", "+=", "-=", "*=", "/=", "%=", "**=", "&=", "|=", "^=", "<<=", ">>=", "||=", "&&=",
				"+", "-", "*", "/", "%", "**",
				"&", "|", "^", "<<", ">>", "~",
			},
		},
		{
			lang: "lua",
			operators: []string{
				"+", "-", "*", "/", "//", "%", "^",
				"==", "~=", "<", "<=", ">", ">=",
				"and", "or", "not",
				"=",
				"&", "|", "~", "<<", ">>",
				"..", "#",
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

func TestRulesUnordered(t *testing.T) {
	r := &Rules{
		Unordered: []string{"object", "start_tag"},
	}
	if len(r.Unordered) != 2 {
		t.Fatalf("expected 2 unordered entries, got %d", len(r.Unordered))
	}
	if r.Unordered[0] != "object" || r.Unordered[1] != "start_tag" {
		t.Errorf("unexpected unordered entries: %v", r.Unordered)
	}
}

func TestRulesPairs(t *testing.T) {
	r := &Rules{
		Pairs: []string{"pair", "key_value_pair"},
	}
	if len(r.Pairs) != 2 {
		t.Fatalf("expected 2 pairs entries, got %d", len(r.Pairs))
	}
	if r.Pairs[0] != "pair" || r.Pairs[1] != "key_value_pair" {
		t.Errorf("unexpected pairs entries: %v", r.Pairs)
	}
}

func TestRulesTOML(t *testing.T) {
	rules := GetRules("toml")
	if rules == nil {
		t.Fatal("expected toml rules to be loaded")
	}
	if len(rules.Scaffolding) == 0 {
		t.Error("expected scaffolding in toml rules")
	}
	if len(rules.Unordered) == 0 {
		t.Error("expected unordered nodes in toml rules")
	}
}

func TestRulesAreTypesEquivalent(t *testing.T) {
	r := &Rules{
		EquivalentTypes: [][]string{
			{"function_declaration", "function_definition", "variable_declaration"},
			{"if_statement", "elseif_statement"},
		},
	}

	if !r.AreTypesEquivalent("function_declaration", "variable_declaration") {
		t.Errorf("expected function_declaration and variable_declaration to be equivalent")
	}
	if !r.AreTypesEquivalent("if_statement", "elseif_statement") {
		t.Errorf("expected if_statement and elseif_statement to be equivalent")
	}
	if r.AreTypesEquivalent("function_declaration", "if_statement") {
		t.Errorf("expected function_declaration and if_statement NOT to be equivalent")
	}
	if !r.AreTypesEquivalent("same", "same") {
		t.Errorf("expected same types to be equivalent")
	}
}

func TestEveryLanguageEquivalentTypesAreValidSymbols(t *testing.T) {
	for _, ext := range []string{
		"c.c", "cpp.cc", "css.css", "go.go", "html.html", "java.java",
		"javascript.js", "json.json", "lua.lua", "php.php", "python.py",
		"ruby.rb", "rust.rs", "toml.toml", "tsx.tsx", "typescript.ts",
		"yaml.yaml", "zig.zig",
	} {
		entry := DetectGrammarEntry(ext)
		if entry == nil {
			continue
		}
		lang := entry.Language()
		namedSymbols := make(map[string]bool)
		for i := 0; i < int(lang.SymbolCount) && i < len(lang.SymbolNames); i++ {
			name := lang.SymbolNames[i]
			isNamed := i < len(lang.SymbolMetadata) && lang.SymbolMetadata[i].Named
			if name != "" && isNamed {
				namedSymbols[name] = true
			}
		}

		rules := GetRules(entry.Name)
		if rules == nil {
			continue
		}
		for _, group := range rules.EquivalentTypes {
			for _, sym := range group {
				if !namedSymbols[sym] {
					t.Errorf("language %s: equivalent_types symbol %q is not a valid named symbol in grammar", entry.Name, sym)
				}
			}
		}
	}
}
