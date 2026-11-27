package signpane

import "charm.land/bubbles/v2/key"

// statusMsg clears the pane Status after a short delay. It carries the text it
// was scheduled for so an older timer cannot wipe a newer message.
type statusMsg string

type (
	Mode      int
	InputMode int
	Verdict   int
)

const (
	ManualInput InputMode = iota
	FileInput
)

const (
	Sign Mode = iota
	Verify
)

const (
	VerdictNone Verdict = iota
	VerdictValid
	VerdictInvalid
)

const (
	FocusParameters int = iota
	FocusGenerate
	FocusPublicKey
	FocusPrivateKey
	FocusSource
	FocusMode
	FocusMessage
	FocusSignature
	FocusRun
	FocusSave
	FocusCount
)

type PaneKeymap = struct {
	next, prev       key.Binding
	prepareVerify    key.Binding
	clearMessage     key.Binding
	clearKey         key.Binding
	generate, reveal key.Binding
}
