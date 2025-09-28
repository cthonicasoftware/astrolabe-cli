package main

import (
	"log"

	tea "github.com/charmbracelet/bubbletea"

	"qa_cli/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.NewModel())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
