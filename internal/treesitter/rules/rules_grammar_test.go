package rules_test

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestAllLanguageRulesMatchGrammarSymbols(t *testing.T) {
	langs := []struct {
		name     string
		filename string
	}{
		{"c", "main.c"},
		{"cpp", "main.cpp"},
		{"go", "main.go"},
		{"rust", "main.rs"},
		{"python", "main.py"},
		{"javascript", "main.js"},
		{"typescript", "main.ts"},
		{"tsx", "main.tsx"},
		{"java", "Main.java"},
		{"php", "main.php"},
		{"ruby", "main.rb"},
		{"lua", "main.lua"},
		{"zig", "main.zig"},
		{"css", "style.css"},
		{"html", "index.html"},
		{"json", "data.json"},
		{"toml", "config.toml"},
		{"yaml", "config.yaml"},
	}

	for _, l := range langs {
		t.Run(l.name, func(t *testing.T) {
			entry := grammars.DetectLanguage(l.filename)
			if entry == nil {
				t.Fatalf("Failed to detect language grammar for %s", l.name)
			}
			lang := entry.Language()

			grammarSymbols := make(map[string]bool)
			for i, meta := range lang.SymbolMetadata {
				name := ""
				if i < len(lang.SymbolNames) {
					name = lang.SymbolNames[i]
				}
				if meta.Named && name != "" {
					grammarSymbols[name] = true
				}
			}

			r := rules.Get(l.name)
			if r == nil {
				return
			}

			checkField := func(fieldName string, items []string) {
				for _, item := range items {
					if !grammarSymbols[item] {
						t.Errorf("[%s] %s item %q does NOT exist in gotreesitter grammar", l.name, fieldName, item)
					}
				}
			}

			checkField("Declarations", r.Declarations)
			checkField("Blocks", r.Blocks)
			checkField("Scaffolding", r.Scaffolding)
			checkField("Wrappers", r.Wrappers)
			checkField("Pairs", r.Pairs)
		})
	}
}
