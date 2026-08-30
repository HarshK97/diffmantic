package rules

var cssRules = &Rules{
	Flattened: []string{
		"plain_value",
		"color_value",
		"integer_value",
		"float_value",
		"string_value",
	},
	Ignored: []string{
		"{",
		"}",
		":",
		";",
		",",
		"(",
		")",
		"comment",
	},
	Scaffolding: []string{
		"stylesheet",
		"rule_set",
		"selectors",
		"block",
		"declaration",
		"media_statement",
		"keyframes_statement",
	},
	Pairs: []string{
		"declaration",
	},
	Unordered: []string{
		"selectors",
	},
	Comments: []string{
		"comment",
	},
	Calls: []string{
		"call_expression",
	},
}
