package cipherpane

import (
	"os"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
)

func newTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Cursor.Style = cursorStyle
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = focusedBorderStyle
	ta.BlurredStyle.Base = blurredBorderStyle
	ta.Focus()

	return ta
}

func newOutputTextarea(placeholder string) textarea.Model {
	ta := textarea.New()
	ta.Prompt = ""
	ta.Placeholder = placeholder
	ta.ShowLineNumbers = false
	ta.FocusedStyle.Base = focusedBorderStyle
	ta.BlurredStyle.Base = blurredBorderStyle

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
