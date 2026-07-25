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
		if m.searchActive {
			var cmd tea.Cmd
			m.textinput, cmd = m.textinput.Update(msg)

			switch msg.String() {
			case "enter":
				m.searchQuery = m.textinput.Value()
				m.searchActive = false
				m.computeSearchMatches()
				m.jumpToSearchMatch()
			case "esc":
				m.searchActive = false
				m.textinput.Blur()
			default:
				m.searchQuery = m.textinput.Value()
				m.computeSearchMatches()
			}
			return m, cmd
		}
		return m.handleKey(msg)
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}
	return m, nil
}
