package cipherpane

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"strings"
	"time"

	"go-cipher/internal/cipher"
	"go-cipher/internal/ui"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	// pickerClickRow is the row selected by the previous click; clicking it
	// again opens the entry.
	pickerClickRow int
}

func NewCipherPane(c cipher.Cipher) CipherPane {
	return CipherPane{
		PaneBase: ui.PaneBase{
			Zones: ui.NewZones("cipher:" + c.Name()),
			Focus: FocusInput,
		},
		Filepicker:     newFilepicker(),
		pickerClickRow: -1,
		PaneKeymap:     newPaneKeymap(),
		Input:          newTextarea("Type text you want to cipher..."),
		Output:         newOutputTextarea("Result will appear here..."),
		SaveFilename:   newTextinput("output.txt"),
		Key:            newTextinput("Enter key..."),
		Cipher:         c,
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

		layout := NewLayout(msg.Width, msg.Height)
		p.Input.SetWidth(layout.TextareaInputWidth)
		p.Input.SetHeight(layout.TextareaInputHeight)
		p.Output.SetWidth(layout.TextareaOutputWidth)
		p.Output.SetHeight(layout.TextareaOutputHeight)
		p.Filepicker.SetHeight(layout.FilepickerHeight)
		p.SaveFilename.Width = 34

	case tea.MouseMsg:
		return p.handleMouse(msg)

	case statusMsg:
		p.Status = ""
		return p, nil

	case tea.KeyMsg:
		// Save Popup Logic
		if p.ShowSavePopup {
			switch msg.String() {
			case "esc":
				p.ShowSavePopup = false
				p.SaveFilename.Blur()
				return p, nil
			case "enter":
				return p.saveOutput()
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
			return p, nil
		}
	}

	// Directory listings are delivered as non-key messages, so they must reach
	// the picker even while another field owns the focus.
	if p.InputMode == File && !p.FileConfirmed {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var cmd tea.Cmd
			p.Filepicker, cmd = p.Filepicker.Update(msg)
			cmds = append(cmds, cmd)
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

func (p CipherPane) handleMouse(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	if p.ShowSavePopup {
		return p.handleSavePopupMouse(msg)
	}

	switch {
	case p.Zones.Clicked("input-mode", msg):
		p.Focus = FocusInputMode
		p.InputMode = (p.InputMode + 1) % 2
		p.FileConfirmed = false
		p.SelectedFile = ""
		p.pickerClickRow = -1
		p.updateFocus()
		return p, p.Filepicker.Init()
	case p.Zones.Clicked("method", msg):
		p.Focus = FocusMethod
		p.Method = (p.Method + 1) % len(p.Cipher.Methods())
		return p, nil
	case p.Zones.Clicked("mode", msg):
		p.Focus = FocusMode
		p.Mode = (p.Mode + 1) % 2
		return p, nil
	case p.Zones.Clicked("key", msg):
		p.Focus = FocusKey
		p.updateFocus()
		return p, nil
	case p.Zones.Clicked("input", msg):
		p.Focus = FocusInput
		p.updateFocus()
		return p, nil
	case p.Zones.Clicked("filepicker", msg):
		return p.clickFileRow(msg)
	case p.Zones.Clicked("file-selected", msg):
		p.FileConfirmed = false
		p.SelectedFile = ""
		p.Focus = FocusFilepicker
		p.pickerClickRow = -1
		return p, p.Filepicker.Init()
	case p.Zones.Clicked("run", msg):
		p.Focus = FocusBtn
		p.run()
		return p, nil
	case p.Zones.Clicked("save", msg):
		p.Focus = FocusSaveBtn
		return p.openSavePopup()
	}

	if p.InputMode == File && !p.FileConfirmed && p.Zones.InBounds("filepicker", msg) {
		return p.scrollFilepicker(msg)
	}

	return p, nil
}

func (p CipherPane) handleSavePopupMouse(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	switch {
	case p.Zones.Clicked("popup-save", msg):
		return p.saveOutput()
	case p.Zones.Clicked("popup-cancel", msg):
		p.ShowSavePopup = false
		p.SaveFilename.Blur()
		return p, nil
	case p.Zones.Clicked("popup-filename", msg):
		p.SaveFilename.Focus()
		return p, nil
	case ui.IsLeftClick(msg) && !p.Zones.InBounds("popup", msg):
		p.ShowSavePopup = false
		p.SaveFilename.Blur()
	}
	return p, nil
}

func (p CipherPane) clickFileRow(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	p.Focus = FocusFilepicker
	p.updateFocus()
	if p.InputMode != File || p.FileConfirmed {
		return p, nil
	}

	_, y := p.Zones.Pos("filepicker", msg)
	if y == p.pickerClickRow {
		enter := tea.KeyMsg{Type: tea.KeyEnter}
		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(enter)
		if didSelect, path := p.Filepicker.DidSelectFile(enter); didSelect {
			p.SelectedFile = path
			p.FileConfirmed = true
		}
		return p, cmd
	}

	p.pickerClickRow = y
	p.Filepicker, _ = p.Filepicker.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	for i := 0; i < y; i++ {
		p.Filepicker, _ = p.Filepicker.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	return p, nil
}

func (p CipherPane) scrollFilepicker(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	var key tea.KeyMsg
	switch ui.WheelDirection(msg) {
	case -1:
		key = tea.KeyMsg{Type: tea.KeyUp}
	case 1:
		key = tea.KeyMsg{Type: tea.KeyDown}
	default:
		return p, nil
	}

	var cmd tea.Cmd
	p.Filepicker, cmd = p.Filepicker.Update(key)
	return p, cmd
}

func (p CipherPane) openSavePopup() (ui.Pane, tea.Cmd) {
	p.ShowSavePopup = true
	p.SaveFilename.SetValue("")
	p.SaveFilename.Focus()
	return p, nil
}

func (p CipherPane) saveOutput() (ui.Pane, tea.Cmd) {
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

	return p, tea.Tick(3*time.Second, func(time.Time) tea.Msg {
		return statusMsg("")
	})
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
	inputModeValue = p.Zones.Mark("input-mode", inputModeValue)

	methodLabel := labelStyle.Render("METHOD")
	methodValue := p.Cipher.Methods()[p.Method]

	if p.Focus == FocusMethod {
		methodValue = selectedStyle.Render(methodValue) + " (Enter)"
	}
	methodValue = p.Zones.Mark("method", methodValue)

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
	modeValue = p.Zones.Mark("mode", modeValue)

	keyLabel := labelStyle.Render("KEY")

	runBtnLabel := "RUN"
	if p.Focus == FocusBtn {
		runBtnLabel = buttonStyle.Render(runBtnLabel)
	}
	runBtnLabel = p.Zones.Mark("run", runBtnLabel)

	saveBtnLabel := "SAVE TO FILE"
	if p.Focus == FocusSaveBtn {
		saveBtnLabel = saveBtnStyle.Render(saveBtnLabel)
	}
	saveBtnLabel = p.Zones.Mark("save", saveBtnLabel)

	innerWidth := LeftPanelWidth - 2
	// Hardwrap keeps "> " attached to the key; word-wrap would push it to its own line.
	keyWrapped := p.Zones.Mark("key", ansi.Hardwrap(p.Key.View(), innerWidth, true))

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

	inputPanelStyle := lipgloss.NewStyle().
		Height(layout.TopPanelHeight).
		Width(layout.RightPanelWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1)

	outputPanelStyle := lipgloss.NewStyle().
		Height(layout.BottomPanelHeight).
		Width(layout.RightPanelWidth).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Padding(1)

	inputTitle := "INPUT"
	var inputContent string
	switch {
	case p.InputMode == File && p.FileConfirmed:
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
		inputContent = p.Zones.Mark("file-selected", lipgloss.JoinVertical(lipgloss.Left, selectedLine, "", hintLine))
	case p.InputMode == File:
		inputTitle = "FILE PICKER"
		inputContent = p.Zones.Mark("filepicker", p.Filepicker.View())
	default:
		inputContent = p.Zones.Mark("input", p.Input.View())
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
			p.Zones.Mark("popup-filename", p.SaveFilename.View()),
			"",
			lipgloss.JoinHorizontal(
				lipgloss.Left,
				p.Zones.Mark("popup-save", saveBtnStyle.Render("SAVE")),
				"  ",
				p.Zones.Mark("popup-cancel", lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[ CANCEL ]")),
			),
		)
		popup := p.Zones.Mark("popup", popupStyle.Render(popupContent))
		return lipgloss.Place(
			p.Width, max(p.Height-ui.TabBarHeight, 1),
			lipgloss.Center, lipgloss.Center,
			popup,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("237")),
		)
	}

	return baseView
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

func (p CipherPane) shouldClearKey(msg tea.KeyMsg) bool {
	return p.Focus == FocusKey && key.Matches(msg, p.PaneKeymap.clearKey) && p.Key.Value() != ""
}

func generateRandomKey(size int) (string, error) {
	bytes := make([]byte, size)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}
