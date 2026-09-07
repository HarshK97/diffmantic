package rules

import (
	"slices"
	"strings"
)

// LanguageKind classifies grammars into high-level parsing and diffing categories.
type LanguageKind uint8

const (
	// KindCode is for general programming languages (Go, Rust, Python, C++, etc.).
	KindCode LanguageKind = iota
	// KindData is for structured key-value formats (JSON, YAML, TOML).
	KindData
	// KindMarkup is for markup and styling languages (HTML, CSS).
	KindMarkup
)

// Rules configures language-specific AST transformations and node matching.
type Rules struct {
	Kind                  LanguageKind
	Flattened             []string
	Ignored               []string
	Aliased               map[string]string
	LabelIgnored          []string
	Scaffolding           []string
	Keywords              []string
	Declarations          []string
	Identifiers           []string
	Blocks                []string
	Wrappers              []string
	Pairs                 []string
	Unordered             []string
	EquivalentTypes       [][]string
	Comments              []string
	Calls                 []string
	ScopedDeclarations    []string
	Indexed               []string // Subscript nodes with prefix receivers (e.g. arr[i]).
	LocalVarDeclarations  []string
	ContainerDeclarations []string // Major declaration scope boundaries (functions, classes, structs, etc.).
	Closures              []string // Anonymous functions, lambdas, and callbacks.
	Types                 []string // Type annotations and type expressions.

	flattenedSet             map[string]struct{}
	ignoredSet               map[string]struct{}
	labelIgnoredSet          map[string]struct{}
	keywordsSet              map[string]struct{}
	declarationsSet          map[string]struct{}
	identifiersSet           map[string]struct{}
	scaffoldingSet           map[string]struct{}
	blocksSet                map[string]struct{}
	wrappersSet              map[string]struct{}
	pairsSet                 map[string]struct{}
	unorderedSet             map[string]struct{}
	commentsSet              map[string]struct{}
	callsSet                 map[string]struct{}
	scopedDeclarationsSet    map[string]struct{}
	indexedSet               map[string]struct{}
	localVarDeclarationsSet  map[string]struct{}
	containerDeclarationsSet map[string]struct{}
	closuresSet              map[string]struct{}
	typesSet                 map[string]struct{}
	equivGroups              map[string][]int
}

func sliceToSet[T comparable](items []T) map[T]struct{} {
	if len(items) == 0 {
		return nil
	}
	set := make(map[T]struct{}, len(items))
	for _, item := range items {
		set[item] = struct{}{}
	}
	return set
}

// CompileSets builds the internal lookup sets for fast querying.
func (r *Rules) CompileSets() {
	r.flattenedSet = sliceToSet(r.Flattened)
	r.ignoredSet = sliceToSet(r.Ignored)
	r.labelIgnoredSet = sliceToSet(r.LabelIgnored)
	r.keywordsSet = sliceToSet(r.Keywords)
	r.declarationsSet = sliceToSet(r.Declarations)
	r.identifiersSet = sliceToSet(r.Identifiers)
	r.scaffoldingSet = sliceToSet(r.Scaffolding)
	r.blocksSet = sliceToSet(r.Blocks)
	r.wrappersSet = sliceToSet(r.Wrappers)
	r.pairsSet = sliceToSet(r.Pairs)
	r.unorderedSet = sliceToSet(r.Unordered)
	r.commentsSet = sliceToSet(r.Comments)
	r.callsSet = sliceToSet(r.Calls)
	r.scopedDeclarationsSet = sliceToSet(r.ScopedDeclarations)
	r.indexedSet = sliceToSet(r.Indexed)
	r.localVarDeclarationsSet = sliceToSet(r.LocalVarDeclarations)
	r.containerDeclarationsSet = sliceToSet(r.ContainerDeclarations)
	r.closuresSet = sliceToSet(r.Closures)
	r.typesSet = sliceToSet(r.Types)
	if len(r.EquivalentTypes) > 0 {
		r.equivGroups = make(map[string][]int)
		for idx, group := range r.EquivalentTypes {
			for _, typ := range group {
				r.equivGroups[typ] = append(r.equivGroups[typ], idx)
			}
		}
	}
}

// GetKind returns the language category (KindCode, KindData, or KindMarkup).
func (r *Rules) GetKind() LanguageKind {
	if r == nil {
		return KindCode
	}
	return r.Kind
}

// IsCall reports whether nodeType is a function, method, or macro invocation.
func (r *Rules) IsCall(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.callsSet) > 0 {
		_, ok := r.callsSet[nodeType]
		return ok
	}
	return slices.Contains(r.Calls, nodeType)
}

// IsIndexed reports whether nodeType is a subscript container with a prefix receiver.
func (r *Rules) IsIndexed(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.indexedSet) > 0 {
		_, ok := r.indexedSet[nodeType]
		return ok
	}
	return slices.Contains(r.Indexed, nodeType)
}

// IsComment reports whether nodeType is a comment in the language grammar.
func (r *Rules) IsComment(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.commentsSet) > 0 {
		_, ok := r.commentsSet[nodeType]
		return ok
	}
	return slices.Contains(r.Comments, nodeType)
}

// IsDeclaration reports whether nodeType is a declaration.
func (r *Rules) IsDeclaration(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.declarationsSet) > 0 {
		_, ok := r.declarationsSet[nodeType]
		return ok
	}
	return slices.Contains(r.Declarations, nodeType)
}

// IsScopedDeclaration reports whether nodeType is a declaration with an explicit signature receiver or scope.
func (r *Rules) IsScopedDeclaration(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.scopedDeclarationsSet) > 0 {
		_, ok := r.scopedDeclarationsSet[nodeType]
		return ok
	}
	return slices.Contains(r.ScopedDeclarations, nodeType)
}

// IsLocalVarDeclaration reports whether nodeType is a local variable declaration.
func (r *Rules) IsLocalVarDeclaration(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.localVarDeclarationsSet) > 0 {
		_, ok := r.localVarDeclarationsSet[nodeType]
		return ok
	}
	return slices.Contains(r.LocalVarDeclarations, nodeType)
}

// IsContainerDeclaration reports whether nodeType is a major container declaration
// (such as a function, method, class, struct, interface, trait, or enum).
func (r *Rules) IsContainerDeclaration(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.containerDeclarationsSet) > 0 {
		_, ok := r.containerDeclarationsSet[nodeType]
		return ok
	}
	return slices.Contains(r.ContainerDeclarations, nodeType)
}

// IsClosure reports whether nodeType is an anonymous function, closure, or lambda callback.
func (r *Rules) IsClosure(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.closuresSet) > 0 {
		_, ok := r.closuresSet[nodeType]
		return ok
	}
	return slices.Contains(r.Closures, nodeType)
}

// IsType reports whether nodeType is a type annotation or type expression in the language.
func (r *Rules) IsType(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.typesSet) > 0 {
		_, ok := r.typesSet[nodeType]
		return ok
	}
	return slices.Contains(r.Types, nodeType)
}

// IsIdentifier reports whether nodeType is an identifier token.
func (r *Rules) IsIdentifier(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.identifiersSet) > 0 {
		_, ok := r.identifiersSet[nodeType]
		return ok
	}
	return slices.Contains(r.Identifiers, nodeType)
}

// IsScaffolding reports whether nodeType is scaffolding.
func (r *Rules) IsScaffolding(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.scaffoldingSet) > 0 {
		_, ok := r.scaffoldingSet[nodeType]
		return ok
	}
	return slices.Contains(r.Scaffolding, nodeType)
}

// AreTypesEquivalent reports whether t1 and t2 belong to the same equivalent_types group.
func (r *Rules) AreTypesEquivalent(t1, t2 string) bool {
	if r == nil || t1 == "" || t2 == "" {
		return t1 == t2
	}
	if t1 == t2 {
		return true
	}
	if len(r.equivGroups) > 0 {
		g1, ok1 := r.equivGroups[t1]
		g2, ok2 := r.equivGroups[t2]
		if !ok1 || !ok2 {
			return false
		}
		for _, id1 := range g1 {
			if slices.Contains(g2, id1) {
				return true
			}
		}
		return false
	}
	for _, group := range r.EquivalentTypes {
		if slices.Contains(group, t1) && slices.Contains(group, t2) {
			return true
		}
	}
	return false
}

// IsIgnored checks if a node type or label is filtered out when building the AST.
func (r *Rules) IsIgnored(nodeType, label string) bool {
	if r == nil {
		return false
	}
	if len(r.ignoredSet) > 0 {
		if _, ok := r.ignoredSet[nodeType]; ok {
			return true
		}
		if label != "" {
			if _, ok := r.ignoredSet[label]; ok {
				return true
			}
		}
		return false
	}
	return slices.Contains(r.Ignored, nodeType) || (label != "" && slices.Contains(r.Ignored, label))
}

// IsKeyword checks if a node type or label is a language keyword.
func (r *Rules) IsKeyword(nodeType, label string) bool {
	if r == nil {
		return false
	}
	if len(r.keywordsSet) > 0 {
		if _, ok := r.keywordsSet[nodeType]; ok {
			return true
		}
		if label != "" {
			if _, ok := r.keywordsSet[label]; ok {
				return true
			}
		}
		return false
	}
	return slices.Contains(r.Keywords, nodeType) || (label != "" && slices.Contains(r.Keywords, label))
}

// IsLabelIgnored checks if node labels should be dropped for this type.
func (r *Rules) IsLabelIgnored(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.labelIgnoredSet) > 0 {
		_, ok := r.labelIgnoredSet[nodeType]
		return ok
	}
	return slices.Contains(r.LabelIgnored, nodeType)
}

// IsPair checks if nodeType represents a key-value property pair.
func (r *Rules) IsPair(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.pairsSet) > 0 {
		_, ok := r.pairsSet[nodeType]
		return ok
	}
	return slices.Contains(r.Pairs, nodeType)
}

// IsUnordered checks if child order doesn't matter for this container.
func (r *Rules) IsUnordered(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.unorderedSet) > 0 {
		_, ok := r.unorderedSet[nodeType]
		return ok
	}
	return slices.Contains(r.Unordered, nodeType)
}

// IsFlattened checks if intermediate nodes of this type should merge into their parent.
func (r *Rules) IsFlattened(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.flattenedSet) > 0 {
		_, ok := r.flattenedSet[nodeType]
		return ok
	}
	return slices.Contains(r.Flattened, nodeType)
}

// IsFlattened reports whether nodeType is configured as flattened in any language rule set.
func IsFlattened(nodeType string) bool {
	if nodeType == "" {
		return false
	}
	for _, r := range registry {
		if r.IsFlattened(nodeType) {
			return true
		}
	}
	return false
}

// IsBlock checks if this node type is a code block.
func (r *Rules) IsBlock(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.blocksSet) > 0 {
		_, ok := r.blocksSet[nodeType]
		return ok
	}
	return slices.Contains(r.Blocks, nodeType)
}

// IsWrapper reports whether nodeType is a syntactic wrapper (e.g. parentheses, generics, subscripts).
func (r *Rules) IsWrapper(nodeType string) bool {
	if r == nil || nodeType == "" {
		return false
	}
	if len(r.wrappersSet) > 0 {
		_, ok := r.wrappersSet[nodeType]
		return ok
	}
	if len(r.Wrappers) > 0 {
		return slices.Contains(r.Wrappers, nodeType)
	}
	return false
}

// Alias returns the replacement node type if one exists for the label or node type.
func (r *Rules) Alias(nodeType, label string) (string, bool) {
	if r == nil || len(r.Aliased) == 0 {
		return "", false
	}
	if label != "" {
		if a, ok := r.Aliased[label]; ok {
			return a, true
		}
	}
	if a, ok := r.Aliased[nodeType]; ok {
		return a, true
	}
	return "", false
}

// IsDelimiter reports whether nodeType or label is a delimiter token (semicolon or comma).
func (r *Rules) IsDelimiter(nodeType, label string) bool {
	return label == ";" || label == "," || nodeType == "semicolon" || nodeType == "comma" || nodeType == "_automatic_semicolon"
}

// IsOperatorLiteral reports whether nodeType is an aliased operator literal.
func (r *Rules) IsOperatorLiteral(nodeType string) bool {
	if nodeType == "" {
		return false
	}
	return strings.HasSuffix(nodeType, "_operator_literal")
}

// DefaultRootType returns the top-level root AST node type for the language.
func (r *Rules) DefaultRootType() string {
	if r != nil && len(r.Scaffolding) > 0 {
		return r.Scaffolding[0]
	}
	return ""
}

// IsFlattenedType reports whether nodeType is configured as flattened in any language rule set.
func IsFlattenedType(nodeType string) bool {
	for _, r := range registry {
		if r.IsFlattened(nodeType) {
			return true
		}
	}
	return false
}

// IsComment reports whether nodeType is configured as a comment in any language rule set.
func IsComment(nodeType string) bool {
	for _, r := range registry {
		if r.IsComment(nodeType) {
			return true
		}
	}
	return false
}

// IsDeclaration reports whether nodeType is configured as a declaration in any language rule set.
func IsDeclaration(nodeType string) bool {
	for _, r := range registry {
		if r.IsDeclaration(nodeType) {
			return true
		}
	}
	return false
}

// IsLocalVarDeclaration reports whether nodeType is configured as a local variable declaration in any language rule set.
func IsLocalVarDeclaration(nodeType string) bool {
	for _, r := range registry {
		if r.IsLocalVarDeclaration(nodeType) {
			return true
		}
	}
	return false
}

// IsIdentifier reports whether nodeType is configured as an identifier in any language rule set.
func IsIdentifier(nodeType string) bool {
	for _, r := range registry {
		if r.IsIdentifier(nodeType) {
			return true
		}
	}
	return false
}

// IsScaffolding reports whether nodeType is configured as scaffolding in any language rule set.
func IsScaffolding(nodeType string) bool {
	for _, r := range registry {
		if r.IsScaffolding(nodeType) {
			return true
		}
	}
	return false
}

// IsBlock reports whether nodeType is configured as a block in any language rule set.
func IsBlock(nodeType string) bool {
	for _, r := range registry {
		if r.IsBlock(nodeType) {
			return true
		}
	}
	return false
}

// IsWrapper reports whether nodeType is configured as a wrapper in any language rule set.
func IsWrapper(nodeType string) bool {
	for _, r := range registry {
		if r.IsWrapper(nodeType) {
			return true
		}
	}
	return false
}

// IsOperatorLiteral reports whether nodeType is configured as an operator literal in any language rule set.
func IsOperatorLiteral(nodeType string) bool {
	return defaultRules.IsOperatorLiteral(nodeType)
}

// IsKeyword reports whether nodeType or label is configured as a keyword in any language rule set.
func IsKeyword(nodeType, label string) bool {
	for _, r := range registry {
		if r.IsKeyword(nodeType, label) {
			return true
		}
	}
	return false
}

// IsPunctuation reports whether a string is a structural punctuation token (braces, brackets, parentheses, delimiters).
func (r *Rules) IsPunctuation(token string) bool {
	switch token {
	case "}", "};", "],", "]", ")", ");", "},", "{", "begin", "end", ";", ",", "(", "[", ":", "->", "=>", "\"", "'", "`":
		return true
	}
	if r == nil || token == "" {
		return false
	}
	return r.IsIgnored(token, token) || r.IsDelimiter(token, token)
}

// IsPunctuation reports whether a string is a structural punctuation token in any language rule set.
func IsPunctuation(token string) bool {
	switch token {
	case "}", "};", "],", "]", ")", ");", "},", "{", "begin", "end", ";", ",", "(", "[", ":", "->", "=>", "\"", "'", "`":
		return true
	}
	for _, r := range registry {
		if r.IsPunctuation(token) {
			return true
		}
	}
	return false
}

// IsDelimiter reports whether nodeType or label is a delimiter token (semicolon or comma).
func IsDelimiter(nodeType, label string) bool {
	return label == ";" || label == "," || nodeType == "semicolon" || nodeType == "comma" || nodeType == "_automatic_semicolon"
}

// IsCall reports whether nodeType is configured as a call in any language rule set.
func IsCall(nodeType string) bool {
	for _, r := range registry {
		if r.IsCall(nodeType) {
			return true
		}
	}
	return false
}

// IsIndexed reports whether nodeType is configured as an indexed container in any language rule set.
func IsIndexed(nodeType string) bool {
	for _, r := range registry {
		if r.IsIndexed(nodeType) {
			return true
		}
	}
	return false
}

// IsContainerDeclaration reports whether nodeType is configured as a container declaration in any language rule set.
func IsContainerDeclaration(nodeType string) bool {
	if nodeType == "" {
		return false
	}
	for _, r := range registry {
		if r.IsContainerDeclaration(nodeType) {
			return true
		}
	}
	return false
}

// IsClosure reports whether nodeType is configured as a closure in any language rule set.
func IsClosure(nodeType string) bool {
	if nodeType == "" {
		return false
	}
	for _, r := range registry {
		if r.IsClosure(nodeType) {
			return true
		}
	}
	return false
}

// IsType reports whether nodeType is a type annotation or type expression in any language rule set.
func IsType(nodeType string) bool {
	if nodeType == "" {
		return false
	}
	for _, r := range registry {
		if r.IsType(nodeType) {
			return true
		}
	}
	return false
}

var registry = map[string]*Rules{
	"c":          cRules,
	"cpp":        cppRules,
	"css":        cssRules,
	"go":         golangRules,
	"html":       htmlRules,
	"java":       javaRules,
	"javascript": javascriptRules,
	"json":       jsonRules,
	"lua":        luaRules,
	"php":        phpRules,
	"python":     pythonRules,
	"ruby":       rubyRules,
	"rust":       rustRules,
	"toml":       tomlRules,
	"tsx":        tsxRules,
	"typescript": typescriptRules,
	"yaml":       yamlRules,
	"zig":        zigRules,
}

var defaultRules = &Rules{}

// Get returns the compiled AST rules for a language, or a default empty rule set if none exist.
func Get(lang string) *Rules {
	if r, ok := registry[lang]; ok && r != nil {
		return r
	}
	return defaultRules
}

func init() {
	defaultRules.CompileSets()
	for _, r := range registry {
		r.CompileSets()
	}
}
