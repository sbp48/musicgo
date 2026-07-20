package main

import (
	"fmt"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/speaker"

	tea "github.com/charmbracelet/bubbletea"
)

type appState int

const (
	stateBrowsing = iota
	statePlaying
)

type backToBrowserMsg struct {}

type appModel struct {
	state appState
	browser browserModel
	player *playerModel
	prefs Preferences

	speakerReady bool

	//TODO:
	// this is kind of stupid because the sample rate for the entire program is set
	// by the first file that was opened, and everything after needs to be resampled
	// works for now :)
	masterRate beep.SampleRate

	art *artState
}

func (m appModel) Init() tea.Cmd {
	return m.browser.Init()
}

func (m appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
		case folderChosenMsg:
			cmd, err := m.openFolder(msg.path)
			if err != nil {
				m.browser.errMsg = err.Error()
				m.browser.selected = false
				return m, nil
			}
			m.browser.errMsg = ""
			return m, cmd
		case backToBrowserMsg:
			m.teardownPlayer()
			m.browser.errMsg = ""
			m.browser.selected = false
			return m, tea.Batch(tea.ClearScreen, clearAlbumArtCmd(m.art))
		
		case tea.KeyMsg:
			switch msg.String() {
				// quits music player from both screens
				case "ctrl+c":
					return m, tea.Quit
				case "esc":
					// quits from browsing
					if m.state == stateBrowsing {
						return m, tea.Quit
					}
			}

	}

	switch m.state {
		case stateBrowsing:
			var cmd tea.Cmd
			m.browser, cmd = m.browser.Update(msg)
			return m, cmd
		case statePlaying:
			if m.player == nil {
				m.state = stateBrowsing
				return m, nil
			}
			var cmd tea.Cmd
			m.player, cmd = m.player.Update(msg)
			return m, cmd
	}

	return m, nil
}

func (m appModel) View() string {
	switch m.state {
		case stateBrowsing:
			return m.browser.View()
		case statePlaying:
			return m.player.View()
	}
	return ""
}

func (m *appModel) openFolder(path string) (tea.Cmd, error) {
	playlist, err := loadFolder(path)
	if err != nil {
		return nil, err
	}

	if len(playlist) == 0 {
		return nil, fmt.Errorf("no flac files in dir %s", path)
	}

	if !m.speakerReady {
		rate, err := peekSampleRate(playlist[0].path)
		if err != nil {
			return nil, err
		}
		speaker.Init(rate, rate.N(time.Second/10))
		m.masterRate = rate
		m.speakerReady = true
	}
	
	if m.art == nil {
		m.art = newArtState()
	}

	player := &playerModel {
		ctrl: &beep.Ctrl{},
		playlist: playlist,
		sampleRate: m.masterRate,
		volumePercent: m.prefs.InitialVolume,
		resampleQuality: m.prefs.ResamplingQuality,
		volumeStep: m.prefs.VolumeStep,
		displayCurrentTrack: m.prefs.DisplayCurrentTrack,
		displayNextTrack: m.prefs.DisplayNextTrack,
		displayKeybinds: m.prefs.DisplayKeybinds,
		art: m.art,
	}

	artBytes, err := player.switchTrack(0)
	if err != nil {
		return nil, err
	}

	m.teardownPlayer()

	m.player = player
	m.state = statePlaying

	return tea.Batch(tea.ClearScreen, tick(), player.drawArtCmd(artBytes)), nil
}

func (m *appModel) teardownPlayer() {
	if m.player == nil {
		return
	}

	speaker.Clear()
	if m.player.streamer != nil {
		_ = m.player.streamer.Close()
	}
	m.player = nil
}
