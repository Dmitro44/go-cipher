package asympane

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

// newTextarea builds a borderless editor. The surrounding message panel draws
// the border and highlights it while the editor is focused.
func newTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Cursor.Style = cursorStyle
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = mutedStyle
	ta.BlurredStyle.Placeholder = mutedStyle
	ta.Focus()

	return ta
}

func newOutputTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Cursor.Style = cursorStyle
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = mutedStyle
	ta.BlurredStyle.Placeholder = mutedStyle

	return ta
}

func newTextinput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Cursor.Style = cursorStyle
	return ti
}

func newFilepicker() filepicker.Model {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".txt", ".md", ".go"}
	dir, _ := os.Getwd()
	fp.CurrentDirectory = dir
	fp.FileAllowed = true
	fp.DirAllowed = false
	return fp
}

func newPaneKeymap() PaneKeymap {
	return PaneKeymap{
		next: key.NewBinding(
			key.WithKeys("tab"),
		),
		prev: key.NewBinding(
			key.WithKeys("shift+tab"),
		),
		copyOutput: key.NewBinding(
			key.WithKeys("ctrl+d"),
		),
		clearInput: key.NewBinding(
			key.WithKeys("alt+d"),
		),
		clearKey: key.NewBinding(
			key.WithKeys("ctrl+k"),
		),
		generate: key.NewBinding(
			key.WithKeys("ctrl+g"),
		),
		reveal: key.NewBinding(
			key.WithKeys("ctrl+r"),
		),
		nextCandidate: key.NewBinding(
			key.WithKeys("ctrl+n"),
		),
	}
}
