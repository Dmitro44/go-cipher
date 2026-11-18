package hashpane

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"os"
	"strings"
	"time"

	"go-cipher/internal/crypto/hash"
	"go-cipher/internal/ui"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textarea"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type HashPane struct {
	ui.PaneBase
	Algorithms []hash.Algorithm
	Algorithm  int

	Input      textarea.Model
	DigestView textarea.Model
	Expected   textinput.Model
	Digest     string

	InputMode     InputMode
	Filepicker    filepicker.Model
	SelectedFile  string
	FileConfirmed bool

	SaveFilename  textinput.Model
	ShowSavePopup bool

	StatusError bool
	PaneKeymap  PaneKeymap
}

func NewHashPane() HashPane {
	p := HashPane{
		Algorithms: hash.Algorithms,
		Focus:      FocusInput,
		PaneKeymap: newPaneKeymap(),
		Input:      newInput("Type a message to hash, or switch SOURCE to File..."),
		DigestView: newDigestView("Press HASH to compute the digest..."),
		Expected:   newTextinput("Expected digest in hex"),
		Filepicker: newFilepicker(),
		SaveFilename: newTextinput(
			"digest.txt",
		),
	}
	p.updateFocus()
	return p
}

func (p HashPane) Init() tea.Cmd {
	return p.Filepicker.Init()
}

func (p HashPane) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width, p.Height = msg.Width, msg.Height

		l := NewLayout(msg.Width, msg.Height)
		p.Input.SetWidth(l.TextareaWidth)
		p.Input.SetHeight(l.TextareaHeight)
		p.DigestView.SetWidth(l.RightPanelWidth - 4)
		p.DigestView.SetHeight(l.DigestHeight)
		p.Expected.SetWidth(l.RightPanelWidth - 4)
		p.Filepicker.SetHeight(l.FilepickerHeight)
		p.SaveFilename.SetWidth(34)

	case statusMsg:
		if p.Status == string(msg) {
			p.Status = ""
			p.StatusError = false
		}
		return p, nil

	case tea.KeyPressMsg:
		if p.ShowSavePopup {
			switch msg.String() {
			case "esc":
				p.ShowSavePopup = false
				p.SaveFilename.Blur()
				return p, nil
			case "enter":
				filename := p.SaveFilename.Value()
				if filename == "" {
					filename = "digest.txt"
				}

				err := os.WriteFile(filename, []byte(p.Digest+"\n"), 0o644)
				if err != nil {
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

		switch {
		case key.Matches(msg, p.PaneKeymap.copyDigest):
			if p.Digest == "" {
				p.setError("No digest yet — press HASH first")
				return p, p.clearStatusCmd()
			}
			p.Expected.SetValue(p.Digest)
			p.setStatus("Digest copied to EXPECTED")
			return p, p.clearStatusCmd()
		case key.Matches(msg, p.PaneKeymap.clearInput):
			p.Input.SetValue("")
			p.Focus = FocusInput
			p.updateFocus()
			return p, nil
		}

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

		switch {
		case p.Focus == FocusAlgorithm && msg.String() == "enter":
			p.Algorithm = (p.Algorithm + 1) % len(p.Algorithms)
			p.Digest = ""
			p.DigestView.SetValue("")
			return p, nil
		case p.Focus == FocusSource && msg.String() == "enter":
			p.toggleInputMode()
			return p, p.Filepicker.Init()
		case p.Focus == FocusHash && msg.String() == "enter":
			return p, p.runHash()
		case p.Focus == FocusAvalanche && msg.String() == "enter":
			return p, p.runAvalanche()
		case p.Focus == FocusSave && msg.String() == "enter":
			if p.Digest == "" {
				p.setError("No digest to save — press HASH first")
				return p, p.clearStatusCmd()
			}
			p.ShowSavePopup = true
			p.SaveFilename.SetValue("")
			p.SaveFilename.Focus()
			return p, nil
		}
	}

	switch p.Focus {
	case FocusInput:
		if p.InputMode == ManualInput {
			var cmd tea.Cmd
			p.Input, cmd = p.Input.Update(msg)
			cmds = append(cmds, cmd)
		}
	case FocusExpected:
		var cmd tea.Cmd
		p.Expected, cmd = p.Expected.Update(msg)
		cmds = append(cmds, cmd)
	}

	return p, tea.Batch(cmds...)
}

func (p HashPane) ConsumesQuit(msg tea.KeyMsg) bool {
	return p.ShowSavePopup
}

func (p HashPane) currentAlgorithm() hash.Algorithm {
	return p.Algorithms[p.Algorithm]
}

func (p *HashPane) runHash() tea.Cmd {
	message, err := p.readInput()
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	alg := p.currentAlgorithm()
	digest := sum(alg, message)
	p.Digest = hex.EncodeToString(digest)
	p.DigestView.SetValue(p.Digest)
	p.setStatus(fmt.Sprintf("%s: %d B вход → %d B дайджест", alg.Name, len(message), len(digest)))
	return p.clearStatusCmd()
}

func (p *HashPane) runAvalanche() tea.Cmd {
	message, err := p.readInput()
	if err != nil {
		p.setError(err.Error())
		return p.clearStatusCmd()
	}

	flipped := append([]byte(nil), message...)
	flipped[0] ^= 0x01

	alg := p.currentAlgorithm()
	first := sum(alg, message)
	second := sum(alg, flipped)

	changed := 0
	for i := range first {
		changed += bits.OnesCount8(first[i] ^ second[i])
	}

	p.Digest = hex.EncodeToString(first)
	p.DigestView.SetValue(p.Digest)

	percent := 100 * float64(changed) / float64(len(first)*8)
	p.setStatus(fmt.Sprintf("1 бит входа → %d из %d бит выхода (%.1f %%)", changed, len(first)*8, percent))
	return p.clearStatusCmd()
}

func sum(alg hash.Algorithm, message []byte) []byte {
	h := alg.New()
	h.Write(message)
	return h.Sum(nil)
}

func (p HashPane) readInput() ([]byte, error) {
	if p.InputMode == FileInput {
		if p.SelectedFile == "" {
			return nil, errors.New("no file selected")
		}
		content, err := os.ReadFile(p.SelectedFile)
		if err != nil {
			return nil, errors.New("cannot read file: " + err.Error())
		}
		if len(content) == 0 {
			return nil, errors.New("file is empty")
		}
		return content, nil
	}

	value := p.Input.Value()
	if len(value) == 0 {
		return nil, errors.New("input is empty")
	}
	return []byte(value), nil
}

func (p *HashPane) toggleInputMode() {
	if p.InputMode == ManualInput {
		p.InputMode = FileInput
	} else {
		p.InputMode = ManualInput
	}
	p.FileConfirmed = false
	p.SelectedFile = ""
	p.updateFocus()
}

func (p *HashPane) updateFocus() {
	p.Input.Blur()
	p.Expected.Blur()

	switch p.Focus {
	case FocusInput:
		if p.InputMode == ManualInput {
			p.Input.Focus()
		}
	case FocusExpected:
		p.Expected.Focus()
	}
}

func (p *HashPane) setStatus(text string) {
	p.Status = text
	p.StatusError = false
}

func (p *HashPane) setError(text string) {
	p.Status = text
	p.StatusError = true
}

func (p HashPane) clearStatusCmd() tea.Cmd {
	text := p.Status
	if text == "" {
		return nil
	}
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return statusMsg(text)
	})
}

func (p HashPane) View() string {
	l := NewLayout(p.Width, p.Height)

	base := lipgloss.JoinVertical(
		lipgloss.Left,
		p.algorithmView(l),
		p.messageView(l),
		p.actionView(l),
	)

	if p.ShowSavePopup {
		popupContent := lipgloss.JoinVertical(
			lipgloss.Left,
			labelStyle.Render("SAVE DIGEST TO FILE"),
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

func (p HashPane) algorithmView(l Layout) string {
	contentWidth := l.PanelWidth - 4
	alg := p.currentAlgorithm()

	name := p.actionButton(alg.Name, p.Focus == FocusAlgorithm, buttonStyle)
	header := spread(labelStyle.Render("ALGORITHM")+"  "+name, mutedStyle.Render("ENTER to switch"), contentWidth)
	params := mutedStyle.Render(fmt.Sprintf(
		"OUTPUT %d bit  ·  BLOCK %d bit  ·  %d algorithms",
		alg.OutputBits, alg.BlockSize, len(p.Algorithms),
	))

	content := lipgloss.JoinVertical(lipgloss.Left, header, params)

	return lipgloss.NewStyle().
		Width(contentWidth+4).
		Height(l.AlgPanelHeight+2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(keyBorderColor).
		Padding(0, 1).
		Render(content)
}

func (p HashPane) messageView(l Layout) string {
	inputTitle := "INPUT"
	inputContent := p.Input.View()

	switch {
	case p.InputMode == FileInput && p.FileConfirmed:
		inputTitle = "FILE SELECTED"

		style := selectedStyle
		if p.Focus != FocusInput {
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
	leftPanel := p.messagePanel(l.LeftPanelWidth, l.MessagePanelHeight, p.Focus == FocusInput).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, leftContent))

	rightContent := lipgloss.JoinVertical(
		lipgloss.Left,
		labelStyle.Render("DIGEST"),
		p.DigestView.View(),
		labelStyle.Render("EXPECTED")+mutedStyle.Render(" (hex)"),
		p.Expected.View(),
		p.verdictView(),
	)
	rightPanel := p.messagePanel(l.RightPanelWidth, l.MessagePanelHeight, p.Focus == FocusExpected).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight, lipgloss.Top, rightContent))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (p HashPane) verdictView() string {
	text, match := p.verdict()
	if text == "" {
		return mutedStyle.Render("—")
	}
	if match {
		return matchStyle.Render(text)
	}
	return mismatchStyle.Render(text)
}

func (p HashPane) verdict() (string, bool) {
	expected := strings.TrimSpace(p.Expected.Value())
	if expected == "" || p.Digest == "" {
		return "", false
	}
	if strings.EqualFold(expected, p.Digest) {
		return "СОВПАДАЕТ", true
	}
	return "НЕ СОВПАДАЕТ", false
}

func (p HashPane) actionView(l Layout) string {
	source := "Manual"
	if p.InputMode == FileInput {
		source = "File"
	}
	if p.Focus == FocusSource {
		source = selectedStyle.Render(source) + mutedStyle.Render(" Enter")
	}

	controls := lipgloss.JoinHorizontal(
		lipgloss.Top,
		labelStyle.Render("SOURCE "), source, "   ",
		p.actionButton("HASH", p.Focus == FocusHash, buttonStyle), "  ",
		p.actionButton("AVALANCHE", p.Focus == FocusAvalanche, avalancheBtnStyle), "  ",
		p.actionButton("SAVE TO FILE", p.Focus == FocusSave, saveBtnStyle),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ansi.Truncate(controls, l.PanelWidth, "…"),
		p.hintLine(l.PanelWidth),
	)
}

func (p HashPane) hintLine(width int) string {
	if p.Status != "" {
		style := statusStyle
		if p.StatusError {
			style = errorStyle
		}
		return style.Render(ansi.Truncate(p.Status, width, "…"))
	}
	hint := "Tab switch · Ctrl+D digest→expected · Alt+D clear input"
	return mutedStyle.Render(ansi.Truncate(hint, width, "…"))
}

func (p HashPane) actionButton(text string, focused bool, style lipgloss.Style) string {
	if focused {
		return style.Render(text)
	}
	return mutedStyle.Render("[ " + text + " ]")
}

func (p HashPane) messagePanel(totalWidth, contentHeight int, emphasized bool) lipgloss.Style {
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
