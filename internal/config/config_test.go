package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Theme != "" {
		t.Errorf("expected empty default theme, got %q", cfg.Theme)
	}
	if cfg.ThemeStyle != "dark" {
		t.Errorf("expected default theme_style 'dark', got %q", cfg.ThemeStyle)
	}
	if cfg.TUI.TabWidth != 4 {
		t.Errorf("expected default tab_width 4, got %d", cfg.TUI.TabWidth)
	}
	if !cfg.TUI.Mouse {
		t.Errorf("expected default mouse=true")
	}
	if cfg.TUI.Icons != "unicode" {
		t.Errorf("expected default icons='unicode', got %q", cfg.TUI.Icons)
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	// Point to an empty directory to ensure we cleanly fall back to defaults
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading non-existent config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.ThemeStyle != "dark" {
		t.Errorf("expected default theme_style 'dark', got %q", cfg.ThemeStyle)
	}
}

func TestLoadValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "diffmantic")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configYAML := `
theme: my-custom-theme
theme_style: light
format: json
ignore_comments: true
parse_error_limit: 5
tui:
  tab_width: 2
  mouse: false
  icons: ascii
themes:
  my-custom-theme:
    dark: false
    ui:
      base: "#ffffff"
      text: "#000000"
    actions:
      insert_fg: "#00ff00"
      delete_fg: "#ff0000"
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("failed to write config.yml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Theme != "my-custom-theme" {
		t.Errorf("theme = %q, want 'my-custom-theme'", cfg.Theme)
	}
	if cfg.ThemeStyle != "light" {
		t.Errorf("theme_style = %q, want 'light'", cfg.ThemeStyle)
	}
	if cfg.Format != "json" {
		t.Errorf("format = %q, want 'json'", cfg.Format)
	}
	if !cfg.IgnoreComments {
		t.Errorf("ignore_comments = false, want true")
	}
	if cfg.ParseErrorLimit != 5 {
		t.Errorf("parse_error_limit = %d, want 5", cfg.ParseErrorLimit)
	}
	if cfg.TUI.TabWidth != 2 {
		t.Errorf("tui.tab_width = %d, want 2", cfg.TUI.TabWidth)
	}
	if cfg.TUI.Mouse {
		t.Errorf("tui.mouse = true, want false")
	}
	if cfg.TUI.Icons != "ascii" {
		t.Errorf("tui.icons = %q, want 'ascii'", cfg.TUI.Icons)
	}

	customTheme, exists := cfg.Themes["my-custom-theme"]
	if !exists {
		t.Fatal("expected 'my-custom-theme' to be present in cfg.Themes")
	}
	if customTheme.UI.Base != "#ffffff" {
		t.Errorf("customTheme.UI.Base = %q, want '#ffffff'", customTheme.UI.Base)
	}
	if customTheme.Actions.InsertFg != "#00ff00" {
		t.Errorf("customTheme.Actions.InsertFg = %q, want '#00ff00'", customTheme.Actions.InsertFg)
	}
}

func TestLoadValidConfigYAMLFallback(t *testing.T) {
	// Verify config.yaml works when config.yml isn't present
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "diffmantic")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configYAML := `
theme: latte
format: actions
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config.yaml: %v", err)
	}

	if cfg.Theme != "latte" {
		t.Errorf("theme = %q, want 'latte'", cfg.Theme)
	}
	if cfg.Format != "actions" {
		t.Errorf("format = %q, want 'actions'", cfg.Format)
	}
}

func TestLoadExternalThemes(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	themesDir := filepath.Join(tmpDir, "diffmantic", "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("failed to create themes dir: %v", err)
	}

	themeYAML := `
dark: true
ui:
  base: "#2e3440"
  text: "#eceff4"
actions:
  insert_fg: "#a3be8c"
  delete_fg: "#bf616a"
`
	if err := os.WriteFile(filepath.Join(themesDir, "nord.yml"), []byte(themeYAML), 0o644); err != nil {
		t.Fatalf("failed to write nord.yml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config with external themes: %v", err)
	}

	nord, exists := cfg.Themes["nord"]
	if !exists {
		t.Fatal("expected 'nord' theme loaded from themes/nord.yml")
	}
	if nord.Name != "nord" {
		t.Errorf("nord.Name = %q, want 'nord'", nord.Name)
	}
	if nord.UI.Base != "#2e3440" {
		t.Errorf("nord.UI.Base = %q, want '#2e3440'", nord.UI.Base)
	}
}

func TestLoadExternalThemes_MalformedError(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	themesDir := filepath.Join(tmpDir, "diffmantic", "themes")
	if err := os.MkdirAll(themesDir, 0o755); err != nil {
		t.Fatalf("failed to create themes dir: %v", err)
	}

	invalidYAML := "dark: [broken"
	if err := os.WriteFile(filepath.Join(themesDir, "bad.yml"), []byte(invalidYAML), 0o644); err != nil {
		t.Fatalf("failed to write bad.yml: %v", err)
	}

	_, err := Load()
	if err == nil {
		t.Fatal("expected error when loading malformed external theme YAML, got nil")
	}
	if !strings.Contains(err.Error(), "parsing theme file") {
		t.Errorf("expected 'parsing theme file' error, got: %v", err)
	}
}
