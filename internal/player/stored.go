package player

import (
	"context"
	"sync"

	"github.com/HengWh/woodwind-music/internal/i18n"
	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

func playbackContext(b *playback) context.Context {
	if b != nil && b.ctx != nil {
		return b.ctx
	}
	return context.Background()
}

func managedDownload(h *offline.Handle, st *music.Stream) *download {
	stop := make(chan struct{})
	var once sync.Once
	d := &download{size: st.Size, reader: h, users: 1, finished: make(chan struct{})}
	d.cond.L = &d.mu
	d.cancel = func() { once.Do(func() { close(stop); _ = h.Close() }) }
	initial := h.Snapshot()
	d.n, d.done, d.err = initial.Received, initial.Done, initial.Err
	d.onDisk = initial.Done && initial.Err == nil
	go func() {
		defer close(d.finished)
		for {
			s := h.Snapshot()
			d.mu.Lock()
			if !d.closed {
				d.n, d.done, d.err = s.Received, s.Done, s.Err
				d.cond.Broadcast()
			}
			d.mu.Unlock()
			if s.Done {
				return
			}
			select {
			case <-stop:
				return
			case <-s.Changed:
			}
		}
	}()
	return d
}
func (p *Player) storedInput(t music.Track, q string, b *playback) (*music.Stream, *download, error) {
	h, err := p.Library.Acquire(t, q)
	if err != nil {
		return nil, nil, err
	}
	st, err := h.Info(playbackContext(b))
	if err != nil {
		h.Close()
		return nil, nil, err
	}
	d, err := b.acquire(func() (*download, error) { return managedDownload(h, st), nil })
	if err != nil {
		h.Close()
		return nil, nil, err
	}
	return st, d, nil
}
func (p *Player) openStored(t music.Track, q string, pre *ahead, b *playback) (*music.Stream, stream, error) {
	var st *music.Stream
	var d *download
	var err error
	if pre != nil {
		defer pre.close()
		select {
		case <-playbackContext(b).Done():
			return nil, nil, context.Canceled
		case <-pre.done:
		}
		if pre.err == nil {
			st = pre.info
			d, err = b.acquire(pre.resource.takeDownload)
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if d == nil {
		st, d, err = p.storedInput(t, q, b)
		if err != nil {
			return nil, nil, err
		}
	}
	s, err := openStream(d, t.Duration)
	if err != nil {
		d.Close()
		return nil, nil, err
	}
	return st, s, nil
}

// DetachSource stops only online work; complete local playback keeps its lease.
func (p *Player) DetachSource(id string) {
	if p.ahead != nil && p.ahead.source == id {
		p.dropAhead()
	}
	if p.track != nil && p.track.Source == id {
		p.endRadio()
	}
	if p.track != nil && p.track.Source == id && (p.Library == nil || !p.Library.Has(*p.track)) {
		p.stop()
		p.gen++
		p.playing, p.loading = false, false
		p.err = i18n.T("The plugin was removed and this song has no complete local copy")
		p.sync()
	}
}
