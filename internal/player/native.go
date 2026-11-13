package player

import (
	"errors"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
)

// NativeOutput is a platform media player. Calls and event callbacks run on
// the app's engine goroutine. Each Open owns its events until Close.
type NativeOutput interface {
	Open(source string, paused bool, events NativeEvents)
	SetPaused(bool)
	SetVolume(float64)
	Seek(time.Duration)
	Close()
}

type NativeEvents struct {
	Loaded   func(time.Duration)
	Progress func(position, duration time.Duration)
	Buffer   func(bool)
	Ended    func()
	Error    func(error)
}

type nativePlayback struct {
	ready              bool
	position, duration time.Duration
	ended              bool
}

func (p *Player) loadNative(gen int, src music.Source, t music.Track, quality string, at, skipped, dir int) {
	n := &nativePlayback{position: p.pending}
	p.native = n
	go func() {
		var info *music.Stream
		var err error
		if src == nil {
			err = errNoSource
		} else {
			info, err = src.Stream(t.ID, quality)
			if err == nil && (info == nil || info.URL == "") {
				err = music.ErrUnavailable
			}
			// core.Video has no request-header API. Never silently send an
			// authenticated stream to it without its required headers.
			if err == nil && len(info.Headers) > 0 {
				err = errors.New("native playback: stream request headers are not supported")
			}
		}
		p.post(func() {
			if gen != p.gen || p.native != n {
				return
			}
			if err != nil {
				p.native = nil
				p.loading = false
				p.loadFailed(t.Source, err, at, skipped, dir)
				return
			}
			p.info, p.gain = info, normGain(info)
			// Posting events also prevents queue changes during NSGo's
			// dispatch of a Video event. Old tracks cannot change new ones.
			current := func(fn func()) {
				p.post(func() {
					if gen == p.gen && p.native == n {
						fn()
					}
				})
			}
			p.Native.Open(info.URL, !p.playing, NativeEvents{
				Loaded: func(d time.Duration) {
					current(func() {
						n.ready, n.duration = true, max(0, d)
						p.loading = false
						p.nativeSeek(p.pending)
						p.pending = 0
						p.record()
						p.sync()
					})
				},
				Progress: func(pos, d time.Duration) {
					current(func() {
						n.position = max(0, pos)
						if d > 0 {
							n.duration = d
						}
					})
				},
				Buffer: func(waiting bool) {
					current(func() {
						p.loading = !n.ready || waiting
						p.sync()
					})
				},
				Ended: func() {
					current(func() {
						if n.ended {
							return
						}
						p.Native.SetPaused(true)
						n.ended = true
						if n.duration > 0 {
							n.position = n.duration
						}
						p.loading = false
						p.ended()
					})
				},
				Error: func(err error) {
					current(func() {
						p.Native.Close()
						p.native = nil
						p.loading = false
						p.loadFailed(t.Source, err, at, skipped, dir)
					})
				},
			})
			p.applyGain()
			p.sync()
		})
	}()
}

func (p *Player) nativeToggle() {
	n := p.native
	p.playing = !p.playing
	if p.playing {
		if n.ended {
			p.nativeSeek(0)
		}
		if n.ready {
			p.record()
		}
	}
	p.Native.SetPaused(!p.playing)
	p.sync()
}

func (p *Player) nativeSeek(t time.Duration) {
	n := p.native
	t = max(0, t)
	if d := p.Duration(); d > 0 {
		t = min(t, d)
	}
	n.position, n.ended = t, false
	if n.ready {
		p.Native.Seek(t)
	} else {
		p.pending = t
	}
	p.sync()
}
