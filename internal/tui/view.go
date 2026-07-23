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

	leftLines := m.renderPane(m.srcLines, m.srcHighlights, m.srcSyntax, m.scrollXLeft, height, pw, gw, tw, true)
	rightLines := m.renderPane(m.dstLines, m.dstHighlights, m.dstSyntax, m.scrollXRight, height, pw, gw, tw, false)

	div := dividerStyle.Render("│")

	var b strings.Builder
	for i := 0; i < height; i++ {
		if i > 0 {
			b.WriteByte('\n')
		}
		// Check if this row is a fold marker: render a unified fold line across the divider.
		vIdx := m.scrollY + i
		if vIdx < len(m.virtualLines) && m.virtualLines[vIdx].foldIdx >= 0 {
			b.WriteString(leftLines[i])
			b.WriteString(dividerStyle.Background(colorSurface0).Render("│"))
			b.WriteString(rightLines[i])
		} else {
			b.WriteString(leftLines[i])
			b.WriteString(div)
			b.WriteString(rightLines[i])
		}
	}

	return b.String()
}

func (m model) renderPane(lines []string, hl *highlights, syntax map[int][]syntaxSpan, scrollX, height, paneWidth, gutterW, textW int, isLeftPane bool) []string {
	result := make([]string, height)

	for i := 0; i < height; i++ {
		vIdx := m.scrollY + i

		// Past the end of virtual lines.
		if vIdx >= len(m.virtualLines) {
			gutter := lineNumStyle.Render(padRight("~", gutterW))
			content := contentStyle.Render(strings.Repeat(" ", textW))
			result[i] = gutter + content
			continue
		}

		vl := m.virtualLines[vIdx]
		isCursorRow := vIdx == m.cursorY
		isActivePane := (isLeftPane && m.activePane == "left") || (!isLeftPane && m.activePane == "right")
		isCursor := isCursorRow && isActivePane

		// Fold marker row.
		if vl.foldIdx >= 0 {
			result[i] = m.renderFoldLine(vl.foldIdx, paneWidth, isCursor)
			continue
		}

		// Real line.
		var lineIdx int
		if isLeftPane {
			lineIdx = vl.leftLine
		} else {
			lineIdx = vl.rightLine
		}

		if lineIdx == -1 {
			var gutter string
			if isCursorRow && isActivePane {
				gutter = cursorGutterStyle.Render(strings.Repeat(" ", gutterW))
			} else {
				gutter = lineNumStyle.Render(strings.Repeat(" ", gutterW))
			}

			var content string
			fillerText := strings.Repeat("╱", textW)

			if isCursor {
				content = cursorContentStyle.Render(fillerText)
			} else {
				content = lineNumStyle.Render(fillerText)
			}

			result[i] = gutter + content
			continue
		}

		if lineIdx < len(lines) {
			lineSpans := hl.spans[lineIdx]

			lineNum := fmt.Sprintf("%*d ", gutterW-gutterPadding, lineIdx+1)
			var gutter string
			if isCursorRow && isActivePane {
				runes := []rune(lineNum)
				if len(runes) > 0 {
					runes[0] = '█'
				}
				gutter = cursorGutterStyle.Render(string(runes))
			} else {
				gutter = lineNumStyle.Render(lineNum)
			}

			rawLine := lines[lineIdx]
			var content string
			cursorCol := -1
			if isCursorRow && isActivePane {
				cursorCol = m.cursorX
			}

			syntaxSpans := syntax[lineIdx]

			if len(lineSpans) > 0 || len(syntaxSpans) > 0 {
				content = m.renderStyledLine(rawLine, lineSpans, syntaxSpans, scrollX, textW, cursorCol)
			} else {
				// Fast path: no highlights and no syntax
				line := strings.ReplaceAll(rawLine, "\t", "    ")
				runes := []rune(line)
				runeLen := len(runes)

				style := contentStyle
				if isCursor {
					style = cursorContentStyle
				} else if kind, ok := hl.tinted[lineIdx]; ok {
					style = hlStyle(kind)
				}

				padStyle := lipgloss.NewStyle()
				if isCursor {
					padStyle = padStyle.Background(colorSurface0)
				}

				var b strings.Builder
				for idx := 0; idx < textW; idx++ {
					col := scrollX + idx
					var r rune
					var s lipgloss.Style
					if col < runeLen {
						r = runes[col]
						s = style
					} else {
						r = ' '
						s = padStyle
					}

					if col == cursorCol {
						b.WriteString(s.Reverse(true).Blink(true).Render(string(r)))
					} else {
						b.WriteString(s.Render(string(r)))
					}
				}
				content = b.String()
			}
			result[i] = gutter + content
		} else {
			// EOF for this side (the other side might still have lines).
			gutter := lineNumStyle.Render(padRight("~", gutterW))
			content := contentStyle.Render(strings.Repeat(" ", textW))
			result[i] = gutter + content
		}
	}

	return result
}

func (m model) renderFoldLine(foldIdx, paneWidth int, isCursor bool) string {
	f := m.folds[foldIdx]
	count := f.endLine - f.startLine + 1
	label := fmt.Sprintf("⋯ %d lines hidden ⋯", count)
	style := foldStyle
	if isCursor {
		style = cursorFoldStyle
	}
	return style.Render(centerPad(label, paneWidth))
}

func centerPad(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	totalPad := width - len(runes)
	leftPad := totalPad / 2
	rightPad := totalPad - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}

// Combine diff action highlights (background) with syntax colors (foreground).
func (m model) renderStyledLine(rawLine string, lineSpans []span, synSpans []syntaxSpan, scrollX, textW int, cursorCol int) string {
	// Expand tabs and map original byte offsets to visual column positions.
	expanded, byteToVisual := expandLine(rawLine)
	runeLen := len([]rune(expanded))

	colHighlight := make([]int, runeLen)
	for i := range colHighlight {
		colHighlight[i] = -1
	}
	for _, s := range lineSpans {
		sc := -1
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		var ec int
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = runeLen
		}
		if sc >= 0 && ec > sc {
			for col := sc; col < ec && col < runeLen; col++ {
				if colHighlight[col] == -1 || s.kind < actionKind(colHighlight[col]) {
					colHighlight[col] = int(s.kind)
				}
			}
		}
	}

	colSyntax := make([]lipgloss.Color, runeLen)
	for _, s := range synSpans {
		sc := -1
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		var ec int
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = runeLen
		}
		if sc >= 0 && ec > sc {
			for col := sc; col < ec && col < runeLen; col++ {
				if colSyntax[col] == "" {
					colSyntax[col] = s.color
				}
			}
		}
	}

	basePadStyle := lipgloss.NewStyle()
	if cursorCol >= 0 {
		basePadStyle = basePadStyle.Background(colorSurface0)
	}

	expRunes := []rune(expanded)
	var b strings.Builder
	for idx := 0; idx < textW; idx++ {
		col := scrollX + idx
		var style lipgloss.Style
		var r rune

		if col < runeLen {
			r = expRunes[col]
			actionIdx := colHighlight[col]
			synColor := colSyntax[col]

			if actionIdx >= 0 {
				style = hlStyle(actionKind(actionIdx))
				if synColor != "" {
					style = style.Foreground(synColor)
				}
			} else if synColor != "" {
				if cursorCol >= 0 {
					style = cursorContentStyle.Foreground(synColor)
				} else {
					style = contentStyle.Foreground(synColor)
				}
			} else {
				if cursorCol >= 0 {
					style = cursorContentStyle
				} else {
					style = contentStyle
				}
			}
		} else {
			r = ' '
			style = basePadStyle
		}

		if col == cursorCol {
			b.WriteString(style.Reverse(true).Blink(true).Render(string(r)))
		} else {
			b.WriteString(style.Render(string(r)))
		}
	}
	return b.String()
}

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
	keys := " j/k: scroll • n/N: change • za: fold • zR/zM: all • q: quit"

	prefix := m.digitBuffer
	if m.pendingZ {
		prefix += "z"
	}
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
