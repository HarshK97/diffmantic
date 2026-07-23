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
		srcFile:    srcFile,
		dstFile:    dstFile,
		srcLines:   strings.Split(string(srcBytes), "\n"),
		dstLines:   strings.Split(string(dstBytes), "\n"),
		activePane: "left",
	}

	if env == nil || len(env.LineAlignment) == 0 {
		total := len(m.srcLines)
		if len(m.dstLines) > total {
			total = len(m.dstLines)
		}
		m.lineAlignment = make([]serialize.LineAlignmentPair, total)
		for i := 0; i < total; i++ {
			left := -1
			if i < len(m.srcLines) {
				left = i
			}
			right := -1
			if i < len(m.dstLines) {
				right = i
			}
			m.lineAlignment[i] = serialize.LineAlignmentPair{LeftLine: left, RightLine: right}
		}
	} else {
		m.lineAlignment = env.LineAlignment
	}

	if env != nil {
		m.srcHighlights, m.dstHighlights = buildHighlights(srcBytes, dstBytes, env.Actions)
		var changeRows []int
		for r, pair := range m.lineAlignment {
			isChange := false
			if pair.LeftLine == -1 || pair.RightLine == -1 {
				isChange = true
			} else {
				if _, ok := m.srcHighlights.tinted[pair.LeftLine]; ok {
					isChange = true
				}
				if _, ok := m.dstHighlights.tinted[pair.RightLine]; ok {
					isChange = true
				}
			}
			if isChange {
				changeRows = append(changeRows, r)
			}
		}
		m.allChanges = changeRows
	} else {
		m.srcHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
		m.dstHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
	}

	// Build collapsible folds from unchanged lines.
	m.folds = computeFolds(m.allChanges, len(m.lineAlignment), foldContext)
	m.rebuildVirtualLines()

	// Pre-compute syntax colors upfront so rendering stays fast on scroll.
	m.srcSyntax = highlightSyntax(srcFile, srcBytes)
	m.dstSyntax = highlightSyntax(dstFile, dstBytes)

	return m
}

// rebuildVirtualLines updates display mappings and virtual change indices after folding/unfolding.
func (m *model) rebuildVirtualLines() {
	m.virtualLines = buildVirtualLines(m.folds, len(m.lineAlignment), m.lineAlignment)

	// Map physical change lines to their display rows.
	m.vchanges = make([]int, 0, len(m.allChanges))
	for _, rl := range m.allChanges {
		vi := realToVirtual(m.virtualLines, m.folds, rl)
		if vi >= 0 {
			m.vchanges = append(m.vchanges, vi)
		}
	}
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
	max := len(m.virtualLines) - m.contentHeight()
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

func (m *model) clampCursor() {
	m.cursorY = clamp(m.cursorY, 0, len(m.virtualLines)-1)
	maxCol := m.lineVisualLength(m.cursorY) - 1
	if maxCol < 0 {
		maxCol = 0
	}
	m.cursorX = clamp(m.cursorX, 0, maxCol)
}

func (m model) lineVisualLength(vIdx int) int {
	if vIdx < 0 || vIdx >= len(m.virtualLines) {
		return 0
	}
	vl := m.virtualLines[vIdx]
	if vl.foldIdx >= 0 {
		return m.paneWidth()
	}

	var rawLine string
	if m.activePane == "left" {
		if vl.leftLine >= 0 && vl.leftLine < len(m.srcLines) {
			rawLine = m.srcLines[vl.leftLine]
		}
	} else {
		if vl.rightLine >= 0 && vl.rightLine < len(m.dstLines) {
			rawLine = m.dstLines[vl.rightLine]
		}
	}
	expanded := strings.ReplaceAll(rawLine, "\t", "    ")
	return len([]rune(expanded))
}

func (m model) lineVisualRunes(vIdx int) []rune {
	if vIdx < 0 || vIdx >= len(m.virtualLines) {
		return nil
	}
	vl := m.virtualLines[vIdx]
	if vl.foldIdx >= 0 {
		return nil
	}

	var rawLine string
	if m.activePane == "left" {
		if vl.leftLine >= 0 && vl.leftLine < len(m.srcLines) {
			rawLine = m.srcLines[vl.leftLine]
		}
	} else {
		if vl.rightLine >= 0 && vl.rightLine < len(m.dstLines) {
			rawLine = m.dstLines[vl.rightLine]
		}
	}
	expanded := strings.ReplaceAll(rawLine, "\t", "    ")
	return []rune(expanded)
}

func (m *model) keepCursorInViewport() {
	h := m.contentHeight()
	if h <= 0 {
		return
	}
	if m.cursorY < m.scrollY {
		m.scrollY = m.cursorY
	} else if m.cursorY >= m.scrollY+h {
		m.scrollY = m.cursorY - h + 1
	}
	m.scrollY = clamp(m.scrollY, 0, m.maxScrollY())

	tw := m.textWidth()
	if tw <= 0 {
		return
	}
	if m.activePane == "left" {
		if m.cursorX < m.scrollXLeft {
			m.scrollXLeft = m.cursorX
		} else if m.cursorX >= m.scrollXLeft+tw {
			m.scrollXLeft = m.cursorX - tw + 1
		}
		m.scrollXLeft = clamp(m.scrollXLeft, 0, maxScrollX(m.srcLines, tw))
	} else {
		if m.cursorX < m.scrollXRight {
			m.scrollXRight = m.cursorX
		} else if m.cursorX >= m.scrollXRight+tw {
			m.scrollXRight = m.cursorX - tw + 1
		}
		m.scrollXRight = clamp(m.scrollXRight, 0, maxScrollX(m.dstLines, tw))
	}
}
