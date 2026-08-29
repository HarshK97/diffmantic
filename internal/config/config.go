// Package config handles user configuration files and custom theme definitions.
package config

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all user configuration settings for diffmantic.
type Config struct {
	Theme           string                 `yaml:"theme"`
	ThemeStyle      string                 `yaml:"theme_style"`
	Format          string                 `yaml:"format"`
	IgnoreComments  bool                   `yaml:"ignore_comments"`
	ParseErrorLimit int                    `yaml:"parse_error_limit"`
	TUI             TUIConfig              `yaml:"tui"`
	Themes          map[string]ThemeConfig `yaml:"themes"`
}

// TUIConfig controls TUI layout, mouse support, and glyph styles.
type TUIConfig struct {
	TabWidth int    `yaml:"tab_width"`
	Mouse    bool   `yaml:"mouse"`
	Icons    string `yaml:"icons"` // "unicode" | "ascii" | "nerd-font"
}

// ThemeConfig defines overrides for a color theme palette.
type ThemeConfig struct {
	Name    string             `yaml:"name"`
	Dark    *bool              `yaml:"dark"`
	UI      UIColorsConfig     `yaml:"ui"`
	Actions ActionColorsConfig `yaml:"actions"`
}

// UIColorsConfig defines hex color overrides for TUI chrome.
type UIColorsConfig struct {
	Base      string `yaml:"base"`
	Mantle    string `yaml:"mantle"`
	Crust     string `yaml:"crust"`
	Surface0  string `yaml:"surface0"`
	Surface1  string `yaml:"surface1"`
	Surface2  string `yaml:"surface2"`
	Overlay0  string `yaml:"overlay0"`
	Overlay1  string `yaml:"overlay1"`
	Overlay2  string `yaml:"overlay2"`
	Subtext0  string `yaml:"subtext0"`
	Subtext1  string `yaml:"subtext1"`
	Text      string `yaml:"text"`
	Lavender  string `yaml:"lavender"`
	Mauve     string `yaml:"mauve"`
	Blue      string `yaml:"blue"`
	Green     string `yaml:"green"`
	Peach     string `yaml:"peach"`
	Sky       string `yaml:"sky"`
	Yellow    string `yaml:"yellow"`
	Pink      string `yaml:"pink"`
	Red       string `yaml:"red"`
	Teal      string `yaml:"teal"`
	Rosewater string `yaml:"rosewater"`
	Sapphire  string `yaml:"sapphire"`
	Maroon    string `yaml:"maroon"`
	Flamingo  string `yaml:"flamingo"`
}

// ActionColorsConfig defines hex colors for AST diff actions.
type ActionColorsConfig struct {
	DeleteFg     string `yaml:"delete_fg"`
	InsertFg     string `yaml:"insert_fg"`
	UpdateFg     string `yaml:"update_fg"`
	MoveFg       string `yaml:"move_fg"`
	MoveUpdateFg string `yaml:"move_update_fg"`

	DeleteBg     string `yaml:"delete_bg"`
	InsertBg     string `yaml:"insert_bg"`
	UpdateBg     string `yaml:"update_bg"`
	MoveBg       string `yaml:"move_bg"`
	MoveUpdateBg string `yaml:"move_update_bg"`
}

// DefaultConfig returns the baseline configuration when no config file is found.
func DefaultConfig() Config {
	return Config{
		Theme:           "",
		ThemeStyle:      "dark",
		Format:          "",
		IgnoreComments:  false,
		ParseErrorLimit: 0,
		TUI: TUIConfig{
			TabWidth: 4,
			Mouse:    true,
			Icons:    "unicode",
		},
		Themes: make(map[string]ThemeConfig),
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

// ThemesDir returns the path to the directory where external theme files live.
func ThemesDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "themes"), nil
}

// Load reads configuration from ~/.config/diffmantic/config.yml (or config.yaml)
// and any external themes from ~/.config/diffmantic/themes/*.{yml,yaml}.
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
		if cfg.Themes == nil {
			cfg.Themes = make(map[string]ThemeConfig)
		}
	}

	// Load any standalone theme files dropped into themes/
	if themesDir, err := ThemesDir(); err == nil {
		extThemes, err := LoadExternalThemes(themesDir)
		if err != nil {
			return nil, fmt.Errorf("loading external themes from %s: %w", themesDir, err)
		}
		for name, th := range extThemes {
			if _, exists := cfg.Themes[name]; !exists {
				cfg.Themes[name] = th
			}
		}
	}

	return &cfg, nil
}

// LoadExternalThemes loads all .yml and .yaml theme definitions from the given directory.
func LoadExternalThemes(dir string) (map[string]ThemeConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	themes := make(map[string]ThemeConfig)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".yml" && ext != ".yaml" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading theme file %s: %w", filePath, err)
		}

		var tc ThemeConfig
		if err := yaml.Unmarshal(data, &tc); err != nil {
			return nil, fmt.Errorf("parsing theme file %s: %w", filePath, err)
		}

		themeName := cmp.Or(tc.Name, strings.TrimSuffix(entry.Name(), ext))
		tc.Name = themeName
		themes[themeName] = tc
	}

	return themes, nil
}
