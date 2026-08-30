package rules

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
			rules := Get(tt.lang)
			if rules == nil {
				t.Fatalf("rules for %s not loaded", tt.lang)
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
	rules := Get("toml")
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

func TestRulesHelperMethods(t *testing.T) {
	newSampleRules := func() *Rules {
		return &Rules{
			Ignored:      []string{"comment", ";"},
			Keywords:     []string{"func", "return"},
			Aliased:      map[string]string{"type_alias": "aliased_type", "label_val": "aliased_label"},
			LabelIgnored: []string{"identifier"},
			Unordered:    []string{"object", "hash"},
			Flattened:    []string{"string_literal"},
			Blocks:       []string{"block", "compound_statement"},
			EquivalentTypes: [][]string{
				{"function_declaration", "function_definition", "variable_declaration"},
				{"assignment_statement", "variable_declaration"},
			},
		}
	}

	t.Run("compiled sets", func(t *testing.T) {
		r := newSampleRules()
		r.CompileSets()

		if !r.IsIgnored("comment", "") {
			t.Errorf("IsIgnored(comment) = false, want true")
		}
		if !r.IsIgnored("other", ";") {
			t.Errorf("IsIgnored(other, ;) = false, want true")
		}
		if r.IsIgnored("node", "val") {
			t.Errorf("IsIgnored(node, val) = true, want false")
		}

		if !r.IsKeyword("func", "") {
			t.Errorf("IsKeyword(func) = false, want true")
		}
		if !r.IsKeyword("other", "return") {
			t.Errorf("IsKeyword(other, return) = false, want true")
		}
		if r.IsKeyword("node", "val") {
			t.Errorf("IsKeyword(node, val) = true, want false")
		}

		if !r.IsLabelIgnored("identifier") {
			t.Errorf("IsLabelIgnored(identifier) = false, want true")
		}
		if r.IsLabelIgnored("other") {
			t.Errorf("IsLabelIgnored(other) = true, want false")
		}

		if !r.IsUnordered("object") {
			t.Errorf("IsUnordered(object) = false, want true")
		}
		if r.IsUnordered("array") {
			t.Errorf("IsUnordered(array) = true, want false")
		}

		if !r.IsFlattened("string_literal") {
			t.Errorf("IsFlattened(string_literal) = false, want true")
		}
		if r.IsFlattened("other") {
			t.Errorf("IsFlattened(other) = true, want false")
		}

		if !r.IsBlock("compound_statement") {
			t.Errorf("IsBlock(compound_statement) = false, want true")
		}
		if r.IsBlock("other") {
			t.Errorf("IsBlock(other) = true, want false")
		}

		if !r.AreTypesEquivalent("function_declaration", "variable_declaration") {
			t.Errorf("AreTypesEquivalent(function_declaration, variable_declaration) = false, want true")
		}
		if !r.AreTypesEquivalent("assignment_statement", "variable_declaration") {
			t.Errorf("AreTypesEquivalent(assignment_statement, variable_declaration) = false, want true")
		}
		if r.AreTypesEquivalent("function_declaration", "assignment_statement") {
			t.Errorf("AreTypesEquivalent(function_declaration, assignment_statement) = true, want false")
		}

		if got, ok := r.Alias("type_alias", ""); !ok || got != "aliased_type" {
			t.Errorf("Alias(type_alias, \"\") = (%q, %v), want (\"aliased_type\", true)", got, ok)
		}
		if got, ok := r.Alias("other", "label_val"); !ok || got != "aliased_label" {
			t.Errorf("Alias(other, label_val) = (%q, %v), want (\"aliased_label\", true)", got, ok)
		}
		if _, ok := r.Alias("other", "unknown"); ok {
			t.Errorf("Alias(other, unknown) returned ok = true, want false")
		}
	})

	t.Run("uncompiled fallback", func(t *testing.T) {
		r := newSampleRules()

		if !r.IsIgnored("comment", "") || !r.IsIgnored("other", ";") || r.IsIgnored("node", "val") {
			t.Errorf("IsIgnored uncompiled fallback failed")
		}
		if !r.IsKeyword("func", "") || !r.IsKeyword("other", "return") || r.IsKeyword("node", "val") {
			t.Errorf("IsKeyword uncompiled fallback failed")
		}
		if !r.IsLabelIgnored("identifier") || r.IsLabelIgnored("other") {
			t.Errorf("IsLabelIgnored uncompiled fallback failed")
		}
		if !r.IsUnordered("object") || r.IsUnordered("array") {
			t.Errorf("IsUnordered uncompiled fallback failed")
		}
		if !r.IsFlattened("string_literal") || r.IsFlattened("other") {
			t.Errorf("IsFlattened uncompiled fallback failed")
		}
		if !r.IsBlock("compound_statement") || r.IsBlock("other") {
			t.Errorf("IsBlock uncompiled fallback failed")
		}
		if !r.AreTypesEquivalent("function_declaration", "variable_declaration") {
			t.Errorf("AreTypesEquivalent uncompiled fallback failed")
		}
		if got, ok := r.Alias("type_alias", ""); !ok || got != "aliased_type" {
			t.Errorf("Alias uncompiled fallback failed")
		}
	})

	t.Run("nil receiver safe", func(t *testing.T) {
		var r *Rules
		if r.IsIgnored("a", "b") {
			t.Errorf("nil.IsIgnored returned true")
		}
		if r.IsKeyword("a", "b") {
			t.Errorf("nil.IsKeyword returned true")
		}
		if r.IsLabelIgnored("a") {
			t.Errorf("nil.IsLabelIgnored returned true")
		}
		if r.IsUnordered("a") {
			t.Errorf("nil.IsUnordered returned true")
		}
		if r.IsFlattened("a") {
			t.Errorf("nil.IsFlattened returned true")
		}
		if r.IsBlock("a") {
			t.Errorf("nil.IsBlock returned true")
		}
		if !r.AreTypesEquivalent("a", "a") {
			t.Errorf("nil.AreTypesEquivalent(a, a) returned false, want true")
		}
		if _, ok := r.Alias("a", "b"); ok {
			t.Errorf("nil.Alias returned ok = true, want false")
		}
	})

	t.Run("unknown language fallback", func(t *testing.T) {
		r := Get("nonexistent_lang")
		if r == nil {
			t.Fatalf("Get(unknown) returned nil, want non-nil defaultRules")
		}
		if r.IsDeclaration("func") {
			t.Errorf("defaultRules.IsDeclaration returned true")
		}
		if !r.AreTypesEquivalent("a", "a") {
			t.Errorf("defaultRules.AreTypesEquivalent(a, a) returned false")
		}
		if r.AreTypesEquivalent("a", "b") {
			t.Errorf("defaultRules.AreTypesEquivalent(a, b) returned true")
		}
	})

	t.Run("package-level IsFlattened", func(t *testing.T) {
		if !IsFlattened("raw_string_literal") {
			t.Errorf("IsFlattened(raw_string_literal) = false, want true")
		}
		if IsFlattened("nonexistent_type_xyz") {
			t.Errorf("IsFlattened(nonexistent_type_xyz) = true, want false")
		}
		if IsFlattened("") {
			t.Errorf("IsFlattened(\"\") = true, want false")
		}
	})
}
