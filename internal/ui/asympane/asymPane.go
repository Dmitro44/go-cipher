package asympane

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

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

type AsymPane struct {
	ui.PaneBase
	Cipher         cipher.AsymCipher
	PublicKey      textinput.Model
	PrivateKey     textinput.Model
	PrivateVisible bool
	KeyPreset      int
	Input          textarea.Model
	Output         textarea.Model
	InputMode      InputMode
	Mode           Mode
	Filepicker     filepicker.Model
	SelectedFile   string
	FileConfirmed  bool
	SaveFilename   textinput.Model
	ShowSavePopup  bool
	Generating     bool
	StatusError    bool
	PaneKeymap     PaneKeymap
	// pickerClickRow is the row selected by the previous click; clicking it
	// again opens the entry.
	pickerClickRow int
}

type keysGeneratedMsg struct {
	keys cipher.KeyPair
	err  error
}

func NewAsymPane(c cipher.AsymCipher) AsymPane {
	p := AsymPane{
		PaneBase: ui.PaneBase{
			Zones: ui.NewZones("asym:" + c.Name()),
			Focus: FocusKeySize,
		},
		Cipher:         c,
		PaneKeymap:     newPaneKeymap(),
		PublicKey:      newTextinput("No public key yet — press Ctrl+G"),
		PrivateKey:     newTextinput("No private key yet — press Ctrl+G"),
		Input:          newTextarea("Type a message to encrypt, or hex ciphertext to decrypt..."),
		Output:         newOutputTextarea("Result will appear here..."),
		Filepicker:     newFilepicker(),
		SaveFilename:   newTextinput("output.txt"),
		pickerClickRow: -1,
	}
	p.PrivateKey.EchoMode = textinput.EchoPassword
	p.PrivateKey.EchoCharacter = '•'
	p.updateFocus()
	return p
}

func (p AsymPane) Init() tea.Cmd {
	return p.Filepicker.Init()
}

func (p AsymPane) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width, p.Height = msg.Width, msg.Height

		l := NewLayout(msg.Width, msg.Height)
		p.Input.SetWidth(l.TextareaWidth)
		p.Input.SetHeight(l.TextareaHeight)
		p.Output.SetWidth(l.TextareaWidth)
		p.Output.SetHeight(l.TextareaHeight)
		p.Filepicker.SetHeight(l.FilepickerHeight)
		p.PublicKey.Width = l.KeyFieldWidth
		p.PrivateKey.Width = l.KeyFieldWidth
		p.SaveFilename.Width = 34

	case tea.MouseMsg:
		return p.handleMouse(msg)

	case keysGeneratedMsg:
		p.Generating = false
		if msg.err != nil {
			p.setError("Key generation failed: " + msg.err.Error())
			return p, p.clearStatusCmd()
		}
		p.PublicKey.SetValue(hex.EncodeToString(msg.keys.Public))
		p.PrivateKey.SetValue(hex.EncodeToString(msg.keys.Private))
		p.hidePrivate()
		p.setStatus(fmt.Sprintf(
			"Keypair ready — public %d B, private %d B",
			len(msg.keys.Public), len(msg.keys.Private),
		))
		p.updateFocus()
		return p, p.clearStatusCmd()

	case statusMsg:
		if p.Status == string(msg) {
			p.Status = ""
			p.StatusError = false
		}
		return p, nil

	case tea.KeyMsg:
		// Save popup owns the keyboard while open.
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

		// Focus navigation.
		switch {
		case key.Matches(msg, p.PaneKeymap.next):
			p.Focus = (p.Focus + 1) % FocusCount
			p.updateFocus()
			return p, p.Filepicker.Init()
		case key.Matches(msg, p.PaneKeymap.prev):
			p.Focus = (p.Focus - 1 + FocusCount) % FocusCount
			p.updateFocus()
			return p, p.Filepicker.Init()
		}

		// Global shortcuts work from any field.
		switch {
		case key.Matches(msg, p.PaneKeymap.reveal):
			p.togglePrivate()
			return p, nil
		case key.Matches(msg, p.PaneKeymap.generate):
			if cmd := p.startGeneration(); cmd != nil {
				return p, cmd
			}
			return p, nil
		case key.Matches(msg, p.PaneKeymap.copyOutput):
			p.Input.SetValue(p.Output.Value())
			p.Output.SetValue("")
			p.InputMode = ManualInput
			p.FileConfirmed = false
			p.SelectedFile = ""
			p.Focus = FocusInput
			p.updateFocus()
			return p, nil
		case key.Matches(msg, p.PaneKeymap.clearInput):
			p.Input.SetValue("")
			p.Focus = FocusInput
			p.updateFocus()
			return p, nil
		}

		// File picker navigation.
		if p.InputMode == FileInput && p.Focus == FocusInput {
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

		// Focus-specific actions.
		switch {
		case p.Focus == FocusKeySize && msg.String() == "enter":
			if n := len(p.Cipher.KeyOptions()); n > 1 {
				p.KeyPreset = (p.KeyPreset + 1) % n
			}
			return p, nil
		case p.Focus == FocusGenerate && msg.String() == "enter":
			if cmd := p.startGeneration(); cmd != nil {
				return p, cmd
			}
			return p, nil
		case p.Focus == FocusSource && msg.String() == "enter":
			p.toggleInputMode()
			return p, p.Filepicker.Init()
		case p.Focus == FocusMode && msg.String() == "enter":
			p.Mode = (p.Mode + 1) % 2
			return p, nil
		case p.Focus == FocusRun && msg.String() == "enter":
			cmd := p.run()
			return p, cmd
		case p.Focus == FocusSave && msg.String() == "enter":
			return p.openSavePopup()
		case p.Focus == FocusPublicKey && key.Matches(msg, p.PaneKeymap.clearKey):
			p.PublicKey.SetValue("")
			return p, nil
		case p.Focus == FocusPrivateKey && key.Matches(msg, p.PaneKeymap.clearKey):
			p.PrivateKey.SetValue("")
			return p, nil
		}
	}

	// Directory listings are delivered as non-key messages, so they must reach
	// the picker even while another field owns the focus.
	if p.InputMode == FileInput && !p.FileConfirmed {
		if _, isKey := msg.(tea.KeyMsg); !isKey {
			var cmd tea.Cmd
			p.Filepicker, cmd = p.Filepicker.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	// Forward to the focused widget.
	switch p.Focus {
	case FocusInput:
		if p.InputMode == ManualInput {
			var cmd tea.Cmd
			p.Input, cmd = p.Input.Update(msg)
			cmds = append(cmds, cmd)
		}
	case FocusPublicKey:
		var cmd tea.Cmd
		p.PublicKey, cmd = p.PublicKey.Update(msg)
		cmds = append(cmds, cmd)
	case FocusPrivateKey:
		var cmd tea.Cmd
		p.PrivateKey, cmd = p.PrivateKey.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

// ConsumesQuit tells whether a quit key should be handled by this pane instead of quitting the app.
func (p AsymPane) ConsumesQuit(msg tea.KeyMsg) bool {
	return p.ShowSavePopup
}

func (p AsymPane) handleMouse(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	if p.ShowSavePopup {
		return p.handleSavePopupMouse(msg)
	}

	switch {
	case p.Zones.Clicked("parameter", msg):
		p.Focus = FocusKeySize
		if n := len(p.Cipher.KeyOptions()); n > 1 {
			p.KeyPreset = (p.KeyPreset + 1) % n
		}
		return p, nil
	case p.Zones.Clicked("generate", msg):
		p.Focus = FocusGenerate
		if cmd := p.startGeneration(); cmd != nil {
			return p, cmd
		}
		return p, nil
	case p.Zones.Clicked("public-key", msg):
		p.Focus = FocusPublicKey
		p.updateFocus()
		return p, nil
	case p.Zones.Clicked("private-reveal", msg):
		p.togglePrivate()
		return p, nil
	case p.Zones.Clicked("private-key", msg):
		p.Focus = FocusPrivateKey
		p.updateFocus()
		return p, nil
	case p.Zones.Clicked("source", msg):
		p.Focus = FocusSource
		p.toggleInputMode()
		p.pickerClickRow = -1
		return p, p.Filepicker.Init()
	case p.Zones.Clicked("mode", msg):
		p.Focus = FocusMode
		p.Mode = (p.Mode + 1) % 2
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
		p.Focus = FocusInput
		p.pickerClickRow = -1
		p.updateFocus()
		return p, p.Filepicker.Init()
	case p.Zones.Clicked("run", msg):
		p.Focus = FocusRun
		return p, p.run()
	case p.Zones.Clicked("save", msg):
		p.Focus = FocusSave
		return p.openSavePopup()
	}

	if p.InputMode == FileInput && !p.FileConfirmed && p.Zones.InBounds("filepicker", msg) {
		return p.scrollFilepicker(msg)
	}

	return p, nil
}

func (p AsymPane) handleSavePopupMouse(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
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

func (p AsymPane) clickFileRow(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
	p.Focus = FocusInput
	p.updateFocus()
	if p.InputMode != FileInput || p.FileConfirmed {
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

func (p AsymPane) scrollFilepicker(msg tea.MouseMsg) (ui.Pane, tea.Cmd) {
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

func (p AsymPane) openSavePopup() (ui.Pane, tea.Cmd) {
	p.ShowSavePopup = true
	p.SaveFilename.SetValue("")
	p.SaveFilename.Focus()
	return p, nil
}

func (p AsymPane) saveOutput() (ui.Pane, tea.Cmd) {
	filename := p.SaveFilename.Value()
	if filename == "" {
		filename = "output.txt"
	}

	err := os.WriteFile(filename, []byte(p.Output.Value()), 0o644)
	if err != nil {
		p.setError("Save failed: " + err.Error())
	} else {
		p.setStatus("Saved to: " + filename)
	}

	p.ShowSavePopup = false
	p.SaveFilename.Blur()
	return p, p.clearStatusCmd()
}

func (p *AsymPane) run() tea.Cmd {
	p.Output.SetValue("")
	p.Status = ""
	p.StatusError = false

	decrypt := p.Mode == Decrypt

	in, err := p.readInput(decrypt)
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	if decrypt {
		priv, err := decodeHex("private key", p.PrivateKey.Value())
		if err != nil {
			p.setError(err.Error())
			return p.clearStatusCmd()
		}

		plain, err := p.Cipher.Decrypt(priv, in)
		if err != nil {
			p.setError("Decryption failed: " + err.Error())
			return p.clearStatusCmd()
		}
		if len(plain) == 0 {
			p.setError("Decryptor returned an empty plaintext")
			return p.clearStatusCmd()
		}

		p.Output.SetValue(displayBytes(plain))
		p.setStatus("Decryption succeeded")
		return p.clearStatusCmd()
	}

	pub, err := decodeHex("public key", p.PublicKey.Value())
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	ct, err := p.Cipher.Encrypt(pub, in)
	if err != nil {
		p.setError("Encryption failed: " + err.Error())
		return p.clearStatusCmd()
	}

	p.Output.SetValue(hex.EncodeToString(ct))
	p.setStatus("Encryption succeeded")
	return p.clearStatusCmd()
}

func (p AsymPane) readInput(decrypt bool) ([]byte, error) {
	var raw []byte

	if p.InputMode == FileInput {
		if p.SelectedFile == "" {
			return nil, errors.New("no file selected")
		}
		content, err := os.ReadFile(p.SelectedFile)
		if err != nil {
			return nil, errors.New("cannot read file: " + err.Error())
		}
		if decrypt {
			decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
			if err != nil {
				return nil, errors.New("file must contain hex ciphertext")
			}
			if len(decoded) == 0 {
				return nil, errors.New("ciphertext file is empty")
			}
			return decoded, nil
		}
		raw = content
	} else {
		if decrypt {
			decoded, err := hex.DecodeString(strings.TrimSpace(p.Input.Value()))
			if err != nil {
				return nil, errors.New("ciphertext must be hex")
			}
			if len(decoded) == 0 {
				return nil, errors.New("input is empty")
			}
			return decoded, nil
		}
		raw = []byte(p.Input.Value())
	}

	if len(raw) == 0 {
		return nil, errors.New("input is empty")
	}
	return raw, nil
}

func (p *AsymPane) startGeneration() tea.Cmd {
	if p.Generating {
		return nil
	}

	options := p.Cipher.KeyOptions()
	if len(options) == 0 {
		p.setError("No key presets configured")
		return p.clearStatusCmd()
	}
	if p.KeyPreset >= len(options) {
		p.KeyPreset = 0
	}

	c := p.Cipher
	option := options[p.KeyPreset]
	p.Generating = true
	p.setStatus("Generating " + option.Label + " keypair…")

	return func() tea.Msg {
		keys, err := c.GenerateKeys(option)
		return keysGeneratedMsg{keys: keys, err: err}
	}
}

func (p *AsymPane) toggleInputMode() {
	if p.InputMode == ManualInput {
		p.InputMode = FileInput
	} else {
		p.InputMode = ManualInput
	}
	p.FileConfirmed = false
	p.SelectedFile = ""
	p.updateFocus()
}

func (p *AsymPane) togglePrivate() {
	if p.PrivateVisible {
		p.hidePrivate()
		return
	}
	p.PrivateVisible = true
	p.PrivateKey.EchoMode = textinput.EchoNormal
}

func (p *AsymPane) hidePrivate() {
	p.PrivateVisible = false
	p.PrivateKey.EchoMode = textinput.EchoPassword
	p.PrivateKey.EchoCharacter = '•'
}

func (p *AsymPane) updateFocus() {
	p.Input.Blur()
	p.PublicKey.Blur()
	p.PrivateKey.Blur()
	p.SaveFilename.Blur()

	switch p.Focus {
	case FocusInput:
		if p.InputMode == ManualInput {
			p.Input.Focus()
		}
	case FocusPublicKey:
		p.PublicKey.Focus()
	case FocusPrivateKey:
		p.PrivateKey.Focus()
	}
}

func (p *AsymPane) setStatus(text string) {
	p.Status = text
	p.StatusError = false
}

func (p *AsymPane) setError(text string) {
	p.Status = text
	p.StatusError = true
}

func (p AsymPane) clearStatusCmd() tea.Cmd {
	text := p.Status
	if text == "" {
		return nil
	}
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return statusMsg(text)
	})
}

func (p AsymPane) View() string {
	l := NewLayout(p.Width, p.Height)

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		p.keysView(l),
		p.messageView(l),
		p.actionView(l),
	)

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
				p.Zones.Mark("popup-cancel", mutedStyle.Render("[ CANCEL ]")),
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

	return base
}

func (p AsymPane) keysView(l Layout) string {
	keyContentWidth := l.PanelWidth - 4

	title := labelStyle.Render("KEYRING") + mutedStyle.Render(" · "+p.Cipher.Name())

	options := p.Cipher.KeyOptions()
	presetText := "n/a"
	if len(options) > 0 && p.KeyPreset < len(options) {
		presetText = options[p.KeyPreset].Label
	}
	if p.Focus == FocusKeySize {
		presetText = selectedStyle.Render(presetText)
	}
	presetText = p.Zones.Mark("parameter", presetText)
	sizeBlock := labelStyle.Render("PARAMETERS ") + presetText + mutedStyle.Render(" (Enter)")
	header := spread(title, sizeBlock, keyContentWidth)

	pubStyle := labelStyle
	if p.Mode == Encrypt {
		pubStyle = pubStyle.Foreground(lipgloss.Color("212"))
	}
	pubLabel := pubStyle.Render("PUBLIC KEY")

	privState := "hidden"
	if p.PrivateVisible {
		privState = "visible"
	}
	privStyle := labelStyle
	if p.Mode == Decrypt {
		privStyle = privStyle.Foreground(lipgloss.Color("212"))
	}
	privLabel := privStyle.Render("PRIVATE KEY") + mutedStyle.Render(" · "+privState)

	pubLabel = ansi.Truncate(pubLabel, l.KeyColumnWidth, "…")
	privLabel = ansi.Truncate(privLabel, l.KeyColumnWidth, "…")

	privLabel = p.Zones.Mark("private-reveal", privLabel)
	pubColumn := lipgloss.NewStyle().
		Width(l.KeyColumnWidth).
		Render(p.Zones.Mark("public-key", pubLabel+"\n"+p.PublicKey.View()))
	privColumn := lipgloss.NewStyle().
		Width(l.KeyColumnWidth).
		Render(privLabel + "\n" + p.Zones.Mark("private-key", p.PrivateKey.View()))
	fieldsRow := lipgloss.JoinHorizontal(lipgloss.Top, pubColumn, " ", privColumn)

	var generate string
	switch {
	case p.Generating:
		generate = statusStyle.Render("generating…")
	case p.Focus == FocusGenerate:
		generate = keyBtnStyle.Render("GENERATE KEYPAIR")
	default:
		generate = mutedStyle.Render("[ GENERATE KEYPAIR ]")
	}
	generate = p.Zones.Mark("generate", generate)
	footer := spread(generate, mutedStyle.Render("Ctrl+G generate · Ctrl+R reveal"), keyContentWidth)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		fieldsRow,
		"",
		footer,
	)

	return lipgloss.NewStyle().
		Width(keyContentWidth+2).
		Height(l.KeyPanelHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(keyBorderColor).
		Padding(0, 1).
		Render(content)
}

func (p AsymPane) messageView(l Layout) string {
	inputTitle := "INPUT"
	var inputContent string
	switch {
	case p.InputMode == FileInput && p.FileConfirmed:
		inputTitle = "FILE SELECTED"

		style := selectedStyle
		if p.Focus != FocusInput {
			style = mutedStyle
		}

		selectedLine := style.Render("Selected: " + p.SelectedFile)
		hintLine := mutedStyle.Render("Backspace to change file")
		inputContent = p.Zones.Mark("file-selected", lipgloss.JoinVertical(lipgloss.Left, selectedLine, "", hintLine))
	case p.InputMode == FileInput:
		inputTitle = "FILE PICKER"
		inputContent = p.Zones.Mark("filepicker", p.Filepicker.View())
	default:
		inputContent = p.Zones.Mark("input", p.Input.View())
	}

	leftContent := labelStyle.Render(inputTitle) + "\n" + inputContent
	leftPanel := p.messagePanel(l.LeftPanelWidth, l.MessagePanelHeight, p.Focus == FocusInput).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, leftContent))

	rightContent := labelStyle.Render("OUTPUT") + "\n" + p.Output.View()
	rightPanel := p.messagePanel(l.RightPanelWidth, l.MessagePanelHeight, false).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, rightContent))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (p AsymPane) actionView(l Layout) string {
	source := "Manual"
	if p.InputMode == FileInput {
		source = "File"
	}
	if p.Focus == FocusSource {
		source = selectedStyle.Render(source) + mutedStyle.Render(" Enter")
	}
	source = p.Zones.Mark("source", source)

	mode := "Encrypt"
	keyHint := "public key"
	if p.Mode == Decrypt {
		mode = "Decrypt"
		keyHint = "private key"
	}
	if p.Focus == FocusMode {
		mode = selectedStyle.Render(mode) + mutedStyle.Render(" Enter")
	}
	mode = p.Zones.Mark("mode", mode)

	controls := lipgloss.JoinHorizontal(
		lipgloss.Top,
		labelStyle.Render("SOURCE "), source, "   ",
		labelStyle.Render("MODE "), mode, mutedStyle.Render(" ("+keyHint+")"), "   ",
		p.Zones.Mark("run", p.actionButton("RUN", p.Focus == FocusRun, buttonStyle)), "  ",
		p.Zones.Mark("save", p.actionButton("SAVE TO FILE", p.Focus == FocusSave, saveBtnStyle)),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ansi.Truncate(controls, l.PanelWidth, "…"),
		p.hintLine(l.PanelWidth),
	)
}

func (p AsymPane) hintLine(width int) string {
	switch {
	case p.Status != "":
		style := statusStyle
		if p.StatusError {
			style = errorStyle
		}
		return style.Render(ansi.Truncate(p.Status, width, "…"))
	case p.Generating:
		return statusStyle.Render("Generating keypair…")
	default:
		hint := "Ctrl+G keys · Ctrl+R private · Ctrl+D out→input"
		return mutedStyle.Render(ansi.Truncate(hint, width, "…"))
	}
}

func (p AsymPane) actionButton(text string, focused bool, style lipgloss.Style) string {
	if focused {
		return style.Render(text)
	}
	return mutedStyle.Render("[ " + text + " ]")
}

func (p AsymPane) messagePanel(totalWidth, contentHeight int, emphasized bool) lipgloss.Style {
	border := panelBorderColor
	if emphasized {
		border = keyBorderColor
	}

	return lipgloss.NewStyle().
		Width(totalWidth-2).
		Height(contentHeight).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
}

func spread(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

func decodeHex(label, value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New(label + " is empty")
	}
	decoded, err := hex.DecodeString(value)
	if err != nil {
		return nil, errors.New(label + " is not valid hex")
	}
	return decoded, nil
}

func displayBytes(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	return hex.EncodeToString(b)
}
