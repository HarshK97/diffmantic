package tui
import (
	"fmt"
	"sort"
	"strings"
	"charm.land/lipgloss/v2"
	"github.com/HarshK97/diffmantic/internal/output"
	"github.com/alecthomas/chroma/v2"
	chromastyles "github.com/alecthomas/chroma/v2/styles"
)
const lineNumberWidth = 4
type moveConnection struct {
	srcMid int
	dstMid int
}
func renderDiffContent(file DiffFile, width int, s styles) string {
	if width < 12 {
		width = 12
	}
	leftAnn, rightAnn := buildLineAnnotations(file)
	// Collect move connections for drawing curve lines in the middle panel
	var moves []moveConnection
	for _, h := range file.displayHunks() {
		if h.Kind == output.ChangeMove {
			srcMid := (h.SrcStartLine + h.SrcEndLine) / 2
			dstMid := (h.DstStartLine + h.DstEndLine) / 2
			if srcMid > 0 && dstMid > 0 {
				moves = append(moves, moveConnection{srcMid: srcMid, dstMid: dstMid})
			}
		}
	}
	middleW := 3 // Width of the connection curve area
	// Border separators: left border is 1, right border is 1. Total = 2 columns.
	panelW := maxInt(1, (width-middleW-2)/2)
	rightW := maxInt(1, width-panelW-middleW-2)
	total := maxInt(len(file.OldLines), len(file.NewLines))
	if total == 0 {
		return s.Help.Render("(empty files)")
	}
	var b strings.Builder
	for i := 1; i <= total; i++ {
		leftLine, leftOK := lineAt(file.OldLines, i)
		rightLine, rightOK := lineAt(file.NewLines, i)
		leftToks := tokensAt(file.OldTokens, i)
		rightToks := tokensAt(file.NewTokens, i)
		leftAnnotation, leftAnnotated := leftAnn[i]
		rightAnnotation, rightAnnotated := rightAnn[i]
		leftOut := renderSideLine(i, leftLine, leftToks, leftOK, leftAnnotation, leftAnnotated, panelW, s)
		middleContent := renderMiddlePart(i, moves, middleW, s)
		rightOut := renderSideLine(i, rightLine, rightToks, rightOK, rightAnnotation, rightAnnotated, rightW, s)
		b.WriteString(leftOut)
		b.WriteString(s.Separator.Render("│"))
		b.WriteString(middleContent)
		b.WriteString(s.Separator.Render("│"))
		b.WriteString(rightOut)
		if i < total {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func renderMiddlePart(line int, moves []moveConnection, width int, s styles) string {
	if width <= 0 {
		return ""
	}
	for _, m := range moves {
		if line == m.srcMid {
			if m.srcMid < m.dstMid {
				return s.Move.Render("─╮" + strings.Repeat(" ", maxInt(0, width-2)))
			} else if m.srcMid > m.dstMid {
				return s.Move.Render("─╯" + strings.Repeat(" ", maxInt(0, width-2)))
			} else {
				if width > 1 {
					return s.Move.Render(strings.Repeat("─", width-1) + ">")
				}
				return s.Move.Render(">")
			}
		}
		if line == m.dstMid {
			if m.srcMid < m.dstMid {
				// Coming from above: e.g. " ╰>"
				if width >= 3 {
					return s.Move.Render(" " + "╰" + strings.Repeat("─", width-3) + ">")
				}
				return s.Move.Render("╰" + strings.Repeat("─", maxInt(0, width-2)) + ">")
			} else if m.srcMid > m.dstMid {
				// Coming from below: e.g. " ╭>"
				if width >= 3 {
					return s.Move.Render(" " + "╭" + strings.Repeat("─", width-3) + ">")
				}
				return s.Move.Render("╭" + strings.Repeat("─", maxInt(0, width-2)) + ">")
			}
		}
		// Vertical connecting line
		minL := minInt(m.srcMid, m.dstMid)
		maxL := maxInt(m.srcMid, m.dstMid)
		if line > minL && line < maxL {
			leftSpace := maxInt(0, (width-1)/2)
			rightSpace := maxInt(0, width-leftSpace-1)
			return strings.Repeat(" ", leftSpace) + s.Move.Render("│") + strings.Repeat(" ", rightSpace)
		}
	}
	return strings.Repeat(" ", width)
}
func lineAt(lines []string, line int) (string, bool) {
	if line <= 0 || line > len(lines) {
		return "", false
	}
	return lines[line-1], true
}
func tokensAt(tokens [][]chroma.Token, line int) []chroma.Token {
	if line <= 0 || line > len(tokens) {
		return nil
	}
	return tokens[line-1]
}
func renderSideLine(line int, content string, tokens []chroma.Token, ok bool, ann lineAnnotation, annotated bool, width int, s styles) string {
	if width <= 0 {
		return ""
	}
	lineNo := fmt.Sprintf("%*d", lineNumberWidth, line)
	if !ok {
		lineNo = strings.Repeat(" ", lineNumberWidth)
	}
	indicator := " "
	fillStyle := lipgloss.NewStyle()
	baseStyle := s.Context
	if annotated {
		switch ann.Kind {
		case output.ChangeInsert:
			indicator = "+"
			fillStyle = s.InsertFill
		case output.ChangeDelete:
			indicator = "-"
			fillStyle = s.DeleteFill
		case output.ChangeUpdate:
			indicator = "~"
			fillStyle = s.UpdateFill
		case output.ChangeMove:
			indicator = ">"
			fillStyle = s.MoveFill
		}
		baseStyle = fillStyle
	}
	if !ok {
		return strings.Repeat(" ", width)
	}
	numStyle := s.LineNumber
	indicatorStyle := s.Context
	if annotated {
		numStyle = fillStyle.Inherit(s.LineNumber)
		indicatorStyle = fillStyle
	}
	prefix := numStyle.Render(lineNo) + " " + indicatorStyle.Render(indicator) + " "
	contentW := maxInt(0, width-lipgloss.Width(prefix))
	spans := ann.Spans
	if annotated && (ann.Kind == output.ChangeInsert || ann.Kind == output.ChangeDelete) {
		spans = nil
	}
	rendered := renderTokenizedContent(content, tokens, spans, contentW, baseStyle, s)
	padding := ""
	if pad := contentW - lipgloss.Width(rendered); pad > 0 {
		padding = fillStyle.Render(strings.Repeat(" ", pad))
	}
	return prefix + rendered + padding
}
func renderTokenizedContent(originalContent string, tokens []chroma.Token, spans []visualSpan, width int, baseStyle lipgloss.Style, s styles) string {
	if width <= 0 {
		return ""
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].StartCol != spans[j].StartCol {
			return spans[i].StartCol < spans[j].StartCol
		}
		return spans[i].Priority > spans[j].Priority
	})
	if len(tokens) > 0 {
		var b strings.Builder
		used := 0
		byteIndex := 0
		monokaiStyle := chromastyles.Get("monokai")
		if monokaiStyle == nil {
			monokaiStyle = chromastyles.Fallback
		}
		for _, tok := range tokens {
			chromaEntry := monokaiStyle.Get(tok.Type)
			for _, r := range tok.Value {
				style := baseStyle
				if chromaEntry.Colour.IsSet() {
					style = style.Foreground(lipgloss.Color(chromaEntry.Colour.String()))
				}
				if chromaEntry.Bold == chroma.Yes {
					style = style.Bold(true)
				}
				if chromaEntry.Italic == chroma.Yes {
					style = style.Italic(true)
				}
				if chromaEntry.Underline == chroma.Yes {
					style = style.Underline(true)
				}
				if kind, ok := spanKindAt(spans, byteIndex); ok {
					spanStyle := tokenStyleForKind(kind, s)
					style = style.Inherit(spanStyle)
				}
				runeLen := len(string(r))
				if r == '\t' {
					// Tab is expanded to 4 spaces
					for k := 0; k < 4; k++ {
						rw := 1
						if used+rw > width {
							if used < width {
								b.WriteString(baseStyle.Render("."))
								used += rw
							}
							break
						}
						b.WriteString(style.Render(" "))
						used += rw
					}
					if used >= width {
						break
					}
				} else {
					rw := lipgloss.Width(string(r))
					if used+rw > width {
						if used < width {
							b.WriteString(baseStyle.Render("."))
						}
						break
					}
					b.WriteString(style.Render(string(r)))
					used += rw
				}
				byteIndex += runeLen
			}
			if used >= width {
				break
			}
		}
		return b.String()
	}
	// Fallback to raw plain text rendering
	var b strings.Builder
	used := 0
	for byteIndex, r := range originalContent {
		style := baseStyle
		if kind, ok := spanKindAt(spans, byteIndex); ok {
			style = tokenStyleForKind(kind, s)
		}
		if r == '\t' {
			for k := 0; k < 4; k++ {
				rw := 1
				if used+rw > width {
					if used < width {
						b.WriteString(baseStyle.Render("."))
						used += rw
					}
					break
				}
				b.WriteString(style.Render(" "))
				used += rw
			}
			if used >= width {
				break
			}
		} else {
			rw := lipgloss.Width(string(r))
			if used+rw > width {
				if used < width {
					b.WriteString(baseStyle.Render("."))
				}
				break
			}
			b.WriteString(style.Render(string(r)))
			used += rw
		}
	}
	return b.String()
}
func spanKindAt(spans []visualSpan, col int) (output.ChangeKind, bool) {
	bestPriority := 0
	var kind output.ChangeKind
	for _, span := range spans {
		end := span.EndCol
		if end <= 0 {
			end = span.StartCol + 1
		}
		if col >= span.StartCol && col < end && span.Priority >= bestPriority {
			bestPriority = span.Priority
			kind = span.Kind
		}
	}
	return kind, bestPriority > 0
}
func tokenStyleForKind(kind output.ChangeKind, s styles) lipgloss.Style {
	switch kind {
	case output.ChangeInsert:
		return s.Insert
	case output.ChangeDelete:
		return s.Delete
	case output.ChangeUpdate:
		return s.Update
	case output.ChangeMove:
		return s.Move
	default:
		return s.Context
	}
}
func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if lipgloss.Width(b.String()+string(r)) > width-1 {
			break
		}
		b.WriteRune(r)
	}
	if width == 1 {
		return "."
	}
	return b.String() + "."
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
