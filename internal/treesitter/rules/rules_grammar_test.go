package rules_test

import (
	"encoding/json"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter"
	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

func TestAllLanguageRulesMatchGrammarSymbols(t *testing.T) {
	langs := []struct {
		name string
	}{
		{"c"},
		{"cpp"},
		{"go"},
		{"rust"},
		{"python"},
		{"javascript"},
		{"typescript"},
		{"tsx"},
		{"java"},
		{"lua"},
		{"zig"},
	}

	for _, l := range langs {
		ptr, err := treesitter.GetNativeLanguage(l.name)
		if err != nil || ptr == nil {
			t.Fatalf("Failed to get native language for %s: %v", l.name, err)
		}

		symbols := treesitter.NativeLanguageSymbols(ptr)
		grammarSymbols := make(map[string]bool, len(symbols))
		for _, s := range symbols {
			if s != "" {
				grammarSymbols[s] = true
			}
		}

		r := rules.Get(l.name)
		if r == nil {
			continue
		}

		checkField := func(fieldName string, items []string) {
			for _, item := range items {
				if !grammarSymbols[item] {
					t.Errorf("[%s] %s item %q does NOT exist in native grammar", l.name, fieldName, item)
				}
			}
		}

		checkField("Declarations", r.Declarations)
		checkField("Blocks", r.Blocks)
		checkField("Scaffolding", r.Scaffolding)
		checkField("Wrappers", r.Wrappers)
		checkField("Pairs", r.Pairs)
		checkField("Indexed", r.Indexed)
		checkField("Unordered", r.Unordered)
		checkField("Comments", r.Comments)
		checkField("Calls", r.Calls)
		checkField("LocalVarDeclarations", r.LocalVarDeclarations)
		checkField("Assignments", r.Assignments)
		checkField("ContainerDeclarations", r.ContainerDeclarations)
		checkField("Closures", r.Closures)
		checkField("Types", r.Types)
		checkField("Identifiers", r.Identifiers)
		checkField("JumpStatements", r.JumpStatements)
		checkField("DelimitedContainers", r.DelimitedContainers)
		checkField("Selectors", r.Selectors)
		checkField("Expressions", r.Expressions)
		checkField("CaseClauses", r.CaseClauses)
		checkField("Clauses", r.Clauses)
		checkField("Tags", r.Tags)
		checkField("Statements", r.Statements)
		checkField("Imports", r.Imports)
		checkField("Attributes", r.Attributes)
		checkField("Declarators", r.Declarators)

		fields := treesitter.NativeLanguageFields(ptr)
		for fieldName := range r.FieldRoles {
			if !slices.Contains(fields, fieldName) {
				t.Errorf("[%s] FieldRoles item %q does NOT exist in native grammar fields", l.name, fieldName)
			}
		}
	}
}

func TestAllLanguageGrammarStatementsAndDeclarationsCovered(t *testing.T) {
	nodeTypesPaths := map[string]string{
		"c":          "../../../native/bridge/grammars/c_repo/src/node-types.json",
		"cpp":        "../../../native/bridge/grammars/cpp_repo/src/node-types.json",
		"go":         "../../../native/bridge/grammars/go_repo/src/node-types.json",
		"java":       "../../../native/bridge/grammars/java_repo/src/node-types.json",
		"javascript": "../../../native/bridge/grammars/javascript_repo/src/node-types.json",
		"lua":        "../../../native/bridge/grammars/lua_repo/src/node-types.json",
		"python":     "../../../native/bridge/grammars/python_repo/src/node-types.json",
		"rust":       "../../../native/bridge/grammars/rust_repo/src/node-types.json",
		"tsx":        "../../../native/bridge/grammars/tsx_repo/tsx/src/node-types.json",
		"typescript": "../../../native/bridge/grammars/typescript_repo/typescript/src/node-types.json",
		"zig":        "../../../native/bridge/grammars/zig_repo/src/node-types.json",
	}

	for _, lang := range slices.Sorted(maps.Keys(nodeTypesPaths)) {
		relPath := nodeTypesPaths[lang]
		t.Run(lang, func(t *testing.T) {
			data, err := os.ReadFile(relPath)
			if err != nil {
				t.Skipf("node-types.json not found for %s: %v", lang, err)
			}

			var items []struct {
				Type     string `json:"type"`
				Named    bool   `json:"named"`
				Subtypes []struct {
					Type  string `json:"type"`
					Named bool   `json:"named"`
				} `json:"subtypes"`
			}
			if err := json.Unmarshal(data, &items); err != nil {
				t.Fatalf("failed to parse node-types.json for %s: %v", lang, err)
			}

			r := rules.Get(lang)
			if r == nil {
				t.Fatalf("rules not found for %s", lang)
			}

			supertypes := make(map[string][]string)
			for _, it := range items {
				if len(it.Subtypes) > 0 {
					var subs []string
					for _, sub := range it.Subtypes {
						if sub.Named {
							subs = append(subs, sub.Type)
						}
					}
					supertypes[it.Type] = subs
				}
			}

			var getConcrete func(string, map[string]bool) []string
			getConcrete = func(typeName string, stack map[string]bool) []string {
				if stack[typeName] {
					return nil
				}
				if subs, ok := supertypes[typeName]; ok {
					stack[typeName] = true
					var out []string
					for _, s := range subs {
						out = append(out, getConcrete(s, stack)...)
					}
					delete(stack, typeName)
					return out
				}
				return []string{typeName}
			}

			for _, item := range items {
				lower := strings.ToLower(item.Type)
				isSuperStmt := strings.Contains(lower, "statement")
				isSuperDecl := strings.Contains(lower, "declaration") || strings.Contains(lower, "item")

				if isSuperStmt || isSuperDecl {
					for _, sub := range item.Subtypes {
						if !sub.Named {
							continue
						}
						for _, concrete := range getConcrete(sub.Type, make(map[string]bool)) {
							if strings.HasPrefix(concrete, "_") {
								continue
							}
							isKnown := r.IsStatement(concrete) ||
								r.IsDeclaration(concrete) ||
								r.IsContainerDeclaration(concrete) ||
								r.IsBlock(concrete) ||
								r.IsAttribute(concrete) ||
								r.IsComment(concrete) ||
								r.IsImport(concrete) ||
								r.IsJumpStatement(concrete)
							if !isKnown {
								t.Errorf("[%s] grammar supertype %q concrete subtype %q is NOT recognized as a statement/declaration/block/attribute", lang, item.Type, concrete)
							}
						}
					}
				}
			}
		})
	}
}
