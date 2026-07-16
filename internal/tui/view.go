package tui

import (
	"fmt"
	"strings"
)

func (m model) View() string {
	if !m.ready {
		return "Loading..."
	}

	var b strings.Builder

	b.WriteString(m.renderTitleBar())
	b.WriteByte('\n')

	b.WriteString(m.renderContent())

	b.WriteByte('\n')
	b.WriteString(m.renderStatusBar())

	return b.String()
}

func (m model) renderTitleBar() string {
	pw := m.paneWidth()

	left := truncateStr(" "+m.srcFile, pw)
	right := truncateStr(" "+m.dstFile, pw)

	leftRendered := titleStyle.Render(padRight(left, pw))
	rightRendered := titleStyle.Render(padRight(right, pw))

	div := titleStyle.Render(dividerStyle.Render("│"))

	// Pad any extra column if the screen width is odd.
	totalUsed := pw + dividerWidth + pw
	remainder := ""
	if m.width > totalUsed {
		remainder = titleStyle.Render(strings.Repeat(" ", m.width-totalUsed))
	}

	return leftRendered + div + rightRendered + remainder
}

func (m model) renderContent() string {
	height := m.contentHeight()
	if height <= 0 {
		return ""
	}

	pw := m.paneWidth()
	gw := m.gutterWidth()
	tw := pw - gw
	if tw < 1 {
		tw = 1
	}

	leftLines := m.renderPane(m.srcLines, m.scrollXLeft, height, pw, gw, tw)
	rightLines := m.renderPane(m.dstLines, m.scrollXRight, height, pw, gw, tw)

	div := dividerStyle.Render("│")

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(leftLines[i])
		b.WriteString(div)
		b.WriteString(rightLines[i])
	}

	return b.String()
}

// renderPane formats lines for a single panel.
func (m model) renderPane(lines []string, scrollX, height, paneWidth, gutterW, textW int) []string {
	result := make([]string, height)

	for i := 0; i < height; i++ {
		lineIdx := m.scrollY + i

		if lineIdx < len(lines) {
			lineNum := fmt.Sprintf("%*d ", gutterW-gutterPadding, lineIdx+1)
			gutter := lineNumStyle.Render(lineNum)

			line := strings.ReplaceAll(lines[lineIdx], "\t", "    ")

			// Slice runes to handle horizontal scroll and multi-byte characters.
			runes := []rune(line)
			if scrollX > 0 && scrollX < len(runes) {
				runes = runes[scrollX:]
			} else if scrollX >= len(runes) {
				runes = nil
			}
			line = string(runes)

			line = truncateStr(line, textW)
			content := contentStyle.Render(padRight(line, textW))

			result[i] = gutter + content
		} else {
			// Draw tildes past EOF, like Vim.
			gutter := lineNumStyle.Render(padRight("~", gutterW))
			content := contentStyle.Render(strings.Repeat(" ", textW))
			result[i] = gutter + content
		}
	}

	return result
}

func (m model) renderStatusBar() string {
	keys := " ↑/k: up • ↓/j: down • ←/h: left • →/l: right • g/G: top/bottom • Ctrl+d/u: page • q: quit"

	prefix := m.digitBuffer
	prefixLen := len([]rune(prefix))

	var bar string
	if prefixLen > 0 {
		avail := m.width - prefixLen - 2
		if avail > 0 {
			bar = truncateStr(keys, avail)
			bar = padRight(bar, avail) + " " + prefix + " "
		} else {
			bar = prefix
		}
	} else {
		bar = truncateStr(keys, m.width)
		bar = padRight(bar, m.width)
	}
	return statusStyle.Render(bar)
}

func truncateStr(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 1 {
		return "…"
	}
	return string(runes[:maxLen-1]) + "…"
}

func padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(runes))
}
