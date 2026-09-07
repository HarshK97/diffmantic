package treesitter

// Parse detects the language for the given filename and parses source bytes into an ASTNode.
func Parse(src []byte, filename string) (*ASTNode, error) {
	lang, err := DetectLanguage(filename)
	if err != nil {
		return nil, err
	}
	return ParseWithLanguage(src, lang.Name)
}
