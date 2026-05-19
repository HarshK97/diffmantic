package tui
import (
	"fmt"
	"sort"
	"strings"
	"charm.land/lipgloss/v2"
	"github.com/HarshK97/diffmantic/internal/output"
)
const lineNumberWidth = 4
func renderDiffContent(file DiffFile, width int, s styles) string {
	if width < 8 {
		width = 8
	}
	leftAnn, rightAnn := buildLineAnnotations(file)
	panelW := maxInt(1, (width-1)/2)
	rightW := maxInt(1, width-panelW-1)
	total := maxInt(len(file.OldLines), len(file.NewLines))
	if total == 0 {
		return s.Help.Render("(empty files)")
	}
	var b strings.Builder
	for i := 1; i <= total; i++ {
		leftLine, leftOK := lineAt(file.OldLines, i)
		rightLine, rightOK := lineAt(file.NewLines, i)
		leftAnnotation, leftAnnotated := leftAnn[i]
		rightAnnotation, rightAnnotated := rightAnn[i]
		b.WriteString(renderSideLine(i, leftLine, leftOK, leftAnnotation, leftAnnotated, panelW, s))
		b.WriteString(s.Separator.Render("|"))
		b.WriteString(renderSideLine(i, rightLine, rightOK, rightAnnotation, rightAnnotated, rightW, s))
		if i < total {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
func lineAt(lines []string, line int) (string, bool) {
	if line <= 0 || line > len(lines) {
		return "", false
	}
	return lines[line-1], true
}
func renderSideLine(line int, content string, ok bool, ann lineAnnotation, annotated bool, width int, s styles) string {
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
	rendered := renderTokenizedContent(sanitizeLine(content), ann.Spans, contentW, baseStyle, s)
	padding := ""
	if pad := contentW - lipgloss.Width(rendered); pad > 0 {
		padding = fillStyle.Render(strings.Repeat(" ", pad))
	}
	return prefix + rendered + padding
}
func sanitizeLine(line string) string {
	return strings.ReplaceAll(line, "\t", "    ")
}
func renderTokenizedContent(content string, spans []visualSpan, width int, baseStyle lipgloss.Style, s styles) string {
	if width <= 0 {
		return ""
	}
	sort.SliceStable(spans, func(i, j int) bool {
		if spans[i].StartCol != spans[j].StartCol {
			return spans[i].StartCol < spans[j].StartCol
		}
		return spans[i].Priority > spans[j].Priority
	})
	var b strings.Builder
	used := 0
	for byteIndex, r := range content {
		rw := lipgloss.Width(string(r))
		if used+rw > width {
			if used < width {
				b.WriteString(baseStyle.Render("."))
			}
			break
		}
		style := baseStyle
		if kind, ok := spanKindAt(spans, byteIndex); ok {
			style = tokenStyleForKind(kind, s)
		}
		b.WriteString(style.Render(string(r)))
		used += rw
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
