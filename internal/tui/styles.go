package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorSurface0 = lipgloss.Color("#313244")
	colorSurface1 = lipgloss.Color("#45475a")
	colorOverlay0 = lipgloss.Color("#6c7086")
	colorText     = lipgloss.Color("#cdd6f4")
	colorLavender = lipgloss.Color("#b4befe")
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
)

var (
	bgDeleteTint = lipgloss.Color("#5f242a")
	bgInsertTint = lipgloss.Color("#245f32")
	bgUpdateTint = lipgloss.Color("#5f5224")
	bgMoveTint   = lipgloss.Color("#244b5f")
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
