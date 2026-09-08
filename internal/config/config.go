// Package config handles user configuration files for Diffmantic.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all user configuration settings for diffmantic.
type Config struct {
	Format          string `yaml:"format"`
	TabWidth        int    `yaml:"tab_width"`
	IgnoreComments  bool   `yaml:"ignore_comments"`
	ParseErrorLimit int    `yaml:"parse_error_limit"`
	SizeLimit       *int   `yaml:"size_limit,omitempty"`
	LineLimit       *int   `yaml:"line_limit,omitempty"`
}

// UnmarshalYAML supports both modern config fields and legacy tui.tab_width.
func (c *Config) UnmarshalYAML(value *yaml.Node) error {
	type rawConfig Config
	var raw rawConfig
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*c = Config(raw)
	if c.TabWidth == 0 {
		c.TabWidth = 4
	}
	if c.SizeLimit == nil {
		c.SizeLimit = new(1024)
	}
	if c.LineLimit == nil {
		c.LineLimit = new(10000)
	}
	return nil
}

// DefaultConfig returns the baseline configuration when no config file is found.
func DefaultConfig() Config {
	return Config{
		Format:          "",
		TabWidth:        4,
		IgnoreComments:  false,
		ParseErrorLimit: 0,
		SizeLimit:       new(1024),
		LineLimit:       new(10000),
	}
}

// ConfigDir returns the path to the diffmantic config directory,
// respecting $XDG_CONFIG_HOME or falling back to ~/.config/diffmantic.
func ConfigDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "diffmantic"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "diffmantic"), nil
}

// CandidateConfigFilePaths returns config file paths to try in order of preference.
func CandidateConfigFilePaths() ([]string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return nil, err
	}
	return []string{
		filepath.Join(dir, "config.yml"),
		filepath.Join(dir, "config.yaml"),
	}, nil
}

// Load reads configuration from ~/.config/diffmantic/config.yml (or config.yaml).
// If no config file exists, it returns DefaultConfig.
func Load() (*Config, error) {
	cfg := DefaultConfig()

	paths, err := CandidateConfigFilePaths()
	if err != nil {
		return nil, fmt.Errorf("locating candidate config files: %w", err)
	}

	var loadedPath string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			loadedPath = p
			break
		}
	}

	if loadedPath != "" {
		data, err := os.ReadFile(loadedPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", loadedPath, err)
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", loadedPath, err)
		}
	}

	return &cfg, nil
}
