package tui

import (
	"fmt"
	"strings"

	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/charmbracelet/lipgloss"
)

// humanNodeTypes maps common tree-sitter node types to short, user-friendly display names.
var humanNodeTypes = map[string]string{
	// Functions & methods
	"function_declaration":    "function",
	"function_definition":     "function",
	"method_declaration":      "method",
	"method_definition":       "method",
	"arrow_function":          "arrow fn",
	"generator_function":      "generator fn",
	"anonymous_function":      "anon fn",
	"function_item":           "function",
	"function_signature_item": "fn signature",
	"closure_expression":      "closure",
	"lambda":                  "lambda",
	"lambda_expression":       "lambda",

	// Classes & types
	"class_declaration":      "class",
	"class_definition":       "class",
	"struct_item":            "struct",
	"struct_type":            "struct",
	"enum_item":              "enum",
	"enum_declaration":       "enum",
	"interface_declaration":  "interface",
	"type_alias_declaration": "type alias",
	"type_declaration":       "type",
	"type_spec":              "type",
	"trait_item":             "trait",
	"impl_item":              "impl",

	// Expressions
	"call_expression":          "call",
	"selector_expression":      "selector",
	"member_expression":        "member access",
	"binary_expression":        "binary expr",
	"unary_expression":         "unary expr",
	"assignment_expression":    "assignment",
	"conditional_expression":   "ternary",
	"ternary_expression":       "ternary",
	"index_expression":         "index",
	"subscript_expression":     "subscript",
	"parenthesized_expression": "grouped expr",
	"template_string":          "template string",
	"new_expression":           "new",
	"await_expression":         "await",
	"yield_expression":         "yield",
	"spread_element":           "spread",
	"type_assertion":           "type assert",
	"as_expression":            "as cast",

	// Statements
	"if_statement":                "if",
	"for_statement":               "for",
	"for_in_statement":            "for-in",
	"while_statement":             "while",
	"do_statement":                "do-while",
	"switch_statement":            "switch",
	"return_statement":            "return",
	"expression_statement":        "expression",
	"variable_declaration":        "variable",
	"lexical_declaration":         "declaration",
	"try_statement":               "try",
	"throw_statement":             "throw",
	"break_statement":             "break",
	"continue_statement":          "continue",
	"defer_statement":             "defer",
	"go_statement":                "goroutine",
	"select_statement":            "select",
	"expression_switch_statement": "switch",
	"type_switch_statement":       "type switch",

	// Declarations & imports
	"import_declaration":    "import",
	"import_statement":      "import",
	"import_spec":           "import",
	"export_statement":      "export",
	"package_clause":        "package",
	"const_declaration":     "const",
	"var_declaration":       "var",
	"short_var_declaration": "short var",

	// Containers & structure
	"argument_list":       "arguments",
	"parameter_list":      "parameters",
	"formal_parameters":   "parameters",
	"statement_block":     "block",
	"source_file":         "file",
	"program":             "program",
	"module":              "module",
	"class_body":          "class body",
	"field_declaration":   "field",
	"field_identifier":    "field",
	"property_identifier": "property",

	// Clauses
	"catch_clause":   "catch",
	"finally_clause": "finally",
	"else_clause":    "else",
	"elif_clause":    "elif",
	"except_clause":  "except",
	"switch_case":    "case",
	"switch_default": "default",
	"select_case":    "select case",

	// Literals & values
	"string_literal":             "string",
	"interpreted_string_literal": "string",
	"raw_string_literal":         "raw string",
	"number_literal":             "number",
	"integer_literal":            "integer",
	"float_literal":              "float",
	"boolean":                    "bool",
	"true":                       "true",
	"false":                      "false",
	"nil":                        "nil",
	"null":                       "null",
	"undefined":                  "undefined",
	"comment":                    "comment",
	"line_comment":               "comment",
	"block_comment":              "comment",

	// Identifiers
	"identifier":                            "identifier",
	"field_expression":                      "field access",
	"type_identifier":                       "type name",
	"shorthand_property_identifier_pattern": "property",

	// Go-specific
	"composite_literal":  "literal",
	"func_literal":       "func literal",
	"slice_expression":   "slice",
	"range_clause":       "range",
	"communication_case": "chan case",

	// CSS-specific
	"rule_set":    "rule",
	"declaration": "declaration",
	"selectors":   "selectors",

	// HTML-specific
	"element":          "element",
	"start_tag":        "tag",
	"self_closing_tag": "self-closing tag",
	"attribute":        "attribute",
}

func humanizeNodeType(nodeType string) string {
	if h, ok := humanNodeTypes[nodeType]; ok {
		return h
	}
	s := nodeType
	for _, suffix := range []string{"_declaration", "_definition", "_statement", "_expression", "_literal", "_clause", "_item", "_spec"} {
		if cut, ok := strings.CutSuffix(s, suffix); ok {
			s = cut
			break
		}
	}
	return strings.ReplaceAll(s, "_", " ")
}

// actionsAtCursor returns actions under the cursor on the current line.
func actionsAtCursor(m *model) []*serialize.Action {
	if m.cursorY < 0 || m.cursorY >= len(m.virtualLines) {
		return nil
	}

	vl := m.virtualLines[m.cursorY]
	if vl.foldIdx >= 0 {
		return nil
	}

	var hl *highlights
	var lineIdx int
	if m.activePane == "left" {
		hl = m.srcHighlights
		lineIdx = vl.leftLine
	} else {
		hl = m.dstHighlights
		lineIdx = vl.rightLine
	}

	if lineIdx < 0 || hl == nil {
		return nil
	}

	lineSpans := hl.spans[lineIdx]
	if len(lineSpans) == 0 {
		return nil
	}

	var lines []string
	if m.activePane == "left" {
		lines = m.srcLines
	} else {
		lines = m.dstLines
	}
	if lineIdx >= len(lines) {
		return nil
	}

	_, byteToVisual := expandLine(lines[lineIdx])

	seen := map[*serialize.Action]bool{}
	var result []*serialize.Action

	for _, s := range lineSpans {
		if s.action == nil {
			continue
		}

		var sc, ec int
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		} else {
			ec = byteToVisual[len(byteToVisual)-1]
		}

		if m.cursorX >= sc && m.cursorX < ec {
			if !seen[s.action] {
				seen[s.action] = true
				result = append(result, s.action)
			}
		}
	}

	return result
}

func (m *model) updateInspectActions() {
	m.inspectActions = actionsAtCursor(m)
}

func actionKindFromString(s string) actionKind {
	switch s {
	case "delete":
		return kindDelete
	case "insert":
		return kindInsert
	case "update":
		return kindUpdate
	case "move":
		return kindMove
	default:
		return kindDelete
	}
}

// formatActionPreview creates a single-line summary of an action for the status bar.
func formatActionPreview(actions []*serialize.Action, maxWidth int, themeOpt ...*Theme) string {
	if len(actions) == 0 || maxWidth <= 0 {
		return ""
	}

	t := defaultTheme
	if len(themeOpt) > 0 && themeOpt[0] != nil {
		t = themeOpt[0]
	}
	bg := t.UI.Surface0

	a := actions[0]
	kind := actionKindFromString(a.Action)
	icon := t.ActionIcon(kind)
	fg := t.ActionFg(kind)

	label := strings.ToUpper(a.Action)
	nodeType := ""
	nodeName := ""
	if a.Node != nil {
		nodeType = humanizeNodeType(a.Node.Type)
		nodeName = a.Node.Label
	}

	var detail string
	switch a.Action {
	case "update":
		if a.OldValue != "" && a.NewValue != "" {
			old := truncateStr(a.OldValue, 20)
			newVal := truncateStr(a.NewValue, 20)
			detail = fmt.Sprintf("'%s' → '%s'", old, newVal)
		} else if nodeType != "" {
			detail = nodeType
			if nodeName != "" {
				detail += " '" + nodeName + "'"
			}
		}
	default:
		detail = nodeType
		if nodeName != "" {
			detail += " '" + nodeName + "'"
		}
	}

	parts := []styledPart{
		{text: " ▸ ", style: t.Styles.Status},
		{text: icon, style: lipgloss.NewStyle().Foreground(fg)},
		{text: " ", style: lipgloss.NewStyle()},
		{text: label, style: lipgloss.NewStyle().Foreground(fg).Bold(true)},
		{text: "  ", style: lipgloss.NewStyle()},
		{text: detail, style: t.Styles.Status},
	}

	if len(actions) > 1 {
		parts = append(parts, styledPart{
			text:  fmt.Sprintf("  (+%d more)", len(actions)-1),
			style: t.Styles.InspectDim,
		})
	}

	return renderInspectRow(parts, maxWidth, bg)
}

// formatByteRange formats byte offsets to human-readable line and column offsets.
func formatByteRange(lines []string, startByte, endByte uint32) string {
	startL, startC := byteToLineColFromLines(lines, startByte)
	endL, endC := byteToLineColFromLines(lines, endByte)
	return fmt.Sprintf("L%d:%d - L%d:%d", startL+1, startC+1, endL+1, endC+1)
}

func byteToLineColFromLines(lines []string, byteOffset uint32) (int, int) {
	if len(lines) == 0 {
		return 0, 0
	}
	lineIndex := make([]int, len(lines))
	curr := 0
	for i, l := range lines {
		lineIndex[i] = curr
		curr += len(l) + 1
	}
	if int(byteOffset) >= curr {
		return len(lines) - 1, 0
	}
	return serialize.ByteToLineCol(lineIndex, byteOffset)
}

type styledPart struct {
	text  string
	style lipgloss.Style
}

func renderInspectRow(parts []styledPart, totalWidth int, bg lipgloss.Color) string {
	var b strings.Builder
	used := 0
	baseStyle := lipgloss.NewStyle().Background(bg)

	for _, p := range parts {
		if used >= totalWidth || p.text == "" {
			continue
		}
		runes := []rune(p.text)
		avail := totalWidth - used
		if len(runes) > avail {
			if avail > 1 {
				runes = append(runes[:avail-1], '…')
			} else {
				runes = []rune{'…'}
			}
		}
		st := p.style.Background(bg)
		b.WriteString(st.Render(string(runes)))
		used += len(runes)
	}

	if used < totalWidth {
		b.WriteString(baseStyle.Render(strings.Repeat(" ", totalWidth-used)))
	}
	return b.String()
}

func padStyledParts(parts []styledPart, targetWidth int) []styledPart {
	used := 0
	var res []styledPart
	for _, p := range parts {
		if used >= targetWidth || p.text == "" {
			continue
		}
		runes := []rune(p.text)
		avail := targetWidth - used
		if len(runes) > avail {
			if avail > 1 {
				runes = append(runes[:avail-1], '…')
			} else {
				runes = []rune{'…'}
			}
		}
		res = append(res, styledPart{text: string(runes), style: p.style})
		used += len(runes)
	}
	if used < targetWidth {
		res = append(res, styledPart{text: strings.Repeat(" ", targetWidth-used), style: lipgloss.NewStyle()})
	}
	return res
}

func formatActionParts(lines []string, a *serialize.Action, colWidth int, t *Theme) [][]styledPart {
	if t == nil {
		t = defaultTheme
	}

	rows := make([][]styledPart, 3)

	kind := actionKindFromString(a.Action)
	fg := t.ActionFg(kind)
	icon := t.ActionIcon(kind)

	// Row 0: icon + " " + ACTION + (" " + nodeDesc)
	row0 := []styledPart{
		{text: icon, style: lipgloss.NewStyle().Foreground(fg)},
		{text: " ", style: lipgloss.NewStyle()},
		{text: strings.ToUpper(a.Action), style: lipgloss.NewStyle().Foreground(fg).Bold(true)},
	}
	if a.Node != nil {
		nodeDesc := humanizeNodeType(a.Node.Type)
		if a.Node.Label != "" {
			nodeDesc += " '" + a.Node.Label + "'"
		}
		row0 = append(row0,
			styledPart{text: " ", style: lipgloss.NewStyle()},
			styledPart{text: nodeDesc, style: lipgloss.NewStyle().Foreground(t.UI.Text)},
		)
	}
	rows[0] = row0

	// Row 1: Parent or Destination
	var row1 []styledPart
	switch a.Action {
	case "update":
		if a.OldValue != "" && a.NewValue != "" {
			old := truncateStr(a.OldValue, colWidth/2-2)
			newVal := truncateStr(a.NewValue, colWidth/2-2)
			row1 = []styledPart{
				{text: fmt.Sprintf("'%s' → '%s'", old, newVal), style: t.Styles.InspectDetail},
			}
		} else if a.Parent != nil {
			row1 = []styledPart{
				{text: fmt.Sprintf("parent: %s '%s'", humanizeNodeType(a.Parent.Type), a.Parent.Label), style: t.Styles.InspectDetail},
			}
		}
	case "move":
		if a.DestNode != nil {
			row1 = []styledPart{
				{text: fmt.Sprintf("→ dest: %s '%s' (Enter to jump)", humanizeNodeType(a.DestNode.Type), a.DestNode.Label), style: t.Styles.InspectDetail},
			}
		} else if a.DestStartByte != nil && a.DestEndByte != nil {
			row1 = []styledPart{
				{text: fmt.Sprintf("→ dest: %s (Enter to jump)", formatByteRange(lines, *a.DestStartByte, *a.DestEndByte)), style: t.Styles.InspectDetail},
			}
		}
	default:
		if a.Parent != nil {
			row1 = []styledPart{
				{text: fmt.Sprintf("parent: %s '%s'", humanizeNodeType(a.Parent.Type), a.Parent.Label), style: t.Styles.InspectDetail},
			}
		}
	}
	rows[1] = row1

	// Row 2: Line/Col range and Group ID
	var row2 []styledPart
	if a.Node != nil {
		row2 = append(row2, styledPart{
			text:  formatByteRange(lines, a.Node.StartByte, a.Node.EndByte),
			style: t.Styles.InspectDim,
		})
	}
	if a.GroupID != "" && a.Action != "move" {
		sep := ""
		if len(row2) > 0 {
			sep = " │ "
		}
		row2 = append(row2, styledPart{
			text:  sep + "grp: " + a.GroupID,
			style: t.Styles.InspectDim,
		})
	}
	rows[2] = row2

	return rows
}

func (m model) renderInspectPanel() string {
	width := m.width
	if width <= 0 {
		return ""
	}

	t := m.getTheme()
	bg := t.UI.Surface0
	panelLines := make([]string, inspectPanelHeight)

	if len(m.inspectActions) == 0 {
		panelLines[0] = renderInspectRow([]styledPart{
			{text: "  No action at cursor", style: t.Styles.InspectDim},
		}, width, bg)
		emptyRow := renderInspectRow(nil, width, bg)
		for i := 1; i < inspectPanelHeight; i++ {
			panelLines[i] = emptyRow
		}
		return strings.Join(panelLines, "\n")
	}

	var lines []string
	if m.activePane == "left" {
		lines = m.srcLines
	} else {
		lines = m.dstLines
	}

	if len(m.inspectActions) == 1 {
		colWidth := max(width-4, 10)
		accent := t.ActionFg(actionKindFromString(m.inspectActions[0].Action))
		borderStyle := lipgloss.NewStyle().Foreground(accent)
		titleStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)

		titleParts := []styledPart{
			{text: "│", style: borderStyle},
			{text: " ", style: lipgloss.NewStyle()},
			{text: "SEMANTIC INSPECTOR", style: titleStyle},
		}
		panelLines[0] = renderInspectRow(titleParts, width, bg)

		actionParts := formatActionParts(lines, m.inspectActions[0], colWidth, t)
		for i := range 3 {
			colParts := padStyledParts(actionParts[i], colWidth)
			rowParts := []styledPart{
				{text: "│", style: borderStyle},
				{text: " ", style: lipgloss.NewStyle()},
			}
			rowParts = append(rowParts, colParts...)
			panelLines[i+1] = renderInspectRow(rowParts, width, bg)
		}
	} else {
		// Cap at 2 side-by-side columns so the drawer stays readable.
		numCols := min(len(m.inspectActions), 2)

		divSpacing := 3 * (numCols - 1)
		availWidth := width - 4 - divSpacing
		colWidth := max(availWidth/numCols, 15)

		colsParts := make([][][]styledPart, numCols)
		for c := range numCols {
			colsParts[c] = formatActionParts(lines, m.inspectActions[c], colWidth, t)
		}

		accent := t.ActionFg(actionKindFromString(m.inspectActions[0].Action))
		borderStyle := lipgloss.NewStyle().Foreground(accent)
		titleStyle := lipgloss.NewStyle().Foreground(accent).Bold(true)

		titleParts := []styledPart{
			{text: "│", style: borderStyle},
			{text: " ", style: lipgloss.NewStyle()},
			{text: "SEMANTIC INSPECTOR", style: titleStyle},
		}
		panelLines[0] = renderInspectRow(titleParts, width, bg)

		for i := range 3 {
			rowParts := []styledPart{
				{text: "│", style: borderStyle},
				{text: " ", style: lipgloss.NewStyle()},
			}
			for c := range numCols {
				if c > 0 {
					rowParts = append(rowParts, styledPart{
						text:  " │ ",
						style: t.Styles.InspectDim,
					})
				}
				colParts := padStyledParts(colsParts[c][i], colWidth)
				rowParts = append(rowParts, colParts...)
			}
			panelLines[i+1] = renderInspectRow(rowParts, width, bg)
		}
	}

	return strings.Join(panelLines, "\n")
}

// visualColFromByte converts a line index and byte column into a visual display column.
func visualColFromByte(lines []string, lineIdx, byteCol int) int {
	if lineIdx >= len(lines) {
		return 0
	}
	_, byteToVisual := expandLine(lines[lineIdx])
	if byteCol < len(byteToVisual) {
		return byteToVisual[byteCol]
	}
	if len(byteToVisual) > 0 {
		return byteToVisual[len(byteToVisual)-1]
	}
	return 0
}

// jumpToMoveCounterpart jumps the cursor to the other side of a move action.
func (m *model) jumpToMoveCounterpart() {
	if len(m.inspectActions) == 0 {
		return
	}

	// Find the first MOVE action at cursor.
	var moveAct *serialize.Action
	for _, a := range m.inspectActions {
		if a.Action == "move" {
			moveAct = a
			break
		}
	}
	if moveAct == nil {
		return
	}

	targetRow := -1
	var targetPane string
	var targetCol int

	if m.activePane == "left" {
		if moveAct.DestStartByte == nil {
			return
		}
		dstLine, dstCol := byteToLineColFromLines(m.dstLines, *moveAct.DestStartByte)
		targetCol = visualColFromByte(m.dstLines, dstLine, dstCol)

		// Find the aligned grid row.
		for r, pair := range m.lineAlignment {
			if pair.RightLine == dstLine {
				targetRow = r
				break
			}
		}
		targetPane = "right"
	} else {
		if moveAct.Node == nil {
			return
		}
		srcLine, srcCol := byteToLineColFromLines(m.srcLines, moveAct.Node.StartByte)
		targetCol = visualColFromByte(m.srcLines, srcLine, srcCol)

		// Find the aligned grid row.
		for r, pair := range m.lineAlignment {
			if pair.LeftLine == srcLine {
				targetRow = r
				break
			}
		}
		targetPane = "left"
	}

	if targetRow == -1 {
		return
	}

	// Check if this row is inside a collapsed fold.
	foldOpened := false
	for fi := range m.folds {
		f := &m.folds[fi]
		if !f.open && targetRow >= f.startLine && targetRow <= f.endLine {
			f.open = true
			foldOpened = true
		}
	}
	if foldOpened {
		m.rebuildVirtualLines()
	}

	// Find the display line for the target row.
	for i, vl := range m.virtualLines {
		if vl.alignedRow == targetRow {
			m.cursorY = i
			m.cursorX = targetCol
			m.activePane = targetPane
			h := m.contentHeight()
			maxScroll := max(0, len(m.virtualLines)-h)
			m.scrollY = min(max(0, m.cursorY-(h/2)), maxScroll)

			m.clampCursor()
			m.keepCursorInViewport()
			m.updateInspectActions()
			break
		}
	}
}
