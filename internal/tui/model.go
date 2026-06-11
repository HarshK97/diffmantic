package tui
import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/output"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

type focusArea int

const (
	focusTree focusArea = iota
	focusDiff
)

const (
	minWidth      = 40
	minHeight     = 8
	minTreeWidth  = 28
	maxTreeWidth  = 36
	statusRows    = 1
	separatorCols = 1
)

type model struct {
	files       []DiffFile
	root        *treeNode
	rows        []treeRow
	treeCursor  int
	treeOffset  int
	treeVisible bool
	selected    int
	focus       focusArea
	viewport    viewport.Model
	styles      styles
	width       int
	height      int
	treeWidth   int
	diffWidth   int
	contentRows int
	ready       bool
	loading     bool
	frame       int
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

type engineDoneMsg struct {
	Result []DiffFile
	Err    error
}

func computeDiffsCmd(files []DiffFile) tea.Cmd {
	return func() tea.Msg {
		finalized := make([]DiffFile, len(files))
		var wg sync.WaitGroup
		errs := make([]error, len(files))

		for i, f := range files {
			wg.Add(1)
			go func(idx int, file DiffFile) {
				defer wg.Done()
				oldSrc := []byte(strings.Join(file.OldLines, "\n"))
				newSrc := []byte(strings.Join(file.NewLines, "\n"))

				detectPathA := file.OldPath
				if detectPathA == "" {
					detectPathA = file.NewPath
				}
				detectPathB := file.NewPath
				if detectPathB == "" {
					detectPathB = file.OldPath
				}

				astA, errA := treesitter.Parse(oldSrc, detectPathA)
				astB, errB := treesitter.Parse(newSrc, detectPathB)

				if errA != nil || errB != nil {
					// Fallback gracefully for unsupported files (like go.mod, go.sum, LICENSE)
					// We build a standard DiffFile without detail mappings (rendered as raw plain text comparison).
					finalized[idx] = NewDiffFile(file.OldPath, file.NewPath, oldSrc, newSrc, nil)
					finalized[idx].RelPath = file.RelPath
					return
				}

				result := engine.Match(astA, astB)
				actions := engine.GenerateActions(astA, astB, result.Mappings)
				hunks := output.Classify(actions)
				hunks = output.Coalesce(hunks)

				finalized[idx] = NewDiffFileWithDetails(
					file.OldPath, file.NewPath,
					oldSrc, newSrc,
					hunks, actions, result.Mappings,
				)
				finalized[idx].RelPath = file.RelPath

				// Async background logging
				go func(relPath string, r *engine.MatchResult, act *engine.EditScript, h []output.Hunk) {
					var buf bytes.Buffer
					fmt.Fprintf(&buf, "=== Diffing %s ===\n\n", relPath)
					engine.FprintMappings(&buf, r)
					engine.FprintActions(&buf, act)
					output.FprintHunks(&buf, h)
					fmt.Fprintln(&buf, "====================")
					log.Printf("\n%s", buf.String())
				}(file.RelPath, result, actions, hunks)
			}(i, f)
		}

		wg.Wait()

		// Add 2 second timer to show the shimmering skeleton loading screen for demo/testing
		time.Sleep(2 * time.Second)

		for _, err := range errs {
			if err != nil {
				return engineDoneMsg{Err: err}
			}
		}

		return engineDoneMsg{Result: finalized}
	}
}

func Run(files []DiffFile) error {
	if len(files) == 0 {
		return nil
	}
	logPath := "/tmp/diffmantic.log"
	if os.PathSeparator == '\\' {
		logPath = filepath.Join(os.TempDir(), "diffmantic.log")
	}
	os.Remove(logPath)

	f, err := tea.LogToFile(logPath, "debug")
	if err == nil {
		defer f.Close()
		log.Println("[DEBUG] TUI Session started")
	}

	_, err = tea.NewProgram(newModel(files)).Run()
	if err == nil {
		fmt.Printf("Debug log stored at: %s\n", logPath)
	}
	return err
}

func newModel(files []DiffFile) model {
	loading := false
	if len(files) > 0 && files[0].NeedsCompute {
		loading = true
	}

	root := buildFileTree(files)
	rows := flattenTree(root)
	cursor := firstVisibleFile(rows)
	selected := 0
	if len(rows) > 0 && rows[cursor].node.fileIndex >= 0 {
		selected = rows[cursor].node.fileIndex
	}
	return model{
		files:       files,
		root:        root,
		rows:        rows,
		treeCursor:  cursor,
		treeVisible: true,
		selected:    selected,
		focus:       focusDiff,
		viewport:    viewport.New(viewport.WithWidth(1), viewport.WithHeight(1)),
		styles:      newStyles(),
		loading:     loading,
	}
}

func (m model) Init() tea.Cmd {
	if m.loading {
		return tea.Batch(
			tea.RequestWindowSize,
			tickCmd(),
			computeDiffsCmd(m.files),
		)
	}
	return tea.RequestWindowSize
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
		return m, nil
	case tickMsg:
		if m.loading {
			m.frame++
			return m, tickCmd()
		}
		return m, nil
	case engineDoneMsg:
		m.loading = false
		if msg.Err == nil {
			m.files = msg.Result
			m.root = buildFileTree(m.files)
			m.rows = flattenTree(m.root)
			m.treeVisible = true
			m.reflowViewport(true)
		}
		return m, nil
	case tea.KeyPressMsg:
		return m.updateKey(msg)
	}
	return m, nil
}

func (m model) View() tea.View {
	if !m.ready {
		v := tea.NewView("")
		v.AltScreen = true
		return v
	}
	if m.width < minWidth || m.height < minHeight {
		v := tea.NewView(fmt.Sprintf("Terminal too small (%dx%d). Minimum: %dx%d", m.width, m.height, minWidth, minHeight))
		v.AltScreen = true
		return v
	}

	tree := m.renderTree()
	diff := m.renderDiff()
	body := diff
	if m.treeVisible {
		body = lipgloss.JoinHorizontal(lipgloss.Top, tree, m.styles.Separator.Render("│"), diff)
	}
	help := m.renderHelp()

	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, help))
	v.AltScreen = true
	v.WindowTitle = "diffmantic"
	return v
}

func (m *model) resize(width, height int) {
	m.width = width
	m.height = height
	m.contentRows = maxInt(1, height-statusRows)
	m.reflowViewport(true)
	m.ready = true
	m.ensureTreeCursorVisible()
}

func (m model) updateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "tab":
		if !m.treeVisible {
			return m, nil
		}
		if m.focus == focusTree {
			m.focus = focusDiff
		} else {
			m.focus = focusTree
		}
		return m, nil
	case "t":
		m.treeVisible = !m.treeVisible
		if !m.treeVisible {
			m.focus = focusDiff
		}
		m.reflowViewport(false)
		return m, nil
	}

	if m.focus == focusTree {
		return m.updateTreeKey(msg), nil
	}

	switch msg.String() {
	case "g":
		m.viewport.GotoTop()
	case "G":
		m.viewport.GotoBottom()
	default:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) reflowViewport(reset bool) {
	if m.treeVisible {
		m.treeWidth = minInt(maxTreeWidth, maxInt(minTreeWidth, m.width/4))
		m.diffWidth = maxInt(1, m.width-m.treeWidth-separatorCols)
	} else {
		m.treeWidth = 0
		m.diffWidth = maxInt(1, m.width)
	}
	offset := m.viewport.YOffset()
	m.viewport = viewport.New(
		viewport.WithWidth(m.diffWidth),
		viewport.WithHeight(maxInt(1, m.contentRows-1)),
	)
	m.setViewportContent(reset)
	if !reset {
		m.viewport.SetYOffset(offset)
	}
}

func (m model) updateTreeKey(msg tea.KeyPressMsg) model {
	switch msg.String() {
	case "j", "down":
		m.treeCursor = clampTreeCursor(m.treeCursor+1, m.rows)
	case "k", "up":
		m.treeCursor = clampTreeCursor(m.treeCursor-1, m.rows)
	case "g":
		m.treeCursor = 0
	case "G":
		m.treeCursor = maxInt(0, len(m.rows)-1)
	case "enter", "space":
		m.activateTreeRow()
	case "left":
		m.collapseTreeRow()
	case "right":
		m.expandTreeRow()
	}
	m.treeCursor = clampTreeCursor(m.treeCursor, m.rows)
	m.selectTreeFile()
	m.ensureTreeCursorVisible()
	return m
}

func (m *model) activateTreeRow() {
	if len(m.rows) == 0 {
		return
	}
	node := m.rows[m.treeCursor].node
	if len(node.children) > 0 {
		node.collapsed = !node.collapsed
		m.rows = flattenTree(m.root)
		m.treeCursor = clampTreeCursor(m.treeCursor, m.rows)
		return
	}
	m.selectTreeFile()
}

func (m *model) collapseTreeRow() {
	if len(m.rows) == 0 {
		return
	}
	node := m.rows[m.treeCursor].node
	if len(node.children) > 0 && !node.collapsed {
		node.collapsed = true
		m.rows = flattenTree(m.root)
		return
	}
	if node.parent != nil && node.parent.parent != nil {
		for i, row := range m.rows {
			if row.node == node.parent {
				m.treeCursor = i
				return
			}
		}
	}
}

func (m *model) expandTreeRow() {
	if len(m.rows) == 0 {
		return
	}
	node := m.rows[m.treeCursor].node
	if len(node.children) > 0 && node.collapsed {
		node.collapsed = false
		m.rows = flattenTree(m.root)
	}
}

func (m *model) selectTreeFile() {
	if len(m.rows) == 0 {
		return
	}
	node := m.rows[m.treeCursor].node
	if node.fileIndex < 0 || node.fileIndex == m.selected {
		return
	}
	m.selected = node.fileIndex
	m.setViewportContent(true)
}

func (m *model) ensureTreeCursorVisible() {
	if m.contentRows <= 1 {
		m.treeOffset = 0
		return
	}
	treeRows := m.contentRows - 1
	if m.treeCursor < m.treeOffset {
		m.treeOffset = m.treeCursor
	}
	if m.treeCursor >= m.treeOffset+treeRows {
		m.treeOffset = m.treeCursor - treeRows + 1
	}
	if m.treeOffset < 0 {
		m.treeOffset = 0
	}
}

func (m *model) setViewportContent(reset bool) {
	if len(m.files) == 0 || m.selected >= len(m.files) {
		m.viewport.SetContent("")
		return
	}
	m.viewport.SetContent(renderDiffContent(m.files[m.selected], m.diffWidth, m.styles))
	if reset {
		m.viewport.GotoTop()
	}
}

func (m model) renderTree() string {
	header := m.header("Files", m.treeWidth, m.focus == focusTree)
	visibleRows := maxInt(0, m.contentRows-1)
	end := minInt(len(m.rows), m.treeOffset+visibleRows)
	var lines []string
	for i := m.treeOffset; i < end; i++ {
		lines = append(lines, m.renderTreeRow(i, m.rows[i]))
	}
	for len(lines) < visibleRows {
		lines = append(lines, strings.Repeat(" ", m.treeWidth))
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, strings.Join(lines, "\n"))
}

func (m model) renderTreeRow(index int, row treeRow) string {
	node := row.node
	prefix := strings.Repeat("  ", row.depth)
	icon := "  "
	if len(node.children) > 0 {
		if node.collapsed {
			icon = "+ "
		} else {
			icon = "- "
		}
	}
	name := node.name
	if node.fileIndex >= 0 {
		name = filepath.Base(displayPath(m.files[node.fileIndex]))
	}
	text := prefix + icon + name
	if m.loading {
		length := len(name)
		if length < 6 {
			length = 6
		}
		if length > 15 {
			length = 15
		}
		colorIdx := (m.frame + index) % len(shimmerColors)
		color := shimmerColors[colorIdx]
		shimmerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(color))
		text = prefix + icon + shimmerStyle.Render(strings.Repeat("█", length))
	}
	text = truncateToWidth(text, m.treeWidth)
	style := m.styles.Tree
	if index == m.treeCursor {
		if m.focus == focusTree {
			style = m.styles.SelectedFocus
		} else {
			style = m.styles.TreeSelected
		}
	} else if len(node.children) > 0 {
		style = m.styles.TreeFocused
	}
	return style.Width(m.treeWidth).Render(text)
}

func (m model) renderDiff() string {
	if m.loading {
		title := "Comparing..."
		if len(m.files) > 0 {
			title = filepath.Base(m.files[m.selected].OldPath) + " ↔ " + filepath.Base(m.files[m.selected].NewPath)
		}
		header := m.header(title, m.diffWidth, false)
		
		skeletonH := maxInt(1, m.contentRows-1)
		skeletonLines := m.renderSkeletonLines(m.diffWidth, skeletonH)
		return lipgloss.JoinVertical(lipgloss.Left, header, skeletonLines)
	}

	file := m.files[m.selected]
	title := filepath.Base(displayPath(file))
	header := m.header(title, m.diffWidth, m.focus == focusDiff)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}

var shimmerColors = []string{
	"#1e1e2e",
	"#252538",
	"#2c2c42",
	"#34344d",
	"#3b3b57",
	"#434362",
	"#3b3b57",
	"#34344d",
	"#2c2c42",
	"#252538",
}

func (m model) shimmerBlock(width int, rowIdx int) string {
	if width <= 0 {
		return ""
	}
	colorIdx := (m.frame + rowIdx) % len(shimmerColors)
	colorHex := shimmerColors[colorIdx]
	style := lipgloss.NewStyle().
		Background(lipgloss.Color(colorHex)).
		Foreground(lipgloss.Color(colorHex))
	return style.Render(strings.Repeat(" ", width))
}

func (m model) renderSkeletonLines(width, height int) string {
	if len(m.files) == 0 || m.selected >= len(m.files) {
		return ""
	}
	file := m.files[m.selected]
	middleW := 3
	panelW := maxInt(1, (width-middleW-2)/2)
	rightW := maxInt(1, width-panelW-middleW-2)
	totalLines := maxInt(len(file.OldLines), len(file.NewLines))
	if totalLines == 0 {
		return ""
	}
	var lines []string
	limit := minInt(totalLines, height)
	for i := 1; i <= limit; i++ {
		// --- Left Panel ---
		leftOut := ""
		if i <= len(file.OldLines) {
			lineText := file.OldLines[i-1]
			// Measure indentation
			indentStr := ""
			contentStart := 0
			for idx, r := range lineText {
				if r == '\t' {
					indentStr += "    " // Expand tabs to 4 spaces
					contentStart = idx + 1
				} else if r == ' ' {
					indentStr += " "
					contentStart = idx + 1
				} else {
					break
				}
			}
			contentLen := lipgloss.Width(lineText[contentStart:])
			lineNoStr := m.styles.LineNumber.Render(fmt.Sprintf("%4d", i))
			prefix := lineNoStr + " " + indentStr
			prefixW := lipgloss.Width(prefix)
			// Cap the shimmer if it exceeds panel width
			shimmerW := contentLen
			if prefixW+shimmerW > panelW {
				shimmerW = maxInt(0, panelW-prefixW)
			}
			leftOut = prefix + m.shimmerBlock(shimmerW, i)
			// Pad the rest of the panel
			if pad := panelW - lipgloss.Width(leftOut); pad > 0 {
				leftOut += strings.Repeat(" ", pad)
			}
		} else {
			leftOut = strings.Repeat(" ", panelW)
		}
		sep := m.styles.Separator.Render("│") + strings.Repeat(" ", middleW) + m.styles.Separator.Render("│")
		rightOut := ""
		if i <= len(file.NewLines) {
			lineText := file.NewLines[i-1]
			// Measure indentation
			indentStr := ""
			contentStart := 0
			for idx, r := range lineText {
				if r == '\t' {
					indentStr += "    " // Expand tabs to 4 spaces
					contentStart = idx + 1
				} else if r == ' ' {
					indentStr += " "
					contentStart = idx + 1
				} else {
					break
				}
			}
			contentLen := lipgloss.Width(lineText[contentStart:])
			lineNoStr := m.styles.LineNumber.Render(fmt.Sprintf("%4d", i))
			prefix := lineNoStr + " " + indentStr
			prefixW := lipgloss.Width(prefix)
			// Cap the shimmer if it exceeds panel width
			shimmerW := contentLen
			if prefixW+shimmerW > rightW {
				shimmerW = maxInt(0, rightW-prefixW)
			}
			rightOut = prefix + m.shimmerBlock(shimmerW, i)
			// Pad the rest of the panel
			if pad := rightW - lipgloss.Width(rightOut); pad > 0 {
				rightOut += strings.Repeat(" ", pad)
			}
		} else {
			rightOut = strings.Repeat(" ", rightW)
		}
		lines = append(lines, leftOut+sep+rightOut)
	}
	// Pad empty lines if the viewport is taller than the files
	for len(lines) < height {
		leftPad := strings.Repeat(" ", panelW)
		sep := m.styles.Separator.Render("│") + strings.Repeat(" ", middleW) + m.styles.Separator.Render("│")
		rightPad := strings.Repeat(" ", rightW)
		lines = append(lines, leftPad+sep+rightPad)
	}
	return strings.Join(lines, "\n")
}

func (m model) header(title string, width int, focused bool) string {
	if focused {
		title = "* " + title
	} else {
		title = "  " + title
	}
	return m.styles.Header.Width(width).Render(truncateToWidth(title, maxInt(0, width-2)))
}

func (m model) renderHelp() string {
	pairs := []struct {
		key  string
		desc string
	}{
		{"t", "tree"},
		{"tab", "focus"},
		{"j/k", "move"},
		{"g/G", "top/bottom"},
		{"enter", "select/toggle"},
		{"left/right", "collapse/expand"},
		{"q", "quit"},
	}
	parts := make([]string, 0, len(pairs))
	for _, pair := range pairs {
		parts = append(parts, m.styles.HelpKey.Render(pair.key)+" "+m.styles.Help.Render(pair.desc))
	}
	return lipgloss.NewStyle().Width(m.width).Render(" " + strings.Join(parts, "  "))
}
