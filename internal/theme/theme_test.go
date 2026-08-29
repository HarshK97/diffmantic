package theme

import (
	"testing"

	"github.com/HarshK97/diffmantic/internal/config"
)

func TestResolveTheme(t *testing.T) {
	tests := []struct {
		input       string
		wantName    string
		wantDark    bool
		expectError bool
	}{
		{input: "", wantName: "mocha", wantDark: true, expectError: false},
		{input: "mocha", wantName: "mocha", wantDark: true, expectError: false},
		{input: "dark", wantName: "mocha", wantDark: true, expectError: false},
		{input: "catppuccin-mocha", wantName: "mocha", wantDark: true, expectError: false},
		{input: "MOCHA", wantName: "mocha", wantDark: true, expectError: false},
		{input: "  Dark  ", wantName: "mocha", wantDark: true, expectError: false},
		{input: "latte", wantName: "latte", wantDark: false, expectError: false},
		{input: "light", wantName: "latte", wantDark: false, expectError: false},
		{input: "catppuccin-latte", wantName: "latte", wantDark: false, expectError: false},
		{input: "LATTE", wantName: "latte", wantDark: false, expectError: false},
		{input: "  Light  ", wantName: "latte", wantDark: false, expectError: false},
		{input: "solarized", expectError: true},
		{input: "invalid_theme", expectError: true},
	}

	for _, tt := range tests {
		t.Run("input="+tt.input, func(t *testing.T) {
			th, err := ResolveThemeWithConfig(tt.input, "dark", nil)
			if tt.expectError {
				if err == nil {
					t.Errorf("ResolveThemeWithConfig(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveThemeWithConfig(%q) unexpected error: %v", tt.input, err)
			}
			if th.Name != tt.wantName {
				t.Errorf("ResolveThemeWithConfig(%q).Name = %q, want %q", tt.input, th.Name, tt.wantName)
			}
			if th.IsDark != tt.wantDark {
				t.Errorf("ResolveThemeWithConfig(%q).IsDark = %v, want %v", tt.input, th.IsDark, tt.wantDark)
			}
		})
	}
}

func TestThemeColorSeparation(t *testing.T) {
	mocha := CatppuccinMochaTheme()
	latte := CatppuccinLatteTheme()

	if mocha.Actions.DeleteFg == "" || mocha.Actions.InsertFg == "" || mocha.Actions.UpdateFg == "" || mocha.Actions.MoveFg == "" {
		t.Errorf("Mocha action colors must not be empty")
	}
	if mocha.Actions.DeleteBg == "" || mocha.Actions.InsertBg == "" || mocha.Actions.UpdateBg == "" || mocha.Actions.MoveBg == "" {
		t.Errorf("Mocha action background tints must not be empty")
	}

	if latte.Actions.DeleteFg == "" || latte.Actions.InsertFg == "" || latte.Actions.UpdateFg == "" || latte.Actions.MoveFg == "" {
		t.Errorf("Latte action colors must not be empty")
	}
	if latte.Actions.DeleteBg == "" || latte.Actions.InsertBg == "" || latte.Actions.UpdateBg == "" || latte.Actions.MoveBg == "" {
		t.Errorf("Latte action background tints must not be empty")
	}

	// Mocha and Latte need distinct base and text colors so panes don't look inverted.
	if mocha.UI.Base == latte.UI.Base {
		t.Errorf("Mocha and Latte Base colors should differ: %v vs %v", mocha.UI.Base, latte.UI.Base)
	}
	if mocha.UI.Text == latte.UI.Text {
		t.Errorf("Mocha and Latte Text colors should differ: %v vs %v", mocha.UI.Text, latte.UI.Text)
	}

	// Syntax capture colors should differ between dark and light palettes.
	mochaKeyword := mocha.CaptureColor("keyword")
	latteKeyword := latte.CaptureColor("keyword")
	if mochaKeyword == "" || latteKeyword == "" {
		t.Errorf("keyword color should not be empty")
	}
	if mochaKeyword == latteKeyword {
		t.Errorf("Mocha and Latte keyword colors should differ: %v vs %v", mochaKeyword, latteKeyword)
	}

	// Nested captures like function.method fall back to their parent category.
	mochaMethod := mocha.CaptureColor("function.method")
	if mochaMethod != mocha.UI.Blue {
		t.Errorf("CaptureColor(\"function.method\") = %v, want %v", mochaMethod, mocha.UI.Blue)
	}
}

func TestThemeActionHelpers(t *testing.T) {
	mocha := CatppuccinMochaTheme()
	latte := CatppuccinLatteTheme()

	kinds := []ActionKind{ActionDelete, ActionInsert, ActionUpdate, ActionMove, ActionMoveUpdate}
	for _, k := range kinds {
		iconMocha := mocha.ActionIcon(k)
		iconLatte := latte.ActionIcon(k)
		if iconMocha == "" || iconLatte == "" {
			t.Errorf("ActionIcon(%v) must not be empty", k)
		}
		if iconMocha != iconLatte {
			t.Errorf("ActionIcon(%v) should match across themes: %q vs %q", k, iconMocha, iconLatte)
		}

		fgMocha := mocha.ActionFg(k)
		fgLatte := latte.ActionFg(k)
		if fgMocha == "" || fgLatte == "" {
			t.Errorf("ActionFg(%v) must not be empty", k)
		}
	}
}

func TestCardBorderBackgrounds(t *testing.T) {
	themes := []*Theme{CatppuccinMochaTheme(), CatppuccinLatteTheme()}
	for _, th := range themes {
		t.Run(th.Name, func(t *testing.T) {
			hoverBorderBg := th.Styles.HoverCard.GetBorderTopBackground()
			if hoverBorderBg != th.UI.Surface0 {
				t.Errorf("HoverCard border background = %v, want %v (Surface0)", hoverBorderBg, th.UI.Surface0)
			}
			helpBorderBg := th.Styles.HelpCard.GetBorderTopBackground()
			if helpBorderBg != th.UI.Base {
				t.Errorf("HelpCard border background = %v, want %v (Base)", helpBorderBg, th.UI.Base)
			}
		})
	}
}

func TestResolveThemeWithConfig(t *testing.T) {
	customThemes := map[string]config.ThemeConfig{
		"my-nord": {
			Name: "my-nord",
			Dark: new(true),
			UI: config.UIColorsConfig{
				Base: "#2e3440",
				Text: "#eceff4",
			},
			Actions: config.ActionColorsConfig{
				InsertFg: "#a3be8c",
				DeleteFg: "#bf616a",
			},
		},
		"solarized-light": {
			Name: "solarized-light",
			Dark: new(false),
			UI: config.UIColorsConfig{
				Base: "#fdf6e3",
				Text: "#657b83",
			},
		},
	}

	// Style preference fallback when no theme name is given
	th, err := ResolveThemeWithConfig("", "light", customThemes)
	if err != nil {
		t.Fatalf("unexpected error resolving empty theme with light style: %v", err)
	}
	if th.Name != "latte" || th.IsDark {
		t.Errorf("expected latte light theme, got name=%q dark=%v", th.Name, th.IsDark)
	}

	// Default to dark mocha when style="dark"
	th, err = ResolveThemeWithConfig("", "dark", customThemes)
	if err != nil {
		t.Fatalf("unexpected error resolving empty theme with dark style: %v", err)
	}
	if th.Name != "mocha" || !th.IsDark {
		t.Errorf("expected mocha dark theme, got name=%q dark=%v", th.Name, th.IsDark)
	}

	// Custom theme with partial color overrides
	th, err = ResolveThemeWithConfig("my-nord", "dark", customThemes)
	if err != nil {
		t.Fatalf("unexpected error resolving custom theme 'my-nord': %v", err)
	}
	if th.Name != "my-nord" || !th.IsDark {
		t.Errorf("expected my-nord dark theme, got name=%q dark=%v", th.Name, th.IsDark)
	}
	if th.UI.Base != "#2e3440" {
		t.Errorf("th.UI.Base = %v, want '#2e3440'", th.UI.Base)
	}
	if th.Actions.InsertFg != "#a3be8c" {
		t.Errorf("th.Actions.InsertFg = %v, want '#a3be8c'", th.Actions.InsertFg)
	}
	// Lavender wasn't overridden, so it should inherit from mocha
	if th.UI.Lavender != CatppuccinMochaTheme().UI.Lavender {
		t.Errorf("th.UI.Lavender = %v, want inherited %v", th.UI.Lavender, CatppuccinMochaTheme().UI.Lavender)
	}

	// Light custom theme
	th, err = ResolveThemeWithConfig("solarized-light", "dark", customThemes)
	if err != nil {
		t.Fatalf("unexpected error resolving 'solarized-light': %v", err)
	}
	if th.IsDark {
		t.Errorf("expected light theme for solarized-light")
	}
	if th.UI.Base != "#fdf6e3" {
		t.Errorf("th.UI.Base = %v, want '#fdf6e3'", th.UI.Base)
	}

	// Unregistered theme returns an error
	_, err = ResolveThemeWithConfig("nonexistent", "dark", customThemes)
	if err == nil {
		t.Error("expected error for nonexistent theme, got nil")
	}
}
