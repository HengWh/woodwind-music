// Package desktoplyrics provides an independent, non-activating native lyric
// window. UI state is supplied by the app; native callbacks are polled on its
// clock, so neither platform calls back into the reactive graph.
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

// Event reports an action or a completed drag in desktop screen coordinates.
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
