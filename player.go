package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/flac"
	"github.com/faiface/beep/speaker"

	"github.com/dhowden/tag"

	"golang.org/x/sys/unix"
)

// one day the user shall control this
const SAMPLE_QUALITY int = 10

// track struct to keep info about a track
type Track struct {
	path string

	//metadata
	title  string
	artist string
	genre  string
	album  string
}

type playerModel struct {
	ctrl     *beep.Ctrl
	isPaused bool

	playlist   []Track
	currentIdx int

	streamer beep.StreamSeekCloser

	sampleRate beep.SampleRate

	volume        *effects.Volume
	volumePercent int
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

func getWindowSize(fd int) (cols, rows, xpixel, ypixel int, err error) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	return int(ws.Col), int(ws.Row), int(ws.Xpixel), int(ws.Ypixel), nil
}

func drawAlbumArt(imgBytes []byte) {
	if len(imgBytes) == 0 {
		return
	}

	args := []string{
		"+kitten",
		"icat",
		"--transfer-mode=stream",
		"--stdin=yes",
		"--place=24x12@2x2",
	}

	cols, rows, xpx, ypx, err := getWindowSize(int(os.Stdout.Fd()))
	if err == nil && xpx > 0 && ypx > 0 {
		args = append(args, fmt.Sprintf("--use-window-size=%d,%d,%d,%d", cols, rows, xpx, ypx))
	}

	cmd := exec.Command("kitty", args...)

	cmd.Stdin = bytes.NewReader(imgBytes)
	cmd.Stdout = os.Stderr

	fmt.Fprint(os.Stderr, "\x1b7") 
	_ = cmd.Run()
	fmt.Fprint(os.Stderr, "\x1b8") 
}

func clearAlbumArt() {
	cmd := exec.Command("kitty", "+kitten", "icat", "--clear")
	cmd.Stdout = os.Stderr

	fmt.Fprint(os.Stderr, "\x1b7")
	_ = cmd.Run()
	fmt.Fprint(os.Stderr, "\x1b8")
}

func drawAlbumArtCmd(imgBytes []byte) tea.Cmd {
	return func() tea.Msg {
		clearAlbumArt()
		if len(imgBytes) > 0 {
			drawAlbumArt(imgBytes)
		}
		return nil
	}
}

func loadFolder(folderPath string) ([]Track, error) {

	var playlist []Track
	files, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
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
				continue
			}

			// sets defaults for metadata
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

			// closes the file (memory be free!)
			f.Close()

			// creates a new version of the Track struct to keep info about the track
			newTrack := Track{
				path:   fullPath,
				title:  songTitle,
				album:  songAlbum,
				genre:  songGenre,
				artist: songArtist,
			}

			// appends the found track to the playlist
			playlist = append(playlist, newTrack)
		}
	}
	// returns the playlist after the loop has finished and every file in a folder has been checked
	return playlist, nil
}

func peekSampleRate(path string) (beep.SampleRate, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	streamer, format, err := flac.Decode(f)
	if err != nil {
		return 0, err
	}
	streamer.Close()

	return format.SampleRate, nil
}

func (m *playerModel) switchTrack(newIdx int) ([]byte, error) {

	// checks if there is currently a streamer
	if m.streamer != nil {
		err := m.streamer.Close()
		if err != nil {
			log.Println("failed to close old streamer", err)
		}
		m.streamer = nil
	}

	newTrack := m.playlist[newIdx]

	// opens the new track
	f, err := os.Open(newTrack.path)
	if err != nil {
		return nil, err
	}

	var artBytes []byte
	meta, err := tag.ReadFrom(f)
	if err == nil {
		img := meta.Picture()
		if img != nil {
			artBytes = img.Data
		}
	}

	_, err = f.Seek(0, io.SeekStart)
	if err != nil {
		f.Close()
		return nil, err
	}

	// decodes the file and creates a streamer and gets sample rate info
	streamer, format, err := flac.Decode(f)
	if err != nil {
		f.Close()
		return nil, err
	}

	// resamples the audio file to match speaker sample rate
	resampled := beep.Resample(SAMPLE_QUALITY, format.SampleRate, m.sampleRate, streamer)

	baseVolume, baseSilent := 0.0, false
	if m.volume != nil {
		baseVolume, baseSilent = m.volume.Volume, m.volume.Silent
	}
	newVol := &effects.Volume{
		Streamer: resampled,
		Base:     2,
		Volume:   baseVolume,
		Silent:   baseSilent,
	}

	// locks the speaker before changing variables
	speaker.Lock()
	// changes the streamer to the volume-wrapped one
	m.ctrl.Streamer = newVol
	m.volume = newVol
	m.streamer = streamer
	m.currentIdx = newIdx

	m.isPaused = false
	m.ctrl.Paused = false
	speaker.Unlock()
	speaker.Clear()
	speaker.Play(m.ctrl)
	// returns nil if fucntion is good
	return artBytes, nil
}

func (m *playerModel) setVolume(percent int) {
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}

	speaker.Lock()
	m.volumePercent = percent
	if percent == 0 {
		m.volume.Silent = true
	} else {
		m.volume.Silent = false
		m.volume.Volume = math.Log2(float64(percent) / 100.0)
	}
	speaker.Unlock()
}

// the view function draws the terminal
func (m *playerModel) View() string {
	if m == nil || m.streamer == nil || len(m.playlist) == 0 {
		return "\n  loading...\n"
	}

	currentTrack := m.playlist[m.currentIdx]
	currentTime := songTime(m.streamer.Position(), m.sampleRate)
	totalTime := songTime(m.streamer.Len(), m.sampleRate)

	var status string = "playing..."
	if m.isPaused {
		status = "paused..."
	}

	const pad = "\x1b[28C"

	var output strings.Builder
	output.WriteString("\n")
	output.WriteString(pad + "GO MUSIC PLAYER BUBBLES\n\n")
	output.WriteString(pad + "TITLE: " + currentTrack.title + "\n")
	output.WriteString(pad + "ALBUM: " + currentTrack.album + "\n")
	output.WriteString(pad + "ARTIST: " + currentTrack.artist + "\n\n")
	output.WriteString(pad + "TIME:   " + currentTime + " / " + totalTime + "\n")
	output.WriteString(pad + "STATUS: " + status + "\n")
	output.WriteString(pad + fmt.Sprintf("VOLUME: %d%%", m.volumePercent) + "\n\n")
	output.WriteString(pad + "[q]       QUIT\n")
	output.WriteString(pad + "[esc]     BACK TO FOLDERS\n")
	output.WriteString(pad + "[space]   PLAY / PAUSE\n")
	output.WriteString(pad + "[p/n]     PREV / NEXT\n")
	output.WriteString(pad + "[up/down] VOLUME\n")

	output.WriteString("\n")

	return output.String()
}

// the update function actually updates the state
// listens to tea messages to know what to do
// recieves a message, outputs a updated model and a message
func (m *playerModel) Update(msg tea.Msg) (*playerModel, tea.Cmd) {
	// listens for a message and reads its type
	switch msg := msg.(type) {
	// keyborad inputs
	case tea.KeyMsg:
		// reads the string of the input
		switch msg.String() {
		// quits the whole app
		case "q":
			return m, tea.Quit
		// goes back to the folder picker without quitting
		case "esc":
			return m, func() tea.Msg { return backToBrowserMsg{} }
		// updates the playback state
		case " ":
			m.isPaused = !m.isPaused
			speaker.Lock()
			m.ctrl.Paused = m.isPaused
			speaker.Unlock()
			return m, nil
		// skips to next song
		case "n":
			nextIdx := (m.currentIdx + 1) % len(m.playlist)
			artBytes, err := m.switchTrack(nextIdx)
			if err != nil {
				return m, nil
			}
			return m, drawAlbumArtCmd(artBytes)
		// skips to prev song
		case "p":
			prevIdx := (m.currentIdx - 1 + len(m.playlist)) % len(m.playlist)
			artBytes, err := m.switchTrack(prevIdx)
			if err != nil {
				return m, nil
			}
			return m, drawAlbumArtCmd(artBytes)
		// raises volume
		case "up":
			m.setVolume(m.volumePercent + 5)
			return m, nil
		// lowers volume
		case "down":
			m.setVolume(m.volumePercent - 5)
			return m, nil
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
				artBytes, err := m.switchTrack(nextIdx)
				if err == nil {
					return m, tea.Batch(tick(), drawAlbumArtCmd(artBytes))
				}
			}
		}
		// returns the new state and makes the time movr forwards
		return m, tick()
	}
	// returns state and no command
	return m, nil
}
