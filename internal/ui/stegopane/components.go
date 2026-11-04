package stegopane

import (
	"image"
	"image/color"
	"math"
	"os"

	"go-cipher/internal/crypto/stego"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
)

func newMessageTextarea() textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = "Type the message to hide..."
	ta.ShowLineNumbers = false
	styles := ta.Styles()
	styles.Cursor.Color = cursorColor
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	styles.Focused.Placeholder = mutedStyle
	styles.Blurred.Placeholder = mutedStyle
	ta.SetStyles(styles)
	ta.Focus()
	return ta
}

func newFilepicker() filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".jpg", ".jpeg", ".png"}
	dir, _ := os.Getwd()
	fp.CurrentDirectory = dir
	fp.FileAllowed = true
	fp.DirAllowed = false
	return fp
}

func newMessageFilepicker() filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".txt", ".md", ".go", ".json"}
	dir, _ := os.Getwd()
	fp.CurrentDirectory = dir
	fp.FileAllowed = true
	fp.DirAllowed = false
	return fp
}

// newPicture returns a pointer so the pane copies (Update and View both work
// on values) keep sharing one model: picture caches its rendered frame inside
// the model, and a copy would rebuild it on every repaint — for a 4K image
// that is megabytes of resampling per keystroke.
func newPicture(kittyID int) *picture.Model {
	model := picture.NewWithConfig(picture.Config{
		KittyID:    kittyID,
		Fit:        picture.FitContain,
		Background: color.Black,
	})
	return &model
}

func newSaveFilename() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "stego.png"
	ti.Prompt = "> "
	ti.SetWidth(60)
	styles := ti.Styles()
	styles.Cursor.Color = cursorColor
	ti.SetStyles(styles)
	return ti
}

func newPaneKeymap() PaneKeymap {
	return PaneKeymap{
		fullscreen:  key.NewBinding(key.WithKeys("f")),
		messageFile: key.NewBinding(key.WithKeys("ctrl+o")),
		next:        key.NewBinding(key.WithKeys("tab")),
		prev:        key.NewBinding(key.WithKeys("shift+tab")),
		clearMessage: key.NewBinding(
			key.WithKeys("alt+d"),
		),
		extractedToMessage: key.NewBinding(
			key.WithKeys("ctrl+d"),
		),
	}
}

// differenceImage returns a grayscale picture where each pixel is
// |Y(stego) - Y(original)| multiplied by gain. Without the gain the changes
// caused by QIM are too small to see in a terminal preview.
func differenceImage(original, stegoImage image.Image, gain float64) image.Image {
	bounds := original.Bounds()
	diff := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			before := stego.Luma(original.At(bounds.Min.X+x, bounds.Min.Y+y))
			after := stego.Luma(stegoImage.At(bounds.Min.X+x, bounds.Min.Y+y))
			value := math.Round(math.Abs(after-before) * gain)
			level := uint8(min(value, 255))
			diff.SetRGBA(x, y, color.RGBA{R: level, G: level, B: level, A: 255})
		}
	}

	return diff
}
