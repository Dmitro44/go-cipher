package hashpane

import (
	"charm.land/bubbles/v2/key"
)

type statusMsg string

type InputMode int

const (
	ManualInput InputMode = iota
	FileInput
)

const (
	FocusAlgorithm int = iota
	FocusInput
	FocusExpected
	FocusSource
	FocusHash
	FocusAvalanche
	FocusSave
	FocusCount
)

type PaneKeymap = struct {
	next, prev             key.Binding
	copyDigest, clearInput key.Binding
}
