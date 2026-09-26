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
				"!", "~", "++", "--", "?",
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
				"!", "~", "++", "--", "?",
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
				"!", "~", "++", "--", "?",
				"=>",
				"/>", "</",
			},
		},
		{
			lang: "rust",
			operators: []string{
				"+", "-", "*", "/", "%",
				"==", "!=", "<", "<=", ">", ">=",
				"&&", "||", "!", "?",
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
				"&&", "||", "!", "?",
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
				"&&", "||", "!", "?",
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
				"&&", "||", "!", "?",
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
				"++", "--", "??", "?",
			},
		},
		{
			lang: "ruby",
			operators: []string{
				"==", "!=", "===", "<=>", "<", "<=", ">", ">=", "=~", "!~",
				"&&", "||", "!", "?",
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
			Calls:        []string{"call_expression"},
			Indexed:      []string{"subscript_expression", "index_expression"},
			Tags:         []string{"element", "jsx_element", "start_tag", "jsx_opening_element"},
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

		if !r.IsCall("call_expression") {
			t.Errorf("IsCall(call_expression) = false, want true")
		}
		if r.IsCall("other") {
			t.Errorf("IsCall(other) = true, want false")
		}

		if !r.IsIndexed("subscript_expression") || !r.IsIndexed("index_expression") {
			t.Errorf("IsIndexed(subscript_expression/index_expression) = false, want true")
		}
		if r.IsIndexed("other") {
			t.Errorf("IsIndexed(other) = true, want false")
		}

		if !r.IsTag("element") || !r.IsTag("jsx_element") || !r.IsTag("start_tag") || !r.IsTag("jsx_opening_element") || r.IsTag("other") {
			t.Errorf("IsTag failed in compiled sets")
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
		if !r.IsCall("call_expression") || r.IsCall("other") {
			t.Errorf("IsCall uncompiled fallback failed")
		}
		if !r.IsIndexed("subscript_expression") || r.IsIndexed("other") {
			t.Errorf("IsIndexed uncompiled fallback failed")
		}
		if !r.IsTag("element") || !r.IsTag("jsx_element") || !r.IsTag("start_tag") || !r.IsTag("jsx_opening_element") || r.IsTag("other") {
			t.Errorf("IsTag uncompiled fallback failed")
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
		if r.IsCall("a") {
			t.Errorf("nil.IsCall returned true")
		}
		if r.IsIndexed("a") {
			t.Errorf("nil.IsIndexed returned true")
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

	t.Run("package-level helpers", func(t *testing.T) {
		if !IsFlattened("raw_string_literal") {
			t.Errorf("IsFlattened(raw_string_literal) = false, want true")
		}
		if IsFlattened("nonexistent_type_xyz") {
			t.Errorf("IsFlattened(nonexistent_type_xyz) = true, want false")
		}
		if IsFlattened("") {
			t.Errorf("IsFlattened(\"\") = true, want false")
		}
		if !IsCall("call_expression") {
			t.Errorf("IsCall(call_expression) = false, want true")
		}
		if IsCall("nonexistent_call_xyz") {
			t.Errorf("IsCall(nonexistent_call_xyz) = true, want false")
		}
		if !IsIndexed("subscript_expression") || !IsIndexed("index_expression") {
			t.Errorf("IsIndexed(subscript_expression) = false, want true")
		}
		if IsIndexed("nonexistent_indexed_xyz") {
			t.Errorf("IsIndexed(nonexistent_indexed_xyz) = true, want false")
		}
	})
}

func TestRulesIsOperatorLiteral(t *testing.T) {
	r := &Rules{}

	positive := []string{
		"arithmetic_operator_literal",
		"logical_operator_literal",
		"comparison_operator_literal",
		"assignment_operator_literal",
		"bitwise_operator_literal",
		"unary_operator_literal",
		"channel_operator_literal",
		"update_operator_literal",
		"range_operator_literal",
		"concat_operator_literal",
		"length_operator_literal",
		"null_coalescing_operator_literal",
		"ternary_operator_literal",
		"try_operator_literal",
	}

	for _, p := range positive {
		if !r.IsOperatorLiteral(p) {
			t.Errorf("r.IsOperatorLiteral(%q) = false, want true", p)
		}
		if !IsOperatorLiteral(p) {
			t.Errorf("IsOperatorLiteral(%q) = false, want true", p)
		}
	}

	negative := []string{
		"",
		"identifier",
		"field_identifier",
		"block",
		"function_declaration",
		"literal",
		"string",
		"is_operator",
		"is_not_operator",
		"not_in_operator",
	}

	for _, n := range negative {
		if r.IsOperatorLiteral(n) {
			t.Errorf("r.IsOperatorLiteral(%q) = true, want false", n)
		}
		if IsOperatorLiteral(n) {
			t.Errorf("IsOperatorLiteral(%q) = true, want false", n)
		}
	}
}

func TestRulesIsExpression(t *testing.T) {
	r := &Rules{
		Calls:   []string{"custom_call"},
		Indexed: []string{"custom_index"},
	}
	r.CompileSets()

	universal := []string{
		"binary_expression",
		"unary_expression",
		"call_expression",
		"update_expression",
		"assignment_expression",
		"binary_operator",
		"boolean_operator",
		"comparison_operator",
		"unary_operator",
		"expression",
		"binary",
		"unary",
	}

	for _, p := range universal {
		if !r.IsExpression(p) {
			t.Errorf("r.IsExpression(%q) = false, want true", p)
		}
		if !IsExpression(p) {
			t.Errorf("IsExpression(%q) = false, want true", p)
		}
	}

	custom := []string{
		"custom_call",
		"custom_index",
	}
	for _, c := range custom {
		if !r.IsExpression(c) {
			t.Errorf("r.IsExpression(%q) = false, want true", c)
		}
	}

	negative := []string{
		"",
		"expression_statement",
		"function_declaration",
		"method_declaration",
		"block",
		"short_var_declaration",
		"assignment_statement",
		"identifier",
	}

	for _, n := range negative {
		if r.IsExpression(n) {
			t.Errorf("r.IsExpression(%q) = true, want false", n)
		}
		if IsExpression(n) {
			t.Errorf("IsExpression(%q) = true, want false", n)
		}
	}
}

func TestLanguageKind(t *testing.T) {
	expectedKinds := map[string]LanguageKind{
		"c":          KindCode,
		"cpp":        KindCode,
		"go":         KindCode,
		"rust":       KindCode,
		"python":     KindCode,
		"javascript": KindCode,
		"typescript": KindCode,
		"tsx":        KindCode,
		"java":       KindCode,
		"php":        KindCode,
		"ruby":       KindCode,
		"lua":        KindCode,
		"zig":        KindCode,
		"json":       KindData,
		"yaml":       KindData,
		"toml":       KindData,
		"html":       KindMarkup,
		"css":        KindMarkup,
	}

	for lang, expected := range expectedKinds {
		r := Get(lang)
		if r == nil {
			t.Errorf("Get(%q) returned nil", lang)
			continue
		}
		if r.GetKind() != expected {
			t.Errorf("Get(%q).GetKind() = %v, want %v", lang, r.GetKind(), expected)
		}
		if r.Kind != expected {
			t.Errorf("Get(%q).Kind = %v, want %v", lang, r.Kind, expected)
		}
	}

	var nilRules *Rules
	if nilRules.GetKind() != KindCode {
		t.Errorf("nil.GetKind() = %v, want KindCode", nilRules.GetKind())
	}
}

func TestIsIdentifier(t *testing.T) {
	tests := []struct {
		lang     string
		nodeType string
		want     bool
	}{
		{"javascript", "identifier", true},
		{"javascript", "property_identifier", true},
		{"javascript", "shorthand_property_identifier", true},
		{"javascript", "private_property_identifier", true},
		{"typescript", "property_identifier", true},
		{"tsx", "property_identifier", true},
		{"c", "identifier", true},
		{"c", "field_identifier", true},
		{"cpp", "destructor_name", true},
		{"rust", "field_identifier", true},
		{"rust", "shorthand_field_identifier", true},
		{"go", "field_identifier", true},
		{"go", "package_identifier", true},
		{"c", "comment", false},
		{"javascript", "string", false},
	}

	for _, tt := range tests {
		r := Get(tt.lang)
		if r == nil {
			t.Fatalf("Get(%q) returned nil", tt.lang)
		}
		if got := r.IsIdentifier(tt.nodeType); got != tt.want {
			t.Errorf("Get(%q).IsIdentifier(%q) = %v, want %v", tt.lang, tt.nodeType, got, tt.want)
		}
		if tt.want && !IsIdentifier(tt.nodeType) {
			t.Errorf("IsIdentifier(%q) = false, want true", tt.nodeType)
		}
	}

	var nilRules *Rules
	if nilRules.IsIdentifier("identifier") {
		t.Error("nilRules.IsIdentifier should return false")
	}
}

func TestRulesIsCaseClause(t *testing.T) {
	tests := []struct {
		lang     string
		nodeType string
		want     bool
	}{
		{"go", "expression_case", true},
		{"go", "default_case", true},
		{"go", "type_case", true},
		{"go", "communication_case", true},
		{"go", "block", false},
	}
	for _, tt := range tests {
		r := Get(tt.lang)
		if got := r.IsCaseClause(tt.nodeType); got != tt.want {
			t.Errorf("Get(%q).IsCaseClause(%q) = %v, want %v", tt.lang, tt.nodeType, got, tt.want)
		}
	}

	globalTests := []struct {
		nodeType string
		want     bool
	}{
		{"switch_case", true},
		{"case_clause", true},
		{"match_arm", true},
		{"some_random_type", false},
	}
	for _, tt := range globalTests {
		if got := IsCaseClause(tt.nodeType); got != tt.want {
			t.Errorf("IsCaseClause(%q) = %v, want %v", tt.nodeType, got, tt.want)
		}
	}

	// Uncompiled fallback uses slices.Contains.
	uncompiled := &Rules{CaseClauses: []string{"custom_case"}}
	if !uncompiled.IsCaseClause("custom_case") {
		t.Errorf("uncompiled.IsCaseClause(custom_case) = false, want true")
	}
	if uncompiled.IsCaseClause("other") {
		t.Errorf("uncompiled.IsCaseClause(other) = true, want false")
	}

	var nilR *Rules
	if nilR.IsCaseClause("expression_case") {
		t.Errorf("nilR.IsCaseClause = true, want false")
	}

	r := Get("go")
	if r.IsCaseClause("") {
		t.Errorf("r.IsCaseClause(\"\") = true, want false")
	}
}
