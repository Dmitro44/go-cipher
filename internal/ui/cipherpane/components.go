package cipherpane

import (
	"os"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
)

func newTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	styles := ta.Styles()
	styles.Cursor.Color = cursorColor
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
		copy: key.NewBinding(
			key.WithKeys("ctrl+d"),
		),
		clearInput: key.NewBinding(
			key.WithKeys("alt+d"),
		),
		clearKey: key.NewBinding(
			key.WithKeys("ctrl+k"),
		),
		genKey: key.NewBinding(
			key.WithKeys("ctrl+g"),
		),
	}
}
