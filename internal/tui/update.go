package tui

import tea "github.com/charmbracelet/bubbletea"

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.scrollY = clamp(m.scrollY, 0, m.maxScrollY())
		m.scrollXLeft = clamp(m.scrollXLeft, 0, maxScrollX(m.srcLines, m.textWidth()))
		m.scrollXRight = clamp(m.scrollXRight, 0, maxScrollX(m.dstLines, m.textWidth()))
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}
