package treesitter

import (
	"embed"
	"io/fs"
	"path"

	"gopkg.in/yaml.v3"
)

type Rules struct {
	Flattened    []string          `yaml:"flattened"`
	Ignored      []string          `yaml:"ignored"`
	Aliased      map[string]string `yaml:"aliased"`
	LabelIgnored []string          `yaml:"label_ignored"`
	Scaffolding  []string          `yaml:"scaffolding"`
	Keywords     []string          `yaml:"keywords"`
	Declarations []string          `yaml:"declarations"`
	Pairs        []string          `yaml:"pairs"`
	Unordered    []string          `yaml:"unordered"`
}

//go:embed */rules.yml
var rulesFS embed.FS

var rulesCache map[string]Rules

func GetRules(lang string) *Rules {
	if rulesCache == nil {
		return nil
	}
	r, ok := rulesCache[lang]
	if !ok {
		return nil
	}
	return &r
}

func init() {
	rulesCache = make(map[string]Rules)
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
			rulesCache[lang] = r
		}
	}
}
