package player

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

func TestUnidentifiedHistoricalCachePlaysWithoutSource(t *testing.T) {
	root := t.TempDir()
	b, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(filepath.Join(root, "songs"), 64<<20)
	st := &music.Stream{Type: "mp3", Quality: "high"}
	f := cache.part(st)
	if f == nil {
		t.Fatal("could not create fixture")
	}
	f.Write(b)
	f.Close()
	cache.keep("removed", "old", "high", st, f.Name(), int64(len(b)))
	s, err := offline.Open(root, 64<<20, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	records := s.List()
	if len(records) != 1 || !records[0].Legacy {
		t.Fatal("missing historical cache")
	}
	tr := s.Track(records[0])
	p := &Player{Library: s}
	_, stream, err := p.open(nil, tr, "lossless", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	samples := make([]float32, 1024)
	if n, err := stream.Read(samples); n == 0 || err != nil {
		t.Fatal("historical audio did not decode", err)
	}
}

func savedPlayerTrack(t *testing.T) (*offline.Store, music.Track) {
	t.Helper()
	root := t.TempDir()
	s, err := offline.Open(root, 64<<20, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	path, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := fileSource{path: path + "/testdata/seek.mp3"}
	s.SetSource(func(string) music.Source { return src })
	tr := music.Track{ID: "one", Source: "gone", Name: "Independent", Duration: 10000}
	h, err := s.Acquire(tr, "high")
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		snap := h.Snapshot()
		if snap.Done {
			if snap.Err != nil {
				t.Fatal(snap.Err)
			}
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("save did not finish")
		case <-snap.Changed:
		}
	}
	s.RemoveSource("gone")
	r, ok := s.Get(tr)
	if !ok {
		t.Fatal("no local record")
	}
	return s, s.Track(r)
}

func TestStoredPlaybackWithoutPluginDesktopAndNative(t *testing.T) {
	for _, native := range []bool{false, true} {
		t.Run(map[bool]string{false: "desktop", true: "native"}[native], func(t *testing.T) {
			s, tr := savedPlayerTrack(t)
			ch := make(chan func(), 128)
			p := New(func(fn func()) { ch <- fn })
			t.Cleanup(p.Close)
			p.Library = s
			p.Source = func(string) music.Source { return nil }
			p.Quality = func() string { return "lossless" }
			if native {
				p.Native = &nativeOutput{}
			}
			p.PlayTrack([]music.Track{tr}, 0)
			if native {
				out := p.Native.(*nativeOutput)
				nativeUntil(t, ch, func() bool { return out.source != "" || p.Err() != "" })
				if p.Err() != "" {
					t.Fatal(p.Err())
				}
				out.events.Loaded(10 * time.Second)
				nativeUntil(t, ch, p.Ready)
			} else {
				settle(t, ch, p)
				if !p.Ready() {
					t.Fatal(p.Err())
				}
			}
			queue := p.Queue()
			p.DetachSource("gone")
			if len(p.Queue()) != len(queue) || !p.Playing() {
				t.Fatal("source removal cleared local playback")
			}
			session, ok := p.Session()
			if !ok || !session.Queue[0].LocalOnly || session.Queue[0].StoredID == "" {
				t.Fatal("session lost local playback identity")
			}
		})
	}
}
