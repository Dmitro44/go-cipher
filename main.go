package main

import (
	"fmt"
	"os"

	"go-cipher/internal/cipher"
	"go-cipher/internal/ui"
	"go-cipher/internal/ui/cipherpane"

	tea "github.com/charmbracelet/bubbletea"
	zone "github.com/lrstanley/bubblezone"
)

func main() {
	zone.NewGlobal()
	defer zone.Close()

	gost := cipher.NewGost()
	belt := cipher.NewBelt()

	m := ui.NewModel(
		ui.Tab{Title: gost.Name(), Pane: cipherpane.NewCipherPane(gost)},
		ui.Tab{Title: belt.Name(), Pane: cipherpane.NewCipherPane(belt)},
	)

	if _, err := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	).Run(); err != nil {
		fmt.Printf("error: %s", err)
		os.Exit(1)
	}
}
