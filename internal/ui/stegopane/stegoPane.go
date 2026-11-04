package stegopane

import (
	"errors"
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
		PreviewOriginal: newPicture(43),
		PreviewStego:    newPicture(44),
		PreviewDiff:     newPicture(45),
		PaneKeymap:      newPaneKeymap(),
	}
	p.Width, p.Height = 80, 24
	p.applyLayout(NewLayout(p.Width, p.Height, false))
	p.updateFocus()
	return p
}

func (p StegoPane) Init() tea.Cmd {
	// tmux hides the outer terminal's environment, so the Kitty probe has to
	// travel through the passthrough wrapper when we run inside a session.
	if os.Getenv("TMUX") != "" {
		picture.SetTmuxPassthrough(true)
	}
	return tea.Batch(p.Filepicker.Init(), picture.QueryKittySupport())
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

	case loadMsg:
		p.Busy = false
		if msg.err != nil {
			p.SelectedFile = ""
			p.OriginalImage = nil
			p.StegoImage = nil
			p.DiffImage = nil
			p.Container = ""
			p.LastExtracted = nil
			p.HasExtracted = false
			p.Stats = stego.Stats{}
			p.setError("Cannot open image: " + msg.err.Error())
			cmds = append(cmds,
				p.PreviewOriginal.SetImage(nil),
				p.PreviewStego.SetImage(nil),
				p.PreviewDiff.SetImage(nil),
				p.clearStatusCmd(),
			)
			break
		}
		p.SelectedFile = msg.path
		p.OriginalImage = msg.img
		p.StegoImage = nil
		p.DiffImage = nil
		p.Container = ""
		p.LastExtracted = nil
		p.HasExtracted = false
		p.Stats = stego.Stats{CapacityBits: stego.Capacity(msg.img)}
		p.setStatus(fmt.Sprintf(
			"Loaded %s · capacity %d B",
			filepath.Base(msg.path), stego.Capacity(msg.img)/8,
		))
		cmds = append(cmds,
			p.PreviewOriginal.SetImage(msg.img),
			p.PreviewStego.SetImage(nil),
			p.PreviewDiff.SetImage(nil),
			p.clearStatusCmd(),
		)

	case embedMsg:
		p.Busy = false
		if msg.err != nil {
			p.setError("Embedding failed: " + msg.err.Error())
			cmds = append(cmds, p.clearStatusCmd())
			break
		}
		p.OriginalImage = msg.img
		p.StegoImage = msg.container
		p.DiffImage = msg.diff
		p.Stats = msg.stats
		p.Container = msg.path
		p.setStatus(fmt.Sprintf(
			"Embedded %d B into %s · PSNR %.2f dB · verified",
			msg.size, filepath.Base(msg.path), msg.stats.PSNR,
		))
		cmds = append(cmds,
			p.PreviewOriginal.SetImage(msg.img),
			p.PreviewStego.SetImage(msg.container),
			p.PreviewDiff.SetImage(msg.diff),
			p.clearStatusCmd(),
		)

	case extractMsg:
		p.Busy = false
		if msg.err != nil {
			p.setError("Extraction failed: " + msg.err.Error())
			cmds = append(cmds, p.clearStatusCmd())
			break
		}
		p.LastExtracted = msg.payload
		p.HasExtracted = true
		p.Message.SetValue(string(msg.payload))
		p.Message.CursorEnd()
		p.OriginalImage = msg.img
		p.StegoImage = nil
		p.DiffImage = nil
		p.Stats = stego.Stats{CapacityBits: stego.Capacity(msg.img)}
		p.setStatus(fmt.Sprintf(
			"Extracted %d B from %s",
			len(msg.payload), filepath.Base(p.SelectedFile),
		))
		cmds = append(cmds,
			p.PreviewOriginal.SetImage(msg.img),
			p.PreviewStego.SetImage(nil),
			p.PreviewDiff.SetImage(nil),
			p.clearStatusCmd(),
		)

	case tea.KeyPressMsg:
		p.enableKitty(&cmds)
		return p.handleKey(msg, cmds)

	default:
		var cmd tea.Cmd
		p.Filepicker, cmd = p.Filepicker.Update(msg)
		cmds = append(cmds, cmd)
	}

	p.forwardToPictures(msg, &cmds)
	p.enableKitty(&cmds)
	return p, tea.Batch(cmds...)
}

// BackgroundUpdate feeds messages to the previews while another tab is open.
// The Kitty probe answers once, right after start, and the picture models are
// the only place that records the answer, so they have to see it happen.
func (p StegoPane) BackgroundUpdate(msg tea.Msg) (ui.Pane, tea.Cmd) {
	var cmds []tea.Cmd
	p.forwardToPictures(msg, &cmds)
	p.enableKitty(&cmds)
	return p, tea.Batch(cmds...)
}

// enableKitty switches the previews to the Kitty graphics protocol once the
// terminal probe has confirmed support. Until then (and on terminals without
// it) the previews stay in the universal half-block mode.
func (p *StegoPane) enableKitty(cmds *[]tea.Cmd) {
	if p.KittyOn || picture.KittySupported() != picture.KittyCapabilitySupported {
		return
	}
	p.KittyOn = true
	*cmds = append(*cmds,
		p.PreviewOriginal.Toggle(),
		p.PreviewStego.Toggle(),
		p.PreviewDiff.Toggle(),
	)
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
			cmds = append(cmds, p.applyLayout(p.layout()))
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

	p.PreviewRows = l.PreviewRows

	if p.Mode == ModeExtract {
		p.PreviewColumns = l.PanelWidth
		return tea.Batch(
			p.PreviewOriginal.SetSize(l.PanelWidth, l.PreviewRows),
			p.PreviewStego.SetSize(0, 0),
			p.PreviewDiff.SetSize(0, 0),
		)
	}

	p.PreviewColumns = l.ColumnWidth
	return tea.Batch(
		p.PreviewOriginal.SetSize(l.ColumnWidth, l.PreviewRows),
		p.PreviewStego.SetSize(l.ColumnWidth, l.PreviewRows),
		p.PreviewDiff.SetSize(l.ColumnWidth, l.PreviewRows),
	)
}

// loadSource decodes the picked file off the UI goroutine: decoding a large
// JPEG plus the capacity scan takes long enough to freeze the interface if it
// ran inside Update.
func (p *StegoPane) loadSource(path string) tea.Cmd {
	p.Busy = true
	p.setStatus("Opening " + filepath.Base(path) + "…")
	return func() tea.Msg {
		img, err := stego.DecodeFile(path)
		return loadMsg{path: path, img: img, err: err}
	}
}

func (p *StegoPane) run() tea.Cmd {
	if p.Busy {
		return nil
	}
	if p.SelectedFile == "" {
		p.setError("Select a source image first: Enter on SOURCE")
		return p.clearStatusCmd()
	}

	if p.Mode == ModeExtract {
		return p.runExtract()
	}
	return p.runEmbed()
}

// runEmbed does the whole embedding pipeline in a goroutine: decoding, the DCT
// pass, writing the container and building the difference preview. Everything
// that touches pane state is left to the embedMsg handler.
func (p *StegoPane) runEmbed() tea.Cmd {
	path := p.SelectedFile
	message := []byte(p.Message.Value())
	p.Busy = true
	p.setStatus(fmt.Sprintf("Embedding %d B…", len(message)))

	return func() tea.Msg {
		img, err := stego.DecodeFile(path)
		if err != nil {
			return embedMsg{err: errors.New("cannot read source: " + err.Error())}
		}

		container, stats, err := stego.Embed(img, message)
		if err != nil {
			return embedMsg{err: err}
		}

		out := stego.OutputPath(path)
		if err := stego.WritePNG(out, container); err != nil {
			return embedMsg{err: errors.New("cannot save container: " + err.Error())}
		}

		return embedMsg{
			img:       img,
			container: container,
			diff:      differenceImage(img, container, diffGain),
			stats:     stats,
			path:      out,
			size:      len(message),
		}
	}
}

func (p *StegoPane) runExtract() tea.Cmd {
	path := p.SelectedFile
	p.Busy = true
	p.setStatus("Extracting…")

	return func() tea.Msg {
		img, err := stego.DecodeFile(path)
		if err != nil {
			return extractMsg{err: errors.New("cannot read source: " + err.Error())}
		}

		payload, err := stego.Extract(img)
		if err != nil {
			return extractMsg{err: err}
		}
		return extractMsg{img: img, payload: payload}
	}
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
	if p.Mode == ModeExtract {
		return p.previewColumn("CONTAINER", p.OriginalImage, &p.PreviewOriginal, l.PanelWidth, l.PreviewRows)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		p.previewColumn("ORIGINAL", p.OriginalImage, &p.PreviewOriginal, l.ColumnWidth, l.PreviewRows),
		"  ",
		p.previewColumn("STEGO", p.StegoImage, &p.PreviewStego, l.ColumnWidth, l.PreviewRows),
		"  ",
		p.previewColumn("DIFF ×20", p.DiffImage, &p.PreviewDiff, l.ColumnWidth, l.PreviewRows),
	)
}

func (p StegoPane) previewColumn(label string, img image.Image, model *picture.Model, width, rows int) string {
	header := labelStyle.Render(label)
	if img != nil {
		bounds := img.Bounds()
		header += mutedStyle.Render(fmt.Sprintf(" %d×%d", bounds.Dx(), bounds.Dy()))
	}
	header = ansi.Truncate(header, width, "…")
	header = lipgloss.NewStyle().Width(width).Render(header)

	content := model.View().Content
	if img == nil || content == "" {
		content = mutedStyle.Render("—")
	}

	return header + "\n" + padBlock(content, rows)
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
