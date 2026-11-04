package main

import (
	"fmt"
	"os"

	"go-cipher/internal/cipher"
	"go-cipher/internal/ui"
	"go-cipher/internal/ui/asympane"
	"go-cipher/internal/ui/cipherpane"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	gost := cipher.NewGost()
	belt := cipher.NewBelt()
	rabin := cipher.NewRabin()

	m := ui.NewModel(
		ui.Tab{Title: gost.Name(), Pane: cipherpane.NewCipherPane(gost)},
		ui.Tab{Title: belt.Name(), Pane: cipherpane.NewCipherPane(belt)},
		ui.Tab{Title: rabin.Name(), Pane: asympane.NewAsymPane(rabin)},
	)

	if _, err := tea.NewProgram(
		m,
		tea.WithAltScreen(),
	).Run(); err != nil {
		fmt.Printf("error: %s", err)
		os.Exit(1)
	}
}
