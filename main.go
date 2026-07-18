package main

import (
	"log"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	initial := appModel {
		state: stateBrowsing,
		browser: newBrowserModel(),
	}

	p := tea.NewProgram(initial, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal("error running program: %v", err)
	}
}
