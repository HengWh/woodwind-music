// Package nowplaying shows what the app plays where the system shows
// media, and reports the media keys and its controls: purego on macOS,
// nothing elsewhere yet.
package nowplaying

import "time"

// Info is what the system shows of what plays.
type Info struct {
	Title, Artist, Album string
	// Artwork is the cover's bytes, a PNG or JPEG.
	Artwork  []byte
	Duration time.Duration
	Position time.Duration
	Playing  bool
}

// Command is a command of the media keys or the system's controls.
type Command int

const (
	Play Command = iota
	Pause
	Next
	Previous
	Seek
)

// OnCommand runs fn with each command while something shows, with where
// Seek goes. Set it before the first Set.
var OnCommand func(Command, time.Duration)
