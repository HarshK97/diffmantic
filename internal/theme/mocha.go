package theme

import (
	"sync"

	"github.com/charmbracelet/lipgloss"
)

var mochaThemeOnce = sync.OnceValue(func() *Theme {
	ui := UIColors{
		Base:      lipgloss.Color("#1e1e2e"),
		Mantle:    lipgloss.Color("#181825"),
		Crust:     lipgloss.Color("#11111b"),
		Surface0:  lipgloss.Color("#313244"),
		Surface1:  lipgloss.Color("#45475a"),
		Surface2:  lipgloss.Color("#585b70"),
		Overlay0:  lipgloss.Color("#6c7086"),
		Overlay1:  lipgloss.Color("#7f849f"),
		Overlay2:  lipgloss.Color("#9399b2"),
		Subtext0:  lipgloss.Color("#a6adc8"),
		Subtext1:  lipgloss.Color("#bac2de"),
		Text:      lipgloss.Color("#cdd6f4"),
		Lavender:  lipgloss.Color("#b4befe"),
		Mauve:     lipgloss.Color("#cba6f7"),
		Blue:      lipgloss.Color("#89b4fa"),
		Green:     lipgloss.Color("#a6e3a1"),
		Peach:     lipgloss.Color("#fab387"),
		Sky:       lipgloss.Color("#89dceb"),
		Yellow:    lipgloss.Color("#f9e2af"),
		Pink:      lipgloss.Color("#f5c2e7"),
		Red:       lipgloss.Color("#f38ba8"),
		Teal:      lipgloss.Color("#94e2d5"),
		Rosewater: lipgloss.Color("#f5e0dc"),
		Sapphire:  lipgloss.Color("#74c7ec"),
		Maroon:    lipgloss.Color("#eba0ac"),
		Flamingo:  lipgloss.Color("#f2cdcd"),
	}

	actions := ActionColors{
		DeleteFg:     lipgloss.Color("#f38ba8"),
		InsertFg:     lipgloss.Color("#a6e3a1"),
		UpdateFg:     lipgloss.Color("#f9e2af"),
		MoveFg:       lipgloss.Color("#89b4fa"),
		MoveUpdateFg: lipgloss.Color("#94e2d5"),

		DeleteBg:     lipgloss.Color("#5f242a"),
		InsertBg:     lipgloss.Color("#245f32"),
		UpdateBg:     lipgloss.Color("#5f5224"),
		MoveBg:       lipgloss.Color("#1e4a70"),
		MoveUpdateBg: lipgloss.Color("#1e4a70"),
	}

	return newTheme("mocha", true, ui, actions)
})

// CatppuccinMochaTheme returns the dark Catppuccin Mocha theme.
func CatppuccinMochaTheme() *Theme {
	return mochaThemeOnce()
}
