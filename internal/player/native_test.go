package player

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

type nativeSource struct{ nothing }

func (nativeSource) Stream(id, quality string) (*music.Stream, error) {
	return &music.Stream{URL: "file:///music/" + id + ".mp3", Quality: quality}, nil
}

type nativeOutput struct {
	source string
	paused bool
	volume float64
	seek   time.Duration
	events NativeEvents
}

func (o *nativeOutput) Open(source string, paused bool, events NativeEvents) {
	o.source, o.paused, o.events = source, paused, events
}
func (o *nativeOutput) SetPaused(v bool)     { o.paused = v }
func (o *nativeOutput) SetVolume(v float64)  { o.volume = v }
func (o *nativeOutput) Seek(v time.Duration) { o.seek = v }
func (o *nativeOutput) Close()               { o.source = "" }

func newNativeTest() (*Player, *nativeOutput, chan func()) {
	ch := make(chan func(), 64)
	p := New(func(fn func()) { ch <- fn })
	o := &nativeOutput{}
	p.Native = o
	p.Source = func(string) music.Source { return nativeSource{} }
	return p, o, ch
}

func nativeUntil(t *testing.T, ch chan func(), done func() bool) {
	t.Helper()
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for !done() {
		select {
		case fn := <-ch:
			fn()
		case <-deadline.C:
			t.Fatal("native playback did not settle")
		}
	}
}

func TestNativeRestoreAndControls(t *testing.T) {
	p, out, ch := newNativeTest()
	p.Restore(Session{Queue: []music.Track{{ID: "one", Duration: 60000}}, Order: []int{0}, At: 0, Position: 12000})
	nativeUntil(t, ch, func() bool { return out.source != "" })
	if !out.paused || !p.Loading() || p.Ready() || p.out != nil || p.stream != nil {
		t.Fatal("restore started playback or used the Go audio engine")
	}
	p.Seek(20 * time.Second) // a newer seek replaces the restored one during load
	out.events.Loaded(90 * time.Second)
	nativeUntil(t, ch, p.Ready)
	if out.seek != 20*time.Second || p.Position() != out.seek || p.Duration() != 90*time.Second || p.Loading() {
		t.Fatalf("restore: seek %v, position %v, duration %v", out.seek, p.Position(), p.Duration())
	}
	p.TogglePlay()
	if out.paused || !p.Playing() {
		t.Fatal("did not resume")
	}
	out.events.Progress(23*time.Second, 90*time.Second)
	nativeUntil(t, ch, func() bool { return p.Position() == 23*time.Second })
	p.TogglePlay()
	if !out.paused || p.Playing() {
		t.Fatal("did not pause")
	}
	p.Seek(2 * time.Minute)
	if out.seek != 90*time.Second || p.Position() != out.seek {
		t.Fatal("seek was not clamped")
	}
	p.SetVolume(0)
	if out.volume != 0 {
		t.Fatal("mute was lost")
	}
	p.SetVolume(0.5)
	if out.volume != 0.25 {
		t.Fatalf("volume %v", out.volume)
	}
	session, _ := p.Session()
	if session.Position != 90000 {
		t.Fatalf("session position %d", session.Position)
	}
}

func TestNativeQueueRepeatAndStaleEvents(t *testing.T) {
	p, out, ch := newNativeTest()
	p.PlayQueue([]music.Track{{ID: "one"}, {ID: "two"}}, 0)
	nativeUntil(t, ch, func() bool { return strings.Contains(out.source, "one.mp3") })
	old := out.events
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, p.Ready)
	out.events.Ended()
	nativeUntil(t, ch, func() bool { return strings.Contains(out.source, "two.mp3") })
	old.Error(errors.New("late error from one"))
	old.Progress(50*time.Second, time.Minute)
	out.events.Loaded(2 * time.Minute)
	nativeUntil(t, ch, p.Ready)
	if p.Err() != "" || p.Current().ID != "two" || p.Position() != 0 {
		t.Fatal("old events changed the new song")
	}
	p.repeat = RepeatOne
	out.events.Progress(30*time.Second, 2*time.Minute)
	nativeUntil(t, ch, func() bool { return p.Position() == 30*time.Second })
	before := p.native
	out.events.Ended()
	nativeUntil(t, ch, func() bool { return p.native != before && out.source != "" })
	out.events.Loaded(2 * time.Minute)
	nativeUntil(t, ch, p.Ready)
	if p.Current().ID != "two" {
		t.Fatal("repeat one advanced")
	}
	p.repeat = RepeatOff
	out.events.Ended()
	nativeUntil(t, ch, func() bool { return !p.Playing() })
	if !out.paused {
		t.Fatal("finished native player was left unpaused")
	}
	p.TogglePlay()
	if out.paused || out.seek != 0 || !p.Playing() {
		t.Fatal("finished song did not restart")
	}
	p.repeat = RepeatAll
	out.events.Ended()
	nativeUntil(t, ch, func() bool { return strings.Contains(out.source, "one.mp3") })
}

func TestNativeErrorsSkipOrStay(t *testing.T) {
	p, out, ch := newNativeTest()
	tracks := []music.Track{{ID: "one"}, {ID: "two"}}
	p.PlayQueue(tracks, 0)
	nativeUntil(t, ch, func() bool { return out.source != "" })
	out.events.Error(errors.New("unsupported codec"))
	nativeUntil(t, ch, func() bool { return strings.Contains(out.source, "two.mp3") })
	out.events.Error(errors.New("cannot open two"))
	nativeUntil(t, ch, func() bool { return !p.Loading() })
	if p.Playing() || p.Ready() || p.Err() != "cannot open two" {
		t.Fatal("failed queue did not stop")
	}
	p.PlayTrack(tracks, 0)
	nativeUntil(t, ch, func() bool { return strings.Contains(out.source, "one.mp3") })
	out.events.Error(errors.New("cannot open one"))
	nativeUntil(t, ch, func() bool { return !p.Loading() })
	if p.Current().ID != "one" || p.Playing() {
		t.Fatal("explicit selection skipped its error")
	}
}
