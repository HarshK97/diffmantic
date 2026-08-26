package theme

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

var latteThemeOnce = sync.OnceValue(func() *Theme {
	ui := UIColors{
		Base:      lipgloss.Color("#eff1f5"),
		Mantle:    lipgloss.Color("#e6e9ef"),
		Crust:     lipgloss.Color("#dce0e8"),
		Surface0:  lipgloss.Color("#ccd0da"),
		Surface1:  lipgloss.Color("#bcc0cc"),
		Surface2:  lipgloss.Color("#acb0be"),
		Overlay0:  lipgloss.Color("#9ca0b0"),
		Overlay1:  lipgloss.Color("#8c8fa1"),
		Overlay2:  lipgloss.Color("#7c7f93"),
		Subtext0:  lipgloss.Color("#6c6f85"),
		Subtext1:  lipgloss.Color("#5c5f77"),
		Text:      lipgloss.Color("#4c4f69"),
		Lavender:  lipgloss.Color("#7287fd"),
		Mauve:     lipgloss.Color("#8839ef"),
		Blue:      lipgloss.Color("#1e66f5"),
		Green:     lipgloss.Color("#40a02b"),
		Peach:     lipgloss.Color("#fe640b"),
		Sky:       lipgloss.Color("#04a5e5"),
		Yellow:    lipgloss.Color("#df8e1d"),
		Pink:      lipgloss.Color("#ea76cb"),
		Red:       lipgloss.Color("#d20f39"),
		Teal:      lipgloss.Color("#179299"),
		Rosewater: lipgloss.Color("#dc8a78"),
		Sapphire:  lipgloss.Color("#209fb5"),
		Maroon:    lipgloss.Color("#e64553"),
		Flamingo:  lipgloss.Color("#dd7878"),
	}

	actions := ActionColors{
		DeleteFg:     lipgloss.Color("#d20f39"),
		InsertFg:     lipgloss.Color("#40a02b"),
		UpdateFg:     lipgloss.Color("#df8e1d"),
		MoveFg:       lipgloss.Color("#1e66f5"),
		MoveUpdateFg: lipgloss.Color("#179299"),

		DeleteBg:     lipgloss.Color("#ffdce0"),
		InsertBg:     lipgloss.Color("#dcf5dc"),
		UpdateBg:     lipgloss.Color("#fff2cc"),
		MoveBg:       lipgloss.Color("#dceeff"),
		MoveUpdateBg: lipgloss.Color("#dceeff"),
	}

	return newTheme("latte", false, ui, actions)
})

// CatppuccinLatteTheme returns the light Catppuccin Latte theme.
func CatppuccinLatteTheme() *Theme {
	return latteThemeOnce()
}
