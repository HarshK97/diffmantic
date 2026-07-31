package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorBase      = lipgloss.Color("#1e1e2e")
	colorSurface0  = lipgloss.Color("#313244")
	colorSurface1  = lipgloss.Color("#45475a")
	colorOverlay0  = lipgloss.Color("#6c7086")
	colorSubtext0  = lipgloss.Color("#a6adc8")
	colorText      = lipgloss.Color("#cdd6f4")
	colorBlue      = lipgloss.Color("#89b4fa")
	colorLavender  = lipgloss.Color("#b4befe")
	colorMauve     = lipgloss.Color("#cba6f7")
	colorGreen     = lipgloss.Color("#a6e3a1")
	colorPeach     = lipgloss.Color("#fab387")
	colorSky       = lipgloss.Color("#89dceb")
	colorYellow    = lipgloss.Color("#f9e2af")
	colorPink      = lipgloss.Color("#f5c2e7")
	colorRed       = lipgloss.Color("#f38ba8")
	colorTeal      = lipgloss.Color("#94e2d5")
	colorRosewater = lipgloss.Color("#f5e0dc")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorLavender).
			Background(colorSurface0)

	statusStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0).
			Background(colorSurface0)

	lineNumStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0)

	gutterFoldStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colorLavender)

	contentStyle = lipgloss.NewStyle().
			Foreground(colorText)

	searchHlStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("#fab387")).
			Foreground(lipgloss.Color("#11111b"))

	searchActiveHlStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#a6e3a1")).
				Foreground(lipgloss.Color("#11111b")).
				Bold(true)

	dividerStyle = lipgloss.NewStyle().
			Foreground(colorSurface1)

	foldStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0).
			Background(colorSurface0).
			Italic(true)

	cursorGutterStyle = lipgloss.NewStyle().
				Foreground(colorLavender).
				Background(colorSurface1).
				Blink(true)

	cursorContentStyle = lipgloss.NewStyle().
				Background(colorSurface0)

	cursorFoldStyle = lipgloss.NewStyle().
			Foreground(colorText).
			Background(colorSurface1).
			Italic(true).
			Blink(true)
)

var (
	bgDeleteTint = lipgloss.Color("#5f242a")
	bgInsertTint = lipgloss.Color("#245f32")
	bgUpdateTint = lipgloss.Color("#5f5224")
	bgMoveTint   = lipgloss.Color("#1e4a70")
)

var (
	hlDeleteStyle = lipgloss.NewStyle().Background(bgDeleteTint)
	hlInsertStyle = lipgloss.NewStyle().Background(bgInsertTint)
	hlUpdateStyle = lipgloss.NewStyle().Background(bgUpdateTint)
	hlMoveStyle   = lipgloss.NewStyle().Background(bgMoveTint)
)

var (
	hlStyles    = [...]lipgloss.Style{hlDeleteStyle, hlInsertStyle, hlUpdateStyle, hlMoveStyle}
	actionFgs   = [...]lipgloss.Color{colorRed, colorGreen, colorYellow, colorBlue}
	actionIcons = [...]string{"✘", "✚", "✎", "➤"}
)

func hlStyle(kind actionKind) lipgloss.Style {
	if uint(kind) < uint(len(hlStyles)) {
		return hlStyles[kind]
	}
	return contentStyle
}

// Inspect panel styles.
var (
	inspectDetailStyle = lipgloss.NewStyle().
				Foreground(colorSubtext0)

	inspectDimStyle = lipgloss.NewStyle().
			Foreground(colorOverlay0)

	inspectPanelStyle = lipgloss.NewStyle().
				Background(colorSurface0)
)

// actionFg returns the foreground color for an action kind.
func actionFg(kind actionKind) lipgloss.Color {
	if uint(kind) < uint(len(actionFgs)) {
		return actionFgs[kind]
	}
	return colorText
}

// actionIcon returns a unicode icon for an action kind.
func actionIcon(kind actionKind) string {
	if uint(kind) < uint(len(actionIcons)) {
		return actionIcons[kind]
	}
	return "•"
}
