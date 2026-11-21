package player

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/HengWh/woodwind-music/internal/music"
	"github.com/HengWh/woodwind-music/internal/offline"
)

func TestUnidentifiedHistoricalCachePlaysWithoutSource(t *testing.T) {
	root := t.TempDir()
	seedHistoricalCache(t, root, "removed", "old")
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

func seedHistoricalCache(t *testing.T, root, source, id string) {
	t.Helper()
	data, err := os.ReadFile("testdata/seek.mp3")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewCache(filepath.Join(root, "songs"), 64<<20)
	stream := &music.Stream{Type: "mp3", Quality: "high"}
	part := cache.part(stream)
	if part == nil {
		t.Fatal("could not create fixture")
	}
	_, writeErr := part.Write(data)
	closeErr := part.Close()
	if writeErr != nil || closeErr != nil {
		t.Fatalf("could not write fixture: %v, %v", writeErr, closeErr)
	}
	if _, kept := cache.keep(source, id, "high", stream, part.Name(), int64(len(data))); !kept {
		t.Fatal("could not keep fixture")
	}
}

type cachedAssetSource struct{ nothing }

func (cachedAssetSource) Info() music.Info {
	return music.Info{ID: "plugin", Name: "Test plugin", Playback: music.PlaybackStream}
}

func (cachedAssetSource) Can(name string) bool { return name == "lyrics" }

func (cachedAssetSource) Lyrics(string) (*music.Lyrics, error) {
	return &music.Lyrics{LRC: "[00:01.00]Recovered lyrics"}, nil
}

const cachedCoverURL = "https://covers.example.test/cached.png"

type cachedCoverTransport struct {
	http.RoundTripper
	cover []byte
}

func (transport cachedCoverTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.String() != cachedCoverURL {
		return transport.RoundTripper.RoundTrip(req)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(transport.cover)),
	}, nil
}

func TestCachedPlaybackCompletesPendingLyricsAndCover(t *testing.T) {
	var cover bytes.Buffer
	if err := png.Encode(&cover, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	transport := http.DefaultTransport
	http.DefaultTransport = cachedCoverTransport{RoundTripper: transport, cover: cover.Bytes()}
	t.Cleanup(func() { http.DefaultTransport = transport })
	root := t.TempDir()
	src := cachedAssetSource{}
	track := music.Track{ID: "one", Source: src.Info().ID, Name: "Cached song"}
	track.Album.ImageURL = cachedCoverURL
	seedHistoricalCache(t, root, track.Source, track.ID)
	store, err := offline.Open(root, 64<<20, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	before, _ := store.Get(track)
	if !store.Has(track) || before.LyricsState != "pending" || before.CoverState != "pending" {
		t.Fatal("fixture must have audio and pending lyrics and cover")
	}

	ch := make(chan func(), 128)
	p := New(func(fn func()) { ch <- fn })
	t.Cleanup(p.Close)
	p.Library = store
	p.Source = func(string) music.Source { return src }
	store.OnChange(func() { p.post(p.RefreshStored) })
	p.PlayTrack([]music.Track{track}, 0)
	nativeUntil(t, ch, func() bool {
		_, loaded := p.Lyrics()
		return p.Err() != "" || p.Ready() && loaded
	})
	if p.Err() != "" {
		t.Fatal(p.Err())
	}
	if lines, _ := p.Lyrics(); len(lines) != 1 || lines[0].Text != "Recovered lyrics" {
		t.Fatalf("lyrics were not completed during cached playback: %+v", lines)
	}
	after, _ := store.Get(track)
	if after.LyricsState != "saved" || after.CoverState != "saved" || after.Cover == "" {
		t.Fatalf("assets were not saved: lyrics=%q, cover=%q", after.LyricsState, after.CoverState)
	}
	if len(after.Variants) != 1 || after.Variants[0].Path != before.Variants[0].Path {
		t.Fatal("completion changed the existing audio variant")
	}
	coverURL := store.Cover(track)
	if coverURL == "" {
		t.Fatal("saved cover is not readable")
	}
	nativeUntil(t, ch, func() bool { return p.Current().Album.ImageURL == coverURL })
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
