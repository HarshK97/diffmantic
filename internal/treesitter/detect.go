package treesitter

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Language represents a recognized programming or data language.
type Language struct {
	Name string
}

var extToLang = map[string]string{
	".go":      "go",
	".c":       "c",
	".h":       "c",
	".cpp":     "cpp",
	".cc":      "cpp",
	".cxx":     "cpp",
	".c++":     "cpp",
	".hpp":     "cpp",
	".hh":      "cpp",
	".hxx":     "cpp",
	".h++":     "cpp",
	".rs":      "rust",
	".py":      "python",
	".pyi":     "python",
	".js":      "javascript",
	".jsx":     "javascript",
	".mjs":     "javascript",
	".cjs":     "javascript",
	".ts":      "typescript",
	".mts":     "typescript",
	".cts":     "typescript",
	".tsx":     "tsx",
	".java":    "java",
	".php":     "php",
	".phtml":   "php",
	".php3":    "php",
	".php4":    "php",
	".php5":    "php",
	".php7":    "php",
	".phps":    "php",
	".rb":      "ruby",
	".rake":    "ruby",
	".gemspec": "ruby",
	".zig":     "zig",
	".lua":     "lua",
	".html":    "html",
	".htm":     "html",
	".css":     "css",
	".json":    "json",
	".toml":    "toml",
	".yaml":    "yaml",
	".yml":     "yaml",
}

var basenameToLang = map[string]string{
	"rakefile": "ruby",
	"gemfile":  "ruby",
}

// DetectLanguage detects the language for a given filename or path.
func DetectLanguage(filename string) (*Language, error) {
	name, err := DetectLanguageName(filename)
	if err != nil {
		return nil, err
	}
	return &Language{Name: name}, nil
}

// DetectLanguageName returns the canonical language name string for a given filename or path.
func DetectLanguageName(filename string) (string, error) {
	base := filepath.Base(filename)
	lowerBase := strings.ToLower(base)

	if lang, ok := basenameToLang[lowerBase]; ok {
		return lang, nil
	}

	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return "", fmt.Errorf("unsupported language for file: %s", filename)
	}

	if lang, ok := extToLang[ext]; ok {
		return lang, nil
	}

	// Support compression wrappers (e.g. .json.gz, .go.zst, .yaml.xz)
	if ext == ".gz" || ext == ".zst" || ext == ".xz" || ext == ".bz2" {
		trimmed := strings.TrimSuffix(lowerBase, ext)
		if innerLang, ok := basenameToLang[trimmed]; ok {
			return innerLang, nil
		}
		innerExt := strings.ToLower(filepath.Ext(trimmed))
		if innerExt != "" {
			if lang, ok := extToLang[innerExt]; ok {
				return lang, nil
			}
		}
	}

	return "", fmt.Errorf("unsupported language for file: %s", filename)
}
