// Package desktoplyrics displays lyrics in an NSGo transparent, non-activating
// panel. The app supplies playback snapshots and polls controls on its clock.
package desktoplyrics

// Word is a piece of a lyric with its sung fraction, from zero to one.
type Word struct {
	Text     string
	Progress float64
}

// Frame is a complete immutable snapshot. Colors use 0xRRGGBB.
type Frame struct {
	Words           []Word
	Secondary       string
	FontSize        int
	Color           uint32
	Locked, Playing bool
	X, Y            int
	Positioned      bool
	// LegacyPixels marks positions saved by the former Windows native window.
	LegacyPixels bool
}

type Action int

const (
	None Action = iota
	Hide
	Lock
	Previous
	TogglePlay
	Next
)

// Event reports an action or a position change in desktop screen points.
type Event struct {
	Action Action
	Moved  bool
	X, Y   int
}

type Window interface {
	Update(Frame)
	Poll() []Event
	Close()
}
