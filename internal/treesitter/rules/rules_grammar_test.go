package rules_test

import (
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
		{"php"},
		{"ruby"},
		{"lua"},
		{"zig"},
		{"css"},
		{"html"},
		{"json"},
		{"toml"},
		{"yaml"},
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
		checkField("ContainerDeclarations", r.ContainerDeclarations)
		checkField("Closures", r.Closures)
		checkField("Types", r.Types)
	}
}
