package tui

import (
	"sort"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

	// Handle the second key of a folding command.
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
		return m, nil
	}

	if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
		// Vim counts don't start with 0, so ignore it if the buffer is empty.
		if keyStr[0] != '0' || len(m.digitBuffer) > 0 {
			m.digitBuffer += keyStr
			return m, nil
		}
	}

	count := 1
	if len(m.digitBuffer) > 0 {
		if c, err := strconv.Atoi(m.digitBuffer); err == nil {
			count = c
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
		for i := 0; i < count; i++ {
			m.cursorY = m.nextChange()
		}
		m.cursorX = 0
		m.scrollY = clamp(m.cursorY-m.contentHeight()/2, 0, m.maxScrollY())
		m.keepCursorInViewport()
	case "N":
		for i := 0; i < count; i++ {
			m.cursorY = m.prevChange()
		}
		m.cursorX = 0
		m.scrollY = clamp(m.cursorY-m.contentHeight()/2, 0, m.maxScrollY())
		m.keepCursorInViewport()

	case "z":
		m.pendingZ = true
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

	default:
		// Keep the buffer if we're still typing a count.
		if len(keyStr) == 1 && keyStr[0] >= '0' && keyStr[0] <= '9' {
			resetBuffer = false
		}
	}

	if resetBuffer {
		m.digitBuffer = ""
	}

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
			if vl.realLine == m.folds[fi].startLine {
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
		if vl.realLine == m.folds[fi].startLine {
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
	idx := sort.SearchInts(m.vchanges, m.cursorY+1)
	if idx < len(m.vchanges) {
		return m.vchanges[idx]
	}
	return m.vchanges[0]
}

func (m model) prevChange() int {
	if len(m.vchanges) == 0 {
		return m.cursorY
	}
	idx := sort.SearchInts(m.vchanges, m.cursorY) - 1
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
