// Package config handles user configuration files for Diffmantic.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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

const (
	defaultSizeLimit = 1024
	defaultLineLimit = 10000
)

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
		size := defaultSizeLimit
		c.SizeLimit = &size
	}
	if c.LineLimit == nil {
		lines := defaultLineLimit
		c.LineLimit = &lines
	}
	return nil
}

// DefaultConfig returns the baseline configuration when no config file is found.
func DefaultConfig() Config {
	size := defaultSizeLimit
	lines := defaultLineLimit
	return Config{
		Format:          "",
		TabWidth:        4,
		IgnoreComments:  false,
		ParseErrorLimit: 0,
		SizeLimit:       &size,
		LineLimit:       &lines,
	}
}

// CandidateConfigDirs returns user configuration directory candidates in priority order.
func CandidateConfigDirs() []string {
	var dirs []string

	if runtime.GOOS == "windows" {
		if appData := os.Getenv("APPDATA"); appData != "" {
			dirs = append(dirs, filepath.Join(appData, "diffmantic"))
		}
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			dirs = append(dirs, filepath.Join(localAppData, "diffmantic"))
		}
	}

	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "diffmantic"))
	} else if home, err := os.UserHomeDir(); err == nil && home != "" {
		dirs = append(dirs, filepath.Join(home, ".config", "diffmantic"))
	}

	return dirs
}

// ConfigDir returns the primary path to the diffmantic configuration directory.
func ConfigDir() (string, error) {
	dirs := CandidateConfigDirs()
	if len(dirs) > 0 {
		return dirs[0], nil
	}
	return "", fmt.Errorf("could not determine user configuration directory")
}

// CandidateConfigFilePaths returns config file paths to try in order of preference.
func CandidateConfigFilePaths() ([]string, error) {
	dirs := CandidateConfigDirs()
	if len(dirs) == 0 {
		return nil, fmt.Errorf("could not determine user configuration directory")
	}

	var paths []string
	for _, dir := range dirs {
		paths = append(paths,
			filepath.Join(dir, "config.yml"),
			filepath.Join(dir, "config.yaml"),
		)
	}
	return paths, nil
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
