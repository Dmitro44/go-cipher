package asympane

import (
	"os"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
)

// newTextarea builds a borderless editor. The surrounding message panel draws
// the border and highlights it while the editor is focused.
func newTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = placeholder
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

func newOutputTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	styles := ta.Styles()
	styles.Cursor.Color = cursorColor
	styles.Focused.CursorLine = lipgloss.NewStyle()
	styles.Blurred.CursorLine = lipgloss.NewStyle()
	styles.Focused.Placeholder = mutedStyle
	styles.Blurred.Placeholder = mutedStyle
	ta.SetStyles(styles)

	return ta
}

func newTextinput(placeholder string) textinput.Model {
	ti := textinput.New()
	ti.Placeholder = placeholder
	styles := ti.Styles()
	styles.Cursor.Color = cursorColor
	ti.SetStyles(styles)
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
	}
}
