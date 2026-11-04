package stegopane

import (
	"image"

	"go-cipher/internal/crypto/stego"
	"go-cipher/internal/ui"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
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

// FullscreenMode selects which preview fills the whole pane; None keeps the
// regular layout.
type FullscreenMode int

const (
	FullscreenNone FullscreenMode = iota
	FullscreenOriginal
	FullscreenStego
	FullscreenDiff
)

// PickerTarget says what the open file picker is choosing.
type PickerTarget int

const (
	PickerImage PickerTarget = iota
	PickerMessage
)

const (
	FocusSource int = iota
	FocusMessage
	FocusMode
	FocusRun
	// The focus order follows the action bar from left to right.
	FocusSaveImage
	FocusSave
	FocusCount
)

type PaneKeymap = struct {
	next, prev         key.Binding
	clearMessage       key.Binding
	extractedToMessage key.Binding
	fullscreen         key.Binding
	messageFile        key.Binding
}

type StegoPane struct {
	ui.PaneBase

	Message      textarea.Model
	Filepicker   filepicker.Model
	SelectedFile string
	PickerOpen   bool
	// PickerTarget distinguishes the image picker from the message-file picker.
	PickerTarget  PickerTarget
	Mode          Mode
	Stats         stego.Stats
	Container     string
	LastExtracted []byte
	HasExtracted  bool

	OriginalImage image.Image
	StegoImage    image.Image
	DiffImage     image.Image

	PreviewOriginal *picture.Model
	PreviewStego    *picture.Model
	PreviewDiff     *picture.Model
	PreviewColumns  int
	PreviewRows     int

	// Busy is set while a decode/embed/extract runs off the UI goroutine.
	Busy bool
	// KittyOn records that the previews were switched to the Kitty graphics
	// protocol after the terminal probe confirmed support. When Kitty is off,
	// a helper program translates the images into ANSI colour cells.
	KittyOn bool
	// Fullscreen shows one preview over the whole pane.
	Fullscreen FullscreenMode

	// ShowSavePopup owns the keyboard while the user picks a file name for
	// the container image.
	ShowSavePopup bool
	SaveFilename  textinput.Model

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

// messageLoadedMsg carries the contents of a message file back to the pane.
type messageLoadedMsg struct {
	path string
	text string
	err  error
}

// extractMsg carries the recovered payload back to the pane.
type extractMsg struct {
	img     image.Image
	payload []byte
	err     error
}

// saveContainerMsg reports the result of writing the container image to a
// user-chosen path.
type saveContainerMsg struct {
	path string
	err  error
}
