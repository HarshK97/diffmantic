package rules

var yamlRules = &Rules{
	Flattened: []string{
		"string_scalar",
		"double_quote_scalar",
		"single_quote_scalar",
		"plain_scalar",
		"comment",
		"block_scalar",
	},
	Ignored: []string{
		":",
		"-",
		"{",
		"}",
		"[",
		"]",
		",",
		"?",
		"comment",
	},
	Scaffolding: []string{
		"stream",
		"document",
		"block_node",
		"flow_node",
		"block_mapping",
		"block_mapping_pair",
		"block_sequence",
		"block_sequence_item",
		"flow_mapping",
		"flow_sequence",
		"flow_pair",
	},
	Identifiers: []string{
		"tag",
		"anchor_name",
		"alias_name",
	},
	Pairs: []string{
		"block_mapping_pair",
		"flow_pair",
	},
	Unordered: []string{
		"block_mapping",
		"flow_mapping",
	},
	EquivalentTypes: [][]string{
		{"block_mapping", "flow_mapping"},
		{"block_sequence", "flow_sequence"},
		{"plain_scalar", "double_quote_scalar", "single_quote_scalar"},
	},
	Comments: []string{
		"comment",
	},
}
