package treesitter

import "testing"

const rustEdgeFieldSource = `fn sample(input: i32, ready: bool) {
    // Keep binding comment.
    let binding = factory(input /* keep argument */, 7);
    if ready { accept(binding); } else { reject(input); }
}
`

const goEdgeFieldSource = `package main
func sample(input int, ready bool) {
    // Keep binding comment.
    binding := factory(input /* keep argument */, 7)
    if ready { accept(binding) } else { reject(input) }
}
`

func TestEdgeFieldsParse(t *testing.T) {
	type edge struct {
		nodeType, parentType, text, field string
	}
	for _, tc := range []struct {
		lang, src, declarationType, declaration, argumentsType string
		edges                                                  []edge
	}{
		{
			lang: "rust", src: rustEdgeFieldSource,
			declarationType: "let_declaration", declaration: "let binding = factory(input /* keep argument */, 7);", argumentsType: "arguments",
			edges: []edge{
				{"identifier", "function_item", "sample", "name"},
				{"parameters", "function_item", "(input: i32, ready: bool)", "parameters"},
				{"identifier", "parameter", "input", "pattern"},
				{"primitive_type", "parameter", "i32", "type"},
				{"identifier", "let_declaration", "binding", "pattern"},
				{"call_expression", "let_declaration", "factory(input /* keep argument */, 7)", "value"},
				{"identifier", "call_expression", "factory", "function"},
				{"arguments", "call_expression", "(input /* keep argument */, 7)", "arguments"},
				{"identifier", "if_expression", "ready", "condition"},
				{"block", "if_expression", "{ accept(binding); }", "consequence"},
				{"else_clause", "if_expression", "else { reject(input); }", "alternative"},
				{"block", "else_clause", "{ reject(input); }", ""},
			},
		},
		{
			lang: "go", src: goEdgeFieldSource,
			declarationType: "short_var_declaration", declaration: "binding := factory(input /* keep argument */, 7)", argumentsType: "argument_list",
			edges: []edge{
				{"identifier", "function_declaration", "sample", "name"},
				{"parameter_list", "function_declaration", "(input int, ready bool)", "parameters"},
				{"identifier", "parameter_declaration", "input", "name"},
				{"type_identifier", "parameter_declaration", "int", "type"},
				{"expression_list", "short_var_declaration", "binding", "left"},
				{"identifier", "expression_list", "binding", ""},
				{"expression_list", "short_var_declaration", "factory(input /* keep argument */, 7)", "right"},
				{"call_expression", "expression_list", "factory(input /* keep argument */, 7)", ""},
				{"identifier", "call_expression", "factory", "function"},
				{"argument_list", "call_expression", "(input /* keep argument */, 7)", "arguments"},
				{"identifier", "if_statement", "ready", "condition"},
				{"block", "if_statement", "{ accept(binding) }", "consequence"},
				{"block", "if_statement", "{ reject(input) }", "alternative"},
			},
		},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			root := parseEdgeFields(t, tc.src, tc.lang)
			if got := root.FieldName(); got != "" {
				t.Errorf("root.FieldName() = %q, want empty", got)
			}
			if got, want := len(root.Index.ChildFields), len(root.Index.Nodes); got != want {
				t.Fatalf("len(ChildFields) = %d, want %d", got, want)
			}
			for _, want := range tc.edges {
				node := edgeFieldNode(t, root, tc.src, want.nodeType, want.parentType, want.text)
				if got := node.FieldName(); got != want.field {
					t.Errorf("%s(%q).FieldName() = %q, want %q", want.nodeType, want.text, got, want.field)
				}
			}

			declaration := edgeFieldNode(t, root, tc.src, tc.declarationType, "", tc.declaration)
			if got := declaration.LeadingTrivia; len(got) != 1 || got[0].Text != "// Keep binding comment." {
				t.Errorf("declaration.LeadingTrivia = %+v, want preserved binding comment", got)
			}
			args := edgeFieldNode(t, root, tc.src, tc.argumentsType, "call_expression", "(input /* keep argument */, 7)")
			trivia := args.SyntaxTrivia()
			if trivia == nil {
				t.Fatal("arguments.SyntaxTrivia() = nil, want parentheses")
			}
			if opening, closing := tc.src[trivia.OpeningStart:trivia.OpeningEnd], tc.src[trivia.ClosingStart:trivia.ClosingEnd]; opening != "(" || closing != ")" {
				t.Errorf("argument delimiters = (%q, %q), want parentheses", opening, closing)
			}
			if len(args.Children) != 2 {
				t.Fatalf("len(arguments.Children) = %d, want 2", len(args.Children))
			}
			for _, arg := range args.Children {
				if got := arg.FieldName(); got != "" {
					t.Errorf("argument(%q).FieldName() = %q, want empty", arg.Label, got)
				}
			}
			first := args.Children[0]
			comma := first.SyntaxTrivia()
			if comma == nil || tc.src[comma.TrailingStart:comma.TrailingEnd] != "," {
				t.Errorf("first argument.SyntaxTrivia() = %+v, want trailing comma", comma)
			}
			if got := first.TrailingTrivia; len(got) != 1 || got[0].Text != "/* keep argument */" {
				t.Errorf("first argument.TrailingTrivia = %+v, want preserved argument comment", got)
			}
		})
	}
}

func TestEdgeFieldsFlattenedParent(t *testing.T) {
	const src = `def sample(input):
    text = f"hello {factory(input)}"
    return text
`
	root := parseEdgeFields(t, src, "python")
	call := edgeFieldNode(t, root, src, "call", "string", "factory(input)")
	if got := call.FieldName(); got != "" {
		t.Errorf("promoted call.FieldName() = %q, want empty", got)
	}
	if got := root.Index.ChildFields[call.ID]; got != 0 {
		t.Errorf("promoted call field ID = %d, want 0", got)
	}
	for _, want := range []struct{ nodeType, parentType, text, field string }{
		{"identifier", "assignment", "text", "left"},
		{"string", "assignment", `f"hello {factory(input)}"`, "right"},
		{"identifier", "call", "factory", "function"},
		{"argument_list", "call", "(input)", "arguments"},
	} {
		node := edgeFieldNode(t, root, src, want.nodeType, want.parentType, want.text)
		if got := node.FieldName(); got != want.field {
			t.Errorf("%s(%q).FieldName() = %q, want %q", want.nodeType, want.text, got, want.field)
		}
	}

	nodes, _, err := ParseCST([]byte(src), "python")
	if err != nil {
		t.Fatalf("ParseCST(python) failed: %v", err)
	}
	for _, node := range nodes {
		if node.StartByte == call.StartByte && node.EndByte == call.EndByte {
			if got := root.Index.Fields[node.FieldID]; got != "expression" {
				t.Errorf("original CST call field = %q, want expression", got)
			}
			return
		}
	}
	t.Fatal("ParseCST(python) did not retain the original call")
}

func TestNativeLanguageFields(t *testing.T) {
	if got := NativeLanguageFields(nil); got != nil {
		t.Errorf("NativeLanguageFields(nil) = %v, want nil", got)
	}
	for _, lang := range []string{"c", "cpp", "go", "rust", "python", "javascript", "typescript", "tsx", "java", "lua", "zig"} {
		t.Run(lang, func(t *testing.T) {
			t.Parallel()
			ptr, err := GetNativeLanguage(lang)
			if err != nil {
				t.Fatalf("GetNativeLanguage(%q) failed: %v", lang, err)
			}
			fields := NativeLanguageFields(ptr)
			if len(fields) < 2 || fields[0] != "" {
				t.Fatalf("NativeLanguageFields(%q) = %v, want empty field zero and named fields", lang, fields)
			}
			if got := NativeLanguageFields(ptr); &got[0] != &fields[0] {
				t.Error("NativeLanguageFields returned an uncached table")
			}
			for id, name := range fields[1:] {
				if name == "" {
					t.Errorf("NativeLanguageFields(%q)[%d] = empty, want field name", lang, id+1)
				}
			}
		})
	}
}

func TestEdgeFieldNameNil(t *testing.T) {
	var node *ASTNode
	if got := node.FieldName(); got != "" {
		t.Errorf("nil.FieldName() = %q, want empty", got)
	}
}

func parseEdgeFields(t *testing.T, src, lang string) *ASTNode {
	t.Helper()
	nodes, _, err := ParseCST([]byte(src), lang)
	if err != nil {
		t.Fatalf("ParseCST(%q) failed: %v", lang, err)
	}
	for i, node := range nodes {
		if node.Flags&(FlatNodeError|FlatNodeMissing) != 0 {
			t.Fatalf("ParseCST(%q) node %d has ERROR/MISSING flags: %d", lang, i, node.Flags)
		}
	}
	root, err := ParseWithLanguage([]byte(src), lang)
	if err != nil || root == nil {
		t.Fatalf("ParseWithLanguage(%q) = %v, %v, want parsed tree", lang, root, err)
	}
	return root
}

func edgeFieldNode(t *testing.T, root *ASTNode, src, nodeType, parentType, text string) *ASTNode {
	t.Helper()
	var found *ASTNode
	for _, node := range root.Index.Nodes {
		if node.Type != nodeType || src[node.StartByte:node.EndByte] != text {
			continue
		}
		if parentType != "" && (node.Parent == nil || node.Parent.Type != parentType) {
			continue
		}
		if found != nil {
			t.Fatalf("multiple %s(%q) nodes under %q", nodeType, text, parentType)
		}
		found = node
	}
	if found == nil {
		t.Fatalf("missing %s(%q) node under %q", nodeType, text, parentType)
	}
	return found
}
