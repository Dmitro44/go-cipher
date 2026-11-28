package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

var (
	activeTabStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("212")).
			Foreground(lipgloss.Color("212")).
			Bold(true).
			Padding(0, 0)

	inactiveTabStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240")).
				Foreground(lipgloss.Color("240")).
				Padding(0, 0)
)

func newKeymap() Keymap {
	return Keymap{
		quit: key.NewBinding(
			key.WithKeys("esc", "ctrl+c"),
		),
		tabSwitch: key.NewBinding(
			key.WithKeys("alt+1", "alt+2", "alt+3", "alt+4", "alt+5", "alt+6", "alt+7", "alt+8"),
		),
	}
}

func NewModel(tabs ...Tab) Model {
	return Model{
		Keymap: newKeymap(),
		Tabs:   tabs,
	}
}

func (m Model) Init() tea.Cmd {
	// Every pane is initialised, not only the visible one: a pane may need to
	// probe the terminal at start-up (the image previews ask about the Kitty
	// graphics protocol) and does that from Init.
	cmds := make([]tea.Cmd, 0, len(m.Tabs))
	for i := range m.Tabs {
		cmds = append(cmds, m.Tabs[i].Pane.Init())
	}
	return tea.Batch(cmds...)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		var cmds []tea.Cmd
		for i := range m.Tabs {
			var cmd tea.Cmd
			m.Tabs[i].Pane, cmd = m.Tabs[i].Pane.Update(msg)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.Keymap.quit):
			// If the active pane consumes the key,
			// forward it and keep the app running.
			if consumer, ok := m.Tabs[m.ActiveTab].Pane.(interface {
				ConsumesQuit(tea.KeyMsg) bool
			}); ok && consumer.ConsumesQuit(msg) {
				var cmd tea.Cmd
				m.Tabs[m.ActiveTab].Pane, cmd = m.Tabs[m.ActiveTab].Pane.Update(msg)
				return m, cmd
			}
			return m, tea.Quit
		case key.Matches(msg, m.Keymap.tabSwitch):
			if i := int(msg.String()[4] - '1'); i >= 0 && i < len(m.Tabs) {
				m.ActiveTab = i
			}
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.Tabs[m.ActiveTab].Pane, cmd = m.Tabs[m.ActiveTab].Pane.Update(msg)
	cmds := []tea.Cmd{cmd}

	// Panes that need terminal replies regardless of the open tab get the
	// message too; the rest never see it, so widget messages cannot leak from
	// one pane's components into another's.
	for i := range m.Tabs {
		if i == m.ActiveTab {
			continue
		}
		bg, ok := m.Tabs[i].Pane.(BackgroundPane)
		if !ok {
			continue
		}
		m.Tabs[i].Pane, cmd = bg.BackgroundUpdate(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Model) View() tea.View {
	var tabs []string
	for i, tab := range m.Tabs {
		if i == m.ActiveTab {
			tabs = append(tabs, activeTabStyle.Render(tab.Title))
		} else {
			tabs = append(tabs, inactiveTabStyle.Render(tab.Title))
		}
	}

	view := tea.NewView(lipgloss.JoinHorizontal(0, tabs...) + "\n" + m.Tabs[m.ActiveTab].Pane.View())
	view.AltScreen = true
	return view
}
