package treesitter

import (
	"embed"
	"io/fs"
	"path"
	"slices"

	"gopkg.in/yaml.v3"
)

// Rules configures language-specific AST transformations and node matching.
type Rules struct {
	Flattened       []string          `yaml:"flattened"`
	Ignored         []string          `yaml:"ignored"`
	Aliased         map[string]string `yaml:"aliased"`
	LabelIgnored    []string          `yaml:"label_ignored"`
	Scaffolding     []string          `yaml:"scaffolding"`
	Keywords        []string          `yaml:"keywords"`
	Declarations    []string          `yaml:"declarations"`
	Identifiers     []string          `yaml:"identifiers"`
	Blocks          []string          `yaml:"blocks"`
	Pairs           []string          `yaml:"pairs"`
	Unordered       []string          `yaml:"unordered"`
	EquivalentTypes [][]string        `yaml:"equivalent_types"`
	Comments        []string          `yaml:"comments"`

	flattenedSet    map[string]struct{}
	ignoredSet      map[string]struct{}
	labelIgnoredSet map[string]struct{}
	keywordsSet     map[string]struct{}
	declarationsSet map[string]struct{}
	identifiersSet  map[string]struct{}
	scaffoldingSet  map[string]struct{}
	blocksSet       map[string]struct{}
	unorderedSet    map[string]struct{}
	commentsSet     map[string]struct{}
	equivGroups     map[string][]int
}

func (r *Rules) compileSets() {
	if len(r.Flattened) > 0 {
		r.flattenedSet = make(map[string]struct{}, len(r.Flattened))
		for _, s := range r.Flattened {
			r.flattenedSet[s] = struct{}{}
		}
	}
	if len(r.Ignored) > 0 {
		r.ignoredSet = make(map[string]struct{}, len(r.Ignored))
		for _, s := range r.Ignored {
			r.ignoredSet[s] = struct{}{}
		}
	}
	if len(r.LabelIgnored) > 0 {
		r.labelIgnoredSet = make(map[string]struct{}, len(r.LabelIgnored))
		for _, s := range r.LabelIgnored {
			r.labelIgnoredSet[s] = struct{}{}
		}
	}
	if len(r.Keywords) > 0 {
		r.keywordsSet = make(map[string]struct{}, len(r.Keywords))
		for _, s := range r.Keywords {
			r.keywordsSet[s] = struct{}{}
		}
	}
	if len(r.Declarations) > 0 {
		r.declarationsSet = make(map[string]struct{}, len(r.Declarations))
		for _, s := range r.Declarations {
			r.declarationsSet[s] = struct{}{}
		}
	}
	if len(r.Identifiers) > 0 {
		r.identifiersSet = make(map[string]struct{}, len(r.Identifiers))
		for _, s := range r.Identifiers {
			r.identifiersSet[s] = struct{}{}
		}
	}
	if len(r.Scaffolding) > 0 {
		r.scaffoldingSet = make(map[string]struct{}, len(r.Scaffolding))
		for _, s := range r.Scaffolding {
			r.scaffoldingSet[s] = struct{}{}
		}
	}
	if len(r.Blocks) > 0 {
		r.blocksSet = make(map[string]struct{}, len(r.Blocks))
		for _, s := range r.Blocks {
			r.blocksSet[s] = struct{}{}
		}
	}
	if len(r.Unordered) > 0 {
		r.unorderedSet = make(map[string]struct{}, len(r.Unordered))
		for _, s := range r.Unordered {
			r.unorderedSet[s] = struct{}{}
		}
	}
	if len(r.Comments) > 0 {
		r.commentsSet = make(map[string]struct{}, len(r.Comments))
		for _, s := range r.Comments {
			r.commentsSet[s] = struct{}{}
		}
	}
	if len(r.EquivalentTypes) > 0 {
		r.equivGroups = make(map[string][]int)
		for idx, group := range r.EquivalentTypes {
			for _, typ := range group {
				r.equivGroups[typ] = append(r.equivGroups[typ], idx)
			}
		}
	}
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
			for _, id2 := range g2 {
				if id1 == id2 {
					return true
				}
			}
		}
		return false
	}
	for _, group := range r.EquivalentTypes {
		has1, has2 := false, false
		for _, typ := range group {
			if typ == t1 {
				has1 = true
			}
			if typ == t2 {
				has2 = true
			}
		}
		if has1 && has2 {
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

// IsBlock checks if this node type is a code block.
func (r *Rules) IsBlock(nodeType string) bool {
	if r == nil {
		return false
	}
	if len(r.blocksSet) > 0 {
		_, ok := r.blocksSet[nodeType]
		return ok
	}
	if len(r.Blocks) > 0 {
		return slices.Contains(r.Blocks, nodeType)
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

//go:embed */rules.yml
var rulesFS embed.FS

var rulesCache map[string]*Rules

// GetRules returns the compiled AST rules for a language, or nil if none exist.
func GetRules(lang string) *Rules {
	if rulesCache == nil {
		return nil
	}
	return rulesCache[lang]
}

func init() {
	rulesCache = make(map[string]*Rules)
	entries, err := fs.ReadDir(rulesFS, ".")
	if err != nil {
		panic("failed to read embedded rules directory: " + err.Error())
	}
	for _, entry := range entries {
		if entry.IsDir() {
			lang := entry.Name()
			rulePath := path.Join(lang, "rules.yml")
			data, err := rulesFS.ReadFile(rulePath)
			if err != nil {
				continue
			}
			var r Rules
			if err := yaml.Unmarshal(data, &r); err != nil {
				panic("failed to load " + rulePath + ": " + err.Error())
			}
			r.compileSets()
			rulesCache[lang] = &r
		}
	}
}
