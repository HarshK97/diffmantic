package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func newTestCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringP("format", "f", "", "")
	cmd.Flags().BoolP("unified", "u", false, "")
	return cmd
}

func TestResolveFormat(t *testing.T) {
	origTermFdWidth := termFdWidthFn
	origIsTerminal := isTerminalFn
	t.Cleanup(func() {
		termFdWidthFn = origTermFdWidth
		isTerminalFn = origIsTerminal
	})

	tests := []struct {
		name       string
		setupCmd   func(c *cobra.Command)
		env        map[string]string
		isTerm     bool
		termFdW    int
		wantFormat string
	}{
		{
			name: "explicit format unified",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "unified")
			},
			wantFormat: "unified",
		},
		{
			name: "explicit format sbs alias",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "sbs")
			},
			wantFormat: "side-by-side",
		},
		{
			name: "explicit format json",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "json")
			},
			wantFormat: "json",
		},
		{
			name: "explicit -u flag",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("unified", "true")
			},
			wantFormat: "unified",
		},
		{
			name: "DIFFM_FORMAT env var",
			env: map[string]string{
				"DIFFM_FORMAT": "unified",
			},
			wantFormat: "unified",
		},
		{
			name: "DIFFM_FORMAT whitespace only falls through to width detection",
			env: map[string]string{
				"DIFFM_FORMAT": "   ",
			},
			isTerm:     true,
			termFdW:    140,
			wantFormat: "side-by-side",
		},
		{
			name: "format flag overrides DIFFM_FORMAT",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "side-by-side")
			},
			env: map[string]string{
				"DIFFM_FORMAT": "unified",
			},
			wantFormat: "side-by-side",
		},
		{
			name:       "auto format on narrow terminal (<120 cols)",
			isTerm:     true,
			termFdW:    80,
			wantFormat: "unified",
		},
		{
			name:       "auto format on 119 cols",
			isTerm:     true,
			termFdW:    119,
			wantFormat: "unified",
		},
		{
			name:       "auto format on wide terminal (>=120 cols)",
			isTerm:     true,
			termFdW:    120,
			wantFormat: "side-by-side",
		},
		{
			name:       "auto format on 160 cols",
			isTerm:     true,
			termFdW:    160,
			wantFormat: "side-by-side",
		},
		{
			name: "explicit --format=auto on narrow terminal",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "auto")
			},
			isTerm:     true,
			termFdW:    100,
			wantFormat: "unified",
		},
		{
			name: "explicit --format=auto on wide terminal",
			setupCmd: func(c *cobra.Command) {
				_ = c.Flags().Set("format", "auto")
			},
			isTerm:     true,
			termFdW:    140,
			wantFormat: "side-by-side",
		},
		{
			name: "COLUMNS fallback when fd width is zero",
			env: map[string]string{
				"COLUMNS": "90",
			},
			wantFormat: "unified",
		},
		{
			name: "COLUMNS fallback wide",
			env: map[string]string{
				"COLUMNS": "130",
			},
			wantFormat: "side-by-side",
		},
		{
			name:       "interactive terminal with unknown width defaults to unified",
			isTerm:     true,
			termFdW:    0,
			wantFormat: "unified",
		},
		{
			name:       "non-interactive pipe with unknown width defaults to side-by-side",
			isTerm:     false,
			termFdW:    0,
			wantFormat: "side-by-side",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DIFFM_FORMAT", "")
			t.Setenv("COLUMNS", "")
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			termFdWidthFn = func() int { return tt.termFdW }
			isTerminalFn = func() bool { return tt.isTerm }

			cmd := newTestCmd()
			if tt.setupCmd != nil {
				tt.setupCmd(cmd)
			}

			got := resolveFormat(cmd)
			if got != tt.wantFormat {
				t.Errorf("resolveFormat() = %q, want %q", got, tt.wantFormat)
			}
		})
	}
}
