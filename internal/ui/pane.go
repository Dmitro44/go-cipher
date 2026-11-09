package ui

import (
	tea "charm.land/bubbletea/v2"
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
}
