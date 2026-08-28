package tui

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Draw the centered help card showing keybindings, mouse controls, and color legend.
func (m model) renderHelpModal() string {
	width := m.width
	height := m.height
	if width <= 0 || height <= 0 {
		return ""
	}

	t := m.getTheme()

	cardWidth := 90
	if width < cardWidth {
		cardWidth = max(width-4, 20)
	}

	var b strings.Builder

	// Center title before applying styles so ANSI escape codes don't break padding calculation.
	titleText := "HELP & KEYBINDINGS"
	centeredTitle := centerPad(titleText, cardWidth)
	title := t.Styles.HelpTitle.Render(centeredTitle)

	colWidth := max((cardWidth-6)/2, 10)

	leftCol := renderLeftColumn(t)
	rightCol := renderRightColumn(t)

	leftLines := strings.Split(leftCol, "\n")
	rightLines := strings.Split(rightCol, "\n")

	maxLines := max(len(rightLines), len(leftLines))

	var colsBuilder strings.Builder
	for i := range maxLines {
		var l, r string
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}

		lw := lipgloss.Width(l)
		if lw < colWidth {
			l += strings.Repeat(" ", colWidth-lw)
		}
		rw := lipgloss.Width(r)
		if rw < colWidth {
			r += strings.Repeat(" ", colWidth-rw)
		}

		fmt.Fprintf(&colsBuilder, "  %s   %s\n", l, r)
	}

	sep := t.Styles.HoverDim.Render(strings.Repeat("─", cardWidth))

	legendTitle := t.Styles.HelpSectionHeader.Render("COLOR LEGEND")
	legend := fmt.Sprintf(
		"  %s   %s   %s   %s   %s",
		t.Styles.HlInsert.Render(" ✚ INSERT "),
		t.Styles.HlDelete.Render(" ✘ DELETE "),
		t.Styles.HlUpdate.Render(" ✎ UPDATE "),
		t.Styles.HlMove.Render(" ➤ MOVE "),
		t.Styles.HlMoveUpdate.Render(" ✎ MOVE+UPDATE "),
	)

	mouseTitle := t.Styles.HelpSectionHeader.Render("MOUSE CONTROLS")
	mouseHelp := []string{
		"  • Scroll Wheel : Scroll vertical view",
		"  • Shift+Scroll : Scroll horizontal view",
		"  • Left Click   : Select pane/row, toggle fold (on gutter)",
		"  • Hover Mouse  : Show token info popover",
	}

	b.WriteString("\n")
	b.WriteString(title + "\n\n")
	b.WriteString(colsBuilder.String() + "\n")
	b.WriteString("  " + sep + "\n\n")
	b.WriteString("  " + legendTitle + "\n")
	b.WriteString(legend + "\n\n")
	b.WriteString("  " + sep + "\n\n")
	b.WriteString("  " + mouseTitle + "\n")
	b.WriteString(strings.Join(mouseHelp, "\n") + "\n\n")
	b.WriteString("  " + sep + "\n\n")

	footerText := "Press Esc or ? to return to diff view"
	centeredFooter := centerPad(footerText, cardWidth)
	footer := t.Styles.HelpFooter.Render(centeredFooter)
	b.WriteString(footer + "\n")

	renderedCard := t.Styles.HelpCard.Width(cardWidth + 2).Render(b.String())

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		renderedCard,
		lipgloss.WithWhitespaceBackground(t.UI.Base),
	)
}

func renderLeftColumn(t *Theme) string {
	t = cmp.Or(t, defaultTheme)
	b := t.Styles.HelpSectionHeader.Render("NAVIGATION")
	lines := []string{
		b,
		"  j / k  : Scroll row up/down",
		"  h / l  : Scroll view left/right",
		"  ]h / [h: Jump next/prev span",
		"  w / b  : Move word forward/back",
		"  0 / $  : Go to start/end of line",
		"  n / N  : Next/prev change or match",
		"  Tab    : Switch active pane",
		"  /      : Search code",
		"  Enter  : Jump to destination (if moved)",
	}
	return strings.Join(lines, "\n")
}

func renderRightColumn(t *Theme) string {
	t = cmp.Or(t, defaultTheme)
	b := t.Styles.HelpSectionHeader.Render("ACTIONS & FOLDING")
	lines := []string{
		b,
		"  K      : Show hover info",
		"  za     : Toggle fold under cursor",
		"  zo / zc: Open / close fold",
		"  zR / zM: Open / close all folds",
		"  Enter  : Jump to move counterpart",
		"  ?      : Toggle help menu",
		"  q / Esc: Quit diffmantic",
	}
	return strings.Join(lines, "\n")
}
