package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Preferences struct {
	ResamplingQuality   int      `json:"resamplingquality"`
	InitialVolume       int      `json:"initialvolume"`
	VolumeStep          int      `json:"volumestep"`
	MusicDirectories    []string `json:"musicdirectories"`
	MaxVisibleResults   int      `json:"maxvisibleresults"`
	DisplayCurrentTrack bool     `json:"displaycurrenttrack"`
	DisplayNextTrack    bool     `json:"displaynexttrack"`
	DisplayKeybinds     bool     `json:"displaykeybinds"`
}

func defaultPreferences() Preferences {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}

	return Preferences{
		ResamplingQuality:   10,
		InitialVolume:       100,
		VolumeStep:          5,
		MusicDirectories:    []string{home},
		MaxVisibleResults:   15,
		DisplayCurrentTrack: true,
		DisplayNextTrack:    true,
		DisplayKeybinds:     true,
	}
}

func preferencesPath() (string, error) {
	if wd, err := os.Getwd(); err == nil {
		local := filepath.Join(wd, "preferences.json")
		if _, err := os.Stat(local); err == nil {
			return local, nil
		}
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "gomusic", "preferences.json"), nil
}

func loadPreferences() Preferences {
	defaults := defaultPreferences()

	path, err := preferencesPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, "gomusic: could not locate preferences.json, using defaults:", err)
		return defaults
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if writeErr := writeDefaultPreferences(path, defaults); writeErr != nil {
			fmt.Fprintln(os.Stderr, "gomusic: could not create preferences.json, using defaults:", writeErr)
		}
		return defaults
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gomusic: could not read preferences.json, using defaults:", err)
		return defaults
	}

	prefs := defaults
	if err := json.Unmarshal(data, &prefs); err != nil {
		fmt.Fprintln(os.Stderr, "gomusic: preferences.json is invalid, using defaults:", err)
		return defaults
	}

	prefs.sanitize(defaults)
	return prefs
}

func (p *Preferences) sanitize(defaults Preferences) {
	if p.ResamplingQuality < 1 || p.ResamplingQuality > 64 {
		p.ResamplingQuality = defaults.ResamplingQuality
	}
	if p.InitialVolume < 0 || p.InitialVolume > 100 {
		p.InitialVolume = defaults.InitialVolume
	}
	if p.VolumeStep < 1 || p.VolumeStep > 100 {
		p.VolumeStep = defaults.VolumeStep
	}
	if p.MaxVisibleResults < 1 {
		p.MaxVisibleResults = defaults.MaxVisibleResults
	}
	if len(p.MusicDirectories) == 0 {
		p.MusicDirectories = defaults.MusicDirectories
	}
}

func writeDefaultPreferences(path string, prefs Preferences) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	return os.WriteFile(path, data, 0o644)
}
