package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

type Pane interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Pane, tea.Cmd)
	View() string
}

type PaneBase struct {
	Width, Height int
	Status        string
	Focus         int
	Keymap        Keymap
	Zones         Zones
}
