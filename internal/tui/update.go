package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/HarshK97/diffmantic/internal/git"
)

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
		if m.gitCommitOpen {
			var cmd tea.Cmd
			m.gitCommitInput, cmd = m.gitCommitInput.Update(msg)

			switch msg.String() {
			case "enter":
				msgVal := m.gitCommitInput.Value()
				m.gitCommitOpen = false
				m.gitCommitInput.Blur()
				if strings.TrimSpace(msgVal) != "" {
					_ = git.Commit(m.repoPath, msgVal)
					m.refreshGitStatus()
					if len(m.gitItems) > 0 {
						firstIdx := -1
						for idx, it := range m.gitItems {
							if !it.isHeader {
								firstIdx = idx
								break
							}
						}
						if firstIdx != -1 {
							m.gitCursorY = firstIdx
							_ = m.loadGitFileDiff(firstIdx)
						} else {
							m.setupEmptyPlaceholder()
						}
					} else {
						m.setupEmptyPlaceholder()
					}
				}
			case "esc":
				m.gitCommitOpen = false
				m.gitCommitInput.Blur()
			}
			return m, cmd
		}
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
