package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/HarshK97/diffmantic/internal/git"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

	// If git status tree is open, intercept input
	if m.gitMode && m.gitTreeOpen {
		// Clear conflict warning on any input
		m.conflictWarning = ""

		switch keyStr {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "esc", "t":
			m.gitTreeOpen = false
			return m, nil
		case "j", "down":
			m.gitCursorDown()
			return m, nil
		case "k", "up":
			m.gitCursorUp()
			return m, nil
		case "enter":
			if len(m.gitItems) > 0 && m.gitCursorY >= 0 && m.gitCursorY < len(m.gitItems) {
				if !m.gitItems[m.gitCursorY].isHeader {
					_ = m.loadGitFileDiff(m.gitCursorY)
					m.gitTreeOpen = false // Close on select to read the diff
				}
			}
			return m, nil
		case "s":
			if m.refA != "" {
				return m, nil
			}
			if len(m.gitItems) > 0 && m.gitCursorY >= 0 && m.gitCursorY < len(m.gitItems) {
				item := m.gitItems[m.gitCursorY]
				if !item.isHeader && !item.isStaged {
					isConflict := strings.Contains(item.rawStatus, "U") || item.rawStatus == "AA" || item.rawStatus == "DD"
					hasMarkers := false
					if data, err := os.ReadFile(filepath.Join(m.repoPath, item.path)); err == nil {
						hasMarkers = hasConflictMarkers(data)
					}

					if isConflict || hasMarkers {
						m.conflictWarning = "resolve conflicts before staging"
						return m, nil
					}

					_ = git.StageFile(m.repoPath, item.path)
					m.refreshGitStatus()
					m.syncGitCursorAndDiff()
				}
			}
			return m, nil
		case "u":
			if m.refA != "" {
				return m, nil
			}
			if len(m.gitItems) > 0 && m.gitCursorY >= 0 && m.gitCursorY < len(m.gitItems) {
				item := m.gitItems[m.gitCursorY]
				if !item.isHeader && item.isStaged {
					_ = git.UnstageFile(m.repoPath, item.path)
					m.refreshGitStatus()
					m.syncGitCursorAndDiff()
				}
			}
			return m, nil
		case "c":
			if m.refA != "" {
				return m, nil
			}
			if !slices.ContainsFunc(m.gitItems, func(it gitTreeItem) bool { return !it.isHeader && it.isStaged }) {
				m.conflictWarning = "no changes staged for commit"
				return m, nil
			}
			m.gitCommitOpen = true
			m.gitCommitInput.Focus()
			m.gitCommitInput.SetValue("")
			return m, nil
		}
		return m, nil
	}

	// Dismiss help modal on key press unless it's a quit key.
	if m.helpOpen {
		switch keyStr {
		case "q", "ctrl+c":
			return m, tea.Quit
		default:
			m.helpOpen = false
			return m, nil
		}
	}

	// Handle second key of a folding shortcut (e.g. za, zo).
	if m.pendingZ {
		m.pendingZ = false
		m.digitBuffer = ""
		switch keyStr {
		case "a":
			m.toggleFoldAt(m.cursorY)
		case "o":
			m.openFoldAt(m.cursorY)
		case "c":
			m.closeFoldAt(m.cursorY)
		case "R":
			m.openAllFolds()
		case "M":
			m.closeAllFolds()
		}
		m.updateInspectActions()
		return m, nil
	}

	count := 1
	if len(m.digitBuffer) > 0 {
		if c, err := strconv.Atoi(m.digitBuffer); err == nil {
			count = c
		}
	}

	// Handle pending bracket navigation (e.g. ]h, [h, ]c, [c, ]], [[).
	if m.pendingBracket != "" {
		bracket := m.pendingBracket
		m.pendingBracket = ""
		m.digitBuffer = ""
		switch keyStr {
		case "h", "c", bracket:
			switch bracket {
			case "]":
				m.jumpToNextSpan(count)
			case "[":
				m.jumpToPrevSpan(count)
			}
		}
		m.updateInspectActions()
		return m, nil
	}

	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		// Vim counts don't start with 0, so ignore it if the buffer is empty.
		if keyStr[0] != '0' || len(m.digitBuffer) > 0 {
			m.digitBuffer += keyStr
			return m, nil
		}
	}

	resetBuffer := true

	switch keyStr {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit

	case "j", "down":
		m.cursorY = clamp(m.cursorY+count, 0, len(m.virtualLines)-1)
		m.clampCursor()
		m.keepCursorInViewport()
	case "k", "up":
		m.cursorY = clamp(m.cursorY-count, 0, len(m.virtualLines)-1)
		m.clampCursor()
		m.keepCursorInViewport()
	case "ctrl+d", "pgdown":
		half := m.contentHeight() / 2
		m.cursorY = clamp(m.cursorY+(half*count), 0, len(m.virtualLines)-1)
		m.scrollY = clamp(m.scrollY+(half*count), 0, m.maxScrollY())
		m.clampCursor()
		m.keepCursorInViewport()
	case "ctrl+u", "pgup":
		half := m.contentHeight() / 2
		m.cursorY = clamp(m.cursorY-(half*count), 0, len(m.virtualLines)-1)
		m.scrollY = clamp(m.scrollY-(half*count), 0, m.maxScrollY())
		m.clampCursor()
		m.keepCursorInViewport()
	case "g", "home":
		m.cursorY = 0
		m.cursorX = 0
		m.scrollY = 0
		m.scrollXLeft = 0
		m.scrollXRight = 0
	case "G", "end":
		m.cursorY = len(m.virtualLines) - 1
		m.cursorX = 0
		m.scrollY = m.maxScrollY()
		m.keepCursorInViewport()

	case "n":
		if m.searchQuery != "" && len(m.searchMatches) > 0 {
			m.searchMatchIdx = (m.searchMatchIdx + count) % len(m.searchMatches)
			m.jumpToSearchMatch()
		} else {
			for i := 0; i < count; i++ {
				m.cursorY = m.nextChange()
			}
			m.cursorX = 0
			m.scrollY = clamp(m.cursorY-m.contentHeight()/2, 0, m.maxScrollY())
			m.keepCursorInViewport()
		}
	case "N":
		if m.searchQuery != "" && len(m.searchMatches) > 0 {
			m.searchMatchIdx = (m.searchMatchIdx - count) % len(m.searchMatches)
			if m.searchMatchIdx < 0 {
				m.searchMatchIdx += len(m.searchMatches)
			}
			m.jumpToSearchMatch()
		} else {
			for i := 0; i < count; i++ {
				m.cursorY = m.prevChange()
			}
			m.cursorX = 0
			m.scrollY = clamp(m.cursorY-m.contentHeight()/2, 0, m.maxScrollY())
			m.keepCursorInViewport()
		}

	case "z":
		m.pendingZ = true
		resetBuffer = false

	case "]":
		m.pendingBracket = "]"
		resetBuffer = false

	case "[":
		m.pendingBracket = "["
		resetBuffer = false

	case "h", "left":
		m.cursorX -= count
		m.clampCursor()
		m.keepCursorInViewport()
	case "l", "right":
		m.cursorX += count
		m.clampCursor()
		m.keepCursorInViewport()

	case "0":
		// Move to the start of the line on '0' (if we're not typing a count).
		m.cursorX = 0
		m.keepCursorInViewport()

	case "$":
		runes := m.lineVisualRunes(m.cursorY)
		if len(runes) > 0 {
			m.cursorX = len(runes) - 1
		} else {
			m.cursorX = 0
		}
		m.keepCursorInViewport()

	case "^":
		runes := m.lineVisualRunes(m.cursorY)
		m.cursorX = 0
		for i, r := range runes {
			if r != ' ' {
				m.cursorX = i
				break
			}
		}
		m.keepCursorInViewport()

	case "w":
		for i := 0; i < count; i++ {
			m.moveWordForward()
		}
		m.keepCursorInViewport()

	case "b":
		for i := 0; i < count; i++ {
			m.moveWordBackward()
		}
		m.keepCursorInViewport()

	case "e":
		for i := 0; i < count; i++ {
			m.moveWordEnd()
		}
		m.keepCursorInViewport()

	case "tab":
		if m.activePane == "left" {
			m.activePane = "right"
		} else {
			m.activePane = "left"
		}
		m.clampCursor()
		m.keepCursorInViewport()

	case "t":
		if m.gitMode {
			m.gitTreeOpen = true
		}

	case "i":
		m.inspectOpen = !m.inspectOpen
		m.keepCursorInViewport()

	case "enter":
		m.jumpToMoveCounterpart()

	case "/":
		m.searchActive = true
		m.textinput.Focus()
		m.textinput.SetValue(m.searchQuery)
		m.textinput.CursorEnd()

	case "?":
		m.helpOpen = true

	default:
		// Keep the buffer if we're still typing a count.
		if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
			resetBuffer = false
		}
	}

	if resetBuffer {
		m.digitBuffer = ""
	}

	m.updateInspectActions()

	return m, nil
}

func (m *model) toggleFoldAt(virtualIdx int) {
	fi := foldAtVirtual(m.virtualLines, m.folds, virtualIdx)
	if fi < 0 {
		return
	}
	wasOpen := m.folds[fi].open
	m.folds[fi].open = !wasOpen
	m.rebuildVirtualLines()

	if wasOpen {
		for i, vl := range m.virtualLines {
			if vl.foldIdx == fi {
				m.cursorY = i
				break
			}
		}
	} else {
		for i, vl := range m.virtualLines {
			if vl.alignedRow == m.folds[fi].startLine {
				m.cursorY = i
				break
			}
		}
	}
	m.clampCursor()
	m.keepCursorInViewport()
}

func (m *model) openFoldAt(virtualIdx int) {
	fi := foldAtVirtual(m.virtualLines, m.folds, virtualIdx)
	if fi < 0 {
		return
	}
	if m.folds[fi].open {
		return
	}
	m.folds[fi].open = true
	m.rebuildVirtualLines()
	for i, vl := range m.virtualLines {
		if vl.alignedRow == m.folds[fi].startLine {
			m.cursorY = i
			break
		}
	}
	m.clampCursor()
	m.keepCursorInViewport()
}

func (m *model) closeFoldAt(virtualIdx int) {
	fi := foldAtVirtual(m.virtualLines, m.folds, virtualIdx)
	if fi < 0 {
		return
	}
	if !m.folds[fi].open {
		return
	}
	m.folds[fi].open = false
	m.rebuildVirtualLines()
	for i, vl := range m.virtualLines {
		if vl.foldIdx == fi {
			m.cursorY = i
			break
		}
	}
	m.clampCursor()
	m.keepCursorInViewport()
}

func (m *model) openAllFolds() {
	for i := range m.folds {
		m.folds[i].open = true
	}
	m.rebuildVirtualLines()
	m.scrollY = clamp(m.scrollY, 0, m.maxScrollY())
}

func (m *model) closeAllFolds() {
	for i := range m.folds {
		m.folds[i].open = false
	}
	m.rebuildVirtualLines()
	m.scrollY = clamp(m.scrollY, 0, m.maxScrollY())
}

func (m model) nextChange() int {
	if len(m.vchanges) == 0 {
		return m.cursorY
	}
	idx, _ := slices.BinarySearch(m.vchanges, m.cursorY+1)
	if idx < len(m.vchanges) {
		return m.vchanges[idx]
	}
	return m.vchanges[0]
}

func (m model) prevChange() int {
	if len(m.vchanges) == 0 {
		return m.cursorY
	}
	idx, _ := slices.BinarySearch(m.vchanges, m.cursorY)
	idx--
	if idx >= 0 {
		return m.vchanges[idx]
	}
	return m.vchanges[len(m.vchanges)-1]
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_'
}

func (m *model) moveWordForward() {
	runes := m.lineVisualRunes(m.cursorY)
	if len(runes) == 0 || m.cursorX >= len(runes)-1 {
		m.moveToNextLineStart()
		return
	}

	idx := m.cursorX
	curIsWord := isWordChar(runes[idx])

	for idx < len(runes) && isWordChar(runes[idx]) == curIsWord && runes[idx] != ' ' {
		idx++
	}
	for idx < len(runes) && runes[idx] == ' ' {
		idx++
	}

	if idx >= len(runes) {
		m.moveToNextLineStart()
	} else {
		m.cursorX = idx
	}
}

func (m *model) moveToNextLineStart() {
	if m.cursorY < len(m.virtualLines)-1 {
		m.cursorY++
		m.cursorX = 0
		runes := m.lineVisualRunes(m.cursorY)
		for i, r := range runes {
			if r != ' ' {
				m.cursorX = i
				break
			}
		}
	}
}

func (m *model) moveWordBackward() {
	runes := m.lineVisualRunes(m.cursorY)
	if len(runes) == 0 || m.cursorX <= 0 {
		m.moveToPrevLineEnd()
		return
	}

	idx := m.cursorX - 1
	for idx >= 0 && runes[idx] == ' ' {
		idx--
	}
	if idx < 0 {
		m.moveToPrevLineEnd()
		return
	}

	isWord := isWordChar(runes[idx])
	for idx >= 0 && isWordChar(runes[idx]) == isWord && runes[idx] != ' ' {
		idx--
	}
	m.cursorX = idx + 1
}

func (m *model) moveToPrevLineEnd() {
	if m.cursorY > 0 {
		m.cursorY--
		runes := m.lineVisualRunes(m.cursorY)
		if len(runes) > 0 {
			m.cursorX = len(runes) - 1
		} else {
			m.cursorX = 0
		}
	}
}

func (m *model) moveWordEnd() {
	runes := m.lineVisualRunes(m.cursorY)
	if len(runes) == 0 || m.cursorX >= len(runes)-1 {
		m.moveToNextLineWordEnd()
		return
	}

	idx := m.cursorX + 1
	for idx < len(runes) && runes[idx] == ' ' {
		idx++
	}
	if idx >= len(runes) {
		m.moveToNextLineWordEnd()
		return
	}

	isWord := isWordChar(runes[idx])
	for idx < len(runes) && isWordChar(runes[idx]) == isWord && runes[idx] != ' ' {
		idx++
	}
	m.cursorX = idx - 1
}

func (m *model) moveToNextLineWordEnd() {
	if m.cursorY < len(m.virtualLines)-1 {
		m.cursorY++
		runes := m.lineVisualRunes(m.cursorY)
		if len(runes) > 0 {
			idx := 0
			for idx < len(runes) && runes[idx] == ' ' {
				idx++
			}
			if idx < len(runes) {
				isWord := isWordChar(runes[idx])
				for idx < len(runes) && isWordChar(runes[idx]) == isWord && runes[idx] != ' ' {
					idx++
				}
				m.cursorX = idx - 1
			} else {
				m.cursorX = 0
			}
		} else {
			m.cursorX = 0
		}
	}
}

func (m *model) gitCursorDown() {
	if len(m.gitItems) == 0 {
		return
	}
	originalY := m.gitCursorY
	for {
		m.gitCursorY++
		if m.gitCursorY >= len(m.gitItems) {
			m.gitCursorY = len(m.gitItems) - 1
			break
		}
		if !m.gitItems[m.gitCursorY].isHeader {
			break
		}
	}
	if m.gitItems[m.gitCursorY].isHeader {
		m.gitCursorY = originalY
	}
}

func (m *model) gitCursorUp() {
	if len(m.gitItems) == 0 {
		return
	}
	originalY := m.gitCursorY
	for {
		m.gitCursorY--
		if m.gitCursorY < 0 {
			m.gitCursorY = 0
			break
		}
		if !m.gitItems[m.gitCursorY].isHeader {
			break
		}
	}
	if m.gitItems[m.gitCursorY].isHeader {
		m.gitCursorY = originalY
	}
}

func (m *model) syncGitCursorAndDiff() {
	if len(m.gitItems) == 0 {
		m.setupEmptyPlaceholder()
		return
	}
	if m.gitCursorY >= len(m.gitItems) {
		m.gitCursorY = len(m.gitItems) - 1
	}
	for m.gitCursorY >= 0 && m.gitCursorY < len(m.gitItems) && m.gitItems[m.gitCursorY].isHeader {
		m.gitCursorY--
	}
	if m.gitCursorY >= 0 && m.gitCursorY < len(m.gitItems) && !m.gitItems[m.gitCursorY].isHeader {
		_ = m.loadGitFileDiff(m.gitCursorY)
	} else {
		m.setupEmptyPlaceholder()
	}
}

func (m *model) currentLineSpansAndText() ([]span, string, *int) {
	if m.cursorY < 0 || m.cursorY >= len(m.virtualLines) {
		return nil, "", nil
	}
	vl := m.virtualLines[m.cursorY]
	if vl.foldIdx >= 0 {
		return nil, "", nil
	}

	var lineIdx int
	var lines []string
	var hl *highlights
	var scrollX *int

	if m.activePane == "left" {
		lineIdx = vl.leftLine
		lines = m.srcLines
		hl = m.srcHighlights
		scrollX = &m.scrollXLeft
	} else {
		lineIdx = vl.rightLine
		lines = m.dstLines
		hl = m.dstHighlights
		scrollX = &m.scrollXRight
	}

	if lineIdx < 0 || lineIdx >= len(lines) || hl == nil {
		return nil, "", nil
	}

	spans := hl.spans[lineIdx]
	return spans, lines[lineIdx], scrollX
}

type visualSpan struct {
	startCol int
	endCol   int
	kind     actionKind
}

func getVisualSpans(spans []span, rawLine string) []visualSpan {
	if len(spans) == 0 {
		return nil
	}
	expanded, byteToVisual := expandLine(rawLine)
	runeLen := len([]rune(expanded))

	var vSpans []visualSpan
	for _, s := range spans {
		sc := 0
		if s.startCol < len(byteToVisual) {
			sc = byteToVisual[s.startCol]
		}
		ec := runeLen
		if s.endCol < len(byteToVisual) {
			ec = byteToVisual[s.endCol]
		}
		if sc >= 0 && ec > sc {
			vSpans = append(vSpans, visualSpan{startCol: sc, endCol: ec, kind: s.kind})
		}
	}
	slices.SortFunc(vSpans, func(a, b visualSpan) int {
		if a.startCol != b.startCol {
			return a.startCol - b.startCol
		}
		return a.endCol - b.endCol
	})
	return vSpans
}

func (m *model) jumpSpan(dir int, count int) {
	spans, rawLine, scrollXPtr := m.currentLineSpansAndText()
	if len(spans) == 0 || scrollXPtr == nil {
		return
	}
	vSpans := getVisualSpans(spans, rawLine)
	if len(vSpans) == 0 {
		return
	}

	textW := m.textWidth()
	curScroll := *scrollXPtr

	var target visualSpan
	found := false

	if dir > 0 {
		for _, vs := range vSpans {
			if vs.startCol > curScroll || (vs.startCol == curScroll && m.cursorX < vs.startCol) {
				target = vs
				found = true
				break
			}
		}
		if !found {
			target = vSpans[0]
		}
	} else {
		for i := len(vSpans) - 1; i >= 0; i-- {
			vs := vSpans[i]
			if vs.startCol < curScroll || (vs.startCol == curScroll && m.cursorX > vs.startCol) {
				target = vs
				found = true
				break
			}
		}
		if !found {
			target = vSpans[len(vSpans)-1]
		}
	}

	if count > 1 {
		idx := 0
		for i, vs := range vSpans {
			if vs.startCol == target.startCol && vs.endCol == target.endCol {
				idx = i
				break
			}
		}
		targetIdx := (idx + dir*(count-1)) % len(vSpans)
		if targetIdx < 0 {
			targetIdx += len(vSpans)
		}
		target = vSpans[targetIdx]
	}

	expanded, _ := expandLine(rawLine)
	runeLen := len([]rune(expanded))
	maxScroll := max(0, runeLen-textW)

	midCol := (target.startCol + target.endCol) / 2
	desiredScroll := midCol - (textW / 2)
	*scrollXPtr = clamp(desiredScroll, 0, maxScroll)

	m.cursorX = target.startCol
	m.keepCursorInViewport()
}

func (m *model) jumpToNextSpan(count int) { m.jumpSpan(1, count) }
func (m *model) jumpToPrevSpan(count int) { m.jumpSpan(-1, count) }
