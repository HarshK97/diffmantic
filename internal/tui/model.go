package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/HarshK97/diffmantic/internal/serialize"
)

// Run launches the side-by-side terminal diff viewer.
func Run(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope) error {
	m := newModel(srcFile, dstFile, srcBytes, dstBytes, env)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func newModel(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope) model {
	m := model{
		srcFile:  srcFile,
		dstFile:  dstFile,
		srcLines: strings.Split(string(srcBytes), "\n"),
		dstLines: strings.Split(string(dstBytes), "\n"),
	}

	if env != nil && len(env.Actions) > 0 {
		m.srcHighlights, m.dstHighlights = buildHighlights(srcBytes, dstBytes, env.Actions)
		m.allChanges = mergedChangeLines(m.srcHighlights, m.dstHighlights)
	} else {
		m.srcHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
		m.dstHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
	}

	return m
}

func (m model) contentHeight() int {
	return m.height - titleBarHeight - statusBarHeight
}

func (m model) paneWidth() int {
	return (m.width - dividerWidth) / 2
}

func (m model) gutterWidth() int {
	maxLines := len(m.srcLines)
	if len(m.dstLines) > maxLines {
		maxLines = len(m.dstLines)
	}
	w := len(fmt.Sprintf("%d", maxLines))
	if w < 3 {
		w = 3
	}
	return w + gutterPadding
}

func (m model) textWidth() int {
	return m.paneWidth() - m.gutterWidth()
}

func (m model) maxScrollY() int {
	maxLines := len(m.srcLines)
	if len(m.dstLines) > maxLines {
		maxLines = len(m.dstLines)
	}
	max := maxLines - m.contentHeight()
	if max < 0 {
		return 0
	}
	return max
}

func maxScrollX(lines []string, textWidth int) int {
	maxLen := 0
	for _, l := range lines {
		// Expand tabs so they don't mess up horizontal scroll limits.
		expanded := strings.ReplaceAll(l, "\t", "    ")
		if len([]rune(expanded)) > maxLen {
			maxLen = len([]rune(expanded))
		}
	}
	max := maxLen - textWidth
	if max < 0 {
		return 0
	}
	return max
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
