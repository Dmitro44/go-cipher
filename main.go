package main

import (
	"fmt"
	"os"

	"go-cipher/internal/cipher"
	"go-cipher/internal/ui"
	"go-cipher/internal/ui/asympane"
	"go-cipher/internal/ui/cipherpane"
	"go-cipher/internal/ui/hashpane"
	"go-cipher/internal/ui/signpane"

	tea "charm.land/bubbletea/v2"
)

func main() {
	gost := cipher.NewGost()
	belt := cipher.NewBelt()
	rabin := cipher.NewRabin()
	signature := cipher.NewGost3410()

	m := ui.NewModel(
		ui.Tab{Title: gost.Name(), Pane: cipherpane.NewCipherPane(gost)},
		ui.Tab{Title: belt.Name(), Pane: cipherpane.NewCipherPane(belt)},
		ui.Tab{Title: rabin.Name(), Pane: asympane.NewAsymPane(rabin)},
		ui.Tab{Title: "Hash", Pane: hashpane.NewHashPane()},
		ui.Tab{Title: "Signature", Pane: signpane.NewSignPane(signature)},
	)

	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Printf("error: %s", err)
		os.Exit(1)
	}
}
