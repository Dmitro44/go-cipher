package asympane

import "github.com/charmbracelet/lipgloss"

var (
	focusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	blurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("237"))

	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("99")).
			Bold(true)

	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Italic(true)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("212"))

	buttonStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("99")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)

	saveBtnStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("86")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)

	keyBtnStyle = lipgloss.NewStyle().
			Background(lipgloss.Color("212")).
			Foreground(lipgloss.Color("0")).
			Padding(0, 1).
			Bold(true)

	keyBorderColor   = lipgloss.Color("99")
	panelBorderColor = lipgloss.Color("240")

	statusStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true)

	popupStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Padding(1, 2).
			Width(40)
)
