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

// one day the user shall control this
const SAMPLE_QUALITY int = 10

// track struct to keep info about a track
type Track struct {
	path string
	
	//metadata
	title string
	artist string
	genre string
	album string
}

// the model of the tea 
type model struct {
	ctrl *beep.Ctrl
	isPaused bool

	playlist []Track
	currentIdx int

	streamer beep.StreamSeekCloser

	sampleRate beep.SampleRate
}

// time 
type tickMsg time.Time

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func songTime(samples int, rate beep.SampleRate) string {
	var totalTime int = samples / int(rate)
	var minutes int = totalTime / 60
	var seconds int = totalTime % 60
	var humanTime string = fmt.Sprintf("%d:%02d", minutes, seconds)

	return humanTime
}

// loads folder with songs from a path with an return of a array
func loadFolder(folderPath string) []Track {
	
	var playlist []Track
	files, err := os.ReadDir(folderPath)
	if err != nil {
		log.Fatal("error reading path", err)
	}
	
	// goes trough every file
	for _, file := range files {
		// if file is a directory it skips it
		if file.IsDir() {
			continue
		}
		
		// if the file is a .flac file it will add it to the array 
		if strings.HasSuffix(file.Name(), ".flac") {
			// the full path of the song (ex: "music/song.flac")
			fullPath := folderPath + "/" + file.Name()
			
			// errors if the song cannot be opened by the full path, wont crash the program and will just skip 
			f, err := os.Open(fullPath)
			if err != nil {
				log.Println("could not open file: ", fullPath, err)
			}
			
			// sets defaults for metadata
			songTitle := file.Name()
			songArtist := "Unknown Artist"
			songAlbum := "Unknown Album"
			songGenre := "Unknown Genre"
			
			// adds metadata to corresponding variables if there is no error in reading metadata
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
			
			// closes the file (memory be free!)
			f.Close()
			
			// creates a new version of the Track struct to keep info about the track
			newTrack := Track {
				path: fullPath,
				title: songTitle,
				album: songAlbum,
				genre: songGenre,
				artist: songArtist,
			}
			
			// appends the found track to the playlist
			playlist = append(playlist, newTrack)
		}
	}
	// returns the playlist after the loop has finished and every file in a folder has been checked
	return playlist
}

// a function for switching tracks takes a track index and outputs an error if an error is thrown
// if there is no error that means that the function has succeeded
func (m *model) switchTrack(newIdx int) error {

	// checks if there is currently a streamer
	if m.streamer != nil {
		// if there is a streamer it will close it
		err := m.streamer.Close()
		if err != nil {
			log.Println("failed to close old streamer", err)	
		}
	}
	
	// the next song to be switched to is set by using the newIdx 
	newTrack := m.playlist[newIdx]

	// opens the new track
	f, err := os.Open(newTrack.path)
	if err != nil {
		return err
	}
	
	// decodes the file and creates a streamer and gets sample rate info
	streamer, format, err := flac.Decode(f)
	if err != nil {
		f.Close()
		return err
	}
	
	// resamples the audio file to match speaker sample rate
	resampled := beep.Resample(SAMPLE_QUALITY, format.SampleRate, m.sampleRate, streamer)
	
	// locks the speaker before changing variables
	speaker.Lock()
	// changes the streamer to resampled one
	m.ctrl.Streamer = resampled
	m.streamer = streamer
	m.currentIdx = newIdx
	
	m.isPaused = false
	m.ctrl.Paused = false
	speaker.Unlock()
	speaker.Clear()
	speaker.Play(m.ctrl)
	// returns nil if fucntion is good
	return nil
}

// main loop
func main(){
	// loads music folder
	playlist := loadFolder("./music/")
	if len(playlist) == 0 {
		// if nothing in the directory then programm quits
		log.Fatal("no songs in folder")
	}
	// selects the first track from the playlist
	firsttrack := playlist[0]
	
	// opens first track
	f, err := os.Open(firsttrack.path)
	if err != nil {
		log.Fatal(err)
	}
	
	// creates streamer and decodes file
	streamer, format, err := flac.Decode(f)
	if err != nil {
		log.Fatal(err)
	}
	
	// initializes the speaker and resamples
	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))
	resampled := beep.Resample(SAMPLE_QUALITY, format.SampleRate, format.SampleRate, streamer)	
	
	// creates a control so playback can be controlled
	ctrl := &beep.Ctrl {
		Streamer: resampled,
		Paused: false,
	}
	
	// plays the song in the background
	speaker.Play(ctrl)
	
	// initializes the bubble tea model
	initModel := model{
		ctrl: ctrl,
		isPaused: false,

		playlist: playlist,
		currentIdx: 0,

		streamer: streamer,

		sampleRate: format.SampleRate,
	}
	
	// starts a bubble tea program
	p := tea.NewProgram(initModel)
	if _, err := p.Run(); err != nil {
		log.Fatalf("error running program: %v", err)
	}
}

// tea init
func (m model) Init() tea.Cmd {
	return tick()
}

// the view function
// basically this one draws the terminal and everything
func (m model) View() string {
	// finds current track so metadata can be displayed
	currentTrack := m.playlist[m.currentIdx]
	currentTime := songTime(m.streamer.Position(), m.sampleRate)
	totalTime := songTime(m.streamer.Len(), m.sampleRate)
	
	// play / pause status
	var status string = "playing..."
	if m.isPaused {
		status = "paused..."
	}
	
	// outputs a string of letters and info
	var output string = fmt.Sprintf(`
	GO MUSIC PLAYER BUBBLES

	TITLE: %s
	ALBUM: %s
	ARTIST: %s
	
	%s/%s

	STATUS: %s

	[q/esc] QUIT
	[space] PLAY / PAUSE
	[p/n] PREV / NEXT
	`,
	currentTrack.title,
	currentTrack.album,
	currentTrack.artist,
	currentTime, totalTime,
	status)
	return output
}

// the update function actually updates the state
// listens to tea messages to know what to do
// recieves a message, outputs a updated model and a message
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// listens for a message and reads its type
	switch msg := msg.(type) {
		// keyborad inputs
		case tea.KeyMsg:
			// reads the string of the input
			switch msg.String() {
				// quitting, sends a message to tea to quit
				case "q", "ctrl+c", "esc":
					return m, tea.Quit
				// updates the playback state
				case " ":
					m.isPaused = !m.isPaused
					speaker.Lock()
					m.ctrl.Paused = m.isPaused
					speaker.Unlock()
				// skips to next song
				case "n":
					nextIdx := (m.currentIdx + 1) % len(m.playlist)
					_ = m.switchTrack(nextIdx)
				// skips to prev song
				case "p":
					prevIdx := (m.currentIdx - 1 + len(m.playlist)) % len(m.playlist)
					_ = m.switchTrack(prevIdx)
			}
		// time update
		case tickMsg:
			// if there is a streamer
			if m.streamer != nil {
				// checks if the current streamer position is greater than the songs lenght
				// if it is then the next song is played
				// this is basically autoplay
				if m.streamer.Position() >= m.streamer.Len() {
					nextIdx := (m.currentIdx + 1) % len(m.playlist)
					_ = m.switchTrack(nextIdx)
				}
			}
			// returns the new state and makes the time movr forwards
			return m, tick()
	}
	// returns state and no command
	return m, nil
}
