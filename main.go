package main

import (
	"log"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	prefs := loadPreferences()

	initial := appModel {
		state: stateBrowsing,
		browser: newBrowserModel(prefs),
		prefs: prefs,
	}

	p := tea.NewProgram(initial, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatalf("error running program: %v", err)
	}
}
