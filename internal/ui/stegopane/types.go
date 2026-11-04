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

	// Busy is set while a decode/embed/extract runs off the UI goroutine.
	Busy bool
	// KittyOn records that the previews were switched to the Kitty graphics
	// protocol after the terminal probe confirmed support.
	KittyOn bool

	StatusError bool
	PaneKeymap  PaneKeymap
}

// loadMsg carries a decoded source image back to the pane.
type loadMsg struct {
	path string
	img  image.Image
	err  error
}

// embedMsg carries the finished container and its previews back to the pane.
type embedMsg struct {
	img       image.Image
	container image.Image
	diff      image.Image
	stats     stego.Stats
	path      string
	size      int
	err       error
}

// extractMsg carries the recovered payload back to the pane.
type extractMsg struct {
	img     image.Image
	payload []byte
	err     error
}
