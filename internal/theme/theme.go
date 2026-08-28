// Package theme provides color palettes, precomputed lipgloss styles,
// and Tree-sitter syntax capture mappings for Diffmantic themes.
package theme

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ActionKind dictates how a diff action or line is styled.
type ActionKind int

const (
	// ActionDelete styles deleted content or lines.
	ActionDelete ActionKind = iota
	// ActionInsert styles inserted content or lines.
	ActionInsert
	// ActionUpdate styles updated/modified content.
	ActionUpdate
	// ActionMove styles moved AST nodes.
	ActionMove
	// ActionMoveUpdate styles moved and simultaneously modified content.
	ActionMoveUpdate
)

// UIColors contains the standard UI chrome palette.
type UIColors struct {
	Base      lipgloss.Color
	Mantle    lipgloss.Color
	Crust     lipgloss.Color
	Surface0  lipgloss.Color
	Surface1  lipgloss.Color
	Surface2  lipgloss.Color
	Overlay0  lipgloss.Color
	Overlay1  lipgloss.Color
	Overlay2  lipgloss.Color
	Subtext0  lipgloss.Color
	Subtext1  lipgloss.Color
	Text      lipgloss.Color
	Lavender  lipgloss.Color
	Mauve     lipgloss.Color
	Blue      lipgloss.Color
	Green     lipgloss.Color
	Peach     lipgloss.Color
	Sky       lipgloss.Color
	Yellow    lipgloss.Color
	Pink      lipgloss.Color
	Red       lipgloss.Color
	Teal      lipgloss.Color
	Rosewater lipgloss.Color
	Sapphire  lipgloss.Color
	Maroon    lipgloss.Color
	Flamingo  lipgloss.Color
}

// ActionColors contains the dedicated diff action foregrounds and background tints.
type ActionColors struct {
	DeleteFg     lipgloss.Color
	InsertFg     lipgloss.Color
	UpdateFg     lipgloss.Color
	MoveFg       lipgloss.Color
	MoveUpdateFg lipgloss.Color

	DeleteBg     lipgloss.Color
	InsertBg     lipgloss.Color
	UpdateBg     lipgloss.Color
	MoveBg       lipgloss.Color
	MoveUpdateBg lipgloss.Color
}

// Styles contains precomputed lipgloss styles for efficient rendering.
type Styles struct {
	Title          lipgloss.Style
	Status         lipgloss.Style
	LineNum        lipgloss.Style
	GutterFold     lipgloss.Style
	Content        lipgloss.Style
	SearchHl       lipgloss.Style
	SearchActiveHl lipgloss.Style
	Divider        lipgloss.Style
	Fold           lipgloss.Style
	CursorGutter   lipgloss.Style
	CursorContent  lipgloss.Style
	CursorFold     lipgloss.Style

	HlDelete     lipgloss.Style
	HlInsert     lipgloss.Style
	HlUpdate     lipgloss.Style
	HlMove       lipgloss.Style
	HlMoveUpdate lipgloss.Style

	HoverCard   lipgloss.Style
	HoverDetail lipgloss.Style
	HoverDim    lipgloss.Style

	GitHeader   lipgloss.Style
	GitCursor   lipgloss.Style
	GitNormal   lipgloss.Style
	CommitPanel lipgloss.Style

	HelpCard          lipgloss.Style
	HelpTitle         lipgloss.Style
	HelpSectionHeader lipgloss.Style
	HelpFooter        lipgloss.Style
}

// Theme encapsulates UI palette, Action palette, Tree-sitter capture map, and UI styles.
type Theme struct {
	Name        string
	IsDark      bool
	UI          UIColors
	Actions     ActionColors
	Styles      Styles
	captureMap  map[string]lipgloss.Color
	actionFgs   [5]lipgloss.Color
	actionIcons [5]string
	hlStyles    [5]lipgloss.Style
}

func newTheme(name string, isDark bool, ui UIColors, actions ActionColors) *Theme {
	hlDelete := lipgloss.NewStyle().Background(actions.DeleteBg).Foreground(ui.Text)
	hlInsert := lipgloss.NewStyle().Background(actions.InsertBg).Foreground(ui.Text)
	hlUpdate := lipgloss.NewStyle().Background(actions.UpdateBg).Foreground(ui.Text)
	hlMove := lipgloss.NewStyle().Background(actions.MoveBg).Foreground(ui.Text)
	hlMoveUpdate := lipgloss.NewStyle().Background(actions.MoveUpdateBg).Foreground(actions.UpdateFg).Underline(true)

	styles := Styles{
		Title: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Lavender).
			Background(ui.Surface0),

		Status: lipgloss.NewStyle().
			Foreground(ui.Overlay0).
			Background(ui.Surface0),

		LineNum: lipgloss.NewStyle().
			Foreground(ui.Overlay0).
			Background(ui.Base),

		GutterFold: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Lavender).
			Background(ui.Base),

		Content: lipgloss.NewStyle().
			Foreground(ui.Text).
			Background(ui.Base),

		SearchHl: lipgloss.NewStyle().
			Background(ui.Peach).
			Foreground(ui.Crust),

		SearchActiveHl: lipgloss.NewStyle().
			Background(ui.Green).
			Foreground(ui.Crust).
			Bold(true),

		Divider: lipgloss.NewStyle().
			Foreground(ui.Surface1).
			Background(ui.Base),

		Fold: lipgloss.NewStyle().
			Foreground(ui.Overlay0).
			Background(ui.Surface0).
			Italic(true),

		CursorGutter: lipgloss.NewStyle().
			Foreground(ui.Lavender).
			Background(ui.Surface1).
			Blink(true),

		CursorContent: lipgloss.NewStyle().
			Foreground(ui.Text).
			Background(ui.Surface0),

		CursorFold: lipgloss.NewStyle().
			Foreground(ui.Text).
			Background(ui.Surface1).
			Italic(true).
			Blink(true),

		HlDelete:     hlDelete,
		HlInsert:     hlInsert,
		HlUpdate:     hlUpdate,
		HlMove:       hlMove,
		HlMoveUpdate: hlMoveUpdate,

		HoverCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.Lavender).
			BorderBackground(ui.Surface0).
			Background(ui.Surface0).
			Padding(0, 1),

		HoverDetail: lipgloss.NewStyle().
			Foreground(ui.Subtext0).
			Background(ui.Surface0),

		HoverDim: lipgloss.NewStyle().
			Foreground(ui.Overlay0).
			Background(ui.Surface0),

		GitHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Mauve).
			Background(ui.Base),

		GitCursor: lipgloss.NewStyle().
			Background(ui.Surface1).
			Foreground(ui.Text),

		GitNormal: lipgloss.NewStyle().
			Background(ui.Base).
			Foreground(ui.Overlay0),

		CommitPanel: lipgloss.NewStyle().
			Background(ui.Surface0).
			Foreground(ui.Text),

		HelpCard: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ui.Lavender).
			BorderBackground(ui.Base).
			Background(ui.Base).
			Padding(1, 1, 1, 1),

		HelpTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Lavender),

		HelpSectionHeader: lipgloss.NewStyle().
			Bold(true).
			Foreground(ui.Subtext0),

		HelpFooter: lipgloss.NewStyle().
			Foreground(ui.Overlay0),
	}

	captures := map[string]lipgloss.Color{
		// Keywords
		"keyword":     ui.Mauve,
		"conditional": ui.Mauve,
		"repeat":      ui.Mauve,
		"include":     ui.Mauve,
		"exception":   ui.Mauve,

		// Functions
		"function": ui.Blue,
		"method":   ui.Blue,

		// Strings and literals
		"string":  ui.Green,
		"escape":  ui.Pink,
		"number":  ui.Peach,
		"boolean": ui.Peach,
		"float":   ui.Peach,

		// Types
		"type":        ui.Yellow,
		"constructor": ui.Yellow,

		// Constants
		"constant": ui.Peach,

		// Variables and properties
		"variable":  ui.Text,
		"parameter": ui.Maroon,
		"property":  ui.Lavender,
		"field":     ui.Lavender,
		"attribute": ui.Lavender,

		// Operators and punctuation
		"operator":    ui.Sky,
		"punctuation": ui.Overlay0,

		// Comments
		"comment": ui.Overlay0,

		// Tags (HTML/XML)
		"tag":      ui.Mauve,
		"embedded": ui.Red,

		// Other
		"label":     ui.Teal,
		"namespace": ui.Rosewater,
		"error":     ui.Red,
	}

	return &Theme{
		Name:       name,
		IsDark:     isDark,
		UI:         ui,
		Actions:    actions,
		Styles:     styles,
		captureMap: captures,
		actionFgs: [5]lipgloss.Color{
			actions.DeleteFg,
			actions.InsertFg,
			actions.UpdateFg,
			actions.MoveFg,
			actions.MoveUpdateFg,
		},
		actionIcons: [5]string{"✘", "✚", "✎", "➤", "✎"},
		hlStyles: [5]lipgloss.Style{
			hlDelete,
			hlInsert,
			hlUpdate,
			hlMove,
			hlMoveUpdate,
		},
	}
}

// ActionFg returns the foreground color for an action kind.
func (t *Theme) ActionFg(kind ActionKind) lipgloss.Color {
	th := cmp.Or(t, CatppuccinMochaTheme())
	if uint(kind) < uint(len(th.actionFgs)) {
		return th.actionFgs[kind]
	}
	return th.UI.Text
}

// ActionIcon returns the icon for an action kind.
func (t *Theme) ActionIcon(kind ActionKind) string {
	th := cmp.Or(t, CatppuccinMochaTheme())
	if uint(kind) < uint(len(th.actionIcons)) {
		return th.actionIcons[kind]
	}
	return "•"
}

// HlStyle returns the highlight style for an action kind.
func (t *Theme) HlStyle(kind ActionKind) lipgloss.Style {
	th := cmp.Or(t, CatppuccinMochaTheme())
	if uint(kind) < uint(len(th.hlStyles)) {
		return th.hlStyles[kind]
	}
	return th.Styles.Content
}

// CaptureColor maps Tree-sitter capture names to theme foreground colors.
// It falls back to parent categories (e.g. "function.method" matches "function").
func (t *Theme) CaptureColor(capture string) lipgloss.Color {
	th := cmp.Or(t, CatppuccinMochaTheme())
	if c, ok := th.captureMap[capture]; ok {
		return c
	}

	for {
		idx := strings.LastIndex(capture, ".")
		if idx < 0 {
			break
		}
		capture = capture[:idx]
		if c, ok := th.captureMap[capture]; ok {
			return c
		}
	}

	return ""
}

// ResolveTheme resolves a theme name string (case-insensitive) to a Theme instance.
// Supported names: "mocha", "latte", "dark", "light", "catppuccin-mocha", "catppuccin-latte".
func ResolveTheme(name string) (*Theme, error) {
	norm := strings.ToLower(strings.TrimSpace(name))
	switch norm {
	case "", "mocha", "dark", "catppuccin-mocha":
		return CatppuccinMochaTheme(), nil
	case "latte", "light", "catppuccin-latte":
		return CatppuccinLatteTheme(), nil
	default:
		return nil, fmt.Errorf("unsupported theme %q: supported themes are mocha (dark), latte (light)", name)
	}
}
