package cipherpane

import "github.com/charmbracelet/bubbles/key"

// statusMsg clears the pane Status after a short delay
type statusMsg string

type (
	Mode      int
	InputMode int
)

const (
	ManualInput InputMode = iota
	File
)

const (
	Encrypt Mode = iota
	Decrypt
)

const (
	FocusInputMode int = iota
	FocusFilepicker
	FocusMethod
	FocusMode
	FocusKey
	FocusInput
	FocusBtn
	FocusSaveBtn
	FocusCount
)

type PaneKeymap = struct {
	next, prev, copy, clearInput, clearKey, genKey key.Binding
}
