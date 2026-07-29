package tui

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/HarshK97/diffmantic/internal/actions"
	"github.com/HarshK97/diffmantic/internal/engine"
	"github.com/HarshK97/diffmantic/internal/git"
	"github.com/HarshK97/diffmantic/internal/postprocess"
	"github.com/HarshK97/diffmantic/internal/serialize"
	"github.com/HarshK97/diffmantic/internal/treesitter"
)

// Run launches the side-by-side terminal diff viewer.
func Run(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope) error {
	m := newModel(srcFile, dstFile, srcBytes, dstBytes, env)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

// RunGit starts the TUI in Git mode inside the specified repository path.
func RunGit(repoPath string, refA, refB string, pathFilter string, stagedOnly bool) error {
	m := newGitModel(repoPath, refA, refB, pathFilter, stagedOnly)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func newModel(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope) model {
	m := model{
		activePane: "left",
	}

	m.setupDiff(srcFile, dstFile, srcBytes, dstBytes, env)

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Prompt = " / "
	ti.CharLimit = 50
	ti.Width = 30
	m.textinput = ti

	return m
}

func newGitModel(repoPath string, refA, refB string, pathFilter string, stagedOnly bool) model {
	m := model{
		gitMode:       true,
		gitTreeOpen:   true,
		repoPath:      repoPath,
		refA:          refA,
		refB:          refB,
		gitStagedOnly: stagedOnly,
		pathFilter:    pathFilter,
		activePane:    "left",
	}

	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.Prompt = " / "
	ti.CharLimit = 50
	ti.Width = 30
	m.textinput = ti

	ci := textinput.New()
	ci.Placeholder = "Commit message..."
	ci.Prompt = " Commit: "
	ci.CharLimit = 100
	ci.Width = 60
	m.gitCommitInput = ci

	m.refreshGitStatus()

	if len(m.gitItems) > 0 {
		firstFileIdx := -1
		for i, item := range m.gitItems {
			if !item.isHeader {
				firstFileIdx = i
				break
			}
		}
		if firstFileIdx != -1 {
			m.gitCursorY = firstFileIdx
			_ = m.loadGitFileDiff(firstFileIdx)
		} else {
			m.setupEmptyPlaceholder()
		}
	} else {
		m.setupEmptyPlaceholder()
	}

	return m
}

func (m *model) setupEmptyPlaceholder() {
	m.srcFile = "No changes"
	m.dstFile = "No changes"
	m.srcLines = []string{""}
	m.dstLines = []string{""}
	m.lineAlignment = []serialize.LineAlignmentPair{{LeftLine: 0, RightLine: 0}}
	m.srcHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
	m.dstHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
	m.folds = nil
	m.rebuildVirtualLines()
	m.srcSyntax = nil
	m.dstSyntax = nil
}

func (m *model) setupDiff(srcFile, dstFile string, srcBytes, dstBytes []byte, env *serialize.Envelope) {
	m.srcFile = srcFile
	m.dstFile = dstFile
	m.srcLines = strings.Split(string(srcBytes), "\n")
	m.dstLines = strings.Split(string(dstBytes), "\n")

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
		m.allChanges = nil
	}

	// Build collapsible folds from unchanged lines.
	m.folds = computeFolds(m.allChanges, len(m.lineAlignment), foldContext)
	m.rebuildVirtualLines()

	// Pre-compute syntax colors upfront so rendering stays fast on scroll.
	m.srcSyntax = highlightSyntax(srcFile, srcBytes)
	m.dstSyntax = highlightSyntax(dstFile, dstBytes)

	// Reset scroll and cursor.
	m.cursorY = 0
	m.cursorX = 0
	m.scrollY = 0
	m.scrollXLeft = 0
	m.scrollXRight = 0
	m.clampCursor()
	m.keepCursorInViewport()
}

func (m *model) refreshGitStatus() {
	if m.refA != "" {
		files, err := git.GetChangedFiles(m.repoPath, m.refA, m.refB, m.pathFilter)
		if err != nil {
			m.gitItems = nil
			return
		}

		var items []gitTreeItem
		headerText := fmt.Sprintf("Changes: %s → Working Copy", m.refA)
		if m.refB != "" {
			headerText = fmt.Sprintf("Changes: %s → %s", m.refA, m.refB)
		}
		items = append(items, gitTreeItem{isHeader: true, headerText: headerText})

		for _, f := range files {
			status := f.Status
			if len(status) == 1 {
				status += " "
			}
			items = append(items, gitTreeItem{
				path:      f.Path,
				oldPath:   f.OldPath,
				status:    status,
				rawStatus: f.Status,
				isStaged:  false,
			})
		}

		if len(files) == 0 {
			items = []gitTreeItem{{isHeader: true, headerText: "No changes detected"}}
		}

		m.gitItems = items
	} else {
		files, err := git.GetStatus(m.repoPath, m.pathFilter)
		if err != nil {
			m.gitItems = nil
			return
		}

		var staged []gitTreeItem
		var unstaged []gitTreeItem

		for _, f := range files {
			if f.Staged {
				staged = append(staged, gitTreeItem{
					path:      f.Path,
					oldPath:   f.OldPath,
					status:    string(f.Status[0]) + " ",
					rawStatus: f.Status,
					isStaged:  true,
				})
			}
			if f.Unstaged && !m.gitStagedOnly {
				status := " " + string(f.Status[1])
				if f.Status == "??" {
					status = "??"
				}
				unstaged = append(unstaged, gitTreeItem{
					path:      f.Path,
					oldPath:   f.OldPath,
					status:    status,
					rawStatus: f.Status,
					isStaged:  false,
				})
			}
		}

		var items []gitTreeItem
		if len(staged) > 0 {
			items = append(items, gitTreeItem{isHeader: true, headerText: "Staged Changes"})
			items = append(items, staged...)
		}
		if len(unstaged) > 0 && !m.gitStagedOnly {
			items = append(items, gitTreeItem{isHeader: true, headerText: "Unstaged Changes"})
			items = append(items, unstaged...)
		}

		if len(items) == 0 {
			items = append(items, gitTreeItem{isHeader: true, headerText: "No changes in repository"})
		}

		m.gitItems = items
	}

	if m.gitCursorY >= len(m.gitItems) {
		m.gitCursorY = len(m.gitItems) - 1
	}
	if m.gitCursorY < 0 {
		m.gitCursorY = 0
	}
}

func (m *model) loadGitFileDiff(idx int) error {
	if idx < 0 || idx >= len(m.gitItems) {
		return fmt.Errorf("file index out of bounds")
	}
	item := m.gitItems[idx]
	if item.isHeader {
		return fmt.Errorf("cannot diff a header")
	}

	m.gitSelectedFileIdx = idx

	beforeFile := item.path
	if item.oldPath != "" {
		beforeFile = item.oldPath
	}
	afterFile := item.path

	var srcBytes, dstBytes []byte
	var err error

	isConflict := strings.Contains(item.rawStatus, "U") || item.rawStatus == "AA" || item.rawStatus == "DD"

	if isConflict {
		srcBytes, err = git.GetContent(m.repoPath, beforeFile, "HEAD")
		if err != nil {
			return err
		}
		dstBytes, err = git.GetContent(m.repoPath, afterFile, "")
		if err != nil {
			return err
		}
	} else if m.refA != "" {
		srcBytes, err = git.GetContent(m.repoPath, beforeFile, m.refA)
		if err != nil {
			return err
		}
		dstBytes, err = git.GetContent(m.repoPath, afterFile, m.refB)
		if err != nil {
			return err
		}
	} else if item.isStaged {
		srcBytes, err = git.GetContent(m.repoPath, beforeFile, "HEAD")
		if err != nil {
			return err
		}
		dstBytes, err = git.GetContent(m.repoPath, afterFile, ":")
		if err != nil {
			return err
		}
	} else {
		srcBytes, err = git.GetContent(m.repoPath, beforeFile, ":")
		if err != nil || len(srcBytes) == 0 {
			srcBytes, err = git.GetContent(m.repoPath, beforeFile, "HEAD")
			if err != nil {
				return err
			}
		}
		dstBytes, err = git.GetContent(m.repoPath, afterFile, "")
		if err != nil {
			return err
		}
	}

	if isBinary(srcBytes) || isBinary(dstBytes) {
		m.srcFile = beforeFile
		m.dstFile = afterFile
		m.srcLines = []string{"[Binary File Diff Not Supported]"}
		m.dstLines = []string{"[Binary File Diff Not Supported]"}
		m.lineAlignment = []serialize.LineAlignmentPair{{LeftLine: 0, RightLine: 0}}
		m.srcHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
		m.dstHighlights = &highlights{spans: map[int][]span{}, tinted: map[int]actionKind{}}
		m.folds = nil
		m.rebuildVirtualLines()
		m.srcSyntax = nil
		m.dstSyntax = nil
		m.cursorY = 0
		m.cursorX = 0
		m.scrollY = 0
		return nil
	}

	if len(srcBytes) == 0 && len(dstBytes) == 0 {
		m.setupEmptyPlaceholder()
		return nil
	}

	env, err := computeBytesDiff(srcBytes, dstBytes, beforeFile, afterFile, isConflict)
	if err != nil {
		env = &serialize.Envelope{}
	}

	m.setupDiff(beforeFile, afterFile, srcBytes, dstBytes, env)
	return nil
}

func isBinary(data []byte) bool {
	limit := len(data)
	if limit > 8000 {
		limit = 8000
	}
	for i := 0; i < limit; i++ {
		if data[i] == 0 {
			return true
		}
	}
	return false
}

func hasConflictMarkers(data []byte) bool {
	return bytes.Contains(data, []byte("<<<<<<<")) || bytes.Contains(data, []byte("=======")) || bytes.Contains(data, []byte(">>>>>>>"))
}

func generateLineDiff(srcBytes, dstBytes []byte) *serialize.Envelope {
	alignment := serialize.AlignLines(srcBytes, dstBytes, nil, nil, nil, nil)

	buildLineOffsets := func(data []byte) []uint32 {
		offsets := []uint32{0}
		for i, b := range data {
			if b == '\n' {
				offsets = append(offsets, uint32(i+1))
			}
		}
		return offsets
	}

	offsetsSrc := buildLineOffsets(srcBytes)
	offsetsDst := buildLineOffsets(dstBytes)

	getBounds := func(lineIdx int, offsets []uint32, maxLen int) (uint32, uint32) {
		end := uint32(maxLen)
		if lineIdx+1 < len(offsets) {
			end = offsets[lineIdx+1]
		}
		return offsets[lineIdx], end
	}

	var actions []serialize.Action

	for _, pair := range alignment {
		if pair.RightLine == -1 && pair.LeftLine != -1 {
			start, end := getBounds(pair.LeftLine, offsetsSrc, len(srcBytes))
			actions = append(actions, serialize.Action{
				Action: "delete",
				Node: &serialize.NodeRef{
					StartByte: start,
					EndByte:   end,
				},
			})
		} else if pair.LeftLine == -1 && pair.RightLine != -1 {
			start, end := getBounds(pair.RightLine, offsetsDst, len(dstBytes))
			actions = append(actions, serialize.Action{
				Action: "insert",
				Node: &serialize.NodeRef{
					StartByte: start,
					EndByte:   end,
				},
			})
		}
	}

	return &serialize.Envelope{
		Actions:       actions,
		LineAlignment: alignment,
	}
}

func computeBytesDiff(srcBytes, dstBytes []byte, srcFile, dstFile string, isConflict bool) (*serialize.Envelope, error) {
	if isConflict || hasConflictMarkers(srcBytes) || hasConflictMarkers(dstBytes) {
		return generateLineDiff(srcBytes, dstBytes), nil
	}

	srcAST, err1 := treesitter.Parse(srcBytes, srcFile)
	dstAST, err2 := treesitter.Parse(dstBytes, dstFile)
	if err1 != nil || err2 != nil {
		return generateLineDiff(srcBytes, dstBytes), nil
	}

	matchResult := engine.Match(srcAST, dstAST)
	es := actions.GenerateEditScript(srcAST, dstAST, matchResult.Mappings)
	es = postprocess.Run(es, matchResult.Mappings, srcAST, dstAST)

	return serialize.BuildEnvelope(es, matchResult.Mappings, srcAST, dstAST, srcBytes, dstBytes)
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
	h := m.height - titleBarHeight - statusBarHeight
	if m.inspectOpen {
		h -= inspectPanelHeight
	}
	if m.gitCommitOpen {
		h -= 1 // 1 line for the commit input bar
	}
	if h < 1 {
		h = 1
	}
	return h
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
