package ui

import (
	tea "charm.land/bubbletea/v2"
)

type Pane interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Pane, tea.Cmd)
	View() string
}

// BackgroundPane is an optional interface for panes that must see messages even
// while another tab is open. Terminal capability replies (the Kitty graphics
// probe, for one) arrive once, shortly after start, and would otherwise be
// delivered to whichever tab happens to be active and lost.
type BackgroundPane interface {
	BackgroundUpdate(msg tea.Msg) (Pane, tea.Cmd)
}

type PaneBase struct {
	Width, Height int
	Status        string
	Focus         int
	Keymap        Keymap
}
