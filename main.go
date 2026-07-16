package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/faiface/beep"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/speaker"

	"github.com/dhowden/tag"
)

type Track struct {
	path string
	
	//metadata
	title string
	artist string
	genre string
	album string
}

type model struct {
	ctrl *beep.Ctrl
	isPaused bool

	playlist []Track
	currentIdx int

	streamer beep.StreamSeekCloser

	sampleRate beep.SampleRate
}

type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func loadFolder(folderPath string) []Track {
	var playlist []Track
	files, err := os.ReadDir(folderPath)
	if err != nil {
		log.Fatal("error reading path", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if strings.HasSuffix(file.Name(), ".flac") {
			fullPath := folderPath + "/" + file.Name()

			f, err := os.Open(fullPath)
			if err != nil {
				log.Println("could not open file: ", fullPath, err)
			}

			songTitle := file.Name()
			songArtist := "Unknown Artist"
			songAlbum := "Unknown Album"
			songGenre := "Unknown Genre"

			metadata, err := tag.ReadFrom(f)
			if err == nil {
				if metadata.Title() != "" {
					songTitle = metadata.Title()
				}
				if metadata.Genre() != "" {
					songGenre = metadata.Genre()
				}
				if metadata.Artist() != "" {
					songArtist = metadata.Artist()
				}
				if metadata.Album() != "" {
					songAlbum = metadata.Album()
				}
			}

			f.Close()

			newTrack := Track {
				path: fullPath,
				title: songTitle,
				album: songAlbum,
				genre: songGenre,
				artist: songArtist,
			}

			playlist = append(playlist, newTrack)
		}
	}
	return playlist
}

func (m *model) switchTrack(newIdx int) error {
	if m.streamer != nil {
		err := m.streamer.Close()
		if err != nil {
			log.Println("failed to close old streamer", err)	
		}
	}

	newTrack := m.playlist[newIdx]
	f, err := os.Open(newTrack.path)
	if err != nil {
		return err
	}

	streamer, format, err := flac.Decode(f)
	if err != nil {
		f.Close()
		return err
	}
	
	resampled := beep.Resample(4, format.SampleRate, m.sampleRate, streamer)

	speaker.Lock()
	m.ctrl.Streamer = resampled
	m.streamer = streamer
	m.currentIdx = newIdx
	
	m.isPaused = false
	m.ctrl.Paused = false
	speaker.Unlock()
	speaker.Clear()
	speaker.Play(m.ctrl)

	return nil
}

func main(){
	playlist := loadFolder("./music/")
	if len(playlist) == 0 {
		log.Fatal("no songs in folder")
	}
	firsttrack := playlist[0]

	f, err := os.Open(firsttrack.path)
	if err != nil {
		log.Fatal(err)
	}

	streamer, format, err := flac.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	resampled := beep.Resample(4, format.SampleRate, format.SampleRate, streamer)	
	ctrl := &beep.Ctrl {
		Streamer: resampled,
		Paused: false,
	}

	speaker.Play(ctrl)

	initModel := model{
		ctrl: ctrl,
		isPaused: false,

		playlist: playlist,
		currentIdx: 0,

		streamer: streamer,

		sampleRate: format.SampleRate,
	}

	p := tea.NewProgram(initModel)
	if _, err := p.Run(); err != nil {
		log.Fatalf("error running program: %v", err)
	}
}

func (m model) Init() tea.Cmd {
	return tick()
}

func (m model) View() string {
	currentTrack := m.playlist[m.currentIdx]

	var status string = "playing..."
	if m.isPaused {
		status = "paused..."
	}

	var output string = fmt.Sprintf(
		"\nBUBBLES MUSIC PLAYER GO\n\n%s\n%s\n%s\n%s\nStatus: %s\n\n[Space] pause/play\n[q] quit", 
		currentTrack.title, currentTrack.album, currentTrack.artist, currentTrack.genre, 
		status)
	return output
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
		case tea.KeyMsg:
			switch msg.String() {
				case "q", "ctrl+c":
					return m, tea.Quit
				case " ":
					m.isPaused = !m.isPaused
					speaker.Lock()
					m.ctrl.Paused = m.isPaused
					speaker.Unlock()
				case "n":
					nextIdx := (m.currentIdx + 1) % len(m.playlist)
					_ = m.switchTrack(nextIdx)
				case "p":
					prevIdx := (m.currentIdx - 1 + len(m.playlist)) % len(m.playlist)
					_ = m.switchTrack(prevIdx)
			}
		case tickMsg:
			if m.streamer != nil {
				if m.streamer.Position() >= m.streamer.Len() {
					nextIdx := (m.currentIdx + 1) % len(m.playlist)
					_ = m.switchTrack(nextIdx)
				}
			}
			return m, tick()
	}

	return m, nil
}
