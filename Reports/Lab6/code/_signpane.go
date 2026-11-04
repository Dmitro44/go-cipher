package signpane

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"

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

// SignPane — вкладка электронной цифровой подписи: кольцо ключей,
// сообщение и подпись r||s, режимы Sign и Verify.
type SignPane struct {
	ui.PaneBase
	Scheme         cipher.SignatureScheme
	PublicKey      textinput.Model
	PrivateKey     textinput.Model
	PrivateVisible bool
	KeyPreset      int
	Message        textarea.Model
	Signature      textarea.Model
	InputMode      InputMode
	Mode           Mode
	Verdict        Verdict
	VerdictReason  string
	Filepicker     filepicker.Model
	SelectedFile   string
	FileConfirmed  bool
	SaveFilename   textinput.Model
	ShowSavePopup  bool
	Generating     bool
	StatusError    bool
	PaneKeymap     PaneKeymap
}

type keysGeneratedMsg struct {
	keys cipher.KeyPair
	err  error
}

func NewSignPane(s cipher.SignatureScheme) SignPane {
	p := SignPane{
		Scheme:       s,
		Focus:        FocusParameters,
		PaneKeymap:   newPaneKeymap(),
		PublicKey:    newTextinput("No public key yet — press Ctrl+G"),
		PrivateKey:   newTextinput("No private key yet — press Ctrl+G"),
		Message:      newTextarea("Type a message to sign, or switch SOURCE to File..."),
		Signature:    newSignatureTextarea("Signature r||s will appear here after RUN..."),
		Filepicker:   newFilepicker(),
		SaveFilename: newTextinput("signature.txt"),
	}
	p.PrivateKey.EchoMode = textinput.EchoPassword
	p.PrivateKey.EchoCharacter = '•'
	p.updateFocus()
	return p
}

func (p SignPane) Init() tea.Cmd {
	return p.Filepicker.Init()
}

func (p SignPane) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width, p.Height = msg.Width, msg.Height

		l := NewLayout(msg.Width, msg.Height)
		p.Message.SetWidth(l.TextareaWidth)
		p.Message.SetHeight(l.TextareaHeight)
		p.Signature.SetWidth(l.RightPanelWidth - 4)
		p.Signature.SetHeight(l.SignatureHeight)
		p.Filepicker.SetHeight(l.FilepickerHeight)
		p.PublicKey.SetWidth(l.KeyFieldWidth)
		p.PrivateKey.SetWidth(l.KeyFieldWidth)
		p.SaveFilename.SetWidth(34)

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

	case tea.KeyPressMsg:
		// Save popup owns the keyboard while open.
		if p.ShowSavePopup {
			switch msg.String() {
			case "esc":
				p.ShowSavePopup = false
				p.SaveFilename.Blur()
				return p, nil
			case "enter":
				filename := p.SaveFilename.Value()
				if filename == "" {
					filename = "signature.txt"
				}

				sig := strings.TrimSpace(p.Signature.Value())
				if sig == "" {
					p.setError("No signature to save")
				} else if err := os.WriteFile(filename, []byte(sig+"\n"), 0o644); err != nil {
					p.setError("Save failed: " + err.Error())
				} else {
					p.setStatus("Saved to: " + filename)
				}

				p.ShowSavePopup = false
				p.SaveFilename.Blur()
				return p, p.clearStatusCmd()
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
		case key.Matches(msg, p.PaneKeymap.prepareVerify):
			return p, p.prepareForVerify()
		case key.Matches(msg, p.PaneKeymap.clearMessage):
			p.Message.SetValue("")
			p.Focus = FocusMessage
			p.updateFocus()
			return p, nil
		}

		// File picker navigation.
		if p.InputMode == FileInput && p.Focus == FocusMessage {
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
		case p.Focus == FocusParameters && msg.String() == "enter":
			if n := len(p.Scheme.KeyOptions()); n > 1 {
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
			p.toggleMode()
			return p, nil
		case p.Focus == FocusRun && msg.String() == "enter":
			return p, p.run()
		case p.Focus == FocusSave && msg.String() == "enter":
			p.ShowSavePopup = true
			p.SaveFilename.SetValue("")
			p.SaveFilename.Focus()
			return p, nil
		case p.Focus == FocusPublicKey && key.Matches(msg, p.PaneKeymap.clearKey):
			p.PublicKey.SetValue("")
			p.Verdict = VerdictNone
			p.VerdictReason = ""
			return p, nil
		case p.Focus == FocusPrivateKey && key.Matches(msg, p.PaneKeymap.clearKey):
			p.PrivateKey.SetValue("")
			return p, nil
		}
	}

	// Forward to the focused widget.
	switch p.Focus {
	case FocusMessage:
		if p.InputMode == ManualInput {
			before := p.Message.Value()
			var cmd tea.Cmd
			p.Message, cmd = p.Message.Update(msg)
			if p.Message.Value() != before {
				p.clearVerdict()
			}
			cmds = append(cmds, cmd)
		}
	case FocusSignature:
		// В режиме формирования поле подписи — вывод; редактируется
		// только при проверке.
		if p.Mode == Verify {
			before := p.Signature.Value()
			var cmd tea.Cmd
			p.Signature, cmd = p.Signature.Update(msg)
			if p.Signature.Value() != before {
				p.clearVerdict()
			}
			cmds = append(cmds, cmd)
		}
	case FocusPublicKey:
		before := p.PublicKey.Value()
		var cmd tea.Cmd
		p.PublicKey, cmd = p.PublicKey.Update(msg)
		if p.PublicKey.Value() != before {
			p.clearVerdict()
		}
		cmds = append(cmds, cmd)
	case FocusPrivateKey:
		var cmd tea.Cmd
		p.PrivateKey, cmd = p.PrivateKey.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

// ConsumesQuit tells whether a quit key should be handled by this pane instead of quitting the app.
func (p SignPane) ConsumesQuit(msg tea.KeyMsg) bool {
	return p.ShowSavePopup
}

func (p *SignPane) run() tea.Cmd {
	p.clearVerdict()
	p.Status = ""
	p.StatusError = false

	msg, err := p.readMessage()
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	if p.Mode == Sign {
		priv, err := decodeHex("private key", p.PrivateKey.Value())
		if err != nil {
			p.setError(err.Error())
			return p.clearStatusCmd()
		}

		sig, err := p.Scheme.Sign(priv, msg)
		if err != nil {
			p.setError("Signing failed: " + err.Error())
			return p.clearStatusCmd()
		}

		p.Signature.SetValue(hex.EncodeToString(sig))
		p.setStatus(fmt.Sprintf("Signature ready — %d B r||s", len(sig)))
		return p.clearStatusCmd()
	}

	pub, err := decodeHex("public key", p.PublicKey.Value())
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	sig, err := decodeSignature(p.Signature.Value())
	if err != nil {
		p.Verdict = VerdictInvalid
		p.VerdictReason = "некорректная подпись"
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	ok, err := p.Scheme.Verify(pub, msg, sig)
	switch {
	case err != nil:
		p.Verdict = VerdictInvalid
		p.VerdictReason = "ошибка проверки: " + err.Error()
		p.setError("Verification failed: " + err.Error())
	case ok:
		p.Verdict = VerdictValid
		p.VerdictReason = "R = r"
		p.setStatus("Signature is valid")
	default:
		p.Verdict = VerdictInvalid
		p.VerdictReason = "R ≠ r"
		p.setError("Signature is invalid")
	}
	return p.clearStatusCmd()
}

func (p SignPane) readMessage() ([]byte, error) {
	if p.InputMode == FileInput {
		if p.SelectedFile == "" {
			return nil, errors.New("no file selected")
		}
		content, err := os.ReadFile(p.SelectedFile)
		if err != nil {
			return nil, errors.New("cannot read file: " + err.Error())
		}
		return content, nil
	}
	return []byte(p.Message.Value()), nil
}

func (p *SignPane) startGeneration() tea.Cmd {
	if p.Generating {
		return nil
	}

	options := p.Scheme.KeyOptions()
	if len(options) == 0 {
		p.setError("No key presets configured")
		return p.clearStatusCmd()
	}
	if p.KeyPreset >= len(options) {
		p.KeyPreset = 0
	}

	scheme := p.Scheme
	option := options[p.KeyPreset]
	p.Generating = true
	p.setStatus("Generating " + option.Label + " keypair…")

	return func() tea.Msg {
		keys, err := scheme.GenerateKeys(option)
		return keysGeneratedMsg{keys: keys, err: err}
	}
}

func (p *SignPane) prepareForVerify() tea.Cmd {
	if strings.TrimSpace(p.Signature.Value()) == "" {
		p.setError("No signature yet — press RUN in sign mode first")
		return p.clearStatusCmd()
	}

	p.Mode = Verify
	p.clearVerdict()
	p.Focus = FocusSignature
	p.updateFocus()
	p.setStatus("Verify mode — the signature is kept; press RUN to check")
	return p.clearStatusCmd()
}

func (p *SignPane) toggleInputMode() {
	if p.InputMode == ManualInput {
		p.InputMode = FileInput
	} else {
		p.InputMode = ManualInput
	}
	p.FileConfirmed = false
	p.SelectedFile = ""
	p.updateFocus()
}

func (p *SignPane) toggleMode() {
	if p.Mode == Sign {
		p.Mode = Verify
	} else {
		p.Mode = Sign
	}
	p.clearVerdict()
	p.updateFocus()
}

func (p *SignPane) clearVerdict() {
	p.Verdict = VerdictNone
	p.VerdictReason = ""
}

func (p *SignPane) togglePrivate() {
	if p.PrivateVisible {
		p.hidePrivate()
		return
	}
	p.PrivateVisible = true
	p.PrivateKey.EchoMode = textinput.EchoNormal
}

func (p *SignPane) hidePrivate() {
	p.PrivateVisible = false
	p.PrivateKey.EchoMode = textinput.EchoPassword
	p.PrivateKey.EchoCharacter = '•'
}

func (p *SignPane) updateFocus() {
	p.Message.Blur()
	p.Signature.Blur()
	p.PublicKey.Blur()
	p.PrivateKey.Blur()
	p.SaveFilename.Blur()

	switch p.Focus {
	case FocusMessage:
		if p.InputMode == ManualInput {
			p.Message.Focus()
		}
	case FocusSignature:
		if p.Mode == Verify {
			p.Signature.Focus()
		}
	case FocusPublicKey:
		p.PublicKey.Focus()
	case FocusPrivateKey:
		p.PrivateKey.Focus()
	}
}

func (p *SignPane) setStatus(text string) {
	p.Status = text
	p.StatusError = false
}

func (p *SignPane) setError(text string) {
	p.Status = text
	p.StatusError = true
}

func (p SignPane) clearStatusCmd() tea.Cmd {
	text := p.Status
	if text == "" {
		return nil
	}
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return statusMsg(text)
	})
}

func (p SignPane) View() string {
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
			labelStyle.Render("SAVE SIGNATURE TO FILE"),
			"",
			"Filename:",
			p.SaveFilename.View(),
			"",
			mutedStyle.Render("Enter to save  •  Esc to cancel"),
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

	return base
}

func (p SignPane) keysView(l Layout) string {
	keyContentWidth := l.PanelWidth - 4

	title := labelStyle.Render("KEYRING") + mutedStyle.Render(" · "+p.Scheme.Name())

	options := p.Scheme.KeyOptions()
	presetText := "n/a"
	if len(options) > 0 && p.KeyPreset < len(options) {
		presetText = options[p.KeyPreset].Label
	}
	if p.Focus == FocusParameters {
		presetText = selectedStyle.Render(presetText)
	}
	sizeBlock := labelStyle.Render("PARAMETERS ") + presetText + mutedStyle.Render(" (Enter)")
	header := spread(title, sizeBlock, keyContentWidth)

	pubStyle := labelStyle
	if p.Mode == Sign {
		pubStyle = pubStyle.Foreground(lipgloss.Color("212"))
	}
	pubLabel := pubStyle.Render("PUBLIC KEY")

	privState := "hidden"
	if p.PrivateVisible {
		privState = "visible"
	}
	privStyle := labelStyle
	if p.Mode == Verify {
		privStyle = privStyle.Foreground(lipgloss.Color("212"))
	}
	privLabel := privStyle.Render("PRIVATE KEY") + mutedStyle.Render(" · "+privState)

	pubLabel = ansi.Truncate(pubLabel, l.KeyColumnWidth, "…")
	privLabel = ansi.Truncate(privLabel, l.KeyColumnWidth, "…")

	pubColumn := lipgloss.NewStyle().
		Width(l.KeyColumnWidth).
		Render(pubLabel + "\n" + p.PublicKey.View())
	privColumn := lipgloss.NewStyle().
		Width(l.KeyColumnWidth).
		Render(privLabel + "\n" + p.PrivateKey.View())
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
	footer := spread(generate, mutedStyle.Render("Ctrl+G generate · Ctrl+R reveal"), keyContentWidth)

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		fieldsRow,
		"",
		footer,
	)

	// lipgloss v2 sizes the whole block, border included; the layout numbers
	// used here are v1 dimensions that exclude it, so the border frame is added.
	return lipgloss.NewStyle().
		Width(keyContentWidth+4).
		Height(l.KeyPanelHeight+2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(keyBorderColor).
		Padding(0, 1).
		Render(content)
}

func (p SignPane) messageView(l Layout) string {
	inputTitle := "MESSAGE"
	inputContent := p.Message.View()

	switch {
	case p.InputMode == FileInput && p.FileConfirmed:
		inputTitle = "FILE SELECTED"

		style := selectedStyle
		if p.Focus != FocusMessage {
			style = mutedStyle
		}

		selectedLine := style.Render("Selected: " + p.SelectedFile)
		hintLine := mutedStyle.Render("Backspace to change file")
		inputContent = lipgloss.JoinVertical(lipgloss.Left, selectedLine, "", hintLine)
	case p.InputMode == FileInput:
		inputTitle = "FILE PICKER"
		inputContent = p.Filepicker.View()
	}

	leftContent := labelStyle.Render(inputTitle) + "\n" + inputContent
	leftPanel := p.messagePanel(l.LeftPanelWidth, l.MessagePanelHeight, p.Focus == FocusMessage).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, leftContent))

	sigTitle := "SIGNATURE"
	if p.Mode == Sign {
		sigTitle += " · output (hex r||s)"
	} else {
		sigTitle += " · input (hex r||s)"
	}
	rightContent := lipgloss.JoinVertical(
		lipgloss.Left,
		labelStyle.Render(sigTitle),
		p.Signature.View(),
		ansi.Truncate(p.verdictView(), l.RightPanelWidth-4, "…"),
	)
	rightPanel := p.messagePanel(l.RightPanelWidth, l.MessagePanelHeight, p.Focus == FocusSignature).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, rightContent))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (p SignPane) verdictView() string {
	switch p.Verdict {
	case VerdictValid:
		return verdictValidStyle.Render("ПОДПИСЬ ВЕРНА") + mutedStyle.Render(" · "+p.VerdictReason)
	case VerdictInvalid:
		return verdictInvalidStyle.Render("ПОДПИСЬ НЕВЕРНА") + mutedStyle.Render(" · "+p.VerdictReason)
	default:
		if p.Mode == Sign {
			return mutedStyle.Render("Подпись появится после RUN")
		}
		return mutedStyle.Render("Введите подпись r||s и нажмите RUN")
	}
}

func (p SignPane) actionView(l Layout) string {
	source := "Manual"
	if p.InputMode == FileInput {
		source = "File"
	}
	if p.Focus == FocusSource {
		source = selectedStyle.Render(source) + mutedStyle.Render(" Enter")
	}

	mode := "Sign"
	keyHint := "private key"
	if p.Mode == Verify {
		mode = "Verify"
		keyHint = "public key"
	}
	if p.Focus == FocusMode {
		mode = selectedStyle.Render(mode) + mutedStyle.Render(" Enter")
	}

	controls := lipgloss.JoinHorizontal(
		lipgloss.Top,
		labelStyle.Render("SOURCE "), source, "   ",
		labelStyle.Render("MODE "), mode, mutedStyle.Render(" ("+keyHint+")"), "   ",
		p.actionButton("RUN", p.Focus == FocusRun, buttonStyle), "  ",
		p.actionButton("SAVE TO FILE", p.Focus == FocusSave, saveBtnStyle),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ansi.Truncate(controls, l.PanelWidth, "…"),
		p.hintLine(l.PanelWidth),
	)
}

func (p SignPane) hintLine(width int) string {
	style := mutedStyle
	text := "Ctrl+G keys · Ctrl+R private · Ctrl+D to verify · Alt+D clear message"
	if p.Status != "" {
		text = p.Status
		if p.StatusError {
			style = errorStyle
		} else {
			style = statusStyle
		}
	}
	return style.Render(ansi.Truncate(text, width, "…"))
}

func (p SignPane) actionButton(text string, focused bool, style lipgloss.Style) string {
	if focused {
		return style.Render(text)
	}
	return mutedStyle.Render("[ " + text + " ]")
}

func (p SignPane) messagePanel(totalWidth, contentHeight int, emphasized bool) lipgloss.Style {
	border := panelBorderColor
	if emphasized {
		border = keyBorderColor
	}

	return lipgloss.NewStyle().
		Width(totalWidth).
		Height(contentHeight+2).
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

// decodeSignature разбирает подпись r||s, пропуская пробелы и переводы
// строк, которые появляются при вставке в поле подписи.
func decodeSignature(value string) ([]byte, error) {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, value)

	if cleaned == "" {
		return nil, errors.New("signature is empty")
	}
	decoded, err := hex.DecodeString(cleaned)
	if err != nil {
		return nil, errors.New("signature must be hex r||s")
	}
	return decoded, nil
}
