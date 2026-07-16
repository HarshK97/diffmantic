package tui

import (
	"strconv"

	tea "github.com/charmbracelet/bubbletea"
)

func (m model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	keyStr := msg.String()

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
		m.scrollY = clamp(m.scrollY+count, 0, m.maxScrollY())
	case "k", "up":
		m.scrollY = clamp(m.scrollY-count, 0, m.maxScrollY())
	case "ctrl+d", "pgdown":
		half := m.contentHeight() / 2
		m.scrollY = clamp(m.scrollY+(half*count), 0, m.maxScrollY())
	case "ctrl+u", "pgup":
		half := m.contentHeight() / 2
		m.scrollY = clamp(m.scrollY-(half*count), 0, m.maxScrollY())
	case "g", "home":
		m.scrollY = 0
		m.scrollXLeft = 0
		m.scrollXRight = 0
	case "G", "end":
		m.scrollY = m.maxScrollY()

	case "h", "left":
		m.scrollXLeft = clamp(m.scrollXLeft-(4*count), 0, maxScrollX(m.srcLines, m.textWidth()))
		m.scrollXRight = clamp(m.scrollXRight-(4*count), 0, maxScrollX(m.dstLines, m.textWidth()))
	case "l", "right":
		m.scrollXLeft = clamp(m.scrollXLeft+(4*count), 0, maxScrollX(m.srcLines, m.textWidth()))
		m.scrollXRight = clamp(m.scrollXRight+(4*count), 0, maxScrollX(m.dstLines, m.textWidth()))

	case "0":
		m.scrollXLeft = 0
		m.scrollXRight = 0

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
