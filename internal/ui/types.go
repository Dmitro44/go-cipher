package ui

import (
	"github.com/charmbracelet/bubbles/key"
)

type Model struct {
	Keymap    Keymap
	Tabs      []Tab
	ActiveTab int
	Zones     Zones
}

type Tab struct {
	Title string
	Pane  Pane
}

type Keymap = struct {
	quit, tabSwitch key.Binding
}
