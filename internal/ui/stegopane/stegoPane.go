package stegopane

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go-cipher/internal/crypto/stego"
	"go-cipher/internal/ui"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/NimbleMarkets/ntcharts/v2/picture"
	"github.com/charmbracelet/x/ansi"
)

const diffGain = 20.0

func NewStegoPane() StegoPane {
	p := StegoPane{
		Focus:           FocusSource,
		Message:         newMessageTextarea(),
		Filepicker:      newFilepicker(),
		Mode:            ModeEmbed,
		PreviewOriginal: newPicture(),
		PreviewStego:    newPicture(),
		PreviewDiff:     newPicture(),
		PaneKeymap:      newPaneKeymap(),
	}
	p.Width, p.Height = 80, 24
	p.applyLayout(NewLayout(p.Width, p.Height, false))
	p.updateFocus()
	return p
}

func (p StegoPane) Init() tea.Cmd {
	return p.Filepicker.Init()
}

func (p StegoPane) Update(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.Width, p.Height = msg.Width, msg.Height
		cmds = append(cmds, p.applyLayout(NewLayout(msg.Width, msg.Height, p.PickerOpen)))
		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(msg)
		cmds = append(cmds, cmd)

	case statusMsg:
		if p.Status == string(msg) {
			p.Status = ""
			p.StatusError = false
		}

	case tea.KeyPressMsg:
		return p.handleKey(msg, cmds)

	default:
		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(msg)
		cmds = append(cmds, cmd)
	}

	p.forwardToPictures(msg, &cmds)
	return p, tea.Batch(cmds...)
}

func (p StegoPane) handleKey(msg tea.KeyPressMsg, cmds []tea.Cmd) (ui.Pane, tea.Cmd) {
	if p.PickerOpen {
		if msg.String() == "esc" {
			p.PickerOpen = false
			p.Filepicker = newFilepicker()
			cmds = append(cmds, p.applyLayout(p.layout()))
			p.forwardToPictures(msg, &cmds)
			return p, tea.Batch(cmds...)
		}

		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(msg)
		cmds = append(cmds, cmd)

		if selected, path := p.Filepicker.DidSelectFile(msg); selected {
			p.PickerOpen = false
			cmds = append(cmds, p.loadSource(path))
			cmds = append(cmds, p.applyLayout(p.layout()))
		}

		p.forwardToPictures(msg, &cmds)
		return p, tea.Batch(cmds...)
	}

	switch {
	case key.Matches(msg, p.PaneKeymap.next):
		p.Focus = (p.Focus + 1) % FocusCount
		p.updateFocus()
		p.forwardToPictures(msg, &cmds)
		return p, tea.Batch(cmds...)
	case key.Matches(msg, p.PaneKeymap.prev):
		p.Focus = (p.Focus - 1 + FocusCount) % FocusCount
		p.updateFocus()
		p.forwardToPictures(msg, &cmds)
		return p, tea.Batch(cmds...)
	case key.Matches(msg, p.PaneKeymap.clearMessage):
		p.Message.SetValue("")
		p.setStatus("Message cleared")
		p.forwardToPictures(msg, &cmds)
		return p, tea.Batch(append(cmds, p.clearStatusCmd())...)
	case key.Matches(msg, p.PaneKeymap.extractedToMessage):
		if p.LastExtracted == nil {
			p.setError("Nothing extracted yet")
		} else {
			p.Message.SetValue(string(p.LastExtracted))
			p.Message.CursorEnd()
			p.setStatus("Extracted message moved to MESSAGE")
		}
		p.forwardToPictures(msg, &cmds)
		return p, tea.Batch(append(cmds, p.clearStatusCmd())...)
	}

	if msg.String() == "enter" {
		switch p.Focus {
		case FocusSource:
			p.PickerOpen = true
			p.Filepicker = newFilepicker()
			p.Filepicker.SetHeight(FilepickerHeight)
			cmds = append(cmds, p.Filepicker.Init())
			cmds = append(cmds, p.applyLayout(p.layout()))
			p.forwardToPictures(msg, &cmds)
			return p, tea.Batch(cmds...)
		case FocusMode:
			if p.Mode == ModeEmbed {
				p.Mode = ModeExtract
			} else {
				p.Mode = ModeEmbed
			}
			p.forwardToPictures(msg, &cmds)
			return p, tea.Batch(cmds...)
		case FocusRun:
			cmds = append(cmds, p.run())
			p.forwardToPictures(msg, &cmds)
			return p, tea.Batch(cmds...)
		case FocusSave:
			cmds = append(cmds, p.saveMessage())
			p.forwardToPictures(msg, &cmds)
			return p, tea.Batch(cmds...)
		}
	}

	if p.Focus == FocusMessage {
		var cmd tea.Cmd
		p.Message, cmd = p.Message.Update(msg)
		cmds = append(cmds, cmd)
	}

	p.forwardToPictures(msg, &cmds)
	return p, tea.Batch(cmds...)
}

// ConsumesQuit keeps Esc inside the pane while the file picker is open.
func (p StegoPane) ConsumesQuit(msg tea.KeyMsg) bool {
	return p.PickerOpen
}

func (p *StegoPane) forwardToPictures(msg tea.Msg, cmds *[]tea.Cmd) {
	*cmds = append(*cmds,
		p.PreviewOriginal.Update(msg),
		p.PreviewStego.Update(msg),
		p.PreviewDiff.Update(msg),
	)
}

func (p *StegoPane) layout() Layout {
	return NewLayout(p.Width, p.Height, p.PickerOpen)
}

func (p *StegoPane) applyLayout(l Layout) tea.Cmd {
	p.Message.SetWidth(l.TextareaWidth)
	p.Message.SetHeight(l.TextareaHeight)
	p.Filepicker.SetHeight(FilepickerHeight)

	p.PreviewColumns = l.ColumnWidth
	p.PreviewRows = l.PreviewRows

	return tea.Batch(
		p.PreviewOriginal.SetSize(l.ColumnWidth, l.PreviewRows),
		p.PreviewStego.SetSize(l.ColumnWidth, l.PreviewRows),
		p.PreviewDiff.SetSize(l.ColumnWidth, l.PreviewRows),
	)
}

func (p *StegoPane) loadSource(path string) tea.Cmd {
	img, err := stego.DecodeFile(path)
	if err != nil {
		p.SelectedFile = ""
		p.OriginalImage = nil
		p.StegoImage = nil
		p.DiffImage = nil
		p.Container = ""
		p.LastExtracted = nil
		p.HasExtracted = false
		p.Stats = stego.Stats{}
		p.setError("Cannot open image: " + err.Error())
		return tea.Batch(
			p.PreviewOriginal.SetImage(nil),
			p.PreviewStego.SetImage(nil),
			p.PreviewDiff.SetImage(nil),
			p.clearStatusCmd(),
		)
	}

	p.SelectedFile = path
	p.OriginalImage = img
	p.StegoImage = nil
	p.DiffImage = nil
	p.Container = ""
	p.LastExtracted = nil
	p.HasExtracted = false
	p.Stats = stego.Stats{CapacityBits: stego.Capacity(img)}
	p.setStatus(fmt.Sprintf(
		"Loaded %s · capacity %d B",
		filepath.Base(path), stego.Capacity(img)/8,
	))

	return tea.Batch(
		p.PreviewOriginal.SetImage(img),
		p.PreviewStego.SetImage(nil),
		p.PreviewDiff.SetImage(nil),
		p.clearStatusCmd(),
	)
}

func (p *StegoPane) run() tea.Cmd {
	if p.SelectedFile == "" {
		p.setError("Select a source image first: Enter on SOURCE")
		return p.clearStatusCmd()
	}

	if p.Mode == ModeExtract {
		return p.runExtract()
	}
	return p.runEmbed()
}

func (p *StegoPane) runEmbed() tea.Cmd {
	img, err := stego.DecodeFile(p.SelectedFile)
	if err != nil {
		p.setError("Cannot read source: " + err.Error())
		return p.clearStatusCmd()
	}

	message := []byte(p.Message.Value())
	container, stats, err := stego.Embed(img, message)
	if err != nil {
		p.setError("Embedding failed: " + err.Error())
		return p.clearStatusCmd()
	}

	path := stego.OutputPath(p.SelectedFile)
	if err := stego.WritePNG(path, container); err != nil {
		p.setError("Cannot save container: " + err.Error())
		return p.clearStatusCmd()
	}

	p.OriginalImage = img
	p.StegoImage = container
	p.DiffImage = differenceImage(img, container, diffGain)
	p.Stats = stats
	p.Container = path
	p.setStatus(fmt.Sprintf(
		"Embedded %d B into %s · PSNR %.2f dB · verified",
		len(message), filepath.Base(path), stats.PSNR,
	))

	return tea.Batch(
		p.PreviewOriginal.SetImage(img),
		p.PreviewStego.SetImage(container),
		p.PreviewDiff.SetImage(p.DiffImage),
		p.clearStatusCmd(),
	)
}

func (p *StegoPane) runExtract() tea.Cmd {
	img, err := stego.DecodeFile(p.SelectedFile)
	if err != nil {
		p.setError("Cannot read source: " + err.Error())
		return p.clearStatusCmd()
	}

	payload, err := stego.Extract(img)
	if err != nil {
		p.setError("Extraction failed: " + err.Error())
		return p.clearStatusCmd()
	}

	p.LastExtracted = payload
	p.HasExtracted = true
	p.Message.SetValue(string(payload))
	p.Message.CursorEnd()
	p.OriginalImage = img
	p.StegoImage = nil
	p.DiffImage = nil
	p.Stats = stego.Stats{CapacityBits: stego.Capacity(img)}
	p.setStatus(fmt.Sprintf(
		"Extracted %d B from %s",
		len(payload), filepath.Base(p.SelectedFile),
	))

	return tea.Batch(
		p.PreviewOriginal.SetImage(img),
		p.PreviewStego.SetImage(nil),
		p.PreviewDiff.SetImage(nil),
		p.clearStatusCmd(),
	)
}

func (p *StegoPane) saveMessage() tea.Cmd {
	text := p.Message.Value()
	if text == "" {
		p.setError("Message is empty")
		return p.clearStatusCmd()
	}

	dir := "."
	if p.SelectedFile != "" {
		dir = filepath.Dir(p.SelectedFile)
	}
	path := filepath.Join(dir, "message.txt")

	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		p.setError("Save failed: " + err.Error())
		return p.clearStatusCmd()
	}

	p.setStatus("Message saved to " + path)
	return p.clearStatusCmd()
}

func (p *StegoPane) updateFocus() {
	p.Message.Blur()
	if p.Focus == FocusMessage {
		p.Message.Focus()
	}
}

func (p *StegoPane) setStatus(text string) {
	p.Status = text
	p.StatusError = false
}

func (p *StegoPane) setError(text string) {
	p.Status = text
	p.StatusError = true
}

func (p StegoPane) clearStatusCmd() tea.Cmd {
	text := p.Status
	if text == "" {
		return nil
	}
	return tea.Tick(4*time.Second, func(time.Time) tea.Msg {
		return statusMsg(text)
	})
}

func (p StegoPane) View() string {
	l := p.layout()

	return lipgloss.JoinVertical(
		lipgloss.Left,
		p.sourceView(l),
		p.messageView(l),
		p.actionView(l),
		p.previewView(l),
	)
}

func (p StegoPane) sourceView(l Layout) string {
	path := p.SelectedFile
	if path == "" {
		path = "no file selected"
	}

	pathLine := labelStyle.Render("SOURCE ") + selectedStyle.Render(path)
	if p.Focus == FocusSource {
		pathLine += mutedStyle.Render("  Enter picker")
	} else {
		pathLine += mutedStyle.Render("  (Enter on SOURCE opens picker)")
	}
	pathLine = ansi.Truncate(pathLine, l.PanelWidth, "…")

	capacity := stego.Capacity(p.OriginalImage)
	capacityText := "n/a"
	payloadText := "0.0%"
	payloadStyle := selectedStyle

	if capacity > 0 {
		capacityText = fmt.Sprintf("%d B (%d bits)", capacity/8, capacity)
		usedBytes := len(p.Message.Value())
		percent := 100 * float64(usedBytes) / float64(capacity/8)
		payloadText = fmt.Sprintf("%.1f%%", percent)
		if percent > 100 {
			payloadStyle = errorStyle
		}
	}

	params := labelStyle.Render("CAPACITY ") + capacityText +
		mutedStyle.Render("   PAYLOAD ") + payloadStyle.Render(payloadText)
	params = ansi.Truncate(params, l.PanelWidth, "…")

	lines := []string{pathLine, params}
	if p.PickerOpen {
		p.Filepicker.SetHeight(FilepickerHeight)
		lines = append(lines, p.Filepicker.View())
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (p StegoPane) messageView(l Layout) string {
	leftWidth := l.PanelWidth / 2
	rightWidth := l.PanelWidth - leftWidth

	leftContent := labelStyle.Render("MESSAGE") + "\n" + p.Message.View()
	leftPanel := p.panel(leftWidth, l.MessagePanelHeight, p.Focus == FocusMessage).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight-2, lipgloss.Top, leftContent))

	rightContent := labelStyle.Render("STATS") + "\n" + p.statsView(rightWidth-4)
	rightPanel := p.panel(rightWidth, l.MessagePanelHeight, false).
		Render(lipgloss.PlaceVertical(l.MessagePanelHeight-2, lipgloss.Top, rightContent))

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (p StegoPane) statsView(width int) string {
	if p.OriginalImage == nil {
		return mutedStyle.Render("No image selected")
	}
	if p.Stats.UsedBits == 0 {
		if p.HasExtracted {
			return strings.Join([]string{
				fmt.Sprintf("EXTRACTED   %d B", len(p.LastExtracted)),
				fmt.Sprintf("CAPACITY    %d B", p.Stats.CapacityBits/8),
				mutedStyle.Render("Container ") + filepath.Base(p.SelectedFile),
			}, "\n")
		}
		return mutedStyle.Render("Not embedded yet")
	}

	lines := []string{
		fmt.Sprintf("PSNR        %.2f dB", p.Stats.PSNR),
		fmt.Sprintf("CHANGED     %d px", p.Stats.ChangedPixels),
		fmt.Sprintf("MAX |delta| %.2f", p.Stats.MaxDelta),
		fmt.Sprintf("BLOCKS      %d / %d", p.Stats.UsedBlocks, p.Stats.Blocks),
	}
	if p.Container != "" {
		lines = append(lines, mutedStyle.Render("CONTAINER ")+filepath.Base(p.Container))
	}

	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "…")
	}
	return strings.Join(lines, "\n")
}

func (p StegoPane) actionView(l Layout) string {
	mode := "Embed"
	if p.Mode == ModeExtract {
		mode = "Extract"
	}
	if p.Focus == FocusMode {
		mode = selectedStyle.Render(mode) + mutedStyle.Render(" Enter")
	}

	controls := lipgloss.JoinHorizontal(
		lipgloss.Top,
		labelStyle.Render("MODE "), mode, "   ",
		p.actionButton("RUN", p.Focus == FocusRun, buttonStyle), "  ",
		p.actionButton("SAVE MESSAGE TO FILE", p.Focus == FocusSave, saveBtnStyle),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		ansi.Truncate(controls, l.PanelWidth, "…"),
		p.hintLine(l.PanelWidth),
	)
}

func (p StegoPane) actionButton(text string, focused bool, style lipgloss.Style) string {
	if focused {
		return style.Render(text)
	}
	return mutedStyle.Render("[ " + text + " ]")
}

func (p StegoPane) hintLine(width int) string {
	if p.Status != "" {
		style := statusStyle
		if p.StatusError {
			style = errorStyle
		}
		return style.Render(ansi.Truncate(p.Status, width, "…"))
	}
	hint := "Tab fields · Enter activate · Alt+D clear · Ctrl+D extracted→message · Esc quit"
	return mutedStyle.Render(ansi.Truncate(hint, width, "…"))
}

func (p StegoPane) previewView(l Layout) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		p.previewColumn("ORIGINAL", p.OriginalImage, &p.PreviewOriginal, l),
		"  ",
		p.previewColumn("STEGO", p.StegoImage, &p.PreviewStego, l),
		"  ",
		p.previewColumn("DIFF ×20", p.DiffImage, &p.PreviewDiff, l),
	)
}

func (p StegoPane) previewColumn(label string, img image.Image, model *picture.Model, l Layout) string {
	header := labelStyle.Render(label)
	if img != nil {
		bounds := img.Bounds()
		header += mutedStyle.Render(fmt.Sprintf(" %d×%d", bounds.Dx(), bounds.Dy()))
	}
	header = ansi.Truncate(header, l.ColumnWidth, "…")
	header = lipgloss.NewStyle().Width(l.ColumnWidth).Render(header)

	content := model.View().Content
	if img == nil || content == "" {
		content = mutedStyle.Render("—")
	}

	return header + "\n" + padBlock(content, l.PreviewRows)
}

func padBlock(content string, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (p StegoPane) panel(width, height int, focused bool) lipgloss.Style {
	border := panelBorderColor
	if focused {
		border = cursorColor
	}
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(border).
		Padding(0, 1)
}
