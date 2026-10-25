package asympane

const (
	// PanelMargin is the vertical space taken by the tab bar and the gap
	// between it and the pane.
	PanelMargin = 3

	// KeyPanelContentHeight is the number of rows inside the keyring panel:
	// header, blank, key labels + fields, blank, footer.
	KeyPanelContentHeight = 6

	// ActionBarHeight is the source/mode/buttons row plus the hint row.
	ActionBarHeight = 2

	// MinMessagePanelHeight keeps the message panels usable on short terminals.
	MinMessagePanelHeight = 6
)

// Layout places the pane in three horizontal bands: a full-width keyring, two
// side-by-side message panels, and a compact action bar.
type Layout struct {
	WindowWidth  int
	WindowHeight int

	// PanelWidth is the total rendered width shared by all bands.
	PanelWidth int

	// Heights are content heights (borders are added by the styles).
	KeyPanelHeight     int
	MessagePanelHeight int

	// LeftPanelWidth and RightPanelWidth are total rendered widths.
	LeftPanelWidth  int
	RightPanelWidth int

	// Internal component dimensions.
	KeyColumnWidth   int
	KeyFieldWidth    int
	TextareaWidth    int
	TextareaHeight   int
	FilepickerHeight int
}

func NewLayout(width, height int) Layout {
	l := Layout{
		WindowWidth:  width,
		WindowHeight: height,
		PanelWidth:   max(width, 60),
	}

	// Vertical budget: tab bar (PanelMargin) + keyring + message panels +
	// action bar. Each bordered panel adds two rows.
	totalH := height - PanelMargin
	keyRendered := KeyPanelContentHeight + 2
	l.KeyPanelHeight = KeyPanelContentHeight
	l.MessagePanelHeight = max(totalH-keyRendered-ActionBarHeight-2, MinMessagePanelHeight)

	l.LeftPanelWidth = l.PanelWidth / 2
	l.RightPanelWidth = l.PanelWidth - l.LeftPanelWidth

	// Keyring content width is PanelWidth - border(2) - padding(2).
	keyContent := l.PanelWidth - 4
	l.KeyColumnWidth = (keyContent - 1) / 2
	l.KeyFieldWidth = l.KeyColumnWidth - 2

	l.TextareaWidth = l.LeftPanelWidth - 4
	l.TextareaHeight = l.MessagePanelHeight - 1
	l.FilepickerHeight = l.MessagePanelHeight - 1

	return l
}
