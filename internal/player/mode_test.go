package player

import (
	"slices"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

func TestPlaybackModeCycleKeepsCurrentTrack(t *testing.T) {
	p := &Player{}
	p.Cue([]music.Track{{ID: "one"}, {ID: "two"}, {ID: "three"}, {ID: "four"}})
	p.at, p.track = 2, &p.queue[2]
	p.Hold(23 * time.Second)
	p.SetPlaybackMode(ModeListLoop)
	for _, want := range []PlaybackMode{ModeSingleLoop, ModeShuffle, ModeListLoop, ModeSingleLoop, ModeShuffle, ModeListLoop} {
		p.CyclePlaybackMode()
		if p.PlaybackMode() != want || p.Shuffle() != (want == ModeShuffle) {
			t.Fatalf("mode %v: shuffle=%v, repeat=%v", want, p.Shuffle(), p.Repeat())
		}
		repeat := RepeatAll
		if want == ModeSingleLoop {
			repeat = RepeatOne
		}
		if p.Repeat() != repeat {
			t.Fatalf("mode %v has repeat %v", want, p.Repeat())
		}
		if p.Current().ID != "three" || p.order[p.at] != 2 || p.Position() != 23*time.Second || !p.Playing() {
			t.Fatal("switching modes changed the current track, position or playback state")
		}
		order := slices.Clone(p.order)
		slices.Sort(order)
		if !slices.Equal(order, []int{0, 1, 2, 3}) {
			t.Fatalf("queue lost or repeated tracks: %v", p.order)
		}
		if want != ModeShuffle && !slices.Equal(p.order, order) {
			t.Fatalf("non-shuffle mode did not restore list order: %v", p.order)
		}
	}
}

func TestPlaybackModesLoopAtQueueEnd(t *testing.T) {
	for _, mode := range []PlaybackMode{ModeListLoop, ModeSingleLoop, ModeShuffle} {
		p, out, ch := newNativeTest()
		p.SetPlaybackMode(mode)
		p.PlayQueue([]music.Track{{ID: "one"}, {ID: "two"}, {ID: "three"}}, 0)
		nativeUntil(t, ch, func() bool { return out.source != "" })
		p.load(len(p.order) - 1)
		nativeUntil(t, ch, func() bool { return out.source != "" })
		out.events.Loaded(time.Minute)
		nativeUntil(t, ch, p.Ready)
		want := p.queue[p.order[0]].ID
		if mode == ModeSingleLoop {
			want = p.Current().ID
		}
		previous := p.native
		out.events.Ended()
		nativeUntil(t, ch, func() bool { return p.native != previous && out.source != "" && p.Current().ID == want })
		if !p.Playing() {
			t.Fatalf("mode %v stopped at the queue's end", mode)
		}
		p.Clear()
	}
}

func TestPlayShuffledLeavesSingleLoop(t *testing.T) {
	p, out, ch := newNativeTest()
	p.SetPlaybackMode(ModeSingleLoop)
	p.PlayShuffled([]music.Track{{ID: "one"}, {ID: "two"}})
	nativeUntil(t, ch, func() bool { return out.source != "" })
	defer p.Clear()
	if p.PlaybackMode() != ModeShuffle || p.Repeat() != RepeatAll {
		t.Fatalf("shuffle retained single-track repeat: shuffle=%v repeat=%v", p.Shuffle(), p.Repeat())
	}
}
