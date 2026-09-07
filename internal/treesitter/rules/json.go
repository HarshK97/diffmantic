package rules

var jsonRules = &Rules{
	Kind: KindData,
	Flattened: []string{
		"string",
	},
	Ignored: []string{
		"{",
		"}",
		"[",
		"]",
		",",
		":",
	},
	Scaffolding: []string{
		"document",
		"object",
		"array",
		"pair",
	},
	Keywords: []string{
		"true",
		"false",
		"null",
	},
	Pairs: []string{
		"pair",
	},
	Wrappers: []string{
		"array",
		"object",
	},
	Unordered: []string{
		"object",
	},
}
