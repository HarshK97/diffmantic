package tui

import "github.com/charmbracelet/lipgloss"

var (
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

	contentStyle = lipgloss.NewStyle().
			Foreground(colorText)

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

func hlStyle(kind actionKind) lipgloss.Style {
	switch kind {
	case kindDelete:
		return hlDeleteStyle
	case kindInsert:
		return hlInsertStyle
	case kindUpdate:
		return hlUpdateStyle
	case kindMove:
		return hlMoveStyle
	default:
		return contentStyle
	}
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
	switch kind {
	case kindDelete:
		return colorRed
	case kindInsert:
		return colorGreen
	case kindUpdate:
		return colorYellow
	case kindMove:
		return colorBlue
	default:
		return colorText
	}
}

// actionIcon returns a unicode icon for an action kind.
func actionIcon(kind actionKind) string {
	switch kind {
	case kindDelete:
		return "✘"
	case kindInsert:
		return "✚"
	case kindUpdate:
		return "✎"
	case kindMove:
		return "➤"
	default:
		return "•"
	}
}
