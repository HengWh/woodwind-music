package player

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/audio"
	"github.com/HengWh/woodwind-music/internal/music"
)

type closingSilence struct {
	silence
	closed chan struct{}
}

func (s closingSilence) Close() error { close(s.closed); return nil }

func TestCloseWaitsForDecoder(t *testing.T) {
	closed := make(chan struct{})
	p := New(func(func()) {})
	p.Attach(audio.NewPlayer(closingSilence{closed: closed}))
	p.Close()
	select {
	case <-closed:
	default:
		t.Fatal("Close returned before the decoder released its input")
	}
}

func TestCloseNativeIgnoresLateEvents(t *testing.T) {
	p, out, ch := newNativeTest()
	p.PlayQueue([]music.Track{{ID: "one"}}, 0)
	nativeUntil(t, ch, func() bool { return out.source != "" })
	events := out.events
	p.Close()
	events.Loaded(time.Minute)
	events.Progress(time.Second, time.Minute)
	events.Ended()
	events.Error(errors.New("late error"))
	for len(ch) > 0 {
		(<-ch)()
	}
	if out.source != "" || p.Ready() || p.Playing() || p.Current() != nil {
		t.Fatal("native events revived the closed player")
	}
}

// Closing must release files even while paused, during the previous track's
// fade, or before the UI has accepted a completed load.
func TestCloseReleasesFiles(t *testing.T) {
	for _, state := range []string{"playing", "paused", "fading", "posted"} {
		t.Run(state, func(t *testing.T) {
			data, err := os.ReadFile("testdata/seek.mp3")
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "音乐 #100%.mp3")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			ch := make(chan func(), 64)
			p := New(func(fn func()) { ch <- fn })
			t.Cleanup(p.Close)
			p.SetVolume(0)
			p.Source = func(string) music.Source { return fileSource{path: path} }
			p.PlayQueue([]music.Track{{ID: "one", Source: "disk"}}, 0)
			b := p.playback
			if state == "posted" {
				eventually(t, "audio was not opened", func() bool {
					b.mu.Lock()
					defer b.mu.Unlock()
					return b.stream != nil
				})
			} else {
				settle(t, ch, p)
				if !p.Ready() {
					t.Fatalf("audio was not ready: %s", p.Err())
				}
			}
			b.mu.Lock()
			file := b.dl.file
			b.mu.Unlock()
			var probe [1]byte
			if _, err := file.ReadAt(probe[:], 0); err != nil {
				t.Fatalf("audio file was not readable before closing: %v", err)
			}
			switch state {
			case "paused":
				p.TogglePlay()
			case "fading":
				p.Clear()
			}
			p.Close()
			p.Close() // teardown can be requested more than once
			// Stat reports ERROR_INVALID_HANDLE on Windows after Close;
			// ReadAt uses Go's cross-platform closed-file error instead.
			if _, err := file.ReadAt(probe[:], 0); !errors.Is(err, os.ErrClosed) {
				t.Fatalf("audio file was not closed: %v", err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			for len(ch) > 0 {
				(<-ch)()
			}
			if p.Ready() || p.Loading() || p.Playing() || p.Current() != nil {
				t.Fatal("a queued callback revived the closed player")
			}
		})
	}
}

type closingSource struct {
	nothing
	stream func(string) (*music.Stream, error)
}

func (s closingSource) Stream(id, _ string) (*music.Stream, error) { return s.stream(id) }

// A slow plugin must not hold up teardown or open a file when it replies later.
func TestCloseDuringSourceRequest(t *testing.T) {
	p := New(func(func()) {})
	b := p.newPlayback()
	started, release := make(chan struct{}), make(chan struct{})
	src := closingSource{stream: func(string) (*music.Stream, error) {
		close(started)
		<-release
		return &music.Stream{URL: music.FileURL("testdata/seek.mp3")}, nil
	}}
	result := make(chan error, 1)
	go func() {
		_, _, err := p.openPlayback(src, music.Track{ID: "one"}, "high", nil, b)
		result <- err
	}()
	<-started
	p.Close()
	close(release)
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("late source result was not canceled: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the canceled load did not return")
	}
}

// Cancel inputs whose headers have arrived but whose audio is still missing,
// including the next song's prefetch, and release their cache files.
func TestCloseCancelsDownloads(t *testing.T) {
	for _, prefetch := range []bool{false, true} {
		name := "loading"
		if prefetch {
			name = "prefetch"
		}
		t.Run(name, func(t *testing.T) {
			started := make(chan struct{}, 1)
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.(http.Flusher).Flush()
				started <- struct{}{}
				<-r.Context().Done()
			}))
			defer srv.Close()
			ch := make(chan func(), 64)
			p := New(func(fn func()) { ch <- fn })
			defer p.Close()
			p.SetVolume(0)
			p.Cache = NewCache(t.TempDir(), 1<<20)
			path, err := filepath.Abs("testdata/seek.mp3")
			if err != nil {
				t.Fatal(err)
			}
			p.Source = func(string) music.Source {
				return closingSource{stream: func(id string) (*music.Stream, error) {
					if prefetch && id == "one" {
						return &music.Stream{URL: music.FileURL(path)}, nil
					}
					return &music.Stream{URL: srv.URL, Type: "mp3", Quality: "high"}, nil
				}}
			}
			q := []music.Track{{ID: "one", Source: "disk"}}
			if prefetch {
				q = append(q, music.Track{ID: "two", Source: "disk"})
			}
			p.PlayQueue(q, 0)
			if prefetch {
				settle(t, ch, p)
			}
			select {
			case <-started:
			case <-time.After(2 * time.Second):
				t.Fatal("download did not start")
			}
			p.Close()
			entries, err := os.ReadDir(p.Cache.dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("cache files survived shutdown: %v, %v", entries, err)
			}
		})
	}
}
