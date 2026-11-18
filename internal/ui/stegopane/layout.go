package stegopane

const (
	// PanelMargin is the vertical space taken by the tab bar and the gap
	// between it and the pane.
	PanelMargin = 3

	SourceBandHeight  = 2
	ActionBandHeight  = 2
	MessageBandHeight = 8
	FilepickerHeight  = 8
	MinPreviewRows    = 5
)

type Layout struct {
	WindowWidth  int
	WindowHeight int
	PanelWidth   int

	PickerHeight       int
	MessagePanelHeight int
	PreviewHeight      int

	ColumnWidth int
	PreviewRows int
	// FullscreenRows is the cell height of a preview that fills the pane.
	FullscreenRows int

	TextareaWidth  int
	TextareaHeight int
}

func NewLayout(width, height int, pickerOpen bool) Layout {
	l := Layout{
		WindowWidth:  width,
		WindowHeight: height,
		PanelWidth:   max(width, 60),
	}

	l.PickerHeight = 0
	if pickerOpen {
		l.PickerHeight = FilepickerHeight
	}

	l.MessagePanelHeight = MessageBandHeight
	l.TextareaWidth = l.PanelWidth/2 - 4
	l.TextareaHeight = l.MessagePanelHeight - 3

	l.ColumnWidth = (l.PanelWidth - 4) / 3

	available := height - PanelMargin - SourceBandHeight - l.PickerHeight -
		l.MessagePanelHeight - ActionBandHeight - 1
	l.PreviewRows = max(available, MinPreviewRows)
	l.PreviewHeight = l.PreviewRows + 1
	l.FullscreenRows = max(height-PanelMargin-1, MinPreviewRows)

	return l
}
