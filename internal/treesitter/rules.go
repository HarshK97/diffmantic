package treesitter

import (
	_ "embed"

	"gopkg.in/yaml.v3"
)

type Rules struct {
	Flattened    []string          `yaml:"flattened"`
	Ignored      []string          `yaml:"ignored"`
	Aliased      map[string]string `yaml:"aliased"`
	LabelIgnored []string          `yaml:"label_ignored"`
	Scaffolding  []string          `yaml:"scaffolding"`
}

type RulesConfig struct {
	Languages map[string]Rules `yaml:",inline"`
}

var (
	//go:embed rules.yml
	rulesYAML []byte

	rulesCache map[string]Rules
)

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
	var config RulesConfig
	if err := yaml.Unmarshal(rulesYAML, &config); err != nil {
		panic("failed to load rules.yml: " + err.Error())
	}
	rulesCache = config.Languages
}
