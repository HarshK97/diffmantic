package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
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

	leftLines := m.renderPane(m.srcLines, m.srcHighlights, m.scrollXLeft, height, pw, gw, tw)
	rightLines := m.renderPane(m.dstLines, m.dstHighlights, m.scrollXRight, height, pw, gw, tw)

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

// renderPane formats and renders the content for a single side of the diff.
func (m model) renderPane(lines []string, hl *highlights, scrollX, height, paneWidth, gutterW, textW int) []string {
	result := make([]string, height)

	for i := 0; i < height; i++ {
		lineIdx := m.scrollY + i

		if lineIdx < len(lines) {
			lineSpans := hl.spans[lineIdx]

			lineNum := fmt.Sprintf("%*d ", gutterW-gutterPadding, lineIdx+1)
			gutter := lineNumStyle.Render(lineNum)

			rawLine := lines[lineIdx]
			var content string
			if len(lineSpans) > 0 {
				content = m.renderHighlightedLine(rawLine, lineSpans, scrollX, textW)
			} else {
				line := strings.ReplaceAll(rawLine, "\t", "    ")
				runes := []rune(line)
				if scrollX > 0 && scrollX < len(runes) {
					runes = runes[scrollX:]
				} else if scrollX >= len(runes) {
					runes = nil
				}
				line = string(runes)
				line = truncateStr(line, textW)
				content = contentStyle.Render(padRight(line, textW))
			}
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

func (m model) renderHighlightedLine(rawLine string, lineSpans []span, scrollX, textW int) string {
	expanded, byteToVisual := expandLine(rawLine)

	visualSpans := make([]span, 0, len(lineSpans))
	for _, s := range lineSpans {
		vs := span{kind: s.kind}
		if s.startCol < len(byteToVisual) {
			vs.startCol = byteToVisual[s.startCol]
		} else {
			vs.startCol = len([]rune(expanded))
		}
		if s.endCol < len(byteToVisual) {
			vs.endCol = byteToVisual[s.endCol]
		} else {
			vs.endCol = len([]rune(expanded))
		}
		if vs.endCol > vs.startCol {
			visualSpans = append(visualSpans, vs)
		}
	}

	runes := []rune(expanded)
	if scrollX > 0 && scrollX < len(runes) {
		runes = runes[scrollX:]
		for i := range visualSpans {
			visualSpans[i].startCol -= scrollX
			visualSpans[i].endCol -= scrollX
			if visualSpans[i].startCol < 0 {
				visualSpans[i].startCol = 0
			}
		}
	} else if scrollX >= len(runes) {
		runes = nil
		visualSpans = nil
	}

	if len(runes) > textW {
		runes = runes[:textW]
	}

	runeLen := len(runes)

	// Resolve overlaps (highest priority wins).
	colHighlight := make([]int, runeLen)
	for i := range colHighlight {
		colHighlight[i] = -1
	}

	for _, vs := range visualSpans {
		sc := vs.startCol
		ec := vs.endCol
		if sc < 0 {
			sc = 0
		}
		if ec > runeLen {
			ec = runeLen
		}
		for col := sc; col < ec; col++ {
			if colHighlight[col] == -1 || vs.kind < actionKind(colHighlight[col]) {
				colHighlight[col] = int(vs.kind)
			}
		}
	}

	// Merge adjacent characters with the same style.
	var cleanSpans []span
	inSpan := false
	var start int
	var currentKind actionKind

	for col := 0; col < runeLen; col++ {
		kind := colHighlight[col]
		if inSpan {
			if kind == -1 || actionKind(kind) != currentKind {
				cleanSpans = append(cleanSpans, span{
					startCol: start,
					endCol:   col,
					kind:     currentKind,
				})
				if kind != -1 {
					start = col
					currentKind = actionKind(kind)
				} else {
					inSpan = false
				}
			}
		} else {
			if kind != -1 {
				inSpan = true
				start = col
				currentKind = actionKind(kind)
			}
		}
	}
	if inSpan {
		cleanSpans = append(cleanSpans, span{
			startCol: start,
			endCol:   runeLen,
			kind:     currentKind,
		})
	}
	visualSpans = cleanSpans

	baseStyle := contentStyle
	basePadStyle := lipgloss.NewStyle()

	if len(visualSpans) == 0 {
		text := string(runes)
		text = padRight(text, textW)
		return baseStyle.Render(text)
	}

	var b strings.Builder
	pos := 0

	for _, vs := range visualSpans {
		if vs.startCol >= runeLen || vs.endCol <= 0 {
			continue
		}
		sc := vs.startCol
		ec := vs.endCol
		if sc < 0 {
			sc = 0
		}
		if ec > runeLen {
			ec = runeLen
		}

		if pos < sc {
			b.WriteString(baseStyle.Render(string(runes[pos:sc])))
		}

		style := hlStyle(vs.kind)
		b.WriteString(style.Render(string(runes[sc:ec])))

		pos = ec
	}

	if pos < runeLen {
		b.WriteString(baseStyle.Render(string(runes[pos:])))
	}

	rendered := b.String()
	visualLen := runeLen
	if visualLen < textW {
		rendered += basePadStyle.Render(strings.Repeat(" ", textW-visualLen))
	}

	return rendered
}

// expandLine replaces tabs with 4 spaces and tracks where byte offsets land visually.
func expandLine(line string) (string, []int) {
	byteToVisual := make([]int, len(line)+1)
	var expanded strings.Builder
	visualCol := 0

	for i := 0; i < len(line); i++ {
		byteToVisual[i] = visualCol
		if line[i] == '\t' {
			expanded.WriteString("    ")
			visualCol += 4
		} else {
			expanded.WriteByte(line[i])
			visualCol++
		}
	}
	byteToVisual[len(line)] = visualCol

	return expanded.String(), byteToVisual
}

func (m model) renderStatusBar() string {
	keys := " ↑/k: up • ↓/j: down • ←/h →/l: scroll • n/N: next/prev change • g/G: top/bottom • q: quit"

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
