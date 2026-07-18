package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// messages
type dirScannedMsg struct {
	dirs []string
	err error
}

type folderChosenMsg struct {
	path string
}

// how many dir matches can the user see
const MAX_VISIBLE_RESULTS = 15

type browserModel struct {
	input textinput.Model

	roots []string
	allDirs []string
	filtered []string
	cursor int

	scanning bool
	errMsg string
}

func newBrowserModel() browserModel {
	ti := textinput.New()
	ti.Placeholder = "SEARCH FOR A FOLDER..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 50

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	roots := []string{home, "/mnt/DATA/"}

	return browserModel {
		input: ti,
		roots: roots,
		scanning: true,
	}
}

func (m browserModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, scanForFolders(m.roots))
}

func scanForFolders(roots []string) tea.Cmd {
	return func() tea.Msg {
		var found []string
 
		for _, root := range roots {
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if !d.IsDir() {
					return nil
				}
				if d.Name() != "." && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
 
				entries, err := os.ReadDir(path)
				if err != nil {
					return nil
				}
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".flac") {
						found = append(found, path)
						break
					}
				}
				return nil
			})
		}
 
		sort.Strings(found)
		return dirScannedMsg{dirs: found}
	}
}




func (m *browserModel) applyFilter() {
	query := strings.ToLower(strings.TrimSpace(m.input.Value()))
	filtered := make([]string, 0, len(m.allDirs)) //wha
	for _, d := range m.allDirs {
		if query == "" || strings.Contains(strings.ToLower(d), query) {
			filtered = append(filtered, d)
		}
	}

	m.filtered = filtered

	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}

	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m browserModel) Update(msg tea.Msg) (browserModel, tea.Cmd) {
	switch msg := msg.(type) {
		case dirScannedMsg:
			m.scanning = false
		
			if msg.err != nil {
				m.errMsg = msg.err.Error()
			}

			m.allDirs = msg.dirs
			m.applyFilter()

			return m, nil
		case tea.KeyMsg:
			switch msg.String() {
				case "up":
					if m.cursor > 0 {
						m.cursor--
					}
					return m, nil
				case "down":
					if m.cursor < len(m.filtered) - 1 {
						m.cursor++
					}
					return m, nil
				case "enter":
					if len(m.filtered) == 0 {
						return m, nil
					}
					 chosen := m.filtered[m.cursor]
					 return m, func() tea.Msg {return folderChosenMsg{path: chosen}}
			}
	}

	prevValue := m.input.Value()
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevValue {
		m.applyFilter()
		m.cursor = 0
	}	
	return m, cmd
}

func (m browserModel) View() string {
	var b strings.Builder

	b.WriteString("\n SELECT MUSIC DIRECTORY\n\n")
	b.WriteString("   " + m.input.View() + "\n\n")

	switch {
		case m.scanning:
			b.WriteString("  SCANNING FOR DIRECTORIES\n")
		case m.errMsg != "":
			b.WriteString("  ERROR: " + m.errMsg + "\n")
		case len(m.filtered) == 0:
			b.WriteString("  NO MATCHES!")
		default:
			for i, d := range m.filtered {
				if i >= MAX_VISIBLE_RESULTS {
					b.WriteString(fmt.Sprintf(" ... AND %d MORE\n", len(m.filtered)-MAX_VISIBLE_RESULTS))
					break
				}
				marker := "   "
				if i == m.cursor {
					marker = "> "
				}

				b.WriteString(marker + d + "\n")
			}
	}

	b.WriteString("\n [enter] SELECT [up/down] NAVIGATE [esc] QUIT \n")
	return b.String()
}
