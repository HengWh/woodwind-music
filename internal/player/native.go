package player

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

// NativeOutput is a platform media player. Calls and event callbacks run on
// the app's engine goroutine. Each Open owns its events until Close.
type NativeOutput interface {
	Open(source string, paused bool, events NativeEvents)
	SetPaused(bool)
	SetVolume(float64)
	SetSpeed(float64)
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
	mu                 sync.Mutex
	ctx                context.Context
	cancel             context.CancelFunc
	handle             *offline.Handle
	unroute            func()
	closed             bool
}

func (n *nativePlayback) release() {
	n.mu.Lock()
	if n.closed {
		n.mu.Unlock()
		return
	}
	n.closed = true
	if n.cancel != nil {
		n.cancel()
	}
	h, unroute := n.handle, n.unroute
	n.handle, n.unroute = nil, nil
	n.mu.Unlock()
	if unroute != nil {
		unroute()
	}
	if h != nil {
		h.Close()
	}
}

func (p *Player) loadNative(gen int, src music.Source, t music.Track, quality string, at, skipped, dir int) {
	n := &nativePlayback{position: p.pending}
	n.ctx, n.cancel = context.WithCancel(context.Background())
	p.native = n
	go func() {
		var info *music.Stream
		var err error
		if p.Library != nil && (t.StoredID != "" || t.Source != "local" && t.Source != "") {
			var h *offline.Handle
			h, err = p.Library.Acquire(t, quality)
			if err == nil {
				n.mu.Lock()
				if n.closed {
					h.Close()
					n.mu.Unlock()
					return
				}
				n.handle = h
				n.mu.Unlock()
				info, err = h.Info(n.ctx)
				if err == nil {
					var url string
					var remove func()
					url, remove, err = h.URL()
					if err == nil {
						n.mu.Lock()
						if n.closed {
							remove()
							n.mu.Unlock()
							return
						}
						n.unroute = remove
						n.mu.Unlock()
						info.URL = url
						info.Headers = nil
					}
				}
			}
		} else if src == nil {
			err = errNoSource
		} else {
			info, err = src.Stream(t.ID, quality)
			if err == nil && (info == nil || info.URL == "") {
				err = music.ErrUnavailable
			}
			// core.Video has no request-header API. Never silently send an
			// authenticated stream to it without its required headers.
			_, headerOutput := p.Native.(interface {
				OpenWithHeaders(string, map[string]string, bool, NativeEvents)
			})
			if err == nil && len(info.Headers) > 0 && !headerOutput {
				err = errors.New("native playback: stream request headers are not supported")
			}
		}
		p.post(func() {
			if gen != p.gen || p.native != n {
				n.release()
				return
			}
			if err != nil {
				n.release()
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
			events := NativeEvents{
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
						n.release()
						p.Native.Close()
						p.native = nil
						p.loading = false
						p.loadFailed(t.Source, err, at, skipped, dir)
					})
				},
			}
			if output, ok := p.Native.(interface {
				OpenWithHeaders(string, map[string]string, bool, NativeEvents)
			}); ok {
				output.OpenWithHeaders(info.URL, info.Headers, !p.playing, events)
			} else {
				p.Native.Open(info.URL, !p.playing, events)
			}
			p.applyGain()
			p.applySpeed()
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
