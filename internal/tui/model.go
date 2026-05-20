package tui
import (
	"fmt"
	"path/filepath"
	"strings"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	statusRows    = 2
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
}
func Run(files []DiffFile) error {
	if len(files) == 0 {
		return nil
	}
	_, err := tea.NewProgram(newModel(files)).Run()
	return err
}
func newModel(files []DiffFile) model {
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
	}
}
func (m model) Init() tea.Cmd {
	return tea.RequestWindowSize
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.resize(msg.Width, msg.Height)
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
		body = lipgloss.JoinHorizontal(lipgloss.Top, tree, m.styles.Separator.Render("|"), diff)
	}
	status := m.renderStatus()
	help := m.renderHelp()
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, body, status, help))
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
	file := m.files[m.selected]
	title := filepath.Base(displayPath(file))
	header := m.header(title, m.diffWidth, m.focus == focusDiff)
	return lipgloss.JoinVertical(lipgloss.Left, header, m.viewport.View())
}
func (m model) header(title string, width int, focused bool) string {
	if focused {
		title = "* " + title
	} else {
		title = "  " + title
	}
	return m.styles.Header.Width(width).Render(truncateToWidth(title, maxInt(0, width-2)))
}
func (m model) renderStatus() string {
	file := m.files[m.selected]
	focus := "diff"
	if m.focus == focusTree {
		focus = "files"
	}
	text := fmt.Sprintf(" %d files  %d hunks  %s  focus:%s", len(m.files), len(file.Hunks), displayPath(file), focus)
	return m.styles.Status.Width(m.width).Render(truncateToWidth(text, m.width))
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
