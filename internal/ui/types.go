package ui

import (
	"charm.land/bubbles/v2/key"
)

type Model struct {
	Keymap    Keymap
	Tabs      []Tab
	ActiveTab int
}

type Tab struct {
	Title string
	Pane  Pane
}

type Keymap = struct {
	quit, tabSwitch key.Binding
}
