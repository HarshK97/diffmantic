package treesitter

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/HarshK97/diffmantic/internal/treesitter/rules"
)

// Language represents a recognized programming language.
type Language struct {
	Name string
}

var extToLang = map[string]string{
	".go":   "go",
	".c":    "c",
	".h":    "c",
	".cpp":  "cpp",
	".cc":   "cpp",
	".cxx":  "cpp",
	".c++":  "cpp",
	".hpp":  "cpp",
	".hh":   "cpp",
	".hxx":  "cpp",
	".h++":  "cpp",
	".rs":   "rust",
	".py":   "python",
	".pyi":  "python",
	".js":   "javascript",
	".jsx":  "javascript",
	".mjs":  "javascript",
	".cjs":  "javascript",
	".ts":   "typescript",
	".mts":  "typescript",
	".cts":  "typescript",
	".tsx":  "tsx",
	".java": "java",
	".zig":  "zig",
	".lua":  "lua",
}

var basenameToLang map[string]string

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

	// Support compression wrappers (e.g. .go.zst, .py.gz)
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

// DetectRules returns the language rules for srcPath, falling back to dstPath.
func DetectRules(srcPath, dstPath string) *rules.Rules {
	if langName, err := DetectLanguageName(srcPath); err == nil {
		if r := rules.Get(langName); r != nil {
			return r
		}
	}
	if langName, err := DetectLanguageName(dstPath); err == nil {
		if r := rules.Get(langName); r != nil {
			return r
		}
	}
	return nil
}

// DetectSignificantWhitespace reports whether the language detected from srcPath
// (falling back to dstPath) has block scoping defined by leading indentation (e.g. Python).
// Unknown grammars default conservatively to true.
func DetectSignificantWhitespace(srcPath, dstPath string) bool {
	if r := DetectRules(srcPath, dstPath); r != nil {
		return r.SignificantWhitespace
	}
	return true
}
