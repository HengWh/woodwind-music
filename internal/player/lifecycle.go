package player

import (
	"context"
	"sync"
	"time"

	"github.com/HengWh/woodwind-music/internal/audio"
)

// playback owns a load from opening its file through the output's final
// fade. It releases resources even if the UI drops the load's posted result.
type playback struct {
	mu      sync.Mutex
	closed  bool
	dl      *download
	stream  stream
	out     *audio.Player
	release func()
}

func (p *Player) newPlayback() *playback {
	b := new(playback)
	p.resourcesMu.Lock()
	if p.resources == nil {
		p.resources = make(map[*playback]struct{})
	}
	p.resources[b] = struct{}{}
	p.resourcesMu.Unlock()
	b.release = func() {
		p.resourcesMu.Lock()
		delete(p.resources, b)
		p.resourcesMu.Unlock()
	}
	return b
}

// acquire prevents a canceled load from opening a file after shutdown.
// Source calls happen outside this lock; only acquiring the input holds it.
func (b *playback) acquire(open func() (*download, error)) (*download, error) {
	if b == nil { // direct stream tests do not have an owning player
		return open()
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, context.Canceled
	}
	dl, err := open()
	b.dl = dl
	return dl, err
}

func (b *playback) decoded(s stream) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		if s != nil {
			s.Close()
		}
		return false
	}
	b.stream = s
	return true
}

// takeDownload transfers a prefetched input to the load that will play it.
func (b *playback) takeDownload() (*download, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil, context.Canceled
	}
	dl := b.dl
	b.dl = nil
	return dl, nil
}

func (b *playback) output(w *watched) *audio.Player {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.out = audio.NewPlayer(w)
	return b.out
}

func (b *playback) stop() {
	b.mu.Lock()
	out := b.out
	if out != nil {
		out.Pause()
	}
	b.mu.Unlock()
	if out == nil {
		go b.close()
	} else {
		// Normal track changes keep their short fade; Close skips the delay.
		time.AfterFunc(100*time.Millisecond, b.close)
	}
}

func (b *playback) close() {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return
	}
	b.closed = true
	// Wake a decoder waiting for network bytes before waiting for it to exit.
	if b.dl != nil {
		b.dl.Close()
		if b.dl.finished != nil {
			<-b.dl.finished
		}
	}
	if b.out != nil {
		b.out.CloseAndWait()
	} else if b.stream != nil {
		b.stream.Close()
	}
	b.mu.Unlock()
	b.release()
}

// Close releases playback resources before returning. Call it on the main
// thread when the app is disposed, after saving the playback session.
func (p *Player) Close() {
	if p.closed {
		return
	}
	p.closed = true
	p.gen++
	p.endRadio()
	p.dropAhead()
	p.fade.attach(nil, 1)
	if p.Native != nil {
		p.Native.Close()
	}
	// Attach is also used by UI fixtures with an output not opened here.
	if p.playback == nil && p.out != nil {
		b := p.newPlayback()
		b.out, b.stream = p.out, p.stream
		if p.stream != nil {
			b.dl, _ = p.stream.source()
		}
	}
	p.resourcesMu.Lock()
	resources := make([]*playback, 0, len(p.resources))
	for b := range p.resources {
		resources = append(resources, b)
	}
	p.resourcesMu.Unlock()
	for _, b := range resources {
		b.close()
	}
	p.playback, p.out, p.stream, p.watch, p.info = nil, nil, nil, nil, nil
	p.native, p.remote = nil, nil
	p.queue, p.order, p.track = nil, nil, nil
	p.at = -1
	p.playing, p.loading, p.seeking = false, false, false
	p.pending = 0
}
