package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	zone "github.com/lrstanley/bubblezone"
)

// Zones gives one rendered view its own mouse hit areas.
//
// Every pane owns a Zones value whose scope is unique among the panes rendered
// at the same time. View() wraps interactive fragments in Mark(); Update() asks
// Clicked/InBounds about the same names. Bubblezone resolves screen coordinates,
// so a pane never calculates offsets and keeps working when it is moved or
// nested inside a different layout.
//
// Adding mouse support to a new pane (for example the lab 8 image pane):
//   - create it with ui.NewZones("lab8") in PaneBase.Zones;
//   - wrap buttons, fields and image regions in Zones.Mark in View();
//   - handle tea.MouseMsg in Update and dispatch with Zones.Clicked/InBounds;
//   - use Zones.Pos when the position inside a region matters, such as mapping
//     a click on an image back to image coordinates.
//
// Only the root model scans the final view. Panes must never scan their own
// output, otherwise zones from sibling panes are lost.
// TabBarHeight is the number of screen rows the root model renders above a
// pane: the bordered tab row.
const TabBarHeight = 3

type Zones struct {
	scope string
}

// NewZones creates a zone namespace. The scope must stay stable for the life of
// the pane and be unique among concurrently visible panes.
func NewZones(scope string) Zones {
	return Zones{scope: scope}
}

// ID returns the application-wide identifier for name in this scope.
func (z Zones) ID(name string) string {
	return z.scope + "/" + name
}

// Mark wraps content in the hit area named name. Multi-line content is padded
// to a rectangle because Bubblezone rejects a box whose last line ends left of
// its first line, which happens for editors and lists padded with blank lines.
func (z Zones) Mark(name, content string) string {
	return zone.Mark(z.ID(name), rectangle(content))
}

func rectangle(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 2 {
		return content
	}

	width := 0
	for _, line := range lines {
		width = max(width, lipgloss.Width(line))
	}
	for i, line := range lines {
		lines[i] = line + strings.Repeat(" ", width-lipgloss.Width(line))
	}
	return strings.Join(lines, "\n")
}

// InBounds reports whether msg happened inside the named zone.
func (z Zones) InBounds(name string, msg tea.MouseMsg) bool {
	return zone.Get(z.ID(name)).InBounds(msg)
}

// Clicked reports whether msg is a completed left-button click inside the named
// zone. Acting on release keeps the start of a drag from pressing buttons.
func (z Zones) Clicked(name string, msg tea.MouseMsg) bool {
	return IsLeftClick(msg) && z.InBounds(name, msg)
}

// Pos returns msg coordinates relative to the named zone, with (0, 0) at its
// top-left cell. It returns (-1, -1) when msg is outside the zone.
func (z Zones) Pos(name string, msg tea.MouseMsg) (x, y int) {
	return zone.Get(z.ID(name)).Pos(msg)
}

// IsLeftClick reports whether msg is the release of a primary-button click.
func IsLeftClick(msg tea.MouseMsg) bool {
	return msg.Action == tea.MouseActionRelease && msg.Button == tea.MouseButtonLeft
}

// WheelDirection returns -1 for wheel up, 1 for wheel down, and 0 for any
// other event.
func WheelDirection(msg tea.MouseMsg) int {
	if msg.Action != tea.MouseActionPress {
		return 0
	}
	switch msg.Button {
	case tea.MouseButtonWheelUp:
		return -1
	case tea.MouseButtonWheelDown:
		return 1
	}
	return 0
}

// Scan registers every zone in the final application view and strips the
// markers before the view reaches the terminal.
func Scan(view string) string {
	return zone.Scan(view)
}
