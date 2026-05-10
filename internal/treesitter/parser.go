package treesitter

import (
	"fmt"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func DetectLanguage(filename string) (*gotreesitter.Language, error) {
	entry := grammars.DetectLanguage(filename)
	if entry == nil {
		return nil, fmt.Errorf("unsupported language for file: %s", filename)
	}
	return entry.Language(), nil
}

func Parse(src []byte, filename string) (*ASTNode, error) {
	lang, err := DetectLanguage(filename)
	if err != nil {
		return nil, err
	}
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(src)
	if err != nil {
		return nil, err
	}
	return BuildAST(tree.RootNode(), src, lang, nil), nil
}
