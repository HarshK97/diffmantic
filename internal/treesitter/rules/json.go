package rules

var jsonRules = &Rules{
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
	Unordered: []string{
		"object",
	},
}
