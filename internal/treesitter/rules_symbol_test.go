package treesitter

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

var allLanguageExtensions = []string{
	"c.c", "cpp.cc", "css.css", "go.go", "html.html", "java.java",
	"javascript.js", "json.json", "lua.lua", "php.php", "python.py",
	"ruby.rb", "rust.rs", "toml.toml", "tsx.tsx", "typescript.ts",
	"yaml.yaml", "zig.zig",
}

func getNamedGrammarSymbols(ext string) (string, map[string]bool, *rules.Rules) {
	lang, err := DetectLanguage(ext)
	if err != nil || lang == nil {
		return "", nil, nil
	}
	ptr, err := GetNativeLanguage(lang.Name)
	if err != nil || ptr == nil {
		return "", nil, nil
	}
	symbols := NativeLanguageSymbols(ptr)
	namedSymbols := make(map[string]bool, len(symbols))
	for _, s := range symbols {
		if s != "" {
			namedSymbols[s] = true
		}
	}
	return lang.Name, namedSymbols, rules.Get(lang.Name)
}

func TestEveryLanguageEquivalentTypesAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, group := range r.EquivalentTypes {
			for _, sym := range group {
				if !namedSymbols[sym] {
					t.Errorf("language %s: equivalent_types symbol %q is not a valid named symbol in grammar", name, sym)
				}
			}
		}
	}
}

func TestEveryLanguageCommentsAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Comments {
			if !namedSymbols[sym] {
				t.Errorf("language %s: comments symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageIdentifiersAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Identifiers {
			if !namedSymbols[sym] {
				t.Errorf("language %s: identifiers symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageBlocksAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Blocks {
			if !namedSymbols[sym] {
				t.Errorf("language %s: blocks symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageCallsAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Calls {
			if !namedSymbols[sym] {
				t.Errorf("language %s: calls symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageDeclarationsAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Declarations {
			if !namedSymbols[sym] {
				t.Errorf("language %s: declarations symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageScaffoldingAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Scaffolding {
			if !namedSymbols[sym] {
				t.Errorf("language %s: scaffolding symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageFlattenedAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Flattened {
			if !namedSymbols[sym] {
				t.Errorf("language %s: flattened symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguagePairsAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Pairs {
			if !namedSymbols[sym] {
				t.Errorf("language %s: pairs symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageUnorderedAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.Unordered {
			if !namedSymbols[sym] {
				t.Errorf("language %s: unordered symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}

func TestEveryLanguageScopedDeclarationsAreValidSymbols(t *testing.T) {
	for _, ext := range allLanguageExtensions {
		name, namedSymbols, r := getNamedGrammarSymbols(ext)
		if r == nil || namedSymbols == nil {
			continue
		}
		for _, sym := range r.ScopedDeclarations {
			if !namedSymbols[sym] {
				t.Errorf("language %s: scoped_declarations symbol %q is not a valid named symbol in grammar", name, sym)
			}
		}
	}
}
