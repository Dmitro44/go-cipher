package hashpane

const (
	// PanelMargin is the tab bar plus the newline between it and the pane.
	PanelMargin = 4

	// AlgPanelContentHeight is the header row plus the parameters row.
	AlgPanelContentHeight = 2

	// ActionBarHeight is the controls row plus the hint row.
	ActionBarHeight = 2

	MinMessagePanelHeight = 6
)

// Layout places the pane in three bands: the algorithm panel, two message
// panels (input and digest), and a compact action bar.
type Layout struct {
	WindowWidth  int
	WindowHeight int

	PanelWidth int

	AlgPanelHeight     int
	MessagePanelHeight int

	LeftPanelWidth  int
	RightPanelWidth int

	TextareaWidth    int
	TextareaHeight   int
	FilepickerHeight int
	DigestHeight     int
}

func NewLayout(width, height int) Layout {
	l := Layout{
		WindowWidth:  width,
		WindowHeight: height,
		PanelWidth:   max(width, 60),
	}

	totalH := height - PanelMargin
	algRendered := AlgPanelContentHeight + 2
	l.AlgPanelHeight = AlgPanelContentHeight
	l.MessagePanelHeight = max(totalH-algRendered-ActionBarHeight-2, MinMessagePanelHeight)

	l.LeftPanelWidth = l.PanelWidth / 2
	l.RightPanelWidth = l.PanelWidth - l.LeftPanelWidth

	l.TextareaWidth = l.LeftPanelWidth - 4
	l.TextareaHeight = l.MessagePanelHeight - 1
	l.FilepickerHeight = l.MessagePanelHeight - 1
	l.DigestHeight = max(l.MessagePanelHeight-5, 2)

	return l
}
