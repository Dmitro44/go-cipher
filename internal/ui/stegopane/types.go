package stegopane

import (
	"image"

	"go-cipher/internal/crypto/stego"
	"go-cipher/internal/ui"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
)

// statusMsg clears the pane status after a short delay. The text travels with
// the timer so an old timer cannot erase a newer message.
type statusMsg string

type Mode int

const (
	ModeEmbed Mode = iota
	ModeExtract
)

const (
	FocusSource int = iota
	FocusMessage
	FocusMode
	FocusRun
	FocusSave
	FocusCount
)

type PaneKeymap = struct {
	next, prev         key.Binding
	clearMessage       key.Binding
	extractedToMessage key.Binding
}

type StegoPane struct {
	ui.PaneBase

	Message       textarea.Model
	Filepicker    filepicker.Model
	SelectedFile  string
	PickerOpen    bool
	Mode          Mode
	Stats         stego.Stats
	Container     string
	LastExtracted []byte
	HasExtracted  bool

	OriginalImage image.Image
	StegoImage    image.Image
	DiffImage     image.Image

	PreviewOriginal picture.Model
	PreviewStego    picture.Model
	PreviewDiff     picture.Model
	PreviewColumns  int
	PreviewRows     int

	StatusError bool
	PaneKeymap  PaneKeymap
}
