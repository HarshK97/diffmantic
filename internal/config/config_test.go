package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Format != "" {
		t.Errorf("expected empty default format, got %q", cfg.Format)
	}
	if cfg.TabWidth != 4 {
		t.Errorf("expected default tab_width 4, got %d", cfg.TabWidth)
	}
	if cfg.SizeLimit == nil || *cfg.SizeLimit != 1024 {
		t.Errorf("expected default size limit 1024")
	}
	if cfg.LineLimit == nil || *cfg.LineLimit != 10000 {
		t.Errorf("expected default line limit 10000")
	}
}

func TestLoadNonExistentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading non-existent config: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config")
	}
	if cfg.TabWidth != 4 {
		t.Errorf("expected default tab_width 4, got %d", cfg.TabWidth)
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
format: json
tab_width: 8
ignore_comments: true
parse_error_limit: 5
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("failed to write config.yml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config: %v", err)
	}

	if cfg.Format != "json" {
		t.Errorf("format = %q, want 'json'", cfg.Format)
	}
	if cfg.TabWidth != 8 {
		t.Errorf("tab_width = %d, want 8", cfg.TabWidth)
	}
	if !cfg.IgnoreComments {
		t.Errorf("ignore_comments = false, want true")
	}
	if cfg.ParseErrorLimit != 5 {
		t.Errorf("parse_error_limit = %d, want 5", cfg.ParseErrorLimit)
	}
}

func TestLoadValidConfigYAMLFallback(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmpDir)

	configDir := filepath.Join(tmpDir, "diffmantic")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configYAML := `
format: actions
tab_width: 2
`
	if err := os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(configYAML), 0o644); err != nil {
		t.Fatalf("failed to write config.yaml: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error loading config.yaml: %v", err)
	}

	if cfg.Format != "actions" {
		t.Errorf("format = %q, want 'actions'", cfg.Format)
	}
	if cfg.TabWidth != 2 {
		t.Errorf("tab_width = %d, want 2", cfg.TabWidth)
	}
}
