package tui

import "charm.land/lipgloss/v2"

type styles struct {
	Chrome        lipgloss.Style
	Tree          lipgloss.Style
	TreeFocused   lipgloss.Style
	TreeSelected  lipgloss.Style
	TreeMuted     lipgloss.Style
	Header        lipgloss.Style
	Status        lipgloss.Style
	HelpKey       lipgloss.Style
	Help          lipgloss.Style
	LineNumber    lipgloss.Style
	Context       lipgloss.Style
	Separator     lipgloss.Style
	Insert        lipgloss.Style
	Delete        lipgloss.Style
	Update        lipgloss.Style
	Move          lipgloss.Style
	InsertFill    lipgloss.Style
	DeleteFill    lipgloss.Style
	UpdateFill    lipgloss.Style
	MoveFill      lipgloss.Style
	SelectedFocus lipgloss.Style
}

func newStyles() styles {
	return styles{
		Chrome: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),
		Tree: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")),
		TreeFocused: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true),
		TreeSelected: lipgloss.NewStyle().
			Background(lipgloss.Color("#1F2937")).
			Foreground(lipgloss.Color("#F9FAFB")).
			Bold(true),
		TreeMuted: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),
		Header: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#111827")).
			Bold(true).
			Padding(0, 1),
		Status: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#D1D5DB")).
			Background(lipgloss.Color("#111827")),
		HelpKey: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#93C5FD")).
			Bold(true),
		Help: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")),
		LineNumber: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#6B7280")),
		Context: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E7EB")),
		Separator: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4B5563")),
		Insert: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DCFCE7")).
			Background(lipgloss.Color("#166534")).
			Bold(true),
		Delete: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FEE2E2")).
			Background(lipgloss.Color("#7F1D1D")).
			Bold(true),
		Update: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FEF3C7")).
			Background(lipgloss.Color("#92400E")).
			Bold(true),
		Move: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#DBEAFE")).
			Background(lipgloss.Color("#1D4ED8")).
			Bold(true),
		InsertFill: lipgloss.NewStyle().
			Background(lipgloss.Color("#052E16")),
		DeleteFill: lipgloss.NewStyle().
			Background(lipgloss.Color("#450A0A")),
		UpdateFill: lipgloss.NewStyle().
			Background(lipgloss.Color("#422006")),
		MoveFill: lipgloss.NewStyle().
			Background(lipgloss.Color("#172554")),
		SelectedFocus: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F9FAFB")).
			Background(lipgloss.Color("#1D4ED8")).
			Bold(true),
	}
}
