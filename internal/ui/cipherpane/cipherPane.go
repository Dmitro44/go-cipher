package cipherpane

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"go-cipher/internal/cipher"
	"go-cipher/internal/ui"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type CipherPane struct {
	ui.PaneBase
	Cipher        cipher.Cipher
	Input         textarea.Model
	Output        textarea.Model
	Key           textinput.Model
	InputMode     InputMode
	Method        int
	Mode          Mode
	Filepicker    filepicker.Model
	SelectedFile  string
	FileConfirmed bool
	SaveFilename  textinput.Model
	ShowSavePopup bool
	PaneKeymap    PaneKeymap
}

func NewCipherPane(c cipher.Cipher) CipherPane {
	return CipherPane{
		Filepicker:   newFilepicker(),
		Focus:        FocusInput,
		PaneKeymap:   newPaneKeymap(),
		Input:        newTextarea("Type text you want to cipher..."),
		Output:       newOutputTextarea("Result will appear here..."),
		SaveFilename: newTextinput("output.txt"),
		Key:          newTextinput("Enter key..."),
		Cipher:       c,
	}
}

func (p CipherPane) Init() tea.Cmd {
	return p.Filepicker.Init()
}

func (p CipherPane) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width, p.Height = msg.Width, msg.Height

		// The input border is drawn by the view, so the widget itself gets the
		// inner width; the output stays borderless like in v1.
		layout := NewLayout(msg.Width, msg.Height)
		p.Input.SetWidth(layout.TextareaInputWidth - 2)
		p.Input.SetHeight(layout.TextareaInputHeight)
		p.Output.SetWidth(layout.TextareaOutputWidth)
		p.Output.SetHeight(layout.TextareaOutputHeight)
		p.Filepicker.SetHeight(layout.FilepickerHeight)
		p.syncKeyWidth()
		p.SaveFilename.SetWidth(34)

	case statusMsg:
		p.Status = ""
		return p, nil

	case tea.KeyPressMsg:
		// Save Popup Logic
		if p.ShowSavePopup {
			switch msg.String() {
			case "esc":
				p.ShowSavePopup = false
				p.SaveFilename.Blur()
				return p, nil
			case "enter":
				filename := p.SaveFilename.Value()
				if filename == "" {
					filename = "output.txt"
				}

				err := os.WriteFile(filename, []byte(p.Output.Value()), 0o644)
				if err != nil {
					p.Status = "Error: " + err.Error()
				} else {
					p.Status = "Saved to: " + filename
				}

				p.ShowSavePopup = false
				p.SaveFilename.Blur()

				return p, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
					return statusMsg("")
				})
			default:
				var cmd tea.Cmd
				p.SaveFilename, cmd = p.SaveFilename.Update(msg)
				return p, cmd
			}
		}

		// Focus Navigation
		switch {
		case key.Matches(msg, p.PaneKeymap.next):
			p.Focus = (p.Focus + 1) % FocusCount
			p.skipInactive(1)
			p.updateFocus()
			return p, p.Filepicker.Init()
		case key.Matches(msg, p.PaneKeymap.prev):
			p.Focus = (p.Focus - 1 + FocusCount) % FocusCount
			p.skipInactive(-1)
			p.updateFocus()
			return p, p.Filepicker.Init()
		}

		// Filepicker specific
		if p.InputMode == File && p.Focus == FocusFilepicker {
			if !p.FileConfirmed {
				var cmd tea.Cmd
				p.Filepicker, cmd = p.Filepicker.Update(msg)
				if didSelect, path := p.Filepicker.DidSelectFile(msg); didSelect {
					p.SelectedFile = path
					p.FileConfirmed = true
				}
				return p, cmd
			}
			if msg.String() == "backspace" {
				p.FileConfirmed = false
				p.SelectedFile = ""
				return p, p.Filepicker.Init()
			}
			return p, nil
		}

		// Other focusable elements
		switch {
		case key.Matches(msg, p.PaneKeymap.copy):
			p.Input.SetValue(p.Output.Value())
			p.Output.SetValue("")
			p.InputMode = ManualInput
			p.Focus = FocusInput
			p.updateFocus()
			return p, nil
		case key.Matches(msg, p.PaneKeymap.clearInput):
			p.Input.SetValue("")
			p.Focus = FocusInput
			p.updateFocus()
			return p, nil
		case p.shouldClearKey(msg):
			p.Key.SetValue("")
			p.syncKeyWidth()
			return p, nil
		case p.Focus == FocusInputMode && msg.String() == "enter":
			p.InputMode = (p.InputMode + 1) % 2
			p.FileConfirmed = false
			p.SelectedFile = ""
			p.updateFocus()
			return p, p.Filepicker.Init()
		case p.Focus == FocusMethod && msg.String() == "enter":
			p.Method = (p.Method + 1) % len(p.Cipher.Methods())
			p.skipInactive(1)
			p.updateFocus()
			return p, nil
		case p.Focus == FocusMode && msg.String() == "enter":
			p.Mode = (p.Mode + 1) % 2
			return p, nil
		case p.Focus == FocusBtn && msg.String() == "enter":
			p.run()
			return p, nil
		case p.Focus == FocusSaveBtn && msg.String() == "enter":
			p.ShowSavePopup = true
			p.SaveFilename.SetValue("")
			p.SaveFilename.Focus()
			return p, nil
		case p.Focus == FocusKey && key.Matches(msg, p.PaneKeymap.genKey):
			generated, err := generateRandomKey(p.Cipher.KeySize())
			if err != nil {
				p.Status = "failed to generate key"
				return p, tea.Tick(3*time.Second, func(time.Time) tea.Msg { return statusMsg("") })
			}
			p.Key.SetValue(generated)
			p.syncKeyWidth()
			return p, nil
		}
	}

	// Forward to the focused widget
	if p.Focus == FocusInput {
		var cmd tea.Cmd
		p.Input, cmd = p.Input.Update(msg)
		cmds = append(cmds, cmd)
	}
	if p.Focus == FocusFilepicker {
		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(msg)
		cmds = append(cmds, cmd)
	}
	if p.Focus == FocusKey {
		var cmd tea.Cmd
		p.Key, cmd = p.Key.Update(msg)
		p.syncKeyWidth()
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

func (p *CipherPane) run() {
	p.Output.SetValue("")
	decrypt := p.Mode == Decrypt

	var inputSource []byte
	if p.InputMode == File {
		if p.SelectedFile == "" {
			p.Output.SetValue("No file selected")
			return
		}
		content, err := os.ReadFile(p.SelectedFile)
		if err != nil {
			p.Output.SetValue("Error reading file: " + err.Error())
			return
		}
		if decrypt {
			inputSource, err = hex.DecodeString(strings.TrimSpace(string(content)))
			if err != nil {
				p.Output.SetValue("File must contain hex ciphertext")
				return
			}
		} else {
			inputSource = content
		}
	} else {
		if decrypt {
			decoded, err := hex.DecodeString(strings.TrimSpace(p.Input.Value()))
			if err != nil {
				return
			}
			inputSource = decoded
		} else {
			inputSource = []byte(p.Input.Value())
		}
	}

	if len(inputSource) < 8 {
		p.Output.SetValue("Input should be longer than 8 bytes")
		return
	}

	key, err := hex.DecodeString(strings.TrimSpace(p.Key.Value()))
	if err != nil {
		p.Output.SetValue("Cannot decode key")
		return
	}

	res, err := p.Cipher.Run(int(p.Method), decrypt, inputSource, key)
	if err != nil {
		p.Output.SetValue("Error: " + err.Error())
		return
	}
	p.Output.SetValue(res)
}

// ConsumesQuit tells whether a quit key should be handled by this pane instead of quitting the app.
func (p CipherPane) ConsumesQuit(msg tea.KeyMsg) bool {
	return p.ShowSavePopup
}

func (p CipherPane) View() string {
	inputModeLabel := labelStyle.Render("INPUT MODE")
	inputModeValue := "File"
	if p.InputMode == ManualInput {
		inputModeValue = "Manual input"
	}
	if p.Focus == FocusInputMode {
		inputModeValue = selectedStyle.Render(inputModeValue) + " (Enter)"
	}

	methodLabel := labelStyle.Render("METHOD")
	methodValue := p.Cipher.Methods()[p.Method]

	if p.Focus == FocusMethod {
		methodValue = selectedStyle.Render(methodValue) + " (Enter)"
	}

	modeLabel := labelStyle.Render("MODE")
	var modeValue string
	switch p.Mode {
	case Encrypt:
		modeValue = "Encrypt"
	case Decrypt:
		modeValue = "Decrypt"
	}

	if p.Focus == FocusMode {
		modeValue = selectedStyle.Render(modeValue) + " (Enter)"
	}

	keyLabel := labelStyle.Render("KEY")

	runBtnLabel := "RUN"
	if p.Focus == FocusBtn {
		runBtnLabel = buttonStyle.Render(runBtnLabel)
	}

	saveBtnLabel := "SAVE TO FILE"
	if p.Focus == FocusSaveBtn {
		saveBtnLabel = saveBtnStyle.Render(saveBtnLabel)
	}

	innerWidth := LeftPanelWidth - 2
	// Hardwrap keeps "> " attached to the key; word-wrap would push it to its own line.
	keyWrapped := ansi.Hardwrap(p.Key.View(), innerWidth, true)

	leftContent := lipgloss.JoinVertical(
		lipgloss.Left,
		inputModeLabel,
		inputModeValue,
		"",
		methodLabel,
		methodValue,
		"",
		modeLabel,
		modeValue,
		"",
		keyLabel,
		keyWrapped,
		"",
		"",
		runBtnLabel,
		saveBtnLabel,
	)

	return p.frameView(leftContent)
}

func (p CipherPane) frameView(content string) string {
	layout := NewLayout(p.Width, p.Height)

	innerWidth := LeftPanelWidth - 2
	wrappedContent := lipgloss.NewStyle().Width(innerWidth).Render(content)

	innerHeight := layout.LeftPanelHeight
	if wrappedHeight := lipgloss.Height(wrappedContent); wrappedHeight < innerHeight {
		wrappedContent = lipgloss.PlaceVertical(innerHeight, lipgloss.Top, wrappedContent)
	}

	leftPanel := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1).
		Render(wrappedContent)

	// lipgloss v2 sizes the whole block, border included; the layout numbers
	// below are v1 dimensions that exclude it, so the border frame is added.
	inputPanelStyle := lipgloss.NewStyle().
		Height(layout.TopPanelHeight + 2).
		Width(layout.RightPanelWidth + 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1)

	outputPanelStyle := lipgloss.NewStyle().
		Height(layout.BottomPanelHeight + 2).
		Width(layout.RightPanelWidth + 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1)

	inputTitle := "INPUT"
	inputContent := p.textareaFrame(p.Focus == FocusInput).Render(p.Input.View())

	if p.InputMode == File {
		if p.FileConfirmed {
			inputTitle = "FILE SELECTED"

			style := selectedStyle
			if p.Focus != FocusFilepicker {
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
			}

			selectedLine := style.Render("Selected: " + p.SelectedFile)
			hintLine := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true).
				Render("Press Backspace to change file")
			inputContent = lipgloss.JoinVertical(lipgloss.Left, selectedLine, "", hintLine)
		} else {
			inputTitle = "FILE PICKER"
			inputContent = p.Filepicker.View()
		}
	}

	inputPanel := inputPanelStyle.Render(lipgloss.PlaceVertical(layout.TopPanelHeight, lipgloss.Top, labelStyle.Render(inputTitle)+"\n"+inputContent))

	outputView := p.Output.View()
	if p.Status != "" {
		outputView = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true).
			Render(p.Status)
	}

	outputPanel := outputPanelStyle.Render(lipgloss.PlaceVertical(layout.BottomPanelHeight, lipgloss.Top, labelStyle.Render("OUTPUT")+"\n"+outputView))

	rightPanel := lipgloss.JoinVertical(0, inputPanel, outputPanel)

	baseView := lipgloss.JoinHorizontal(0, leftPanel, rightPanel)

	if p.ShowSavePopup {
		popupContent := lipgloss.JoinVertical(
			lipgloss.Left,
			labelStyle.Render("SAVE OUTPUT TO FILE"),
			"",
			"Filename:",
			p.SaveFilename.View(),
			"",
			lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				Italic(true).
				Render("Enter to save  •  Esc to cancel"),
		)
		popup := popupStyle.Render(popupContent)
		return lipgloss.Place(
			p.Width, p.Height,
			lipgloss.Center, lipgloss.Center,
			popup,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("237"))),
		)
	}

	return baseView
}

// syncKeyWidth keeps v1's key display: bubbles v2 needs a non-zero width to
// draw the whole placeholder, but width 0 is intentional once the field holds a
// key so the widget emits the full value and ansi.Hardwrap wraps it like v1.
func (p *CipherPane) syncKeyWidth() {
	if p.Key.Value() == "" {
		innerWidth := LeftPanelWidth - 2
		p.Key.SetWidth(innerWidth - lipgloss.Width(p.Key.Prompt) - 1)
		return
	}

	p.Key.SetWidth(0)
	// SetWidth does not re-layout the field; SetCursor recomputes the overflow
	// offsets, without which a value set while the width was non-zero stays
	// clipped instead of being emitted in full for Hardwrap.
	p.Key.SetCursor(p.Key.Position())
}

// textareaFrame draws the border that v1 kept on the input textarea itself.
// bubbles v2.2 draws a bordered Base style twice while the placeholder is
// shown (View and placeholderView both apply it), which nests a second box.
func (p CipherPane) textareaFrame(focused bool) lipgloss.Style {
	if focused {
		return focusedBorderStyle
	}
	return blurredBorderStyle
}

func (p *CipherPane) skipInactive(dir int) {
	for {
		inactive := false
		if p.InputMode == ManualInput && p.Focus == FocusFilepicker {
			inactive = true
		}
		if p.InputMode == File && p.Focus == FocusInput {
			inactive = true
		}

		if !inactive {
			break
		}
		p.Focus = (p.Focus + dir + FocusCount) % FocusCount
	}
}

func (p *CipherPane) updateFocus() {
	p.Input.Blur()
	p.Key.Blur()
	p.SaveFilename.Blur()

	switch p.Focus {
	case FocusInput:
		p.Input.Focus()
	case FocusKey:
		p.Key.Focus()
	}

	if p.InputMode == File {
		p.Input.Blur()
	}
}

func (p CipherPane) shouldClearKey(msg tea.KeyPressMsg) bool {
	return p.Focus == FocusKey && key.Matches(msg, p.PaneKeymap.clearKey) && p.Key.Value() != ""
}

func generateRandomKey(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
