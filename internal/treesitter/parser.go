package treesitter

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func DetectGrammarEntry(filename string) *grammars.LangEntry {
	base := filepath.Base(filename)
	return grammars.DetectLanguage(base)
}

func DetectLanguage(filename string) (*gotreesitter.Language, error) {
	entry := DetectGrammarEntry(filename)
	if entry == nil {
		return nil, fmt.Errorf("unsupported language for file: %s", filename)
	}
	return entry.Language(), nil
}

// ParseWithLanguageAndTree parses source bytes and returns both the AST and raw tree-sitter tree.
func ParseWithLanguageAndTree(src []byte, lang *gotreesitter.Language) (*ASTNode, *gotreesitter.Tree, error) {
	if lang == nil {
		return nil, nil, errors.New("nil language")
	}
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, nil, err
	}
	return BuildAST(tree.RootNode(), src, lang, nil), tree, nil
}

func Parse(src []byte, filename string) (*ASTNode, error) {
	lang, err := DetectLanguage(filename)
	if err != nil {
		return nil, err
	}
	ast, _, err := ParseWithLanguageAndTree(src, lang)
	return ast, err
}
