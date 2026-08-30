package rules

var tomlRules = &Rules{
	Flattened: []string{
		"string",
		"offset_date_time",
		"local_date_time",
		"local_date",
		"local_time",
	},
	Ignored: []string{
		"=",
		"[",
		"]",
		"[[",
		"]]",
		"{",
		"}",
		",",
		".",
		"\"",
		"\"\"\"",
		"'",
		"'''",
		"comment",
	},
	Scaffolding: []string{
		"document",
		"table",
		"table_array_element",
		"array",
		"inline_table",
		"pair",
	},
	Keywords: []string{
		"true",
		"false",
	},
	Pairs: []string{
		"pair",
	},
	Unordered: []string{
		"document",
		"table",
		"inline_table",
	},
	EquivalentTypes: [][]string{
		{"table", "inline_table"},
	},
	Comments: []string{
		"comment",
	},
}
