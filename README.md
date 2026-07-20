# A TERMINAL MUSIC PLAYER WRITTEN IN GO

A TUI music player written in go utilizing beep and bubbletea.

## Features
- can play music
- shows artwork (terminals that support kitty icat)
- fast idk

## Utils
- [BubbleTea](https://github.com/charmbracelet/bubbletea)
- [beep](https://github.com/faiface/beep)
- [tag](https://github.com/dhowden/tag)

## How to run
```bash
go install github.com/sbp48/musicgo@latest
```

## Preferences
edit `preferences.json` directly. It is created
with defaults the first time you run gomusic, at:
- Linux: `~/.config/gomusic/preferences.json`
- (or `$XDG_CONFIG_HOME/gomusic/preferences.json`)

Fields:
| key | meaning | default |
| --- | --- | --- |
| `resamplingquality` | beep resample quality, 1-64 (higher = better) | `10` |
| `initialvolume` | volume percent (0-100) a track starts at | `100` |
| `volumestep` | how much up/down changes the volume per press | `5` |
| `musicdirectories` | folders scanned (recursively) for `.flac` files | `[$HOME]` |
| `maxvisibleresults` | how many folder matches the browser shows at once | `15` |
| `displaycurrenttrack` | show the `TRACK: n/total` indicator on the player screen | `true` |
| `displaynexttrack` | show the `NEXT: <title>` indicator on the player screen | `true` |
| `displaykeybinds` | show the keybind hints at the bottom of the player screen | `true` |

also an `preferences.json` file can be added into the directory of a project for easier changes

## Other
maybe Linux also needs ALSA
