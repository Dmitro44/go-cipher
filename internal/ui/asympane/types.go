package asympane

import "github.com/charmbracelet/bubbles/key"

// statusMsg clears the pane Status after a short delay. It carries the text it
// was scheduled for so an older timer cannot wipe a newer message.
type statusMsg string

type (
	Mode      int
	InputMode int
)

const (
	ManualInput InputMode = iota
	FileInput
)

const (
	Encrypt Mode = iota
	Decrypt
)

const (
	FocusKeySize int = iota
	FocusGenerate
	FocusPublicKey
	FocusPrivateKey
	FocusSource
	FocusMode
	FocusInput
	FocusRun
	FocusSave
	FocusCount
)

type PaneKeymap = struct {
	next, prev             key.Binding
	copyOutput, clearInput key.Binding
	clearKey               key.Binding
	generate, reveal       key.Binding
	nextCandidate          key.Binding
}
