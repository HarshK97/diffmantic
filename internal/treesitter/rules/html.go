package rules

var htmlRules = &Rules{
	Flattened: []string{
		"text",
		"raw_text",
	},
	Ignored: []string{
		"<",
		">",
		"</",
		"/>",
		"=",
		"comment",
	},
	Aliased: map[string]string{
		"self_closing_tag": "start_tag",
	},
	Scaffolding: []string{
		"document",
		"element",
		"start_tag",
		"self_closing_tag",
		"end_tag",
		"attribute",
	},
	Pairs: []string{
		"attribute",
	},
	Unordered: []string{
		"start_tag",
		"self_closing_tag",
	},
	Comments: []string{
		"comment",
	},
}
