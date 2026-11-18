package player

import (
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

func TestSleepTimerPostsExpiry(t *testing.T) {
	ch := make(chan func(), 4)
	p := New(func(fn func()) { ch <- fn })
	defer p.Close()
	p.Cue([]music.Track{{ID: "one"}})
	p.Hold(23 * time.Second)
	expired := 0
	p.SleepTimerExpired = func() { expired++ }
	p.SleepAfter(time.Millisecond)
	var fn func()
	select {
	case fn = <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timer did not post its expiry")
	}
	if !p.Playing() || expired != 0 {
		t.Fatal("timer changed playback off the main thread")
	}
	fn()
	if p.Playing() || expired != 1 || p.Position() != 23*time.Second || len(p.Queue()) != 1 || p.SleepRemaining() != 0 {
		t.Fatal("expiry did not pause once while preserving queue and position")
	}
	fn()
	if expired != 1 {
		t.Fatal("expiry ran twice")
	}
}

func TestSleepTimerReplacementCancellationAndClose(t *testing.T) {
	p := New(func(fn func()) { fn() })
	defer p.Close()
	now := time.Unix(1000, 0)
	p.now = func() time.Time { return now }
	p.Hold(10 * time.Second)
	expired, changed := 0, 0
	p.SleepTimerExpired = func() { expired++ }
	p.SleepTimerChanged = func() { changed++ }
	p.SleepAfter(30 * time.Minute)
	old := p.sleepGen
	now = now.Add(10 * time.Minute)
	if p.SleepRemaining() != 20*time.Minute {
		t.Fatal("countdown did not use elapsed time")
	}
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(old)
	if !p.Playing() || p.SleepRemaining() != time.Hour || expired != 0 {
		t.Fatal("replaced timer paused playback")
	}
	old = p.sleepGen
	p.SleepAfter(0)
	p.expireSleepTimer(old)
	if !p.Playing() || p.SleepRemaining() != 0 || expired != 0 || changed != 3 {
		t.Fatal("cancellation paused playback or failed to notify")
	}
	p.SleepAfter(time.Hour)
	old = p.sleepGen
	p.Close()
	p.expireSleepTimer(old)
	p.SleepAfter(time.Hour)
	if p.sleepTimer != nil || !p.sleepUntil.IsZero() || expired != 0 {
		t.Fatal("closed player retained a timer or accepted a new one")
	}
}

func TestSleepTimerDesktopFade(t *testing.T) {
	ch, p := posted()
	defer p.Close()
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(p.sleepGen)
	if p.Playing() || !p.out.Playing() {
		t.Fatal("sleep did not use pause fade")
	}
	next(t, ch)
	if p.out.Playing() || p.Current() == nil {
		t.Fatal("sleep did not pause the output and keep the track")
	}
}

func TestSleepTimerNativeAndPaused(t *testing.T) {
	p, out, ch := newNativeTest()
	defer p.Close()
	p.PlayQueue([]music.Track{{ID: "one", Duration: 60000}, {ID: "two"}}, 0)
	nativeUntil(t, ch, func() bool { return out.source != "" })
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, p.Ready)
	out.events.Progress(23*time.Second, time.Minute)
	nativeUntil(t, ch, func() bool { return p.Position() == 23*time.Second })
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(p.sleepGen)
	if !out.paused || p.Playing() || p.Position() != 23*time.Second {
		t.Fatal("native output was not paused in place")
	}
	// An end event already on its way must not start the next song.
	out.events.Ended()
	nativeUntil(t, ch, func() bool { return p.native.ended })
	if p.Current().ID != "one" || p.Playing() {
		t.Fatal("late native end event restarted playback")
	}
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(p.sleepGen)
	if !out.paused || p.Playing() {
		t.Fatal("expiry resumed an already paused player")
	}
}

func TestSleepTimerDuringLoadAndRadioFeed(t *testing.T) {
	p, out, ch := newNativeTest()
	defer p.Close()
	p.PlayQueue([]music.Track{{ID: "one"}}, 0)
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(p.sleepGen)
	nativeUntil(t, ch, func() bool { return out.source != "" })
	out.events.Loaded(time.Minute)
	nativeUntil(t, ch, p.Ready)
	if p.Playing() || !out.paused {
		t.Fatal("load completed playing after sleep")
	}
	var feed func([]music.Track)
	p.AttachRadio("station", func(got func([]music.Track)) { feed = got })
	p.awaiting, p.playing = true, true
	p.SleepAfter(time.Hour)
	p.expireSleepTimer(p.sleepGen)
	feed([]music.Track{{ID: "two"}})
	if p.Playing() || p.Current().ID != "one" || p.awaiting || len(p.Queue()) != 2 {
		t.Fatal("late radio feed resumed playback after sleep")
	}
}

func TestSleepTimerRemote(t *testing.T) {
	r := newRemoteRig(t, 2)
	defer r.p.Close()
	r.p.SleepAfter(time.Hour)
	r.p.expireSleepTimer(r.p.sleepGen)
	r.await("pause")
	if r.p.Playing() || r.p.remote.running || len(r.p.Queue()) != 2 {
		t.Fatal("sleep did not pause the remote source")
	}
}
